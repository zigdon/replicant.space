package cmd

import (
	"strings"
	"testing"

	"github.com/zigdon/rsp/cache"
	"github.com/zigdon/rsp/common"
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

func TestNeighbourFlags(t *testing.T) {
	// Verify mapCmd neighbour flags
	if mapCmd.Flags().Lookup("distances") == nil {
		t.Errorf("mapCmd missing --distances flag")
	}
	if mapCmd.Flags().ShorthandLookup("D") == nil {
		t.Errorf("mapCmd missing -D shorthand flag")
	}

	// Verify plotMapCmd neighbour flags
	if plotMapCmd.Flags().Lookup("distances") == nil {
		t.Errorf("plotMapCmd missing --distances flag")
	}
	if plotMapCmd.Flags().ShorthandLookup("D") == nil {
		t.Errorf("plotMapCmd missing -D shorthand flag")
	}
}

func TestMapMiningFlags(t *testing.T) {
	// Verify mapCmd mining flags
	if mapCmd.Flags().Lookup("mining") == nil {
		t.Errorf("mapCmd missing --mining flag")
	}
	if mapCmd.Flags().ShorthandLookup("m") == nil {
		t.Errorf("mapCmd missing -m shorthand flag")
	}
	if mapCmd.Flags().Lookup("mining_only") == nil {
		t.Errorf("mapCmd missing --mining_only flag")
	}

	// Verify plotMapCmd mining flags
	if plotMapCmd.Flags().Lookup("mining") == nil {
		t.Errorf("plotMapCmd missing --mining flag")
	}
	if plotMapCmd.Flags().ShorthandLookup("M") == nil {
		t.Errorf("plotMapCmd missing -M shorthand flag")
	}
	if plotMapCmd.Flags().Lookup("mining_only") == nil {
		t.Errorf("plotMapCmd missing --mining_only flag")
	}
}

func TestLoadNeighboursForStar(t *testing.T) {
	center := &models.Star{
		Designation: "SOL",
		Name:        "Sol",
		Position:    models.NewPosition(0, 0, 0),
	}

	allStars := []*models.Star{
		center,
		{
			Designation:  "ALPHA",
			Name:         "Alpha Centauri",
			Position:     models.NewPosition(4.3, 0, 0),
			SpectralType: "K1V",
			HasLife:      true,
		},
		{
			Designation:  "BARNARD",
			Name:         "Barnard's Star",
			Position:     models.NewPosition(0, 8.5, 0),
			SpectralType: "M4V",
		},
		{
			Designation:  "SIRIUS",
			Name:         "Sirius",
			Position:     models.NewPosition(0, 0, 12.5),
			SpectralType: "A1V",
			HasHub:       true, // HasHub -> RelayDevice = "system_hub"
		},
		{
			Designation: "FAR_STAR",
			Name:        "Far Star",
			Position:    models.NewPosition(20.0, 0, 0), // 20.0ly -> exceeds 15ly cap
		},
	}

	// 1. Default max distance (capped at 15ly)
	neighbours := loadNeighboursForStar(center, 15.0, allStars)
	if len(neighbours) != 3 {
		t.Fatalf("Expected 3 neighbours within 15ly, got %d", len(neighbours))
	}

	// Verify sorting (closest first) and relay devices
	if neighbours[0].Star.Designation != "ALPHA" || neighbours[0].Distance != 4.3 || neighbours[0].RelayDevice != "" {
		t.Errorf("Neighbour #1 mismatch: got %+v", neighbours[0])
	}
	if neighbours[1].Star.Designation != "BARNARD" || neighbours[1].Distance != 8.5 || neighbours[1].RelayDevice != "" {
		t.Errorf("Neighbour #2 mismatch: got %+v", neighbours[1])
	}
	if neighbours[2].Star.Designation != "SIRIUS" || neighbours[2].Distance != 12.5 || neighbours[2].RelayDevice != "system_hub" {
		t.Errorf("Neighbour #3 mismatch (expected system_hub): got %+v", neighbours[2])
	}

	// 2. Custom smaller max distance (e.g. 7.5ly)
	closeNeighbours := loadNeighboursForStar(center, 7.5, allStars)
	if len(closeNeighbours) != 1 || closeNeighbours[0].Star.Designation != "ALPHA" {
		t.Errorf("Expected 1 close neighbour <= 7.5ly (ALPHA), got %d", len(closeNeighbours))
	}

	// 3. Attempting max distance > 15ly should be capped at 15ly
	cappedNeighbours := loadNeighboursForStar(center, 100.0, allStars)
	if len(cappedNeighbours) != 3 {
		t.Errorf("Expected max_dist to be capped at 15ly (3 neighbours), got %d", len(cappedNeighbours))
	}

	// 4. Center with nil position
	if nils := loadNeighboursForStar(&models.Star{}, 15.0, allStars); nils != nil {
		t.Errorf("Expected nil for star with nil position, got %+v", nils)
	}
}

func TestSelectionPersistenceInViewport(t *testing.T) {
	cam := common.NewCamera3D(80, 24)
	cam.Center = common.Vec3{X: 0, Y: 0, Z: 0}
	cam.Radius = 50.0

	stars := []*models.Star{
		{
			Designation: "SOL",
			Position:    models.NewPosition(0, 0, 0),
		},
		{
			Designation: "ALPHA",
			Position:    models.NewPosition(4.3, 0, 0),
		},
		{
			Designation: "SIRIUS",
			Position:    models.NewPosition(0, 0, 8.6),
		},
	}

	opts := common.DefaultMapLayerOptions()
	opts.SelectedStar = "ALPHA"

	// 1. Initial render
	_, mapped := common.RenderGalaxyMap(cam, stars, opts)
	if len(mapped) != 3 {
		t.Fatalf("Expected 3 mapped stars, got %d", len(mapped))
	}

	// Check that ALPHA is in mapped
	foundIndex := -1
	for i, mp := range mapped {
		if mp.Star != nil && string(mp.Star.Designation) == opts.SelectedStar {
			foundIndex = i
			break
		}
	}
	if foundIndex < 0 {
		t.Errorf("Expected ALPHA to be found in mapped stars")
	}

	// 2. Pan camera slightly (ALPHA still within viewport)
	cam.Center.X += 5.0
	_, mappedPan := common.RenderGalaxyMap(cam, stars, opts)
	foundPanIndex := -1
	for i, mp := range mappedPan {
		if mp.Star != nil && string(mp.Star.Designation) == opts.SelectedStar {
			foundPanIndex = i
			break
		}
	}
	if foundPanIndex < 0 {
		t.Errorf("Expected ALPHA to remain mapped after camera pan")
	}

	// 3. Zoom camera in (ALPHA still within viewport radius 10)
	cam.Center = common.Vec3{X: 0, Y: 0, Z: 0}
	cam.Radius = 10.0
	_, mappedZoom := common.RenderGalaxyMap(cam, stars, opts)
	foundZoomIndex := -1
	for i, mp := range mappedZoom {
		if mp.Star != nil && string(mp.Star.Designation) == opts.SelectedStar {
			foundZoomIndex = i
			break
		}
	}
	if foundZoomIndex < 0 {
		t.Errorf("Expected ALPHA to remain mapped after camera zoom")
	}
}

func TestLoadMinedBeltsStarResolution(t *testing.T) {
	records := []*cache.MinedBeltRecord{
		{Designation: "ABUNA-BELT-1", Star: "", Density: "moderate"},
		{Designation: "ACAMARAN-BELT-1", Star: "", Density: "dense"},
		{Designation: "QUADANAL-BELT-1", Star: "QUADANAL", Density: "dense"},
	}

	mined := make(map[string][]*common.MinedBeltInfo)
	for _, r := range records {
		starName := strings.ToUpper(strings.TrimSpace(r.Star))
		if starName == "" {
			starName = strings.ToUpper(strings.TrimSpace(models.LocationID(r.Designation).Star()))
		}
		if starName == "" {
			continue
		}
		mined[starName] = append(mined[starName], &common.MinedBeltInfo{
			Designation: r.Designation,
			Star:        starName,
			Density:     r.Density,
			Resources:   r.Resources,
		})
	}

	if len(mined) != 3 {
		t.Fatalf("Expected 3 distinct systems, got %d", len(mined))
	}
	if _, ok := mined["ABUNA"]; !ok {
		t.Errorf("Expected ABUNA to be resolved from ABUNA-BELT-1")
	}
	if _, ok := mined["ACAMARAN"]; !ok {
		t.Errorf("Expected ACAMARAN to be resolved from ACAMARAN-BELT-1")
	}
	if _, ok := mined["QUADANAL"]; !ok {
		t.Errorf("Expected QUADANAL to be resolved")
	}
}



