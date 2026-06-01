package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/rest"
)

var infoCmd = &cobra.Command{
	Use:   "info",
	Short: "Show detailed information about a device",
	RunE: func(cmd *cobra.Command, args []string) error {
		id, _ := cmd.Flags().GetString("device")
		resp, err := rest.DeviceInfo(id)
		if err != nil {
			return fmt.Errorf("Failed to get info for %q: %v", id, err)
		}
		if raw, _ := cmd.Flags().GetBool("raw"); raw {
			prettyPrint(resp)
		} else {
			printTable(
				[]string{"Code", "Type", "Location", "Features", "Status",
					"Replicant", "Commands", "Ops Capacity"},
				[][]string{{resp.Code, resp.Type, resp.Location,
					list(resp.Features), resp.Status, resp.ReplicantCode,
					list(resp.AvailableCommands), f(resp.OperationalCapacity)}},
				0,
			)
			if resp.Printing.EtaSeconds > 0 {
				print := resp.Printing
				printTable([]string{
					"Type", "Progress", "ETA", "Started", "Ends",
				}, [][]string{{
					print.DeviceType, p(print.ProgressPercent),
					print.EtaSeconds.String(), print.StartedAt, print.CompletesAt,
				}}, 0)
			}
		}
		return nil
	},
}

func init() {
	deviceCmd.AddCommand(infoCmd)
}
