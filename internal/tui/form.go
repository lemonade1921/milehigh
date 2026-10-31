package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// field is one line of a form.
type field struct {
	key, label string
	input      textinput.Model
	hint       func(string) string // shown dimmed next to the field
	choices    []string            // when set, ←/→ cycle through these instead of typing
}

// form is a simple vertical form in a modal.
type form struct {
	title  string
	fields []*field
	focus  int
	err    string
	note   string
	width  int
}

func newField(key, label, value, placeholder string, width int) *field {
	in := textinput.New()
	in.Prompt = ""
	in.Placeholder = placeholder
	in.SetValue(value)
	in.CharLimit = 200
	in.Width = width
	in.PlaceholderStyle = sFaint
	in.TextStyle = sText
	in.Cursor.Style = sAccent
	return &field{key: key, label: label, input: in}
}

func (f *form) get(key string) string {
	for _, fl := range f.fields {
		if fl.key == key {
			return strings.TrimSpace(fl.input.Value())
		}
	}
	return ""
}

func (f *form) set(key, v string) {
	for _, fl := range f.fields {
		if fl.key == key {
			fl.input.SetValue(v)
			fl.input.CursorEnd()
		}
	}
}

func (f *form) current() *field { return f.fields[f.focus] }

func (f *form) setFocus(i int) {
	if i < 0 {
		i = len(f.fields) - 1
	}
	if i >= len(f.fields) {
		i = 0
	}
	for j, fl := range f.fields {
		if j == i {
			fl.input.Focus()
		} else {
			fl.input.Blur()
		}
	}
	f.focus = i
}

// update handles navigation and typing. moved reports which field lost focus (or "").
func (f *form) update(msg tea.KeyMsg) (cmd tea.Cmd, left string) {
	cur := f.current()
	switch msg.String() {
	case "tab", "down":
		f.setFocus(f.focus + 1)
		return nil, cur.key
	case "shift+tab", "up":
		f.setFocus(f.focus - 1)
		return nil, cur.key
	case "left", "right":
		if len(cur.choices) > 0 {
			v := cur.input.Value()
			i := 0
			for j, c := range cur.choices {
				if c == v {
					i = j
				}
			}
			if msg.String() == "right" {
				i = (i + 1) % len(cur.choices)
			} else {
				i = (i + len(cur.choices) - 1) % len(cur.choices)
			}
			cur.input.SetValue(cur.choices[i])
			return nil, ""
		}
	}
	if len(cur.choices) > 0 && msg.Type == tea.KeyRunes {
		return nil, ""
	}
	var c tea.Cmd
	cur.input, c = cur.input.Update(msg)
	return c, ""
}

func (f *form) view() string {
	lw := 0
	for _, fl := range f.fields {
		if w := lipgloss.Width(fl.label); w > lw {
			lw = w
		}
	}
	var b strings.Builder
	b.WriteString(sTitle.Render(f.title) + "\n\n")
	for i, fl := range f.fields {
		lbl := padLeft(fl.label, lw)
		if i == f.focus {
			lbl = sAccent.Render(lbl)
		} else {
			lbl = sDim.Render(lbl)
		}
		val := fl.input.View()
		if len(fl.choices) > 0 {
			v := fl.input.Value()
			if i == f.focus {
				val = sAccent.Render("‹ ") + sBold.Render(v) + sAccent.Render(" ›")
			} else {
				val = "  " + sText.Render(v)
			}
		}
		line := lbl + "  " + val
		if fl.hint != nil {
			if h := fl.hint(fl.input.Value()); h != "" {
				line += "  " + sDim.Render(truncate(h, 40))
			}
		}
		b.WriteString(line + "\n")
	}
	if f.note != "" {
		b.WriteString("\n" + sDim.Render(f.note))
	}
	if f.err != "" {
		b.WriteString("\n" + sBad.Render(f.err))
	}
	b.WriteString("\n" + sDim.Render("tab/↑↓ move · enter save · esc cancel"))
	return b.String()
}
