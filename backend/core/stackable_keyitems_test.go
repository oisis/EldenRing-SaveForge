package core

import "testing"

const godskinPrayerbookID uint32 = 0x40002299
const godskinPrayerbookHandle uint32 = 0xB0002299

func TestAddItemsToSlotBatch_SkipsStackableAlreadyInKeyItems(t *testing.T) {
	slot := &SaveSlot{
		GaMap: map[uint32]uint32{},
		Inventory: EquipInventoryData{
			CommonItems: []InventoryItem{},
			KeyItems: []InventoryItem{{
				GaItemHandle: godskinPrayerbookHandle,
				Quantity:     1,
				Index:        4029,
			}},
		},
	}

	err := AddItemsToSlotBatch(slot, []ItemToAdd{{
		ItemID:     godskinPrayerbookID,
		InvQty:     1,
		StorageQty: 1,
	}})
	if err != nil {
		t.Fatalf("AddItemsToSlotBatch: %v", err)
	}
	if len(slot.Inventory.CommonItems) != 0 {
		t.Fatalf("CommonItems len = %d, want 0", len(slot.Inventory.CommonItems))
	}
	if _, ok := slot.GaMap[godskinPrayerbookHandle]; ok {
		t.Fatalf("GaMap got handle 0x%08X for item already in KeyItems", godskinPrayerbookHandle)
	}
}

func TestCheckAddCapacity_SkipsStackableAlreadyInKeyItems(t *testing.T) {
	common := make([]InventoryItem, CommonItemCount)
	for i := range common {
		common[i] = InventoryItem{
			GaItemHandle: ItemTypeItem | uint32(i+1),
			Quantity:     1,
			Index:        uint32(i + 1),
		}
	}
	slot := &SaveSlot{
		GaMap: map[uint32]uint32{},
		Inventory: EquipInventoryData{
			CommonItems: common,
			KeyItems: []InventoryItem{{
				GaItemHandle: godskinPrayerbookHandle,
				Quantity:     1,
				Index:        4029,
			}},
		},
	}

	report := CheckAddCapacity(slot, []ItemToAdd{{
		ItemID: godskinPrayerbookID,
		InvQty: 1,
	}})
	if !report.CanFitAll {
		t.Fatalf("CanFitAll = false, CapHit=%q; item already in KeyItems should be a no-op", report.CapHit)
	}
	if report.NeededInv != 0 {
		t.Fatalf("NeededInv = %d, want 0", report.NeededInv)
	}
}
