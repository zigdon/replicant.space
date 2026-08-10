package cmd

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/models"
	"github.com/zigdon/rsp/rest"
)

var locationCmd = &cobra.Command{
	Use:               "location",
	Short:             "List the contents of a location",
	ValidArgsFunction: completeStars,
	RunE: func(cmd *cobra.Command, args []string) error {
		var loc string
		if len(args) > 0 {
			loc = args[0]
		}
		res, err := rest.Location(loc)
		if err != nil {
			return fmt.Errorf("Failed to get location %q: %v", loc, err)
		}
		if raw := getBool(cmd, "raw"); raw {
			prettyPrint(res)
			return nil
		}

		getInv := getBool(cmd, "inventory")
		filter := getString(cmd, "location")

		var data [][]any
		var locs []string
		for loc := range res.Locations {
			if filter != "" && !strings.Contains(string(loc), filter) {
				continue
			}
			locs = append(locs, string(loc))
		}
		var mu sync.Mutex
		var wg sync.WaitGroup
		for _, loc := range locs {
			sum := res.Locations[models.LocationID(loc)]
			wg.Go(func() {
				line := []any{loc, sum.Replicants, sum.Devices, sum.LocationEvents, sum.ResourceSites}
				if getInv {
					if sum.Resources == 0 {
						line = append(line, "N/A")
					} else {
						inv, err := rest.Location(loc)
						if err != nil {
							line = append(line, fmt.Sprintf("Err: %v", err))
						} else {
							var r []string
							for _, i := range inv.Inventory {
								r = append(r, i.String())
							}
							line = append(line, lines(r))
						}
					}
				} else {
					line = append(line, sum.Resources)
				}
				mu.Lock()
				data = append(data, line)
				mu.Unlock()
			})
		}
		wg.Wait()
		slices.SortFunc(data, func(a, b []any) int {
			return cmp.Compare(a[0].(string), b[0].(string))
		})
		if len(data) > 0 {
			headers := []string{"Designation", "Replicants", "Devices", "Events", "Sites"}
			if getInv {
				headers = append(headers, "Inventory")
			} else {
				headers = append(headers, "Resources")
			}
			printTable(headers, data)
		}

		if res.Type == "star" {
			s := res.Star
			printTable([]string{
				"Designation", "Name", "Entry Point", "Class", "Mining Bonus",
				"Position", "Distance from SOL",
			}, [][]any{{
				s.Designation, s.Name, res.EntryPoint, s.StellarClass,
				fmt.Sprintf("%d%%", s.MiningBonusPct), s.Position,
				fmt.Sprintf("%.2fly", s.DistanceFromSol),
			}})
			var pp, mp int
			if res.PlanetsTotal > 0 {
				pp = res.PlanetsScanned * 100 / res.PlanetsTotal
			}
			if res.MoonsTotal > 0 {
				mp = res.MoonsScanned * 100 / res.MoonsTotal
			}
			printTable([]string{
				"System Scanned", "Planets", "Moons",
			}, [][]any{{
				res.SystemScanned,
				fmt.Sprintf("%d/%d (%d%%)", res.PlanetsScanned, res.PlanetsTotal, pp),
				fmt.Sprintf("%d/%d (%d%%)", res.MoonsScanned, res.MoonsTotal, mp),
			}})
			if res.AsteroidBelt != nil {
				if res.AsteroidBelt.Present {
					data = [][]any{}
					for _, b := range res.AsteroidBelt.Belts {
						data = append(data, []any{
							b.Designation, b.Density, m(b.Resources),
						})
					}
					printTable([]string{"Belt", "Density", "Resources"}, data)
				}
			}
			data = [][]any{}
			for _, p := range res.Planets {
				var inv []string
				for _, i := range p.Inventory {
					inv = append(inv, fmt.Sprintf("%d × %s", i.Quantity, i.ResourceType))
				}
				data = append(data, []any{
					p.Designation, p.Name, p.Type, p.LifeStage,
					p.MoonCount, p.Scanned, lines(inv),
				})
			}
			printTable([]string{
				"Designation", "Name", "Type", "Life stage", "Moons", "Scanned", "Inventory",
			}, data)
		}

		if res.Type == "planet" {
			p := res.Planet
			printTable([]string{
				"Designation", "Name", "Habitable", "LifeStage", "Type", "Moons",
				"Rings", "Tags",
			}, [][]any{{
				p.Designation, p.Name, p.InHabitableZone, p.LifeStage,
				p.Type, len(res.Moons), p.Rings, list(p.Tags),
			}})

			data = [][]any{}
			for _, m := range res.Moons {
				data = append(data, []any{m.Designation, m.Type, m.Name, m.Scanned})
			}
			if len(data) > 0 {
				printTable([]string{"Designation", "Type", "Name", "Scanned"}, data)
			}
		}

		if res.Type == "moon" {
			m := res.Moon
			printTable([]string{
				"Designation", "Name", "Type", "Parent",
			}, [][]any{{m.Designation, m.Name, m.Type, m.ParentPlanet}})
		}

		if res.Type == "object" {
			var data [][]any
			so := res.Object
			data = append(data, []any{
				so.Designation, so.Status, so.ObjectType, so.SizeClass, so.OrbitalDistanceAu,
				so.ImpactTarget, so.ImpactEta.Time(), p(so.ImpactLikelihood), so.RequiredStrength,
				so.ActivePlates, p(so.ProgressPct), so.CurrentThrustPerHour,
			})
			printTable([]string{
				"Designation", "Status", "Type", "Class", "Distance AU", "Impact Target",
				"ETA", "Likelyhood", "Required Strength", "Active Plates", "Progress",
				"Thrust/hr"}, data)
		}

		data = [][]any{}
		for _, i := range res.Inventory {
			data = append(data, []any{i.ResourceType, i.Quantity})
		}
		if len(data) > 0 {
			printTable([]string{"Resource", "Quantity"}, data)
		}

		data = [][]any{}
		slices.SortFunc(res.Devices, func(a, b *models.Device) int {
			return cmp.Compare(a.Type, b.Type)
		})
		for _, d := range res.Devices {
			data = append(data, []any{d, d.Type, d.Status})
		}
		if len(data) > 0 {
			printTable([]string{"Device Code", "Type", "Status"}, data)
		}

		if len(res.ResourceSites) > 0 {
			data = [][]any{}
			for _, s := range res.ResourceSites {
				data = append(data, []any{
					s.Index, s.Type, s.Designation, s.Name, m(s.ResourcesRemainingPct),
				})
			}
			printTable([]string{
				"Index", "Type", "Designation", "Name", "Resources Pct",
			}, data)
		}

		if res.LocationEvent != nil {
			printEventSummary([]*models.Event{res.LocationEvent})
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(locationCmd)
	locationCmd.Flags().BoolP("inventory", "i", false, "Fetch inventory in each location")
	locationCmd.Flags().StringP("location", "l", "", "Filter only locations that match")
}
