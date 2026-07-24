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
	dryRun bool
	dev    *models.Device
	supply *models.Device
	dest   string
	state  string
}

func (rm *RelayMachine) Start(d *models.Device, dryRun bool) error {
	// Make sure the device is a replicant
	if !slices.Contains([]string{"heaven_vessel", "racing_vessel", "cargo_vessel"}, d.Type) {
		return fmt.Errorf("%s is not a vessel: %q", d.Code.Alias(), d.Type)
	}
	rm.dev = d
	rm.dryRun = dryRun
	dest := getTags(rm.dev)["relay"]
	if dest == "" {
		return fmt.Errorf("Relay destination not tagged on %s", d.Code.Alias())
	}
	rm.dest = dest
	state := getTags(rm.dev)["state"]
	if state == "" {
		return fmt.Errorf("State not tagged on %s", d.Code.Alias())
	}
	rm.state = state

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
	frInv := slices.ContainsFunc(rm.dev.StowedDevices.Devices, func(d *models.StowedDevice) bool {
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
			sysFRs, err := rest.Devices(map[string]string{
				"device_type": "ftl_relay",
				"location":    string(star.Designation),
			})
			if err != nil {
				return fmt.Errorf("Can't get ftl relays at %q: %v", star.Designation, err)
			}
			for _, fr := range sysFRs {
				if fr.Status == "relaying" {
					sysFRRelaying = true
					break
				}
			}
		}
	}
	sysHasSpareFR := len(sysFRs) > 1

	switch {
	case rm.state == "leaving":
	case rm.state == "cleanup" && sysHasSpareFR:
		rm.state = "cleanup"
	case rm.dev.Location == "" || status != "idle":
		rm.state = "transit"
	case rm.state == "transit" && sysFRRelaying && !sysHasSpareFR:
		rm.state = "leaving"
	case rm.state == "transit" && !inL4:
		rm.state = "incoming"
	case rm.state == "transit" && inL4:
		rm.state = "deploying"
	case rm.state == "deploying" && sysHasSpareFR:
		rm.state = "cleanup"
	case rm.state == "cleanup" && !sysHasSpareFR:
		rm.state = "leaving"
	case rm.state == "deploying" && !frInv:
		rm.state = "empty"
	case frInv && (rm.state == "empty" || rm.state == "deploying"):
		rm.state = "leaving"
	default:
		return fmt.Errorf(
			"Unknown state (%s): state: %q, FRs: ship %v, sys %d (relaying: %v)",
			rm.dev.Code.Alias(), rm.state, frInv, len(sysFRs), sysFRRelaying)
	}
	return nil
}

func (rm *RelayMachine) Process() (time.Time, error) {
	var eta time.Time
	switch rm.state {
	case "transit":
		if t := rm.dev.Travel; t != nil {
			eta = t.Arrives.Time()
		}
	case "incoming":
		if strings.Contains(string(rm.dev.Location), "L4") {
			return eta, nil
		}
		scan, err := rest.ReplicantScan(rm.dev.Code)
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
		res, err := deviceCommand(rm.dev.Code, "travel", map[string]any{
			"destination": scan.EntryPoint,
		})
		eta = res.Arrives.Time()
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
			return eta, fmt.Errorf("No FTL relays found in hold")
		}
		// Deploy
		_, err := deviceCommand(fr, "deploy", nil)
		if err != nil {
			return eta, err
		}
		// Activate
		_, err = deviceCommand(fr, "activate", nil)
		if err != nil {
			return eta, err
		}
		// Tag
		_, err = rest.UpdateTags(fr, rest.AddTag, []string{"infrastructure"})
		return eta, fmt.Errorf("Can't update tags on %q: %v", fr.Alias(), err)
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
					})
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
				})
				if err != nil {
					return eta, err
				}
				eta = res.Arrives.Time()
			}
		}
	case "empty":
		if rm.dev.Location != rm.supply.Location {
			log("Waiting for resupply at %q", rm.dev.Location)
		} else {
			if len(rm.supply.StowedDevices.Devices) == 0 {
				return eta, fmt.Errorf("Resupply vessage %q unexpectedly empty at %q",
					rm.supply.Code.Alias(), rm.dev.Location)
			}
			var stowed = 0
			for _, d := range rm.supply.StowedDevices.Devices {
				_, err := deviceCommand(d.Code, "deploy", nil)
				if err != nil {
					return eta, err
				}
				_, err = deviceCommand(d.Code, "stow", map[string]any{"target": rm.dev.Code})
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
			log("TODO: Queuing %d new FRs to be printed", rm.supply.StowCapacity)
			// TODO make a generic queue command
		}
	case "leaving":
		// plot the next hop
		route, err := common.PlotTrip(string(rm.dev.Location), rm.dest, nil)
		if err != nil {
			return eta, err
		}
		var lost = true
		for _, l := range route.Legs {
			log("Checking %s", l.String())
			if l.From != rm.dev.Location.Star() {
				continue
			}
			log("%s->%s (cur: %s)", l.From, l.To, rm.dev.Location)
			lost = false
			eta, err = common.Travel(rm.dev.Code, l.To, rm.dryRun)
			if err != nil {
				return eta, err
			}
			rm.dev.Location = models.LocationID(l.To)
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
		homeFRs, err := rest.RefreshDevices(map[string]string{
			"location":    home,
			"device_type": "ftl_relay",
		})
		if err != nil {
			return eta, fmt.Errorf("Can't find ftl relays at %q: %v", home, err)
		}
		if len(homeFRs) == 0 {
			return eta, fmt.Errorf("No FRs available at home")
		}
		log("Loading %d FRs at home, %d available...", slots, len(homeFRs))
		for slots > 0 && len(homeFRs) > 0 {
			_, err := deviceCommand(homeFRs[0].Code, "stow", map[string]any{
				"target": rm.supply.Code.String(),
			})
			if err != nil {
				return eta, err
			}
			slots--
			homeFRs = homeFRs[1:]
		}
		log("Shipping out to %q to deliver FRs", rm.dev.Location)
		eta, err := common.Travel(rm.supply.Code, string(rm.dev.Location), rm.dryRun)
		if err != nil {
			return eta, err
		}
		log("Supply ship in transit: %s (%s)", eta.Truncate(time.Second),
			time.Until(eta).Truncate(time.Second))
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
	return eta, nil
}

func (rm *RelayMachine) SaveState(string) error {
	return nil
}

func (rm *RelayMachine) Status() string {
	return ""
}
