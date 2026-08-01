package cmd

import (
	"fmt"
	"slices"

	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/common"
	"github.com/zigdon/rsp/models"
	"github.com/zigdon/rsp/rest"
)

func autoRock(cmd *cobra.Command, args []string) error {
	objs, err := common.GetRocks()
	if err != nil {
		return err
	}

	fmt.Println()

	getLocs := func(t string) (map[string][]*models.CodeAlias, error) {
		ps, err := rest.Devices(map[string]string{"device_type": t})
		if err != nil {
			return nil, err
		}
		locs := make(map[string][]*models.CodeAlias)
		for _, p := range ps {
			locs[string(p.Location)] = append(locs[string(p.Location)], p.Code)
		}
		return locs, err
	}
	pLocs, err := getLocs("propulsor")
	if err != nil {
		return err
	}
	mLocs, err := getLocs("mobile_fleet")
	if err != nil {
		return err
	}

	var data [][]any
	var done []string
	var next []string
	for _, o := range objs {
		var mf string
		if loc := mLocs[string(o.Designation)]; len(loc) > 0 {
			mf = loc[0].Alias()
		}
		if o.Status != "active" {
			if mf != "" {
				// Keep track of fleets that are ready to relocate
				done = append(done, mf)
			} else if len(pLocs[string(o.Designation)]) == 0 {
				// Skip objects that are done and we've moved away from
				continue
			}
		} else if mf == "" {
			// Collect list of rocks that are in need of a fleet
			next = append(next, string(o.Designation))
		}
		data = append(data, []any{
			o.Designation, o.SizeClass, o.ImpactEta.Time(), o.Status,
			o.ActivePlates, o.CurrentThrustPerHour,
			o.RequiredStrength, p(o.ImpactLikelihood),
			len(pLocs[string(o.Designation)]), mf,
		})
	}

	printTable([]string{
		"Designation", "Size", "ETA", "Status", "Active Plates", "Current TPH",
		"Requirered PTH", "Impact Likelyhood", "Propulsors", "Carrier",
	}, data)

	if len(done) > 0 {
		data = [][]any{}
		slices.Sort(next)
		slices.Sort(done)
		for _, l := range next {
			line := []any{l}
			for _, mf := range done {
				dist, err := common.Distance(mf, l)
				if err != nil {
					return err
				}
				line = append(line, dist)
			}
			data = append(data, line)
		}
		headers := []string{"Designation"}
		headers = append(headers, done...)
		printTable(headers, data)
	}

	return nil
}
