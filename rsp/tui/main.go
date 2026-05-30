package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	lg "charm.land/lipgloss/v2"
)

func newMainScreen() *Screen {
	return &Screen{
		Visible: true,
		GetSize: func(m *Model) int {
			return len(m.Account.Replicants) + 2
		},
		Render: mainView,
	}
}

func mainView(m *Model) *lg.Layer {
	var opts []menuOption
	for n, r := range m.Account.Replicants {
		opts = append(opts, menuOption{
			Text: fmt.Sprintf("%s (%s)", r.Name, r.CurrentLocation),
			Action: func(m *Model) (*Model, tea.Cmd) {
				m.Screens[replicantMenu].Load(r.ReplicantCode)
				return m, nil
			},
			NextScreen: replicantMenu,
			BreakAfter: n == len(m.Account.Replicants)-1,
		})
	}
	opts = append(opts, menuOption{
		Text: "Messages",
	})
	opts = append(opts, menuOption{
		Text: "Quit",
		Action: func(m *Model) (*Model, tea.Cmd) {
			return m, tea.Quit
		},
	})
	m.Screens[mainMenu].Options = opts

	header := box(headerStyle, 0, 0, "XP: %d | Unread Messages: %d", m.Account.ExperiencePointsTotal, m.Account.UnreadMessageCount)
	title := box(titleStyle, 0, 0, "[[ %s ]]", m.Account.Name)
	return lg.NewLayer(m.executeTmpl("menu", menuData{
		Title: title,
		Header: header,
		Footer: "Arrows/Enter to select, ctrl-c to quit",
		Options: opts,
		Cursor: m.Screens[mainMenu].Cursor,
	}))
}
