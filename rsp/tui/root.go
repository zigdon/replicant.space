package tui

import (
	"github.com/rivo/tview"
	"github.com/spf13/cobra"

	"github.com/zigdon/rsp/rest"
)

var devList = tview.NewList()
var tree = tview.NewTreeView().SetRoot(tview.NewTreeNode("Details"))

var TUI = &cobra.Command{
	Use: "tui",
	Short: "Launch a TUI interface",
	RunE: func(cmd *cobra.Command, args []string) error {
		app := tview.NewApplication()
		flex := tview.NewFlex()
		flex.
		  AddItem(devList, 0, 1, true).
		  AddItem(tree, 0, 5, false)
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
		devList.AddItem(m, s, 0, nil)
	}
	devList.SetChangedFunc(func(i int, _, _ string, _ rune) {
		tree.GetRoot().ClearChildren()
		for _, tn := range rs[i].Details() {
			tree.GetRoot().AddChild(tn)
		}
	})

	return nil
}
