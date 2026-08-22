package auto

import (
	"fmt"
	"time"

	"github.com/zigdon/rsp/common"
	"github.com/zigdon/rsp/models"
	"github.com/zigdon/rsp/rest"
)

type FollowState string

const (
	FollowState_Initializing = "initializing"
	FollowState_Transit      = "transit"
	FollowState_Waiting      = "waiting"
	FollowState_Idle    = "idle"
	FollowState_Departing    = "departing"
)

type FollowMachine struct {
	dev        *models.Device
	target     *models.Device
	dest       models.LocationID
	state      FollowState
	status     string
	dryRun     bool
}

func (fm *FollowMachine) Start(d *models.Device, dryRun bool) error {
	fm.dev = d
	fm.dryRun = dryRun
	fm.status = "initializing"
	fm.state = FollowState_Initializing

	tags := getTags(d)
	targetID := tags["follow"]
	if targetID == "" {
		return fmt.Errorf("no 'follow:<id>' tag found on %s", d.Code.Alias())
	}
	targetCode := models.NewCodeAlias(targetID)

	target, err := rest.DeviceInfo(targetCode)
	if err != nil {
		return fmt.Errorf("can't get info for target device %q: %v", targetID, err)
	}
	fm.target = target

	return fm.UpdateState()
}

func (fm *FollowMachine) UpdateState() error {
	dev, err := rest.DeviceInfo(fm.dev.Code)
	if err != nil {
		return fmt.Errorf("can't get info for follower %q: %v", fm.dev.Code.Alias(), err)
	}
	fm.dev = dev

	targetID := getTags(fm.dev)["follow"]
	if targetID != "" && fm.dev.Code.Alias() != targetID {
		fm.target, err = rest.DeviceInfo(models.NewCodeAlias(targetID))
	} else {
		fm.target, err = rest.DeviceInfo(fm.dev.Code)
	}
	if err != nil {
		return fmt.Errorf("can't get info for target %q: %v", fm.dev.Code.Alias(), err)
	}

	// Determine the target's current location or destination
	if fm.target.Travel != nil && fm.target.Travel.Destination != "" {
		fm.dest = fm.target.Travel.Destination
	} else if fm.target.Location != "" {
		fm.dest = fm.target.Location
	} else {
		return fmt.Errorf("target %s has no known location or travel destination", fm.target.Code.Alias())
	}

	oldState := fm.state
	switch {
	case fm.dev.Location == "" || fm.dev.Travel != nil:
		fm.state = FollowState_Transit
		if fm.dev.Travel != nil {
			fm.status = fmt.Sprintf("in transit to %s", fm.dev.Travel.Destination)
		} else {
			fm.status = "in transit"
		}
	case fm.target.Travel != nil && fm.target.Travel.Destination != "":
		if fm.dev.Location.Star() == fm.dest.Star() {
			fm.state = FollowState_Waiting
			fm.status = fmt.Sprintf("waiting at %s for %s to arrive", fm.dev.Location, fm.target.Code.Alias())
		} else {
			fm.state = FollowState_Departing
			fm.status = fmt.Sprintf("following %s to %s", fm.target.Code.Alias(), fm.dest.Star())
		}
	case fm.target.Location != "":
		if fm.dev.Location.Star() == fm.target.Location.Star() {
			fm.state = FollowState_Idle
			fm.status = fmt.Sprintf("waiting for %s to leave %s", fm.target.Code.Alias(), fm.dev.Location.Star())
		} else {
			fm.state = FollowState_Departing
			fm.status = fmt.Sprintf("following %s to %s", fm.target.Code.Alias(), fm.dest.Star())
		}
	default:
		return fmt.Errorf("unknown follow state for %s", fm.dev.Code.Alias())
	}

	if oldState != fm.state {
		log("Follow state update: %s -> %s (%s)", oldState, fm.state, fm.status)
	}

	return nil
}

func (fm *FollowMachine) Process() (time.Time, error) {
	eta := time.Now().Add(time.Minute)
	if err := fm.UpdateState(); err != nil {
		return eta, err
	}

	log("State: %s (%s)", fm.state, fm.status)
	switch fm.state {
	case FollowState_Transit:
		if t := fm.dev.Travel; t != nil && t.Arrives != nil {
			eta = t.Arrives.Time()
			log("%s in transit to %s: arriving %s (%s)",
				fm.dev.Code.Alias(), t.Destination, eta.Format(time.Stamp), time.Until(eta).Truncate(time.Second))
		} else {
			eta = time.Now().Add(30 * time.Second)
		}
	case FollowState_Idle:
		log("%s idle at %s", fm.target.Code.Alias(), fm.dev.Location.Star())
		eta = time.Now().Add(time.Minute)
	case FollowState_Waiting:
		if fm.target.Travel != nil && fm.target.Travel.Arrives != nil {
			targetArrival := fm.target.Travel.Arrives.Time()
			log("%s waiting at %s for %s to arrive at %s (%s)",
				fm.dev, fm.dev.Location, fm.target, fm.dest, time.Until(targetArrival))
			eta = targetArrival.Add(5 * time.Second)
		} else {
			eta = time.Now().Add(time.Minute)
		}
	case FollowState_Departing:
		destSys := fm.dest.Star()
		log("%s departing to follow %s to %s", fm.dev, fm.target, destSys)
		travelEta, err := common.Travel(fm.dev.Code, destSys, fm.dryRun)
		if err != nil {
			return eta, fmt.Errorf("failed to follow %s to %s: %v", fm.target.Code.Alias(), destSys, err)
		}
		eta = travelEta
		fm.state = FollowState_Transit
		fm.status = fmt.Sprintf("in transit to %s (following %s)", destSys, fm.target.Code.Alias())
	default:
		return eta, fmt.Errorf("unknown state: %q", fm.state)
	}

	if !eta.IsZero() && eta.Before(time.Now()) {
		eta = time.Now().Add(time.Minute)
	}

	return eta, nil
}

func (fm *FollowMachine) SaveState(string) error {
	return nil
}

func (fm *FollowMachine) Status() string {
	return fm.status
}

func (fm *FollowMachine) Name() string {
	return "Follow Machine"
}
