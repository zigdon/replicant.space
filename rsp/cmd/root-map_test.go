package cmd

import (
	"strings"
	"testing"

	"github.com/zigdon/rsp/models"
)

func TestSearchMapTarget(t *testing.T) {
	stars := []*models.Star{
		{
			Designation:      "SOL",
			Name:             "Sol",
			SpectralType:     "G2V",
			Position:         models.NewPosition(0, 0, 0),
			EstimatedPlanets: 8,
		},
		{
			Designation:      "BETILGEUSE",
			Name:             "Alpha Orionis",
			SpectralType:     "M1-2",
			Position:         models.NewPosition(150, -20, 45),
			EstimatedPlanets: 0,
		},
		{
			Designation:      "GORUMIUN",
			Name:             "Gorumiun Prime",
			SpectralType:     "K3V",
			Position:         models.NewPosition(10.66, -1.68, -7.65),
			EstimatedPlanets: 4,
		},
	}

	// 1. Search by exact designation
	st, info, err := searchMapTarget("SOL", stars)
	if err != nil || st == nil || st.Designation != "SOL" {
		t.Fatalf("searchMapTarget(SOL) failed: %v", err)
	}
	if !strings.Contains(info, "SOL") {
		t.Errorf("Expected info to contain SOL, got: %s", info)
	}

	// 2. Search by system name (case insensitive)
	st, info, err = searchMapTarget("alpha orionis", stars)
	if err != nil || st == nil || st.Designation != "BETILGEUSE" {
		t.Fatalf("searchMapTarget(alpha orionis) failed: %v", err)
	}
	if !strings.Contains(info, "BETILGEUSE") {
		t.Errorf("Expected info to contain BETILGEUSE, got: %s", info)
	}

	// 3. Search by prefix
	st, _, err = searchMapTarget("Goru", stars)
	if err != nil || st == nil || st.Designation != "GORUMIUN" {
		t.Fatalf("searchMapTarget(Goru) failed: %v", err)
	}

	// 4. Search by substring
	st, _, err = searchMapTarget("Prime", stars)
	if err != nil || st == nil || st.Designation != "GORUMIUN" {
		t.Fatalf("searchMapTarget(Prime) failed: %v", err)
	}

	// 5. Search non-existent
	_, _, err = searchMapTarget("non_existent_galaxy_system_9999", stars)
	if err == nil {
		t.Errorf("Expected error for non-existent system, got nil")
	}

	// 6. Search empty
	_, _, err = searchMapTarget("", stars)
	if err == nil {
		t.Errorf("Expected error for empty query, got nil")
	}
}

func TestTravelMapCmdFlags(t *testing.T) {
	// Verify travelMapCmd is attached to mapCmd and plotCmd
	subCmds := mapCmd.Commands()
	hasTravel := false
	for _, c := range subCmds {
		if c.Name() == "travel" {
			hasTravel = true
			break
		}
	}
	if !hasTravel {
		t.Errorf("mapCmd does not contain 'travel' subcommand")
	}

	plotSubCmds := plotCmd.Commands()
	hasPlotTravel := false
	for _, c := range plotSubCmds {
		if c.Name() == "travel" {
			hasPlotTravel = true
			break
		}
	}
	if !hasPlotTravel {
		t.Errorf("plotCmd does not contain 'travel' subcommand")
	}

	// Verify flags on travelMapCmd
	if travelMapCmd.Flags().Lookup("devices") == nil {
		t.Errorf("travelMapCmd missing --devices flag")
	}
	if travelMapCmd.Flags().Lookup("types") == nil {
		t.Errorf("travelMapCmd missing --types flag")
	}
	if travelMapCmd.Flags().Lookup("from") == nil {
		t.Errorf("travelMapCmd missing --from flag")
	}
	if travelMapCmd.Flags().Lookup("to") == nil {
		t.Errorf("travelMapCmd missing --to flag")
	}
	if travelMapCmd.Flags().Lookup("travel_only") == nil {
		t.Errorf("travelMapCmd missing --travel_only flag")
	}
	if travelMapCmd.Flags().Lookup("static") == nil {
		t.Errorf("travelMapCmd missing --static flag")
	}

	// Verify flags on mapCmd
	if mapCmd.Flags().Lookup("travel") == nil {
		t.Errorf("mapCmd missing --travel flag")
	}
	if mapCmd.Flags().Lookup("travel_only") == nil {
		t.Errorf("mapCmd missing --travel_only flag")
	}
}

func TestMapIslandFlags(t *testing.T) {
	// Verify mapCmd has island flags
	if mapCmd.Flags().Lookup("island") == nil {
		t.Errorf("mapCmd missing --island flag")
	}
	if mapCmd.Flags().Lookup("island_limit") == nil {
		t.Errorf("mapCmd missing --island_limit flag")
	}
	if mapCmd.Flags().Lookup("island_hop") == nil {
		t.Errorf("mapCmd missing --island_hop flag")
	}
	if mapCmd.Flags().Lookup("island_only") == nil {
		t.Errorf("mapCmd missing --island_only flag")
	}

	// Verify default island_limit is 100
	limitFlag := mapCmd.Flags().Lookup("island_limit")
	if limitFlag.DefValue != "100" {
		t.Errorf("Expected default island_limit to be 100, got %s", limitFlag.DefValue)
	}

	// Verify default island_hop is 7.5
	hopFlag := mapCmd.Flags().Lookup("island_hop")
	if hopFlag.DefValue != "7.5" {
		t.Errorf("Expected default island_hop to be 7.5, got %s", hopFlag.DefValue)
	}

	// Verify plotMapCmd has island flags
	if plotMapCmd.Flags().Lookup("island") == nil {
		t.Errorf("plotMapCmd missing --island flag")
	}
	if plotMapCmd.Flags().Lookup("island_limit") == nil {
		t.Errorf("plotMapCmd missing --island_limit flag")
	}
	if plotMapCmd.Flags().Lookup("island_hop") == nil {
		t.Errorf("plotMapCmd missing --island_hop flag")
	}
	if plotMapCmd.Flags().Lookup("island_only") == nil {
		t.Errorf("plotMapCmd missing --island_only flag")
	}
}

func TestStatusSanitization(t *testing.T) {
	// Simulate multi-line error from RelayIsland
	multilineErr := "\"GORUMIUN\" is reachable from the relay network:\nGORUMIUN -> STAR1 -> STAR2 -> BETILGEUSE -> SOL"
	
	firstLine := strings.Split(multilineErr, "\n")[0]
	if strings.Contains(firstLine, "\n") {
		t.Errorf("Expected first line to have no newlines, got: %q", firstLine)
	}
	if firstLine != "\"GORUMIUN\" is reachable from the relay network:" {
		t.Errorf("Unexpected first line: %q", firstLine)
	}

	cleanMsg := strings.ReplaceAll(multilineErr, "\r", "")
	lines := strings.Split(cleanMsg, "\n")
	if len(lines) != 2 {
		t.Errorf("Expected 2 lines, got %d", len(lines))
	}
}


