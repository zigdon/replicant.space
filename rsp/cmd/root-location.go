package cmd

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
	"sync"

	lg "charm.land/lipgloss/v2"
	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/models"
	"github.com/zigdon/rsp/rest"
)

// locationCmd represents the location command
var locationCmd = &cobra.Command{
	Use:   "location",
	Short: "List the contents of a location",
	RunE: func(cmd *cobra.Command, args []string) error {
		var loc string
		if len(args) > 0 {
			loc = args[0]
		}
		res, err := rest.Location(loc)
		if err != nil {
			return fmt.Errorf("Failed to get location %q: %v", loc, err)
		}
		if raw, _ := cmd.Flags().GetBool("raw"); raw {
			prettyPrint(res)
			return nil
		}

		getInv, _ := cmd.Flags().GetBool("inventory")
		filter, _ := cmd.Flags().GetString("location")

		var data [][]string
		var locs []string
		for loc := range res.Locations {
			if filter != "" && !strings.Contains(loc, filter) {
				continue
			}
			locs = append(locs, loc)
		}
		var mu sync.Mutex
		var wg sync.WaitGroup
		for _, loc := range locs {
			sum := res.Locations[loc]
			wg.Go(func() {
				line := []string{
					loc, d(sum.Replicants), d(sum.Devices),
					d(sum.LocationEvents), d(sum.ResourceSites),
				}
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
								r = append(r, fmt.Sprintf("%.0f x %s", i.Quantity, i.ResourceType))
							}
							line = append(line, lines(r))
						}
					}
				} else {
					line = append(line, d(sum.Resources))
				}
				mu.Lock()
				data = append(data, line)
				mu.Unlock()
			})
		}
		wg.Wait()
		slices.SortFunc(data, func(a, b []string) int {
			return cmp.Compare(a[0], b[0])
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
			}, [][]string{{
				s.Designation, s.Name, res.EntryPoint, s.StellarClass,
				d(s.MiningBonusPct)+"%", s.Position.String(),
				f(s.DistanceFromSol)+"ly",
			}})
			var pp, mp int
			if res.PlanetsTotal > 0 { pp = res.PlanetsScanned*100 / res.PlanetsTotal }
			if res.MoonsTotal > 0 { mp = res.MoonsScanned*100 / res.MoonsTotal }
			printTable([]string{
				"System Scanned", "Planets", "Moons",
			}, [][]string{{
				b(res.SystemScanned),
				fmt.Sprintf("%d/%d (%d%%)", res.PlanetsScanned, res.PlanetsTotal, pp),
				fmt.Sprintf("%d/%d (%d%%)", res.MoonsScanned, res.MoonsTotal, mp),
			}})
			if res.AsteroidBelt.Present {
				data = [][]string{}
				for _, b := range res.AsteroidBelt.Belts {
					data = append(data, []string{
						b.Designation, b.Density, m(b.Resources),
					})
				}
				printTable([]string{"Belt", "Density", "Resources"}, data)
			}
			data = [][]string{}
			for _, p := range res.Planets {
				var inv []string
				for _, i := range p.Inventory {
					inv = append(inv, fmt.Sprintf("%.2f x %s", i.Quantity, i.ResourceType))
				}
				data = append(data, []string{
					p.Designation, p.Name, p.Type, p.LifeStage,
					d(p.MoonCount), b(p.Scanned), lines(inv),
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
			}, [][]string{{
				p.Designation, p.Name, b(p.InHabitableZone), p.LifeStage,
				p.Type, d(len(res.Moons)), b(p.Rings), list(p.Tags),
			}})

			data = [][]string{}
			for _, m := range res.Moons {
				data = append(data, []string{
					m.Designation, m.Type, m.Name, b(m.Scanned),
				})
			}
			if len(data) > 0 {
				printTable([]string{"Designation", "Type", "Name", "Scanned"}, data)
			}
		}

		if res.Type == "moon" {
			m := res.Moon
			printTable([]string{
				"Designation", "Name", "Type", "Parent",
			}, [][]string{{
				m.Designation, m.Name, m.Type, m.ParentPlanet,
			}})
		}

		data = [][]string{}
		for _, i := range res.Inventory {
			data = append(data, []string{i.ResourceType, f(i.Quantity)})
		}
		if len(data) > 0 {
			printTable([]string{"Resource", "Quantity"}, data)
		}

		data = [][]string{}
		slices.SortFunc(res.Devices, func(a, b *models.Device) int {
			return cmp.Compare(a.Type, b.Type)
		})
		for _, d := range res.Devices {
			data = append(data, []string{d.Code.Alias(), d.Type, d.Status})
		}
		if len(data) > 0 {
			printTable([]string{"Device Code", "Type", "Status"}, data)
		}

		if len(res.ResourceSites) > 0 {
			data = [][]string{}
			for _, s := range res.ResourceSites {
				data = append(data, []string{
					d(s.Index), s.Type, s.Designation, s.Name,
					m(s.ResourcesRemainingPct),
				})
			}
			printTable([]string{
				"Index", "Type", "Designation", "Name", "Resources Pct",
			}, data)
		}

		if res.LocationEvent != nil {
			printEvent(res.LocationEvent, lg.NewStyle().Width(40))
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(locationCmd)
	locationCmd.Flags().BoolP("inventory", "i", false, "Fetch inventory in each location")
	locationCmd.Flags().StringP("location", "l", "", "Filter only locations that match")
}
