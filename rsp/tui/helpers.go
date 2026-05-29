package tui

import (
	"embed"
	"fmt"
	"strings"

	"text/template"

	lg "charm.land/lipgloss/v2"
)

type menuData struct {
	Title string
	Header string
	Options []string
	Cursor int
}

//go:embed templates/*.tmpl
var templates embed.FS

func (m *Model) executeTmpl(name string, data any) *lg.Layer {
	tmpl := t(name)
	var s strings.Builder
	if err := tmpl.Execute(&s, data); err != nil {
		log("Error executing %q: %v", name, err)
	}
	return lg.NewLayer(screen(s.String()))
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

type boxStyle int
const (
	titleStyle boxStyle = iota
	headerStyle
)

func box(style boxStyle, tmpl string, args ...any) string {
	st := lg.NewStyle().
		Border(lg.RoundedBorder()).
		PaddingLeft(3).
		PaddingRight(3).
		Width(40)
	if style == titleStyle || style == headerStyle {
		st = st.Align(lg.Center)
	}
	return st.Render(fmt.Sprintf(tmpl, args...))
}

func screen(contents string) string {
	return lg.NewStyle().
		Border(lg.ThickBorder()).
		Padding(0, 1, 2, 2).
		Render(contents)
}
