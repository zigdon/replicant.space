package cmd

import (
	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/models"
)

var testCmd = &cobra.Command{
	Use: "test",
	RunE: func(cmd *cobra.Command, args []string) error {
		bs := &models.Blueprints{}
		if err := bs.Get(); err != nil {
			return err
		}
		prettyPrint(bs)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(testCmd)
}
