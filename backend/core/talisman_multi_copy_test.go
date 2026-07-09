package core

import (
	"os"
	"testing"
)

// Talismans are handle-encoded (no GaItem) but each copy is a distinct physical
// inventory record sharing the id-derived handle 0xA0|itemID. Adding N copies must
// produce N separate records (qty 1 each), NOT one merged record. Regression guard
// for the favorites bug where "add 4 to inventory + 4 to storage" collapsed to 1/1.
func TestTalismanMultiCopyNoMerge(t *testing.T) {
	savePath := "../../tmp/save/ER0000.sl2"
	if _, err := os.Stat(savePath); os.IsNotExist(err) {
		t.Skipf("Test save not found: %s", savePath)
	}
	save, err := LoadSave(savePath)
	if err != nil {
		t.Fatalf("LoadSave: %v", err)
	}

	var slot *SaveSlot
	for i := 0; i < 10; i++ {
		if save.Slots[i].Version != 0 {
			slot = &save.Slots[i]
			break
		}
	}
	if slot == nil {
		t.Skip("no active slot")
	}

	// Radagon's Scarseal 0x2000041A — pick an id-derived handle and count records for it.
	talismanID := uint32(0x2000041A)
	handle := (talismanID & 0x0FFFFFFF) | ItemTypeAccessory // 0xA000041A

	countRecords := func(items []InventoryItem) (n int, qtySum uint32) {
		for _, it := range items {
			if it.GaItemHandle == handle {
				n++
				qtySum += it.Quantity & 0x7FFFFFFF
			}
		}
		return
	}

	invBefore, _ := countRecords(slot.Inventory.CommonItems)
	stoBefore, _ := countRecords(slot.Storage.CommonItems)

	if err := AddItemsToSlotBatch(slot, []ItemToAdd{{
		ItemID:     talismanID,
		InvQty:     4,
		StorageQty: 4,
	}}); err != nil {
		t.Fatalf("AddItemsToSlotBatch: %v", err)
	}

	invAfter, invQty := countRecords(slot.Inventory.CommonItems)
	stoAfter, stoQty := countRecords(slot.Storage.CommonItems)

	if got := invAfter - invBefore; got != 4 {
		t.Errorf("inventory: expected 4 new talisman records, got %d", got)
	}
	if got := stoAfter - stoBefore; got != 4 {
		t.Errorf("storage: expected 4 new talisman records, got %d", got)
	}
	// Each copy must be qty 1 (no quantity stacking).
	if want := uint32(invAfter); invQty != want {
		t.Errorf("inventory: expected qty sum %d (1 per record), got %d", want, invQty)
	}
	if want := uint32(stoAfter); stoQty != want {
		t.Errorf("storage: expected qty sum %d (1 per record), got %d", want, stoQty)
	}
	// Talismans must never be backed by a GaItem record.
	for _, ga := range slot.GaItems {
		if !ga.IsEmpty() && ga.ItemID == talismanID {
			t.Errorf("talisman 0x%08X must not have a GaItem record", talismanID)
		}
	}
}
