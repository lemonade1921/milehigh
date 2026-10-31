// Package data holds the open datasets milehigh ships with, so it works fully offline:
// airports, airlines and aircraft from OpenFlights (ODbL) and OurAirports (public domain),
// country outlines from Natural Earth (public domain) and the route index built from
// Virtual Radar Server's open route data (CC0). See NOTICE and tools/gendata.py.
package data

import (
	"bufio"
	"bytes"
	"compress/gzip"
	_ "embed"
	"encoding/binary"
	"io"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"
	_ "time/tzdata" // airport time zones work even where the OS has no tz database
)

//go:embed airports.tsv.gz
var airportsGz []byte

//go:embed airlines.tsv.gz
var airlinesGz []byte

//go:embed planes.tsv.gz
var planesGz []byte

//go:embed countries.tsv.gz
var countriesGz []byte

//go:embed world.bin.gz
var worldGz []byte

//go:embed routes.tsv.gz
var routesGz []byte

func gunzip(b []byte) []byte {
	r, err := gzip.NewReader(bytes.NewReader(b))
	if err != nil {
		panic(err)
	}
	out, err := io.ReadAll(r)
	if err != nil {
		panic(err)
	}
	return out
}

func lines(b []byte, fn func([]string)) {
	sc := bufio.NewScanner(bytes.NewReader(gunzip(b)))
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		fn(strings.Split(sc.Text(), "\t"))
	}
}

// Airport is one airport with an IATA code.
type Airport struct {
	IATA, ICAO, Name, City, Country string // Country is an ISO 3166 code
	Lat, Lon                        float64
	TZ                              string
	loc                             *time.Location
	locOnce                         sync.Once
}

// Location is the airport's time zone (UTC when unknown).
func (a *Airport) Location() *time.Location {
	a.locOnce.Do(func() {
		a.loc = time.UTC
		if a.TZ != "" {
			if l, err := time.LoadLocation(a.TZ); err == nil {
				a.loc = l
			}
		}
	})
	return a.loc
}

// Airline is an airline from OpenFlights.
type Airline struct {
	IATA, ICAO, Name, Callsign, Country string
	Active                              bool
	seed                                bool
}

// Country is an ISO country with its English name and continent code (EU, AS, NA, SA, AF, OC, AN).
type Country struct {
	ISO, Name, Continent string
}

var (
	loadOnce  sync.Once
	airports  map[string]*Airport
	byICAO    map[string]*Airport
	airlines  []*Airline
	alByIATA  map[string]*Airline
	alByICAO  map[string]*Airline
	planes    map[string]string
	countries map[string]*Country
)

func load() {
	loadOnce.Do(func() {
		airports = map[string]*Airport{}
		byICAO = map[string]*Airport{}
		lines(airportsGz, func(c []string) {
			if len(c) < 8 {
				return
			}
			lat, _ := strconv.ParseFloat(c[5], 64)
			lon, _ := strconv.ParseFloat(c[6], 64)
			a := &Airport{IATA: c[0], ICAO: c[1], Name: c[2], City: c[3], Country: c[4], Lat: lat, Lon: lon, TZ: c[7]}
			airports[a.IATA] = a
			if a.ICAO != "" {
				byICAO[a.ICAO] = a
			}
		})
		alByIATA = map[string]*Airline{}
		alByICAO = map[string]*Airline{}
		lines(airlinesGz, func(c []string) {
			if len(c) < 6 {
				return
			}
			a := &Airline{IATA: c[0], ICAO: c[1], Name: c[2], Callsign: c[3], Country: c[4], Active: c[5] != "N", seed: c[5] == "S"}
			airlines = append(airlines, a)
			// OpenFlights reuses codes of defunct airlines: the curated seed wins, then the last active row.
			win := func(old *Airline) bool { return old == nil || a.seed || (a.Active && !old.seed) }
			if a.IATA != "" && win(alByIATA[a.IATA]) {
				alByIATA[a.IATA] = a
			}
			if a.ICAO != "" && win(alByICAO[a.ICAO]) {
				alByICAO[a.ICAO] = a
			}
		})
		planes = map[string]string{}
		lines(planesGz, func(c []string) {
			if len(c) < 3 {
				return
			}
			for _, code := range c[1:] {
				if code != "" {
					planes[strings.ToUpper(code)] = c[0]
				}
			}
		})
		countries = map[string]*Country{}
		lines(countriesGz, func(c []string) {
			if len(c) >= 3 {
				countries[c[0]] = &Country{ISO: c[0], Name: c[1], Continent: c[2]}
			}
		})
	})
}

// AirportByCode finds an airport by IATA (3 letters) or ICAO (4 letters) code.
func AirportByCode(code string) *Airport {
	load()
	code = strings.ToUpper(strings.TrimSpace(code))
	if a := airports[code]; a != nil {
		return a
	}
	return byICAO[code]
}

// AddAirport registers an airport learned from an online lookup (doesn't replace known ones).
func AddAirport(a *Airport) {
	load()
	if a == nil || a.IATA == "" || airports[a.IATA] != nil {
		return
	}
	airports[a.IATA] = a
}

// SearchAirports matches code, city or name (case-insensitive), best matches first.
func SearchAirports(q string, limit int) []*Airport {
	load()
	q = strings.ToLower(strings.TrimSpace(q))
	if q == "" {
		return nil
	}
	var exact, prefix, other []*Airport
	for _, a := range airports {
		code := strings.ToLower(a.IATA)
		switch {
		case code == q || strings.ToLower(a.ICAO) == q:
			exact = append(exact, a)
		case strings.HasPrefix(strings.ToLower(a.City), q):
			prefix = append(prefix, a)
		case strings.Contains(strings.ToLower(a.Name), q) || strings.Contains(strings.ToLower(a.City), q):
			other = append(other, a)
		}
	}
	sortAirports(prefix)
	sortAirports(other)
	out := append(append(exact, prefix...), other...)
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

func sortAirports(a []*Airport) {
	// Airports with an ICAO code and "International" first, then alphabetical: a cheap proxy for size.
	score := func(x *Airport) int {
		s := 0
		if x.ICAO != "" {
			s++
		}
		if strings.Contains(x.Name, "International") {
			s += 2
		}
		return s
	}
	for i := 1; i < len(a); i++ {
		for j := i; j > 0; j-- {
			si, sj := score(a[j]), score(a[j-1])
			if si > sj || (si == sj && a[j].IATA < a[j-1].IATA) {
				a[j], a[j-1] = a[j-1], a[j]
			} else {
				break
			}
		}
	}
}

// AirlineByCode finds an airline by IATA (2 chars) or ICAO (3 chars) code.
func AirlineByCode(code string) *Airline {
	load()
	code = strings.ToUpper(strings.TrimSpace(code))
	if len(code) == 3 {
		if a := alByICAO[code]; a != nil {
			return a
		}
	}
	if a := alByIATA[code]; a != nil {
		return a
	}
	return alByICAO[code]
}

// AirlineName is the airline's name, or the code itself when unknown.
func AirlineName(code string) string {
	if a := AirlineByCode(code); a != nil {
		return a.Name
	}
	return code
}

// AircraftName turns an IATA/ICAO type code ("320", "A20N") into a name; anything else is returned as is.
func AircraftName(s string) string {
	load()
	if n, ok := planes[strings.ToUpper(strings.TrimSpace(s))]; ok {
		return n
	}
	return s
}

// CountryByISO returns the country for an ISO code (nil when unknown).
func CountryByISO(iso string) *Country {
	load()
	return countries[strings.ToUpper(iso)]
}

// Countries lists every known country.
func Countries() []*Country {
	load()
	out := make([]*Country, 0, len(countries))
	for _, c := range countries {
		out = append(out, c)
	}
	return out
}

// Flag turns "ES" into the 🇪🇸 emoji.
func Flag(iso string) string {
	if len(iso) != 2 {
		return "  "
	}
	r := []rune(strings.ToUpper(iso))
	if r[0] < 'A' || r[0] > 'Z' || r[1] < 'A' || r[1] > 'Z' {
		return "  "
	}
	return string([]rune{0x1F1E6 + (r[0] - 'A'), 0x1F1E6 + (r[1] - 'A')})
}

// World is the country raster: one byte per cell, 0 for sea.
type World struct {
	W, H  int
	Codes []string // cell value n -> Codes[n-1]
	Cells []byte
}

var (
	worldOnce sync.Once
	world     *World
)

// WorldMap returns the 0.25° country raster.
func WorldMap() *World {
	worldOnce.Do(func() {
		b := gunzip(worldGz)
		w := int(binary.LittleEndian.Uint16(b[0:]))
		h := int(binary.LittleEndian.Uint16(b[2:]))
		n := int(binary.LittleEndian.Uint16(b[4:]))
		world = &World{W: w, H: h, Codes: strings.Split(string(b[6:6+n]), ","), Cells: b[6+n:]}
	})
	return world
}

// At returns the ISO code of the country at a coordinate ("" for sea).
func (w *World) At(lat, lon float64) string {
	v := w.Value(lat, lon)
	if v == 0 {
		return ""
	}
	return w.Codes[v-1]
}

// Value returns the raw cell value at a coordinate.
func (w *World) Value(lat, lon float64) byte {
	x := int(math.Floor((lon + 180) / 360 * float64(w.W)))
	y := int(math.Floor((90 - lat) / 180 * float64(w.H)))
	x = ((x % w.W) + w.W) % w.W
	if y < 0 || y >= w.H {
		return 0
	}
	return w.Cells[y*w.W+x]
}

// Route is a flight number known to fly between two airports.
type Route struct{ Airline, Number string }

var (
	routesOnce sync.Once
	routes     map[string][]string
)

// RoutesBetween lists flight numbers known to fly from -> to (routes, not timetables).
func RoutesBetween(from, to string) []Route {
	routesOnce.Do(func() {
		routes = map[string][]string{}
		lines(routesGz, func(c []string) {
			if len(c) >= 3 {
				routes[c[0]+c[1]] = strings.Split(c[2], ",")
			}
		})
	})
	var out []Route
	for _, code := range routes[strings.ToUpper(from)+strings.ToUpper(to)] {
		i := 0
		for i < len(code) && !(code[i] >= '0' && code[i] <= '9' && i >= 2) {
			i++
		}
		if i == 0 || i >= len(code) {
			continue
		}
		out = append(out, Route{Airline: code[:i], Number: code[i:]})
	}
	return out
}
