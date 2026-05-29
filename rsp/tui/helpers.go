package tui

import (
	"embed"
	"strings"

	"text/template"

	"charm.land/lipgloss/v2"
)

//go:embed templates/*.tmpl
var templates embed.FS

func (m *Model) executeTmpl(name string, data any) *lipgloss.Layer {
	tmpl := t(name)
	var s strings.Builder
	if err := tmpl.Execute(&s, data); err != nil {
		log("Error executing %q: %v", name, err)
	}
	return lipgloss.NewLayer(s.String())
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
