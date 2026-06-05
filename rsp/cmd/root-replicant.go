package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/rest"
)

// replicantCmd represents the replicant command
var replicantCmd = &cobra.Command{
	Use:   "replicant",
	Short: "Get replicant details",
	RunE: func(cmd *cobra.Command, args []string) error {
		rID, err := getRID(cmd)
		if err != nil {
			return fmt.Errorf("Replicant not found: %v", err)
		}
		repl, err := rest.Replicant(rID)
		if err != nil {
			return fmt.Errorf("Error scanning: %v", err)
		}
		if raw, _ := cmd.Flags().GetBool("raw"); raw {
			prettyPrint(repl)
		} else {
			printTable([]string{
				"Name", "Code", "Location", "XP", "Description", "Status",
			}, [][]string{{
				repl.Name, repl.ReplicantCode, repl.Location,
				d(repl.ExperiencePoints), repl.Description, repl.Status,
			}})
			if len(repl.PrintQueue) > 0 {
				var q [][]string
				for _, pq := range repl.PrintQueue {
					q = append(q, []string{
						pq.DeviceType,
						pq.Notify.Device,
						b(pq.Notify.Email),
						b(pq.Notify.Webhook),
					})
				}
				printTable([]string{
					"Type", "Notify device", "Notify email", "Notify webhook",
				}, q)
			}
			if len(repl.WaitingFor) > 0 {
				var w [][]string
				for k, v := range repl.WaitingFor {
					w = append(w, []string{
						k, d(v.Have), d(v.Need),
					})
				}
				printTable([]string{"Resource", "Have", "Need"}, w)
			}
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(replicantCmd)
	replicantCmd.PersistentFlags().StringP("code", "c", "", "Replicant ID to use (e.g. A32A933F)")
	replicantCmd.PersistentFlags().Int("id", 1, "Replicant ID to use (default 1, i.e. zigdon-1)")
}

func getRID(cmd *cobra.Command) (string, error) {
	rID, _ := cmd.Flags().GetString("code")
	if rID == "" {
		id, _ := cmd.Flags().GetInt("id")
		code, err := rest.ReplicantID(id)
		if err != nil {
			return "", fmt.Errorf("Replicant #%d not found: %v", id, err)
		}
		rID = code
	}
	return rID, nil
}
