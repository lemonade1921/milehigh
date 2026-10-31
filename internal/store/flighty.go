package store

import (
	"bytes"
	"encoding/csv"
	"errors"
	"strings"

	"github.com/lemonade1921/milehigh/internal/model"
)

// ParseFlighty reads the CSV that Flighty exports (Settings › Export flights). Columns are matched
// by header name; times have no zone: they are local airport times. Ported from Skyline.
func ParseFlighty(b []byte) (flights []*model.Flight, skipped int, err error) {
	r := csv.NewReader(bytes.NewReader(trimBOM(b)))
	r.FieldsPerRecord = -1
	r.LazyQuotes = true
	rows, err := r.ReadAll()
	if err != nil {
		return nil, 0, err
	}
	if len(rows) == 0 {
		return nil, 0, errors.New("empty file")
	}
	header := make(map[string]int)
	for i, h := range rows[0] {
		header[strings.ToLower(strings.TrimSpace(h))] = i
	}
	col := func(name string) int {
		if i, ok := header[strings.ToLower(name)]; ok {
			return i
		}
		return -1
	}
	iDate, iFrom, iTo := col("Date"), col("From"), col("To")
	if iDate < 0 || iFrom < 0 || iTo < 0 {
		return nil, 0, errors.New("not a Flighty export (missing Date/From/To columns)")
	}
	for _, row := range rows[1:] {
		v := func(name string) string {
			i := col(name)
			if i < 0 || i >= len(row) {
				return ""
			}
			return strings.TrimSpace(row[i])
		}
		tm := func(names ...string) *model.LocalTime {
			for _, n := range names {
				s := v(n)
				if len(s) >= 16 {
					if t, err := model.ParseLocalTime(s[:16]); err == nil {
						return &t
					}
				}
			}
			return nil
		}
		d, err := model.ParseDate(v("Date"))
		from, to := strings.ToUpper(v("From")), strings.ToUpper(v("To"))
		if err != nil || len(from) != 3 || len(to) != 3 {
			skipped++
			continue
		}
		f := &model.Flight{
			ID: model.NewID(), Airline: strings.ToUpper(v("Airline")), Number: v("Flight"),
			From: from, To: to, Date: d,
			DepLocal:    tm("Gate Departure (Scheduled)", "Take off (Scheduled)"),
			ArrLocal:    tm("Gate Arrival (Scheduled)", "Landing (Scheduled)"),
			DepActual:   tm("Gate Departure (Actual)", "Take off (Actual)"),
			ArrActual:   tm("Gate Arrival (Actual)", "Landing (Actual)"),
			DepTerminal: model.Opt(v("Dep Terminal")), DepGate: model.Opt(v("Dep Gate")),
			ArrTerminal: model.Opt(v("Arr Terminal")), ArrGate: model.Opt(v("Arr Gate")),
			Canceled: strings.EqualFold(v("Canceled"), "true"),
			Aircraft: model.Opt(v("Aircraft Type Name")), Tail: model.Opt(v("Tail Number")),
			PNR: model.Opt(v("PNR")), Seat: model.Opt(v("Seat")), Notes: model.Opt(v("Notes")),
			Source: "flighty",
		}
		if dv := strings.ToUpper(v("Diverted To")); len(dv) == 3 {
			f.DivertedTo = &dv
		}
		if c := strings.ToLower(v("Cabin Class")); c != "" {
			c = strings.ToUpper(c[:1]) + c[1:]
			f.Cabin = &c
		}
		flights = append(flights, f)
	}
	return flights, skipped, nil
}
