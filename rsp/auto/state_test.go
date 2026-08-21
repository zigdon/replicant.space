package auto

import (
	"testing"
	"time"

	"github.com/zigdon/rsp/models"
)

func TestSoonerAndLater(t *testing.T) {
	t1 := time.Date(2026, 8, 20, 10, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)
	var tZero time.Time

	// sooner
	if got := sooner(t1, t2); !got.Equal(t1) {
		t.Errorf("sooner(t1, t2) = %v, expected %v", got, t1)
	}
	if got := sooner(t2, t1); !got.Equal(t1) {
		t.Errorf("sooner(t2, t1) = %v, expected %v", got, t1)
	}
	if got := sooner(tZero, t2); !got.Equal(t2) {
		t.Errorf("sooner(zero, t2) = %v, expected %v", got, t2)
	}
	if got := sooner(t1, tZero); !got.Equal(t1) {
		t.Errorf("sooner(t1, zero) = %v, expected %v", got, t1)
	}

	// later
	if got := later(t1, t2); !got.Equal(t2) {
		t.Errorf("later(t1, t2) = %v, expected %v", got, t2)
	}
	if got := later(t2, t1); !got.Equal(t2) {
		t.Errorf("later(t2, t1) = %v, expected %v", got, t2)
	}
}

func TestGetTags(t *testing.T) {
	dev := &models.Device{
		Tags: []string{"role:miner", "auto:mine", "invalid_no_colon", "system:sol"},
	}

	tags := getTags(dev)
	if len(tags) != 3 {
		t.Fatalf("getTags returned %d tags, expected 3", len(tags))
	}
	if tags["role"] != "miner" || tags["auto"] != "mine" || tags["system"] != "sol" {
		t.Errorf("getTags map mismatch: %v", tags)
	}
}

func TestEventQueue(t *testing.T) {
	eq := NewEventQueue(5 * time.Minute)
	if len(eq.List()) != 0 {
		t.Errorf("NewEventQueue should be empty")
	}

	now := time.Now()
	ev1 := now.Add(2 * time.Minute)
	ev2 := now.Add(1 * time.Minute)

	eq.AddEvent("ev1", "event 1", ev1, nil, "data1")
	eq.AddEvent("ev2", "event 2", ev2, nil, "data2")

	list := eq.List()
	if len(list) != 2 {
		t.Fatalf("EventQueue expected 2 events, got %d", len(list))
	}
	// ev2 (1 min) should be sorted before ev1 (2 min)
	if list[0].Name != "ev2" || list[1].Name != "ev1" {
		t.Errorf("EventQueue not sorted chronologically: %v, %v", list[0].Name, list[1].Name)
	}

	next := eq.Next()
	if !next.Equal(ev2) {
		t.Errorf("EventQueue.Next() = %v, expected %v", next, ev2)
	}
}
