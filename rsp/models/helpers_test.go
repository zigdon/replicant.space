package models

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestJSONTime(t *testing.T) {
	fixedTime := time.Date(2026, 8, 20, 15, 30, 45, 0, time.UTC)
	jt := new(JSONTime).Set(fixedTime)

	if !jt.Time().Equal(fixedTime) {
		t.Errorf("JSONTime.Time() mismatch: got %v, expected %v", jt.Time(), fixedTime)
	}

	str := jt.String()
	if str != "2026-08-20T15:30:45Z" {
		t.Errorf("JSONTime.String() mismatch: got %q", str)
	}

	// Format
	formatted := jt.Format()
	if !strings.Contains(formatted, "Aug 20 15:30:45") {
		t.Errorf("JSONTime.Format() mismatch: got %q", formatted)
	}

	// Marshal / Unmarshal
	data, err := json.Marshal(jt)
	if err != nil {
		t.Fatalf("json.Marshal(JSONTime) failed: %v", err)
	}

	var unmarshaled JSONTime
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("json.Unmarshal(JSONTime) failed: %v", err)
	}
	if !unmarshaled.Time().Equal(fixedTime) {
		t.Errorf("Unmarshaled JSONTime mismatch: got %v", unmarshaled.Time())
	}

	// Empty string unmarshal
	var emptyJT JSONTime
	if err := json.Unmarshal([]byte(`""`), &emptyJT); err != nil {
		t.Errorf("Unmarshal empty string failed: %v", err)
	}

	// nil JSONTime methods
	var nilJT *JSONTime
	if nilJT.String() != "" || nilJT.Format() != "" || !nilJT.Time().IsZero() {
		t.Errorf("nil JSONTime methods returned unexpected values")
	}
	nilSet := nilJT.Set(fixedTime)
	if nilSet == nil || !nilSet.Time().Equal(fixedTime) {
		t.Errorf("nil JSONTime.Set failed: %v", nilSet)
	}
}

func TestJSONTimeDelta(t *testing.T) {
	// 1. Unmarshal from duration string "300s"
	var jtd1 JSONTimeDelta
	if err := json.Unmarshal([]byte(`"300s"`), &jtd1); err != nil {
		t.Fatalf("Unmarshal JSONTimeDelta string failed: %v", err)
	}
	if jtd1.Duration() != 300*time.Second {
		t.Errorf("Duration mismatch: got %v, expected 300s", jtd1.Duration())
	}
	if jtd1.String() != "5m0s" {
		t.Errorf("String mismatch: got %q, expected 5m0s", jtd1.String())
	}

	// 2. Unmarshal from float (120.0 seconds)
	var jtd2 JSONTimeDelta
	if err := json.Unmarshal([]byte(`120.5`), &jtd2); err != nil {
		t.Fatalf("Unmarshal JSONTimeDelta float failed: %v", err)
	}
	if jtd2.Duration() != 120*time.Second {
		t.Errorf("Duration mismatch: got %v, expected 120s", jtd2.Duration())
	}

	// 3. Marshal to JSON
	data, err := json.Marshal(&jtd1)
	if err != nil {
		t.Fatalf("Marshal JSONTimeDelta failed: %v", err)
	}
	if string(data) != "300" {
		t.Errorf("Marshal JSONTimeDelta mismatch: got %s, expected 300", string(data))
	}

	// nil methods
	var nilJTD *JSONTimeDelta
	if nilJTD.String() != "" || nilJTD.Duration() != 0 {
		t.Errorf("nil JSONTimeDelta methods returned unexpected values")
	}
}

func TestPsqlDuration(t *testing.T) {
	tests := []struct {
		input    string
		expected time.Duration
	}{
		{"01:30:15", 1*time.Hour + 30*time.Minute + 15*time.Second},
		{"00:05:00", 5 * time.Minute},
		{"02:00:00", 2 * time.Hour},
	}

	for _, tt := range tests {
		got, err := psqlDuration(tt.input)
		if err != nil {
			t.Errorf("psqlDuration(%q) unexpected error: %v", tt.input, err)
		}
		if got != tt.expected {
			t.Errorf("psqlDuration(%q) = %v, expected %v", tt.input, got, tt.expected)
		}
	}
}

type testFillableObj struct {
	Name   string `json:"name"`
	Filled bool   `json:"-"`
}

func (t *testFillableObj) Fill() error {
	t.Filled = true
	return nil
}

func TestParseHelper(t *testing.T) {
	jsonPayload := `{"name": "test_object"}`
	parsed, err := Parse[testFillableObj]([]byte(jsonPayload))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if parsed.Name != "test_object" {
		t.Errorf("Parsed object name mismatch: got %q", parsed.Name)
	}
	if !parsed.Filled {
		t.Errorf("Parse should call Fill() on Fillable interface")
	}

	// Invalid JSON
	_, err = Parse[testFillableObj]([]byte(`invalid-json`))
	if err == nil {
		t.Errorf("Parse invalid JSON expected error, got nil")
	}
}

func TestProgressTime(t *testing.T) {
	start := time.Now().Add(-5 * time.Second)
	end := time.Now().Add(5 * time.Second)
	bar := ProgressTime(10, start, end)

	if len(bar) == 0 {
		t.Errorf("ProgressTime returned empty string")
	}
	if !strings.Contains(bar, "%") {
		t.Errorf("ProgressTime should include percentage, got: %s", bar)
	}
}

func TestTreeNodes(t *testing.T) {
	node := TreeNode("Item: %s (%d)", "probe", 3)
	if node == nil || !strings.Contains(node.GetText(), "Item: probe (3)") {
		t.Errorf("TreeNode text mismatch: %v", node.GetText())
	}

	fnNode := TreeNodeFn("Status: %s", func() []any {
		return []any{"active"}
	})
	if fnNode == nil || !strings.Contains(fnNode.GetText(), "Status: active") {
		t.Errorf("TreeNodeFn text mismatch: %v", fnNode.GetText())
	}

	genNode := TreeNodeGen("Category", func() []string {
		return []string{"sub1", "sub2"}
	})
	if genNode == nil || genNode.GetText() != "Category" {
		t.Errorf("TreeNodeGen text mismatch: %v", genNode.GetText())
	}
}
