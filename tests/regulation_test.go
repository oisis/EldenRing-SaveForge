package tests

import (
	"bytes"
	"crypto/md5"
	"math"
	"os"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
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

	assertVanillaNetworkParams(t, vals)
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

	assertVanillaNetworkParams(t, vals)
}

func assertVanillaNetworkParams(t *testing.T, vals *core.NetworkParamValues) {
	t.Helper()
	// Invader
	if vals.MaxBreakInTargetListCount != 5 {
		t.Errorf("maxBreakInTargetListCount = %d, want 5", vals.MaxBreakInTargetListCount)
	}
	if !floatEq(vals.BreakInRequestIntervalTimeSec, 30.0) {
		t.Errorf("breakInRequestIntervalTimeSec = %f, want 30.0", vals.BreakInRequestIntervalTimeSec)
	}
	if !floatEq(vals.BreakInRequestTimeOutSec, 20.0) {
		t.Errorf("breakInRequestTimeOutSec = %f, want 20.0", vals.BreakInRequestTimeOutSec)
	}
	// Cooperator
	if !floatEq(vals.ReloadSignIntervalTime2, 60.0) {
		t.Errorf("reloadSignIntervalTime2 = %f, want 60.0", vals.ReloadSignIntervalTime2)
	}
	if vals.ReloadSignTotalCount != 20 {
		t.Errorf("reloadSignTotalCount = %d, want 20", vals.ReloadSignTotalCount)
	}
	if vals.ReloadSignCellCount != 10 {
		t.Errorf("reloadSignCellCount = %d, want 10", vals.ReloadSignCellCount)
	}
	if !floatEq(vals.UpdateSignIntervalTime, 30.0) {
		t.Errorf("updateSignIntervalTime = %f, want 30.0", vals.UpdateSignIntervalTime)
	}
	if vals.SingGetMax != 32 {
		t.Errorf("singGetMax = %d, want 32", vals.SingGetMax)
	}
	if !floatEq(vals.SignDownloadSpan, 30.0) {
		t.Errorf("signDownloadSpan = %f, want 30.0", vals.SignDownloadSpan)
	}
	if !floatEq(vals.SignUpdateSpan, 60.0) {
		t.Errorf("signUpdateSpan = %f, want 60.0", vals.SignUpdateSpan)
	}
	// Blue
	if !floatEq(vals.ReloadVisitListCoolTime, 20.0) {
		t.Errorf("reloadVisitListCoolTime = %f, want 20.0", vals.ReloadVisitListCoolTime)
	}
	if vals.MaxCoopBlueSummonCount != 2 {
		t.Errorf("maxCoopBlueSummonCount = %d, want 2", vals.MaxCoopBlueSummonCount)
	}
	if vals.MaxVisitListCount != 5 {
		t.Errorf("maxVisitListCount = %d, want 5", vals.MaxVisitListCount)
	}
	if !floatEq(vals.ReloadSearchCoopBlueMin, 30.0) {
		t.Errorf("reloadSearchCoopBlueMin = %f, want 30.0", vals.ReloadSearchCoopBlueMin)
	}
	if !floatEq(vals.ReloadSearchCoopBlueMax, 180.0) {
		t.Errorf("reloadSearchCoopBlueMax = %f, want 180.0", vals.ReloadSearchCoopBlueMax)
	}
	if vals.AllAreaSearchRateCoopBlue != 30 {
		t.Errorf("allAreaSearchRateCoopBlue = %d, want 30", vals.AllAreaSearchRateCoopBlue)
	}
	if vals.AllAreaSearchRateVsBlue != 30 {
		t.Errorf("allAreaSearchRateVsBlue = %d, want 30", vals.AllAreaSearchRateVsBlue)
	}
	// Host
	if vals.VisitorListMax != 10 {
		t.Errorf("visitorListMax = %d, want 10", vals.VisitorListMax)
	}
	if !floatEq(vals.VisitorTimeOutTime, 60.0) {
		t.Errorf("visitorTimeOutTime = %f, want 60.0", vals.VisitorTimeOutTime)
	}
	if !floatEq(vals.VisitorDownloadSpan, 60.0) {
		t.Errorf("visitorDownloadSpan = %f, want 60.0", vals.VisitorDownloadSpan)
	}
}

func TestPatchNetworkParams_PC_RoundTrip(t *testing.T) {
	save := loadPCSave(t)
	if len(save.UserData11) == 0 {
		t.Skip("PC save has no UserData11")
	}

	// Apply FastSummons preset (exercises cooperator fields)
	patch := core.NetworkParamFastSummons()
	patched, err := core.PatchNetworkParams(save.UserData11, patch)
	if err != nil {
		t.Fatalf("PatchNetworkParams: %v", err)
	}

	// PC save: ud11[0:0x10] must be MD5(ud11[0x10:]) after patching.
	if len(patched) > 0x10 {
		h := md5.Sum(patched[0x10:])
		if !bytes.Equal(patched[:0x10], h[:]) {
			t.Errorf("ud11 MD5 prefix mismatch after patch: prefix=%X want=%X", patched[:0x10], h[:])
		}
	}

	vals, err := core.ReadNetworkParams(patched)
	if err != nil {
		t.Fatalf("ReadNetworkParams after patch: %v", err)
	}

	if !floatEq(vals.ReloadSignIntervalTime2, 15.0) {
		t.Errorf("reloadSignIntervalTime2 = %f, want 15.0", vals.ReloadSignIntervalTime2)
	}
	if vals.ReloadSignTotalCount != 40 {
		t.Errorf("reloadSignTotalCount = %d, want 40", vals.ReloadSignTotalCount)
	}
	if vals.SingGetMax != 64 {
		t.Errorf("singGetMax = %d, want 64", vals.SingGetMax)
	}
	if !floatEq(vals.SignDownloadSpan, 10.0) {
		t.Errorf("signDownloadSpan = %f, want 10.0", vals.SignDownloadSpan)
	}

	// Apply FastBlue preset
	patchBlue := core.NetworkParamFastBlue()
	patched2, err := core.PatchNetworkParams(save.UserData11, patchBlue)
	if err != nil {
		t.Fatalf("PatchNetworkParams (blue): %v", err)
	}
	valsBlue, err := core.ReadNetworkParams(patched2)
	if err != nil {
		t.Fatalf("ReadNetworkParams after blue patch: %v", err)
	}
	if !floatEq(valsBlue.ReloadVisitListCoolTime, 5.0) {
		t.Errorf("reloadVisitListCoolTime = %f, want 5.0", valsBlue.ReloadVisitListCoolTime)
	}
	if valsBlue.MaxCoopBlueSummonCount != 4 {
		t.Errorf("maxCoopBlueSummonCount = %d, want 4", valsBlue.MaxCoopBlueSummonCount)
	}
	if valsBlue.AllAreaSearchRateCoopBlue != 75 {
		t.Errorf("allAreaSearchRateCoopBlue = %d, want 75", valsBlue.AllAreaSearchRateCoopBlue)
	}

	// Reset to defaults and verify
	reset, err := core.PatchNetworkParams(patched, core.NetworkParamDefaults())
	if err != nil {
		t.Fatalf("PatchNetworkParams (reset): %v", err)
	}
	valsReset, err := core.ReadNetworkParams(reset)
	if err != nil {
		t.Fatalf("ReadNetworkParams after reset: %v", err)
	}
	assertVanillaNetworkParams(t, valsReset)
}

func TestPatchNetworkParams_PS4_RoundTrip(t *testing.T) {
	save := loadPS4Save(t)
	if len(save.UserData11) == 0 {
		t.Skip("PS4 save has no UserData11")
	}

	// Apply AggressiveHost preset (exercises host/visitor fields)
	patch := core.NetworkParamAggressiveHost()
	patched, err := core.PatchNetworkParams(save.UserData11, patch)
	if err != nil {
		t.Fatalf("PatchNetworkParams: %v", err)
	}

	vals, err := core.ReadNetworkParams(patched)
	if err != nil {
		t.Fatalf("ReadNetworkParams after patch: %v", err)
	}

	if vals.VisitorListMax != 20 {
		t.Errorf("visitorListMax = %d, want 20", vals.VisitorListMax)
	}
	if !floatEq(vals.VisitorTimeOutTime, 60.0) {
		t.Errorf("visitorTimeOutTime = %f, want 60.0", vals.VisitorTimeOutTime)
	}
	if !floatEq(vals.VisitorDownloadSpan, 10.0) {
		t.Errorf("visitorDownloadSpan = %f, want 10.0", vals.VisitorDownloadSpan)
	}

	// FastInvasions backward compat
	patchInv := core.NetworkParamFast()
	patched2, err := core.PatchNetworkParams(save.UserData11, patchInv)
	if err != nil {
		t.Fatalf("PatchNetworkParams (fast): %v", err)
	}
	valsInv, err := core.ReadNetworkParams(patched2)
	if err != nil {
		t.Fatalf("ReadNetworkParams after fast patch: %v", err)
	}
	if valsInv.MaxBreakInTargetListCount != 10 {
		t.Errorf("maxBreakInTargetListCount = %d, want 10", valsInv.MaxBreakInTargetListCount)
	}
	if !floatEq(valsInv.BreakInRequestIntervalTimeSec, 4.0) {
		t.Errorf("breakInRequestIntervalTimeSec = %f, want 4.0", valsInv.BreakInRequestIntervalTimeSec)
	}
}

func TestPatchNetworkParams_Validation(t *testing.T) {
	d := core.NetworkParamDefaults()

	mutate := func(f func(*core.NetworkParamValues)) core.NetworkParamValues {
		cp := d
		f(&cp)
		return cp
	}

	tests := []struct {
		name  string
		patch core.NetworkParamValues
	}{
		{"targets too low", mutate(func(p *core.NetworkParamValues) { p.MaxBreakInTargetListCount = 0 })},
		{"targets too high", mutate(func(p *core.NetworkParamValues) { p.MaxBreakInTargetListCount = 21 })},
		{"interval too low", mutate(func(p *core.NetworkParamValues) { p.BreakInRequestIntervalTimeSec = 1.0 })},
		{"interval too high", mutate(func(p *core.NetworkParamValues) { p.BreakInRequestIntervalTimeSec = 31.0 })},
		{"timeout too low", mutate(func(p *core.NetworkParamValues) { p.BreakInRequestTimeOutSec = 2.0 })},
		{"timeout too high", mutate(func(p *core.NetworkParamValues) { p.BreakInRequestTimeOutSec = 21.0 })},
		{"sign interval too low", mutate(func(p *core.NetworkParamValues) { p.ReloadSignIntervalTime2 = 0.0 })},
		{"sign count too high", mutate(func(p *core.NetworkParamValues) { p.ReloadSignTotalCount = 200 })},
		{"blue search rate too high", mutate(func(p *core.NetworkParamValues) { p.AllAreaSearchRateCoopBlue = 101 })},
		{"visitor list too high", mutate(func(p *core.NetworkParamValues) { p.VisitorListMax = 101 })},
		{"blue summon too high", mutate(func(p *core.NetworkParamValues) { p.MaxCoopBlueSummonCount = 11 })},
		{"visitor timeout too high", mutate(func(p *core.NetworkParamValues) { p.VisitorTimeOutTime = 601.0 })},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := core.ValidateNetworkParams(tc.patch)
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
