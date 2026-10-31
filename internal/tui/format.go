package tui

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/lemonade1921/milehigh/internal/data"
	"github.com/lemonade1921/milehigh/internal/model"
	"github.com/lemonade1921/milehigh/internal/stats"
)

func (m *Model) clock(t time.Time) string {
	if m.settings.TimeFormat == "H12" {
		return t.Format("3:04 PM")
	}
	return t.Format("15:04")
}

func (m *Model) distance(km float64) string {
	if m.settings.Units == "MI" {
		return thousands(km/stats.KmPerMile) + " mi"
	}
	return thousands(km) + " km"
}

func thousands(v float64) string {
	n := int64(math.Round(v))
	s := fmt.Sprint(n)
	neg := strings.HasPrefix(s, "-")
	s = strings.TrimPrefix(s, "-")
	var out []byte
	for i := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, s[i])
	}
	if neg {
		return "-" + string(out)
	}
	return string(out)
}

func duration(d time.Duration) string {
	d = d.Round(time.Minute)
	h := int(d.Hours())
	mm := int(d.Minutes()) % 60
	switch {
	case h >= 48:
		return fmt.Sprintf("%dd %dh", h/24, h%24)
	case h > 0:
		return fmt.Sprintf("%dh %02dm", h, mm)
	default:
		return fmt.Sprintf("%dm", mm)
	}
}

func longDuration(d time.Duration) string {
	h := int(d.Hours())
	if h < 48 {
		return duration(d)
	}
	days := h / 24
	return fmt.Sprintf("%dd %dh", days, h%24)
}

// relative is "in 3h 12m", "2 days ago"…
func relative(t, now time.Time) string {
	d := t.Sub(now)
	future := d > 0
	if !future {
		d = -d
	}
	var s string
	switch {
	case d < time.Minute:
		return "now"
	case d < 48*time.Hour:
		s = duration(d)
	case d < 60*24*time.Hour:
		s = fmt.Sprintf("%d days", int(d.Hours()/24))
	case d < 730*24*time.Hour:
		s = fmt.Sprintf("%d months", int(d.Hours()/24/30))
	default:
		s = fmt.Sprintf("%d years", int(d.Hours()/24/365))
	}
	if future {
		return "in " + s
	}
	return s + " ago"
}

func delayText(d time.Duration) (string, lipgloss.Style) {
	min := int(math.Round(d.Minutes()))
	switch {
	case min <= -1:
		return fmt.Sprintf("%dm early", -min), sGood
	case min < 1:
		return "on time", sGood
	case min < 15:
		return fmt.Sprintf("%dm late", min), sWarn
	default:
		return duration(d) + " late", sBad
	}
}

func city(code string) string {
	if a := data.AirportByCode(code); a != nil {
		if a.City != "" {
			return a.City
		}
		return a.Name
	}
	return code
}

func airportName(code string) string {
	if a := data.AirportByCode(code); a != nil {
		return a.Name
	}
	return "Unknown airport"
}

func flightCode(f *model.Flight) string {
	if f.Airline == "" && f.Number == "" {
		return "—"
	}
	return strings.TrimSpace(f.Airline + " " + f.Number)
}

// local shows a time in an airport's zone.
func local(t time.Time, code string) time.Time {
	if a := data.AirportByCode(code); a != nil {
		return t.In(a.Location())
	}
	return t
}

func truncate(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= w {
		return s
	}
	r := []rune(s)
	for len(r) > 0 && lipgloss.Width(string(r))+1 > w {
		r = r[:len(r)-1]
	}
	return string(r) + "…"
}

func pad(s string, w int) string {
	if d := w - lipgloss.Width(s); d > 0 {
		return s + strings.Repeat(" ", d)
	}
	return s
}

func padLeft(s string, w int) string {
	if d := w - lipgloss.Width(s); d > 0 {
		return strings.Repeat(" ", d) + s
	}
	return s
}

func bar(frac float64, w int, st lipgloss.Style) string {
	if w <= 0 {
		return ""
	}
	full := frac * float64(w)
	n := int(full)
	parts := []string{"", "▏", "▎", "▍", "▌", "▋", "▊", "▉"}
	s := strings.Repeat("█", n)
	if n < w {
		s += parts[int((full-float64(n))*8)]
	}
	return st.Render(s)
}

// status is the pill for a flight's state.
func (m *Model) statusPill(f *model.Flight, now time.Time) string {
	switch {
	case f.Canceled:
		return sPillBad.Render("CANCELED")
	case f.DivertedTo != nil:
		return sPillBad.Render("DIVERTED")
	case f.InAir(now):
		return sPillSky.Render("IN THE AIR · lands " + relative(f.Arrival(), now))
	case f.HasLanded(now):
		if d, ok := f.ArrDelay(); ok {
			t, _ := delayText(d)
			if d.Minutes() >= 15 {
				return sPillBad.Render("LANDED · " + strings.ToUpper(t))
			}
			return sPillGood.Render("LANDED · " + strings.ToUpper(t))
		}
		return sPillDim.Render("LANDED " + strings.ToUpper(relative(f.Arrival(), now)))
	default:
		d := f.Departure().Sub(now)
		if d < 24*time.Hour {
			return sPillAmb.Render("DEPARTS " + strings.ToUpper(relative(f.Departure(), now)))
		}
		return sPillDim.Render("DEPARTS " + strings.ToUpper(relative(f.Departure(), now)))
	}
}

// airportCountry is the ISO country code of an airport ("" when unknown).
func airportCountry(code string) string {
	if a := data.AirportByCode(code); a != nil {
		return a.Country
	}
	return ""
}
