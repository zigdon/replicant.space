package common

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/zigdon/rsp/models"
	"github.com/zigdon/rsp/rest"
)

type TravelCoordinator struct {
	queue  map[string]map[string][]*models.CodeAlias
	added  map[string]bool
	afc    *models.CodeAlias
	dryRun bool
	etas   map[string]map[string]time.Time
	oor    map[string]bool
}

func NewTravelCoordinator(afc *models.CodeAlias, dryRun bool) *TravelCoordinator {
	return &TravelCoordinator{
		afc:    afc,
		dryRun: dryRun,
		added:  make(map[string]bool),
		queue:  make(map[string]map[string][]*models.CodeAlias),
		etas:   make(map[string]map[string]time.Time),
		oor:    make(map[string]bool),
	}
}

func (tc *TravelCoordinator) Queue(ca *models.CodeAlias, from, to string) (time.Time, error) {
	if from == to {
		return time.Time{}, nil
	}
	if tc.oor[from] {
		Log("%s is out-of-range", from)
		return time.Time{}, nil
	}
	if _, ok := tc.queue[from]; !ok {
		tc.queue[from] = make(map[string][]*models.CodeAlias)
	}
	if _, ok := tc.etas[from]; !ok {
		tc.etas[from] = make(map[string]time.Time)
	}
	if tc.added[ca.String()] {
		Log("%s already has been queued to ship", ca.Alias())
		return tc.etas[from][to], nil
	}

	// Get the ETA for the trip
	if _, ok := tc.etas[from][to]; !ok {
		eta, err := Travel(ca, to, true)
		if err != nil {
			if strings.Contains(err.Error(), "Device is out of comms range") {
				tc.oor[from] = true
			}
			return tc.etas[from][to], fmt.Errorf("Error calculating trip to %s: %v", to, err)
		}
		tc.etas[from][to] = eta
	}
	Log("Adding %s to a %s->%s convoy", ca, from, to)
	tc.added[ca.String()] = true
	tc.queue[from][to] = append(tc.queue[from][to], ca)

	return tc.etas[from][to], nil
}

func (tc *TravelCoordinator) Ship() error {
	if tc.afc == nil {
		return fmt.Errorf("Can't ship without an AFC")
	}

	Log("Shipping manifest:")
	for from, v := range tc.queue {
		for to, ds := range v {
			Log("... %s -> %s: %d devices", from, to, len(ds))
		}
	}

	adopt := func(ids []*models.CodeAlias) error {
		var n int
		for len(ids) > 0 {
			n = min(len(ids), 100)
			_, err := tc.dc(tc.afc, "adopt", map[string]any{"devices": ids[:n]})
			if err != nil {
				Log("Failed to adopt %s: %v", ids, err)
			}
			ids = ids[n:]
		}
		return nil
	}

	release := func(ids []*models.CodeAlias) error {
		var n int
		for len(ids) > 0 {
			n = min(len(ids), 100)
			_, err := tc.dc(tc.afc, "release", map[string]any{"devices": ids[:n]})
			if err != nil {
				Log("Failed to release %s: %v", ids, err)
			}
			ids = ids[n:]
		}
		return nil
	}

	manual := func(ids []*models.CodeAlias, dest string) error {
		var errs []error
		Log("%d devices do not a convoy to %s make: %s", len(ids), dest, ids)
		for _, id := range ids {
			_, err := Travel(id, dest, tc.dryRun)
			errs = append(errs, err)
		}
		return errors.Join(errs...)
	}

	// Make sure we're stationary before starting anything
	afc, err := rest.RefreshDeviceInfo(tc.afc)
	if err != nil {
		return err
	}
	if afc.Location == "" {
		return fmt.Errorf("AFC in motion")
	}

	// Loop over the from/to pairs
	for from, dests := range tc.queue {
		for to, passengers := range dests {
			if len(passengers) == 0 {
				continue
			}

			afc, err := rest.DeviceInfo(tc.afc)
			if err != nil {
				return err
			}

			// Make sure there aren't any adopted devices
			if devs := afc.ControlledDevices; len(devs) > 0 {
				var ids []*models.CodeAlias
				for _, d := range devs {
					ids = append(ids, d.Code)
				}
				if err := release(ids); err != nil {
					return err
				}
			}

			// If there are 5 or fewer devices to ship, just ship them normally, done.
			if len(passengers) <= 5 {
				if err := manual(passengers, to); err != nil {
					return err
				}
				continue
			}
			Log("Shipping %d devices from %s to %s...", len(passengers), from, to)
			// Adopt all the devices that need to be shipped, batch 100 at a time
			for n := 0; n < len(passengers); n += 100 {
				// Make sure the AFC is settled before we start
				for {
					afc, err = rest.RefreshDeviceInfo(afc.Code)
					if err != nil {
						return err
					}
					if afc.Location != "" {
						break
					}
					time.Sleep(time.Second)
				}
				var ids []*models.CodeAlias
				if n+100 < len(passengers) {
					ids = passengers[n : n+100]
				} else {
					ids = passengers[n:]
				}
				if err := adopt(ids); err != nil {
					return err
				}
				// Send travel command to the afc
				Log("Launching convoy %s->%s: %s", from, to, ids)
				if string(afc.Location) != to {
					if _, err := Travel(afc.Code, to, tc.dryRun); err != nil {
						return fmt.Errorf("Failed to send AFC: %v", err)
					}
				} else {
					if _, err := tc.dc(afc.Code, "assemble", nil); err != nil {
						return fmt.Errorf("Failed to assemble fleet to AFC: %v", err)
					}
				}
				// Punt all the devices
				var errs []error
				errs = append(errs, release(ids))
				// Even if there's an error, abort the AFC's travel
				if string(afc.Location) != to {
					_, err = tc.dc(afc.Code, "deactivate", nil)
					errs = append(errs, err)
				}
				if err := errors.Join(errs...); err != nil {
					return fmt.Errorf("Failed to punt %d devices to %s: %v", len(ids), to, err)
				}
			}
			delete(tc.queue[from], to)

			// Wait until the AFC is stationary again, only poll the DB, it should be
			// updated by the return event.
			Log("Waiting for %s to return", afc.Code)
			check := time.Now()
			var fn func(*models.CodeAlias) (*models.Device, error)
			for {
				if time.Since(check) > 5*time.Second {
					fn = rest.RefreshDeviceInfo
					check = time.Now()
				} else {
					fn = rest.DeviceInfo
				}
				afc, err := fn(tc.afc)
				if err != nil {
					Log("Error getting AFC info: %v", err)
				}
				if afc != nil && afc.Location != "" {
					Log("AFC is stationary at %s", afc.Location)
					break
				}
				time.Sleep(200 * time.Millisecond)
			}
		}
	}
	return nil
}

func (tc *TravelCoordinator) dc(id *models.CodeAlias, cmd string, cfg map[string]any) (*models.CommandResp, error) {
	if tc.dryRun {
		Log("[DRYRUN] %q -> %q (%v)", cmd, id, cfg)
		return new(models.CommandResp), nil
	}
	return rest.DeviceCommand[models.CommandResp](id, cmd, cfg)
}
