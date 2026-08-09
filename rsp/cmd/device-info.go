package cmd

import (
	"cmp"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/models"
	"github.com/zigdon/rsp/rest"
)

var infoCmd = &cobra.Command{
	Use:   "info",
	Short: "Show detailed information about a device",
	RunE: func(cmd *cobra.Command, args []string) error {
		id := getString(cmd, "device")

		refresh := getBool(cmd, "refresh")
		dev, err := rest.CachedDeviceInfo(models.NewCodeAlias(id), !refresh)
		if err != nil {
			return fmt.Errorf("Failed to get info for %q: %v", id, err)
		}
		if raw := getBool(cmd, "raw"); raw {
			prettyPrint(dev)
			return nil
		}
		code := dev.Code.Alias()
		code = lines([]string{code, unalias(code)})
		var cargo []string
		if dev.CargoCapacity > 0 {
			var totalCargo int
			for _, c := range dev.Cargo {
				totalCargo += c.Quantity
				cargo = append(cargo, fmt.Sprintf("%d × %s", c.Quantity, c.ResourceType))
			}
			cargo = append([]string{fmt.Sprintf("%d/%d (%d%%)",
				totalCargo, dev.CargoCapacity, 100*totalCargo/dev.CargoCapacity)}, cargo...)
		}
		status := dev.Status
		if !dev.InControlRange {
			status += "\n(out of range)"
		}
		printTable(
			[]string{"Code", "Type", "Location", "Status", "Attached", "Controller",
				"Replicant", "Ops Capacity", "Cargo"},
			[][]any{{code, dev.Type, dev.Location, status,
				dev.AttachedToDeviceCode,
				dev.ControllerDeviceCode, dev.ReplicantCode,
				p(dev.OperationalCapacity),
				lines(cargo),
			}},
		)
		var upkeep []string
		for _, u := range dev.UpkeepRequirements {
			upkeep = append(upkeep, u.String())
		}
		var owner string
		if dev.Owner != nil {
			owner = lines([]string{
				dev.Owner.Name, dev.Owner.Code.String(),
			})
		}
		printTable([]string{
			"Owner",
			"Created", "Deployed", "Grace", "Repairs", "System Active", "Stowed In",
			"Upkeep Requirements", "Taxi Mode", "Commands", "Tags", "Features"},
			[][]any{{
				owner,
				dev.Created.Time(), dev.Deployed.Time(), dev.GracePeriodRemaining,
				p(dev.RepairPaidPct), dev.SystemStatus, dev.StowedInDeviceCode, lines(upkeep),
				dev.TaxiMode, lines(dev.AvailableCommands), lines(dev.Tags), lines(dev.Features)}})
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
			}, [][]any{{
				name,
				dev.AmiDirectiveStatus,
				cfg,
				lines(dev.AvailableDirectives),
			}})
		}
		if dev.Compact != nil {
			u := dev.Compact
			printTable([]string{"Compacting started", "Progress", "Completes"},
				[][]any{{
					u.Started.Time(), u.ProgressPercent, u.Completes.Time(),
				}})
		}
		if dev.Unfurl != nil {
			u := dev.Unfurl
			printTable([]string{"Unfurling started", "Progress", "Completes"},
				[][]any{{
					u.Started.Time(), u.ProgressPercent, u.Completes.Time(),
				}})
		}
		if dev.Printing != nil || len(dev.PrintQueue) > 0 {
			printTime := make(map[string]time.Duration)
			bps := &models.Blueprints{}
			err := bps.Get()
			if err != nil {
				return err
			}
			for _, bp := range bps.Blueprints {
				printTime[bp.DeviceType] = bp.PrintTime.Duration()
			}
			var data [][]any
			var est time.Time
			if print := dev.Printing; print != nil {
				data = [][]any{{"-",
					print.DeviceType, p(print.ProgressPercent),
					print.Eta.String(), list(print.Tags), print.Started.Time(), print.Completes.Time(),
				}}
				est = print.Completes.Time()
			}
			for i, q := range dev.PrintQueue {
				dur := printTime[q.Type]
				data = append(data, []any{i,
					q.Type, "Queued", dur.String(), list(q.Tags), est, est.Add(dur),
				})
				est = est.Add(dur)
			}
			printTable([]string{"#", "Type", "Progress", "ETA", "Tags", "Started", "Ends"}, data)
		}
		if len(dev.WaitingFor.Components) > 0 || len(dev.WaitingFor.Resources) > 0 {

			var w [][]any
			for k, v := range dev.WaitingFor.Resources {
				w = append(w, []any{
					k, v.Have, v.Need, v.Need - v.Have,
				})
			}
			for k, v := range dev.WaitingFor.Components {
				w = append(w, []any{
					k, v.Have, v.Need, v.Need - v.Have,
				})
			}
			printTable([]string{"Resource", "Have", "Need", "Missing"}, w)
		}
		if len(dev.ControlledDevices) > 0 {
			var cds [][]any
			var mu sync.Mutex
			var wg sync.WaitGroup
			for _, d := range dev.ControlledDevices {
				wg.Go(func() {
					info, err := getInfo(d.Code)
					if err != nil {
						log("Error getting info for %q: %v", d, err)
						return
					}
					route := info.Travel.Short()
					var cargo string
					if len(info.Cargo) > 0 {
						var i []string
						for _, c := range info.Cargo {
							i = append(i, c.String())
						}
						cargo = lines(i)
					}
					mu.Lock()
					cds = append(cds, []any{
						d.Code, d.Type, d.Location, d.Status, route, cargo,
					})
					mu.Unlock()
				})
			}
			wg.Wait()
			slices.SortFunc(cds, func(a, b []any) int {
				as, aok := a[0].(*models.CodeAlias)
				bs, bok := b[0].(*models.CodeAlias)
				if !aok {
					return -1
				}
				if !bok {
					return 1
				}
				return cmp.Compare(as.Num(), bs.Num())
			})
			printTable([]string{
				"Code", "Type", "Location", "Status", "Route", "Cargo",
			}, cds)
		}
		if len(dev.AttachedDevices) > 0 {
			fmt.Printf("Attached devices (%d/%d):\n",
				len(dev.AttachedDevices), dev.AttachCapacity)
			var ds [][]any
			for _, d := range dev.AttachedDevices {
				ds = append(ds, []any{d.Type, d, d.Code.String()})
			}
			printTable([]string{"Type", "Alias", "Code"}, ds)
		}
		if dev.StowedDevices != nil && len(dev.StowedDevices.Devices) > 0 {
			fmt.Printf("Stowed devices (%d/%d):\n",
				len(dev.StowedDevices.Devices), dev.StowCapacity)
			var ds [][]any
			for _, d := range dev.StowedDevices.Devices {
				ds = append(ds, []any{d.Type, d, d.Code.String()})
			}
			printTable([]string{"Type", "Alias", "Code"}, ds)
		}
		if dev.Scan != nil {
			s := dev.Scan
			printTable([]string{
				"Target", "Started", "Progress", "ETA",
			}, [][]any{{
				s.Target, s.Started.String(), p(s.ProgressPercent), s.Eta,
			}})
		}
		if dev.Travel != nil {
			trip := dev.Travel
			printTable([]string{
				"Origin", "Destination", "ETA", "Percent", "Time Left", "Type",
			}, [][]any{{
				trip.Origin, trip.Destination,
				trip.Arrives.Time(), p(trip.ProgressPercent),
				trip.Eta, trip.Type,
			}})
			var legs [][]any
			for _, l := range trip.Route {
				dist := l.DistanceAu + l.DistanceLy
				legs = append(legs, []any{l.Leg, l.Active, l.From, l.To, dist, l.Type})
			}
			printTable([]string{"Leg", "Active", "From", "To", "Distance", "Type"}, legs)
		}
		return nil
	},
}

func init() {
	deviceCmd.AddCommand(infoCmd)
	infoCmd.Flags().BoolP("refresh", "r", false, "When set, refresh the cached info")
}
