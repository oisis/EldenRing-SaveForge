package main

import (
	"encoding/binary"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
)

const endpointTalismanID = uint32(0x2000041A) // Radagon's Scarseal

func appTalismanCapacityFixture(t *testing.T) *App {
	t.Helper()

	const magicOff = 16
	commonStart := magicOff + core.InvStartFromMagic
	keyStart := commonStart + core.CommonItemCount*core.InvRecordLen + core.InvKeyCountHeader
	nextEquipOff := keyStart + core.KeyItemCount*core.InvRecordLen
	nextAcqOff := nextEquipOff + 4
	bufSize := nextAcqOff + 4 + 64

	app := NewApp()
	app.save = &core.SaveFile{}
	slot := &app.save.Slots[0]
	slot.Version = 1
	slot.MagicOffset = magicOff
	slot.StorageBoxOffset = bufSize // intentionally out of range: this test adds inventory only
	slot.Data = make([]byte, bufSize)
	slot.GaMap = make(map[uint32]uint32)

	slot.Inventory.CommonItems = make([]core.InventoryItem, core.CommonItemCount)
	for i := range slot.Inventory.CommonItems {
		slot.Inventory.CommonItems[i] = core.InventoryItem{GaItemHandle: core.GaHandleEmpty}
	}
	slot.Inventory.NextEquipIndex = 500
	slot.Inventory.NextAcquisitionSortId = 1000
	binary.LittleEndian.PutUint32(slot.Data[nextEquipOff:], slot.Inventory.NextEquipIndex)
	binary.LittleEndian.PutUint32(slot.Data[nextAcqOff:], slot.Inventory.NextAcquisitionSortId)

	// One occupied entry in a one-entry table gives the production preflight zero
	// free GaItems while inventory has ample free physical slots. Talismans must
	// still be accepted because their writer path allocates no serialized GaItem.
	slot.GaItems = []core.GaItemFull{{Handle: 0x80000001, ItemID: 1}}
	return app
}

// TestAddItemsToCharacter_RepeatedTalismansNeedNoGaItem exercises the actual
// public app endpoint used by DatabaseTab: the UI sends N repeated talisman IDs
// with invQty=1. A full GaItem table must not produce a false gaitem_full result.
func TestAddItemsToCharacter_RepeatedTalismansNeedNoGaItem(t *testing.T) {
	app := appTalismanCapacityFixture(t)
	ids := []uint32{endpointTalismanID, endpointTalismanID, endpointTalismanID, endpointTalismanID}

	result, err := app.AddItemsToCharacter(0, ids, 0, 0, 0, 0, 1, 0)
	if err != nil {
		t.Fatalf("AddItemsToCharacter: %v", err)
	}
	if result.CapHit != "" {
		t.Fatalf("CapHit=%q, want successful add with no GaItem requirement", result.CapHit)
	}
	if result.Added != len(ids) {
		t.Fatalf("Added=%d, want %d", result.Added, len(ids))
	}

	handle := (endpointTalismanID & 0x0FFFFFFF) | core.ItemTypeAccessory
	records := 0
	for _, item := range app.save.Slots[0].Inventory.CommonItems {
		if item.GaItemHandle != handle {
			continue
		}
		records++
		if item.Quantity&0x7FFFFFFF != 1 {
			t.Errorf("talisman record quantity=%d, want 1", item.Quantity&0x7FFFFFFF)
		}
	}
	if records != len(ids) {
		t.Errorf("talisman records=%d, want %d", records, len(ids))
	}
	if got := len(app.save.Slots[0].GaItems); got != 1 {
		t.Errorf("GaItems length=%d, want unchanged full table length 1", got)
	}
	if got := len(app.undoStacks[0]); got != 1 {
		t.Errorf("undo depth=%d, want 1 after successful endpoint mutation", got)
	}
}
