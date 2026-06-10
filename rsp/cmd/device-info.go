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
			code := alias(resp.Code.String())
			code = lines([]string{code, unalias(code)})
			var cargo []string
			if resp.CargoCapacity > 0 {
				var totalCargo float32
				for _, c := range resp.Cargo {
					totalCargo += c.Quantity
					cargo = append(cargo, fmt.Sprintf("%.2f x %s", c.Quantity, c.ResourceType))
				}
				cargo = append([]string{fmt.Sprintf("%.2f/%d (%.0f%%)",
					totalCargo, resp.CargoCapacity, totalCargo/float32(resp.CargoCapacity)*100)}, cargo...)
			}
			printTable(
				[]string{"Code", "Type", "Location", "Features", "Status", "Taxi Mode",
					"Replicant", "Commands", "Ops Capacity", "Cargo", "Tags"},
				[][]string{{code, resp.Type, resp.Location,
					lines(resp.Features), resp.Status, resp.TaxiMode, alias(resp.ReplicantCode.String()),
					lines(resp.AvailableCommands), f(resp.OperationalCapacity),
					lines(cargo), lines(resp.Tags),
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
			if len(resp.WaitingFor) > 0 {
				var w [][]string
				for k, v := range resp.WaitingFor {
					w = append(w, []string{
						k, d(v.Have), d(v.Need),
					})
				}
				printTable([]string{"Resource", "Have", "Need"}, w)
			}
			if len(resp.ControlledDevices) > 0 {
				var cds [][]string
				for _, d := range resp.ControlledDevices {
					cds = append(cds, []string{
						alias(d.Code.String()), d.Type, d.Location, d.Status,
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
					ds = append(ds, []string{d.Type, d.Code.String()})
				}
				printTable([]string{"Type", "Code"}, ds)
			}
			if resp.Scan != nil {
				s := resp.Scan
				printTable([]string{
					"Target", "Started", "Progress", "ETA",
				}, [][]string{{
					s.Target, s.StartedAt, f(s.ProgressPercent) + "%", s.Eta.String(),
				}})
			}
			if resp.Travel != nil {
				t := resp.Travel
				printTable([]string{
					"Origin", "Destination", "ETA", "Percent", "Time Left", "Type",
				}, [][]string{{
					t.Origin, t.Destination, t.ArrivesAt, f(t.ProgressPercent), t.Eta.String(), t.Type,
				}})
				var legs [][]string
				for _, l := range t.Route {
					legs = append(legs, []string{
						d(l.Leg), b(l.Active), l.From, l.To, f(l.DistanceAu), l.Type,
					})
				}
				printTable([]string{"Leg", "Active", "From", "To", "Distance", "Type"}, legs)
			}
		}
		return nil
	},
}

func init() {
	deviceCmd.AddCommand(infoCmd)
}
