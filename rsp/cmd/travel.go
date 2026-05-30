package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/cfg"
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
			rID = cfg.GetID(id)
		}
		trip, err := rest.Travel(rID, dest)
		if err != nil {
			log("Error starting trip: %v", err)
			return
		}
		fmt.Printf("%#v\n", trip)
	},
}

func init() {
	replicantCmd.AddCommand(travelCmd)
}
