package tui

import (
	"github.com/zigdon/rsp/models"
	"github.com/zigdon/rsp/rest"

	tea "charm.land/bubbletea/v2"
	lg "charm.land/lipgloss/v2"
)

var (
	r *models.Replicant
)

func loadReplicant(id string) {
	var err error
	r, err = rest.Replicant(id)
	if err != nil {
		log("Error loading replicant %q: %v", id, err)
		return
	}
}

type InvItem struct {
	Qty int
	Type string
}

type replicantData struct {
	R *models.Replicant
	ScanData *models.Scan
}

func replicantView(m *Model) *lg.Layer {
	opts := []menuOption{
		{
			Text: "Travel",
			Action: func(m *Model) (*Model, tea.Cmd) {
				m.Prompt("Enter destination:", 50, 10, []string{}, func(m *Model, dest string) {
					trip, err := rest.Travel(r.ReplicantCode, dest)
					if err != nil {
						m.Log("Travel failed: %v", err)
						return
					}
					m.Log("Travel initiated: %v", trip)
					loadReplicant(r.ReplicantCode)
				})
				return m, nil
			},
		},
		{
			Text: "Scan",
			Action: func(m *Model) (*Model, tea.Cmd) {
				m.Screens[scanOutput].Load(r.ReplicantCode)
				return m, nil
			},
			NextScreen: scanOutput,
		},
		{
			Text: "Close",
			Action: func(m *Model) (*Model, tea.Cmd) {
				m.Screens[replicantMenu].Visible = false
				m.Focus = mainMenu
				return m, nil
			},
		},
	}
	m.Screens[replicantMenu].Options = opts
	header := m.executeTmpl("replicant", replicantData{
		R: r,
	})
	return lg.NewLayer(m.executeTmpl("menu", menuData{
		Header: header,
		Options: opts,
		Cursor: m.Screens[replicantMenu].Cursor,
	}))
}

func newReplicantScreen() *Screen {
	return &Screen{
		GetSize: func(*Model) int { return 3 },
		Load: loadReplicant,
		Render: replicantView,
	}
}
