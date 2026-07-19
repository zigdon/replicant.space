package cmd

import (
	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/rest"
)

var testCmd = &cobra.Command{
	Use: "test",
	RunE: func(cmd *cobra.Command, args []string) error {
		out, err := rest.Devices(nil)
		prettyPrint(len(out))
		return err
	},
}

func init() {
	rootCmd.AddCommand(testCmd)
}
