package tui

import (
	"fmt"

	lg "charm.land/lipgloss/v2"
)

func (m *Model) mainView() *lg.Layer {
	var opts []string
	for _, r := range m.Account.Replicants {
		opts = append(opts, fmt.Sprintf("%s (%s)", r.Name, r.CurrentLocation))
	}
	header := box(headerStyle, "XP: %d | Unread Messages: %d", m.Account.ExperiencePointsTotal, m.Account.UnreadMessageCount)
	title := box(titleStyle, "[[ %s ]]", m.Account.Name)
	return m.executeTmpl("menu", menuData{
		Title: title,
		Header: header,
		Options: opts,
		Cursor: m.Screens[mainMenu].Cursor,
	})
}
