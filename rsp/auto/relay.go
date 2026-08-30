package auto

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/zigdon/rsp/common"
	"github.com/zigdon/rsp/constants"
	"github.com/zigdon/rsp/models"
	"github.com/zigdon/rsp/rest"
)

// if we don't have relays, get some delivered
// if there are not enough relays to fill a ship, print more
// if there are spare relays in the system, pick them up
// if we're in a system, and it doesn't have exactly one relay, go to l4
// - check distance to nearest relay. If it's >7.5 <=10, deploy dsrs
//   - detach/unfurl/wait/activate/tag
// - then deploy/activate/tag
// - pick up and de-tag spares
// if there is a relay, check our tags for dest, plot the course from the nearest relay on the home network
// - if no course is available, try again with dsrs support
// States:
// cargo vessel:
//   transit: wait
//   incoming: scan, check frs, {go to L4 point / or skip to leaving}
//   deploying: deploy, activate, tag
//   cleanup: stow/detag spares
//   empty: move FRs and DSRSs from mf
//   leaving: find the next system, head there
// mobile fleet:
//   empty: head home, queue FRs, DSRSs as needed
//   resupplying: attach 3 DSRS, FRs until full capacity
//   full: follow cv

type RelayMachine_State string

const (
	RelayMachine_Done             = "done"
	RelayMachine_Transit          = "transit"
	RelayMachine_Incoming         = "incoming"
	RelayMachine_DeployingRelay   = "deploying relay"
	RelayMachine_DeployingStation = "deploying dsrs"
	RelayMachine_Cleanup          = "cleanup"
	RelayMachine_Empty            = "empty"
	RelayMachine_Leaving          = "leaving"
)

type RelayMachine struct {
	dryRun    bool
	dev       *models.Device
	supply    *models.Device
	dest      models.LocationID
	state     RelayMachine_State
	replicant *models.CodeAlias
	status    string
	regions   []string
}

func (rm *RelayMachine) Start(d *models.Device, dryRun bool) error {
	// Make sure the device is a vessel
	if !slices.Contains([]string{"heaven_vessel", "racing_vessel", "cargo_vessel", "crystal_vessel"}, d.Type) {
		return fmt.Errorf("%s is not a vessel: %q", d.Code.Alias(), d.Type)
	}
	rm.dev = d
	rm.status = "initializing"

	// Save the resident replicant
	if d.StowedDevices == nil {
		return fmt.Errorf("No stowed devices found in %q", d.Code.Alias())
	}
	for _, st := range d.StowedDevices.Devices {
		if st.Type != "replicant_matrix" {
			continue
		}
		dev, err := rest.DeviceInfo(st.Code)
		if err != nil {
			return fmt.Errorf("Can't get resident replicant from %q: %v", st.Code.Alias(), err)
		}
		if dev.ReplicantCode == nil {
			return fmt.Errorf("No replicant found in %q", st.Code.Alias())
		}
		rm.replicant = dev.ReplicantCode
		break
	}
	if rm.replicant == nil {
		return fmt.Errorf("No replicant found in %q", d.Code.Alias())
	}

	if regions := getTags(rm.dev)["regions"]; regions != "" {
		rm.regions = strings.Split(regions, ":")
	} else {
		rm.regions = []string{"solzone", "alpha", "beta", "gamma"}
	}

	rm.dryRun = dryRun
	switch {
	case getTags(rm.dev)["relay"] != "":
		rm.dest = models.LocationID(strings.ToUpper(getTags(rm.dev)["relay"]))
	case getTags(rm.dev)["follow"] != "":
		dest, err := rest.DeviceInfo(models.NewCodeAlias(getTags(rm.dev)["follow"]))
		if err != nil {
			return fmt.Errorf("Can't follow %q: %v", getTags(rm.dev)["follow"], err)
		}
		rm.dest = dest.Location
	case getTags(rm.dev)["fill"] != "":
		// Just say our current location is the destination, we'll figure it out later
		rm.dest = rm.dev.Location
	default:
		return fmt.Errorf("Can't figure relay destination")
	}

	p, err := rest.GetTagged(fmt.Sprintf("supply:%s", d.Code.Alias()))
	if err != nil {
		return fmt.Errorf("Can't get tagged supply ship: %v", err)
	}
	if len(p.Devices) != 1 {
		return fmt.Errorf("Can't find exactly one device tagged supply:%s, found %d", d.Code.Alias(), len(p.Devices))
	}
	rm.supply = p.Devices[0]

	return rm.UpdateState()
}

func (rm *RelayMachine) UpdateState() error {
	dev, err := rest.RefreshDeviceInfo(rm.dev.Code)
	if err != nil {
		return fmt.Errorf("Can't refresh info for %q: %v", rm.dev.Code.Alias(), err)
	}
	rm.dev = dev
	status := rm.dev.Status

	supply, err := rest.RefreshDeviceInfo(rm.supply.Code)
	if err != nil {
		return fmt.Errorf("Can't refresh info for supply ship %q: %v", rm.supply.Code.Alias(), err)
	}
	rm.supply = supply

	// State flags
	var sysFRs []*models.Device // FRs in system
	var sysFRRelaying bool      // FRs operational
	inL4 := strings.Contains(string(rm.dev.Location), "L4")

	// Check FR inventory
	frInv := slices.ContainsFunc(rm.dev.StowedDevices.Devices, func(d *models.DevicePointer) bool {
		return d.Type == "ftl_relay"
	})

	// Check the current location
	if rm.dev.Location != "" {
		star := rm.dev.Location.Star()
		frs, err := rest.RefreshDevices(map[string]string{
			"device_type": "ftl_relay",
			"location":    star,
		})
		if err != nil {
			return fmt.Errorf("Can't get ftl relays at %q: %v", star, err)
		}
		for _, fr := range frs {
			if fr.AttachedToDeviceCode != nil {
				continue
			}
			sysFRs = append(sysFRs, fr)
			if fr.Status == "relaying" {
				sysFRRelaying = true
			}
		}
	}
	sysHasSpareFR := len(sysFRs) > 1

	// Check the distance from the nearest relay
	dist, err := rm.findNetworkDist()
	if err != nil {
		return fmt.Errorf("Can't find distance from network: %v", err)
	}

	log("State: %s@%s, %s@%s; Dest: %q (%v), System FRs: %d, relaying: %v",
		rm.dev.Code.Alias(), rm.dev.Location,
		rm.supply.Code.Alias(), rm.supply.Location,
		rm.dest, rm.regions, len(sysFRs), sysFRRelaying)

	oldState := rm.state
	switch {
	case rm.dev.Location == "" || status != "idle":
		log("In transit")
		rm.state = RelayMachine_Transit
		rm.status = "relocating"
	case slices.Contains(constants.Homes, string(rm.dev.Location)):
		log("Leaving home")
		rm.state = RelayMachine_Leaving
	case rm.state == "" && status == "idle":
		log("Blank state, stationary")
		rm.state = RelayMachine_Incoming
	case rm.state == "" && status != "idle":
		log("Blank state, moving")
		rm.state = RelayMachine_Transit
	case sysFRRelaying && sysHasSpareFR:
		log("System relayed, cleanup available")
		rm.state = RelayMachine_Cleanup
		rm.status = "collecting spare relays"
	case !inL4:
		log("Not in L4")
		rm.state = RelayMachine_Incoming
		rm.status = "repositioning"
	case !frInv:
		log("Out of inventory")
		rm.state = RelayMachine_Empty
		rm.status = "resupplying"
	case sysFRRelaying && !sysHasSpareFR:
		log("System relayed, no cleanup")
		rm.state = RelayMachine_Leaving
		rm.status = "departing"
	case inL4 && !sysFRRelaying && dist <= 7.5:
		log("At L4, ready to deploy relay")
		rm.state = RelayMachine_DeployingRelay
		rm.status = "deploying relay"
	case inL4 && !sysFRRelaying && dist <= 10 && rm.dev.AttachCapacity > 0:
		log("At L4, ready to deploy station")
		rm.state = RelayMachine_DeployingStation
		rm.status = "deploying dsrs"
	default:
		return fmt.Errorf(
			"Unknown state (%s): state: %q, FRs: ship %v, sys %d (relaying: %v, edge dist: %.2fly)",
			rm.dev.Code.Alias(), rm.state, frInv, len(sysFRs), sysFRRelaying, dist)
	}
	log("Update state: %s -> %s", oldState, rm.state)
	return nil
}

func (rm *RelayMachine) Process() (time.Time, error) {
	eta := time.Now().Add(5 * time.Minute)
	if err := rm.UpdateState(); err != nil {
		return eta, err
	}
	nextState := rm.state
	log("State: %s", rm.state)
	switch rm.state {
	case RelayMachine_Done:
		log("*******************************************")
		log("* RELAY COMPLETE: reached %s", rm.dest)
		log("*******************************************")
		err := rest.UpdateTags(rm.dev.Code, rest.DelTag, []string{"relay:" + string(rm.dest)})
		if err != nil {
			log("Failed to remove tags: %v", err)
		}
		if dest := getTags(rm.dev)["relay"]; dest != "" {
			log("Next destination: %s", dest)
			rm.dest = models.LocationID(dest)
		} else {
			return eta, MachineDoneErr(fmt.Sprintf("*** Relay to %s complete ***", rm.dest))
		}
	case RelayMachine_Transit:
		if t := rm.dev.Travel; t != nil {
			eta = t.Arrives.Time()
		}
	case RelayMachine_Incoming:
		if strings.Contains(string(rm.dev.Location), "L4") {
			return eta, nil
		}
		scan, err := rest.ReplicantScan(rm.replicant)
		if err != nil {
			return eta, fmt.Errorf("Can't trigger scan at %q: %v", rm.dev.Location, err)
		}
		if scan.AsteroidBelt.Present {
			// TODO: add a task to send a mining fleet
			log("Asteroid belt detected: %v", scan.AsteroidBelt.Belts)
		}
		if len(scan.SystemObjects) > 0 {
			var objs []string
			for _, so := range scan.SystemObjects {
				objs = append(objs, string(so.Designation))
			}
			log("System objects found: %s", strings.Join(objs, ", "))
		}
		if rm.dev.Location != scan.EntryPoint {
			res, err := deviceCommand(rm.dev.Code, "travel", map[string]any{
				"destination": scan.EntryPoint,
			}, rm.dryRun)
			eta = res.Arrives.Time()
			if err != nil {
				return eta, err
			}
		} else {
			log("at %s entry point: %s", rm.dev.Location.Star(), scan.EntryPoint)
			dist, err := rm.findNetworkDist()
			if err != nil {
				return eta, err
			}
			if dist <= 7.5 {
				nextState = RelayMachine_DeployingRelay
			} else if dist <= 10 && rm.dev.AttachCapacity > 0 {
				nextState = RelayMachine_DeployingStation
			} else {
				return eta, fmt.Errorf("Too far from network to deploy: %.2fly", dist)
			}
		}
	case RelayMachine_DeployingRelay:
		// Find an FR in our hold
		var fr *models.CodeAlias
		for _, d := range rm.dev.StowedDevices.Devices {
			if d.Type != "ftl_relay" {
				continue
			}
			fr = d.Code
			break
		}
		if fr == nil {
			log("Out of relays")
			nextState = RelayMachine_Empty
		} else {
			// Change owner before we attempt to deploy
			_, err := deviceCommand(fr, "change_owner",
				map[string]any{"target": rm.replicant.String()}, rm.dryRun)
			if err != nil {
				log("Ignoring owner change error: %v", err)
			}
			// Deploy
			_, err = deviceCommand(fr, "deploy", nil, rm.dryRun)
			if err != nil {
				return eta, err
			}
			// Activate
			_, err = deviceCommand(fr, "activate", nil, rm.dryRun)
			if err != nil {
				return eta, err
			}
			// Tag
			err = rest.UpdateTags(fr, rest.AddTag, []string{"infrastructure"})
			if err != nil {
				return eta, fmt.Errorf("Can't update tags on %q: %v", fr.Alias(), err)
			}
			log("Relay deployed at %s", rm.dev.Location)
			// Refresh the location, so newly in-network devices will notice
			if _, err = rest.RefreshDevices(map[string]string{"location": rm.dev.Location.Star()}); err != nil {
				log("Error refreshing system: %v", err)
			}
			nextState = RelayMachine_Cleanup
		}
	case RelayMachine_DeployingStation:
		// See if we already have a dsrs here, and we're just waiting for it to unful
		devs, err := rest.Devices(map[string]string{
			"location":    string(rm.dev.Location),
			"device_type": "deep_space_relay_station"})
		if err != nil {
			return eta, err
		}
		if len(devs) > 0 {
			// We already are in the process of deploying one.
			dsrs := devs[0]
			// Check if it's compacted
			switch dsrs.Status {
			case "compacted":
				res, err := deviceCommand(dsrs.Code, "unfurl", nil, rm.dryRun)
				if err != nil {
					return eta, err
				}
				eta = res.Completes.Time()
			case "idle":
				// Activate
				_, err = deviceCommand(dsrs.Code, "activate", nil, rm.dryRun)
				if err != nil {
					return eta, err
				}
				// Tag
				err = rest.UpdateTags(dsrs.Code, rest.AddTag, []string{"infrastructure"})
				if err != nil {
					return eta, fmt.Errorf("Can't update tags on %q: %v", dsrs.Code.Alias(), err)
				}
				log("Station deployed at %s", rm.dev.Location)
				// Refresh the location, so newly in-network devices will notice
				if _, err = rest.RefreshDevices(map[string]string{"location": rm.dev.Location.Star()}); err != nil {
					log("Error refreshing system: %v", err)
				}
				nextState = RelayMachine_Cleanup
			default:
				log("Found %s is %s, nothing to do", dsrs.Code, dsrs.Status)
				nextState = RelayMachine_Cleanup
			}
		} else if rm.dev.AttachCapacity > 0 {
			// None deployed, find a dsrs
			var dsrs *models.CodeAlias
			for _, d := range rm.dev.AttachedDevices {
				if d.Type != "deep_space_relay_station" {
					continue
				}
				dsrs = d.Code
				break
			}
			if dsrs == nil {
				log("Out of stations")
				nextState = RelayMachine_Empty
			} else {
				// Change owner before we attempt to deploy
				_, err := deviceCommand(dsrs, "change_owner",
					map[string]any{"target": rm.replicant.String()}, rm.dryRun)
				if err != nil {
					log("Ignoring owner change error: %v", err)
				}
				// Detach
				_, err = deviceCommand(rm.dev.Code, "detach", map[string]any{"target": dsrs}, rm.dryRun)
				if err != nil {
					return eta, err
				}
				// Unfurl
				res, err := deviceCommand(dsrs, "unfurl", nil, rm.dryRun)
				if err != nil {
					return eta, err
				}
				eta = res.Completes.Time()

				// Refresh the location, to make sure we'll remember it's already here
				if _, err = rest.RefreshDevices(map[string]string{"location": rm.dev.Location.Star()}); err != nil {
					log("Error refreshing system: %v", err)
				}
			}
		} else {
			log("%s can't deploy DSRS", rm.dev.Code)
			nextState = RelayMachine_Cleanup
		}
	case RelayMachine_Cleanup:
		// Find spares in system
		frs, err := rest.RefreshDevices(map[string]string{
			"location":    rm.dev.Location.Star(),
			"device_type": "ftl_relay",
		})
		if err != nil {
			return eta, fmt.Errorf("Can't find system spares: %v", err)
		}
		if len(frs) >= 1 {
			frs = frs[1:]
			// Pick up the local FRs first
			var next string
			for _, d := range frs {
				if d.AttachedToDeviceCode != nil {
					continue
				}
				if d.Location == rm.dev.Location {
					_, err = deviceCommand(d.Code, "stow", map[string]any{
						"target": rm.dev.Code,
					}, rm.dryRun)
					if err != nil {
						return eta, err
					}
				} else {
					next = string(d.Location)
				}
			}
			if next != "" {
				log("Moving to %q to pick up more spare FRs", next)
				res, err := deviceCommand(rm.dev.Code, "travel", map[string]any{
					"destination": next,
				}, rm.dryRun)
				if err != nil {
					return eta, err
				}
				eta = res.Arrives.Time()
			}
		} else {
			log("Cleanup done")
			nextState = RelayMachine_Leaving
		}
	case RelayMachine_Empty:
		if rm.dev.Location != rm.supply.Location {
			log("Waiting for resupply at %q", rm.dev.Location)
			return eta, rm.resupply()
		}
		if len(rm.supply.AttachedDevices) == 0 {
			return eta, fmt.Errorf("Resupply vessage %q unexpectedly empty at %q",
				rm.supply.Code.Alias(), rm.dev.Location)
		}
		stowCap := rm.dev.StowCapacity - len(rm.dev.StowedDevices.Devices)
		atCap := rm.dev.AttachCapacity - len(rm.dev.AttachedDevices)
		var stowed, attached int
		for _, d := range rm.supply.AttachedDevices {
			if d.Type == "ftl_relay" && stowed < stowCap {
				_, err := deviceCommand(rm.supply.Code, "detach",
					map[string]any{"target": d.Code.Alias()}, rm.dryRun)
				if err != nil {
					return eta, err
				}
				_, err = deviceCommand(d.Code, "stow",
					map[string]any{"target": rm.dev.Code}, rm.dryRun)
				if err != nil {
					return eta, err
				}
				stowed++
			} else if d.Type == "deep_space_relay_station" && attached < atCap {
				_, err := deviceCommand(rm.supply.Code, "detach",
					map[string]any{"target": d.Code.Alias()}, rm.dryRun)
				if err != nil {
					return eta, err
				}
				_, err = deviceCommand(d.Code, "attach",
					map[string]any{"target": rm.dev.Code}, rm.dryRun)
				if err != nil {
					return eta, err
				}
				attached++
			} else {
				log("Ignoring %s attached to %s", d.Code, rm.supply.Code)
			}
		}
		log("Picked up %d FRs, %d DSRSs, shipping resupply back home", stowed, attached)
		var err error
		resupplyHome := common.ClosestHomes(rm.supply.Location)[0]
		eta, err = common.Travel(rm.supply.Code, resupplyHome, rm.dryRun)
		if err != nil {
			return eta, err
		}
		frCount := rm.supply.AttachCapacity - rm.dev.AttachCapacity
		pPlan, err := common.Print(resupplyHome, "ftl_relay", frCount, true, rm.dryRun, nil)
		if err != nil {
			log("Error printing relays: %v", err)
		} else {
			log("Queued %d ftl_relays: ETA %s (%s)", frCount, pPlan.ETA, time.Until(pPlan.ETA))
		}
		pPlan, err = common.Print(resupplyHome, "deep_space_relay_station", rm.dev.AttachCapacity, true, rm.dryRun, nil)
		if err != nil {
			log("Error printing DSRS: %v", err)
		} else {
			log("Queued %d DSRS: ETA %s (%s)", rm.dev.AttachCapacity, pPlan.ETA, time.Until(pPlan.ETA))
		}

		nextState = RelayMachine_Leaving
	case RelayMachine_Leaving:
		if rm.dev.Location.Star() == rm.dest.Star() || rm.dest == "" {
			next, err := rm.getNext()
			if err != nil {
				return eta, err
			}
			if len(next) == 0 {
				rm.state = RelayMachine_Done
				err := rest.UpdateTags(rm.dev.Code, rest.DelTag, []string{"auto"})
				if err != nil {
					log("Error removing the 'auto' tag from %q: %v", rm.dev, err)
				}
				return eta, MachineDoneErr(
					fmt.Sprintf("Relay destination reached: %s", rm.dev.Location))
			}

			// Sort the list of possible next destinations by distance
			dists := make(map[string]float32)
			for _, n := range next {
				d, err := common.Distance(rm.dev.Location.Star(), n.Star())
				if err != nil {
					return eta, err
				}
				dists[n.Star()] = d
			}
			slices.SortFunc(next, func(a, b models.LocationID) int {
				return cmp.Compare(dists[a.Star()], dists[b.Star()])
			})

			// Make sure we have a path to the next location
			var found bool
			for _, n := range next {
				// Start from the nearest edge of the relay network
				edge, err := common.NearestRelay(n.Star())
				if err != nil {
					log("Can't find nearest relay to %s: %v", rm.dest, err)
					continue
				}
				log("Next destination: %s (%.2f LY away):", n, dists[n.Star()])
				if edge != n.Star() {
					path, err := common.PlotTrip(edge, n.Star(), nil)
					if err != nil {
						log("Can't plot path %s->%s: %v", edge, n.Star(), err)
						continue
					}
					for _, l := range path.Legs {
						log("  %s -> %s ", l.From, l.To)
					}
				}

				rm.dest = n
				found = true
				break
			}

			if !found {
				return eta, fmt.Errorf("Can't find an routable destination")
			}
		}

		// find the nearest relay to the destination
		next, err := common.NearestRelay(rm.dest.Star())
		if err != nil {
			return eta, fmt.Errorf("Can't find nearest relay to %s: %v", rm.dest, err)
		}
		curDist, err := common.Distance(rm.dev.Location.Star(), rm.dest.Star())
		if err != nil {
			return eta, fmt.Errorf("Can't get distance between %q and %q: %v", rm.dev.Location, rm.dest, err)
		}
		if curDist == 0 {
			log("At destination, waiting...")
			return eta, nil
		}
		relayDist, err := common.Distance(next, rm.dest.Star())
		if err != nil {
			return eta, fmt.Errorf("Can't get distance between the relay at %q and %q: %v",
				next, rm.dest, err)
		}
		log("Nearest relay to %s is %s (%.2f LY away)", rm.dest.Star(), next, relayDist)
		if next != rm.dev.Location.Star() && relayDist < curDist {
			eta, err = common.Travel(rm.dev.Code, next, rm.dryRun)
			if err != nil {
				return eta, err
			}
			rm.dev.Location = models.LocationID(next)
			nextState = RelayMachine_Transit
		} else {
			log("Already %.2f LY away, venturing out to %s", curDist, rm.dest.Star())

			// plot the next hop
			route, err := common.PlotTrip(string(rm.dev.Location), rm.dest.Star(), nil)
			if err != nil {
				return eta, err
			}
			var lost = true
			devs, err := rest.Devices(map[string]string{"device_type": "ftl_relay"})
			if err != nil {
				return eta, err
			}
			hasFR := make(map[string]bool)
			for _, d := range devs {
				if d.Status != "relaying" {
					continue
				}
				hasFR[d.Location.Star()] = true
			}
			earlier := true
			for _, l := range route.Legs {
				if earlier && l.From != rm.dev.Location.Star() {
					continue
				}
				earlier = false
				// See if there's already an FR there
				if hasFR[l.To] {
					continue
				}
				lost = false
				eta, err = common.Travel(rm.dev.Code, l.To, rm.dryRun)
				if err != nil {
					return eta, err
				}
				rm.dev.Location = models.LocationID(l.To)
				nextState = RelayMachine_Transit
				break
			}
			if lost {
				return eta, fmt.Errorf("Can't figure out the next step from %q to %q",
					rm.dev.Location, rm.dest)
			}
		}
	default:
		return eta, fmt.Errorf("Unknown state: %q", rm.state)
	}

	if err := rm.resupply(); err != nil {
		return eta, err
	}

	if nextState != rm.state {
		log("Shifting state %s -> %s", rm.state, nextState)
		rm.state = nextState
	}

	return eta, nil
}

func (rm *RelayMachine) resupply() error {
	dest := rm.dev.Location
	if dest == "" && rm.dev.Travel != nil {
		dest = rm.dev.Travel.Destination
	}

	// Handle supply vessal
	switch {
	case rm.supply.Location == "":
		supplyETA := rm.supply.Travel.Arrives.Time().Truncate(time.Second)
		log("Resupply platform %s in transit... ETA: %s (%s)",
			rm.supply, supplyETA, time.Until(supplyETA).Truncate(time.Second))
	case slices.Contains(constants.Homes, string(rm.supply.Location)):
		// Count how many DSRS we have attached, we want 3.
		var dsrsCount int
		for _, d := range rm.supply.AttachedDevices {
			if d.Type == "deep_space_relay_station" {
				dsrsCount++
			}
		}
		slots := rm.supply.AttachCapacity - len(rm.supply.AttachedDevices)
		if dsrsCount < 3 {
			// Make sure we have 3 open slots
			devs := rm.supply.AttachedDevices
			for slots < 3 {
				_, err := deviceCommand(rm.supply.Code, "detach", map[string]any{
					"targets": devs[:3-slots],
				}, rm.dryRun)
				if err != nil {
					return fmt.Errorf("Can't free slots on %s: %v", rm.supply.Code, err)
				}
			}
			devs, err := rest.RefreshDevices(map[string]string{
				"location":    string(rm.supply.Location),
				"device_type": "deep_space_relay_station",
			})
			if err != nil {
				return fmt.Errorf("Can't find stations at %q: %v", rm.supply.Location, err)
			}
			if len(devs) < 3-dsrsCount {
				return fmt.Errorf("Not enough stations at %q: need %d, found %d",
					rm.supply.Location, 3-dsrsCount, len(devs))
			}
			// Attach the missing stations
			var ids []*models.CodeAlias
			for _, d := range devs[:3-dsrsCount] {
				ids = append(ids, d.Code)
			}
			_, err = deviceCommand(rm.supply.Code, "attach", map[string]any{"targets": ids}, rm.dryRun)
			if err != nil {
				return fmt.Errorf("Error attaching stations %v to %s: %v", ids, rm.supply.Code, err)
			}
			slots -= len(ids)
		}

		// Now pick up remaining FRs
		devs, err := rest.RefreshDevices(map[string]string{
			"location":    string(rm.supply.Location),
			"device_type": "ftl_relay",
		})
		if err != nil {
			return fmt.Errorf("Can't find ftl relays at %q: %v", rm.supply.Location, err)
		}
		var homeFRs []*models.Device
		for _, d := range devs {
			if d.AttachedToDeviceCode != nil {
				continue
			}
			homeFRs = append(homeFRs, d)
		}
		if len(homeFRs) == 0 {
			log("No FRs available at home")
		}
		log("Loading %d FRs at home, %d available...", slots, len(homeFRs))
		if slots > 0 && len(homeFRs) > 0 {
			if slots < len(homeFRs) {
				homeFRs = homeFRs[:slots]
			}
			ids := make([]string, len(homeFRs))
			for n := range homeFRs {
				ids[n] = homeFRs[n].Code.Alias()
			}
			_, err := deviceCommand(rm.supply.Code, "attach", map[string]any{
				"targets": ids,
			}, rm.dryRun)
			if err != nil {
				return err
			}
		}
		if len(rm.supply.AttachedDevices) > 0 && dest != "" {
			log("Shipping out to %q to deliver FRs", dest)
			eta, err := common.Travel(rm.supply.Code, string(dest), rm.dryRun)
			if err != nil {
				return err
			}
			log("Supply ship in transit: %s (%s)", eta, time.Until(eta))
		} else {
			log("Supply ship waiting for new relays -- consider printing some")
		}
	case rm.supply.Location == rm.dev.Location:
		log("Waiting for resupply at %q", rm.dev.Location)
	default:
		if dest != "" {
			log("Following %s to %q", rm.dev.Code.Alias(), dest)
			eta, err := common.Travel(rm.supply.Code, string(dest), rm.dryRun)
			if err != nil {
				return err
			}
			log("Supply ship in transit: %s (%s)", eta, time.Until(eta))
		} else {
			log("Waiting for %s to reappear", rm.dev.Code.Alias())
		}
	}

	return nil
}

func (rm *RelayMachine) SaveState(string) error {
	return nil
}

func (rm *RelayMachine) Status() string {
	return rm.status
}

func (rm *RelayMachine) Name() string {
	return "Relay Machine"
}

func (rm *RelayMachine) getNext() ([]models.LocationID, error) {
	if follow := getTags(rm.dev)["follow"]; follow != "" {
		next, err := rm.getNextFollow(follow)
		log("Following %s to %s", follow, next)
		return []models.LocationID{next}, err
	}
	fill := getTags(rm.dev)["fill"]
	switch fill {
	case "beacons":
		return rm.getNextBeacons()
	case "oor":
		return rm.getNextStranded()
	default:
		return nil, fmt.Errorf("Unknown fill mode %q", fill)
	}
}

func (rm *RelayMachine) getNextBeacons() ([]models.LocationID, error) {
	// find the closest system that has an ftl beacons but is not networked
	net, err := rest.DeviceNetwork(models.NewCodeAlias("sh-1"))
	if err != nil {
		return nil, fmt.Errorf("Can't get ftl network: %v", err)
	}
	inNet := make(map[string]bool)
	inNet["MENKUNT"] = true
	for _, c := range net.Connections {
		inNet[c.Star] = true
	}
	log("%d systems in network", len(inNet))
	fbs, err := rest.Devices(map[string]string{"device_type": "ftl_beacon"})
	if err != nil {
		return nil, fmt.Errorf("Can't get beacons: %v", err)
	}
	shs, err := rest.Devices(map[string]string{"device_type": "system_hub"})
	if err != nil {
		return nil, fmt.Errorf("Can't get hubs: %v", err)
	}
	fbs = append(fbs, shs...)

	var res []models.LocationID
	for _, fb := range fbs {
		if fb.Location == "" {
			continue
		}
		if inNet[fb.Location.Star()] {
			continue
		}
		res = append(res, fb.Location)
	}
	log("%d beacons out of network", len(res))
	return res, nil
}

func (rm *RelayMachine) getNextStranded() ([]models.LocationID, error) {
	// find systems that has devices that report being out-of-range
	// that doesn't already have an auto:relay devices heading there
	rows, err := DB.Query(`
	  SELECT DISTINCT(location)
	  FROM json_devices JOIN stars ON location = designation
	  WHERE status = 'out_of_range'
		AND region = ANY($1)
		AND location NOT IN (
		  SELECT split_part(data->'travel'->>'destination', '-', 1)
		  FROM json_devices
		  WHERE status = 'travelling'
			AND data->'tags' @> '"auto:relay"'
			AND data->'travel'->>'destination' IS NOT NULL
		)
	`, rm.regions)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var oors []models.LocationID
	for rows.Next() {
		var loc string
		if err := rows.Scan(&loc); err != nil {
			return nil, fmt.Errorf("Can't find next OOR device: %v", err)
		}
		oors = append(oors, models.LocationID(loc))
	}
	log("%d systems with out-of-network devices", len(oors))

	var res []models.LocationID
	for _, n := range oors {
		edge, err := common.NearestRelay(n.Star())
		if err != nil {
			log("Can't find nearest relay to %s: %v", rm.dest, err)
			continue
		}
		// Make sure no other fill:oor ship is heading there already
		row := DB.QueryRow(`
					SELECT code
					FROM json_devices
					WHERE data->'tags' @> '"fill:oor"'
					  AND data->'travel'->>'destination'=$1
					LIMIT 1`, edge)
		var other string
		if err := row.Scan(&other); err == nil {
			log("%s is already travelling to %s", models.NewCodeAlias(other), edge)
			continue
		}
		res = append(res, n)
	}
	log("%d unassigned systems with out-of-network devices", len(res))
	return res, nil
}

func (rm *RelayMachine) getNextFollow(target string) (models.LocationID, error) {
	info, err := rest.DeviceInfo(models.NewCodeAlias(target))
	if err != nil {
		return "", fmt.Errorf("Can't follow %q: %v", target, err)
	}
	if info.Location != "" {
		log("%s is stationary at %s", target, info.Location)
		return info.Location, nil
	} else if info.Travel != nil {
		log("%s is travelling to %s", target, info.Travel.Destination)
		return info.Travel.Destination, nil
	} else {
		return "", fmt.Errorf("Can't figure out how to follow %q", target)
	}
}

func (rm *RelayMachine) findNetworkDist() (float32, error) {
	edge, err := common.NearestRelay(string(rm.dev.Location))
	if err != nil {
		return 0, fmt.Errorf("Can't find network edge: %v", err)
	}
	dist, err := common.Distance(edge, string(rm.dev.Location))
	if err != nil {
		return 0, fmt.Errorf("Can't find distance to network edge %q: %v", edge, err)
	}
	return dist, nil
}
