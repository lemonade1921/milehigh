package store

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/lemonade1921/milehigh/internal/model"
)

// Backup is a Skyline backup file: {"app":"skyline","format":2,"exportedAt":…,"flights":[…],"preferences":{…}}.
// milehigh writes the same format (app stays "skyline" so the Android app accepts it).
type Backup struct {
	App         string          `json:"app"`
	Format      int             `json:"format"`
	ExportedAt  string          `json:"exportedAt"`
	Flights     []*model.Flight `json:"flights"`
	Preferences Prefs           `json:"preferences,omitempty"`
}

// ExportBackup writes the whole log and every preference, ready to restore on Skyline or milehigh.
func (s *Store) ExportBackup() ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b := Backup{
		App: "skyline", Format: 2,
		// Skyline writes Instant.toString(): UTC with a Z.
		ExportedAt:  Now().UTC().Truncate(time.Millisecond).Format("2006-01-02T15:04:05.000Z07:00"),
		Flights:     append([]*model.Flight{}, s.flights...),
		Preferences: s.prefs.Clone(),
	}
	return json.MarshalIndent(b, "", "  ")
}

// DecodeBackup accepts a Skyline backup or a bare array of flights.
func DecodeBackup(b []byte) (flights []*model.Flight, prefs Prefs, err error) {
	t := trimBOM(b)
	if len(t) == 0 {
		return nil, nil, errors.New("empty file")
	}
	if t[0] == '[' {
		fs, err := DecodeFlights(t)
		return fs, nil, err
	}
	var raw struct {
		Flights     json.RawMessage `json:"flights"`
		Preferences Prefs           `json:"preferences"`
	}
	if err := json.Unmarshal(t, &raw); err != nil {
		return nil, nil, err
	}
	if raw.Flights == nil {
		return nil, nil, errors.New("not a Skyline backup (no flights)")
	}
	fs, err := DecodeFlights(raw.Flights)
	return fs, raw.Preferences, err
}

// RestoreResult says what a restore did.
type RestoreResult struct {
	Added, Updated int
	Preferences    bool
}

// Restore merges a backup into the log exactly like Skyline does: flights not in the log are
// added; for flights already there, details the log is missing are filled in from the backup.
// Settings are replaced by the backup's, visited-country edits merged (the backup wins where the
// two disagree) and Live Update opt-outs are the union.
func (s *Store) Restore(backup []*model.Flight, prefs Prefs) (RestoreResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var r RestoreResult
	for _, b := range backup {
		i := -1
		for j, e := range s.flights {
			if e.ID == b.ID || e.SameFlightAs(b) {
				i = j
				break
			}
		}
		if i < 0 {
			s.flights = append(s.flights, b)
			r.Added++
			continue
		}
		e := *s.flights[i]
		before, _ := json.Marshal(e)
		fillStr := func(dst **string, src *string) {
			if *dst == nil {
				*dst = src
			}
		}
		fillTime := func(dst **model.LocalTime, src *model.LocalTime) {
			if *dst == nil {
				*dst = src
			}
		}
		if e.BoardingPass == nil {
			e.BoardingPass = b.BoardingPass
		}
		fillStr(&e.Seat, b.Seat)
		fillTime(&e.DepLocal, b.DepLocal)
		fillTime(&e.ArrLocal, b.ArrLocal)
		fillTime(&e.DepActual, b.DepActual)
		fillTime(&e.ArrActual, b.ArrActual)
		fillStr(&e.DepGate, b.DepGate)
		fillStr(&e.PNR, b.PNR)
		fillStr(&e.Notes, b.Notes)
		fillStr(&e.Aircraft, b.Aircraft)
		after, _ := json.Marshal(e)
		if string(before) != string(after) {
			s.flights[i] = &e
			r.Updated++
		}
	}
	if prefs != nil {
		r.Preferences = true
		if set := prefs[FileSettings]; set != nil {
			for k, v := range set {
				s.prefs[FileSettings][k] = v
			}
		}
		if v := prefs[FileVisited]; v != nil {
			bp := Prefs{FileVisited: v}
			bAdded, bRemoved := bp.VisitedEdits()
			added, removed := s.prefs.VisitedEdits()
			for k := range bRemoved {
				delete(added, k)
			}
			for k := range bAdded {
				added[k] = true
				delete(removed, k)
			}
			for k := range bRemoved {
				removed[k] = true
			}
			s.prefs.PutSet(FileVisited, "added", added)
			s.prefs.PutSet(FileVisited, "removed", removed)
		}
		if live := prefs[FileLive]; live != nil {
			for k, v := range live {
				cur, ok := s.prefs[FileLive][k]
				switch {
				case v.T == "set":
					union := Prefs{FileLive: {k: cur}}.Set(FileLive, k)
					for x := range (Prefs{FileLive: {k: v}}).Set(FileLive, k) {
						union[x] = true
					}
					s.prefs.PutSet(FileLive, k, union)
				case !ok:
					s.prefs[FileLive][k] = v
				}
			}
		}
		// Any other preference file a newer Skyline adds travels along untouched.
		for f, m := range prefs {
			if f != FileSettings && f != FileVisited && f != FileLive {
				if s.prefs[f] == nil {
					s.prefs[f] = map[string]Entry{}
				}
				for k, v := range m {
					s.prefs[f][k] = v
				}
			}
		}
	}
	if err := s.saveFlights(); err != nil {
		return r, err
	}
	if prefs != nil {
		return r, s.savePrefs()
	}
	return r, nil
}

// Kind of an import file.
type Kind int

const (
	KindUnknown Kind = iota
	KindSkyline
	KindFlighty
)

// Detect guesses whether a file is a Skyline backup/flight list or a Flighty CSV export.
func Detect(b []byte) Kind {
	t := trimBOM(b)
	if len(t) == 0 {
		return KindUnknown
	}
	if t[0] == '{' || t[0] == '[' {
		return KindSkyline
	}
	first := strings.ToLower(strings.SplitN(string(t), "\n", 2)[0])
	if strings.Contains(first, "date") && strings.Contains(first, "from") && strings.Contains(first, "to") {
		return KindFlighty
	}
	return KindUnknown
}
