package data

// weapon_type_subcat.go — canonical wepType → sub-category maps.
//
// Sub-category classification is driven by the game's authoritative
// EquipParamWeapon.wepType (see weapon_weptype_generated.go), not by weapon-name
// heuristics. The name-based classifiers in melee_subcat.go /
// ranged_and_catalysts_subcat.go / shields_subcat.go remain only as a fallback
// for items missing from the wepType table.
//
// Maps are split per tab so a wepType can only resolve to a sub-group that
// actually belongs to that tab. This deliberately leaves cross-category source
// mislabels untouched: e.g. "Rotten Staff" sits in the Ranged/Catalysts tab but
// its canonical wepType is 41 (Colossal Weapon, a melee type). Since 41 is not a
// ranged sub-group it is not remapped here — that is a Category-level fix,
// outside the scope of sub-category labeling.

// meleeWepTypeSubcat maps melee EquipParamWeapon.wepType values to Melee
// Armaments sub-groups. wepType 33 (Unarmed) is intentionally absent — it has no
// sub-group and falls through to the name fallback.
var meleeWepTypeSubcat = map[uint16]string{
	1:  SubcatMeleeDaggers,
	3:  SubcatMeleeStraightSwords,
	5:  SubcatMeleeGreatswords,
	7:  SubcatMeleeColossalSwords,
	9:  SubcatMeleeCurvedSwords,
	11: SubcatMeleeCurvedGreatswords,
	13: SubcatMeleeKatanas,
	14: SubcatMeleeTwinblades,
	15: SubcatMeleeThrustingSwords,
	16: SubcatMeleeHeavyThrustingSwords,
	17: SubcatMeleeAxes,
	19: SubcatMeleeGreataxes,
	21: SubcatMeleeHammers,
	23: SubcatMeleeGreatHammers,
	24: SubcatMeleeFlails,
	25: SubcatMeleeSpears,
	28: SubcatMeleeGreatSpears,
	29: SubcatMeleeHalberds,
	31: SubcatMeleeReapers,
	35: SubcatMeleeFists,
	37: SubcatMeleeClaws,
	39: SubcatMeleeWhips,
	41: SubcatMeleeColossalWeapons,
	88: SubcatMeleeHandToHand,
	89: SubcatMeleePerfumeBottles,
	91: SubcatMeleeThrowingBlades,
	92: SubcatMeleeBackhandBlades,
	93: SubcatMeleeLightGreatswords,
	94: SubcatMeleeGreatKatanas,
	95: SubcatMeleeBeastClaws,
}

// rangedWepTypeSubcat maps ranged/catalyst wepType values to their sub-groups.
var rangedWepTypeSubcat = map[uint16]string{
	50: SubcatRangedLightBows,
	51: SubcatRangedBows,
	53: SubcatRangedGreatbows,
	55: SubcatRangedCrossbows,
	56: SubcatRangedBallistas,
	57: SubcatRangedGlintstoneStaffs,
	61: SubcatRangedSacredSeals,
}

// shieldWepTypeSubcat maps shield wepType values to their sub-groups.
var shieldWepTypeSubcat = map[uint16]string{
	65: SubcatShieldsSmall,
	67: SubcatShieldsMedium,
	69: SubcatShieldsGreatshields,
	87: SubcatShieldsTorches,
	90: SubcatShieldsThrusting,
}

// wepTypeSubcat returns the canonical sub-category for an item ID within the
// given per-tab map, plus whether a mapping exists.
func wepTypeSubcat(id uint32, table map[uint16]string) (string, bool) {
	wt, ok := weaponWepType[id]
	if !ok {
		return "", false
	}
	sc, ok := table[wt]
	return sc, ok
}
