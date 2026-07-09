package core

import (
	"encoding/binary"
	"testing"
)

// Radagon's Scarseal — a real talisman ID (prefix 0x20 → handle prefix 0xA0).
const testTalismanID = uint32(0x2000041A)

// testTalismanHandle is the id-derived handle every copy shares.
const testTalismanHandle = (testTalismanID & 0x0FFFFFFF) | ItemTypeAccessory

// capSlot builds a minimal SaveSlot whose inventory has usedInv occupied common
// slots (so FreeInv == CommonItemCount-usedInv) and the given GaItems array (so
// FreeGaItems == count of empty entries). Storage/GaItemData are left empty.
func capSlot(usedInv int, gaItems []GaItemFull) *SaveSlot {
	common := make([]InventoryItem, usedInv)
	for i := range common {
		common[i] = InventoryItem{
			GaItemHandle: ItemTypeItem | uint32(i+1),
			Quantity:     1,
			Index:        uint32(i + 1),
		}
	}
	return &SaveSlot{
		GaMap:     map[uint32]uint32{},
		GaItems:   gaItems,
		Inventory: EquipInventoryData{CommonItems: common},
	}
}

// Test 1: a single ItemToAdd{InvQty:4} for a talisman needs 4 inventory slots and
// ZERO GaItems, so a full GaItem table must not reject it.
func TestCapacity_TalismanNeedsNoGaItem(t *testing.T) {
	slot := capSlot(CommonItemCount-4, nil) // FreeInv=4, FreeGaItems=0

	report := CheckAddCapacity(slot, []ItemToAdd{{ItemID: testTalismanID, InvQty: 4}})

	if !report.CanFitAll {
		t.Fatalf("CanFitAll=false, CapHit=%q; 4 talismans must fit in 4 free inv slots with no GaItem", report.CapHit)
	}
	if report.NeededInv != 4 {
		t.Errorf("NeededInv=%d, want 4", report.NeededInv)
	}
	if report.NeededGaItems != 0 {
		t.Errorf("NeededGaItems=%d, want 0 (talismans allocate no GaItem)", report.NeededGaItems)
	}
}

// Test 2: with only 3 free inventory slots the same request is rejected as
// inventory_full — never gaitem_full.
func TestCapacity_TalismanBoundaryInventoryFull(t *testing.T) {
	slot := capSlot(CommonItemCount-3, nil) // FreeInv=3

	report := CheckAddCapacity(slot, []ItemToAdd{{ItemID: testTalismanID, InvQty: 4}})

	if report.CanFitAll {
		t.Fatal("CanFitAll=true, want rejection: 4 talismans cannot fit in 3 free inv slots")
	}
	if report.CapHit != "inventory_full" {
		t.Errorf("CapHit=%q, want inventory_full", report.CapHit)
	}
}

// Test 3: regression — weapons still consume GaItem capacity, and ordinary
// stackable goods still collapse to one physical record.
func TestCapacity_WeaponAndStackRegression(t *testing.T) {
	const weaponID = uint32(0x001E8480) // weapon (prefix 0x00)
	const goodsID = uint32(0x40000064)  // stackable good (prefix 0x40)

	// Weapon with no free GaItem → gaitem_full even though inventory is free.
	if r := CheckAddCapacity(capSlot(0, nil), []ItemToAdd{{ItemID: weaponID, InvQty: 1}}); r.CanFitAll || r.CapHit != "gaitem_full" {
		t.Errorf("weapon w/ 0 free GaItems: CanFitAll=%v CapHit=%q, want gaitem_full", r.CanFitAll, r.CapHit)
	}

	// Weapon with a free GaItem slot → accepted, needs exactly one GaItem.
	if r := CheckAddCapacity(capSlot(0, make([]GaItemFull, 10)), []ItemToAdd{{ItemID: weaponID, InvQty: 1}}); !r.CanFitAll || r.NeededGaItems != 1 {
		t.Errorf("weapon w/ free GaItems: CanFitAll=%v NeededGaItems=%d, want true/1", r.CanFitAll, r.NeededGaItems)
	}

	// Stackable good qty 99 → one physical record, zero GaItems.
	if r := CheckAddCapacity(capSlot(0, nil), []ItemToAdd{{ItemID: goodsID, InvQty: 99}}); r.NeededInv != 1 || r.NeededGaItems != 0 {
		t.Errorf("goods qty 99: NeededInv=%d NeededGaItems=%d, want 1/0", r.NeededInv, r.NeededGaItems)
	}
}

// buildInvStorageFixture builds a SaveSlot with both a pre-allocated (empty)
// inventory and storage binary region in one Data buffer, plus valid counters,
// so AddItemsToSlotBatch can write to either destination deterministically.
func buildInvStorageFixture(t *testing.T) *SaveSlot {
	t.Helper()

	magicOff := 0
	invCommonStart := magicOff + InvStartFromMagic
	invKeyStart := invCommonStart + CommonItemCount*InvRecordLen + InvKeyCountHeader
	invNextEquipOff := invKeyStart + KeyItemCount*InvRecordLen
	invNextAcqOff := invNextEquipOff + 4

	storageBoxOff := invNextAcqOff + 8
	storageStart := storageBoxOff + StorageHeaderSkip
	stoNextEquipOff := storageStart + StorageNextEquipIdxRel
	stoNextAcqOff := storageStart + StorageNextAcqSortRel
	bufSize := stoNextAcqOff + 4 + 64

	slot := &SaveSlot{
		Version:          1,
		MagicOffset:      magicOff,
		StorageBoxOffset: storageBoxOff,
		Data:             make([]byte, bufSize),
		GaMap:            make(map[uint32]uint32),
	}

	// Fully pre-allocated 2688-slot inventory, all empty.
	slot.Inventory.CommonItems = make([]InventoryItem, CommonItemCount)
	for i := range slot.Inventory.CommonItems {
		slot.Inventory.CommonItems[i] = InventoryItem{GaItemHandle: GaHandleEmpty}
	}
	slot.Inventory.NextEquipIndex = 500
	slot.Inventory.NextAcquisitionSortId = 1000
	slot.Inventory.nextEquipIndexOff = invNextEquipOff
	slot.Inventory.nextAcqSortIdOff = invNextAcqOff
	binary.LittleEndian.PutUint32(slot.Data[invNextEquipOff:], 500)
	binary.LittleEndian.PutUint32(slot.Data[invNextAcqOff:], 1000)

	// Storage binary is all-zero (empty); in-memory list starts nil.
	slot.Storage.NextEquipIndex = 500
	slot.Storage.NextAcquisitionSortId = 1000
	slot.Storage.nextEquipIndexOff = stoNextEquipOff
	slot.Storage.nextAcqSortIdOff = stoNextAcqOff
	binary.LittleEndian.PutUint32(slot.Data[stoNextEquipOff:], 500)
	binary.LittleEndian.PutUint32(slot.Data[stoNextAcqOff:], 1000)

	return slot
}

func countTalismanRecords(items []InventoryItem) (n int, qtySum uint32) {
	for _, it := range items {
		if it.GaItemHandle == testTalismanHandle {
			n++
			qtySum += it.Quantity & 0x7FFFFFFF
		}
	}
	return
}

// Test 4: writer — one ItemToAdd{InvQty:4, StorageQty:4} produces four separate
// qty-1 records per destination and never allocates a GaItem.
func TestWriter_TalismanFourCopiesNoGaItem(t *testing.T) {
	slot := buildInvStorageFixture(t)

	if err := AddItemsToSlotBatch(slot, []ItemToAdd{{ItemID: testTalismanID, InvQty: 4, StorageQty: 4}}); err != nil {
		t.Fatalf("AddItemsToSlotBatch: %v", err)
	}

	invN, invQty := countTalismanRecords(slot.Inventory.CommonItems)
	stoN, stoQty := countTalismanRecords(slot.Storage.CommonItems)

	if invN != 4 {
		t.Errorf("inventory: got %d talisman records, want 4", invN)
	}
	if stoN != 4 {
		t.Errorf("storage: got %d talisman records, want 4", stoN)
	}
	if invQty != 4 {
		t.Errorf("inventory qty sum = %d, want 4 (1 per record)", invQty)
	}
	if stoQty != 4 {
		t.Errorf("storage qty sum = %d, want 4 (1 per record)", stoQty)
	}
	for _, ga := range slot.GaItems {
		if !ga.IsEmpty() && ga.ItemID == testTalismanID {
			t.Errorf("talisman 0x%08X must not have a GaItem record", testTalismanID)
		}
	}
}

// Test 5: production preflight+write path — the frontend adds N talismans as N
// separate ItemToAdd{InvQty:1} (repeated IDs). A full GaItem table with free
// inventory slots must NOT reject the add, and all N records must be written.
func TestProductionPath_RepeatedTalismansFullGaItemTable(t *testing.T) {
	slot := buildInvStorageFixture(t)
	slot.GaItems = nil // GaItem table full: FreeGaItems == 0

	items := make([]ItemToAdd, 4)
	for i := range items {
		items[i] = ItemToAdd{ItemID: testTalismanID, InvQty: 1}
	}

	// Preflight (app.go:addItemsToCharacter calls CheckAddCapacity before writing).
	report := CheckAddCapacity(slot, items)
	if !report.CanFitAll {
		t.Fatalf("preflight rejected talismans as %q despite 0 GaItem need and free inventory", report.CapHit)
	}
	if report.NeededGaItems != 0 {
		t.Errorf("NeededGaItems=%d, want 0", report.NeededGaItems)
	}

	if err := AddItemsToSlotBatch(slot, items); err != nil {
		t.Fatalf("AddItemsToSlotBatch: %v", err)
	}
	if n, _ := countTalismanRecords(slot.Inventory.CommonItems); n != 4 {
		t.Errorf("inventory: got %d talisman records, want 4", n)
	}
}
