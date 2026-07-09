package core

import (
	"encoding/binary"
	"testing"
)

// buildStorageFixtureRecords builds a slot whose storage box holds a full
// StorageCommonCount physical array. records are written at the given PHYSICAL
// slot positions (allowing sparse gaps); the compacted in-memory list is built
// in physical order, mirroring a real ReadStorage. The header count is seeded
// correctly.
func buildStorageFixtureRecords(t *testing.T, records map[int]InventoryItem) *SaveSlot {
	t.Helper()
	const storageBoxOff = 8 // non-zero: MagicOffset/StorageBoxOffset == 0 means "absent"
	storageStart := storageBoxOff + StorageHeaderSkip
	bufSize := storageStart + StorageCommonCount*InvRecordLen + 4

	slot := &SaveSlot{
		Version:          1,
		StorageBoxOffset: storageBoxOff,
		Data:             make([]byte, bufSize),
		GaMap:            make(map[uint32]uint32),
	}
	// Point MagicOffset past Data so inventory binary is not parsed on re-scan.
	slot.MagicOffset = bufSize

	count := uint32(0)
	for physical, it := range records {
		off := storageStart + physical*InvRecordLen
		binary.LittleEndian.PutUint32(slot.Data[off:], it.GaItemHandle)
		binary.LittleEndian.PutUint32(slot.Data[off+4:], it.Quantity)
		binary.LittleEndian.PutUint32(slot.Data[off+8:], it.Index)
		count++
	}
	binary.LittleEndian.PutUint32(slot.Data[storageBoxOff:], count)

	// Build compacted in-memory list in physical order (ReadStorage behaviour).
	for i := 0; i < StorageCommonCount; i++ {
		if it, ok := records[i]; ok {
			slot.Storage.CommonItems = append(slot.Storage.CommonItems, it)
		}
	}
	return slot
}

// buildInvFixtureNZ builds an inventory-only slot with a NON-ZERO MagicOffset so
// ReconcileInventoryHeader (which treats MagicOffset<=0 as "absent") actually runs.
func buildInvFixtureNZ(t *testing.T, common []InventoryItem) *SaveSlot {
	t.Helper()
	const magicOff = 16
	commonStart := magicOff + InvStartFromMagic
	keyStart := commonStart + CommonItemCount*InvRecordLen + InvKeyCountHeader
	bufSize := keyStart + KeyItemCount*InvRecordLen + 8

	slot := &SaveSlot{
		Version:     1,
		MagicOffset: magicOff,
		Data:        make([]byte, bufSize),
		GaMap:       make(map[uint32]uint32),
	}
	for i, it := range common {
		off := commonStart + i*InvRecordLen
		binary.LittleEndian.PutUint32(slot.Data[off:], it.GaItemHandle)
		binary.LittleEndian.PutUint32(slot.Data[off+4:], it.Quantity)
		binary.LittleEndian.PutUint32(slot.Data[off+8:], it.Index)
	}
	slot.Inventory.CommonItems = append([]InventoryItem(nil), common...)
	binary.LittleEndian.PutUint32(slot.Data[commonStart-4:], uint32(len(common)))
	return slot
}

func storageHeaderCount(slot *SaveSlot) uint32 {
	return binary.LittleEndian.Uint32(slot.Data[slot.StorageBoxOffset:])
}

func rawInvRecord(t *testing.T, slot *SaveSlot, blockStart, row int) InventoryItem {
	t.Helper()
	off := blockStart + row*InvRecordLen
	return InventoryItem{
		GaItemHandle: binary.LittleEndian.Uint32(slot.Data[off:]),
		Quantity:     binary.LittleEndian.Uint32(slot.Data[off+4:]),
		Index:        binary.LittleEndian.Uint32(slot.Data[off+8:]),
	}
}

// ---- inventory: only the targeted row is removed ---------------------------

func TestRemoveInventoryRecordAt_InventoryTargetsSingleRow(t *testing.T) {
	// Two records sharing the same talisman handle (0xA0) at rows 0 and 1,
	// distinct acquisition indices — a legal duplicate talisman scenario.
	const handle = 0xA00017B6
	slot := buildInvFixtureNZ(t, []InventoryItem{
		{GaItemHandle: handle, Quantity: 1, Index: 500},
		{GaItemHandle: handle, Quantity: 1, Index: 600},
	})
	slot.GaMap[handle] = 0x200017B6

	commonStart := slot.MagicOffset + InvStartFromMagic
	fp := fingerprintInventoryItem(slot.Inventory.CommonItems[0])

	if err := RemoveInventoryRecordAt(slot, repairScopeInventoryCommon, 0, fp); err != nil {
		t.Fatalf("RemoveInventoryRecordAt: %v", err)
	}

	// Row 0 cleared in binary and in-memory.
	if got := rawInvRecord(t, slot, commonStart, 0); got.GaItemHandle != 0 || got.Quantity != 0 {
		t.Fatalf("row 0 not cleared in binary: %+v", got)
	}
	if slot.Inventory.CommonItems[0].GaItemHandle != 0 {
		t.Fatalf("row 0 not cleared in memory: %+v", slot.Inventory.CommonItems[0])
	}
	// Row 1 (same handle) untouched — the whole point of position-based removal.
	if got := rawInvRecord(t, slot, commonStart, 1); got.GaItemHandle != handle || got.Index != 600 {
		t.Fatalf("row 1 (duplicate handle) was disturbed: %+v", got)
	}
	// GaMap entry preserved — row 1 still uses the handle.
	if _, ok := slot.GaMap[handle]; !ok {
		t.Fatalf("GaMap entry for shared handle was GC'd")
	}
	// Header reconciled to the one remaining non-empty record.
	invStart := slot.MagicOffset + InvStartFromMagic
	if got := binary.LittleEndian.Uint32(slot.Data[invStart-4:]); got != 1 {
		t.Fatalf("inventory header count = %d, want 1", got)
	}
}

func TestRemoveInventoryRecordAt_FingerprintMismatchRefuses(t *testing.T) {
	slot := buildInvFixtureNZ(t, []InventoryItem{
		{GaItemHandle: 0xA00017B6, Quantity: 1, Index: 500},
	})
	if err := RemoveInventoryRecordAt(slot, repairScopeInventoryCommon, 0, "deadbeefdeadbeef"); err == nil {
		t.Fatal("expected fingerprint-mismatch error, got nil")
	}
	// No mutation on refusal.
	if slot.Inventory.CommonItems[0].GaItemHandle != 0xA00017B6 {
		t.Fatalf("record was mutated despite stale fingerprint: %+v", slot.Inventory.CommonItems[0])
	}
}

// ---- storage: header reconciled, only targeted physical row removed --------

func TestRemoveInventoryRecordAt_StorageHeaderCorrectAndSingleRow(t *testing.T) {
	const dupHandle = 0xA00017B6 // talisman, appears twice
	// Sparse physical layout: gap at slot 1, duplicate handle at physical 0 and 3.
	slot := buildStorageFixtureRecords(t, map[int]InventoryItem{
		0: {GaItemHandle: dupHandle, Quantity: 1, Index: 700},
		2: {GaItemHandle: 0x80001111, Quantity: 1, Index: 710},
		3: {GaItemHandle: dupHandle, Quantity: 1, Index: 720},
	})
	// Compacted in-memory order: [phys0, phys2, phys3] → rows 0,1,2.
	if len(slot.Storage.CommonItems) != 3 {
		t.Fatalf("setup: expected 3 compacted records, got %d", len(slot.Storage.CommonItems))
	}

	// Remove compacted row 2 (physical slot 3, the SECOND copy of dupHandle).
	fp := fingerprintInventoryItem(slot.Storage.CommonItems[2])
	if err := RemoveInventoryRecordAt(slot, repairScopeStorageCommon, 2, fp); err != nil {
		t.Fatalf("RemoveInventoryRecordAt: %v", err)
	}

	storageStart := slot.StorageBoxOffset + StorageHeaderSkip
	// Physical slot 3 cleared.
	if got := binary.LittleEndian.Uint32(slot.Data[storageStart+3*InvRecordLen:]); got != 0 {
		t.Fatalf("physical slot 3 not cleared: 0x%08X", got)
	}
	// Physical slot 0 (first copy, same handle) untouched.
	if got := binary.LittleEndian.Uint32(slot.Data[storageStart+0*InvRecordLen:]); got != dupHandle {
		t.Fatalf("physical slot 0 (first copy) was disturbed: 0x%08X", got)
	}
	// In-memory list compacted down to the two survivors.
	if len(slot.Storage.CommonItems) != 2 {
		t.Fatalf("in-memory list len = %d, want 2", len(slot.Storage.CommonItems))
	}
	if slot.Storage.CommonItems[0].Index != 700 || slot.Storage.CommonItems[1].Index != 710 {
		t.Fatalf("wrong survivors: %+v", slot.Storage.CommonItems)
	}
	// Header reconciled to the actual physical non-empty count (2).
	if got := storageHeaderCount(slot); got != 2 {
		t.Fatalf("storage header count = %d, want 2", got)
	}
}

func TestRemoveInventoryRecordAt_StorageDropsFirstCopyOnly(t *testing.T) {
	const dupHandle = 0xA00017B6
	slot := buildStorageFixtureRecords(t, map[int]InventoryItem{
		0: {GaItemHandle: dupHandle, Quantity: 1, Index: 700},
		1: {GaItemHandle: dupHandle, Quantity: 1, Index: 720},
	})
	// Remove compacted row 0 (physical 0) — the FIRST copy.
	fp := fingerprintInventoryItem(slot.Storage.CommonItems[0])
	if err := RemoveInventoryRecordAt(slot, repairScopeStorageCommon, 0, fp); err != nil {
		t.Fatalf("RemoveInventoryRecordAt: %v", err)
	}
	storageStart := slot.StorageBoxOffset + StorageHeaderSkip
	if got := binary.LittleEndian.Uint32(slot.Data[storageStart:]); got != 0 {
		t.Fatalf("physical slot 0 not cleared: 0x%08X", got)
	}
	// Second copy at physical slot 1 survives.
	if got := binary.LittleEndian.Uint32(slot.Data[storageStart+InvRecordLen:]); got != dupHandle {
		t.Fatalf("second copy at physical slot 1 removed: 0x%08X", got)
	}
	if len(slot.Storage.CommonItems) != 1 || slot.Storage.CommonItems[0].Index != 720 {
		t.Fatalf("wrong survivor: %+v", slot.Storage.CommonItems)
	}
	if got := storageHeaderCount(slot); got != 1 {
		t.Fatalf("storage header count = %d, want 1", got)
	}
}
