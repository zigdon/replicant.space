package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/rest"
)

// locationCmd represents the location command
var locationCmd = &cobra.Command{
	Use:   "location",
	Short: "List the contents of a location",
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 || args[0] == "" {
			return fmt.Errorf("Location is required, pass as an argument")
		}
		res, err := rest.Location(args[0])
		if err != nil {
			return fmt.Errorf("Failed to get location %q: %v", args[0], err)
		}
		if raw, _ := cmd.Flags().GetBool("raw"); raw {
			prettyPrint(res)
			return nil
		}

		var data [][]string
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
			var data [][]string
			for _, p := range res.Planets {
				data = append(data, []string{
					p.Designation, p.Name, p.Type, p.LifeStage,
					d(p.MoonCount), b(p.Scanned),
				})
			}
			printTable([]string{
				"Designation", "Name", "Type", "Life stage", "Moons", "Scanned",
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
		for _, d := range res.Devices {
			data = append(data, []string{d.Code, d.Type, d.Status})
		}
		if len(data) > 0 {
			printTable([]string{"Device Code", "Type", "Status"}, data)
		}

		if len(res.ResourceSites) > 0 {
			data = [][]string{}
			for _, s := range res.ResourceSites {
				data = append(data, []string{
					d(s.Index), s.Type, s.Designation, s.Name,
					m[int](s.ResourcesRemainingPct),
				})
			}
			printTable([]string{
				"Index", "Type", "Designation", "Name", "Resources Pct",
			}, data)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(locationCmd)
}
