package cmd

import (
	"slices"
	"strings"
	"testing"

	"github.com/zigdon/rsp/constants"
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
		if !slices.Contains(constants.Resources, exp) {
			t.Errorf("constants.Resources missing %q", exp)
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

func TestParseIntentResourceArgs(t *testing.T) {
	tests := []struct {
		name    string
		in      []string
		want    map[string]int
		wantErr bool
	}{
		{
			name: "Colon syntax",
			in:   []string{"carbon:400", "conductive:200"},
			want: map[string]int{"carbon": 400, "conductive": 200},
		},
		{
			name: "Wildcard",
			in:   []string{"carbon:400", "all:200"},
			want: map[string]int{
				"carbon": 600, "conductive": 200, "rares": 200,
				"silicates": 200, "structural": 200, "volatiles": 200,
			},
		},
		{
			name: "Units",
			in:   []string{"carbon:4k", "conductive:2m"},
			want: map[string]int{"carbon": 4000, "conductive": 2000000},
		},
		{
			name: "Space-separated syntax",
			in:   []string{"carbon 500", "volatiles 150"},
			want: map[string]int{"carbon": 500, "volatiles": 150},
		},
		{
			name: "Equals syntax",
			in:   []string{"rares=75"},
			want: map[string]int{"rares": 75},
		},
		{
			name:    "Invalid qty",
			in:      []string{"rares:foo"},
			wantErr: true,
		},
		{
			name:    "Missing qty",
			in:      []string{"rares"},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			res := make(map[string]int)
			for _, a := range tc.in {
				err := parseResourceArgs(&res, a)
				if (err != nil) != tc.wantErr {
					t.Fatalf("Unexpected error, want %v, got %v", tc.wantErr, err)
				}
			}
			for k, v := range tc.want {
				if v != res[k] {
					t.Errorf("Wrong value for %q: want %d, got %d", k, v, res[k])
				}
			}
			for k, v := range res {
				if _, ok := tc.want[k]; ok {
					continue
				}
				t.Errorf("Unexpected value for %q: want 0, got %d", k, v)
			}
		})
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
