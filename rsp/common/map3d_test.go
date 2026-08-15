package common

import (
	"strings"
	"testing"

	"github.com/zigdon/rsp/models"
)

func TestVec3Math(t *testing.T) {
	v1 := NewVec3(1, 2, 3)
	v2 := NewVec3(4, 6, 8)

	sum := v1.Add(v2)
	if sum.X != 5 || sum.Y != 8 || sum.Z != 11 {
		t.Errorf("Vec3 Add failed: got %v, expected [5, 8, 11]", sum)
	}

	diff := v2.Sub(v1)
	if diff.X != 3 || diff.Y != 4 || diff.Z != 5 {
		t.Errorf("Vec3 Sub failed: got %v, expected [3, 4, 5]", diff)
	}

	dist := v1.Distance(v2)
	expectedDist := float32(7.0710678)
	if dist < expectedDist-0.01 || dist > expectedDist+0.01 {
		t.Errorf("Vec3 Distance failed: got %f, expected ~%f", dist, expectedDist)
	}
}

func TestBrailleCanvas(t *testing.T) {
	canvas := NewBrailleCanvas(10, 5)

	// Set pixel at (0, 0) -> should set dot 1 (0x01)
	canvas.SetPixel(0, 0)
	r := canvas.RuneAt(0, 0)
	if r != 0x2801 {
		t.Errorf("Braille pixel (0,0) failed: got %U (%c), expected %U", r, r, 0x2801)
	}

	// Draw a diagonal line
	canvas.DrawLine(0, 0, 4, 4)
	r2 := canvas.RuneAt(0, 0)
	if r2 == ' ' {
		t.Errorf("Expected non-empty rune after DrawLine at (0,0)")
	}
}

func TestCameraTransform(t *testing.T) {
	cam := NewCamera3D(80, 40)
	cam.Center = NewVec3(0, 0, 0)
	cam.Radius = 10.0
	cam.Mode = ProjPlaneXY

	// Test center point
	_, sx, sy, vis := cam.Transform(NewVec3(0, 0, 0))
	if !vis {
		t.Errorf("Center point should be visible")
	}
	if sx != 40 || sy != 20 {
		t.Errorf("Center point should map to (40, 20), got (%d, %d)", sx, sy)
	}

	// Test bounds
	_, _, _, visFar := cam.Transform(NewVec3(100, 100, 100))
	if visFar {
		t.Errorf("Far point should not be visible in 80x40 viewport with radius 10")
	}
}

func TestRenderGalaxyMap(t *testing.T) {
	cam := NewCamera3D(60, 25)
	cam.Center = NewVec3(0, 0, 0)
	cam.Radius = 15.0

	stars := []*models.Star{
		{
			Designation:      "SOL",
			Name:             "Sol",
			SpectralType:     "G2V",
			Position:         models.NewPosition(0, 0, 0),
			EstimatedPlanets: 8,
			HasLife:          true,
			HasMyHub:         true,
			Explored:         true,
		},
		{
			Designation:      "ALPHA",
			Name:             "Alpha Centauri",
			SpectralType:     "K1V",
			Position:         models.NewPosition(4.3, 1.2, -0.8),
			EstimatedPlanets: 3,
			Explored:         true,
		},
	}

	output, mapped := RenderGalaxyMap(cam, stars, nil)
	if len(output) == 0 {
		t.Fatalf("RenderGalaxyMap returned empty output")
	}
	if len(mapped) != 2 {
		t.Errorf("Expected 2 mapped stars, got %d", len(mapped))
	}
	plainOutput := StripANSI(output)
	if !strings.Contains(plainOutput, "Sol") && !strings.Contains(plainOutput, "SOL") {
		t.Errorf("Output should contain star label 'Sol' or 'SOL', got:\n%s", plainOutput)
	}

	// Test FilterLifeOnly
	optsLife := DefaultMapLayerOptions()
	optsLife.FilterLifeOnly = true
	_, mappedLife := RenderGalaxyMap(cam, stars, optsLife)
	if len(mappedLife) != 1 || string(mappedLife[0].Star.Designation) != "SOL" {
		t.Errorf("FilterLifeOnly failed: expected 1 star (SOL), got %d", len(mappedLife))
	}

	// Test FilterHubsOnly
	optsHubs := DefaultMapLayerOptions()
	optsHubs.FilterHubsOnly = true
	_, mappedHubs := RenderGalaxyMap(cam, stars, optsHubs)
	if len(mappedHubs) != 1 || string(mappedHubs[0].Star.Designation) != "SOL" {
		t.Errorf("FilterHubsOnly failed: expected 1 star (SOL), got %d", len(mappedHubs))
	}

	// Test Region Colors & FilterRegion
	stars[0].Region = "solzone"
	stars[1].Region = "alpha"

	optsRegion := DefaultMapLayerOptions()
	optsRegion.FilterRegion = "alpha"
	_, mappedRegion := RenderGalaxyMap(cam, stars, optsRegion)
	if len(mappedRegion) != 1 || string(mappedRegion[0].Star.Designation) != "ALPHA" {
		t.Errorf("FilterRegion failed: expected 1 star (ALPHA), got %d", len(mappedRegion))
	}

	optsShowRegions := DefaultMapLayerOptions()
	optsShowRegions.ShowRegions = true
	outRegions, _ := RenderGalaxyMap(cam, stars, optsShowRegions)
	if len(outRegions) == 0 {
		t.Fatalf("ShowRegions produced empty output")
	}

	solCol := GetRegionColor("solzone")
	if solCol.R != 0 || solCol.G != 229 || solCol.B != 255 {
		t.Errorf("GetRegionColor(solzone) failed: got %v", solCol)
	}

	// Test RenderGalaxyMapTview
	tviewOut, tviewMapped := RenderGalaxyMapTview(cam, stars, optsShowRegions)
	if len(tviewOut) == 0 || len(tviewMapped) != 2 {
		t.Fatalf("RenderGalaxyMapTview failed: output len %d, mapped %d", len(tviewOut), len(tviewMapped))
	}

	// Test Perspective Projection & Planes
	cam.Mode = Proj3DPerspective
	_, _, _, visP := cam.Transform(NewVec3(0, 0, 0))
	if !visP {
		t.Errorf("Center should be visible in perspective mode")
	}

	cam.Mode = ProjPlaneXZ
	_, _, _, visXZ := cam.Transform(NewVec3(0, 0, 0))
	if !visXZ {
		t.Errorf("Center should be visible in XZ mode")
	}

	cam.Mode = ProjPlaneYZ
	_, _, _, visYZ := cam.Transform(NewVec3(0, 0, 0))
	if !visYZ {
		t.Errorf("Center should be visible in YZ mode")
	}
}

func TestDrawLine3D(t *testing.T) {
	cam := NewCamera3D(40, 20)
	cam.Center = NewVec3(0, 0, 0)
	cam.Radius = 10.0
	cam.Mode = Proj3DOrthographic

	canvas := NewBrailleCanvas(40, 20)
	canvas.DrawLine3D(cam, NewVec3(-5, 0, 0), NewVec3(5, 0, 0))

	// Check that runes were set along the line
	hasDrawn := false
	for y := 0; y < 20; y++ {
		for x := 0; x < 40; x++ {
			if canvas.RuneAt(x, y) != ' ' {
				hasDrawn = true
				break
			}
		}
	}
	if !hasDrawn {
		t.Errorf("DrawLine3D should draw braille dots onto canvas")
	}
}

func TestVec3Extended(t *testing.T) {
	v := NewVec3(2, 3, 6)

	// Mul
	vm := v.Mul(2.0)
	if vm.X != 4 || vm.Y != 6 || vm.Z != 12 {
		t.Errorf("Vec3 Mul failed: got %v", vm)
	}

	// Length: sqrt(2^2 + 3^2 + 6^2) = sqrt(4 + 9 + 36) = sqrt(49) = 7
	length := v.Length()
	if length != 7.0 {
		t.Errorf("Vec3 Length failed: got %f, expected 7.0", length)
	}

	// String
	s := v.String()
	if s != "[2.00, 3.00, 6.00]" {
		t.Errorf("Vec3 String failed: got %q", s)
	}
}

func TestRGBExtended(t *testing.T) {
	col := RGB{R: 100, G: 150, B: 200}

	// ANSI
	ansi := col.ANSI()
	if ansi != "\x1b[38;2;100;150;200m" {
		t.Errorf("RGB ANSI mismatch: got %q", ansi)
	}

	// ANSIBg
	ansibg := col.ANSIBg()
	if ansibg != "\x1b[48;2;100;150;200m" {
		t.Errorf("RGB ANSIBg mismatch: got %q", ansibg)
	}

	// Dim: normal (0.5)
	dim50 := col.Dim(0.5)
	if dim50.R != 50 || dim50.G != 75 || dim50.B != 100 {
		t.Errorf("RGB Dim(0.5) failed: got %v", dim50)
	}

	// Dim: clamp < 0
	dimNeg := col.Dim(-0.5)
	if dimNeg.R != 0 || dimNeg.G != 0 || dimNeg.B != 0 {
		t.Errorf("RGB Dim(-0.5) failed: got %v", dimNeg)
	}

	// Dim: clamp > 1
	dimOver := col.Dim(1.5)
	if dimOver.R != 100 || dimOver.G != 150 || dimOver.B != 200 {
		t.Errorf("RGB Dim(1.5) failed: got %v", dimOver)
	}
}

func TestColorConversion(t *testing.T) {
	// Spectral Colors
	classes := []string{"O", "B", "A", "F", "G", "K", "M", "g", "", "Z"}
	for _, c := range classes {
		col := GetSpectralColor(c)
		if col.R == 0 && col.G == 0 && col.B == 0 {
			t.Errorf("GetSpectralColor(%q) returned zero color", c)
		}
	}

	// HSVtoRGB across hue angles
	for h := 0.0; h <= 360.0; h += 30.0 {
		rgb := HSVtoRGB(h, 0.8, 0.9)
		if rgb.R == 0 && rgb.G == 0 && rgb.B == 0 {
			t.Errorf("HSVtoRGB(%f) returned zero color", h)
		}
	}

	// Known Regions
	regSol := GetRegionColor("solzone")
	if regSol.R != 0 || regSol.G != 229 || regSol.B != 255 {
		t.Errorf("GetRegionColor(solzone) mismatch: %v", regSol)
	}
	regEmpty := GetRegionColor("")
	if regEmpty.R != 160 {
		t.Errorf("GetRegionColor(\"\") mismatch: %v", regEmpty)
	}
	// Unknown hashed region
	regUnknown := GetRegionColor("unknown_cluster_42")
	if regUnknown.R == 0 && regUnknown.G == 0 && regUnknown.B == 0 {
		t.Errorf("GetRegionColor(unknown) returned zero color")
	}
}

func TestFormatMapHeaderAndLegend(t *testing.T) {
	cam := NewCamera3D(80, 40)
	selectedStar := &StarMapPoint{
		Star: &models.Star{
			Designation:      "SOL",
			Name:             "Sol",
			SpectralType:     "G2V",
			EstimatedPlanets: 8,
			HasLife:          true,
			HasMyHub:         true,
			Position:         models.NewPosition(0, 0, 0),
		},
	}

	hdr := FormatMapHeader(cam, 100, 15, selectedStar)
	if !strings.Contains(hdr, "GALAXY 3D MAP") || !strings.Contains(hdr, "SOL") {
		t.Errorf("FormatMapHeader output unexpected: %s", hdr)
	}

	// Legend without regions
	leg := FormatMapLegend(nil)
	if !strings.Contains(leg, "Classes:") || !strings.Contains(leg, "My Hub") {
		t.Errorf("FormatMapLegend without regions mismatch: %s", leg)
	}

	// Legend with regions
	opts := &MapLayerOptions{ShowRegions: true}
	legRegions := FormatMapLegend(opts)
	if !strings.Contains(legRegions, "Regions:") || !strings.Contains(legRegions, "Solzone") {
		t.Errorf("FormatMapLegend with regions mismatch: %s", legRegions)
	}

	// Legend with devices
	optsDev := &MapLayerOptions{ShowDevices: true}
	legDev := FormatMapLegend(optsDev)
	if !strings.Contains(legDev, "Device") {
		t.Errorf("FormatMapLegend with devices mismatch: %s", legDev)
	}
}

func TestDeviceOverlay(t *testing.T) {
	cam := NewCamera3D(60, 25)
	cam.Center = NewVec3(0, 0, 0)
	cam.Radius = 15.0

	stars := []*models.Star{
		{
			Designation:      "SOL",
			Name:             "Sol",
			SpectralType:     "G2V",
			Position:         models.NewPosition(0, 0, 0),
			EstimatedPlanets: 8,
		},
		{
			Designation:      "ALPHA",
			Name:             "Alpha Centauri",
			SpectralType:     "K1V",
			Position:         models.NewPosition(4.3, 1.2, -0.8),
			EstimatedPlanets: 3,
		},
	}

	starDevices := map[string][]*DeviceLocationInfo{
		"SOL": {
			{Code: "AF-01", Type: "autofactory", Location: "SOL-1", Status: "idle"},
			{Code: "MD-01", Type: "mining_drone", Location: "SOL-1-1", Status: "mining"},
		},
	}

	// Test ShowDevices overlay
	opts := DefaultMapLayerOptions()
	opts.ShowDevices = true
	opts.StarDevices = starDevices
	output, mapped := RenderGalaxyMap(cam, stars, opts)

	if len(mapped) != 2 {
		t.Errorf("Expected 2 mapped stars, got %d", len(mapped))
	}
	if len(mapped[0].Devices) != 2 && len(mapped[1].Devices) != 2 {
		t.Errorf("Expected SOL to have 2 devices mapped")
	}

	plainOutput := StripANSI(output)
	if !strings.Contains(plainOutput, "[2 dev]") {
		t.Errorf("Expected star label with device count '[2 dev]', got:\n%s", plainOutput)
	}

	// Test FilterDevicesOnly
	optsOnly := DefaultMapLayerOptions()
	optsOnly.FilterDevicesOnly = true
	optsOnly.StarDevices = starDevices
	_, mappedOnly := RenderGalaxyMap(cam, stars, optsOnly)

	if len(mappedOnly) != 1 || string(mappedOnly[0].Star.Designation) != "SOL" {
		t.Errorf("FilterDevicesOnly failed: expected 1 star (SOL), got %d", len(mappedOnly))
	}
}

func TestBuildNetworkGraph(t *testing.T) {
	// Star positions
	starLookup := map[string]Vec3{
		"SOL":   {X: 0, Y: 0, Z: 0},
		"ALPHA": {X: 12, Y: 0, Z: 0}, // 12ly from SOL
		"BETA":  {X: 24, Y: 0, Z: 0}, // 12ly from ALPHA, 24ly from SOL
		"FAR":   {X: 100, Y: 0, Z: 0},
	}

	// Test 1: Relay at SOL (range 7.5) and Relay at ALPHA (range 7.5)
	// Distance is 12ly -> Neither can reach -> No connection
	devsRelayOnly := []*NetworkDevice{
		{Code: "R1", Type: "ftl_relay", Location: "SOL-1", Status: "relaying", RangeLy: 7.5},
		{Code: "R2", Type: "ftl_relay", Location: "ALPHA-1", Status: "relaying", RangeLy: 7.5},
	}
	g1 := BuildNetworkGraph(devsRelayOnly, starLookup)
	if len(g1.Links) != 0 {
		t.Errorf("Expected 0 links between relays 12ly apart, got %d", len(g1.Links))
	}
	if len(g1.Subnets) != 2 {
		t.Errorf("Expected 2 separate subnets, got %d", len(g1.Subnets))
	}

	// Test 2: System Hub at SOL (range 15) and Relay at ALPHA (range 7.5)
	// Distance is 12ly -> SOL reaches ALPHA (12 <= 15) -> Asymmetric connection valid!
	devsHubAndRelay := []*NetworkDevice{
		{Code: "H1", Type: "system_hub", Location: "SOL", Status: "relaying", RangeLy: 15.0},
		{Code: "R2", Type: "ftl_relay", Location: "ALPHA-1", Status: "relaying", RangeLy: 7.5},
	}
	g2 := BuildNetworkGraph(devsHubAndRelay, starLookup)
	if len(g2.Links) != 1 {
		t.Fatalf("Expected 1 link between hub and relay 12ly apart, got %d", len(g2.Links))
	}
	link := g2.Links[0]
	if !link.IsFromReach || link.IsToReach || link.Bidirectional {
		t.Errorf("Expected asymmetric reach from SOL to ALPHA, got FromReach=%v, ToReach=%v, Bi=%v",
			link.IsFromReach, link.IsToReach, link.Bidirectional)
	}
	if len(g2.Subnets) != 1 {
		t.Errorf("Expected 1 connected subnet, got %d", len(g2.Subnets))
	}

	// Test 3: Deep space station (range 10) and relay
	devsDeepSpace := []*NetworkDevice{
		{Code: "D1", Type: "deep_space_relay_station", Location: "ALPHA", Status: "relaying", RangeLy: 10.0},
		{Code: "R3", Type: "ftl_relay", Location: "BETA", Status: "relaying", RangeLy: 7.5},
	}
	g3 := BuildNetworkGraph(devsDeepSpace, starLookup)
	if len(g3.Links) != 0 { // 12ly is > 10.0
		t.Errorf("Expected 0 links between station and relay 12ly apart, got %d", len(g3.Links))
	}
}

func TestNetworkOverlay(t *testing.T) {
	cam := NewCamera3D(60, 25)
	cam.Center = NewVec3(0, 0, 0)
	cam.Radius = 20.0

	stars := []*models.Star{
		{Designation: "SOL", Name: "Sol", Position: models.NewPosition(0, 0, 0)},
		{Designation: "BETILGEUSE", Name: "Betilgeuse", Position: models.NewPosition(5, 0, 0)},
		{Designation: "ISOLATED", Name: "Isolated", Position: models.NewPosition(10, 10, 0)},
	}

	starLookup := map[string]Vec3{
		"SOL":        {X: 0, Y: 0, Z: 0},
		"BETILGEUSE": {X: 5, Y: 0, Z: 0},
	}

	devs := []*NetworkDevice{
		{Code: "H1", Type: "system_hub", Location: "SOL", Status: "relaying", RangeLy: 15.0},
		{Code: "R1", Type: "ftl_relay", Location: "BETILGEUSE", Status: "relaying", RangeLy: 7.5},
	}
	netGraph := BuildNetworkGraph(devs, starLookup)

	opts := DefaultMapLayerOptions()
	opts.ShowNetwork = true
	opts.Network = netGraph
	output, mapped := RenderGalaxyMap(cam, stars, opts)

	if len(mapped) != 3 {
		t.Errorf("Expected 3 mapped stars, got %d", len(mapped))
	}

	plainOutput := StripANSI(output)
	if !strings.Contains(plainOutput, "Net#1") {
		t.Errorf("Expected network badge 'Net#1' in output, got:\n%s", plainOutput)
	}

	// FilterNetworkOnly
	optsOnly := DefaultMapLayerOptions()
	optsOnly.ShowNetwork = true
	optsOnly.FilterNetworkOnly = true
	optsOnly.Network = netGraph
	_, mappedOnly := RenderGalaxyMap(cam, stars, optsOnly)

	if len(mappedOnly) != 2 {
		t.Errorf("FilterNetworkOnly failed: expected 2 networked stars, got %d", len(mappedOnly))
	}

	// Legend test
	leg := FormatMapLegend(opts)
	if !strings.Contains(leg, "Relay Net") {
		t.Errorf("FormatMapLegend with network mismatch: %s", leg)
	}
}
