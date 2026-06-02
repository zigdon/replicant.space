package tui

import (
	lg "charm.land/lipgloss/v2"
)

func box(style *lg.Style, w, h int, content string) string {
	if style == nil {
		s := lg.NewStyle()
		style = &s
	}
	return style.
		Border(lg.NormalBorder()).
		Height(h).
		Width(w).
		Render(content)
}
