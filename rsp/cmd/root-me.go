package cmd

import (
	"fmt"
	"slices"

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
				}})
			var reps [][]string
			var names []string
			for name := range me.Replicants {
				names = append(names, name)
			}
			slices.Sort(names)
			for _, name := range names {
				r := me.Replicants[name]
				code, err := db.Alias(r.ReplicantCode.String(), "replicant")
				if err != nil {
					log("Error creating alias for %q: %v", err)
					code = r.ReplicantCode.String()
				}

				reps = append(reps, []string{
					r.Name,
					code,
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
