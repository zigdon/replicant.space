package cmd

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/cache"
	"github.com/zigdon/rsp/common"
	"github.com/zigdon/rsp/models"
	"github.com/zigdon/rsp/rest"
)

// Look up the requirements for an event
// If there are multiple options
//   check that we have all the required blueprints
//   require the user specify which one to take
// Check which of the requirements are still missing
// If there are missing components
//   check if we already have some available (with either no tag or event:id tag)
//   if not, check if they're already being printed with an 'event:id' tag
//   if not, queue them
//   ship them
// If there are missing resources
//   check if there are already any in transit
//   if not, ship them
// Once everything is in place
//   if there's a replicant there, resolve the event
//   if not, check if there's an ERM, and teleport to it
//   if not, send the nearest replicant there

func autoEvent(cmd *cobra.Command, args []string) error {
	// Load event details
	eID := getString(cmd, "id")
	dryRun := getBool(cmd, "dry_run")
	evs, err := rest.Events()
	if err != nil {
		return err
	}
	var ev *models.Event
	var data [][]any
	for _, e := range evs.Events {
		data = append(data, []any{
			e.Designation, e.Title, e.Location,
		})
		if e.Designation != eID {
			continue
		}
		ev = e
	}
	if ev == nil {
		if len(evs.Events) == 1 {
			ev = evs.Events[0]
			eID = ev.Designation
			log("Selecting event %s", ev.Designation)
		} else {
			eventsDesc := new(strings.Builder)
			printTablef(eventsDesc, []string{"ID", "Title", "Location"}, data)
			return fmt.Errorf("Can't find event ID %q. Pick from:\n%s", eID, eventsDesc.String())
		}
	}

	tag := fmt.Sprintf("event:%s", strings.ToLower(ev.Designation))

	teleportReplicant := func(r *models.Replicant) error {
		if r.CurrentLocation == ev.Location {
			log("Completing event with %s...", r.Code.Alias())
			return eventComplete(eID)
		}
		if r.CurrentLocation.Star() == ev.Location.Star() {
			log("Moving %s to %s...", r.Code.Alias(), ev.Location)
			if dryRun {
				return nil
			}
			_, err := rest.ReplicantTravel(
				r.Code, string(ev.Location), nil, false)
			return err
		}
		log("Searching for teleport targets in %s", ev.Location)
		dests, err := getTeleportDests(string(ev.Location))
		if err != nil {
			return err
		}
		if len(dests) == 0 {
			return fmt.Errorf("No teleport target found at %s", ev.Location)
		}
		log("Attempting to teleport %s to %s", r.Code.Alias(), dests[0].StowedDevices.Devices[0].Code.Alias())
		if dryRun {
			return nil
		}
		res, err := rest.ReplicantTeleport(r.Code, dests[0].StowedDevices.Devices[0].Code)
		if err != nil {
			return err
		}
		log("Replicant teleported, waiting... eta %s (%s)", res.Completes.Time(), time.Until(res.Completes.Time()))
		time.Sleep(time.Until(res.Completes.Time()) + 5*time.Second)
		return nil
	}

	resolveEvent := func() error {
		acc, err := rest.Account()
		if err != nil {
			return err
		}
		for _, r := range acc.ReplicantList {
			if r, err = rest.Replicant(r.Code); err != nil {
				return err
			}
			if r.Status != "stationary" {
				log("%s is not available: %s", r.Code.Alias(), r.Status)
				continue
			}
			if r.Location == ev.Location {
				return eventComplete(eID)
			}
		}
		return fmt.Errorf("No replicant available")
	}

	// Load the blueprints we know
	bps := make(map[string]bool)
	modular := make(map[string]bool)
	if blueprints, err := db.ListIDs(cache.BlueprintsTable); err != nil {
		return err
	} else {
		for _, bp := range blueprints {
			bps[bp.(string)] = true
		}
	}
	if rows, err := db.DB.Query(`
		SELECT blueprint_type
		FROM blueprint_features
		WHERE feature = 'modular'`); err != nil {
		return err
	} else {
		for rows.Next() {
			var t string
			if err := rows.Scan(&t); err != nil {
				return err
			}
			modular[t] = true
		}
		if err := rows.Err(); err != nil {
			return err
		}
	}

	// Examine resolution options
	var eps []*models.EventProgressOption
	data = [][]any{}
	for n, op := range ev.Progress.Options {
		canDo := true
		for _, bp := range op.Devices {
			if !bps[bp.DeviceType] {
				log("Missing blueprint %s for %s", bp.DeviceType, op.Name)
				canDo = false
				break
			}
		}
		if !canDo {
			continue
		}
		data = append(data, []any{
			n + 1, op.Resources, op.Devices,
		})
		eps = append(eps, op)
	}

	// If we have more than one possible way to go about it, make the user pick
	if len(eps) == 0 {
		return fmt.Errorf("No valid paths available")
	}
	var ep *models.EventProgressOption
	if len(eps) > 1 {
		cid := getInt(cmd, "criteria")
		if cid == 0 {
			paths := new(strings.Builder)
			printTablef(paths, []string{"ID", "Resources", "Devices"}, data)
			return fmt.Errorf("Multiple paths available, select one:\n%s", paths)
		}
		ep = eps[cid-1]
	} else {
		ep = eps[0]
	}

	// Check what is already there
	log("Checking inventory at %s", ev.Location)
	home := getString(cmd, "home")
	type missingEnt struct {
		Need       int
		Transiting int
		Printing   int
		Pickup     int
	}
	missing := make(map[string]*missingEnt)
	for _, r := range ep.Resources {
		missing[r.ResourceType] = &missingEnt{Need: r.Required - int(r.Current)}
	}
	for _, d := range ep.Devices {
		missing[d.DeviceType] = &missingEnt{Need: d.Required - d.Current}
	}
	maxEta := func(etas []time.Time) time.Time {
		log("ETAs:")
		for _, eta := range etas {
			log("  %s (%s)", eta, time.Until(eta))
		}
		if len(etas) == 0 {
			return time.Time{}
		}
		max := etas[0]
		for _, e := range etas {
			if e.After(max) {
				max = e
			}
		}
		return max
	}
	queue := make(map[string]time.Duration)
	etas := []time.Time{time.Now()}
	stillMissing := func() (bool, bool) {
		var needRes, needDev bool
		for k, v := range missing {
			if v.Need-v.Transiting-v.Printing <= 0 {
				continue
			}
			if isResource(k) {
				needRes = true
			} else {
				needDev = true
			}
		}
		return needRes, needDev
	}
	deliver := func() error {
		needRes, needDev := stillMissing()
		if !needRes && !needDev {
			log("All required resources are already en-route")
			return nil
		}

		var freeCFs, freeSPs []*models.Device
		if needRes {
			log("Finding available freighters...")
			cfs, err := rest.Devices(map[string]string{"device_type": "cargo_freighter", "location": home})
			if err != nil {
				return err
			}
			for _, cf := range cfs {
				cf, err := rest.DeviceInfo(cf.Code)
				if err != nil {
					return err
				}
				if string(cf.Location) == home && len(cf.Cargo) == 0 {
					freeCFs = append(freeCFs, cf)
					continue
				}
			}
		}

		if needDev {
			log("Finding available platforms...")
			for _, t := range []string{"surge_platform", "mobile_fleet"} {
				sps, err := rest.Devices(map[string]string{"device_type": t, "location": home})
				if err != nil {
					return err
				}
				for _, sp := range sps {
					sp, err := rest.DeviceInfo(sp.Code)
					if err != nil {
						return err
					}
					if string(sp.Location) == home && len(sp.AttachedDevices) == 0 {
						freeSPs = append(freeSPs, sp)
					}
				}
			}
		}

		if needRes {
			// Find an empty cf at home, use it
			if len(freeCFs) == 0 {
				return fmt.Errorf("No freighters available to deliver %v to %s", missing, ev.Location)
			}
			for _, cf := range freeCFs {
				needRes, _ = stillMissing()
				if !needRes {
					break
				}
				avail := cf.CargoCapacity
				log("%d available on %s", avail, cf.Code.Alias())
				if avail <= 0 {
					continue
				}
				get := make(map[string]int)
				for k, v := range missing {
					if v.Need <= 0 {
						log("All %s got", k)
						continue
					}
					if !isResource(k) {
						log("%s is not a resource", k)
						continue
					}
					if v.Need <= avail {
						get[k] = v.Need
						log("%d x %s to be picked up", v.Need, k)
						avail -= v.Need
						missing[k].Need = 0
						missing[k].Transiting += v.Need
					} else {
						get[k] = avail
						log("%d/%d x %s to be picked up", avail, v.Need, k)
						missing[k].Need -= avail
						missing[k].Transiting += avail
						avail = 0
						break
					}
				}
				if len(get) == 0 {
					break
				}
				log("%s collecting resources: %v", cf.Code, get)
				if !dryRun {
					if _, err := rest.DeviceCommand[models.CommandResp](cf.Code, "collect_resources", map[string]any{
						"resources": get,
					}); err != nil {
						return err
					}
				}
				log("%s shipping to %s", cf.Code, ev.Location)
				if !dryRun {
					log("Adding %q tag to %s", tag, cf.Code)
					if _, err := rest.UpdateTags(cf.Code, rest.AddTag, []string{tag}); err != nil {
						return err
					}
					newEta, err := travel(cf.Code, string(ev.Location))
					if err != nil {
						return err
					}
					etas = append(etas, newEta)
				}
			}
		}

		if needDev {
			// Collect the devices already available
			var pickUp []*models.Device
			tagged, err := rest.GetTagged(tag)
			if err != nil {
				return err
			}
			log("Searching for existing devices...")
			for _, d := range tagged.Devices {
				if slices.Contains([]string{"cargo_freighter", "mobile_fleet", "surge_platform"}, d.Type) {
					continue
				}
				log("... %s (%s) at %s", d.Code.Alias(), d.Type, d.Location)
				missing[d.Type].Need--
				if string(d.Location) == home {
					pickUp = append(pickUp, d)
					missing[d.Type].Pickup++
				} else if d.Location != ev.Location {
					missing[d.Type].Transiting++
				}
			}

			// Print any missing devices
			data = [][]any{}
			for k, ent := range missing {
				data = append(data, []any{
					k, ent.Need, ent.Printing, ent.Pickup, ent.Transiting,
				})
				if isResource(k) || ent.Need <= 0 {
					continue
				}
				pPlan, err := common.Print(
					home, k, ent.Need, true, dryRun, map[string]any{"tags": []string{tag}})
				if err != nil {
					return err
				}
				missing[k].Printing += ent.Need
				missing[k].Need = 0
				log("printing: ETA %s (%s)", pPlan.ETA, time.Until(pPlan.ETA))
				for p, plan := range pPlan.Printers {
					log("... %s: %s", p.Alias(), common.CountList(plan.Queued))
				}
			}
			common.PrintTable([]string{"Delivery", "Need", "Printing", "Pickup", "Transit"}, data)

			if len(pickUp) == 0 {
				log("Nothing to pick up yet, waiting for print jobs to complete")
				return nil
			}

			// Find an empty platform at home, use it
			if len(freeSPs) == 0 {
				return fmt.Errorf("No platforms available to deliver %v to %s", missing, ev.Location)
			}
			slices.SortFunc(freeSPs, func(a, b *models.Device) int {
				return cmp.Compare(a.AttachCapacity, b.AttachCapacity)
			})
			log("available platforms: %v", devList(freeSPs))
			var ids []*models.CodeAlias
			for _, d := range pickUp {
				if modular[d.Type] {
					if d.Status == "idle" {
						log("Compacting %s", d.Code)
						if dryRun {
							continue
						}
						if res, err := rest.DeviceCommand[models.CommandResp](d.Code, "compact", nil); err != nil {
							return err
						} else {
							etas = append(etas, res.Completes.Time())
							log("... %s (%s)", res.Completes.Format(), time.Until(res.Completes.Time()))
						}
						continue
					} else if d.Status == "compacting" {
						etas = append(etas, d.Compact.Completes.Time())
						log("Waiting for %s to compact: %s", d.Code, time.Until(d.Compact.Completes.Time()))
						continue
					}
				}
				missing[d.Type].Transiting++
				ids = append(ids, d.Code)
			}
			if len(ids) > 0 {
				for _, sp := range freeSPs {
					if len(pickUp) == 0 {
						break
					}
					avail := sp.AttachCapacity - len(sp.AttachedDevices)
					if avail == 0 {
						continue
					}
					if avail >= len(pickUp) {
						log("Attaching %s to %s", strings.Join(codeList(ids), ", "), sp.Code.Alias())
						if !dryRun {
							_, err := rest.DeviceCommand[models.CommandResp](sp.Code, "attach", map[string]any{"targets": ids})
							if err != nil {
								return err
							}
						}
						log("%s shipping to %s", sp.Code.Alias(), ev.Location)
						if !dryRun {
							newEta, err := travel(sp.Code, string(ev.Location))
							if err != nil {
								return err
							}
							etas = append(etas, newEta)
						}
						break
					} else {
						log("Attaching %s to %s", strings.Join(devList(pickUp[:avail]), ", "), sp.Code.Alias())
						if !dryRun {
							_, err := rest.DeviceCommand[models.CommandResp](sp.Code, "attach", map[string]any{"targets": ids[:avail]})
							if err != nil {
								return err
							}
						}
						log("%s shipping to %s", sp.Code.Alias(), ev.Location)
						if !dryRun {
							newEta, err := travel(sp.Code, string(ev.Location))
							if err != nil {
								return err
							}
							etas = append(etas, newEta)
						}
						pickUp = pickUp[avail:]
					}
				}
			}
		}

		return nil
	}

	data = [][]any{}
	for _, dev := range ep.Devices {
		data = append(data, []any{dev.DeviceType, dev.Required, dev.Current})
	}

	// See what is currently being printed
	needRes, needDev := stillMissing()
	if needDev {
		log("Searching for devices being printed...")
		printers, err := getHomeFactories(home)
		if err != nil {
			return err
		}
		for _, p := range printers {
			info, err := rest.DeviceInfo(p)
			if err != nil {
				return err
			}
			if info.Printing != nil && slices.Contains(info.Printing.Tags, tag) {
				log("... %s is printing %s", p.Alias(), info.Printing.DeviceType)
				etas = append(etas, time.Now().Add(info.Printing.Eta.Duration()))
				queue[p.String()] += info.Printing.Eta.Duration()
				missing[info.Printing.DeviceType].Need--
				missing[info.Printing.DeviceType].Printing++
			}
			for _, pq := range info.PrintQueue {
				if slices.Contains(pq.Tags, tag) {
					log("... %s has %s queued", p.Alias(), pq.Type)
					bp := getBP(pq.Type)
					queue[p.String()] += bp.PrintTime.Duration()
					etas = append(etas, time.Now().Add(queue[p.String()]))
					missing[pq.Type].Need--
					missing[pq.Type].Printing++
				}
			}
		}
	}

	// Check any deliveries on the way
	if needRes || needDev {
		log("Checking for existing deliveries: %s", tag)
		devs, err := rest.GetTagged(tag)
		if err != nil {
			return err
		}
		for _, d := range devs.Devices {
			d, err := rest.DeviceInfo(d.Code)
			if err != nil {
				return err
			}
			log("%s @ %s:", d.Code, d.Location)
			for _, c := range d.Cargo {
				log("... %.0f x %s", c.Quantity, c.ResourceType)
				missing[c.ResourceType].Need -= int(c.Quantity)
				missing[c.ResourceType].Transiting += int(c.Quantity)
			}
			for _, c := range d.AttachedDevices {
				log("... %s", c.Type)
				missing[c.Type].Need -= 1
				missing[c.Type].Transiting += 1
			}
			if d.Location == ev.Location {
				if len(d.Cargo) > 0 {
					log("Unloading %v from %s", d.Cargo, d.Code)
					if !dryRun {
						if _, err := rest.DeviceCommand[models.CommandResp](d.Code, "deposit_resources", nil); err != nil {
							return err
						}
					}
				}
				if len(d.AttachedDevices) > 0 {
					log("Detaching %v from %s", d.AttachedDevices, d.Code)
					if !dryRun {
						if _, err := rest.DeviceCommand[models.CommandResp](d.Code, "detach", nil); err != nil {
							return err
						}
						for _, ad := range d.AttachedDevices {
							if slices.Contains(ad.Features, "modular") {
								log("Unfurling and untagging %s", ad.Code)
								if _, err := rest.DeviceCommand[models.CommandResp](ad.Code, "unfurl", nil); err != nil {
									return err
								}
								if _, err = rest.UpdateTags(d.Code, rest.DelTag, []string{tag}); err != nil {
									return err
								}
							}
						}
					}
				}
				log("Untagging %s", d.Code)
				if !dryRun {
					_, err = rest.UpdateTags(d.Code, rest.DelTag, []string{tag})
					if err != nil {
						return err
					}
				}
				if slices.Contains(d.Features, "surge") {
					eta, err := travel(d.Code, home)
					log("Returning %s home: %s (%s)", d.Code, eta, time.Until(eta))
					if err != nil {
						return err
					}
				}
			} else if string(d.Location) == "" {
				eta := d.Travel.Arrives.Time()
				if len(d.Cargo) > 0 {
					log("%s is still en-route with %v: %s (%s)", d.Code, d.Cargo, eta, time.Until(eta))
				}
				if len(d.AttachedDevices) > 0 {
					log("%s is still en-route with %v: %s (%s)", d.Code, d.AttachedDevices, eta, time.Until(eta))
				}
				etas = append(etas, eta)
			}
		}
	}
	for _, r := range ep.Resources {
		data = append(data, []any{r.ResourceType, r.Required, r.Current})
	}
	printTable([]string{"Type", "Required", "Current"}, data)

	data = [][]any{}
	for k, v := range missing {
		data = append(data, []any{k, v.Need, v.Transiting, v.Printing})
	}
	slices.SortFunc(data, func(a, b []any) int {
		return cmp.Compare(a[0].(string), b[0].(string))
	})
	printTable([]string{"Resource", "Need", "Transiting", "Printing"}, data)
	if err := deliver(); err != nil {
		return err
	}
	eta := maxEta(etas)
	if time.Now().Before(eta) {
		log("Waiting %s (%s)", time.Until(eta), eta)
		return nil
	}

	// Requirements all met, try and resolve the event
	log("Event ready to complete...")
	if err := resolveEvent(); err == nil {
		log("Done.")
		return nil
	}

	// Otherwise, teleport a replicant there
	rid := getInt(cmd, "replicant")
	rep, err := rest.ReplicantID(rid)
	if err != nil {
		return err
	}
	r, err := rest.Replicant(rep)
	if err != nil {
		return err
	}
	if err := teleportReplicant(r); err == nil {
		return eventComplete(eID)
	}

	// Otherwise, see who's nearby
	acc, err := rest.Account()
	if err != nil {
		return err
	}
	data = [][]any{}
	for _, r := range acc.ReplicantList {
		var src, loc string
		if r.Travel == nil {
			loc = r.CurrentLocation.Star()
			src = loc
		} else {
			log("travel: %#v", r.Travel)
			src = r.Travel.Destination.Star()
			loc = fmt.Sprintf("-> (%s) %s", r.Travel.Eta.Duration(), r.Travel.Destination)
		}
		dist, err := common.Distance(src, ev.Location.Star())
		if err != nil {
			data = append(data, []any{r.Code, loc, err.Error()})
		} else {
			data = append(data, []any{r.Code, loc, dist})
		}
	}
	printTable([]string{"Replicant", "Location", "Distance from" + ev.Location.Star()}, data)

	return nil
}
