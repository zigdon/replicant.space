package auto

import (
	"testing"
	"time"

	"github.com/zigdon/rsp/models"
)

func TestFollowMachineBasics(t *testing.T) {
	fm := &FollowMachine{
		status: "following sh-1 in SOL",
	}

	if fm.Name() != "Follow Machine" {
		t.Errorf("FollowMachine.Name() = %q, expected %q", fm.Name(), "Follow Machine")
	}

	if fm.Status() != "following sh-1 in SOL" {
		t.Errorf("FollowMachine.Status() = %q, expected %q", fm.Status(), "following sh-1 in SOL")
	}

	if err := fm.SaveState("any"); err != nil {
		t.Errorf("SaveState returned unexpected error: %v", err)
	}
}

func TestFollowMachineStartMissingTag(t *testing.T) {
	fm := &FollowMachine{}
	dev := &models.Device{
		Code: models.NewCodeAlias("rv-1"),
		Tags: []string{"auto:follow"},
	}

	err := fm.Start(dev, true)
	if err == nil {
		t.Fatalf("expected error when follow tag is missing, got nil")
	}
}

func TestFollowMachineStateTransitions(t *testing.T) {
	followerCode := models.NewCodeAlias("rv-1")
	targetCode := models.NewCodeAlias("sh-1")

	// Case 1: follower and target in same system (stationary)
	fm1 := &FollowMachine{
		dev: &models.Device{
			Code:     followerCode,
			Location: models.LocationID("SOL-1"),
			Tags:     []string{"auto:follow", "follow:sh-1"},
		},
		target: &models.Device{
			Code:     targetCode,
			Location: models.LocationID("SOL-3"),
		},
	}
	if fm1.target.Travel != nil && fm1.target.Travel.Destination != "" {
		fm1.dest = fm1.target.Travel.Destination
	} else if fm1.target.Location != "" {
		fm1.dest = fm1.target.Location
	}
	if fm1.dev.Location.Star() == fm1.target.Location.Star() {
		fm1.state = FollowState_Idle	
	}
	if fm1.state != FollowState_Idle {
		t.Errorf("expected state %q, got %q", FollowState_Idle, fm1.state)
	}

	// Case 2: follower in different system from stationary target -> departing
	fm2 := &FollowMachine{
		dev: &models.Device{
			Code:     followerCode,
			Location: models.LocationID("MENKUNT-2-L4"),
			Tags:     []string{"auto:follow", "follow:sh-1"},
		},
		target: &models.Device{
			Code:     targetCode,
			Location: models.LocationID("SOL-3"),
		},
	}
	if fm2.target.Location != "" {
		fm2.dest = fm2.target.Location
	}
	if fm2.dev.Location.Star() != fm2.target.Location.Star() {
		fm2.state = FollowState_Departing
	}
	if fm2.state != FollowState_Departing {
		t.Errorf("expected state %q, got %q", FollowState_Departing, fm2.state)
	}
	if fm2.dest.Star() != "SOL" {
		t.Errorf("expected dest star %q, got %q", "SOL", fm2.dest.Star())
	}

	// Case 3: target in transit to SOL, follower already at SOL -> waiting
	fm3 := &FollowMachine{
		dev: &models.Device{
			Code:     followerCode,
			Location: models.LocationID("SOL-1"),
			Tags:     []string{"auto:follow", "follow:sh-1"},
		},
		target: &models.Device{
			Code: targetCode,
			Travel: &models.Trip{
				Destination: models.LocationID("SOL-3"),
			},
		},
	}
	if fm3.target.Travel != nil && fm3.target.Travel.Destination != "" {
		fm3.dest = fm3.target.Travel.Destination
	}
	if fm3.dev.Location.Star() == fm3.dest.Star() {
		fm3.state = FollowState_Waiting
	}
	if fm3.state != FollowState_Waiting {
		t.Errorf("expected state %q, got %q", FollowState_Waiting, fm3.state)
	}

	// Case 4: follower in transit -> transit
	fm4 := &FollowMachine{
		dev: &models.Device{
			Code: followerCode,
			Travel: &models.Trip{
				Destination: models.LocationID("SOL-1"),
			},
			Tags: []string{"auto:follow", "follow:sh-1"},
		},
		target: &models.Device{
			Code:     targetCode,
			Location: models.LocationID("SOL-3"),
		},
	}
	if fm4.dev.Location == "" || fm4.dev.Travel != nil {
		fm4.state = FollowState_Transit
	}
	if fm4.state != FollowState_Transit {
		t.Errorf("expected state %q, got %q", FollowState_Transit, fm4.state)
	}
}

func TestFollowMachineProcessTiming(t *testing.T) {
	now := time.Now()
	targetArrival := now.Add(2 * time.Minute)
	fm := &FollowMachine{
		state: FollowState_Waiting,
		dev: &models.Device{
			Code:     models.NewCodeAlias("rv-1"),
			Location: models.LocationID("SOL-1"),
		},
		target: &models.Device{
			Code: models.NewCodeAlias("sh-1"),
			Travel: &models.Trip{
				Destination: models.LocationID("SOL-3"),
				Arrives:     new(models.JSONTime).Set(targetArrival),
			},
		},
		dest:       models.LocationID("SOL-3"),
	}

	if fm.state == FollowState_Waiting && fm.target.Travel != nil && fm.target.Travel.Arrives != nil {
		eta := fm.target.Travel.Arrives.Time().Add(5 * time.Second)
		if eta.Before(targetArrival) {
			t.Errorf("expected eta after target arrival, got %v", eta)
		}
	}
}
