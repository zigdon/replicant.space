package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/rest"
)

var replicantScanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Scan a system",
	RunE:  replicantScan,
}

func replicantScan(cmd *cobra.Command, args []string) error {
	rID, err := getRID(cmd)
	if err != nil {
		return fmt.Errorf("Replicant not found: %v", err)
	}
	scan, err := rest.ReplicantScan(rID)
	if err != nil {
		return fmt.Errorf("Error getting replicant details: %v", err)
	}
	if raw := getBool(cmd, "raw"); raw {
		prettyPrint(scan)
		return nil
	}
	printTable([]string{
		"Star",
		"Entry",
		"Life Detected",
		"Mining Bonus %",
		"Tags",
	}, [][]any{{
		scan.Star.Designation,
		scan.EntryPoint,
		scan.LifeDetected,
		scan.MiningBonusPct,
		list(scan.SystemTags),
	}})
	if scan.AsteroidBelt.Present {
		var belts [][]any
		for _, b := range scan.AsteroidBelt.Belts {
			belts = append(belts, []any{
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
		var planets [][]any
		for _, p := range scan.Planets {
			var salvage []string
			for _, s := range p.Salvage {
				salvage = append(salvage, fmt.Sprintf(
					"%s (%s): %s", s.Name, s.Designation, list(s.ResourcesAvailable)))
			}
			planets = append(planets, []any{
				p.Name,
				p.Designation,
				p.Type,
				p.InHabitableZone,
				p.MoonCount,
				p.Scanned,
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

	var outer [][]any
	if scan.OuterSystem.Oort != nil {
		outer = append(outer,
			[]any{scan.OuterSystem.Oort.Designation, scan.OuterSystem.Oort.DistanceAu})
	}
	if scan.OuterSystem.Kuiper != nil {
		outer = append(outer,
			[]any{scan.OuterSystem.Kuiper.Designation, scan.OuterSystem.Kuiper.DistanceAu})
	}
	if len(outer) > 0 {
		printTable([]string{"Outer system", "Distance AU"}, outer)
	}

	if len(scan.ActiveLocationEvents) > 0 {
		var events [][]any
		for _, e := range scan.ActiveLocationEvents {
			events = append(events, []any{
				e.Designation, e.Title, e.EventType, e.Location, e.Tier,
			})
		}
		printTable([]string{"Designation", "Title", "Type", "Location", "Tier"}, events)
	}

	if len(scan.Replicants) > 0 {
		var reps [][]any
		for _, r := range scan.Replicants {
			reps = append(reps, []any{r.Name, r.Code, r.Location, r.LastActive})
		}
		printTable([]string{"Name", "Code", "Location", "Last Active"}, reps)
	}

	if len(scan.Shops) > 0 {
		for _, shop := range scan.Shops {
			var trades [][]any
			printTable(
				[]string{"Name", "Owner", "Location", "Description"},
				[][]any{{shop.ShopName, shop.OwnerName, shop.Location, shop.Description}})
			for _, tr := range shop.Trades {
				trades = append(trades, []any{
					tr.Name, tr.CurrentStock, tr.Code,
					m(tr.Criteria.Resources),
					m(tr.Rewards.Devices),
				})
			}
			printTable([]string{"Name", "Stock", "Code", "Criteria", "Rewards"}, trades)
		}
	}

	if len(scan.SystemObjects) > 0 {
		log("System objects")
		var data [][]any
		var odata [][]any
		for _, so := range scan.SystemObjects {
			data = append(data, []any{
				lines([]string{string(so.Designation), string(so.Location)}),
				so.Status, so.ObjectType, so.SizeClass, so.OrbitalDistanceAu,
				so.ImpactTarget, so.ImpactEta.Time(),
				p(so.ImpactLikelihood), so.RequiredStrength,
				so.ActivePlates, p(so.ProgressPct),
				so.CurrentThrustPerHour, wrap(so.Description, 40),
			})
			for ot, ro := range so.Requirements {
				odata = append(odata, []any{
					ot, ro.Complete, ro.Current, ro.Remaining, ro.Required,
				})
			}
		}
		printTable([]string{
			"Designation", "Status", "Type", "Class", "Distance AU",
			"Impact Target", "ETA", "Likelyhood", "Required Strength",
			"Active Plates", "Progress", "Thrust/hr", "Description"}, data)
		printTable([]string{
			"Type", "Complete", "Current", "Remaining", "Required"}, odata)
	}

	if err := scan.Cache(); err != nil {
		log("Error updating cache for %s: %v", scan.Star.Designation, err)
	}
	return nil
}

func init() {
	replicantCmd.AddCommand(replicantScanCmd)
}
