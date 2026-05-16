package data

import "testing"

// phase2b1Additions enumerates the 6 weapons added in Phase 2B.1 along with
// the Beast Claw ID-collision repair (weapon ID 0x04153A20 moved out of
// Incantations into Weapons).
//
// Data validated against:
//   - tmp/regulation-bin-dump/csv/EquipParamWeapon.csv  (wepType, reinforce)
//   - tmp/regulation-bin-dump/msg/name_mapping.csv      (FMG names)
//   - tmp/item-audit/phase2_classification_missing.csv  (audit classification)
var phase2b1Additions = []struct {
	ID          uint32
	Name        string
	SubCategory string
	MaxUpgrade  uint32
	IsDLC       bool
}{
	{0x002F4D60, "Sacred Relic Sword", SubcatMeleeGreatswords, 10, false},
	{0x005BDBA0, "Great Épée", SubcatMeleeHeavyThrustingSwords, 25, false},
	{0x01038D50, "Mohgwyn's Sacred Spear", SubcatMeleeGreatSpears, 10, false},
	{0x00170A70, "Fire Knight's Shortsword", SubcatMeleeDaggers, 25, true},
	{0x0044F840, "Fire Knight's Greatsword", SubcatMeleeColossalSwords, 25, true},
	{0x04153A20, "Beast Claw", SubcatMeleeBeastClaws, 25, true},
}

// TestPhase2B1WeaponsPresent verifies every Phase 2B.1 weapon exists in the
// Weapons map with the expected Name, Category, SubCategory, MaxUpgrade and
// DLC flag.
func TestPhase2B1WeaponsPresent(t *testing.T) {
	for _, w := range phase2b1Additions {
		item, ok := Weapons[w.ID]
		if !ok {
			t.Errorf("Weapons[0x%08X] (%s): missing entry", w.ID, w.Name)
			continue
		}
		if item.Name != w.Name {
			t.Errorf("Weapons[0x%08X] name = %q, want %q", w.ID, item.Name, w.Name)
		}
		if item.Category != "melee_armaments" {
			t.Errorf("Weapons[0x%08X] (%s) category = %q, want %q",
				w.ID, w.Name, item.Category, "melee_armaments")
		}
		if item.SubCategory != w.SubCategory {
			t.Errorf("Weapons[0x%08X] (%s) sub-category = %q, want %q",
				w.ID, w.Name, item.SubCategory, w.SubCategory)
		}
		if item.MaxUpgrade != w.MaxUpgrade {
			t.Errorf("Weapons[0x%08X] (%s) MaxUpgrade = %d, want %d",
				w.ID, w.Name, item.MaxUpgrade, w.MaxUpgrade)
		}
		hasDLC := false
		for _, f := range item.Flags {
			if f == "dlc" {
				hasDLC = true
				break
			}
		}
		if hasDLC != w.IsDLC {
			t.Errorf("Weapons[0x%08X] (%s) dlc flag = %v, want %v",
				w.ID, w.Name, hasDLC, w.IsDLC)
		}
	}
}

// TestBeastClawIDCollisionFixed verifies the Beast Claw ID collision repair:
//   - 0x04153A20 (weapon ID) lives in Weapons, NOT in Incantations
//   - 0x40001AA4 (the real Beast Claw incantation Goods ID) remains intact
func TestBeastClawIDCollisionFixed(t *testing.T) {
	const weaponID = uint32(0x04153A20)
	const incantationID = uint32(0x40001AA4)

	if _, ok := Incantations[weaponID]; ok {
		t.Errorf("Incantations[0x%08X] still present — should have been removed "+
			"(this is a weapon ID, not an incantation)", weaponID)
	}

	w, ok := Weapons[weaponID]
	if !ok {
		t.Fatalf("Weapons[0x%08X] missing — Beast Claw weapon entry not added", weaponID)
	}
	if w.Name != "Beast Claw" {
		t.Errorf("Weapons[0x%08X] name = %q, want %q", weaponID, w.Name, "Beast Claw")
	}

	inc, ok := Incantations[incantationID]
	if !ok {
		t.Fatalf("Incantations[0x%08X] (Beast Claw incantation) missing — "+
			"must remain after the ID collision fix", incantationID)
	}
	if inc.Name != "Beast Claw" {
		t.Errorf("Incantations[0x%08X] name = %q, want %q",
			incantationID, inc.Name, "Beast Claw")
	}
}

// TestPhase2B1MaxUpgradeSanity is an explicit somber/standard split check.
// Sacred Relic Sword and Mohgwyn's Sacred Spear have reinforceTypeId=2200
// (somber path) and must be capped at +10. The other four use the standard
// +25 reinforce path.
func TestPhase2B1MaxUpgradeSanity(t *testing.T) {
	somber := map[uint32]string{
		0x002F4D60: "Sacred Relic Sword",
		0x01038D50: "Mohgwyn's Sacred Spear",
	}
	standard := map[uint32]string{
		0x005BDBA0: "Great Épée",
		0x00170A70: "Fire Knight's Shortsword",
		0x0044F840: "Fire Knight's Greatsword",
		0x04153A20: "Beast Claw",
	}
	for id, name := range somber {
		if got := Weapons[id].MaxUpgrade; got != 10 {
			t.Errorf("%s (0x%08X) MaxUpgrade = %d, want 10 (somber)", name, id, got)
		}
	}
	for id, name := range standard {
		if got := Weapons[id].MaxUpgrade; got != 25 {
			t.Errorf("%s (0x%08X) MaxUpgrade = %d, want 25 (standard)", name, id, got)
		}
	}
}
