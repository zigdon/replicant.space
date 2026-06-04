package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/rest"
)

// travelCmd represents the travel command
var travelCmd = &cobra.Command{
	Use:   "travel",
	Short: "Instruct a replicant to relocate",
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 || args[0] == "" {
			return fmt.Errorf("A destination is required")
		}
		dest := args[0]
		rID, _ := cmd.Flags().GetString("code")
		if rID == "" {
			id, _ := cmd.Flags().GetInt("id")
			code, err := rest.ReplicantID(id)
			if err != nil {
				return fmt.Errorf("Replicant #%d not found: %v", id, err)
			}
			rID = code
		}
		res, err := rest.ReplicantTravel(rID, dest)
		if err != nil {
			return fmt.Errorf("Error starting trip: %v", err)
		}
		if raw, _ := cmd.Flags().GetBool("raw"); raw {
			prettyPrint(res)
		} else {
			printTable([]string{
				"Origin", "Destination", "Status",
				"Duration", "Departed", "Arrives",
			}, [][]string{{
				res.Origin, res.Destination, res.Status,
				res.TotalTime.String(), res.DepartedAt, res.ArrivesAt,
			}})
			var ls [][]string
			for _, l := range res.Route {
				ls = append(ls, []string{
					d(l.Leg), l.From, l.To, l.Type, l.Time.String(),
				})
			}
			printTable([]string{"Leg", "From", "To", "Type", "Duration"}, ls)
		}
		return nil
	},
}

func init() {
	replicantCmd.AddCommand(travelCmd)
}
