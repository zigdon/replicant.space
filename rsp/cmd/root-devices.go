package cmd

import (
	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/rest"
)

var deviceListCmd = &cobra.Command{
	Use:   "devices",
	Short: "List all the devices on the account",
	RunE: func(cmd *cobra.Command, args []string) error {
		acc, err := rest.Account()
		if err != nil {
			return err
		}
		for _, r := range acc.Replicants {
			printReplicantDeviceList(r)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(deviceListCmd)
}
