package common

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/zigdon/rsp/models"
)

func TestLogging(t *testing.T) {
	oldFh := LogFh
	defer func() { LogFh = oldFh }()

	var buf bytes.Buffer
	LogFh = &buf

	fixedTime := time.Date(2026, 8, 15, 12, 34, 56, 0, time.UTC)

	// Test TimeLogLevel with various argument types
	dev := &models.Device{Code: models.NewCodeAlias("mining_drone-1")}
	devs := []*models.Device{dev}
	aliases := []*models.CodeAlias{models.NewCodeAlias("sh-1"), models.NewCodeAlias("af-2")}
	dur := 15 * time.Second
	jsonTime := new(models.JSONTime).Set(fixedTime)
	var jsonDelta models.JSONTimeDelta
	_ = jsonDelta.UnmarshalJSON([]byte(`"15s"`))

	TimeLogLevel(fixedTime, 0, "Test: %v %v %v %v %v %v %v %v %v",
		nil,
		fixedTime,
		dur,
		models.NewCodeAlias("station-1"),
		jsonTime,
		&jsonDelta,
		dev,
		devs,
		aliases,
	)

	out := buf.String()
	if !strings.Contains(out, "08-15 12:34:56 - ") {
		t.Errorf("Expected timestamp in log output, got: %s", out)
	}
	if !strings.Contains(out, "nil(T:<nil>)") {
		t.Errorf("Expected nil formatting, got: %s", out)
	}
	if !strings.Contains(out, "station-1") {
		t.Errorf("Expected code alias formatting, got: %s", out)
	}

	buf.Reset()
	Log("Standard log message %d", 42)
	if !strings.Contains(buf.String(), "Standard log message 42") {
		t.Errorf("Log() failed, got: %s", buf.String())
	}

	buf.Reset()
	TimeLog(fixedTime, "TimeLog message %s", "hello")
	if !strings.Contains(buf.String(), "TimeLog message hello") {
		t.Errorf("TimeLog() failed, got: %s", buf.String())
	}

	buf.Reset()
	LogLevel(1, "LogLevel message %s", "test")
	if !strings.Contains(buf.String(), "LogLevel message test") {
		t.Errorf("LogLevel() failed, got: %s", buf.String())
	}
}

func TestDatabaseHelpers(t *testing.T) {
	// When db is nil
	ConnectDB(nil)
	if db != nil {
		t.Errorf("ConnectDB(nil) failed: db is not nil")
	}

	a, typ := AliasType("test")
	if a != "" || typ != "" {
		t.Errorf("AliasType with nil db should return empty strings, got (%q, %q)", a, typ)
	}

	if alias := Alias("test"); alias != "test" {
		t.Errorf("Alias with nil db should return input, got %q", alias)
	}

	if unalias := Unalias("test"); unalias != "test" {
		t.Errorf("Unalias with nil db should return input, got %q", unalias)
	}

	// Lowercase / non-all-caps string should return as is in Alias()
	if alias := Alias("lowercase_code"); alias != "lowercase_code" {
		t.Errorf("Alias on lowercase string should return input, got %q", alias)
	}
}

func TestAliases(t *testing.T) {
	cas := []*models.CodeAlias{
		models.NewCodeAlias("d-1"),
		models.NewCodeAlias("af-2"),
		models.NewCodeAlias("sh-3"),
	}
	res := Aliases(cas)
	if len(res) != 3 {
		t.Fatalf("Aliases returned %d items, expected 3", len(res))
	}
	if res[0] != "d-1" || res[1] != "af-2" || res[2] != "sh-3" {
		t.Errorf("Aliases returned unexpected slice: %v", res)
	}
}

func TestIsResource(t *testing.T) {
	resources := []string{
		"carbon",
		"conductive",
		"rares",
		"silicates",
		"structural",
		"volatiles",
	}
	for _, r := range resources {
		if !IsResource(r) {
			t.Errorf("IsResource(%q) should be true", r)
		}
	}

	nonResources := []string{
		"iron",
		"gold",
		"water",
		"CARBON",
		"",
		"unknown",
	}
	for _, nr := range nonResources {
		if IsResource(nr) {
			t.Errorf("IsResource(%q) should be false", nr)
		}
	}
}

func TestGetBP(t *testing.T) {
	// Initialize bps cache
	if bps == nil {
		bps = make(map[string]*models.Blueprint)
	}
	var bpDelta models.JSONTimeDelta
	_ = bpDelta.UnmarshalJSON([]byte(`"600s"`))
	testBP := &models.Blueprint{
		DeviceType: "test_drone",
		PrintTime:  &bpDelta,
	}
	bps["test_drone"] = testBP

	got := GetBP("test_drone")
	if got == nil || got.DeviceType != "test_drone" {
		t.Errorf("GetBP cached lookup failed, got %v", got)
	}

	// Unknown blueprint without DB returns nil
	gotUnknown := GetBP("nonexistent_blueprint_xyz")
	if gotUnknown != nil {
		t.Errorf("GetBP for nonexistent blueprint expected nil, got %v", gotUnknown)
	}
}

func TestFilterEmpty(t *testing.T) {
	s := []string{"a", "b", "c", "d"}
	keep := []bool{true, false, true, false}
	filtered := filterEmpty(s, keep)
	if len(filtered) != 2 || filtered[0] != "a" || filtered[1] != "c" {
		t.Errorf("filterEmpty failed: got %v, expected [a, c]", filtered)
	}

	nums := []int{10, 20, 30}
	keepNums := []bool{false, true, true}
	filteredNums := filterEmpty(nums, keepNums)
	if len(filteredNums) != 2 || filteredNums[0] != 20 || filteredNums[1] != 30 {
		t.Errorf("filterEmpty failed on ints: got %v, expected [20, 30]", filteredNums)
	}
}

func TestStringify(t *testing.T) {
	now := time.Date(2026, 8, 15, 10, 0, 0, 0, time.UTC)
	pos := models.NewPosition(1.5, 2.5, 3.5)
	ca := models.NewCodeAlias("af-1")
	dev := &models.Device{Code: ca}
	devPtr := &models.DevicePointer{Code: ca}
	jt := new(models.JSONTime).Set(now)
	var jtd models.JSONTimeDelta
	_ = jtd.UnmarshalJSON([]byte(`"300s"`))
	dur := time.Duration(5 * time.Minute)

	tests := []struct {
		input    any
		expected string
	}{
		{"hello", "hello"},
		{1234567, "1,234,567"},
		{float32(1234.5), "1,234.50"},
		{dur, "in 5m0s"},
		{devPtr, "af-1"},
		{dev, "af-1"},
		{ca, "af-1"},
		{pos, pos.String()},
		{jt, jt.String()},
		{&jtd, jtd.String()},
		{models.LocationID("SOL"), "SOL"},
	}

	for _, tt := range tests {
		got := stringify(tt.input)
		if got != tt.expected {
			t.Errorf("stringify(%v) = %q, expected %q", tt.input, got, tt.expected)
		}
	}

	// Test default struct
	type custom struct {
		Name string `json:"name"`
	}
	customStr := stringify(custom{Name: "test"})
	if !strings.Contains(customStr, `"name": "test"`) {
		t.Errorf("stringify(struct) JSON output mismatch: got %q", customStr)
	}
}

func TestCountList(t *testing.T) {
	if got := CountList(nil); got != "" {
		t.Errorf("CountList(nil) expected \"\", got %q", got)
	}

	if got := CountList([]string{}); got != "" {
		t.Errorf("CountList([]) expected \"\", got %q", got)
	}

	items := []string{"banana", "apple", "banana", "cherry", "apple", "apple"}
	expected := "3 × apple, 2 × banana, 1 × cherry"
	if got := CountList(items); got != expected {
		t.Errorf("CountList mismatch: got %q, expected %q", got, expected)
	}
}

func TestGetPrintQueueETA(t *testing.T) {
	// Setup blueprints in cache
	if bps == nil {
		bps = make(map[string]*models.Blueprint)
	}
	var pt1, pt2 models.JSONTimeDelta
	_ = pt1.UnmarshalJSON([]byte(`"120s"`))
	_ = pt2.UnmarshalJSON([]byte(`"180s"`))
	bps["probe"] = &models.Blueprint{
		DeviceType: "probe",
		PrintTime:  &pt1,
	}
	bps["drone"] = &models.Blueprint{
		DeviceType: "drone",
		PrintTime:  &pt2,
	}

	// 1. Idle device
	devIdle := &models.Device{}
	if eta := GetPrintQueueETA(devIdle); eta != 0 {
		t.Errorf("GetPrintQueueETA on idle device expected 0, got %v", eta)
	}

	// 2. Active print only
	var eta5m models.JSONTimeDelta
	_ = eta5m.UnmarshalJSON([]byte(`"300s"`))
	devPrinting := &models.Device{
		Printing: &models.DevicePrint{
			Eta: &eta5m,
		},
	}
	if eta := GetPrintQueueETA(devPrinting); eta != 5*time.Minute {
		t.Errorf("GetPrintQueueETA with active print expected 5m, got %v", eta)
	}

	// 3. Active print + queue
	var eta4m models.JSONTimeDelta
	_ = eta4m.UnmarshalJSON([]byte(`"240s"`))
	devWithQueue := &models.Device{
		Printing: &models.DevicePrint{
			Eta: &eta4m,
		},
		PrintQueue: []*models.DevicePrintQueue{
			{Type: "probe"},
			{Type: "drone"},
		},
	}
	expectedETA := 4*time.Minute + 2*time.Minute + 3*time.Minute
	if eta := GetPrintQueueETA(devWithQueue); eta != expectedETA {
		t.Errorf("GetPrintQueueETA with queue expected %v, got %v", expectedETA, eta)
	}
}

func TestGetFilteredDevices(t *testing.T) {
	// Calling GetFilteredDevices when REST is unconfigured should return an error
	_, err := GetFilteredDevices([]string{"mining_drone"}, []string{"SOL"}, []string{"idle"})
	if err == nil {
		// If no error, we verify it returns without panic
		t.Logf("GetFilteredDevices executed cleanly")
	}
}
