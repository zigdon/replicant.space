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

// Design:
//   - racing vessel
//   - asc
//   - service bot
//   - 2+ scanning drones
//
// States:
// transit: not in a system
// incoming: system not fully explored
//   - scan
//   - deploy fleet
//   - start scanning ami
//   - if there's a belt, and it's not mined, add a TODO to send a fleet
//   - if there isn't a relay, add a TODO to deploy one
// scanning: devices deployed, system not fully explored
//   - Collect event logs from drones
//   - (move to stream) if a potentially interesting planet is explored, add a TODO to suggest it
// cleanup: devices deployed, system explored
//   - recall/stow devices
// leaving:
//   - if there's life, add a TODO to install a beacon
//   - find nearest unexplored star
//   - head there

type ExploreStates string

const (
	ExploreState_Initializing = "Initializing"
	ExploreState_Transit      = "In transit"
	ExploreState_Incoming     = "Incoming"
	ExploreState_Scanning     = "Scanning"
	ExploreState_Cleanup      = "Cleanup"
	ExploreState_Leaving      = "Leaving"
)

type ExploreMachine struct {
	dev    *models.Device
	asc    *models.Device
	sb     *models.Device
	sds    []*models.Device
	tag    string
	state  ExploreStates
	dryRun bool
	eta    time.Time
	status string
}

func (em *ExploreMachine) Start(dev *models.Device, dryRun bool) error {
	if !strings.Contains(dev.Type, "vessel") {
		return fmt.Errorf("Explorer state machine only supported on vessels, not %q", dev.Type)
	}
	em.dryRun = dryRun
	em.state = ExploreState_Initializing
	em.dev = dev
	em.status = "initializing"
	em.tag = fmt.Sprintf("explore:%s", dev.Code.Alias())
	log("Searching for support devices, tagged %q", em.tag)
	devs, err := rest.GetTagged(em.tag)
	if err != nil {
		return fmt.Errorf("Can't get devices with %q tag: %v", em.tag, err)
	}
	var ids []string
	for _, d := range devs.Devices {
		d, err = rest.DeviceInfo(d.Code)
		if err != nil {
			return err
		}
		switch d.Type {
		case "cargo_vessel", "heaven_vessel", "racing_vessel":
			// Ignore
		case "service_bot", "maintenence_drone":
			if em.sb != nil {
				return fmt.Errorf(
					"Only one service bot supported: found %s and %s",
					em.sb.Code.Alias(), d.Code.Alias())
			}
			em.sb = d
		case "ami_survey_controller":
			if em.asc != nil {
				return fmt.Errorf(
					"Only one asc supported: found %s and %s", em.asc.Code.Alias(), d.Code.Alias())
			}
			em.asc = d
		case "survey_drone":
			em.sds = append(em.sds, d)
		default:
			return fmt.Errorf("Unknown device tagged %q: %q", em.tag, d.Code)
		}
		if d.ReplicantCode.String() != em.dev.ReplicantCode.String() {
			_, err := deviceCommand(d.Code, "change_owner",
				map[string]any{"target": em.dev.ReplicantCode.String()}, em.dryRun)
			if err != nil {
				return err
			}
		}
	}

	if len(em.sds) < 2 {
		return fmt.Errorf("At least 2 survey drones are required: found %v", em.sds)
	}

	for _, sd := range em.sds {
		if sd.ControllerDeviceCode != nil && sd.ControllerDeviceCode.String() != em.asc.Code.String() {
			return fmt.Errorf(
				"%s is already controlled by %s, not %s",
				sd.Code.Alias(), sd.ControllerDeviceCode.Alias(), em.asc.Code.Alias())
		}
		if sd.ControllerDeviceCode == nil {
			ids = append(ids, sd.Code.String())
		}
	}

	if len(ids) > 0 {
		if _, err := deviceCommand(
			em.asc.Code, "adopt", map[string]any{"targets": ids}, em.dryRun); err != nil {
			return fmt.Errorf("asc can't adopt drones: %v", err)
		}
	}

	if em.sb == nil {
		return fmt.Errorf("One service bot or maintenence drone is required")
	}
	dir := "service"
	if em.sb.Type != "service_bot" {
		dir = "patrol"
	}
	if _, err := deviceCommand(
		em.sb.Code, "set_directive", map[string]any{"directive": dir}, em.dryRun); err != nil {
		return fmt.Errorf("Failed to set directive %q: %v", dir, err)
	}
	if _, err := deviceCommand(
		em.asc.Code, "set_directive", map[string]any{
			"directive": "survey_system",
			"configuration": map[string]any{
				"planets": "all",
				"moons":   "all",
				"recall":  true,
			}}, em.dryRun); err != nil {
		return fmt.Errorf("Failed to set asc directive: %v", err)
	}

	log("Expedition:")
	log("  %s (%s)", em.dev.Type, em.dev.Code)
	log("  %s (%s)", em.sb.Type, em.sb.Code)
	log("  %s (%s)", em.asc.Type, em.asc.Code)
	for _, sd := range em.sds {
		log("    %s (%s)", sd.Type, sd.Code)
	}

	return em.UpdateState()
}

func (em *ExploreMachine) UpdateState() error {
	// Update our state
	dev, err := rest.RefreshDeviceInfo(em.dev.Code)
	if err != nil {
		return err
	}
	em.dev = dev

	// Is the system fully explored?
	loc, err := rest.Location(dev.Location.Star())
	if err != nil {
		return err
	}
	var explored = loc.SystemScanned &&
		(loc.MoonsScanned == loc.MoonsTotal) &&
		(loc.PlanetsScanned == loc.PlanetsTotal)
	log("location %s: system: %v, moons: %d/%d, planets: %d/%d", dev.Location.Star(),
		loc.SystemScanned, loc.MoonsScanned, loc.MoonsTotal, loc.PlanetsScanned, loc.PlanetsTotal)

	// Are devices all stowed?
	var stowed = true
	for _, d := range append([]*models.Device{em.asc, em.sb}, em.sds...) {
		if !slices.ContainsFunc(dev.StowedDevices.Devices, func(sd *models.DevicePointer) bool {
			return sd.Code.String() == d.Code.String()
		}) {
			stowed = false
			break
		}
	}

	oldState := em.state
	log("State (%s): loc:%v ex:%v st:%v", em.state, dev.Location, explored, stowed)
	switch {
	case dev.Location == "":
		em.state = ExploreState_Transit
	case !explored && stowed:
		em.state = ExploreState_Incoming
	case !explored:
		em.state = ExploreState_Scanning
	case !stowed:
		em.state = ExploreState_Cleanup
	case stowed && explored:
		em.state = ExploreState_Leaving
	default:
		return fmt.Errorf("Unknown state!")
	}
	if em.state != oldState {
		log("Updated state: %s -> %s", oldState, em.state)
	}
	return nil
}

func (em *ExploreMachine) Process() (time.Time, error) {
	eta := time.Now().Add(5 * time.Minute)
	if err := em.UpdateState(); err != nil {
		return eta, err
	}
	nextState := em.state
	getLogs := func() error {
		for _, d := range em.sds {
			logs, err := rest.DeviceLogs(d.Code, 1)
			if err != nil {
				return err
			}
			if len(logs.Events) > 0 {
				log("  %s: %s", d.Code, logs.Events[0].Message)
			} else {
				log("  %s: No logs", d.Code)
			}
		}
		return nil
	}
	switch em.state {
	case ExploreState_Transit:
		em.status = "in transit"
		if t := em.dev.Travel; t != nil {
			em.status = fmt.Sprintf("in transit to %s", em.dev.Travel.Destination)
			eta = t.Arrives.Time()
		}
	case ExploreState_Incoming:
		em.status = fmt.Sprintf("setting up at %s", em.dev.Location)
		scan, err := rest.ReplicantScan(em.dev.ReplicantCode)
		if err != nil {
			return eta, err
		}
		if scan.AsteroidBelt.Present {
			// TODO - add a task to send a fleet
			for _, b := range scan.AsteroidBelt.Belts {
				log("TODO: Belt found: %s (%s)", b.Designation, b.Density)
			}
		}
		log("System scanned")
		if _, err := deviceCommand(em.sb.Code, "deploy", nil, em.dryRun); err != nil {
			return eta, err
		}
		if _, err := deviceCommand(em.asc.Code, "launch", nil, em.dryRun); err != nil {
			return eta, err
		}
		log("Devices deployed")
		frs, err := rest.Devices(map[string]string{
			"location":    em.dev.Location.Star(),
			"device_type": "ftl_relay",
		})
		if err != nil {
			return eta, err
		}
		if len(frs) == 0 {
			log("TODO: No FR found")
		} else {
			log("Found %d FRs", len(frs))
		}
		nextState = ExploreState_Scanning
		eta = time.Now()
	case ExploreState_Scanning:
		em.status = "scanning"
		log("Scan in progress:")
		if err := getLogs(); err != nil {
			return eta, err
		}
	case ExploreState_Cleanup:
		em.status = "cleaning up"
		log("Scan complete:")
		if err := getLogs(); err != nil {
			return eta, err
		}
		ids := []*models.CodeAlias{em.asc.Code, em.sb.Code}
		for _, d := range em.sds {
			ids = append(ids, d.Code)
		}
		for _, id := range ids {
			res, err := deviceCommand(id, "recall", nil, em.dryRun)
			if err != nil {
				if strings.Contains(err.Error(), "already stowed") {
					// not an error
					continue
				}
				log("Error recalling %q: %v", id, err)
				continue
			}
			if res.Route != nil {
				recall := time.Now().Add(res.Route[0].Time.Duration())
				if recall.After(eta) {
					eta = recall
				}
			}
		}
		log("Recalled all the devices")
	case ExploreState_Leaving:
		em.status = "leaving"
		// TODO: add a task to install a beacon on planets with life
		stars, err := rest.ReplicantCensus(em.dev.ReplicantCode, 50, 0)
		if err != nil {
			return eta, err
		}
		var next *models.Star
		for _, s := range stars.Stars {
			// TODO: check our cache so we'll go backfill stars we only skimmed earlier
			if s.Explored {
				continue
			}
			next = s
			break
		}
		if next == nil {
			return eta, fmt.Errorf("Can't find next star!")
		}
		log("Next star: %s, %.2f ly away", next.Designation.Star(), next.DistanceFromReplicant)
		eta, err = common.Travel(em.dev.Code, next.Designation.Star(), em.dryRun)
		if err != nil {
			return eta, err
		}
		nextState = ExploreState_Transit
	}

	if em.state != nextState {
		log("State: %s -> %s", em.state, nextState)
	}
	em.state = nextState
	em.eta = eta

	return eta, nil
}

func (em *ExploreMachine) SaveState(string) error {
	return nil
}

func (em *ExploreMachine) Status() string {
	return em.status
}
func (em *ExploreMachine) Name() string {
	return "Explorer"
}
