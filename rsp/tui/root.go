package tui

import (
	"encoding/json"
	"fmt"

	"github.com/rivo/tview"
	"github.com/spf13/cobra"

	"github.com/zigdon/rsp/models"
	"github.com/zigdon/rsp/rest"
)

var app *tview.Application
var devList *tview.List
var details *tview.TextView

var TUI = &cobra.Command{
	Use: "tui",
	Short: "Launch a TUI interface",
	RunE: func(cmd *cobra.Command, args []string) error {
		app := tview.NewApplication()
		devList = tview.NewList()
		details = tview.NewTextView()
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
	dl, err := rest.AllDevices()
	if err != nil {
		return err
	}
	devs := make(map[string]*models.Device)

	for _, d := range dl {
		devs[d.Code.String()] = d
		desc := fmt.Sprintf("%s (%s) @ %s", d.Code.Alias(), d.Code.String(), d.Location)
		devList.AddItem(d.Type, desc,  0, func() {
			data, _ := json.MarshalIndent(d, "", "  ")
			details.SetText(fmt.Sprint(string(data)))
		})
	}

	return nil
}
