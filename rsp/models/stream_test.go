package models

import (
	"encoding/json"
	"testing"
)

func TestStreamDeviceStowed(t *testing.T) {
	// 1. When stowed_in is provided
	jsonWithStowedIn := `{"stowed_in": "sh-1"}`
	var sds1 StreamDeviceStowed
	if err := json.Unmarshal([]byte(jsonWithStowedIn), &sds1); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if err := sds1.Fill(); err != nil {
		t.Fatalf("Fill failed: %v", err)
	}
	if sds1.StowedIn == nil || sds1.StowedIn.Alias() != "sh-1" {
		t.Errorf("StreamDeviceStowed with stowed_in mismatch: %v", sds1.StowedIn)
	}

	// 2. When only stowed_in_device_code is provided (fallback in Fill())
	jsonWithStowedInDC := `{"stowed_in_device_code": "cf-2"}`
	var sds2 StreamDeviceStowed
	if err := json.Unmarshal([]byte(jsonWithStowedInDC), &sds2); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if err := sds2.Fill(); err != nil {
		t.Fatalf("Fill failed: %v", err)
	}
	if sds2.StowedIn == nil || sds2.StowedIn.Alias() != "cf-2" {
		t.Errorf("StreamDeviceStowed fallback to StowedInDC mismatch: %v", sds2.StowedIn)
	}
}

func TestStreamEventCompleted(t *testing.T) {
	jsonPayload := `{
		"designation": "EV-01",
		"location": "SOL-1",
		"event_type": "survey",
		"tier": 2,
		"rewards": {
			"xp": 100,
			"resources": {"carbon": 50, "silicates": 30},
			"devices": [{"device_code": "md-1", "device_type": "mining_drone"}],
			"civilisation_points": 25
		},
		"consumed": {
			"devices": [{"device_code": "probe-1", "device_type": "probe"}],
			"resources": {"conductive": 10}
		}
	}`

	var ev StreamEventCompleted
	if err := json.Unmarshal([]byte(jsonPayload), &ev); err != nil {
		t.Fatalf("Unmarshal StreamEventCompleted failed: %v", err)
	}

	if ev.Designation != "EV-01" || ev.Location != "SOL-1" || ev.Tier != 2 {
		t.Errorf("StreamEventCompleted basic fields mismatch: %v", ev)
	}
	if ev.Rewards.CivilisationPoints != 25 {
		t.Errorf("CivilisationPoints mismatch: got %d, expected 25", ev.Rewards.CivilisationPoints)
	}
	if ev.Rewards.Xp != 100 || ev.Rewards.Resources["carbon"] != 50 {
		t.Errorf("Rewards mismatch: %v", ev.Rewards)
	}
	if len(ev.Rewards.Devices) != 1 || ev.Rewards.Devices[0].Code.Alias() != "md-1" {
		t.Errorf("Rewards.Devices mismatch: %v", ev.Rewards.Devices)
	}
	if len(ev.Consumed.Devices) != 1 || ev.Consumed.Devices[0].Code.Alias() != "probe-1" {
		t.Errorf("Consumed.Devices mismatch: %v", ev.Consumed.Devices)
	}
}

func TestStreamEventDiscovered(t *testing.T) {
	jsonPayload := `{
		"designation": "EV-02",
		"location": "SOL-2",
		"event_type": "ancient_ruins",
		"tier": 1,
		"title": "Ancient Relic",
		"description": "A relic from past civ",
		"criteria": [
			{
				"name": "Analyze",
				"devices": {"probe": 1},
				"resources": {"carbon": 100}
			}
		]
	}`

	var ev StreamEventDiscovered
	if err := json.Unmarshal([]byte(jsonPayload), &ev); err != nil {
		t.Fatalf("Unmarshal StreamEventDiscovered failed: %v", err)
	}

	if ev.Title != "Ancient Relic" || len(ev.Criteria) != 1 {
		t.Errorf("StreamEventDiscovered mismatch: %v", ev)
	}
	if ev.Criteria[0].Devices["probe"] != 1 || ev.Criteria[0].Resources["carbon"] != 100 {
		t.Errorf("Criteria mismatch: %v", ev.Criteria[0])
	}
}

func TestStreamOtherEvents(t *testing.T) {
	// SiteDepleted
	depletedJSON := `{"site": "SOL-1-SITE-1"}`
	var sd StreamSiteDepleted
	if err := json.Unmarshal([]byte(depletedJSON), &sd); err != nil {
		t.Fatalf("Unmarshal StreamSiteDepleted failed: %v", err)
	}
	if sd.Site != "SOL-1-SITE-1" {
		t.Errorf("StreamSiteDepleted mismatch: %v", sd)
	}

	// TransportDelivered
	deliveredJSON := `{"origin": "SOL-1", "resources": {"carbon": 200}}`
	var td StreamTransportDelivered
	if err := json.Unmarshal([]byte(deliveredJSON), &td); err != nil {
		t.Fatalf("Unmarshal StreamTransportDelivered failed: %v", err)
	}
	if td.Resources["carbon"] != 200 {
		t.Errorf("StreamTransportDelivered mismatch: %v", td)
	}

	// TravelArrived
	arrivedJSON := `{"destination": "SOL-3", "origin": "SOL-1"}`
	var ta StreamTravelArrived
	if err := json.Unmarshal([]byte(arrivedJSON), &ta); err != nil {
		t.Fatalf("Unmarshal StreamTravelArrived failed: %v", err)
	}
	if ta.Destination != "SOL-3" || ta.Origin != "SOL-1" {
		t.Errorf("StreamTravelArrived mismatch: %v", ta)
	}
}
