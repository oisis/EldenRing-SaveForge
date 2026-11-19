package db

import "github.com/oisis/EldenRing-SaveForge/backend/db/data"

// ItemUnarmed is the technical "no armament equipped" weapon row (EquipParamWeapon
// row 110000). It is a real placeholder the game stores in inventory, but it has
// no weapon family: its originEquipWep is -1, so the generator records no
// confirmed relation for it. See ResolveWeaponIdentity for the explicit branch.
const ItemUnarmed uint32 = 0x0001ADB0

const (
	// weaponAffinityStep is the EquipParamWeapon row spacing between a family's
	// affinity anchors. Only those anchors are materialized as rows; upgrade
	// levels 1..25 are derived through ReinforceParamWeapon and never exist as
	// rows of their own, so itemID%weaponAffinityStep is exactly the level.
	weaponAffinityStep = 100
	// weaponMaxAffinityOffset is the Occult offset, the highest of the 13.
	weaponMaxAffinityOffset = 1200
)

// WeaponIdentity is the canonical decomposition of a saved weapon item ID into
// the family it belongs to, the affinity applied to it and its upgrade level.
type WeaponIdentity struct {
	BaseID         uint32 // canonical family base row (the Standard +0 item)
	AffinityOffset uint32 // 0, 100, ... 1200
	AffinityName   string // "" for Standard, otherwise a db.InfuseTypes name
	Level          int    // upgrade level as stored (itemID % 100); validity is editor.Validate's call
}

// ResolveWeaponIdentity maps any weapon item ID a save may hold — a family base,
// an affinity anchor, or either of those upgraded — onto its canonical family
// base, affinity and upgrade level.
//
// The family relation comes from exactly one place: EquipParamWeapon.originEquipWep,
// generated into data.WeaponGemMounts. Item names, ReinforceTypeID and
// "nearest lower base" range scans are all unusable as identity: names are
// presentation, ReinforceTypeID is a ReinforceParamWeapon band (somber weapons
// carry 2200 regardless of affinity), and a range scan silently picks the
// affinity anchor instead of the family base, which decodes Cold +5 as
// Standard +5.
//
// Resolution is fail-closed on identity. An ID whose anchor is not a materialized
// row, whose family relation is unconfirmed, whose affinity offset is not one of
// the 13, or whose base is not an app weapon, resolves to (zero, false) rather
// than to a different valid weapon.
func ResolveWeaponIdentity(itemID uint32) (WeaponIdentity, bool) {
	// Weapon rows live in the 0x0... item-ID space. Armor, talismans, goods and
	// Ashes of War carry their own prefix and are never weapon identities.
	if itemID&0xF0000000 != 0 {
		return WeaponIdentity{}, false
	}
	// Unarmed is a technical placeholder with no family (originEquipWep == -1).
	// It resolves to itself at level 0 so callers keep treating it as the known
	// placeholder it is, instead of an unknown item.
	if itemID == ItemUnarmed {
		return WeaponIdentity{BaseID: ItemUnarmed}, true
	}

	anchorID := itemID - itemID%weaponAffinityStep
	level := int(itemID % weaponAffinityStep)

	mount, ok := data.WeaponGemMounts[anchorID]
	if !ok || mount.OriginEquipWep == 0 {
		return WeaponIdentity{}, false
	}
	baseID := mount.OriginEquipWep
	if anchorID < baseID {
		return WeaponIdentity{}, false
	}
	offset := anchorID - baseID
	if offset > weaponMaxAffinityOffset || offset%weaponAffinityStep != 0 {
		return WeaponIdentity{}, false
	}
	affinity, ok := infusionNameForOffset(offset)
	if !ok {
		return WeaponIdentity{}, false
	}

	if !isWeaponCategoryItem(baseID) {
		return WeaponIdentity{}, false
	}
	// An affinity offset on a weapon the regulation forbids infusing is not a
	// valid item ID, so it must not be presented as one.
	if offset != 0 && !GetItemData(baseID).CanChangeAffinity {
		return WeaponIdentity{}, false
	}
	// Level is reported as stored, never clamped or rejected here. Whether it
	// exceeds the family's MaxUpgrade is a validation question owned by
	// editor.Validate (CodeUpgradeOutOfRange), which is what makes an
	// over-upgraded weapon a repairable known item instead of an unknown one.

	return WeaponIdentity{
		BaseID:         baseID,
		AffinityOffset: offset,
		AffinityName:   affinity,
		Level:          level,
	}, true
}

// isWeaponCategoryItem reports whether id is an app weapon entry, i.e. belongs to
// one of the three editable weapon categories. Arrows and bolts share the weapon
// ID space but are ammunition and are deliberately excluded.
func isWeaponCategoryItem(id uint32) bool {
	for _, m := range []map[uint32]data.ItemData{data.Weapons, data.RangedAndCatalysts, data.Shields} {
		if item, ok := m[id]; ok && item.Name != "" {
			return true
		}
	}
	return false
}

// infusionNameForOffset maps an affinity offset onto its db.InfuseTypes name.
// Standard resolves to the empty string, matching the convention every caller
// already uses for "not infused".
func infusionNameForOffset(offset uint32) (string, bool) {
	for _, t := range InfuseTypes {
		if uint32(t.Offset) == offset {
			if t.Name == "Standard" {
				return "", true
			}
			return t.Name, true
		}
	}
	return "", false
}

// IsValidInfuseOffset reports whether offset is one of the affinity offsets in
// InfuseTypes (Standard = 0 included). Any other value would encode a
// non-existent weapon ID.
func IsValidInfuseOffset(offset int) bool {
	for _, infuse := range InfuseTypes {
		if infuse.Offset == offset {
			return true
		}
	}
	return false
}

// WeaponAcceptsBatchAffinity reports whether a batch-selected affinity offset may
// be added to the weapon row id: the regulation must allow affinity changes and
// the row must not be a catalogued pre-infused variant that already carries its
// own affinity.
func WeaponAcceptsBatchAffinity(id uint32) bool {
	if !CanWeaponChangeAffinity(id) {
		return false
	}
	identity, ok := ResolveWeaponIdentity(id)
	return ok && identity.AffinityOffset == 0
}
