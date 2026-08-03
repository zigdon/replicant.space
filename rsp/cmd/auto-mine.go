package cmd

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/common"
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
	// Validate the location
	locName := getString(cmd, "location")
	loc, err := rest.Location(locName)
	if err != nil {
		return err
	}
	var density string
	if loc.AsteroidBelt != nil {
		density = loc.AsteroidBelt.Belts[0].Density
	} else if loc.Belt != nil {
		density = loc.Belt.Density
	} else {
		return fmt.Errorf("Can't get the density of %q", locName)
	}
	star := loc.Location.Star()
	log("Destination system: %s (%s)", star, density)

	mdScale := map[string]int{
		"sparse":   1,
		"moderate": 4,
		"dense":    10,
	}
	sdScale := map[string]int{
		"sparse":   1,
		"moderate": 2,
		"dense":    5,
	}
	md, mok := mdScale[density]
	sd, sok := sdScale[density]
	if !mok || !sok {
		return fmt.Errorf("Unknown density %q, can't figure out scale", density)
	}
	log("Density: %s (x %d mining, %d survey)", density, md, sd)

	// Define the desired fleet shape
	missing := map[string]int{
		"ami_mining_controller": 1,
		"ami_survey_controller": 1,
		"service_bot":           1,
		"mining_drone":          5,
		"belt_surveyor":         2,
	}
	missing["mining_drone"] *= md
	missing["belt_surveyor"] *= sd
	log("Using %d mining drones", missing["mining_drone"])
	log("Using %d survey drones", missing["belt_surveyor"])

	skip := getStringSlice(cmd, "skip")
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
	home := getString(cmd, "home")
	printerStrs := getStringSlice(cmd, "factory")
	var printers []*models.CodeAlias
	if len(printerStrs) == 0 {
		printers, err = common.GetFilteredDevices(
			[]string{"autofactory"},
			[]string{home},
			[]string{"idle", "printing"},
		)
		if err != nil {
			return err
		}
	} else {
		for _, p := range printerStrs {
			dev, err := getInfo(models.NewCodeAlias(p))
			if err != nil {
				return err
			}
			printers = append(printers, dev.Code)
		}
	}
	if len(printers) == 0 {
		return fmt.Errorf("No autofactories found")
	}

	// See if there are any devices already in the print ueues
	fleet := make(map[string][]*models.Device)
	tag := fmt.Sprintf("mine-%s", strings.ToLower(string(loc.Location)))
	var pAliases []string
	var printPending bool
	for _, p := range printers {
		dev, err := getInfo(p)
		if err != nil {
			return err
		}
		pAliases = append(pAliases, dev.Code.Alias())
		if dev.Printing != nil {
			if slices.Contains(dev.Printing.Tags, tag) {
				log("Found %s being printed at %s", dev.Printing.DeviceType, p)
				missing[dev.Printing.DeviceType]--
				printPending = true
			}
		}
		for _, d := range dev.PrintQueue {
			if slices.Contains(d.Tags, tag) {
				log("Found %s in the %s queue", d.Type, p)
				missing[dev.Printing.DeviceType]--
				printPending = true
			}
		}
	}
	log("Found %d printers", len(pAliases))

	tagged, err := rest.GetTagged(tag)
	if err != nil {
		return err
	}
	log("Found %d devices tagged %q: %v", len(tagged.Devices), tag, tagged.Devices)

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
	var devs []*models.Device
	log("Searching for idle devices...")
	for k := range missing {
		log("... %s", k)
		ds, err := rest.Devices(map[string]string{"device_type": k, "location": home})
		if err != nil {
			return err
		}
		devs = append(devs, ds...)
		ds, err = rest.Devices(map[string]string{"device_type": k, "location": loc.Location.Star()})
		if err != nil {
			return err
		}
		devs = append(devs, ds...)
	}

	// Get the existing or idle fleet
	dryRun := getBool(cmd, "dry_run")
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
			if !dryRun {
				log("Tagging idle %s (%s)", t, d.Code.String())
				_, err := rest.UpdateTags(d.Code, rest.AddTag, []string{tag})
				if err != nil {
					return err
				}
			}
		}
	}

	var types []string
	for t := range missing {
		types = append(types, t)
	}
	slices.Sort(types)

	var data [][]any
	for _, t := range types {
		var f []string
		for _, d := range fleet[t] {
			f = append(f, d.Code.Alias())
		}
		slices.Sort(f)
		data = append(data, []any{
			t, missing[t] + len(fleet[t]),
			len(fleet[t]),
			stats[t].idle,
			missing[t],
			stats[t].extra,
			list(f),
		})
	}
	printTable([]string{"Device", "Target", "Found", "Repurposed", "Missing", "Extra", "Members"}, data)

	// Enqueue a build
	extra := make(map[string]time.Duration)
	data = [][]any{}
	var done time.Time
	if noPrint := getBool(cmd, "no_print"); !dryRun && !noPrint {
		for devType, qty := range missing {
			for qty > 0 {
				factory, err := common.FindPrinter(printers, extra)
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
				if !dryRun {
					res, err := rest.DeviceCommand[models.CommandResp](factory, "enqueue_print", cfg)
					if err != nil {
						return err
					}
					data = append(data, []any{
						factory, devType, res.Status, res.QueueLength + 1,
					})
				}
				extra[factory.String()] += common.GetBP(devType).PrintTime.Duration()
				qty -= 1
				if fi, err := getInfo(factory); err == nil {
					eta := common.GetPrintQueueETA(fi)
					qt := time.Now().Add(eta).Add(extra[factory.String()])
					if qt.After(done) {
						done = qt
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
		log("Print queue ETA: %s (in %s)", done.Format(time.Stamp), time.Until(done))
		n := &models.Notification{
			Start:  time.Now(),
			End:    done,
			Device: "Mining fleet",
			Text:   fmt.Sprintf("Fleet ready for %q", locName),
			Object: fleet,
		}
		n.Save()
	}

	if _, err := db.DB.Exec("UPDATE belts SET mining=true WHERE designation=$1", locName); err != nil {
		log("Error updating the belts table: %v", err)
	}
	if len(data) > 0 {
		log("Waiting for missing devices:")
		printTable([]string{
			"Factory", "Type", "Status", "Queue Posititon",
		}, data)
		return nil
	}

	if printPending {
		log("Waiting for devices enqueued earlier")
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
	var freeCarriers []*models.Device
	detachAll := func(ca *models.CodeAlias) error {
		log("Detaching devices from %s", ca)
		_, err = rest.DeviceCommand[models.CommandResp](ca, "detach", nil)
		return err
	}
	if dest != "" {
		log("Need to transport %v", needPicked)
		// Skip fleet carrierss that are not home, or have attached devices
		for _, mf := range allMFs {
			if string(mf.Location) != home {
				continue
			}
			if len(mf.AttachedDevices) > 0 {
				continue
			}
			freeCarriers = append(freeCarriers, mf)
		}
		if len(freeCarriers) == 0 {
			return fmt.Errorf("No available fleets found")
		}
		if !dryRun {
			for _, carrier := range freeCarriers {
				// Detach anything connected to the carrier, if it isn't in motion
				if carrier.Status != "idle" {
					log("Carrier %s is not idle (%s)", carrier.Code.Alias(), carrier.Status)
					continue
				}
				if err := detachAll(carrier.Code); err != nil {
					return err
				}

				if string(carrier.Location) != dest {
					if carrier.Travel != nil {
						log("%s already in transit to %q, ETA %s",
							carrier.Code.Alias(), carrier.Travel.Destination,
							carrier.Travel.Arrives.String())
						continue
					}
					log("Sending %s to %s", carrier.Code.Alias(), dest)
					eta, err := common.Travel(carrier.Code, dest, false)
					if err != nil {
						return err
					}
					log("Carrier in transit, eta %s (%s)", eta, time.Until(eta))
					n := &models.Notification{
						Start:  time.Now(),
						End:    eta,
						Device: "Mining fleet",
						Text:   fmt.Sprintf("Fleet arrived at %q", locName),
						Object: fleet,
					}
					n.Save()
					continue
				}

				// Attach any devices that need to ship
				if len(needPicked) > carrier.AttachCapacity {
					subset := needPicked[:carrier.AttachCapacity]
					log("Attaching %v to %s", subset, carrier.Code.Alias())
					_, err = rest.DeviceCommand[models.CommandResp](carrier.Code, "attach", map[string]any{
						"targets": subset,
					})
					needPicked = needPicked[carrier.AttachCapacity:]
					log("%d remain to be shipped: %v", len(needPicked), needPicked)
				} else {
					log("Attaching %v to %s", needPicked, carrier.Code.Alias())
					_, err = rest.DeviceCommand[models.CommandResp](carrier.Code, "attach", map[string]any{
						"targets": needPicked,
					})
					needPicked = needPicked[:0]
				}
				if err != nil {
					return err
				}

				// Ship em
				eta, err := common.Travel(carrier.Code, locName, false)
				if err != nil {
					return err
				}
				log("Carrier in transit, eta %s (%s)", eta, time.Until(eta))

				if len(needPicked) == 0 {
					return nil
				}
			}
			return fmt.Errorf("Not enough carriers available, %d devices left to ship: %v",
				len(needPicked), needPicked)
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

	// Issue travel commands
	var errs []error
	carriers := make(map[string]*models.Device)
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
			if _, err = travel(fr.Code, string(s.EntryPoint)); err != nil {
				errs = append(errs, err)
			}
		}
	}
	for _, d := range []*models.CodeAlias{amc, asc} {
		if _, err := travel(d, locName); err != nil {
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

	if err := setDirective(sbs[0].Code, "service", nil); err != nil {
		errs = append(errs, err)
	}

	return errors.Join(errs...)
}
