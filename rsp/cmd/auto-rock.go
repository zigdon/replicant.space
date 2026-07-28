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
		if sh.Status != "relaying" {
			continue
		}
		logs, err := rest.DeviceLogs(sh.Code, true, 0, 50)
		if err != nil {
			errs = append(errs, fmt.Errorf("Error getting logs from %q: %v", sh.Code.Alias(), err))
			continue
		}
		for _, e := range logs.Events {
			if e.EventType != "system_object_detected" {
				continue
			}
			log("%s: %s", sh.Code.Alias(), e.Message)
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

	var data [][]string
	for _, o := range objs {
		prettyPrint(o)
		data = append(data, []string{
			string(o.Designation), o.SizeClass, t(o.ImpactEta.Time()), o.Status,
			d(o.ActivePlates), f(o.CurrentThrustPerHour),
			f(o.RequiredStrength), p(o.ImpactLikelihood),
		})
	}

	printTable([]string{
		"Designation", "Size", "ETA", "Status", "Active Plates", "Current TPH",
		"Requirered PTH", "Impact Likelyhood",
	}, data)

	return nil
}
