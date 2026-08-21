package models

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/zigdon/rsp/cache"
)

func TestPosition(t *testing.T) {
	p1 := NewPosition(10.5, -20.25, 30.75)
	if p1.X != 10.5 || p1.Y != -20.25 || p1.Z != 30.75 {
		t.Errorf("NewPosition failed: got %v", p1)
	}

	cube := p1.AsCube()
	if cube.X != 10.5 || cube.Y != -20.25 || cube.Z != 30.75 {
		t.Errorf("AsCube failed: got %v", cube)
	}

	p2, err := ParsePosition("10.5, -20.25, 30.75")
	if err != nil {
		t.Fatalf("ParsePosition failed: %v", err)
	}
	if p2.X != p1.X || p2.Y != p1.Y || p2.Z != p1.Z {
		t.Errorf("ParsePosition mismatch: got %v, expected %v", p2, p1)
	}

	p3 := ParseCube(cache.Position{X: 10.5, Y: -20.25, Z: 30.75})
	if p3 == nil || p3.X != p1.X || p3.Y != p1.Y || p3.Z != p1.Z {
		t.Errorf("ParseCube mismatch: got %v, expected %v", p3, p1)
	}

	dist := p1.Distance(NewPosition(10.5, -20.25, 33.75))
	if dist < 2.99 || dist > 3.01 {
		t.Errorf("Distance calculation failed: got %f, expected ~3.0", dist)
	}
}

func TestPositionExtended(t *testing.T) {
	p1 := NewPosition(1.0, 2.0, 3.0)
	p2 := NewPosition(4.0, 6.0, 8.0)

	// Delta
	delta := p2.Delta(p1)
	if delta.X != 3.0 || delta.Y != 4.0 || delta.Z != 5.0 {
		t.Errorf("Delta failed: got %v", delta)
	}

	// Reverse
	p1.Reverse()
	if p1.X != -1.0 || p1.Y != -2.0 || p1.Z != -3.0 {
		t.Errorf("Reverse failed: got %v", p1)
	}

	// Distance with nil
	if d := p1.Distance(nil); d != 0 {
		t.Errorf("Distance with nil to expected 0, got %f", d)
	}
	var nilPos *Position
	if d := nilPos.Distance(p2); d != 0 {
		t.Errorf("Distance with nil receiver expected 0, got %f", d)
	}

	// String
	if s := p2.String(); s != "[4.00/6.00/8.00]" {
		t.Errorf("Position.String() = %q, expected \"[4.00/6.00/8.00]\"", s)
	}

	// Colon-separated ParsePosition
	pColon, err := ParsePosition("1.5:2.5:3.5")
	if err != nil || pColon.X != 1.5 || pColon.Y != 2.5 || pColon.Z != 3.5 {
		t.Errorf("ParsePosition with colons failed: got %v, err: %v", pColon, err)
	}

	// Invalid ParsePosition
	_, err = ParsePosition("invalid-coords")
	if err == nil {
		t.Errorf("ParsePosition with invalid string expected error, got nil")
	}

	// UnmarshalJSON with object
	var pObj Position
	if err := json.Unmarshal([]byte(`{"x": 10.0, "y": 20.0, "z": 30.0}`), &pObj); err != nil {
		t.Errorf("UnmarshalJSON object failed: %v", err)
	}
	if pObj.X != 10.0 || pObj.Y != 20.0 || pObj.Z != 30.0 {
		t.Errorf("UnmarshalJSON object mismatch: %v", pObj)
	}

	// UnmarshalJSON with array
	var pArr Position
	if err := json.Unmarshal([]byte(`[10.0, 20.0, 30.0]`), &pArr); err != nil {
		t.Errorf("UnmarshalJSON array failed: %v", err)
	}
	if pArr.X != 10.0 || pArr.Y != 20.0 || pArr.Z != 30.0 {
		t.Errorf("UnmarshalJSON array mismatch: %v", pArr)
	}

	// UnmarshalJSON with invalid array length
	var pInvalid Position
	if err := json.Unmarshal([]byte(`[10.0, 20.0]`), &pInvalid); err == nil {
		t.Errorf("UnmarshalJSON invalid array expected error, got nil")
	}
}

func TestBelt(t *testing.T) {
	beltJSON := `{
		"density": "rich",
		"designation": "SOL-BELT-1",
		"inner_radius_au": 2.1,
		"outer_radius_au": 3.3,
		"resources": {
			"carbon": "common",
			"silicates": "abundant",
			"rares": "scarce"
		},
		"mining": true
	}`

	var belt Belt
	if err := json.Unmarshal([]byte(beltJSON), &belt); err != nil {
		t.Fatalf("Failed to unmarshal Belt: %v", err)
	}

	if belt.Designation != "SOL-BELT-1" {
		t.Errorf("Belt designation mismatch: got %q", belt.Designation)
	}
	if belt.Density != "rich" {
		t.Errorf("Belt density mismatch: got %q", belt.Density)
	}
	if !belt.Mining {
		t.Errorf("Belt mining mismatch: got false, expected true")
	}
	if len(belt.Resources) != 3 || belt.Resources["silicates"] != "abundant" {
		t.Errorf("Belt resources mismatch: got %v", belt.Resources)
	}

	// String
	if s := belt.String(); s != "SOL-BELT-1 (rich)" {
		t.Errorf("Belt.String() = %q, expected \"SOL-BELT-1 (rich)\"", s)
	}

	// Cache nil belt
	var nilBelt *Belt
	if err := nilBelt.Cache(); err != nil {
		t.Errorf("nilBelt.Cache() expected nil, got %v", err)
	}

	// Get without DB
	ConnectDB(nil)
	if err := belt.Get(); err == nil || !strings.Contains(err.Error(), "Not connected to cache") {
		t.Errorf("Belt.Get() without DB expected Not connected error, got %v", err)
	}

	// Get with empty designation
	emptyBelt := &Belt{}
	if err := emptyBelt.Get(); err == nil {
		t.Errorf("Belt.Get() with empty designation expected error, got nil")
	}
}

func TestLocationID(t *testing.T) {
	tests := []struct {
		loc      LocationID
		expected string
	}{
		{"SOL", "SOL"},
		{"SOL-1", "SOL"},
		{"SOL-1-1", "SOL"},
		{"ALPHA-BETA-3", "ALPHA"},
		{"", ""},
	}

	for _, tt := range tests {
		if got := tt.loc.Star(); got != tt.expected {
			t.Errorf("LocationID(%q).Star() = %q, expected %q", tt.loc, got, tt.expected)
		}
	}
}

func TestStar(t *testing.T) {
	star := &Star{
		Designation: "SOL",
		Name:        "Sol",
		Position:    NewPosition(0, 0, 0),
	}

	if err := star.Fill(); err != nil {
		t.Errorf("Star.Fill() failed: %v", err)
	}
	if star.DistanceFromSol != 0 {
		t.Errorf("Star.DistanceFromSol for Sol expected 0, got %f", star.DistanceFromSol)
	}

	starAlpha := &Star{
		Designation: "ALPHA",
		Name:        "Alpha Centauri",
		Position:    NewPosition(3, 4, 0),
	}
	if err := starAlpha.Fill(); err != nil {
		t.Errorf("Star.Fill() failed: %v", err)
	}
	if starAlpha.DistanceFromSol < 4.99 || starAlpha.DistanceFromSol > 5.01 {
		t.Errorf("Star.DistanceFromSol expected 5.0, got %f", starAlpha.DistanceFromSol)
	}

	// Nil cache
	var nilStar *Star
	if err := nilStar.Cache(); err != nil {
		t.Errorf("nilStar.Cache() expected nil, got %v", err)
	}

	// Get without DB
	ConnectDB(nil)
	if err := star.Get(); err == nil || !strings.Contains(err.Error(), "Not connected to cache") {
		t.Errorf("Star.Get() without DB expected error, got %v", err)
	}

	// NewStar without DB returns error
	_, err := NewStar("SOL")
	if err == nil {
		t.Errorf("NewStar without DB expected error, got nil")
	}
}

func TestPlanetAndMoon(t *testing.T) {
	planet := &Planet{
		Designation:     "SOL-3",
		Name:            "Earth",
		InHabitableZone: true,
		Atmosphere:      true,
		DensityGcc:      5.51,
		MoonCount:       1,
	}

	var nilPlanet *Planet
	if err := nilPlanet.Cache(); err != nil {
		t.Errorf("nilPlanet.Cache() expected nil, got %v", err)
	}

	ConnectDB(nil)
	if err := planet.Get(); err == nil {
		t.Errorf("Planet.Get() without DB expected error, got nil")
	}

	emptyPlanet := &Planet{}
	if err := emptyPlanet.Get(); err == nil {
		t.Errorf("Planet.Get() with empty designation expected error, got nil")
	}

	moon := &Moon{
		Designation:  "SOL-3-1",
		Name:         "Moon",
		ParentPlanet: "SOL-3",
	}

	var nilMoon *Moon
	if err := nilMoon.Cache(); err != nil {
		t.Errorf("nilMoon.Cache() expected nil, got %v", err)
	}

	if err := moon.Get(); err == nil {
		t.Errorf("Moon.Get() without DB expected error, got nil")
	}

	emptyMoon := &Moon{}
	if err := emptyMoon.Get(); err == nil {
		t.Errorf("Moon.Get() with empty designation expected error, got nil")
	}
}

func TestLocationModel(t *testing.T) {
	loc := &Location{
		Location:   "SOL-1",
		EntryPoint: "SOL-1",
		Star: &Star{
			Designation: "SOL",
		},
		Inventory: []*Inventory{
			{ResourceType: "carbon", Quantity: 500},
		},
		Locations: map[LocationID]*LocationSummary{
			"SOL-1": {Resources: 500},
			"SOL-2": {Resources: 0},
		},
	}

	if err := loc.Fill(); err != nil {
		t.Errorf("Location.Fill() failed: %v", err)
	}
	if loc.Star.EntryPoint != "SOL-1" {
		t.Errorf("Location.Fill() entry point mismatch: got %q", loc.Star.EntryPoint)
	}

	if err := loc.Get(); err == nil || !strings.Contains(err.Error(), "Not implemented") {
		t.Errorf("Location.Get() expected Not implemented error, got %v", err)
	}
}

