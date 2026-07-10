package core

import (
	"encoding/binary"
	"testing"
)

// TestRemoveItemFromSlot_KeyItemHeaderPreserved is the follow-up regression for
// the removal-path twin of issue 10: RemoveItemFromSlot must locate the KeyItems
// section past the 4-byte key_count header (structures.go Read skips it). A
// missing header skip wrote the zeroed record 4 bytes early, clobbering the
// key_count header and neighbouring rows.
func TestRemoveItemFromSlot_KeyItemHeaderPreserved(t *testing.T) {
	const handle, qty, index = uint32(0xB0001FF9), uint32(3), uint32(4000) // Larval Tear
	const headerSentinel = uint32(0xDEADBEEF)

	invStart := InvStartFromMagic
	headerOff := invStart + CommonItemCount*InvRecordLen
	keyStart := headerOff + InvKeyCountHeader // canonical KeyItems row 0
	bufSize := keyStart + KeyItemCount*InvRecordLen + 64

	slot := &SaveSlot{Version: 1, GaMap: make(map[uint32]uint32)}
	slot.MagicOffset = 0
	slot.Data = make([]byte, bufSize)

	binary.LittleEndian.PutUint32(slot.Data[headerOff:], headerSentinel)
	binary.LittleEndian.PutUint32(slot.Data[keyStart:], handle)
	binary.LittleEndian.PutUint32(slot.Data[keyStart+4:], qty)
	binary.LittleEndian.PutUint32(slot.Data[keyStart+8:], index)
	slot.Inventory.KeyItems = []InventoryItem{{GaItemHandle: handle, Quantity: qty, Index: index}}

	if err := RemoveItemFromSlot(slot, handle, true, false); err != nil {
		t.Fatalf("RemoveItemFromSlot: %v", err)
	}

	if got := binary.LittleEndian.Uint32(slot.Data[headerOff:]); got != headerSentinel {
		t.Errorf("key_count header = 0x%08X, want 0x%08X (removal clobbered the header)", got, headerSentinel)
	}
	if got := binary.LittleEndian.Uint32(slot.Data[keyStart:]); got != 0 {
		t.Errorf("KeyItems row 0 handle = 0x%08X, want 0 (not zeroed at correct offset)", got)
	}
	if got := binary.LittleEndian.Uint32(slot.Data[keyStart+4:]); got != 0 {
		t.Errorf("KeyItems row 0 quantity = %d, want 0", got)
	}
	// Existing behavior rewrites the index field with the physical row number (0).
	if got := binary.LittleEndian.Uint32(slot.Data[keyStart+8:]); got != 0 {
		t.Errorf("KeyItems row 0 index = %d, want 0 (physical row)", got)
	}
	if slot.Inventory.KeyItems[0].GaItemHandle != 0 {
		t.Errorf("in-memory KeyItems[0].GaItemHandle = 0x%08X, want 0", slot.Inventory.KeyItems[0].GaItemHandle)
	}
}
