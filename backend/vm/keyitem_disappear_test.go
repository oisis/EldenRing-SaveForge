package vm

import (
	"encoding/binary"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
)

// keyStartOffset returns the canonical byte offset of the first KeyItems record
// in slot.Data (MagicOffset 0), matching structures.go Read which skips a
// 4-byte key_count header between the common and key sections.
func keyStartOffset() int {
	return core.InvStartFromMagic + core.CommonItemCount*core.InvRecordLen + core.InvKeyCountHeader
}

// buildKeyItemSlot lays out a real binary slot.Data (MagicOffset 0) holding a
// single Key Item at physical row 0 and mirrors it into slot.Inventory.KeyItems.
func buildKeyItemSlot(handle, qty, index uint32) *core.SaveSlot {
	keyStart := keyStartOffset()
	nextEquipIdxOff := keyStart + core.KeyItemCount*core.InvRecordLen
	bufSize := nextEquipIdxOff + 4 + 4 + 64

	slot := &core.SaveSlot{Version: 1, GaMap: make(map[uint32]uint32)}
	slot.MagicOffset = 0
	slot.Data = make([]byte, bufSize)

	binary.LittleEndian.PutUint32(slot.Data[keyStart:], handle)
	binary.LittleEndian.PutUint32(slot.Data[keyStart+4:], qty)
	binary.LittleEndian.PutUint32(slot.Data[keyStart+8:], index)
	slot.Inventory.KeyItems = []core.InventoryItem{{GaItemHandle: handle, Quantity: qty, Index: index}}
	return slot
}

// TestApplyVMToParsedSlot_KeyItemHandleSurvives is the issue-10 regression:
// SaveCharacter -> ApplyVMToParsedSlot -> updateItemsAndSync writes a Key Item's
// new quantity back into slot.Data. The write must land on the quantity field,
// NOT the handle field. A missing key_count-header skip put the write 4 bytes
// early, overwriting the handle with the quantity value; on the next parse the
// corrupted handle no longer resolved to an item, so the Key Item "disappeared".
func TestApplyVMToParsedSlot_KeyItemHandleSurvives(t *testing.T) {
	const handle, oldQty, index = uint32(0xB0001FF9), uint32(1), uint32(4000) // Larval Tear
	const newQty = uint32(5)
	slot := buildKeyItemSlot(handle, oldQty, index)

	charVM := &CharacterViewModel{
		Inventory: []ItemViewModel{
			{Handle: handle, Quantity: newQty, MaxInventory: 99},
		},
	}
	if err := ApplyVMToParsedSlot(charVM, slot); err != nil {
		t.Fatalf("ApplyVMToParsedSlot: %v", err)
	}

	keyStart := keyStartOffset()
	gotHandle := binary.LittleEndian.Uint32(slot.Data[keyStart:])
	gotQty := binary.LittleEndian.Uint32(slot.Data[keyStart+4:])

	if gotHandle != handle {
		t.Errorf("KeyItems row 0 handle in slot.Data = 0x%08X, want 0x%08X (handle corrupted — item would vanish on reload)", gotHandle, handle)
	}
	if gotQty != newQty {
		t.Errorf("KeyItems row 0 quantity in slot.Data = %d, want %d (quantity edit not applied to correct field)", gotQty, newQty)
	}
}
