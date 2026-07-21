package cmd

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/cache"
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
	evs, err := rest.Events()
	if err != nil {
		return err
	}
	var ev *models.Event
	var data [][]string
	for _, e := range evs.Events {
		data = append(data, []string{
			e.Designation, e.Title, string(e.Location),
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

	moveReplicant := func() error {
		log("Event ready to complete...")
		rid := getInt(cmd, "replicant")
		rep, err := rest.ReplicantID(rid)
		if err != nil {
			return err
		}
		r, err := rest.Replicant(rep)
		if err != nil {
			return err
		}
		if r.CurrentLocation == ev.Location {
			log("Completing event with %s...", r.Code.Alias())
			return eventComplete(eID)
		}
		if r.CurrentLocation.Star() == ev.Location.Star() {
			log("Moving %s to %s...", r.Code.Alias(), ev.Location)
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
		log("Attempting to teleport %s to %s", rep.Alias(), dests[0].StowedDevices.Devices[0].Code.Alias())
		res, err := rest.ReplicantTeleport(rep, dests[0].StowedDevices.Devices[0].Code)
		if err != nil {
			return err
		}
		log("Replicant teleported: eta %s (%s)", res.Completes.Time(), time.Until(res.Completes.Time()))
		return nil
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
	data = [][]string{}
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
		data = append(data, []string{
			d(n + 1), v(op.Resources), v(op.Devices),
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
	missing := make(map[string]int)
	deliver := func() error {
		log("Finding available freighters...")
		cfs, err := rest.Devices(map[string]string{"device_type": "cargo_freighter"})
		if err != nil {
			return err
		}
		enRoute := make(map[string]int)
		isEnRoute := func(d *models.Device) bool {
			if d.Travel == nil {
				return d.Location == ev.Location
			} else {
				return d.Travel.Destination == ev.Location
			}
		}
		var eta time.Time
		var freeCFs, freeSPs []*models.Device
		for _, cf := range cfs {
			cf, err := rest.DeviceInfo(cf.Code)
			if err != nil {
				return err
			}
			if string(cf.Location) == home && len(cf.Cargo) == 0 {
				freeCFs = append(freeCFs, cf)
				continue
			}
			if !isEnRoute(cf) {
				continue
			}
			if cf.Location != ev.Location {
				log("%s is on the way to %s", cf.Code.Alias(), ev.Location)
				for _, c := range cf.Cargo {
					log("... %.0f x %s", c.Quantity, c.ResourceType)
					enRoute[c.ResourceType] += int(c.Quantity)
					if _, ok := missing[c.ResourceType]; ok && cf.Travel != nil && cf.Travel.Arrives.Time().After(eta) {
						eta = cf.Travel.Arrives.Time()
					}
				}
			} else {
				if len(cf.Cargo) > 0 {
					log("Unloading cargo from %s: %v", cf.Code.Alias(), cf.Cargo)
					_, err := rest.DeviceCommand[models.CommandResp](cf.Code, "deposit_resources", nil)
					if err != nil {
						return err
					}
					for _, c := range cf.Cargo {
						missing[c.ResourceType] -= int(c.Quantity)
					}
				}
				log("Sending %s back home", cf.Code.Alias())
				if _, err := travel(cf.Code, home); err != nil {
					return err
				}
			}
		}

		log("Finding available platforms...")
		for _, t := range []string{"surge_plate", "surge_platform", "mobile_fleet"} {
			sps, err := rest.Devices(map[string]string{"device_type": t})
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
				if !isEnRoute(sp) {
					continue
				}
				if sp.Location == ev.Location {
					log("Detaching %d devices from %s", len(sp.AttachedDevices), sp.Code.Alias())
					if _, err := rest.DeviceCommand[models.CommandResp](sp.Code, "detach", nil); err != nil {
						return err
					}
					for _, ad := range sp.AttachedDevices {
						if !modular[ad.Type] || ad.Status != "compressed" {
							continue
						}
						log("Unfurling %s...", ad.Code.Alias())
						res, err := rest.DeviceCommand[models.CommandResp](ad.Code, "unfurl", nil)
						if err != nil {
							return err
						}
						log("... %s", res.Completes.Time())
					}
				}
				for _, ad := range sp.AttachedDevices {
					enRoute[ad.Type] += 1
					if sp.Travel == nil {
						continue
					}
					if _, ok := missing[ad.Type]; ok && sp.Travel.Arrives.Time().After(eta) {
						if sp.Travel.Arrives.Time().After(eta) {
							eta = sp.Travel.Arrives.Time()
						}
					}
				}
			}
		}

		var needRes, needDev bool
		for k, v := range missing {
			if v-enRoute[k] < 0 {
				log("%d x %s already en-route", enRoute[k], k)
				delete(missing, k)
				continue
			}
			if isResource(k) {
				needRes = true
			} else {
				needDev = true
			}
		}
		if len(missing) == 0 {
			log("All required resources are already en-route, ETA %s (%s)", eta, time.Until(eta))
			return nil
		}

		if needRes {
			// Find an empty cf at home, use it
			if len(freeCFs) == 0 {
				return fmt.Errorf("No freighters available to deliver %v to %s", missing, ev.Location)
			}
			for _, cf := range freeCFs {
				if len(missing) == 0 {
					break
				}
				avail := cf.CargoCapacity
				log("%d available on %s", avail, cf.Code.Alias())
				if avail <= 0 {
					continue
				}
				get := make(map[string]int)
				for k, v := range missing {
					if v <= 0 {
						log("All %s got", k)
						delete(missing, k)
						continue
					}
					if !isResource(k) {
						log("%s is not a resource", k)
						continue
					}
					if v <= avail {
						get[k] = v
						delete(missing, k)
						log("%d x %s to be picked up", v, k)
						avail -= v
					} else {
						get[k] = avail
						missing[k] -= avail
						avail = 0
						log("%d x %s to be picked up", avail, k)
						break
					}
				}
				if len(get) == 0 {
					break
				}
				_, err := rest.DeviceCommand[models.CommandResp](cf.Code, "collect_resources", map[string]any{
					"resources": get,
				})
				if err != nil {
					return err
				}
				newEta, err := travel(cf.Code, string(ev.Location))
				if err != nil {
					return err
				}
				if newEta.After(eta) {
					eta = newEta
				}
			}
		}

		if needDev {
			printers, err := getHomeFactories(home)
			if err != nil {
				return err
			}

			// See if we have the devices we need
			var pickUp []*models.Device
			queue := make(map[string]time.Duration)
			for k, v := range missing {
				if isResource(k) {
					continue
				}
				log("Searching for %d new %ss", v, k)
				devs, err := rest.Devices(map[string]string{
					"location":    home,
					"device_type": k,
				})
				if err != nil {
					return err
				}
				log("%s: want %d, found %d %s", k, v, len(devs), devList(devs))
				if len(devs) >= v {
					log("Have enough %s: %d", k, len(devs))
					pickUp = append(pickUp, devs[:v]...)
					delete(missing, k)
				} else {
					log("Picking up %d %ss", len(devs), k)
					bp := getBP(k)
					pickUp = append(pickUp, devs...)
					missing[k] -= len(devs)

					log("Still missing %d %ss", missing[k], k)
					for missing[k] > 0 {
						p, err := rest.FindPrinter(printers, queue)
						if err != nil {
							return err
						}
						log("Printing %s at %s", k, p.Alias())
						_, err = rest.DeviceCommand[models.CommandResp](
							p, "enqueue_print", map[string]any{"device_type": k})
						queue[p.String()] += bp.PrintTime.Duration()
						missing[k]--
					}
					delete(missing, k)
				}
			}
			log("missing: %v", missing)
			log("pick up: %v", devList(pickUp))

			// Find an empty platform at home, use it
			if len(freeSPs) == 0 {
				return fmt.Errorf("No platforms available to deliver %v to %s", missing, ev.Location)
			}
			slices.SortFunc(freeSPs, func(a, b *models.Device) int {
				return cmp.Compare(a.AttachCapacity, b.AttachCapacity)
			})
			log("available platforms: %v", devList(freeSPs))
			if len(pickUp) == 0 {
				log("Nothing to pick up yet, waiting for print jobs to complete")
				return nil
			}
			var ids []*models.CodeAlias
			for _, d := range pickUp {
				if modular[d.Type] && d.Status == "idle" {
					log("Compacting %s", d.Code.Alias())
					if res, err := rest.DeviceCommand[models.CommandResp](d.Code, "compact", nil); err != nil {
						return err
					} else {
						log("... %s", res.Completes.String())
					}
					continue
				}
				ids = append(ids, d.Code)
			}
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
					_, err := rest.DeviceCommand[models.CommandResp](sp.Code, "attach", map[string]any{"targets": ids})
					if err != nil {
						return err
					}
					newEta, err := travel(sp.Code, string(ev.Location))
					if err != nil {
						return err
					}
					if newEta.After(eta) {
						eta = newEta
					}
					break
				} else {
					log("Attaching %s to %s", strings.Join(devList(pickUp[:avail]), ", "), sp.Code.Alias())
					_, err := rest.DeviceCommand[models.CommandResp](sp.Code, "attach", map[string]any{"targets": ids[:avail]})
					if err != nil {
						return err
					}
					newEta, err := travel(sp.Code, string(ev.Location))
					if err != nil {
						return err
					}
					if newEta.After(eta) {
						eta = newEta
					}
					pickUp = pickUp[avail:]
				}
			}
		}

		if len(missing) == 0 {
			log("Deliveries complete")
			return nil
		}

		return fmt.Errorf("Need to deliver %v to %s", missing, ev.Location)
	}

	data = [][]string{}
	for _, dev := range ep.Devices {
		missing[dev.DeviceType] = dev.Required - dev.Current
		data = append(data, []string{dev.DeviceType, d(dev.Required), d(dev.Current)})
	}
	for _, r := range ep.Resources {
		missing[r.ResourceType] += r.Required - int(r.Current)
		data = append(data, []string{r.ResourceType, d(r.Required), f(r.Current)})
	}
	printTable([]string{"Type", "Required", "Current"}, data)

	data = [][]string{}
	for k, v := range missing {
		if v > 0 {
			data = append(data, []string{k, d(v)})
		} else {
			delete(missing, k)
		}
	}
	slices.SortFunc(data, func(a, b []string) int {
		return cmp.Compare(a[0], b[0])
	})
	if len(data) > 0 {
		log("Missing:")
		printTable([]string{"Resource", "Quantity"}, data)
		return deliver()
	}

	// Requirements all met, get a replicant there
	acc, err := rest.Account()
	if err != nil {
		return err
	}
	for _, r := range acc.ReplicantList {
		if r.Location == ev.Location {
			return eventComplete(eID)
		}
	}

	if err := moveReplicant(); err != nil {
		return err
	}

	return nil
}
