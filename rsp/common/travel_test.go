package common

import (
	"testing"
	"time"

	"github.com/zigdon/rsp/models"
)

func TestGetCachedTrip(t *testing.T) {
	// 1. Cache miss on unknown source
	if got := getCachedTrip("SRC_UNKNOWN", "DST_UNKNOWN", nil); got != nil {
		t.Errorf("getCachedTrip on unknown source expected nil, got %v", got)
	}

	// 2. Cache miss on unknown destination
	if got := getCachedTrip("SOL", "DST_UNKNOWN", nil); got != nil {
		t.Errorf("getCachedTrip on unknown destination expected nil, got %v", got)
	}

	// 3. Cache hit with matching via
	cfg := map[string]any{"via": "direct"}
	travelCache["SOL"]["ALPHA"] = &tce{
		ts:   time.Now(),
		from: "SOL",
		to:   "ALPHA",
		via:  []string{"BETA"},
		cfg:  cfg,
	}

	hit := getCachedTrip("SOL", "ALPHA", []string{"BETA"})
	if hit == nil {
		t.Fatalf("getCachedTrip expected cache hit, got nil")
	}
	if hit.cfg["via"] != "direct" {
		t.Errorf("getCachedTrip cfg mismatch: %v", hit.cfg)
	}

	// 4. Cache miss on mismatched via length
	if miss := getCachedTrip("SOL", "ALPHA", []string{"BETA", "GAMMA"}); miss != nil {
		t.Errorf("getCachedTrip with mismatched via length expected nil, got %v", miss)
	}

	// 5. Cache miss on mismatched via elements
	if miss := getCachedTrip("SOL", "ALPHA", []string{"GAMMA"}); miss != nil {
		t.Errorf("getCachedTrip with mismatched via content expected nil, got %v", miss)
	}

	// 6. Cache miss on expired entry (> 10m)
	travelCache["SOL"]["ALPHA"].ts = time.Now().Add(-15 * time.Minute)
	if expired := getCachedTrip("SOL", "ALPHA", []string{"BETA"}); expired != nil {
		t.Errorf("getCachedTrip with expired entry expected nil, got %v", expired)
	}
}

func TestTravel(t *testing.T) {
	ca := models.NewCodeAlias("mining_drone-1")
	_, err := Travel(ca, "SOL", true)
	if err == nil {
		t.Errorf("Travel without REST API expected error, got nil")
	}
}
