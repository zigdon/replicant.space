package cmd

import (
	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/rest"
)

// scanCmd represents the scan command
var scanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Scan a system",
	Run: func(cmd *cobra.Command, args []string) {
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
		rep, err := rest.ReplicantScan(rID)
		if err != nil {
			log("Error getting replicant details: %v", err)
			return
		}
		prettyPrint(rep)
	},
}

func init() {
	replicantCmd.AddCommand(scanCmd)
}
