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

// Real DLC weapons whose wepType is NOT in WepTypeToCanMountBit must resolve to
// known=false (data insufficient) rather than known=true/incompatible. The
// caller (the active weapon-edit workspace flow) is responsible for the
// passthrough "allow" decision because GemMountType==2 still gates mounting;
// this DB lookup must only signal that it cannot decide, never a false reject.
//
// wepType/GemMountType verified against backend/db/data/weapon_gem_mount.go:
//   0x01E84800 Dragon Towershield → {WepType: 69, GemMountType: 2}
//   0x03F6B5A0 Great Katana (DLC) → {WepType: 94, GemMountType: 2}
// Neither 69 nor 94 has a WepTypeToCanMountBit entry. Sword Dance (0x80003070)
// is a real AoW with a non-zero bitmask, so the unknown result comes solely
// from the unmapped weapon type.
func TestIsAshOfWarCompatibleWithWeapon_DLCUnmappedWepType_Unknown(t *testing.T) {
	const swordDance = uint32(0x80003070)
	cases := []struct {
		name    string
		weapon  uint32
		wepType uint16
	}{
		{"Dragon Towershield", 0x01E84800, 69},
		{"Great Katana", 0x03F6B5A0, 94},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			compatible, known := IsAshOfWarCompatibleWithWeapon(swordDance, c.weapon)
			if known {
				t.Errorf("%s (0x%08X, wepType=%d): expected known=false for unmapped DLC wepType",
					c.name, c.weapon, c.wepType)
			}
			// known=false is fail-closed: compatible is deliberately false so a
			// caller that ignores `known` cannot read it as an affirmative match.
			if compatible {
				t.Errorf("%s (0x%08X): expected compatible=false (fail-closed) when known=false",
					c.name, c.weapon)
			}
		})
	}
}
