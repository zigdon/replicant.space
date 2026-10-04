package cmd

import (
	"fmt"
	"math"
	"testing"

	"github.com/zigdon/rsp/common"
)

func TestVectorParsing(t *testing.T) {
	tests := []struct {
		input       string
		expectError bool
		expectedNx  float32
		expectedNy  float32
		expectedNz  float32
	}{
		{
			input:       "0.97,0.10,-0.20",
			expectError: false,
			expectedNx:  0.9744,
			expectedNy:  0.1005,
			expectedNz:  -0.2009,
		},
		{
			input:       "0.97, 0.10, -0.20",
			expectError: false,
			expectedNx:  0.9744,
			expectedNy:  0.1005,
			expectedNz:  -0.2009,
		},
		{
			input:       "[0.97, 0.10, -0.20]",
			expectError: false,
			expectedNx:  0.9744,
			expectedNy:  0.1005,
			expectedNz:  -0.2009,
		},
		{
			input:       "(0.97:0.10:-0.20)",
			expectError: false,
			expectedNx:  0.9744,
			expectedNy:  0.1005,
			expectedNz:  -0.2009,
		},
		{
			input:       "1, 0, 0",
			expectError: false,
			expectedNx:  1.0,
			expectedNy:  0.0,
			expectedNz:  0.0,
		},
		{
			input:       "0, 0, 0",
			expectError: true,
		},
		{
			input:       "abc,def,ghi",
			expectError: true,
		},
		{
			input:       "1, 2",
			expectError: true,
		},
	}

	for _, tt := range tests {
		vec, err := common.ParseVector(tt.input)
		if tt.expectError {
			if err == nil {
				t.Errorf("ParseVector(%q) expected error, got nil", tt.input)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseVector(%q) unexpected error: %v", tt.input, err)
			continue
		}
		if math.Abs(float64(vec.X-tt.expectedNx)) > 0.01 ||
			math.Abs(float64(vec.Y-tt.expectedNy)) > 0.01 ||
			math.Abs(float64(vec.Z-tt.expectedNz)) > 0.01 {
			t.Errorf("ParseVector(%q) = [%.4f, %.4f, %.4f], expected ~[%.4f, %.4f, %.4f]",
				tt.input, vec.X, vec.Y, vec.Z, tt.expectedNx, tt.expectedNy, tt.expectedNz)
		}
	}
}

func TestVectorDeviationAngle(t *testing.T) {
	// SATR position: (-52.52, -22.86, -18.31)
	// NUSAKANUS position: (-24.97, -19.77, -24.02)
	satrX, satrY, satrZ := float32(-52.52), float32(-22.86), float32(-18.31)
	nusX, nusY, nusZ := float32(-24.97), float32(-19.77), float32(-24.02)

	dx := nusX - satrX
	dy := nusY - satrY
	dz := nusZ - satrZ
	dist := float32(math.Sqrt(float64(dx*dx + dy*dy + dz*dz)))

	if math.Abs(float64(dist-28.30)) > 0.05 {
		t.Errorf("expected distance SATR->NUSAKANUS ~28.30, got %.2f", dist)
	}

	starNx := dx / dist
	starNy := dy / dist
	starNz := dz / dist

	// Target vector: 0.97, 0.10, -0.20 (normalized)
	targetVec, err := common.ParseVector("0.97,0.10,-0.20")
	if err != nil {
		t.Fatalf("ParseVector failed: %v", err)
	}

	// Dot product
	dot := starNx*targetVec.X + starNy*targetVec.Y + starNz*targetVec.Z
	if dot > 1.0 {
		dot = 1.0
	} else if dot < -1.0 {
		dot = -1.0
	}

	angleRad := math.Acos(float64(dot))
	angleDeg := angleRad * 180.0 / math.Pi

	if angleDeg < 0.4 || angleDeg > 0.6 {
		t.Errorf("expected NUSAKANUS deviation ~0.51 deg, got %.3f", angleDeg)
	}

	formattedDev := fmt.Sprintf("%.2f°", angleDeg)
	if formattedDev != "0.50°" && formattedDev != "0.51°" {
		t.Errorf("expected formatted deviation 0.51°, got %s", formattedDev)
	}
}
