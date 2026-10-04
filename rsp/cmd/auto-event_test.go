package cmd

import (
	"testing"

	"github.com/zigdon/rsp/common"
	"github.com/zigdon/rsp/models"
)

func TestPickCriteria(t *testing.T) {
	tc := common.NewTravelCoordinator(models.NewCodeAlias("afc-1"), true)

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
