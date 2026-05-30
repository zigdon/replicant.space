package tui

import (
	"embed"
	"fmt"
	"strings"

	"text/template"

	lg "charm.land/lipgloss/v2"
)

//// Templates

//go:embed templates/*.tmpl
var templates embed.FS

func (m *Model) executeTmpl(name string, data any) string {
	tmpl := t(name)
	var s strings.Builder
	if err := tmpl.Execute(&s, data); err != nil {
		log("Error executing %q: %v", name, err)
	}
	return screen(s.String())
}

func t(name string) *template.Template {
	data, err := templates.ReadFile("templates/"+name+".tmpl")
	if err != nil {
		die("Can't read template %q: %v", name, err)
	}
	tmpl, err := template.New(name).Parse(string(data))
	if err != nil {
		die("Can't parse template %q: %v", name, err)
	}
	return tmpl
}

//// UI Elements
type boxStyle int
const (
	titleStyle boxStyle = iota
	headerStyle
	logStyle
)

func box(style boxStyle, w, h int, tmpl string, args ...any) string {
	if w == 0 { w = 40 }
	st := lg.NewStyle().
		Border(lg.RoundedBorder()).
		PaddingLeft(3).
		PaddingRight(3).
		Width(w)
	if h != 0 { st = st.Height(h) }
	if style == titleStyle || style == headerStyle {
		st = st.Align(lg.Center)
	}
	if style == logStyle {
		st = st.Padding(0, 0, 2, 2)
	}
	return st.Render(fmt.Sprintf(tmpl, args...))
}

func screen(contents string) string {
	return lg.NewStyle().
		Border(lg.ThickBorder()).
		Padding(0, 1, 2, 2).
		Render(contents)
}

func background(w, h int) string {
	return lg.NewStyle().
		Width(w).
		Height(h).
		Align(lg.Center).
		Render("replicant.space")
}

var screenNotImplemented = &Screen{
	Cursor: 0,
	GetSize: func(*Model) int { return 1 },
	Render: func(*Model) *lg.Layer { return nil},
}

//// Menus
type menuOption struct {
	Text string
	Action func(*Model)
	NextScreen screenID
	Hotkey rune
	BreakAfter bool
}

type menuData struct {
	Title string
	Header string
	Footer string
	Options []menuOption
	Cursor int
}
