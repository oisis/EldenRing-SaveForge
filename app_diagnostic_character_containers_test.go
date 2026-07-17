package main

import (
	"encoding/binary"
	"fmt"
	"strings"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
	"github.com/oisis/EldenRing-SaveForge/backend/db/data"
	"github.com/oisis/EldenRing-SaveForge/backend/vm"
)

// sparkAromaticHandle is Spark Aromatic's Inventory/Storage goods handle,
// mirroring firePotHandle/upliftingHandle in app_container_keyitem_test.go.
const sparkAromaticHandle = uint32(0xB0000DB6)

// TestSaveCharacterContainerDiagnosticSuccess bumps Fire Pot 1 -> 3 via the real
// GetCharacter -> SaveCharacter path and asserts the auto-synced Cracked Pot logs
// its container quantity, every required pickup flag, and Kale's vendor flag
// through the full before -> planned -> finished lifecycle, with global phase
// grouping preserved.
func TestSaveCharacterContainerDiagnosticSuccess(t *testing.T) {
	app := containerEditFixture(t, firePotHandle, firePotID, 1, crackedPotHandle, 1)
	slot := &app.save.Slots[0]
	enableDebugJournal(t, app)

	charVM, err := app.GetCharacter(0)
	if err != nil {
		t.Fatalf("GetCharacter: %v", err)
	}
	bumped := false
	for i := range charVM.Inventory {
		if charVM.Inventory[i].Handle == firePotHandle {
			charVM.Inventory[i].Quantity = 3
			bumped = true
			break
		}
	}
	if !bumped {
		t.Fatalf("Fire Pot (0x%08X) absent from GetCharacter VM", firePotHandle)
	}
	if err := app.SaveCharacter(0, *charVM); err != nil {
		t.Fatalf("SaveCharacter: %v", err)
	}

	records := characterRecords(app.journal.Tail())
	cid := data.CrackedPotKeyItemID

	assertSideEffectPhases(t, records,
		fmt.Sprintf("container_0x%08X_quantity", cid),
		"1", "3", "3", characterChangeSuccess, characterStageCompleted)
	// finalQty 3 -> the first three pickup flags are set.
	for _, f := range data.ContainerPickupFlags[cid][:3] {
		assertSideEffectPhases(t, records,
			fmt.Sprintf("container_0x%08X_pickup_flag_%d", cid, f),
			"false", "true", "true", characterChangeSuccess, characterStageCompleted)
	}
	for _, f := range data.ContainerVendorPurchaseFlags[cid] {
		assertSideEffectPhases(t, records,
			fmt.Sprintf("container_0x%08X_vendor_flag_%d", cid, f),
			"false", "true", "true", characterChangeSuccess, characterStageCompleted)
	}

	if _, qty := findCommonItem(slot, crackedPotHandle); qty != 3 {
		t.Errorf("Cracked Pot physical qty = %d, want 3", qty)
	}
	assertPhaseGrouping(t, records)
}

// TestSaveCharacterContainerDiagnosticQuantityNetZero lowers a container the VM
// still needs at full quantity: Cracked Pot 3, the VM drops it to 1 while Fire
// Pot stays at 3, so syncContainerKeyItems raises it straight back to 3. The
// semantic container quantity nets zero change and must emit no lifecycle, while
// the Task 4C.1 direct physical row still logs the writer's 3 -> 1 -> 3 attempt
// and the flags the sync actually flips stay logged.
func TestSaveCharacterContainerDiagnosticQuantityNetZero(t *testing.T) {
	app := containerEditFixture(t, firePotHandle, firePotID, 3, crackedPotHandle, 3)
	slot := &app.save.Slots[0]
	enableDebugJournal(t, app)

	charVM, err := app.GetCharacter(0)
	if err != nil {
		t.Fatalf("GetCharacter: %v", err)
	}
	loweredContainer, keptItem := false, false
	for i := range charVM.Inventory {
		switch charVM.Inventory[i].Handle {
		case crackedPotHandle:
			charVM.Inventory[i].Quantity = 1
			loweredContainer = true
		case firePotHandle:
			charVM.Inventory[i].Quantity = 3
			keptItem = true
		}
	}
	if !loweredContainer || !keptItem {
		t.Fatalf("VM setup incomplete: loweredContainer=%v keptItem=%v", loweredContainer, keptItem)
	}
	if err := app.SaveCharacter(0, *charVM); err != nil {
		t.Fatalf("SaveCharacter: %v", err)
	}

	records := characterRecords(app.journal.Tail())
	cid := data.CrackedPotKeyItemID

	// Semantic container quantity nets zero change -> no lifecycle.
	quantityField := fmt.Sprintf("container_0x%08X_quantity", cid)
	if got := saveCharacterPhases(records, quantityField); len(got) != 0 {
		t.Errorf("field %q emitted %d records, want 0 (net-zero container quantity)", quantityField, len(got))
	}

	// The direct physical row (Task 4C.1) still logs the full 3 -> 1 -> 3 attempt.
	assertSideEffectPhases(t, records,
		fmt.Sprintf("inventory_common_row_1_handle_0x%08X_quantity", crackedPotHandle),
		"3", "1", "3", characterChangeSuccess, characterStageCompleted)

	// Flags the sync flips from false to true are still logged independently.
	for _, f := range data.ContainerPickupFlags[cid][:3] {
		assertSideEffectPhases(t, records,
			fmt.Sprintf("container_0x%08X_pickup_flag_%d", cid, f),
			"false", "true", "true", characterChangeSuccess, characterStageCompleted)
	}
	for _, f := range data.ContainerVendorPurchaseFlags[cid] {
		assertSideEffectPhases(t, records,
			fmt.Sprintf("container_0x%08X_vendor_flag_%d", cid, f),
			"false", "true", "true", characterChangeSuccess, characterStageCompleted)
	}

	if _, qty := findCommonItem(slot, crackedPotHandle); qty != 3 {
		t.Errorf("Cracked Pot physical qty = %d, want 3 (restored by sync)", qty)
	}
	assertPhaseGrouping(t, records)
}

// TestSaveCharacterContainerDiagnosticStorageOnly places a Perfume-gated aromatic
// in Storage only, with no Perfume Bottle container present, and asserts the
// container SaveCharacter forces into existence logs quantity "absent" -> "1"
// plus exactly its first pickup flag (Storage never raises the container past 1).
func TestSaveCharacterContainerDiagnosticStorageOnly(t *testing.T) {
	app := remembranceGameLimitsFixture()
	slot := &app.save.Slots[0]
	storageStart := slot.StorageBoxOffset + core.StorageHeaderSkip
	slot.GaMap[sparkAromaticHandle] = sparkAromaticID
	slot.Storage.CommonItems = make([]core.InventoryItem, core.StorageCommonCount)
	slot.Storage.CommonItems[0] = core.InventoryItem{GaItemHandle: sparkAromaticHandle, Quantity: 2}
	binary.LittleEndian.PutUint32(slot.Data[storageStart:], sparkAromaticHandle)
	binary.LittleEndian.PutUint32(slot.Data[storageStart+4:], 2)
	withContainerEventFlags(app)
	enableDebugJournal(t, app)

	charVM := vm.CharacterViewModel{
		Storage: []vm.ItemViewModel{{Handle: sparkAromaticHandle, Quantity: 2, MaxStorage: 600}},
	}
	if err := app.SaveCharacter(0, charVM); err != nil {
		t.Fatalf("SaveCharacter: %v", err)
	}

	records := characterRecords(app.journal.Tail())
	cid := data.PerfumeBottleKeyItemID

	assertSideEffectPhases(t, records,
		fmt.Sprintf("container_0x%08X_quantity", cid),
		containerAbsent, "1", "1", characterChangeSuccess, characterStageCompleted)
	assertSideEffectPhases(t, records,
		fmt.Sprintf("container_0x%08X_pickup_flag_%d", cid, data.ContainerPickupFlags[cid][0]),
		"false", "true", "true", characterChangeSuccess, characterStageCompleted)

	// The second pickup flag stays unset: Storage forces only the minimal 1.
	second := fmt.Sprintf("container_0x%08X_pickup_flag_%d", cid, data.ContainerPickupFlags[cid][1])
	if got := saveCharacterPhases(records, second); len(got) != 0 {
		t.Errorf("field %q emitted %d records, want 0 (Storage forces only 1)", second, len(got))
	}

	if row, qty := findCommonItem(slot, perfumeBottleHandle); row < 0 || qty != 1 {
		t.Errorf("Perfume Bottle = %d (row %d), want qty 1 created", qty, row)
	}
	assertPhaseGrouping(t, records)
}

// TestSaveCharacterContainerDiagnosticNoEventFlags bumps Fire Pot on a slot with
// no Event Flags region and asserts the container quantity is still logged while
// no pickup/vendor flag record is emitted (the writer cannot touch those flags).
func TestSaveCharacterContainerDiagnosticNoEventFlags(t *testing.T) {
	app := remembranceGameLimitsFixture()
	slot := &app.save.Slots[0]
	invStart := slot.MagicOffset + core.InvStartFromMagic
	slot.GaMap[firePotHandle] = firePotID
	binary.LittleEndian.PutUint32(slot.Data[invStart:], firePotHandle)
	binary.LittleEndian.PutUint32(slot.Data[invStart+4:], 1)
	slot.Inventory.CommonItems[0] = core.InventoryItem{GaItemHandle: firePotHandle, Quantity: 1}
	binary.LittleEndian.PutUint32(slot.Data[invStart+core.InvRecordLen:], crackedPotHandle)
	binary.LittleEndian.PutUint32(slot.Data[invStart+core.InvRecordLen+4:], 1)
	slot.Inventory.CommonItems[1] = core.InventoryItem{GaItemHandle: crackedPotHandle, Quantity: 1}
	// Deliberately no Event Flags region.
	enableDebugJournal(t, app)

	charVM := vm.CharacterViewModel{
		Inventory: []vm.ItemViewModel{{Handle: firePotHandle, Quantity: 2, MaxInventory: 99}},
	}
	if err := app.SaveCharacter(0, charVM); err != nil {
		t.Fatalf("SaveCharacter: %v", err)
	}

	records := characterRecords(app.journal.Tail())
	cid := data.CrackedPotKeyItemID

	assertSideEffectPhases(t, records,
		fmt.Sprintf("container_0x%08X_quantity", cid),
		"1", "2", "2", characterChangeSuccess, characterStageCompleted)

	for _, rec := range records {
		if field := operationField(rec, "field"); strings.Contains(field, "_flag_") {
			t.Errorf("flag record emitted without an Event Flags region: %q", field)
		}
	}
	if _, qty := findCommonItem(slot, crackedPotHandle); qty != 2 {
		t.Errorf("Cracked Pot physical qty = %d, want 2", qty)
	}
	assertPhaseGrouping(t, records)
}

// TestSaveCharacterContainerDiagnosticForcedSyncFailure sizes the slot so
// ApplyVMToParsedSlot's Fire Pot write lands but the container upsert at the next
// row falls out of bounds, forcing a sync_containers failure after a successful
// ApplyVM. The container plan must finish outcome=error, stage=sync_containers,
// reporting the real post-rollback quantity rather than the attempted one.
func TestSaveCharacterContainerDiagnosticForcedSyncFailure(t *testing.T) {
	app := NewApp()
	enableDebugJournal(t, app)
	app.save = &core.SaveFile{}
	slot := &app.save.Slots[0]
	slot.Version = 1
	slot.MagicOffset = 16
	// commonStart = 16 + 505 = 521. Fire Pot row 0 qty at 525 (needs len >= 529);
	// Cracked Pot row 1 qty at 537 (needs len >= 541 to succeed). Sizing to 540
	// lets ApplyVM's Fire Pot write land yet fails the container upsert at row 1.
	slot.Data = make([]byte, 540)
	slot.GaMap = map[uint32]uint32{firePotHandle: firePotID}
	slot.Inventory.CommonItems = []core.InventoryItem{
		{GaItemHandle: firePotHandle, Quantity: 1},
		{GaItemHandle: crackedPotHandle, Quantity: 1},
	}
	// No Event Flags region: the failure is about the quantity plan's terminal state.

	charVM := vm.CharacterViewModel{
		Inventory: []vm.ItemViewModel{{Handle: firePotHandle, Quantity: 2, MaxInventory: 99}},
	}
	if err := app.SaveCharacter(0, charVM); err == nil {
		t.Fatalf("SaveCharacter: expected sync_containers failure, got nil")
	}

	records := characterRecords(app.journal.Tail())
	cid := data.CrackedPotKeyItemID
	assertSideEffectPhases(t, records,
		fmt.Sprintf("container_0x%08X_quantity", cid),
		"1", "2", "1", characterChangeError, characterStageSyncContainers)

	if _, qty := findCommonItem(slot, crackedPotHandle); qty != 1 {
		t.Errorf("Cracked Pot qty after rollback = %d, want 1 (restored)", qty)
	}
}
