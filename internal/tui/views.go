package tui

import (
	"fmt"
	"math"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/lemonade1921/milehigh/internal/data"
	"github.com/lemonade1921/milehigh/internal/model"
	"github.com/lemonade1921/milehigh/internal/stats"
	"github.com/lemonade1921/milehigh/internal/worldmap"
)

// ---------- Map ----------

type mapState struct {
	view worldmap.Viewport
}

func (m *Model) years() []int {
	return stats.Compute(m.flights, 0, m.now).Years
}

func cycleYear(cur int, years []int, dir int) int {
	opts := append([]int{0}, years...)
	i := 0
	for j, y := range opts {
		if y == cur {
			i = j
		}
	}
	i = (i + dir + len(opts)) % len(opts)
	return opts[i]
}

func (m *Model) updateMap(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.mapView == nil {
		return m, nil
	}
	v := &m.mapView.view
	sx, sy := (v.Lon1-v.Lon0)*0.15, (v.Lat1-v.Lat0)*0.15
	zoom := func(f float64) {
		cx, cy := (v.Lon0+v.Lon1)/2, (v.Lat0+v.Lat1)/2
		hx, hy := (v.Lon1-v.Lon0)/2*f, (v.Lat1-v.Lat0)/2*f
		if hx > 180 || hx < 1 {
			return
		}
		v.Lon0, v.Lon1, v.Lat0, v.Lat1 = cx-hx, cx+hx, cy-hy, cy+hy
	}
	switch msg.String() {
	case "left", "h":
		v.Lon0, v.Lon1 = v.Lon0-sx, v.Lon1-sx
	case "right", "l":
		v.Lon0, v.Lon1 = v.Lon0+sx, v.Lon1+sx
	case "up", "k":
		if v.Lat1+sy <= 90 {
			v.Lat0, v.Lat1 = v.Lat0+sy, v.Lat1+sy
		}
	case "down", "j":
		if v.Lat0-sy >= -90 {
			v.Lat0, v.Lat1 = v.Lat0-sy, v.Lat1-sy
		}
	case "+", "=":
		zoom(0.7)
	case "-", "_":
		zoom(1 / 0.7)
	case "0":
		m.mapView = nil
	case "y":
		m.mapYear = cycleYear(m.mapYear, m.years(), 1)
		m.mapView = nil
	case "Y":
		m.mapYear = cycleYear(m.mapYear, m.years(), -1)
		m.mapView = nil
	}
	return m, nil
}

func yearLabel(y int) string {
	if y == 0 {
		return "All time"
	}
	return fmt.Sprint(y)
}

func (m *Model) countryLayer() func(string) worldmap.Layer {
	added, removed := m.st.Prefs().VisitedEdits()
	cs := stats.Countries(m.flights, added, removed, m.now)
	return func(iso string) worldmap.Layer {
		if _, ok := cs.Visited[iso]; ok {
			return worldmap.Visited
		}
		if _, ok := cs.Planned[iso]; ok {
			return worldmap.Planned
		}
		return worldmap.Land
	}
}

func (m *Model) viewMap(w, h int) string {
	var fs []*model.Flight
	var pts [][2]float64
	airports := map[string]bool{}
	for _, f := range m.flights {
		if f.Canceled || (m.mapYear != 0 && f.Date.Year() != m.mapYear) {
			continue
		}
		a, b := data.AirportByCode(f.From), data.AirportByCode(f.ArrivalAirport())
		if a == nil || b == nil {
			continue
		}
		fs = append(fs, f)
		pts = append(pts, [2]float64{a.Lat, a.Lon}, [2]float64{b.Lat, b.Lon})
		airports[a.IATA], airports[b.IATA] = true, true
	}
	mapH := h - 2
	if m.mapView == nil {
		v := worldmap.WorldView
		if m.mapYear != 0 && len(pts) > 0 {
			v = worldmap.Fit(pts, w, mapH, 30)
		} else {
			v = worldmap.Fit([][2]float64{{-56, -170}, {80, 180}}, w, mapH, 30)
		}
		m.mapView = &mapState{view: v}
	}
	c := worldmap.New(w, mapH, m.mapView.view)
	c.Countries = m.countryLayer()
	c.DrawLand()
	for _, f := range fs {
		a, b := data.AirportByCode(f.From), data.AirportByCode(f.ArrivalAirport())
		layer := worldmap.Route
		if !f.HasLanded(m.now) {
			layer = worldmap.RouteHighlight
		}
		c.Path(model.GreatCircle(a.Lat, a.Lon, b.Lat, b.Lon, 96), layer)
	}
	for code := range airports {
		a := data.AirportByCode(code)
		c.Dot(a.Lat, a.Lon, worldmap.Airport)
	}
	// Label the busiest airports when zoomed in enough for them not to collide.
	if span := m.mapView.view.Lon1 - m.mapView.view.Lon0; span < 120 {
		st := stats.Compute(fs, 0, m.now.AddDate(10, 0, 0))
		taken := map[[2]int]bool{}
		for i, ap := range st.Airports {
			if i >= 25 {
				break
			}
			a := data.AirportByCode(ap.Key)
			if a == nil {
				continue
			}
			x, y := c.ProjectCell(a.Lat, a.Lon)
			if taken[[2]int{x / 5, y}] {
				continue
			}
			taken[[2]int{x / 5, y}] = true
			c.Label(a.Lat, a.Lon, a.IATA, worldmap.Airport, 1)
		}
	}
	legend := " " + sBold.Render(yearLabel(m.mapYear)) + sDim.Render(fmt.Sprintf(" · %d flights · %d airports   ", len(fs), len(airports))) +
		mapStyles[worldmap.Route].Render("━ flown") + "  " + mapStyles[worldmap.RouteHighlight].Render("━ upcoming") + "  " +
		mapStyles[worldmap.Visited].Render("⣿ visited") + "  " + mapStyles[worldmap.Planned].Render("⣿ planned")
	return truncate(legend, w) + "\n\n" + c.Render(mapStyles)
}

// ---------- Stats ----------

func (m *Model) updateStats(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "right", "l", "y":
		m.statsYear = cycleYear(m.statsYear, m.years(), 1)
	case "left", "h", "Y":
		m.statsYear = cycleYear(m.statsYear, m.years(), -1)
	}
	return m, nil
}

func (m *Model) viewStats(w, h int) string {
	st := stats.Compute(m.flights, m.statsYear, m.now)
	years := append([]int{0}, st.Years...)
	var yl []string
	for _, y := range years {
		if y == m.statsYear {
			yl = append(yl, sTabActive.Render(yearLabel(y)))
		} else {
			yl = append(yl, sTab.Render(yearLabel(y)))
		}
	}
	head := truncate(" "+strings.Join(yl, ""), w)
	if st.Flights == 0 {
		return head + "\n\n" + sDim.Render("  No flights flown yet in "+strings.ToLower(yearLabel(m.statsYear))+".")
	}
	tile := func(big, label string) string {
		return lipgloss.NewStyle().Width(23).PaddingRight(1).Render(sBig.Foreground(cAccent).Render(big) + "\n" + sDim.Render(label))
	}
	km := st.DistanceKm
	tiles := []string{
		tile(fmt.Sprint(st.Flights), plural2(st.Flights, "flight")),
		tile(m.distance(km), fmt.Sprintf("%.1f× around the Earth", km/stats.EarthCircumferenceKm)),
		tile(longDuration(st.Airtime), "in the air"),
		tile(fmt.Sprint(len(st.Airports)), "airports"),
		tile(fmt.Sprint(len(st.Countries)), "countries"),
		tile(fmt.Sprint(len(st.Airlines)), "airlines"),
	}
	perRow := max(1, (w-2)/23)
	var rows []string
	for i := 0; i < len(tiles); i += perRow {
		rows = append(rows, lipgloss.NewStyle().PaddingLeft(1).Render(lipgloss.JoinHorizontal(lipgloss.Top, tiles[i:min(i+perRow, len(tiles))]...)))
	}
	moon := km / stats.MoonDistanceKm * 100
	extra := sDim.Render(fmt.Sprintf(" %.1f%% of the way to the Moon", moon))
	if st.Longest != nil {
		extra += sDim.Render(" · longest ") + sText.Render(st.Longest.From+"→"+st.Longest.ArrivalAirport()) +
			sDim.Render(" ("+m.distance(st.Longest.DistanceKm())+")")
	}
	if st.Shortest != nil && st.Shortest != st.Longest {
		extra += sDim.Render(" · shortest ") + sText.Render(st.Shortest.From+"→"+st.Shortest.ArrivalAirport())
	}

	colW := 38
	cols := max(1, (w-2)/(colW+2))
	lists := []string{
		topList("TOP AIRPORTS", st.Airports, colW, 6, func(c stats.Count) string { return c.Key + "  " + sDim.Render(c.Label) }),
		topList("TOP AIRLINES", st.Airlines, colW, 6, func(c stats.Count) string { return c.Label }),
		topList("TOP ROUTES", st.Routes, colW, 6, func(c stats.Count) string { return c.Key }),
		topList("COUNTRIES", st.Countries, colW, 6, func(c stats.Count) string { return data.Flag(c.Key) + " " + c.Label }),
		topList("AIRCRAFT", st.Aircraft, colW, 6, func(c stats.Count) string { return c.Key }),
		topList("CONTINENTS", st.Continents, colW, 6, func(c stats.Count) string { return c.Label }),
	}
	var grid []string
	for i := 0; i < len(lists); i += cols {
		part := lists[i:min(i+cols, len(lists))]
		for j := range part {
			part[j] = lipgloss.NewStyle().Width(colW + 2).Render(part[j])
		}
		grid = append(grid, lipgloss.NewStyle().PaddingLeft(1).Render(lipgloss.JoinHorizontal(lipgloss.Top, part...)))
	}
	return head + "\n\n" + strings.Join(rows, "\n\n") + "\n\n" + truncate(extra, w) + "\n\n" + strings.Join(grid, "\n\n")
}

func plural2(n int, s string) string {
	if n == 1 {
		return s
	}
	return s + "s"
}

func topList(title string, cs []stats.Count, w, n int, label func(stats.Count) string) string {
	if len(cs) == 0 {
		return ""
	}
	out := []string{sSection.Render(title)}
	maxN := cs[0].N
	for i, c := range cs {
		if i >= n {
			break
		}
		l := truncate(label(c), w-14)
		num := padLeft(fmt.Sprint(c.N), 4)
		out = append(out, pad(l, w-14)+" "+bar(float64(c.N)/float64(maxN), 8, sSky)+pad("", 8-int(math.Ceil(float64(c.N)/float64(maxN)*8)))+sText.Render(num))
	}
	return strings.Join(out, "\n")
}
