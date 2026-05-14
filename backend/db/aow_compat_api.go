package db

import (
	"sort"

	"github.com/oisis/EldenRing-SaveForge/backend/db/data"
)

// AoWRejectReason explains why a CheckAoWCompatibility call returned false.
// Callers (UI, save-write guards) should use this to surface actionable errors
// to the user instead of a generic "incompatible" message.
type AoWRejectReason int

const (
	// AoWOK signals the AoW can be mounted on the weapon.
	AoWOK AoWRejectReason = iota
	// AoWWeaponNotInfusable — weapon's GemMountType != 2 (unique/somber/no-mount).
	AoWWeaponNotInfusable
	// AoWNotApplicableToWeaponCategory — weapon's wepType is known but the AoW
	// does not advertise compatibility for it.
	AoWNotApplicableToWeaponCategory
	// AoWUnknownWeapon — weapon item ID not found in the local DB at all.
	AoWUnknownWeapon
	// AoWUnknownAoW — AoW item ID not in any compat data source.
	AoWUnknownAoW
	// AoWUnknownWeaponWepType — weapon's wepType has no entry in WepTypeToCanMountBit
	// AND no DLC fallback covers it. Fail-closed: caller must block.
	AoWUnknownWeaponWepType
	// AoWMissingParamData — generic data-gap sentinel for future expansion.
	AoWMissingParamData
)

// String returns a human-readable label for a reject reason.
func (r AoWRejectReason) String() string {
	switch r {
	case AoWOK:
		return "ok"
	case AoWWeaponNotInfusable:
		return "weapon_not_infusable"
	case AoWNotApplicableToWeaponCategory:
		return "ash_not_applicable_to_weapon_category"
	case AoWUnknownWeapon:
		return "unknown_weapon"
	case AoWUnknownAoW:
		return "unknown_ash"
	case AoWUnknownWeaponWepType:
		return "unknown_weapon_wep_type"
	case AoWMissingParamData:
		return "missing_param_data"
	}
	return "unknown"
}

// CheckAoWCompatibility decides whether `aowID` can be mounted on `weaponID`.
// Designed as the canonical backend gate; UI and save-write paths should both
// call this and respect the reject reason.
//
// Algorithm (Phase 1.6 — 3-layer):
//
//	0. Remove (aowID == 0) is always allowed → (true, AoWOK).
//	1. Resolve weapon via GetItemDataFuzzy; require GemMountType == 2 (otherwise
//	   AoWWeaponNotInfusable / AoWUnknownWeapon).
//	2. Layer 1+2: standard 40-bit canMountWep mask. If the weapon's wepType maps
//	   to a bit and that bit is set in AoWCompatMasks[aowID], return true.
//	3. Layer 3: DLC fallback via mountWepTextId + swordArtsParamId. If the AoW
//	   has an AoWDLCFallbackWepTypes entry listing the weapon's wepType, return
//	   true. (Layer 3 is OR with Layer 1+2 — generators precompute it.)
//	4. Otherwise fail-closed with the most specific reason available.
//
// Fail-closed everywhere: unknown weapon, unknown AoW, unmapped wepType all
// produce false. The caller MUST NOT treat false-unknown as compatible.
func CheckAoWCompatibility(aowID, weaponID uint32) (bool, AoWRejectReason) {
	if aowID == 0 {
		return true, AoWOK
	}

	weaponData, _ := GetItemDataFuzzy(weaponID)
	if weaponData.WepType == 0 && weaponData.GemMountType == 0 {
		return false, AoWUnknownWeapon
	}
	if weaponData.GemMountType != 2 {
		return false, AoWWeaponNotInfusable
	}

	mask, hasMask := data.AoWCompatMasks[aowID]
	fallback, hasFallback := data.AoWDLCFallbackWepTypes[aowID]
	if !hasMask && !hasFallback {
		return false, AoWUnknownAoW
	}

	bitPos, bitMapped := data.WepTypeToCanMountBit[weaponData.WepType]
	if bitMapped && hasMask && (mask>>bitPos)&1 == 1 {
		return true, AoWOK
	}

	if hasFallback {
		for _, wt := range fallback {
			if wt == weaponData.WepType {
				return true, AoWOK
			}
		}
	}

	// Final dispositioning: distinguish "weapon wepType is globally unknown" (data
	// gap → AoWUnknownWeaponWepType) from "wepType is known by SOME AoW but not this
	// one" (correct rejection → AoWNotApplicableToWeaponCategory). A wepType is
	// globally known if it appears in WepTypeToCanMountBit OR in any MwtidGroups list.
	if isWepTypeGloballyKnown(weaponData.WepType) {
		return false, AoWNotApplicableToWeaponCategory
	}
	return false, AoWUnknownWeaponWepType
}

// isWepTypeGloballyKnown returns true if the given wepType is referenced by at
// least one compatibility mechanism in the local data (vanilla bit map OR a
// Layer 3 mwtid group). Used to give precise reject reasons.
func isWepTypeGloballyKnown(wt uint16) bool {
	if _, ok := data.WepTypeToCanMountBit[wt]; ok {
		return true
	}
	for _, wepTypes := range data.AoWMwtidGroups {
		for _, w := range wepTypes {
			if w == wt {
				return true
			}
		}
	}
	return false
}

// IsAffinityAllowedForAoW reports whether the given affinity ID (0..23) is in
// the AoW's configurableWepAttr mask. False when the AoW is not in the affinity
// table or the bit is unset.
func IsAffinityAllowedForAoW(aowID uint32, affinityID uint8) bool {
	if affinityID >= 24 {
		return false
	}
	mask, ok := data.AoWAffinityMasks[aowID]
	if !ok {
		return false
	}
	return (mask>>affinityID)&1 == 1
}

// GetDefaultAffinityForAoW returns the AoW's vanilla default affinity ID
// (e.g. Hoarfrost Stomp → 11 = Cold). The boolean is false if the AoW is not
// in the affinity table.
func GetDefaultAffinityForAoW(aowID uint32) (uint8, bool) {
	v, ok := data.AoWDefaultAffinity[aowID]
	return v, ok
}

// GetCompatibleAshesForWeapon returns every AoW in the local item DB that
// CheckAoWCompatibility accepts for the given weapon. Results are sorted by
// item ID for stable output (callers usually re-sort by name in the UI).
func GetCompatibleAshesForWeapon(weaponID uint32) []data.ItemData {
	var out []data.ItemData
	type entry struct {
		id   uint32
		item data.ItemData
	}
	var matches []entry
	for id, aow := range data.Aows {
		ok, _ := CheckAoWCompatibility(id, weaponID)
		if ok {
			matches = append(matches, entry{id, aow})
		}
	}
	sort.Slice(matches, func(i, j int) bool { return matches[i].id < matches[j].id })
	out = make([]data.ItemData, 0, len(matches))
	for _, m := range matches {
		out = append(out, m.item)
	}
	return out
}

// GetCompatibleWeaponsForAoW returns every weapon in the local item DB that
// CheckAoWCompatibility accepts for the given AoW. Scans the union of
// Weapons + Shields + RangedAndCatalysts to cover all gem-mountable categories.
func GetCompatibleWeaponsForAoW(aowID uint32) []data.ItemData {
	var out []data.ItemData
	type entry struct {
		id   uint32
		item data.ItemData
	}
	var matches []entry
	for _, m := range []map[uint32]data.ItemData{data.Weapons, data.Shields, data.RangedAndCatalysts} {
		for id, w := range m {
			ok, _ := CheckAoWCompatibility(aowID, id)
			if ok {
				matches = append(matches, entry{id, w})
			}
		}
	}
	sort.Slice(matches, func(i, j int) bool { return matches[i].id < matches[j].id })
	out = make([]data.ItemData, 0, len(matches))
	for _, m := range matches {
		out = append(out, m.item)
	}
	return out
}
