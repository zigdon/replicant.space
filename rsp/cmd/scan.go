package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/cfg"
	"github.com/zigdon/rsp/models"
	"github.com/zigdon/rsp/rest"
)

// scanCmd represents the scan command
var scanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Scan a system",
	Run: func(cmd *cobra.Command, args []string) {
		id, _ := cmd.Flags().GetInt("id")
		res, err := rest.Post("replicants/%s/scan", nil, cfg.GetID(id))
		if err != nil {
			log("Error scanning: %v", err)
			return
		}
		if raw {
			fmt.Println(res)
			return
		}
		printScan(res)
	},
}

func init() {
	rootCmd.AddCommand(scanCmd)
	scanCmd.Flags().IntP("id", "n", 1, "Replicant ID to use (default 1, i.e. zigdon-1)")
}

func printScan(data []byte) {
	scan, err := models.ParseScan(data)
	if err != nil {
		log("Error parsing scan: %v", err)
		return
	}
	fmt.Printf("%#v\n", scan)
}
