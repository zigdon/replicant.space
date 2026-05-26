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
		fmt.Println(rest.Post("replicants/%s/scan", nil, cfg.GetID(id)))
	},
}

func init() {
	rootCmd.AddCommand(scanCmd)
	scanCmd.Flags().IntP("id", "n", 1, "Replicant ID to use (default 1, i.e. zigdon-1)")
}
