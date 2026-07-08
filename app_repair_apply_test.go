package main

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
)

// buildApplyInvFixture builds an inventory-only slot with a non-zero MagicOffset
// (so ReconcileInventoryHeader runs) holding the given common records, mirroring
// core's buildInvFixtureNZ.
func buildApplyInvFixture(t *testing.T, common []core.InventoryItem) *core.SaveSlot {
	t.Helper()
	const magicOff = 16
	commonStart := magicOff + core.InvStartFromMagic
	keyStart := commonStart + core.CommonItemCount*core.InvRecordLen + core.InvKeyCountHeader
	bufSize := keyStart + core.KeyItemCount*core.InvRecordLen + 8

	slot := &core.SaveSlot{
		Version:     1,
		MagicOffset: magicOff,
		Data:        make([]byte, bufSize),
		GaMap:       make(map[uint32]uint32),
	}
	for i, it := range common {
		off := commonStart + i*core.InvRecordLen
		binary.LittleEndian.PutUint32(slot.Data[off:], it.GaItemHandle)
		binary.LittleEndian.PutUint32(slot.Data[off+4:], it.Quantity)
		binary.LittleEndian.PutUint32(slot.Data[off+8:], it.Index)
	}
	slot.Inventory.CommonItems = append([]core.InventoryItem(nil), common...)
	binary.LittleEndian.PutUint32(slot.Data[commonStart-4:], uint32(len(common)))
	return slot
}

// quantityZeroTarget builds a remove_record target for the quantity_zero issue
// at the given row, resolving the current fingerprint so it is fresh.
func quantityZeroTarget(slot *core.SaveSlot, row int) RepairApplyTarget {
	fp, _ := core.FingerprintRecordAt(slot, "inventory_common", row)
	key := core.IssueKey{
		Slot: 0, Domain: "inventory", Code: core.RepairCodeQuantityZero,
		Scope: "inventory_common", Row: row, Handle: slot.Inventory.CommonItems[row].GaItemHandle,
	}
	return RepairApplyTarget{
		IssueID:        core.IssueKeyID(key),
		Key:            key,
		Fingerprint:    fp,
		SelectedAction: core.RepairActionRemoveRecord,
	}
}

// Two zero-quantity talismans at rows 0 and 1 → both flagged quantity_zero.
func twoZeroQtySlot(t *testing.T) *core.SaveSlot {
	const handle = 0xA00017B6
	return buildApplyInvFixture(t, []core.InventoryItem{
		{GaItemHandle: handle, Quantity: 0, Index: 500},
		{GaItemHandle: handle, Quantity: 0, Index: 600},
	})
}

func TestApplyRepairBatch_ContinuesAfterFailureWhenNotStopping(t *testing.T) {
	slot := twoZeroQtySlot(t)

	stale := quantityZeroTarget(slot, 0)
	stale.Fingerprint = "deadbeefdeadbeef" // force stale → failed
	good := quantityZeroTarget(slot, 1)    // valid → applied

	rep := applyRepairBatchToSlot(slot, 0, []RepairApplyTarget{stale, good}, false)

	if rep.Failed != 1 || rep.Applied != 1 {
		t.Fatalf("want applied=1 failed=1, got applied=%d failed=%d skipped=%d", rep.Applied, rep.Failed, rep.Skipped)
	}
	if rep.Stopped {
		t.Fatal("batch stopped despite stopOnFirstFailure=false")
	}
	if len(rep.Results) != 2 {
		t.Fatalf("want 2 results, got %d", len(rep.Results))
	}
	// Row 1 was actually removed; row 0 (stale) left intact.
	if slot.Inventory.CommonItems[1].GaItemHandle != 0 {
		t.Fatalf("row 1 not removed: %+v", slot.Inventory.CommonItems[1])
	}
	if slot.Inventory.CommonItems[0].GaItemHandle == 0 {
		t.Fatal("row 0 mutated despite stale fingerprint")
	}
}

func TestApplyRepairBatch_StopsOnFirstFailure(t *testing.T) {
	slot := twoZeroQtySlot(t)

	stale := quantityZeroTarget(slot, 0)
	stale.Fingerprint = "deadbeefdeadbeef"
	good := quantityZeroTarget(slot, 1)

	rep := applyRepairBatchToSlot(slot, 0, []RepairApplyTarget{stale, good}, true)

	if !rep.Stopped {
		t.Fatal("batch did not stop on first failure")
	}
	if rep.Failed != 1 || rep.Applied != 0 {
		t.Fatalf("want applied=0 failed=1, got applied=%d failed=%d", rep.Applied, rep.Failed)
	}
	if len(rep.Results) != 1 {
		t.Fatalf("stopOnFirstFailure should record only the first result, got %d", len(rep.Results))
	}
	// The second (good) target must NOT have run.
	if slot.Inventory.CommonItems[1].GaItemHandle == 0 {
		t.Fatal("second target ran despite stopOnFirstFailure after a failure")
	}
}

func TestApplyRepairAction_StaleFingerprintRejectsBeforeMutation(t *testing.T) {
	slot := twoZeroQtySlot(t)
	before := append([]byte(nil), slot.Data...)

	target := quantityZeroTarget(slot, 0)
	target.Fingerprint = "deadbeefdeadbeef"

	r := applyRepairActionToSlot(slot, 0, target)
	if r.Outcome != repairOutcomeFailed {
		t.Fatalf("want failed, got %q (%s)", r.Outcome, r.Message)
	}
	if !bytes.Equal(before, slot.Data) {
		t.Fatal("stale fingerprint mutated slot.Data")
	}
	if slot.Inventory.CommonItems[0].GaItemHandle == 0 {
		t.Fatal("stale fingerprint cleared the record")
	}
}

func TestApplyRepairAction_FailedActionLeavesNoPartialMutation(t *testing.T) {
	// repair_index on a quantity_zero issue: the primitive mutates the Index
	// (real write), but post-validation still sees quantity_zero → RestoreSlot.
	slot := buildApplyInvFixture(t, []core.InventoryItem{
		{GaItemHandle: 0xA00017B6, Quantity: 0, Index: 500},
	})
	before := append([]byte(nil), slot.Data...)
	beforeIdx := slot.Inventory.CommonItems[0].Index

	fp, _ := core.FingerprintRecordAt(slot, "inventory_common", 0)
	key := core.IssueKey{
		Slot: 0, Domain: "inventory", Code: core.RepairCodeQuantityZero,
		Scope: "inventory_common", Row: 0, Handle: 0xA00017B6,
	}
	target := RepairApplyTarget{
		IssueID: core.IssueKeyID(key), Key: key, Fingerprint: fp,
		SelectedAction: core.RepairActionRepairIndex, // mismatched → mutation won't clear the issue
	}

	r := applyRepairActionToSlot(slot, 0, target)
	if r.Outcome != repairOutcomeFailed {
		t.Fatalf("want failed (post-validation), got %q (%s)", r.Outcome, r.Message)
	}
	if !bytes.Equal(before, slot.Data) {
		t.Fatal("failed action left partial mutation in slot.Data")
	}
	if slot.Inventory.CommonItems[0].Index != beforeIdx {
		t.Fatalf("failed action left partial mutation in memory: index %d -> %d", beforeIdx, slot.Inventory.CommonItems[0].Index)
	}
}

func TestApplyRepairAction_LeaveUnchangedIsSkippedNotFailed(t *testing.T) {
	slot := twoZeroQtySlot(t)
	target := quantityZeroTarget(slot, 0)
	target.SelectedAction = RepairActionLeaveUnchanged

	r := applyRepairActionToSlot(slot, 0, target)
	if r.Outcome != repairOutcomeSkipped {
		t.Fatalf("want skipped, got %q (%s)", r.Outcome, r.Message)
	}
	if slot.Inventory.CommonItems[0].GaItemHandle == 0 {
		t.Fatal("leave_unchanged mutated the record")
	}
}

// Loaded and external endpoints share the RepairApplyReport model. Exercise the
// external endpoint end-to-end on an injected diagState save and confirm the
// same-shaped report the shared batch function produces for loaded.
func TestApplyRepairsExternal_SharesResultModel(t *testing.T) {
	slot := twoZeroQtySlot(t)

	save := &core.SaveFile{}
	save.Slots[0] = *slot
	diagState.mu.Lock()
	diagState.save = save
	diagState.mu.Unlock()
	defer func() {
		diagState.mu.Lock()
		diagState.save = nil
		diagState.mu.Unlock()
	}()

	target := quantityZeroTarget(&save.Slots[0], 1)

	rep, err := (&App{}).ApplyRepairsExternal([]RepairApplyTarget{target}, false)
	if err != nil {
		t.Fatalf("ApplyRepairsExternal: %v", err)
	}
	if rep.Applied != 1 || rep.Failed != 0 {
		t.Fatalf("want applied=1 failed=0, got %+v", rep)
	}
	if len(rep.Results) != 1 || rep.Results[0].Outcome != repairOutcomeApplied {
		t.Fatalf("unexpected results: %+v", rep.Results)
	}
	if rep.Results[0].SlotIndex != 0 {
		t.Fatalf("want slotIndex 0, got %d", rep.Results[0].SlotIndex)
	}
	if save.Slots[0].Inventory.CommonItems[1].GaItemHandle != 0 {
		t.Fatal("external apply did not mutate diagState.save slot")
	}
}
