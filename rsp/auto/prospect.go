package auto

import (
	"fmt"
	"time"

	"github.com/zigdon/rsp/models"
	"github.com/zigdon/rsp/rest"
)

// Tags:
//   spf w/ platform:pxa-n
//   pxa w/ prospect:pos
//       w/ state:teardown

// States:
// Name        | Status      | Tag      | Action
// -------------------------------------------------------------------
// prospecting | prospecting |          | wait
// finished    | idle        | teardown | read log, compact
// compacting  | compacting  | teardown | fetch platform, wait
// leaving     | compacted   | teardown | update stars, attach, travel
// travelling  | travelling  |          | wait
// setup       | compacted   | setup    | detach, unfurl
// unfurling   | unfurling   | setup    | wait
// starting    | idle        | setup    | prospect
type ProspectMachine struct {
	dev    *models.Device
	state  ProspectMachine_State
	tag    string
	dest   *models.Position
	plat   *models.Device
	dryRun bool
	status string
}

type ProspectMachine_State string

const (
	ProspectMachine_Prospecting ProspectMachine_State = "prospecting"
	ProspectMachine_Finished    ProspectMachine_State = "finished"
	ProspectMachine_Compacting  ProspectMachine_State = "compacting"
	ProspectMachine_Leaving     ProspectMachine_State = "leaving"
	ProspectMachine_Travelling  ProspectMachine_State = "travelling"
	ProspectMachine_Setup       ProspectMachine_State = "setup"
	ProspectMachine_Unfurling   ProspectMachine_State = "unfurling"
	ProspectMachine_Starting    ProspectMachine_State = "starting"
)

func (pm *ProspectMachine) Status() string {
	return pm.status
}

func (pm *ProspectMachine) Start(d *models.Device, dryRun bool) error {
	pm.dryRun = dryRun
	pm.dev = d
	pm.status = "initializing"
	dest := getTags(pm.dev)["prospect"]
	if dest == "" {
		return fmt.Errorf("No prospecting destination tagged on %s", d.Code.Alias())
	}
	pos, err := models.ParsePosition(dest)
	if err != nil {
		return fmt.Errorf("Invalid position on tag %q: %v", dest, err)
	}
	pm.dest = pos
	p, err := rest.GetTagged(fmt.Sprintf("platform:%s", d.Code.Alias()))
	if err != nil {
		return err
	}
	if len(p.Devices) != 1 {
		return fmt.Errorf("Can't find exactly one platform tagged platform:%s, found %d", d.Code.Alias(), len(p.Devices))
	}
	pm.plat = p.Devices[0]
	return pm.UpdateState()
}

func (pm *ProspectMachine) platform(cmd string, arg string) (*models.CommandResp, error) {
	switch cmd {
	case "travel":
		return deviceCommand(pm.plat.Code, "travel", map[string]any{"destination": arg}, pm.dryRun)
	case "attach", "detach":
		return deviceCommand(pm.plat.Code, cmd, map[string]any{"device": arg}, pm.dryRun)
	}

	return nil, fmt.Errorf("Unknown platform command %q", cmd)
}

func (pm *ProspectMachine) nextDest() (string, error) {
	if out, err := rest.ReloadStars(); err != nil {
		log(out)
		return "", err
	}
	nearest, dist, err := DB.FindNearestStar(pm.dest.X, pm.dest.Y, pm.dest.Z)
	if err != nil {
		return "", err
	}
	if pm.dev.Location.Star() == nearest {
		return "", MachineDoneErr(fmt.Sprintf("Destination %s reached", nearest))
	}
	skip := make(map[string]bool)
	next := nearest
	stars := make(map[string]*models.Star)
	stars[nearest], err = models.NewStar(nearest)
	if err != nil {
		return "", err
	}
	for {
		log("Considering %s as the next site", next)
		// Check if there's aleady an observatory there, or on the way.
		obvs, err := rest.Devices(map[string]any{
			"device_type": "parallax_array",
		})
		if err != nil {
			return "", fmt.Errorf("Can't get observatory list: %v", err)
		}
		for _, o := range obvs {
			if o.Location.Star() == next ||
				(o.Travel != nil && o.Travel.Destination.Star() == next) {
				if o.Travel == nil {
					log("%s is already at %s", o.Code.Alias(), next)
				} else {
					log("%s is on the way to %s", o.Code.Alias(), next)
				}
				skip[next] = true
				break
			}
		}

		if !skip[next] {
			break
		}

		/* TODO: redo the idea of sectors
		// If not there, pick a star in the same sector
		origin := models.NewPosition(0, 0, 0)
		nPos := stars[next].Position
		sector, err := DB.GetSector(nPos.X, nPos.Y, nPos.Z, 10, 5)
		if err != nil {
			return "", err
		}

		// Sort the stars in the sector by distance, furthest away first
		dists := make(map[string]float32)
		for _, s := range sector {
			if skip[s] {
				continue
			}
			stars[s], err = models.NewStar(s)
			if err != nil {
				return "", err
			}
			dists[s] = stars[s].Position.Distance(origin)
		}
		slices.SortFunc(sector, func(a, b string) int {
			return cmp.Compare(dists[b], dists[a])
		})

		// Pick the next star we haven't skipped yet
		found := false
		for _, s := range sector {
			if skip[s] || dists[s] == 0 {
				continue
			}
			log("Next star in the sector: %s (%.2fly from SOL)", s, dists[s])
			next = s
			dist = dists[s]
			found = true
			break
		}

		if !found {
			return "", fmt.Errorf("Next star unknown")
		}
		*/
	}

	log("Next star: %q, %.2f LY away from %s", next, dist, pm.dest)
	return next, nil
}

func (pm *ProspectMachine) UpdateState() error {
	dev, err := rest.DeviceInfo(pm.dev.Code)
	if err != nil {
		return err
	}
	if pm.dev.Status == "" {
		dev, err = rest.RefreshDeviceInfo(pm.dev.Code)
		if err != nil {
			return err
		}
	}
	pm.dev = dev
	status := pm.dev.Status
	pm.tag = getTags(pm.dev)["state"]
	plat, err := rest.DeviceInfo(pm.plat.Code)
	if err != nil {
		return err
	}
	pm.plat = plat
	switch {
	case status == "prospecting":
		pm.state = ProspectMachine_Prospecting
	case status == "idle" && (pm.tag == "teardown" || pm.tag == ""):
		pm.state = ProspectMachine_Finished
	case status == "compacting":
		pm.state = ProspectMachine_Compacting
	case status == "compacted" && (pm.tag == "teardown" || pm.tag == ""):
		pm.state = ProspectMachine_Leaving
	case status == "travelling":
		pm.state = ProspectMachine_Travelling
	case status == "compacted" && pm.tag == "setup":
		pm.state = ProspectMachine_Setup
	case status == "unfurling":
		pm.state = ProspectMachine_Unfurling
	case status == "idle" && pm.tag == "setup":
		pm.state = ProspectMachine_Starting
	default:
		return fmt.Errorf("Invalid state (%s): status=%q, tag=%q", pm.dev.Code.Alias(), status, pm.tag)
	}
	return nil
}

func (pm *ProspectMachine) Process() (time.Time, error) {
	var eta time.Time
	if err := pm.UpdateState(); err != nil {
		return eta, err
	}
	nextTag := pm.tag
	switch pm.state {
	case ProspectMachine_Prospecting:
		nextTag = "teardown"
		eta = pm.dev.Prospect.Completes.Time()
		pm.status = fmt.Sprintf("prospecting: ETA %s", eta.Format(time.Kitchen))
	case ProspectMachine_Finished:
		pm.status = "collecting results"
		res, err := deviceCommand(pm.dev.Code, "compact", nil, pm.dryRun)
		if err != nil {
			return eta, err
		}
		nextTag = "teardown"
		if res.Completes != nil {
			eta = res.Completes.Time()
		} else {
			eta = time.Now().Add(30 * time.Minute)
		}
		log("Reading prospecting logs")
		report, err := rest.ProspectLogs(pm.dev.Code)
		if err != nil {
			log("Error getting new stars: %v")
		} else {
			log("Prospecting results: %s", report)
		}
	case ProspectMachine_Compacting:
		eta = pm.dev.Compact.Completes.Time()
		pm.status = fmt.Sprintf("compacting: ETA %s", eta.Format(time.Kitchen))
		if pm.plat.Location != pm.dev.Location {
			res, err := pm.platform("travel", string(pm.dev.Location))
			if err != nil {
				return eta, err
			}
			log("Platform in transit, eta: %s", res.Arrives.Time())
			if res.Arrives.Time().After(eta) {
				eta = res.Arrives.Time()
			}
		}
		nextTag = "teardown"
	case ProspectMachine_Leaving:
		pm.status = "departing to next location"
		nextTag = "setup"
		if pm.dev.AttachedToDeviceCode == nil {
			_, err := pm.platform("attach", pm.dev.Code.Alias())
			if err != nil {
				return eta, err
			}
		}
		next, err := pm.nextDest()
		if err != nil {
			return eta, err
		}
		res, err := pm.platform("travel", next)
		if err != nil {
			return eta, err
		}
		eta = res.Arrives.Time()
	case ProspectMachine_Travelling:
		pm.status = fmt.Sprintf("in transit: ETA %s", eta.Format(time.Kitchen))
		nextTag = "setup"
		eta = pm.dev.Travel.Arrives.Time()
	case ProspectMachine_Setup:
		pm.status = "setting up"
		if pm.dev.AttachedToDeviceCode != nil {
			_, err := pm.platform("detach", pm.dev.Code.Alias())
			if err != nil {
				return eta, err
			}
		}
		res, err := deviceCommand(pm.dev.Code, "unfurl", nil, pm.dryRun)
		if err != nil {
			return eta, err
		}
		nextTag = "setup"
		eta = res.Completes.Time()
	case ProspectMachine_Unfurling:
		pm.status = fmt.Sprintf("unfurling: ETA %s", eta.Format(time.Kitchen))
		eta = pm.dev.Unfurl.Completes.Time()
		nextTag = "setup"
		// Wait
	case ProspectMachine_Starting:
		pm.status = "starting prospecting"
		delta := pm.dest.Delta(pm.dev.GetPosition())
		_, err := deviceCommand(pm.dev.Code, "prospect",
			map[string]any{"direction": []float32{delta.X, delta.Y, delta.Z}}, pm.dryRun)
		if err != nil {
			return eta, err
		}
		dev, err := rest.DeviceInfo(pm.dev.Code)
		if err != nil {
			return eta, err
		}
		if dev.Prospect != nil {
			eta = dev.Prospect.Completes.Time()
		}
		nextTag = "teardown"
	default:
		return eta, fmt.Errorf("Unknown state: %q", pm.state)
	}
	if !pm.dryRun && !eta.IsZero() && nextTag != pm.tag {
		n := &models.Notification{
			Start:  time.Now(),
			End:    eta,
			Device: pm.dev.Code.Alias(),
			Text:   fmt.Sprintf("Processing of %s state done.", pm.state),
			Object: pm,
		}
		if err := n.Save(); err != nil {
			log("Error creating notification: %v", err)
		}
	}
	if !eta.IsZero() {
		eta.Add(time.Second)
	}
	return eta, pm.SaveState(nextTag)
}

func (pm *ProspectMachine) SaveState(state string) error {
	if state == pm.tag {
		return nil
	}
	if pm.dryRun {
		log("Would update tags on %q: -%s +%s", pm.dev.Code.Alias(), pm.tag, state)
		return nil
	}
	log("Updating tags on %q: -%s +%s", pm.dev.Code.Alias(), pm.tag, state)
	if pm.tag != "" {
		err := rest.UpdateTags(pm.dev.Code, rest.DelTag, []string{fmt.Sprintf("state:%s", pm.tag)})
		if err != nil {
			return err
		}
	}

	if state != "" {
		err := rest.UpdateTags(pm.dev.Code, rest.AddTag, []string{fmt.Sprintf("state:%s", state)})
		if err != nil {
			return err
		}
	}
	pm.tag = state

	return nil
}

func (pm *ProspectMachine) Name() string {
	return "Prospecting Machine"
}
