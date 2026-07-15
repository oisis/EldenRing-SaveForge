package core

import (
	"reflect"
	"testing"
)

func TestRepackGaItems_PreservesRecordsHandlesAndReferences(t *testing.T) {
	fixture := fragmentedRepackRoundTripFixture(t)
	slot := fixture.Slot
	before := CloneSlot(slot)
	beforeRecords := nonEmptyGaItemRecords(before.GaItems)
	beforeHandles := gaItemHandleSet(beforeRecords)
	beforeEquipment := append([]byte(nil), before.Data[before.EquipItemsIDOffset:before.EquipItemsIDOffset+ChrAsmEquipmentSize]...)
	beforeGaItemData := append([]byte(nil), before.Data[before.GaItemDataOffset:before.GaItemDataOffset+16]...)

	result, err := RepackGaItems(slot)
	if err != nil {
		t.Fatalf("RepackGaItems: %v", err)
	}
	if !result.Changed || result.Recovered <= 0 {
		t.Fatalf("result=%+v, want a changed repack with recovered capacity", result)
	}

	afterRecords := nonEmptyGaItemRecords(slot.GaItems)
	if !reflect.DeepEqual(afterRecords, beforeRecords) {
		t.Fatalf("non-empty GaItem records changed\n got: %#v\nwant: %#v", afterRecords, beforeRecords)
	}
	if got := gaItemHandleSet(afterRecords); !reflect.DeepEqual(got, beforeHandles) {
		t.Fatalf("GaItem handles changed\n got: %#v\nwant: %#v", got, beforeHandles)
	}
	for i := range afterRecords {
		if slot.GaItems[i] != afterRecords[i] {
			t.Fatalf("GaItem[%d]=%#v, want compacted record %#v", i, slot.GaItems[i], afterRecords[i])
		}
	}
	for i := len(afterRecords); i < len(slot.GaItems); i++ {
		if !slot.GaItems[i].IsEmpty() {
			t.Fatalf("GaItem[%d] is non-empty after compacted prefix", i)
		}
	}

	if !reflect.DeepEqual(slot.GaMap, before.GaMap) {
		t.Fatalf("GaMap changed\n got: %#v\nwant: %#v", slot.GaMap, before.GaMap)
	}
	if !reflect.DeepEqual(slot.Inventory, before.Inventory) {
		t.Fatalf("Inventory changed\n got: %#v\nwant: %#v", slot.Inventory, before.Inventory)
	}
	if !reflect.DeepEqual(slot.Storage, before.Storage) {
		t.Fatalf("Storage changed\n got: %#v\nwant: %#v", slot.Storage, before.Storage)
	}
	if got := slot.Data[slot.EquipItemsIDOffset : slot.EquipItemsIDOffset+ChrAsmEquipmentSize]; !reflect.DeepEqual(got, beforeEquipment) {
		t.Fatal("equipped item references changed")
	}
	if got := slot.Data[slot.GaItemDataOffset : slot.GaItemDataOffset+16]; !reflect.DeepEqual(got, beforeGaItemData) {
		t.Fatal("GaItemData changed")
	}

	availability := ScanAoWAvailability(slot)
	if len(availability) != 1 || availability[0].Handle != fixture.Handles.AoW || availability[0].UsedByWeaponHandle != fixture.Handles.Weapon {
		t.Fatalf("AoW availability=%+v, want the original weapon link", availability)
	}
	naked := ResolveRecord(slot, repairScopeInventoryCommon, 1, fixture.Handles.NakedHead, 1, 101)
	unarmed := ResolveRecord(slot, repairScopeInventoryCommon, 2, fixture.Handles.Unarmed, 1, 102)
	if naked.Resolution != ResolutionTechnicalPlaceholder || unarmed.Resolution != ResolutionTechnicalPlaceholder {
		t.Fatalf("placeholder resolutions = %q / %q, want both %q", naked.Resolution, unarmed.Resolution, ResolutionTechnicalPlaceholder)
	}

	if slot.NextArmamentIndex > len(afterRecords) || slot.NextAoWIndex > slot.NextArmamentIndex {
		t.Fatalf("invalid post-repack cursors: AoW=%d Armament=%d records=%d", slot.NextAoWIndex, slot.NextArmamentIndex, len(afterRecords))
	}
	if slot.NextGaItemHandle != before.NextGaItemHandle || slot.PartGaItemHandle != before.PartGaItemHandle {
		t.Fatalf("allocator handles changed: next=%d/%d part=0x%02X/0x%02X", slot.NextGaItemHandle, before.NextGaItemHandle, slot.PartGaItemHandle, before.PartGaItemHandle)
	}
}

func TestRepackGaItems_PreservesHandleEncodedStackablesWithoutGaMap(t *testing.T) {
	const (
		inventoryGoods = ItemTypeItem | 0x2738
		keyTalisman    = ItemTypeAccessory | 0x73
		storageGoods   = ItemTypeItem | 0x12C
	)

	fixture := fragmentedRepackRoundTripFixture(t)
	slot := fixture.Slot
	slot.Inventory.CommonItems[3] = InventoryItem{GaItemHandle: inventoryGoods, Quantity: 7, Index: 103}
	slot.Inventory.KeyItems[0] = InventoryItem{GaItemHandle: keyTalisman, Quantity: 1, Index: 104}
	slot.Storage.CommonItems = append(slot.Storage.CommonItems, InventoryItem{GaItemHandle: storageGoods, Quantity: 9, Index: 201})
	writeFixtureInventory(slot, slot.Inventory.CommonItems)
	writeFixtureStorage(slot, slot.Storage.CommonItems)
	if err := slot.parseFromData(); err != nil {
		t.Fatalf("parseFromData: %v", err)
	}

	for _, handle := range []uint32{inventoryGoods, keyTalisman, storageGoods} {
		if _, exists := slot.GaMap[handle]; exists {
			t.Fatalf("GaMap unexpectedly contains handle-encoded stackable 0x%08X", handle)
		}
	}
	if preflight := PreflightGaItemRepack(slot); len(preflight.Blockers) != 0 {
		t.Fatalf("preflight blockers=%+v, want none", preflight.Blockers)
	}
	before := CloneSlot(slot)

	result, err := RepackGaItems(slot)
	if err != nil {
		t.Fatalf("RepackGaItems: %v", err)
	}
	if !result.Changed || result.Recovered <= 0 {
		t.Fatalf("result=%+v, want compacting repack", result)
	}
	if !reflect.DeepEqual(slot.Inventory, before.Inventory) {
		t.Fatalf("Inventory changed\n got: %#v\nwant: %#v", slot.Inventory, before.Inventory)
	}
	if !reflect.DeepEqual(slot.Storage, before.Storage) {
		t.Fatalf("Storage changed\n got: %#v\nwant: %#v", slot.Storage, before.Storage)
	}
	for _, handle := range []uint32{inventoryGoods, keyTalisman, storageGoods} {
		if _, exists := slot.GaMap[handle]; exists {
			t.Fatalf("repack added unexpected GaMap entry for 0x%08X", handle)
		}
	}
}

func nonEmptyGaItemRecords(records []GaItemFull) []GaItemFull {
	result := make([]GaItemFull, 0, len(records))
	for _, record := range records {
		if !record.IsEmpty() {
			result = append(result, record)
		}
	}
	return result
}

func gaItemHandleSet(records []GaItemFull) map[uint32]struct{} {
	result := make(map[uint32]struct{}, len(records))
	for _, record := range records {
		result[record.Handle] = struct{}{}
	}
	return result
}
