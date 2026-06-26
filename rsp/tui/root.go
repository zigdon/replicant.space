package tui

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/spf13/cobra"

	"github.com/zigdon/rsp/rest"
)

var devList = tview.NewList()
var tree = tview.NewTreeView().SetRoot(tview.NewTreeNode("Details"))
var dump = tview.NewTextView()
var logWin = tview.NewTextView()
var app *tview.Application

var TUI = &cobra.Command{
	Use: "tui",
	Short: "Launch a TUI interface",
	RunE: func(cmd *cobra.Command, args []string) error {
		app = tview.NewApplication()
		go processEventQueue()
		Repeat("replicant page", 1 * time.Second, replPage, forever)
		app.SetRoot(layout(), true)
		setKeys()
		return app.Run()
	},
}

func log(tmpl string, args ...any) {
	now := time.Now().Format(time.Stamp)
	args = append([]any{now}, args...)
	if app.GetFocus() != nil {
		fmt.Fprintf(logWin, "%s: " + tmpl + "\n", args...)
		logWin.ScrollToEnd()
	} else {
		fmt.Printf("%s: " + tmpl + "\n", args...)
	}
}

func setKeys() {
	app.SetInputCapture(func (ev *tcell.EventKey) *tcell.EventKey {
		switch {
		case ev.Rune() == 'l' && ev.Modifiers()&tcell.ModAlt != 0:
			log("focus log window")
			app.SetFocus(logWin)
			return nil
		}
		return ev
	})
}

func layout() *tview.Flex {
	logWin.SetDoneFunc(func(tcell.Key) {
		app.SetFocus(devList)
	})

	dump.SetDoneFunc(func(tcell.Key) {
		app.SetFocus(devList)
	})

	return tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(tview.NewFlex().
		AddItem(devList, 0, 1, true).
		AddItem(tview.NewFlex().
			AddItem(tree, 0, 1, false).
			AddItem(dump, 0, 1, false),
			0, 5, false), 0, 1, true).
		AddItem(logWin, 10, 0, false)
}

func replPage() error {
	acc, err := rest.Account()
	if err != nil {
		log("replpage error: %v", err)
		return err
	}

	rs := acc.ReplicantList
	devList.Clear()
	for i, r := range rs {
		rs[i], err = rest.Replicant(r.ReplicantCode.String())
		m, s := rs[i].ListItem()
		devList.AddItem(m, s, 0, func() {
			app.SetFocus(dump)
		})
	}
	devList.SetChangedFunc(func(i int, _, _ string, _ rune) {
		pp, _ := json.MarshalIndent(rs[i], "", "  ")
		dump.SetText(string(pp))
		tree.GetRoot().ClearChildren()
		for _, tn := range rs[i].Details() {
			tree.GetRoot().AddChild(tn)
		}
	})
	devList.SetCurrentItem(1)
	devList.SetCurrentItem(0)

	return nil
}
