package stats

import (
	"sort"
	"time"

	"github.com/lemonade1921/milehigh/internal/data"
	"github.com/lemonade1921/milehigh/internal/model"
)

// WorldCountries is the usual count of the world's countries (UN members and observers).
const WorldCountries = 195

// ContinentOrder is the order continents are listed in.
var ContinentOrder = []string{"EU", "AS", "NA", "SA", "AF", "OC", "AN"}

// CountryVisit is everything the passport knows about one country.
type CountryVisit struct {
	ISO, Name, Continent string

	Flights       int // flights flown that touch the country
	International int // of those, crossing a border
	Domestic      int // of those, staying inside it
	First, Last   *model.Flight
	EntryAirport  string // where the first flight touched the country

	Next     *model.Flight // next upcoming flight touching it
	Upcoming int

	Airports []Count // IATA codes in this country, by flights
	Airlines []Count // airline codes, by flights
	Partners []Count // other countries flown to or from, by flights

	Manual  bool // visited only because it was added by hand
	Planned bool // only reached by upcoming flights
}

// ContinentProgress is how many of a continent's countries have been visited.
type ContinentProgress struct {
	Code, Name     string
	Visited, Total int
}

// YearCount is how many countries were visited for the first time in a year.
type YearCount struct{ Year, N int }

// PassportStats is the whole passport.
type PassportStats struct {
	Visited                 []*CountryVisit // most flights first
	Planned                 []*CountryVisit // soonest first
	Continents              []ContinentProgress
	NewByYear               []YearCount // oldest year first
	International, Domestic int
}

// Lookup finds a country in the passport (visited or planned).
func (p *PassportStats) Lookup(iso string) *CountryVisit {
	for _, l := range [][]*CountryVisit{p.Visited, p.Planned} {
		for _, c := range l {
			if c.ISO == iso {
				return c
			}
		}
	}
	return nil
}

// ContinentsVisited counts continents with at least one visited country.
func (p *PassportStats) ContinentsVisited() int {
	n := 0
	for _, c := range p.Continents {
		if c.Visited > 0 {
			n++
		}
	}
	return n
}

// NewIn is how many countries were first visited in a year.
func (p *PassportStats) NewIn(year int) int {
	for _, y := range p.NewByYear {
		if y.Year == year {
			return y.N
		}
	}
	return 0
}

func countryOf(code string) string {
	if a := data.AirportByCode(code); a != nil {
		return a.Country
	}
	return ""
}

// Passport works out the passport from the flights and the visited-country edits, using the same
// rules as Countries (and Skyline): flights before today count as visits, later ones as plans.
func Passport(all []*model.Flight, added, removed map[string]bool, now time.Time) *PassportStats {
	sum := Countries(all, added, removed, now)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	type acc struct {
		v                            *CountryVisit
		airports, airlines, partners map[string]int
		futAirports                  map[string]int
	}
	byISO := map[string]*acc{}
	get := func(iso string) *acc {
		a := byISO[iso]
		if a == nil {
			name, cont := iso, ""
			if c := data.CountryByISO(iso); c != nil {
				name, cont = c.Name, c.Continent
			}
			a = &acc{v: &CountryVisit{ISO: iso, Name: name, Continent: cont},
				airports: map[string]int{}, airlines: map[string]int{}, partners: map[string]int{}, futAirports: map[string]int{}}
			byISO[iso] = a
		}
		return a
	}

	flights := append([]*model.Flight(nil), all...)
	model.SortFlights(flights)
	p := &PassportStats{}
	for _, f := range flights {
		if f.Canceled {
			continue
		}
		from, to := f.From, f.ArrivalAirport()
		cf, ct := countryOf(from), countryOf(to)
		past := f.Date.Before(today)
		if past && cf != "" && ct != "" {
			if cf == ct {
				p.Domestic++
			} else {
				p.International++
			}
		}
		for _, side := range []struct{ iso, code, other string }{{cf, from, ct}, {ct, to, cf}} {
			if side.iso == "" {
				continue
			}
			a := get(side.iso)
			if side.iso == cf && side.iso == ct && side.code == to {
				// Domestic flight: count the flight once, but both airports.
				if past {
					a.airports[side.code]++
				} else {
					a.futAirports[side.code]++
				}
				continue
			}
			if past {
				a.v.Flights++
				if side.other != "" && side.other != side.iso {
					a.v.International++
					a.partners[side.other]++
				} else if side.other == side.iso {
					a.v.Domestic++
				}
				if a.v.First == nil {
					a.v.First = f
					a.v.EntryAirport = side.code
				}
				a.v.Last = f
				a.airports[side.code]++
				if f.Airline != "" {
					a.airlines[canonicalAirline(f.Airline)]++
				}
			} else {
				a.v.Upcoming++
				if a.v.Next == nil {
					a.v.Next = f
				}
				a.futAirports[side.code]++
			}
		}
	}

	finish := func(iso string, planned bool) *CountryVisit {
		a := get(iso)
		v := a.v
		v.Planned = planned
		v.Manual = !planned && v.Flights == 0
		airports := a.airports
		if planned {
			airports = a.futAirports
		}
		v.Airports = top(airports, func(k string) string {
			if ap := data.AirportByCode(k); ap != nil {
				return ap.City
			}
			return ""
		})
		v.Airlines = top(a.airlines, data.AirlineName)
		v.Partners = top(a.partners, CountryName)
		return v
	}
	for iso := range sum.Visited {
		p.Visited = append(p.Visited, finish(iso, false))
	}
	for iso := range sum.Planned {
		p.Planned = append(p.Planned, finish(iso, true))
	}
	sort.Slice(p.Visited, func(i, j int) bool {
		a, b := p.Visited[i], p.Visited[j]
		if a.Flights != b.Flights {
			return a.Flights > b.Flights
		}
		return a.Name < b.Name
	})
	sort.Slice(p.Planned, func(i, j int) bool {
		a, b := p.Planned[i], p.Planned[j]
		if a.Next != nil && b.Next != nil && !a.Next.Departure().Equal(b.Next.Departure()) {
			return a.Next.Departure().Before(b.Next.Departure())
		}
		return a.Name < b.Name
	})

	// Continents: visited out of every country we know on that continent.
	totals := map[string]int{}
	for _, c := range data.Countries() {
		if c.Continent != "" {
			totals[c.Continent]++
		}
	}
	visited := map[string]int{}
	years := map[int]int{}
	for _, v := range p.Visited {
		visited[v.Continent]++
		if v.First != nil {
			years[v.First.Date.Year()]++
		}
	}
	for _, code := range ContinentOrder {
		p.Continents = append(p.Continents, ContinentProgress{Code: code, Name: ContinentName(code), Visited: visited[code], Total: totals[code]})
	}
	for y, n := range years {
		p.NewByYear = append(p.NewByYear, YearCount{y, n})
	}
	sort.Slice(p.NewByYear, func(i, j int) bool { return p.NewByYear[i].Year < p.NewByYear[j].Year })
	return p
}
