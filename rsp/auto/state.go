package auto

import (
	"fmt"
	"strings"
	"time"

	"github.com/zigdon/rsp/cache"
	"github.com/zigdon/rsp/models"
	"github.com/zigdon/rsp/rest"
)

// Find devices tagged with auto:<label>
// The label identifies the state machine to use
// If there's an optional state:<name> tag, use that to set the state
// The state machine defines Process(*dev) (time.Time, error), and does its thing
// If there's a returned time, it is saved on the device in ts:<seconds>

type Machine interface {
	Start(*models.Device) error
	UpdateState() error
	Process() (*time.Time, error)
	SaveState(string) error
}

func Start[T Machine](d *models.Device) (*T, error) {
	m := new(T)
	return m, (*m).Start(d)
}

func getTags(dev *models.Device) map[string]string {
	res := make(map[string]string)
	tags := dev.Tags
	for _, t := range tags {
		k, v, ok := strings.Cut(t, ":")
		if !ok {
			continue
		}
		res[k] = v
	}

	return res
}

func log(tmpl string, args ...any) {
	fmt.Printf(time.Now().Format(time.Stamp)+" - "+tmpl+"\n", args...)
}

// States:
// Name        | Status      | Tag      | Action
// ------------------------------------------------------------
// prospecting | prospecting |          | wait
// finished    | idle        | teardown | compact, update stars
// compacting  | compacting  | teardown | fetch platform, wait
// leaving     | compacted   | teardown | attach, travel
// travelling  | travelling  |          | wait
// setup       | compacted   | setup    | detach, unfurl
// unfurling   | unfurling   | setup    | wait
// starting    | idle        | setup    | prospect
type ProspectMachine struct {
	dev   *models.Device
	state string
	tag   string
	dest  *models.Position
	plat  *models.Device
	db    *cache.Cache
}

func (pm *ProspectMachine) Start(d *models.Device) error {
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

func (pm *ProspectMachine) goCmd(cmd string, args map[string]any) (*models.CommandResp, error) {
	return rest.DeviceCommand(pm.dev.Code, cmd, args)
}

func (pm *ProspectMachine) platform(cmd string, arg string) (*models.CommandResp, error) {
	switch cmd {
	case "travel":
		return rest.DeviceCommand(pm.plat.Code, "travel", map[string]any{"destination": arg})
	case "attach", "detach":
		return rest.DeviceCommand(pm.plat.Code, cmd, map[string]any{"device": arg})
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
	next, dist, err := pm.db.FindNearestStar(pm.dest.X, pm.dest.Y, pm.dest.Z)
	if err != nil {
		return "", err
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
		return fmt.Errorf("Invalid state: status=%q, tag=%q", status, pm.tag)
	}
	return nil
}

func (pm *ProspectMachine) Process() (*time.Time, error) {
	var nextTag string
	var eta time.Time
	switch pm.state {
	case "prospecting":
		nextTag = "teardown"
		eta = pm.dev.Prospect.Completes.Time()
	case "finished":
		res, err := pm.goCmd("compact", nil)
		if err != nil {
			return nil, err
		}
		eta = res.Completes.Time()
		// TODO: Add new stars to the cache
	case "compacting":
		eta = pm.dev.Compact.Completes.Time()
		res, err := pm.platform("travel", pm.dev.Location)
		if err != nil {
			return nil, err
		}
		if res.Arrives.Time().After(eta) {
			eta = res.Arrives.Time()
		}
	case "leaving":
		nextTag = "setup"
		_, err := pm.platform("attach", pm.dev.Code.Alias())
		if err != nil {
			return nil, err
		}
		next, err := pm.nextDest()
		if err != nil {
			return nil, err
		}
		res, err := pm.platform("travel", next)
		if err != nil {
			return nil, err
		}
		eta = res.Arrives.Time()
	case "travelling":
		nextTag = "setup"
		eta = pm.dev.Travel.Arrives.Time()
	case "setup":
		_, err := pm.platform("detach", pm.dev.Code.Alias())
		if err != nil {
			return nil, err
		}
		res, err := pm.goCmd("unfurl", nil)
		if err != nil {
			return nil, err
		}
		eta = res.Completes.Time()
	case "unfurling":
		eta = pm.dev.Unfurl.Completes.Time()
		// Wait
	case "starting":
		delta := pm.dest.Delta(pm.dev.GetPosition())
		_, err := pm.goCmd("prospect", map[string]any{"direction": []float32{delta.X, delta.Y, delta.Z}})
		if err != nil {
			return nil, err
		}
		dev, err := rest.DeviceInfo(pm.dev.Code)
		if err != nil {
			return nil, err
		}
		eta = dev.Prospect.Completes.Time()
	default:
		return nil, fmt.Errorf("Unknown state: %q", pm.state)
	}
	return &eta, pm.SaveState(nextTag)
}

func (pm *ProspectMachine) SaveState(state string) error {
	if state == pm.tag {
		return nil
	}
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
