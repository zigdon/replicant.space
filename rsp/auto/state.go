package auto

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/zigdon/rsp/cache"
	"github.com/zigdon/rsp/common"
	"github.com/zigdon/rsp/models"
	"github.com/zigdon/rsp/rest"
)

var DB *cache.Cache

type MachineDoneErr string

func (e MachineDoneErr) Error() string {
	return string(e)
}

// Find devices tagged with auto:<label>
// The label identifies the state machine to use
// If there's an optional state:<name> tag, use that to set the state
// The state machine defines Process(*dev) (time.Time, error), and does its thing
// If there's a returned time, it is saved on the device in ts:<seconds>

type Machine interface {
	Start(dev *models.Device, dryRun bool) error
	UpdateState() error
	Process() (time.Time, error)
	SaveState(state string) error
	Status() string
	Name() string
}

func getTags(dev *models.Device) map[string]string {
	res := make(map[string]string)
	tags := dev.Tags
	slices.Sort(tags)
	for _, t := range tags {
		k, v, ok := strings.Cut(t, ":")
		if !ok {
			continue
		}
		res[k] = v
	}

	return res
}

func later(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}

func log(tmpl string, args ...any) {
	common.LogLevel(1, tmpl, args...)
}

func deviceCommand(id *models.CodeAlias, cmd string, args map[string]any, dryRun bool) (*models.CommandResp, error) {
	if dryRun {
		log("[DRYRUN] Issuing %q to %s: %v", cmd, id.Alias(), args)
		return new(models.CommandResp), nil
	}
	log("Issuing %q to %s: %v", cmd, id.Alias(), args)
	res, err := rest.DeviceCommand[models.CommandResp](id, cmd, args)
	if err != nil {
		return res, fmt.Errorf("Error sending %q command to %q: %v", cmd, id.Alias(), err)
	}
	return res, nil
}

type Event struct {
	When       time.Time
	Name, Desc string
	Callback   func() error
	Data       any
}

type EventQueue struct {
	queue   []*Event
	timeout time.Duration
}

func NewEventQueue(to time.Duration) *EventQueue {
	return &EventQueue{
		timeout: to,
	}
}

func (eq *EventQueue) AddEvent(name, desc string, when time.Time, callback func() error, data any) {
	e := &Event{
		Name:     name,
		Desc:     desc,
		When:     when,
		Callback: callback,
		Data:     data,
	}
	eq.queue = append(eq.queue, e)
	log("Added event %q: %s (%s)", e.Name, e.When, time.Until(e.When))
	slices.SortFunc(eq.queue, func(a, b *Event) int {
		return cmp.Compare(a.When.Unix(), b.When.Unix())
	})
}

func (eq *EventQueue) Next() time.Time {
	if len(eq.queue) == 0 {
		return time.Now().Add(eq.timeout)
	}
	if time.Until(eq.queue[0].When) > eq.timeout {
		return time.Now().Add(eq.timeout)
	}
	return eq.queue[0].When
}

func (eq *EventQueue) Wait() *Event {
	if len(eq.queue) == 0 {
		log("No more events")
		return nil
	}
	t := time.NewTimer(time.Until(eq.Next()))
	<-t.C
	ev := eq.queue[0]
	eq.queue = eq.queue[1:]
	return ev
}

func (eq *EventQueue) List() []*Event {
	return eq.queue
}
