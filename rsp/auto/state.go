package auto

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/zigdon/rsp/models"
)

// Find devices tagged with auto:<label>
// The label identifies the state machine to use
// If there's an optional state:<name> tag, use that to set the state
// The state machine defines Process(*dev) (time.Time, error), and does its thing
// If there's a returned time, it is saved on the device in ts:<seconds>

type Machine interface {
	UpdateState(*models.Device) error
	Process() (*time.Time, error)
	GetState() (string, error)
	SaveState(string) error
}

func getTags(dev *models.Device) map[string]string {
	res := make(map[string]string)
	tags  := dev.Tags
	for _, t := range tags {
		k, v, ok := strings.Cut(t, ":")
		if !ok {
			continue
		}
		res[k] = v
	}

	return res
}

// States:
// - Prospecting: nothing to do until it's done
// - Idle (state:teardown): compact, send pickup platform and replicant
// - Compacting: wait
// - Compacted (state:teardown): attach to platform, find next system, ship to it
//   - Nearest system to destination that doesn't have an obv (or one in transit)
// - Travelling: wait
// - Compacted (state:setup): detach from platform, unfurl
// - Unfurling: wait
// - Idle (state:setup): Start prospecting
type ProspectMachine struct{
	dev *models.Device
	state string
	tag string
}

func (pm *ProspectMachine) UpdateState(dev *models.Device) error {
	pm.dev = dev
	status := dev.Status
	if slices.Contains([]string{"compacting", "travelling", "unfurling", "prospecting"}, status) {
		pm.state = status
		pm.tag = ""
		return nil
	} else if status == "surging" || status == "cruising" {
		pm.state = "travelling"
		pm.tag = ""
		return nil
	}
	tags := getTags(dev)
	if s, ok := tags["state"]; ok {
		pm.tag = s
		return nil
	}

	return fmt.Errorf("Unknown state: status=%s", status)
}

func (pm *ProspectMachine) Process() (*time.Time, error) {
	return nil, nil
}

func (pm *ProspectMachine) GetState() (string, error) {
	return "", nil
}

func (pm *ProspectMachine) SaveState(state string) error {
	return nil
}
