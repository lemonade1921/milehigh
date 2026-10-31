// Package stats works out the numbers behind the Stats and Passport screens.
package stats

import (
	"sort"
	"strings"
	"time"

	"github.com/lemonade1921/milehigh/internal/data"
	"github.com/lemonade1921/milehigh/internal/model"
)

const (
	EarthCircumferenceKm = 40075.0
	MoonDistanceKm       = 384400.0
	KmPerMile            = 1.609344
)

// Count is a label with how many flights it covers.
type Count struct {
	Key, Label string
	N          int
}

// Stats for a set of flights.
type Stats struct {
	Flights    int
	DistanceKm float64
	Airtime    time.Duration
	Airports   []Count
	Airlines   []Count
	Routes     []Count
	Countries  []Count
	Continents []Count
	Aircraft   []Count
	Longest    *model.Flight
	Shortest   *model.Flight
	Years      []int // every year with flights, newest first
}

// Compute works out stats for flights that have happened (year 0 = all time).
func Compute(all []*model.Flight, year int, now time.Time) Stats {
	var s Stats
	years := map[int]bool{}
	airports, airlines, routes, countries, continents, aircraft := map[string]int{}, map[string]int{}, map[string]int{}, map[string]int{}, map[string]int{}, map[string]int{}
	for _, f := range all {
		if f.Canceled || !f.HasLanded(now) {
			continue
		}
		years[f.Date.Year()] = true
		if year != 0 && f.Date.Year() != year {
			continue
		}
		s.Flights++
		km := f.DistanceKm()
		s.DistanceKm += km
		s.Airtime += f.Duration()
		if km > 0 {
			if s.Longest == nil || km > s.Longest.DistanceKm() {
				s.Longest = f
			}
			if s.Shortest == nil || km < s.Shortest.DistanceKm() {
				s.Shortest = f
			}
		}
		to := f.ArrivalAirport()
		airports[f.From]++
		airports[to]++
		if f.Airline != "" {
			airlines[canonicalAirline(f.Airline)]++
		}
		a, b := f.From, to
		if a > b {
			a, b = b, a
		}
		routes[a+"–"+b]++
		seen := map[string]bool{}
		for _, code := range []string{f.From, to} {
			if ap := data.AirportByCode(code); ap != nil && ap.Country != "" && !seen[ap.Country] {
				seen[ap.Country] = true
				countries[ap.Country]++
				if c := data.CountryByISO(ap.Country); c != nil && c.Continent != "" {
					continents[c.Continent]++
				}
			}
		}
		if t := strings.TrimSpace(model.Str(f.Aircraft)); t != "" {
			aircraft[data.AircraftName(t)]++
		}
	}
	for y := range years {
		s.Years = append(s.Years, y)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(s.Years)))
	s.Airports = top(airports, func(k string) string {
		if a := data.AirportByCode(k); a != nil {
			return a.City
		}
		return ""
	})
	s.Airlines = top(airlines, data.AirlineName)
	s.Routes = top(routes, func(string) string { return "" })
	s.Countries = top(countries, CountryName)
	s.Continents = top(continents, ContinentName)
	s.Aircraft = top(aircraft, func(string) string { return "" })
	return s
}

// canonicalAirline maps ICAO codes to IATA where we know both, so "VLG" and "VY" count together.
func canonicalAirline(code string) string {
	if a := data.AirlineByCode(code); a != nil && a.IATA != "" {
		return a.IATA
	}
	return strings.ToUpper(code)
}

func top(m map[string]int, label func(string) string) []Count {
	out := make([]Count, 0, len(m))
	for k, n := range m {
		out = append(out, Count{Key: k, Label: label(k), N: n})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].N != out[j].N {
			return out[i].N > out[j].N
		}
		return out[i].Key < out[j].Key
	})
	return out
}

// CountryName is the English name for an ISO code.
func CountryName(iso string) string {
	if c := data.CountryByISO(iso); c != nil {
		return c.Name
	}
	return iso
}

// ContinentName spells out a continent code.
func ContinentName(code string) string {
	return map[string]string{"EU": "Europe", "AS": "Asia", "NA": "North America", "SA": "South America",
		"AF": "Africa", "OC": "Oceania", "AN": "Antarctica"}[code]
}

// CountrySummary: visited = countries of past flights' airports plus manual additions minus
// removals; planned = countries only reached by upcoming flights. ISO -> number of flights.
// Same rules as Skyline.
type CountrySummary struct {
	Visited, Planned map[string]int
}

// Countries summarises visited and planned countries.
func Countries(all []*model.Flight, added, removed map[string]bool, now time.Time) CountrySummary {
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	past, future := map[string]int{}, map[string]int{}
	for _, f := range all {
		if f.Canceled {
			continue
		}
		target := future
		if f.Date.Before(today) {
			target = past
		}
		seen := map[string]bool{}
		for _, code := range []string{f.From, f.ArrivalAirport()} {
			if a := data.AirportByCode(code); a != nil && a.Country != "" && !seen[a.Country] {
				seen[a.Country] = true
				target[a.Country]++
			}
		}
	}
	visited := map[string]int{}
	for k, n := range past {
		if !removed[k] {
			visited[k] = n
		}
	}
	for k := range added {
		if _, ok := visited[k]; !ok && !removed[k] {
			visited[k] = 0
		}
	}
	planned := map[string]int{}
	for k, n := range future {
		if _, ok := visited[k]; !ok && !removed[k] {
			planned[k] = n
		}
	}
	return CountrySummary{Visited: visited, Planned: planned}
}
