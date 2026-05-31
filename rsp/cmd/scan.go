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
			}}, 0)
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
					[]string{"Designation", "Density", "Resources"},
					belts, 0,
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
				}, planets, 0)
			}
		}
		return nil
	},
}

func init() {
	replicantCmd.AddCommand(scanCmd)
}
