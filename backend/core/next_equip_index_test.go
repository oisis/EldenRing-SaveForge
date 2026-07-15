package core

import (
	"encoding/binary"
	"testing"
)

// buildNextEquipFixture builds a SaveSlot with a pre-populated inventory
// (CommonItems slice + binary) and explicit NextEquipIndex / NextAcquisitionSortId.
// StorageBoxOffset is pointed outside Data so mapInventory skips storage parsing.
func buildNextEquipFixture(t *testing.T, items []InventoryItem, nextEquip, nextAcq uint32) *SaveSlot {
	t.Helper()
	magicOff := 0
	commonStart := magicOff + InvStartFromMagic
	keyStart := commonStart + CommonItemCount*InvRecordLen + InvKeyCountHeader
	nextEquipOff := keyStart + KeyItemCount*InvRecordLen
	nextAcqOff := nextEquipOff + 4
	bufSize := nextAcqOff + 4 + 64

	slot := &SaveSlot{
		Version:     1,
		MagicOffset: magicOff,
		Data:        make([]byte, bufSize),
		GaMap:       make(map[uint32]uint32),
	}

	// Full 2688-slot CommonItems slice — empty by default.
	slot.Inventory.CommonItems = make([]InventoryItem, CommonItemCount)
	for i := range slot.Inventory.CommonItems {
		slot.Inventory.CommonItems[i] = InventoryItem{GaItemHandle: GaHandleEmpty}
	}
	for i, it := range items {
		if i >= CommonItemCount {
			t.Fatalf("too many items: %d", len(items))
		}
		slot.Inventory.CommonItems[i] = it
		off := commonStart + i*InvRecordLen
		binary.LittleEndian.PutUint32(slot.Data[off:], it.GaItemHandle)
		binary.LittleEndian.PutUint32(slot.Data[off+4:], it.Quantity)
		binary.LittleEndian.PutUint32(slot.Data[off+8:], it.Index)
	}

	binary.LittleEndian.PutUint32(slot.Data[nextEquipOff:], nextEquip)
	binary.LittleEndian.PutUint32(slot.Data[nextAcqOff:], nextAcq)
	slot.Inventory.NextEquipIndex = nextEquip
	slot.Inventory.NextAcquisitionSortId = nextAcq
	slot.Inventory.nextEquipIndexOff = nextEquipOff
	slot.Inventory.nextAcqSortIdOff = nextAcqOff

	// Prevent storage parsing when mapInventory is called.
	slot.StorageBoxOffset = bufSize
	return slot
}

// buildStorageFixture builds a SaveSlot with empty storage binary and explicit counters.
// MagicOffset is pointed outside Data so mapInventory skips inventory parsing.
func buildStorageFixture(t *testing.T, nextEquip, nextAcq uint32) *SaveSlot {
	t.Helper()
	storageBoxOff := 0
	storageStart := storageBoxOff + StorageHeaderSkip
	nextEquipOff := storageStart + StorageNextEquipIdxRel
	nextAcqOff := storageStart + StorageNextAcqSortRel
	bufSize := nextAcqOff + 4 + 4

	slot := &SaveSlot{
		Version:          1,
		StorageBoxOffset: storageBoxOff,
		Data:             make([]byte, bufSize),
		GaMap:            make(map[uint32]uint32),
	}
	// All binary slots are zeroed (GaHandleEmpty=0) → all empty; in-memory list is nil.

	binary.LittleEndian.PutUint32(slot.Data[nextEquipOff:], nextEquip)
	binary.LittleEndian.PutUint32(slot.Data[nextAcqOff:], nextAcq)
	slot.Storage.NextEquipIndex = nextEquip
	slot.Storage.NextAcquisitionSortId = nextAcq
	slot.Storage.nextEquipIndexOff = nextEquipOff
	slot.Storage.nextAcqSortIdOff = nextAcqOff

	// Prevent inventory parsing when mapInventory is called.
	slot.MagicOffset = bufSize
	return slot
}

// TestNextEquipIndex_InvInsert verifies the regression fix for issue #3:
// when a new inventory item's acqIdx >= NextEquipIndex, NextEquipIndex must
// be bumped to acqIdx+1 in both struct and binary.
func TestNextEquipIndex_InvInsert(t *testing.T) {
	// Simulate genuine PC save: NextEquipIndex=500 well below NextAcquisitionSortId=1000.
	items := make([]InventoryItem, 10)
	for i := range items {
		items[i] = InventoryItem{GaItemHandle: uint32(0xB0000001 + i), Quantity: 1, Index: uint32(990 + i)}
	}
	slot := buildNextEquipFixture(t, items, 500, 1000)
	acqBefore := slot.Inventory.NextAcquisitionSortId // 1000

	const newHandle = uint32(0xB0ABCDEF)
	if err := addToInventory(slot, newHandle, 99, false, false); err != nil {
		t.Fatalf("addToInventory: %v", err)
	}

	// acqIdx=1000 >= NextEquipIndex=500 → must bump to 1001.
	wantEquip := acqBefore + 1
	if slot.Inventory.NextEquipIndex != wantEquip {
		t.Errorf("struct NextEquipIndex: got %d, want %d", slot.Inventory.NextEquipIndex, wantEquip)
	}
	rawEquip := binary.LittleEndian.Uint32(slot.Data[slot.Inventory.nextEquipIndexOff:])
	if rawEquip != wantEquip {
		t.Errorf("binary NextEquipIndex: got %d, want %d", rawEquip, wantEquip)
	}
	if slot.Inventory.NextAcquisitionSortId != acqBefore+1 {
		t.Errorf("NextAcquisitionSortId: got %d, want %d", slot.Inventory.NextAcquisitionSortId, acqBefore+1)
	}
}

// TestNextEquipIndex_StorageInsert verifies the same fix on the storage path.
func TestNextEquipIndex_StorageInsert(t *testing.T) {
	slot := buildStorageFixture(t, 500, 1000)
	acqBefore := slot.Storage.NextAcquisitionSortId // 1000

	const newHandle = uint32(0xB0ABCDEF)
	if err := addToInventory(slot, newHandle, 99, true, false); err != nil {
		t.Fatalf("addToInventory: %v", err)
	}

	// Storage uses NextAcquisitionSortId=1000 as its acquisition index.
	// 1000 >= 500 → NextEquipIndex is bumped locally to 1001.
	const wantEquip = uint32(1001)
	if slot.Storage.NextEquipIndex != wantEquip {
		t.Errorf("struct NextEquipIndex: got %d, want %d", slot.Storage.NextEquipIndex, wantEquip)
	}
	rawEquip := binary.LittleEndian.Uint32(slot.Data[slot.Storage.nextEquipIndexOff:])
	if rawEquip != wantEquip {
		t.Errorf("binary NextEquipIndex: got %d, want %d", rawEquip, wantEquip)
	}
	if slot.Storage.NextAcquisitionSortId != acqBefore+1 {
		t.Errorf("NextAcquisitionSortId: got %d, want %d", slot.Storage.NextAcquisitionSortId, acqBefore+1)
	}
}

// TestNextEquipIndex_MapInventoryNoGlobalReconcile guards against re-introducing
// CE-108255-1: mapInventory must NOT globally bump NextEquipIndex to match
// NextAcquisitionSortId on every load. Items well above NextEquipIndex must not
// trigger a write-back.
func TestNextEquipIndex_MapInventoryNoGlobalReconcile(t *testing.T) {
	items := make([]InventoryItem, 10)
	for i := range items {
		items[i] = InventoryItem{GaItemHandle: uint32(0xB0000001 + i), Quantity: 1, Index: uint32(1000 + i)}
	}
	// NextEquipIndex=50 far below item indices; NextAcquisitionSortId=1200.
	const lowEquip = uint32(50)
	slot := buildNextEquipFixture(t, items, lowEquip, 1200)

	if err := slot.mapInventory(); err != nil {
		t.Fatalf("mapInventory: %v", err)
	}

	if slot.Inventory.NextEquipIndex != lowEquip {
		t.Errorf("mapInventory changed NextEquipIndex: got %d, want %d",
			slot.Inventory.NextEquipIndex, lowEquip)
	}
	rawEquip := binary.LittleEndian.Uint32(slot.Data[slot.Inventory.nextEquipIndexOff:])
	if rawEquip != lowEquip {
		t.Errorf("mapInventory wrote NextEquipIndex to binary: got %d, want %d", rawEquip, lowEquip)
	}
}
