package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/rest"
)

// replicantCmd represents the replicant command
var replicantCmd = &cobra.Command{
	Use:   "replicant",
	Short: "Get replicant details",
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
		repl, err := rest.Replicant(rID)
		if err != nil {
			return fmt.Errorf("Error scanning: %v", err)
		}
		prettyPrint(repl)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(replicantCmd)
	replicantCmd.PersistentFlags().StringP("code", "c", "", "Replicant ID to use (e.g. A32A933F)")
	replicantCmd.PersistentFlags().IntP("id", "n", 1, "Replicant ID to use (default 1, i.e. zigdon-1)")
}
