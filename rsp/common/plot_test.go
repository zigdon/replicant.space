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
