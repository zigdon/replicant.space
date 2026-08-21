package cache

import (
	"encoding/json"
	"testing"
)

func TestIntentRecordDemandJSON(t *testing.T) {
	rec := &IntentRecord{
		ID:       1,
		Location: "SOL-3",
		Demand: map[string]int{
			"carbon":     500,
			"conductive": 200,
		},
		Inventory: map[string]int{
			"carbon":     100,
			"conductive": 50,
			"rares":      0,
		},
	}

	data, err := json.Marshal(rec)
	if err != nil {
		t.Fatalf("Failed to marshal IntentRecord: %v", err)
	}

	var parsed IntentRecord
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal IntentRecord: %v", err)
	}

	if parsed.ID != 1 || parsed.Location != "SOL-3" {
		t.Errorf("Unexpected basic fields: ID=%d, Location=%s", parsed.ID, parsed.Location)
	}
	if parsed.Demand["carbon"] != 500 || parsed.Demand["conductive"] != 200 {
		t.Errorf("Unexpected demand: %v", parsed.Demand)
	}
	if parsed.Inventory["carbon"] != 100 || parsed.Inventory["conductive"] != 50 {
		t.Errorf("Unexpected inventory: %v", parsed.Inventory)
	}
}

func TestDemandMerging(t *testing.T) {
	existing := map[string]int{
		"carbon":     100,
		"conductive": 200,
	}

	updates := map[string]int{
		"carbon":    300,
		"volatiles": 50,
	}

	for k, v := range updates {
		existing[k] = v
	}

	if existing["carbon"] != 300 {
		t.Errorf("Expected carbon=300, got %d", existing["carbon"])
	}
	if existing["conductive"] != 200 {
		t.Errorf("Expected conductive=200, got %d", existing["conductive"])
	}
	if existing["volatiles"] != 50 {
		t.Errorf("Expected volatiles=50, got %d", existing["volatiles"])
	}
}

func TestDemandResourceRemoval(t *testing.T) {
	demand := map[string]int{
		"carbon":     100,
		"conductive": 200,
		"rares":      50,
	}

	toRemove := []string{"carbon", "rares"}
	for _, r := range toRemove {
		delete(demand, r)
	}

	if len(demand) != 1 {
		t.Fatalf("Expected 1 key remaining, got %d", len(demand))
	}
	if _, ok := demand["carbon"]; ok {
		t.Errorf("carbon was not removed")
	}
	if _, ok := demand["rares"]; ok {
		t.Errorf("rares was not removed")
	}
	if demand["conductive"] != 200 {
		t.Errorf("Expected conductive=200, got %d", demand["conductive"])
	}
}
