package auto

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/zigdon/rsp/cache"
	"github.com/zigdon/rsp/models"
	"github.com/zigdon/rsp/rest"
)

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
	state  string
	tag    string
	dest   *models.Position
	plat   *models.Device
	db     *cache.Cache
	dryRun bool
}

func (pm *ProspectMachine) Status() string {
	var res []string
	res = append(res, fmt.Sprintf("Machine state: %s, tag: %s, dry_run: %v",
		pm.state, pm.tag, pm.dryRun))
	if pm.dest != nil {
		res = append(res, fmt.Sprintf("  dest: %s", pm.dest.String()))
	} else {
		res = append(res, "  dest: nil")
	}
	if pm.plat != nil {
		res = append(res, fmt.Sprintf("Platform %s: %s @ %s", pm.plat.Code.Alias(), pm.plat.Status, pm.plat.Location))
	} else {
		res = append(res, "  platform: nil")
	}
	if pm.dev != nil {
		res = append(res, fmt.Sprintf("Device %s: %s @ %s", pm.dev.Code.Alias(), pm.dev.Status, pm.dev.Location))
	} else {
		res = append(res, "  device: nil")
	}

	return strings.Join(res, "\n")
}

func (pm *ProspectMachine) Start(d *models.Device, dryRun bool) error {
	pm.dryRun = dryRun
	pm.dev = d
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

func (pm *ProspectMachine) deviceCmd(id *models.CodeAlias, cmd string, args map[string]any) (*models.CommandResp, error) {
	if pm.dryRun {
		log("Would issue [%q, %v] to %q", cmd, args, id.Alias())
		return &models.CommandResp{}, nil
	}
	log("Issuing [%q, %v] to %q", cmd, args, id.Alias())
	return rest.DeviceCommand[models.CommandResp](id, cmd, args)
}

func (pm *ProspectMachine) goCmd(cmd string, args map[string]any) (*models.CommandResp, error) {
	return pm.deviceCmd(pm.dev.Code, cmd, args)
}

func (pm *ProspectMachine) platform(cmd string, arg string) (*models.CommandResp, error) {
	switch cmd {
	case "travel":
		return pm.deviceCmd(pm.plat.Code, "travel", map[string]any{"destination": arg})
	case "attach", "detach":
		return pm.deviceCmd(pm.plat.Code, cmd, map[string]any{"device": arg})
	}

	return nil, fmt.Errorf("Unknown platform command %q", cmd)
}

func (pm *ProspectMachine) nextDest() (string, error) {
	if pm.db == nil {
		db, err := cache.Connect(false)
		if err != nil {
			return "", err
		}
		pm.db = db
	}
	if out, err := rest.ReloadStars(); err != nil {
		log(out)
		return "", err
	}
	origin := models.NewPosition(0, 0, 0)
	nearest, dist, err := pm.db.FindNearestStar(pm.dest.X, pm.dest.Y, pm.dest.Z)
	if err != nil {
		return "", err
	}
	skip := make(map[string]bool)
	next := nearest
	stars := make(map[string]*models.Star)
	stars[nearest] = &models.Star{Designation: nearest}
	if err := stars[nearest].Get(); err != nil {
		return "", err
	}
	for {
		// Check if there's aleady an observatory there, or on the way.
		obvs, err := rest.Devices(map[string]string{
			"device_type": "galactic_observatory",
		})
		if err != nil {
			return "", fmt.Errorf("Can't get GO list: %v", err)
		}
		for _, o := range obvs {
			if o.Location == next || (o.Travel != nil && o.Travel.Destination == next) {
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

		// If not there, pick a star in the same sector
		nPos := stars[next].Position
		sector, err := pm.db.GetSector(nPos.X, nPos.Y, nPos.Z, 10, 5)
		if err != nil {
			return "", err
		}

		// Sort the stars in the sector by distance, furthest away first
		dists := make(map[string]float32)
		for _, s := range sector {
			if skip[s] {
				continue
			}
			stars[s] = &models.Star{Designation: s}
			if err := stars[s].Get(); err != nil {
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
			log("Next star in the sector: %s (%.2fly)", s, dists[s])
			next = s
			dist = dists[s]
			found = true
			break
		}

		if !found {
			return "", fmt.Errorf("Next star unknown")
		}
	}

	log("Next star: %q, %.2f LY away from %s", next, dist, pm.dest)
	return next, nil
}

func (pm *ProspectMachine) UpdateState() error {
	dev, err := rest.DeviceInfo(pm.dev.Code)
	if err != nil {
		return err
	}
	pm.dev = dev
	status := pm.dev.Status
	pm.tag = getTags(pm.dev)["state"]
	switch {
	case status == "prospecting":
		pm.state = "prospecting"
	case status == "idle" && pm.tag == "teardown":
		pm.state = "finished"
	case status == "compacting":
		pm.state = "compacting"
	case status == "compacted" && pm.tag == "teardown":
		pm.state = "leaving"
	case status == "travelling":
		pm.state = "travelling"
	case status == "compacted" && pm.tag == "setup":
		pm.state = "setup"
	case status == "unfurling":
		pm.state = "unfurling"
	case status == "idle" && pm.tag == "setup":
		pm.state = "starting"
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
	var nextTag string
	switch pm.state {
	case "prospecting":
		nextTag = "teardown"
		eta = pm.dev.Prospect.Completes.Time()
	case "finished":
		res, err := pm.goCmd("compact", nil)
		if err != nil {
			return eta, err
		}
		eta = res.Completes.Time()
		if err = rest.ProspectLogs(pm.dev.Code); err != nil {
			log("Error getting new stars: %v")
		}
	case "compacting":
		eta = pm.dev.Compact.Completes.Time()
		res, err := pm.platform("travel", pm.dev.Location)
		if err != nil {
			return eta, err
		}
		if res.Arrives.Time().After(eta) {
			eta = res.Arrives.Time()
		}
	case "leaving":
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
	case "travelling":
		nextTag = "setup"
		eta = pm.dev.Travel.Arrives.Time()
	case "setup":
		if pm.dev.AttachedToDeviceCode != nil {
			_, err := pm.platform("detach", pm.dev.Code.Alias())
			if err != nil {
				return eta, err
			}
		}
		res, err := pm.goCmd("unfurl", nil)
		if err != nil {
			return eta, err
		}
		eta = res.Completes.Time()
	case "unfurling":
		eta = pm.dev.Unfurl.Completes.Time()
		// Wait
	case "starting":
		delta := pm.dest.Delta(pm.dev.GetPosition())
		_, err := pm.goCmd("prospect", map[string]any{"direction": []float32{delta.X, delta.Y, delta.Z}})
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
	default:
		return eta, fmt.Errorf("Unknown state: %q", pm.state)
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
	_, err := rest.UpdateTags(pm.dev.Code, rest.DelTag, []string{fmt.Sprintf("state:%s", pm.tag)})
	if err != nil {
		return err
	}
	_, err = rest.UpdateTags(pm.dev.Code, rest.AddTag, []string{fmt.Sprintf("state:%s", state)})
	if err != nil {
		return err
	}
	pm.tag = state

	return nil
}
