package main

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
	"github.com/oisis/EldenRing-SaveForge/backend/editor"
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

// smithingStoneHandleQty resolves KnownDB to Smithing Stone [1] (MaxInventory
// 999) — a clampable over-cap record.
const smithingStoneHandleQty = uint32(0xB0002774)

// clampTarget builds a clamp_quantity target for the record at row, resolving
// the current fingerprint so it is fresh.
func clampTarget(slot *core.SaveSlot, row int) RepairApplyTarget {
	fp, _ := core.FingerprintRecordAt(slot, "inventory_common", row)
	key := core.IssueKey{
		Slot: 0, Domain: "inventory", Code: core.RepairCodeQuantityAboveMax,
		Scope: "inventory_common", Row: row, Handle: slot.Inventory.CommonItems[row].GaItemHandle,
		Field: "quantity", Value: "1500",
	}
	return RepairApplyTarget{
		IssueID:        core.IssueKeyID(key),
		Key:            key,
		Fingerprint:    fp,
		SelectedAction: core.RepairActionClampQuantity,
	}
}

func TestApplyRepairAction_ClampQuantity_Applies(t *testing.T) {
	slot := buildApplyInvFixture(t, []core.InventoryItem{
		{GaItemHandle: smithingStoneHandleQty, Quantity: 1500, Index: 500},
	})

	r := applyRepairActionToSlot(slot, 0, clampTarget(slot, 0))
	if r.Outcome != repairOutcomeApplied {
		t.Fatalf("want applied, got %q (%s)", r.Outcome, r.Message)
	}
	if slot.Inventory.CommonItems[0].Quantity != 999 {
		t.Errorf("in-memory quantity = %d, want 999", slot.Inventory.CommonItems[0].Quantity)
	}
	rawOff := slot.MagicOffset + core.InvStartFromMagic + 4
	if got := binary.LittleEndian.Uint32(slot.Data[rawOff:]); got != 999 {
		t.Errorf("raw quantity = %d, want 999", got)
	}
}

func TestApplyRepairAction_ClampQuantity_StaleNoMutation(t *testing.T) {
	slot := buildApplyInvFixture(t, []core.InventoryItem{
		{GaItemHandle: smithingStoneHandleQty, Quantity: 1500, Index: 500},
	})
	before := append([]byte(nil), slot.Data...)

	target := clampTarget(slot, 0)
	target.Fingerprint = "deadbeefdeadbeef"
	r := applyRepairActionToSlot(slot, 0, target)
	if r.Outcome != repairOutcomeFailed {
		t.Fatalf("want failed, got %q", r.Outcome)
	}
	if !bytes.Equal(before, slot.Data) || slot.Inventory.CommonItems[0].Quantity != 1500 {
		t.Fatal("stale clamp mutated the slot")
	}
}

func TestApplyRepairAction_ClampQuantity_BoundaryNoOpFails(t *testing.T) {
	// Already at cap → the primitive rejects (nothing to clamp) → failed, no undo.
	slot := buildApplyInvFixture(t, []core.InventoryItem{
		{GaItemHandle: smithingStoneHandleQty, Quantity: 999, Index: 500},
	})
	before := append([]byte(nil), slot.Data...)
	fp, _ := core.FingerprintRecordAt(slot, "inventory_common", 0)
	key := core.IssueKey{
		Slot: 0, Domain: "inventory", Code: core.RepairCodeQuantityAboveMax,
		Scope: "inventory_common", Row: 0, Handle: smithingStoneHandleQty, Field: "quantity", Value: "999",
	}
	target := RepairApplyTarget{IssueID: core.IssueKeyID(key), Key: key, Fingerprint: fp, SelectedAction: core.RepairActionClampQuantity}

	r := applyRepairActionToSlot(slot, 0, target)
	if r.Outcome != repairOutcomeFailed {
		t.Fatalf("boundary clamp: want failed, got %q", r.Outcome)
	}
	if !bytes.Equal(before, slot.Data) {
		t.Fatal("boundary clamp mutated the slot")
	}
}

func TestClampLeavesQuantityInvalid(t *testing.T) {
	lingering := []core.RepairIssue{{Key: core.IssueKey{
		Domain: "inventory", Code: core.RepairCodeQuantityAboveMax, Scope: "inventory_common", Row: 2}}}
	if !clampLeavesQuantityInvalid(lingering, "inventory_common", 2) {
		t.Error("must flag a lingering quantity_above_max at the clamped row")
	}
	zero := []core.RepairIssue{{Key: core.IssueKey{
		Domain: "inventory", Code: core.RepairCodeQuantityZero, Scope: "inventory_common", Row: 2}}}
	if !clampLeavesQuantityInvalid(zero, "inventory_common", 2) {
		t.Error("must flag a quantity_zero created at the clamped row")
	}
	// A different row / different code is not a clamp failure.
	if clampLeavesQuantityInvalid(lingering, "inventory_common", 3) {
		t.Error("must not flag an issue at a different row")
	}
	other := []core.RepairIssue{{Key: core.IssueKey{
		Domain: "inventory", Code: core.RepairCodeDuplicateHandle, Scope: "inventory_common", Row: 2}}}
	if clampLeavesQuantityInvalid(other, "inventory_common", 2) {
		t.Error("must not flag an unrelated code at the clamped row")
	}
}

func TestApplyRepairsLoaded_ClampPushesUndoOnce(t *testing.T) {
	app := NewApp()
	app.save = &core.SaveFile{}
	app.save.Slots[0] = *buildApplyInvFixture(t, []core.InventoryItem{
		{GaItemHandle: smithingStoneHandleQty, Quantity: 1500, Index: 500},
	})
	slot := &app.save.Slots[0]

	rep, err := app.ApplyRepairsLoaded(0, []RepairApplyTarget{clampTarget(slot, 0)}, false)
	if err != nil {
		t.Fatalf("ApplyRepairsLoaded: %v", err)
	}
	if rep.Applied != 1 {
		t.Fatalf("applied = %d, want 1", rep.Applied)
	}
	if got := len(app.undoStacks[0]); got != 1 {
		t.Errorf("undo depth = %d, want 1", got)
	}
	if slot.Inventory.CommonItems[0].Quantity != 999 {
		t.Errorf("quantity = %d, want clamped 999", slot.Inventory.CommonItems[0].Quantity)
	}
}

func TestApplyRepairsLoaded_BoundaryNoOpClampNoUndo(t *testing.T) {
	app := NewApp()
	app.save = &core.SaveFile{}
	app.save.Slots[0] = *buildApplyInvFixture(t, []core.InventoryItem{
		{GaItemHandle: smithingStoneHandleQty, Quantity: 999, Index: 500},
	})
	slot := &app.save.Slots[0]
	fp, _ := core.FingerprintRecordAt(slot, "inventory_common", 0)
	key := core.IssueKey{
		Slot: 0, Domain: "inventory", Code: core.RepairCodeQuantityAboveMax,
		Scope: "inventory_common", Row: 0, Handle: smithingStoneHandleQty, Field: "quantity", Value: "999",
	}
	target := RepairApplyTarget{IssueID: core.IssueKeyID(key), Key: key, Fingerprint: fp, SelectedAction: core.RepairActionClampQuantity}

	rep, err := app.ApplyRepairsLoaded(0, []RepairApplyTarget{target}, false)
	if err != nil {
		t.Fatalf("ApplyRepairsLoaded: %v", err)
	}
	if rep.Applied != 0 || rep.Failed != 1 {
		t.Fatalf("want applied=0 failed=1, got applied=%d failed=%d", rep.Applied, rep.Failed)
	}
	if got := len(app.undoStacks[0]); got != 0 {
		t.Errorf("undo pushed on a no-op clamp batch: depth=%d", got)
	}
}

// TestApplyRepairsLoaded_ClampInvalidatesActiveSession confirms that a
// successful clamp evicts the active inventory edit session for the character
// through the real ApplyRepairsLoaded path (not invalidateSessionForChar in
// isolation), so a stale workspace can never be committed over the repaired slot.
func TestApplyRepairsLoaded_ClampInvalidatesActiveSession(t *testing.T) {
	app := NewApp()
	app.save = &core.SaveFile{}
	app.save.Slots[0] = *buildApplyInvFixture(t, []core.InventoryItem{
		{GaItemHandle: smithingStoneHandleQty, Quantity: 1500, Index: 500},
	})
	slot := &app.save.Slots[0]

	if _, err := app.StartInventoryEditSession(0); err != nil {
		t.Fatalf("StartInventoryEditSession: %v", err)
	}
	sessID := app.editSessionByChar[0]
	if sessID == "" {
		t.Fatal("no active session registered after Start")
	}

	rep, err := app.ApplyRepairsLoaded(0, []RepairApplyTarget{clampTarget(slot, 0)}, false)
	if err != nil {
		t.Fatalf("ApplyRepairsLoaded: %v", err)
	}
	if rep.Applied != 1 {
		t.Fatalf("applied = %d, want 1", rep.Applied)
	}
	if slot.Inventory.CommonItems[0].Quantity != 999 {
		t.Errorf("quantity = %d, want clamped 999", slot.Inventory.CommonItems[0].Quantity)
	}
	if got := len(app.undoStacks[0]); got != 1 {
		t.Errorf("undo depth = %d, want 1", got)
	}
	if id, ok := app.editSessionByChar[0]; ok {
		t.Errorf("editSessionByChar[0] still present (%q); want evicted", id)
	}
	if _, ok := app.editSessions[sessID]; ok {
		t.Errorf("session %q still in editSessions; want removed", sessID)
	}
	// The old session ID is now unusable — the registry no longer resolves it.
	if _, err := app.GetInventoryEditSession(sessID); err == nil {
		t.Error("old session still resolvable after clamp invalidation")
	}
}

// TestApplyRepairsLoaded_NoOpClampRetainsActiveSession confirms that a failed
// (boundary no-op) clamp leaves the active session intact and usable — the slot
// was not mutated, so there is nothing to invalidate.
func TestApplyRepairsLoaded_NoOpClampRetainsActiveSession(t *testing.T) {
	app := NewApp()
	app.save = &core.SaveFile{}
	app.save.Slots[0] = *buildApplyInvFixture(t, []core.InventoryItem{
		{GaItemHandle: smithingStoneHandleQty, Quantity: 999, Index: 500},
	})
	slot := &app.save.Slots[0]
	before := append([]byte(nil), slot.Data...)

	if _, err := app.StartInventoryEditSession(0); err != nil {
		t.Fatalf("StartInventoryEditSession: %v", err)
	}
	sessID := app.editSessionByChar[0]

	fp, _ := core.FingerprintRecordAt(slot, "inventory_common", 0)
	key := core.IssueKey{
		Slot: 0, Domain: "inventory", Code: core.RepairCodeQuantityAboveMax,
		Scope: "inventory_common", Row: 0, Handle: smithingStoneHandleQty, Field: "quantity", Value: "999",
	}
	target := RepairApplyTarget{IssueID: core.IssueKeyID(key), Key: key, Fingerprint: fp, SelectedAction: core.RepairActionClampQuantity}

	rep, err := app.ApplyRepairsLoaded(0, []RepairApplyTarget{target}, false)
	if err != nil {
		t.Fatalf("ApplyRepairsLoaded: %v", err)
	}
	if rep.Applied != 0 || rep.Failed != 1 {
		t.Fatalf("want applied=0 failed=1, got applied=%d failed=%d", rep.Applied, rep.Failed)
	}
	if got := len(app.undoStacks[0]); got != 0 {
		t.Errorf("undo pushed on a no-op clamp batch: depth=%d", got)
	}
	if id := app.editSessionByChar[0]; id != sessID {
		t.Errorf("editSessionByChar[0] = %q, want retained %q", id, sessID)
	}
	if _, err := app.GetInventoryEditSession(sessID); err != nil {
		t.Errorf("original session no longer usable after no-op clamp: %v", err)
	}
	if !bytes.Equal(before, slot.Data) || slot.Inventory.CommonItems[0].Quantity != 999 {
		t.Fatal("no-op clamp mutated the slot")
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

// golemUpgradeRepairFixture models the exact regression reported from the
// repair modal: one Golem Greatbow +25 in inventory and one in storage. The
// two records deliberately use different weapon-instance handles, while their
// invalid ItemID encoding is the same.
func golemUpgradeRepairFixture() *App {
	const (
		magicOffset   = 0x2000
		storageOffset = 0x12000
		golemPlus25   = uint32(0x02810590 + 25)
	)
	const (
		invHandle = uint32(0x80800021)
		stoHandle = uint32(0x80800022)
	)

	app := NewApp()
	app.save = &core.SaveFile{}
	slot := &app.save.Slots[0]
	slot.Version = 1
	slot.MagicOffset = magicOffset
	slot.StorageBoxOffset = storageOffset
	slot.Data = make([]byte, storageOffset+core.StorageHeaderSkip+core.StorageCommonCount*core.InvRecordLen+64)
	slot.GaMap = map[uint32]uint32{invHandle: golemPlus25, stoHandle: golemPlus25}
	slot.GaItems = []core.GaItemFull{
		{Handle: invHandle, ItemID: golemPlus25, Unk2: -1, Unk3: -1, AoWGaItemHandle: core.NoCustomAoWHandle},
		{Handle: stoHandle, ItemID: golemPlus25, Unk2: -1, Unk3: -1, AoWGaItemHandle: core.NoCustomAoWHandle},
	}

	gaOff := core.GaItemsStart
	for _, item := range slot.GaItems {
		gaOff += item.Serialize(slot.Data[gaOff:])
	}
	slot.InventoryEnd = gaOff

	invOff := magicOffset + core.InvStartFromMagic
	binary.LittleEndian.PutUint32(slot.Data[invOff:], invHandle)
	binary.LittleEndian.PutUint32(slot.Data[invOff+4:], 1)
	binary.LittleEndian.PutUint32(slot.Data[invOff+8:], 100)
	slot.Inventory.CommonItems = []core.InventoryItem{{GaItemHandle: invHandle, Quantity: 1, Index: 100}}

	stoOff := storageOffset + core.StorageHeaderSkip
	binary.LittleEndian.PutUint32(slot.Data[stoOff:], stoHandle)
	binary.LittleEndian.PutUint32(slot.Data[stoOff+4:], 1)
	binary.LittleEndian.PutUint32(slot.Data[stoOff+8:], 200)
	slot.Storage.CommonItems = []core.InventoryItem{{GaItemHandle: stoHandle, Quantity: 1, Index: 200}}

	return app
}

// TestApplyRepairsLoaded_ClampUpgrade_InventoryAndStorage is the automated
// regression for the manual flow: the central modal must apply both upgrades,
// and its next scan must be clean. It also guards against a layout rebuild by
// checking that the original record indices remain untouched.
func TestApplyRepairsLoaded_ClampUpgrade_InventoryAndStorage(t *testing.T) {
	const golemPlus10 = uint32(0x02810590 + 10)
	app := golemUpgradeRepairFixture()
	slot := &app.save.Slots[0]

	beforeInvIndex := binary.LittleEndian.Uint32(slot.Data[slot.MagicOffset+core.InvStartFromMagic+8:])
	beforeStoIndex := binary.LittleEndian.Uint32(slot.Data[slot.StorageBoxOffset+core.StorageHeaderSkip+8:])

	report, err := app.ScanRepairIssuesLoaded(0)
	if err != nil {
		t.Fatalf("ScanRepairIssuesLoaded before repair: %v", err)
	}
	targets := make([]RepairApplyTarget, 0, 2)
	for _, issue := range report.Issues {
		if issue.Key.Code != editor.CodeUpgradeOutOfRange {
			continue
		}
		if issue.Fingerprint == "" {
			t.Fatalf("upgrade issue at %s row %d is missing a fingerprint", issue.Key.Scope, issue.Key.Row)
		}
		targets = append(targets, RepairApplyTarget{
			IssueID: issue.IssueID, Key: issue.Key, Fingerprint: issue.Fingerprint,
			SelectedAction: RepairActionClampUpgrade,
		})
	}
	if len(targets) != 2 {
		t.Fatalf("upgrade targets = %d, want inventory + storage", len(targets))
	}

	rep, err := app.ApplyRepairsLoaded(0, targets, false)
	if err != nil {
		t.Fatalf("ApplyRepairsLoaded: %v", err)
	}
	if rep.Applied != 2 || rep.Skipped != 0 || rep.Failed != 0 {
		t.Fatalf("want 2 applied and no skips/failures, got %+v", rep)
	}
	if got := len(app.undoStacks[0]); got != 1 {
		t.Errorf("undo depth = %d, want one batch snapshot", got)
	}

	for _, handle := range []uint32{0x80800021, 0x80800022} {
		if got := slot.GaMap[handle]; got != golemPlus10 {
			t.Errorf("GaMap[0x%08X] = 0x%08X, want +10 0x%08X", handle, got, golemPlus10)
		}
	}
	if got := binary.LittleEndian.Uint32(slot.Data[slot.MagicOffset+core.InvStartFromMagic+8:]); got != beforeInvIndex {
		t.Errorf("inventory acquisition index changed %d -> %d", beforeInvIndex, got)
	}
	if got := binary.LittleEndian.Uint32(slot.Data[slot.StorageBoxOffset+core.StorageHeaderSkip+8:]); got != beforeStoIndex {
		t.Errorf("storage acquisition index changed %d -> %d", beforeStoIndex, got)
	}

	after, err := app.ScanRepairIssuesLoaded(0)
	if err != nil {
		t.Fatalf("ScanRepairIssuesLoaded after repair: %v", err)
	}
	for _, issue := range after.Issues {
		if issue.Key.Code == editor.CodeUpgradeOutOfRange {
			t.Fatalf("upgrade issue remains after repair: %+v", issue)
		}
	}
}
