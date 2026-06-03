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
		unread, _ := cmd.Flags().GetBool("unread")
		data, err := rest.Messages(cursor, number, latest, unread)
		if err != nil {
			return fmt.Errorf("Error getting status: %v", err)
		}
		if raw, _ := cmd.Flags().GetBool("raw"); raw {
			prettyPrint(data)
		} else {
		  var msgs [][]string
		  for _, m := range data.Messages {
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
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(msgCmd)
	msgCmd.Flags().BoolP("latest", "l", false, "Show latest messages")
	msgCmd.Flags().BoolP("unread", "u", true, "Show unread messages only")
	msgCmd.Flags().IntP("number", "n", 20, "Number of messages to show")
	msgCmd.Flags().IntP("cursor", "c", 0, "Position to start from")
	msgCmd.Flags().IntP("width", "w", 50, "Wrap message body to this width")
}
