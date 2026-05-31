package core

import (
	"bytes"
	"encoding/binary"
	"strings"
	"testing"
)

// makeEquipmentTestSlot builds a synthetic SaveSlot whose ChrAsmEquipment
// section is empty (all 0xFFFFFFFF) and whose GaMap contains a controlled set
// of test handles for weapon / armor / goods / talisman / AoW prefixes.
//
// Layout:
//
//	EquipItemsIDOffset is fixed at a non-zero value with enough headroom for
//	the 88-byte ChrAsmEquipment section and the hash block at the end of the
//	slot. Synthetic SaveSlot does NOT exercise the dynamic offset chain that
//	ComputeSlotHash uses for entries [7..10]; instead the test mutates and
//	verifies the four hash entry bytes directly.
func makeEquipmentTestSlot() *SaveSlot {
	data := make([]byte, SlotSize)
	equipOff := 0x10000 // arbitrary safe offset
	// Initialise the equipment section to 0xFFFFFFFF (empty slot pattern).
	for i := 0; i < ChrAsmFieldCount; i++ {
		binary.LittleEndian.PutUint32(data[equipOff+i*4:], 0xFFFFFFFF)
	}

	slot := &SaveSlot{
		Data:               data,
		EquipItemsIDOffset: equipOff,
		GaMap:              map[uint32]uint32{},
	}

	// Test handles. Each handle's lower bits are unique so collisions don't
	// hide bugs.
	slot.GaMap[0x80000010] = 0x00100020 // weapon: handle 0x80 → itemID 0x00 prefix
	slot.GaMap[0x80000011] = 0x00100021 // second weapon
	slot.GaMap[0x90000020] = 0x10100040 // armor: handle 0x90 → itemID 0x10 prefix
	slot.GaMap[0x90000021] = 0x10100041 // second armor
	slot.GaMap[0xB0000030] = 0x40100050 // goods (arrows): handle 0xB0 → itemID 0x40 prefix
	slot.GaMap[0xB0000031] = 0x40100051 // second goods (bolts)
	slot.GaMap[0xA0000040] = 0x20100060 // talisman: handle 0xA0 → itemID 0x20 prefix
	slot.GaMap[0xC0000050] = 0x40100070 // AoW: handle 0xC0 → itemID (not really used here)

	return slot
}

// hash7 returns the current value of hash entry 7 (equipped weapons).
func hash7(slot *SaveSlot) uint32 {
	return binary.LittleEndian.Uint32(slot.Data[HashOffset+7*4:])
}

// hash8 returns the current value of hash entry 8 (equipped armors + talismans).
func hash8(slot *SaveSlot) uint32 {
	return binary.LittleEndian.Uint32(slot.Data[HashOffset+8*4:])
}

// equipSlot returns the raw u32 at equipment slot index.
func equipSlot(slot *SaveSlot, idx int) uint32 {
	return binary.LittleEndian.Uint32(slot.Data[slot.EquipItemsIDOffset+idx*4:])
}

// TestWriteEquipment_WeaponEncoding verifies that writing a weapon handle
// stores `itemID | 0x80000000` in the slot.
func TestWriteEquipment_WeaponEncoding(t *testing.T) {
	slot := makeEquipmentTestSlot()
	err := slot.WriteEquipment([]EquipmentWrite{
		{Slot: EquipSlotRightHandArmament1, Handle: 0x80000010},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := equipSlot(slot, 1)
	want := uint32(0x00100020) | 0x80000000
	if got != want {
		t.Errorf("weapon slot encoding: got 0x%08X, want 0x%08X", got, want)
	}
}

// TestWriteEquipment_ArmorEncoding verifies that writing an armor handle
// stores `itemID | 0x80000000` in the slot.
func TestWriteEquipment_ArmorEncoding(t *testing.T) {
	slot := makeEquipmentTestSlot()
	err := slot.WriteEquipment([]EquipmentWrite{
		{Slot: EquipSlotHead, Handle: 0x90000020},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := equipSlot(slot, 12)
	want := uint32(0x10100040) | 0x80000000
	if got != want {
		t.Errorf("armor slot encoding: got 0x%08X, want 0x%08X", got, want)
	}
}

// TestWriteEquipment_AmmoEncoding verifies that writing a goods handle into
// an ammo slot stores the raw itemID (already has 0x40 prefix).
func TestWriteEquipment_AmmoEncoding(t *testing.T) {
	slot := makeEquipmentTestSlot()
	err := slot.WriteEquipment([]EquipmentWrite{
		{Slot: EquipSlotArrows1, Handle: 0xB0000030},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := equipSlot(slot, 6)
	want := uint32(0x40100050)
	if got != want {
		t.Errorf("ammo slot encoding: got 0x%08X, want 0x%08X", got, want)
	}
}

// TestWriteEquipment_RejectAoWInWeaponSlot verifies that 0xC0 (Ash of War)
// handles are rejected in weapon slots, even though the read-side accepts
// `itemID | 0x80000000` encoding for AoW.
func TestWriteEquipment_RejectAoWInWeaponSlot(t *testing.T) {
	slot := makeEquipmentTestSlot()
	err := slot.WriteEquipment([]EquipmentWrite{
		{Slot: EquipSlotRightHandArmament1, Handle: 0xC0000050},
	})
	if err == nil {
		t.Fatal("expected error rejecting AoW handle in weapon slot, got nil")
	}
	if !strings.Contains(err.Error(), "Ash of War") {
		t.Errorf("error should mention Ash of War, got: %v", err)
	}
	if equipSlot(slot, 1) != 0xFFFFFFFF {
		t.Error("slot should remain empty after rejected write")
	}
}

// TestWriteEquipment_RejectTalismanInWeaponSlot ensures talisman handles
// (0xA0) cannot be written into weapon slots.
func TestWriteEquipment_RejectTalismanInWeaponSlot(t *testing.T) {
	slot := makeEquipmentTestSlot()
	err := slot.WriteEquipment([]EquipmentWrite{
		{Slot: EquipSlotRightHandArmament1, Handle: 0xA0000040},
	})
	if err == nil {
		t.Fatal("expected error rejecting talisman handle in weapon slot, got nil")
	}
}

// TestWriteEquipment_RejectGoodsInWeaponSlot ensures goods handles (0xB0)
// cannot be written into weapon slots (they belong only in ammo slots).
func TestWriteEquipment_RejectGoodsInWeaponSlot(t *testing.T) {
	slot := makeEquipmentTestSlot()
	err := slot.WriteEquipment([]EquipmentWrite{
		{Slot: EquipSlotLeftHandArmament1, Handle: 0xB0000030},
	})
	if err == nil {
		t.Fatal("expected error rejecting goods handle in weapon slot, got nil")
	}
}

// TestWriteEquipment_RejectWeaponInArmorSlot ensures weapon handles (0x80)
// cannot be written into armor slots.
func TestWriteEquipment_RejectWeaponInArmorSlot(t *testing.T) {
	slot := makeEquipmentTestSlot()
	err := slot.WriteEquipment([]EquipmentWrite{
		{Slot: EquipSlotChest, Handle: 0x80000010},
	})
	if err == nil {
		t.Fatal("expected error rejecting weapon handle in armor slot, got nil")
	}
}

// TestWriteEquipment_RejectMissingHandle verifies that handles not present in
// GaMap are rejected (strict — no auto-add in Phase 7b.0).
func TestWriteEquipment_RejectMissingHandle(t *testing.T) {
	slot := makeEquipmentTestSlot()
	err := slot.WriteEquipment([]EquipmentWrite{
		{Slot: EquipSlotRightHandArmament1, Handle: 0x80000099}, // not in GaMap
	})
	if err == nil {
		t.Fatal("expected error for missing handle, got nil")
	}
	if !strings.Contains(err.Error(), "not present in inventory") {
		t.Errorf("error should mention inventory presence, got: %v", err)
	}
}

// TestWriteEquipment_ClearSlot verifies that Handle == 0 clears the slot to
// 0xFFFFFFFF, regardless of the slot's class.
func TestWriteEquipment_ClearSlot(t *testing.T) {
	slot := makeEquipmentTestSlot()
	// First populate the slot.
	if err := slot.WriteEquipment([]EquipmentWrite{
		{Slot: EquipSlotRightHandArmament1, Handle: 0x80000010},
	}); err != nil {
		t.Fatalf("setup write: %v", err)
	}
	// Then clear it.
	if err := slot.WriteEquipment([]EquipmentWrite{
		{Slot: EquipSlotRightHandArmament1, Handle: 0},
	}); err != nil {
		t.Fatalf("clear write: %v", err)
	}
	if equipSlot(slot, 1) != 0xFFFFFFFF {
		t.Errorf("cleared slot should hold 0xFFFFFFFF, got 0x%08X", equipSlot(slot, 1))
	}
}

// TestWriteEquipment_RejectInvalidSentinel ensures Handle == 0xFFFFFFFF is
// rejected — callers must use Handle == 0 to clear a slot.
func TestWriteEquipment_RejectInvalidSentinel(t *testing.T) {
	slot := makeEquipmentTestSlot()
	err := slot.WriteEquipment([]EquipmentWrite{
		{Slot: EquipSlotRightHandArmament1, Handle: 0xFFFFFFFF},
	})
	if err == nil {
		t.Fatal("expected error rejecting 0xFFFFFFFF handle, got nil")
	}
}

// TestWriteEquipment_RejectInvalidSlotKind ensures slot kinds outside the
// supported set (e.g. talismans, great rune, unknowns) are rejected.
func TestWriteEquipment_RejectInvalidSlotKind(t *testing.T) {
	slot := makeEquipmentTestSlot()
	// EquipSlotLegs is the highest valid enum (value 13). Anything ≥14 is unsupported.
	err := slot.WriteEquipment([]EquipmentWrite{
		{Slot: EquipmentSlotKind(99), Handle: 0x80000010},
	})
	if err == nil {
		t.Fatal("expected error rejecting unknown slot kind, got nil")
	}
	if !strings.Contains(err.Error(), "unsupported slot kind") {
		t.Errorf("error should mention unsupported slot kind, got: %v", err)
	}
}

// TestWriteEquipment_RejectDuplicateSlot ensures the same slot kind appearing
// twice in a batch is rejected (deterministic error rather than silent
// last-wins).
func TestWriteEquipment_RejectDuplicateSlot(t *testing.T) {
	slot := makeEquipmentTestSlot()
	err := slot.WriteEquipment([]EquipmentWrite{
		{Slot: EquipSlotRightHandArmament1, Handle: 0x80000010},
		{Slot: EquipSlotRightHandArmament1, Handle: 0x80000011},
	})
	if err == nil {
		t.Fatal("expected error rejecting duplicate slot in batch, got nil")
	}
	if equipSlot(slot, 1) != 0xFFFFFFFF {
		t.Error("slot should remain empty after rejected batch")
	}
}

// TestWriteEquipment_AtomicRollbackOnSecondInvalid is the core atomicity test:
// the first write is valid, the second invalid → no equipment bytes or hash
// bytes are mutated.
func TestWriteEquipment_AtomicRollbackOnSecondInvalid(t *testing.T) {
	slot := makeEquipmentTestSlot()

	beforeEquip := make([]byte, ChrAsmEquipmentSize)
	copy(beforeEquip, slot.Data[slot.EquipItemsIDOffset:slot.EquipItemsIDOffset+ChrAsmEquipmentSize])
	beforeHash := make([]byte, HashSize)
	copy(beforeHash, slot.Data[HashOffset:HashOffset+HashSize])

	err := slot.WriteEquipment([]EquipmentWrite{
		{Slot: EquipSlotRightHandArmament1, Handle: 0x80000010}, // valid
		{Slot: EquipSlotHead, Handle: 0x80000010},                // invalid: weapon in armor slot
	})
	if err == nil {
		t.Fatal("expected error from second write, got nil")
	}

	afterEquip := slot.Data[slot.EquipItemsIDOffset : slot.EquipItemsIDOffset+ChrAsmEquipmentSize]
	if !bytes.Equal(beforeEquip, afterEquip) {
		t.Error("equipment section bytes mutated despite batch failure")
	}
	afterHash := slot.Data[HashOffset : HashOffset+HashSize]
	if !bytes.Equal(beforeHash, afterHash) {
		t.Error("hash bytes mutated despite batch failure")
	}
}

// TestWriteEquipment_Hash7ChangesOnWeaponWrite verifies hash entry 7 is
// recomputed after a weapon-slot write.
func TestWriteEquipment_Hash7ChangesOnWeaponWrite(t *testing.T) {
	slot := makeEquipmentTestSlot()
	before := hash7(slot)
	if err := slot.WriteEquipment([]EquipmentWrite{
		{Slot: EquipSlotRightHandArmament1, Handle: 0x80000010},
	}); err != nil {
		t.Fatalf("write: %v", err)
	}
	after := hash7(slot)
	if before == after {
		t.Errorf("hash 7 should change after weapon write (got 0x%08X both times)", before)
	}
}

// TestWriteEquipment_Hash7ChangesOnAmmoWrite verifies hash entry 7 changes for
// ammo writes too (slots 6–9 belong to hash 7).
func TestWriteEquipment_Hash7ChangesOnAmmoWrite(t *testing.T) {
	slot := makeEquipmentTestSlot()
	before := hash7(slot)
	if err := slot.WriteEquipment([]EquipmentWrite{
		{Slot: EquipSlotArrows1, Handle: 0xB0000030},
	}); err != nil {
		t.Fatalf("write: %v", err)
	}
	after := hash7(slot)
	if before == after {
		t.Errorf("hash 7 should change after ammo write (got 0x%08X both times)", before)
	}
}

// TestWriteEquipment_Hash8ChangesOnArmorWrite verifies hash entry 8 is
// recomputed after an armor-slot write.
func TestWriteEquipment_Hash8ChangesOnArmorWrite(t *testing.T) {
	slot := makeEquipmentTestSlot()
	before := hash8(slot)
	if err := slot.WriteEquipment([]EquipmentWrite{
		{Slot: EquipSlotChest, Handle: 0x90000020},
	}); err != nil {
		t.Fatalf("write: %v", err)
	}
	after := hash8(slot)
	if before == after {
		t.Errorf("hash 8 should change after armor write (got 0x%08X both times)", before)
	}
}

// TestWriteEquipment_Hash7StableOnArmorOnlyWrite verifies that hash 7 is NOT
// recomputed when only armor slots are written.
func TestWriteEquipment_Hash7StableOnArmorOnlyWrite(t *testing.T) {
	slot := makeEquipmentTestSlot()
	// Pre-seed a sentinel value in hash 7.
	binary.LittleEndian.PutUint32(slot.Data[HashOffset+7*4:], 0xDEADBEEF)

	if err := slot.WriteEquipment([]EquipmentWrite{
		{Slot: EquipSlotHead, Handle: 0x90000020},
	}); err != nil {
		t.Fatalf("write: %v", err)
	}
	if got := hash7(slot); got != 0xDEADBEEF {
		t.Errorf("hash 7 should be untouched on armor-only write, got 0x%08X", got)
	}
}

// TestWriteEquipment_Hash8StableOnWeaponOnlyWrite verifies that hash 8 is NOT
// recomputed when only weapon/ammo slots are written.
func TestWriteEquipment_Hash8StableOnWeaponOnlyWrite(t *testing.T) {
	slot := makeEquipmentTestSlot()
	binary.LittleEndian.PutUint32(slot.Data[HashOffset+8*4:], 0xDEADBEEF)

	if err := slot.WriteEquipment([]EquipmentWrite{
		{Slot: EquipSlotRightHandArmament1, Handle: 0x80000010},
	}); err != nil {
		t.Fatalf("write: %v", err)
	}
	if got := hash8(slot); got != 0xDEADBEEF {
		t.Errorf("hash 8 should be untouched on weapon-only write, got 0x%08X", got)
	}
}

// TestWriteEquipment_BatchTouchesBothHashes verifies that a mixed batch
// (weapon + armor) recomputes both hash 7 and hash 8.
func TestWriteEquipment_BatchTouchesBothHashes(t *testing.T) {
	slot := makeEquipmentTestSlot()
	before7 := hash7(slot)
	before8 := hash8(slot)
	if err := slot.WriteEquipment([]EquipmentWrite{
		{Slot: EquipSlotRightHandArmament1, Handle: 0x80000010},
		{Slot: EquipSlotHead, Handle: 0x90000020},
	}); err != nil {
		t.Fatalf("write: %v", err)
	}
	if hash7(slot) == before7 {
		t.Error("hash 7 should change after mixed batch")
	}
	if hash8(slot) == before8 {
		t.Error("hash 8 should change after mixed batch")
	}
}

// TestWriteEquipment_IdempotentWriteStableHash verifies that writing the same
// handle twice produces identical hash bytes both times.
func TestWriteEquipment_IdempotentWriteStableHash(t *testing.T) {
	slot := makeEquipmentTestSlot()
	if err := slot.WriteEquipment([]EquipmentWrite{
		{Slot: EquipSlotRightHandArmament1, Handle: 0x80000010},
	}); err != nil {
		t.Fatalf("first write: %v", err)
	}
	h7First := hash7(slot)
	if err := slot.WriteEquipment([]EquipmentWrite{
		{Slot: EquipSlotRightHandArmament1, Handle: 0x80000010},
	}); err != nil {
		t.Fatalf("second write: %v", err)
	}
	h7Second := hash7(slot)
	if h7First != h7Second {
		t.Errorf("idempotent write should yield stable hash 7: 0x%08X then 0x%08X", h7First, h7Second)
	}
}

// TestWriteEquipment_NilSlot ensures a nil receiver returns an error rather
// than panicking.
func TestWriteEquipment_NilSlot(t *testing.T) {
	var slot *SaveSlot
	err := slot.WriteEquipment([]EquipmentWrite{
		{Slot: EquipSlotRightHandArmament1, Handle: 0x80000010},
	})
	if err == nil {
		t.Fatal("expected error for nil slot, got nil")
	}
}

// TestWriteEquipment_EmptyBatch is a no-op.
func TestWriteEquipment_EmptyBatch(t *testing.T) {
	slot := makeEquipmentTestSlot()
	before := make([]byte, ChrAsmEquipmentSize)
	copy(before, slot.Data[slot.EquipItemsIDOffset:slot.EquipItemsIDOffset+ChrAsmEquipmentSize])
	if err := slot.WriteEquipment(nil); err != nil {
		t.Fatalf("empty batch should be no-op, got error: %v", err)
	}
	after := slot.Data[slot.EquipItemsIDOffset : slot.EquipItemsIDOffset+ChrAsmEquipmentSize]
	if !bytes.Equal(before, after) {
		t.Error("empty batch should not mutate equipment section")
	}
}

// TestWriteEquipment_UnparseableOffset ensures the writer rejects slots whose
// EquipItemsIDOffset was not parsed (e.g. empty slot).
func TestWriteEquipment_UnparseableOffset(t *testing.T) {
	slot := &SaveSlot{
		Data:               make([]byte, SlotSize),
		EquipItemsIDOffset: 0,
		GaMap:              map[uint32]uint32{},
	}
	err := slot.WriteEquipment([]EquipmentWrite{
		{Slot: EquipSlotRightHandArmament1, Handle: 0x80000010},
	})
	if err == nil {
		t.Fatal("expected error for unparseable slot, got nil")
	}
}

// TestWriteEquipment_TalismanEncoding verifies that writing a talisman handle
// stores `itemID` directly (GaMap value already carries the 0x20 prefix, no
// OR-mask is applied).
func TestWriteEquipment_TalismanEncoding(t *testing.T) {
	slot := makeEquipmentTestSlot()
	err := slot.WriteEquipment([]EquipmentWrite{
		{Slot: EquipSlotTalisman1, Handle: 0xA0000040},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := equipSlot(slot, 17)
	want := uint32(0x20100060)
	if got != want {
		t.Errorf("talisman slot encoding: got 0x%08X, want 0x%08X", got, want)
	}
}

// TestWriteEquipment_RejectWeaponInTalismanSlot ensures 0x80 handles are
// rejected in talisman slots.
func TestWriteEquipment_RejectWeaponInTalismanSlot(t *testing.T) {
	slot := makeEquipmentTestSlot()
	err := slot.WriteEquipment([]EquipmentWrite{
		{Slot: EquipSlotTalisman1, Handle: 0x80000010},
	})
	if err == nil {
		t.Fatal("expected error rejecting weapon handle in talisman slot, got nil")
	}
	if !strings.Contains(err.Error(), "weapon prefix") {
		t.Errorf("error should mention weapon prefix, got: %v", err)
	}
}

// TestWriteEquipment_RejectArmorInTalismanSlot ensures 0x90 handles are
// rejected in talisman slots.
func TestWriteEquipment_RejectArmorInTalismanSlot(t *testing.T) {
	slot := makeEquipmentTestSlot()
	err := slot.WriteEquipment([]EquipmentWrite{
		{Slot: EquipSlotTalisman2, Handle: 0x90000020},
	})
	if err == nil {
		t.Fatal("expected error rejecting armor handle in talisman slot, got nil")
	}
	if !strings.Contains(err.Error(), "armor prefix") {
		t.Errorf("error should mention armor prefix, got: %v", err)
	}
}

// TestWriteEquipment_RejectGoodsInTalismanSlot ensures 0xB0 handles are
// rejected in talisman slots.
func TestWriteEquipment_RejectGoodsInTalismanSlot(t *testing.T) {
	slot := makeEquipmentTestSlot()
	err := slot.WriteEquipment([]EquipmentWrite{
		{Slot: EquipSlotTalisman3, Handle: 0xB0000030},
	})
	if err == nil {
		t.Fatal("expected error rejecting goods handle in talisman slot, got nil")
	}
	if !strings.Contains(err.Error(), "goods prefix") {
		t.Errorf("error should mention goods prefix, got: %v", err)
	}
}

// TestWriteEquipment_RejectAoWInTalismanSlot ensures 0xC0 handles are rejected
// in talisman slots.
func TestWriteEquipment_RejectAoWInTalismanSlot(t *testing.T) {
	slot := makeEquipmentTestSlot()
	err := slot.WriteEquipment([]EquipmentWrite{
		{Slot: EquipSlotTalisman4, Handle: 0xC0000050},
	})
	if err == nil {
		t.Fatal("expected error rejecting AoW handle in talisman slot, got nil")
	}
	if !strings.Contains(err.Error(), "Ash of War") {
		t.Errorf("error should mention Ash of War, got: %v", err)
	}
}

// TestWriteEquipment_Hash8ChangesOnTalismanWrite verifies hash entry 8 is
// recomputed after a talisman-slot write (talismans share hash 8 with armor).
func TestWriteEquipment_Hash8ChangesOnTalismanWrite(t *testing.T) {
	slot := makeEquipmentTestSlot()
	before := hash8(slot)
	if err := slot.WriteEquipment([]EquipmentWrite{
		{Slot: EquipSlotTalisman1, Handle: 0xA0000040},
	}); err != nil {
		t.Fatalf("write: %v", err)
	}
	after := hash8(slot)
	if before == after {
		t.Errorf("hash 8 should change after talisman write (got 0x%08X both times)", before)
	}
}

// TestWriteEquipment_Hash7StableOnTalismanOnlyWrite ensures hash 7 is NOT
// recomputed when only talisman slots are written.
func TestWriteEquipment_Hash7StableOnTalismanOnlyWrite(t *testing.T) {
	slot := makeEquipmentTestSlot()
	binary.LittleEndian.PutUint32(slot.Data[HashOffset+7*4:], 0xDEADBEEF)

	if err := slot.WriteEquipment([]EquipmentWrite{
		{Slot: EquipSlotTalisman1, Handle: 0xA0000040},
	}); err != nil {
		t.Fatalf("write: %v", err)
	}
	if got := hash7(slot); got != 0xDEADBEEF {
		t.Errorf("hash 7 should be untouched on talisman-only write, got 0x%08X", got)
	}
}

// TestWriteEquipment_IdempotentTalismanWriteStableHash verifies repeated
// talisman writes yield identical hash 8 bytes.
func TestWriteEquipment_IdempotentTalismanWriteStableHash(t *testing.T) {
	slot := makeEquipmentTestSlot()
	if err := slot.WriteEquipment([]EquipmentWrite{
		{Slot: EquipSlotTalisman1, Handle: 0xA0000040},
	}); err != nil {
		t.Fatalf("first write: %v", err)
	}
	h8First := hash8(slot)
	if err := slot.WriteEquipment([]EquipmentWrite{
		{Slot: EquipSlotTalisman1, Handle: 0xA0000040},
	}); err != nil {
		t.Fatalf("second write: %v", err)
	}
	h8Second := hash8(slot)
	if h8First != h8Second {
		t.Errorf("idempotent talisman write should yield stable hash 8: 0x%08X then 0x%08X", h8First, h8Second)
	}
}

// TestWriteEquipment_MixedArmorTalismanBatchHash8 verifies that a mixed batch
// (armor + talisman) recomputes hash 8 exactly once and correctly.
func TestWriteEquipment_MixedArmorTalismanBatchHash8(t *testing.T) {
	slot := makeEquipmentTestSlot()
	before := hash8(slot)
	if err := slot.WriteEquipment([]EquipmentWrite{
		{Slot: EquipSlotHead, Handle: 0x90000020},
		{Slot: EquipSlotTalisman1, Handle: 0xA0000040},
	}); err != nil {
		t.Fatalf("write: %v", err)
	}
	if hash8(slot) == before {
		t.Error("hash 8 should change after mixed armor+talisman batch")
	}
	// Verify both slots got their expected encoded values.
	if got := equipSlot(slot, 12); got != uint32(0x10100040)|0x80000000 {
		t.Errorf("armor slot encoding mismatch: 0x%08X", got)
	}
	if got := equipSlot(slot, 17); got != 0x20100060 {
		t.Errorf("talisman slot encoding mismatch: 0x%08X", got)
	}
}

// TestWriteEquipment_AtomicRollbackOnInvalidTalisman verifies that a valid
// talisman1 write followed by an invalid talisman2 (wrong prefix) leaves the
// equipment section and hash bytes unchanged.
func TestWriteEquipment_AtomicRollbackOnInvalidTalisman(t *testing.T) {
	slot := makeEquipmentTestSlot()

	beforeEquip := make([]byte, ChrAsmEquipmentSize)
	copy(beforeEquip, slot.Data[slot.EquipItemsIDOffset:slot.EquipItemsIDOffset+ChrAsmEquipmentSize])
	beforeHash := make([]byte, HashSize)
	copy(beforeHash, slot.Data[HashOffset:HashOffset+HashSize])

	err := slot.WriteEquipment([]EquipmentWrite{
		{Slot: EquipSlotTalisman1, Handle: 0xA0000040}, // valid
		{Slot: EquipSlotTalisman2, Handle: 0x80000010}, // invalid: weapon in talisman slot
	})
	if err == nil {
		t.Fatal("expected error from second write, got nil")
	}

	afterEquip := slot.Data[slot.EquipItemsIDOffset : slot.EquipItemsIDOffset+ChrAsmEquipmentSize]
	if !bytes.Equal(beforeEquip, afterEquip) {
		t.Error("equipment section bytes mutated despite batch failure")
	}
	afterHash := slot.Data[HashOffset : HashOffset+HashSize]
	if !bytes.Equal(beforeHash, afterHash) {
		t.Error("hash bytes mutated despite batch failure")
	}
}

// TestWriteEquipment_RoundTripWeaponSwap simulates the real workflow: equip
// weapon A, then swap to weapon B, then read it back via the same encoding
// formula used by readers.
func TestWriteEquipment_RoundTripWeaponSwap(t *testing.T) {
	slot := makeEquipmentTestSlot()

	if err := slot.WriteEquipment([]EquipmentWrite{
		{Slot: EquipSlotRightHandArmament1, Handle: 0x80000010},
	}); err != nil {
		t.Fatalf("equip first weapon: %v", err)
	}
	wantFirst := uint32(0x00100020) | 0x80000000
	if got := equipSlot(slot, 1); got != wantFirst {
		t.Fatalf("after first write: got 0x%08X, want 0x%08X", got, wantFirst)
	}

	if err := slot.WriteEquipment([]EquipmentWrite{
		{Slot: EquipSlotRightHandArmament1, Handle: 0x80000011},
	}); err != nil {
		t.Fatalf("swap to second weapon: %v", err)
	}
	wantSecond := uint32(0x00100021) | 0x80000000
	if got := equipSlot(slot, 1); got != wantSecond {
		t.Fatalf("after swap: got 0x%08X, want 0x%08X", got, wantSecond)
	}

	if err := slot.WriteEquipment([]EquipmentWrite{
		{Slot: EquipSlotRightHandArmament1, Handle: 0},
	}); err != nil {
		t.Fatalf("clear: %v", err)
	}
	if got := equipSlot(slot, 1); got != 0xFFFFFFFF {
		t.Fatalf("after clear: got 0x%08X, want 0xFFFFFFFF", got)
	}
}
