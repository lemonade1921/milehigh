// Package online talks to open, key-less flight and weather services (the same ones Skyline uses):
//
//	adsbdb.com     route of a flight number (open source, community ADS-B data)
//	adsb.lol       live aircraft positions (ODbL)
//	open-meteo.com weather forecasts (CC BY 4.0)
//
// and, only when you add your own key, AeroDataBox for schedules, delays and gates.
package online

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/lemonade1921/milehigh/internal/data"
	"github.com/lemonade1921/milehigh/internal/model"
)

// Version is set at build time and sent in the User-Agent.
var Version = "dev"

var client = &http.Client{Timeout: 15 * time.Second}

var (
	ErrNotFound     = errors.New("not found")
	ErrUnauthorized = errors.New("the key was refused")
	ErrRateLimited  = errors.New("rate limited, try again later")
)

func get(url string, headers map[string]string, into any) error {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "milehigh/"+Version+" (+https://github.com/lemonade1921/milehigh)")
	req.Header.Set("Accept", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == 404:
		return ErrNotFound
	case resp.StatusCode == 401 || resp.StatusCode == 403:
		return ErrUnauthorized
	case resp.StatusCode == 429:
		return ErrRateLimited
	case resp.StatusCode == 204:
		return ErrNotFound
	case resp.StatusCode >= 300:
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return err
	}
	return json.Unmarshal(b, into)
}

// Callsign is the ICAO callsign an aircraft broadcasts: IB 6251 -> IBE6251.
func Callsign(airline, number string) string {
	icao := ""
	if a := data.AirlineByCode(airline); a != nil {
		icao = a.ICAO
	}
	if icao == "" && len(strings.TrimSpace(airline)) == 3 {
		icao = strings.ToUpper(strings.TrimSpace(airline))
	}
	n := strings.TrimLeft(strings.TrimSpace(number), "0")
	if icao == "" || n == "" {
		return ""
	}
	return icao + n
}

var flightNo = regexp.MustCompile(`^\s*([A-Z0-9]{2,3}?)\s*(\d{1,5}[A-Z]?)\s*$`)

// SplitNumber splits "BA 473" / "ba0473" into ("BA", "473").
func SplitNumber(s string) (airline, number string, ok bool) {
	m := flightNo.FindStringSubmatch(strings.ToUpper(s))
	if m == nil {
		return "", "", false
	}
	n := strings.TrimLeft(m[2], "0")
	if n == "" {
		n = "0"
	}
	return m[1], n, true
}

// Route is where a flight number flies, according to adsbdb.
type Route struct {
	From, To    string
	AirlineName string
	AirlineIATA string
}

// LookupRoute finds the usual route of a flight number.
func LookupRoute(airline, number string) (*Route, error) {
	cs := Callsign(airline, number)
	if cs == "" {
		return nil, ErrNotFound
	}
	var r struct {
		Response struct {
			FlightRoute *struct {
				Airline *struct {
					Name string `json:"name"`
					IATA string `json:"iata"`
				} `json:"airline"`
				Origin      adsbdbAirport `json:"origin"`
				Destination adsbdbAirport `json:"destination"`
			} `json:"flightroute"`
		} `json:"response"`
	}
	if err := get("https://api.adsbdb.com/v0/callsign/"+cs, nil, &r); err != nil {
		return nil, err
	}
	fr := r.Response.FlightRoute
	if fr == nil || fr.Origin.IATA == "" || fr.Destination.IATA == "" {
		return nil, ErrNotFound
	}
	fr.Origin.register()
	fr.Destination.register()
	out := &Route{From: fr.Origin.IATA, To: fr.Destination.IATA}
	if fr.Airline != nil {
		out.AirlineName, out.AirlineIATA = fr.Airline.Name, fr.Airline.IATA
	}
	return out, nil
}

type adsbdbAirport struct {
	IATA string  `json:"iata_code"`
	ICAO string  `json:"icao_code"`
	Name string  `json:"name"`
	City string  `json:"municipality"`
	ISO  string  `json:"country_iso_name"`
	Lat  float64 `json:"latitude"`
	Lon  float64 `json:"longitude"`
}

func (a adsbdbAirport) register() {
	if len(a.IATA) == 3 && data.AirportByCode(a.IATA) == nil {
		data.AddAirport(&data.Airport{IATA: a.IATA, ICAO: a.ICAO, Name: a.Name, City: a.City, Country: a.ISO, Lat: a.Lat, Lon: a.Lon})
	}
}

// Position is where an aircraft is right now.
type Position struct {
	Lat, Lon           float64
	AltitudeFt         int // 0 when on the ground or unknown
	SpeedKt            float64
	Track              float64
	OnGround           bool
	Source             string
	Registration, Type string
	Time               time.Time
}

// LivePosition asks adsb.lol where the flight is (nil when no receiver hears it).
func LivePosition(f *model.Flight) (*Position, error) {
	cs := Callsign(f.Airline, f.Number)
	if cs == "" {
		return nil, ErrNotFound
	}
	var r struct {
		AC []struct {
			Lat   *float64        `json:"lat"`
			Lon   *float64        `json:"lon"`
			Alt   json.RawMessage `json:"alt_baro"`
			GS    *float64        `json:"gs"`
			Track *float64        `json:"track"`
			Reg   string          `json:"r"`
			Type  string          `json:"t"`
		} `json:"ac"`
	}
	if err := get("https://api.adsb.lol/v2/callsign/"+cs, nil, &r); err != nil {
		return nil, err
	}
	for _, a := range r.AC {
		if a.Lat == nil || a.Lon == nil {
			continue
		}
		p := &Position{Lat: *a.Lat, Lon: *a.Lon, Source: "adsb.lol", Registration: a.Reg, Type: a.Type, Time: time.Now()}
		if string(a.Alt) == `"ground"` {
			p.OnGround = true
		} else if n, err := strconv.ParseFloat(string(a.Alt), 64); err == nil {
			p.AltitudeFt = int(n)
		}
		if a.GS != nil {
			p.SpeedKt = *a.GS
		}
		if a.Track != nil {
			p.Track = *a.Track
		}
		return p, nil
	}
	return nil, ErrNotFound
}

// Weather at one place and time.
type Weather struct {
	Celsius float64
	Code    int
	IsDay   bool
}

// WeatherAt takes Open-Meteo's forecast for the hour closest to t.
func WeatherAt(lat, lon float64, t time.Time) (*Weather, error) {
	var r struct {
		Hourly struct {
			Time  []string  `json:"time"`
			Temp  []float64 `json:"temperature_2m"`
			Code  []int     `json:"weather_code"`
			IsDay []int     `json:"is_day"`
		} `json:"hourly"`
	}
	url := fmt.Sprintf("https://api.open-meteo.com/v1/forecast?latitude=%.4f&longitude=%.4f"+
		"&hourly=temperature_2m,weather_code,is_day&timezone=GMT&past_days=1&forecast_days=7", lat, lon)
	if err := get(url, nil, &r); err != nil {
		return nil, err
	}
	best, bestD := -1, math.MaxFloat64
	for i, s := range r.Hourly.Time {
		ht, err := time.Parse("2006-01-02T15:04", s)
		if err != nil {
			continue
		}
		if d := math.Abs(ht.Sub(t).Hours()); d < bestD {
			best, bestD = i, d
		}
	}
	if best < 0 || bestD > 3 || best >= len(r.Hourly.Temp) || best >= len(r.Hourly.Code) {
		return nil, ErrNotFound
	}
	w := &Weather{Celsius: r.Hourly.Temp[best], Code: r.Hourly.Code[best], IsDay: true}
	if best < len(r.Hourly.IsDay) {
		w.IsDay = r.Hourly.IsDay[best] == 1
	}
	return w, nil
}

// WeatherText describes a WMO weather code with a symbol.
func WeatherText(code int, day bool) string {
	sun := "☀"
	if !day {
		sun = "☾"
	}
	switch {
	case code == 0:
		return sun + " Clear"
	case code <= 2:
		return "⛅ Partly cloudy"
	case code == 3:
		return "☁ Overcast"
	case code == 45 || code == 48:
		return "🌫 Fog"
	case code >= 51 && code <= 57:
		return "🌦 Drizzle"
	case code >= 61 && code <= 67, code >= 80 && code <= 82:
		return "🌧 Rain"
	case code >= 71 && code <= 77, code == 85 || code == 86:
		return "🌨 Snow"
	case code >= 95:
		return "⛈ Thunderstorm"
	}
	return "·"
}
