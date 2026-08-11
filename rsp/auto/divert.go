package auto

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/zigdon/rsp/common"
	"github.com/zigdon/rsp/models"
	"github.com/zigdon/rsp/rest"
)

// States:
// incoming: mf has devices, at an active object site
//   - detach all
//   - activate all the propulsors
// working: mf has no devices, at an active site
//   - wait
// cleanup: mf is at an inactive site, some devices not attached
//   - attach all propulsors
//   - "recall" rocks mtd in the system
// departing: mf has all devices, inactive site
//   - find closest active site, head there
//   - If there isn't one, keep waiting
// travelling: in tranit
//   - wait

type DivertMachine_State string

const (
	DivertMachine_Transit  = "transit"
	DivertMachine_Incoming = "incoming"
	DivertMachine_Working  = "working"
	DivertMachine_Cleanup  = "cleanup"
	DivertMachine_Leaving  = "leaving"
)

type DivertMachine struct {
	dev    *models.Device
	mtd    *models.Device
	dryRun bool
	state  string
	status string
}

func (dm *DivertMachine) Start(d *models.Device, dryRun bool) error {
	if d.Type != "mobile_fleet" {
		return fmt.Errorf("Invalid device for diverting: %q is a %q, not a mobile fleet", d.Code, d.Type)
	}
	dm.dev = d
	dm.dryRun = dryRun
	dm.status = "initializing"
	for _, dev := range dm.dev.AttachedDevices {
		if dev.Type != "maintenance_drone" {
			continue
		}
		dm.mtd = dev
		break
	}
	if dm.mtd == nil && dm.dev.Location != "" {
		mtds, err := rest.RefreshDevices(map[string]string{"device_type": "maintenance_drone", "tag": fmt.Sprintf("support:%s", dm.dev.Code.Alias())})
		if err != nil {
			return err
		}
		if len(mtds) == 0 {
			return fmt.Errorf("Could not find mtd for %q at %q", dm.dev.Code.Alias(), dm.dev.Location.Star())
		}
		dm.mtd = mtds[0]
	}
	if dm.mtd == nil {
		return fmt.Errorf("Can't find a mtd for %s @ %s", dm.dev.Code.Alias(), dm.dev.Location)
	}

	dm.state = "initializing"

	return dm.UpdateState()
}

func (dm *DivertMachine) UpdateState() error {
	dev, err := rest.RefreshDeviceInfo(dm.dev.Code)
	if err != nil {
		return fmt.Errorf("Can't refresh info for %q: %v", dm.dev.Code.Alias(), err)
	}
	dm.dev = dev
	mtd, err := rest.RefreshDeviceInfo(dm.mtd.Code)
	if err != nil {
		return fmt.Errorf("Can't refresh info for %q: %v", dm.mtd.Code.Alias(), err)
	}
	dm.mtd = mtd

	// State flags
	hasDevices := len(dev.AttachedDevices) > 0
	hasMtd := slices.ContainsFunc(dev.AttachedDevices, func(d *models.Device) bool {
		return d.Type == "maintenance_drone"
	})
	inTransit := dev.Location == ""
	var activeSite bool
	if !inTransit {
		loc, err := rest.Location(string(dev.Location))
		if err != nil {
			return err
		}
		activeSite = loc.Object != nil && loc.Object.Status == "active"
	}
	log("State: %s@%s, devs:%v mtd:%v transit:%v active:%v",
		dev, dev.Location, hasDevices, hasMtd, inTransit, activeSite)
	oldState := dm.state
	switch {
	case inTransit:
		dm.state = DivertMachine_Transit
	case hasDevices && activeSite:
		dm.state = DivertMachine_Incoming
	case activeSite:
		dm.state = DivertMachine_Working
	case !hasMtd:
		dm.state = DivertMachine_Cleanup
	case hasMtd:
		dm.state = DivertMachine_Leaving
	default:
		return fmt.Errorf("Unknown state")
	}
	if oldState != dm.state {
		log("State updated: %q -> %q", oldState, dm.state)
	}
	return nil
}

func (dm *DivertMachine) Process() (time.Time, error) {
	eta := time.Now()
	if err := dm.UpdateState(); err != nil {
		return eta, err
	}
	nextState := dm.state
	log("State: %s", dm.state)
	switch dm.state {
	case DivertMachine_Transit:
		if t := dm.dev.Travel; t != nil {
			eta = later(eta, t.Arrives.Time())
		} else {
			eta = later(eta, time.Now().Add(5*time.Minute))
		}
	case DivertMachine_Incoming:
		res, err := deviceCommand(dm.dev.Code, "detach", nil, dm.dryRun)
		if err != nil {
			return eta, err
		}
		// If we're in dry run, we won't actually get the list of devices from
		// the command, so lets construct it manually.
		if dm.dryRun {
			ps, err := rest.Devices(map[string]string{"device_type": "propulsor", "location": dm.dev.Location.Star()})
			if err != nil {
				return eta, err
			}
			if res.Detached == nil {
				res.Detached = new(models.StowedDevices)
			}
			for _, p := range ps {
				res.Detached.Devices = append(res.Detached.Devices, &models.DevicePointer{
					Code: p.Code,
					Type: "propulsor",
				})
			}
		}

		var errs []error
		if res.Detached != nil {
			var ids []string
			for _, d := range res.Detached.Devices {
				ids = append(ids, d.Code.Alias())
				if d.Code.String() == dm.mtd.Code.String() {
					continue
				}
				_, err = deviceCommand(d.Code, "activate", nil, dm.dryRun)
				errs = append(errs, err)
			}
			slices.Sort(ids)
			log("Detached %d devices: %s", len(res.Detached.Devices), strings.Join(ids, ", "))
			if err := errors.Join(errs...); err != nil {
				log("Errors during activate:\n%v", errs)
				return eta, err
			}
		}

		nextState = DivertMachine_Working
	case DivertMachine_Working:
		obj, err := rest.Location(string(dm.dev.Location))
		if err != nil {
			return eta, err
		}
		need := obj.Object.RequiredStrength
		have := obj.Object.CurrentThrustPerHour
		est := time.Duration(need/have*3600) * time.Second
		// Since the estimate varies a lot, lets set the ETA at 50% of when it should be.
		if est > 0 {
			log("Diversion in progress, with %.2f/h of %.2f, ETA: %s (%s)",
				have, need, time.Now().Add(est), est)
			eta = later(eta, time.Now().Add(est/2))
		} else {
			log("Diversion in progress, need %.2f, no ETA yet", need)
		}
	case DivertMachine_Cleanup:
		props, err := rest.Devices(map[string]string{"device_type": "propulsor", "location": dm.dev.Location.Star()})
		if err != nil {
			return eta, err
		}
		var ids []string
		for _, p := range props {
			if p.AttachedToDeviceCode != nil {
				continue
			}
			ids = append(ids, p.Code.String())
		}
		if len(ids) > 0 && len(dm.dev.AttachedDevices) == 0 {
			if len(ids) > 30 {
				ids = ids[:30]
			}
			res, err := deviceCommand(dm.dev.Code, "attach", map[string]any{"targets": ids}, dm.dryRun)
			if err != nil {
				return eta, err
			}
			log("Attached %d propulsors", len(res.AttachedDevices))
		}
		log("%s is at %s", dm.mtd, dm.mtd.Location)
		if dm.mtd.Location != dm.dev.Location {
			mtdEta, err := common.Travel(dm.mtd.Code, string(dm.dev.Location), dm.dryRun)
			if err != nil {
				return eta, err
			}
			eta = later(eta, mtdEta)
			log("Recalling %q from %q, ETA: %s (%s)", dm.mtd, dm.mtd.Location, eta, time.Until(eta))
		} else if dm.mtd.AttachedToDeviceCode == nil {
			_, err := deviceCommand(dm.dev.Code, "attach", map[string]any{"target": dm.mtd.Code.String()}, dm.dryRun)
			if err != nil {
				return eta, err
			}
			nextState = DivertMachine_Leaving
		} else {
			nextState = DivertMachine_Leaving
		}
	case DivertMachine_Leaving:
		rocks, err := common.GetRocks()
		if err != nil {
			return eta, err
		}
		// Check where other fleets are heading, to avoid clustering
		mfs, err := rest.RefreshDevices(map[string]string{"device_type": "mobile_fleet", "tag": "rocks"})
		if err != nil {
			return eta, err
		}
		taken := make(map[string]string)
		log("Checking other rock fleets...")
		for _, mf := range mfs {
			if mf.Travel == nil {
				log("... skipping %s, not moving", mf)
				continue
			}
			log("... %s is on the way to %s", mf, mf.Travel.Destination.Star())
			taken[mf.Travel.Destination.Star()] = mf.Code.Alias()
		}
		var closest float32
		getNext := func(noGang bool) (*models.Object, error) {
			var next *models.Object
			for _, r := range rocks {
				if r.Status != "active" {
					continue
				}
				if mf, ok := taken[r.Designation.Star()]; ok {
					if noGang {
						log("%s has dibs on %s", mf, r.Designation)
						continue
					}
					log("Ignoring %s dibs on %s", mf, r.Designation)
				}
				dist, err := common.Distance(dm.dev.Location.Star(), r.Designation.Star())
				if err != nil {
					return nil, err
				}
				if closest == 0 || dist < closest {
					closest = dist
					next = r
				}
			}
			return next, nil
		}
		// See what's the next rock we can have for ourselves
		next, err := getNext(true)
		if err != nil {
			return eta, err
		}
		// See what rock we can help with
		if next == nil {
			next, err = getNext(false)
			if err != nil {
				return eta, err
			}
		}
		if next == nil {
			log("No more rocks incoming, waiting...")
			return time.Now().Add(10 * time.Minute), nil
		}
		log("Next rock: %s, %.2f LY away", next.Short(), closest)

		newEta, err := common.Travel(dm.dev.Code, string(next.Designation), dm.dryRun)
		if err != nil {
			return eta, err
		}
		eta = later(eta, newEta)
		nextState = DivertMachine_Transit
	default:
		return eta, fmt.Errorf("Unknown state: %q", dm.state)
	}

	if nextState != dm.state {
		log("State change: %q -> %q", dm.state, nextState)
		dm.state = nextState
	}

	return eta, nil
}

func (dm *DivertMachine) SaveState(string) error {
	return nil
}

func (dm *DivertMachine) Status() string {
	return dm.status
}

func (dm *DivertMachine) Name() string {
	return "Rock Diverting Machine"
}
