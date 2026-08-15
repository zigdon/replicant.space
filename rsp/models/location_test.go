package models

import (
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
