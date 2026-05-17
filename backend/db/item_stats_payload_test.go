package db

import (
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/db/data"
)

// TestEnrichItemEntrySetsWeaponStatsPayload asserts the Phase 3C.3
// payload appears on a known weapon-like item with a V1 entry.
// Lance (0x010450A0) anchors the standard infusable case.
func TestEnrichItemEntrySetsWeaponStatsPayload(t *testing.T) {
	const id uint32 = 0x010450A0 // Lance
	v, ok := data.WeaponStatsV1ByID[id]
	if !ok {
		t.Fatalf("WeaponStatsV1ByID[0x%08X] missing; cannot anchor test", id)
	}
	e := &ItemEntry{ID: id, Category: "melee_armaments"}
	enrichItemEntry(e)
	if e.Stats == nil {
		t.Fatalf("e.Stats is nil; expected V1 payload for Lance")
	}
	if e.Stats.Kind != data.ItemStatsKindWeapon {
		t.Errorf("e.Stats.Kind = %q, want %q", e.Stats.Kind, data.ItemStatsKindWeapon)
	}
	if e.Stats.Weapon == nil {
		t.Fatal("e.Stats.Weapon nil; payload should carry the V1 record")
	}
	if e.Stats.Weapon.ItemID != id {
		t.Errorf("e.Stats.Weapon.ItemID = 0x%08X, want 0x%08X", e.Stats.Weapon.ItemID, id)
	}
	if e.Stats.SourceParam != "EquipParamWeapon" {
		t.Errorf("e.Stats.SourceParam = %q, want %q", e.Stats.SourceParam, "EquipParamWeapon")
	}
	if e.Stats.SourceRowID != v.SourceRowID {
		t.Errorf("e.Stats.SourceRowID = %d, want %d", e.Stats.SourceRowID, v.SourceRowID)
	}
}

// TestEnrichItemEntryStatsPayloadHolyMapping is the R-STA-01 guard at
// the payload layer. The V1 record exposed via e.Stats.Weapon must carry
// the same AttackHoly as the source map (sourced from
// EquipParamWeapon.attackBaseDark), AND the legacy projection at
// e.Weapon.HolyDamage must match.
func TestEnrichItemEntryStatsPayloadHolyMapping(t *testing.T) {
	const id uint32 = 0x002F4D60 // Sacred Relic Sword
	v, ok := data.WeaponStatsV1ByID[id]
	if !ok {
		t.Fatalf("WeaponStatsV1ByID[0x%08X] missing", id)
	}
	if v.AttackHoly == 0 {
		t.Fatalf("V1 AttackHoly is zero for Sacred Relic Sword; Phase 3C.1 regression")
	}
	e := &ItemEntry{ID: id, Category: "melee_armaments"}
	enrichItemEntry(e)
	if e.Stats == nil || e.Stats.Weapon == nil {
		t.Fatalf("e.Stats / e.Stats.Weapon nil; expected Sacred Relic Sword payload")
	}
	if e.Stats.Weapon.AttackHoly != v.AttackHoly {
		t.Errorf("e.Stats.Weapon.AttackHoly = %d, want %d (R-STA-01 payload regression)",
			e.Stats.Weapon.AttackHoly, v.AttackHoly)
	}
	if e.Weapon == nil {
		t.Fatal("e.Weapon nil; legacy projection must still run alongside Stats payload")
	}
	if e.Weapon.HolyDamage != uint32(v.AttackHoly) {
		t.Errorf("legacy e.Weapon.HolyDamage = %d, want %d (R-STA-01 legacy regression)",
			e.Weapon.HolyDamage, v.AttackHoly)
	}
}

// TestEnrichItemEntryStatsPayloadV1OnlyFields locks down the V1-only
// surface that is the whole point of Phase 3C.3 — fields legacy
// WeaponStats does not carry (GuardHoly, IsSomber, IsInfusable,
// MaxUpgrade, GemMountType).
func TestEnrichItemEntryStatsPayloadV1OnlyFields(t *testing.T) {
	cases := []struct {
		id           uint32
		name         string
		category     string
		wantSomber   bool
		wantInfuse   bool
		wantMaxUp    int32
		wantGemMount uint8
	}{
		{0x001E8480, "Longsword", "melee_armaments", false, true, 25, 2},
		{0x002F4D60, "Sacred Relic Sword", "melee_armaments", true, false, 10, 0},
		{0x01EA6AE0, "Icon Shield", "shields", true, false, 10, 0},
	}
	for _, c := range cases {
		v, ok := data.WeaponStatsV1ByID[c.id]
		if !ok {
			t.Errorf("%s V1 missing", c.name)
			continue
		}
		e := &ItemEntry{ID: c.id, Category: c.category}
		enrichItemEntry(e)
		if e.Stats == nil || e.Stats.Weapon == nil {
			t.Errorf("%s e.Stats / e.Stats.Weapon nil", c.name)
			continue
		}
		w := e.Stats.Weapon
		if w.IsSomber != c.wantSomber {
			t.Errorf("%s payload IsSomber = %v, want %v", c.name, w.IsSomber, c.wantSomber)
		}
		if w.IsInfusable != c.wantInfuse {
			t.Errorf("%s payload IsInfusable = %v, want %v", c.name, w.IsInfusable, c.wantInfuse)
		}
		if w.MaxUpgrade != c.wantMaxUp {
			t.Errorf("%s payload MaxUpgrade = %d, want %d", c.name, w.MaxUpgrade, c.wantMaxUp)
		}
		if w.GemMountType != c.wantGemMount {
			t.Errorf("%s payload GemMountType = %d, want %d", c.name, w.GemMountType, c.wantGemMount)
		}
		// Cross-check: payload value equals the source map value.
		if w.IsSomber != v.IsSomber || w.IsInfusable != v.IsInfusable ||
			w.MaxUpgrade != v.MaxUpgrade || w.GemMountType != v.GemMountType {
			t.Errorf("%s payload drifted from source map", c.name)
		}
	}

	// Shield-specific: GuardHoly is V1-only (no legacy field), and
	// must be non-zero for items with holy guard data.
	if v, ok := data.WeaponStatsV1ByID[0x002F4D60]; ok && v.GuardHoly != 0 {
		e := &ItemEntry{ID: 0x002F4D60, Category: "melee_armaments"}
		enrichItemEntry(e)
		if e.Stats == nil || e.Stats.Weapon == nil {
			t.Fatal("Sacred Relic Sword Stats nil")
		}
		if e.Stats.Weapon.GuardHoly != v.GuardHoly {
			t.Errorf("Sacred Relic Sword payload GuardHoly = %d, want %d",
				e.Stats.Weapon.GuardHoly, v.GuardHoly)
		}
	}
}

// TestEnrichItemEntryStatsPayloadNilForNonWeapon asserts non-weapon
// items get a nil Stats payload while legacy enrichment (Armor / Spell /
// Text) still runs.
func TestEnrichItemEntryStatsPayloadNilForNonWeapon(t *testing.T) {
	// Pick an armor anchor dynamically — descriptions.go has the
	// authoritative list of legacy Armor entries.
	var armorID uint32
	var armorWant *data.ArmorStats
	for id, desc := range data.Descriptions {
		if desc.Armor != nil {
			if _, inV1 := data.WeaponStatsV1ByID[id]; inV1 {
				continue // skip rare ID overlaps
			}
			armorID = id
			armorWant = desc.Armor
			break
		}
	}
	if armorID == 0 {
		t.Skip("no armor anchor available in descriptions.go")
	}
	e := &ItemEntry{ID: armorID}
	enrichItemEntry(e)
	if e.Stats != nil {
		t.Errorf("armor 0x%08X: e.Stats = %+v, want nil (no V1 payload for armor in 3C.3)",
			armorID, e.Stats)
	}
	if e.Armor == nil || *e.Armor != *armorWant {
		t.Errorf("armor 0x%08X: legacy Armor pointer not preserved (got %+v, want %+v)",
			armorID, e.Armor, armorWant)
	}

	// Pick a spell anchor too.
	var spellID uint32
	var spellWant *data.SpellStats
	for id, desc := range data.Descriptions {
		if desc.Spell != nil {
			if _, inV1 := data.WeaponStatsV1ByID[id]; inV1 {
				continue
			}
			spellID = id
			spellWant = desc.Spell
			break
		}
	}
	if spellID != 0 {
		e := &ItemEntry{ID: spellID}
		enrichItemEntry(e)
		if e.Stats != nil {
			t.Errorf("spell 0x%08X: e.Stats = %+v, want nil", spellID, e.Stats)
		}
		if e.Spell == nil || *e.Spell != *spellWant {
			t.Errorf("spell 0x%08X: legacy Spell pointer not preserved", spellID)
		}
	}
}

// TestEnrichItemEntryStatsPayloadDoesNotBreakLegacyWeapon asserts the
// Phase 3C.3 addition is purely additive — for a weapon-like item with
// a V1 entry the legacy `e.Weapon` projection still appears and its
// key fields match the V1-mapped values that the mapper produces.
func TestEnrichItemEntryStatsPayloadDoesNotBreakLegacyWeapon(t *testing.T) {
	const id uint32 = 0x001E8480 // Longsword
	v, ok := data.WeaponStatsV1ByID[id]
	if !ok {
		t.Fatalf("WeaponStatsV1ByID[0x%08X] missing", id)
	}
	e := &ItemEntry{ID: id, Category: "melee_armaments"}
	enrichItemEntry(e)
	if e.Weapon == nil {
		t.Fatal("legacy e.Weapon nil; 3C.3 must not remove the legacy projection")
	}
	if e.Stats == nil || e.Stats.Weapon == nil {
		t.Fatal("e.Stats payload missing; 3C.3 must attach payload for weapons")
	}
	// Legacy projection key fields equal the V1 mapper output.
	if e.Weapon.PhysDamage != nonNegU32(v.AttackPhysical) {
		t.Errorf("PhysDamage = %d, want V1 mapped %d", e.Weapon.PhysDamage, v.AttackPhysical)
	}
	if e.Weapon.HolyDamage != nonNegU32(v.AttackHoly) {
		t.Errorf("HolyDamage = %d, want V1 mapped %d", e.Weapon.HolyDamage, v.AttackHoly)
	}
	if e.Weapon.ReqStr != nonNegU32(v.StatReqStr) {
		t.Errorf("ReqStr = %d, want V1 mapped %d", e.Weapon.ReqStr, v.StatReqStr)
	}
	// Payload mirrors the source map.
	if e.Stats.Weapon.AttackPhysical != v.AttackPhysical {
		t.Errorf("payload AttackPhysical = %d, want %d",
			e.Stats.Weapon.AttackPhysical, v.AttackPhysical)
	}
}

// TestEnrichItemEntryStatsPayloadNilForMissing asserts unknown IDs
// produce no panic and no Stats payload.
func TestEnrichItemEntryStatsPayloadNilForMissing(t *testing.T) {
	const unknown uint32 = 0xDEADBEEF
	if _, ok := data.WeaponStatsV1ByID[unknown]; ok {
		t.Fatalf("0x%08X collides with a known item; pick a different sentinel", unknown)
	}
	e := &ItemEntry{ID: unknown}
	enrichItemEntry(e) // must not panic
	if e.Stats != nil {
		t.Errorf("unknown ID 0x%08X: e.Stats = %+v, want nil", unknown, e.Stats)
	}
}
