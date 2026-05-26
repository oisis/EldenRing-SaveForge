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

// --- functional preset tests (v0.9 unified system) ---

func TestNetworkParamDefaults_VanillaValues(t *testing.T) {
	d := core.NetworkParamDefaults()
	if d.ReloadSignTotalCount != 20 {
		t.Errorf("reloadSignTotalCount = %d, want 20 (binary is source of truth, not the old 32)", d.ReloadSignTotalCount)
	}
	if d.ReloadSignCellCount != 10 {
		t.Errorf("reloadSignCellCount = %d, want 10 (binary is source of truth, not the old 8)", d.ReloadSignCellCount)
	}
	// Unknown 0x7C field — confirmed vanilla value, unconfirmed semantics. Must stay 5.
	if d.BreakInRequestAreaCount != 5 {
		t.Errorf("breakInRequestAreaCount (0x7C) = %d, want 5", d.BreakInRequestAreaCount)
	}
}

func TestNetworkParamFasterReds(t *testing.T) {
	d := core.NetworkParamDefaults()
	p := core.NetworkParamFasterReds()

	if p.MaxBreakInTargetListCount != 8 {
		t.Errorf("maxBreakInTargetListCount = %d, want 8", p.MaxBreakInTargetListCount)
	}
	if !floatEq(p.BreakInRequestIntervalTimeSec, 12.0) {
		t.Errorf("breakInRequestIntervalTimeSec = %f, want 12", p.BreakInRequestIntervalTimeSec)
	}
	if !floatEq(p.BreakInRequestTimeOutSec, 15.0) {
		t.Errorf("breakInRequestTimeOutSec = %f, want 15", p.BreakInRequestTimeOutSec)
	}
	// 0x7C must stay vanilla 5.
	if p.BreakInRequestAreaCount != 5 {
		t.Errorf("breakInRequestAreaCount = %d, want 5 (must never be raised by a preset)", p.BreakInRequestAreaCount)
	}
	// Other groups untouched.
	if p.ReloadSignIntervalTime2 != d.ReloadSignIntervalTime2 || p.SingGetMax != d.SingGetMax {
		t.Errorf("Faster Reds modified cooperator fields")
	}
	if p.ReloadVisitListCoolTime != d.ReloadVisitListCoolTime || p.AllAreaSearchRateCoopBlue != d.AllAreaSearchRateCoopBlue {
		t.Errorf("Faster Reds modified blue fields")
	}
	if err := core.ValidateNetworkParams(p); err != nil {
		t.Errorf("Faster Reds failed validation: %v", err)
	}
}

func TestNetworkParamFasterSummons(t *testing.T) {
	d := core.NetworkParamDefaults()
	p := core.NetworkParamFasterSummons()

	if !floatEq(p.ReloadSignIntervalTime2, 20.0) || p.ReloadSignTotalCount != 40 ||
		p.ReloadSignCellCount != 20 || !floatEq(p.UpdateSignIntervalTime, 15.0) ||
		p.SingGetMax != 64 || !floatEq(p.SignDownloadSpan, 15.0) || !floatEq(p.SignUpdateSpan, 20.0) {
		t.Errorf("Faster Summons values mismatch: %+v", p)
	}
	// Invariant cell <= total <= getMax.
	if !(p.ReloadSignCellCount <= p.ReloadSignTotalCount && p.ReloadSignTotalCount <= p.SingGetMax) {
		t.Errorf("Faster Summons violates cell<=total<=getMax: %d/%d/%d",
			p.ReloadSignCellCount, p.ReloadSignTotalCount, p.SingGetMax)
	}
	// Invasion + blue groups untouched.
	if p.MaxBreakInTargetListCount != d.MaxBreakInTargetListCount || p.BreakInRequestAreaCount != d.BreakInRequestAreaCount {
		t.Errorf("Faster Summons modified invasion fields")
	}
	if p.ReloadVisitListCoolTime != d.ReloadVisitListCoolTime {
		t.Errorf("Faster Summons modified blue fields")
	}
	if err := core.ValidateNetworkParams(p); err != nil {
		t.Errorf("Faster Summons failed validation: %v", err)
	}
}

func TestNetworkParamFasterBlue(t *testing.T) {
	d := core.NetworkParamDefaults()
	p := core.NetworkParamFasterBlue()

	if !floatEq(p.ReloadVisitListCoolTime, 8.0) || !floatEq(p.ReloadSearchCoopBlueMin, 10.0) ||
		!floatEq(p.ReloadSearchCoopBlueMax, 40.0) || p.MaxVisitListCount != 10 ||
		p.AllAreaSearchRateCoopBlue != 60 {
		t.Errorf("Faster Blue values mismatch: %+v", p)
	}
	// Must NOT inflate active-blue cap or retribution rate.
	if p.MaxCoopBlueSummonCount != 2 {
		t.Errorf("maxCoopBlueSummonCount = %d, want 2 (must stay vanilla)", p.MaxCoopBlueSummonCount)
	}
	if p.AllAreaSearchRateVsBlue != 30 {
		t.Errorf("allAreaSearchRateVsBlue = %d, want 30 (must stay vanilla)", p.AllAreaSearchRateVsBlue)
	}
	// Invasion + signs groups untouched.
	if p.MaxBreakInTargetListCount != d.MaxBreakInTargetListCount {
		t.Errorf("Faster Blue modified invasion fields")
	}
	if p.ReloadSignIntervalTime2 != d.ReloadSignIntervalTime2 || p.SingGetMax != d.SingGetMax {
		t.Errorf("Faster Blue modified signs fields")
	}
	if p.ReloadSearchCoopBlueMin > p.ReloadSearchCoopBlueMax {
		t.Errorf("Faster Blue violates min<=max")
	}
	if err := core.ValidateNetworkParams(p); err != nil {
		t.Errorf("Faster Blue failed validation: %v", err)
	}
}

func TestNetworkParamAggressiveReds(t *testing.T) {
	d := core.NetworkParamDefaults()
	p := core.NetworkParamAggressiveReds()

	if p.MaxBreakInTargetListCount != 12 {
		t.Errorf("maxBreakInTargetListCount = %d, want 12", p.MaxBreakInTargetListCount)
	}
	if !floatEq(p.BreakInRequestIntervalTimeSec, 8.0) {
		t.Errorf("breakInRequestIntervalTimeSec = %f, want 8", p.BreakInRequestIntervalTimeSec)
	}
	if !floatEq(p.BreakInRequestTimeOutSec, 12.0) {
		t.Errorf("breakInRequestTimeOutSec = %f, want 12", p.BreakInRequestTimeOutSec)
	}
	// 0x7C must stay vanilla 5 in the returned profile.
	if p.BreakInRequestAreaCount != 5 {
		t.Errorf("breakInRequestAreaCount = %d, want 5 (Aggressive must never raise 0x7C)", p.BreakInRequestAreaCount)
	}
	// Other groups untouched.
	if p.ReloadSignIntervalTime2 != d.ReloadSignIntervalTime2 || p.SingGetMax != d.SingGetMax {
		t.Errorf("Aggressive Reds modified cooperator fields")
	}
	if p.ReloadVisitListCoolTime != d.ReloadVisitListCoolTime || p.AllAreaSearchRateCoopBlue != d.AllAreaSearchRateCoopBlue {
		t.Errorf("Aggressive Reds modified blue fields")
	}
	if p.VisitorListMax != d.VisitorListMax {
		t.Errorf("Aggressive Reds modified visitor fields")
	}
	if err := core.ValidateNetworkParams(p); err != nil {
		t.Errorf("Aggressive Reds failed validation: %v", err)
	}
}

func TestNetworkParamAggressiveSummons(t *testing.T) {
	d := core.NetworkParamDefaults()
	p := core.NetworkParamAggressiveSummons()

	if !floatEq(p.ReloadSignIntervalTime2, 10.0) || p.ReloadSignTotalCount != 64 ||
		p.ReloadSignCellCount != 32 || !floatEq(p.UpdateSignIntervalTime, 10.0) ||
		p.SingGetMax != 96 || !floatEq(p.SignDownloadSpan, 10.0) || !floatEq(p.SignUpdateSpan, 10.0) {
		t.Errorf("Aggressive Summons values mismatch: %+v", p)
	}
	// Invariant cell <= total <= getMax (32 <= 64 <= 96).
	if !(p.ReloadSignCellCount <= p.ReloadSignTotalCount && p.ReloadSignTotalCount <= p.SingGetMax) {
		t.Errorf("Aggressive Summons violates cell<=total<=getMax: %d/%d/%d",
			p.ReloadSignCellCount, p.ReloadSignTotalCount, p.SingGetMax)
	}
	// Invasion + blue groups untouched.
	if p.MaxBreakInTargetListCount != d.MaxBreakInTargetListCount || p.BreakInRequestAreaCount != d.BreakInRequestAreaCount {
		t.Errorf("Aggressive Summons modified invasion fields")
	}
	if p.ReloadVisitListCoolTime != d.ReloadVisitListCoolTime {
		t.Errorf("Aggressive Summons modified blue fields")
	}
	if err := core.ValidateNetworkParams(p); err != nil {
		t.Errorf("Aggressive Summons failed validation: %v", err)
	}
}

func TestNetworkParamAggressiveBlue(t *testing.T) {
	d := core.NetworkParamDefaults()
	p := core.NetworkParamAggressiveBlue()

	if !floatEq(p.ReloadVisitListCoolTime, 5.0) || !floatEq(p.ReloadSearchCoopBlueMin, 5.0) ||
		!floatEq(p.ReloadSearchCoopBlueMax, 20.0) || p.MaxVisitListCount != 15 ||
		p.AllAreaSearchRateCoopBlue != 100 {
		t.Errorf("Aggressive Blue values mismatch: %+v", p)
	}
	// Must NOT inflate active-blue cap (Blue Search Parallelism) or retribution rate.
	if p.MaxCoopBlueSummonCount != 2 {
		t.Errorf("maxCoopBlueSummonCount = %d, want 2 (Experimental, must stay vanilla)", p.MaxCoopBlueSummonCount)
	}
	if p.AllAreaSearchRateVsBlue != 30 {
		t.Errorf("allAreaSearchRateVsBlue = %d, want 30 (Experimental, must stay vanilla)", p.AllAreaSearchRateVsBlue)
	}
	// Invasion + signs groups untouched.
	if p.MaxBreakInTargetListCount != d.MaxBreakInTargetListCount {
		t.Errorf("Aggressive Blue modified invasion fields")
	}
	if p.ReloadSignIntervalTime2 != d.ReloadSignIntervalTime2 || p.SingGetMax != d.SingGetMax {
		t.Errorf("Aggressive Blue modified signs fields")
	}
	// Invariants: min<=max, rate in 0-100.
	if p.ReloadSearchCoopBlueMin > p.ReloadSearchCoopBlueMax {
		t.Errorf("Aggressive Blue violates min<=max")
	}
	if p.AllAreaSearchRateCoopBlue < 0 || p.AllAreaSearchRateCoopBlue > 100 {
		t.Errorf("allAreaSearchRateCoopBlue out of 0-100: %d", p.AllAreaSearchRateCoopBlue)
	}
	if err := core.ValidateNetworkParams(p); err != nil {
		t.Errorf("Aggressive Blue failed validation: %v", err)
	}
}

func TestNetworkParamInvariants(t *testing.T) {
	mutate := func(f func(*core.NetworkParamValues)) core.NetworkParamValues {
		cp := core.NetworkParamDefaults()
		f(&cp)
		return cp
	}
	tests := []struct {
		name  string
		patch core.NetworkParamValues
	}{
		{"cellCount > totalCount", mutate(func(p *core.NetworkParamValues) { p.ReloadSignCellCount = 25; p.ReloadSignTotalCount = 20 })},
		{"totalCount > singGetMax", mutate(func(p *core.NetworkParamValues) { p.ReloadSignTotalCount = 40; p.SingGetMax = 32 })},
		{"blue min > max", mutate(func(p *core.NetworkParamValues) { p.ReloadSearchCoopBlueMin = 100; p.ReloadSearchCoopBlueMax = 40 })},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err := core.ValidateNetworkParams(tc.patch); err == nil {
				t.Errorf("expected invariant validation error for %s", tc.name)
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
