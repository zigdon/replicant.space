package auto

import (
	"testing"

	"github.com/zigdon/rsp/models"
)

func TestDispatchMachine(t *testing.T) {
	dm := &DispatchMachine{
		tasks: []*pickupTask{
			{
				pickup:    models.LocationID("SOL-1"),
				dropoff:   models.LocationID("SOL-3"),
				resources: map[string]int{"carbon": 100, "silicates": 50},
			},
			{
				pickup:    models.LocationID("SOL-1"),
				dropoff:   models.LocationID("SOL-4"),
				resources: map[string]int{"carbon": 50, "rares": 20},
			},
			{
				pickup:    models.LocationID("ALPHA-1"),
				dropoff:   models.LocationID("ALPHA-2"),
				resources: map[string]int{"conductive": 80},
			},
		},
	}

	// Status & Name & SaveState
	if dm.Name() != "Dispatch machine" {
		t.Errorf("DispatchMachine.Name() = %q", dm.Name())
	}
	if dm.Status() != "3 deliveries in flight" {
		t.Errorf("DispatchMachine.Status() = %q, expected \"3 deliveries in flight\"", dm.Status())
	}
	if err := dm.SaveState("running"); err != nil {
		t.Errorf("SaveState returned unexpected error: %v", err)
	}

	// pendingPickup for SOL-1
	pendingSol1 := dm.pendingPickup("SOL-1")
	if pendingSol1["carbon"] != 150 || pendingSol1["silicates"] != 50 || pendingSol1["rares"] != 20 {
		t.Errorf("pendingPickup(SOL-1) mismatch: got %v", pendingSol1)
	}

	// pendingPickup for ALPHA-1
	pendingAlpha := dm.pendingPickup("ALPHA-1")
	if pendingAlpha["conductive"] != 80 {
		t.Errorf("pendingPickup(ALPHA-1) mismatch: got %v", pendingAlpha)
	}

	// pendingPickup for non-existent location
	pendingNone := dm.pendingPickup("NONEXISTENT")
	if len(pendingNone) != 0 {
		t.Errorf("pendingPickup(NONEXISTENT) expected empty map, got %v", pendingNone)
	}
}
