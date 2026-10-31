// Package tui is milehigh's terminal interface, built on Bubble Tea.
package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/lemonade1921/milehigh/internal/model"
	"github.com/lemonade1921/milehigh/internal/online"
	"github.com/lemonade1921/milehigh/internal/store"
)

type tab int

const (
	tabFlights tab = iota
	tabMap
	tabStats
	tabPassport
)

var tabNames = []string{"Flights", "Map", "Stats", "Passport"}

type modal int

const (
	modalNone modal = iota
	modalForm
	modalPrompt
	modalConfirm
	modalHelp
)

type promptKind int

const (
	promptImport promptKind = iota
	promptExport
	promptFilter
	promptCountry
)

// Model is the whole app.
type Model struct {
	st       *store.Store
	settings store.Settings
	flights  []*model.Flight // sorted by departure
	now      time.Time
	w, h     int
	tab      tab

	// Flights tab
	rows   []row
	sel    int // index into rows (a flight row)
	scroll int
	filter string

	// Map tab
	mapYear   int
	mapView   *mapState
	statsYear int
	passSel   int

	modal       modal
	form        *form
	editingID   string
	prompt      textinput.Model
	promptKind  promptKind
	promptTitle string
	confirmText string
	confirmFn   func() tea.Cmd

	status    string
	statusErr bool

	live    map[string]*online.Position
	weather map[string][2]*online.Weather
	busy    map[string]bool
}

// New builds the app around an open store.
func New(st *store.Store) *Model {
	m := &Model{st: st, now: store.Now(), live: map[string]*online.Position{}, weather: map[string][2]*online.Weather{}, busy: map[string]bool{}}
	m.reload()
	m.selectNext()
	return m
}

// Run starts the full-screen app.
func Run(st *store.Store) error {
	p := tea.NewProgram(New(st), tea.WithAltScreen(), tea.WithMouseCellMotion())
	_, err := p.Run()
	return err
}

func (m *Model) reload() {
	m.flights = m.st.Flights()
	m.settings = m.st.Prefs().Settings()
	m.buildRows()
}

type tickMsg time.Time

func tick() tea.Cmd {
	return tea.Tick(30*time.Second, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (m *Model) Init() tea.Cmd { return tick() }

func (m *Model) setStatus(s string, isErr bool) {
	m.status, m.statusErr = s, isErr
}

// Update handles every event.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
		return m, nil
	case tickMsg:
		m.now = time.Time(msg)
		return m, tick()
	case liveMsg:
		delete(m.busy, "live"+msg.id)
		if msg.err != nil {
			m.setStatus("Live position: "+friendly(msg.err, "no receiver is hearing this flight right now"), true)
		} else {
			m.live[msg.id] = msg.pos
			m.setStatus(fmt.Sprintf("Live from %s: %s ft, %.0f kt", msg.pos.Source, thousands(float64(msg.pos.AltitudeFt)), msg.pos.SpeedKt), false)
		}
		return m, nil
	case weatherMsg:
		delete(m.busy, "wx"+msg.id)
		if msg.err != nil && msg.w[0] == nil && msg.w[1] == nil {
			m.setStatus("Weather: "+friendly(msg.err, "no forecast for that time (Open-Meteo covers the next 7 days)"), true)
		} else {
			m.weather[msg.id] = msg.w
			m.setStatus("Weather from Open-Meteo", false)
		}
		return m, nil
	case lookupMsg:
		return m.onLookup(msg)
	case refreshMsg:
		delete(m.busy, "ref"+msg.id)
		if msg.err != nil {
			m.setStatus("Schedule: "+friendly(msg.err, "AeroDataBox doesn't know this flight on that date"), true)
			return m, nil
		}
		if f := m.st.Get(msg.id); f != nil {
			nf := msg.status.Apply(*f)
			if err := m.st.Put(&nf); err != nil {
				m.setStatus(err.Error(), true)
			} else {
				m.setStatus("Updated from AeroDataBox", false)
			}
			m.reload()
		}
		return m, nil
	case tea.MouseMsg:
		if m.modal == modalNone && m.tab == tabFlights {
			switch msg.Button {
			case tea.MouseButtonWheelUp:
				m.move(-1)
			case tea.MouseButtonWheelDown:
				m.move(1)
			}
		}
		return m, nil
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		switch m.modal {
		case modalForm:
			return m.updateForm(msg)
		case modalPrompt:
			return m.updatePrompt(msg)
		case modalConfirm:
			switch msg.String() {
			case "y", "Y", "enter":
				m.modal = modalNone
				return m, m.confirmFn()
			default:
				m.modal = modalNone
				m.setStatus("", false)
			}
			return m, nil
		case modalHelp:
			m.modal = modalNone
			return m, nil
		}
		return m.updateKeys(msg)
	}
	return m, nil
}

func friendly(err error, notFound string) string {
	if err == online.ErrNotFound {
		return notFound
	}
	return err.Error()
}

func (m *Model) updateKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	k := msg.String()
	switch k {
	case "q":
		return m, tea.Quit
	case "1", "2", "3", "4":
		m.tab = tab(k[0] - '1')
		return m, nil
	case "tab":
		m.tab = (m.tab + 1) % 4
		return m, nil
	case "shift+tab":
		m.tab = (m.tab + 3) % 4
		return m, nil
	case "?":
		m.modal = modalHelp
		return m, nil
	case "a":
		m.openFlightForm(nil)
		return m, textinput.Blink
	case "i":
		m.openPrompt(promptImport, "Import a Skyline backup or Flighty CSV", "path to file", "")
		return m, nil
	case "x":
		def := filepath.Join(homeDir(), fmt.Sprintf("skyline-backup-%s.json", m.now.Format("2006-01-02")))
		m.openPrompt(promptExport, "Export a backup (opens in Skyline too)", "path to save", def)
		return m, nil
	case "S":
		m.openSettings()
		return m, nil
	}
	switch m.tab {
	case tabFlights:
		return m.updateFlights(msg)
	case tabMap:
		return m.updateMap(msg)
	case tabStats:
		return m.updateStats(msg)
	case tabPassport:
		return m.updatePassport(msg)
	}
	return m, nil
}

func homeDir() string {
	h, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	return h
}

func (m *Model) openPrompt(kind promptKind, title, placeholder, value string) {
	in := textinput.New()
	in.Prompt = sAccent.Render("› ")
	in.Placeholder = placeholder
	in.SetValue(value)
	in.CursorEnd()
	in.Width = 60
	in.Focus()
	m.prompt, m.promptKind, m.promptTitle, m.modal = in, kind, title, modalPrompt
}

func (m *Model) updatePrompt(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.modal = modalNone
		if m.promptKind == promptFilter {
			m.filter = ""
			m.buildRows()
		}
		return m, nil
	case "enter":
		m.modal = modalNone
		v := strings.TrimSpace(m.prompt.Value())
		switch m.promptKind {
		case promptImport:
			m.importFile(expand(v))
		case promptExport:
			m.exportFile(expand(v))
		case promptFilter:
			m.filter = v
			m.buildRows()
			m.selectNext()
		case promptCountry:
			m.markCountry(v)
		}
		return m, nil
	}
	var cmd tea.Cmd
	m.prompt, cmd = m.prompt.Update(msg)
	if m.promptKind == promptFilter {
		m.filter = m.prompt.Value()
		m.buildRows()
	}
	return m, cmd
}

func expand(p string) string {
	if strings.HasPrefix(p, "~/") {
		return filepath.Join(homeDir(), p[2:])
	}
	return p
}

func (m *Model) importFile(path string) {
	if path == "" {
		return
	}
	msg, err := ImportFile(m.st, path)
	if err != nil {
		m.setStatus(err.Error(), true)
		return
	}
	m.reload()
	m.selectNext()
	m.setStatus(msg, false)
}

// ImportFile imports a Skyline backup/flight list or a Flighty CSV and describes what happened.
func ImportFile(st *store.Store, path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	switch store.Detect(b) {
	case store.KindSkyline:
		fs, prefs, err := store.DecodeBackup(b)
		if err != nil {
			return "", fmt.Errorf("not a readable Skyline backup: %w", err)
		}
		r, err := st.Restore(fs, prefs)
		if err != nil {
			return "", err
		}
		s := fmt.Sprintf("Restored %d new flights, updated %d", r.Added, r.Updated)
		if r.Preferences {
			s += ", settings and visited countries"
		}
		return s, nil
	case store.KindFlighty:
		fs, skipped, err := store.ParseFlighty(b)
		if err != nil {
			return "", err
		}
		n, err := st.AddNew(fs)
		if err != nil {
			return "", err
		}
		s := fmt.Sprintf("Imported %d flights from Flighty", n)
		if dup := len(fs) - n; dup > 0 {
			s += fmt.Sprintf(" (%d already in the log)", dup)
		}
		if skipped > 0 {
			s += fmt.Sprintf(", skipped %d rows", skipped)
		}
		return s, nil
	}
	return "", fmt.Errorf("%s isn't a Skyline backup or a Flighty CSV export", filepath.Base(path))
}

func (m *Model) exportFile(path string) {
	if path == "" {
		return
	}
	b, err := m.st.ExportBackup()
	if err == nil {
		err = os.WriteFile(path, b, 0o644)
	}
	if err != nil {
		m.setStatus(err.Error(), true)
		return
	}
	m.setStatus(fmt.Sprintf("Saved %d flights to %s — restore it in Skyline under Settings › Backup", len(m.flights), path), false)
}

// View draws the screen.
func (m *Model) View() string {
	if m.w == 0 {
		return ""
	}
	header := m.viewHeader()
	footer := m.viewFooter()
	bodyH := m.h - lipgloss.Height(header) - lipgloss.Height(footer)
	if bodyH < 3 {
		bodyH = 3
	}
	var body string
	switch m.tab {
	case tabFlights:
		body = m.viewFlights(m.w, bodyH)
	case tabMap:
		body = m.viewMap(m.w, bodyH)
	case tabStats:
		body = m.viewStats(m.w, bodyH)
	case tabPassport:
		body = m.viewPassport(m.w, bodyH)
	}
	body = lipgloss.NewStyle().Width(m.w).Height(bodyH).MaxHeight(bodyH).Render(body)
	screen := lipgloss.JoinVertical(lipgloss.Left, header, body, footer)
	if m.modal != modalNone {
		var box string
		switch m.modal {
		case modalForm:
			box = sModal.Render(m.form.view())
		case modalPrompt:
			box = sModal.Render(sTitle.Render(m.promptTitle) + "\n\n" + m.prompt.View() + "\n\n" + sDim.Render("enter ok · esc cancel"))
		case modalConfirm:
			box = sModal.Render(m.confirmText + "\n\n" + sDim.Render("y yes · any other key no"))
		case modalHelp:
			box = sModal.Render(helpText())
		}
		return lipgloss.Place(m.w, m.h, lipgloss.Center, lipgloss.Center, box,
			lipgloss.WithWhitespaceChars(" "))
	}
	return screen
}

func (m *Model) viewHeader() string {
	logo := sTitle.Render("✈ milehigh")
	var tabs []string
	for i, n := range tabNames {
		label := fmt.Sprintf("%d %s", i+1, n)
		if tab(i) == m.tab {
			tabs = append(tabs, sTabActive.Render(label))
		} else {
			tabs = append(tabs, sTab.Render(label))
		}
	}
	left := logo + "  " + strings.Join(tabs, "")
	right := ""
	if m.settings.Name != "" {
		right = sDim.Render(m.settings.Name + " · ")
	}
	right += sDim.Render(fmt.Sprintf("%d flights", len(m.flights)))
	gap := m.w - lipgloss.Width(left) - lipgloss.Width(right) - 2
	if gap < 1 {
		gap = 1
	}
	line := " " + left + strings.Repeat(" ", gap) + right + " "
	return line + "\n" + sFaint.Render(strings.Repeat("─", m.w))
}

func (m *Model) viewFooter() string {
	var keys []string
	add := func(k, v string) { keys = append(keys, sKey.Render(k)+" "+sDim.Render(v)) }
	switch m.tab {
	case tabFlights:
		add("↑↓", "move")
		add("a", "add")
		add("e", "edit")
		add("d", "delete")
		add("l", "live")
		add("w", "weather")
		add("r", "refresh")
		add("/", "filter")
	case tabMap:
		add("←↑↓→", "pan")
		add("+/-", "zoom")
		add("0", "reset")
		add("y", "year")
	case tabStats:
		add("←→", "year")
	case tabPassport:
		add("↑↓", "move")
		add("m", "mark visited")
		add("u", "unmark")
	}
	add("i", "import")
	add("x", "export")
	add("S", "settings")
	add("?", "help")
	add("q", "quit")
	line := " " + strings.Join(keys, "  ")
	line = truncate(line, m.w)
	st := ""
	if m.status != "" {
		if m.statusErr {
			st = " " + sBad.Render("✗ "+m.status)
		} else {
			st = " " + sGood.Render("✓ ") + sText.Render(m.status)
		}
		st = truncate(st, m.w)
	}
	return sFaint.Render(strings.Repeat("─", m.w)) + "\n" + st + "\n" + line
}

func helpText() string {
	k := func(a, b string) string { return pad(sKey.Render(a), 14) + sText.Render(b) + "\n" }
	return sTitle.Render("milehigh") + sDim.Render("  your flight log in the terminal") + "\n\n" +
		k("1-4 / tab", "switch screen: Flights, Map, Stats, Passport") +
		k("a", "add a flight (type a flight number to look up its route)") +
		k("e / enter", "edit the selected flight") +
		k("d", "delete the selected flight") +
		k("l", "live position from ADS-B receivers (adsb.lol)") +
		k("w", "weather at departure and arrival (Open-Meteo)") +
		k("r", "refresh times and gates (AeroDataBox, your own key)") +
		k("/", "filter flights by code, airport, city or airline") +
		k("t", "jump to the next flight") +
		k("i", "import a Skyline backup or a Flighty CSV") +
		k("x", "export a backup that Skyline on Android can restore") +
		k("S", "settings: name, units, clock, AeroDataBox key") +
		"\n" + sDim.Render("Data: OpenFlights, OurAirports, Natural Earth, adsbdb, adsb.lol, Open-Meteo.") +
		"\n" + sDim.Render("Press any key to close.")
}
