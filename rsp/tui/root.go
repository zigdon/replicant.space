package tui

import (
	"github.com/rivo/tview"
	"github.com/spf13/cobra"

	"github.com/zigdon/rsp/rest"
)

var devList = tview.NewList()
var details = tview.NewGrid()

var TUI = &cobra.Command{
	Use: "tui",
	Short: "Launch a TUI interface",
	RunE: func(cmd *cobra.Command, args []string) error {
		app := tview.NewApplication()
		flex := tview.NewFlex()
		flex.
		  AddItem(devList, 0, 2, true).
		  AddItem(details, 0, 5, false)
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
	details.SetBorders(true)
	devList.SetChangedFunc(func(i int, _, _ string, _ rune) {
		ModelGrid(details, rs[i].Details())
	})
	ModelGrid(details, rs[0].Details())

	return nil
}
