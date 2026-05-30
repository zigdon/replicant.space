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
		rep, err := rest.Replicant(rID)
		if err != nil {
			log("Error getting replicant details: %v", err)
			return
		}
		fmt.Printf("%#v\n", rep)
	},
}

func init() {
	rootCmd.AddCommand(scanCmd)
	scanCmd.Flags().StringP("code", "c", "", "Replicant ID to use (e.g. A32A933F)")
	scanCmd.Flags().IntP("id", "n", 1, "Replicant ID to use (default 1, i.e. zigdon-1)")
}
