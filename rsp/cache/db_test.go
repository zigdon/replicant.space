package cache

import (
	"testing"
)

func TestStrs(t *testing.T) {
	in := []any{"SOL", "SOL-1", "ALPHA"}
	out := Strs(in)

	if len(out) != 3 {
		t.Fatalf("Strs returned %d items, expected 3", len(out))
	}
	if out[0] != "SOL" || out[1] != "SOL-1" || out[2] != "ALPHA" {
		t.Errorf("Strs output mismatch: %v", out)
	}

	// Empty slice
	if len(Strs(nil)) != 0 {
		t.Errorf("Strs(nil) expected empty slice")
	}
}

func TestQueryStarsAlongVectorNilDB(t *testing.T) {
	orig := &Position{0, 0, 0}
	vec := &Position{1, 0, 0}
	var c *Cache
	_, err := c.QueryStarsAlongVector(orig, vec, 5.0, 0, 0, 10)
	if err == nil {
		t.Fatalf("expected error for nil cache, got nil")
	}

	c = &Cache{}
	_, err = c.QueryStarsAlongVector(orig, vec, 5.0, 0, 0, 10)
	if err == nil {
		t.Fatalf("expected error for empty cache, got nil")
	}

	_, err = c.QueryStarsAlongVector(nil, vec, 5.0, 0, 0, 10)
	if err == nil {
		t.Fatalf("expected error for nil origin, got nil")
	}
}

func TestQueryStarsAlongVectorLiveDB(t *testing.T) {
	c, err := ConnectTest("test_vector_search")
	if err != nil || c == nil || c.DB == nil {
		t.Skipf("skipping live DB test: database not accessible (%v)", err)
	}
	defer c.DB.Close()

	// SATR position: (-52.52, -22.86, -18.31)
	// Vector towards NUSAKANUS (normalized ~0.9744, 0.1005, -0.2009)
	orig := &Position{-52.52, -22.86, -18.31}
	vec := &Position{0.974444, 0.100458, -0.200916}
	records, err := c.QueryStarsAlongVector(
		orig, vec,
		5.0, 0, 0, 10,
	)
	if err != nil {
		t.Fatalf("QueryStarsAlongVector failed: %v", err)
	}
	if len(records) == 0 {
		t.Fatalf("expected at least 1 star along vector, got 0")
	}

	// First star should be NUSAKANUS (closest)
	if records[0].Designation != "NUSAKANUS" {
		t.Errorf("expected first star to be NUSAKANUS, got %s", records[0].Designation)
	}
	if records[0].Distance < 28.0 || records[0].Distance > 29.0 {
		t.Errorf("expected NUSAKANUS distance ~28.3, got %.2f", records[0].Distance)
	}
	if records[0].Deviation < 0.4 || records[0].Deviation > 0.6 {
		t.Errorf("expected NUSAKANUS deviation ~0.51 deg, got %.3f", records[0].Deviation)
	}

	// Check sorted by distance
	for i := 1; i < len(records); i++ {
		if records[i].Distance < records[i-1].Distance {
			t.Errorf("stars not sorted by distance: record %d (%.2f) < record %d (%.2f)",
				i, records[i].Distance, i-1, records[i-1].Distance)
		}
	}
}

