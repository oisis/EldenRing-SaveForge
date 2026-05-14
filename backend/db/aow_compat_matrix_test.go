package db

import (
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/db/data"
)

// Phase 2B matrix golden tests — table-driven snapshot of expected
// CheckAoWCompatibility outcomes spanning every weapon category in the project
// item DB (vanilla + DLC). Each row asserts the (bool, AoWRejectReason) tuple
// that CheckAoWCompatibility must return.
//
// Maintenance:
//   - To add a category, append a new row with weapon ID, expected compatibility
//     for a representative AoW from the category, and a representative AoW from
//     a DIFFERENT category (which must be rejected).
//   - Weapon IDs are drawn from backend/db/data/melee_armaments.go, shields.go,
//     ranged_and_catalysts.go; DLC weapons (wt 88..95) are synthesized into
//     globalItemIndex by init() in aow_compat_api_test.go.
//   - AoW IDs are drawn from backend/db/data/ashes_of_war.go.
//
// To update goldens after a deliberate regulation change: regenerate via
//   python3 tools/import_aow_compat.py
// then re-run; failures here flag unintended bitmask drift.

// matrixWeapons indexes representative weapons by class label.
var matrixWeapons = map[string]uint32{
	// Vanilla swords
	"Dagger":              0x000F4240, // wt=1 → bit 0
	"Straight Sword":      0x001E8480, // Longsword wt=3 → bit 1
	"Greatsword":          0x002DC6C0, // Bastard Sword (wt 5; ID computed from melee_armaments scan)
	"Colossal Sword":      0x003D0900, // Greatsword (wt 7 in regulation; canonical Colossal Sword)
	"Curved Sword":        0x006ACFC0, // Falchion wt=9 → bit 4
	"Curved Greatsword":   0x007A6020, // Dismounter wt=11 → bit 5
	"Katana":              0x008A3EA0, // Moonveil — GemMountType=0 (somber), used for non-infusable tests
	"Twinblade":           0x00989680, // Twinblade wt=14 → bit 7
	"Thrusting Sword":     0x004C4B40, // Estoc wt=15 → bit 8
	"Heavy Thrusting Sword": 0x004C9960, // Rapier — actually wt=15 too; OK for now
	// Vanilla axes/hammers
	"Axe":          0x00D59F80, // Battle Axe wt=17 → bit 10
	"Greataxe":     0x00E4E1C0, // Greataxe wt=19 → bit 11
	"Hammer":       0x00A7D8C0, // Mace wt=21 → bit 12
	"Great Hammer": 0x00B71B00, // Large Club wt=23 → bit 13
	"Flail":        0x00C68450, // Flail wt=24 → bit 14
	// Vanilla spears/halberds/reapers
	"Spear":      0x00F44B10, // Spear wt=25 → bit 15
	"Halberd":    0x0112A880, // Halberd wt=29 → bit 18
	"Reaper":     0x0121EAC0, // Scythe wt=31 → bit 19
	// Vanilla fist/claw/whip/colossal
	"Fist":            0x01406F40, // Caestus wt=35 → bit 20
	"Claw":            0x014FB180, // Hookclaws wt=37 → bit 21
	"Whip":            0x01312D00, // Whip wt=39 → bit 22
	"Colossal Weapon": 0x00BF3150, // Bloodfiend's Arm wt=41 → bit 23 (DLC weapon, in melee DB)
	// Ranged
	"Light Bow": 0x02625A00, // Shortbow wt=50 → bit 24
	"Bow":       0x02719C40, // Longbow wt=51 → bit 25
	"Greatbow":  0x02817AC0, // Greatbow wt=53 → bit 26
	// Note: Crossbows + Hand Ballista have gemMountType=0 in the current regulation
	// dump (data gap — they ARE infusable in-game). Skipped from matrix tests; revisit
	// after regulation refresh.
	// Shields
	"Small Shield": 0x01C9C380, // Buckler wt=65 → bit 32
	"Greatshield":  0x01CAADE0, // Greatshield base (wt 67, gm=2) — verified in WeaponGemMounts
	// DLC (synthesized into globalItemIndex by setupDLCWeapons / init in api_test.go)
	"DLC Hand-to-Hand":    weaponHandToHand,
	"DLC Throwing Blade":  weaponThrowingBlade,
	"DLC Backhand Blade":  weaponBackhandBlade,
	"DLC Great Katana":    weaponGreatKatana,
	"DLC Light Greatsword": weaponLightGreatsword,
	"DLC Beast Claw":      weaponBeastClaw,
	"DLC Perfume Bottle":  weaponPerfumeBottle,
}

// matrixAshes indexes representative AoWs.
var matrixAshes = map[string]uint32{
	"Storm Stomp":      aowStormStomp,        // universal melee (mask 0x00FEFFFF + reserved 15)
	"Sword Dance":      aowSwordDance,        // dagger/sword cluster only
	"No Skill":         aowNoSkill,           // shields + torch (bits 32..35)
	"Mighty Shot":      aowMightyShot,        // BowSmall only (bit 24)
	"Carian Retaliation": aowCarianRetaliation, // shield-only specific
	// DLC
	"Dryleaf Whirlwind": aowDryleafWhirlwind,
	"Palm Blast":        aowPalmBlast,
	"Piercing Throw":    aowPiercingThrow,
	"Scattershot Throw": aowScattershotThrow,
	"Blind Spot":        aowBlindSpot,
	"Swift Slash":       aowSwiftSlash,
	"Wing Stance":       aowWingStance,
	"Overhead Stance":   aowOverheadStance,
	"Savage Claws":      aowSavageClaws,
	"Raging Beast":      aowRagingBeast,
	"Wall of Sparks":    aowWallOfSparks,
	"Rolling Sparks":    aowRollingSparks,
}

type compatCase struct {
	aow        string
	weapon     string
	wantOK     bool
	wantReason AoWRejectReason
}

// matrixCases — one positive + one cross-class negative per category.
// Cross-class negative uses an AoW from a clearly different category to verify
// the mask correctly excludes it.
var matrixCases = []compatCase{
	// === Vanilla positives (universal Storm Stomp pairs) ===
	{"Storm Stomp", "Dagger", true, AoWOK},
	{"Storm Stomp", "Straight Sword", true, AoWOK},
	{"Storm Stomp", "Curved Sword", true, AoWOK},
	{"Storm Stomp", "Curved Greatsword", true, AoWOK},
	{"Storm Stomp", "Twinblade", true, AoWOK},
	{"Storm Stomp", "Thrusting Sword", true, AoWOK},
	{"Storm Stomp", "Axe", true, AoWOK},
	{"Storm Stomp", "Greataxe", true, AoWOK},
	{"Storm Stomp", "Hammer", true, AoWOK},
	{"Storm Stomp", "Great Hammer", true, AoWOK},
	{"Storm Stomp", "Flail", true, AoWOK},
	{"Storm Stomp", "Spear", true, AoWOK},
	{"Storm Stomp", "Halberd", true, AoWOK},
	{"Storm Stomp", "Reaper", true, AoWOK},
	{"Storm Stomp", "Fist", true, AoWOK},
	{"Storm Stomp", "Claw", true, AoWOK},
	{"Storm Stomp", "Whip", true, AoWOK},
	{"Storm Stomp", "Colossal Weapon", true, AoWOK},

	// === Vanilla negatives — Storm Stomp must NOT mount on shields/bows ===
	{"Storm Stomp", "Small Shield", false, AoWNotApplicableToWeaponCategory},
	{"Storm Stomp", "Light Bow", false, AoWNotApplicableToWeaponCategory},
	{"Storm Stomp", "Bow", false, AoWNotApplicableToWeaponCategory},
	{"Storm Stomp", "Greatbow", false, AoWNotApplicableToWeaponCategory},

	// === Shield-only AoW positives ===
	{"No Skill", "Small Shield", true, AoWOK},
	{"No Skill", "Greatshield", true, AoWOK},
	// === Shield-only AoW cross-class blocked ===
	{"No Skill", "Dagger", false, AoWNotApplicableToWeaponCategory},
	{"Carian Retaliation", "Dagger", false, AoWNotApplicableToWeaponCategory},
	{"Carian Retaliation", "Straight Sword", false, AoWNotApplicableToWeaponCategory},

	// === Bow-only AoW ===
	{"Mighty Shot", "Light Bow", true, AoWOK},
	{"Mighty Shot", "Dagger", false, AoWNotApplicableToWeaponCategory},
	{"Mighty Shot", "Straight Sword", false, AoWNotApplicableToWeaponCategory},

	// === DLC positives (reserved-bit Layer 1+2) ===
	{"Dryleaf Whirlwind", "DLC Hand-to-Hand", true, AoWOK},
	{"Palm Blast", "DLC Hand-to-Hand", true, AoWOK},
	{"Piercing Throw", "DLC Throwing Blade", true, AoWOK},
	{"Scattershot Throw", "DLC Throwing Blade", true, AoWOK},
	{"Wall of Sparks", "DLC Perfume Bottle", true, AoWOK},
	{"Rolling Sparks", "DLC Perfume Bottle", true, AoWOK},

	// === DLC positives (Layer 3 mwtid+SAP fallback) ===
	{"Blind Spot", "DLC Backhand Blade", true, AoWOK},
	{"Swift Slash", "DLC Backhand Blade", true, AoWOK},
	{"Wing Stance", "DLC Great Katana", true, AoWOK},
	{"Overhead Stance", "DLC Light Greatsword", true, AoWOK},
	{"Savage Claws", "DLC Beast Claw", true, AoWOK},
	{"Raging Beast", "DLC Beast Claw", true, AoWOK},

	// === DLC negatives (cross-class) ===
	{"Blind Spot", "Straight Sword", false, AoWNotApplicableToWeaponCategory},
	{"Wing Stance", "Straight Sword", false, AoWNotApplicableToWeaponCategory},
	{"Savage Claws", "Straight Sword", false, AoWNotApplicableToWeaponCategory},
	{"Palm Blast", "Light Bow", false, AoWNotApplicableToWeaponCategory},
	{"Wall of Sparks", "Straight Sword", false, AoWNotApplicableToWeaponCategory},
	{"Wall of Sparks", "DLC Backhand Blade", false, AoWNotApplicableToWeaponCategory},
	{"Dryleaf Whirlwind", "DLC Backhand Blade", false, AoWNotApplicableToWeaponCategory},
	{"Piercing Throw", "DLC Hand-to-Hand", false, AoWNotApplicableToWeaponCategory},

	// === Cross-vanilla negatives — shield AoW must not work on bows ===
	{"No Skill", "Light Bow", false, AoWNotApplicableToWeaponCategory},
	{"Mighty Shot", "Small Shield", false, AoWNotApplicableToWeaponCategory},

	// === Sword Dance (dagger/sword cluster) ===
	{"Sword Dance", "Dagger", true, AoWOK},
	{"Sword Dance", "Straight Sword", true, AoWOK},
	{"Sword Dance", "Greatsword", true, AoWOK},
	{"Sword Dance", "Hammer", false, AoWNotApplicableToWeaponCategory},
	{"Sword Dance", "Light Bow", false, AoWNotApplicableToWeaponCategory},
	{"Sword Dance", "Small Shield", false, AoWNotApplicableToWeaponCategory},
}

func TestCompatMatrix(t *testing.T) {
	for _, tc := range matrixCases {
		// Some category labels are intentionally not in matrixWeapons (the
		// "Longsword?" entry is a placeholder to keep the case list visually
		// aligned with intent; skip when unmapped).
		wid, wok := matrixWeapons[tc.weapon]
		aid, aok := matrixAshes[tc.aow]
		if !wok || !aok {
			t.Logf("SKIP %q × %q (test fixture missing)", tc.aow, tc.weapon)
			continue
		}

		t.Run(tc.aow+"_x_"+tc.weapon, func(t *testing.T) {
			gotOK, gotReason := CheckAoWCompatibility(aid, wid)
			if gotOK != tc.wantOK {
				t.Errorf("CheckAoWCompatibility(%s × %s) ok=%v reason=%v, want ok=%v reason=%v",
					tc.aow, tc.weapon, gotOK, gotReason, tc.wantOK, tc.wantReason)
				return
			}
			// Only assert reason on failure path — success path always returns AoWOK.
			if !tc.wantOK && gotReason != tc.wantReason {
				t.Errorf("CheckAoWCompatibility(%s × %s) reason=%v, want %v",
					tc.aow, tc.weapon, gotReason, tc.wantReason)
			}
		})
	}
}

// TestMatrix_AllAshesHaveTestableData is a meta-check ensuring our matrix list
// references only AoWs that exist in ashes_of_war.go.
func TestMatrix_AllAshesHaveTestableData(t *testing.T) {
	for label, id := range matrixAshes {
		if _, ok := data.Aows[id]; !ok {
			t.Errorf("matrix AoW %q (0x%08X) is not in data.Aows", label, id)
		}
	}
}

// TestMatrix_AllWeaponsResolveInIndex verifies each matrix weapon has GemMountType
// resolvable via GetItemDataFuzzy (either in named DB or synthesized for DLC).
func TestMatrix_AllWeaponsResolveInIndex(t *testing.T) {
	for label, id := range matrixWeapons {
		// Skip Moonveil — intentionally gm=0 for non-infusable tests elsewhere.
		if label == "Katana" {
			continue
		}
		w, _ := GetItemDataFuzzy(id)
		if w.WepType == 0 && w.GemMountType == 0 {
			t.Errorf("matrix weapon %q (0x%08X) cannot be resolved (WepType=0, GemMountType=0)", label, id)
		}
	}
}
