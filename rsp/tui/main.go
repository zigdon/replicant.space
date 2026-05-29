package tui

import (
	"github.com/zigdon/rsp/models"

	"charm.land/lipgloss/v2"
)

type mainData struct {
	Account *models.Me
	Cursor int
}

func (m *Model) mainView() *lipgloss.Layer {
	return m.executeTmpl("main", mainData{
		Account: m.Account,
		Cursor: m.ScreensCursor[mainMenu],
	})
}
