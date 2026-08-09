package auto

import (
	"fmt"
	"slices"
	"time"

	"github.com/zigdon/rsp/common"
	"github.com/zigdon/rsp/models"
	"github.com/zigdon/rsp/rest"
)

// if we don't have beacons, get some delivered
// if there are not enough beacons to fill a ship, print more
// if we're in a system, there's intelligent life on a planet, and it doesn't have a beacon
// - travel to that planet
// - scan it
// - deploy a beacon
// - tag it
// find the nearest system with life that has a networked relay and is missing
// a beacon, head there
//
// States:
// cargo vessel:
//   transit: wait
//   scanning: if not scanned, scan
//   incoming: check for life, {go to planet/ or skip to leaving}
//   deploying: deploy, tag
//   empty: meet mf at nearest relay system, move FBs from mf
//   leaving: find the next system, head there
// mobile fleet:
//   empty: head home, queue FBs
//   resupplying: attach until full capacity
//   full: follow cv

type BeaconStates string

const (
	BeaconStates_Transit   = "In transit"
	BeaconStates_Scanning  = "Scanning"
	BeaconStates_Incoming  = "Incoming"
	BeaconStates_Deploying = "Deploying"
	BeaconStates_Empty     = "Empty"
	BeaconStates_Cleanup   = "Cleanup"
	BeaconStates_Leaving   = "Leaving"
)

type BeaconMachine struct {
	dryRun    bool
	dev       *models.Device
	supply    *models.Device
	scan      *models.Device
	state     string
	replicant *models.CodeAlias
	missingFB map[string]bool
	needsScan map[string]bool
	status    string
}

func (bm *BeaconMachine) Start(d *models.Device, dryRun bool) error {
	bm.status = "initializing"

	// Make sure the device is a vessel
	if !slices.Contains([]string{"heaven_vessel", "racing_vessel", "cargo_vessel"}, d.Type) {
		return fmt.Errorf("%s is not a vessel: %q", d.Code.Alias(), d.Type)
	}
	bm.dev = d

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
		bm.replicant = dev.ReplicantCode
		break
	}
	if bm.replicant == nil {
		return fmt.Errorf("No replicant found in %q", d.Code.Alias())
	}

	bm.dryRun = dryRun

	mf, err := rest.GetTagged(fmt.Sprintf("supply:%s", d.Code.Alias()))
	if err != nil {
		return fmt.Errorf("Can't get tagged supply ship: %v", err)
	}
	if len(mf.Devices) != 1 {
		return fmt.Errorf("Can't find exactly one device tagged supply:%s, found %d", d.Code.Alias(), len(mf.Devices))
	}
	bm.supply = mf.Devices[0]

	sd, err := rest.GetTagged(fmt.Sprintf("scan:%s", d.Code.Alias()))
	if err != nil {
		return fmt.Errorf("Can't get tagged survey drone: %v", err)
	}
	if len(sd.Devices) != 1 {
		return fmt.Errorf("Can't find exactly one device tagged scan:%s, found %d", d.Code.Alias(), len(sd.Devices))
	}
	bm.scan = sd.Devices[0]

	return bm.UpdateState()
}

func (bm *BeaconMachine) UpdateState() error {
	dev, err := rest.RefreshDeviceInfo(bm.dev.Code)
	if err != nil {
		return fmt.Errorf("Can't refresh info for %q: %v", bm.dev.Code.Alias(), err)
	}
	bm.dev = dev
	status := bm.dev.Status

	supply, err := rest.RefreshDeviceInfo(bm.supply.Code)
	if err != nil {
		return fmt.Errorf("Can't refresh info for supply ship %q: %v", bm.supply.Code.Alias(), err)
	}
	bm.supply = supply

	scan, err := rest.RefreshDeviceInfo(bm.scan.Code)
	if err != nil {
		return fmt.Errorf("Can't refresh info for survey drone %q: %v", bm.scan.Code.Alias(), err)
	}
	bm.scan = scan

	// State flags
	// Has the system been scanned?
	loc, err := rest.Location(dev.Location.Star())
	if err != nil {
		return err
	}
	isScanned := loc.SystemScanned

	// Check FB inventory
	fbInv := slices.ContainsFunc(bm.dev.StowedDevices.Devices, func(d *models.DevicePointer) bool {
		return d.Type == "ftl_beacon"
	})

	// Find beacons already deployed
	hasBeacon := make(map[string]bool)
	beacons, err := rest.Devices(map[string]string{"location": dev.Location.Star(), "device_type": "ftl_beacon"})
	if err != nil {
		return err
	}
	for _, b := range beacons {
		hasBeacon[string(b.Location)] = true
	}
	log("Found %d beacons", len(hasBeacon))

	// Find life in the system that is missing a beacon
	lifeStages := []string{"spacefaring", "intelligent"}
	if bm.missingFB == nil {
		bm.missingFB = make(map[string]bool)
	}
	if bm.needsScan == nil {
		bm.needsScan = make(map[string]bool)
	}
	for _, p := range loc.Planets {
		if !p.Scanned {
			bm.needsScan[string(p.Designation)] = true
		}
		if !slices.Contains(lifeStages, p.LifeStage) {
			continue
		}
		if hasBeacon[string(p.Designation)] {
			continue
		}
		bm.missingFB[string(p.Designation)] = true
	}

	log("State: %s@%s, %s@%s, %s@%s; System Scanned: %v, not scanned: %d, missing: %v",
		bm.dev.Code.Alias(), bm.dev.Location,
		bm.supply.Code.Alias(), bm.supply.Location,
		bm.scan.Code.Alias(), bm.scan.Location,
		isScanned, len(bm.needsScan), bm.missingFB)

	oldState := bm.state
	switch {
	case bm.dev.Location == "" || status != "idle":
		log("In transit")
		bm.state = BeaconStates_Transit
	case bm.dev.Location == home:
		log("Leaving home")
		bm.state = BeaconStates_Leaving
	case bm.state == "" && status == "idle":
		log("Blank state, stationary")
		bm.state = BeaconStates_Incoming
	case bm.state == "" && status != "idle":
		log("Blank state, moving")
		bm.state = BeaconStates_Transit
	case !fbInv:
		log("No more beacons")
		bm.state = BeaconStates_Empty
	case !isScanned:
		log("System not scanned")
		bm.state = BeaconStates_Scanning
	case bm.missingFB[string(bm.dev.Location)]:
		bm.state = BeaconStates_Deploying
	case len(bm.missingFB) > 0 && !bm.missingFB[string(bm.dev.Location)]:
		bm.state = BeaconStates_Incoming
	case bm.scan.StowedInDeviceCode == nil:
		log("Recalling survey drone")
		bm.state = BeaconStates_Cleanup
	case bm.scan.StowedInDeviceCode != nil:
		log("All done")
		bm.state = BeaconStates_Leaving
	default:
		return fmt.Errorf("Unknown state (%s)", bm.dev.Code.Alias())
	}
	log("Update state: %s -> %s", oldState, bm.state)
	return nil
}

func (bm *BeaconMachine) Process() (time.Time, error) {
	eta := time.Now().Add(30 * time.Second)
	if err := bm.UpdateState(); err != nil {
		return eta, err
	}
	nextState := bm.state
	log("State: %s", bm.state)
	switch bm.state {
	case BeaconStates_Transit:
		if t := bm.dev.Travel; t != nil {
			eta = t.Arrives.Time()
		}
	case BeaconStates_Scanning:
		scan, err := rest.ReplicantScan(bm.replicant)
		if err != nil {
			return eta, fmt.Errorf("Can't trigger scan at %q: %v", bm.dev.Location, err)
		}
		log("System scanned")
		if scan.AsteroidBelt.Present {
			log("Asteroid belt detected: %v", scan.AsteroidBelt.Belts)
		}
		nextState = BeaconStates_Incoming
	case BeaconStates_Incoming:
		if bm.missingFB[string(bm.dev.Location)] {
			if bm.needsScan[string(bm.dev.Location)] {
				if _, err := deviceCommand(bm.scan.Code, "deploy", nil, bm.dryRun); err != nil {
					return eta, fmt.Errorf("Can't deploy survey drone: %v", err)
				}
				res, err := deviceCommand(bm.scan.Code, "scan", nil, bm.dryRun)
				if err != nil {
					return eta, fmt.Errorf("Can't scan %q with %s: %v", bm.dev.Location, bm.scan.Code, err)
				}
				eta = res.Completes.Time()
				log("Scan started, ETA %s (%s)", eta, time.Until(eta))
			} else {
				if bm.scan.StowedInDeviceCode == nil {
					if res, err := deviceCommand(bm.scan.Code, "recall", nil, bm.dryRun); err != nil {
						return eta, fmt.Errorf("Can't recall %q: %v", bm.scan.Code.Alias(), err)
					} else {
						log("Recalled %q: %s", bm.scan.Code.Alias(), res.Status)
					}
				}
				nextState = BeaconStates_Deploying
			}
		} else if len(bm.missingFB) > 0 {
			var dests []string
			for k := range bm.missingFB {
				dests = append(dests, string(k))
			}
			slices.Sort(dests)
			var err error
			eta, err = common.Travel(bm.dev.Code, dests[0], bm.dryRun)
			if err != nil {
				return eta, err
			}
			nextState = BeaconStates_Incoming
		} else {
			log("Done with %s", bm.dev.Location.Star())
			nextState = BeaconStates_Cleanup
		}
	case BeaconStates_Deploying:
		var fb *models.CodeAlias
		for _, d := range bm.dev.StowedDevices.Devices {
			if d.Type != "ftl_beacon" {
				continue
			}
			fb = d.Code
			break
		}
		if fb == nil {
			bm.state = BeaconStates_Empty
			return eta, fmt.Errorf("No FB found in hold")
		}
		log("Deploying beacon (%s) at %s", fb.Alias(), bm.dev.Location)
		if _, err := deviceCommand(fb, "deploy", nil, bm.dryRun); err != nil {
			return eta, err
		}
		if _, err := rest.UpdateTags(fb, rest.AddTag, []string{"infrastructure"}); err != nil {
			return eta, err
		}
		delete(bm.missingFB, string(bm.dev.Location))
	case BeaconStates_Empty:
		if bm.dev.Location != bm.supply.Location {
			relay, err := common.NearestRelay(bm.dev.Location.Star())
			if err != nil {
				return eta, err
			}
			log("Heading to %s to wait for resupply", relay)
			eta, err = common.Travel(bm.dev.Code, relay, bm.dryRun)
			if err != nil {
				return eta, err
			}
		} else {
			if len(bm.supply.AttachedDevices) == 0 {
				return eta, fmt.Errorf("Resupply vessage %q unexpectedly empty at %q",
					bm.supply.Code.Alias(), bm.dev.Location)
			}
			var stowed = 0
			for _, d := range bm.supply.AttachedDevices {
				_, err := deviceCommand(bm.supply.Code, "detach",
					map[string]any{"target": d.Code.Alias()}, bm.dryRun)
				if err != nil {
					return eta, err
				}
				_, err = deviceCommand(d.Code, "stow",
					map[string]any{"target": bm.dev.Code}, bm.dryRun)
				if err != nil {
					return eta, err
				}
				stowed++
			}
			log("Picked up %d BRs, shipping resupply back home", stowed)
			var err error
			eta, err = common.Travel(bm.supply.Code, home, bm.dryRun)
			if err != nil {
				return eta, err
			}
			homeFBs, err := rest.Devices(map[string]string{"location": home, "device_type": "ftl_beacon"})
			if err != nil {
				return eta, err
			}
			log("Found %d beacons at home", len(homeFBs))
			if len(homeFBs) < bm.supply.AttachCapacity {
				need := bm.supply.AttachCapacity - len(homeFBs)
				pPlan, err := common.Print(home, "ftl_beacon", need, true, bm.dryRun, nil)
				if err != nil {
					log("Error printing beacons: %v", err)
				} else {
					log("Queued %d ftl_beacons: ETA %s (%s)", need, pPlan.ETA, time.Until(pPlan.ETA))
				}
			}

			nextState = BeaconStates_Cleanup
		}
	case BeaconStates_Cleanup:
		if bm.scan.StowedInDeviceCode == nil {
			if res, err := deviceCommand(bm.scan.Code, "recall", nil, bm.dryRun); err != nil {
				return eta, fmt.Errorf("Can't recall %q: %v", bm.scan.Code.Alias(), err)
			} else {
				log("Recalled %q: %s", bm.scan.Code.Alias(), res.Status)
				if res.Arrives != nil {
					eta = res.Arrives.Time()
				}
			}
		} else {
			nextState = BeaconStates_Leaving
		}
	case BeaconStates_Leaving:
		// Find all the deployed beacons
		beacons := make(map[string]bool)
		log("Finding all ftl_beacons...")
		res, err := rest.Devices(map[string]string{
			"device_type": "ftl_beacon",
		})
		if err != nil {
			return eta, err
		}
		for _, d := range res {
			if d.Status != "monitoring" {
				continue
			}
			beacons[string(d.Location)] = true
		}
		log("... %d found", len(beacons))

		log("Finding nearest systems with life...")
		pos := bm.dev.GetPosition()
		rows, err := DB.DB.Query(`
		  SELECT * FROM (
			SELECT p.designation,
			  SQRT(POWER(position_x-$1, 2) + POWER(position_y-$2, 2) + POWER(position_z-$3,2)) AS dist
			FROM planets p JOIN stars s ON p.star = s.designation
			WHERE (life_stage = 'intelligent' or life_stage = 'spacefaring'))
		  WHERE dist > 0.1
		  ORDER BY dist;
		`, pos.X, pos.Y, pos.Z)
		if err != nil {
			return eta, err
		}
		var next string
		var dist float32
		for rows.Next() {
			if err := rows.Scan(&next, &dist); err != nil {
				return eta, err
			}
			if !beacons[next] {
				break
			}
		}
		if beacons[next] {
			st, _ := models.NewStar(home)
			log("All beaconed up, heading home")
			next = home
			dist = bm.dev.GetPosition().Distance(st.Position)
		} else {
			log("Next stop, %s, %.2f LY away", next, dist)
		}
		eta, err = common.Travel(bm.dev.Code, next, bm.dryRun)
		if err != nil {
			return eta, err
		}
	default:
		return eta, fmt.Errorf("Unknown state: %q", bm.state)
	}

	// Handle supply vessal
	// When not home, hang out at the edge of the relay network
	devLoc := bm.dev.Location
	if devLoc == "" && bm.dev.Travel != nil {
		devLoc = bm.dev.Travel.Destination
	}
	relay, err := common.NearestRelay(devLoc.Star())
	if err != nil {
		return eta, err
	}

	switch {
	case bm.supply.Location == "":
		log("Resupply platform in transit...")
	case bm.supply.Location == home:
		slots := bm.supply.AttachCapacity - len(bm.supply.AttachedDevices)
		devs, err := rest.RefreshDevices(map[string]string{
			"location":    home,
			"device_type": "ftl_beacon",
		})
		if err != nil {
			return eta, fmt.Errorf("Can't find ftl beacons at %q: %v", home, err)
		}
		var homeFBs []*models.Device
		for _, d := range devs {
			if d.AttachedToDeviceCode != nil {
				continue
			}
			homeFBs = append(homeFBs, d)
		}
		if len(homeFBs) == 0 {
			log("No FBs available at home")
		}
		log("Loading %d FBs at home, %d available...", slots, len(homeFBs))
		if slots > 0 && len(homeFBs) > 0 {
			if slots < len(homeFBs) {
				homeFBs = homeFBs[:slots]
			}
			ids := make([]string, len(homeFBs))
			for n := range homeFBs {
				ids[n] = homeFBs[n].Code.Alias()
			}
			_, err := deviceCommand(bm.supply.Code, "attach", map[string]any{
				"targets": ids,
			}, bm.dryRun)
			if err != nil {
				return eta, err
			}
		}
		if len(bm.supply.AttachedDevices) > 0 {
			log("Shipping out to %q to deliver FBs", relay)
			eta, err := common.Travel(bm.supply.Code, relay, bm.dryRun)
			if err != nil {
				return eta, err
			}
			log("Supply ship in transit: %s (%s)", eta, time.Until(eta))
		} else {
			log("Supply ship waiting for new beacons")
		}
	case bm.supply.Location.Star() == relay:
		log("Waiting to resupply at %q", relay)
	default:
		log("Restaging to %s", relay)
		eta, err := common.Travel(bm.supply.Code, relay, bm.dryRun)
		if err != nil {
			return eta, err
		}
		log("Supply ship in transit: %s (%s)", eta, time.Until(eta))
	}

	if nextState != bm.state {
		log("Shifting state %s -> %s", bm.state, nextState)
		bm.state = nextState
	}

	return eta, nil
}

func (rm *BeaconMachine) SaveState(string) error {
	return nil
}

func (rm *BeaconMachine) Status() string {
	return rm.status
}

func (rm *BeaconMachine) Name() string {
	return "Beacon Machine"
}
