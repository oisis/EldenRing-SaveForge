package data

import "testing"

// Crystal Tears (Issue #6): every Flask of Wondrous Physick tear must land in
// the "Crystal Tears" sub-group. Canonical membership = EquipParamGoods.goodsType
// == 10, verified against tmp/regulation-bin-dump/csv/EquipParamGoods.csv. The
// explicit ID+name list below is the reviewed set (40 tears; base + DLC).

var expectedCrystalTears = []struct {
	id   uint32
	name string
}{
	{0x40002AF8, "Crimsonspill Crystal Tear"},
	{0x40002AF9, "Greenspill Crystal Tear"},
	{0x40002AFA, "Crimson Crystal Tear (Variant)"},
	{0x40002AFB, "Crimson Crystal Tear"},
	{0x40002AFC, "Cerulean Crystal Tear (Variant)"},
	{0x40002AFD, "Cerulean Crystal Tear"},
	{0x40002AFE, "Speckled Hardtear"},
	{0x40002AFF, "Crimson Bubbletear"},
	{0x40002B00, "Opaline Bubbletear"},
	{0x40002B01, "Crimsonburst Crystal Tear"},
	{0x40002B02, "Greenburst Crystal Tear"},
	{0x40002B03, "Opaline Hardtear"},
	{0x40002B04, "Winged Crystal Tear"},
	{0x40002B05, "Thorny Cracked Tear"},
	{0x40002B06, "Spiked Cracked Tear"},
	{0x40002B07, "Windy Crystal Tear"},
	{0x40002B08, "Ruptured Crystal Tear (Variant)"},
	{0x40002B09, "Ruptured Crystal Tear"},
	{0x40002B0A, "Leaden Hardtear"},
	{0x40002B0B, "Twiggy Cracked Tear"},
	{0x40002B0C, "Crimsonwhorl Bubbletear"},
	{0x40002B0D, "Strength-knot Crystal Tear"},
	{0x40002B0E, "Dexterity-knot Crystal Tear"},
	{0x40002B0F, "Intelligence-knot Crystal Tear"},
	{0x40002B10, "Faith-knot Crystal Tear"},
	{0x40002B11, "Cerulean Hidden Tear"},
	{0x40002B12, "Stonebarb Cracked Tear"},
	{0x40002B13, "Purifying Crystal Tear"},
	{0x40002B14, "Flame-Shrouding Cracked Tear"},
	{0x40002B15, "Magic-Shrouding Cracked Tear"},
	{0x40002B16, "Lightning-Shrouding Cracked Tear"},
	{0x40002B17, "Holy-Shrouding Cracked Tear"},
	{0x401EAF78, "Viridian Hidden Tear"},
	{0x401EAF82, "Crimsonburst Dried Tear"},
	{0x401EAF8C, "Crimson-Sapping Cracked Tear"},
	{0x401EAF96, "Cerulean-Sapping Cracked Tear"},
	{0x401EAFA0, "Oil-Soaked Tear"},
	{0x401EAFAA, "Bloodsucking Cracked Tear"},
	{0x401EAFB4, "Glovewort Crystal Tear"},
	{0x401EAFBE, "Deflecting Hardtear"},
}

//  1. Every expected tear is present, correctly named, and in Crystal Tears
//     (not left in the "Inactive Great Runes + Keys + Medallions" catch-all).
func TestCrystalTears_AllPhysickTearsClassified(t *testing.T) {
	for _, want := range expectedCrystalTears {
		item, ok := KeyItems[want.id]
		if !ok {
			t.Errorf("crystal tear 0x%08X (%s) missing from KeyItems", want.id, want.name)
			continue
		}
		if item.Name != want.name {
			t.Errorf("0x%08X: name = %q, want %q", want.id, item.Name, want.name)
		}
		if item.SubCategory != SubcatKeyCrystalTears {
			t.Errorf("%q (0x%08X): SubCategory = %q, want %q",
				item.Name, want.id, item.SubCategory, SubcatKeyCrystalTears)
		}
		if item.SubCategory == SubcatKeyInactiveRunesKeys {
			t.Errorf("%q (0x%08X): still in catch-all %q",
				item.Name, want.id, SubcatKeyInactiveRunesKeys)
		}
	}
	if len(crystalTearIDs) != len(expectedCrystalTears) {
		t.Errorf("crystalTearIDs has %d entries, want %d", len(crystalTearIDs), len(expectedCrystalTears))
	}
}

// 2. Larval Tear, Asimi and Silver Tear must NOT be Crystal Tears.
func TestCrystalTears_ExcludesLarvalAndAsimi(t *testing.T) {
	for _, id := range []uint32{
		0x40001FF9, // Larval Tear
		0x40001FD3, // Asimi, Silver Tear
	} {
		item, ok := KeyItems[id]
		if !ok {
			continue // absent from DB is also fine
		}
		if item.SubCategory == SubcatKeyCrystalTears {
			t.Errorf("%q (0x%08X) wrongly classified as Crystal Tears", item.Name, id)
		}
		if _, ok := crystalTearIDs[id]; ok {
			t.Errorf("%q (0x%08X) must not be in crystalTearIDs", item.Name, id)
		}
	}
}

// 3. No stray member: everything in crystalTearIDs resolves to Crystal Tears.
func TestCrystalTears_NoForeignMembers(t *testing.T) {
	for id, item := range KeyItems {
		if item.SubCategory != SubcatKeyCrystalTears {
			continue
		}
		if _, ok := crystalTearIDs[id]; !ok {
			t.Errorf("%q (0x%08X) classified as Crystal Tears but not in crystalTearIDs", item.Name, id)
		}
	}
}
