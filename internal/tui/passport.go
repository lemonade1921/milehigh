package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/lemonade1921/milehigh/internal/data"
	"github.com/lemonade1921/milehigh/internal/model"
	"github.com/lemonade1921/milehigh/internal/stats"
	"github.com/lemonade1921/milehigh/internal/store"
)

type passSortMode int

const (
	sortFlights passSortMode = iota
	sortName
	sortFirst
	sortContinent
)

var passSortNames = []string{"most flights", "name", "first visit", "continent"}

// Stamp colours by continent.
var continentColor = map[string]lipgloss.AdaptiveColor{
	"EU": cSky, "AS": cAccent, "NA": cGood, "SA": cWarn, "AF": cBad, "OC": cPlanned, "AN": cDim,
}

type passItem struct {
	header string
	c      *stats.CountryVisit
}

func (m *Model) passportStats() *stats.PassportStats {
	added, removed := m.st.Prefs().VisitedEdits()
	return stats.Passport(m.flights, added, removed, m.now)
}

func continentIndex(code string) int {
	for i, c := range stats.ContinentOrder {
		if c == code {
			return i
		}
	}
	return len(stats.ContinentOrder)
}

func (m *Model) passItems(p *stats.PassportStats) []passItem {
	visited := append([]*stats.CountryVisit(nil), p.Visited...)
	switch m.passSort {
	case sortName:
		sort.SliceStable(visited, func(i, j int) bool { return visited[i].Name < visited[j].Name })
	case sortFirst:
		sort.SliceStable(visited, func(i, j int) bool {
			a, b := visited[i].First, visited[j].First
			switch {
			case a == nil || b == nil:
				return a != nil
			default:
				return a.Departure().Before(b.Departure())
			}
		})
	case sortContinent:
		sort.SliceStable(visited, func(i, j int) bool {
			return continentIndex(visited[i].Continent) < continentIndex(visited[j].Continent)
		})
	}
	var items []passItem
	if len(visited) > 0 {
		items = append(items, passItem{header: fmt.Sprintf("VISITED · %d · by %s", len(visited), passSortNames[m.passSort])})
		last := ""
		for _, c := range visited {
			if m.passSort == sortContinent && c.Continent != last {
				last = c.Continent
				items = append(items, passItem{header: "  " + strings.ToUpper(stats.ContinentName(c.Continent))})
			}
			items = append(items, passItem{c: c})
		}
	}
	if len(p.Planned) > 0 {
		items = append(items, passItem{header: fmt.Sprintf("PLANNED · %d", len(p.Planned))})
		for _, c := range p.Planned {
			items = append(items, passItem{c: c})
		}
	}
	return items
}

// passIndex returns the selected item, choosing the first country if the selection is gone.
func (m *Model) passIndex(items []passItem) int {
	first := -1
	for i, it := range items {
		if it.c == nil {
			continue
		}
		if first < 0 {
			first = i
		}
		if it.c.ISO == m.passISO {
			return i
		}
	}
	if first >= 0 {
		m.passISO = items[first].c.ISO
	}
	return first
}

func (m *Model) passMove(d int) {
	items := m.passItems(m.passportStats())
	i := m.passIndex(items)
	if i < 0 {
		return
	}
	step := 1
	if d < 0 {
		step = -1
	}
	for n := 0; n < abs(d); n++ {
		j := i + step
		for j >= 0 && j < len(items) && items[j].c == nil {
			j += step
		}
		if j < 0 || j >= len(items) {
			break
		}
		i = j
	}
	m.passISO = items[i].c.ISO
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func (m *Model) updatePassport(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		m.passMove(-1)
	case "down", "j":
		m.passMove(1)
	case "pgup":
		m.passMove(-10)
	case "pgdown":
		m.passMove(10)
	case "home", "g":
		m.passMove(-100000)
	case "end", "G":
		m.passMove(100000)
	case "s":
		m.passSort = (m.passSort + 1) % passSortMode(len(passSortNames))
		m.setStatus("Sorted by "+passSortNames[m.passSort], false)
	case "m", "a":
		m.openPrompt(promptCountry, "Add a country you've been to", "country name or ISO code (e.g. Andorra, AD)", "")
	case "u", "d", "delete":
		if c := m.passportStats().Lookup(m.passISO); c != nil {
			iso, name := c.ISO, c.Name
			m.confirmText = fmt.Sprintf("Remove %s %s from your passport?", data.Flag(iso), name)
			if c.Flights > 0 || c.Planned {
				m.confirmText += "\n" + sDim.Render("Your flights stay in the log; the country just won't count as visited.")
			}
			m.confirmFn = func() tea.Cmd {
				_ = m.st.UpdatePrefs(func(p *store.Prefs) { p.MarkVisited(iso, false) })
				m.setStatus("Removed "+name+" from your passport", false)
				return nil
			}
			m.modal = modalConfirm
		}
	case "enter":
		if c := m.passportStats().Lookup(m.passISO); c != nil && (c.Flights > 0 || c.Upcoming > 0) {
			m.filter = "in:" + c.ISO
			m.tab = tabFlights
			m.buildRows()
			m.setStatus("Flights touching "+c.Name+" (esc clears the filter)", false)
		}
	}
	return m, nil
}

func (m *Model) markCountry(q string) {
	q = strings.TrimSpace(q)
	if q == "" {
		return
	}
	var hit *data.Country
	for _, c := range data.Countries() {
		if strings.EqualFold(c.ISO, q) || strings.EqualFold(c.Name, q) {
			hit = c
			break
		}
	}
	if hit == nil {
		var cands []*data.Country
		for _, c := range data.Countries() {
			if strings.HasPrefix(strings.ToLower(c.Name), strings.ToLower(q)) {
				cands = append(cands, c)
			}
		}
		sort.Slice(cands, func(i, j int) bool { return cands[i].Name < cands[j].Name })
		if len(cands) > 0 {
			hit = cands[0]
		}
	}
	if hit == nil {
		m.setStatus("No country called "+q, true)
		return
	}
	_ = m.st.UpdatePrefs(func(p *store.Prefs) { p.MarkVisited(hit.ISO, true) })
	m.passISO = hit.ISO
	m.setStatus("Stamped "+data.Flag(hit.ISO)+" "+hit.Name+" in your passport", false)
}

// ---------- view ----------

func (m *Model) viewPassport(w, h int) string {
	p := m.passportStats()
	if len(p.Visited)+len(p.Planned) == 0 {
		msg := sTitle.Render("Your passport is empty") + "\n\n" +
			sText.Render("Countries fill in as you log flights. Press ") + sKey.Render("m") +
			sText.Render(" to add one you've been to.")
		return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, msg)
	}
	header := m.passHeader(p, w)
	bodyH := h - lipgloss.Height(header) - 1
	if bodyH < 4 {
		bodyH = 4
	}
	items := m.passItems(p)
	sel := m.passIndex(items)
	var cur *stats.CountryVisit
	if sel >= 0 {
		cur = items[sel].c
	}
	rank := 0
	for i, c := range p.Visited {
		if cur != nil && c.ISO == cur.ISO && c.Flights > 0 {
			rank = i + 1
		}
	}

	sep := sFaint.Render(strings.TrimSuffix(strings.Repeat("│\n", bodyH), "\n"))
	const listW = 36
	var body string
	switch {
	case w >= 124:
		ovW := 40
		detW := w - listW - ovW - 6
		body = lipgloss.JoinHorizontal(lipgloss.Top,
			m.passList(items, sel, listW, bodyH), " ", sep, " ",
			fit(m.viewCountry(cur, rank, detW), detW, bodyH), " ", sep, " ",
			fit(strings.Join(m.passOverview(p, ovW), "\n\n"), ovW, bodyH))
	case w >= 80:
		detW := w - listW - 3
		right := m.viewCountry(cur, rank, detW)
		used := lipgloss.Height(right)
		for _, sec := range m.passOverview(p, detW) {
			if used+1+lipgloss.Height(sec) > bodyH {
				break
			}
			right += "\n\n" + sec
			used += 1 + lipgloss.Height(sec)
		}
		body = lipgloss.JoinHorizontal(lipgloss.Top, m.passList(items, sel, listW, bodyH), " ", sep, " ", fit(right, detW, bodyH))
	default:
		body = m.passList(items, sel, w, bodyH)
	}
	return header + "\n\n" + body
}

// fit cuts a block to w×h.
func fit(s string, w, h int) string {
	lines := strings.Split(s, "\n")
	if len(lines) > h {
		lines = lines[:h]
	}
	for i := range lines {
		lines[i] = truncate(lines[i], w)
	}
	return strings.Join(lines, "\n")
}

func (m *Model) passHeader(p *stats.PassportStats, w int) string {
	pct := float64(len(p.Visited)) / stats.WorldCountries * 100
	if pct > 100 {
		pct = 100
	}
	year := m.now.Year()
	tile := func(big, label string) string {
		return lipgloss.NewStyle().Width(20).Render(sBig.Foreground(cAccent).Render(big) + "\n" + sDim.Render(label))
	}
	tiles := []string{
		tile(fmt.Sprint(len(p.Visited)), countriesWord(len(p.Visited))),
		tile(fmt.Sprintf("%.0f%%", pct), fmt.Sprintf("of the world's %d", stats.WorldCountries)),
		tile(fmt.Sprintf("%d / 7", p.ContinentsVisited()), "continents"),
		tile(fmt.Sprint(p.NewIn(year)), fmt.Sprintf("new in %d", year)),
		tile(fmt.Sprint(p.International), "flights abroad"),
		tile(fmt.Sprint(p.Domestic), "domestic flights"),
		tile(fmt.Sprint(len(p.Planned)), "planned"),
	}
	n := max(1, (w-1)/20)
	if n > len(tiles) {
		n = len(tiles)
	}
	return lipgloss.NewStyle().PaddingLeft(1).Render(lipgloss.JoinHorizontal(lipgloss.Top, tiles[:n]...))
}

func countriesWord(n int) string {
	if n == 1 {
		return "country visited"
	}
	return "countries visited"
}

func (m *Model) passList(items []passItem, sel, w, h int) string {
	var lines []string
	selLine := 0
	for i, it := range items {
		if it.c == nil {
			if len(lines) > 0 && !strings.HasPrefix(it.header, "  ") {
				lines = append(lines, "")
			}
			lines = append(lines, " "+sSection.Render(it.header))
			continue
		}
		if i == sel {
			selLine = len(lines)
		}
		c := it.c
		var right string
		switch {
		case c.Planned && c.Next != nil:
			right = sPlannedText.Render(local(c.Next.Departure(), c.Next.From).Format("2 Jan 06"))
		case c.Manual:
			right = sDim.Render("by hand")
		case m.passSort == sortFirst && c.First != nil:
			right = sDim.Render(c.First.Date.Format("Jan 2006"))
		default:
			right = sText.Render(fmt.Sprint(c.Flights)) + sDim.Render(" ✈")
		}
		nameW := w - 7 - lipgloss.Width(right) - 1
		line := data.Flag(c.ISO) + "  " + pad(truncate(c.Name, nameW), nameW) + " " + right
		if i == sel {
			line = lipgloss.NewStyle().Background(cSelBg).Render(pad(sAccent.Render("▌ ")+line, w))
		} else {
			line = "  " + line
		}
		lines = append(lines, line)
	}
	if selLine < m.passScroll+1 {
		m.passScroll = selLine - 1
	}
	if selLine+1 > m.passScroll+h-1 {
		m.passScroll = selLine + 2 - h
	}
	if m.passScroll < 0 {
		m.passScroll = 0
	}
	end := min(len(lines), m.passScroll+h)
	if m.passScroll > end {
		m.passScroll = 0
	}
	out := lines[m.passScroll:end]
	for i := range out {
		out[i] = pad(truncate(out[i], w), w)
	}
	return strings.Join(out, "\n")
}

var sPlannedText = lipgloss.NewStyle().Foreground(cPlanned)

func (m *Model) viewCountry(c *stats.CountryVisit, rank, w int) string {
	if c == nil {
		return ""
	}
	var b []string
	add := func(s ...string) { b = append(b, s...) }

	title := data.Flag(c.ISO) + "  " + sBig.Render(c.Name)
	var pill string
	switch {
	case c.Planned:
		pill = lipgloss.NewStyle().Foreground(lipgloss.Color("#0B0F2E")).Background(cPlanned).Bold(true).Padding(0, 1).Render("PLANNED")
	case c.Manual:
		pill = sPillDim.Render("ADDED BY HAND")
	case rank == 1:
		pill = sPillAmb.Render("MOST VISITED")
	case rank > 0:
		pill = sPillDim.Render(fmt.Sprintf("#%d BY FLIGHTS", rank))
	}
	sub := stats.ContinentName(c.Continent) + " · " + c.ISO
	if n := len(c.Airports); n > 0 {
		sub += fmt.Sprintf(" · %d %s", n, plural2(n, "airport"))
	}
	head := []string{title, sDim.Render(sub)}
	if pill != "" {
		head = append(head, "", pill)
	}
	// The passport stamp sits beside the name when there's room.
	stamp := m.stamp(c)
	if sw := lipgloss.Width(stamp); w-sw-2 >= 24 {
		left := lipgloss.NewStyle().Width(w - sw - 2).Render(fit(strings.Join(head, "\n"), w-sw-2, 10))
		add(lipgloss.JoinHorizontal(lipgloss.Top, left, "  ", stamp))
	} else {
		add(head...)
	}
	add("")

	var facts []string
	fact := func(k, v string) { facts = append(facts, sDim.Render(pad(k, 13))+v) }
	visit := func(f *model.Flight) string {
		d := local(f.Departure(), f.From)
		when := relative(f.Departure(), m.now)
		switch {
		case f.InAir(m.now):
			when = "in the air now"
		case f.HasLanded(m.now) && m.now.Sub(f.Arrival()) < 24*time.Hour:
			when = "landed " + relative(f.Arrival(), m.now)
		}
		return sText.Render(d.Format("2 Jan 2006")) + sDim.Render(" · "+when)
	}
	route := func(f *model.Flight) string {
		return sDim.Render(strings.TrimSpace(flightCode(f)+" ") + " " + f.From + " → " + f.ArrivalAirport())
	}
	switch {
	case c.Planned:
		if c.Next != nil {
			fact("First visit", visit(c.Next))
			fact("", route(c.Next))
		}
	case c.Manual:
		facts = append(facts, sDim.Render("Added by hand: no flights logged here yet."))
	default:
		fact("Flights", sText.Render(fmt.Sprint(c.Flights)))
		if c.International > 0 && c.Domestic > 0 {
			fact("", sDim.Render(fmt.Sprintf("%d international · %d domestic", c.International, c.Domestic)))
		} else if c.Domestic > 0 {
			fact("", sDim.Render("all domestic"))
		}
		fact("First visit", visit(c.First))
		fact("", route(c.First))
		if c.Last != c.First {
			fact("Last visit", visit(c.Last))
			fact("", route(c.Last))
		}
	}
	if !c.Planned && c.Next != nil {
		fact("Next visit", visit(c.Next))
		fact("", route(c.Next))
	}
	if c.Upcoming > 1 {
		fact("Upcoming", sText.Render(fmt.Sprint(c.Upcoming))+sDim.Render(" flights booked"))
	}

	add(facts...)

	// Airports and the countries it's connected to.
	listW := (w - 3) / 2
	ap := countList("AIRPORTS", c.Airports, listW, 5, func(x stats.Count) string { return x.Key + "  " + sDim.Render(x.Label) })
	pa := countList("FLOWN TO AND FROM", c.Partners, listW, 5, func(x stats.Count) string { return data.Flag(x.Key) + " " + x.Label })
	switch {
	case ap != "" && pa != "" && listW >= 26:
		add("", lipgloss.JoinHorizontal(lipgloss.Top, lipgloss.NewStyle().Width(listW).Render(ap), "   ", pa))
	default:
		ap = countList("AIRPORTS", c.Airports, w, 5, func(x stats.Count) string { return x.Key + "  " + sDim.Render(x.Label) })
		pa = countList("FLOWN TO AND FROM", c.Partners, w, 5, func(x stats.Count) string { return data.Flag(x.Key) + " " + x.Label })
		for _, s := range []string{ap, pa} {
			if s != "" {
				add("", s)
			}
		}
	}
	if len(c.Airlines) > 0 {
		var al []string
		for i, x := range c.Airlines {
			if i == 4 {
				al = append(al, fmt.Sprintf("+%d more", len(c.Airlines)-4))
				break
			}
			al = append(al, sText.Render(x.Label)+sDim.Render(fmt.Sprintf(" %d", x.N)))
		}
		add("", sSection.Render("AIRLINES"), strings.Join(al, sDim.Render(" · ")))
	}
	if c.Flights > 0 || c.Upcoming > 0 {
		add("", sDim.Render("enter: show these flights"))
	}
	return strings.Join(b, "\n")
}

// stamp draws a passport stamp: arrival date and airport, coloured by continent.
func (m *Model) stamp(c *stats.CountryVisit) string {
	col, ok := continentColor[c.Continent]
	if !ok {
		col = cDim
	}
	st := lipgloss.NewStyle().Foreground(col)
	var l1, l2, l3 string
	switch {
	case c.Planned:
		l1 = "PLANNED"
		if c.Next != nil {
			l2 = strings.ToUpper(local(c.Next.Departure(), c.Next.From).Format("02 Jan 2006"))
		}
		l3 = c.ISO
	case c.Manual:
		l1, l2, l3 = "VISITED", "· · ·", c.ISO
	default:
		l1 = "✈ ARRIVED"
		l2 = strings.ToUpper(c.First.Date.Format("02 Jan 2006"))
		l3 = c.EntryAirport + " · " + c.ISO
	}
	inner := lipgloss.JoinVertical(lipgloss.Center, st.Bold(true).Render(l1), st.Render(l2), st.Render(l3))
	border := lipgloss.DoubleBorder()
	if c.Planned {
		border = lipgloss.NormalBorder()
	}
	return lipgloss.NewStyle().Border(border).BorderForeground(col).Width(15).Align(lipgloss.Center).Render(inner)
}

func countList(title string, cs []stats.Count, w, n int, label func(stats.Count) string) string {
	if len(cs) == 0 {
		return ""
	}
	out := []string{sSection.Render(title)}
	maxN := cs[0].N
	for i, c := range cs {
		if i == n {
			out = append(out, sDim.Render(fmt.Sprintf("+%d more", len(cs)-n)))
			break
		}
		num := padLeft(fmt.Sprint(c.N), 3)
		lw := w - 11
		out = append(out, pad(truncate(label(c), lw), lw)+" "+track(float64(c.N)/float64(maxN), 6, sSky)+" "+sText.Render(num))
	}
	return strings.Join(out, "\n")
}

// track is a progress bar with a faint track behind it.
func track(frac float64, w int, st lipgloss.Style) string {
	n := int(frac*float64(w) + 0.5)
	if frac > 0 && n == 0 {
		n = 1
	}
	if n > w {
		n = w
	}
	return st.Render(strings.Repeat("━", n)) + sFaint.Render(strings.Repeat("━", w-n))
}

// passOverview is the passport-wide stats, as separate sections.
func (m *Model) passOverview(p *stats.PassportStats, w int) []string {
	var secs []string

	// Continents.
	lines := []string{sSection.Render("CONTINENTS")}
	for _, c := range p.Continents {
		frac := 0.0
		if c.Total > 0 {
			frac = float64(c.Visited) / float64(c.Total)
		}
		num := fmt.Sprintf("%d/%d", c.Visited, c.Total)
		bw := max(4, w-15-7)
		name := sDim.Render(pad(c.Name, 15))
		if c.Visited > 0 {
			name = sText.Render(pad(c.Name, 15))
		}
		col, _ := continentColor[c.Code]
		lines = append(lines, name+track(frac, bw, lipgloss.NewStyle().Foreground(col))+" "+sDim.Render(padLeft(num, 6)))
	}
	secs = append(secs, strings.Join(lines, "\n"))

	// New countries per year (the last eight years with any).
	if len(p.NewByYear) > 0 {
		lines = []string{sSection.Render("NEW COUNTRIES BY YEAR")}
		ys := p.NewByYear
		if len(ys) > 8 {
			ys = ys[len(ys)-8:]
		}
		maxN := 1
		for _, y := range ys {
			maxN = max(maxN, y.N)
		}
		for _, y := range ys {
			lines = append(lines, sDim.Render(fmt.Sprint(y.Year))+"  "+bar(float64(y.N)/float64(maxN), max(4, w-12), sAccent)+" "+sText.Render(fmt.Sprint(y.N)))
		}
		secs = append(secs, strings.Join(lines, "\n"))
	}

	// Highlights.
	var hl []string
	h := func(k string, c *stats.CountryVisit, v string) {
		if c != nil {
			hl = append(hl, sDim.Render(pad(k, 13))+data.Flag(c.ISO)+" "+sText.Render(c.Name)+sDim.Render(" · "+v))
		}
	}
	var most, first, newest, away *stats.CountryVisit
	for _, c := range p.Visited {
		if c.First == nil {
			continue
		}
		if most == nil || c.Flights > most.Flights {
			most = c
		}
		if first == nil || c.First.Departure().Before(first.First.Departure()) {
			first = c
		}
		if newest == nil || c.First.Departure().After(newest.First.Departure()) {
			newest = c
		}
		if c.Next == nil && (away == nil || c.Last.Departure().Before(away.Last.Departure())) {
			away = c
		}
	}
	if most != nil {
		h("Most visited", most, fmt.Sprintf("%d %s", most.Flights, plural2(most.Flights, "flight")))
	}
	if first != nil {
		h("First stamp", first, first.First.Date.Format("Jan 2006"))
	}
	if newest != nil && newest != first {
		h("Newest stamp", newest, newest.First.Date.Format("Jan 2006"))
	}
	if away != nil && away != most && m.now.Sub(away.Last.Departure()) > 30*24*time.Hour {
		h("Longest away", away, relative(away.Last.Departure(), m.now))
	}
	if len(p.Planned) > 0 && p.Planned[0].Next != nil {
		h("Next new one", p.Planned[0], relative(p.Planned[0].Next.Departure(), m.now))
	}
	if len(hl) > 0 {
		secs = append(secs, sSection.Render("HIGHLIGHTS")+"\n"+strings.Join(hl, "\n"))
	}
	return secs
}
