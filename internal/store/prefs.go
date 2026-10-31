package store

import (
	"encoding/json"
	"sort"
)

// Entry is one typed preference, as Skyline's PrefsBackup writes it:
// t = "s" string, "b" bool, "i" int, "l" long, "f" float, "set" string set.
type Entry struct {
	T string          `json:"t"`
	V json.RawMessage `json:"v"`
}

// Prefs is the "preferences" object of a Skyline backup: SharedPreferences file -> key -> entry.
// milehigh keeps its own settings in the same "settings" file with the same keys, so a backup
// carries your name, units and clock format both ways, and everything else Skyline stores
// (map style, theme, language…) passes through untouched.
type Prefs map[string]map[string]Entry

const (
	FileSettings = "settings"
	FileVisited  = "visited"
	FileLive     = "live_update"
)

// NewPrefs makes empty preferences.
func NewPrefs() Prefs {
	p := Prefs{}
	p.ensure()
	return p
}

func (p *Prefs) ensure() {
	if *p == nil {
		*p = Prefs{}
	}
	for _, f := range []string{FileSettings, FileVisited, FileLive} {
		if (*p)[f] == nil {
			(*p)[f] = map[string]Entry{}
		}
	}
}

// Clone deep-copies the preferences.
func (p Prefs) Clone() Prefs {
	out := Prefs{}
	for f, m := range p {
		c := make(map[string]Entry, len(m))
		for k, v := range m {
			c[k] = v
		}
		out[f] = c
	}
	out.ensure()
	return out
}

// String reads a string setting.
func (p Prefs) String(file, key, def string) string {
	e, ok := p[file][key]
	if !ok || e.T != "s" {
		return def
	}
	var s string
	if json.Unmarshal(e.V, &s) != nil {
		return def
	}
	return s
}

// SetString writes a string setting.
func (p Prefs) SetString(file, key, v string) {
	p.ensure()
	b, _ := json.Marshal(v)
	p[file][key] = Entry{T: "s", V: b}
}

// Bool reads a boolean setting.
func (p Prefs) Bool(file, key string, def bool) bool {
	e, ok := p[file][key]
	if !ok || e.T != "b" {
		return def
	}
	var b bool
	if json.Unmarshal(e.V, &b) != nil {
		return def
	}
	return b
}

// SetBool writes a boolean setting.
func (p Prefs) SetBool(file, key string, v bool) {
	p.ensure()
	b, _ := json.Marshal(v)
	p[file][key] = Entry{T: "b", V: b}
}

// Set reads a string-set setting.
func (p Prefs) Set(file, key string) map[string]bool {
	out := map[string]bool{}
	e, ok := p[file][key]
	if !ok || e.T != "set" {
		return out
	}
	var list []string
	_ = json.Unmarshal(e.V, &list)
	for _, s := range list {
		out[s] = true
	}
	return out
}

// PutSet writes a string-set setting (sorted, as Skyline writes it).
func (p Prefs) PutSet(file, key string, set map[string]bool) {
	p.ensure()
	list := make([]string, 0, len(set))
	for s, ok := range set {
		if ok {
			list = append(list, s)
		}
	}
	sort.Strings(list)
	b, _ := json.Marshal(list)
	p[file][key] = Entry{T: "set", V: b}
}

// Settings milehigh understands (same keys and values as Skyline's SettingsStore).
type Settings struct {
	Name        string
	Units       string // KM or MI
	TimeFormat  string // SYSTEM, H24 or H12
	Temp        string // C or F
	ScheduleKey string // AeroDataBox key (RapidAPI), optional
}

// Settings reads the settings milehigh uses.
func (p Prefs) Settings() Settings {
	return Settings{
		Name:        p.String(FileSettings, "name", ""),
		Units:       p.String(FileSettings, "units", "KM"),
		TimeFormat:  p.String(FileSettings, "timeFormat", "SYSTEM"),
		Temp:        p.String(FileSettings, "temp", "C"),
		ScheduleKey: p.String(FileSettings, "scheduleKey", ""),
	}
}

// PutSettings writes the settings milehigh uses, leaving Skyline's other settings alone.
func (p Prefs) PutSettings(s Settings) {
	p.SetString(FileSettings, "name", s.Name)
	p.SetString(FileSettings, "units", s.Units)
	p.SetString(FileSettings, "timeFormat", s.TimeFormat)
	p.SetString(FileSettings, "temp", s.Temp)
	p.SetString(FileSettings, "scheduleKey", s.ScheduleKey)
}

// VisitedEdits are the countries added or removed by hand (ISO codes).
func (p Prefs) VisitedEdits() (added, removed map[string]bool) {
	return p.Set(FileVisited, "added"), p.Set(FileVisited, "removed")
}

// MarkVisited adds (true) or removes (false) a country by hand, exactly like Skyline's VisitedStore.
func (p Prefs) MarkVisited(iso string, visited bool) {
	added, removed := p.VisitedEdits()
	if visited {
		added[iso] = true
		delete(removed, iso)
	} else {
		delete(added, iso)
		removed[iso] = true
	}
	p.PutSet(FileVisited, "added", added)
	p.PutSet(FileVisited, "removed", removed)
}
