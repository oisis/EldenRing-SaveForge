package tests

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
	"github.com/oisis/EldenRing-SaveForge/backend/db"
)

// ---------- Step 1: upsertGaItemData overflow ----------

func TestGaItemDataFull_ErrorNotSilent(t *testing.T) {
	save := loadTestSave(t, pcSavePath)
	idx := findActiveSlot(t, save)
	slot := &save.Slots[idx]

	if slot.GaItemDataOffset <= 0 {
		t.Skip("GaItemDataOffset not set")
	}

	binary.LittleEndian.PutUint32(slot.Data[slot.GaItemDataOffset:], uint32(core.GaItemDataMaxCount))

	err := core.AddItemsToSlot(slot, []uint32{0x00000064}, 1, 0, false)
	if err == nil {
		t.Fatal("Expected error when GaItemData is full, got nil")
	}
	t.Logf("Correctly returned error: %v", err)
}

// ---------- Step 2: Pre-flight capacity ----------

func TestPreFlightCapacity_Empty(t *testing.T) {
	save := loadTestSave(t, pcSavePath)
	idx := findActiveSlot(t, save)
	slot := &save.Slots[idx]

	items := []core.ItemToAdd{
		{ItemID: 0x40000064, InvQty: 1, StorageQty: 0, IsStackable: true},
	}
	report := core.CheckAddCapacity(slot, items)
	if !report.CanFitAll {
		t.Fatalf("Expected CanFitAll=true for single stackable item, got CapHit=%s", report.CapHit)
	}
	if report.FreeInv <= 0 {
		t.Fatal("Expected FreeInv > 0")
	}
	t.Logf("Free: inv=%d storage=%d gaItems=%d gaItemData=%d",
		report.FreeInv, report.FreeStorage, report.FreeGaItems, report.FreeGaItemData)
}

func TestPreFlightCapacity_CountsUsage(t *testing.T) {
	save := loadTestSave(t, pcSavePath)
	idx := findActiveSlot(t, save)
	slot := &save.Slots[idx]

	usage := core.CountSlotUsage(slot)
	if usage.GaItemsMax == 0 {
		t.Fatal("GaItemsMax should not be 0")
	}
	if usage.InventoryMax != core.CommonItemCount {
		t.Fatalf("InventoryMax=%d, want %d", usage.InventoryMax, core.CommonItemCount)
	}
	t.Logf("Usage: GaItems=%d/%d Inv=%d/%d Storage=%d/%d GaItemData=%d/%d",
		usage.GaItemsUsed, usage.GaItemsMax,
		usage.InventoryUsed, usage.InventoryMax,
		usage.StorageUsed, usage.StorageMax,
		usage.GaItemDataUsed, usage.GaItemDataMax)
}

// ---------- Step 4: Snapshot / Rollback ----------

func TestSnapshotRestore_ByteForByte(t *testing.T) {
	save := loadTestSave(t, pcSavePath)
	idx := findActiveSlot(t, save)
	slot := &save.Slots[idx]

	snap := core.SnapshotSlot(slot)
	originalData := make([]byte, len(slot.Data))
	copy(originalData, slot.Data)

	// Mutate slot
	slot.Data[0x100] ^= 0xFF
	slot.Player.Level = 999
	slot.GaMap[0xDEADBEEF] = 0x12345678

	// Restore
	core.RestoreSlot(slot, snap)

	if !bytes.Equal(slot.Data, originalData) {
		t.Fatal("Data not restored byte-for-byte")
	}
	if slot.Player.Level == 999 {
		t.Fatal("Player.Level not restored")
	}
	if _, ok := slot.GaMap[0xDEADBEEF]; ok {
		t.Fatal("GaMap phantom entry not cleaned")
	}
}

func TestSnapshotRestore_AfterError(t *testing.T) {
	save := loadTestSave(t, pcSavePath)
	idx := findActiveSlot(t, save)
	slot := &save.Slots[idx]

	snap := core.SnapshotSlot(slot)
	origGaMapLen := len(slot.GaMap)

	// Force GaItemData full, then attempt add
	if slot.GaItemDataOffset > 0 {
		binary.LittleEndian.PutUint32(slot.Data[slot.GaItemDataOffset:], uint32(core.GaItemDataMaxCount))
	}
	err := core.AddItemsToSlot(slot, []uint32{0x00000064}, 1, 0, false)
	if err != nil {
		core.RestoreSlot(slot, snap)
	}
	if len(slot.GaMap) != origGaMapLen {
		t.Fatalf("GaMap size changed after rollback: %d -> %d", origGaMapLen, len(slot.GaMap))
	}
}

// ---------- Step 5: Batch add ----------

func TestBatchAdd_SingleRebuild(t *testing.T) {
	save := loadTestSave(t, pcSavePath)
	idx := findActiveSlot(t, save)
	slot := &save.Slots[idx]

	weapons := []uint32{0x001E8480, 0x003D0900, 0x005B8D80}
	var items []core.ItemToAdd
	for _, id := range weapons {
		items = append(items, core.ItemToAdd{
			ItemID:     id,
			InvQty:     1,
			StorageQty: 0,
		})
	}

	gaMapBefore := len(slot.GaMap)
	err := core.AddItemsToSlotBatch(slot, items)
	if err != nil {
		t.Fatalf("AddItemsToSlotBatch failed: %v", err)
	}
	gaMapAfter := len(slot.GaMap)
	if gaMapAfter <= gaMapBefore {
		t.Fatalf("GaMap didn't grow: before=%d after=%d", gaMapBefore, gaMapAfter)
	}

	for _, id := range weapons {
		found := false
		for _, mapID := range slot.GaMap {
			if mapID == id {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Weapon 0x%08X not found in GaMap after batch add", id)
		}
	}
}

func TestBatchAdd_MixedStackableNonStackable(t *testing.T) {
	save := loadTestSave(t, pcSavePath)
	idx := findActiveSlot(t, save)
	slot := &save.Slots[idx]

	items := []core.ItemToAdd{
		{ItemID: 0x001E8480, InvQty: 1, StorageQty: 0, IsStackable: false},
		{ItemID: 0x40000064, InvQty: 99, StorageQty: 0, IsStackable: true},
	}

	err := core.AddItemsToSlotBatch(slot, items)
	if err != nil {
		t.Fatalf("Batch add mixed items failed: %v", err)
	}

	weaponFound := false
	goodsFound := false
	for _, id := range slot.GaMap {
		if id == 0x001E8480 {
			weaponFound = true
		}
		if id == 0x40000064 {
			goodsFound = true
		}
	}
	if !weaponFound {
		t.Error("Weapon not in GaMap")
	}
	if !goodsFound {
		t.Error("Goods not in GaMap")
	}
}

// ---------- Step 6: Post-mutation validation ----------

func TestPostValidation_CleanSlot(t *testing.T) {
	save := loadTestSave(t, pcSavePath)
	idx := findActiveSlot(t, save)
	slot := &save.Slots[idx]

	errs := core.ValidatePostMutation(slot)
	if len(errs) > 0 {
		for _, e := range errs {
			t.Logf("Violation: %s", e.Error())
		}
		t.Fatalf("Clean slot has %d post-mutation violations", len(errs))
	}
}

func TestPostValidation_DuplicateIndex(t *testing.T) {
	save := loadTestSave(t, pcSavePath)
	idx := findActiveSlot(t, save)
	slot := &save.Slots[idx]

	// Find two non-empty inventory items and set same Index
	var firstIdx, secondIdx int
	found := 0
	for i, item := range slot.Inventory.CommonItems {
		if item.GaItemHandle != 0 && item.GaItemHandle != core.GaHandleInvalid {
			if found == 0 {
				firstIdx = i
				found++
			} else if found == 1 {
				secondIdx = i
				found++
				break
			}
		}
	}
	if found < 2 {
		t.Skip("Need at least 2 inventory items to test duplicate index")
	}

	slot.Inventory.CommonItems[secondIdx].Index = slot.Inventory.CommonItems[firstIdx].Index
	errs := core.ValidatePostMutation(slot)
	hasDup := false
	for _, e := range errs {
		if e.Check == "duplicate_index" {
			hasDup = true
			break
		}
	}
	if !hasDup {
		t.Fatal("Expected duplicate_index violation, got none")
	}
}

func TestPostValidation_GaMapZeroID(t *testing.T) {
	save := loadTestSave(t, pcSavePath)
	idx := findActiveSlot(t, save)
	slot := &save.Slots[idx]

	slot.GaMap[0xBADBAD00] = 0
	errs := core.ValidatePostMutation(slot)
	hasZero := false
	for _, e := range errs {
		if e.Check == "gamap_zero_id" {
			hasZero = true
			break
		}
	}
	if !hasZero {
		t.Fatal("Expected gamap_zero_id violation, got none")
	}
}

// ---------- Step 9: Storage header reconciliation ----------

func TestStorageHeaderReconcile(t *testing.T) {
	save := loadTestSave(t, pcSavePath)
	idx := findActiveSlot(t, save)
	slot := &save.Slots[idx]

	if slot.StorageBoxOffset <= 0 {
		t.Skip("No storage offset")
	}

	// Set wrong header
	binary.LittleEndian.PutUint32(slot.Data[slot.StorageBoxOffset:], 9999)

	wrongHeader := binary.LittleEndian.Uint32(slot.Data[slot.StorageBoxOffset:])
	if wrongHeader != 9999 {
		t.Fatal("Failed to set wrong header")
	}

	core.ReconcileStorageHeader(slot)

	corrected := binary.LittleEndian.Uint32(slot.Data[slot.StorageBoxOffset:])
	if corrected == 9999 {
		t.Fatal("ReconcileStorageHeader did not fix the header")
	}

	// Count actual items
	actualCount := uint32(0)
	storageStart := slot.StorageBoxOffset + core.StorageHeaderSkip
	for i := 0; i < core.StorageCommonCount; i++ {
		off := storageStart + i*core.InvRecordLen
		if off+core.InvRecordLen > len(slot.Data) {
			break
		}
		h := binary.LittleEndian.Uint32(slot.Data[off:])
		if h != core.GaHandleEmpty && h != core.GaHandleInvalid {
			actualCount++
		}
	}
	if corrected != actualCount {
		t.Fatalf("Header %d != actual %d after reconciliation", corrected, actualCount)
	}
	t.Logf("Storage header reconciled: was 9999, now %d (actual items)", corrected)
}

// ---------- Roundtrip with batch add ----------

func TestRoundtrip_BatchAdd(t *testing.T) {
	save := loadTestSave(t, pcSavePath)
	idx := findActiveSlot(t, save)
	slot := &save.Slots[idx]

	items := []core.ItemToAdd{
		{ItemID: 0x001E8480, InvQty: 1, StorageQty: 0},
		{ItemID: 0x003D0900, InvQty: 1, StorageQty: 0},
		{ItemID: 0x40000064, InvQty: 99, StorageQty: 0, IsStackable: true},
	}

	snap := core.SnapshotSlot(slot)
	if err := core.AddItemsToSlotBatch(slot, items); err != nil {
		core.RestoreSlot(slot, snap)
		t.Fatalf("Batch add failed: %v", err)
	}

	// Post-validation
	if errs := core.ValidatePostMutation(slot); len(errs) > 0 {
		t.Fatalf("Post-mutation violations: %v", errs[0].Error())
	}

	// Roundtrip
	reloaded := saveTmpAndReload(t, save, "cap_roundtrip.sl2")
	rSlot := &reloaded.Slots[idx]

	for _, item := range items {
		found := false
		for _, id := range rSlot.GaMap {
			if id == item.ItemID {
				found = true
				break
			}
		}
		if !found {
			handlePrefix := db.ItemIDToHandlePrefix(item.ItemID)
			isStackable := handlePrefix == core.ItemTypeItem || handlePrefix == core.ItemTypeAccessory
			if !isStackable {
				t.Errorf("Item 0x%08X not found in GaMap after roundtrip", item.ItemID)
			}
		}
	}
}
