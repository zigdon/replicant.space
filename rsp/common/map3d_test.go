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
