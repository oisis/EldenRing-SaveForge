package core

import (
	"encoding/binary"
	"testing"
)

// Phase 7d.0 pre-refactor regression guards.
//
// These tests pin the offset semantics established during the spell-encoding
// audit: EquippedSpells starts at the end of inventory_held (no gap), and the
// EquippedSpellsOffset field on SaveSlot is what readSpellIDs / a future spell
// writer must use. Previously `equipedSpells` was a misnamed local pointing at
// the next section (EquippedItems), and hash.go's readSpellIDs read 14 u32s
// from there. The refactor splits the chain into clearly named offsets.

// expectedSpellsOff mirrors the formula in calculateDynamicOffsets().
func expectedSpellsOff(magicOff int) int {
	return magicOff +
		DynSpEffect +
		DynEquipedItemIndex +
		DynActiveEquipedItems +
		DynEquipedItemsID +
		DynActiveEquipedItemsGa +
		DynInventoryHeld
}

func newMinimalSlot(t *testing.T) *SaveSlot {
	t.Helper()
	data := make([]byte, SlotSize)
	magicOff := FallbackMagicBase
	copy(data[magicOff:], MagicPattern)
	return &SaveSlot{
		Data:        data,
		MagicOffset: magicOff,
		Player:      PlayerGameData{Level: 1, Class: 0},
	}
}

func TestCalculateDynamicOffsets_SetsEquippedSpellsOffset(t *testing.T) {
	slot := newMinimalSlot(t)

	if err := slot.calculateDynamicOffsets(); err != nil {
		t.Fatalf("calculateDynamicOffsets: %v", err)
	}

	want := expectedSpellsOff(slot.MagicOffset)
	if slot.EquippedSpellsOffset != want {
		t.Errorf("EquippedSpellsOffset = 0x%X, want 0x%X", slot.EquippedSpellsOffset, want)
	}

	// EquipItemsIDOffset must still precede EquippedSpellsOffset in the chain
	// (sanity: the refactor did not collapse them).
	if slot.EquipItemsIDOffset >= slot.EquippedSpellsOffset {
		t.Errorf("EquipItemsIDOffset (0x%X) should be < EquippedSpellsOffset (0x%X)",
			slot.EquipItemsIDOffset, slot.EquippedSpellsOffset)
	}
}

func TestComputeSlotHash_SpellsReadFromEquippedSpellsOffset(t *testing.T) {
	slot := newMinimalSlot(t)
	if err := slot.calculateDynamicOffsets(); err != nil {
		t.Fatalf("calculateDynamicOffsets: %v", err)
	}

	spellsOff := slot.EquippedSpellsOffset
	equipedItemsOff := spellsOff + DynEquipedSpells

	// Plant a recognizable spell pattern at the real EquippedSpells start:
	// slot 0 = Catch Flame raw MagicParam ID (0x1770) + unk 0xFFFFFFFF,
	// remaining 13 slots = empty sentinel (0xFFFFFFFF, 0x00000000).
	binary.LittleEndian.PutUint32(slot.Data[spellsOff:], 0x00001770)
	binary.LittleEndian.PutUint32(slot.Data[spellsOff+4:], 0xFFFFFFFF)
	for i := 1; i < 14; i++ {
		binary.LittleEndian.PutUint32(slot.Data[spellsOff+i*8:], 0xFFFFFFFF)
		binary.LittleEndian.PutUint32(slot.Data[spellsOff+i*8+4:], 0x00000000)
	}

	// Plant deliberate garbage at the EquippedItems region — this is where the
	// pre-refactor code read spell IDs from. If the refactor took effect,
	// hash[10] must NOT reflect this garbage.
	garbage := uint32(0xDEADBEEF)
	for i := 0; i < 14; i++ {
		binary.LittleEndian.PutUint32(slot.Data[equipedItemsOff+i*8:], garbage)
		binary.LittleEndian.PutUint32(slot.Data[equipedItemsOff+i*8+4:], garbage)
	}

	hash := ComputeSlotHash(slot)
	got := binary.LittleEndian.Uint32(hash[10*4 : 10*4+4])

	expectedIDs := []uint32{
		0x00001770,
		0xFFFFFFFF, 0xFFFFFFFF, 0xFFFFFFFF, 0xFFFFFFFF, 0xFFFFFFFF, 0xFFFFFFFF,
		0xFFFFFFFF, 0xFFFFFFFF, 0xFFFFFFFF, 0xFFFFFFFF, 0xFFFFFFFF, 0xFFFFFFFF, 0xFFFFFFFF,
	}
	want := equipmentHash(expectedIDs)
	if got != want {
		t.Errorf("hash[10] = 0x%08X, want 0x%08X (refactor regression: spell read offset incorrect)",
			got, want)
	}

	// Cross-check: ensure hash[10] did NOT match the garbage-region hash.
	garbageIDs := make([]uint32, 14)
	for i := range garbageIDs {
		garbageIDs[i] = garbage
	}
	garbageHash := equipmentHash(garbageIDs)
	if got == garbageHash {
		t.Errorf("hash[10] = 0x%08X matches the OLD buggy-offset hash; refactor did not take effect",
			got)
	}
}
