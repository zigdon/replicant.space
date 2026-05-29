package tui

import (
	"fmt"
	"os"
	"slices"

	"github.com/zigdon/rsp/models"
	"github.com/zigdon/rsp/rest"

	tea "charm.land/bubbletea/v2"
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
	screensVisible map[screenID]bool
	screensCursor map[screenID]int
	// Current account info
	account *models.Me
	// Map of replicant ID to a recent scan
	scans map[string]*models.Scan
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return nil, nil
}

func (m *Model) View() tea.View {
	var visible []screenID
	for id, v := range m.screensVisible {
		if !v { continue }
		visible = append(visible, id)
	}
	slices.Sort(visible)

	for range visible {
		// call the windows view method
	}

	return tea.NewView("")
}

func (m *Model) Init() tea.Cmd {
	return nil
}

func (m *Model) updateData() error {
	me, err := rest.Me()
	if err != nil {
		return fmt.Errorf("can't get account info: %v", err)
	}
	m.account = me

	for _, r := range m.account.Replicants {
		rs, err := rest.ReplicantScan(r.ReplicantCode)
		if err != nil {
			log("error getting scan for %s: %v", r.ReplicantCode, err)
			continue
		}
		m.scans[r.ReplicantCode] = rs
	}

	return nil
}

func Init() *Model {
	m := &Model{
		screensVisible: map[screenID]bool{
			mainMenu: true,
		},
		screensCursor: make(map[screenID]int),
		scans: make(map[string]*models.Scan),
	}
	if err := m.updateData(); err != nil {
		die("Failed to initialize model: %v", err)
	}

	return m
}
