package cmd

import (
	"slices"

	"github.com/spf13/cobra"
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
		oldest := getBool(cmd, "oldest")
		limit := getInt(cmd, "number")
		page := getInt(cmd, "page")
		logs, err := rest.DeviceLogs(models.NewCodeAlias(id), !oldest, page, limit)
		if err != nil {
			return err
		}

		if !oldest {
			slices.Reverse(logs.Events)
		}

		if raw := getBool(cmd, "raw"); raw {
			prettyPrint(logs)
			return nil
		}

		style := lg.NewStyle().Width(width)
		var ev [][]string
		for _, e := range logs.Events {
			ev = append(ev, []string{
				t(e.Created.Time()),
				style.Render(e.Message),
				v(e.Payload)})
		}
		printTable([]string{"Time", "Message", "Payload"}, ev)

		return nil
	},
}

func init() {
	deviceCmd.AddCommand(deviceLogsCmd)
	deviceLogsCmd.Flags().BoolP("oldest", "o", false, "Show oldest events")
	deviceLogsCmd.Flags().IntP("number", "n", 20, "Number of events to return")
	deviceLogsCmd.Flags().IntP("page", "p", 0, "Page number to return")
	deviceLogsCmd.Flags().IntP("width", "w", 50, "Wrap message body to this width")
}
