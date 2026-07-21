package cmd

import (
	"cmp"
	"fmt"
	"slices"

	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/models"
	"github.com/zigdon/rsp/rest"
)

// Check that the current ATC directive target is empty (or near empty)
// Find the location that has the most resources that isn't home
// Set new directive

func autoFerry(cmd *cobra.Command, args []string) error {
	atcStr := getString(cmd, "atc")
	atc, err := getInfo(models.NewCodeAlias(atcStr))
	if err != nil {
		return err
	}
	if atc.AmiDirective != nil {
		dir := atc.AmiDirective
		if dir.Name != "" && dir.Name != "ferry" {
			return fmt.Errorf("atc has a %q directive", dir.Name)
		}
		// If we were already ferrying, check if there is cargo left at the origin
		cur, ok := dir.Config["collect"]
		if !ok {
			return fmt.Errorf("Can't find the current ferry location: %v", dir.Config)
		}
		loc, err := rest.Location(cur.(string))
		if err != nil {
			return err
		}
		var total float32
		for _, i := range loc.Inventory {
			total += i.Quantity
		}
		if total > 1000 {
			log("Existing ferry command still has inventory to pick up at %q", cur)
			return nil
		}
	}

	// Pick the location to clean out
	locs, err := rest.Location("")
	if err != nil {
		return err
	}

	// Skip locations that have autofactories and not system-hubs
	afs, err := rest.Devices(map[string]string{"device_type": "autofactory"})
	if err != nil {
		return err
	}
	shs, err := rest.Devices(map[string]string{"device_type": "system_hub"})
	if err != nil {
		return err
	}
	skip := make(map[models.LocationID]bool)
	for _, af := range afs {
		log("Found autofactory %q in %q", af.Code.Alias(), af.Location)
		skip[af.Location] = true
	}
	for _, sh := range shs {
		log("Found system hub %q in %q", sh.Code.Alias(), sh.Location)
		skip[sh.Location] = false
	}

	type dest struct {
		location string
		count    int
	}
	var dests []dest
	home := getString(cmd, "home")
	// Get the distribution of resources at home so we can prioritize accordingly
	priorities, err := getResourcePriorities(home)
	for l, loc := range locs.Locations {
		if string(l) == home || skip[l] {
			continue
		}
		if loc.Resources > 0 {
			log("... %s: %d", l, loc.Resources)
			dests = append(dests, dest{
				location: string(l),
				count:    loc.Resources,
			})
		}
	}
	slices.SortFunc(dests, func(a, b dest) int {
		return cmp.Compare(b.count, a.count)
	})
	log("Resource pile found at %s: %d", dests[0].location, dests[0].count)
	log("Priorities: %v", priorities)

	return setDirective(atc.Code, "ferry", map[string]any{
		"collect":  dests[0].location,
		"deliver":  home,
		"priority": priorities,
	})
}

func getResourcePriorities(loc string) ([]string, error) {
	var res []string
	inv, err := rest.Location(loc)
	if err != nil {
		return res, err
	}
	slices.SortFunc(inv.Inventory, func(a, b *models.Inventory) int {
		return cmp.Compare(a.Quantity, b.Quantity)
	})
	for _, i := range inv.Inventory {
		res = append(res, i.ResourceType)
	}
	return res, nil
}
