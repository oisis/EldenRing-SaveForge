package db

import "testing"

// TestFlaskFamilyBaseID covers the ownership/counting identity for the two
// upgradeable Tears flasks: every level, its even technical alias, and its
// goods handle must collapse to the +0 family base the picker exposes.
func TestFlaskFamilyBaseID(t *testing.T) {
	cases := []struct {
		id       uint32
		wantBase uint32
		wantOK   bool
		desc     string
	}{
		{0x400003E9, 0x400003E9, true, "Crimson +0"},
		{0x400003F3, 0x400003E9, true, "Crimson +5"},
		{0x40000401, 0x400003E9, true, "Crimson +12"},
		{0x4000041B, 0x4000041B, true, "Cerulean +0"},
		{0x40000433, 0x4000041B, true, "Cerulean +12"},

		// Even technical "empty flask" alias rows resolve through TechnicalItemAliases.
		{0x40000400, 0x400003E9, true, "Crimson +12 even alias"},
		{0x40000432, 0x4000041B, true, "Cerulean +12 even alias"},

		// Goods handle forms (0xB0…) convert to the item ID first.
		{0xB00003E9, 0x400003E9, true, "Crimson +0 handle"},
		{0xB0000401, 0x400003E9, true, "Crimson +12 handle"},
		{0xB0000433, 0x4000041B, true, "Cerulean +12 handle"},

		// Non-flask items and Wondrous Physick are not a Tears family.
		{0x40000334, 0, false, "Boiled Crab"},
		{ItemFlaskWondrousPhysickEmpty, 0, false, "Wondrous Physick empty"},
		{ItemFlaskWondrousPhysickFilled, 0, false, "Wondrous Physick filled"},
		{0x40000435, 0, false, "just past Cerulean +12"},
	}
	for _, c := range cases {
		gotBase, gotOK := FlaskFamilyBaseID(c.id)
		if gotBase != c.wantBase || gotOK != c.wantOK {
			t.Errorf("%s: FlaskFamilyBaseID(0x%08X) = (0x%08X, %v), want (0x%08X, %v)",
				c.desc, c.id, gotBase, gotOK, c.wantBase, c.wantOK)
		}
	}
}
