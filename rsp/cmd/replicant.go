package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/cfg"
	"github.com/zigdon/rsp/rest"
)

// replicantCmd represents the replicant command
var replicantCmd = &cobra.Command{
	Use:   "replicant",
	Short: "Get replicant details",
	Run: func(cmd *cobra.Command, args []string) {
		rID, _ := cmd.Flags().GetString("code")
		if rID == "" {
			id, _ := cmd.Flags().GetInt("id")
			rID = cfg.GetID(id)
		}
		scan, err := rest.ReplicantScan(rID)
		if err != nil {
			log("Error scanning: %v", err)
			return
		}
		fmt.Printf("%#v\n", scan)
	},
}

func init() {
	rootCmd.AddCommand(replicantCmd)
	replicantCmd.PersistentFlags().StringP("code", "c", "", "Replicant ID to use (e.g. A32A933F)")
	replicantCmd.PersistentFlags().IntP("id", "n", 1, "Replicant ID to use (default 1, i.e. zigdon-1)")
}
