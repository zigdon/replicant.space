package tui

import (
	"github.com/zigdon/rsp/models"
	"github.com/zigdon/rsp/rest"

	lg "charm.land/lipgloss/v2"
)

var r *models.Replicant

func loadReplicant(id string) {
	rep, err := rest.Replicant(id)
	if err == nil {
		r = rep
	}
}

type replicantData struct {
	R *models.Replicant
	ScanData *models.Scan
}

func replicantView(m *Model) *lg.Layer{
	opts := []menuOption{
		{
			Text: "Close",
			Action: func(m *Model) {
				m.Screens[replicantMenu].Visible = false
				m.Focus = mainMenu
			},
		},
	}
	m.Screens[replicantMenu].Options = opts
	scan, err := rest.ReplicantScan(r.ReplicantCode)
	if err != nil {
		m.Log("Error scanning system from %s: %v", r.ReplicantCode, err)
	}
	header := m.executeTmpl("replicant", replicantData{
		R: r,
		ScanData: scan,
	})
	return lg.NewLayer(m.executeTmpl("menu", menuData{
		Header: header,
		Options: opts,
		Cursor: m.Screens[replicantMenu].Cursor,
	}))
}

func newReplicantScreen() *Screen {
	return &Screen{
		GetSize: func(*Model) int { return 1 },
		Load: loadReplicant,
		Render: replicantView,
	}
}
