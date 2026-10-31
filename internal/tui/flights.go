package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/lemonade1921/milehigh/internal/data"
	"github.com/lemonade1921/milehigh/internal/model"
	"github.com/lemonade1921/milehigh/internal/online"
	"github.com/lemonade1921/milehigh/internal/worldmap"
)

type row struct {
	header string
	f      *model.Flight
}

func (m *Model) matches(f *model.Flight) bool {
	q := strings.ToLower(strings.TrimSpace(m.filter))
	if q == "" {
		return true
	}
	hay := strings.ToLower(strings.Join([]string{
		f.Airline + f.Number, f.Airline + " " + f.Number, f.From, f.To, city(f.From), city(f.To),
		data.AirlineName(f.Airline), model.Str(f.Aircraft), model.Str(f.Notes), model.Str(f.PNR), f.Date.String(),
	}, " "))
	for _, w := range strings.Fields(q) {
		if !strings.Contains(hay, w) {
			return false
		}
	}
	return true
}

func (m *Model) buildRows() {
	var keep string
	if f := m.selected(); f != nil {
		keep = f.ID
	}
	m.rows = nil
	var up, past []*model.Flight
	for _, f := range m.flights {
		if !m.matches(f) {
			continue
		}
		if f.HasLanded(m.now) || (f.Canceled && f.Departure().Before(m.now)) {
			past = append(past, f)
		} else {
			up = append(up, f)
		}
	}
	if len(up) > 0 {
		m.rows = append(m.rows, row{header: fmt.Sprintf("UPCOMING · %d", len(up))})
		for _, f := range up {
			m.rows = append(m.rows, row{f: f})
		}
	}
	year := -1
	for i := len(past) - 1; i >= 0; i-- {
		f := past[i]
		if y := f.Date.Year(); y != year {
			year = y
			n := 0
			for _, g := range past {
				if g.Date.Year() == y {
					n++
				}
			}
			m.rows = append(m.rows, row{header: fmt.Sprintf("%d · %d", y, n)})
		}
		m.rows = append(m.rows, row{f: f})
	}
	m.sel = 0
	for i, r := range m.rows {
		if r.f != nil && r.f.ID == keep {
			m.sel = i
			return
		}
	}
	m.fixSel(1)
}

func (m *Model) fixSel(dir int) {
	if len(m.rows) == 0 {
		m.sel = 0
		return
	}
	if m.sel < 0 {
		m.sel = 0
	}
	if m.sel >= len(m.rows) {
		m.sel = len(m.rows) - 1
	}
	for m.sel >= 0 && m.sel < len(m.rows) && m.rows[m.sel].f == nil {
		m.sel += dir
	}
	if m.sel < 0 || m.sel >= len(m.rows) {
		m.sel = 0
		for m.sel < len(m.rows) && m.rows[m.sel].f == nil {
			m.sel++
		}
		if m.sel >= len(m.rows) {
			m.sel = 0
		}
	}
}

func (m *Model) selected() *model.Flight {
	if m.sel >= 0 && m.sel < len(m.rows) {
		return m.rows[m.sel].f
	}
	return nil
}

// selectNext picks the next flight that hasn't landed, else the latest one.
func (m *Model) selectNext() {
	for i, r := range m.rows {
		if r.f != nil && !r.f.HasLanded(m.now) {
			m.sel = i
			return
		}
	}
	m.sel = 0
	m.fixSel(1)
}

func (m *Model) move(d int) {
	old := m.sel
	m.sel += d
	if m.sel < 0 || m.sel >= len(m.rows) {
		m.sel = old
		return
	}
	m.fixSel(d)
	if m.rows[m.sel].f == nil {
		m.sel = old
	}
}

func (m *Model) updateFlights(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	f := m.selected()
	switch msg.String() {
	case "up", "k":
		m.move(-1)
	case "down", "j":
		m.move(1)
	case "pgup":
		m.move(-8)
	case "pgdown":
		m.move(8)
	case "home", "g":
		m.sel = 0
		m.fixSel(1)
	case "end", "G":
		m.sel = len(m.rows) - 1
		m.fixSel(-1)
	case "t":
		m.selectNext()
	case "/":
		m.openPrompt(promptFilter, "Filter flights", "code, airport, city, airline, year…", m.filter)
	case "esc":
		if m.filter != "" {
			m.filter = ""
			m.buildRows()
		}
	case "e", "enter":
		if f != nil {
			m.openFlightForm(f)
			return m, textinput.Blink
		}
	case "d", "delete":
		if f != nil {
			id := f.ID
			m.confirmText = fmt.Sprintf("Delete %s %s → %s on %s?", flightCode(f), f.From, f.To, f.Date.Format("2 Jan 2006"))
			m.confirmFn = func() tea.Cmd {
				if err := m.st.Delete(id); err != nil {
					m.setStatus(err.Error(), true)
				} else {
					m.setStatus("Flight deleted", false)
				}
				m.reload()
				return nil
			}
			m.modal = modalConfirm
		}
	case "l":
		if f != nil {
			m.busy["live"+f.ID] = true
			m.setStatus("Asking ADS-B receivers where "+flightCode(f)+" is…", false)
			return m, liveCmd(f)
		}
	case "w":
		if f != nil {
			m.busy["wx"+f.ID] = true
			m.setStatus("Fetching the forecast…", false)
			return m, weatherCmd(f)
		}
	case "r":
		if f != nil {
			if m.settings.ScheduleKey == "" {
				m.setStatus("Refreshing times needs your own free AeroDataBox key: press S", true)
				return m, nil
			}
			m.busy["ref"+f.ID] = true
			m.setStatus("Checking the schedule…", false)
			return m, refreshCmd(m.settings.ScheduleKey, f)
		}
	}
	return m, nil
}

type liveMsg struct {
	id  string
	pos *online.Position
	err error
}

func liveCmd(f *model.Flight) tea.Cmd {
	return func() tea.Msg {
		p, err := online.LivePosition(f)
		return liveMsg{f.ID, p, err}
	}
}

type weatherMsg struct {
	id  string
	w   [2]*online.Weather
	err error
}

func weatherCmd(f *model.Flight) tea.Cmd {
	return func() tea.Msg {
		var out [2]*online.Weather
		var lastErr error
		for i, x := range []struct {
			code string
			t    time.Time
		}{{f.From, f.Departure()}, {f.ArrivalAirport(), f.Arrival()}} {
			a := data.AirportByCode(x.code)
			if a == nil {
				continue
			}
			w, err := online.WeatherAt(a.Lat, a.Lon, x.t.UTC())
			if err != nil {
				lastErr = err
			}
			out[i] = w
		}
		return weatherMsg{f.ID, out, lastErr}
	}
}

type refreshMsg struct {
	id     string
	status *online.Status
	err    error
}

func refreshCmd(key string, f *model.Flight) tea.Cmd {
	return func() tea.Msg {
		s, err := online.FlightStatus(key, f)
		return refreshMsg{f.ID, s, err}
	}
}

// ---- views ----

func (m *Model) viewFlights(w, h int) string {
	if len(m.flights) == 0 {
		return m.viewEmpty(w, h)
	}
	lw := 46
	if w < 100 {
		lw = 38
	}
	if w < 70 {
		return m.viewList(w, h)
	}
	list := m.viewList(lw, h)
	sep := sFaint.Render(strings.Repeat("│\n", h-1) + "│")
	detail := m.viewDetail(w-lw-3, h)
	return lipgloss.JoinHorizontal(lipgloss.Top, list, " ", sep, " ", detail)
}

func (m *Model) viewEmpty(w, h int) string {
	msg := sTitle.Render("Your logbook is empty") + "\n\n" +
		sText.Render("Press ") + sKey.Render("a") + sText.Render(" to add a flight, or ") + sKey.Render("i") +
		sText.Render(" to import a Skyline backup or a Flighty CSV.") + "\n\n" +
		sDim.Render("Your log is saved in "+m.st.Path())
	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, msg)
}

func (m *Model) viewList(w, h int) string {
	var lines []string
	selLine := 0
	for i, r := range m.rows {
		if r.f == nil {
			if len(lines) > 0 {
				lines = append(lines, "")
			}
			lines = append(lines, " "+sSection.Render(r.header))
			continue
		}
		if i == m.sel {
			selLine = len(lines)
		}
		lines = append(lines, m.listRow(r.f, w, i == m.sel)...)
	}
	if m.filter != "" {
		lines = append([]string{sAccent.Render(" filter: ") + sText.Render(m.filter) + sDim.Render("  (esc clears)"), ""}, lines...)
		selLine += 2
	}
	if len(m.rows) == 0 {
		lines = append(lines, sDim.Render(" No flights match."))
	}
	// Keep the selection in view.
	if selLine < m.scroll+1 {
		m.scroll = selLine - 1
	}
	if selLine+2 > m.scroll+h {
		m.scroll = selLine + 2 - h
	}
	if m.scroll < 0 {
		m.scroll = 0
	}
	end := m.scroll + h
	if end > len(lines) {
		end = len(lines)
	}
	if m.scroll > end {
		m.scroll = 0
	}
	out := lines[m.scroll:end]
	for i := range out {
		out[i] = pad(truncate(out[i], w), w)
	}
	return strings.Join(out, "\n")
}

func (m *Model) listRow(f *model.Flight, w int, sel bool) []string {
	dep := local(f.Departure(), f.From)
	date := dep.Format("Mon 02 Jan")
	route := f.From + " → " + f.ArrivalAirport()
	code := flightCode(f)
	l1 := sDim.Render(date) + "  " + sBold.Render(route)
	right := sDim.Render(code)
	gap := w - 2 - lipgloss.Width(l1) - lipgloss.Width(right) - 1
	if gap < 1 {
		gap = 1
	}
	line1 := l1 + strings.Repeat(" ", gap) + right

	var info string
	switch {
	case f.Canceled:
		info = sBad.Render("Canceled")
	case f.InAir(m.now):
		info = sSky.Render("In the air · lands " + relative(f.Arrival(), m.now))
	case !f.HasLanded(m.now):
		info = sText.Render(m.clock(dep)) + sDim.Render(" · "+relative(f.Departure(), m.now))
	default:
		info = sDim.Render(m.clock(dep) + " → " + m.clock(local(f.Arrival(), f.ArrivalAirport())) + " · " + duration(f.Duration()))
		if d, ok := f.ArrDelay(); ok && d >= 15*time.Minute {
			t, st := delayText(d)
			info += " " + st.Render(t)
		}
	}
	room := w - 2 - lipgloss.Width(info) - 2
	cities := city(f.From) + " – " + city(f.ArrivalAirport())
	var line2 string
	if room >= 8 {
		line2 = sDim.Render(pad(truncate(cities, room), room)) + "  " + info
	} else {
		line2 = info
	}

	marker := "  "
	if sel {
		marker = sAccent.Render("▌ ")
	}
	out := []string{marker + line1, marker + line2}
	if sel {
		for i := range out {
			out[i] = lipgloss.NewStyle().Background(cSelBg).Render(pad(truncate(out[i], w), w))
		}
	}
	return out
}

func (m *Model) viewDetail(w, h int) string {
	f := m.selected()
	if f == nil {
		return ""
	}
	now := m.now
	var b []string
	add := func(s ...string) { b = append(b, s...) }

	// Title.
	title := sBig.Render(flightCode(f))
	if n := data.AirlineName(f.Airline); n != "" && n != f.Airline {
		title += sDim.Render("  " + n)
	}
	pill := m.statusPill(f, now)
	gap := w - lipgloss.Width(title) - lipgloss.Width(pill)
	if gap < 1 {
		add(title, pill)
	} else {
		add(title + strings.Repeat(" ", gap) + pill)
	}
	add(sDim.Render(local(f.Departure(), f.From).Format("Monday 2 January 2006")), "")

	// Big route with progress.
	to := f.ArrivalAirport()
	frac := 0.0
	switch {
	case f.HasLanded(now):
		frac = 1
	case f.HasDeparted(now):
		frac = float64(now.Sub(f.Departure())) / float64(f.Duration())
	}
	trackW := w - 12
	if trackW < 6 {
		trackW = 6
	}
	pos := int(frac * float64(trackW-1))
	track := sAccent.Render(strings.Repeat("━", pos)) + sAccent.Render("✈") + sFaint.Render(strings.Repeat("┄", trackW-1-pos))
	add(sBig.Foreground(cAccent).Render(" "+f.From+" ") + " " + track + " " + sBig.Foreground(cAccent).Render(" "+to+" "))
	c1, c2 := truncate(city(f.From), w/2-1), truncate(city(to), w/2-1)
	add(" " + sText.Render(c1) + strings.Repeat(" ", max(1, w-2-lipgloss.Width(c1)-lipgloss.Width(c2))) + sText.Render(c2))
	sub := m.distance(f.DistanceKm()) + " · " + duration(f.Duration())
	if f.InAir(now) {
		sub = fmt.Sprintf("%s · %s left", sub, duration(f.Arrival().Sub(now)))
	}
	add(lipgloss.PlaceHorizontal(w, lipgloss.Center, sDim.Render(sub)), "")

	// Departure and arrival columns.
	colW := (w - 3) / 2
	wx := m.weather[f.ID]
	depCol := m.endpoint("DEPARTURE", f.From, f.DepLocal, f.DepEstimated, f.DepActual, f.Departure(),
		model.Str(f.DepTerminal), model.Str(f.DepGate), wx[0], colW, func() (time.Duration, bool) { return f.DepDelay() })
	arrCol := m.endpoint("ARRIVAL", to, f.ArrLocal, f.ArrEstimated, f.ArrActual, f.Arrival(),
		model.Str(f.ArrTerminal), model.Str(f.ArrGate), wx[1], colW, func() (time.Duration, bool) { return f.ArrDelay() })
	add(lipgloss.JoinHorizontal(lipgloss.Top, pad2(depCol, colW), "   ", arrCol), "")

	// Details.
	var facts [][2]string
	fact := func(k, v string) {
		if strings.TrimSpace(v) != "" {
			facts = append(facts, [2]string{k, v})
		}
	}
	fact("Aircraft", data.AircraftName(model.Str(f.Aircraft)))
	fact("Tail", model.Str(f.Tail))
	seat := model.Str(f.Seat)
	if c := model.Str(f.Cabin); c != "" {
		seat = strings.TrimSpace(seat + " · " + c)
		seat = strings.TrimPrefix(seat, "· ")
	}
	fact("Seat", seat)
	fact("Booking", model.Str(f.PNR))
	if f.DivertedTo != nil {
		fact("Diverted", "to "+*f.DivertedTo+" (planned "+f.To+")")
	}
	if f.BoardingPass != nil {
		fact("Pass", "boarding pass saved ("+strings.ToLower(strings.ReplaceAll(f.BoardingPass.Format, "_", " "))+")")
	}
	for _, t := range model.Trips(m.flights) {
		for _, l := range t.Legs {
			if l.ID == f.ID {
				codes := []string{t.Legs[0].From}
				for _, x := range t.Legs {
					codes = append(codes, x.ArrivalAirport())
				}
				fact("Trip", strings.Join(codes, " → ")+fmt.Sprintf(" (%d connection%s)", len(t.Legs)-1, plural(len(t.Legs)-1)))
			}
		}
	}
	fact("Notes", model.Str(f.Notes))
	if p := m.live[f.ID]; p != nil {
		alt := "on the ground"
		if !p.OnGround {
			alt = thousands(float64(p.AltitudeFt)) + " ft"
		}
		fact("Live", fmt.Sprintf("%s · %.0f kt · %s %s", alt, p.SpeedKt, p.Registration, p.Type)+sDim.Render(" ("+relative(p.Time, now)+", "+p.Source+")"))
	}
	for _, kv := range facts {
		add(sDim.Render(pad(kv[0], 10)) + sText.Render(truncate(kv[1], w-10)))
	}

	// Route map in whatever room is left.
	used := len(b)
	mapH := h - used - 1
	if mapH >= 5 {
		add("")
		add(m.routeMap(f, w, mapH))
	}
	return strings.Join(b, "\n")
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

func pad2(s string, w int) string {
	lines := strings.Split(s, "\n")
	for i := range lines {
		lines[i] = pad(lines[i], w)
	}
	return strings.Join(lines, "\n")
}

func (m *Model) endpoint(title, code string, sched, est, act *model.LocalTime, best time.Time, term, gate string, wx *online.Weather, w int, delay func() (time.Duration, bool)) string {
	var b []string
	b = append(b, sSection.Render(title))
	b = append(b, sText.Render(truncate(airportName(code), w)))
	t := local(best, code)
	big := sBig.Render(m.clock(t))
	if d, ok := delay(); ok {
		txt, st := delayText(d)
		big = st.Bold(true).Render(m.clock(t)) + " " + st.Render(txt)
	}
	b = append(b, big)
	kind := "estimated"
	switch {
	case act != nil:
		kind = "actual"
	case est != nil:
		kind = "estimated"
	case sched != nil:
		kind = "scheduled"
	default:
		kind = "no time set"
	}
	line := kind
	if sched != nil && (act != nil || est != nil) {
		line += " · sched " + m.clock(sched.Time)
	}
	if a := data.AirportByCode(code); a != nil {
		_, off := t.Zone()
		line += fmt.Sprintf(" · UTC%+d", off/3600)
	}
	b = append(b, sDim.Render(truncate(line, w)))
	var tg []string
	if term != "" {
		tg = append(tg, "Terminal "+term)
	}
	if gate != "" {
		tg = append(tg, "Gate "+gate)
	}
	if len(tg) > 0 {
		b = append(b, sAccent.Render(strings.Join(tg, " · ")))
	}
	if wx != nil {
		temp := fmt.Sprintf("%.0f°C", wx.Celsius)
		if m.settings.Temp == "F" {
			temp = fmt.Sprintf("%.0f°F", wx.Celsius*9/5+32)
		}
		b = append(b, sText.Render(online.WeatherText(wx.Code, wx.IsDay)+" "+temp))
	}
	return strings.Join(b, "\n")
}

func (m *Model) routeMap(f *model.Flight, w, h int) string {
	a, b := data.AirportByCode(f.From), data.AirportByCode(f.ArrivalAirport())
	if a == nil || b == nil {
		return sDim.Render("(map unavailable: unknown airport)")
	}
	path := model.GreatCircle(a.Lat, a.Lon, b.Lat, b.Lon, 128)
	v := worldmap.Fit(path, w, h, 12)
	c := worldmap.New(w, h, v)
	c.DrawLand()
	layer := worldmap.RouteHighlight
	if f.HasLanded(m.now) {
		layer = worldmap.Route
	}
	c.Path(path, layer)
	c.Dot(a.Lat, a.Lon, worldmap.Airport)
	c.Dot(b.Lat, b.Lon, worldmap.Airport)
	ax, _ := c.ProjectCell(a.Lat, a.Lon)
	bx, _ := c.ProjectCell(b.Lat, b.Lon)
	da, db := 1, 1
	if ax > bx {
		da, db = 1, -4
	} else {
		da, db = -4, 1
	}
	c.Label(a.Lat, a.Lon, a.IATA, worldmap.Airport, da)
	c.Label(b.Lat, b.Lon, b.IATA, worldmap.Airport, db)
	if p := m.live[f.ID]; p != nil {
		c.Label(p.Lat, p.Lon, "✈", worldmap.Plane, 0)
	} else if f.InAir(m.now) {
		frac := float64(m.now.Sub(f.Departure())) / float64(f.Duration())
		i := int(frac * float64(len(path)-1))
		if i >= 0 && i < len(path) {
			c.Label(path[i][0], path[i][1], "✈", worldmap.Plane, 0)
		}
	}
	return c.Render(mapStyles)
}
