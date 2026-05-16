package data

import "testing"

// somberGreatshields lists every greatshield that uses the Somber Smithing
// Stone reinforce path (reinforceTypeId=8300, gemMountType=0 in
// regulation.bin → EquipParamWeapon). These weapons must have MaxUpgrade=10,
// not 25 — the editor's add-items pipeline routes MaxUpgrade==10 items through
// a branch that ignores infusion offsets and uses the upgrade10 slider,
// matching how Elden Ring actually represents somber upgrades.
//
// A wrong MaxUpgrade here lets the frontend pick infusions (Heavy/Keen/…) or
// upgrade levels above +10, both of which produce ItemIDs that don't exist in
// the game's parameter tables, making the item invisible after the save is
// loaded in-game.
// Decimal IDs map back to EquipParamWeapon rows where reinforceTypeId=8300 and
// gemMountType=0. 32690000 is also somber but is absent from the Shields map
// (no editor-facing definition), so it is intentionally excluded.
var somberGreatshields = []struct {
	ID      uint32
	DecID   uint32
	Name    string
}{
	{0x01E8BD30, 32030000, "Crucible Hornshield"},
	{0x01E8E440, 32040000, "Dragonclaw Shield"},
	{0x01E98080, 32080000, "Erdtree Greatshield"},
	{0x01EA1CC0, 32120000, "Jellyfish Shield"},
	{0x01EA6AE0, 32140000, "Icon Shield"},
	{0x01EA91F0, 32150000, "One-Eyed Shield"},
	{0x01EAB900, 32160000, "Visage Shield"},
	{0x01EBA360, 32220000, "Ant's Skull Plate"},
	{0x01F03740, 32520000, "Verdigris Greatshield"},
}

func TestSomberGreatshields_MaxUpgradeIs10(t *testing.T) {
	for _, sg := range somberGreatshields {
		if sg.ID != sg.DecID {
			t.Errorf("table-row sanity: 0x%08X (%s) != %d decimal", sg.ID, sg.Name, sg.DecID)
		}
		item, ok := Shields[sg.ID]
		if !ok {
			t.Errorf("0x%08X (%s): not present in Shields map", sg.ID, sg.Name)
			continue
		}
		if item.Name != sg.Name {
			t.Errorf("0x%08X: name mismatch — DB=%q, expected=%q", sg.ID, item.Name, sg.Name)
		}
		if item.MaxUpgrade != 10 {
			t.Errorf("0x%08X (%s): MaxUpgrade=%d, want 10 (somber)", sg.ID, sg.Name, item.MaxUpgrade)
		}
		if item.Category != "shields" {
			t.Errorf("0x%08X (%s): Category=%q, want %q", sg.ID, sg.Name, item.Category, "shields")
		}
	}
}

// TestDragonTowershield_StaysStandard guards that the somber fix did not
// accidentally hit Dragon Towershield (0x01E84800 = 32000000), which uses the
// standard reinforce path (reinforceTypeId=8200, gemMountType=2) and must
// keep MaxUpgrade=25.
func TestDragonTowershield_StaysStandard(t *testing.T) {
	item, ok := Shields[0x01E84800]
	if !ok {
		t.Fatal("Dragon Towershield (0x01E84800) missing from Shields map")
	}
	if item.Name != "Dragon Towershield" {
		t.Errorf("0x01E84800 name=%q, want %q", item.Name, "Dragon Towershield")
	}
	if item.MaxUpgrade != 25 {
		t.Errorf("Dragon Towershield MaxUpgrade=%d, want 25 (standard infusable)", item.MaxUpgrade)
	}
}

// TestFingerprintStoneShield_StaysStandard guards that the fix does not
// accidentally downgrade infusable greatshields. Fingerprint Stone Shield uses
// the standard reinforce path (reinforceTypeId=8200, gemMountType=2 in
// regulation) and must remain MaxUpgrade=25 so it keeps showing infusion and
// the +0..+25 slider.
func TestFingerprintStoneShield_StaysStandard(t *testing.T) {
	item, ok := Shields[0x01EA43D0]
	if !ok {
		t.Fatal("Fingerprint Stone Shield (0x01EA43D0) missing from Shields map")
	}
	if item.MaxUpgrade != 25 {
		t.Errorf("Fingerprint Stone Shield MaxUpgrade=%d, want 25 (standard infusable)", item.MaxUpgrade)
	}
}
