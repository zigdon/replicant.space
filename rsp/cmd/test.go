package cmd

import (
	"fmt"
	"slices"

	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/common"
)

var testCmd = &cobra.Command{
	Use: "test",
	RunE: func(cmd *cobra.Command, args []string) error {
		prettyPrint(common.GetBP(args[0]))
		fmt.Println(
			slices.Contains(common.GetBP(args[0]).Features, "modular"),
		)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(testCmd)
}
