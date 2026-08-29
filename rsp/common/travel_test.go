package common

import (
	"testing"

	"github.com/zigdon/rsp/models"
)

func TestGetCachedTrip(t *testing.T) {
	// 1. Cache miss on unknown source
	if got := getCachedTrip("vessel_type", "SRC_UNKNOWN", "DST_UNKNOWN"); got != nil {
		t.Errorf("getCachedTrip on unknown source expected nil, got %v", got)
	}

	// 2. Cache miss on unknown destination
	if got := getCachedTrip("vessel_type", "SOL", "DST_UNKNOWN"); got != nil {
		t.Errorf("getCachedTrip on unknown destination expected nil, got %v", got)
	}
}

func TestTravel(t *testing.T) {
	ca := models.NewCodeAlias("mining_drone-1")
	_, err := Travel(ca, "SOL", true)
	if err == nil {
		t.Errorf("Travel without REST API expected error, got nil")
	}
}
