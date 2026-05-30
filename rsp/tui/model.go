package tui

import (
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/zigdon/rsp/models"
	"github.com/zigdon/rsp/rest"

	tea "charm.land/bubbletea/v2"
	"charm.land/bubbles/v2/textinput"
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
	Options []menuOption
	Size int

	GetSize func(*Model) int
	Load func(string)
	Render func(*Model) *lg.Layer
}

type Model struct {
	// UI ELEMENTS
	// Which screen has focus
	Focus screenID
	// Screen state
	Screens map[screenID]*Screen
	// Terminal size
	Width, Height int
	// General output
	Messages []string

	// Modal dialog
	modalTextInput textinput.Model
	modalEnabled bool
	modalCallback func(*Model, string)
	modalWidth, modalHeight int
	modalPrompt string

	// GAME STATE
	// Current account info
	Account *models.Me
	// Map of replicant ID to a recent scan
	Scans map[string]*models.Scan

}

func (m *Model) Log(tmpl string, args ...any) {
	m.Messages = append(m.Messages, fmt.Sprintf(tmpl, args...))
	if len(m.Messages) > 5 {
		m.Messages = m.Messages[len(m.Messages)-5:]
	}
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		if m.modalEnabled {
			// All input goes to the dialog.
			m.modalTextInput.Focus()
			switch msg.String() {
			case "ctrl+c":
				return m, tea.Quit
			case "esc":
				m.modalEnabled = false
				m.modalTextInput.Reset()
				return m, nil
			case "enter":
				m.modalEnabled = false
				m.modalCallback(m, m.modalTextInput.Value())
				m.modalTextInput.Reset()
				return m, nil
			}
			// While the dialog is empty, all other keystrokes should go to it.
			var cmd tea.Cmd
			m.modalTextInput, cmd = m.modalTextInput.Update(msg)
			return m, cmd
		} else {
			// The normal UI is enabled.
			f := m.Focus
			switch msg.String() {

			// These keys should exit the program.
			case "ctrl+c":
				return m, tea.Quit

			case "j", "down":
				m.Screens[f].Cursor++
				if m.Screens[f].Cursor >= m.Screens[f].Size {
					m.Screens[f].Cursor = 0
				}
				return m, nil
			case "k", "up":
				m.Screens[f].Cursor--
				if m.Screens[f].Cursor < 0 {
					m.Screens[f].Cursor = m.Screens[f].Size-1
				}
				return m, nil
			case "enter":
				cur := m.Screens[f].Cursor
				opt := m.Screens[f].Options[cur]
				if act := opt.Action; act != nil {
					act(m)
				}
				if next := opt.NextScreen; next != none {
					m.Focus = next
					m.Screens[next].Visible = true
				}
				return m, nil
		}
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
		lg.NewLayer(background(m.Width, m.Height)).Z(-10),
		lg.NewLayer(box(logStyle, m.Width-10, 5, "%s", strings.Join(m.Messages, "\n"))).
		  X(5).Y(m.Height-7).Z(-5),
	}
	for i, v := range visible {
		layers = append(layers, m.Screens[v].Render(m).X(3+i*3).Y(2+i*2).Z(i))
	}

	if m.modalEnabled {
		layers = append(layers, lg.NewLayer(
			box(modalStyle, m.modalWidth, m.modalHeight,
			    "%s\n%s", m.modalPrompt, m.modalTextInput.View())).
			X((m.Width-m.modalWidth)/2).
			Y((m.Height-m.modalHeight)/2).
			Z(100),
		)
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

func (m *Model) Prompt(text string, width, height int, suggestions []string, callback func(*Model, string)) {
	m.modalEnabled = true
	m.modalCallback = callback
	m.modalHeight = height
	m.modalWidth = width
	m.modalPrompt = text
	mti := textinput.New()
	if len(suggestions) > 0 {
		mti.SetSuggestions(suggestions)
		mti.ShowSuggestions = true
	}
	m.modalTextInput = mti
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
