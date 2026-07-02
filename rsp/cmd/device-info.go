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
		id, _ := cmd.Flags().GetString("device")
		dev, err := rest.DeviceInfo(models.NewCodeAlias(id))
		if err != nil {
			return fmt.Errorf("Failed to get info for %q: %v", id, err)
		}
		if raw, _ := cmd.Flags().GetBool("raw"); raw {
			prettyPrint(dev)
			return nil
		}
		code := dev.Code.Alias()
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
		printTable(
			[]string{"Code", "Type", "Location", "Status", "Attached", "Controller",
				"Replicant", "Ops Capacity", "Cargo"},
			[][]string{{code, dev.Type, dev.Location, dev.Status,
				dev.AttachedToDeviceCode.Alias(),
				dev.ControllerDeviceCode.Alias(), dev.ReplicantCode.Alias(),
				f(dev.OperationalCapacity),
				lines(cargo),
			}},
		)
		var upkeep []string
		for _, u := range dev.UpkeepRequirements {
			upkeep = append(upkeep, u.String())
		}
		var grace string
		if dev.GracePeriodRemaining > 0 {
			grace = d(dev.GracePeriodRemaining)
		}
		printTable([]string{
			"Created", "Deployed", "Grace", "Repairs", "System Active",
			"Upkeep Requirements", "Taxi Mode", "Commands", "Tags", "Features"},
			[][]string{{
				t(dev.Created.Time()), t(dev.Deployed.Time()), grace,
				p(dev.RepairPaidPct), v(dev.SystemStatus), lines(upkeep),
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
			}, [][]string{{
				name,
				dev.AmiDirectiveStatus,
				v(cfg),
				lines(dev.AvailableDirectives),
			}})
		}
		if dev.Printing != nil {
			printTime := make(map[string]time.Duration)
			bps := &models.Blueprints{}
			err := bps.Get()
			if err != nil {
				return err
			}
			for _, bp := range bps.Blueprints {
				printTime[bp.DeviceType] = bp.PrintTime.Duration()
			}
			print := dev.Printing
			data := [][]string{{
				print.DeviceType, p(print.ProgressPercent),
				print.Eta.String(), t(print.Started.Time()), t(print.Completes.Time()),
			}}
			est := print.Completes.Time()
			for _, q := range dev.PrintQueue {
				dur := printTime[q.Type]
				data = append(data, []string{
					q.Type, "Queued", dur.String(), t(est), t(est.Add(dur)),
				})
				est = est.Add(dur)
			}
			printTable([]string{"Type", "Progress", "ETA", "Started", "Ends"}, data)
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
							i = append(i, c.Short())
						}
						cargo = lines(i)
					}
					mu.Lock()
					cds = append(cds, []string{
						d.Code.Alias(), d.Type, d.Location, d.Status, route, cargo,
					})
					mu.Unlock()
				})
			}
			wg.Wait()
			slices.SortFunc(cds, func(a, b []string) int {
				return cmp.Compare(a[0], b[0])
			})
			printTable([]string{
				"Code", "Type", "Location", "Status", "Route", "Cargo",
			}, cds)
		}
		if len(dev.AttachedDevices) > 0 {
			fmt.Printf("Attached devices (%d/%d):\n",
				len(dev.AttachedDevices), dev.AttachCapacity)
			var ds [][]string
			for _, d := range dev.AttachedDevices {
				ds = append(ds, []string{d.Type, d.Code.Alias(), d.Code.String()})
			}
			printTable([]string{"Type", "Alias", "Code"}, ds)
		}
		if len(dev.StowedDevices) > 0 {
			fmt.Printf("Stowed devices (%d/%d):\n",
				len(dev.StowedDevices), dev.StowCapacity)
			var ds [][]string
			for _, d := range dev.StowedDevices {
				ds = append(ds, []string{d.Type, d.Code.Alias(), d.Code.String()})
			}
			printTable([]string{"Type", "Alias", "Code"}, ds)
		}
		if dev.Scan != nil {
			s := dev.Scan
			printTable([]string{
				"Target", "Started", "Progress", "ETA",
			}, [][]string{{
				s.Target, s.Started.String(), f(s.ProgressPercent) + "%", s.Eta.String(),
			}})
		}
		if dev.Travel != nil {
			trip := dev.Travel
			printTable([]string{
				"Origin", "Destination", "ETA", "Percent", "Time Left", "Type",
			}, [][]string{{
				trip.Origin, trip.Destination, t(trip.Arrives.Time()), f(trip.ProgressPercent),
				trip.Eta.String(), trip.Type,
			}})
			var legs [][]string
			for _, l := range trip.Route {
				dist := l.DistanceAu + l.DistanceLy
				legs = append(legs, []string{
					d(l.Leg), b(l.Active), l.From, l.To, f(dist), l.Type,
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
