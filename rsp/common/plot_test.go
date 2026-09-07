package common

import (
	"container/heap"
	"slices"
	"strings"
	"testing"

	"github.com/zigdon/rsp/models"
)

func TestDistance(t *testing.T) {
	// Empty source or destination should return error
	_, err := Distance("", "SOL")
	if err == nil || !strings.Contains(err.Error(), "Can't get distance to nowhere") {
		t.Errorf("Distance(\"\", \"SOL\") expected nowhere error, got %v", err)
	}

	_, err = Distance("SOL", "")
	if err == nil || !strings.Contains(err.Error(), "Can't get distance to nowhere") {
		t.Errorf("Distance(\"SOL\", \"\") expected nowhere error, got %v", err)
	}
}

func TestPlotTrip(t *testing.T) {
	// PlotTrip with unconfigured cache/star returns error
	_, err := PlotTrip("SOL", "ALPHA", nil)
	if err == nil {
		t.Errorf("PlotTrip without cache expected error, got nil")
	}

	// PlotTrip with invalid destination position
	_, err = PlotTrip("SOL", "invalid,pos,format", nil)
	if err == nil {
		t.Errorf("PlotTrip with invalid position expected error, got nil")
	}

	// PlotTrip with custom config
	cfg := &PlotCfg{
		Hop:        5.0,
		Debug:      true,
		UseStation: true,
	}
	_, err = PlotTrip("SOL", "10,20,30", cfg)
	if err == nil {
		t.Errorf("PlotTrip to arbitrary position without DB expected error, got nil")
	}
}

func TestTripStepCandidate(t *testing.T) {
	// Unconnected DB returns error
	src := models.NewPosition(0, 0, 0)
	dst := models.NewPosition(10, 10, 10)
	legs, err := TripStepCandidate("SOL", src, dst, 0, 7.5)
	if err == nil {
		t.Errorf("TripStepCandidate with nil DB expected error, got %v", legs)
	}
}

func TestNearestHub(t *testing.T) {
	_, _, err := NearestHub(false, "SOL")
	if err == nil {
		t.Errorf("NearestHub without DB expected error, got nil")
	}
}

func TestGetPartialJourney(t *testing.T) {
	j := &models.Journey{
		Source: "SOL",
		Dest:   "ALPHA",
	}
	_, err := GetPartialJourney(j)
	if err == nil {
		t.Errorf("GetPartialJourney with nil DB expected error, got nil")
	}
}

func TestNearestRelay(t *testing.T) {
	_, err := NearestRelay("SOL")
	if err == nil {
		t.Errorf("NearestRelay without REST API expected error, got nil")
	}
}

func TestJourneyLegReversalAndRenumbering(t *testing.T) {
	legs := []*models.JourneyLeg{
		{From: "ALPHA", To: "SOL", DistFromSrc: 3.0, DistToDest: 4.3, Step: 2},
		{From: "BETA", To: "ALPHA", DistFromSrc: 2.0, DistToDest: 7.3, Step: 1},
	}

	// Reverse and swap as PlotTrip / GetPartialJourney does
	for i := range legs {
		legs[i].From, legs[i].To = legs[i].To, legs[i].From
		legs[i].DistFromSrc, legs[i].DistToDest = legs[i].DistToDest, legs[i].DistFromSrc
	}
	for i := range legs {
		legs[i].Step = i + 1
	}

	if legs[0].From != "SOL" || legs[0].To != "ALPHA" || legs[0].Step != 1 {
		t.Errorf("Leg 0 mismatch after reversal: %+v", legs[0])
	}
	if legs[1].From != "ALPHA" || legs[1].To != "BETA" || legs[1].Step != 2 {
		t.Errorf("Leg 1 mismatch after reversal: %+v", legs[1])
	}
}

func TestJourneyModelFields(t *testing.T) {
	j := &models.Journey{
		Source:     "SOL",
		Dest:       "ALPHA",
		MaxHop:     7.5,
		UseStation: true,
		UseHub:     true,
	}

	if j.Source != "SOL" || j.Dest != "ALPHA" || j.MaxHop != 7.5 || !j.UseStation || !j.UseHub {
		t.Errorf("Journey field mismatch: %+v", j)
	}

	cfg := &PlotCfg{
		Hop:        7.5,
		UseStation: true,
		UseHub:     true,
	}
	if !cfg.UseHub || !cfg.UseStation || cfg.Hop != 7.5 {
		t.Errorf("PlotCfg field mismatch: %+v", cfg)
	}
}

func TestSpatialStarGrid(t *testing.T) {
	sg := NewSpatialStarGrid(7.5)

	sg.Insert("SOL", models.NewPosition(0, 0, 0))
	sg.Insert("ALPHA", models.NewPosition(4.3, 0, 0))
	sg.Insert("BARNARD", models.NewPosition(5.9, 0, 0))
	sg.Insert("SIRIUS", models.NewPosition(8.6, 0, 0))
	sg.Insert("VEGA", models.NewPosition(25.0, 0, 0))

	if sg.Count() != 5 {
		t.Errorf("Expected 5 stars in grid, got %d", sg.Count())
	}

	// Neighbors of SOL within 7.5 ly (standard jump)
	nbrs := sg.FindNeighbors(models.NewPosition(0, 0, 0), 0, 7.5)
	if len(nbrs) != 2 {
		t.Fatalf("Expected 2 neighbors for SOL (ALPHA, BARNARD), got %d", len(nbrs))
	}
	names := []string{nbrs[0].Designation, nbrs[1].Designation}
	slices.Sort(names)
	if names[0] != "ALPHA" || names[1] != "BARNARD" {
		t.Errorf("Unexpected neighbors: %v", names)
	}

	// Extended station neighbors for SOL (7.5 to 10 ly)
	stationNbrs := sg.FindNeighbors(models.NewPosition(0, 0, 0), 7.5, 10.0)
	if len(stationNbrs) != 1 || stationNbrs[0].Designation != "SIRIUS" {
		t.Errorf("Expected SIRIUS as station neighbor, got %v", stationNbrs)
	}

	// Duplicate insert should be ignored
	sg.Insert("SOL", models.NewPosition(0, 0, 0))
	if sg.Count() != 5 {
		t.Errorf("Expected 5 stars after duplicate insert, got %d", sg.Count())
	}
}

func TestAStarPriorityQueue(t *testing.T) {
	pq := &aStarPriorityQueue{}
	heap.Init(pq)

	heap.Push(pq, &aStarItem{Star: "C", Priority: 15.0, DistToDest: 5.0})
	heap.Push(pq, &aStarItem{Star: "A", Priority: 10.0, DistToDest: 8.0})
	heap.Push(pq, &aStarItem{Star: "B2", Priority: 12.0, DistToDest: 2.0})
	heap.Push(pq, &aStarItem{Star: "B1", Priority: 12.0, DistToDest: 6.0})

	first := heap.Pop(pq).(*aStarItem)
	if first.Star != "A" {
		t.Errorf("Expected lowest priority 'A', got %s", first.Star)
	}

	// B2 and B1 have same priority (12.0), but B2 has smaller DistToDest (2.0 < 6.0)
	second := heap.Pop(pq).(*aStarItem)
	if second.Star != "B2" {
		t.Errorf("Expected tie-breaker to pick 'B2', got %s", second.Star)
	}

	third := heap.Pop(pq).(*aStarItem)
	if third.Star != "B1" {
		t.Errorf("Expected third item 'B1', got %s", third.Star)
	}

	fourth := heap.Pop(pq).(*aStarItem)
	if fourth.Star != "C" {
		t.Errorf("Expected fourth item 'C', got %s", fourth.Star)
	}
}

func TestAStarGridPathfinding(t *testing.T) {
	// Constellation with a direct path and a longer detour:
	// START (0,0,0) -> MID (5,0,0) -> END (10,0,0) [Total distance = 10]
	// START (0,0,0) -> DETOUR1 (0,6,0) -> DETOUR2 (10,6,0) -> END (10,0,0) [Total distance = 22]
	sg := NewSpatialStarGrid(7.5)
	sg.Insert("START", models.NewPosition(0, 0, 0))
	sg.Insert("MID", models.NewPosition(5, 0, 0))
	sg.Insert("END", models.NewPosition(10, 0, 0))
	sg.Insert("DETOUR1", models.NewPosition(0, 6, 0))
	sg.Insert("DETOUR2", models.NewPosition(10, 6, 0))

	sPos := sg.Get("START").Position
	dPos := sg.Get("END").Position
	origDist := sPos.Distance(dPos)

	openSet := &aStarPriorityQueue{}
	heap.Init(openSet)

	gScore := make(map[string]float32)
	gScore["START"] = 0

	cameFrom := make(map[string]*models.JourneyLeg)
	closedSet := make(map[string]bool)

	const eps float32 = 1e-4

	heap.Push(openSet, &aStarItem{
		Star:        "START",
		Position:    sPos,
		CostFromSrc: 0,
		DistFromSrc: 0,
		DistToDest:  origDist,
		Priority:    origDist,
		Step:        0,
	})

	var found bool
	for openSet.Len() > 0 {
		curr := heap.Pop(openSet).(*aStarItem)
		if closedSet[curr.Star] {
			continue
		}
		closedSet[curr.Star] = true

		if curr.Star == "END" {
			found = true
			break
		}

		neighbors := sg.FindNeighbors(curr.Position, 0, 7.5)
		for _, nbr := range neighbors {
			if closedSet[nbr.Designation] {
				continue
			}
			hopDist := curr.Position.Distance(nbr.Position)
			tentativeCost := curr.CostFromSrc + hopDist
			tentativeDist := curr.DistFromSrc + hopDist
			currentCost, visited := gScore[nbr.Designation]
			if !visited || tentativeCost < currentCost {
				gScore[nbr.Designation] = tentativeCost
				h := nbr.Position.Distance(dPos)
				f := tentativeCost + h
				priority := f*(1.0+eps) - h*eps

				cameFrom[nbr.Designation] = &models.JourneyLeg{
					From:         curr.Star,
					FromPosition: curr.Position,
					To:           nbr.Designation,
					ToPosition:   nbr.Position,
					DistFromSrc:  tentativeDist,
					DistToDest:   h,
					Step:         curr.Step + 1,
				}

				heap.Push(openSet, &aStarItem{
					Star:        nbr.Designation,
					Position:    nbr.Position,
					CostFromSrc: tentativeCost,
					DistFromSrc: tentativeDist,
					DistToDest:  h,
					Priority:    priority,
					From:        curr.Star,
					FromPos:     curr.Position,
					HopDist:     hopDist,
					Step:        curr.Step + 1,
				})
			}
		}
	}

	if !found {
		t.Fatalf("Failed to find path from START to END")
	}

	// Reconstruct path
	var path []string
	cur := "END"
	for {
		leg, ok := cameFrom[cur]
		if !ok {
			break
		}
		path = append([]string{leg.To}, path...)
		if leg.From == "START" {
			path = append([]string{leg.From}, path...)
			break
		}
		cur = leg.From
	}

	if len(path) != 3 || path[0] != "START" || path[1] != "MID" || path[2] != "END" {
		t.Errorf("Expected optimal path [START, MID, END], got %v", path)
	}
}

func TestHierarchicalHopPenalties(t *testing.T) {
	// Scenario 1: Standard hops (<=7.5) vs Station jump (9.0).
	// START (0,0,0) -> END (9,0,0) has:
	// Path A: Direct station jump of 9.0 ly (cost = 9.0 + 1000 = 1009.0)
	// Path B: 2 standard hops via MID (4.5,0,0) (cost = 4.5 + 4.5 = 9.0)
	// The pathfinder must pick Path B (pure relay hops).
	sg := NewSpatialStarGrid(15.0)
	sg.Insert("START", models.NewPosition(0, 0, 0))
	sg.Insert("MID", models.NewPosition(4.5, 0, 0))
	sg.Insert("END", models.NewPosition(9.0, 0, 0))

	sPos := sg.Get("START").Position
	dPos := sg.Get("END").Position
	origDist := sPos.Distance(dPos)

	runSearch := func(hop float32, useStation, useHub bool) []string {
		openSet := &aStarPriorityQueue{}
		heap.Init(openSet)
		gScore := make(map[string]float32)
		gScore["START"] = 0
		cameFrom := make(map[string]*models.JourneyLeg)
		closedSet := make(map[string]bool)
		const eps float32 = 1e-4

		heap.Push(openSet, &aStarItem{
			Star:        "START",
			Position:    sPos,
			CostFromSrc: 0,
			DistFromSrc: 0,
			DistToDest:  origDist,
			Priority:    origDist,
		})

		for openSet.Len() > 0 {
			curr := heap.Pop(openSet).(*aStarItem)
			if closedSet[curr.Star] {
				continue
			}
			closedSet[curr.Star] = true
			if curr.Star == "END" {
				var p []string
				c := "END"
				for {
					l, ok := cameFrom[c]
					if !ok {
						break
					}
					p = append([]string{l.To}, p...)
					if l.From == "START" {
						p = append([]string{l.From}, p...)
						break
					}
					c = l.From
				}
				return p
			}

			neighbors := sg.FindNeighbors(curr.Position, 0, hop)
			if useStation && 10.0 > hop {
				neighbors = append(neighbors, sg.FindNeighbors(curr.Position, hop, 10.0)...)
			}
			if useHub && 15.0 > hop {
				minR := hop
				if useStation && 10.0 > minR {
					minR = 10.0
				}
				neighbors = append(neighbors, sg.FindNeighbors(curr.Position, minR, 15.0)...)
			}

			for _, nbr := range neighbors {
				if closedSet[nbr.Designation] {
					continue
				}
				hopDist := curr.Position.Distance(nbr.Position)
				hopCost := hopDist
				if hopDist > 10.0 {
					hopCost += hubHopPenalty
				} else if hopDist > hop {
					if useStation {
						hopCost += stationHopPenalty
					} else {
						hopCost += hubHopPenalty
					}
				}

				tentativeCost := curr.CostFromSrc + hopCost
				tentativeDist := curr.DistFromSrc + hopDist
				currentCost, visited := gScore[nbr.Designation]
				if !visited || tentativeCost < currentCost {
					gScore[nbr.Designation] = tentativeCost
					h := nbr.Position.Distance(dPos)
					f := tentativeCost + h
					priority := f*(1.0+eps) - h*eps

					cameFrom[nbr.Designation] = &models.JourneyLeg{
						From:         curr.Star,
						FromPosition: curr.Position,
						To:           nbr.Designation,
						ToPosition:   nbr.Position,
						DistFromSrc:  tentativeDist,
						DistToDest:   h,
					}

					heap.Push(openSet, &aStarItem{
						Star:        nbr.Designation,
						Position:    nbr.Position,
						CostFromSrc: tentativeCost,
						DistFromSrc: tentativeDist,
						DistToDest:  h,
						Priority:    priority,
						From:        curr.Star,
						FromPos:     curr.Position,
						HopDist:     hopDist,
					})
				}
			}
		}
		return nil
	}

	path := runSearch(7.5, true, true)
	if len(path) != 3 || path[0] != "START" || path[1] != "MID" || path[2] != "END" {
		t.Errorf("Hierarchical penalty failed: expected standard hop path [START, MID, END], got %v", path)
	}
}
