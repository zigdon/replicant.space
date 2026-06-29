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
		if total > 100 {
			return fmt.Errorf("Existing ferry command still has inventory to pick up at %q", cur)
		}
	}

	// Pick the location to clean out
	locs, err := rest.Location("")
	if err != nil {
		return err
	}

	var dests []string
	cnts := make(map[string]float32)
	home, _ := cmd.Flags().GetString("home")
	for l := range locs.Locations {
		if l == home {
			continue
		}
		loc, err := rest.Location(l)
		if err != nil {
			log("Error getting location %q: %v", l, err)
			continue
		}
		dests = append(dests, l)
		cnts[l] = 0
		for _, r := range loc.Inventory {
			cnts[l] += r.Quantity
		}
	}
	slices.SortFunc(dests, func(a, b string) int {
		return cmp.Compare(cnts[b], cnts[a])
	})
	log("Resource pile found at %s: %.0f", dests[0], cnts[dests[0]])

	return setDirective(atc.Code, "ferry", map[string]any{
		"collect": dests[0],
		"deliver": home,
	})
}
