package core

import (
	"encoding/binary"
	"fmt"
)

// EquippedSpellSlotCount is the number of spell slots in the EquippedSpells
// section (memory + skills shortcuts). Mirrors the 14-slot constant used by
// readSpellIDs in hash.go.
const EquippedSpellSlotCount = 14

// EquippedSpellSlotSize is the per-slot byte stride: spell_id u32 LE followed
// by follower/unk u32 LE.
const EquippedSpellSlotSize = 8

// EquippedSpellEmptySentinel marks an empty spell slot. Both spell_id ==
// 0xFFFFFFFF AND follower == 0x00000000 are required by the game; mixing
// these is a corrupt state.
const EquippedSpellEmptySentinel uint32 = 0xFFFFFFFF

// EquippedSpellOccupiedFollower is written into the follower/unk u32 for any
// occupied slot. The lower-level meaning of this field (follower toggle / unk)
// is not relevant here; vanilla saves consistently use 0xFFFFFFFF for every
// occupied spell slot regardless of which spell is equipped.
const EquippedSpellOccupiedFollower uint32 = 0xFFFFFFFF

// PatchEquippedSpell writes a single spell slot in the EquippedSpells section
// of slot.Data.
//
// spellID is the raw MagicParam ID (e.g. Catch Flame = 0x1770), NOT a full
// item ID (the 0x40XXXXXX / 0x60XXXXXX prefixed form used elsewhere in the
// save). Passing EquippedSpellEmptySentinel (0xFFFFFFFF) clears the slot.
//
// Semantics:
//
//	spellID == 0xFFFFFFFF → write (spell_id=0xFFFFFFFF, follower=0x00000000)
//	spellID != 0xFFFFFFFF → write (spell_id=spellID,   follower=0xFFFFFFFF)
//
// Out of scope: this writer does NOT touch the slot hash block. Callers that
// want the in-save hash refreshed must invoke ComputeSlotHash separately, the
// same way other low-level core writers (PatchWeaponItemID, etc.) leave the
// hash update to the apply layer.
//
// Errors are returned WITHOUT mutating slot.Data. Idempotent writes (target
// bytes already match) are skipped.
func PatchEquippedSpell(slot *SaveSlot, slotIndex int, spellID uint32) error {
	if slot == nil {
		return fmt.Errorf("PatchEquippedSpell: slot is nil")
	}
	if slotIndex < 0 || slotIndex >= EquippedSpellSlotCount {
		return fmt.Errorf("PatchEquippedSpell: slotIndex %d out of range [0,%d)",
			slotIndex, EquippedSpellSlotCount)
	}
	if slot.EquippedSpellsOffset <= 0 {
		return fmt.Errorf("PatchEquippedSpell: EquippedSpellsOffset not initialised (got %d); call calculateDynamicOffsets first",
			slot.EquippedSpellsOffset)
	}

	off := slot.EquippedSpellsOffset + slotIndex*EquippedSpellSlotSize
	end := off + EquippedSpellSlotSize
	if end > len(slot.Data) {
		return fmt.Errorf("PatchEquippedSpell: slot %d at offset 0x%X exceeds Data length %d",
			slotIndex, off, len(slot.Data))
	}

	var newSpellID, newFollower uint32
	if spellID == EquippedSpellEmptySentinel {
		newSpellID = EquippedSpellEmptySentinel
		newFollower = 0x00000000
	} else {
		newSpellID = spellID
		newFollower = EquippedSpellOccupiedFollower
	}

	curSpellID := binary.LittleEndian.Uint32(slot.Data[off:])
	curFollower := binary.LittleEndian.Uint32(slot.Data[off+4:])
	if curSpellID == newSpellID && curFollower == newFollower {
		return nil
	}

	binary.LittleEndian.PutUint32(slot.Data[off:], newSpellID)
	binary.LittleEndian.PutUint32(slot.Data[off+4:], newFollower)
	return nil
}
