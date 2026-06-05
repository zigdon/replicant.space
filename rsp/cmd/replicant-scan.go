package cmd

import (
	"fmt"
	"slices"
	"strings"

	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/models"
	"github.com/zigdon/rsp/rest"
)

// scanCmd represents the scan command
var scanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Scan a system",
	RunE: func(cmd *cobra.Command, args []string) error {
		rID, _ := cmd.Flags().GetString("code")
		if rID == "" {
			id, _ := cmd.Flags().GetInt("id")
			code, err := rest.ReplicantID(id)
			if err != nil {
				return fmt.Errorf("Replicant #%d not found: %v", id, err)
			}
			rID = code
		}
		scan, err := rest.ReplicantScan(rID)
		if err != nil {
			return fmt.Errorf("Error getting replicant details: %v", err)
		}
		if raw, _ := cmd.Flags().GetBool("raw"); raw {
			prettyPrint(scan)
		} else {
			printTable([]string{
				"Star",
				"Entry",
				"Life Detected",
				"Mining Bonus %",
				"Tags",
			}, [][]string{{
				scan.Star.Designation,
				scan.EntryPoint,
				b(scan.LifeDetected),
				d(scan.MiningBonusPct),
				list(scan.SystemTags),
			}})
			if scan.AsteroidBelt.Present {
				var belts [][]string
				for _, b := range scan.AsteroidBelt.Belts {
					belts = append(belts, []string{
						b.Designation,
						b.Density,
						m(b.Resources),
					})
				}
				printTable(
					[]string{"Designation", "Density", "Resources"}, belts,
				)
			}
			if len(scan.Planets) > 0 {
				var planets [][]string
				for _, p := range scan.Planets {
					var salvage []string
					for _, s := range p.Salvage {
						salvage = append(salvage, fmt.Sprintf(
							"%s (%s): %s", s.Name, s.Designation, list(s.ResourcesAvailable)))
					}
					planets = append(planets, []string{
						p.Name,
						p.Designation,
						p.Type,
						b(p.InHabitableZone),
						d(p.MoonCount),
						b(p.Scanned),
						strings.Join(salvage, "\n"),
					})
				}
				printTable([]string{
					"Name",
					"Designation",
					"Type",
					"Habitable Zone",
					"Moons",
					"Scanned",
					"Salvage",
				}, planets)

				if getInv, _ := cmd.Flags().GetBool("inventory"); getInv {
					inv := getInventory(scan)
					var names []string
					for k := range inv {
						names = append(names, k)
					}
					slices.Sort(names)
					var invLines [][]string
					for _, n := range names {
						invLines = append(invLines, []string{n, lines(inv[n])})
					}
					printTable([]string{"Name", "Inventory"}, invLines)
				}
			}
		}
		return nil
	},
}

func init() {
	replicantCmd.AddCommand(scanCmd)
	scanCmd.Flags().BoolP("inventory", "i", false, "Recursively scan planets and moons for loose change")
}

func getInventory(scan *models.Scan) map[string][]string {
	var targets []string
	if scan.AsteroidBelt.Present {
		for _, b := range scan.AsteroidBelt.Belts {
			targets = append(targets, b.Designation)
		}
	}
	for _, p := range scan.Planets {
		targets = append(targets, p.Designation)
	}

	var moons []string
	res := make(map[string][]string)
	for _, t := range targets {
		l, err := rest.Location(t)
		if err != nil {
			log("Error getting location %q: %v", t, err)
			continue
		}
		if len(l.Inventory) > 0 {
			for _, i := range l.Inventory {
				res[t] = append(res[t], fmt.Sprintf("%.0f x %s", i.Quantity, i.ResourceType))
			}
		}
		if len(l.ResourceSites) > 0 {
			for _, rs := range l.ResourceSites {
				for k, v := range rs.ResourcesRemainingPct {
					res[rs.Designation] = append(res[rs.Designation],
						fmt.Sprintf("%.2f%% x %s", v, k))
				}
			}
		}
		if len(l.Moons) > 0 {
			for _, m := range l.Moons {
				moons = append(moons, m.Designation)
			}
		}
	}

	for _, t := range moons {
		l, err := rest.Location(t)
		if err != nil {
			log("Error getting location %q: %v", t, err)
			continue
		}
		if len(l.Inventory) > 0 {
			for _, i := range l.Inventory {
				res[t] = append(res[t], fmt.Sprintf("%.0f x %s", i.Quantity, i.ResourceType))
			}
		}
		if len(l.ResourceSites) > 0 {
			for _, rs := range l.ResourceSites {
				for k, v := range rs.ResourcesRemainingPct {
					res[rs.Designation] = append(res[rs.Designation],
						fmt.Sprintf("%d%% %s", v, k))
				}
			}
		}
	}

	return res
}
