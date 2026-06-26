package tui

import (
	"encoding/json"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/spf13/cobra"

	"github.com/zigdon/rsp/rest"
)

var devList = tview.NewList()
var tree = tview.NewTreeView().SetRoot(tview.NewTreeNode("Details"))
var dump = tview.NewTextView()
var app *tview.Application

var TUI = &cobra.Command{
	Use: "tui",
	Short: "Launch a TUI interface",
	RunE: func(cmd *cobra.Command, args []string) error {
		dump.SetDoneFunc(func(tcell.Key) {
			app.SetFocus(devList)
		})
		app = tview.NewApplication()
		flex := tview.NewFlex()
		flex.
		  AddItem(devList, 0, 1, true).
		  AddItem(tview.NewFlex().
		  	AddItem(tree, 0, 1, false).
		  	AddItem(dump, 0, 1, false),
		  0, 5, false)
		if err := initData(); err != nil {
			return err
		}
		app.SetRoot(flex, true)
		return app.Run()
	},
}

func initData() error {
	acc, err := rest.Account()
	if err != nil {
		return err
	}

	rs := acc.ReplicantList
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
	devList.SetDoneFunc(func() { app.Stop() })

	return nil
}
