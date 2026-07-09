package core

import (
	"encoding/binary"
	"fmt"

	"github.com/oisis/EldenRing-SaveForge/backend/db"
)

// AoW repair primitives back the current_aow_* repair actions surfaced by the
// scanner (current_aow_missing, current_aow_shared, current_aow_non_aow_category).
//
// Compatibility rules are NOT duplicated here — the single source of truth is
// db.IsAshOfWarCompatibleWithWeapon, exactly as the workspace save path uses in
// backend/editor/save.go::validatePendingAoWChanges. Core cannot import editor
// (import cycle), so the thin validate wrapper is re-expressed here but still
// delegates the actual rule to db.
//
// All three primitives are atomic at the caller level: every compatibility and
// capacity check is read-only and runs BEFORE any mutation, so an
// incompatibility or capacity failure leaves slot.Data untouched.

// ClearWeaponAoW removes the Ash of War from a weapon, writing the canonical
// no-custom sentinel in-place. Backs the clear_aow action for all three
// current_aow_* codes. Always legal — no compatibility check needed.
func ClearWeaponAoW(slot *SaveSlot, weaponHandle uint32) error {
	if slot == nil {
		return fmt.Errorf("ClearWeaponAoW: nil slot")
	}
	return PatchWeaponAoWHandle(slot, weaponHandle, NoCustomAoWHandle)
}

// AttachExistingWeaponAoW attaches an existing, free AoW GaItem (identified by
// its 0xC0 handle) to the weapon. Backs attach_existing_aow. Compatibility is
// validated against the exact weapon before mutation; PatchWeaponAoWHandle then
// enforces that the AoW GaItem exists and is not already referenced by another
// weapon, and writes exactly 4 bytes on success — so a rejection never mutates.
func AttachExistingWeaponAoW(slot *SaveSlot, weaponHandle, aowHandle uint32) error {
	if slot == nil {
		return fmt.Errorf("AttachExistingWeaponAoW: nil slot")
	}
	aowItemID, err := aowItemIDForHandle(slot, aowHandle)
	if err != nil {
		return fmt.Errorf("AttachExistingWeaponAoW: %w", err)
	}
	if err := validateAoWWeaponCompat(slot, weaponHandle, aowItemID); err != nil {
		return fmt.Errorf("AttachExistingWeaponAoW: %w", err)
	}
	return PatchWeaponAoWHandle(slot, weaponHandle, aowHandle)
}

// CreateWeaponAoWCopy allocates a fresh AoW GaItem for aowItemID and attaches it
// to the weapon. Backs create_new_aow_copy (default for current_aow_shared).
// Compatibility and GaItems/GaItemData capacity are prechecked read-only before
// PatchWeaponAoW runs, so an incompatible AoW or a full slot reports an error
// with no partial mutation.
func CreateWeaponAoWCopy(slot *SaveSlot, weaponHandle, aowItemID uint32) error {
	if slot == nil {
		return fmt.Errorf("CreateWeaponAoWCopy: nil slot")
	}
	if err := validateAoWWeaponCompat(slot, weaponHandle, aowItemID); err != nil {
		return fmt.Errorf("CreateWeaponAoWCopy: %w", err)
	}
	if err := checkAoWCopyCapacity(slot, aowItemID); err != nil {
		return fmt.Errorf("CreateWeaponAoWCopy: %w", err)
	}
	return PatchWeaponAoW(slot, weaponHandle, aowItemID)
}

// ---- shared helpers ---------------------------------------------------------

// validateAoWWeaponCompat mirrors editor.validatePendingAoWChanges: the AoW must
// resolve in the DB with category ashes_of_war and be compatible with the exact
// weapon. Fail-closed on unknown compatibility (DLC weapons not yet wired into
// the compat table) so we never produce a state the game can't load.
func validateAoWWeaponCompat(slot *SaveSlot, weaponHandle, aowItemID uint32) error {
	weaponItemID, err := weaponItemIDForHandle(slot, weaponHandle)
	if err != nil {
		return err
	}
	if aowItemID>>28 != 8 {
		return fmt.Errorf("itemID 0x%08X is not an Ash of War item ID", aowItemID)
	}
	aowData, _ := db.GetItemDataFuzzy(aowItemID)
	if aowData.Name == "" {
		return fmt.Errorf("AoW 0x%08X unknown in item DB", aowItemID)
	}
	if aowData.Category != "ashes_of_war" {
		return fmt.Errorf("AoW 0x%08X (%s) is category %q, not ashes_of_war", aowItemID, aowData.Name, aowData.Category)
	}
	compat, known := db.IsAshOfWarCompatibleWithWeapon(aowItemID, weaponItemID)
	if !known {
		return fmt.Errorf("AoW/weapon compatibility unknown for AoW %s (0x%08X) on weapon 0x%08X — refusing fail-closed",
			aowData.Name, aowItemID, weaponItemID)
	}
	if !compat {
		return fmt.Errorf("AoW %s (0x%08X) is not compatible with weapon 0x%08X",
			aowData.Name, aowItemID, weaponItemID)
	}
	return nil
}

// CurrentWeaponAoWItemID resolves the itemID of the Ash of War currently
// attached to weaponHandle, via the weapon GaItem's AoWGaItemHandle → GaMap.
// ok=false when the weapon is absent, carries no custom AoW, or the AoW handle
// is unmapped. Used by the repair apply endpoint to derive the copy source for
// create_new_aow_copy on a shared AoW without asking the UI to supply it.
func CurrentWeaponAoWItemID(slot *SaveSlot, weaponHandle uint32) (uint32, bool) {
	if slot == nil {
		return 0, false
	}
	for i := range slot.GaItems {
		g := &slot.GaItems[i]
		if g.IsEmpty() || g.Handle != weaponHandle {
			continue
		}
		if IsNoCustomAoWHandle(g.AoWGaItemHandle) {
			return 0, false
		}
		id, ok := slot.GaMap[g.AoWGaItemHandle]
		if !ok || id == 0 {
			return 0, false
		}
		return id, true
	}
	return 0, false
}

// weaponItemIDForHandle resolves a weapon handle to its GaItem itemID.
func weaponItemIDForHandle(slot *SaveSlot, weaponHandle uint32) (uint32, error) {
	if weaponHandle&GaHandleTypeMask != ItemTypeWeapon {
		return 0, fmt.Errorf("handle 0x%08X is not a weapon handle", weaponHandle)
	}
	for i := range slot.GaItems {
		g := &slot.GaItems[i]
		if !g.IsEmpty() && g.Handle == weaponHandle {
			return g.ItemID, nil
		}
	}
	return 0, fmt.Errorf("weapon handle 0x%08X not found in GaItems", weaponHandle)
}

// aowItemIDForHandle resolves an AoW handle to its GaItem itemID.
func aowItemIDForHandle(slot *SaveSlot, aowHandle uint32) (uint32, error) {
	if aowHandle&GaHandleTypeMask != ItemTypeAow {
		return 0, fmt.Errorf("handle 0x%08X is not an AoW handle", aowHandle)
	}
	for i := range slot.GaItems {
		g := &slot.GaItems[i]
		if !g.IsEmpty() && g.Handle == aowHandle {
			return g.ItemID, nil
		}
	}
	return 0, fmt.Errorf("AoW handle 0x%08X not found in GaItems", aowHandle)
}

// checkAoWCopyCapacity verifies room for one fresh AoW GaItem (and its
// GaItemData entry, if the itemID needs one and is not already present) before
// any allocation happens.
func checkAoWCopyCapacity(slot *SaveSlot, aowItemID uint32) error {
	usage := CountSlotUsage(slot)
	if usage.GaItemsMax-usage.GaItemsUsed < 1 {
		return fmt.Errorf("no free GaItems capacity (used %d/%d): gaitem_full",
			usage.GaItemsUsed, usage.GaItemsMax)
	}
	if needsGaItemData(aowItemID) && !gaItemDataContains(slot, aowItemID) {
		if usage.GaItemDataMax-usage.GaItemDataUsed < 1 {
			return fmt.Errorf("no free GaItemData capacity (used %d/%d): gaitemdata_full",
				usage.GaItemDataUsed, usage.GaItemDataMax)
		}
	}
	return nil
}

// gaItemDataContains reports whether the GaItemData array already holds an entry
// for itemID (in which case no new GaItemData slot is consumed by an upsert).
func gaItemDataContains(slot *SaveSlot, itemID uint32) bool {
	off := slot.GaItemDataOffset
	if off <= 0 || off+GaItemDataArrayOff > len(slot.Data) {
		return false
	}
	count := int(binary.LittleEndian.Uint32(slot.Data[off:]))
	if count <= 0 || count > GaItemDataMaxCount {
		return false
	}
	arrayBase := off + GaItemDataArrayOff
	for i := 0; i < count; i++ {
		entryOff := arrayBase + i*GaItemDataEntryLen
		if entryOff+4 > len(slot.Data) {
			break
		}
		if binary.LittleEndian.Uint32(slot.Data[entryOff:]) == itemID {
			return true
		}
	}
	return false
}
