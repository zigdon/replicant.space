package cmd

import (
	"strings"
	"testing"

	"github.com/zigdon/rsp/models"
)

func TestSearchMapTarget(t *testing.T) {
	stars := []*models.Star{
		{
			Designation:      "SOL",
			Name:             "Sol",
			SpectralType:     "G2V",
			Position:         models.NewPosition(0, 0, 0),
			EstimatedPlanets: 8,
		},
		{
			Designation:      "BETILGEUSE",
			Name:             "Alpha Orionis",
			SpectralType:     "M1-2",
			Position:         models.NewPosition(150, -20, 45),
			EstimatedPlanets: 0,
		},
		{
			Designation:      "GORUMIUN",
			Name:             "Gorumiun Prime",
			SpectralType:     "K3V",
			Position:         models.NewPosition(10.66, -1.68, -7.65),
			EstimatedPlanets: 4,
		},
	}

	// 1. Search by exact designation
	st, info, err := searchMapTarget("SOL", stars)
	if err != nil || st == nil || st.Designation != "SOL" {
		t.Fatalf("searchMapTarget(SOL) failed: %v", err)
	}
	if !strings.Contains(info, "SOL") {
		t.Errorf("Expected info to contain SOL, got: %s", info)
	}

	// 2. Search by system name (case insensitive)
	st, info, err = searchMapTarget("alpha orionis", stars)
	if err != nil || st == nil || st.Designation != "BETILGEUSE" {
		t.Fatalf("searchMapTarget(alpha orionis) failed: %v", err)
	}
	if !strings.Contains(info, "BETILGEUSE") {
		t.Errorf("Expected info to contain BETILGEUSE, got: %s", info)
	}

	// 3. Search by prefix
	st, _, err = searchMapTarget("Goru", stars)
	if err != nil || st == nil || st.Designation != "GORUMIUN" {
		t.Fatalf("searchMapTarget(Goru) failed: %v", err)
	}

	// 4. Search by substring
	st, _, err = searchMapTarget("Prime", stars)
	if err != nil || st == nil || st.Designation != "GORUMIUN" {
		t.Fatalf("searchMapTarget(Prime) failed: %v", err)
	}

	// 5. Search non-existent
	_, _, err = searchMapTarget("non_existent_galaxy_system_9999", stars)
	if err == nil {
		t.Errorf("Expected error for non-existent system, got nil")
	}

	// 6. Search empty
	_, _, err = searchMapTarget("", stars)
	if err == nil {
		t.Errorf("Expected error for empty query, got nil")
	}
}
