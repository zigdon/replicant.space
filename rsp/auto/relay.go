package auto

import (
	"fmt"
	"slices"
	"strings"
	"time"

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
//
// return: build new FRs, fly home
// resupply: slurp all FRs
// transit: wait
// incoming: go to L4 point
// cleanup: pick up any FRs that are not releying, remove tags
// leaving: find the next system, head there
// activating: deploy, activate, tag

const home = "MENKUNT-2-L4"

type RelayMachine struct {
	dryRun bool
	dev    *models.Device
	dest   string
	state  string
}

func (rm *RelayMachine) Start(d *models.Device, dryRun bool) error {
	// Make sure the device is a replicant
	if !slices.Contains([]string{"heaven_vessel", "racing_vessel", "cargo_vessel"}, d.Type) {
		return fmt.Errorf("%s is not a vessel", d.Code.Alias())
	}
	rm.dev = d
	rm.dryRun = dryRun
	dest := getTags(rm.dev)["relay"]
	if dest == "" {
		return fmt.Errorf("Relay destination not tagged on %s", d.Code.Alias())
	}
	rm.dest = dest
	return rm.UpdateState()
}

func (rm *RelayMachine) UpdateState() error {
	dev, err := rest.DeviceInfo(rm.dev.Code)
	if err != nil {
		return err
	}
	rm.dev = dev
	status := rm.dev.Status
	frInv := slices.ContainsFunc(rm.dev.StowedDevices.Devices, func(d *models.StowedDevice) bool {
		return d.Type == "ftl_relay"
	})
	var star *models.Star
	if s, _, ok := strings.Cut(rm.dev.Location, "-"); ok {
		star = &models.Star{Designation: s}
		if err := star.Get(); err != nil {
			return err
		}
	}
	atHome := rm.dev.Location == home
	var sysFRs []*models.Device
	var sysFRRelaying bool
	if star != nil {
		sysFRs, err := rest.Devices(map[string]string{
			"device_type": "ftl_relay",
			"location":    star.Designation,
		})
		if err != nil {
			return err
		}
		for _, fr := range sysFRs {
			if fr.Status == "relaying" {
				sysFRRelaying = true
				break
			}
		}
	}
	sysHasFR := len(sysFRs) > 0
	sysHasSpareFR := len(sysFRs) > 1

	switch {
	case atHome:
		rm.state = "resupply"
	case !frInv:
		rm.state = "return"
	case status != "idle":
		rm.state = "transit"
	case rm.dev.Location != star.EntryPoint:
		rm.state = "incoming"
	case sysHasSpareFR:
		rm.state = "cleanup"
	case sysFRRelaying:
		rm.state = "leaving"
	case sysHasFR:
		rm.state = "activating"
	default:
		return fmt.Errorf("Unknown state (%s): FRs: ship %v, sys %d (relaying: %v)",
			rm.dev.Code.Alias(), frInv, len(sysFRs), sysFRRelaying)
	}
	return nil
}

func (rm *RelayMachine) Process() (time.Time, error) {
	var eta time.Time
	switch rm.state {
	case "return":
	case "resupply":
	case "transit":
	case "incoming":
	case "cleanup":
	case "leaving":
	case "activating":
	default:
		return eta, fmt.Errorf("Unknown state: %q", rm.state)
	}
	return eta, nil
}

func (rm *RelayMachine) SaveState(string) error {
	return nil
}

func (rm *RelayMachine) Status() string {
	return ""
}
