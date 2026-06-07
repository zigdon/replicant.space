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
				var cfg map[string]any
				var name string
				if resp.AmiDirective != nil {
					cfg = resp.AmiDirective.Config
					name = resp.AmiDirective.Name
				} else {
					name = "N/A"
				}
				printTable([]string{
					"Current Directive", "Status", "Configuration", "Available Directives",
				}, [][]string{{
					name,
					resp.AmiDirectiveStatus,
					v(cfg),
					lines(resp.AvailableDirectives),
				}})
			}
			if resp.Printing != nil {
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
			if len(resp.AttachedDevices) > 0 {
				fmt.Printf("Attached devices (%d/%d):\n",
					len(resp.AttachedDevices), resp.AttachCapacity)
				var ds [][]string
				for _, d := range resp.AttachedDevices {
					ds = append(ds, []string{d.Type, d.Code})
				}
				printTable([]string{"Type", "Code"}, ds)
			}
		}
		return nil
	},
}

func init() {
	deviceCmd.AddCommand(infoCmd)
}
