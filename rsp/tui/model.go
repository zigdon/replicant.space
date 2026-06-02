package tui

import (
	"github.com/zigdon/rsp/models"

	fb "github.com/76creates/stickers/flexbox"
	tea "charm.land/bubbletea/v2"
	// "charm.land/bubbles/v2/textinput"
	// lg "charm.land/lipgloss/v2"
)

type Model struct {
	// Hold the current account info
	account *models.Account

	// Flexbox describing the UI
	flex *fb.FlexBox

	// Cell contents
	topBar, leftBar, centerPane, rightBar, bottomBar string

	// Indicator what value is selected in each menu
	selected map[string]string
}

func InitModel() *Model {
	m := &Model{
		selected: make(map[string]string),
		flex: fb.New(0, 0),
		topBar: "I am top",
		leftBar: "I am left",
		centerPane: "I am center",
		rightBar: "I am right",
		bottomBar: "I am bottom",
	}

	m.flex.AddRows([]*fb.Row{
		// Top bar
		m.flex.NewRow().AddCells(
			fb.NewCell(10, 1).SetContentGenerator(func(int, int) string {
				return m.topBar
			}),
		),
		// Content area
		m.flex.NewRow().AddCells(
			// Left menu
			fb.NewCell(2, 5).SetContentGenerator(func(int, int) string {
				return m.leftBar
			}),
			// Center window
			fb.NewCell(5, 5).SetContentGenerator(func(int, int) string {
				return m.centerPane
			}),
			// Right bar
			fb.NewCell(3, 5).SetContentGenerator(func(int, int) string {
				return m.rightBar
			}),
		),
		// bottom bar
		m.flex.NewRow().AddCells(
			fb.NewCell(10, 1).SetContentGenerator(func(int, int) string {
				return m.bottomBar
			}),
		),
	})

	return m
}

func (m *Model) Init() tea.Cmd {
	return nil
}

func (m *Model) getBoxSize(box string) (int, int) {
	var cell *fb.Cell
	switch box {
	case "top":
		cell = m.flex.GetRowCellCopy(0, 0)
	case "left":
		cell = m.flex.GetRowCellCopy(0, 1)
	case "center":
		cell = m.flex.GetRowCellCopy(1, 1)
	case "right":
		cell = m.flex.GetRowCellCopy(2, 1)
	case "bottom":
		cell = m.flex.GetRowCellCopy(0, 2)
	}
	return cell.GetWidth(), cell.GetHeight()
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.flex.SetWidth(msg.Width)
		m.flex.SetHeight(msg.Height)
		w, h := m.getBoxSize("top")
		m.topBar = box(nil, w, h, "loaded")
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		}

	}
	return m, nil
}
func (m *Model) View() tea.View {
	return tea.NewView(m.flex.Render())
}
