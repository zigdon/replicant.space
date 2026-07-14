package cmd

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/models"
	"github.com/zigdon/rsp/rest"
)

func autoRent(cmd *cobra.Command, args []string) error {
	hubs, err := rest.Devices(map[string]string{
		"device_type": "system_hub",
	})
	if err != nil {
		return err
	}
	atcStr, _ := cmd.Flags().GetString("atc")
	home, _ := cmd.Flags().GetString("home")
	dryRun, _ := cmd.Flags().GetBool("dry_run")
	atc, err := getInfo(models.NewCodeAlias(atcStr))
	if err != nil {
		return err
	}
	deliver := func(loc string, inv map[string]int) error {
		// Find a ship
		var ship *models.Device
		for _, cf := range atc.ControlledDevices {
			info, err := getInfo(cf.Code)
			if err != nil {
				log("Error getting info for %q: %v", cf.Code.Alias(), err)
				continue
			}
			if info.Location != home {
				continue
			}
			if info.Status != "idle" {
				continue
			}
			if len(info.Cargo) != 0 {
				continue
			}
			ship = info
			break
		}
		if ship == nil {
			return fmt.Errorf("Can't find an available ship")
		}

		// Load cargo
		if dryRun {
			log("Would load %v into %s", inv, ship.Code.Alias())
		} else {
			_, err := rest.DeviceCommand[models.CommandResp](ship.Code, "collect_resources",
				map[string]any{"resources": inv})
			if err != nil {
				return err
			}
		}

		// Ship it
		if dryRun {
			log("Would ship %s to %s", ship.Code.Alias(), loc)
			return nil
		} else {
			return travel(ship.Code, loc)
		}
	}

	// Find our ships that are not at home, deposit their cargo, and call back
	var errs []error
	for _, cf := range atc.ControlledDevices {
		info, err := getInfo(cf.Code)
		if err != nil {
			log("Error getting info for %q: %v", cf.Code.Alias(), err)
			continue
		}
		if info.Location == home {
			continue
		}
		if info.Status != "idle" {
			continue
		}
		if len(info.Cargo) > 0 {
			if dryRun {
				log("Would deposit contents of %s at %s", info.Code.Alias(), info.Location)
			} else if _, err := rest.DeviceCommand[models.CommandResp](info.Code, "deposit_resources", nil); err != nil {
				errs = append(errs, err)
				continue
			}
		}
		if dryRun {
			log("Would ship %s to %s", info.Code.Alias(), home)
		} else {
			errs = append(errs, travel(info.Code, home))
		}
	}

	// Check hubs for missing resources, find a ship at home, load it, and send
	// it over
	for _, sh := range hubs {
		if sh.Status != "relaying" {
			continue
		}
		inv, err := rest.Location(sh.Location)
		if err != nil {
			errs = append(errs, fmt.Errorf("Can't get resources at %s: %v", sh.Location, err))
			continue
		}
		res := make(map[string]int)
		for _, i := range inv.Inventory {
			res[i.ResourceType] = int(i.Quantity)
		}
		for _, r := range sh.UpkeepRequirements {
			res[r.ResourceType] -= r.QuantityPer20pct
		}
		missing := make(map[string]int)
		for k, v := range res {
			if v >= 0 {
				continue
			}
			missing[k] -= v
		}
		if len(missing) > 0 {
			errs = append(errs, deliver(sh.Location, missing))
		}
	}
	return errors.Join(errs...)
}
