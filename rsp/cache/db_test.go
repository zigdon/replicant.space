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
