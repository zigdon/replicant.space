package cmd

import (
	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/models"
	"github.com/zigdon/rsp/rest"
)

var replicantCmd = &cobra.Command{
	Use:   "replicant",
	Short: "Get replicant details",
	RunE: replicantInfoCmd.RunE,
}

func init() {
	rootCmd.AddCommand(replicantCmd)
	replicantCmd.PersistentFlags().StringP("code", "c", "", "Replicant ID to use (e.g. A32A933F)")
	replicantCmd.PersistentFlags().Int("id", 1, "Replicant ID to use (default 1, i.e. zigdon-1)")
}

func getRID(cmd *cobra.Command) (*models.CodeAlias, error) {
	rID, _ := cmd.Flags().GetString("code")
	if rID != "" {
		return models.NewCodeAlias(rID), nil
	}

	id, _ := cmd.Flags().GetInt("id")
	return rest.ReplicantID(id)
}
