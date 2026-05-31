package core

import (
	"encoding/binary"
	"fmt"
)

// EquipmentSlotKind identifies a writable equipment slot within ChrAsmEquipment.
//
// Phase 7b.0 — backend-only foundation for weapon/ammo slots (0–9, hash 7) and
// armor slots (12–15, hash 8).
// Phase 7c — extends the writer to talisman slots (17–21, hash 8). Talisman5
// (index 21) accepts only the clear sentinel because vanilla Elden Ring caps
// the Talisman Pouch at 4 active slots; non-empty Talisman5 writes are rejected
// by the resolver at the apply layer (see app_templates_v2_apply.go).
// The unknown slots 10/11/16 and EquippedGreatRune remain out of scope.
type EquipmentSlotKind int

const (
	EquipSlotLeftHandArmament1 EquipmentSlotKind = iota
	EquipSlotRightHandArmament1
	EquipSlotLeftHandArmament2
	EquipSlotRightHandArmament2
	EquipSlotLeftHandArmament3
	EquipSlotRightHandArmament3
	EquipSlotArrows1
	EquipSlotBolts1
	EquipSlotArrows2
	EquipSlotBolts2
	EquipSlotHead
	EquipSlotChest
	EquipSlotArms
	EquipSlotLegs
	EquipSlotTalisman1
	EquipSlotTalisman2
	EquipSlotTalisman3
	EquipSlotTalisman4
	EquipSlotTalisman5
)

// equipmentSlotKindClass classifies what handle type a slot accepts.
type equipmentSlotKindClass int

const (
	slotClassWeapon   equipmentSlotKindClass = iota // accepts handle prefix 0x80 (ItemTypeWeapon)
	slotClassAmmo                                   // accepts handle prefix 0xB0 (ItemTypeItem / goods)
	slotClassArmor                                  // accepts handle prefix 0x90 (ItemTypeArmor)
	slotClassTalisman                               // accepts handle prefix 0xA0 (ItemTypeAccessory)
)

// equipmentSlotInfo maps a slot kind to its index in ChrAsmEquipment and its class.
type equipmentSlotInfo struct {
	index int                    // 0..21 within ChrAsmEquipment
	class equipmentSlotKindClass // expected handle class
}

var equipmentSlotTable = map[EquipmentSlotKind]equipmentSlotInfo{
	EquipSlotLeftHandArmament1:  {0, slotClassWeapon},
	EquipSlotRightHandArmament1: {1, slotClassWeapon},
	EquipSlotLeftHandArmament2:  {2, slotClassWeapon},
	EquipSlotRightHandArmament2: {3, slotClassWeapon},
	EquipSlotLeftHandArmament3:  {4, slotClassWeapon},
	EquipSlotRightHandArmament3: {5, slotClassWeapon},
	EquipSlotArrows1:            {6, slotClassAmmo},
	EquipSlotBolts1:             {7, slotClassAmmo},
	EquipSlotArrows2:            {8, slotClassAmmo},
	EquipSlotBolts2:             {9, slotClassAmmo},
	EquipSlotHead:               {12, slotClassArmor},
	EquipSlotChest:              {13, slotClassArmor},
	EquipSlotArms:               {14, slotClassArmor},
	EquipSlotLegs:               {15, slotClassArmor},
	EquipSlotTalisman1:          {17, slotClassTalisman},
	EquipSlotTalisman2:          {18, slotClassTalisman},
	EquipSlotTalisman3:          {19, slotClassTalisman},
	EquipSlotTalisman4:          {20, slotClassTalisman},
	EquipSlotTalisman5:          {21, slotClassTalisman},
}

// EquipmentWrite is one entry in a WriteEquipment batch. Handle == 0 clears
// the slot (writes 0xFFFFFFFF).
type EquipmentWrite struct {
	Slot   EquipmentSlotKind
	Handle uint32
}

// WriteEquipment applies a batch of equipment slot writes atomically.
//
// Semantics:
//   - All writes are validated before any byte is mutated. If any write fails
//     validation, the slot data and hash bytes remain unchanged.
//   - Handle == 0 clears the slot to 0xFFFFFFFF.
//   - Non-zero handles must exist in slot.GaMap and match the slot's class
//     (weapon / ammo / armor). The 0xC0 AoW handles are rejected for weapon
//     slots in Phase 7b.0 — equipping an Ash of War as a weapon is out of
//     scope, even though the read-side encoding rule would technically accept
//     it.
//   - After a successful write, hash 7 is recomputed if any slot 0–9 was
//     touched, and hash 8 is recomputed if any slot 12–15 was touched. Other
//     hash entries are not modified.
//
// Concurrency: callers that share a SaveSlot across goroutines must hold the
// slot-level lock for the entire WriteEquipment call.
func (s *SaveSlot) WriteEquipment(writes []EquipmentWrite) error {
	if s == nil {
		return fmt.Errorf("WriteEquipment: nil slot")
	}
	if s.EquipItemsIDOffset <= 0 {
		return fmt.Errorf("WriteEquipment: EquipItemsIDOffset not parsed")
	}
	if s.EquipItemsIDOffset+ChrAsmEquipmentSize > len(s.Data) {
		return fmt.Errorf("WriteEquipment: ChrAsmEquipment section out of bounds")
	}
	if HashOffset+HashSize > len(s.Data) {
		return fmt.Errorf("WriteEquipment: hash block out of bounds")
	}
	if len(writes) == 0 {
		return nil
	}

	// Validate every write first; record resolved (index, encoded value) tuples.
	type resolved struct {
		index   int
		encoded uint32
	}
	resolvedWrites := make([]resolved, 0, len(writes))
	seenIndex := make(map[int]int, len(writes)) // index → position in writes for duplicate-detection diagnostics

	for i, w := range writes {
		info, ok := equipmentSlotTable[w.Slot]
		if !ok {
			return fmt.Errorf("WriteEquipment[%d]: unsupported slot kind %d (slots 10/11/16, spells, quick items, great rune, and unknown slots are out of scope)", i, int(w.Slot))
		}
		if prev, dup := seenIndex[info.index]; dup {
			return fmt.Errorf("WriteEquipment[%d]: slot index %d already written at writes[%d]", i, info.index, prev)
		}
		seenIndex[info.index] = i

		encoded, err := s.encodeEquipmentValue(w.Handle, info.class)
		if err != nil {
			return fmt.Errorf("WriteEquipment[%d]: %w", i, err)
		}
		resolvedWrites = append(resolvedWrites, resolved{index: info.index, encoded: encoded})
	}

	// All writes valid — perform mutation.
	touchedHash7 := false
	touchedHash8 := false
	for _, r := range resolvedWrites {
		off := s.EquipItemsIDOffset + r.index*4
		binary.LittleEndian.PutUint32(s.Data[off:], r.encoded)
		if r.index <= 9 {
			touchedHash7 = true
		} else if r.index >= 12 && r.index <= 15 {
			touchedHash8 = true
		} else if r.index >= 17 && r.index <= 21 {
			touchedHash8 = true
		}
	}

	// Recompute affected hash entries only. We read the full equipment section
	// fresh from slot.Data and re-derive the relevant hash entries, then patch
	// the matching 4-byte slots inside the hash block. Unrelated entries
	// (level/stats/etc.) are untouched.
	if touchedHash7 || touchedHash8 {
		section := readEquipSection(s.Data, s.EquipItemsIDOffset)
		if touchedHash7 {
			weaponIDs := extractSlots(section, weaponSlotIndices)
			binary.LittleEndian.PutUint32(s.Data[HashOffset+7*4:], equipmentHash(weaponIDs))
		}
		if touchedHash8 {
			armorIDs := extractSlots(section, armorSlotIndices)
			binary.LittleEndian.PutUint32(s.Data[HashOffset+8*4:], equipmentHash(armorIDs))
		}
	}

	return nil
}

// encodeEquipmentValue validates a handle for the given slot class and returns
// the encoded item-ID form to write into the ChrAsmEquipment slot.
//
// Encoding rules (mirror IsHandleEquipped's candidate set):
//
//	weapon:   itemID | 0x80000000 (itemID resolved via slot.GaMap[handle])
//	armor:    itemID | 0x80000000 (itemID resolved via slot.GaMap[handle])
//	ammo:     itemID directly (goods item IDs are stored as 0x40XXXXXX in GaMap)
//	talisman: itemID directly (talisman item IDs are stored as 0x20XXXXXX in GaMap)
//
// Handle == 0 → 0xFFFFFFFF (clear slot), no GaMap lookup.
func (s *SaveSlot) encodeEquipmentValue(handle uint32, class equipmentSlotKindClass) (uint32, error) {
	if handle == 0 {
		return 0xFFFFFFFF, nil
	}
	if handle == GaHandleInvalid {
		return 0, fmt.Errorf("handle 0xFFFFFFFF is invalid; use Handle=0 to clear a slot")
	}

	prefix := handle & GaHandleTypeMask
	switch class {
	case slotClassWeapon:
		switch prefix {
		case ItemTypeWeapon:
			// accepted below
		case ItemTypeAow:
			return 0, fmt.Errorf("handle 0x%08X has Ash of War prefix 0xC0; AoW equipping is out of scope for Phase 7b.0", handle)
		case ItemTypeArmor:
			return 0, fmt.Errorf("handle 0x%08X has armor prefix 0x90; cannot equip armor in a weapon slot", handle)
		case ItemTypeAccessory:
			return 0, fmt.Errorf("handle 0x%08X has talisman prefix 0xA0; cannot equip talisman in a weapon slot", handle)
		case ItemTypeItem:
			return 0, fmt.Errorf("handle 0x%08X has goods prefix 0xB0; cannot equip goods in a weapon slot", handle)
		default:
			return 0, fmt.Errorf("handle 0x%08X has unknown type prefix 0x%X for weapon slot", handle, prefix>>28)
		}
	case slotClassArmor:
		switch prefix {
		case ItemTypeArmor:
			// accepted below
		case ItemTypeWeapon:
			return 0, fmt.Errorf("handle 0x%08X has weapon prefix 0x80; cannot equip weapon in an armor slot", handle)
		case ItemTypeAow:
			return 0, fmt.Errorf("handle 0x%08X has Ash of War prefix 0xC0; cannot equip AoW in an armor slot", handle)
		case ItemTypeAccessory:
			return 0, fmt.Errorf("handle 0x%08X has talisman prefix 0xA0; cannot equip talisman in an armor slot", handle)
		case ItemTypeItem:
			return 0, fmt.Errorf("handle 0x%08X has goods prefix 0xB0; cannot equip goods in an armor slot", handle)
		default:
			return 0, fmt.Errorf("handle 0x%08X has unknown type prefix 0x%X for armor slot", handle, prefix>>28)
		}
	case slotClassAmmo:
		switch prefix {
		case ItemTypeItem:
			// accepted below
		case ItemTypeWeapon:
			return 0, fmt.Errorf("handle 0x%08X has weapon prefix 0x80; cannot equip weapon in an ammo slot", handle)
		case ItemTypeArmor:
			return 0, fmt.Errorf("handle 0x%08X has armor prefix 0x90; cannot equip armor in an ammo slot", handle)
		case ItemTypeAow:
			return 0, fmt.Errorf("handle 0x%08X has Ash of War prefix 0xC0; cannot equip AoW in an ammo slot", handle)
		case ItemTypeAccessory:
			return 0, fmt.Errorf("handle 0x%08X has talisman prefix 0xA0; cannot equip talisman in an ammo slot", handle)
		default:
			return 0, fmt.Errorf("handle 0x%08X has unknown type prefix 0x%X for ammo slot", handle, prefix>>28)
		}
	case slotClassTalisman:
		switch prefix {
		case ItemTypeAccessory:
			// accepted below
		case ItemTypeWeapon:
			return 0, fmt.Errorf("handle 0x%08X has weapon prefix 0x80; cannot equip weapon in a talisman slot", handle)
		case ItemTypeArmor:
			return 0, fmt.Errorf("handle 0x%08X has armor prefix 0x90; cannot equip armor in a talisman slot", handle)
		case ItemTypeAow:
			return 0, fmt.Errorf("handle 0x%08X has Ash of War prefix 0xC0; cannot equip AoW in a talisman slot", handle)
		case ItemTypeItem:
			return 0, fmt.Errorf("handle 0x%08X has goods prefix 0xB0; cannot equip goods in a talisman slot", handle)
		default:
			return 0, fmt.Errorf("handle 0x%08X has unknown type prefix 0x%X for talisman slot", handle, prefix>>28)
		}
	default:
		return 0, fmt.Errorf("internal: unknown slot class %d", int(class))
	}

	itemID, ok := s.GaMap[handle]
	if !ok || itemID == 0 || itemID == GaHandleInvalid {
		return 0, fmt.Errorf("handle 0x%08X not present in inventory (GaMap)", handle)
	}

	switch class {
	case slotClassWeapon, slotClassArmor:
		return itemID | ItemTypeWeapon, nil
	case slotClassAmmo, slotClassTalisman:
		return itemID, nil
	}
	return 0, fmt.Errorf("internal: unreachable encoding path for class %d", int(class))
}
