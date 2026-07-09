package cmd

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/spf13/cobra"
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
	locName, _ := cmd.Flags().GetString("location")
	loc, err := rest.Location(locName)
	if err != nil {
		return err
	}
	star := locName[:strings.Index(locName, "-")]
	log("Destination system: %s", star)

	// Define the desired fleet shape
	missing := map[string]int{
		"ami_mining_controller": 1,
		"ami_survey_controller": 1,
		"service_bot":           1,
		"mining_drone":          3,
		"belt_surveyor":         1,
		"ftl_relay":             1,
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
	locs := make(map[string]bool)
	printerStrs, _ := cmd.Flags().GetStringSlice("factory")
	var printers []*models.CodeAlias
	if len(printerStrs) == 0 {
		// Just get all the home factories
		home, _ := cmd.Flags().GetString("home")
		facts, err := rest.Devices(map[string]string{
			"location":    home,
			"device_type": "autofactory",
		})
		if err != nil {
			return err
		}
		for _, f := range facts {
			if f.Status == "compacted" {
				continue
			}
			printerStrs = append(printerStrs, f.Code.Alias())
		}
	}
	for _, f := range printerStrs {
		dev, err := getInfo(models.NewCodeAlias(f))
		if err != nil {
			return err
		}
		locs[dev.Location] = true
		printers = append(printers, dev.Code)
	}
	log("Printers found: %v", printers)

	// Get the existing or idle fleet
	fleet := make(map[string][]*models.Device)
	tag := fmt.Sprintf("mine-%s", strings.ToLower(loc.Location))
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
			log("ami found: %s -> %s", t, d.Code.String())
		}
		if m := missing[t]; m <= 0 {
			if t == "maintenance_drone" && missing["service_bot"] > 0 {
				missing["service_bot"]--
				stats["service_bot"].idle++
				fleet["service_bot"] = append(fleet["service_bot"], d)
				continue
			}
			stats[t].extra += 1
			log("Found a spare tagged %s: %s", t, d.Code.String())
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
		if t == "ftl_relay" && strings.HasPrefix(d.Location, star) && missing[t] > 0 {
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
	data = [][]string{}
	extra := make(map[string]time.Duration)
	if noPrint, _ := cmd.Flags().GetBool("no_print"); !dryRun && !noPrint {
		for devType, qty := range missing {
			for qty > 0 {
				factory, err := findPrinter(printers, extra)
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
				bp := &models.Blueprint{DeviceType: devType}
				if err := bp.Get(); err != nil {
					log("Couldn't find blueprint for %q: %v", devType, err)
				} else {
					extra[factory.String()] += bp.PrintTime.Duration()
				}
				data = append(data, []string{
					factory.Alias(), devType, res.Status, d(res.QueueLength + 1),
				})
				qty -= 1
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
			dStar := locName[:strings.Index(d.Location, "-")]
			if star != dStar {
				needPicked = append(needPicked, d.Code.Alias())
				dest = d.Location
			}

			continue
		}
		for _, d := range ds {
			dStar := d.Location[:strings.Index(d.Location, "-")]
			if dStar == star {
				log("%s is at destination star %q: %s", d.Code.Alias(), star, d.Location)
				continue
			}
			if dest != "" && d.Location != dest {
				log("%s is not at the pickup location %q: %s", d.Code.Alias(), dest, d.Location)
				continue
			}
			if d.Status != "idle" && d.Status != "inactive" {
				log("%s is not ready to pickup", d.Code.Alias())
				continue
			}
			needPicked = append(needPicked, d.Code.Alias())
			log("%s needs pickup at %q", d.Code.Alias(), d.Location)
			if dest == "" {
				dest = d.Location
			}
		}
	}

	// If there isn't an assigned afc, we're done
	afcStr, _ := cmd.Flags().GetString("fleet")
	var afcInfo *models.Device
	var afc *models.CodeAlias
	if afcStr != "" {
		afc = models.NewCodeAlias(afcStr)
		afcInfo, err = getInfo(afc)
		if err != nil {
			return err
		}
	}
	detachAll := func(afc *models.CodeAlias) error {
		afcInfo, err := getInfo(afc)
		if err != nil {
			return err
		}
		platforms := afcInfo.ControlledDevices
		for _, p := range platforms {
			info, err := getInfo(p.Code)
			if err != nil {
				return err
			}
			if len(info.AttachedDevices) == 0 {
				continue
			}
			if info.Status != "idle" {
				return fmt.Errorf("%s not idle: %s", info.Code.Alias(), info.Status)
			}
			var devs []string
			for _, d := range info.AttachedDevices {
				devs = append(devs, d.Code.Alias())
			}
			if len(devs) == 0 {
				continue
			}
			log("Detaching %v from %s", devs, info.Code.Alias())
			_, err = rest.DeviceCommand[models.CommandResp](info.Code, "detach", map[string]any{"targets": devs})
			if err != nil {
				return err
			}
		}
		return nil
	}
	if dest != "" {
		if afcStr == "" {
			log("No assigned AFC, will not ship ")
			return nil
		}
		log("Need to transport %v", needPicked)

		if !dryRun {
			// Detach anything connected to the afc, if it isn't in motion
			if err := detachAll(afc); err != nil {
				return err
			}

			// Use larger platforms before smaller ones
			platforms := afcInfo.ControlledDevices
			slices.SortFunc(platforms, func(a, b *models.ControlledDevice) int {
				ca, _ := getInfo(a.Code)
				cb, _ := getInfo(b.Code)
				return cmp.Compare(cb.AttachCapacity, ca.AttachCapacity)
			})

			i, err := getInfo(afc)
			if err != nil {
				return err
			}
			if i.Location != dest {
				log("Sending %s to %s", afc.Alias(), dest)
				res, err := rest.DeviceCommand[models.CommandResp](afc, "travel", map[string]any{"destination": dest})
				if err != nil {
					if !strings.Contains(err.Error(), "Already at destination") {
						return err
					}
				}
				log("Fleet in transit, eta %s", res.TotalTime.String())
				return nil
			}
			var needAssemble bool
			for _, d := range i.ControlledDevices {
				di, err := getInfo(d.Code)
				if err != nil {
					return err
				}
				if di.Location != dest {
					needAssemble = true
					break
				}
			}
			if needAssemble {
				log("Aseembling fleet at %s", dest)
				_, err = rest.DeviceCommand[models.CommandResp](afc, "assemble", nil)
				return err
			}

			// Attach any devices that need to ship
			for _, p := range platforms {
				info, err := getInfo(p.Code)
				if err != nil {
					return err
				}
				cap := min(info.AttachCapacity-len(info.AttachedDevices), len(needPicked))
				if cap > 0 {
					log("Attaching %v to %s", needPicked[0:cap], p.Code.Alias())
					_, err := rest.DeviceCommand[models.CommandResp](p.Code, "attach", map[string]any{
						"targets": needPicked[0:cap],
					})
					if err != nil {
						return err
					}
					needPicked = needPicked[cap:]
				}
			}

			// Ship em
			res, err := rest.DeviceCommand[models.CommandResp](afc, "travel", map[string]any{"destination": locName})
			if err != nil {
				return err
			}
			log("Fleet in transit, eta %s", res.TotalTime.String())
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
		return fmt.Errorf("Can't find ftl relay")
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
	if err := detachAll(afc); err != nil {
		return err
	}
	fr := frs[0]
	if fr.Location != s.EntryPoint {
		if err = travel(fr.Code, s.EntryPoint); err != nil {
			errs = append(errs, err)
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

	if fr.Location == s.EntryPoint {
		if _, err := rest.DeviceCommand[models.CommandResp](fr.Code, "activate", nil); err != nil {
			if !strings.Contains(err.Error(), "Relay is already active") {
				errs = append(errs, fmt.Errorf("Error activating relay at %s: %v", s.EntryPoint, err))
			}
		}
	} else {
		log("Waiting for FTL relay %s to reach entry point %s", fr.Code.Alias(), s.EntryPoint)
	}

	if err := setDirective(amc, "deplete_smallest", nil); err != nil {
		errs = append(errs, err)
	}

	if err := setDirective(asc, "belt_search", nil); err != nil {
		errs = append(errs, err)
	}

	if err := setDirective(sd.Code, "service", nil); err != nil {
		errs = append(errs, err)
	}

	afcInfo, err = getInfo(afc)
	if afcInfo.Location == locName && len(afcInfo.AttachedDevices) == 0 {
		// If the fleet is at the destination, send it home
		home, _ := cmd.Flags().GetString("home")
		res, err := rest.DeviceCommand[models.CommandResp](afc, "travel", map[string]any{"destination": home})
		if err != nil {
			return err
		}
		log("Fleet returning to %q, eta %s", home, res.TotalTime.String())
	}

	return errors.Join(errs...)
}

func findPrinter(printers []*models.CodeAlias, extra map[string]time.Duration) (*models.CodeAlias, error) {
	// Check the queue for each potential printer. If there is an idle printer,
	// use that. Otherwise, pick the one with the shortest queue, by remaining
	// print time.
	info := make(map[*models.CodeAlias]*models.Device)
	log("Printers:")
	for _, p := range printers {
		i, err := getInfo(p)
		if err != nil {
			return nil, fmt.Errorf("can't get device info for %q: %v", p, err)
		}
		info[p] = i
		log("  %s: %s (%s already queued)", p.Alias(), i.Type, extra[p.String()])
	}

	// Calculate the queue length for each printer
	queue := make(map[*models.CodeAlias]time.Duration)
	for _, p := range printers {
		eta, err := rest.GetPrintQueueETA(info[p])
		if err != nil {
			return nil, fmt.Errorf("error getting print queue for %q: %v", p, err)
		}
		queue[p] = eta + extra[p.String()]
	}
	if len(queue) == 0 {
		return nil, fmt.Errorf("No available printer found")
	}
	slices.SortFunc(printers, func(a, b *models.CodeAlias) int {
		ta, _ := queue[a]
		tb, _ := queue[b]
		return cmp.Compare(ta, tb)
	})
	for _, p := range printers {
		log("%s: %s", p.Alias(), queue[p])
	}

	return printers[0], nil
}
