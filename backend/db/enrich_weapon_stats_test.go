package db

import (
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/db/data"
)

// TestEnrichItemEntryUsesWeaponStatsV1 anchors on Lance (0x010450A0) and
// asserts that after Phase 3C.2 the legacy ItemEntry.Weapon pointer is
// non-nil and carries the V1-mapped values for every field present in
// data.WeaponStats.
//
// Lance happens to have matching values in descriptions.go and V1, so
// the test would also pass on the legacy fallback path. To prove the V1
// override is the one running, we additionally assert e.Weight matches
// V1.Weight (descriptions.go does carry a Weight on Lance too, but the
// equality is still meaningful because the V1 hookup must not zero it).
func TestEnrichItemEntryUsesWeaponStatsV1(t *testing.T) {
	const id uint32 = 0x010450A0 // Lance
	v, ok := data.WeaponStatsV1ByID[id]
	if !ok {
		t.Fatalf("WeaponStatsV1ByID[0x%08X] missing; cannot anchor test on Lance", id)
	}

	e := &ItemEntry{ID: id, Category: "melee_armaments"}
	enrichItemEntry(e)
	if e.Weapon == nil {
		t.Fatalf("e.Weapon is nil after enrichment; expected V1-mapped pointer")
	}
	checks := []struct {
		name      string
		got, want uint32
	}{
		{"PhysDamage", e.Weapon.PhysDamage, nonNegU32(v.AttackPhysical)},
		{"MagDamage", e.Weapon.MagDamage, nonNegU32(v.AttackMagic)},
		{"FireDamage", e.Weapon.FireDamage, nonNegU32(v.AttackFire)},
		{"LitDamage", e.Weapon.LitDamage, nonNegU32(v.AttackLightning)},
		{"HolyDamage", e.Weapon.HolyDamage, nonNegU32(v.AttackHoly)},
		{"ScaleStr", e.Weapon.ScaleStr, nonNegU32(v.ScalingStrRaw)},
		{"ScaleDex", e.Weapon.ScaleDex, nonNegU32(v.ScalingDexRaw)},
		{"ScaleInt", e.Weapon.ScaleInt, nonNegU32(v.ScalingIntRaw)},
		{"ScaleFai", e.Weapon.ScaleFai, nonNegU32(v.ScalingFaiRaw)},
		{"ReqStr", e.Weapon.ReqStr, nonNegU32(v.StatReqStr)},
		{"ReqDex", e.Weapon.ReqDex, nonNegU32(v.StatReqDex)},
		{"ReqInt", e.Weapon.ReqInt, nonNegU32(v.StatReqInt)},
		{"ReqFai", e.Weapon.ReqFai, nonNegU32(v.StatReqFai)},
		{"ReqArc", e.Weapon.ReqArc, nonNegU32(v.StatReqArc)},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("Lance Weapon.%s = %d, want V1 mapped %d", c.name, c.got, c.want)
		}
	}
	if e.Weapon.Weight != v.Weight {
		t.Errorf("Lance Weapon.Weight = %v, want V1 mapped %v", e.Weapon.Weight, v.Weight)
	}
	if v.Weight > 0 && e.Weight != v.Weight {
		t.Errorf("Lance e.Weight = %v, want V1 mapped %v", e.Weight, v.Weight)
	}
}

// TestEnrichItemEntryWeaponStatsV1HolyMapping is the R-STA-01 guard at
// the runtime layer. It asserts that V1.AttackHoly (sourced from
// EquipParamWeapon.attackBaseDark) lands in legacy WeaponStats.HolyDamage
// after enrichment — there is no Dark-named field in legacy WeaponStats,
// and HolyDamage must NOT be zero for Sacred Relic Sword.
func TestEnrichItemEntryWeaponStatsV1HolyMapping(t *testing.T) {
	const id uint32 = 0x002F4D60 // Sacred Relic Sword
	v, ok := data.WeaponStatsV1ByID[id]
	if !ok {
		t.Fatalf("WeaponStatsV1ByID[0x%08X] missing; cannot anchor holy test", id)
	}
	if v.AttackHoly == 0 {
		t.Fatalf("V1 AttackHoly is zero for Sacred Relic Sword; Phase 3C.1 regression detected")
	}

	e := &ItemEntry{ID: id, Category: "melee_armaments"}
	enrichItemEntry(e)
	if e.Weapon == nil {
		t.Fatalf("e.Weapon is nil after enrichment; expected V1-mapped pointer")
	}
	if e.Weapon.HolyDamage != uint32(v.AttackHoly) {
		t.Errorf("Sacred Relic Sword HolyDamage = %d, want %d (V1 AttackHoly mapped through weaponStatsV1ToLegacy)",
			e.Weapon.HolyDamage, v.AttackHoly)
	}
	if e.Weapon.HolyDamage == 0 {
		t.Errorf("HolyDamage is zero — R-STA-01 regression: V1 AttackHoly did not flow into legacy HolyDamage")
	}
}

// TestEnrichItemEntryWeaponStatsV1SomberAndStandardAnchors checks that
// the V1 entry — and therefore its IsSomber / MaxUpgrade flags — is the
// one driving enrichment for known anchors. Legacy WeaponStats has no
// somber/upgrade fields, so the surrogate signal is e.Weapon matching
// the V1-mapped PhysDamage / HolyDamage (anything pulled from
// descriptions.go would in principle drift over time).
func TestEnrichItemEntryWeaponStatsV1SomberAndStandardAnchors(t *testing.T) {
	cases := []struct {
		id         uint32
		name       string
		wantSomber bool
		wantMax    int32
	}{
		{0x001E8480, "Longsword", false, 25},
		{0x005BDBA0, "Great Épée", false, 25},
		{0x0044F840, "Fire Knight's Greatsword", false, 25},
		{0x002F4D60, "Sacred Relic Sword", true, 10},
		{0x01EA6AE0, "Icon Shield", true, 10},
	}
	for _, c := range cases {
		v, ok := data.WeaponStatsV1ByID[c.id]
		if !ok {
			t.Errorf("%s V1 missing", c.name)
			continue
		}
		if v.IsSomber != c.wantSomber {
			t.Errorf("%s V1 IsSomber = %v, want %v (V1 regression, not enrichment)",
				c.name, v.IsSomber, c.wantSomber)
		}
		if v.MaxUpgrade != c.wantMax {
			t.Errorf("%s V1 MaxUpgrade = %d, want %d (V1 regression, not enrichment)",
				c.name, v.MaxUpgrade, c.wantMax)
		}

		e := &ItemEntry{ID: c.id, Category: "melee_armaments"}
		enrichItemEntry(e)
		if e.Weapon == nil {
			t.Errorf("%s enriched Weapon is nil", c.name)
			continue
		}
		// Surrogate: confirm enrichment used the V1 entry by matching
		// PhysDamage and HolyDamage (HolyDamage is the discriminator for
		// somber holy weapons like Sacred Relic Sword).
		if e.Weapon.PhysDamage != nonNegU32(v.AttackPhysical) {
			t.Errorf("%s PhysDamage = %d, want V1 mapped %d — enrichment did not source V1",
				c.name, e.Weapon.PhysDamage, v.AttackPhysical)
		}
		if e.Weapon.HolyDamage != nonNegU32(v.AttackHoly) {
			t.Errorf("%s HolyDamage = %d, want V1 mapped %d — enrichment did not source V1",
				c.name, e.Weapon.HolyDamage, v.AttackHoly)
		}
	}
}

// TestEnrichItemEntryWeaponStatsV1Ammo asserts ammo enrichment populates
// e.Weapon for arrows / bolts. Fire Arrow has a non-zero FireDamage;
// Lightning Bolt has a non-zero LitDamage. Both must come from V1 since
// descriptions.go historically does not carry ammo stats.
func TestEnrichItemEntryWeaponStatsV1Ammo(t *testing.T) {
	cases := []struct {
		id        uint32
		name      string
		category  string
		wantField string
	}{
		{0x02FB1790, "Fire Arrow", "arrows_and_bolts", "FireDamage"},
		{0x03199C10, "Lightning Bolt", "arrows_and_bolts", "LitDamage"},
	}
	for _, c := range cases {
		v, ok := data.WeaponStatsV1ByID[c.id]
		if !ok {
			t.Errorf("%s V1 missing", c.name)
			continue
		}
		e := &ItemEntry{ID: c.id, Category: c.category}
		enrichItemEntry(e)
		if e.Weapon == nil {
			t.Errorf("%s e.Weapon is nil; ammo should get V1-mapped Weapon stats", c.name)
			continue
		}
		var got uint32
		switch c.wantField {
		case "FireDamage":
			got = e.Weapon.FireDamage
		case "LitDamage":
			got = e.Weapon.LitDamage
		}
		if got == 0 {
			t.Errorf("%s %s is zero after enrichment; V1 hookup did not run for ammo", c.name, c.wantField)
		}
		switch c.wantField {
		case "FireDamage":
			if e.Weapon.FireDamage != nonNegU32(v.AttackFire) {
				t.Errorf("%s FireDamage = %d, want V1 mapped %d",
					c.name, e.Weapon.FireDamage, v.AttackFire)
			}
		case "LitDamage":
			if e.Weapon.LitDamage != nonNegU32(v.AttackLightning) {
				t.Errorf("%s LitDamage = %d, want V1 mapped %d",
					c.name, e.Weapon.LitDamage, v.AttackLightning)
			}
		}
	}
}

// TestEnrichItemEntryWeaponStatsFallbackToDescriptions asserts the
// legacy fallback still runs for IDs that exist in data.Descriptions
// (with a non-nil Weapon pointer) but are NOT in WeaponStatsV1ByID. The
// test discovers such an orphan dynamically so it tolerates churn.
//
// In the current data the Phase 3C.1 V1 table only covers the four
// weapon-like categories; any descriptions.go entry whose ID is outside
// that category set will be a valid orphan candidate.
func TestEnrichItemEntryWeaponStatsFallbackToDescriptions(t *testing.T) {
	var (
		chosenID    uint32
		chosenWep   *data.WeaponStats
		foundOrphan bool
	)
	for id, desc := range data.Descriptions {
		if desc.Weapon == nil {
			continue
		}
		if _, inV1 := data.WeaponStatsV1ByID[id]; inV1 {
			continue
		}
		chosenID = id
		chosenWep = desc.Weapon
		foundOrphan = true
		break
	}
	if !foundOrphan {
		t.Skip("no descriptions.go orphan with WeaponStats outside V1 coverage — skipping fallback test")
	}

	e := &ItemEntry{ID: chosenID}
	enrichItemEntry(e)
	if e.Weapon == nil {
		t.Fatalf("orphan 0x%08X: e.Weapon nil; expected descriptions.go fallback", chosenID)
	}
	if *e.Weapon != *chosenWep {
		t.Errorf("orphan 0x%08X Weapon = %+v, want descriptions.go fallback %+v",
			chosenID, *e.Weapon, *chosenWep)
	}
}

// TestEnrichItemEntryPreservesArmorSpellStats asserts that the Phase
// 3C.2 V1 hookup is scoped to the Weapon pointer and does not touch
// Armor or Spell. Anchors come dynamically from data.Descriptions so
// the test does not hard-code IDs whose category data may shift.
func TestEnrichItemEntryPreservesArmorSpellStats(t *testing.T) {
	var armorID, spellID uint32
	var armorWant *data.ArmorStats
	var spellWant *data.SpellStats
	for id, desc := range data.Descriptions {
		if armorID == 0 && desc.Armor != nil {
			armorID = id
			armorWant = desc.Armor
		}
		if spellID == 0 && desc.Spell != nil {
			spellID = id
			spellWant = desc.Spell
		}
		if armorID != 0 && spellID != 0 {
			break
		}
	}
	if armorID != 0 {
		e := &ItemEntry{ID: armorID}
		enrichItemEntry(e)
		if e.Armor == nil || *e.Armor != *armorWant {
			t.Errorf("armor anchor 0x%08X: Armor not preserved (got %+v, want %+v)",
				armorID, e.Armor, armorWant)
		}
	} else {
		t.Logf("no Armor anchor available in descriptions.go — armor preservation untested this run")
	}
	if spellID != 0 {
		e := &ItemEntry{ID: spellID}
		enrichItemEntry(e)
		if e.Spell == nil || *e.Spell != *spellWant {
			t.Errorf("spell anchor 0x%08X: Spell not preserved (got %+v, want %+v)",
				spellID, e.Spell, spellWant)
		}
	} else {
		t.Logf("no Spell anchor available in descriptions.go — spell preservation untested this run")
	}
}

// TestEnrichItemEntryWeaponStatsDoesNotAffectText asserts that the V1
// hookup leaves Description, Location, and the e.Text payload exactly as
// Phase 3B.3 set them. Sacred Relic Sword is the anchor — it carries
// FMG description and curated location, and now also a V1 stats entry.
func TestEnrichItemEntryWeaponStatsDoesNotAffectText(t *testing.T) {
	const id uint32 = 0x002F4D60 // Sacred Relic Sword
	text, ok := data.ItemTexts[id]
	if !ok {
		t.Fatalf("ItemTexts[0x%08X] missing; cannot anchor regression test", id)
	}

	e := &ItemEntry{ID: id, Category: "melee_armaments"}
	enrichItemEntry(e)
	if e.Text == nil {
		t.Fatalf("e.Text nil; Phase 3B.3 text payload regression introduced by 3C.2 hookup")
	}
	if e.Text.DisplayName != text.DisplayName {
		t.Errorf("e.Text.DisplayName = %q, want %q", e.Text.DisplayName, text.DisplayName)
	}
	if text.Description != "" && e.Description != text.Description {
		t.Errorf("Description = %q, want %q", e.Description, text.Description)
	}
	if text.Location != "" && e.Location != text.Location {
		t.Errorf("Location = %q, want %q", e.Location, text.Location)
	}
}

// TestNonNegU32 exercises the int32→uint32 clamp helper to lock its
// contract (negatives → 0; zero stays zero; positives pass through).
func TestNonNegU32(t *testing.T) {
	cases := []struct {
		in   int32
		want uint32
	}{
		{0, 0},
		{1, 1},
		{123, 123},
		{-1, 0},
		{-99, 0},
	}
	for _, c := range cases {
		if got := nonNegU32(c.in); got != c.want {
			t.Errorf("nonNegU32(%d) = %d, want %d", c.in, got, c.want)
		}
	}
}
