package cmd

import (
	"slices"

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
		var names []string
		for name := range acc.Replicants {
			names = append(names, name)
		}
		slices.Sort(names)

		for _, n := range names {
			printReplicantDeviceList(acc.Replicants[n])
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(deviceListCmd)
}
