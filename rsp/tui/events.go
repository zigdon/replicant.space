package tui

import (
	"cmp"
	"slices"
	"sync"
	"time"
)

type event struct {
	label string
	ts time.Time
	fn func() error
}
var mu sync.Mutex
var eventQueue []event

func Queue(label string, ts time.Time, fn func() error) {
	mu.Lock()
	defer mu.Unlock()
	eventQueue = append(eventQueue, event{label, ts, fn})
	slices.SortFunc(eventQueue, func(a, b event) int {
		return cmp.Compare(a.ts.Second(), b.ts.Second())
	})
}

func forever() bool { return false }

func Repeat(label string, interval time.Duration, fn func() error, stop func () bool) {
	var ev func () error
	ev = func() error {
		if stop() {
			log("Stopping repeated %q", label)
			return nil
		}
		go app.QueueUpdateDraw(func() {fn()})
		log("requeuing repeated event %q for %s", label, time.Now().Add(interval))
		Queue(label, time.Now().Add(interval), ev)
		return nil
	}
	Queue(label, time.Now(), ev)
	tick <- false
}

var tick = make(chan bool, 1)
func processEventQueue() {
	// Wait for the app to be available
	for app.GetFocus() == nil {
		log("Waiting for app...")
		time.Sleep(time.Second)
	}
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		log("%d events in queue", len(eventQueue))
		var next time.Time
		select {
		case t := <-ticker.C:
			log("timer tick: %s", t)
			next = processEvents()
		case c := <- tick:
			log("manual tick")
			if c {
				return
			}
			next = processEvents()
		}
		log("next event: %s", next.String())
		time.AfterFunc(time.Until(next), func() { tick <- false })
		log("waiting %s for next event, %d events in queue", time.Until(next), len(eventQueue))
		app.Draw()
	}
}

// Process the pending events, reutrn the time of the next event to process.
func processEvents() time.Time {
	for len(eventQueue) > 0 && time.Now().After(eventQueue[0].ts) {
		mu.Lock()
		ev := eventQueue[0]
		eventQueue = eventQueue[1:]
		mu.Unlock()
		log("Processing event %q (%s)", ev.label, ev.ts.String())
		if err := ev.fn(); err != nil {
			log("Error processing event: %v", err)
		}
	}
	if len(eventQueue) > 0 {
		mu.Lock()
		defer mu.Unlock()
		return eventQueue[0].ts
	}
	return time.Now().Add(10 * time.Second)
}

