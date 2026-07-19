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
	ships := make(map[string]*models.Device)
	for _, cf := range atc.ControlledDevices {
		info, err := getInfo(cf.Code)
		if err != nil {
			log("Error getting info for %q: %v", cf.Code.Alias(), err)
			continue
		}
		if info.Status != "idle" {
			continue
		}
		ships[info.Code.Alias()] = info
	}
	log("%d/%d ships available in rent fleet", len(ships), len(atc.ControlledDevices))

	deliver := func(loc string, inv map[string]int) error {
		// Find a ship
		var ship *models.Device
		for _, cf := range ships {
			if len(cf.Cargo) != 0 {
				continue
			}
			ship = cf
			break
		}
		if ship == nil {
			return fmt.Errorf("%s: Can't find an available ship", loc)
		}

		// Load cargo
		if dryRun {
			log("Would load %v into %s", inv, ship.Code.Alias())
		} else {
			_, err := rest.DeviceCommand[models.CommandResp](ship.Code, "collect_resources",
				map[string]any{"resources": inv})
			if err != nil {
				return fmt.Errorf("Error loading %v into %s: %v", inv, ship.Code.Alias(), err)
			}
		}

		// Ship it
		if dryRun {
			log("Would ship %s to %s", ship.Code.Alias(), loc)
		} else if err := travel(ship.Code, loc); err != nil {
			return err
		}

		// Remove the ship from our available list
		delete(ships, ship.Code.Alias())
		return nil
	}

	res, err := rest.Location(home)
	log("Resources available at home:")
	for _, i := range res.Inventory {
		log("  %s: %s", i.ResourceType, f(i.Quantity))
	}

	// Find our ships that are not at home, deposit their cargo, and call back
	var errs []error
	for _, cf := range atc.ControlledDevices {
		info, err := getInfo(cf.Code)
		if err != nil {
			log("Error getting info for %q: %v", cf.Code.Alias(), err)
			continue
		}
		if string(info.Location) == home {
			continue
		}
		if info.Status != "idle" {
			continue
		}
		if len(info.Cargo) > 0 {
			if dryRun {
				log("Would deposit contents of %s at %s", info.Code.Alias(), info.Location)
			} else if _, err := rest.DeviceCommand[models.CommandResp](info.Code, "deposit_resources", nil); err != nil {
				log("Deposited cargo from %s at %s", info.Code.Alias(), info.Location)
				errs = append(errs, err)
				continue
			}
		}
		if dryRun {
			log("Would ship %s to %s", info.Code.Alias(), home)
		} else {
			errs = append(errs, travel(info.Code, home))
			log("Shiping %s to %s", info.Code.Alias(), home)
		}
	}

	// Check hubs for missing resources, find a ship at home, load it, and send
	// it over
	for _, sh := range hubs {
		if sh.Status != "relaying" {
			log("%s @ %s not online", sh.Code.Alias(), sh.Location)
			continue
		}
		inv, err := rest.Location(string(sh.Location))
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
			log("%s @ %s : need %v", sh.Code.Alias(), sh.Location, missing)
			errs = append(errs, deliver(string(sh.Location), missing))
		} else {
			log("%s @ %s up-to-date", sh.Code.Alias(), sh.Location)
		}
	}
	if err := errors.Join(errs...); err != nil {
		log("Couldn't pay all the rent:\n%v", err)
	}
	return nil
}
