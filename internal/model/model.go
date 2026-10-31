// Package model is the flight log, in exactly the shape Skyline (the Android app) stores and
// backs it up, so the two can share one backup file.
package model

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Date is a calendar date, serialised like java.time.LocalDate ("2026-10-31").
type Date struct{ time.Time }

// LocalTime is a wall-clock time at an airport, serialised like java.time.LocalDateTime
// ("2026-10-31T14:05", with seconds only when they aren't zero).
type LocalTime struct{ time.Time }

const (
	dateLayout = "2006-01-02"
)

// ParseDate reads "2026-10-31".
func ParseDate(s string) (Date, error) {
	t, err := time.Parse(dateLayout, strings.TrimSpace(s))
	return Date{t}, err
}

// ParseLocalTime reads "2026-10-31T14:05", "2026-10-31T14:05:30(.nnn)" or "2026-10-31 14:05".
func ParseLocalTime(s string) (LocalTime, error) {
	s = strings.Replace(strings.TrimSpace(s), " ", "T", 1)
	for _, l := range []string{"2006-01-02T15:04:05.999999999", "2006-01-02T15:04:05", "2006-01-02T15:04"} {
		if t, err := time.Parse(l, s); err == nil {
			return LocalTime{t}, nil
		}
	}
	return LocalTime{}, fmt.Errorf("bad local time %q", s)
}

func (d Date) String() string { return d.Format(dateLayout) }

func (t LocalTime) String() string {
	switch {
	case t.Nanosecond() != 0:
		return t.Format("2006-01-02T15:04:05.000000000")
	case t.Second() != 0:
		return t.Format("2006-01-02T15:04:05")
	default:
		return t.Format("2006-01-02T15:04")
	}
}

func (d Date) MarshalJSON() ([]byte, error) { return json.Marshal(d.String()) }
func (d *Date) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	v, err := ParseDate(s)
	*d = v
	return err
}

func (t LocalTime) MarshalJSON() ([]byte, error) { return json.Marshal(t.String()) }
func (t *LocalTime) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	v, err := ParseLocalTime(s)
	*t = v
	return err
}

// BoardingPass is a barcode kept exactly as issued.
type BoardingPass struct {
	Format        string  `json:"format"` // QR_CODE, AZTEC or PDF_417
	Message       string  `json:"message"`
	Source        string  `json:"source"` // "pkpass" or "screenshot"
	PassengerName *string `json:"passengerName,omitempty"`
	Seat          *string `json:"seat,omitempty"`
	Sequence      *string `json:"sequence,omitempty"`
	Cabin         *string `json:"cabin,omitempty"`
}

// Flight mirrors app.skyline.data.Flight field for field.
type Flight struct {
	ID           string        `json:"id"`
	Airline      string        `json:"airline"`
	Number       string        `json:"number"`
	From         string        `json:"from"`
	To           string        `json:"to"`
	Date         Date          `json:"date"`
	DepLocal     *LocalTime    `json:"depLocal,omitempty"`
	ArrLocal     *LocalTime    `json:"arrLocal,omitempty"`
	DepEstimated *LocalTime    `json:"depEstimated,omitempty"`
	ArrEstimated *LocalTime    `json:"arrEstimated,omitempty"`
	DepActual    *LocalTime    `json:"depActual,omitempty"`
	ArrActual    *LocalTime    `json:"arrActual,omitempty"`
	DepTerminal  *string       `json:"depTerminal,omitempty"`
	DepGate      *string       `json:"depGate,omitempty"`
	ArrTerminal  *string       `json:"arrTerminal,omitempty"`
	ArrGate      *string       `json:"arrGate,omitempty"`
	Aircraft     *string       `json:"aircraft,omitempty"`
	Tail         *string       `json:"tail,omitempty"`
	Seat         *string       `json:"seat,omitempty"`
	Cabin        *string       `json:"cabin,omitempty"`
	PNR          *string       `json:"pnr,omitempty"`
	Notes        *string       `json:"notes,omitempty"`
	Canceled     bool          `json:"canceled"`
	DivertedTo   *string       `json:"divertedTo,omitempty"`
	BoardingPass *BoardingPass `json:"boardingPass,omitempty"`
	Source       string        `json:"source"` // "manual", "flighty", "pkpass", "screenshot", "milehigh"

	// Fields a newer Skyline wrote that this version doesn't know: kept so they survive a round trip.
	Extra map[string]json.RawMessage `json:"-"`
}

type flightAlias Flight

var knownKeys = func() map[string]bool {
	b, _ := json.Marshal(flightAlias{ID: "x", DepLocal: &LocalTime{}, ArrLocal: &LocalTime{}, DepEstimated: &LocalTime{},
		ArrEstimated: &LocalTime{}, DepActual: &LocalTime{}, ArrActual: &LocalTime{}, DepTerminal: new(string),
		DepGate: new(string), ArrTerminal: new(string), ArrGate: new(string), Aircraft: new(string), Tail: new(string),
		Seat: new(string), Cabin: new(string), PNR: new(string), Notes: new(string), DivertedTo: new(string),
		BoardingPass: &BoardingPass{}})
	var m map[string]json.RawMessage
	_ = json.Unmarshal(b, &m)
	k := map[string]bool{}
	for key := range m {
		k[key] = true
	}
	return k
}()

func (f *Flight) UnmarshalJSON(b []byte) error {
	var a flightAlias
	if err := json.Unmarshal(b, &a); err != nil {
		return err
	}
	var all map[string]json.RawMessage
	if err := json.Unmarshal(b, &all); err != nil {
		return err
	}
	for k, v := range all {
		if !knownKeys[k] {
			if a.Extra == nil {
				a.Extra = map[string]json.RawMessage{}
			}
			a.Extra[k] = v
		}
	}
	if a.Source == "" {
		a.Source = "manual"
	}
	*f = Flight(a)
	return nil
}

func (f Flight) MarshalJSON() ([]byte, error) {
	b, err := json.Marshal(flightAlias(f))
	if err != nil || len(f.Extra) == 0 {
		return b, err
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	for k, v := range f.Extra {
		if _, ok := m[k]; !ok {
			m[k] = v
		}
	}
	return json.Marshal(m)
}

// NewID makes a random UUID (v4), like Skyline's UUID.randomUUID().
func NewID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

// ArrivalAirport is where the aircraft actually landed.
func (f *Flight) ArrivalAirport() string {
	if f.DivertedTo != nil && *f.DivertedTo != "" {
		return *f.DivertedTo
	}
	return f.To
}

// Label is "BA478".
func (f *Flight) Label() string { return f.Airline + f.Number }

// SameFlightAs: same date, origin and number (airline codes may differ between IATA and ICAO).
func (f *Flight) SameFlightAs(o *Flight) bool {
	return f.Date.Equal(o.Date.Time) && strings.EqualFold(f.From, o.From) &&
		strings.TrimLeft(f.Number, "0") == strings.TrimLeft(o.Number, "0")
}

// Str returns the value of an optional string, or "".
func Str(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// Opt returns nil for an empty string, so it's left out of the JSON like Kotlin's putOpt.
func Opt(s string) *string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return &s
}
