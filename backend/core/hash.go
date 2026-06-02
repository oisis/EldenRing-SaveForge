package core

import (
	"encoding/binary"
)

// Hash algorithm constants.
const (
	HashMagic    = 0x80078071 // Magic constant for ComputeHashedValue
	HashSize     = 0x80       // 128 bytes = 32 u32 entries (12 used + padding)
	HashOffset   = SlotSize - HashSize // 0x27FF80 — start of hash block in slot
	HashEntries  = 12                   // Number of meaningful hash entries
)

// PlayerGameData field offsets relative to MagicOffset, for hash computation.
// These correspond to the PGD struct layout documented in the audit.
const (
	OffHumanity   = -347 // PGD+0x54
	OffSoulMemory = -327 // PGD+0x68
	OffPGD0xB8    = -247 // PGD+0xB8 (1 byte, used for hash index 3)
)

// ChrAsmEquipment field count — 22 u32 values per equipment section (0x58 bytes).
const ChrAsmFieldCount = 22

// ChrAsmEquipmentSize is the byte size of the ChrAsmEquipment header (22 × 4 = 0x58).
const ChrAsmEquipmentSize = ChrAsmFieldCount * 4

// Equipment section layout within ChrAsmEquipment (0x58 bytes):
// [0]  LeftHandArmament1    [1]  RightHandArmament1
// [2]  LeftHandArmament2    [3]  RightHandArmament2
// [4]  LeftHandArmament3    [5]  RightHandArmament3
// [6]  Arrows1              [7]  Bolts1
// [8]  Arrows2              [9]  Bolts2
// [10] unk0x28              [11] unk0x2C
// [12] Head                 [13] Chest
// [14] Arms                 [15] Legs
// [16] unk0x40
// [17] Talisman1            [18] Talisman2
// [19] Talisman3            [20] Talisman4
// [21] Talisman5

// Weapon slot indices in ChrAsmEquipment (for hash index 7).
// 10 entries: L1,R1,L2,R2,L3,R3,Arrows1,Bolts1,Arrows2,Bolts2
var weaponSlotIndices = []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}

// Armor+Talisman slot indices in ChrAsmEquipment (for hash index 8).
// 9 entries: Head,Chest,Arms,Legs,Talisman1-5
var armorSlotIndices = []int{12, 13, 14, 15, 17, 18, 19, 20, 21}

// computeHashedValue applies the modified modular reduction.
//
//	product = uint64(0x80078071) * uint64(input)
//	upper  = uint32(product >> 32)
//	shifted = upper >> 15
//	mod     = int32(shifted) * (-0xFFF1)
//	return input + uint32(mod)
func computeHashedValue(input uint32) uint32 {
	product := uint64(HashMagic) * uint64(input)
	upper := uint32(product >> 32)
	shifted := upper >> 15
	mod := int32(shifted) * (-0xFFF1)
	return input + uint32(mod)
}

// valueHash computes the hash for a single u32 value.
// Treats the value as 4 little-endian bytes and runs BytesHash.
func valueHash(value uint32) uint32 {
	var buf [4]byte
	binary.LittleEndian.PutUint32(buf[:], value)
	return bytesHash(buf[:])
}

// bytesHash computes the modified Adler-like checksum over a byte slice.
func bytesHash(data []byte) uint32 {
	var lo uint32 = 1
	var hi uint32 = 0
	for _, b := range data {
		lo = lo + uint32(b)
		hi = hi + lo
	}
	loH := computeHashedValue(lo)
	hiH := computeHashedValue(hi)
	return (loH | (hiH << 16)) * 2
}

// statsHash computes the hash for the 9 stat values.
// IMPORTANT: Intelligence and Faith are SWAPPED in hash order.
// Order: Vigor, Mind, Endurance, Strength, Dexterity, Intelligence, Faith, Arcane, Humanity
// But in hash: Vigor, Mind, End, Str, Dex, FAITH, INT, Arc, Humanity
func statsHash(vigor, mind, endurance, strength, dexterity, intelligence, faith, arcane, humanity uint32) uint32 {
	var buf [36]byte // 9 * 4 bytes
	binary.LittleEndian.PutUint32(buf[0:], vigor)
	binary.LittleEndian.PutUint32(buf[4:], mind)
	binary.LittleEndian.PutUint32(buf[8:], endurance)
	binary.LittleEndian.PutUint32(buf[12:], strength)
	binary.LittleEndian.PutUint32(buf[16:], dexterity)
	// SWAPPED: Faith before Intelligence
	binary.LittleEndian.PutUint32(buf[20:], faith)
	binary.LittleEndian.PutUint32(buf[24:], intelligence)
	binary.LittleEndian.PutUint32(buf[28:], arcane)
	binary.LittleEndian.PutUint32(buf[32:], humanity)
	return bytesHash(buf[:])
}

// equipmentHash computes BytesHash over a slice of u32 item IDs.
func equipmentHash(ids []uint32) uint32 {
	buf := make([]byte, len(ids)*4)
	for i, id := range ids {
		binary.LittleEndian.PutUint32(buf[i*4:], id)
	}
	return bytesHash(buf)
}

// quickItemsHash computes hash over quick item IDs, using only lower 28 bits.
func quickItemsHash(ids []uint32) uint32 {
	buf := make([]byte, len(ids)*4)
	for i, id := range ids {
		binary.LittleEndian.PutUint32(buf[i*4:], id&0x0FFFFFFF)
	}
	return bytesHash(buf)
}

// readEquipSection reads ChrAsmFieldCount (22) u32 values from the given offset.
func readEquipSection(data []byte, offset int) []uint32 {
	result := make([]uint32, ChrAsmFieldCount)
	for i := 0; i < ChrAsmFieldCount; i++ {
		off := offset + i*4
		if off+4 <= len(data) {
			result[i] = binary.LittleEndian.Uint32(data[off:])
		}
	}
	return result
}

// extractSlots picks values at the given indices from an equipment section.
func extractSlots(section []uint32, indices []int) []uint32 {
	result := make([]uint32, len(indices))
	for i, idx := range indices {
		if idx < len(section) {
			result[i] = section[idx]
		}
	}
	return result
}

// readSpellIDs reads 14 spell IDs from the EquipMagicData section.
// Each spell entry = 8 bytes (SpellID i32 + unk i32).
func readSpellIDs(data []byte, offset int) []uint32 {
	const spellSlots = 14
	result := make([]uint32, spellSlots)
	for i := 0; i < spellSlots; i++ {
		off := offset + i*8
		if off+4 <= len(data) {
			result[i] = binary.LittleEndian.Uint32(data[off:])
		}
	}
	return result
}

// readQuickItemIDs reads 16 quick item / pouch IDs.
// The caller must pass the correct offset (after ChrAsmEquipment header).
// Layout: 10 quick slots + 6 pouch slots = 16 × u32 IDs.
func readQuickItemIDs(data []byte, offset int) []uint32 {
	const quickItemCount = 16
	result := make([]uint32, quickItemCount)
	for i := 0; i < quickItemCount; i++ {
		off := offset + i*4
		if off+4 <= len(data) {
			result[i] = binary.LittleEndian.Uint32(data[off:])
		}
	}
	return result
}

// ComputeSlotHash calculates the full CSPlayerGameDataHash (0x80 bytes) for a slot.
// The hash block is written at SlotSize - 0x80.
//
// The offset chain for equipment sections mirrors calculateDynamicOffsets() in structures.go,
// including the dynamic projSize field. This ensures hash entries [7]-[10] read from the
// correct positions in the slot data.
//
// Hash entries:
//
//	[0]  Level
//	[1]  Stats (with Int/Faith swapped)
//	[2]  ArcheType (Class)
//	[3]  PGD+0xB8 byte
//	[4]  padding (0)
//	[5]  Souls
//	[6]  SoulMemory
//	[7]  EquippedWeapons (10 IDs)
//	[8]  EquippedArmors (4 armor + 5 talismans = 9 IDs)
//	[9]  EquippedItems (16 quick/pouch IDs, lower 28 bits)
//	[10] EquippedSpells (14 spell IDs)
//	[11] padding (0)
func ComputeSlotHash(slot *SaveSlot) [HashSize]byte {
	var hash [HashSize]byte
	sa := NewSlotAccessor(slot.Data)
	mo := slot.MagicOffset

	// Helper to write a u32 at hash entry index
	writeEntry := func(index int, value uint32) {
		binary.LittleEndian.PutUint32(hash[index*4:], value)
	}

	// [0] Level
	writeEntry(0, valueHash(slot.Player.Level))

	// [1] Stats — read Humanity from slot data
	humanity, _ := sa.ReadU32(mo + OffHumanity)
	writeEntry(1, statsHash(
		slot.Player.Vigor, slot.Player.Mind, slot.Player.Endurance,
		slot.Player.Strength, slot.Player.Dexterity,
		slot.Player.Intelligence, slot.Player.Faith,
		slot.Player.Arcane, humanity,
	))

	// [2] ArcheType (Class)
	writeEntry(2, valueHash(uint32(slot.Player.Class)))

	// [3] PGD+0xB8 — single byte hash
	pgdB8, _ := sa.ReadU8(mo + OffPGD0xB8)
	writeEntry(3, bytesHash([]byte{pgdB8}))

	// [4] padding
	writeEntry(4, 0)

	// [5] Souls
	writeEntry(5, valueHash(slot.Player.Souls))

	// [6] SoulMemory
	soulMemory, _ := sa.ReadU32(mo + OffSoulMemory)
	writeEntry(6, valueHash(soulMemory))

	// [7-10] Equipment hashes — full dynamic offset chain from MagicOffset.
	// Must match calculateDynamicOffsets() in structures.go exactly.
	spEffect := mo + DynSpEffect
	equipedItemIndex := spEffect + DynEquipedItemIndex
	activeEquipedItems := equipedItemIndex + DynActiveEquipedItems
	equipItemsIDOff := activeEquipedItems + DynEquipedItemsID
	activeEquipedItemsGa := equipItemsIDOff + DynActiveEquipedItemsGa
	// EquippedSpells starts immediately at the end of inventory_held (no gap).
	// Mirrors calculateDynamicOffsets() exactly. Chain layout:
	//   spellsOff (size 0x74) → equipedItemsOff (size 0x8C) → gestures (size 0x18) → projHeaderOff
	// Live save verification confirmed raw MagicParam spell_ids appear at spellsOff
	// and quick items appear inside [equipedItemsOff + ChrAsmEquipmentSize, +DynEquipedItems).
	spellsOff := activeEquipedItemsGa + DynInventoryHeld
	equipedItemsOff := spellsOff + DynEquipedSpells
	projHeaderOff := equipedItemsOff + DynEquipedItems + DynEquipedGestures

	// [7-8] Equipment IDs — from EquipedItemsID section (before dynamic fields)
	if equipItemsIDOff+ChrAsmEquipmentSize <= len(slot.Data) {
		equipSection := readEquipSection(slot.Data, equipItemsIDOff)

		// [7] Equipped Weapons: L1,R1,L2,R2,L3,R3,Arrows1,Bolts1,Arrows2,Bolts2
		weaponIDs := extractSlots(equipSection, weaponSlotIndices)
		writeEntry(7, equipmentHash(weaponIDs))

		// [8] Equipped Armors: Head,Chest,Arms,Legs,Talisman1-5
		armorIDs := extractSlots(equipSection, armorSlotIndices)
		writeEntry(8, equipmentHash(armorIDs))
	}

	// [9-10] Validate offset chain is reachable by checking projHeader bounds.
	err := sa.CheckBounds(projHeaderOff, 4, "hash/projHeader")
	if err == nil {

		// [10] Equipped Spells — from EquipedSpells section
		// Each spell entry = 8 bytes (SpellID i32 + unk i32), 14 spell slots.
		if spellsOff+14*8 <= len(slot.Data) {
			spellIDs := readSpellIDs(slot.Data, spellsOff)
			writeEntry(10, equipmentHash(spellIDs))
		}

		// [9] Equipped Quick Items / Pouch — from EquipedItems section (BUG-5 fix).
		// The EquipedItems section is 0x8C bytes:
		//   ChrAsmEquipment (0x58) + QuickSlots (10×4=0x28) + PouchSlots (6×4=0x18) - overlap
		// Quick items start AFTER the ChrAsmEquipment header.
		quickItemsOff := equipedItemsOff + ChrAsmEquipmentSize
		if quickItemsOff+16*4 <= len(slot.Data) {
			quickIDs := readQuickItemIDs(slot.Data, quickItemsOff)
			writeEntry(9, quickItemsHash(quickIDs))
		}
	}

	// [11] padding
	writeEntry(11, 0)

	return hash
}

// RecalculateSlotHash computes and writes the CSPlayerGameDataHash into slot data.
func RecalculateSlotHash(slot *SaveSlot) {
	hash := ComputeSlotHash(slot)
	copy(slot.Data[HashOffset:HashOffset+HashSize], hash[:])
}
