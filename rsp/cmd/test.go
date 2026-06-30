package cmd

import (
	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/rest"
)

var testCmd = &cobra.Command{
	Use: "test",
	RunE: func(cmd *cobra.Command, args []string) error {
		devs, err := rest.Devices(nil)
		if err != nil {
			return err
		}
		log("%+v", devs)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(testCmd)
}
