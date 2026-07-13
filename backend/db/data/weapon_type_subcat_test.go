package data

import "testing"

// lookupArmament finds an item across the three weapon-bearing maps.
func lookupArmament(id uint32) (ItemData, bool) {
	if it, ok := Weapons[id]; ok {
		return it, true
	}
	if it, ok := Shields[id]; ok {
		return it, true
	}
	if it, ok := RangedAndCatalysts[id]; ok {
		return it, true
	}
	return ItemData{}, false
}

// reportedWeapons — every weapon Issue #6 (DrippingSoup22) flagged as mislabeled,
// with the sub-category demanded by canonical EquipParamWeapon.wepType. All 104
// reporter claims were verified correct against regulation.bin. (The 105th,
// "Rotten Staff", is a Category-level mislabel handled separately — see
// TestWeaponSubcat_RottenStaffDiscrepancy.)
var reportedWeapons = []struct {
	id   uint32
	name string
	want string
}{
	{0x0044AA20, "Ancient Meteoric Ore Greatsword", SubcatMeleeColossalSwords},
	{0x02796470, "Ansbach's Longbow", SubcatRangedBows},
	{0x00BEE330, "Anvil Hammer", SubcatMeleeColossalWeapons},
	{0x010B5580, "Barbed Staff-Spear", SubcatMeleeGreatSpears},
	{0x00D59F80, "Battle Axe", SubcatMeleeAxes},
	{0x00B76920, "Battle Hammer", SubcatMeleeGreatHammers},
	{0x007B4A80, "Beastman's Cleaver", SubcatMeleeCurvedGreatswords},
	{0x01CAADE0, "Beastman's Jar-Shield", SubcatShieldsMedium},
	{0x00BF3150, "Bloodfiend's Arm", SubcatMeleeColossalWeapons},
	{0x00FC6160, "Bloodfiend's Fork", SubcatMeleeSpears},
	{0x00FC8870, "Bloodfiend's Sacred Spear", SubcatMeleeGreatSpears},
	{0x007A8730, "Bloodhound's Fang", SubcatMeleeCurvedGreatswords},
	{0x00F58390, "Bolt of Gransax", SubcatMeleeSpears},
	{0x0269FB20, "Bone Bow", SubcatRangedLightBows},
	{0x00ECA9F0, "Bonny Butchering Knife", SubcatMeleeGreataxes},
	{0x03B9D3B0, "Carian Thrusting Shield", SubcatShieldsThrusting},
	{0x00B916D0, "Celebrant's Skull", SubcatMeleeGreatHammers},
	{0x01426B10, "Cipher Pata", SubcatMeleeFists},
	{0x015752A0, "Claws of Night", SubcatMeleeClaws},
	{0x004C7250, "Cleanrot Knight's Sword", SubcatMeleeThrustingSwords},
	{0x02631D50, "Composite Bow", SubcatRangedLightBows},
	{0x00B85380, "Curved Great Club", SubcatMeleeGreatHammers},
	{0x0072BF00, "Dancing Blade of Ranah", SubcatMeleeCurvedSwords},
	{0x03AB06A0, "Deadly Poison Perfume Bottle", SubcatMeleePerfumeBottles},
	{0x00EC82E0, "Death Knight's Longhaft Axe", SubcatMeleeGreataxes},
	{0x00DD67B0, "Death Knight's Twin Axes", SubcatMeleeAxes},
	{0x016694E0, "Devonia's Hammer", SubcatMeleeColossalWeapons},
	{0x00BA2840, "Devourer's Scepter", SubcatMeleeGreatHammers},
	{0x007A6020, "Dismounter", SubcatMeleeCurvedGreatswords},
	{0x03B9ACA0, "Dueling Shield", SubcatShieldsThrusting},
	{0x015F9000, "Duelist Greataxe", SubcatMeleeColossalWeapons},
	{0x006C5660, "Eclipse Shotel", SubcatMeleeCurvedSwords},
	{0x015F68F0, "Envoy's Greathorn", SubcatMeleeColossalWeapons},
	{0x00A95F60, "Envoy's Horn", SubcatMeleeHammers},
	{0x00B98C00, "Envoy's Long Horn", SubcatMeleeGreatHammers},
	{0x00A037A0, "Euporia", SubcatMeleeTwinblades},
	{0x006ACFC0, "Falchion", SubcatMeleeCurvedSwords},
	{0x007297F0, "Falx", SubcatMeleeCurvedSwords},
	{0x00AF79E0, "Flowerstone Gavel", SubcatMeleeHammers},
	{0x0081DA30, "Freyja's Greatsword", SubcatMeleeCurvedGreatswords},
	{0x00E704A0, "Gargoyle's Black Axe", SubcatMeleeGreataxes},
	{0x0099F610, "Gargoyle's Black Blades", SubcatMeleeTwinblades},
	{0x00E6DD90, "Gargoyle's Great Axe", SubcatMeleeGreataxes},
	{0x0166E300, "Gazing Finger", SubcatMeleeColossalWeapons},
	{0x01607A60, "Ghiza's Wheel", SubcatMeleeColossalWeapons},
	{0x0098BD90, "Godskin Peeler", SubcatMeleeTwinblades},
	{0x003E1A70, "Godslayer's Greatsword", SubcatMeleeColossalSwords},
	{0x0160C880, "Golem's Halberd", SubcatMeleeColossalWeapons},
	{0x003E8FA0, "Grafted Blade Greatsword", SubcatMeleeColossalSwords},
	{0x015F41E0, "Great Club", SubcatMeleeColossalWeapons},
	{0x00E52FE0, "Great Omenkiller Cleaver", SubcatMeleeGreataxes},
	{0x00B9DA20, "Great Stars", SubcatMeleeGreatHammers},
	{0x01DB28A0, "Great Turtle Shell", SubcatShieldsMedium},
	{0x00456D70, "Greatsword of Radahn (Light)", SubcatMeleeColossalSwords},
	{0x00451F50, "Greatsword of Radahn (Lord)", SubcatMeleeColossalSwords},
	{0x006D19B0, "Grossmesser", SubcatMeleeCurvedSwords},
	{0x0262CF30, "Harp Bow", SubcatRangedLightBows},
	{0x00820140, "Horned Warrior's Greatsword", SubcatMeleeCurvedGreatswords},
	{0x0072E610, "Horned Warrior's Sword", SubcatMeleeCurvedSwords},
	{0x001FE410, "Inseparable Sword", SubcatMeleeGreatswords},
	{0x03AADF90, "Lightning Perfume Bottle", SubcatMeleePerfumeBottles},
	{0x02719C40, "Longbow", SubcatRangedBows},
	{0x01488590, "Madding Hand", SubcatMeleeFists},
	{0x006B9310, "Magma Blade", SubcatMeleeCurvedSwords},
	{0x007AAE40, "Magma Wyrm's Scalesword", SubcatMeleeCurvedGreatswords},
	{0x006CA480, "Mantis Blade", SubcatMeleeCurvedSwords},
	{0x010B2E70, "Messmer Soldier's Spear", SubcatMeleeGreatSpears},
	{0x007B2370, "Monk's Flameblade", SubcatMeleeCurvedGreatswords},
	{0x00A93850, "Monk's Flamemace", SubcatMeleeHammers},
	{0x00454660, "Moonrithyll's Knight Sword", SubcatMeleeColossalSwords},
	{0x007B98A0, "Morgott's Cursed Sword", SubcatMeleeCurvedGreatswords},
	{0x001FBD00, "Nox Flowing Sword", SubcatMeleeCurvedSwords},
	{0x007AFC60, "Omen Cleaver", SubcatMeleeCurvedGreatswords},
	{0x007A3910, "Onyx Lord's Greatsword", SubcatMeleeCurvedGreatswords},
	{0x0112CF90, "Pest's Glaive", SubcatMeleeHalberds},
	{0x01485E80, "Poisoned Hand", SubcatMeleeFists},
	{0x011A70B0, "Poleblade of the Bud", SubcatMeleeHalberds},
	{0x0081B320, "Putrescence Cleaver", SubcatMeleeGreataxes},
	{0x00632EA0, "Queelign's Greatsword", SubcatMeleeHeavyThrustingSwords},
	{0x00A9D490, "Ringed Finger", SubcatMeleeHammers},
	{0x00D662D0, "Ripple Blade", SubcatMeleeAxes},
	{0x011392E0, "Ripple Crescent Halberd", SubcatMeleeHalberds},
	{0x00BA4F50, "Rotten Battle Hammer", SubcatMeleeGreatHammers},
	{0x006CF2A0, "Scimitar", SubcatMeleeCurvedSwords},
	{0x008A8CC0, "Serpentbone Blade", SubcatMeleeKatanas},
	{0x0166BBF0, "Shadow Sunflower Blossom", SubcatMeleeColossalWeapons},
	{0x006B44F0, "Shamshir", SubcatMeleeCurvedSwords},
	{0x01DB9DD0, "Shield of the Guilty", SubcatShieldsSmall},
	{0x006B1DE0, "Shotel", SubcatMeleeCurvedSwords},
	{0x03D85830, "Smithscript Cirque", SubcatMeleeBackhandBlades},
	{0x010B0760, "Spear of the Impaler", SubcatMeleeGreatSpears},
	{0x007270E0, "Spirit Sword", SubcatMeleeCurvedSwords},
	{0x002673C0, "Star-Lined Sword", SubcatMeleeKatanas},
	{0x00271000, "Sword of Darkness", SubcatMeleeStraightSwords},
	{0x0026E8F0, "Sword of Light", SubcatMeleeStraightSwords},
	{0x0090F560, "Sword of Night", SubcatMeleeKatanas},
	{0x01481060, "Thiollier's Hidden Needle", SubcatMeleeFists},
	{0x010477B0, "Treespear", SubcatMeleeGreatSpears},
	{0x0160EF90, "Troll's Hammer", SubcatMeleeColossalWeapons},
	{0x00990BB0, "Twinned Knight Swords", SubcatMeleeTwinblades},
	{0x00A8C320, "Varr's Bouquet", SubcatMeleeHammers},
	{0x00E508D0, "Warped Axe", SubcatMeleeAxes},
	{0x006BE130, "Wing of Astel", SubcatMeleeCurvedSwords},
	{0x007AD550, "Zamor Curved Sword", SubcatMeleeCurvedGreatswords},
}

//  1. Every reported weapon now carries the canonical sub-category. This is the
//     regression guard: a wrong wepType→sub-category mapping (type confusion)
//     fails here with the offending name.
func TestWeaponSubcat_ReportedItemsFixed(t *testing.T) {
	for _, w := range reportedWeapons {
		item, ok := lookupArmament(w.id)
		if !ok {
			t.Errorf("%q (0x%08X) missing from item DB", w.name, w.id)
			continue
		}
		if item.Name != w.name {
			t.Errorf("0x%08X: name = %q, want %q", w.id, item.Name, w.name)
		}
		if item.SubCategory != w.want {
			t.Errorf("%q (0x%08X): SubCategory = %q, want %q",
				item.Name, w.id, item.SubCategory, w.want)
		}
	}
}

//  2. wepType is authoritative: for every armament whose canonical wepType has a
//     sub-group in its tab's map, the assigned SubCategory must equal it. This
//     protects unlisted weapons from name-heuristic drift and catches any future
//     foreign assignment (a wepType leaking into the wrong sub-group).
func TestWeaponSubcat_WepTypeAuthoritative(t *testing.T) {
	check := func(name string, m map[uint32]ItemData, table map[uint16]string) {
		for id, item := range m {
			want, ok := wepTypeSubcat(id, table)
			if !ok {
				continue // no canonical sub-group for this wepType (e.g. Unarmed)
			}
			if item.SubCategory != want {
				t.Errorf("%s: %q (0x%08X) SubCategory = %q, want %q (wepType %d)",
					name, item.Name, id, item.SubCategory, want, weaponWepType[id])
			}
		}
	}
	check("Weapons", Weapons, meleeWepTypeSubcat)
	check("Shields", Shields, shieldWepTypeSubcat)
	check("RangedAndCatalysts", RangedAndCatalysts, rangedWepTypeSubcat)
}

// 2b. weaponWepType must cover every armament. TestWeaponSubcat_WepTypeAuthoritative
//
//	skips items with no ID→wepType entry, so a missing entry (which silently
//	drops the item to the name heuristic) would pass unnoticed there. Here a
//	missing ID→wepType is a hard failure — only a missing wepType→sub-category
//	mapping is allowed. The size check also flags stale/extra table entries.
func TestWeaponSubcat_WepTypeTableComplete(t *testing.T) {
	total := 0
	for _, m := range []map[uint32]ItemData{Weapons, Shields, RangedAndCatalysts} {
		total += len(m)
		for id, item := range m {
			if _, ok := weaponWepType[id]; !ok {
				t.Errorf("%q (0x%08X) missing from weaponWepType — falls back to name heuristic", item.Name, id)
			}
		}
	}
	if len(weaponWepType) != total {
		t.Errorf("weaponWepType has %d entries, want %d (len(Weapons)+len(Shields)+len(RangedAndCatalysts)); size mismatch means missing or extra entries",
			len(weaponWepType), total)
	}
}

//  3. Documented discrepancy: "Rotten Staff" lives in the Ranged/Catalysts tab
//     but its canonical wepType is 41 (Colossal Weapon — a melee type). Since 41
//     is not a ranged sub-group it is intentionally NOT remapped; fixing it needs
//     a Category move, out of scope for sub-category labeling. This test pins the
//     current behavior so the discrepancy is not silently "fixed" the wrong way.
func TestWeaponSubcat_RottenStaffDiscrepancy(t *testing.T) {
	const rottenStaff = 0x016116A0 // wepType 41, Category ranged_and_catalysts
	item, ok := RangedAndCatalysts[rottenStaff]
	if !ok {
		t.Skip("Rotten Staff not in RangedAndCatalysts under expected ID")
	}
	if wt := weaponWepType[rottenStaff]; wt != 41 {
		t.Fatalf("Rotten Staff wepType = %d, expected canonical 41 (Colossal Weapon)", wt)
	}
	if _, mapped := rangedWepTypeSubcat[41]; mapped {
		t.Fatalf("wepType 41 must not be a ranged sub-group")
	}
	// It keeps the name-fallback classification, NOT a melee sub-category.
	if item.SubCategory == SubcatMeleeColossalWeapons {
		t.Errorf("Rotten Staff must not receive melee sub-category %q while in the ranged tab", SubcatMeleeColossalWeapons)
	}
}
