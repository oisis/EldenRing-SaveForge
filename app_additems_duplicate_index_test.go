package main

import (
	"strings"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
)

// dupIndexFixture builds a minimal App whose slot 0 already contains two
// non-empty CommonItems sharing the same acquisition Index — the exact shape
// the new fail-closed pre-flight is meant to refuse before AddItemsToCharacter
// mutates anything.
func dupIndexFixture() *App {
	app := NewApp()
	app.save = &core.SaveFile{}
	slot := &app.save.Slots[0]
	slot.Version = 1
	slot.Inventory.CommonItems = []core.InventoryItem{
		{GaItemHandle: 0xB0000334, Quantity: 99, Index: 552},
		{GaItemHandle: 0xB00003C0, Quantity: 99, Index: 552},
	}
	slot.Inventory.NextAcquisitionSortId = 600
	slot.Inventory.NextEquipIndex = 600
	return app
}

// TestAddItemsToCharacter_RejectsPreExistingDuplicateIndex pins the new
// fail-closed behaviour: when the target slot already contains duplicate
// acquisition indices, AddItemsToCharacter must refuse to mutate and surface
// the integrity problem. The pre-existing records must remain untouched so a
// downstream RepairDuplicateInventoryIndices call can still see the original
// duplicates.
func TestAddItemsToCharacter_RejectsPreExistingDuplicateIndex(t *testing.T) {
	app := dupIndexFixture()
	slot := &app.save.Slots[0]

	preLenCommon := len(slot.Inventory.CommonItems)
	preIndex0 := slot.Inventory.CommonItems[0].Index
	preIndex1 := slot.Inventory.CommonItems[1].Index
	preHandle0 := slot.Inventory.CommonItems[0].GaItemHandle
	preHandle1 := slot.Inventory.CommonItems[1].GaItemHandle
	preQty0 := slot.Inventory.CommonItems[0].Quantity
	preQty1 := slot.Inventory.CommonItems[1].Quantity
	preNextAcq := slot.Inventory.NextAcquisitionSortId
	preNextEquip := slot.Inventory.NextEquipIndex

	result, err := app.AddItemsToCharacter(0, []uint32{0x000F4240}, 0, 0, 0, 0, 1, 0)

	if err == nil {
		t.Fatalf("expected fail-closed error on pre-existing duplicates, got nil (result=%+v)", result)
	}
	msg := err.Error()
	if !strings.Contains(msg, "inventory integrity issue") {
		t.Errorf("error should describe the integrity issue, got %q", msg)
	}
	if !strings.Contains(msg, "repair is required") {
		t.Errorf("error should mention that repair is required, got %q", msg)
	}
	// The historical tolerance phrase must not leak back into the error path.
	for _, forbidden := range []string{"tolerating", "game accepts them"} {
		if strings.Contains(msg, forbidden) {
			t.Errorf("error must not carry tolerance phrasing %q, got %q", forbidden, msg)
		}
	}

	// No items added: AddResult.Added stays at the zero value.
	if result.Added != 0 {
		t.Errorf("Added should be 0 on rejection, got %d", result.Added)
	}

	// Slot must be untouched: no new rows, no Index rewrites, no quantity drift.
	if got := len(slot.Inventory.CommonItems); got != preLenCommon {
		t.Errorf("CommonItems length changed: pre=%d post=%d", preLenCommon, got)
	}
	if slot.Inventory.CommonItems[0].Index != preIndex0 || slot.Inventory.CommonItems[1].Index != preIndex1 {
		t.Errorf("pre-existing duplicate indices were altered: %d, %d (want %d, %d)",
			slot.Inventory.CommonItems[0].Index, slot.Inventory.CommonItems[1].Index,
			preIndex0, preIndex1)
	}
	if slot.Inventory.CommonItems[0].GaItemHandle != preHandle0 || slot.Inventory.CommonItems[1].GaItemHandle != preHandle1 {
		t.Errorf("pre-existing handles were altered")
	}
	if slot.Inventory.CommonItems[0].Quantity != preQty0 || slot.Inventory.CommonItems[1].Quantity != preQty1 {
		t.Errorf("pre-existing quantities were altered")
	}
	if slot.Inventory.NextAcquisitionSortId != preNextAcq || slot.Inventory.NextEquipIndex != preNextEquip {
		t.Errorf("inventory counters drifted: NextAcq %d→%d, NextEquip %d→%d",
			preNextAcq, slot.Inventory.NextAcquisitionSortId,
			preNextEquip, slot.Inventory.NextEquipIndex)
	}

	// The scanner must still report the same duplicates after the rejection so
	// a follow-up RepairDuplicateInventoryIndices call has work to do.
	if got := len(core.ScanDuplicateInventoryIndices(slot)); got != 1 {
		t.Errorf("post-reject scan should still report 1 duplicate, got %d", got)
	}

	// Fail-closed rejection happens before pushUndoLocked / SnapshotSlot, so
	// the undo stack for this slot must stay at depth 0. A non-empty stack
	// would mean the operation poisoned undo history with the unchanged
	// pre-state — confusing for the user and a regression of the contract.
	if got := len(app.undoStacks[0]); got != 0 {
		t.Errorf("undo stack should be empty after fail-closed reject, got depth %d", got)
	}

	// Defence vs future AddResult shape changes: every secondary field must
	// stay at its zero value when the call rejects before any planning.
	if result.Trimmed != nil {
		t.Errorf("Trimmed should be nil on rejection, got %+v", result.Trimmed)
	}
	if result.CapHit != "" {
		t.Errorf("CapHit should be empty on rejection, got %q", result.CapHit)
	}
}

// dupIndexFixtureKey mirrors dupIndexFixture but plants the duplicate
// acquisition Index in Inventory.KeyItems instead of CommonItems. The
// fail-closed pre-flight must reject either scope identically.
func dupIndexFixtureKey() *App {
	app := NewApp()
	app.save = &core.SaveFile{}
	slot := &app.save.Slots[0]
	slot.Version = 1
	slot.Inventory.KeyItems = []core.InventoryItem{
		{GaItemHandle: 0xB0000334, Quantity: 1, Index: 552},
		{GaItemHandle: 0xB00003C0, Quantity: 1, Index: 552},
	}
	slot.Inventory.NextAcquisitionSortId = 600
	slot.Inventory.NextEquipIndex = 600
	return app
}

// TestAddItemsToCharacter_RejectsPreExistingDuplicateIndex_InKeyItems pins the
// fail-closed behaviour against a key-items duplicate, mirroring the
// CommonItems test above. core.ScanDuplicateInventoryIndices walks both
// scopes, so the pre-flight must refuse on either side.
func TestAddItemsToCharacter_RejectsPreExistingDuplicateIndex_InKeyItems(t *testing.T) {
	app := dupIndexFixtureKey()
	slot := &app.save.Slots[0]

	preLenKey := len(slot.Inventory.KeyItems)
	preIndex0 := slot.Inventory.KeyItems[0].Index
	preIndex1 := slot.Inventory.KeyItems[1].Index

	result, err := app.AddItemsToCharacter(0, []uint32{0x000F4240}, 0, 0, 0, 0, 1, 0)

	if err == nil {
		t.Fatalf("expected fail-closed error on KeyItems duplicate, got nil (result=%+v)", result)
	}
	if !strings.Contains(err.Error(), "inventory integrity issue") {
		t.Errorf("error should describe the integrity issue, got %q", err.Error())
	}
	if result.Added != 0 {
		t.Errorf("Added should be 0 on rejection, got %d", result.Added)
	}
	if got := len(slot.Inventory.KeyItems); got != preLenKey {
		t.Errorf("KeyItems length changed: pre=%d post=%d", preLenKey, got)
	}
	if slot.Inventory.KeyItems[0].Index != preIndex0 || slot.Inventory.KeyItems[1].Index != preIndex1 {
		t.Errorf("KeyItems indices were altered: %d, %d (want %d, %d)",
			slot.Inventory.KeyItems[0].Index, slot.Inventory.KeyItems[1].Index,
			preIndex0, preIndex1)
	}
	if got := len(app.undoStacks[0]); got != 0 {
		t.Errorf("undo stack should be empty after fail-closed reject, got depth %d", got)
	}
}
