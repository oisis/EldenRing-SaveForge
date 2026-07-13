package data

import "testing"

// Thrusting Shields (Issue #6) are classified by canonical wepType == 90, not
// by name (covers the Dueling Shield + Carian Thrusting Shield families).

// 1. Every wepType == 90 shield resolves to the Thrusting Shields sub-group.
func TestThrustingShields_AllVariantsClassified(t *testing.T) {
	count := 0
	for id, item := range Shields {
		if weaponWepType[id] != 90 {
			continue
		}
		count++
		if item.SubCategory != SubcatShieldsThrusting {
			t.Errorf("%q (0x%08X): SubCategory = %q, want %q",
				item.Name, id, item.SubCategory, SubcatShieldsThrusting)
		}
	}
	if count != 26 {
		t.Errorf("found %d wepType-90 shields, want 26 (13 Dueling + 13 Carian Thrusting)", count)
	}
}

//  2. Ordinary Small/Medium/Great shields and Torches keep their sub-groups —
//     the new rule must not disturb existing classification.
func TestThrustingShields_OtherShieldsUnchanged(t *testing.T) {
	cases := []struct {
		id   uint32
		want string
	}{
		{0x01C9C380, SubcatShieldsSmall},        // Buckler
		{0x01CAFC00, SubcatShieldsSmall},        // Scripture Wooden Shield
		{0x01D905C0, SubcatShieldsMedium},       // Kite Shield
		{0x01DB0190, SubcatShieldsMedium},       // Brass Shield
		{0x01E89620, SubcatShieldsGreatshields}, // Distinguished Greatshield
		{0x01EFE920, SubcatShieldsGreatshields}, // Black Steel Greatshield
	}
	for _, c := range cases {
		item, ok := Shields[c.id]
		if !ok {
			t.Fatalf("shield 0x%08X missing from Shields map", c.id)
		}
		if item.SubCategory != c.want {
			t.Errorf("%q (0x%08X): SubCategory = %q, want %q",
				item.Name, c.id, item.SubCategory, c.want)
		}
	}
}

// 3. Nothing outside wepType == 90 leaks into Thrusting Shields.
func TestThrustingShields_NoForeignMembers(t *testing.T) {
	for id, item := range Shields {
		if item.SubCategory != SubcatShieldsThrusting {
			continue
		}
		if weaponWepType[id] != 90 {
			t.Errorf("%q (0x%08X) classified as Thrusting Shields but wepType = %d",
				item.Name, id, weaponWepType[id])
		}
	}
}
