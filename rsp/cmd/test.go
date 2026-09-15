package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/common"
)

var testCmd = &cobra.Command{
	Use: "test",
	RunE: func(cmd *cobra.Command, args []string) error {
		res, err := common.GetUpkeep()
		fmt.Println(err)
		prettyPrint(res)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(testCmd)
}
