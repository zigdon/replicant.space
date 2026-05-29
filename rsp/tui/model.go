package tui

import (
	"fmt"
	"os"
	"slices"
	"strconv"

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
	none screenID = iota
	mainMenu
	replicantMenu
)

type Screen struct {
	Visible bool
	Cursor int
	Size int

	GetSize func(*Model) int
	Load func(string) error
}

type Model struct {
	// UI ELEMENTS
	// Which screen has focus
	Focus screenID
	// Screen state
	Screens map[screenID]*Screen
	// Terminal size
	Width, Height int

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
			case "enter":
				log("Selected: %d.%d\n\n", m.Focus, m.Screens[m.Focus].Cursor)
				return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
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

	layers := []*lg.Layer{
		lg.NewLayer(background(m.Width, m.Height)).Z(-1),
	}
	for i, v := range visible {
		layers = append(layers, m.render(v).X(3+i*3).Y(2+i*2).Z(i))
	}

	view := tea.NewView(lg.NewCompositor(layers...).Render())
	view.WindowTitle = "replicant.space"
	view.AltScreen = true
	return view
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

	for id, s := range m.Screens {
		m.Screens[id].Size = s.GetSize(m)
	}

	return nil
}

func Init() *Model {
	w, _ := strconv.Atoi(os.Getenv("COLUMNS"))
	h, _ := strconv.Atoi(os.Getenv("LINES"))
	if w == 0 { w = 80 }
	if h == 0 { h = 40 }
	m := &Model{
		Focus: mainMenu,
		Screens: map[screenID]*Screen{
			mainMenu: newMainScreen(),
			replicantMenu: newReplicantScreen(),
		},
		Scans: make(map[string]*models.Scan),
		Width: w,
		Height: h,
	}
	if err := m.updateData(); err != nil {
		die("Failed to initialize model: %v", err)
	}

	return m
}
