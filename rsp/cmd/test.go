package cmd

import (
	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/common"
)

var testCmd = &cobra.Command{
	Use: "test",
	RunE: func(cmd *cobra.Command, args []string) error {
		path, err := common.RelayIsland(args[0], 7.5, 0)
		prettyPrint(path)
		return err
	},
}

func init() {
	rootCmd.AddCommand(testCmd)
}
