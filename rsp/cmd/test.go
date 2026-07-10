package cmd

import (
	"github.com/spf13/cobra"
)

var testCmd = &cobra.Command{
	Use: "test",
	RunE: func(cmd *cobra.Command, args []string) error {
		log("%v", explode(args[0]))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(testCmd)
}
