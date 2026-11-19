package db

import "testing"

func TestIsValidInfuseOffset(t *testing.T) {
	for _, infuse := range InfuseTypes {
		if !IsValidInfuseOffset(infuse.Offset) {
			t.Errorf("offset %d (%s) should be valid", infuse.Offset, infuse.Name)
		}
	}
	for _, offset := range []int{-100, -1, 1, 50, 150, 1300, 100000} {
		if IsValidInfuseOffset(offset) {
			t.Errorf("offset %d should be invalid", offset)
		}
	}
}

func TestWeaponAcceptsBatchAffinity(t *testing.T) {
	cases := []struct {
		name string
		id   uint32
		want bool
	}{
		{"Dagger (infusable)", 0x000F4240, true},
		{"Ghostflame Torch (regulation forbids)", 0x016EF950, false},
		{"Crystal Knife (regulation forbids)", 0x00100590, false},
		{"Longbow (regulation forbids)", 0x02719C40, false},
		{"Flame Art Swift Spear (pre-infused row)", 0x00FC1534, false},
		{"unknown ID", 0x00000001, false},
		{"goods ID", 0x4000012C, false},
	}
	for _, tc := range cases {
		if got := WeaponAcceptsBatchAffinity(tc.id); got != tc.want {
			t.Errorf("%s: WeaponAcceptsBatchAffinity(0x%08X) = %v, want %v", tc.name, tc.id, got, tc.want)
		}
	}
}
