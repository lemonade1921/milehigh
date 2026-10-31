// Package store keeps the flight log on disk, in the same files Skyline uses on Android
// (flights.json, plus its preferences in Skyline's backup format), and reads and writes
// Skyline backups so a log can move freely between the phone and the terminal.
package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/lemonade1921/milehigh/internal/model"
)

// Dir is where the log lives:
//
//	$MILEHIGH_HOME, else
//	macOS: ~/Library/Application Support/milehigh
//	Linux: $XDG_DATA_HOME/milehigh (~/.local/share/milehigh)
func Dir() (string, error) {
	if d := os.Getenv("MILEHIGH_HOME"); d != "" {
		return d, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	if runtime.GOOS == "darwin" {
		return filepath.Join(home, "Library", "Application Support", "milehigh"), nil
	}
	if x := os.Getenv("XDG_DATA_HOME"); x != "" {
		return filepath.Join(x, "milehigh"), nil
	}
	return filepath.Join(home, ".local", "share", "milehigh"), nil
}

// Store is the open log. Safe for concurrent use.
type Store struct {
	mu      sync.Mutex
	dir     string
	flights []*model.Flight
	prefs   Prefs
}

// Open loads (or creates) the log in dir ("" = Dir()).
func Open(dir string) (*Store, error) {
	if dir == "" {
		d, err := Dir()
		if err != nil {
			return nil, err
		}
		dir = d
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	s := &Store{dir: dir, prefs: NewPrefs()}
	if b, err := os.ReadFile(filepath.Join(dir, "flights.json")); err == nil {
		fs, err := DecodeFlights(b)
		if err != nil {
			return nil, fmt.Errorf("reading flights.json: %w", err)
		}
		s.flights = fs
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if b, err := os.ReadFile(filepath.Join(dir, "preferences.json")); err == nil {
		if err := json.Unmarshal(b, &s.prefs); err != nil {
			return nil, fmt.Errorf("reading preferences.json: %w", err)
		}
		s.prefs.ensure()
	}
	return s, nil
}

// Path is the directory the store lives in.
func (s *Store) Path() string { return s.dir }

// Flights returns the flights sorted by departure (oldest first). The slice is a copy; the
// flights are shared, so treat them as read-only and change them through Put.
func (s *Store) Flights() []*model.Flight {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := append([]*model.Flight(nil), s.flights...)
	model.SortFlights(out)
	return out
}

// Get finds a flight by id.
func (s *Store) Get(id string) *model.Flight {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, f := range s.flights {
		if f.ID == id {
			return f
		}
	}
	return nil
}

// Put adds a flight or replaces the one with the same id.
func (s *Store) Put(f *model.Flight) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if f.ID == "" {
		f.ID = model.NewID()
	}
	for i, e := range s.flights {
		if e.ID == f.ID {
			s.flights[i] = f
			return s.saveFlights()
		}
	}
	s.flights = append(s.flights, f)
	return s.saveFlights()
}

// Delete removes a flight.
func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, e := range s.flights {
		if e.ID == id {
			s.flights = append(s.flights[:i], s.flights[i+1:]...)
			return s.saveFlights()
		}
	}
	return nil
}

// AddNew adds flights that aren't in the log yet (an import). Returns how many were added.
func (s *Store) AddNew(fs []*model.Flight) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	added := 0
	for _, f := range fs {
		dup := false
		for _, e := range s.flights {
			if e.SameFlightAs(f) {
				dup = true
				break
			}
		}
		if !dup {
			if f.ID == "" {
				f.ID = model.NewID()
			}
			s.flights = append(s.flights, f)
			added++
		}
	}
	if added == 0 {
		return 0, nil
	}
	return added, s.saveFlights()
}

// Prefs returns a copy of the preferences.
func (s *Store) Prefs() Prefs {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.prefs.Clone()
}

// UpdatePrefs changes the preferences and saves them.
func (s *Store) UpdatePrefs(fn func(*Prefs)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	fn(&s.prefs)
	return s.savePrefs()
}

func (s *Store) saveFlights() error {
	list := append([]*model.Flight(nil), s.flights...)
	// Skyline keeps insertion order; sorting by date keeps diffs of the file readable.
	sort.SliceStable(list, func(i, j int) bool { return list[i].Date.Before(list[j].Date.Time) })
	b, err := json.Marshal(list)
	if err != nil {
		return err
	}
	return writeAtomic(filepath.Join(s.dir, "flights.json"), b)
}

func (s *Store) savePrefs() error {
	b, err := json.MarshalIndent(s.prefs, "", "  ")
	if err != nil {
		return err
	}
	return writeAtomic(filepath.Join(s.dir, "preferences.json"), b)
}

func writeAtomic(path string, b []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// DecodeFlights reads a bare JSON array of flights, skipping entries it can't read (like Skyline).
func DecodeFlights(b []byte) ([]*model.Flight, error) {
	var raw []json.RawMessage
	if err := json.Unmarshal(trimBOM(b), &raw); err != nil {
		return nil, err
	}
	out := make([]*model.Flight, 0, len(raw))
	for _, r := range raw {
		var f model.Flight
		if err := json.Unmarshal(r, &f); err != nil || f.ID == "" || f.From == "" || f.To == "" || f.Date.IsZero() {
			continue
		}
		out = append(out, &f)
	}
	return out, nil
}

func trimBOM(b []byte) []byte {
	s := strings.TrimSpace(string(b))
	return []byte(strings.TrimPrefix(s, "\uFEFF"))
}

// Now is the clock, replaceable in tests.
var Now = time.Now
