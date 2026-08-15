package common

import (
	"testing"
	"time"

	"github.com/zigdon/rsp/models"
)

func TestGetRocksCached(t *testing.T) {
	// Setup cached rocks
	mockRocks := []*models.Object{
		{
			Designation: models.LocationID("ROCK-1"),
			Status:      "tracking",
		},
		{
			Designation: models.LocationID("ROCK-2"),
			Status:      "intercepted",
		},
	}

	rockTS = time.Now()
	rocks = mockRocks

	got, err := GetRocks()
	if err != nil {
		t.Fatalf("GetRocks cached returned error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("GetRocks expected 2 rocks, got %d", len(got))
	}
	if got[0].Designation != "ROCK-1" || got[1].Designation != "ROCK-2" {
		t.Errorf("GetRocks cached mismatch: %v", got)
	}
}

func TestGetRocksUncached(t *testing.T) {
	// Expire cache
	rockTS = time.Time{}
	rocks = nil

	_, err := GetRocks()
	// When REST is unconfigured, devices search returns empty list or error
	if err != nil {
		t.Logf("GetRocks uncached returned expected error: %v", err)
	}
}
