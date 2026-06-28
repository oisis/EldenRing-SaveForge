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
	// Summon Host
	if !floatEq(vals.SummonTimeoutTime, 45.0) {
		t.Errorf("summonTimeoutTime = %f, want 45.0", vals.SummonTimeoutTime)
	}
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
	if !floatEq(p.BreakInRequestTimeOutSec, 8.0) {
		t.Errorf("breakInRequestTimeOutSec = %f, want 8", p.BreakInRequestTimeOutSec)
	}
	if p.BreakInRequestAreaCount != 8 {
		t.Errorf("breakInRequestAreaCount = %d, want 8", p.BreakInRequestAreaCount)
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

func TestNetworkParamFasterSummonHost(t *testing.T) {
	d := core.NetworkParamDefaults()
	p := core.NetworkParamFasterSummonHost()

	if !floatEq(p.SummonTimeoutTime, 10.0) || !floatEq(p.ReloadSignIntervalTime2, 20.0) ||
		p.ReloadSignTotalCount != 24 || p.SingGetMax != 40 || !floatEq(p.SignDownloadSpan, 15.0) {
		t.Errorf("Faster SummonHost values mismatch: %+v", p)
	}
	// Invariant total <= getMax.
	if p.ReloadSignTotalCount > p.SingGetMax {
		t.Errorf("Faster SummonHost violates total<=getMax: %d/%d", p.ReloadSignTotalCount, p.SingGetMax)
	}
	// Other groups untouched.
	if p.MaxBreakInTargetListCount != d.MaxBreakInTargetListCount {
		t.Errorf("Faster SummonHost modified invasion fields")
	}
	if p.UpdateSignIntervalTime != d.UpdateSignIntervalTime {
		t.Errorf("Faster SummonHost modified summonGuest fields")
	}
	if p.ReloadVisitListCoolTime != d.ReloadVisitListCoolTime {
		t.Errorf("Faster SummonHost modified hunter fields")
	}
	if err := core.ValidateNetworkParams(p); err != nil {
		t.Errorf("Faster SummonHost failed validation: %v", err)
	}
}

func TestNetworkParamFasterSummonGuest(t *testing.T) {
	d := core.NetworkParamDefaults()
	p := core.NetworkParamFasterSummonGuest()

	if !floatEq(p.UpdateSignIntervalTime, 15.0) || !floatEq(p.SignUpdateSpan, 20.0) {
		t.Errorf("Faster SummonGuest values mismatch: %+v", p)
	}
	// Other groups untouched.
	if p.SummonTimeoutTime != d.SummonTimeoutTime || p.ReloadSignIntervalTime2 != d.ReloadSignIntervalTime2 {
		t.Errorf("Faster SummonGuest modified summonHost fields")
	}
	if p.MaxBreakInTargetListCount != d.MaxBreakInTargetListCount {
		t.Errorf("Faster SummonGuest modified invasion fields")
	}
	if err := core.ValidateNetworkParams(p); err != nil {
		t.Errorf("Faster SummonGuest failed validation: %v", err)
	}
}

func TestNetworkParamFasterHunter(t *testing.T) {
	d := core.NetworkParamDefaults()
	p := core.NetworkParamFasterHunter()

	if !floatEq(p.ReloadVisitListCoolTime, 10.0) || p.MaxVisitListCount != 8 ||
		!floatEq(p.ReloadSearchCoopBlueMin, 12.0) || !floatEq(p.ReloadSearchCoopBlueMax, 45.0) {
		t.Errorf("Faster Hunter values mismatch: %+v", p)
	}
	// Experimental extras must stay vanilla.
	if p.MaxCoopBlueSummonCount != 2 {
		t.Errorf("maxCoopBlueSummonCount = %d, want 2 (must stay vanilla)", p.MaxCoopBlueSummonCount)
	}
	if p.AllAreaSearchRateVsBlue != 30 {
		t.Errorf("allAreaSearchRateVsBlue = %d, want 30 (must stay vanilla)", p.AllAreaSearchRateVsBlue)
	}
	// Other groups untouched.
	if p.MaxBreakInTargetListCount != d.MaxBreakInTargetListCount {
		t.Errorf("Faster Hunter modified invasion fields")
	}
	if p.SummonTimeoutTime != d.SummonTimeoutTime || p.ReloadSignIntervalTime2 != d.ReloadSignIntervalTime2 {
		t.Errorf("Faster Hunter modified summonHost fields")
	}
	if p.ReloadSearchCoopBlueMin > p.ReloadSearchCoopBlueMax {
		t.Errorf("Faster Hunter violates min<=max")
	}
	if err := core.ValidateNetworkParams(p); err != nil {
		t.Errorf("Faster Hunter failed validation: %v", err)
	}
}

func TestNetworkParamAggressiveReds(t *testing.T) {
	d := core.NetworkParamDefaults()
	p := core.NetworkParamAggressiveReds()

	if p.MaxBreakInTargetListCount != 12 {
		t.Errorf("maxBreakInTargetListCount = %d, want 12", p.MaxBreakInTargetListCount)
	}
	if p.BreakInRequestAreaCount != 12 {
		t.Errorf("breakInRequestAreaCount = %d, want 12", p.BreakInRequestAreaCount)
	}
	if !floatEq(p.BreakInRequestIntervalTimeSec, 10.0) {
		t.Errorf("breakInRequestIntervalTimeSec = %f, want 10", p.BreakInRequestIntervalTimeSec)
	}
	if !floatEq(p.BreakInRequestTimeOutSec, 7.0) {
		t.Errorf("breakInRequestTimeOutSec = %f, want 7", p.BreakInRequestTimeOutSec)
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

func TestNetworkParamAggressiveSummonHost(t *testing.T) {
	d := core.NetworkParamDefaults()
	p := core.NetworkParamAggressiveSummonHost()

	if !floatEq(p.SummonTimeoutTime, 7.0) || !floatEq(p.ReloadSignIntervalTime2, 12.0) ||
		p.ReloadSignTotalCount != 20 || p.SingGetMax != 48 || !floatEq(p.SignDownloadSpan, 10.0) {
		t.Errorf("Aggressive SummonHost values mismatch: %+v", p)
	}
	// Invariant total <= getMax.
	if p.ReloadSignTotalCount > p.SingGetMax {
		t.Errorf("Aggressive SummonHost violates total<=getMax: %d/%d", p.ReloadSignTotalCount, p.SingGetMax)
	}
	// Other groups untouched.
	if p.MaxBreakInTargetListCount != d.MaxBreakInTargetListCount {
		t.Errorf("Aggressive SummonHost modified invasion fields")
	}
	if p.UpdateSignIntervalTime != d.UpdateSignIntervalTime {
		t.Errorf("Aggressive SummonHost modified summonGuest fields")
	}
	if p.ReloadVisitListCoolTime != d.ReloadVisitListCoolTime {
		t.Errorf("Aggressive SummonHost modified hunter fields")
	}
	if err := core.ValidateNetworkParams(p); err != nil {
		t.Errorf("Aggressive SummonHost failed validation: %v", err)
	}
}

func TestNetworkParamAggressiveSummonGuest(t *testing.T) {
	d := core.NetworkParamDefaults()
	p := core.NetworkParamAggressiveSummonGuest()

	if !floatEq(p.UpdateSignIntervalTime, 10.0) || !floatEq(p.SignUpdateSpan, 12.0) {
		t.Errorf("Aggressive SummonGuest values mismatch: %+v", p)
	}
	// Other groups untouched.
	if p.SummonTimeoutTime != d.SummonTimeoutTime || p.ReloadSignIntervalTime2 != d.ReloadSignIntervalTime2 {
		t.Errorf("Aggressive SummonGuest modified summonHost fields")
	}
	if p.MaxBreakInTargetListCount != d.MaxBreakInTargetListCount {
		t.Errorf("Aggressive SummonGuest modified invasion fields")
	}
	if err := core.ValidateNetworkParams(p); err != nil {
		t.Errorf("Aggressive SummonGuest failed validation: %v", err)
	}
}

func TestNetworkParamAggressiveHunter(t *testing.T) {
	d := core.NetworkParamDefaults()
	p := core.NetworkParamAggressiveHunter()

	if !floatEq(p.ReloadVisitListCoolTime, 6.0) || p.MaxVisitListCount != 12 ||
		!floatEq(p.ReloadSearchCoopBlueMin, 8.0) || !floatEq(p.ReloadSearchCoopBlueMax, 25.0) {
		t.Errorf("Aggressive Hunter values mismatch: %+v", p)
	}
	// Experimental extras must stay vanilla.
	if p.MaxCoopBlueSummonCount != 2 {
		t.Errorf("maxCoopBlueSummonCount = %d, want 2 (Experimental, must stay vanilla)", p.MaxCoopBlueSummonCount)
	}
	if p.AllAreaSearchRateVsBlue != 30 {
		t.Errorf("allAreaSearchRateVsBlue = %d, want 30 (Experimental, must stay vanilla)", p.AllAreaSearchRateVsBlue)
	}
	// Other groups untouched.
	if p.MaxBreakInTargetListCount != d.MaxBreakInTargetListCount {
		t.Errorf("Aggressive Hunter modified invasion fields")
	}
	if p.SummonTimeoutTime != d.SummonTimeoutTime || p.ReloadSignIntervalTime2 != d.ReloadSignIntervalTime2 {
		t.Errorf("Aggressive Hunter modified summonHost fields")
	}
	// Invariants: min<=max.
	if p.ReloadSearchCoopBlueMin > p.ReloadSearchCoopBlueMax {
		t.Errorf("Aggressive Hunter violates min<=max")
	}
	if err := core.ValidateNetworkParams(p); err != nil {
		t.Errorf("Aggressive Hunter failed validation: %v", err)
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
