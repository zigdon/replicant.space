package common

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestHumanize(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"100", "100"},
		{"1000", "1,000"},
		{"1000000", "1,000,000"},
		{"1234567.89", "1,234,567.89"},
		{"-1234567.89", "-1,234,567.89"},
		{"-500", "-500"},
		{"-1000", "-1,000"},
		{"already,formatted", "already,formatted"},
		{"0", "0"},
		{"42", "42"},
	}

	for _, tt := range tests {
		got := humanize(tt.input)
		if got != tt.expected {
			t.Errorf("humanize(%q) = %q, expected %q", tt.input, got, tt.expected)
		}
	}
}

func TestF(t *testing.T) {
	tests := []struct {
		input    float32
		expected string
	}{
		{123.456, "123.46"},
		{1234567.8, "1,234,567.75"}, // float32 representation
		{-9876.54, "-9,876.54"},
		{0.0, "0.00"},
	}

	for _, tt := range tests {
		got := f(tt.input)
		if tt.input == 1234567.8 {
			if !strings.HasPrefix(got, "1,234,567") {
				t.Errorf("f(%f) = %q, expected prefix 1,234,567", tt.input, got)
			}
		} else if got != tt.expected {
			t.Errorf("f(%f) = %q, expected %q", tt.input, got, tt.expected)
		}
	}
}

func TestD(t *testing.T) {
	tests := []struct {
		input    int
		expected string
	}{
		{0, "0"},
		{42, "42"},
		{1000, "1,000"},
		{1234567, "1,234,567"},
		{-50000, "-50,000"},
	}

	for _, tt := range tests {
		got := d(tt.input)
		if got != tt.expected {
			t.Errorf("d(%d) = %q, expected %q", tt.input, got, tt.expected)
		}
	}
}

func TestV(t *testing.T) {
	if got := v(nil); got != "" {
		t.Errorf("v(nil) expected \"\", got %q", got)
	}

	type sample struct {
		Name string `json:"name"`
		Val  int    `json:"val"`
	}
	got := v(sample{Name: "alpha", Val: 10})
	if !strings.Contains(got, `"name": "alpha"`) || !strings.Contains(got, `"val": 10`) {
		t.Errorf("v(sample) output unexpected: %s", got)
	}
}

func TestDt(t *testing.T) {
	// Zero duration
	if got := Dt(0); got != "" {
		t.Errorf("Dt(0) expected \"\", got %q", got)
	}

	// Positive duration (< 24h)
	if got := Dt(5 * time.Minute); got != "in 5m0s" {
		t.Errorf("Dt(5m) = %q, expected \"in 5m0s\"", got)
	}

	// Negative duration (< 24h)
	if got := Dt(-10 * time.Second); got != "10s ago" {
		t.Errorf("Dt(-10s) = %q, expected \"10s ago\"", got)
	}

	// Positive duration (> 24h)
	d25h := 25*time.Hour + 30*time.Minute
	if got := Dt(d25h); got != "in 1d2h30m" {
		t.Errorf("Dt(25h30m) = %q, expected \"in 1d2h30m\"", got)
	}

	// Exactly multiple of days
	d48h := 48 * time.Hour
	if got := Dt(d48h); got != "in 2d" {
		t.Errorf("Dt(48h) = %q, expected \"in 2d\"", got)
	}

	// Negative duration (> 24h)
	neg50h := -50 * time.Hour
	if got := Dt(neg50h); !strings.HasPrefix(got, "2d") || !strings.HasSuffix(got, "ago") {
		t.Errorf("Dt(-50h) = %q, expected \"2d2h ago\"", got)
	}
}

func TestT(t *testing.T) {
	// Zero time
	if got := T(time.Time{}); got != "" {
		t.Errorf("T(time.Time{}) expected \"\", got %q", got)
	}

	// Future time
	future := time.Now().Add(2 * time.Hour)
	tFuture := T(future)
	if !strings.Contains(tFuture, "in ") {
		t.Errorf("T(future) should contain \"in \", got %q", tFuture)
	}

	// Past time
	past := time.Now().Add(-2 * time.Hour)
	tPast := T(past)
	if !strings.Contains(tPast, "ago") {
		t.Errorf("T(past) should contain \"ago\", got %q", tPast)
	}
}

func TestLines(t *testing.T) {
	if got := Lines([]string{}); got != "" {
		t.Errorf("Lines([]) expected \"\", got %q", got)
	}

	got := Lines([]string{"line1", "line2", "line3"})
	if got != "line1\nline2\nline3" {
		t.Errorf("Lines output unexpected: %q", got)
	}
}

func TestPrintTablef(t *testing.T) {
	var buf bytes.Buffer

	headers := []string{"Name", "Role", "Score", "EmptyCol"}
	data := [][]any{
		{"Alice", "Engineer\nLead", 100, ""},
		{"Bob", "Designer", 85, "0"},
	}

	PrintTablef(&buf, headers, data)
	output := buf.String()

	if !strings.Contains(output, "Alice") || !strings.Contains(output, "Bob") {
		t.Errorf("PrintTablef output missing data rows: %s", output)
	}
	if !strings.Contains(output, "Name") || !strings.Contains(output, "Role") {
		t.Errorf("PrintTablef output missing headers: %s", output)
	}

	// Test without headers (empty headers slice)
	buf.Reset()
	dataNoHeaders := [][]any{
		{"X", "Y"},
		{"1", "2"},
	}
	PrintTablef(&buf, nil, dataNoHeaders)
	if !strings.Contains(buf.String(), "X") {
		t.Errorf("PrintTablef without headers failed: %s", buf.String())
	}
}

func TestPrintTable(t *testing.T) {
	// Simple invocation check to ensure PrintTable does not panic
	PrintTable([]string{"H1"}, [][]any{{"Val1"}})
}
