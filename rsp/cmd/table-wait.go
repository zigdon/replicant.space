package cmd

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/models"
	"github.com/zigdon/rsp/rest"
)

var waitCmd = &cobra.Command{
	Use:   "wait",
	Short: "Follow all pending tasks",
	RunE:  waitPending,
}

func init() {
	rootCmd.AddCommand(waitCmd)
}

type ETA struct {
	Device       *models.Device
	Source, Dest models.LocationID
	Start        time.Time
	Ends         time.Time
	Note         string
}

func (e *ETA) Short() string {
	return fmt.Sprintf("%s->%s: %s", e.Source, e.Dest, time.Until(e.Ends).Truncate(time.Second))
}

func getETA(d *models.Device) *ETA {
	now := d.Fetched()
	if now.IsZero() {
		log("no fetched stamp on %s", d.Code.Alias())
		now = time.Now()
	}
	if d.Travel != nil {
		t := d.Travel
		var note string
		if d.CargoCapacity > 0 {
			var fill int
			for _, i := range d.Cargo {
				fill += int(i.Quantity)
			}
			note = fmt.Sprintf("%d/%d (%d%%)", fill, d.CargoCapacity, 100*fill/d.CargoCapacity)
		}
		return &ETA{
			Device: d,
			Source: t.Origin,
			Dest:   t.Destination,
			Start:  t.Departed.Time(),
			Ends:   t.Arrives.Time(),
			Note:   note,
		}
	}
	if d.Unfurl != nil {
		c := d.Unfurl
		return &ETA{
			Device: d,
			Source: d.Location,
			Start:  c.Started.Time(),
			Ends:   c.Completes.Time(),
			Note:   fmt.Sprintf("%.f%%", c.ProgressPercent),
		}
	}
	if d.Compact != nil {
		c := d.Compact
		return &ETA{
			Device: d,
			Source: d.Location,
			Start:  c.Started.Time(),
			Ends:   c.Completes.Time(),
			Note:   fmt.Sprintf("%.f%%", c.ProgressPercent),
		}
	}
	if d.Prospect != nil {
		p := d.Prospect
		return &ETA{
			Device: d,
			Source: d.Location,
			Start:  p.Started.Time(),
			Ends:   p.Completes.Time(),
			Note:   fmt.Sprintf("%.f%%", p.ProgressPercent),
		}
	}
	if d.Scan != nil {
		s := d.Scan
		return &ETA{
			Device: d,
			Source: d.Location,
			Start:  s.Started.Time(),
			Ends:   now.Add(s.Eta.Duration()),
		}
	}
	if d.Repair != nil {
		r := d.Repair
		eta := r.Started.Time().Add(
			time.Duration(1-r.ProgressPercent/100) * time.Since(r.Started.Time()))
		return &ETA{
			Device: d,
			Source: d.Location,
			Dest:   models.LocationID(r.TargetDeviceCode.Alias()),
			Start:  r.Started.Time(),
			Ends:   eta,
			Note:   fmt.Sprintf("%.f%%", r.ProgressPercent),
		}
	}
	if d.Status == "diverting" {
		loc, err := rest.Location(string(d.Location))
		if err != nil {
			log("Error getting info for %q: %v", d.Location, err)
			return nil
		}
		if loc.Object == nil {
			log("No object found in %q", d.Location)
			return nil
		}
		obj := loc.Object
		req := obj.RequiredStrength
		tph := obj.CurrentThrustPerHour
		pct := obj.ProgressPct
		impact := obj.ImpactEta.Time()
		missing := req - req*pct/100
		eta := now.Add(time.Second * time.Duration(3600*missing/tph))
		note := fmt.Sprintf("%.0f%% diverted", pct)
		if eta.After(impact) {
			note += fmt.Sprintf("!!! too late by %s", eta.Sub(impact))
		}
		return &ETA{
			Device: d,
			Source: d.Location,
			Start:  obj.Discovered.Time(),
			Ends:   eta,
			Note:   note,
		}
	}
	if strings.HasPrefix(d.Status, "printing") {
		d.Status = "printing"
		p := d.Printing
		return &ETA{
			Device: d,
			Source: d.Location,
			Start:  p.Started.Time(),
			Ends:   p.Completes.Time(),
			Note:   fmt.Sprintf("%s: %.0f%%", p.DeviceType, p.ProgressPercent),
		}
	}
	return &ETA{
		Device: d,
		Note:   fmt.Sprintf("Unknown status: %s", d.Status),
	}
}

func waitPending(cmd *cobra.Command, args []string) error {
	colFn := func(eta *ETA) []*tview.TableCell {
		d := eta.Device
		s := tcell.StyleDefault
		now := time.Now()
		if now.After(eta.Ends) {
			s = s.Bold(true)
		}
		return []*tview.TableCell{
			NewCell(true, d.Code.Alias()).SetStyle(s),
			NewCell(true, d.Type).SetStyle(s),
			NewCell(true, d.Status).SetStyle(s),
			NewCell(true, string(eta.Source)).SetStyle(s),
			NewCell(true, string(eta.Dest)).SetStyle(s),
			NewCell(true, dt(time.Until(eta.Start))).SetStyle(s),
			NewCell(true, dt(time.Until(eta.Ends))).SetStyle(s),
			NewCell(true, eta.Note).SetStyle(s),
		}
	}

	table := tview.NewTable().
		SetSeparator(tview.Borders.Vertical).
		SetSelectable(true, false)
	logWin := newLogWindow()
	layout := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(table, 0, 1, true).
		AddItem(logWin, 10, 0, false)
	app := tview.NewApplication().SetRoot(layout, true)
	boring := []string{
		"collecting",
		"coordinating",
		"depositing",
		"idle",
		"inactive",
		"mining",
		"monitoring",
		"paused",
		"relaying",
		"scanning",
		"searching",
		"stowed",
		"tracking",
	}
	for cn, h := range []string{
		"Code",
		"Type",
		"Status",
		"Source",
		"Destination",
		"Started",
		"Ends",
		"Notes",
	} {
		table.SetCell(0, cn,
			tview.NewTableCell(h).
				SetAlign(tview.AlignCenter).
				SetStyle(tcell.StyleDefault.Bold(true)).
				SetSelectable(false).
				SetExpansion(1),
		)
	}
	table.SetFixed(1, 1)
	go func() {
		rows := make(map[string]int)
		rows["_title"] = 0
		for {
			devs, err := rest.Devices(nil)
			if err != nil {
				log("Error reading devices: %v", err)
				app.Draw()
				time.Sleep(time.Second)
				continue
			}
			models.SortDevices(devs)
			var r int
			for _, d := range devs {
				if slices.ContainsFunc(boring, func(s string) bool {
					return strings.HasPrefix(d.Status, s)
				}) {
					rows, r = removeRow(d.Code, rows)
					if r >= 0 {
						st := table.GetCell(r, 2).Text
						log("Removed %s: %s", d.Code.Alias(), st)
						table.RemoveRow(r)
					}
					continue
				}
				eta := getETA(d)
				if eta == nil {
					rows, r = removeRow(d.Code, rows)
					if r >= 0 {
						st := table.GetCell(r, 2).Text
						log("Removed %s: %s", d.Code.Alias(), st)
						table.RemoveRow(r)
					}
					continue
				}
				if time.Now().Add(-1 * time.Minute).After(eta.Ends) {
					rows, r = removeRow(d.Code, rows)
					if r >= 0 {
						st := table.GetCell(r, 2).Text
						log("Removed %s: %s", d.Code.Alias(), st)
						table.RemoveRow(r)
					}
					continue
				}
				r, ok := rows[d.Code.String()]
				if !ok {
					for _, v := range rows {
						if v >= r {
							r = v + 1
						}
					}
					rows[d.Code.String()] = r
					table.InsertRow(r)
					log("Added %s: %s", d.Code.Alias(), eta.Short())
				}
				for n, c := range colFn(eta) {
					table.SetCell(r, n, c)
				}
			}
			app.Draw()
			time.Sleep(time.Second)
		}
	}()
	inputCapture := func(ev *tcell.EventKey) *tcell.EventKey {
		switch {
		case ev.Rune() == 'q':
			app.Stop()
		}
		return ev
	}
	app.SetInputCapture(inputCapture)
	return app.Run()
}

func removeRow(id *models.CodeAlias, rows map[string]int) (map[string]int, int) {
	r, ok := rows[id.String()]
	if !ok {
		return rows, -1
	}
	for k, v := range rows {
		if v < r {
			continue
		}
		rows[k] = v - 1
	}
	delete(rows, id.String())
	return rows, r
}
