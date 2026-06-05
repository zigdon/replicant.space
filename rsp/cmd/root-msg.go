package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
    "github.com/zigdon/rsp/rest"

	lg "charm.land/lipgloss/v2"
)

var msgCmd = &cobra.Command{
	Use:   "msg",
	Short: "Read the current messages",
	RunE: func(cmd *cobra.Command, args []string) error {
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
		  var ids []int
		  var msgs [][]string
		  for _, m := range data.Messages {
			ids = append(ids, m.ID)
			msgs = append(msgs, []string{
			  d(m.ID),
			  m.Type,
			  lg.NewStyle().Width(20).Render(m.Title),
			  lg.NewStyle().Width(width).Render(m.Body),
			  b(m.Read),
			  m.CreatedAt,
			})
		  }
		  printTable([]string{"ID", "Type", "Title", "Body", "Read", "Created"}, msgs)

		  if mark, _ := cmd.Flags().GetBool("mark"); mark {
			log("Marking messages read: %v", ids)
			if err := rest.MarkRead(ids); err != nil {
			  log("Error marking messages read: %v", err)
			}
		  }
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(msgCmd)
	msgCmd.Flags().BoolP("mark", "m", false, "Mark messages as read")
	msgCmd.Flags().BoolP("latest", "l", false, "Show latest messages")
	msgCmd.Flags().BoolP("read", "r", false, "Show also read messages")
	msgCmd.Flags().IntP("number", "n", 20, "Number of messages to show")
	msgCmd.Flags().IntP("cursor", "c", 0, "Position to start from")
	msgCmd.Flags().IntP("width", "w", 50, "Wrap message body to this width")
}
