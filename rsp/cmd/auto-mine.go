package cmd

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/cache"
	"github.com/zigdon/rsp/models"
	"github.com/zigdon/rsp/rest"
)

// Automatically set up persistent belt mining site
// ami mining + mining drone
// ami scanning + scanning drone
// ftl relay
// Tag with mine-SYSTEM-BELT-1
// Build missing devices
// Deliver built devices
// Adopt drones to ami
// Set ami policy

func autoMine(cmd *cobra.Command, args []string) error {
	getStar := func(loc string) (string, bool) {
		star, _, ok := strings.Cut(loc, "-")
		return star, ok
	}
	// Validate the location
	locName, _ := cmd.Flags().GetString("location")
	loc, err := rest.Location(locName)
	if err != nil {
		return err
	}
	star, ok := getStar(locName)
	if !ok {
		return fmt.Errorf("Can't figure out the star from %q", locName)
	}
	log("Destination system: %s", star)

	// Define the desired fleet shape
	missing := map[string]int{
		"ami_mining_controller": 1,
		"ami_survey_controller": 1,
		"service_bot":           1,
		"mining_drone":          3,
		"belt_surveyor":         1,
	}
	skip, _ := cmd.Flags().GetStringSlice("skip")
	for _, sk := range skip {
		log("Skipping %q", sk)
		delete(missing, sk)
	}
	type statLine struct {
		found, idle, extra int
	}
	stats := make(map[string]*statLine)
	for k := range missing {
		stats[k] = new(statLine)
	}

	// Get printer locations
	home, _ := cmd.Flags().GetString("home")
	locs := make(map[models.LocationID]bool)
	printerStrs, _ := cmd.Flags().GetStringSlice("factory")
	var printers []*models.CodeAlias
	if len(printerStrs) == 0 {
		// Just get all the home factories
		facts, err := rest.Devices(map[string]string{
			"location":    home,
			"device_type": "autofactory",
		})
		if err != nil {
			return err
		}
		for _, f := range facts {
			if slices.Contains([]string{"compacted", "unfurling", "compacting"}, f.Status) {
				continue
			}
			printerStrs = append(printerStrs, f.Code.Alias())
		}
	}
	if len(printerStrs) == 0 {
		return fmt.Errorf("No autofactories found")
	}
	var pAliases []string
	for _, f := range printerStrs {
		dev, err := getInfo(models.NewCodeAlias(f))
		if err != nil {
			return err
		}
		locs[dev.Location] = true
		printers = append(printers, dev.Code)
		pAliases = append(pAliases, dev.Code.Alias())
	}
	log("Printers found: %v", pAliases)

	// Get the existing or idle fleet
	fleet := make(map[string][]*models.Device)
	tag := fmt.Sprintf("mine-%s", strings.ToLower(string(loc.Location)))
	tagged, err := rest.GetTagged(tag)

	// Find what is missing
	amis := make(map[string]*models.CodeAlias)
	for _, d := range tagged.Devices {
		t := d.Type
		if _, ok := stats[t]; !ok {
			stats[t] = new(statLine)
		}
		stats[t].found += 1
		if strings.Contains(t, "ami") {
			amis[t] = d.Code
		}
		if m := missing[t]; m <= 0 {
			if t == "maintenance_drone" && missing["service_bot"] > 0 {
				missing["service_bot"]--
				stats["service_bot"].idle++
				fleet["service_bot"] = append(fleet["service_bot"], d)
				continue
			}
			stats[t].extra += 1
			log("Found a spare tagged %s: %s", t, d.Code.Alias())
			continue
		}

		missing[t] -= 1
		fleet[t] = append(fleet[t], d)
	}

	// See if we can repurpose idle devices
	devs, err := rest.Devices(nil)
	if err != nil {
		return err
	}
	for _, d := range devs {
		// Special case for relays - if there's one working in the system, we
		// don't need another.
		t := d.Type
		if t == "ftl_relay" && d.Location.Star() == star && missing[t] > 0 {
			log("Found a relay already in system: %q", d.Code.Alias())
			missing[t] = 0
			fleet[t] = append(fleet[t], d)
			continue
		}

		if slices.ContainsFunc(d.Tags, func(t string) bool {
			return strings.HasPrefix(t, "mine-")
		}) {
			continue
		}
		if _, ok := locs[d.Location]; !ok {
			continue
		}
		if d.Status != "idle" && d.Status != "inactive" {
			continue
		}
		if d.ControllerDeviceCode != nil {
			continue
		}
		if m := missing[t]; m > 0 {
			stats[t].idle += 1
			missing[t] -= 1
			fleet[t] = append(fleet[t], d)
			log("Tagging idle %s (%s)", t, d.Code.String())
			_, err := rest.UpdateTags(d.Code, rest.AddTag, []string{tag})
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

	dryRun, _ := cmd.Flags().GetBool("dry_run")

	// Enqueue a build
	buildTimes := make(map[string]time.Duration)
	for t := range missing {
		row, err := db.GetVal(cache.BlueprintsTable, "print_time", t)
		if err != nil {
			return fmt.Errorf("Can't get cached blueprint for %q: %v", t, err)
		}
		var secs float32
		row(&secs)
		bt, err := time.ParseDuration(fmt.Sprintf("%.0fs", secs))
		if err != nil {
			return err
		}
		buildTimes[t] = bt
	}

	extra := make(map[string]time.Duration)
	data = [][]string{}
	var done time.Time
	if noPrint, _ := cmd.Flags().GetBool("no_print"); !dryRun && !noPrint {
		for devType, qty := range missing {
			for qty > 0 {
				factory, err := rest.FindPrinter(printers, extra)
				if err != nil {
					return fmt.Errorf("No available factory found to queue %s: %v", devType, err)
				}
				cfg := map[string]any{
					"device_type": devType,
					"tags":        []string{tag},
				}
				if t, ok := strings.CutSuffix(devType, "_drone"); ok {
					if c, ok := amis[fmt.Sprintf("ami_%s_controller", t)]; ok {
						cfg["controller"] = c.String()
					}
				} else if devType == "belt_surveyor" {
					if c, ok := amis["ami_survey_controller"]; ok {
						cfg["controller"] = c.String()
					}
				}
				log("Printing %q at %q...", devType, factory.Alias())
				res, err := rest.DeviceCommand[models.CommandResp](factory, "enqueue_print", cfg)
				if err != nil {
					return err
				}
				extra[factory.String()] += buildTimes[devType]
				data = append(data, []string{
					factory.Alias(), devType, res.Status, d(res.QueueLength + 1),
				})
				qty -= 1
				if fi, err := getInfo(factory); err == nil {
					if eta, err := rest.GetPrintQueueETA(fi); err == nil {
						qt := time.Now().Add(eta).Add(extra[factory.String()])
						if qt.After(done) {
							done = qt
						}
					}
				}
			}

		}
	} else if len(missing) > 0 {
		var skip []string
		for k, v := range missing {
			if v == 0 {
				continue
			}
			skip = append(skip, fmt.Sprintf("%d %s", v, k))
		}
		if len(skip) > 0 {
			log("Skipping printing missing devices: %s", list(skip))
		}
	}

	if !done.IsZero() {
		log("Print queue ETA: %s (in %s)", done.Format(time.Stamp), time.Until(done).Truncate(time.Second))
		n := &models.Notification{
			Start:  time.Now(),
			End:    done,
			Device: "Mining fleet",
			Text:   fmt.Sprintf("Fleet ready for %q", locName),
		}
		n.Save()
	}

	if len(data) > 0 {
		log("Waiting for missing devices:")
		printTable([]string{
			"Factory", "Type", "Status", "Queue Posititon",
		}, data)
		return nil
	}

	// Check if the fleet needs transport
	var dest string
	var needPicked []string
	for ty, ds := range fleet {
		if ty == "ftl_relay" {
			if len(ds) == 0 {
				log("No FTL relays to transport")
				continue
			}
			d := ds[0]
			dStar := d.Location.Star()
			if star != dStar {
				needPicked = append(needPicked, d.Code.Alias())
				dest = string(d.Location)
			}

			continue
		}
		for _, d := range ds {
			if d.Location == "" {
				continue
			}
			dStar := d.Location.Star()
			if dStar == star {
				continue
			}
			if dest != "" && string(d.Location) != dest {
				continue
			}
			if d.Status != "idle" && d.Status != "inactive" {
				continue
			}
			needPicked = append(needPicked, d.Code.Alias())
			if dest == "" {
				dest = string(d.Location)
			}
		}
	}

	// Find an available fleet carrier. If none available, send the nearest one home.
	allMFs, err := rest.Devices(map[string]string{"device_type": "mobile_fleet"})
	if err != nil {
		return err
	}
	if len(allMFs) == 0 {
		return fmt.Errorf("No fleet carriers found")
	}
	var carrier *models.Device
	detached := make(map[string]bool)
	detachAll := func(ca *models.CodeAlias) error {
		if detached[ca.Alias()] {
			return nil
		}
		log("Attempting to detach devices from %q", ca.Alias())
		detached[ca.Alias()] = true
		info, err := getInfo(ca)
		if err != nil {
			return err
		}
		if len(info.AttachedDevices) == 0 {
			return nil
		}
		if info.Status != "idle" {
			return fmt.Errorf("%s not idle: %s", info.Code.Alias(), info.Status)
		}
		var devs []string
		for _, d := range info.AttachedDevices {
			devs = append(devs, d.Code.Alias())
		}
		if len(devs) == 0 {
			return nil
		}
		log("Detaching %v from %s", devs, info.Code.Alias())
		_, err = rest.DeviceCommand[models.CommandResp](info.Code, "detach", map[string]any{"targets": devs})
		return err
	}
	if dest != "" {
		log("Need to transport %v", needPicked)
		// Skip fleets that are not home, or have attached devices
		for _, mf := range allMFs {
			if string(mf.Location) != home {
				continue
			}
			if len(mf.AttachedDevices) > 0 {
				continue
			}
			carrier = mf
			break
		}
		if carrier == nil {
			return fmt.Errorf("No available fleet found")
		}
		if !dryRun {
			// Detach anything connected to the carrier, if it isn't in motion
			if carrier.Status != "idle" {
				log("Carrier %s is not idle (%s)", carrier.Code.Alias(), carrier.Status)
				return nil
			}
			if err := detachAll(carrier.Code); err != nil {
				return err
			}

			if string(carrier.Location) != dest {
				if carrier.Travel != nil {
					log("%s already in transit to %q, ETA %s",
						carrier.Code.Alias(), carrier.Travel.Destination, carrier.Travel.Arrives.String())
					return nil
				}
				log("Sending %s to %s", carrier.Code.Alias(), dest)
				res, err := rest.DeviceCommand[models.CommandResp](carrier.Code, "travel", map[string]any{"destination": dest})
				if err != nil {
					if !strings.Contains(err.Error(), "Already at destination") {
						return err
					}
				}
				log("Carrier in transit, eta %s", res.TotalTime.String())
				n := &models.Notification{
					Start:  time.Now(),
					End:    res.Arrives.Time(),
					Device: "Mining fleet",
					Text:   fmt.Sprintf("Fleet arrived at %q", locName),
				}
				n.Save()
				return nil
			}

			// Attach any devices that need to ship
			log("Attaching %v to %s", needPicked, carrier.Code.Alias())
			_, err := rest.DeviceCommand[models.CommandResp](carrier.Code, "attach", map[string]any{
				"targets": needPicked,
			})
			if err != nil {
				return err
			}

			// Ship em
			res, err := rest.DeviceCommand[models.CommandResp](carrier.Code, "travel", map[string]any{"destination": locName})
			if err != nil {
				return err
			}
			log("Carrier in transit, eta %s", res.TotalTime.String())
			return nil
		}
	}
	if dryRun {
		return nil
	}

	// Set up the directives:
	// fr: go to the star entry point
	// amc: gather_smallest
	// asc: search belt
	// mtd: patrol
	s, err := rest.Location(star)

	// Find the devices
	frs, ok := fleet["ftl_relay"]
	if !ok || len(frs) == 0 {
		log("Skipping ftl relay")
	}
	amc, ok := amis["ami_mining_controller"]
	if !ok {
		return fmt.Errorf("Can't find amc")
	}
	asc, ok := amis["ami_survey_controller"]
	if !ok {
		return fmt.Errorf("Can't find asc")
	}
	sbs, ok := fleet["service_bot"]
	if !ok || len(sbs) == 0 {
		return fmt.Errorf("Can't find mtd")
	}
	sd := sbs[0]

	// Issue travel commands
	var errs []error
	carriers := make(map[string]*models.Device)
	if carrier != nil {
		carriers[carrier.Code.Alias()] = carrier
	}
	for _, ds := range fleet {
		for _, d := range ds {
			if d.AttachedToDeviceCode != nil {
				i, err := getInfo(d.AttachedToDeviceCode)
				if err != nil {
					errs = append(errs, err)
					continue
				}
				carriers[d.AttachedToDeviceCode.Alias()] = i
			}
		}
	}
	log("Carriers: %v", carriers)
	for _, c := range carriers {
		if c.Status != "idle" {
			return fmt.Errorf("Carrier %s is not idle (%s)", c.Code.Alias(), c.Status)
		}
		if err := detachAll(c.Code); err != nil {
			return err
		}
		c.AttachedDevices = nil
		log("Carrier: %s at %s", c.Code.Alias(), c.Location)
		if c.Location.Star() == star {
			// If the fleet is at the destination, send it home
			res, err := rest.DeviceCommand[models.CommandResp](c.Code, "travel", map[string]any{"destination": home})
			if err != nil {
				return err
			}
			log("Fleet returning to %q, eta %s", home, res.TotalTime.String())
			c.Location = ""
		}
	}

	var fr *models.Device
	if len(frs) > 0 {
		fr = frs[0]
		if fr.Location != s.EntryPoint {
			if err = travel(fr.Code, string(s.EntryPoint)); err != nil {
				errs = append(errs, err)
			}
		}
	}
	for _, d := range []*models.CodeAlias{amc, asc, sd.Code} {
		if err := travel(d, locName); err != nil {
			errs = append(errs, err)
		}
	}
	err = errors.Join(errs...)
	if err != nil {
		return err
	}

	if fr != nil {
		if fr.Location == s.EntryPoint {
			if _, err := rest.DeviceCommand[models.CommandResp](fr.Code, "activate", nil); err != nil {
				if !strings.Contains(err.Error(), "Relay is already active") {
					errs = append(errs, fmt.Errorf("Error activating relay at %s: %v", s.EntryPoint, err))
				}
			}
		} else {
			log("Waiting for FTL relay %s to reach entry point %s", fr.Code.Alias(), s.EntryPoint)
		}
	}

	if err := setDirective(amc, "deplete_smallest", nil); err != nil {
		errs = append(errs, err)
	}
	var mds []*models.CodeAlias
	for _, d := range fleet["mining_drone"] {
		if d.ControllerDeviceCode == nil {
			mds = append(mds, d.Code)
			continue
		}
		if d.ControllerDeviceCode.String() != amc.String() {
			errs = append(errs,
				fmt.Errorf("%s is assigned to the wrong controller %s", d.Code.Alias(), d.ControllerDeviceCode.Alias()))
		}
	}
	if len(mds) > 0 {
		errs = append(errs, adopt(amc, mds))
	}

	if err := setDirective(asc, "belt_search", nil); err != nil {
		errs = append(errs, err)
	}
	var sds []*models.CodeAlias
	for _, d := range fleet["belt_surveyor"] {
		if d.ControllerDeviceCode == nil {
			sds = append(sds, d.Code)
			continue
		}
		if d.ControllerDeviceCode.String() != asc.String() {
			errs = append(errs,
				fmt.Errorf("%s is assigned to the wrong controller %s", d.Code.Alias(), d.ControllerDeviceCode.Alias()))
		}
	}
	if len(sds) > 0 {
		errs = append(errs, adopt(asc, sds))
	}

	if err := setDirective(sd.Code, "service", nil); err != nil {
		errs = append(errs, err)
	}

	return errors.Join(errs...)
}
