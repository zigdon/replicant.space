package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/rest"

	lg "charm.land/lipgloss/v2"
)

var eventCmd = &cobra.Command{
	Use:   "events",
	Short: "Read the event log",
	RunE: func(cmd *cobra.Command, args []string) error {
		rID, err := getRID(cmd)
		if err != nil {
			return fmt.Errorf("Replicant not found: %v", err)
		}
		width, _ := cmd.Flags().GetInt("width")
		cursor, _ := cmd.Flags().GetInt("cursor")
		number, _ := cmd.Flags().GetInt("number")
		latest, _ := cmd.Flags().GetBool("latest")
		eventType, _ := cmd.Flags().GetString("event_type")
		deviceType, _ := cmd.Flags().GetString("device_type")
		deviceCode, _ := cmd.Flags().GetString("device_code")
		data, err := rest.ReplicantEvents(rID, cursor, number, latest, eventType, deviceType, deviceCode)
		if err != nil {
			return fmt.Errorf("Error getting event log: %v", err)
		}
		if raw, _ := cmd.Flags().GetBool("raw"); raw {
			prettyPrint(data)
		} else {
			var events [][]string
			for _, e := range data.ReplicantEvents {
				var payload []string
				for k, v := range e.Payload {
					payload = append(payload, fmt.Sprintf("%s: %v", k, v))
				}
				events = append(events, []string{
					e.DeviceCode.String(),
					e.DeviceType,
					e.Type,
					lg.NewStyle().Width(width).Render(e.Message),
					lg.NewStyle().Width(width).Render(lines(payload)),
					t(e.Created),
				})
			}
			printTable([]string{
				"Device Code", "Device Type", "Event Type", "Message",
				"Details", "Created At"}, events)
		}
		return nil
	},
}

func init() {
	replicantCmd.AddCommand(eventCmd)
	eventCmd.Flags().BoolP("latest", "l", true, "Show latest messages")
	eventCmd.Flags().IntP("number", "n", 20, "Number of messages to show")
	eventCmd.Flags().IntP("cursor", "p", 0, "Position to start from")
	eventCmd.Flags().IntP("width", "w", 30, "Wrap message body to this width")
	eventCmd.Flags().StringP("event_type", "T", "", "Only show these event types")
	eventCmd.Flags().StringP("device_type", "D", "", "Only show events for these device types")
	eventCmd.Flags().StringP("device_code", "C", "", "Only show events for this device")
}
