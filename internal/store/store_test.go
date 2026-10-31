package store

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/lemonade1921/milehigh/internal/model"
)

// A backup exactly as Skyline 's FlightJson.encodeBackup + PrefsBackup.export write it,
// plus a field from a hypothetical newer Skyline ("future").
const skylineBackup = `{
  "app": "skyline",
  "format": 2,
  "exportedAt": "2026-10-27T12:00:00Z",
  "flights": [
    {"id":"7d0e0c52-6c1c-4e86-9a40-1d7c7a1f3c11","airline":"IB","number":"6251","from":"MAD","to":"JFK","date":"2026-07-14",
     "depLocal":"2026-07-14T10:45","arrEstimated":"2026-07-14T13:40","depActual":"2026-07-14T10:52",
     "seat":"32A","canceled":false,"source":"flighty","future":{"x":1},
     "boardingPass":{"format":"AZTEC","message":"M1TEST","source":"pkpass"}},
    {"id":"b1","airline":"VY","number":"1234","from":"BCN","to":"LHR","date":"2026-11-04","canceled":false,"source":"manual"},
    {"broken": true}
  ],
  "preferences": {
    "settings": {"name":{"t":"s","v":"Joan"},"units":{"t":"s","v":"MI"},"mapStyle":{"t":"s","v":"DARK"},"liveUpdates":{"t":"b","v":true}},
    "visited": {"added":{"t":"set","v":["AD"]},"removed":{"t":"set","v":["GB"]}},
    "live_update": {"off":{"t":"set","v":["b1"]}}
  }
}`

func TestSkylineBackupRoundTrip(t *testing.T) {
	fs, prefs, err := DecodeBackup([]byte("\uFEFF" + skylineBackup))
	if err != nil {
		t.Fatal(err)
	}
	if len(fs) != 2 {
		t.Fatalf("want 2 readable flights, got %d", len(fs))
	}
	f := fs[0]
	if f.DepLocal.String() != "2026-07-14T10:45" || model.Str(f.Seat) != "32A" || f.BoardingPass.Format != "AZTEC" {
		t.Fatalf("bad flight %+v", f)
	}
	if prefs.Settings().Name != "Joan" || prefs.Settings().Units != "MI" {
		t.Fatalf("bad settings %+v", prefs.Settings())
	}

	st, _ := Open(t.TempDir())
	r, err := st.Restore(fs, prefs)
	if err != nil || r.Added != 2 || !r.Preferences {
		t.Fatalf("restore: %+v %v", r, err)
	}
	out, err := st.ExportBackup()
	if err != nil {
		t.Fatal(err)
	}
	var back map[string]any
	if err := json.Unmarshal(out, &back); err != nil {
		t.Fatal(err)
	}
	if back["app"] != "skyline" || back["format"].(float64) != 2 {
		t.Fatalf("header %v", back)
	}
	s := string(out)
	for _, want := range []string{`"future"`, `"mapStyle"`, `"DARK"`, `"depLocal": "2026-07-14T10:45"`, `"off"`, `"AD"`} {
		if !strings.Contains(s, want) {
			t.Errorf("export lost %s:\n%s", want, s)
		}
	}
	// Re-reading our own export gives the same flights.
	fs2, _, err := DecodeBackup(out)
	if err != nil || len(fs2) != 2 {
		t.Fatal(err)
	}
	a, _ := json.Marshal(fs)
	b, _ := json.Marshal(fs2)
	if string(a) != string(b) {
		t.Errorf("round trip changed flights:\n%s\n%s", a, b)
	}
	// Reopening from disk keeps everything.
	st2, err := Open(st.Path())
	if err != nil || len(st2.Flights()) != 2 || st2.Prefs().Settings().Name != "Joan" {
		t.Fatalf("reopen: %v", err)
	}
}

func TestRestoreMerges(t *testing.T) {
	st, _ := Open(t.TempDir())
	d, _ := model.ParseDate("2026-07-14")
	mine := &model.Flight{ID: "mine", Airline: "IBE", Number: "06251", From: "MAD", To: "JFK", Date: d, Notes: model.Opt("my note"), Source: "manual"}
	_ = st.Put(mine)
	_ = st.UpdatePrefs(func(p *Prefs) {
		p.MarkVisited("GB", true)
		p.MarkVisited("FR", false)
		p.PutSet(FileLive, "off", map[string]bool{"x": true})
	})
	fs, prefs, _ := DecodeBackup([]byte(skylineBackup))
	r, err := st.Restore(fs, prefs)
	if err != nil {
		t.Fatal(err)
	}
	if r.Added != 1 || r.Updated != 1 {
		t.Fatalf("want 1 added 1 updated, got %+v", r)
	}
	f := st.Get("mine")
	if model.Str(f.Notes) != "my note" || model.Str(f.Seat) != "32A" || f.BoardingPass == nil {
		t.Fatalf("merge didn't fill gaps only: %+v", f)
	}
	added, removed := st.Prefs().VisitedEdits()
	if !added["AD"] || added["GB"] || !removed["GB"] || !removed["FR"] {
		t.Fatalf("visited merge: added=%v removed=%v", added, removed)
	}
	if off := st.Prefs().Set(FileLive, "off"); !off["x"] || !off["b1"] {
		t.Fatalf("live opt-outs should be the union: %v", off)
	}
}

func TestFlightyCSV(t *testing.T) {
	csv := "\uFEFFDate,Airline,Flight,From,To,Dep Terminal,Dep Gate,Arr Terminal,Arr Gate,Canceled,Diverted To," +
		"Gate Departure (Scheduled),Gate Departure (Actual),Take off (Scheduled),Take off (Actual)," +
		"Landing (Scheduled),Landing (Actual),Gate Arrival (Scheduled),Gate Arrival (Actual)," +
		"Aircraft Type Name,Tail Number,PNR,Seat,Seat Type,Cabin Class,Flight Reason,Notes\n" +
		"2026-07-14,IBE,6251,MAD,JFK,4S,S24,8,,false,,2026-07-14T10:45,2026-07-14T10:52,,,,,2026-07-14T13:50,2026-07-14T13:41," +
		"Airbus A330-300,EC-LUB,X7K2QP,32A,WINDOW,ECONOMY,LEISURE,\"Great view, \"\"nice\"\" crew\"\n" +
		"2026-08-01,VLG,,BCN,XX,,,,,,,,,,,,,,,,,,,,,,\n"
	if Detect([]byte(csv)) != KindFlighty {
		t.Fatal("not detected as Flighty")
	}
	fs, skipped, err := ParseFlighty([]byte(csv))
	if err != nil || len(fs) != 1 || skipped != 1 {
		t.Fatalf("got %d flights, %d skipped, %v", len(fs), skipped, err)
	}
	f := fs[0]
	checks := map[string][2]string{
		"airline": {f.Airline, "IBE"}, "number": {f.Number, "6251"},
		"depLocal": {f.DepLocal.String(), "2026-07-14T10:45"}, "depActual": {f.DepActual.String(), "2026-07-14T10:52"},
		"arrLocal": {f.ArrLocal.String(), "2026-07-14T13:50"}, "arrActual": {f.ArrActual.String(), "2026-07-14T13:41"},
		"gate": {model.Str(f.DepGate), "S24"}, "seat": {model.Str(f.Seat), "32A"}, "cabin": {model.Str(f.Cabin), "Economy"},
		"notes": {model.Str(f.Notes), `Great view, "nice" crew`},
	}
	for k, v := range checks {
		if v[0] != v[1] {
			t.Errorf("%s = %q, want %q", k, v[0], v[1])
		}
	}
	if f.Canceled {
		t.Error("canceled")
	}
	if km := f.DistanceKm(); km < 5700 || km > 5800 {
		t.Errorf("MAD-JFK distance %v", km)
	}
}
