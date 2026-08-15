package common

import (
	"testing"
	"time"

	"github.com/zigdon/rsp/models"
)

func TestPrintPlanTypes(t *testing.T) {
	now := time.Now()
	pRec := &PrintPlanRec{
		Queued: []string{"probe", "mining_drone"},
		ETA:    now.Add(10 * time.Minute),
	}
	if len(pRec.Queued) != 2 || pRec.Queued[0] != "probe" {
		t.Errorf("PrintPlanRec fields mismatch: %v", pRec)
	}

	ca := models.NewCodeAlias("af-1")
	pPlan := &PrintPlan{
		Location: "SOL",
		Qty:      5,
		Device:   "probe",
		ETA:      pRec.ETA,
		Printers: map[*models.CodeAlias]*PrintPlanRec{
			ca: pRec,
		},
	}

	if pPlan.Location != "SOL" || pPlan.Qty != 5 || len(pPlan.Printers) != 1 {
		t.Errorf("PrintPlan fields mismatch: %v", pPlan)
	}
}

func TestCheckQueue(t *testing.T) {
	// CheckQueue with unconfigured REST returns 0, eta
	qty, eta := CheckQueue("SOL", "test-tag", "mining_drone", 2)
	if qty != 0 {
		t.Errorf("CheckQueue without connection expected 0 qty, got %d", qty)
	}
	if eta.IsZero() {
		t.Errorf("CheckQueue expected non-zero ETA timestamp")
	}
}

func TestPrint(t *testing.T) {
	// Seed blueprint
	if bps == nil {
		bps = make(map[string]*models.Blueprint)
	}
	var bpDelta models.JSONTimeDelta
	_ = bpDelta.UnmarshalJSON([]byte(`"300s"`))
	bps["test_unit"] = &models.Blueprint{
		DeviceType: "test_unit",
		PrintTime:  &bpDelta,
		Resources:  map[string]int{"carbon": 10},
	}

	// Calling Print without REST/DB returns error from GetFilteredDevices or rest.Location
	_, err := Print("SOL", "test_unit", 1, false, true, nil)
	if err == nil {
		t.Errorf("Print without backend expected error, got nil")
	}
}

func TestFindPrinter(t *testing.T) {
	// Empty printers slice
	res, err := FindPrinter(nil, nil)
	if err == nil {
		t.Errorf("FindPrinter(nil) expected error, got %v", res)
	}

	// Printers with unavailable devices info
	cas := []*models.CodeAlias{models.NewCodeAlias("af-1")}
	_, err = FindPrinter(cas, nil)
	if err == nil {
		t.Errorf("FindPrinter without REST API expected error, got nil")
	}
}
