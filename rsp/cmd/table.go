package cmd

import (
	"fmt"
	"slices"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/models"
	"github.com/zigdon/rsp/rest"
)

var tableCmd = &cobra.Command{
	Use:   "table",
	Short: "Load an interactive table of all the devices",
	RunE:  runTable,
}

func init() {
	rootCmd.AddCommand(tableCmd)
}

func SelectableUnlessEmpty(t string) (bool, string) {
	return len(t) != 0, t
}

func NewDeviceCell(selectable bool, t string) *tview.TableCell {
	return tview.NewTableCell(t).SetSelectable(selectable)
}

var defaultTags = []string{"infrastructure", "mine", "matrix"}

func runTable(cmd *cobra.Command, args []string) error {
	table, err := getTable()
	if err != nil {
		return err
	}
	pages := tview.NewPages().
		AddAndSwitchToPage("devices", table, true)
	return tview.NewApplication().SetRoot(pages, true).Run()
}

func getTable() (*tview.Table, error) {
	devs, err := rest.Devices(nil)
	if err != nil {
		return nil, err
	}
	for _, d := range devs {
		d.Alias()
	}
	devs, _ = filterDevices(devs, defaultTags, nil)
	slices.SortFunc(devs, func(a, b *models.Device) int {
		return models.CompareAliases(a.Code, b.Code)
	})
	colFn := func(d *models.Device) []*tview.TableCell {
		status := d.Status
		if strings.Contains(status, "repairing (") {
			target := status[strings.Index(status, "(")+1 : strings.Index(status, ")")]
			status = fmt.Sprintf("repairing (%s)", alias(target))
		}
		return []*tview.TableCell{
			NewDeviceCell(true, d.Type),
			NewDeviceCell(true, d.Code.Alias()),
			NewDeviceCell(SelectableUnlessEmpty(d.ControllerDeviceCode.Alias())),
			NewDeviceCell(SelectableUnlessEmpty(d.Location)),
			NewDeviceCell(false, p(d.OperationalCapacity)),
			NewDeviceCell(false, status),
			NewDeviceCell(
				SelectableUnlessEmpty(d.StowedInDeviceCode.Alias() + d.AttachedToDeviceCode.Alias())),
			NewDeviceCell(true, list(d.Tags)),
		}
	}

	bold := tcell.Style{}.Bold(true)
	table := tview.NewTable().
		SetFixed(1, 0).
		SetSeparator(tview.Borders.Vertical).
		SetEvaluateAllRows(true).
		SetSelectable(true, true)
	for cn, h := range []string{
		"Type",
		"Code",
		"Controller",
		"Location",
		"Ops %",
		"Status",
		"With",
		"Tags",
	} {
		table.SetCell(0, cn,
			tview.NewTableCell(h).
				SetAlign(tview.AlignCenter).
				SetStyle(bold).
				SetSelectable(false))
	}
	for i, d := range devs {
		for cn, col := range colFn(d) {
			table.SetCell(i+1, cn, col)
		}
	}
	return table, nil
}
