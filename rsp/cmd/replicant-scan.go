package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/rest"
)

// scanCmd represents the scan command
var scanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Scan a system",
	RunE: func(cmd *cobra.Command, args []string) error {
		rID, err := getRID(cmd)
		if err != nil {
			return fmt.Errorf("Replicant not found: %v", err)
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
			}

			if len(scan.ActiveLocationEvents) > 0 {
				var events [][]string
				for _, e := range scan.ActiveLocationEvents {
					events = append(events, []string{
						e.Designation, e.Title, e.EventType, e.Location, d(e.Tier),
					})
				}
				printTable([]string{"Designation", "Title", "Type", "Location", "Tier"}, events)
			}

			if len(scan.Replicants) > 0 {
				var reps [][]string
				for _, r := range scan.Replicants {
					reps = append(reps, []string{r.Name, r.Code, r.Location, r.LastActive})
				}
				printTable([]string{"Name", "Code", "Location", "Last Active"}, reps)
			}

			if len(scan.Shops) > 0 {
				for _, shop := range scan.Shops {
					var trades [][]string
					printTable(
						[]string{"Name", "Owner", "Location", "Description"},
						[][]string{{shop.ShopName, shop.OwnerName, shop.Location, shop.Description}})
					for _, tr := range shop.Trades {
						trades = append(trades, []string{
							tr.Name, d(tr.CurrentStock), tr.TradeCode,
							m(tr.Criteria.Resources),
							m(tr.Rewards.Devices),
						})
					}
					printTable([]string{"Name", "Stock", "Code", "Criteria", "Rewards"}, trades)
				}
			}
		}
		return nil
	},
}

func init() {
	replicantCmd.AddCommand(scanCmd)
}
