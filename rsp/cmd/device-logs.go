package cmd

import (
	"cmp"
	"encoding/json"
	"fmt"
	"slices"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/common"
	"github.com/zigdon/rsp/models"
	"github.com/zigdon/rsp/rest"

	lg "charm.land/lipgloss/v2"
)

var deviceLogsCmd = &cobra.Command{
	Use:     "log",
	Aliases: []string{"logs"},
	Short:   "Read the device logs",
	RunE: func(cmd *cobra.Command, args []string) error {
		id := getString(cmd, "device")
		width := getInt(cmd, "width")
		limit := getInt(cmd, "number")
		logs, err := rest.DeviceLogs(models.NewCodeAlias(id), limit)
		if err != nil {
			return err
		}

		if raw := getBool(cmd, "raw"); raw {
			prettyPrint(logs)
			return nil
		}

		style := lg.NewStyle().Width(width)
		var ev [][]any
		for _, e := range logs.Events {
			ev = append(ev, []any{
				e.Created.Time(),
				style.Render(e.Message),
				e.Payload})
		}
		printTable([]string{"Time", "Message", "Payload"}, ev)

		return nil
	},
}

func init() {
	deviceCmd.AddCommand(deviceLogsCmd)
	deviceLogsCmd.Flags().BoolP("oldest", "o", false, "Show oldest events")
	deviceLogsCmd.Flags().IntP("number", "n", 20, "Number of events to return")
	deviceLogsCmd.Flags().IntP("cursor", "c", 0, "Pointer to the oldest read message")
	deviceLogsCmd.Flags().IntP("width", "w", 50, "Wrap message body to this width")

	deviceLogsCmd.AddCommand(logTableCmd)
}

var logTableCmd = &cobra.Command{
	Use:   "table",
	Short: "Interactive event viewer",
	RunE:  logTable,
}

func logTable(cmd *cobra.Command, args []string) error {
	devID := getString(cmd, "device")
	devCA := models.NewCodeAlias(devID)
	listWin := tview.NewTable().SetSelectable(true, false)
	msgWin := tview.NewTextView()
	filterType := ""
	var eventTypes []string
	var events []*models.DeviceEvent

	app := tview.NewApplication()
	setEventLine := func(n int, msg *models.DeviceEvent) {
		style := tcell.StyleDefault
		listWin.SetCell(n, 0,
			NewCell(true, common.Dt(time.Until(msg.Created.Time()))).
				SetStyle(style).
				SetReference(msg))
		listWin.SetCell(n, 1, NewCell(true, msg.EventType).SetStyle(style))
		listWin.SetCell(n, 2, NewCell(true, msg.Message).SetStyle(style))
	}
	fetchNewEvents := func() error {
		_, err := rest.DeviceLogs(devCA, 0)
		return err
	}
	getEvents := func() {
		if err := fetchNewEvents(); err != nil {
			log(err.Error())
		}
		rows, err := db.DB.Query(`
		  SELECT id, created, device, type, message, payload
		  FROM device_logs WHERE device = $1`, devCA.String())
		if err != nil {
			log(err.Error())
		}

		// Clear reset the cached list.
		events = events[:0]
		for rows.Next() {
			e := new(models.DeviceEvent)
			var dc string
			var data []byte
			if err := rows.Scan(
				&e.Id, &e.Created, &dc, &e.EventType, &e.Message, &data); err != nil {
				log("Error reading data: %v", err)
				continue
			}
			e.DeviceCode = models.NewCodeAlias(dc)
			if err := json.Unmarshal(data, &e.Payload); err != nil {
				log(err.Error())
			}
			events = append(events, e)
		}
		if err := rows.Err(); err != nil {
			log("Error closing query: %v", err)
		}

		slices.SortFunc(events, func(a, b *models.DeviceEvent) int {
			return cmp.Compare(a.Id, b.Id)
		})

		// Clear the list
		for listWin.GetRowCount() > 1 {
			listWin.RemoveRow(1)
		}

		line := 1
		filterCnt := 0
		for _, ev := range events {
			if !slices.Contains(eventTypes, ev.EventType) {
				eventTypes = append(eventTypes, ev.EventType)
			}
			if filterType != "" && ev.EventType != filterType {
				filterCnt++
				continue
			}
			line++
			setEventLine(line, ev)
		}
		slices.Sort(eventTypes)
		log("Showing %d %smessages (%d filtered)", line-1, filterType, filterCnt)
	}
	displayCell := func(row, col int) {
		ref := listWin.GetCell(row, 0).GetReference()
		if ref == nil {
			return
		}
		ev := ref.(*models.DeviceEvent)
		msgWin.Clear()
		fmt.Fprintf(msgWin, "%s (%s ago) %-20s\n\n%s\n\n",
			ev.Created.Time().Truncate(time.Second).Format(time.Stamp),
			time.Since(ev.Created.Time()).Truncate(time.Second), ev.EventType,
			ev.Message,
		)
		data, err := json.MarshalIndent(ev.Payload, "", "  ")
		if err != nil {
			log(err.Error())
		}
		fmt.Fprintf(msgWin, "%s", string(data))
	}
	listWin.SetSelectionChangedFunc(displayCell).
		SetBorder(true)
	titleStyle := tcell.StyleDefault.Underline(true)
	listWin.SetBorderPadding(1, 1, 1, 1)
	listWin.
		SetCell(0, 0, NewCell(false, "When").SetAlign(tview.AlignCenter).SetStyle(titleStyle)).
		SetCell(0, 1, NewCell(false, "Type").SetAlign(tview.AlignCenter).SetStyle(titleStyle)).
		SetCell(0, 2, NewCell(false, "Title").SetAlign(tview.AlignCenter).SetStyle(titleStyle)).
		SetFixed(1, 0)

	logWin := newLogWindow()
	msgWin.SetBorder(true).SetBorderPadding(2, 2, 2, 2)
	layout := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(tview.NewFlex().
			AddItem(listWin, 0, 1, true).
			AddItem(msgWin, 0, 2, false), 0, 1, true).
		AddItem(logWin, 20, 0, false)
	getEvents()
	listWin.Select(listWin.GetRowCount()-1, 0)
	inputCapture := func(ev *tcell.EventKey) *tcell.EventKey {
		switch {
		case ev.Rune() == 'r':
			getEvents()
		case ev.Rune() == 't':
			if len(eventTypes) == 0 || filterType == eventTypes[len(eventTypes)-1] {
				filterType = ""
			} else {
				filterType = eventTypes[slices.Index(eventTypes, filterType)+1]
			}
			getEvents()
		case ev.Rune() == 'q':
			app.Stop()
		}
		// Only allow keystroke handling if we actually have messages to view.
		if listWin.GetRowCount() > 1 {
			return ev
		}
		return nil
	}
	app.SetInputCapture(inputCapture)

	return app.SetRoot(layout, true).Run()
}
