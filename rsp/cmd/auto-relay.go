package cmd

import (
	"fmt"
	"slices"
	"strings"

	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/models"
	"github.com/zigdon/rsp/rest"
)

// Check if the destination already has a working relay
// - If yes, check that it's in the home network
// - If it isn't, plot the relay path from home, and rerun with the next step
//   that is missing a relay.
// If there wasn't a working relay, find an idle one (or print one)
// Transport it to the destination system
// Activate it

func autoRelay(cmd *cobra.Command, args []string) error {
	locName, _ := cmd.Flags().GetString("location")
	home, _ := cmd.Flags().GetString("home")
	devs, err := getFTLRelays(locName)
	if err != nil {
		return err
	}
	if len(devs) == 0 {
		return missingRelay(locName)
	}
	var valid bool
	var extras []*models.Device
	for _, d := range devs {
		if valid { // We already found a relay on the home network, just clean up extras
			extras = append(extras, d)
			continue
		}
		net, err := rest.DeviceNetwork(d.Code)
		if err != nil {
			return err
		}
		for _, n := range net.Connections {
			if n.Star == home {
				valid = true
				break
			}
		}
		if valid {
			continue
		}
		extras = append(extras, d)
	}
	if !valid {
		log("None of the relays at %s are in the home network", locName)
		return activateRelay(locName, devs[0])
	}
	if len(extras) > 0 {
		var es []string
		for _, e := range extras {
			es = append(es, e.Code.Alias())
		}
		log("Found %d extra relays: %s", len(extras), strings.Join(es, ", "))
		return returnExtraRelays(devs)
	}
	return nil
}

func autoFR(cmd *cobra.Command, args []string) error {
	// Simple version:
	// - check we have an FR
	// - check if there's a working FR in the system
	// - If not, move to the L4 point
	// - Deploy
	// - Activate
	// - Tag
	rID, _ := cmd.Flags().GetInt("replicant")
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
		return travel(r.HostedDeviceCode, string(s.EntryPoint))
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

func activateRelay(loc string, d *models.Device) error {
	log("Position and activate a relay (%s) in %s", d.Code.Alias(), loc)
	return nil
}

func missingRelay(loc string) error {
	log("Need to find/make a relay for %s", loc)
	return nil
}

func returnExtraRelays(devs []*models.Device) error {
	log("Return/recycle %d extra relays", len(devs))
	return nil
}

func getFTLRelays(loc string) ([]*models.Device, error) {
	ds, err := rest.Devices(nil)
	var devs []*models.Device
	if err != nil {
		return nil, err
	}
	for _, d := range ds {
		if d.Type != "ftl_relay" {
			continue
		}
		if d.Location.Star() == loc {
			devs = append(devs, d)
		}
	}

	return devs, nil
}
