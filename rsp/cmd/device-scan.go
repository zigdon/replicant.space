package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/rest"
)

var deviceScanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Initiate a scan of the current location",
	RunE: func(cmd *cobra.Command, args []string) error {
		id, _ := cmd.Flags().GetString("device")
		resp, err := rest.DeviceCommand(id, "scan", nil)
		if err != nil {
			return fmt.Errorf("%q scan failed: %v", id, err)
		}
		if raw, _ := cmd.Flags().GetBool("raw"); raw {
			prettyPrint(resp)
		} else {
			printTable(
				[]string{"Code", "Belt", "ETA", "Status"},
				[][]string{{resp.DeviceCode, resp.Belt, resp.EtaSeconds.String(), resp.Status}},
				0,
			)
		}
		return nil
	},
}

func init() {
	deviceCmd.AddCommand(deviceScanCmd)
}
