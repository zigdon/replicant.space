package cmd

import (
	"cmp"
	"fmt"
	"slices"
	"sync"

	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/models"
	"github.com/zigdon/rsp/rest"
)

// Check that the current ATC directive target is empty (or near empty)
// Find the location that has the most resources that isn't home
// Set new directive

func autoFerry(cmd *cobra.Command, args []string) error {
	atcStr, _ := cmd.Flags().GetString("atc")
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
			return fmt.Errorf("Existing ferry command still has inventory to pick up at %q", cur)
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
	skip := make(map[string]bool)
	for _, af := range afs {
		log("Found autofactory %q in %q", af.Code.Alias(), af.Location)
		skip[af.Location] = true
	}
	for _, sh := range shs {
		log("Found system hub %q in %q", sh.Code.Alias(), sh.Location)
		skip[sh.Location] = false
	}

	var dests []string
	var cnts sync.Map
	home, _ := cmd.Flags().GetString("home")
	var wg sync.WaitGroup
	for l := range locs.Locations {
		if l == home || skip[l] {
			continue
		}
		wg.Go(func() {
			loc, err := rest.Location(l)
			if err != nil {
				log("Error getting location %q: %v", l, err)
				return
			}
			var c float32
			for _, r := range loc.Inventory {
				c += r.Quantity
			}
			if c > 0 {
				log("... %s: %.0f", l, c)
				dests = append(dests, l)
				cnts.Store(l, c)
			}
		})
	}
	wg.Wait()
	slices.SortFunc(dests, func(a, b string) int {
		ca, _ := cnts.Load(a)
		cb, _ := cnts.Load(b)
		return cmp.Compare(cb.(float32), ca.(float32))
	})
	loot, _ := cnts.Load(dests[0])
	log("Resource pile found at %s: %.0f", dests[0], loot)

	return setDirective(atc.Code, "ferry", map[string]any{
		"collect": dests[0],
		"deliver": home,
	})
}
