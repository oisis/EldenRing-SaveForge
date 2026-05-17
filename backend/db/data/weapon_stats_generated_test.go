package data

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestWeaponStatsV1HasKnownIDs anchors the generated table on a handful
// of canonical item IDs covering all four weapon-like categories plus a
// somber boss weapon (Sacred Relic Sword) and an ammo entry.
//
// Anchor IDs come from app_items.csv and the Phase 3A anchor list:
//   - Longsword                  0x001E8480  (melee_armaments, infusable)
//   - Lance                      0x010450A0  (melee_armaments, infusable)
//   - Sacred Relic Sword         0x002F4D60  (melee_armaments, somber)
//   - Great Épée                 0x005BDBA0  (melee_armaments, infusable)
//   - Fire Knight's Greatsword   0x0044F840  (melee_armaments, infusable)
//   - Beast Claw                 0x04153A20  (melee_armaments, infusable)
//   - Icon Shield                0x01EA6AE0  (shields, somber)
//   - Fire Arrow                 0x02FB1790  (arrows_and_bolts)
//   - Lightning Bolt             0x03199C10  (arrows_and_bolts)
func TestWeaponStatsV1HasKnownIDs(t *testing.T) {
	want := []uint32{
		0x001E8480, // Longsword
		0x010450A0, // Lance
		0x002F4D60, // Sacred Relic Sword
		0x005BDBA0, // Great Épée
		0x0044F840, // Fire Knight's Greatsword
		0x04153A20, // Beast Claw
		0x01EA6AE0, // Icon Shield
		0x02FB1790, // Fire Arrow
		0x03199C10, // Lightning Bolt
	}
	for _, id := range want {
		if _, ok := WeaponStatsV1ByID[id]; !ok {
			t.Errorf("WeaponStatsV1ByID[0x%08X] missing", id)
		}
	}
}

// TestWeaponStatsV1Coverage asserts every item in the four weapon-like
// app maps has a generated stats entry. Phase 3C.1 ships no allow-list —
// the generator reads app_items.csv directly, so any miss would indicate
// a regulation row that vanished or a category-classification bug.
func TestWeaponStatsV1Coverage(t *testing.T) {
	groups := map[string]map[uint32]ItemData{
		"melee_armaments":      Weapons,
		"shields":              Shields,
		"ranged_and_catalysts": RangedAndCatalysts,
		"arrows_and_bolts":     ArrowsAndBolts,
	}
	missing := map[string][]uint32{}
	total := 0
	for cat, m := range groups {
		for id := range m {
			total++
			if _, ok := WeaponStatsV1ByID[id]; !ok {
				missing[cat] = append(missing[cat], id)
			}
		}
	}
	if total == 0 {
		t.Fatal("category maps are empty; cannot validate coverage")
	}
	for cat, ids := range missing {
		for _, id := range ids {
			t.Errorf("WeaponStatsV1ByID[0x%08X] missing (category %s)", id, cat)
		}
	}
	if len(missing) == 0 {
		t.Logf("coverage OK: %d weapon-like items, all covered", total)
	}
}

// TestWeaponStatsV1HolyMapping is the R-STA-01 guard: it confirms that
// AttackHoly is populated for Sacred Relic Sword (the canonical holy
// weapon) and that GuardHoly is non-zero. The CSV column that backs
// AttackHoly is EquipParamWeapon.attackBaseDark — there is no separate
// "Holy" column in regulation data.
func TestWeaponStatsV1HolyMapping(t *testing.T) {
	const id uint32 = 0x002F4D60 // Sacred Relic Sword
	s, ok := WeaponStatsV1ByID[id]
	if !ok {
		t.Fatalf("WeaponStatsV1ByID[0x%08X] missing; cannot anchor holy test", id)
	}
	if s.AttackHoly == 0 {
		t.Errorf("Sacred Relic Sword AttackHoly = 0; expected non-zero (sourced from EquipParamWeapon.attackBaseDark)")
	}
	if s.GuardHoly == 0 {
		t.Errorf("Sacred Relic Sword GuardHoly = 0; expected non-zero (sourced from EquipParamWeapon.darkGuardCutRate)")
	}
}

// TestWeaponStatsV1MaxUpgradeSomberStandard locks down the somber /
// standard inference from ReinforceParamWeapon band sizes:
//   - 26-row band → IsSomber=false, MaxUpgrade=25
//   - 11-row band → IsSomber=true,  MaxUpgrade=10
func TestWeaponStatsV1MaxUpgradeSomberStandard(t *testing.T) {
	cases := []struct {
		id        uint32
		name      string
		wantSomb  bool
		wantMaxUp int32
	}{
		{0x001E8480, "Longsword", false, 25},
		{0x002F4D60, "Sacred Relic Sword", true, 10},
		{0x01EA6AE0, "Icon Shield", true, 10},
		{0x005BDBA0, "Great Épée", false, 25},
	}
	for _, c := range cases {
		s, ok := WeaponStatsV1ByID[c.id]
		if !ok {
			t.Errorf("%s (0x%08X) missing", c.name, c.id)
			continue
		}
		if s.IsSomber != c.wantSomb {
			t.Errorf("%s IsSomber = %v, want %v", c.name, s.IsSomber, c.wantSomb)
		}
		if s.MaxUpgrade != c.wantMaxUp {
			t.Errorf("%s MaxUpgrade = %d, want %d", c.name, s.MaxUpgrade, c.wantMaxUp)
		}
	}
}

// TestWeaponStatsV1GemMountType locks the EquipParamWeapon.gemMountType
// surface for known infusable / somber weapons.
//   - 2 = standard infusable
//   - 0 = no AoW slot (somber or unique)
func TestWeaponStatsV1GemMountType(t *testing.T) {
	cases := []struct {
		id      uint32
		name    string
		wantGMT uint8
	}{
		{0x001E8480, "Longsword", 2},
		{0x005BDBA0, "Great Épée", 2},
		{0x0044F840, "Fire Knight's Greatsword", 2},
		{0x04153A20, "Beast Claw", 2},
		{0x002F4D60, "Sacred Relic Sword", 0},
		{0x01EA6AE0, "Icon Shield", 0},
	}
	for _, c := range cases {
		s, ok := WeaponStatsV1ByID[c.id]
		if !ok {
			t.Errorf("%s (0x%08X) missing", c.name, c.id)
			continue
		}
		if s.GemMountType != c.wantGMT {
			t.Errorf("%s GemMountType = %d, want %d", c.name, s.GemMountType, c.wantGMT)
		}
	}
}

// TestWeaponStatsV1Ammo asserts arrows/bolts get stats entries with
// MaxUpgrade=0 (single-row reinforce band, no upgrades) and an
// ammo-bracket WepType (>= 80 in ER schema). It also confirms ammo rows
// carry no melee guard data (guard fields are zero per CSV).
func TestWeaponStatsV1Ammo(t *testing.T) {
	cases := []struct {
		id   uint32
		name string
	}{
		{0x02FB1790, "Fire Arrow"},
		{0x03199C10, "Lightning Bolt"},
	}
	for _, c := range cases {
		s, ok := WeaponStatsV1ByID[c.id]
		if !ok {
			t.Errorf("%s (0x%08X) missing", c.name, c.id)
			continue
		}
		if s.MaxUpgrade != 0 {
			t.Errorf("%s MaxUpgrade = %d, want 0 (ammo)", c.name, s.MaxUpgrade)
		}
		if s.WepType < 80 {
			t.Errorf("%s WepType = %d, want >= 80 (ammo bracket in ER schema)", c.name, s.WepType)
		}
		if s.GuardPhysical != 0 || s.GuardMagic != 0 || s.GuardBoost != 0 {
			t.Errorf("%s carries guard data (Phys=%d Mag=%d Boost=%d); ammo rows should have zero guard",
				c.name, s.GuardPhysical, s.GuardMagic, s.GuardBoost)
		}
	}
}

// TestWeaponStatsV1GeneratorReproducible runs the generator a second
// time and asserts the generated file's SHA256 is unchanged. We hash
// before and after invocation rather than diffing to keep the test
// lightweight, and skip when the generator can't be invoked (e.g. in a
// minimal sandbox without `go run`).
func TestWeaponStatsV1GeneratorReproducible(t *testing.T) {
	// Resolve repo root: this test file lives at backend/db/data, so walk up.
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd: %v", err)
	}
	repoRoot := filepath.Join(wd, "..", "..", "..")
	genScript := filepath.Join(repoRoot, "tmp", "scripts", "generate_weapon_stats.go")
	if _, err := os.Stat(genScript); err != nil {
		t.Skipf("generator script not found at %s: %v", genScript, err)
	}
	target := filepath.Join(repoRoot, "backend", "db", "data", "weapon_stats_generated.go")

	hashBefore, err := sha256OfFile(target)
	if err != nil {
		t.Fatalf("hash before: %v", err)
	}

	cmd := exec.Command("go", "run", "tmp/scripts/generate_weapon_stats.go")
	cmd.Dir = repoRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("re-run generator: %v\n%s", err, out)
	}

	hashAfter, err := sha256OfFile(target)
	if err != nil {
		t.Fatalf("hash after: %v", err)
	}
	if hashBefore != hashAfter {
		t.Errorf("generator output changed across runs:\n  before: %s\n  after:  %s\n  output:\n%s",
			hashBefore, hashAfter, out)
	}
}

func sha256OfFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// TestWeaponStatsV1NoPanicOnMissing asserts that map lookups for unknown
// IDs return the zero value without panic. The zero WeaponStatsV1 must
// look like a fresh value (ItemID == 0, no warnings).
func TestWeaponStatsV1NoPanicOnMissing(t *testing.T) {
	const unknown uint32 = 0xDEADBEEF
	if _, ok := WeaponStatsV1ByID[unknown]; ok {
		t.Fatalf("0x%08X collides with a known item; pick a different sentinel", unknown)
	}
	s := WeaponStatsV1ByID[unknown] // must not panic
	if s.ItemID != 0 || s.AttackPhysical != 0 || len(s.Warnings) != 0 {
		t.Errorf("missing lookup returned non-zero value: %+v", s)
	}
}
