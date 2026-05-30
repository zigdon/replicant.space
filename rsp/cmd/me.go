package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
    "github.com/zigdon/rsp/rest"
)

// meCmd represents the me command
var meCmd = &cobra.Command{
	Use:   "me",
	Short: "Show current status",
	RunE: func(cmd *cobra.Command, args []string) error {
		me, err := rest.Account()
		if err != nil {
			return fmt.Errorf("Error getting status: %v", err)
		}
		if raw, _ := cmd.Flags().GetBool("raw"); raw {
			prettyPrint(me)
		} else {
			printTable(
				[]string{"Name", "Bobnet", "XP", "Status", "Unread messages"},
				[][]string{{
					me.Name,
					list(me.BobnetChannels),
					d(me.ExperiencePointsTotal),
					me.Status,
					d(me.UnreadMessageCount),
				}},
			)
			var reps [][]string
			for _, r := range me.Replicants {
				reps = append(reps, []string{
					r.Name,
					r.ReplicantCode,
					r.CurrentLocation,
					d(r.ExperiencePoints),
					r.Status,
				})
			}
			printTable([]string{"Name", "Code", "Location", "XP", "Status"}, reps)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(meCmd)
}
