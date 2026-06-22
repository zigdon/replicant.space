package cmd

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/models"
	"github.com/zigdon/rsp/rest"
)

// " huh, so each site keeps survey/mining controllers, and drones, so keep mining forever?"

// Automatically set up persistent belt mining site
// ami mining + mining drone
// ami scanning + scanning drone
// ftl relay
// Tag with mine-SYSTEM-BELT-1
// Build missing devices
// Deliver built devices
// Adopt drones to ami
// Set ami policy

// Collect resources
// Pick site outside of the allowlist that has the most resources
// Set ami transport to ferry stuff home

var autoCmd = &cobra.Command{
	Use:   "auto",
	Short: "High level automation commands",
}

var autoMineCmd = &cobra.Command{
	Use:   "mine",
	Short: "Set up a new belt mine",
	RunE:  autoMine,
}

func init() {
	rootCmd.AddCommand(autoCmd)

	autoCmd.PersistentFlags().Bool("dry_run", true, "When set, only describe what will be done")
	autoCmd.PersistentFlags().String("owner", "zigdon-2", "Replicant responsible for printing new devices")

	autoCmd.AddCommand(autoMineCmd)
	autoMineCmd.Flags().StringP("location", "l", "", "Belt location to mine")
	autoMineCmd.MarkFlagRequired("location")
	autoMineCmd.Flags().StringSliceP("factory", "f", []string{"a-1"}, "Devices for building new ships")
	autoMineCmd.Flags().BoolP("dry_run", "n", false, "Only plan, don't actually queue prints")
	autoMineCmd.Flags().String("fleet", "", "Fleet controller to use for transportation")
}

func autoMine(cmd *cobra.Command, args []string) error {
	// Validate the location
	locName, _ := cmd.Flags().GetString("location")
	loc, err := rest.Location(locName)
	if err != nil {
		return err
	}

	// Define the desired fleet shape
	missing := map[string]int{
		"ami_mining_controller": 1,
		"ami_survey_controller": 1,
		"maintenance_drone":     1,
		"mining_drone":          3,
		"survey_drone":          2,
		"ftl_relay":             1,
	}
	type statLine struct{
		found, idle, extra int
	}
	stats := make(map[string]*statLine)
	for k := range missing {
		stats[k] = new(statLine)
	}

	// Get printer locations
	locs := make(map[string]bool)
	printers, _ := cmd.Flags().GetStringSlice("factory")
	for _, f := range printers {
		dev, err := rest.DeviceInfo(f)
		if err != nil {
			return err
		}
		locs[dev.Location] = true
	}
	log("Printers found: %v", locs)

	// Get the existing or idle fleet
	fleet := make(map[string][]*models.Device)
	tag := fmt.Sprintf("mine-%s", strings.ToLower(loc.Location))
	tagged, err := rest.GetTagged(tag)

	// Find what is missing
	amis := make(map[string]string)
	for _, d := range tagged.Devices {
		t := d.Type
		stats[t].found += 1
		if strings.Contains(t, "ami") {
			amis[t] = d.Code.String()
			log("ami found: %s -> %s", t, d.Code.String())
		}
		if m := missing[t]; m <= 0 {
			stats[t].extra += 1
			log("Found a spare tagged %s: %s", t, d.Code.String())
			continue
		}

		missing[t] -= 1
		fleet[t] = append(fleet[t], d)
	}

	// See if we can repurpose idle devices
	devs, err := rest.AllDevices() 
	if err != nil {
		return err
	}
	for _, d := range devs {
		if slices.Contains(d.Tags, tag) {
			continue
		}
		if _, ok := locs[d.Location]; !ok {
			continue
		}
		if d.Status != "idle" && d.Status != "inactive" {
			continue
		}
		t := d.Type
		if m := missing[t]; m > 0 {
			stats[t].idle += 1
			missing[t] -= 1
			fleet[t] = append(fleet[t], d)
			log("Tagging idle %s (%s)", t, d.Code.String())
			_, err := rest.UpdateTags(d.Code.String(), rest.AddTag, []string{tag})
			if err != nil {
				return err
			}
		}
	}

	var types []string
	for t := range missing {
		types = append(types, t)
	}
	slices.Sort(types)

	var data [][]string
	for _, t := range types {
		var f []string
		for _, d := range fleet[t] {
			f = append(f, d.Code.Alias())
		}
		slices.Sort(f)
		data = append(data, []string{
			t, d(missing[t] + len(fleet[t])), d(len(fleet[t])), d(stats[t].idle), d(missing[t]), d(stats[t].extra), list(f),
		})
	}
	printTable([]string{"Device", "Target", "Found", "Repurposed", "Missing", "Extra", "Members"}, data)

	if dr, _ := cmd.Flags().GetBool("dry_run"); dr {
		return nil
	}

	// Enqueue a build
	data = [][]string{}
	for devType, qty := range missing {
		if qty <= 0 {
			continue
		}
		factory, err := findPrinter(printers)
		if err != nil {
			return fmt.Errorf("No available factory found to queue %s: %v", devType, err)
		}
		cfg := map[string]any{
			"device_type": devType,
		}
		if t, ok := strings.CutSuffix(devType, "_drone"); ok {
			if c, ok := amis[fmt.Sprintf("ami_%s_controller", t)]; ok {
				cfg["controller"] = c
			}
		}
		log("Printing %q at %q...", devType, factory)
		res, err := rest.DeviceCommand(factory, "enqueue_print", cfg)
		if err != nil {
			return err
		}
		data = append(data, []string{
			factory, devType, res.Status, d(res.QueueLength+1),
		})
	}

	if len(data) > 0 {
		log("Waiting for missing devices:")
		printTable([]string{
			"Factory", "Type", "Status", "Queue Posititon",
		}, data)
		return nil
	}

	log("Fleet ready to ship")

	// If there isn't an assigned afc, we're done
	afc, _ := cmd.Flags().GetString("fleet")
	if afc == "" {
		log("No assigned AFC, fleet still needs to be transported")
		return nil
	}

	// If the afc is at the destination, unattach all the things
	afcInfo, err := rest.DeviceInfo(afc)
	if err != nil {
		return err
	}
	platforms := afcInfo.ControlledDevices
	for _, p := range platforms {
		info, err := rest.DeviceInfo(p.Code.String())
		if err != nil {
			return err
		}
		var devs []string
		for _, d := range info.AttachedDevices {
			devs = append(devs, d.Code.String())
		}
		if len(devs) == 0 {
			continue
		}
		log("Detaching %v from %s", devs, info.Code.Alias())
		_, err = rest.DeviceCommand(info.Code.String(), "detach", map[string]any{"targets": devs})
		if err != nil {
			return err
		}
	}

	// Figure out where we have devices that are not at our destination
	var dest string
	var needPicked []string
	for _, ds := range fleet {
		for _, d := range ds {
			if d.Location == loc.Location {
				continue
			}
			if dest != "" && d.Location != dest {
				continue
			}
			if d.Status != "idle" && d.Status != "inactive" {
				continue
			}
			needPicked = append(needPicked, d.Code.String())
			if dest == "" {
				dest = d.Location
			}
		}
	}
	if dest == "" {
		log("Can't identify pickup location")
		return nil
	}

	log("Sending %s to %s", afc, dest)
	res, err := rest.DeviceCommand(afc, "travel", map[string]any{"destination": dest})
	if err != nil && !strings.Contains(err.Error(), "Already at destination") {
		return err
	}
	if err == nil {
		log("Fleet in transit, eta %s", res.TotalTime.String())
		return nil
	}

	// Attach any devices that need to ship
	for _, p := range platforms {
		info, err := rest.DeviceInfo(p.Code.String())
		if err != nil {
			return err
		}
		cap := min(info.AttachCapacity - len(info.AttachedDevices), len(needPicked))
		if cap > 0 {
			log("Attaching %v to %s", needPicked[0:cap], p.Code.Alias())
			_, err := rest.DeviceCommand(p.Code.String(), "attach", map[string]any{
				"targets": needPicked[0:cap],
			})
			if err != nil {
				return err
			}
			needPicked = needPicked[cap:]
		}
	}

	// Ship em
	res, err = rest.DeviceCommand(afc, "travel", map[string]any{"destination": locName})
	if err != nil {
		return err
	}

	return nil
}

func findPrinter(printers []string) (string, error) {
	// Check the queue for each potential printer. If there is an idle printer,
	// use that. Otherwise, pick the one with the shortest queue, by remaining
	// print time.
	info := make(map[string]*models.Device)
	log("Printers:")
	for _, p := range printers {
		i, err := rest.DeviceInfo(p)
		if err != nil {
			return "", fmt.Errorf("can't get device info for %q: %v", p, err)
		}
		info[p] = i
		log("  %s: %s", p, i.Type)
	}

	// Calculate the queue length for each printer
	queue := make(map[string]time.Duration)
	for _, p := range printers {
		eta, err := rest.GetPrintQueueETA(info[p])
		if err != nil {
			return "", fmt.Errorf("error getting print queue for %q: %v", p, err)
		}
		queue[p] = eta
	}
	if len(queue) == 0 {
		return "", fmt.Errorf("No available printer found")
	}
	slices.SortFunc(printers, func(a, b string) int {
		ta, _ := queue[a]
		tb, _ := queue[b]
		return cmp.Compare(ta, tb)
	})

	return printers[0], nil
}
