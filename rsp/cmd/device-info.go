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
		dev, err := rest.DeviceInfo(id)
		if err != nil {
			return fmt.Errorf("Failed to get info for %q: %v", id, err)
		}
		if raw, _ := cmd.Flags().GetBool("raw"); raw {
			prettyPrint(dev)
			return nil
		}
		code := alias(dev.Code.String())
		code = lines([]string{code, unalias(code)})
		var cargo []string
		if dev.CargoCapacity > 0 {
			var totalCargo float32
			for _, c := range dev.Cargo {
				totalCargo += c.Quantity
				cargo = append(cargo, fmt.Sprintf("%.2f x %s", c.Quantity, c.ResourceType))
			}
			cargo = append([]string{fmt.Sprintf("%.2f/%d (%.0f%%)",
				totalCargo, dev.CargoCapacity, totalCargo/float32(dev.CargoCapacity)*100)}, cargo...)
		}
		var pq []string
		for _, q := range dev.PrintQueue {
			pq = append(pq, q.Type)
		}
		printTable(
			[]string{"Code", "Type", "Location", "Features", "Status", "Taxi Mode",
				"Replicant", "Commands", "Ops Capacity", "Cargo", "Tags", "Print Queue"},
			[][]string{{code, dev.Type, dev.Location,
				lines(dev.Features), dev.Status, dev.TaxiMode, alias(dev.ReplicantCode.String()),
				lines(dev.AvailableCommands), f(dev.OperationalCapacity),
				lines(cargo), lines(dev.Tags), lines(pq),
			}},
		)
		if len(dev.AvailableDirectives) > 0 {
			var cfg map[string]any
			var name string
			if dev.AmiDirective != nil {
				cfg = dev.AmiDirective.Config
				name = dev.AmiDirective.Name
			} else {
				name = "N/A"
			}
			printTable([]string{
				"Current Directive", "Status", "Configuration", "Available Directives",
			}, [][]string{{
				name,
				dev.AmiDirectiveStatus,
				v(cfg),
				lines(dev.AvailableDirectives),
			}})
		}
		if dev.Printing != nil {
			print := dev.Printing
			printTable([]string{
				"Type", "Progress", "ETA", "Started", "Ends",
			}, [][]string{{
				print.DeviceType, p(print.ProgressPercent),
				print.Eta.String(), print.StartedAt, print.CompletesAt,
			}})
		}
		if len(dev.WaitingFor) > 0 {
			var w [][]string
			for k, v := range dev.WaitingFor {
				w = append(w, []string{
					k, d(v.Have), d(v.Need),
				})
			}
			printTable([]string{"Resource", "Have", "Need"}, w)
		}
		if len(dev.ControlledDevices) > 0 {
			var cds [][]string
			for _, d := range dev.ControlledDevices {
				cds = append(cds, []string{
					alias(d.Code.String()), d.Type, d.Location, d.Status,
				})
			}
			printTable([]string{
				"Code", "Type", "Location", "Status",
			}, cds)
		}
		if len(dev.AttachedDevices) > 0 {
			fmt.Printf("Attached devices (%d/%d):\n",
				len(dev.AttachedDevices), dev.AttachCapacity)
			var ds [][]string
			for _, d := range dev.AttachedDevices {
				ds = append(ds, []string{d.Type, d.Code.String()})
			}
			printTable([]string{"Type", "Code"}, ds)
		}
		if dev.Scan != nil {
			s := dev.Scan
			printTable([]string{
				"Target", "Started", "Progress", "ETA",
			}, [][]string{{
				s.Target, s.StartedAt, f(s.ProgressPercent) + "%", s.Eta.String(),
			}})
		}
		if dev.Travel != nil {
			trip := dev.Travel
			printTable([]string{
				"Origin", "Destination", "ETA", "Percent", "Time Left", "Type",
			}, [][]string{{
				trip.Origin, trip.Destination, t(trip.Arrives), f(trip.ProgressPercent), trip.Eta.String(), trip.Type,
			}})
			var legs [][]string
			for _, l := range trip.Route {
				legs = append(legs, []string{
					d(l.Leg), b(l.Active), l.From, l.To, f(l.DistanceAu), l.Type,
				})
			}
			printTable([]string{"Leg", "Active", "From", "To", "Distance", "Type"}, legs)
		}
		return nil
	},
}

func init() {
	deviceCmd.AddCommand(infoCmd)
}
