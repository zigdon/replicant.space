package models

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestCodeAlias(t *testing.T) {
	ca1 := NewCodeAlias("mining_drone-42")
	if ca1.Alias() != "mining_drone-42" {
		t.Errorf("CodeAlias.Alias() mismatch: got %q", ca1.Alias())
	}
	if ca1.Type() != "mining_drone" {
		t.Errorf("CodeAlias.Type() mismatch: got %q", ca1.Type())
	}
	if ca1.Num() != 42 {
		t.Errorf("CodeAlias.Num() mismatch: got %d, expected 42", ca1.Num())
	}

	// Code without alias/dash
	ca2 := NewCodeAlias("ABCDEF123456")
	if ca2.String() != "ABCDEF123456" {
		t.Errorf("CodeAlias.String() for plain code expected ABCDEF123456, got %q", ca2.String())
	}
	if ca2.Alias() != "ABCDEF123456" {
		t.Errorf("CodeAlias.Alias() for plain code expected ABCDEF123456, got %q", ca2.Alias())
	}
	if ca2.Type() != "" {
		t.Errorf("CodeAlias.Type() for plain code expected \"\", got %q", ca2.Type())
	}
	if ca2.Num() != 0 {
		t.Errorf("CodeAlias.Num() for plain code expected 0, got %d", ca2.Num())
	}

	// Contained
	list := []*CodeAlias{ca2}
	if !ca2.Contained(list) {
		t.Errorf("Contained should find ca2 in list")
	}
	caOther := NewCodeAlias("OTHER123")
	if caOther.Contained(list) {
		t.Errorf("Contained should not find caOther in list")
	}

	// CompareAliases
	caA := NewCodeAlias("af-1")
	caB := NewCodeAlias("af-2")
	caC := NewCodeAlias("sh-1")
	if CompareAliases(caA, caB) >= 0 {
		t.Errorf("CompareAliases(af-1, af-2) should be negative")
	}
	if CompareAliases(caB, caA) <= 0 {
		t.Errorf("CompareAliases(af-2, af-1) should be positive")
	}
	if CompareAliases(caA, caC) >= 0 {
		t.Errorf("CompareAliases(af-1, sh-1) should be negative ('af' < 'sh')")
	}

	// JSON Marshal / Unmarshal
	data, err := json.Marshal(ca2)
	if err != nil {
		t.Fatalf("json.Marshal(CodeAlias) failed: %v", err)
	}
	var unmarshaled CodeAlias
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("json.Unmarshal(CodeAlias) failed: %v", err)
	}
	if unmarshaled.String() != "ABCDEF123456" {
		t.Errorf("Unmarshaled CodeAlias mismatch: got %q", unmarshaled.String())
	}

	var nilCA *CodeAlias
	if nilCA.String() != "" || nilCA.Alias() != "" {
		t.Errorf("nil CodeAlias methods should return empty string")
	}
}

func TestStowedDevices(t *testing.T) {
	// 1. Unmarshal from list of structs
	structJSON := `[
		{"device_code": "MD01", "device_type": "mining_drone"},
		{"device_code": "PR02", "device_type": "probe"}
	]`
	var sd1 StowedDevices
	if err := json.Unmarshal([]byte(structJSON), &sd1); err != nil {
		t.Fatalf("Unmarshal StowedDevices struct failed: %v", err)
	}
	if len(sd1.Devices) != 2 {
		t.Fatalf("Expected 2 stowed devices, got %d", len(sd1.Devices))
	}
	if sd1.Devices[0].Code.String() != "MD01" || sd1.Devices[0].Type != "mining_drone" {
		t.Errorf("StowedDevices[0] mismatch: %v", sd1.Devices[0])
	}

	// 2. Unmarshal from list of strings
	stringsJSON := `["MD01", "PR02", "SH03"]`
	var sd2 StowedDevices
	if err := json.Unmarshal([]byte(stringsJSON), &sd2); err != nil {
		t.Fatalf("Unmarshal StowedDevices strings failed: %v", err)
	}
	if len(sd2.Devices) != 3 {
		t.Fatalf("Expected 3 stowed devices, got %d", len(sd2.Devices))
	}
	if sd2.Devices[2].Code.String() != "SH03" {
		t.Errorf("StowedDevices[2] mismatch: %v", sd2.Devices[2])
	}

	// 3. Marshal back to JSON
	data, err := json.Marshal(&sd2)
	if err != nil {
		t.Fatalf("Marshal StowedDevices failed: %v", err)
	}
	if !strings.Contains(string(data), "SH03") {
		t.Errorf("Marshal output missing device: %s", string(data))
	}
}

func TestAssembleResp(t *testing.T) {
	jsonPayload := `{
		"assembled": [
			{"device_code": "CF01", "method": "mobile_fleet"}
		],
		"controller_code": "MF01",
		"destination": "SOL-1",
		"destination_name": "Mercury",
		"skipped": [
			{"device_code": "CF02", "reason": "out of range"}
		],
		"status": "success"
	}`

	var resp AssembleResp
	if err := json.Unmarshal([]byte(jsonPayload), &resp); err != nil {
		t.Fatalf("Unmarshal AssembleResp failed: %v", err)
	}

	if len(resp.Assembled) != 1 || resp.Assembled[0].DeviceCode.String() != "CF01" || resp.Assembled[0].Method != "mobile_fleet" {
		t.Errorf("AssembleResp.Assembled mismatch: %v", resp.Assembled)
	}
	if resp.Controller.String() != "MF01" {
		t.Errorf("AssembleResp.Controller mismatch: %v", resp.Controller)
	}
	if resp.Destination != "SOL-1" || resp.DestinationName != "Mercury" {
		t.Errorf("AssembleResp destination mismatch: %s / %s", resp.Destination, resp.DestinationName)
	}
	if len(resp.Skipped) != 1 || resp.Skipped[0].Reason != "out of range" {
		t.Errorf("AssembleResp.Skipped mismatch: %v", resp.Skipped)
	}
	if resp.Status != "success" {
		t.Errorf("AssembleResp.Status mismatch: %s", resp.Status)
	}
}

func TestInventory(t *testing.T) {
	inv := &Inventory{
		Quantity:     150,
		ResourceType: "carbon",
	}
	if s := inv.String(); s != "150 × carbon" {
		t.Errorf("Inventory.String() = %q, expected \"150 × carbon\"", s)
	}
}

func TestDeviceMethods(t *testing.T) {
	ca := NewCodeAlias("cf-1")
	dev := &Device{
		Code:          ca,
		Type:          "cargo_freighter",
		CargoCapacity: 0,
		AttachCapacity: 0,
		Features:      []string{"travel", "cargo"},
	}

	// String
	if dev.String() != "cf-1" {
		t.Errorf("Device.String() = %q, expected \"cf-1\"", dev.String())
	}

	// HasCapability
	if !dev.HasCapability("travel") {
		t.Errorf("Device.HasCapability(travel) should be true")
	}
	if dev.HasCapability("mining") {
		t.Errorf("Device.HasCapability(mining) should be false")
	}

	// Fill defaults for freighter and mobile fleet
	if err := dev.Fill(); err != nil {
		t.Errorf("Device.Fill() error: %v", err)
	}
	if dev.CargoCapacity != 500 {
		t.Errorf("cargo_freighter default cargo capacity should be 500, got %d", dev.CargoCapacity)
	}

	mfDev := &Device{
		Code: NewCodeAlias("mf-1"),
		Type: "mobile_fleet",
	}
	if err := mfDev.Fill(); err != nil {
		t.Errorf("mfDev.Fill() error: %v", err)
	}
	if mfDev.AttachCapacity != 36 {
		t.Errorf("mobile_fleet default attach capacity should be 36, got %d", mfDev.AttachCapacity)
	}

	// Fetched / Updated without DB
	if !dev.Fetched().IsZero() {
		t.Errorf("dev.Fetched() should initially be zero")
	}
	if !dev.Updated().IsZero() {
		t.Errorf("dev.Updated() should initially be zero")
	}

	// GetPosition without DB returns nil
	if pos := dev.GetPosition(); pos != nil {
		t.Errorf("dev.GetPosition() without DB expected nil, got %v", pos)
	}

	// Get without DB returns error
	if err := dev.Get(); err == nil {
		t.Errorf("dev.Get() without DB expected error, got nil")
	}
}

func TestDeviceFilters(t *testing.T) {
	devTagged := &Device{
		Tags: []string{"auto:mine", "mine-sol-1", "priority"},
		Type: "mining_drone",
		Status: "idle",
		Location: "SOL-1",
	}

	// 1. DeviceFilterTags
	filterRequire := DeviceFilterTags(nil, []string{"auto:mine"})
	if !filterRequire(devTagged) {
		t.Errorf("FilterTags should match required tag")
	}

	filterReject := DeviceFilterTags([]string{"priority"}, nil)
	if filterReject(devTagged) {
		t.Errorf("FilterTags should reject matching rejected tag")
	}

	filterUnmatched := DeviceFilterTags(nil, []string{"auto:explore"})
	if filterUnmatched(devTagged) {
		t.Errorf("FilterTags should not match missing required tag")
	}

	// 2. DeviceFilterMatrix
	devMatrixStowed := &Device{Type: "replicant_matrix", Status: "stowed"}
	devMatrixActive := &Device{Type: "replicant_matrix", Status: "active"}
	devOther := &Device{Type: "probe", Status: "idle"}

	filterMatrix := DeviceFilterMatrix()
	if filterMatrix(devMatrixStowed) {
		t.Errorf("FilterMatrix should filter out stowed replicant_matrix")
	}
	if !filterMatrix(devMatrixActive) {
		t.Errorf("FilterMatrix should keep active replicant_matrix")
	}
	if !filterMatrix(devOther) {
		t.Errorf("FilterMatrix should keep non-matrix device")
	}

	// 3. DeviceFilterMine
	filterMine := DeviceFilterMine()
	devMiningHere := &Device{
		Tags:     []string{"mine-sol-1"},
		Location: "SOL-1",
	}
	// devMiningHere has only tag "mine-sol-1" and location "SOL-1" -> should return false
	if filterMine(devMiningHere) {
		t.Errorf("FilterMine should filter device when all tags match current mine location")
	}

	devDifferentLoc := &Device{
		Tags:     []string{"mine-sol-1"},
		Location: "SOL-2",
	}
	if !filterMine(devDifferentLoc) {
		t.Errorf("FilterMine should keep device whose mine tag is for different location")
	}

	devInTransit := &Device{
		Tags:     []string{"mine-sol-1"},
		Location: "",
	}
	if !filterMine(devInTransit) {
		t.Errorf("FilterMine should keep device in transit (empty location)")
	}
}
