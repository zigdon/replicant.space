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
		return fmt.Errorf("%s is not a vessel", d.Code.Alias())
	}
	rm.dev = d
	rm.dryRun = dryRun
	dest := getTags(rm.dev)["relay"]
	if dest == "" {
		return fmt.Errorf("Relay destination not tagged on %s", d.Code.Alias())
	}
	rm.dest = dest

	p, err := rest.GetTagged(fmt.Sprintf("supply:%s", d.Code.Alias()))
	if err != nil {
		return err
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
		return err
	}
	rm.dev = dev
	status := rm.dev.Status

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
		star, err = models.NewStar(string(rm.dev.Location.Star()))
		if err != nil {
			return err
		}

		if star != nil {
			sysFRs, err := rest.Devices(map[string]string{
				"device_type": "ftl_relay",
				"location":    string(star.Designation),
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
	}
	sysHasSpareFR := len(sysFRs) > 1

	switch {
	case rm.dev.Location == "" || status != "idle":
		rm.state = "transit"
	case rm.state == "transit" && sysFRRelaying:
		rm.state = "leaving"
	case rm.state == "transit" && !inL4:
		rm.state = "incoming"
	case rm.state == "transit" && inL4:
		rm.state = "deploying"
	case rm.state == "deploying" && sysHasSpareFR:
		rm.state = "cleanup"
	case rm.state == "deploying" && !frInv:
		rm.state = "empty"
	case frInv && (rm.state == "empty" || rm.state == "deploying"):
		rm.state = "leaving"
	default:
		return fmt.Errorf("Unknown state (%s): FRs: ship %v, sys %d (relaying: %v)",
			rm.dev.Code.Alias(), frInv, len(sysFRs), sysFRRelaying)
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
			return eta, err
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
		res, err := rest.DeviceCommand[models.CommandResp](rm.dev.Code, "travel", map[string]any{
			"destination": scan.EntryPoint,
		})
		return res.Arrives.Time(), err
	case "deploying":

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
