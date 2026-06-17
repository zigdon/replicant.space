package cmd

import (
	"fmt"
	"slices"

	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/rest"

	lg "charm.land/lipgloss/v2"
)

var msgCmd = &cobra.Command{
	Use:   "msg",
	Aliases: []string{"msgs"},
	Short: "Read the current messages",
	RunE: func(cmd *cobra.Command, args []string) error {
		var partial bool
		ids, _ := cmd.Flags().GetIntSlice("ids")
		if len(ids) > 0 {
			partial = true
		}
		width, _ := cmd.Flags().GetInt("width")
		cursor, _ := cmd.Flags().GetInt("cursor")
		number, _ := cmd.Flags().GetInt("number")
		latest, _ := cmd.Flags().GetBool("latest")
		readToo, _ := cmd.Flags().GetBool("read")
		data, err := rest.Messages(cursor, number, latest, !readToo)
		if err != nil {
			return fmt.Errorf("Error getting status: %v", err)
		}
		if raw, _ := cmd.Flags().GetBool("raw"); raw {
			prettyPrint(data)
		} else {
		  var msgs [][]string
		  tStyle := lg.NewStyle().Width(20)
		  bStyle := lg.NewStyle().Width(width)
		  for _, m := range data.Messages {
			if !partial {
				ids = append(ids, m.ID)
			}
			msgs = append(msgs, []string{
			  d(m.ID),
			  m.Type,
			  tStyle.Render(m.Title),
			  bStyle.Render(m.Body),
			  b(m.Read),
			  t(m.Created),
			})
		  }
		  printTable([]string{"ID", "Type", "Title", "Body", "Read", "Created"}, msgs)

		  if mark, _ := cmd.Flags().GetBool("mark"); partial || mark {
			log("Marking messages read: %v", ids)
			if err := rest.MarkRead(ids); err != nil {
			  log("Error marking messages read: %v", err)
			}
		  }
		}
		return nil
	},
}

var bobCmd = &cobra.Command{
	Use:   "bob",
	Short: "Read messages from bobnet",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Find a relay we can use
		acc, err := rest.Account()
		if err != nil { return err }

		var relayID string
		for _, r := range acc.Replicants {
			devices, err := rest.ReplicantDevices(r.ReplicantCode.String(), "")
			if err != nil { continue }
			for _, d := range devices {
				if d.Type != "ftl_relay" || d.Status != "relaying" || !d.InControlRange { continue }
				relayID = d.Code.String()
				break
			}
			if relayID != "" { break }
		}

		if relayID == "" {
			return fmt.Errorf("Failed to find an FTL relay in range")
		}

		cursor, _ := cmd.Flags().GetInt("cursor")
		number, _ := cmd.Flags().GetInt("number")
		latest, _ := cmd.Flags().GetBool("latest")
		npcs, _ := cmd.Flags().GetBool("npcs")
		width, _ := cmd.Flags().GetInt("width")
		ids, _ := cmd.Flags().GetBool("replicant_ids")
		locs, _ := cmd.Flags().GetBool("replicant_location")
		channels, _ := cmd.Flags().GetStringSlice("channels")
		data, err := rest.Bobnet(relayID, cursor, number, latest, npcs)
		if err != nil {
			return fmt.Errorf("Error getting bobnet messages: %v", err)
		}
		headers := []string{"Channel", "Name", "Time", "Message"}
		var lines [][]string
		style := lg.NewStyle().Width(width)
		slices.Reverse(data.Messages)
		for _, d := range data.Messages {
			if len(channels) > 0 && !slices.Contains(channels, d.Channel) { continue }
			var who string
			if ids || locs {
				d.ReplicantCode = "#" + d.ReplicantCode
				d.CurrentStar = "@" + d.CurrentStar
				who = fmt.Sprintf("%s (%s%s)", d.ReplicantName, d.ReplicantCode, d.CurrentStar)
			} else {
				who = d.ReplicantName
			}
			lines = append(lines, []string{
				d.Channel, who, t(d.Time), style.Render(d.Message),
			})
		}
		printTable(headers, lines)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(msgCmd)
	msgCmd.Flags().BoolP("mark", "m", false, "Mark messages as read")
	msgCmd.Flags().BoolP("latest", "l", true, "Show latest messages")
	msgCmd.Flags().BoolP("read", "r", false, "Show also read messages")
	msgCmd.Flags().IntP("number", "n", 20, "Number of messages to show")
	msgCmd.Flags().IntP("cursor", "C", 0, "Position to start from")
	msgCmd.Flags().IntP("width", "w", 50, "Wrap message body to this width")
	msgCmd.Flags().IntSlice("ids", []int{}, "Mark these messages as read")

	msgCmd.AddCommand(bobCmd)
	bobCmd.Flags().BoolP("latest", "l", true, "Show latest messages")
	bobCmd.Flags().IntP("number", "n", 20, "Number of messages to show")
	bobCmd.Flags().IntP("cursor", "C", 0, "Position to start from")
	bobCmd.Flags().IntP("width", "w", 50, "Wrap message body to this width")
	bobCmd.Flags().BoolP("npcs", "p", true, "Show messages from NPCs")
	bobCmd.Flags().Bool("replicant_ids", false, "Show replicant IDs")
	bobCmd.Flags().Bool("replicant_location", false, "Show replicant locations")
	bobCmd.Flags().StringSliceP("channels", "c", []string{}, "Only show messages to these channels")
}
