package model

import (
	"math"
	"sort"
	"strings"
	"time"

	"github.com/lemonade1921/milehigh/internal/data"
)

func zone(code string) *time.Location {
	if a := data.AirportByCode(code); a != nil {
		return a.Location()
	}
	return time.UTC
}

func at(t *LocalTime, code string) time.Time {
	l := t.Time
	return time.Date(l.Year(), l.Month(), l.Day(), l.Hour(), l.Minute(), l.Second(), l.Nanosecond(), zone(code))
}

func first(ts ...*LocalTime) *LocalTime {
	for _, t := range ts {
		if t != nil && !t.IsZero() {
			return t
		}
	}
	return nil
}

// Departure is the best known departure: actual, else estimated, else scheduled (noon when nothing is known).
func (f *Flight) Departure() time.Time {
	if t := first(f.DepActual, f.DepEstimated, f.DepLocal); t != nil {
		return at(t, f.From)
	}
	d := f.Date.Time
	return time.Date(d.Year(), d.Month(), d.Day(), 12, 0, 0, 0, zone(f.From))
}

// ScheduledDeparture is the timetable departure, if known.
func (f *Flight) ScheduledDeparture() (time.Time, bool) {
	if f.DepLocal == nil {
		return time.Time{}, false
	}
	return at(f.DepLocal, f.From), true
}

// ScheduledArrival is the timetable arrival, if known.
func (f *Flight) ScheduledArrival() (time.Time, bool) {
	if f.ArrLocal == nil {
		return time.Time{}, false
	}
	return at(f.ArrLocal, f.To), true
}

// Arrival is the best known arrival, or an estimate from distance at ~800 km/h plus 30 min taxi.
func (f *Flight) Arrival() time.Time {
	if t := first(f.ArrActual, f.ArrEstimated, f.ArrLocal); t != nil {
		return at(t, f.ArrivalAirport())
	}
	km := f.DistanceKm()
	if km == 0 {
		km = 1000
	}
	return f.Departure().Add(time.Duration(km/800*60+30) * time.Minute)
}

// HasLanded: the wheels are down (or the arrival time has gone by).
func (f *Flight) HasLanded(now time.Time) bool { return f.Arrival().Before(now) }

// HasDeparted: the departure time has gone by.
func (f *Flight) HasDeparted(now time.Time) bool { return f.Departure().Before(now) }

// InAir: departed, not yet landed.
func (f *Flight) InAir(now time.Time) bool {
	return f.HasDeparted(now) && !f.HasLanded(now) && !f.Canceled
}

// Duration is gate to gate (or the estimate).
func (f *Flight) Duration() time.Duration {
	d := f.Arrival().Sub(f.Departure())
	if d < 0 {
		return 0
	}
	return d
}

// DistanceKm is the great-circle distance (0 when an airport is unknown).
func (f *Flight) DistanceKm() float64 {
	a, b := data.AirportByCode(f.From), data.AirportByCode(f.ArrivalAirport())
	if a == nil || b == nil {
		return 0
	}
	return DistanceKm(a.Lat, a.Lon, b.Lat, b.Lon)
}

// DelayMinutes compares best-known with scheduled times (positive = late). ok is false without both.
func (f *Flight) DepDelay() (time.Duration, bool) {
	s, ok := f.ScheduledDeparture()
	if !ok || first(f.DepActual, f.DepEstimated) == nil {
		return 0, false
	}
	return f.Departure().Sub(s), true
}

func (f *Flight) ArrDelay() (time.Duration, bool) {
	s, ok := f.ScheduledArrival()
	if !ok || first(f.ArrActual, f.ArrEstimated) == nil {
		return 0, false
	}
	return f.Arrival().Sub(s), true
}

const earthRadiusKm = 6371.0

// DistanceKm is the haversine distance between two coordinates.
func DistanceKm(lat1, lon1, lat2, lon2 float64) float64 {
	r := math.Pi / 180
	dLat := (lat2 - lat1) * r
	dLon := (lon2 - lon1) * r
	a := math.Sin(dLat/2)*math.Sin(dLat/2) + math.Cos(lat1*r)*math.Cos(lat2*r)*math.Sin(dLon/2)*math.Sin(dLon/2)
	return 2 * earthRadiusKm * math.Asin(math.Min(1, math.Sqrt(a)))
}

// GreatCircle returns n+1 points along the great circle from a to b (lat, lon pairs).
func GreatCircle(lat1, lon1, lat2, lon2 float64, n int) [][2]float64 {
	r := math.Pi / 180
	φ1, λ1, φ2, λ2 := lat1*r, lon1*r, lat2*r, lon2*r
	d := 2 * math.Asin(math.Sqrt(math.Pow(math.Sin((φ2-φ1)/2), 2)+math.Cos(φ1)*math.Cos(φ2)*math.Pow(math.Sin((λ2-λ1)/2), 2)))
	out := make([][2]float64, 0, n+1)
	if d == 0 {
		return append(out, [2]float64{lat1, lon1})
	}
	for i := 0; i <= n; i++ {
		f := float64(i) / float64(n)
		A := math.Sin((1-f)*d) / math.Sin(d)
		B := math.Sin(f*d) / math.Sin(d)
		x := A*math.Cos(φ1)*math.Cos(λ1) + B*math.Cos(φ2)*math.Cos(λ2)
		y := A*math.Cos(φ1)*math.Sin(λ1) + B*math.Cos(φ2)*math.Sin(λ2)
		z := A*math.Sin(φ1) + B*math.Sin(φ2)
		out = append(out, [2]float64{math.Atan2(z, math.Sqrt(x*x+y*y)) / r, math.Atan2(y, x) / r})
	}
	return out
}

// Trip is a run of connecting flights: each leg leaves from where the previous one landed within 24 h.
type Trip struct{ Legs []*Flight }

// Trips groups flights into trips with at least one connection (newest first).
func Trips(flights []*Flight) []Trip {
	var list []*Flight
	for _, f := range flights {
		if !f.Canceled {
			list = append(list, f)
		}
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Departure().Before(list[j].Departure()) })
	var trips []Trip
	var cur []*Flight
	for _, f := range list {
		if n := len(cur); n > 0 {
			prev := cur[n-1]
			gap := f.Departure().Sub(prev.Arrival())
			if !(strings.EqualFold(prev.ArrivalAirport(), f.From) && gap >= 0 && gap <= 24*time.Hour) {
				if len(cur) > 1 {
					trips = append(trips, Trip{cur})
				}
				cur = nil
			}
		}
		cur = append(cur, f)
	}
	if len(cur) > 1 {
		trips = append(trips, Trip{cur})
	}
	sort.Slice(trips, func(i, j int) bool { return trips[i].Legs[0].Departure().After(trips[j].Legs[0].Departure()) })
	return trips
}

// SortFlights orders flights by departure (oldest first).
func SortFlights(fs []*Flight) {
	sort.SliceStable(fs, func(i, j int) bool { return fs[i].Departure().Before(fs[j].Departure()) })
}
