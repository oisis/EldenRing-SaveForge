package db

import "testing"

// Test data references (from backend/db/data/aow_compat.go and weapon_gem_mount.go):
//
//   AoW 0x80003070  "Sword Dance"  bitmask 0x000C8CF7 — bit 0 (Dagger) = 1, bit 17 (SpearHeavy) = 0
//   AoW 0x80007530  shield-only    bitmask 0x0000000700000000 — ShieldSmall (32), ShieldNormal (33), ShieldLarge (34) only
//   Dagger baseID   0x000F4240  wepType=1, GemMountType=2 (standard infusable)
//   SpearHeavy      wepType=32  → bit 17 in bitmask

func TestIsAoWCompatibleWithWepType_CompatibleDagger(t *testing.T) {
	// AoW 0x80003070 (Sword Dance) has bit 0 (Dagger/wepType=1) set.
	compatible, known := IsAoWCompatibleWithWepType(0x80003070, 1)
	if !known {
		t.Fatal("expected known=true")
	}
	if !compatible {
		t.Error("expected compatible=true for Dagger wepType + AoW 0x80003070")
	}
}

func TestIsAoWCompatibleWithWepType_IncompatibleSpearHeavy(t *testing.T) {
	// AoW 0x80003070 (Sword Dance) has bit 17 (SpearHeavy/wepType=32) = 0.
	compatible, known := IsAoWCompatibleWithWepType(0x80003070, 32)
	if !known {
		t.Fatal("expected known=true")
	}
	if compatible {
		t.Error("expected compatible=false for SpearHeavy wepType + AoW 0x80003070")
	}
}

func TestIsAoWCompatibleWithWepType_ShieldOnlyAoW(t *testing.T) {
	// AoW 0x80007530 is shield-only (bits 32-34). Dagger (wepType=1, bit=0) → incompatible.
	compatible, known := IsAoWCompatibleWithWepType(0x80007530, 1)
	if !known {
		t.Fatal("expected known=true")
	}
	if compatible {
		t.Error("expected compatible=false for Dagger + shield-only AoW 0x80007530")
	}
}

func TestIsAoWCompatibleWithWepType_ShieldOnlyAoW_ShieldCompatible(t *testing.T) {
	// AoW 0x80007530 should be compatible with ShieldSmall (wepType=65, bit=32).
	compatible, known := IsAoWCompatibleWithWepType(0x80007530, 65)
	if !known {
		t.Fatal("expected known=true")
	}
	if !compatible {
		t.Error("expected compatible=true for ShieldSmall + shield-only AoW 0x80007530")
	}
}

func TestIsAoWCompatibleWithWepType_ZeroBitmask(t *testing.T) {
	// An AoW not in the compat map has bitmask=0 → known=false, compatible=false (fail-closed).
	compatible, known := IsAoWCompatibleWithWepType(0x8FFFFFFF, 1)
	if known {
		t.Error("expected known=false for AoW with zero bitmask")
	}
	if compatible {
		t.Error("expected compatible=false (fail-closed) for unknown bitmask")
	}
}

func TestIsAoWCompatibleWithWepType_UnknownWepType(t *testing.T) {
	// wepType 99 is not in WepTypeToCanMountBit → known=false, compatible=false (fail-closed).
	compatible, known := IsAoWCompatibleWithWepType(0x80003070, 99)
	if known {
		t.Error("expected known=false for unknown wepType")
	}
	if compatible {
		t.Error("expected compatible=false (fail-closed) for unknown wepType")
	}
}

func TestIsAshOfWarCompatibleWithWeapon_Compatible(t *testing.T) {
	// Dagger (0x000F4240, wepType=1, GemMountType=2) + AoW 0x80003070 (bit 0 set) → compatible.
	compatible, known := IsAshOfWarCompatibleWithWeapon(0x80003070, 0x000F4240)
	if !known {
		t.Fatal("expected known=true")
	}
	if !compatible {
		t.Error("expected compatible=true for Dagger + AoW 0x80003070 (Sword Dance)")
	}
}

func TestIsAshOfWarCompatibleWithWeapon_Incompatible(t *testing.T) {
	// Dagger (wepType=1) + shield-only AoW 0x80007530 → incompatible.
	compatible, known := IsAshOfWarCompatibleWithWeapon(0x80007530, 0x000F4240)
	if !known {
		t.Fatal("expected known=true")
	}
	if compatible {
		t.Error("expected compatible=false for Dagger + shield-only AoW 0x80007530")
	}
}

func TestIsAshOfWarCompatibleWithWeapon_UpgradedWeapon(t *testing.T) {
	// Upgraded Dagger +3 = 0x000F4240 + 3 = 0x000F4243.
	// GetItemDataFuzzy resolves it to the base Dagger entry (with WepType=1, GemMountType=2).
	compatible, known := IsAshOfWarCompatibleWithWeapon(0x80003070, 0x000F4243)
	if !known {
		t.Fatal("expected known=true — fuzzy lookup should resolve upgraded weapon to its base")
	}
	if !compatible {
		t.Error("expected compatible=true for upgraded Dagger + AoW 0x80003070")
	}
}

func TestIsAshOfWarCompatibleWithWeapon_RemoveAlwaysAllowed(t *testing.T) {
	// newAoWItemID==0 means remove — compatibility is NOT checked for remove.
	// This test verifies the edge: weaponItemID with GemMountType=1 (somber) → incompatible for any set AoW.
	// 0x000186A0 is in WeaponGemMounts as somber (GemMountType=1) but not in the named weapon DB,
	// so GetItemDataFuzzy returns an entry with GemMountType=0 → also fails the GemMountType=2 gate.
	compatible, known := IsAshOfWarCompatibleWithWeapon(0x80003070, 0x000186A0)
	if !known {
		t.Fatal("expected known=true (GemMountType gate returns known=true for non-infusable)")
	}
	if compatible {
		t.Error("expected compatible=false for non-standard-infusable weapon")
	}
}

// Star Fist (0x0141A7C0) is a Fist weapon with wepType=35, GemMountType=2.
// wepType 35 must map to bit 20 (canMountWep_Knuckle). Regression guard against the
// historical 35→16 (SpearLarge) mapping that hid every Fist-compatible AoW from the modal.
func TestIsAshOfWarCompatibleWithWeapon_StarFist_Fist_AoWs(t *testing.T) {
	const starFist = uint32(0x0141A7C0)
	cases := []struct {
		name string
		aow  uint32
	}{
		{"Lifesteal Fist", 0x80005014},
		{"Cragblade", 0x8000ED1C},
		{"Endure", 0x80011170},
		{"Quickstep", 0x80013880},
		{"Bloodhound's Step", 0x800138E4},
	}
	for _, c := range cases {
		compatible, known := IsAshOfWarCompatibleWithWeapon(c.aow, starFist)
		if !known {
			t.Errorf("%s: expected known=true", c.name)
			continue
		}
		if !compatible {
			t.Errorf("%s (0x%08X): expected compatible=true on Star Fist (wepType=35)", c.name, c.aow)
		}
	}
}

func TestIsAshOfWarCompatibleWithWeapon_StarFist_ShieldOnlyAoW_Incompatible(t *testing.T) {
	// Shield-only AoW 0x80007530 (bits 32-34 only) → not compatible with Star Fist (wepType=35, bit 20).
	compatible, known := IsAshOfWarCompatibleWithWeapon(0x80007530, 0x0141A7C0)
	if !known {
		t.Fatal("expected known=true")
	}
	if compatible {
		t.Error("expected compatible=false for shield-only AoW on Star Fist")
	}
}

// Real DLC/base weapons whose wepType was missing from WepTypeToCanMountBit
// must resolve to known compatibility instead of disappearing from the modal.
//
// wepType/GemMountType verified against backend/db/data/weapon_gem_mount.go:
//   - 0x01E84800 Dragon Towershield → {WepType: 69, GemMountType: 2}
//   - 0x03F6B5A0 Great Katana (DLC) → {WepType: 94, GemMountType: 2}
//   - 0x04153A20 Beast Claw (DLC) → {WepType: 95, GemMountType: 2}
func TestIsAshOfWarCompatibleWithWeapon_DLCWepTypesMapped(t *testing.T) {
	cases := []struct {
		name   string
		weapon uint32
		aow    uint32
	}{
		{"Dragon Towershield + Shield Bash", 0x01E84800, 0x80007530},
		{"Great Katana + Sword Dance", 0x03F6B5A0, 0x80003070},
		{"Beast Claw + Cragblade", 0x04153A20, 0x8000ED1C},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			compatible, known := IsAshOfWarCompatibleWithWeapon(c.aow, c.weapon)
			if !known {
				t.Fatalf("%s: expected known=true", c.name)
			}
			if !compatible {
				t.Errorf("%s: expected compatible=true", c.name)
			}
		})
	}
}

func TestAoWCompatMasks_UnresolvedDLCAoWsFailClosed(t *testing.T) {
	expected := map[uint32]string{
		0x80030D40: "Dryleaf Whirlwind",
		0x80061E68: "Palm Blast",
		0x80062250: "Piercing Throw",
		0x80062638: "Scattershot Throw",
		0x80062A20: "Wall of Sparks",
		0x80062E08: "Rolling Sparks",
		0x800631F0: "Raging Beast",
		0x800635D8: "Savage Claws",
		0x80063DA8: "Blind Spot",
		0x80064190: "Swift Slash",
		0x80064578: "Overhead Stance",
		0x80064960: "Wing Stance",
	}

	seen := make(map[uint32]string, len(expected))
	for _, item := range GetItemsByCategory("ashes_of_war", "") {
		isDLC := false
		for _, flag := range item.Flags {
			if flag == "dlc" {
				isDLC = true
				break
			}
		}
		if !isDLC || item.AoWCompatBitmask != 0 {
			continue
		}

		seen[item.ID] = item.Name
		if expectedName, ok := expected[item.ID]; !ok {
			t.Errorf("unexpected unresolved DLC AoW 0x%08X %q", item.ID, item.Name)
		} else if item.Name != expectedName {
			t.Errorf("unresolved DLC AoW 0x%08X name=%q, want %q", item.ID, item.Name, expectedName)
		}

		compatible, known := IsAoWCompatibleWithWepType(item.ID, 1)
		if known || compatible {
			t.Errorf("%s (0x%08X): expected known=false compatible=false for missing mask", item.Name, item.ID)
		}
	}

	for id, name := range expected {
		if _, ok := seen[id]; !ok {
			t.Errorf("expected unresolved DLC AoW 0x%08X %q not found", id, name)
		}
	}
}
