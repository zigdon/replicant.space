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
	eID, _ := cmd.Flags().GetString("id")
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
		acc, err := rest.Account()
		if err != nil {
			return err
		}
		for _, r := range acc.ReplicantList {
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
		}
		return fmt.Errorf("Need a replicant moved to %s to resolve the event", ev.Location)
	}

	// Load the blueprints we know
	bps := make(map[string]bool)
	blueprints, err := db.ListIDs(cache.BlueprintsTable)
	if err != nil {
		return err
	}
	for _, bp := range blueprints {
		bps[bp.(string)] = true
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
		cid, _ := cmd.Flags().GetInt("criteria")
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
	home, _ := cmd.Flags().GetString("home")
	missing := make(map[string]int)
	deliver := func() error {
		cfs, err := rest.RefreshDevices(map[string]string{"device_type": "cargo_freighter"})
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
			cf, err := rest.RefreshDeviceInfo(cf.Code)
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
				if err := travel(cf.Code, home); err != nil {
					return err
				}
			}
		}

		for _, t := range []string{"surge_plate", "surge_platform", "mobile_fleet"} {
			sps, err := rest.Devices(map[string]string{"device_type": t})
			if err != nil {
				return err
			}
			for _, sp := range sps {
				if string(sp.Location) == home && len(sp.AttachedDevices) == 0 {
					freeSPs = append(freeSPs, sp)
				}
				if !isEnRoute(sp) {
					continue
				}
				for _, c := range sp.Cargo {
					enRoute[c.ResourceType] += int(c.Quantity)
					if _, ok := missing[c.ResourceType]; ok && sp.Travel.Arrives.Time().After(eta) {
						eta = sp.Travel.Arrives.Time()
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
				err = travel(cf.Code, string(ev.Location))
				if err != nil {
					return err
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
			var slots int
			queue := make(map[string]time.Duration)
			for k, v := range missing {
				if isResource(k) {
					continue
				}
				devs, err := rest.Devices(map[string]string{
					"location":    string(ev.Location),
					"device_type": k,
				})
				if err != nil {
					return err
				}
				if len(devs) > 0 {
					log("Found %d x %s already at %s", len(devs), k, ev.Location)
					missing[k] -= len(devs)
					if missing[k] <= 0 {
						continue
					}
				}
				bp := getBP(k)
				devs, err = rest.Devices(map[string]string{
					"location":    home,
					"device_type": k,
				})
				if err != nil {
					return err
				}
				log("devs: %v", devs)
				if len(devs) >= v {
					pickUp = append(pickUp, devs[:v]...)
					slots += v
					delete(missing, k)
				} else {
					pickUp = append(pickUp, devs...)
					slots += len(devs)
					missing[k] -= len(devs)

					p, err := rest.FindPrinter(printers, queue)
					if err != nil {
						return err
					}
					// _, err = rest.DeviceCommand[models.CommandResp](
					// 	p, "enqueue_print", map[string]any{"device_type": k})
					queue[p.Alias()] += bp.PrintTime.Duration()
					log("Printing %s at %s", k, p.Alias())
				}
			}
			log("missing: %v", missing)

			// Find an empty platform at home, use it
			if len(freeSPs) == 0 {
				return fmt.Errorf("No platforms available to deliver %v to %s", missing, ev.Location)
			}
			if len(pickUp) == 0 {
				log("Nothing to pick up yet, waiting for print jobs to complete")
				return nil
			}
			ids := make([]*models.CodeAlias, len(pickUp))
			for n, d := range pickUp {
				ids[n] = d.Code
			}
			for _, sp := range freeSPs {
				if len(pickUp) == 0 {
					break
				}
				avail := sp.AttachCapacity
				if len(pickUp) == 0 {
					break
				}
				if avail >= len(pickUp) {
					log("Attaching %s to %s", strings.Join(devList(pickUp), ", "), sp.Code.Alias())
					_, err := rest.DeviceCommand[models.CommandResp](sp.Code, "attach", map[string]any{"targets": ids})
					if err != nil {
						return err
					}
					err = travel(sp.Code, string(ev.Location))
					if err != nil {
						return err
					}
					break
				} else {
					log("Attaching %s to %s", strings.Join(devList(pickUp[:avail]), ", "), sp.Code.Alias())
					_, err := rest.DeviceCommand[models.CommandResp](sp.Code, "attach", map[string]any{"targets": ids[:avail]})
					if err != nil {
						return err
					}
					err = travel(sp.Code, string(ev.Location))
					if err != nil {
						return err
					}
					pickUp = pickUp[avail:]
				}
			}
		}

		if len(missing) == 0 {
			return nil
		}

		return fmt.Errorf("Need to deliver %v to %s", missing, ev.Location)
	}

	data = [][]string{}
	for _, dev := range ep.Devices {
		missing[dev.DeviceType] = dev.Count - dev.Current
		data = append(data, []string{dev.DeviceType, d(dev.Count), d(dev.Current)})
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
