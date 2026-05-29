package tui

import (
	"fmt"
	"os"
	"slices"

	"github.com/zigdon/rsp/models"
	"github.com/zigdon/rsp/rest"

	tea "charm.land/bubbletea/v2"
	gloss "charm.land/lipgloss/v2"
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

type Model struct {
	ScreensVisible map[screenID]bool
	ScreensCursor map[screenID]int
	// Current account info
	Account *models.Me
	// Map of replicant ID to a recent scan
	Scans map[string]*models.Scan
}

func (m *Model) render(id screenID) *gloss.Layer {
	return map[screenID]func()*gloss.Layer {
		mainMenu: m.mainView,
	}[id]()
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

		// Is it a key press?
		case tea.KeyPressMsg:

			// Cool, what was the actual key pressed?
			switch msg.String() {

			// These keys should exit the program.
			case "ctrl+c", "q":
				return m, tea.Quit

			}
	}
	
	return m, nil
}

func (m *Model) View() tea.View {
	var visible []screenID
	for id, v := range m.ScreensVisible {
		if !v { continue }
		visible = append(visible, id)
	}
	slices.Sort(visible)

	var layers []*gloss.Layer
	for z, v := range visible {
		layers = append(layers, m.render(v).Z(z))
	}

	return tea.NewView(gloss.NewCompositor(layers...).Render())
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
		ScreensVisible: map[screenID]bool{
			mainMenu: true,
		},
		ScreensCursor: make(map[screenID]int),
		Scans: make(map[string]*models.Scan),
	}
	if err := m.updateData(); err != nil {
		die("Failed to initialize model: %v", err)
	}

	return m
}
