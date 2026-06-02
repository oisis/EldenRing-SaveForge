package core

import (
	"bytes"
	"encoding/binary"
	"strings"
	"testing"
)

// makeSpellTestSlot builds a synthetic SaveSlot whose EquippedSpells region is
// pre-initialised to the empty-slot sentinel pattern (spell_id 0xFFFFFFFF,
// follower 0x00000000) for every slot. EquippedSpellsOffset is fixed at an
// arbitrary safe offset that leaves room for the full 14×8 block plus a
// trailing guard region used to detect out-of-bounds writes.
func makeSpellTestSlot() *SaveSlot {
	data := make([]byte, SlotSize)
	spellsOff := 0x10000
	for i := 0; i < EquippedSpellSlotCount; i++ {
		off := spellsOff + i*EquippedSpellSlotSize
		binary.LittleEndian.PutUint32(data[off:], EquippedSpellEmptySentinel)
		binary.LittleEndian.PutUint32(data[off+4:], 0x00000000)
	}
	return &SaveSlot{
		Data:                 data,
		EquippedSpellsOffset: spellsOff,
	}
}

// readSpellSlot returns the (spell_id, follower) pair stored at slotIndex.
func readSpellSlot(t *testing.T, slot *SaveSlot, slotIndex int) (uint32, uint32) {
	t.Helper()
	off := slot.EquippedSpellsOffset + slotIndex*EquippedSpellSlotSize
	return binary.LittleEndian.Uint32(slot.Data[off:]),
		binary.LittleEndian.Uint32(slot.Data[off+4:])
}

func TestPatchEquippedSpell_WriteOccupiedSlot_RawMagicParamID(t *testing.T) {
	slot := makeSpellTestSlot()

	// Catch Flame raw MagicParam ID — NOT the 0x40XXXXXX item ID form.
	const catchFlame uint32 = 0x00001770

	if err := PatchEquippedSpell(slot, 3, catchFlame); err != nil {
		t.Fatalf("PatchEquippedSpell: %v", err)
	}

	gotID, gotFollower := readSpellSlot(t, slot, 3)
	if gotID != catchFlame {
		t.Errorf("spell_id = 0x%08X, want 0x%08X", gotID, catchFlame)
	}
	if gotFollower != EquippedSpellOccupiedFollower {
		t.Errorf("follower = 0x%08X, want 0x%08X", gotFollower, EquippedSpellOccupiedFollower)
	}
}

func TestPatchEquippedSpell_WriteEmptySentinel_ClearsFollower(t *testing.T) {
	slot := makeSpellTestSlot()

	// First fill the slot, then clear it. This proves the clear path overwrites
	// the follower back to 0x00000000 rather than leaving it as 0xFFFFFFFF.
	if err := PatchEquippedSpell(slot, 5, 0x00001770); err != nil {
		t.Fatalf("seed write: %v", err)
	}
	if err := PatchEquippedSpell(slot, 5, EquippedSpellEmptySentinel); err != nil {
		t.Fatalf("clear: %v", err)
	}

	gotID, gotFollower := readSpellSlot(t, slot, 5)
	if gotID != EquippedSpellEmptySentinel {
		t.Errorf("spell_id = 0x%08X, want 0xFFFFFFFF", gotID)
	}
	if gotFollower != 0x00000000 {
		t.Errorf("follower = 0x%08X, want 0x00000000", gotFollower)
	}
}

func TestPatchEquippedSpell_FirstAndLastSlot(t *testing.T) {
	cases := []struct {
		name      string
		slotIndex int
		spellID   uint32
	}{
		{"slot 0 (first) occupied", 0, 0x00001770},
		{"slot 13 (last) occupied", 13, 0x00002328}, // arbitrary other MagicParam ID
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			slot := makeSpellTestSlot()
			if err := PatchEquippedSpell(slot, tc.slotIndex, tc.spellID); err != nil {
				t.Fatalf("PatchEquippedSpell: %v", err)
			}
			gotID, gotFollower := readSpellSlot(t, slot, tc.slotIndex)
			if gotID != tc.spellID {
				t.Errorf("spell_id = 0x%08X, want 0x%08X", gotID, tc.spellID)
			}
			if gotFollower != EquippedSpellOccupiedFollower {
				t.Errorf("follower = 0x%08X, want 0x%08X", gotFollower, EquippedSpellOccupiedFollower)
			}
		})
	}
}

func TestPatchEquippedSpell_InvalidSlotIndex_NoMutation(t *testing.T) {
	cases := []int{-1, EquippedSpellSlotCount, EquippedSpellSlotCount + 1, 9999}
	for _, idx := range cases {
		t.Run("", func(t *testing.T) {
			slot := makeSpellTestSlot()
			before := append([]byte(nil), slot.Data...)

			err := PatchEquippedSpell(slot, idx, 0x00001770)
			if err == nil {
				t.Fatalf("slotIndex %d: expected error, got nil", idx)
			}
			if !strings.Contains(err.Error(), "out of range") {
				t.Errorf("slotIndex %d: error %q does not mention 'out of range'", idx, err)
			}
			if !bytes.Equal(before, slot.Data) {
				t.Errorf("slotIndex %d: slot.Data mutated despite error", idx)
			}
		})
	}
}

func TestPatchEquippedSpell_NilSlot(t *testing.T) {
	err := PatchEquippedSpell(nil, 0, 0x00001770)
	if err == nil {
		t.Fatal("expected error for nil slot, got nil")
	}
	if !strings.Contains(err.Error(), "nil") {
		t.Errorf("error %q does not mention 'nil'", err)
	}
}

func TestPatchEquippedSpell_DataTooShort_NoMutation(t *testing.T) {
	// EquippedSpellsOffset points so close to the end of Data that the 8-byte
	// slot 13 write would run past the buffer.
	const shortLen = 200
	data := make([]byte, shortLen)
	// Sentinel-fill so we can compare bytes round-trip.
	for i := range data {
		data[i] = 0xAB
	}
	slot := &SaveSlot{
		Data:                 data,
		EquippedSpellsOffset: shortLen - 50, // leaves < 14*8 bytes of room
	}
	before := append([]byte(nil), slot.Data...)

	err := PatchEquippedSpell(slot, 13, 0x00001770)
	if err == nil {
		t.Fatal("expected error for out-of-range slot write, got nil")
	}
	if !strings.Contains(err.Error(), "exceeds Data length") {
		t.Errorf("error %q does not mention 'exceeds Data length'", err)
	}
	if !bytes.Equal(before, slot.Data) {
		t.Error("slot.Data was mutated despite out-of-range error")
	}
}

func TestPatchEquippedSpell_UninitialisedOffset(t *testing.T) {
	data := make([]byte, SlotSize)
	slot := &SaveSlot{Data: data, EquippedSpellsOffset: 0}
	before := append([]byte(nil), slot.Data...)

	err := PatchEquippedSpell(slot, 0, 0x00001770)
	if err == nil {
		t.Fatal("expected error when EquippedSpellsOffset is 0, got nil")
	}
	if !strings.Contains(err.Error(), "not initialised") {
		t.Errorf("error %q does not mention 'not initialised'", err)
	}
	if !bytes.Equal(before, slot.Data) {
		t.Error("slot.Data mutated despite uninitialised-offset error")
	}
}

func TestPatchEquippedSpell_LeavesOtherSlotsUntouched(t *testing.T) {
	slot := makeSpellTestSlot()

	// Pre-seed slots 0, 5, 13 with distinctive raw IDs so a stray write would
	// produce a detectable mismatch.
	if err := PatchEquippedSpell(slot, 0, 0x000011A1); err != nil {
		t.Fatalf("seed 0: %v", err)
	}
	if err := PatchEquippedSpell(slot, 5, 0x00001770); err != nil {
		t.Fatalf("seed 5: %v", err)
	}
	if err := PatchEquippedSpell(slot, 13, 0x00002328); err != nil {
		t.Fatalf("seed 13: %v", err)
	}

	// Snapshot the entire Data buffer EXCEPT the target slot 7 (which we will
	// overwrite). Verify bit-for-bit equality afterwards.
	targetOff := slot.EquippedSpellsOffset + 7*EquippedSpellSlotSize
	before := append([]byte(nil), slot.Data...)

	if err := PatchEquippedSpell(slot, 7, 0x00001234); err != nil {
		t.Fatalf("target write: %v", err)
	}

	for i := 0; i < len(slot.Data); i++ {
		if i >= targetOff && i < targetOff+EquippedSpellSlotSize {
			continue
		}
		if slot.Data[i] != before[i] {
			t.Fatalf("byte %d mutated outside target slot 7 (off=0x%X)", i, targetOff)
		}
	}

	gotID, gotFollower := readSpellSlot(t, slot, 7)
	if gotID != 0x00001234 || gotFollower != EquippedSpellOccupiedFollower {
		t.Errorf("slot 7 = (0x%08X, 0x%08X), want (0x00001234, 0xFFFFFFFF)", gotID, gotFollower)
	}
}

func TestPatchEquippedSpell_UsesEquippedSpellsOffset_NotLegacyChain(t *testing.T) {
	// Two slots with deliberately DIFFERENT EquippedSpellsOffset values.
	// The write must land at the EquippedSpellsOffset of each slot — proving
	// the writer consults the explicit field rather than recomputing the offset
	// chain (which would yield the same value for both, since neither slot
	// went through calculateDynamicOffsets).
	slotA := makeSpellTestSlot()
	slotB := makeSpellTestSlot()
	slotB.EquippedSpellsOffset = slotA.EquippedSpellsOffset + 0x4000

	const id uint32 = 0x00001770

	if err := PatchEquippedSpell(slotA, 2, id); err != nil {
		t.Fatalf("slotA write: %v", err)
	}
	if err := PatchEquippedSpell(slotB, 2, id); err != nil {
		t.Fatalf("slotB write: %v", err)
	}

	gotA := binary.LittleEndian.Uint32(slotA.Data[slotA.EquippedSpellsOffset+2*EquippedSpellSlotSize:])
	if gotA != id {
		t.Errorf("slotA: spell at EquippedSpellsOffset+slot*8 = 0x%08X, want 0x%08X", gotA, id)
	}
	gotB := binary.LittleEndian.Uint32(slotB.Data[slotB.EquippedSpellsOffset+2*EquippedSpellSlotSize:])
	if gotB != id {
		t.Errorf("slotB: spell at EquippedSpellsOffset+slot*8 = 0x%08X, want 0x%08X", gotB, id)
	}

	// And nothing was planted at slotA's offset inside slotB.Data.
	collision := binary.LittleEndian.Uint32(slotB.Data[slotA.EquippedSpellsOffset+2*EquippedSpellSlotSize:])
	if collision == id {
		t.Errorf("slotB: writer also wrote at slotA's offset 0x%X — would mean writer ignores EquippedSpellsOffset",
			slotA.EquippedSpellsOffset)
	}
}

func TestPatchEquippedSpell_IdempotentWriteNoOp(t *testing.T) {
	slot := makeSpellTestSlot()
	const id uint32 = 0x00001770

	if err := PatchEquippedSpell(slot, 4, id); err != nil {
		t.Fatalf("first write: %v", err)
	}
	before := append([]byte(nil), slot.Data...)

	if err := PatchEquippedSpell(slot, 4, id); err != nil {
		t.Fatalf("second write: %v", err)
	}
	if !bytes.Equal(before, slot.Data) {
		t.Error("idempotent write mutated buffer")
	}
}
