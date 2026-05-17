package data

import "testing"

// TestWeaponStatsV1PassiveOnHitBloodLoss verifies that katanas with a
// blood-loss on-hit SpEffect (Uchigatana #6400 → 45, Moonveil and
// Rivers of Blood #6401 → 50) are correctly resolved into a single
// Known on_hit entry. SpEffectIDs are not asserted directly because they
// are an implementation detail of the regulation data; the test pins the
// user-visible Label + Value pair.
func TestWeaponStatsV1PassiveOnHitBloodLoss(t *testing.T) {
	cases := []struct {
		name      string
		id        uint32
		wantValue int32
	}{
		{"Uchigatana", 0x00895440, 45},
		{"Moonveil", 0x008A3EA0, 50},
		{"Rivers of Blood", 0x0089F080, 50},
	}
	for _, c := range cases {
		v, ok := WeaponStatsV1ByID[c.id]
		if !ok {
			t.Errorf("%s (0x%08X) missing from WeaponStatsV1ByID", c.name, c.id)
			continue
		}
		found := false
		for _, pe := range v.PassiveEffects {
			if pe.Kind == "on_hit" && pe.Label == "Blood Loss" {
				if !pe.Known {
					t.Errorf("%s on-hit Blood Loss Known=false, want true", c.name)
				}
				if pe.Value != c.wantValue {
					t.Errorf("%s on-hit Blood Loss Value=%d, want %d", c.name, pe.Value, c.wantValue)
				}
				found = true
				break
			}
		}
		if !found {
			t.Errorf("%s: expected on-hit Blood Loss PassiveEffect, got %+v", c.name, v.PassiveEffects)
		}
	}
}

// TestWeaponStatsV1PassiveResidentLabels anchors the curated resident
// SpEffect → label map on four weapons that are well-documented and
// whose resident effect ID is stable in the regulation data.
func TestWeaponStatsV1PassiveResidentLabels(t *testing.T) {
	cases := []struct {
		name      string
		id        uint32
		wantLabel string
	}{
		{"Sacrificial Axe", 0x00D74D30, "Restores FP upon defeating enemies"},
		{"Serpent-God's Curved Sword", 0x006C7D70, "Restores HP upon defeating enemies"},
		{"Dragon Communion Seal", 0x02080500, "Boosts Dragon Communion incantations"},
		{"Prince of Death's Staff", 0x01FA4960, "Boosts death sorceries"},
	}
	for _, c := range cases {
		v, ok := WeaponStatsV1ByID[c.id]
		if !ok {
			t.Errorf("%s (0x%08X) missing from WeaponStatsV1ByID", c.name, c.id)
			continue
		}
		found := false
		for _, pe := range v.PassiveEffects {
			if pe.Kind == "resident" && pe.Label == c.wantLabel {
				if !pe.Known {
					t.Errorf("%s resident %q Known=false, want true", c.name, c.wantLabel)
				}
				found = true
				break
			}
		}
		if !found {
			t.Errorf("%s: expected resident %q PassiveEffect, got %+v", c.name, c.wantLabel, v.PassiveEffects)
		}
	}
}

// TestWeaponStatsV1PassiveNoneForPlainWeapon anchors a weapon that has
// neither on-hit nor resident SpEffects in regulation data. Lordsworn's
// Straight Sword is the canonical plain melee weapon — if this test
// starts failing, either the regulation data changed or the resolver
// began emitting spurious effects.
func TestWeaponStatsV1PassiveNoneForPlainWeapon(t *testing.T) {
	const id uint32 = 0x001F20C0 // Lordsworn's Straight Sword
	v, ok := WeaponStatsV1ByID[id]
	if !ok {
		t.Fatalf("Lordsworn's Straight Sword (0x%08X) missing", id)
	}
	if len(v.PassiveEffects) != 0 {
		t.Errorf("Lordsworn's Straight Sword PassiveEffects len=%d, want 0; got %+v",
			len(v.PassiveEffects), v.PassiveEffects)
	}
}
