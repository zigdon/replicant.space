package cmd

import (
	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/rest"
)

// travelCmd represents the travel command
var travelCmd = &cobra.Command{
	Use:   "travel",
	Short: "Instruct a replicant to relocate",
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 || args[0] == "" {
			log("A destination is required")
			return
		}
		dest := args[0]
		rID, _ := cmd.Flags().GetString("code")
		if rID == "" {
			id, _ := cmd.Flags().GetInt("id")
			code, err := rest.ReplicantID(id)
			if err != nil {
				log("Replicant #%d not found: %v", id, err)
				return
			}
			rID = code
		}
		trip, err := rest.Travel(rID, dest)
		if err != nil {
			log("Error starting trip: %v", err)
			return
		}
		prettyPrint(trip)
	},
}

func init() {
	replicantCmd.AddCommand(travelCmd)
}
