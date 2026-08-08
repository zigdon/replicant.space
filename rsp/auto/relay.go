package auto

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/zigdon/rsp/common"
	"github.com/zigdon/rsp/models"
	"github.com/zigdon/rsp/rest"
)

// if we don't have relays, get some delivered
// if there are not enough relays to fill a ship, print more
// if there are spare relays in the system, pick them up
// if we're in a system, and it doesn't have exactly one relay, go to l4
// - then deploy/activate/tag
// - pick up and de-tag spares
// if there is a relay, check our tags for dest, plot the course from the nearest relay on the home network
// States:
// cargo vessel:
//   transit: wait
//   incoming: scan, check frs, {go to L4 point / or skip to leaving}
//   deploying: deploy, activate, tag
//   cleanup: stow/detag spares
//   empty: move FRs from mf
//   leaving: find the next system, head there
// mobile fleet:
//   empty: head home, queue FRs
//   resupplying: attach until full capacity
//   full: follow cv

const home = "MENKUNT-2-L4"

type RelayMachine struct {
	dryRun    bool
	dev       *models.Device
	supply    *models.Device
	dest      models.LocationID
	state     string
	replicant *models.CodeAlias
}

func (rm *RelayMachine) Start(d *models.Device, dryRun bool) error {
	// Make sure the device is a vessel
	if !slices.Contains([]string{"heaven_vessel", "racing_vessel", "cargo_vessel"}, d.Type) {
		return fmt.Errorf("%s is not a vessel: %q", d.Code.Alias(), d.Type)
	}
	rm.dev = d

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
		var star *models.Star
		star, err = models.NewStar(rm.dev.Location.Star())
		if err != nil {
			return err
		}

		if star != nil {
			frs, err := rest.Devices(map[string]string{
				"device_type": "ftl_relay",
				"location":    string(star.Designation),
			})
			if err != nil {
				return fmt.Errorf("Can't get ftl relays at %q: %v", star.Designation, err)
			}
			for _, fr := range frs {
				if fr.AttachedToDeviceCode != nil {
					continue
				}
				sysFRs = append(sysFRs, fr)
				if fr.Status == "relaying" {
					sysFRRelaying = true
					break
				}
			}
		}
	}
	sysHasSpareFR := len(sysFRs) > 1

	log("State: %s@%s, %s@%s; System FRs: %d, relaying: %v",
		rm.dev.Code.Alias(), rm.dev.Location,
		rm.supply.Code.Alias(), rm.supply.Location,
		len(sysFRs), sysFRRelaying)

	oldState := rm.state
	switch {
	case rm.dev.Location == "" || status != "idle":
		log("In transit")
		rm.state = "transit"
	case rm.dev.Location == home:
		log("Leaving home")
		rm.state = "leaving"
	case rm.state == "" && status == "idle":
		log("Blank state, stationary")
		rm.state = "incoming"
	case rm.state == "" && status != "idle":
		log("Blank state, moving")
		rm.state = "transit"
	case sysFRRelaying && sysHasSpareFR:
		log("System relayed, cleanup available")
		rm.state = "cleanup"
	case !inL4:
		log("Not in L4")
		rm.state = "incoming"
	case !frInv:
		log("Out of inventory")
		rm.state = "empty"
	case sysFRRelaying && !sysHasSpareFR:
		log("System relayed, no cleanup")
		rm.state = "leaving"
	case inL4 && !sysFRRelaying:
		log("At L4, ready to deploy")
		rm.state = "deploying"
	default:
		return fmt.Errorf(
			"Unknown state (%s): state: %q, FRs: ship %v, sys %d (relaying: %v)",
			rm.dev.Code.Alias(), rm.state, frInv, len(sysFRs), sysFRRelaying)
	}
	log("Update state: %s -> %s", oldState, rm.state)
	return nil
}

func (rm *RelayMachine) Process() (time.Time, error) {
	eta := time.Now().Add(30 * time.Second)
	if err := rm.UpdateState(); err != nil {
		return eta, err
	}
	nextState := rm.state
	log("State: %s", rm.state)
	switch rm.state {
	case "done":
		log("*******************************************")
		log("* RELAY COMPLETE: reached %s", rm.dest)
		log("*******************************************")
		_, err := rest.UpdateTags(rm.dev.Code, rest.DelTag, []string{"relay:" + string(rm.dest)})
		if err != nil {
			log("Failed to remove tags: %v", err)
		}
		if dest := getTags(rm.dev)["relay"]; dest != "" {
			log("Next destination: %s", dest)
			rm.dest = models.LocationID(dest)
		} else {
			return eta, MachineDoneErr(fmt.Sprintf("*** Relay to %s complete ***", rm.dest))
		}
	case "transit":
		if t := rm.dev.Travel; t != nil {
			eta = t.Arrives.Time()
		}
	case "incoming":
		if strings.Contains(string(rm.dev.Location), "L4") {
			return eta, nil
		}
		scan, err := rest.ReplicantScan(rm.replicant)
		if err != nil {
			return eta, fmt.Errorf("Can't trigger scan at %q: %v", rm.dev.Location, err)
		}
		if scan.AsteroidBelt.Present {
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
			nextState = "deploying"
		}
	case "deploying":
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
			nextState = "empty"
		} else {
			// Deploy
			_, err := deviceCommand(fr, "deploy", nil, rm.dryRun)
			if err != nil {
				return eta, err
			}
			// Activate
			_, err = deviceCommand(fr, "activate", nil, rm.dryRun)
			if err != nil {
				return eta, err
			}
			// Tag
			_, err = rest.UpdateTags(fr, rest.AddTag, []string{"infrastructure"})
			if err != nil {
				return eta, fmt.Errorf("Can't update tags on %q: %v", fr.Alias(), err)
			}
			log("Relay deployed at %s", rm.dev.Location)
			nextState = "cleanup"
		}
	case "cleanup":
		// Find spares in system
		frs, err := rest.RefreshDevices(map[string]string{"location": rm.dev.Location.Star()})
		if err != nil {
			return eta, fmt.Errorf("Can't find system spares: %v", err)
		}
		if len(frs) >= 1 {
			frs = frs[1:]
			// Pick up the local FRs first
			var next string
			for _, d := range frs {
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
			nextState = "leaving"
		}
	case "empty":
		if rm.dev.Location != rm.supply.Location {
			log("Waiting for resupply at %q", rm.dev.Location)
		} else {
			if len(rm.supply.AttachedDevices) == 0 {
				return eta, fmt.Errorf("Resupply vessage %q unexpectedly empty at %q",
					rm.supply.Code.Alias(), rm.dev.Location)
			}
			var stowed = 0
			for _, d := range rm.supply.AttachedDevices {
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
			}
			log("Picked up %d FRs, shipping resupply back home", stowed)
			var err error
			eta, err = common.Travel(rm.supply.Code, home, rm.dryRun)
			if err != nil {
				return eta, err
			}
			pPlan, err := common.Print(home, "ftl_relay", rm.supply.AttachCapacity, true, rm.dryRun, nil)
			if err != nil {
				log("Error printing relays: %v", err)
			} else {
				log("Queued %d ftl_relays: ETA %s (%s)",
					rm.supply.AttachCapacity, pPlan.ETA, time.Until(pPlan.ETA))
			}

			nextState = "leaving"
		}
	case "leaving":
		if rm.dev.Location.Star() == rm.dest.Star() {
			follow := getTags(rm.dev)["follow"]
			if follow == "" {
				rm.state = "done"
				return eta, MachineDoneErr(fmt.Sprintf("Relay destination reached: %s", rm.dev.Location))
			}

			target, err := rest.DeviceInfo(models.NewCodeAlias(follow))
			if err != nil {
				return eta, fmt.Errorf("Can't follow %q: %v", follow, err)
			}
			if target.Location != "" {
				rm.dest = target.Location
			} else if target.Travel != nil {
				rm.dest = target.Travel.Destination
			} else {
				return eta, fmt.Errorf("Can't figure out how to follow %q", follow)
			}
			log("New destination, %s", rm.dest)
		}

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
			nextState = "transit"
			break
		}
		if lost {
			return eta, fmt.Errorf("Can't figure out the next step from %q to %q",
				rm.dev.Location, rm.dest)
		}
	default:
		return eta, fmt.Errorf("Unknown state: %q", rm.state)
	}

	// Handle supply vessal
	switch rm.supply.Location {
	case "":
		log("Resupply platform in transit...")
	case home:
		slots := rm.supply.AttachCapacity - len(rm.supply.AttachedDevices)
		devs, err := rest.RefreshDevices(map[string]string{
			"location":    home,
			"device_type": "ftl_relay",
		})
		if err != nil {
			return eta, fmt.Errorf("Can't find ftl relays at %q: %v", home, err)
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
				return eta, err
			}
		}
		if len(rm.supply.AttachedDevices) > 0 {
			log("Shipping out to %q to deliver FRs", rm.dev.Location)
			eta, err := common.Travel(rm.supply.Code, string(rm.dev.Location), rm.dryRun)
			if err != nil {
				return eta, err
			}
			log("Supply ship in transit: %s (%s)", eta, time.Until(eta))
		} else {
			log("Supply ship waiting for new relays -- consider printing some")
		}
	case rm.dev.Location:
		log("Waiting for resupply at %q", rm.dev.Location)
	default:
		log("Following %s to %q", rm.dev.Code.Alias(), rm.dev.Location)
		eta, err := common.Travel(rm.supply.Code, string(rm.dev.Location), rm.dryRun)
		if err != nil {
			return eta, err
		}
		log("Supply ship in transit: %s (%s)", eta, time.Until(eta))
	}

	if nextState != rm.state {
		log("Shifting state %s -> %s", rm.state, nextState)
		rm.state = nextState
	}

	return eta, nil
}

func (rm *RelayMachine) SaveState(string) error {
	return nil
}

func (rm *RelayMachine) Status() string {
	return ""
}

func (rm *RelayMachine) Name() string {
	return "Relay Machine"
}
