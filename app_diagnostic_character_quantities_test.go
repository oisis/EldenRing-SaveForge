package main

import (
	"fmt"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
	"github.com/oisis/EldenRing-SaveForge/backend/vm"
)

// TestSaveCharacterQuantityDiagnosticSuccess drives a real SaveCharacter that
// changes one physical quantity in each of the three writer-owned sections
// (Inventory Common, Inventory Key, Storage Common) and asserts each logs the
// full before -> planned -> finished lifecycle with the shared clamp applied,
// while an unchanged row stays silent and phase grouping holds globally.
func TestSaveCharacterQuantityDiagnosticSuccess(t *testing.T) {
	const (
		handleCommon    = uint32(0xB0001111) // inventory common, raised 1 -> 3
		handleKey       = uint32(0xB0002222) // inventory key, requested 500 -> clamped 99
		handleStorage   = uint32(0xB0003333) // storage common, requested 700 -> clamped 600
		handleUnchanged = uint32(0xB0004444) // inventory common, stays 1 (no record)
	)

	app := remembranceGameLimitsFixture()
	slot := &app.save.Slots[0]
	slot.Inventory.CommonItems[0] = core.InventoryItem{GaItemHandle: handleCommon, Quantity: 1}
	slot.Inventory.CommonItems[1] = core.InventoryItem{GaItemHandle: handleUnchanged, Quantity: 1}
	slot.Inventory.KeyItems = make([]core.InventoryItem, 3)
	slot.Inventory.KeyItems[2] = core.InventoryItem{GaItemHandle: handleKey, Quantity: 1}
	slot.Storage.CommonItems = []core.InventoryItem{{GaItemHandle: handleStorage, Quantity: 1}}
	enableDebugJournal(t, app)

	charVM := vm.CharacterViewModel{
		Inventory: []vm.ItemViewModel{
			{Handle: handleCommon, Quantity: 3, MaxInventory: 99},
			{Handle: handleKey, Quantity: 500, MaxInventory: 99},
			{Handle: handleUnchanged, Quantity: 1, MaxInventory: 99},
		},
		Storage: []vm.ItemViewModel{
			{Handle: handleStorage, Quantity: 700, MaxStorage: 600},
		},
	}

	if err := app.SaveCharacter(0, charVM); err != nil {
		t.Fatalf("SaveCharacter: %v", err)
	}

	records := characterRecords(app.journal.Tail())

	assertSideEffectPhases(t, records,
		fmt.Sprintf("inventory_common_row_0_handle_0x%08X_quantity", handleCommon),
		"1", "3", "3", characterChangeSuccess, characterStageCompleted)
	assertSideEffectPhases(t, records,
		fmt.Sprintf("inventory_key_row_2_handle_0x%08X_quantity", handleKey),
		"1", "99", "99", characterChangeSuccess, characterStageCompleted)
	assertSideEffectPhases(t, records,
		fmt.Sprintf("storage_common_row_0_handle_0x%08X_quantity", handleStorage),
		"1", "600", "600", characterChangeSuccess, characterStageCompleted)

	// The row whose submitted quantity already matches its physical value emits
	// nothing.
	unchanged := fmt.Sprintf("inventory_common_row_1_handle_0x%08X_quantity", handleUnchanged)
	if got := saveCharacterPhases(records, unchanged); len(got) != 0 {
		t.Errorf("field %q emitted %d records, want 0 (unchanged)", unchanged, len(got))
	}

	// The physical records must actually carry the clamped values.
	if got := slot.Inventory.KeyItems[2].Quantity; got != 99 {
		t.Errorf("key item quantity = %d, want 99 (clamped)", got)
	}
	if got := slot.Storage.CommonItems[0].Quantity; got != 600 {
		t.Errorf("storage item quantity = %d, want 600 (clamped)", got)
	}

	assertPhaseGrouping(t, records)
}

// TestSaveCharacterQuantityDiagnosticForcedFailure forces an ApplyVM failure by
// truncating slot.Data so the inventory common quantity write lands out of
// bounds, and asserts the planned row finishes with outcome=error, stage=apply_vm
// and its real (unchanged) post-error quantity — never claiming the write landed.
func TestSaveCharacterQuantityDiagnosticForcedFailure(t *testing.T) {
	const badHandle = uint32(0xB0001234)

	app := NewApp()
	enableDebugJournal(t, app)
	app.save = &core.SaveFile{}
	slot := &app.save.Slots[0]
	slot.Version = 1
	slot.MagicOffset = 16
	// commonStart = MagicOffset + 505 = 521; row 0 writes qty at 525, one byte
	// past this buffer → ApplyVMToParsedSlot fails at apply_vm before any qty is
	// committed.
	slot.Data = make([]byte, 524)
	slot.GaMap = map[uint32]uint32{}
	slot.Inventory.CommonItems = []core.InventoryItem{{GaItemHandle: badHandle, Quantity: 1}}

	charVM := vm.CharacterViewModel{
		Inventory: []vm.ItemViewModel{{Handle: badHandle, Quantity: 5, MaxInventory: 99}},
	}

	if err := app.SaveCharacter(0, charVM); err == nil {
		t.Fatalf("SaveCharacter: expected failure, got nil")
	}

	records := characterRecords(app.journal.Tail())
	assertSideEffectPhases(t, records,
		fmt.Sprintf("inventory_common_row_0_handle_0x%08X_quantity", badHandle),
		"1", "5", "1", characterChangeError, characterStageApplyVM)

	// The physical record must not have been mutated on the failure path.
	if got := slot.Inventory.CommonItems[0].Quantity; got != 1 {
		t.Errorf("common item quantity = %d, want 1 (never applied)", got)
	}
}
