package tui

import (
	"github.com/zigdon/rsp/models"
	"github.com/zigdon/rsp/rest"

	tea "charm.land/bubbletea/v2"
	lg "charm.land/lipgloss/v2"
)

var output *models.Scan

func loadScanOutput(rc string) {
	var err error
	output, err = rest.ReplicantScan(rc)
	if err != nil {
		log("Error scanning system with %q: %v", rc, err)
		return
	}
}

func scanOutputView(m *Model) *lg.Layer {
	opts := []menuOption{
		{
			Text: "Close",
			Action: func(m *Model) (*Model, tea.Cmd) {
				m.Screens[scanOutput].Visible = false
				m.Focus = replicantMenu
				return m, nil
			},
		},
	}
	m.Screens[scanOutput].Options = opts
	header := m.executeTmpl("scan", output)
	return lg.NewLayer(m.executeTmpl("menu", menuData{
		Header: header,
		Options: opts,
		Cursor: m.Screens[scanOutput].Cursor,
	}))
}

func newScanOutput() *Screen {
	return &Screen{
		Load: loadScanOutput,
		Render: scanOutputView,
	}
}

