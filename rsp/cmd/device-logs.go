package cmd

import (
	"slices"

	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/models"
	"github.com/zigdon/rsp/rest"

	lg "charm.land/lipgloss/v2"
)

var deviceLogsCmd = &cobra.Command{
	Use:   "log",
	Aliases: []string{"logs"},
	Short: "Read the device logs",
	RunE: func(cmd *cobra.Command, args []string) error {
		id, _ := cmd.Flags().GetString("device")
		width, _ := cmd.Flags().GetInt("width")
		latest, _ := cmd.Flags().GetBool("latest")
		limit, _ := cmd.Flags().GetInt("number")
		page, _ := cmd.Flags().GetInt("page")
		logs, err := rest.DeviceLogs(models.NewCodeAlias(id), latest, page, limit)
		if err != nil {
		  return err
		}

		if latest {
		  slices.Reverse(logs.Events)
		}

		if raw, _ := cmd.Flags().GetBool("raw"); raw {
		  prettyPrint(logs)
		  return nil
		}

		style := lg.NewStyle().Width(width)
		var ev [][]string
		for _, e := range logs.Events {
		  ev = append(ev, []string{t(e.Created.Time()), e.EventType, style.Render(e.Message), v(e.Payload)})
		}
		printTable([]string{"Time", "Type", "Message", "Payload"}, ev)

		return nil
	},
}

func init() {
	deviceCmd.AddCommand(deviceLogsCmd)
	deviceLogsCmd.Flags().BoolP("latest", "l", false, "Show latest events")
	deviceLogsCmd.Flags().IntP("number", "n", 20, "Number of events to return")
	deviceLogsCmd.Flags().IntP("page", "p", 0, "Page number to return")
	deviceLogsCmd.Flags().IntP("width", "w", 50, "Wrap message body to this width")
}
