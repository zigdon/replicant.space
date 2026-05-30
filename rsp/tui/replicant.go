package tui

import (
	"github.com/zigdon/rsp/models"
	"github.com/zigdon/rsp/rest"

	lg "charm.land/lipgloss/v2"
)

var (
	r *models.Replicant
	s *models.Scan
)

func loadReplicant(id string) {
	rep, err := rest.Replicant(id)
	if err != nil {
		log("Error loading replicant %q: %v", id, err)
		return
	}

	if r == nil || rep.Location != r.Location {
		s, err = rest.ReplicantScan(rep.ReplicantCode)
		if err != nil {
			log("Error scanning replicant system %q: %v", rep.Location, err)
		}
	}
	r = rep
}

type replicantData struct {
	R *models.Replicant
	ScanData *models.Scan
}

func replicantView(m *Model) *lg.Layer{
	opts := []menuOption{
		{
			Text: "Travel",
			Action: func(m *Model) {
			m.Prompt("Enter destination:", 50, 10, s.ExtractLocations(), func(m *Model, dest string) {
				trip, err := rest.Travel(r.ReplicantCode, dest)
				if err != nil {
					m.Log("Travel failed: %v", err)
					return
				}
				m.Log("Travel initiated: %v", trip)
				loadReplicant(r.ReplicantCode)
				})
			},
		},
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
		GetSize: func(*Model) int { return 2 },
		Load: loadReplicant,
		Render: replicantView,
	}
}
