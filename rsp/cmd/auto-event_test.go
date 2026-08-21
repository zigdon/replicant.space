package cmd

import (
	"testing"
	"time"

	"github.com/zigdon/rsp/models"
)

func TestTravelCoordinatorQueue(t *testing.T) {
	afc := models.NewCodeAlias("afc-1")
	tc := newTravelCoordinator(afc, true)

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

func TestPickCriteria(t *testing.T) {
	tc := newTravelCoordinator(models.NewCodeAlias("afc-1"), true)

	// Event with two options based purely on resources
	ev := &models.Event{
		Designation: "EV-TEST-1",
		Location:    "SOL-1",
		Criteria: []*models.EventCriteria{
			{
				Name:      "Option Expensive",
				Resources: map[string]int{"carbon": 1000, "silicates": 500},
			},
			{
				Name:      "Option Cheap",
				Resources: map[string]int{"carbon": 100, "silicates": 50},
			},
		},
	}

	es, err := pickCriteria(ev, tc, true)
	if err != nil {
		t.Fatalf("pickCriteria failed: %v", err)
	}
	if es == nil {
		t.Fatalf("pickCriteria returned nil eventState")
	}

	// Check that option was selected into es.required
	if es.required["carbon"] == 0 {
		t.Errorf("pickCriteria should populate es.required, got %v", es.required)
	}

	// Event with option requiring nonexistent blueprint should fail if no other options
	evInvalid := &models.Event{
		Designation: "EV-TEST-2",
		Location:    "SOL-1",
		Criteria: []*models.EventCriteria{
			{
				Name: "Option Missing BP",
				Devices: []*models.EventDevice{
					{DeviceType: "unknown_device_type_xyz", Required: 1},
				},
			},
		},
	}

	_, err = pickCriteria(evInvalid, tc, true)
	if err == nil {
		t.Errorf("pickCriteria with invalid/missing blueprint expected error, got nil")
	}
}
