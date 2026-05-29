package tui

import (
	"fmt"
	"os"
	"slices"

	"github.com/zigdon/rsp/models"
	"github.com/zigdon/rsp/rest"

	tea "charm.land/bubbletea/v2"
	lg "charm.land/lipgloss/v2"
)

func log(tmpl string, args ...any) {
	fmt.Fprintf(os.Stderr, tmpl, args...)
}

func die(tmpl string, args ...any) {
	log("FATAL: " + tmpl, args...)
	os.Exit(1)
}

type screenID int
const (
	mainMenu screenID = iota
)

type Screen struct {
	Visible bool
	Cursor int
	Size int
}

type Model struct {
	// UI ELEMENTS
	// Which screen has focus
	Focus screenID
	// Screen state
	Screens map[screenID]*Screen

	// GAME STATE
	// Current account info
	Account *models.Me
	// Map of replicant ID to a recent scan
	Scans map[string]*models.Scan
}

func (m *Model) render(id screenID) *lg.Layer {
	return map[screenID]func()*lg.Layer {
		mainMenu: m.mainView,
	}[id]()
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
		case tea.KeyPressMsg:
			switch msg.String() {

			// These keys should exit the program.
			case "ctrl+c", "q":
				return m, tea.Quit

			case "j", "down":
				f := m.Focus
				m.Screens[f].Cursor++
				if m.Screens[f].Cursor >= m.Screens[f].Size {
					m.Screens[f].Cursor = 0
				}
				return m, nil
			case "k", "up":
				f := m.Focus
				m.Screens[f].Cursor--
				if m.Screens[f].Cursor < 0 {
					m.Screens[f].Cursor = m.Screens[f].Size-1
				}
				return m, nil
		}
	}
	
	return m, nil
}

func (m *Model) View() tea.View {
	var visible []screenID
	for id, s := range m.Screens {
		if !s.Visible { continue }
		visible = append(visible, id)
	}
	slices.Sort(visible)

	var layers []*lg.Layer
	for z, v := range visible {
		layers = append(layers, m.render(v).Z(z))
	}

	return tea.NewView(lg.NewCompositor(layers...).Render())
}

func (m *Model) Init() tea.Cmd {
	return nil
}

func (m *Model) updateData() error {
	me, err := rest.Me()
	if err != nil {
		return fmt.Errorf("can't get account info: %v", err)
	}
	m.Account = me

	m.Screens[mainMenu].Size = len(m.Account.Replicants)
	for _, r := range m.Account.Replicants {
		rs, err := rest.ReplicantScan(r.ReplicantCode)
		if err != nil {
			log("error getting scan for %s: %v", r.ReplicantCode, err)
			continue
		}
		m.Scans[r.ReplicantCode] = rs
	}

	return nil
}

func Init() *Model {
	m := &Model{
		Screens: map[screenID]*Screen{
			mainMenu: &Screen{
				Visible: true,
			},
		},
		Scans: make(map[string]*models.Scan),
	}
	if err := m.updateData(); err != nil {
		die("Failed to initialize model: %v", err)
	}

	return m
}
