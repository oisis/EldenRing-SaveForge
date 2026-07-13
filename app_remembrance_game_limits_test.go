package main

import (
	"encoding/binary"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
)

// remembranceGameLimitsFixture provides both pre-allocated common-item
// containers, including their binary backing regions. It is deliberately
// fixture-free: the test verifies the public add path without a real save.
func remembranceGameLimitsFixture() *App {
	const magicOffset = 16
	invStart := magicOffset + core.InvStartFromMagic
	invKeyStart := invStart + core.CommonItemCount*core.InvRecordLen + core.InvKeyCountHeader
	invNextEquipOff := invKeyStart + core.KeyItemCount*core.InvRecordLen
	invNextAcqOff := invNextEquipOff + 4
	storageBoxOff := invNextAcqOff + 4 + 16
	storageStart := storageBoxOff + core.StorageHeaderSkip
	storageNextEquipOff := storageStart + core.StorageNextEquipIdxRel
	storageNextAcqOff := storageStart + core.StorageNextAcqSortRel

	app := NewApp()
	app.save = &core.SaveFile{}
	slot := &app.save.Slots[0]
	slot.Version = 1
	slot.MagicOffset = magicOffset
	slot.StorageBoxOffset = storageBoxOff
	slot.Data = make([]byte, storageNextAcqOff+4+64)
	slot.GaMap = make(map[uint32]uint32)

	slot.Inventory.CommonItems = make([]core.InventoryItem, core.CommonItemCount)
	for i := range slot.Inventory.CommonItems {
		slot.Inventory.CommonItems[i].GaItemHandle = core.GaHandleEmpty
	}
	slot.Inventory.NextEquipIndex = 500
	slot.Inventory.NextAcquisitionSortId = 1000
	binary.LittleEndian.PutUint32(slot.Data[invNextEquipOff:], slot.Inventory.NextEquipIndex)
	binary.LittleEndian.PutUint32(slot.Data[invNextAcqOff:], slot.Inventory.NextAcquisitionSortId)

	slot.Storage.NextEquipIndex = 500
	slot.Storage.NextAcquisitionSortId = 1000
	binary.LittleEndian.PutUint32(slot.Data[storageNextEquipOff:], slot.Storage.NextEquipIndex)
	binary.LittleEndian.PutUint32(slot.Data[storageNextAcqOff:], slot.Storage.NextAcquisitionSortId)
	return app
}

func quantityInRecords(data []byte, start, count int, handle uint32) (uint32, bool) {
	for i := 0; i < count; i++ {
		off := start + i*core.InvRecordLen
		if binary.LittleEndian.Uint32(data[off:]) == handle {
			return binary.LittleEndian.Uint32(data[off+4:]), true
		}
	}
	return 0, false
}

func TestAddRemembranceWithGameLimits_WritesFullQuantities(t *testing.T) {
	const (
		remembranceID     = uint32(0x40000B86)
		remembranceHandle = uint32(0xB0000B86)
		wantInventory     = uint32(99)
		wantStorage       = uint32(600)
	)
	app := remembranceGameLimitsFixture()
	slot := &app.save.Slots[0]

	result, err := app.AddItemsToCharacterWithGameLimits(0, []uint32{remembranceID}, 0, 0, 0, 0, -1, -1)
	if err != nil {
		t.Fatalf("AddItemsToCharacterWithGameLimits: %v", err)
	}
	if result.Added != 1 || result.CapHit != "" {
		t.Fatalf("AddResult = %+v, want one successful add", result)
	}

	invStart := slot.MagicOffset + core.InvStartFromMagic
	if got, ok := quantityInRecords(slot.Data, invStart, core.CommonItemCount, remembranceHandle); !ok || got != wantInventory {
		t.Fatalf("inventory binary quantity = %d (found=%v), want %d", got, ok, wantInventory)
	}
	storageStart := slot.StorageBoxOffset + core.StorageHeaderSkip
	if got, ok := quantityInRecords(slot.Data, storageStart, core.StorageCommonCount, remembranceHandle); !ok || got != wantStorage {
		t.Fatalf("storage binary quantity = %d (found=%v), want %d", got, ok, wantStorage)
	}
}
