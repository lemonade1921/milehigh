package tui

import (
	"fmt"
	"os"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/lemonade1921/milehigh/internal/model"
	"github.com/lemonade1921/milehigh/internal/store"
)

func demoStore(t *testing.T) *store.Store {
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	add := func(al, num, from, to, dep, arr string) {
		d, _ := model.ParseDate(dep[:10])
		a, _ := model.ParseLocalTime(dep)
		b, _ := model.ParseLocalTime(arr)
		_ = st.Put(&model.Flight{Airline: al, Number: num, From: from, To: to, Date: d, DepLocal: &a, ArrLocal: &b,
			Seat: model.Opt("14A"), Aircraft: model.Opt("A320"), Source: "manual"})
	}
	add("VY", "7820", "BCN", "LHR", "2025-03-02T07:10", "2025-03-02T08:35")
	add("BA", "117", "LHR", "JFK", "2025-03-02T11:25", "2025-03-02T14:20")
	add("IB", "6252", "JFK", "MAD", "2025-03-09T19:00", "2025-03-10T08:15")
	add("JL", "44", "LHR", "HND", "2026-04-11T19:00", "2026-04-12T15:50")
	add("QF", "25", "HND", "SYD", "2026-04-14T21:35", "2026-04-15T08:35")
	add("VY", "1234", "BCN", "PMI", "2026-10-31T09:00", "2026-10-31T09:55")
	add("UX", "7700", "MAD", "EZE", "2026-12-20T23:55", "2026-12-21T08:40")
	add("VY", "1002", "MAD", "BCN", "2025-03-10T10:00", "2025-03-10T11:15")
	_ = st.UpdatePrefs(func(p *store.Prefs) { p.MarkVisited("AD", true) })
	return st
}

func TestRenderAllTabs(t *testing.T) {
	store.Now = func() time.Time { return time.Date(2026, 10, 31, 8, 40, 0, 0, time.UTC) }
	defer func() { store.Now = time.Now }()
	m := New(demoStore(t))
	for _, size := range [][2]int{{140, 42}, {90, 30}, {60, 20}} {
		m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		for tb := 0; tb < 4; tb++ {
			m.tab = tab(tb)
			m.mapView = nil
			out := m.View()
			if out == "" {
				t.Fatalf("empty view tab %d", tb)
			}
			if os.Getenv("MILEHIGH_DUMP") != "" {
				_ = os.WriteFile(fmt.Sprintf("%s-%s-%d.ans", os.Getenv("MILEHIGH_DUMP"), tabNames[tb], size[0]), []byte(out), 0o644)
			}
		}
	}
	// Passport: every sort order, moving down the list, and the jump to flights.
	m.tab = tabPassport
	for i := 0; i < len(passSortNames); i++ {
		m.updatePassport(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
		m.updatePassport(tea.KeyMsg{Type: tea.KeyDown})
		_ = m.View()
	}
	m.Update(tea.WindowSizeMsg{Width: 140, Height: 42})
	for _, iso := range []string{"AD", "AR", "JP"} {
		m.passISO = iso
		out := m.View()
		if os.Getenv("MILEHIGH_DUMP") != "" {
			_ = os.WriteFile(os.Getenv("MILEHIGH_DUMP")+"-pass-"+iso+".ans", []byte(out), 0o644)
		}
	}
	m.passISO = "US"
	m.updatePassport(tea.KeyMsg{Type: tea.KeyEnter})
	if m.tab != tabFlights || len(m.rows) == 0 {
		t.Fatal("enter on a country should show its flights")
	}
	for _, r := range m.rows {
		if r.f != nil && r.f.From != "JFK" && r.f.To != "JFK" {
			t.Fatalf("in:US let %s→%s through", r.f.From, r.f.To)
		}
	}

	// Forms and modals render too.
	m.openFlightForm(nil)
	_ = m.View()
	m.openSettings()
	_ = m.View()
}
