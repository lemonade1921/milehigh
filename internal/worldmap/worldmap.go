// Package worldmap draws a dotted world map with flight routes in the terminal, using braille
// characters (each cell is a 2×4 grid of dots) over the Natural Earth country raster.
package worldmap

import (
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/lemonade1921/milehigh/internal/data"
)

// Layer decides a cell's colour; higher layers win.
type Layer uint8

const (
	None Layer = iota
	Land
	Planned
	Visited
	Route
	RouteHighlight
	Airport
	Plane
)

// Viewport is the part of the world shown: equirectangular, longitudes may run past ±180.
type Viewport struct {
	Lon0, Lon1, Lat0, Lat1 float64 // west, east, south, north
}

// WorldView is the whole world (minus the poles).
var WorldView = Viewport{Lon0: -180, Lon1: 180, Lat0: -58, Lat1: 84}

// Canvas is a braille drawing surface w×h cells (2w×4h dots).
type Canvas struct {
	W, H   int
	View   Viewport
	dots   []Layer // per dot
	labels map[int]label
	// Countries is called for each land dot: return Visited/Planned/Land.
	Countries func(iso string) Layer
	// DotMatrix draws land as a sparse grid of dots instead of solid.
	DotMatrix bool
}

type label struct {
	text  string
	layer Layer
}

// New makes a canvas.
func New(w, h int, v Viewport) *Canvas {
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	return &Canvas{W: w, H: h, View: v, dots: make([]Layer, w*2*h*4), labels: map[int]label{}, DotMatrix: true}
}

// Fit returns a viewport around the points (lat, lon) that fills a w×h cell canvas without
// distorting it (braille dots are about square), with some padding. Longitudes are unwrapped
// around the first point so routes over the Pacific stay together.
func Fit(points [][2]float64, w, h int, minSpanDeg float64) Viewport {
	if len(points) == 0 {
		return WorldView
	}
	ref := points[0][1]
	lat0, lat1, lon0, lon1 := 90.0, -90.0, 1e9, -1e9
	for _, p := range points {
		lon := Unwrap(p[1], ref)
		lat0, lat1 = math.Min(lat0, p[0]), math.Max(lat1, p[0])
		lon0, lon1 = math.Min(lon0, lon), math.Max(lon1, lon)
	}
	cx, cy := (lon0+lon1)/2, (lat0+lat1)/2
	spanX := math.Max((lon1-lon0)*1.35, minSpanDeg)
	spanY := math.Max((lat1-lat0)*1.5, minSpanDeg/2)
	aspect := float64(w*2) / float64(h*4) // dots wide per dot high
	if spanX/spanY < aspect {
		spanX = spanY * aspect
	} else {
		spanY = spanX / aspect
	}
	if spanX > 360 {
		spanX = 360
		spanY = spanX / aspect
	}
	if spanY > 160 {
		spanY = 160
	}
	cy = math.Max(-80+spanY/2, math.Min(85-spanY/2, cy))
	return Viewport{Lon0: cx - spanX/2, Lon1: cx + spanX/2, Lat0: cy - spanY/2, Lat1: cy + spanY/2}
}

// Unwrap moves lon by ±360 so it lies within 180° of ref.
func Unwrap(lon, ref float64) float64 {
	for lon-ref > 180 {
		lon -= 360
	}
	for lon-ref < -180 {
		lon += 360
	}
	return lon
}

func (c *Canvas) dw() int { return c.W * 2 }
func (c *Canvas) dh() int { return c.H * 4 }

// project returns dot coordinates (may be outside the canvas).
func (c *Canvas) project(lat, lon float64) (float64, float64) {
	mid := (c.View.Lon0 + c.View.Lon1) / 2
	lon = Unwrap(lon, mid)
	x := (lon - c.View.Lon0) / (c.View.Lon1 - c.View.Lon0) * float64(c.dw())
	y := (c.View.Lat1 - lat) / (c.View.Lat1 - c.View.Lat0) * float64(c.dh())
	return x, y
}

// ProjectCell is the cell a coordinate falls in.
func (c *Canvas) ProjectCell(lat, lon float64) (int, int) {
	x, y := c.project(lat, lon)
	return int(math.Floor(x / 2)), int(math.Floor(y / 4))
}

func (c *Canvas) set(x, y int, l Layer) {
	if x < 0 || y < 0 || x >= c.dw() || y >= c.dh() {
		return
	}
	if i := y*c.dw() + x; c.dots[i] < l {
		c.dots[i] = l
	}
}

// DrawLand fills the land dots from the country raster.
func (c *Canvas) DrawLand() {
	w := data.WorldMap()
	dw, dh := c.dw(), c.dh()
	lonPerDot := (c.View.Lon1 - c.View.Lon0) / float64(dw)
	latPerDot := (c.View.Lat1 - c.View.Lat0) / float64(dh)
	for y := 0; y < dh; y++ {
		if c.DotMatrix && y%2 == 1 {
			continue
		}
		lat := c.View.Lat1 - (float64(y)+0.5)*latPerDot
		for x := 0; x < dw; x++ {
			if c.DotMatrix && x%2 == 1 {
				continue
			}
			lon := c.View.Lon0 + (float64(x)+0.5)*lonPerDot
			v := w.Value(lat, lon)
			if v == 0 {
				continue
			}
			l := Land
			if c.Countries != nil {
				l = c.Countries(w.Codes[v-1])
			}
			c.set(x, y, l)
		}
	}
}

// Line draws a straight line in dot space between two coordinates (the short way round).
func (c *Canvas) Line(lat1, lon1, lat2, lon2 float64, l Layer) {
	mid := (c.View.Lon0 + c.View.Lon1) / 2
	a := Unwrap(lon1, mid)
	b := Unwrap(lon2, a)
	x0, y0 := c.raw(lat1, a)
	x1, y1 := c.raw(lat2, b)
	steps := int(math.Max(math.Abs(x1-x0), math.Abs(y1-y0))) + 1
	if steps > 4000 {
		return
	}
	for i := 0; i <= steps; i++ {
		t := float64(i) / float64(steps)
		c.set(int(math.Floor(x0+(x1-x0)*t)), int(math.Floor(y0+(y1-y0)*t)), l)
	}
}

func (c *Canvas) raw(lat, lon float64) (float64, float64) {
	x := (lon - c.View.Lon0) / (c.View.Lon1 - c.View.Lon0) * float64(c.dw())
	y := (c.View.Lat1 - lat) / (c.View.Lat1 - c.View.Lat0) * float64(c.dh())
	return x, y
}

// Path draws a polyline of (lat, lon) points.
func (c *Canvas) Path(pts [][2]float64, l Layer) {
	for i := 1; i < len(pts); i++ {
		c.Line(pts[i-1][0], pts[i-1][1], pts[i][0], pts[i][1], l)
	}
}

// Dot marks one coordinate (a 2×2 block of dots so it stands out).
func (c *Canvas) Dot(lat, lon float64, l Layer) {
	x, y := c.project(lat, lon)
	xi, yi := int(math.Floor(x)), int(math.Floor(y))
	for dx := 0; dx < 2; dx++ {
		for dy := 0; dy < 2; dy++ {
			c.set(xi+dx-0, yi+dy-0, l)
		}
	}
}

// Label writes text starting at the cell of a coordinate (shifted to stay on screen).
func (c *Canvas) Label(lat, lon float64, text string, l Layer, dx int) {
	cx, cy := c.ProjectCell(lat, lon)
	cx += dx
	n := len([]rune(text))
	if cx+n > c.W {
		cx = c.W - n
	}
	if cx < 0 {
		cx = 0
	}
	if cy < 0 || cy >= c.H {
		return
	}
	for i, r := range []rune(text) {
		c.labels[cy*c.W+cx+i] = label{string(r), l}
	}
}

// Render turns the canvas into coloured lines of text.
func (c *Canvas) Render(styles map[Layer]lipgloss.Style) string {
	dw := c.dw()
	var b strings.Builder
	bits := [4][2]rune{{0x01, 0x08}, {0x02, 0x10}, {0x04, 0x20}, {0x40, 0x80}}
	for cy := 0; cy < c.H; cy++ {
		var run strings.Builder
		runLayer := Layer(255)
		flush := func() {
			if run.Len() == 0 {
				return
			}
			if st, ok := styles[runLayer]; ok {
				b.WriteString(st.Render(run.String()))
			} else {
				b.WriteString(run.String())
			}
			run.Reset()
		}
		for cx := 0; cx < c.W; cx++ {
			var ch string
			var top Layer
			if lb, ok := c.labels[cy*c.W+cx]; ok {
				ch, top = lb.text, lb.layer
			} else {
				var r rune
				for dy := 0; dy < 4; dy++ {
					for dx := 0; dx < 2; dx++ {
						l := c.dots[(cy*4+dy)*dw+cx*2+dx]
						if l == None {
							continue
						}
						if l > top {
							top = l
						}
					}
				}
				// Only draw the dots of the top layer (and land under it for context): keeps routes crisp.
				for dy := 0; dy < 4; dy++ {
					for dx := 0; dx < 2; dx++ {
						l := c.dots[(cy*4+dy)*dw+cx*2+dx]
						if l != None && (l == top || top <= Visited) {
							r |= bits[dy][dx]
						}
					}
				}
				if r == 0 {
					ch = " "
				} else {
					ch = string(0x2800 + r)
				}
			}
			if top != runLayer {
				flush()
				runLayer = top
			}
			run.WriteString(ch)
		}
		flush()
		if cy < c.H-1 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}
