package models

import (
	"strings"
	"testing"
)

func TestBlueprintRawResources(t *testing.T) {
	// Simple blueprint with raw resources
	bpSimple := &Blueprint{
		DeviceType: "basic_probe",
		Resources: map[string]int{
			"carbon":    50,
			"conductive": 20,
		},
	}

	res, err := bpSimple.RawResources()
	if err != nil {
		t.Fatalf("RawResources failed on simple blueprint: %v", err)
	}
	if res["carbon"] != 50 || res["conductive"] != 20 {
		t.Errorf("RawResources mismatch: got %v", res)
	}

	// Blueprint with missing sub-component blueprint without DB returns error
	bpWithComp := &Blueprint{
		DeviceType: "adv_ship",
		Resources:  map[string]int{"structural": 100},
		Components: map[string]int{"engine": 2},
	}
	_, err = bpWithComp.RawResources()
	if err == nil {
		t.Errorf("RawResources with unresolvable sub-component expected error, got nil")
	}
}

func TestBlueprintCacheAndGet(t *testing.T) {
	bp := &Blueprint{
		DeviceType: "test_drone",
	}

	ConnectDB(nil)
	if err := bp.Get(); err == nil || !strings.Contains(err.Error(), "Not connected to cache") {
		t.Errorf("Blueprint.Get() without DB expected Not connected error, got %v", err)
	}

	emptyBP := &Blueprint{}
	if err := emptyBP.Get(); err == nil {
		t.Errorf("Blueprint.Get() with empty DeviceType expected error, got nil")
	}

	bps := &Blueprints{
		Blueprints: []*Blueprint{bp},
	}
	if err := bps.Get(); err == nil || !strings.Contains(err.Error(), "Not connected to cache") {
		t.Errorf("Blueprints.Get() without DB expected error, got %v", err)
	}
}
