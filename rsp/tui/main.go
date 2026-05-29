package tui

import (
	"charm.land/lipgloss/v2"
)

func (m *Model) mainView() *lipgloss.Layer {
	return lipgloss.NewLayer("main window")
}
