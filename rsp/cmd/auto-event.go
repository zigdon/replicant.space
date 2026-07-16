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
	eID, _ := cmd.Flags().GetString("event_id")
	evs, err := rest.Events()
	if err != nil {
		return err
	}
	var ev *models.Event
	var data [][]string
	for _, e := range evs.Events {
		data = append(data, []string{
			e.Designation, e.Title, e.Location,
		})
		if e.Designation != eID {
			continue
		}
		ev = e
	}
	if ev == nil {
		var eventsDesc *strings.Builder
		printTablef(eventsDesc, []string{"ID", "Title", "Location"}, data)
		return fmt.Errorf("Can't find event ID %q. Pick from:\n%s", eID, eventsDesc.String())
	}

	moveReplicant := func() error {
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
	var ecs []*models.EventCriteria
	data = [][]string{}
	for n, cr := range ev.Criteria {
		canDo := true
		for _, bp := range cr.Devices {
			if !bps[bp.DeviceType] {
				log("Missing blueprint %s for %s", bp.DeviceType, cr.Name)
				canDo = false
				break
			}
		}
		if !canDo {
			continue
		}
		data = append(data, []string{
			d(n + 1), v(cr.Resources), v(cr.Devices),
		})
		ecs = append(ecs, cr)
	}

	// If we have more than one possible way to go about it, make the user pick
	if len(ecs) == 0 {
		return fmt.Errorf("No valid paths available")
	}
	var ec *models.EventCriteria
	if len(ecs) > 1 {
		cid, _ := cmd.Flags().GetInt("criteria")
		if cid == 0 {
			var paths *strings.Builder
			printTablef(paths, []string{"ID", "Resources", "Devices"}, data)
			return fmt.Errorf("Multiple paths available, select one:\n%s", paths)
		}
		ec = ecs[cid-1]
	} else {
		ec = ecs[0]
	}

	// Check what is already there
	home, _ := cmd.Flags().GetString("home")
	missing := make(map[string]int)
	deliver := func() error {
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
			if cf.Location == home && len(cf.Cargo) == 0 {
				freeCFs = append(freeCFs, cf)
				continue
			}
			if !isEnRoute(cf) {
				continue
			}
			for _, c := range cf.Cargo {
				enRoute[c.ResourceType] += int(c.Quantity)
				if _, ok := missing[c.ResourceType]; ok && cf.Travel != nil && cf.Travel.Arrives.Time().After(eta) {
					eta = cf.Travel.Arrives.Time()
				}
			}
			if cf.Location == ev.Location {
				log("Unloading cargo from %s: %v", cf.Code.Alias(), cf.Cargo)
				_, err := rest.DeviceCommand[models.CommandResp](cf.Code, "deposit_resources", nil)
				if err != nil {
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
				if sp.Location == home && len(sp.AttachedDevices) == 0 {
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
		for k, v := range missing {
			if v-enRoute[k] < 0 {
				log("%d x %s already en-route", enRoute[k], k)
				delete(missing, k)
				continue
			}
		}
		if len(missing) == 0 {
			log("All required resources are already en-route, ETA %s (%s)", eta, time.Until(eta))
			return nil
		}
		// Find an empty cf at home, use it
		if len(freeCFs) == 0 {
			return fmt.Errorf("No freighters available to deliver %v to %s", missing, ev.Location)
		}
		for _, cf := range freeCFs {
			if len(missing) == 0 {
				break
			}
			avail := cf.CargoCapacity
			get := make(map[string]int)
			for k, v := range missing {
				if !isResource(k) {
					continue
				}
				if v <= avail {
					get[k] = v
					delete(missing, k)
					avail -= v
				} else {
					get[k] = avail
					missing[k] -= avail
					avail = 0
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
			err = travel(cf.Code, ev.Location)
			if err != nil {
				return err
			}
		}

		if len(missing) == 0 {
			return nil
		}
		// TODO: handle devices

		return fmt.Errorf("Need to deliver %v to %s", missing, ev.Location)
	}

	for _, d := range ec.Devices {
		missing[d.DeviceType] = d.Required - d.Current
	}
	loc, err := rest.Location(ev.Location)
	if err != nil {
		return err
	}
	for _, i := range loc.Inventory {
		missing[i.ResourceType] = int(-i.Quantity)
	}
	for r, q := range ec.Resources {
		missing[r] += q
	}

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
