package cmd

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/zigdon/rsp/models"
)

func TestBobCommandHierarchy(t *testing.T) {
	if bobCmd == nil {
		t.Fatal("bobCmd is nil")
	}
	if bobListCmd == nil {
		t.Fatal("bobListCmd is nil")
	}
	if bobTableCmd == nil {
		t.Fatal("bobTableCmd is nil")
	}
	if bobSendCmd == nil {
		t.Fatal("bobSendCmd is nil")
	}

	// Verify bobCmd is a subcommand of msgCmd
	var foundBob bool
	for _, c := range msgCmd.Commands() {
		if c.Name() == "bob" {
			foundBob = true
			break
		}
	}
	if !foundBob {
		t.Errorf("msgCmd does not contain 'bob' command")
	}

	// Verify subcommands under bobCmd
	subnames := make(map[string]bool)
	for _, c := range bobCmd.Commands() {
		subnames[c.Name()] = true
	}
	for _, expected := range []string{"list", "table", "send"} {
		if !subnames[expected] {
			t.Errorf("bobCmd does not contain %q subcommand", expected)
		}
	}

	// Verify tableCmd aliases
	if !slices.Contains(bobTableCmd.Aliases, "browse") || !slices.Contains(bobTableCmd.Aliases, "view") {
		t.Errorf("bobTableCmd missing 'browse' or 'view' aliases: %v", bobTableCmd.Aliases)
	}
}

func TestBobnetFiltering(t *testing.T) {
	msgs := []*models.Bob{
		{
			Id:            1,
			Channel:       "#general",
			ReplicantName: "Dash",
			ReplicantCode: "340E4CA3",
			CurrentStar:   "BALEL",
			Message:       "Hello general\nmulti-line",
			Time:          models.NewJsonTime(time.Now().Add(-10 * time.Minute)),
		},
		{
			Id:            2,
			Channel:       "#trade",
			ReplicantName: "Dash",
			ReplicantCode: "340E4CA3",
			CurrentStar:   "BALEL",
			Message:       "Selling ore",
			Time:          models.NewJsonTime(time.Now().Add(-5 * time.Minute)),
		},
		{
			Id:            3,
			Channel:       "#general",
			ReplicantName: "Riker",
			ReplicantCode: "B3DDEDE7",
			CurrentStar:   "SOL",
			Message:       "Colony update",
			Time:          models.NewJsonTime(time.Now().Add(-1 * time.Minute)),
		},
	}

	// Filter by channel #general
	var generalMsgs []*models.Bob
	for _, m := range msgs {
		if m.Channel == "#general" {
			generalMsgs = append(generalMsgs, m)
		}
	}
	if len(generalMsgs) != 2 {
		t.Fatalf("Expected 2 #general messages, got %d", len(generalMsgs))
	}

	// Filter by replicant "Dash"
	var dashMsgs []*models.Bob
	for _, m := range msgs {
		if m.ReplicantName == "Dash" {
			dashMsgs = append(dashMsgs, m)
		}
	}
	if len(dashMsgs) != 2 {
		t.Fatalf("Expected 2 Dash messages, got %d", len(dashMsgs))
	}

	// Filter by both channel "#trade" and replicant "Dash"
	var tradeDashMsgs []*models.Bob
	for _, m := range msgs {
		if m.Channel == "#trade" && m.ReplicantName == "Dash" {
			tradeDashMsgs = append(tradeDashMsgs, m)
		}
	}
	if len(tradeDashMsgs) != 1 || tradeDashMsgs[0].Id != 2 {
		t.Fatalf("Expected message 2 for #trade and Dash, got %v", tradeDashMsgs)
	}

	// Test preview single line formatting
	preview := strings.ReplaceAll(msgs[0].Message, "\n", " ")
	if strings.Contains(preview, "\n") {
		t.Errorf("Preview should not contain newlines: %q", preview)
	}
	if preview != "Hello general multi-line" {
		t.Errorf("Unexpected preview: %q", preview)
	}
}

func TestBobnetSenderFormatting(t *testing.T) {
	m := &models.Bob{
		ReplicantName: "Dash",
		ReplicantCode: "340E4CA3",
		CurrentStar:   "BALEL",
	}

	// Without IDs/Locs
	whoSimple := m.ReplicantName
	if whoSimple != "Dash" {
		t.Errorf("Expected 'Dash', got %q", whoSimple)
	}

	// With IDs/Locs
	code := m.ReplicantCode
	if code != "" {
		code = "#" + code
	}
	star := m.CurrentStar
	if star != "" {
		star = "@" + star
	}
	whoDetailed := m.ReplicantName + " (" + code + star + ")"
	if whoDetailed != "Dash (#340E4CA3@BALEL)" {
		t.Errorf("Expected 'Dash (#340E4CA3@BALEL)', got %q", whoDetailed)
	}
}
