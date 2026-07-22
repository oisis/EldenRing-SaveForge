package main

import (
	"encoding/binary"
	"strings"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
)

// repairFixture builds an App whose slot 0 has the inventory layout backed by
// real slot.Data bytes and counter offsets wired through to Inventory — the
// minimum needed for RepairDuplicateInventoryIndices to write Index bytes and
// counters back to raw storage.
func repairFixture(common, key []core.InventoryItem) *App {
	const magicOff = 0
	commonStart := magicOff + core.InvStartFromMagic
	keyStart := commonStart + core.CommonItemCount*core.InvRecordLen + core.InvKeyCountHeader
	nextEquipIdxOff := keyStart + core.KeyItemCount*core.InvRecordLen
	nextAcqSortIdOff := nextEquipIdxOff + 4
	bufSize := nextAcqSortIdOff + 4 + 64

	app := NewApp()
	app.save = &core.SaveFile{}
	slot := &app.save.Slots[0]
	slot.Version = 1
	slot.MagicOffset = magicOff
	slot.Data = make([]byte, bufSize)
	slot.GaMap = make(map[uint32]uint32)

	for i, it := range common {
		off := commonStart + i*core.InvRecordLen
		binary.LittleEndian.PutUint32(slot.Data[off:], it.GaItemHandle)
		binary.LittleEndian.PutUint32(slot.Data[off+4:], it.Quantity)
		binary.LittleEndian.PutUint32(slot.Data[off+8:], it.Index)
	}
	for i, it := range key {
		off := keyStart + i*core.InvRecordLen
		binary.LittleEndian.PutUint32(slot.Data[off:], it.GaItemHandle)
		binary.LittleEndian.PutUint32(slot.Data[off+4:], it.Quantity)
		binary.LittleEndian.PutUint32(slot.Data[off+8:], it.Index)
	}
	slot.Inventory.CommonItems = append([]core.InventoryItem(nil), common...)
	slot.Inventory.KeyItems = append([]core.InventoryItem(nil), key...)

	var maxAcq uint32
	for _, it := range common {
		if it.Index > maxAcq {
			maxAcq = it.Index
		}
	}
	for _, it := range key {
		if it.Index > maxAcq {
			maxAcq = it.Index
		}
	}
	slot.Inventory.NextAcquisitionSortId = maxAcq + 1
	slot.Inventory.NextEquipIndex = maxAcq + 1
	binary.LittleEndian.PutUint32(slot.Data[nextEquipIdxOff:], slot.Inventory.NextEquipIndex)
	binary.LittleEndian.PutUint32(slot.Data[nextAcqSortIdOff:], slot.Inventory.NextAcquisitionSortId)
	return app
}

func TestApp_RepairDuplicateInventoryIndices_NoSave(t *testing.T) {
	app := NewApp()
	_, err := app.RepairDuplicateInventoryIndices(0)
	if err == nil || !strings.Contains(err.Error(), "no save loaded") {
		t.Fatalf("expected 'no save loaded', got %v", err)
	}
}

func TestApp_RepairDuplicateInventoryIndices_InvalidIdx(t *testing.T) {
	app := repairFixture(
		[]core.InventoryItem{{GaItemHandle: 0xB0000001, Quantity: 1, Index: 100}},
		nil,
	)
	for _, idx := range []int{-1, 10, 99} {
		_, err := app.RepairDuplicateInventoryIndices(idx)
		if err == nil || !strings.Contains(err.Error(), "invalid character index") {
			t.Errorf("idx=%d: expected 'invalid character index', got %v", idx, err)
		}
	}
}

func TestApp_RepairDuplicateInventoryIndices_CleanSlot_NoOp_NoUndo(t *testing.T) {
	app := repairFixture(
		[]core.InventoryItem{
			{GaItemHandle: 0xB0000001, Quantity: 1, Index: 100},
			{GaItemHandle: 0xB0000002, Quantity: 1, Index: 102}, // stride-2 → distinct bucket → clean
		},
		nil,
	)
	if got := len(app.undoStacks[0]); got != 0 {
		t.Fatalf("undo stack not empty pre-call: %d", got)
	}
	preIndex0 := app.save.Slots[0].Inventory.CommonItems[0].Index
	preIndex1 := app.save.Slots[0].Inventory.CommonItems[1].Index
	preAcq := app.save.Slots[0].Inventory.NextAcquisitionSortId

	report, err := app.RepairDuplicateInventoryIndices(0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.Changed != 0 || len(report.Changes) != 0 {
		t.Errorf("expected no-op report, got %+v", report)
	}
	if got := len(app.undoStacks[0]); got != 0 {
		t.Errorf("undo pushed on no-op clean slot: depth=%d", got)
	}
	slot := &app.save.Slots[0]
	if slot.Inventory.CommonItems[0].Index != preIndex0 ||
		slot.Inventory.CommonItems[1].Index != preIndex1 {
		t.Errorf("clean slot indices were mutated by no-op call")
	}
	if slot.Inventory.NextAcquisitionSortId != preAcq {
		t.Errorf("counter advanced on no-op: %d → %d", preAcq, slot.Inventory.NextAcquisitionSortId)
	}
}

func TestApp_RepairDuplicateInventoryIndices_DuplicateSlot_RepairsAndPushesUndo(t *testing.T) {
	app := repairFixture(
		[]core.InventoryItem{
			{GaItemHandle: 0xB0000334, Quantity: 99, Index: 552},
			{GaItemHandle: 0xB00003C0, Quantity: 99, Index: 552},
		},
		nil,
	)
	slot := &app.save.Slots[0]
	preAcq := slot.Inventory.NextAcquisitionSortId

	report, err := app.RepairDuplicateInventoryIndices(0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.Changed != 1 || len(report.Changes) != 1 {
		t.Fatalf("expected 1 change, got %+v", report)
	}
	c := report.Changes[0]
	if c.Scope != "inventory_common" || c.OldIndex != 552 || c.Handle != 0xB00003C0 {
		t.Errorf("unexpected change details: %+v", c)
	}
	if c.NewIndex <= 552 {
		t.Errorf("NewIndex must be > 552, got %d", c.NewIndex)
	}
	if got := len(app.undoStacks[0]); got != 1 {
		t.Errorf("undo not pushed: depth=%d", got)
	}
	if issues := core.ScanDuplicateInventoryIndices(slot); len(issues) != 0 {
		t.Errorf("post-repair scan still reports %d issues", len(issues))
	}
	if slot.Inventory.NextAcquisitionSortId <= preAcq {
		t.Errorf("counter did not advance: %d → %d", preAcq, slot.Inventory.NextAcquisitionSortId)
	}
	// Raw slot.Data must reflect the new Index at row 1.
	rawOff := slot.MagicOffset + core.InvStartFromMagic + 1*core.InvRecordLen + 8
	rawIdx := binary.LittleEndian.Uint32(slot.Data[rawOff:])
	if rawIdx != c.NewIndex {
		t.Errorf("raw slot.Data not updated: raw=%d new=%d", rawIdx, c.NewIndex)
	}
}

func TestApp_RepairDuplicateInventoryIndices_UndoRestoresOriginalState(t *testing.T) {
	app := repairFixture(
		[]core.InventoryItem{
			{GaItemHandle: 0xB0000334, Quantity: 99, Index: 552},
			{GaItemHandle: 0xB00003C0, Quantity: 99, Index: 552},
		},
		nil,
	)
	slot := &app.save.Slots[0]
	origIndex0 := slot.Inventory.CommonItems[0].Index
	origIndex1 := slot.Inventory.CommonItems[1].Index
	origAcq := slot.Inventory.NextAcquisitionSortId

	if _, err := app.RepairDuplicateInventoryIndices(0); err != nil {
		t.Fatalf("repair: %v", err)
	}
	if err := app.RevertSlot(0); err != nil {
		t.Fatalf("revert: %v", err)
	}
	slot = &app.save.Slots[0]
	if slot.Inventory.CommonItems[0].Index != origIndex0 ||
		slot.Inventory.CommonItems[1].Index != origIndex1 {
		t.Errorf("undo did not restore CommonItems Index values")
	}
	if slot.Inventory.NextAcquisitionSortId != origAcq {
		t.Errorf("undo did not restore NextAcquisitionSortId: got %d want %d",
			slot.Inventory.NextAcquisitionSortId, origAcq)
	}
	// And a second repair after undo should still detect the original duplicate.
	if issues := core.ScanDuplicateInventoryIndices(slot); len(issues) != 1 {
		t.Errorf("expected 1 issue restored after undo, got %d", len(issues))
	}
}

func TestApp_RepairDuplicateInventoryIndices_PostRepairValidationPasses(t *testing.T) {
	app := repairFixture(
		[]core.InventoryItem{
			{GaItemHandle: 0xB0000010, Quantity: 1, Index: 500},
			{GaItemHandle: 0xB0000020, Quantity: 1, Index: 500},
			{GaItemHandle: 0xB0000030, Quantity: 1, Index: 600},
			{GaItemHandle: 0xB0000040, Quantity: 1, Index: 600},
		},
		nil,
	)
	if _, err := app.RepairDuplicateInventoryIndices(0); err != nil {
		t.Fatalf("repair: %v", err)
	}
	slot := &app.save.Slots[0]
	if v := core.ValidatePostMutation(slot); len(v) != 0 {
		t.Errorf("ValidatePostMutation should pass after repair, got %d violations", len(v))
	}
	if v := core.ScanDuplicateInventoryIndices(slot); len(v) != 0 {
		t.Errorf("ScanDuplicateInventoryIndices should be empty after repair, got %d", len(v))
	}
}

// TestApp_AddBlockedByBucketCollision_670_671_ThenRepairClears is the app-level
// fail-closed regression for the reported bug: a slot carrying an adjacent
// 670/671 acquisition pair (same Index>>1 bucket) must refuse item adds with a
// bucket-collision diagnostic, and after RepairDuplicateInventoryIndices the
// scanner must report zero issues so adds can proceed.
func TestApp_AddBlockedByBucketCollision_670_671_ThenRepairClears(t *testing.T) {
	app := repairFixture([]core.InventoryItem{
		{GaItemHandle: 0xB0000A01, Quantity: 1, Index: 670},
		{GaItemHandle: 0xB0000A02, Quantity: 1, Index: 671}, // shares bucket 335
	}, nil)

	// Pre-flight must fail-closed and name the real defect (any itemID: the
	// guard fires before the ID is resolved).
	_, err := app.AddItemsToCharacter(0, []uint32{1000000}, 0, 0, 0, 0, 1, 0)
	if err == nil {
		t.Fatalf("expected add to be blocked by the bucket-collision pre-flight")
	}
	if !strings.Contains(err.Error(), "bucket collision") {
		t.Errorf("error should name the acquisition-order bucket collision, got %q", err.Error())
	}

	// Repair, then the scanner must be clean and the buckets distinct.
	if _, err := app.RepairDuplicateInventoryIndices(0); err != nil {
		t.Fatalf("RepairDuplicateInventoryIndices: %v", err)
	}
	slot := &app.save.Slots[0]
	if issues := core.ScanDuplicateInventoryIndices(slot); len(issues) != 0 {
		t.Fatalf("scanner still reports %d issue(s) after repair: %+v", len(issues), issues)
	}
	if b0, b1 := slot.Inventory.CommonItems[0].Index>>1, slot.Inventory.CommonItems[1].Index>>1; b0 == b1 {
		t.Errorf("records still share bucket %d after repair", b0)
	}
}
