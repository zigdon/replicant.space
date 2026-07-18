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
	if raw, _ := cmd.Flags().GetBool("raw"); raw {
		prettyPrint(scan)
		return nil
	}
	printTable([]string{
		"Star",
		"Entry",
		"Life Detected",
		"Mining Bonus %",
		"Tags",
	}, [][]string{{
		string(scan.Star.Designation),
		string(scan.EntryPoint),
		b(scan.LifeDetected),
		d(scan.MiningBonusPct),
		list(scan.SystemTags),
	}})
	if scan.AsteroidBelt.Present {
		var belts [][]string
		for _, b := range scan.AsteroidBelt.Belts {
			belts = append(belts, []string{
				string(b.Designation),
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
				string(p.Designation),
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

	var outer [][]string
	if scan.OuterSystem.Oort != nil {
		outer = append(outer,
			[]string{string(scan.OuterSystem.Oort.Designation),
				f(scan.OuterSystem.Oort.DistanceAu)})
	}
	if scan.OuterSystem.Kuiper != nil {
		outer = append(outer,
			[]string{string(scan.OuterSystem.Kuiper.Designation),
				f(scan.OuterSystem.Kuiper.DistanceAu)})
	}
	if len(outer) > 0 {
		printTable([]string{"Outer system", "Distance AU"}, outer)
	}

	if len(scan.ActiveLocationEvents) > 0 {
		var events [][]string
		for _, e := range scan.ActiveLocationEvents {
			events = append(events, []string{
				string(e.Designation), e.Title, e.EventType, string(e.Location), d(e.Tier),
			})
		}
		printTable([]string{"Designation", "Title", "Type", "Location", "Tier"}, events)
	}

	if len(scan.Replicants) > 0 {
		var reps [][]string
		for _, r := range scan.Replicants {
			reps = append(reps, []string{r.Name, r.Code, string(r.Location), r.LastActive.String()})
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
					tr.Name, d(tr.CurrentStock), tr.Code,
					m(tr.Criteria.Resources),
					m(tr.Rewards.Devices),
				})
			}
			printTable([]string{"Name", "Stock", "Code", "Criteria", "Rewards"}, trades)
		}
	}

	if len(scan.SystemObjects) > 0 {
		log("System objects")
		var data [][]string
		var odata [][]string
		for _, so := range scan.SystemObjects {
			data = append(data, []string{
				string(so.Designation), so.Status, so.ObjectType, so.SizeClass, f(so.OrbitalDistanceAu),
				so.ImpactTarget, t(so.ImpactEta.Time()), p(so.ImpactLikelihood), f(so.RequiredStrength),
				d(so.ActivePlates), p(so.ProgressPct), f(so.CurrentThrustPerHour), so.Description,
			})
			for ot, ro := range so.Requirements {
				odata = append(odata, []string{
					ot, b(ro.Complete), d(ro.Current), d(ro.Remaining),
					d(ro.Required),
				})
			}
		}
		printTable([]string{
			"Designation", "Status", "Type", "Class", "Distance AU", "Impact Target",
			"ETA", "Likelyhood", "Required Strength", "Active Plates", "Progress",
			"Thrust/hr", "Description"}, data)
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
