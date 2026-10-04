package common

import (
	"testing"
	"time"

	"github.com/zigdon/rsp/models"
)

func TestTravelCoordinatorQueue(t *testing.T) {
	afc := models.NewCodeAlias("afc-1")
	tc := NewTravelCoordinator(afc, true)

	// 1. Same origin and destination returns zero time and no error
	ca1 := models.NewCodeAlias("cf-1")
	eta, err := tc.Queue(ca1, "SOL-1", "SOL-1")
	if err != nil {
		t.Fatalf("Queue with same origin/destination returned error: %v", err)
	}
	if !eta.IsZero() {
		t.Errorf("Queue with same origin/destination expected zero time, got %v", eta)
	}
	if len(tc.queue) != 0 {
		t.Errorf("Queue should not add entry when origin == destination")
	}

	// 2. Pre-seed ETA to test queueing without network
	presetTime := time.Date(2026, 8, 20, 18, 0, 0, 0, time.UTC)
	tc.etas["SOL-1"] = map[string]time.Time{
		"SOL-3": presetTime,
	}

	eta, err = tc.Queue(ca1, "SOL-1", "SOL-3")
	if err != nil {
		t.Fatalf("Queue returned error: %v", err)
	}
	if !eta.Equal(presetTime) {
		t.Errorf("Queue returned unexpected ETA: got %v, expected %v", eta, presetTime)
	}
	if len(tc.queue["SOL-1"]["SOL-3"]) != 1 {
		t.Fatalf("Expected 1 device in queue, got %d", len(tc.queue["SOL-1"]["SOL-3"]))
	}

	// 3. Queueing same device again should return existing ETA without duplicate entry
	eta2, err := tc.Queue(ca1, "SOL-1", "SOL-3")
	if err != nil {
		t.Fatalf("Re-queueing returned error: %v", err)
	}
	if !eta2.Equal(presetTime) {
		t.Errorf("Re-queueing ETA mismatch: got %v", eta2)
	}
	if len(tc.queue["SOL-1"]["SOL-3"]) != 1 {
		t.Errorf("Re-queueing should not duplicate device in queue, got %d items", len(tc.queue["SOL-1"]["SOL-3"]))
	}
}
