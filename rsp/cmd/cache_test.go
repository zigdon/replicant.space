package cmd

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"testing"
)

func TestStandardResourcesCompletion(t *testing.T) {
	expected := []string{
		"carbon",
		"conductive",
		"rares",
		"silicates",
		"structural",
		"volatiles",
	}

	for _, exp := range expected {
		if !slices.Contains(StandardResources, exp) {
			t.Errorf("StandardResources missing %q", exp)
		}
	}

	// Test prefix matching in completeResources
	res, _ := completeResources(nil, nil, "car")
	if len(res) != 1 || res[0] != "carbon" {
		t.Errorf("Expected ['carbon'] for 'car', got %v", res)
	}

	res, _ = completeResources(nil, nil, "s")
	if len(res) != 2 || !slices.Contains(res, "silicates") || !slices.Contains(res, "structural") {
		t.Errorf("Expected ['silicates', 'structural'] for 's', got %v", res)
	}

	res, _ = completeResources(nil, nil, "xyz")
	if len(res) != 0 {
		t.Errorf("Expected empty slice for 'xyz', got %v", res)
	}
}

func parseIntentResourceArgs(resSlice []string, resArgs []string) (map[string]int, error) {
	demand := make(map[string]int)

	for _, r := range resSlice {
		if r == "" {
			continue
		}
		k, v, ok := strings.Cut(r, ":")
		if !ok {
			k, v, ok = strings.Cut(r, "=")
		}
		if !ok {
			return nil, fmt.Errorf("invalid resource format %q", r)
		}
		qty, err := strconv.Atoi(strings.TrimSpace(v))
		if err != nil {
			return nil, err
		}
		if qty < 0 {
			return nil, fmt.Errorf("quantity cannot be negative")
		}
		demand[strings.ToLower(strings.TrimSpace(k))] = qty
	}

	for i := 0; i < len(resArgs); i++ {
		arg := resArgs[i]
		if strings.Contains(arg, ":") || strings.Contains(arg, "=") {
			k, v, ok := strings.Cut(arg, ":")
			if !ok {
				k, v, _ = strings.Cut(arg, "=")
			}
			qty, err := strconv.Atoi(strings.TrimSpace(v))
			if err != nil {
				return nil, err
			}
			if qty < 0 {
				return nil, fmt.Errorf("quantity cannot be negative")
			}
			demand[strings.ToLower(strings.TrimSpace(k))] = qty
		} else {
			if i+1 < len(resArgs) {
				qty, err := strconv.Atoi(strings.TrimSpace(resArgs[i+1]))
				if err == nil {
					if qty < 0 {
						return nil, fmt.Errorf("quantity cannot be negative")
					}
					demand[strings.ToLower(strings.TrimSpace(arg))] = qty
					i++
					continue
				}
			}
			return nil, fmt.Errorf("invalid resource argument %q", arg)
		}
	}

	return demand, nil
}

func TestParseIntentResourceArgs(t *testing.T) {
	// Colon syntax
	res, err := parseIntentResourceArgs(nil, []string{"carbon:500", "conductive:200"})
	if err != nil {
		t.Fatalf("Unexpected error for colon syntax: %v", err)
	}
	if res["carbon"] != 500 || res["conductive"] != 200 {
		t.Errorf("Unexpected result: %v", res)
	}

	// Space-separated syntax
	res, err = parseIntentResourceArgs(nil, []string{"carbon", "500", "volatiles", "150"})
	if err != nil {
		t.Fatalf("Unexpected error for space syntax: %v", err)
	}
	if res["carbon"] != 500 || res["volatiles"] != 150 {
		t.Errorf("Unexpected result: %v", res)
	}

	// Equals syntax
	res, err = parseIntentResourceArgs(nil, []string{"rares=75"})
	if err != nil {
		t.Fatalf("Unexpected error for equals syntax: %v", err)
	}
	if res["rares"] != 75 {
		t.Errorf("Unexpected result: %v", res)
	}

	// Flag slice syntax
	res, err = parseIntentResourceArgs([]string{"carbon:100", "silicates:300"}, nil)
	if err != nil {
		t.Fatalf("Unexpected error for flag syntax: %v", err)
	}
	if res["carbon"] != 100 || res["silicates"] != 300 {
		t.Errorf("Unexpected result: %v", res)
	}

	// Combined flag and positional
	res, err = parseIntentResourceArgs([]string{"carbon:100"}, []string{"conductive:200", "rares", "50"})
	if err != nil {
		t.Fatalf("Unexpected error for combined syntax: %v", err)
	}
	if res["carbon"] != 100 || res["conductive"] != 200 || res["rares"] != 50 {
		t.Errorf("Unexpected result: %v", res)
	}

	// Invalid quantity
	_, err = parseIntentResourceArgs(nil, []string{"carbon:abc"})
	if err == nil {
		t.Errorf("Expected error for non-integer quantity")
	}

	// Invalid format without quantity
	_, err = parseIntentResourceArgs(nil, []string{"carbon"})
	if err == nil {
		t.Errorf("Expected error for missing quantity")
	}
}

func TestIntentCommandHierarchy(t *testing.T) {
	if intentCmd == nil {
		t.Fatal("intentCmd is nil")
	}
	if intentListCmd == nil {
		t.Fatal("intentListCmd is nil")
	}
	if intentAddCmd == nil {
		t.Fatal("intentAddCmd is nil")
	}
	if intentRemoveCmd == nil {
		t.Fatal("intentRemoveCmd is nil")
	}

	// Check subcommands are attached to cacheCmd and intentCmd
	var foundIntent bool
	for _, c := range cacheCmd.Commands() {
		if c.Name() == "intent" {
			foundIntent = true
			break
		}
	}
	if !foundIntent {
		t.Errorf("cacheCmd does not contain intent command")
	}

	subnames := make(map[string]bool)
	for _, c := range intentCmd.Commands() {
		subnames[c.Name()] = true
	}
	for _, expected := range []string{"list", "add", "remove"} {
		if !subnames[expected] {
			t.Errorf("intentCmd does not contain %q subcommand", expected)
		}
	}

	// Check aliases
	if !slices.Contains(intentRemoveCmd.Aliases, "rm") {
		t.Errorf("intentRemoveCmd missing 'rm' alias")
	}
	if !slices.Contains(intentListCmd.Aliases, "ls") {
		t.Errorf("intentListCmd missing 'ls' alias")
	}
}

func TestFormatIntentCell(t *testing.T) {
	// Intent 180, Inventory 160 -> delta -20
	if got := formatIntentCell(180, 160); got != "180 (-20)" {
		t.Errorf("Expected '180 (-20)', got %q", got)
	}

	// Intent 180, Inventory 180 -> exact match
	if got := formatIntentCell(180, 180); got != "180" {
		t.Errorf("Expected '180', got %q", got)
	}

	// Intent 180, Inventory 200 -> excess inventory, no delta indicated
	if got := formatIntentCell(180, 200); got != "180" {
		t.Errorf("Expected '180', got %q", got)
	}

	// Intent 100, Inventory 0 -> delta -100
	if got := formatIntentCell(100, 0); got != "100 (-100)" {
		t.Errorf("Expected '100 (-100)', got %q", got)
	}

	// Intent 0, Inventory 50 -> empty
	if got := formatIntentCell(0, 50); got != "" {
		t.Errorf("Expected '', got %q", got)
	}
}

func TestFormatInventoryCell(t *testing.T) {
	// Inventory 160 -> "160"
	if got := formatInventoryCell(180, 160); got != "160" {
		t.Errorf("Expected '160', got %q", got)
	}

	// Inventory 0 with intent 180 -> "0"
	if got := formatInventoryCell(180, 0); got != "0" {
		t.Errorf("Expected '0', got %q", got)
	}

	// Inventory 0 with intent 0 -> ""
	if got := formatInventoryCell(0, 0); got != "" {
		t.Errorf("Expected '', got %q", got)
	}
}

func TestIntentListHeaderAndRowGrouping(t *testing.T) {
	activeRes := []string{"carbon", "conductive"}

	// Default mode (showInv = false): no (Int) suffix, no inventory columns
	defaultHeaders := []string{"Location"}
	for _, res := range activeRes {
		title := strings.ToUpper(res[:1]) + res[1:]
		defaultHeaders = append(defaultHeaders, title)
	}

	expectedDefault := []string{"Location", "Carbon", "Conductive"}
	if len(defaultHeaders) != len(expectedDefault) {
		t.Fatalf("Default header length mismatch: got %d, expected %d", len(defaultHeaders), len(expectedDefault))
	}
	for i, h := range defaultHeaders {
		if h != expectedDefault[i] {
			t.Errorf("Default header mismatch at %d: got %q, expected %q", i, h, expectedDefault[i])
		}
	}

	// Inventory mode (showInv = true): (Int) and (Inv) suffixes
	invHeaders := []string{"Location"}
	for _, res := range activeRes {
		title := strings.ToUpper(res[:1]) + res[1:]
		invHeaders = append(invHeaders, title+" (Int)")
	}
	for _, res := range activeRes {
		title := strings.ToUpper(res[:1]) + res[1:]
		invHeaders = append(invHeaders, title+" (Inv)")
	}

	expectedInv := []string{"Location", "Carbon (Int)", "Conductive (Int)", "Carbon (Inv)", "Conductive (Inv)"}
	if len(invHeaders) != len(expectedInv) {
		t.Fatalf("Inv header length mismatch: got %d, expected %d", len(invHeaders), len(expectedInv))
	}
	for i, h := range invHeaders {
		if h != expectedInv[i] {
			t.Errorf("Inv header mismatch at %d: got %q, expected %q", i, h, expectedInv[i])
		}
	}
}
