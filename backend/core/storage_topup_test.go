package core

import (
	"encoding/binary"
	"testing"
)

// TestAddToInventory_StorageTopupRespectsBinaryGaps is a regression test for the
// storage quantity-topup bug. Storage CommonItems is a COMPACTED slice (empty
// binary slots are skipped on load — see spec/10-storage.md), so a slice index
// does NOT equal the binary record position. The old code computed the write
// offset as startOffset + sliceIndex*InvRecordLen, which — when the binary array
// has gaps — wrote the new quantity to the WRONG record (a neighbour), silently
// corrupting it while leaving the real record unchanged. After reload the topup
// appeared to do nothing.
//
// Layout below puts a gap at binary slot 0, so the target item B lives at binary
// slot 2 but compacted slice index 1. The fix must locate B by scanning slot.Data
// for its handle, not by slice index.
func TestAddToInventory_StorageTopupRespectsBinaryGaps(t *testing.T) {
	const (
		storageBoxOff = 64
		handleA       = uint32(0xC0000010) // neighbour at binary slot 1
		handleB       = uint32(0xC0000020) // target at binary slot 2
	)
	startOffset := storageBoxOff + StorageHeaderSkip
	bufSize := startOffset + StorageCommonCount*InvRecordLen + 64

	slot := &SaveSlot{
		Version:          1,
		Data:             make([]byte, bufSize),
		StorageBoxOffset: storageBoxOff,
		GaMap:            make(map[uint32]uint32),
	}

	writeRec := func(binPos int, handle, qty, index uint32) {
		off := startOffset + binPos*InvRecordLen
		binary.LittleEndian.PutUint32(slot.Data[off:], handle)
		binary.LittleEndian.PutUint32(slot.Data[off+4:], qty)
		binary.LittleEndian.PutUint32(slot.Data[off+8:], index)
	}
	// Binary slot 0 = empty gap (handle stays GaHandleEmpty / 0).
	writeRec(1, handleA, 5, 1000)
	writeRec(2, handleB, 1, 1001)

	// In-memory CommonItems is COMPACTED: the gap at binary slot 0 is dropped,
	// so A is at slice index 0 and B is at slice index 1 (binary position 2).
	slot.Storage.CommonItems = []InventoryItem{
		{GaItemHandle: handleA, Quantity: 5, Index: 1000},
		{GaItemHandle: handleB, Quantity: 1, Index: 1001},
	}

	// Top up B to 99. With the bug this would write to binary slot 1 (handle A).
	if err := addToInventory(slot, handleB, 99, true, false, false); err != nil {
		t.Fatalf("addToInventory: %v", err)
	}

	readQty := func(binPos int) uint32 {
		off := startOffset + binPos*InvRecordLen + 4
		return binary.LittleEndian.Uint32(slot.Data[off:])
	}
	readHandle := func(binPos int) uint32 {
		off := startOffset + binPos*InvRecordLen
		return binary.LittleEndian.Uint32(slot.Data[off:])
	}

	// Target B (binary slot 2) must now hold qty 99.
	if got := readQty(2); got != 99 {
		t.Errorf("target record (binary slot 2) qty: want 99, got %d", got)
	}
	if got := readHandle(2); got != handleB {
		t.Errorf("target handle changed: want 0x%08X, got 0x%08X", handleB, got)
	}
	// Neighbour A (binary slot 1) must be untouched — the bug corrupted it.
	if got := readQty(1); got != 5 {
		t.Errorf("neighbour record (binary slot 1) corrupted: want qty 5, got %d", got)
	}
	if got := readHandle(1); got != handleA {
		t.Errorf("neighbour handle changed: want 0x%08X, got 0x%08X", handleA, got)
	}
}
