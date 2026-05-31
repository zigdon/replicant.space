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
		trip, err := rest.Travel(rID, dest)
		if err != nil {
			return fmt.Errorf("Error starting trip: %v", err)
		}
		// if raw, _ := cmd.Flags().GetBool("raw"); raw { prettyPrint(trip) }
		prettyPrint(trip)
		return nil
	},
}

func init() {
	replicantCmd.AddCommand(travelCmd)
}
