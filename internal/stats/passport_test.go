package stats

import (
	"testing"
	"time"

	"github.com/lemonade1921/milehigh/internal/model"
)

func flight(from, to, dep string) *model.Flight {
	d, _ := model.ParseDate(dep[:10])
	t, _ := model.ParseLocalTime(dep)
	return &model.Flight{ID: from + to + dep, Airline: "VY", Number: "1", From: from, To: to, Date: d, DepLocal: &t}
}

func TestPassport(t *testing.T) {
	now := time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)
	fs := []*model.Flight{
		flight("BCN", "LHR", "2025-03-02T07:10"),
		flight("LHR", "BCN", "2025-03-09T19:00"),
		flight("BCN", "MAD", "2026-01-05T09:00"), // domestic
		flight("MAD", "EZE", "2026-12-20T23:55"), // planned
	}
	p := Passport(fs, map[string]bool{"AD": true}, nil, now)
	es := p.Lookup("ES")
	if es == nil || es.Flights != 3 || es.International != 2 || es.Domestic != 1 {
		t.Fatalf("Spain: %+v", es)
	}
	if es.First != fs[0] || es.Last != fs[2] || es.Next != fs[3] || es.EntryAirport != "BCN" {
		t.Fatalf("Spain visits: first %v last %v next %v entry %s", es.First, es.Last, es.Next, es.EntryAirport)
	}
	if len(es.Airports) != 2 || es.Airports[0].Key != "BCN" || es.Airports[0].N != 3 {
		t.Fatalf("Spain airports %+v", es.Airports)
	}
	if len(es.Partners) != 1 || es.Partners[0].Key != "GB" || es.Partners[0].N != 2 {
		t.Fatalf("Spain partners %+v", es.Partners)
	}
	if ad := p.Lookup("AD"); ad == nil || !ad.Manual {
		t.Fatalf("Andorra should be added by hand: %+v", ad)
	}
	if ar := p.Lookup("AR"); ar == nil || !ar.Planned || ar.Next != fs[3] {
		t.Fatalf("Argentina should be planned: %+v", ar)
	}
	if p.International != 2 || p.Domestic != 1 {
		t.Fatalf("flights %d international, %d domestic", p.International, p.Domestic)
	}
	if p.NewIn(2025) != 2 || p.ContinentsVisited() != 1 {
		t.Fatalf("new in 2025 %d, continents %d", p.NewIn(2025), p.ContinentsVisited())
	}
	if len(p.Visited) != 3 || p.Visited[0].ISO != "ES" {
		t.Fatalf("visited order %v", p.Visited)
	}
}
