package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/common"
	"github.com/zigdon/rsp/models"
)

var testCmd = &cobra.Command{
	Use: "test",
	RunE: func(cmd *cobra.Command, args []string) error {
		dev := args[0]
		dest := args[1]
		eta, err := common.Travel(models.NewCodeAlias(dev), dest, true)
		fmt.Printf("eta: %v, err: %v\n", eta, err)
		return err
	},
}

func init() {
	rootCmd.AddCommand(testCmd)
}
