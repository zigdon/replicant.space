package auto

import (
	"cmp"
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
	Start(*models.Device, bool) error
	UpdateState() error
	Process() (time.Time, error)
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

type Event struct {
	when       time.Time
	name, desc string
	callback   func() error
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

func (eq *EventQueue) AddEvent(name, desc string, when time.Time, callback func() error) {
	e := &Event{
		name:     name,
		desc:     desc,
		when:     when,
		callback: callback,
	}
	eq.queue = append(eq.queue, e)
	log("Added event %q: %s", e.name, e.when.Format(time.Stamp))
	slices.SortFunc(eq.queue, func(a, b *Event) int {
		return cmp.Compare(a.when.Unix(), b.when.Unix())
	})
}

func (eq *EventQueue) Next() time.Time {
	if len(eq.queue) == 0 {
		return time.Now().Add(eq.timeout)
	}
	if time.Until(eq.queue[0].when) > eq.timeout {
		return time.Now().Add(eq.timeout)
	}
	return eq.queue[0].when
}

func (eq *EventQueue) Wait() {
	t := time.NewTimer(time.Until(eq.Next()))
	<-t.C
}
