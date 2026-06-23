package tui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/zigdon/rsp/models"
)

func ModelGrid(g *tview.Grid, mg *models.Grid) *tview.Grid {
	g.Clear()
	for _, i := range mg.Items {
		// if i.Text == "" { continue }
		content := tview.NewTextView().SetText(i.Text)
		frame := tview.NewFrame(content)
		if i.Title != "" {
			frame.AddText("  " + i.Title, true, tview.AlignLeft, tcell.ColorDefault)
		}
		w := i.W
		h := i.H
		if w == 0 { w = 1 }
		if h == 0 { h = 1 }

		g.AddItem(frame, i.Y, i.X, h, w, i.MH, i.MW, false)
	}
	return g
}
