package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/rest"
)

// scanCmd represents the scan command
var scanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Scan a system",
	RunE: func(cmd *cobra.Command, args []string) error {
		rID, _ := cmd.Flags().GetString("code")
		if rID == "" {
			id, _ := cmd.Flags().GetInt("id")
			code, err := rest.ReplicantID(id)
			if err != nil {
				return fmt.Errorf("Replicant #%d not found: %v", id, err)
			}
			rID = code
		}
		rep, err := rest.ReplicantScan(rID)
		if err != nil {
			return fmt.Errorf("Error getting replicant details: %v", err)
		}
		prettyPrint(rep)
		return nil
	},
}

func init() {
	replicantCmd.AddCommand(scanCmd)
}
