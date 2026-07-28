package cmd

import (
	"fmt"
	"slices"
	"time"

	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/common"
	"github.com/zigdon/rsp/models"
	"github.com/zigdon/rsp/rest"
)

func autoFR(cmd *cobra.Command, args []string) error {
	// Simple version:
	// - check we have an FR
	// - check if there's a working FR in the system
	// - If not, move to the L4 point
	// - Deploy
	// - Activate
	// - Tag
	rID := getInt(cmd, "replicant")
	r, err := rest.Replicant(models.NewCodeAlias(fmt.Sprintf("r-%d", rID)))
	if err != nil {
		return err
	}
	var fr *models.Device
	if !slices.ContainsFunc(r.StowedDevices, func(d *models.Device) bool {
		if d.Type == "ftl_relay" {
			fr = d
			return true
		}
		return false
	}) {
		return fmt.Errorf("No FTL Relay found stowed in r-%d's ship", rID)
	}
	starName := r.Location.Star()
	if starName == "" {
		return fmt.Errorf("r-%d is not in a system: %s", rID, r.Location)
	}
	devs, err := rest.Devices(map[string]string{"location": starName})
	if err != nil {
		return err
	}
	if slices.ContainsFunc(devs, func(d *models.Device) bool {
		return d.Type == "ftl_relay" && d.Status == "relaying"
	}) {
		log("There is already a relaying FTL Relay in %s", starName)
		return nil
	}
	s, err := models.NewStar(starName)
	if err != nil {
		return fmt.Errorf("Can't load star %s: %v", starName, err)
	}
	if s.EntryPoint == "" {
		return fmt.Errorf("Unknown entry point for %s", starName)
	}
	if r.Location != s.EntryPoint {
		_, err := travel(r.HostedDeviceCode, string(s.EntryPoint))
		return err
	}
	if _, err = rest.DeviceCommand[models.CommandResp](fr.Code, "deploy", nil); err != nil {
		return err
	}
	log("Deployed %s to %s", fr.Code.Alias(), s.EntryPoint)
	if _, err = rest.DeviceCommand[models.CommandResp](fr.Code, "activate", nil); err != nil {
		return err
	}
	log("Activated %s", fr.Code.Alias())
	if _, err = rest.UpdateTags(fr.Code, rest.AddTag, []string{"infrastructure"}); err != nil {
		return err
	}
	log("Tagged %s", fr.Code.Alias())
	return nil
}

func autoFB(cmd *cobra.Command, _ []string) error {
	// Simple version:
	// - check we have an FB
	// - check if there's a working FB in place
	// - If not, move to the destination planet
	// - Deploy
	// - Tag
	rID := getInt(cmd, "replicant")
	r, err := rest.Replicant(models.NewCodeAlias(fmt.Sprintf("r-%d", rID)))
	if err != nil {
		return err
	}
	var fb *models.Device
	if !slices.ContainsFunc(r.StowedDevices, func(d *models.Device) bool {
		if d.Type == "ftl_beacon" {
			fb = d
			return true
		}
		return false
	}) {
		return fmt.Errorf("No FTL Beacon found stowed in r-%d's ship", rID)
	}
	planet := models.LocationID(getString(cmd, "planet"))
	devs, err := rest.Devices(map[string]string{"location": planet.Star()})
	if err != nil {
		return err
	}
	if slices.ContainsFunc(devs, func(d *models.Device) bool {
		return d.Type == "ftl_beacon" && d.Status == "monitoring" && d.Location == planet
	}) {
		log("There is already an FTL Beacon near %s", planet)
		return nil
	}
	if r.Location == planet {
		if _, err = rest.DeviceCommand[models.CommandResp](fb.Code, "deploy", nil); err != nil {
			return err
		}
		log("Deployed %s to %s", fb.Code.Alias(), planet)
		if _, err = rest.UpdateTags(fb.Code, rest.AddTag, []string{"infrastructure"}); err != nil {
			return err
		}
		log("Tagged %s", fb.Code.Alias())
	} else {
		eta, err := common.Travel(r.Code, string(planet), false)
		if err != nil {
			return err
		}
		log("Moving to %s, ETA: %s (%s)", planet, eta.Format(time.Stamp), time.Until(eta))
	}
	return nil
}
