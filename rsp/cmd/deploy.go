package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/rest"
)

// deployCmd represents the deploy command
var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy a device",
	RunE: func(cmd *cobra.Command, args []string) error {
		id, _ := cmd.Flags().GetString("device")
		resp, err := rest.DeviceCommand(id, "deploy", nil)
		if err != nil {
			return fmt.Errorf("%q deploy failed: %v", id, err)
		}
		if raw, _ := cmd.Flags().GetBool("raw"); raw {
			prettyPrint(resp)
		} else {
			printTable(
				[]string{"Code", "Location", "Star", "Status"},
				[][]string{{resp.DeviceCode, resp.Location, resp.Star, resp.Status}},
			)
		}
		return nil
	},
}

func init() {
	deviceCmd.AddCommand(deployCmd)
}
