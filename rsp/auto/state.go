package auto

import (
	"fmt"
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
	Start(*models.Device, bool) error
	UpdateState() error
	Process() (*time.Time, error)
	SaveState(string) error
	Status() string
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
