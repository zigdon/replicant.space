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
			var cargo []string
			for _, c := range resp.Cargo {
				cargo = append(cargo, fmt.Sprintf("%.2f x %s", c.Quantity, c.ResourceType))
			}
			printTable(
				[]string{"Code", "Type", "Location", "Features", "Status",
					"Replicant", "Commands", "Ops Capacity", "Cargo"},
				[][]string{{resp.Code, resp.Type, resp.Location,
					lines(resp.Features), resp.Status, resp.ReplicantCode,
					lines(resp.AvailableCommands), f(resp.OperationalCapacity),
					lines(cargo),
				}},
			)
			if len(resp.AvailableDirectives) > 0 {
				printTable([]string{
					"Current Directive", "Configuration", "Available Directives",
				}, [][]string{{
					resp.AmiDirective.Name,
					v(resp.AmiDirective.Config),
					lines(resp.AvailableDirectives),
				}})
			}
			if resp.Printing.EtaSeconds > 0 {
				print := resp.Printing
				printTable([]string{
					"Type", "Progress", "ETA", "Started", "Ends",
				}, [][]string{{
					print.DeviceType, p(print.ProgressPercent),
					print.EtaSeconds.String(), print.StartedAt, print.CompletesAt,
				}})
			}
			if len(resp.ControlledDevices) > 0 {
				var cds [][]string
				for _, d := range resp.ControlledDevices {
					cds = append(cds, []string{
						d.Code, d.Type, d.Location, d.Status,
					})
				}
				printTable([]string{
					"Code", "Type", "Location", "Status",
				}, cds)
			}
		}
		return nil
	},
}

func init() {
	deviceCmd.AddCommand(infoCmd)
}
