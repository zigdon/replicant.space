package cmd

import (
	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/models"
)

var testCmd = &cobra.Command{
	Use:   "test",
	RunE: func(cmd *cobra.Command, args []string) error {
		bps := &models.Blueprints{}
		if err := bps.Get(); err != nil {
			return err
		}
		log("%+v", bps.Blueprints)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(testCmd)
}
