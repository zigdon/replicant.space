package common

import (
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
	_, _, _, err := NearestHub("SOL")
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
