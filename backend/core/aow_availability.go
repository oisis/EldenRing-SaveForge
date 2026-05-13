package core

// AoWCopyRaw is one Ash of War GaItem found in the save slot.
type AoWCopyRaw struct {
	ItemID                  uint32 // AoW item ID (upper nibble 0x8)
	Handle                  uint32 // AoW GaItem handle (upper nibble 0xC)
	UsedByWeaponHandle      uint32 // 0 if this copy is free; weapon handle if attached
	HasSharedHandleConflict bool   // true if more than one weapon references this handle
}

// ScanAoWAvailability scans slot.GaItems and returns one AoWCopyRaw per AoW GaItem found.
//
// Pass 1 collects every AoW GaItem (handle prefix 0xC0000000) and every weapon's
// AoWGaItemHandle reference. Pass 2 cross-references them to determine which copies
// are free and whether any handle is shared by multiple weapons (save corruption indicator).
//
// Assumptions:
//   - One AoW itemID may have multiple copies (different handles).
//   - A handle must not be shared between two weapons; if it is, both copies are flagged.
//   - Only entries where !g.IsEmpty() are considered.
func ScanAoWAvailability(slot *SaveSlot) []AoWCopyRaw {
	// weaponRefs maps AoW handle → list of weapon handles that reference it.
	// Under normal game rules each AoW handle is referenced by at most one weapon.
	weaponRefs := make(map[uint32][]uint32)
	var copies []AoWCopyRaw

	for i := range slot.GaItems {
		g := &slot.GaItems[i]
		if g.IsEmpty() {
			continue
		}
		switch g.Handle & GaHandleTypeMask {
		case ItemTypeAow:
			copies = append(copies, AoWCopyRaw{ItemID: g.ItemID, Handle: g.Handle})
		case ItemTypeWeapon:
			if g.AoWGaItemHandle != 0xFFFFFFFF {
				weaponRefs[g.AoWGaItemHandle] = append(weaponRefs[g.AoWGaItemHandle], g.Handle)
			}
		}
	}

	// Cross-reference: mark each AoW copy as used or free.
	for i := range copies {
		weapons := weaponRefs[copies[i].Handle]
		if len(weapons) == 0 {
			continue
		}
		copies[i].UsedByWeaponHandle = weapons[0]
		if len(weapons) > 1 {
			// Same AoW handle pointed to by multiple weapons — save corruption indicator.
			copies[i].HasSharedHandleConflict = true
		}
	}
	return copies
}
