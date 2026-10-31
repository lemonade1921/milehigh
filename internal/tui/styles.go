package tui

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/lemonade1921/milehigh/internal/worldmap"
)

// Colours: a night-sky palette with amber accents, adapted for light terminals.
var (
	cAccent  = lipgloss.AdaptiveColor{Light: "#B76E00", Dark: "#FFB547"}
	cSky     = lipgloss.AdaptiveColor{Light: "#1E6FD9", Dark: "#6CB6FF"}
	cGood    = lipgloss.AdaptiveColor{Light: "#127A45", Dark: "#4ADE80"}
	cBad     = lipgloss.AdaptiveColor{Light: "#C62828", Dark: "#FF6B6B"}
	cWarn    = lipgloss.AdaptiveColor{Light: "#A15C00", Dark: "#FBBF24"}
	cText    = lipgloss.AdaptiveColor{Light: "#111827", Dark: "#E5E7EB"}
	cDim     = lipgloss.AdaptiveColor{Light: "#6B7280", Dark: "#8B95A7"}
	cFaint   = lipgloss.AdaptiveColor{Light: "#C9CED6", Dark: "#3A4252"}
	cLand    = lipgloss.AdaptiveColor{Light: "#B8C0CC", Dark: "#39465A"}
	cVisited = lipgloss.AdaptiveColor{Light: "#2A9D8F", Dark: "#2DD4BF"}
	cPlanned = lipgloss.AdaptiveColor{Light: "#7C83DB", Dark: "#818CF8"}
	cSelBg   = lipgloss.AdaptiveColor{Light: "#E8EEF8", Dark: "#1C2433"}
)

var (
	sText    = lipgloss.NewStyle().Foreground(cText)
	sDim     = lipgloss.NewStyle().Foreground(cDim)
	sFaint   = lipgloss.NewStyle().Foreground(cFaint)
	sAccent  = lipgloss.NewStyle().Foreground(cAccent)
	sBold    = lipgloss.NewStyle().Bold(true).Foreground(cText)
	sSky     = lipgloss.NewStyle().Foreground(cSky)
	sGood    = lipgloss.NewStyle().Foreground(cGood)
	sBad     = lipgloss.NewStyle().Foreground(cBad)
	sWarn    = lipgloss.NewStyle().Foreground(cWarn)
	sTitle   = lipgloss.NewStyle().Bold(true).Foreground(cAccent)
	sSection = lipgloss.NewStyle().Bold(true).Foreground(cDim)
	sBig     = lipgloss.NewStyle().Bold(true).Foreground(cText)

	sTab       = lipgloss.NewStyle().Foreground(cDim).Padding(0, 1)
	sTabActive = lipgloss.NewStyle().Foreground(cAccent).Bold(true).Padding(0, 1).Underline(true)

	sPanel = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(cFaint).Padding(0, 1)
	sModal = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(cAccent).Padding(1, 2)
	sKey   = lipgloss.NewStyle().Foreground(cAccent).Bold(true)

	sPillGood = lipgloss.NewStyle().Foreground(lipgloss.Color("#06281A")).Background(cGood).Bold(true).Padding(0, 1)
	sPillBad  = lipgloss.NewStyle().Foreground(lipgloss.Color("#2B0707")).Background(cBad).Bold(true).Padding(0, 1)
	sPillSky  = lipgloss.NewStyle().Foreground(lipgloss.Color("#071A2E")).Background(cSky).Bold(true).Padding(0, 1)
	sPillDim  = lipgloss.NewStyle().Foreground(cText).Background(cFaint).Padding(0, 1)
	sPillAmb  = lipgloss.NewStyle().Foreground(lipgloss.Color("#2B1A00")).Background(cAccent).Bold(true).Padding(0, 1)
)

var mapStyles = map[worldmap.Layer]lipgloss.Style{
	worldmap.Land:           lipgloss.NewStyle().Foreground(cLand),
	worldmap.Planned:        lipgloss.NewStyle().Foreground(cPlanned),
	worldmap.Visited:        lipgloss.NewStyle().Foreground(cVisited),
	worldmap.Route:          lipgloss.NewStyle().Foreground(cSky),
	worldmap.RouteHighlight: lipgloss.NewStyle().Foreground(cAccent),
	worldmap.Airport:        lipgloss.NewStyle().Foreground(cText).Bold(true),
	worldmap.Plane:          lipgloss.NewStyle().Foreground(cAccent).Bold(true),
}
