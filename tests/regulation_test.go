package tests

import (
	"math"
	"os"
	"testing"

	"github.com/oisis/EldenRing-SaveEditor/backend/core"
)

func TestReadNetworkParams_PC(t *testing.T) {
	save := loadPCSave(t)
	if len(save.UserData11) == 0 {
		t.Skip("PC save has no UserData11")
	}

	vals, err := core.ReadNetworkParams(save.UserData11)
	if err != nil {
		t.Fatalf("ReadNetworkParams: %v", err)
	}

	if vals.MaxBreakInTargetListCount != 5 {
		t.Errorf("maxBreakInTargetListCount = %d, want 5", vals.MaxBreakInTargetListCount)
	}
	if !floatEq(vals.BreakInRequestIntervalTimeSec, 30.0) {
		t.Errorf("breakInRequestIntervalTimeSec = %f, want 30.0", vals.BreakInRequestIntervalTimeSec)
	}
	if !floatEq(vals.BreakInRequestTimeOutSec, 20.0) {
		t.Errorf("breakInRequestTimeOutSec = %f, want 20.0", vals.BreakInRequestTimeOutSec)
	}
}

func TestReadNetworkParams_PS4(t *testing.T) {
	save := loadPS4Save(t)
	if len(save.UserData11) == 0 {
		t.Skip("PS4 save has no UserData11")
	}

	vals, err := core.ReadNetworkParams(save.UserData11)
	if err != nil {
		t.Fatalf("ReadNetworkParams: %v", err)
	}

	if vals.MaxBreakInTargetListCount != 5 {
		t.Errorf("maxBreakInTargetListCount = %d, want 5", vals.MaxBreakInTargetListCount)
	}
	if !floatEq(vals.BreakInRequestIntervalTimeSec, 30.0) {
		t.Errorf("breakInRequestIntervalTimeSec = %f, want 30.0", vals.BreakInRequestIntervalTimeSec)
	}
	if !floatEq(vals.BreakInRequestTimeOutSec, 20.0) {
		t.Errorf("breakInRequestTimeOutSec = %f, want 20.0", vals.BreakInRequestTimeOutSec)
	}
}

func TestPatchNetworkParams_PC_RoundTrip(t *testing.T) {
	save := loadPCSave(t)
	if len(save.UserData11) == 0 {
		t.Skip("PC save has no UserData11")
	}

	patch := core.NetworkParamFast()
	patched, err := core.PatchNetworkParams(save.UserData11, patch)
	if err != nil {
		t.Fatalf("PatchNetworkParams: %v", err)
	}

	// Verify patched values can be read back
	vals, err := core.ReadNetworkParams(patched)
	if err != nil {
		t.Fatalf("ReadNetworkParams after patch: %v", err)
	}

	if vals.MaxBreakInTargetListCount != 10 {
		t.Errorf("maxBreakInTargetListCount = %d, want 10", vals.MaxBreakInTargetListCount)
	}
	if !floatEq(vals.BreakInRequestIntervalTimeSec, 4.0) {
		t.Errorf("breakInRequestIntervalTimeSec = %f, want 4.0", vals.BreakInRequestIntervalTimeSec)
	}
	if !floatEq(vals.BreakInRequestTimeOutSec, 4.0) {
		t.Errorf("breakInRequestTimeOutSec = %f, want 4.0", vals.BreakInRequestTimeOutSec)
	}

	// Verify reset works
	reset, err := core.PatchNetworkParams(patched, core.NetworkParamDefaults())
	if err != nil {
		t.Fatalf("PatchNetworkParams (reset): %v", err)
	}
	valsReset, err := core.ReadNetworkParams(reset)
	if err != nil {
		t.Fatalf("ReadNetworkParams after reset: %v", err)
	}
	if valsReset.MaxBreakInTargetListCount != 5 {
		t.Errorf("reset maxBreakInTargetListCount = %d, want 5", valsReset.MaxBreakInTargetListCount)
	}
	if !floatEq(valsReset.BreakInRequestIntervalTimeSec, 30.0) {
		t.Errorf("reset breakInRequestIntervalTimeSec = %f, want 30.0", valsReset.BreakInRequestIntervalTimeSec)
	}
}

func TestPatchNetworkParams_PS4_RoundTrip(t *testing.T) {
	save := loadPS4Save(t)
	if len(save.UserData11) == 0 {
		t.Skip("PS4 save has no UserData11")
	}

	patch := core.NetworkParamFast()
	patched, err := core.PatchNetworkParams(save.UserData11, patch)
	if err != nil {
		t.Fatalf("PatchNetworkParams: %v", err)
	}

	vals, err := core.ReadNetworkParams(patched)
	if err != nil {
		t.Fatalf("ReadNetworkParams after patch: %v", err)
	}

	if vals.MaxBreakInTargetListCount != 10 {
		t.Errorf("maxBreakInTargetListCount = %d, want 10", vals.MaxBreakInTargetListCount)
	}
	if !floatEq(vals.BreakInRequestIntervalTimeSec, 4.0) {
		t.Errorf("breakInRequestIntervalTimeSec = %f, want 4.0", vals.BreakInRequestIntervalTimeSec)
	}
	if !floatEq(vals.BreakInRequestTimeOutSec, 4.0) {
		t.Errorf("breakInRequestTimeOutSec = %f, want 4.0", vals.BreakInRequestTimeOutSec)
	}
}

func TestPatchNetworkParams_Validation(t *testing.T) {
	save := loadPCSave(t)
	if len(save.UserData11) == 0 {
		t.Skip("PC save has no UserData11")
	}

	tests := []struct {
		name  string
		patch core.NetworkParamValues
	}{
		{"targets too low", core.NetworkParamValues{MaxBreakInTargetListCount: 0, BreakInRequestIntervalTimeSec: 4, BreakInRequestTimeOutSec: 4}},
		{"targets too high", core.NetworkParamValues{MaxBreakInTargetListCount: 21, BreakInRequestIntervalTimeSec: 4, BreakInRequestTimeOutSec: 4}},
		{"interval too low", core.NetworkParamValues{MaxBreakInTargetListCount: 5, BreakInRequestIntervalTimeSec: 1.0, BreakInRequestTimeOutSec: 4}},
		{"interval too high", core.NetworkParamValues{MaxBreakInTargetListCount: 5, BreakInRequestIntervalTimeSec: 31.0, BreakInRequestTimeOutSec: 4}},
		{"timeout too low", core.NetworkParamValues{MaxBreakInTargetListCount: 5, BreakInRequestIntervalTimeSec: 4, BreakInRequestTimeOutSec: 2.0}},
		{"timeout too high", core.NetworkParamValues{MaxBreakInTargetListCount: 5, BreakInRequestIntervalTimeSec: 4, BreakInRequestTimeOutSec: 21.0}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := core.PatchNetworkParams(save.UserData11, tc.patch)
			if err == nil {
				t.Errorf("expected validation error for %s", tc.name)
			}
		})
	}
}

// --- helpers ---

func loadPCSave(t *testing.T) *core.SaveFile {
	t.Helper()
	path := os.Getenv("ER_TEST_PC_SAVE")
	if path == "" {
		path = "../tmp/save/ER0000.sl2"
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Skip("PC save not available")
	}
	save, err := core.LoadSave(path)
	if err != nil {
		t.Fatalf("LoadSave PC: %v", err)
	}
	return save
}

func loadPS4Save(t *testing.T) *core.SaveFile {
	t.Helper()
	path := os.Getenv("ER_TEST_PS4_SAVE")
	if path == "" {
		path = "../tmp/save/oisisk_ps4.txt"
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Skip("PS4 save not available")
	}
	save, err := core.LoadSave(path)
	if err != nil {
		t.Fatalf("LoadSave PS4: %v", err)
	}
	return save
}

func floatEq(a, b float32) bool {
	return math.Abs(float64(a-b)) < 0.01
}
