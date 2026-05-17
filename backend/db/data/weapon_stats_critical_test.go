package data

import "testing"

// TestWeaponStatsV1CriticalKnownValues anchors WeaponStatsV1.Critical on
// three weapons whose in-game Critical is well-documented:
//
//	Misericorde                 → 140 (throwAtkRate 40 + 100 base)
//	Lordsworn's Straight Sword  → 110 (throwAtkRate 10 + 100 base)
//	Uchigatana                  → 100 (throwAtkRate  0 + 100 base, normal)
//
// EquipParamWeapon stores throwAtkRate as the *offset* above 100; the
// generator pre-adds the base so consumers can render it verbatim.
func TestWeaponStatsV1CriticalKnownValues(t *testing.T) {
	cases := []struct {
		name string
		id   uint32
		want int32
	}{
		{"Misericorde", 0x000FB770, 140},
		{"Lordsworn's Straight Sword", 0x001F20C0, 110},
		{"Uchigatana", 0x00895440, 100},
	}
	for _, c := range cases {
		v, ok := WeaponStatsV1ByID[c.id]
		if !ok {
			t.Errorf("%s (0x%08X) missing from WeaponStatsV1ByID", c.name, c.id)
			continue
		}
		if v.Critical != c.want {
			t.Errorf("%s Critical = %d, want %d", c.name, v.Critical, c.want)
		}
	}
}
