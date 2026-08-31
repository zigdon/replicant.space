package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/common"
)

var testCmd = &cobra.Command{
	Use: "test",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println(common.FullNetwork())
		return nil
	},
}

func init() {
	rootCmd.AddCommand(testCmd)
}
