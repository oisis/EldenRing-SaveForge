package application

import (
	"bytes"
	"encoding/binary"
	"math"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
)

// torrentRideOffsetApply mirrors the documented WorldHead start contract.
func torrentRideOffsetApply(slot *core.SaveSlot) int {
	return slot.UnlockedRegionsOffset + 4 + 4*len(slot.UnlockedRegions)
}

// buildTorrentApplyFixture builds a synthetic full-size slot whose RideGameData
// carries the given HP / ride state. No real save is read.
func buildTorrentApplyFixture(hp int32, state uint32) *core.SaveSlot {
	slot := &core.SaveSlot{
		Version:               1,
		Data:                  make([]byte, core.SlotSize),
		GaMap:                 make(map[uint32]uint32),
		MagicOffset:           1000,
		UnlockedRegionsOffset: 0x1000,
		UnlockedRegions:       []uint32{10, 20, 30},
	}
	start := torrentRideOffsetApply(slot)
	binary.LittleEndian.PutUint32(slot.Data[start+0:], math.Float32bits(101.5))
	binary.LittleEndian.PutUint32(slot.Data[start+4:], math.Float32bits(-202.25))
	binary.LittleEndian.PutUint32(slot.Data[start+8:], math.Float32bits(303.75))
	copy(slot.Data[start+12:], []byte{0x3C, 0x00, 0x00, 0x3C})
	for i := 0; i < 4; i++ {
		binary.LittleEndian.PutUint32(slot.Data[start+16+4*i:], math.Float32bits(float32(i)+0.5))
	}
	binary.LittleEndian.PutUint32(slot.Data[start+32:], uint32(hp))
	binary.LittleEndian.PutUint32(slot.Data[start+36:], state)
	return slot
}

// torrentApplyTarget builds the apply target from the scanner's own issue, so
// the test can never address the repair with a hand-made key the scanner would
// not emit.
func torrentApplyTarget(t *testing.T, slot *core.SaveSlot, slotIndex int) RepairApplyTarget {
	t.Helper()
	for _, iss := range core.ScanRepairIssues(slotIndex, slot) {
		if iss.Key.Code == core.RepairCodeTorrentDeadActive {
			return RepairApplyTarget{
				IssueID:        iss.IssueID,
				Key:            iss.Key,
				Fingerprint:    iss.Fingerprint,
				SelectedAction: iss.DefaultAction,
			}
		}
	}
	t.Fatalf("scanner emitted no %s issue", core.RepairCodeTorrentDeadActive)
	return RepairApplyTarget{}
}

func TestApplyRepairAction_FixTorrentState(t *testing.T) {
	slot := buildTorrentApplyFixture(0, core.HorseStateActive)
	before := append([]byte(nil), slot.Data...)
	stateOff := torrentRideOffsetApply(slot) + 36

	target := torrentApplyTarget(t, slot, 3)
	if target.SelectedAction != core.RepairActionFixTorrentState {
		t.Fatalf("default action = %q, want %q", target.SelectedAction, core.RepairActionFixTorrentState)
	}

	res := applyRepairActionToSlot(slot, 3, target)

	if res.Outcome != repairOutcomeApplied {
		t.Fatalf("outcome = %q (%s), want applied", res.Outcome, res.Message)
	}
	horse, _, ok := core.ReadTorrentGameData(slot)
	if !ok || horse.State != core.HorseStateDead || horse.HP != 0 {
		t.Fatalf("RideGameData after repair: ok=%v %+v", ok, horse)
	}
	if !bytes.Equal(before[:stateOff], slot.Data[:stateOff]) ||
		!bytes.Equal(before[stateOff+4:], slot.Data[stateOff+4:]) {
		t.Fatal("repair changed bytes outside RideGameData.State")
	}
	for _, iss := range core.ScanRepairIssues(3, slot) {
		if iss.Key.Code == core.RepairCodeTorrentDeadActive {
			t.Fatal("re-scan still reports the torrent issue")
		}
	}
}

// A rejected repair (condition already gone by the time the action is applied)
// fails and leaves the slot byte-identical — the snapshot/rollback guard in
// applyRepairActionToSlot must never publish a partial mutation.
func TestApplyRepairAction_FixTorrentStateRejectedLeavesSlotIntact(t *testing.T) {
	scanned := buildTorrentApplyFixture(0, core.HorseStateActive)
	target := torrentApplyTarget(t, scanned, 0)

	// The real slot is no longer in the freeze condition.
	slot := buildTorrentApplyFixture(0, core.HorseStateDead)
	before := append([]byte(nil), slot.Data...)

	res := applyRepairActionToSlot(slot, 0, target)

	if res.Outcome != repairOutcomeFailed {
		t.Fatalf("outcome = %q, want failed", res.Outcome)
	}
	if !bytes.Equal(before, slot.Data) {
		t.Fatal("slot bytes changed despite the failed repair")
	}
}

// The DTO layer must offer the repair as the default user-facing action.
func TestRepairActionsForCode_TorrentDeadActive(t *testing.T) {
	actions, def := repairActionsForCode(core.RepairCodeTorrentDeadActive)

	if def != core.RepairActionFixTorrentState {
		t.Fatalf("default action = %q, want %q", def, core.RepairActionFixTorrentState)
	}
	if len(actions) != 2 || actions[0].ID != core.RepairActionFixTorrentState ||
		actions[1].ID != RepairActionLeaveUnchanged {
		t.Fatalf("actions = %+v", actions)
	}
	if actions[0].Label == actions[0].ID {
		t.Error("fix_torrent_state has no human-readable label")
	}
}
