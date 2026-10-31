package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/lemonade1921/milehigh/internal/data"
	"github.com/lemonade1921/milehigh/internal/model"
	"github.com/lemonade1921/milehigh/internal/online"
	"github.com/lemonade1921/milehigh/internal/store"
)

func airportHint(v string) string {
	if strings.TrimSpace(v) == "" {
		return ""
	}
	if a := data.AirportByCode(v); a != nil {
		return a.City + " · " + a.Name
	}
	if res := data.SearchAirports(v, 3); len(res) > 0 {
		var s []string
		for _, a := range res {
			s = append(s, a.IATA+" "+a.City)
		}
		return "→ " + strings.Join(s, ", ")
	}
	return "unknown airport"
}

func timeOf(t *model.LocalTime) string {
	if t == nil {
		return ""
	}
	return t.Format("15:04")
}

func (m *Model) openFlightForm(f *model.Flight) {
	const w = 26
	fm := &form{title: "Add a flight"}
	var code, date, from, to, dep, arr, seat, cabin, pnr, ac, notes string
	date = m.now.Format("2006-01-02")
	m.editingID = ""
	if f != nil {
		fm.title = "Edit " + flightCode(f)
		m.editingID = f.ID
		code = strings.TrimSpace(f.Airline + " " + f.Number)
		date, from, to = f.Date.String(), f.From, f.To
		dep, arr = timeOf(f.DepLocal), timeOf(f.ArrLocal)
		seat, cabin, pnr, ac, notes = model.Str(f.Seat), model.Str(f.Cabin), model.Str(f.PNR), model.Str(f.Aircraft), model.Str(f.Notes)
	}
	fm.fields = []*field{
		newField("code", "Flight", code, "VY 1234", w),
		newField("date", "Date", date, "YYYY-MM-DD", w),
		newField("from", "From", from, "BCN or Barcelona", w),
		newField("to", "To", to, "LHR or London", w),
		newField("dep", "Departs", dep, "HH:MM local (optional)", w),
		newField("arr", "Arrives", arr, "HH:MM local (optional)", w),
		newField("seat", "Seat", seat, "12A", w),
		newField("cabin", "Cabin", cabin, "Economy", w),
		newField("pnr", "Booking", pnr, "ABC123", w),
		newField("aircraft", "Aircraft", ac, "A320, Boeing 737-800…", w),
		newField("notes", "Notes", notes, "", w),
	}
	fm.fields[0].hint = func(v string) string {
		if al, _, ok := online.SplitNumber(v); ok {
			if a := data.AirlineByCode(al); a != nil {
				return a.Name
			}
		}
		return ""
	}
	fm.fields[2].hint = airportHint
	fm.fields[3].hint = airportHint
	fm.fields[4].hint = func(string) string { return "" }
	fm.fields[9].hint = func(v string) string {
		if n := data.AircraftName(v); n != v {
			return n
		}
		return ""
	}
	fm.fields[7].choices = []string{"", "Economy", "Premium economy", "Business", "First"}
	if f == nil {
		fm.note = "Type a flight number: its usual route is looked up when you leave the field."
	}
	fm.setFocus(0)
	m.form = fm
	m.modal = modalForm
}

type lookupMsg struct {
	code  string
	route *online.Route
	err   error
}

func lookupCmd(code string) tea.Cmd {
	al, num, ok := online.SplitNumber(code)
	if !ok {
		return nil
	}
	return func() tea.Msg {
		r, err := online.LookupRoute(al, num)
		return lookupMsg{code, r, err}
	}
}

func (m *Model) onLookup(msg lookupMsg) (tea.Model, tea.Cmd) {
	if m.form == nil || m.modal != modalForm || m.form.get("code") != msg.code {
		return m, nil
	}
	if msg.err != nil {
		if msg.err == online.ErrNotFound {
			m.form.note = "No route on file for " + msg.code + ": enter the airports yourself."
		} else {
			m.form.note = "Couldn't look up the route (" + msg.err.Error() + ")."
		}
		return m, nil
	}
	if m.form.get("from") == "" {
		m.form.set("from", msg.route.From)
	}
	if m.form.get("to") == "" {
		m.form.set("to", msg.route.To)
	}
	m.form.note = fmt.Sprintf("%s usually flies %s → %s (adsbdb)", msg.code, msg.route.From, msg.route.To)
	return m, nil
}

func (m *Model) updateForm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.modal, m.form = modalNone, nil
		return m, nil
	case "enter", "ctrl+s":
		if m.form.title == "Settings" {
			return m.saveSettings()
		}
		if msg.String() == "enter" && m.form.focus < len(m.form.fields)-1 && m.form.current().key == "code" {
			cmd := lookupCmd(m.form.get("code"))
			m.form.setFocus(m.form.focus + 1)
			return m, cmd
		}
		return m.saveFlight()
	}
	cmd, left := m.form.update(msg)
	if left == "code" && m.editingID == "" {
		if c := lookupCmd(m.form.get("code")); c != nil {
			m.form.note = "Looking up " + m.form.get("code") + "…"
			return m, tea.Batch(cmd, c)
		}
	}
	if left == "from" || left == "to" {
		if v := m.form.get(left); v != "" && data.AirportByCode(v) == nil {
			if res := data.SearchAirports(v, 1); len(res) == 1 {
				m.form.set(left, res[0].IATA)
			}
		}
		if left == "to" && m.form.get("code") == "" {
			if rs := data.RoutesBetween(m.form.get("from"), m.form.get("to")); len(rs) > 0 {
				var s []string
				for i, r := range rs {
					if i == 6 {
						s = append(s, "…")
						break
					}
					s = append(s, r.Airline+" "+r.Number)
				}
				m.form.note = "Flights on this route: " + strings.Join(s, ", ")
			}
		}
	}
	return m, cmd
}

func resolveAirport(v string) (string, bool) {
	v = strings.ToUpper(strings.TrimSpace(v))
	if a := data.AirportByCode(v); a != nil {
		return a.IATA, true
	}
	if res := data.SearchAirports(v, 1); len(res) == 1 {
		return res[0].IATA, true
	}
	return v, len(v) == 3
}

func (m *Model) saveFlight() (tea.Model, tea.Cmd) {
	fm := m.form
	fm.err = ""
	date, err := model.ParseDate(fm.get("date"))
	if err != nil {
		fm.err = "Date must look like 2026-10-31"
		return m, nil
	}
	from, ok1 := resolveAirport(fm.get("from"))
	to, ok2 := resolveAirport(fm.get("to"))
	if !ok1 || !ok2 {
		fm.err = "From and To need airport codes (like BCN) or a city we can find"
		return m, nil
	}
	var f model.Flight
	if m.editingID != "" {
		if e := m.st.Get(m.editingID); e != nil {
			f = *e
		}
	} else {
		f = model.Flight{ID: model.NewID(), Source: "manual"}
	}
	f.Airline, f.Number = "", ""
	if c := fm.get("code"); c != "" {
		al, num, ok := online.SplitNumber(c)
		if !ok {
			fm.err = "Flight number should look like VY 1234"
			return m, nil
		}
		f.Airline, f.Number = al, num
	}
	f.Date, f.From, f.To = date, from, to
	parseT := func(key string, day model.Date, base *model.LocalTime) (*model.LocalTime, error) {
		v := fm.get(key)
		if v == "" {
			return nil, nil
		}
		t, err := time.Parse("15:04", strings.Replace(v, ".", ":", 1))
		if err != nil {
			return nil, fmt.Errorf("times look like 14:05")
		}
		lt := model.LocalTime{Time: time.Date(day.Year(), day.Month(), day.Day(), t.Hour(), t.Minute(), 0, 0, time.UTC)}
		return &lt, nil
	}
	dep, err := parseT("dep", date, nil)
	if err != nil {
		fm.err = err.Error()
		return m, nil
	}
	arr, err := parseT("arr", date, nil)
	if err != nil {
		fm.err = err.Error()
		return m, nil
	}
	// An arrival "earlier" than departure lands the next day (in local time, allowing for zones).
	if dep != nil && arr != nil {
		f.DepLocal = dep
		f.ArrLocal = arr
		if f.Arrival().Before(f.Departure()) {
			n := model.LocalTime{Time: arr.AddDate(0, 0, 1)}
			f.ArrLocal = &n
		}
	} else {
		f.DepLocal, f.ArrLocal = dep, arr
	}
	f.Seat, f.Cabin, f.PNR = model.Opt(fm.get("seat")), model.Opt(fm.get("cabin")), model.Opt(fm.get("pnr"))
	f.Aircraft, f.Notes = model.Opt(fm.get("aircraft")), model.Opt(fm.get("notes"))
	if err := m.st.Put(&f); err != nil {
		fm.err = err.Error()
		return m, nil
	}
	m.modal, m.form = modalNone, nil
	m.reload()
	for i, r := range m.rows {
		if r.f != nil && r.f.ID == f.ID {
			m.sel = i
		}
	}
	m.setStatus("Saved "+strings.TrimSpace(flightCode(&f)+" "+f.From+" → "+f.To), false)
	return m, nil
}

func (m *Model) openSettings() {
	s := m.settings
	const w = 30
	fm := &form{title: "Settings"}
	fm.fields = []*field{
		newField("name", "Your name", s.Name, "", w),
		newField("units", "Distance", s.Units, "", w),
		newField("time", "Clock", s.TimeFormat, "", w),
		newField("temp", "Temperature", s.Temp, "", w),
		newField("key", "AeroDataBox key", s.ScheduleKey, "optional, for live times & gates", w),
	}
	fm.fields[1].choices = []string{"KM", "MI"}
	fm.fields[2].choices = []string{"SYSTEM", "H24", "H12"}
	fm.fields[3].choices = []string{"C", "F"}
	fm.fields[4].input.EchoMode = textinput.EchoPassword
	fm.note = "Shared with Skyline through backups. Log: " + m.st.Path()
	fm.setFocus(0)
	m.form, m.modal = fm, modalForm
}

func (m *Model) saveSettings() (tea.Model, tea.Cmd) {
	fm := m.form
	s := m.settings
	s.Name, s.Units, s.TimeFormat, s.Temp, s.ScheduleKey = fm.get("name"), fm.get("units"), fm.get("time"), fm.get("temp"), fm.get("key")
	if err := m.st.UpdatePrefs(func(p *store.Prefs) { p.PutSettings(s) }); err != nil {
		fm.err = err.Error()
		return m, nil
	}
	m.modal, m.form = modalNone, nil
	m.reload()
	m.setStatus("Settings saved", false)
	return m, nil
}
