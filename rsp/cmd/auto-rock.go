package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/models"
	"github.com/zigdon/rsp/rest"
)

func autoRock(cmd *cobra.Command, args []string) error {
	shs, err := rest.Devices(map[string]string{"device_type": "system_hub"})
	if err != nil {
		return err
	}

	var errs []error
	var objs []*models.Object
	for _, sh := range shs {
		logs, err := rest.DeviceLogs(sh.Code, true, 0, 50)
		if err != nil {
			errs = append(errs, fmt.Errorf("Error getting logs from %q: %v", sh.Code.Alias(), err))
			continue
		}
		for _, e := range logs.Events {
			if e.EventType != "system_object_detected" {
				continue
			}
			id := e.Payload["object_designation"].(string)
			info, err := rest.Location(id)
			if err != nil {
				errs = append(errs, fmt.Errorf("Can't get info for %q: %v", id, err))
				continue
			}
			if info.Object == nil {
				errs = append(errs, fmt.Errorf("Info for %q does not include object: %v", id, info))
				continue
			}
			objs = append(objs, info.Object)
		}
	}

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

	var data [][]string
	for _, o := range objs {
		var mf string
		if loc := mLocs[string(o.Designation)]; len(loc) > 0 {
			mf = loc[0].Alias()
		}
		if o.Status != "active" && mf == "" && len(pLocs[string(o.Designation)]) == 0 {
			continue
		}
		data = append(data, []string{
			string(o.Designation), o.SizeClass, t(o.ImpactEta.Time()), o.Status,
			d(o.ActivePlates), f(o.CurrentThrustPerHour),
			f(o.RequiredStrength), p(o.ImpactLikelihood),
			d(len(pLocs[string(o.Designation)])), mf,
		})
	}

	printTable([]string{
		"Designation", "Size", "ETA", "Status", "Active Plates", "Current TPH",
		"Requirered PTH", "Impact Likelyhood", "Propulsors", "Carrier",
	}, data)

	return nil
}
