package cmd

import (
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
	var extras []string
	for _, d := range devs {
		if valid { // We already found a valid network
			extras = append(extras, d.Code.Alias())
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
		extras = append(extras, d.Code.Alias())
	}
	if !valid {
		log("None of the relays at %s are in the home network", locName)
	}
	if len(devs) > 1 {
		log("Found %d extra relays: %s", len(extras), strings.Join(extras, ", "))
		return returnExtraRelays(devs)
	}
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
		if strings.Contains(d.Location, loc) {
			devs = append(devs, d)
		}
	}

	return devs, nil
}
