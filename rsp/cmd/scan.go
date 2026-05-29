package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/cfg"
	"github.com/zigdon/rsp/rest"
)

// scanCmd represents the scan command
var scanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Scan a system",
	Run: func(cmd *cobra.Command, args []string) {
		id, _ := cmd.Flags().GetInt("id")
		rID := cfg.GetID(id)
		scan, err := rest.ReplicantScan(rID)
		if err != nil {
			log("Error scanning: %v", err)
			return
		}
		fmt.Printf("%#v\n", scan)
	},
}

func init() {
	rootCmd.AddCommand(scanCmd)
	scanCmd.Flags().IntP("id", "n", 1, "Replicant ID to use (default 1, i.e. zigdon-1)")
}
