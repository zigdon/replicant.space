package cmd

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/zigdon/rsp/models"
)

func TestMessageFiltering(t *testing.T) {
	// Replicates and tests the filtering logic from cmd/root.go
	msgs := []*models.Message{
		{
			ID:      1,
			Type:    "discovery",
			Title:   "New Star Discovered",
			Created: new(models.JSONTime).Set(time.Now()),
		},
		{
			ID:      2,
			Type:    "notification",
			Title:   "Device low fuel",
			Created: new(models.JSONTime).Set(time.Now()),
		},
		{
			ID:      3,
			Type:    "achievement",
			Title:   "Event completed: EV-01",
			Created: new(models.JSONTime).Set(time.Now()),
		},
		{
			ID:      4,
			Type:    "achievement",
			Title:   "First Replicant Built",
			Created: new(models.JSONTime).Set(time.Now()),
		},
		{
			ID:      5,
			Type:    "system",
			Title:   "System Alert",
			Created: new(models.JSONTime).Set(time.Now()),
		},
	}

	skipped := make(map[string]int)
	var data [][]any
	var ids []int

	for _, m := range msgs {
		if slices.Contains([]string{"discovery", "notification"}, m.Type) {
			ids = append(ids, m.ID)
			skipped[m.Type]++
			continue
		}
		if m.Type == "achievement" && strings.HasPrefix(m.Title, "Event completed") {
			ids = append(ids, m.ID)
			skipped["event completed"]++
			continue
		}
		data = append(data, []any{m.Created.Time().Format(time.Kitchen), m.Title})
	}

	// Skipped items check
	if skipped["discovery"] != 1 {
		t.Errorf("Expected 1 discovery skipped, got %d", skipped["discovery"])
	}
	if skipped["notification"] != 1 {
		t.Errorf("Expected 1 notification skipped, got %d", skipped["notification"])
	}
	if skipped["event completed"] != 1 {
		t.Errorf("Expected 1 event completed achievement skipped, got %d", skipped["event completed"])
	}

	// Kept data rows check (should keep "First Replicant Built" and "System Alert")
	if len(data) != 2 {
		t.Fatalf("Expected 2 messages kept in data, got %d", len(data))
	}
	if data[0][1] != "First Replicant Built" || data[1][1] != "System Alert" {
		t.Errorf("Kept messages mismatch: %v", data)
	}

	// IDs marked for read (1, 2, 3)
	if len(ids) != 3 || ids[0] != 1 || ids[1] != 2 || ids[2] != 3 {
		t.Errorf("Mark-read IDs mismatch: %v", ids)
	}
}
