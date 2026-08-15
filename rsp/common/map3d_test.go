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
}
