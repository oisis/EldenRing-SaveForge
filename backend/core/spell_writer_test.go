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

// ─── WriteSpells (batch + hash[10] recompute) ───────────────────────────

// makeCalibratedSpellTestSlot builds a SaveSlot whose dynamic offset
// chain has been calculated against the real MagicPattern anchor — the
// only configuration in which slot.EquippedSpellsOffset equals the
// spellsOff that ComputeSlotHash recomputes for hash entry [10].
//
// Tests that assert hash[10] consistency MUST use this helper rather
// than the simpler makeSpellTestSlot, which pins an arbitrary offset
// purely for per-write unit tests.
func makeCalibratedSpellTestSlot(t *testing.T) *SaveSlot {
	t.Helper()
	data := make([]byte, SlotSize)
	copy(data[FallbackMagicBase:], MagicPattern)
	slot := &SaveSlot{
		Data:        data,
		MagicOffset: FallbackMagicBase,
		Player:      PlayerGameData{Level: 1, Class: 0},
	}
	if err := slot.calculateDynamicOffsets(); err != nil {
		t.Fatalf("calculateDynamicOffsets: %v", err)
	}
	// Initialise every slot to the empty-slot sentinel so writes start
	// from a known-good baseline.
	for i := 0; i < EquippedSpellSlotCount; i++ {
		off := slot.EquippedSpellsOffset + i*EquippedSpellSlotSize
		binary.LittleEndian.PutUint32(slot.Data[off:], EquippedSpellEmptySentinel)
		binary.LittleEndian.PutUint32(slot.Data[off+4:], 0x00000000)
	}
	return slot
}

// readHashEntry reads the u32 hash entry at index (0..15) from the
// slot's hash block. Provides a stable assertion helper for the
// "hash[10] is touched, sibling entries are not" tests below.
func readHashEntry(t *testing.T, slot *SaveSlot, idx int) uint32 {
	t.Helper()
	return binary.LittleEndian.Uint32(slot.Data[HashOffset+idx*4:])
}

func TestWriteSpells_OccupiedSpell_UpdatesHash10ToMatchComputeSlotHash(t *testing.T) {
	slot := makeCalibratedSpellTestSlot(t)

	if err := slot.WriteSpells([]SpellWrite{{SlotIndex: 3, SpellID: 0x00001770}}); err != nil {
		t.Fatalf("WriteSpells: %v", err)
	}

	// Slot bytes match the per-write expectation.
	gotID, gotFollower := readSpellSlot(t, slot, 3)
	if gotID != 0x00001770 || gotFollower != EquippedSpellOccupiedFollower {
		t.Errorf("slot 3 = (0x%08X, 0x%08X), want (0x00001770, 0xFFFFFFFF)", gotID, gotFollower)
	}

	// hash[10] in slot.Data is bit-equivalent to ComputeSlotHash[10].
	got := readHashEntry(t, slot, 10)
	full := ComputeSlotHash(slot)
	want := binary.LittleEndian.Uint32(full[10*4 : 10*4+4])
	if got != want {
		t.Errorf("hash[10] = 0x%08X, want 0x%08X (drifted from ComputeSlotHash)", got, want)
	}
}

func TestWriteSpells_ClearOccupiedSlot_RecomputesHash10(t *testing.T) {
	slot := makeCalibratedSpellTestSlot(t)

	// Seed an occupied slot via WriteSpells so hash[10] reflects that
	// state.
	if err := slot.WriteSpells([]SpellWrite{{SlotIndex: 0, SpellID: 0x00001770}}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	seededHash := readHashEntry(t, slot, 10)

	// Now clear it.
	if err := slot.WriteSpells([]SpellWrite{{SlotIndex: 0, SpellID: EquippedSpellEmptySentinel}}); err != nil {
		t.Fatalf("clear: %v", err)
	}
	clearedHash := readHashEntry(t, slot, 10)

	if seededHash == clearedHash {
		t.Error("hash[10] unchanged after clear: expected a different value than the seeded one")
	}

	// And the cleared hash must match ComputeSlotHash on the
	// current state.
	full := ComputeSlotHash(slot)
	want := binary.LittleEndian.Uint32(full[10*4 : 10*4+4])
	if clearedHash != want {
		t.Errorf("hash[10] = 0x%08X, want 0x%08X (clear path drifted)", clearedHash, want)
	}
}

func TestWriteSpells_DoesNotTouchOtherHashEntries(t *testing.T) {
	slot := makeSpellTestSlot()

	// Plant distinctive sentinels in adjacent hash entries so any
	// stray mutation is loud.
	for _, idx := range []int{0, 5, 7, 8, 9, 11, 12, 15} {
		binary.LittleEndian.PutUint32(slot.Data[HashOffset+idx*4:], 0xDEADBEEF)
	}

	if err := slot.WriteSpells([]SpellWrite{{SlotIndex: 7, SpellID: 0x00002328}}); err != nil {
		t.Fatalf("WriteSpells: %v", err)
	}

	for _, idx := range []int{0, 5, 7, 8, 9, 11, 12, 15} {
		got := readHashEntry(t, slot, idx)
		if got != 0xDEADBEEF {
			t.Errorf("hash[%d] = 0x%08X, want 0xDEADBEEF (WriteSpells must only touch hash[10])", idx, got)
		}
	}
}

func TestWriteSpells_BatchMultipleWrites(t *testing.T) {
	slot := makeCalibratedSpellTestSlot(t)
	writes := []SpellWrite{
		{SlotIndex: 0, SpellID: 0x00001770},
		{SlotIndex: 5, SpellID: 0x000011A1},
		{SlotIndex: 13, SpellID: 0x00002328},
		{SlotIndex: 7, SpellID: EquippedSpellEmptySentinel}, // explicit clear in the middle
	}
	if err := slot.WriteSpells(writes); err != nil {
		t.Fatalf("WriteSpells: %v", err)
	}

	for _, w := range writes {
		gotID, gotFollower := readSpellSlot(t, slot, w.SlotIndex)
		var wantID, wantFollower uint32
		if w.SpellID == EquippedSpellEmptySentinel {
			wantID, wantFollower = EquippedSpellEmptySentinel, 0
		} else {
			wantID, wantFollower = w.SpellID, EquippedSpellOccupiedFollower
		}
		if gotID != wantID || gotFollower != wantFollower {
			t.Errorf("slot %d = (0x%08X, 0x%08X), want (0x%08X, 0x%08X)", w.SlotIndex, gotID, gotFollower, wantID, wantFollower)
		}
	}

	// hash[10] consistent with full recompute.
	got := readHashEntry(t, slot, 10)
	full := ComputeSlotHash(slot)
	want := binary.LittleEndian.Uint32(full[10*4 : 10*4+4])
	if got != want {
		t.Errorf("hash[10] = 0x%08X, want 0x%08X", got, want)
	}
}

func TestWriteSpells_EmptyBatchIsNoOp(t *testing.T) {
	slot := makeSpellTestSlot()
	// Plant a sentinel in hash[10] so we can detect a stray recompute
	// over an empty input.
	binary.LittleEndian.PutUint32(slot.Data[HashOffset+10*4:], 0xCAFEBABE)
	before := append([]byte(nil), slot.Data...)

	if err := slot.WriteSpells(nil); err != nil {
		t.Errorf("WriteSpells(nil): %v", err)
	}
	if err := slot.WriteSpells([]SpellWrite{}); err != nil {
		t.Errorf("WriteSpells(empty): %v", err)
	}

	if !bytes.Equal(before, slot.Data) {
		t.Error("empty batch mutated slot.Data")
	}
	if got := readHashEntry(t, slot, 10); got != 0xCAFEBABE {
		t.Errorf("hash[10] = 0x%08X, want 0xCAFEBABE (empty batch should not recompute)", got)
	}
}

func TestWriteSpells_InvalidSlotIndex_NoMutation_NoHashChange(t *testing.T) {
	cases := []int{-1, EquippedSpellSlotCount, EquippedSpellSlotCount + 5, 99}
	for _, badIdx := range cases {
		t.Run("", func(t *testing.T) {
			slot := makeSpellTestSlot()
			// Plant a sentinel hash[10] so any inadvertent recompute is visible.
			binary.LittleEndian.PutUint32(slot.Data[HashOffset+10*4:], 0xCAFEBABE)
			before := append([]byte(nil), slot.Data...)

			err := slot.WriteSpells([]SpellWrite{
				{SlotIndex: 0, SpellID: 0x00001770}, // valid
				{SlotIndex: badIdx, SpellID: 0x00001234}, // poison pill
			})
			if err == nil {
				t.Fatalf("slotIndex %d: expected pre-validation error, got nil", badIdx)
			}
			if !strings.Contains(err.Error(), "out of range") {
				t.Errorf("error %q does not mention 'out of range'", err)
			}
			if !bytes.Equal(before, slot.Data) {
				t.Errorf("slotIndex %d: pre-validation failure must NOT mutate slot.Data (atomicity)", badIdx)
			}
		})
	}
}

func TestWriteSpells_DuplicateSlotIndex_Rejected(t *testing.T) {
	slot := makeSpellTestSlot()
	before := append([]byte(nil), slot.Data...)

	err := slot.WriteSpells([]SpellWrite{
		{SlotIndex: 4, SpellID: 0x00001770},
		{SlotIndex: 4, SpellID: 0x000011A1},
	})
	if err == nil {
		t.Fatal("expected duplicate-index error, got nil")
	}
	if !strings.Contains(err.Error(), "already written") {
		t.Errorf("error %q does not mention 'already written'", err)
	}
	if !bytes.Equal(before, slot.Data) {
		t.Error("duplicate-index failure must not mutate slot.Data")
	}
}

func TestWriteSpells_NilSlotRejected(t *testing.T) {
	err := (*SaveSlot)(nil).WriteSpells([]SpellWrite{{SlotIndex: 0, SpellID: 0x00001770}})
	if err == nil {
		t.Fatal("expected error for nil receiver, got nil")
	}
	if !strings.Contains(err.Error(), "nil slot") {
		t.Errorf("error %q does not mention 'nil slot'", err)
	}
}

func TestWriteSpells_UninitialisedOffset_NoMutation(t *testing.T) {
	data := make([]byte, SlotSize)
	slot := &SaveSlot{Data: data, EquippedSpellsOffset: 0}
	before := append([]byte(nil), slot.Data...)

	err := slot.WriteSpells([]SpellWrite{{SlotIndex: 0, SpellID: 0x00001770}})
	if err == nil {
		t.Fatal("expected error when EquippedSpellsOffset == 0, got nil")
	}
	if !strings.Contains(err.Error(), "not initialised") {
		t.Errorf("error %q does not mention 'not initialised'", err)
	}
	if !bytes.Equal(before, slot.Data) {
		t.Error("uninitialised offset must not mutate slot.Data")
	}
}

func TestWriteSpells_UsesEquippedSpellsOffset(t *testing.T) {
	// Two slots with different EquippedSpellsOffset values. WriteSpells
	// must write to the slot's own offset, not some shared/calculated
	// value — proves the recompute path also reads from the same offset.
	slotA := makeSpellTestSlot()
	slotB := makeSpellTestSlot()
	slotB.EquippedSpellsOffset = slotA.EquippedSpellsOffset + 0x4000

	const id uint32 = 0x00001770
	if err := slotA.WriteSpells([]SpellWrite{{SlotIndex: 2, SpellID: id}}); err != nil {
		t.Fatalf("slotA: %v", err)
	}
	if err := slotB.WriteSpells([]SpellWrite{{SlotIndex: 2, SpellID: id}}); err != nil {
		t.Fatalf("slotB: %v", err)
	}

	gotA := binary.LittleEndian.Uint32(slotA.Data[slotA.EquippedSpellsOffset+2*EquippedSpellSlotSize:])
	if gotA != id {
		t.Errorf("slotA: spell at own offset = 0x%08X, want 0x%08X", gotA, id)
	}
	gotB := binary.LittleEndian.Uint32(slotB.Data[slotB.EquippedSpellsOffset+2*EquippedSpellSlotSize:])
	if gotB != id {
		t.Errorf("slotB: spell at own offset = 0x%08X, want 0x%08X", gotB, id)
	}

	// And the hash[10] for each slot reflects its OWN spells region.
	// Asserting against equipmentHash(readSpellIDs(slot.Data, slot.EquippedSpellsOffset))
	// — NOT ComputeSlotHash — because the latter recomputes spellsOff from
	// the MagicOffset chain and would diverge from the writer's
	// field-driven offset (which is the whole point of this test).
	for _, s := range []*SaveSlot{slotA, slotB} {
		got := binary.LittleEndian.Uint32(s.Data[HashOffset+10*4:])
		want := equipmentHash(readSpellIDs(s.Data, s.EquippedSpellsOffset))
		if got != want {
			t.Errorf("hash[10] drift on slot offset 0x%X: got 0x%08X, want 0x%08X",
				s.EquippedSpellsOffset, got, want)
		}
	}
}
