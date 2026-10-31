package model

import (
	"encoding/json"
	"testing"
	"time"
)

func TestLocalTimeFormatMatchesJava(t *testing.T) {
	for in, want := range map[string]string{
		"2026-07-14T10:45":    "2026-07-14T10:45",
		"2026-07-14T10:45:00": "2026-07-14T10:45",
		"2026-07-14T10:45:30": "2026-07-14T10:45:30",
		"2026-07-14 10:45":    "2026-07-14T10:45",
	} {
		v, err := ParseLocalTime(in)
		if err != nil || v.String() != want {
			t.Errorf("%s -> %s (%v), want %s", in, v, err, want)
		}
	}
}

func TestFlightTimesUseAirportZones(t *testing.T) {
	var f Flight
	if err := json.Unmarshal([]byte(`{"id":"1","airline":"IB","number":"6251","from":"MAD","to":"JFK","date":"2026-07-14",
		"depLocal":"2026-07-14T10:45","arrLocal":"2026-07-14T13:50","canceled":false,"source":"manual"}`), &f); err != nil {
		t.Fatal(err)
	}
	// 10:45 CEST (08:45Z) -> 13:50 EDT (17:50Z) is 9h05m.
	if d := f.Duration(); d != 9*time.Hour+5*time.Minute {
		t.Fatalf("duration %v", d)
	}
	if got := f.Departure().UTC().Format("15:04"); got != "08:45" {
		t.Fatalf("departure %s", got)
	}
}

func TestTrips(t *testing.T) {
	mk := func(id, from, to, dep, arr string) *Flight {
		d, _ := ParseDate(dep[:10])
		a, _ := ParseLocalTime(dep)
		b, _ := ParseLocalTime(arr)
		return &Flight{ID: id, From: from, To: to, Date: d, DepLocal: &a, ArrLocal: &b}
	}
	fs := []*Flight{
		mk("1", "BCN", "LHR", "2026-05-01T08:00", "2026-05-01T09:30"),
		mk("2", "LHR", "JFK", "2026-05-01T12:00", "2026-05-01T15:00"),
		mk("3", "JFK", "BCN", "2026-05-10T18:00", "2026-05-11T08:00"),
	}
	trips := Trips(fs)
	if len(trips) != 1 || len(trips[0].Legs) != 2 {
		t.Fatalf("trips %+v", trips)
	}
}

func TestGreatCircleEnds(t *testing.T) {
	p := GreatCircle(41.3, 2.08, 40.64, -73.78, 10)
	if len(p) != 11 || abs(p[0][0]-41.3) > 1e-6 || abs(p[10][1]+73.78) > 1e-6 {
		t.Fatalf("%v", p)
	}
	// The great circle from Barcelona to New York bulges north.
	if p[5][0] < 45 {
		t.Fatalf("midpoint lat %v", p[5][0])
	}
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
