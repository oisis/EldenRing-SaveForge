package data

import "testing"

// phase2b3Batch1Additions enumerates the 13 Information entries added in
// Phase 2B.3 batch 1. The list comes from the reclassification audit at
// tmp/item-audit/phase2b3_info_reclassification.csv, restricted to entries
// with goodsType=12, valid iconId, finite sortId and no Set A collision.
//
// Excluded by design:
//   - 15 Set B Notes (0x4000222E–0x4000223F) — duplicates of Set A with
//     unfinished regulation params (goodsType=0, iconId=0, sortId=999999).
//     The omission is documented in info.go and verified by
//     TestPhase2B3SetBDuplicatesAbsent below.
//
// Follow-up commit (Sealed Spiritsprings canonical replacement):
//   - 0x401EA3DF Note: Sealed Spiritsprings (shipped canonical SOTE variant)
//     replaces the broken Set-B-equivalent 0x401EA443. Verified by
//     TestPhase2B3SealedSpiritspringsCanonicalReplacement below.
var phase2b3Batch1Additions = []struct {
	ID       uint32
	Name     string
	IsDLC    bool
	IconPath string
}{
	// 3 unique base info items (low-range IDs / standalone)
	{0x40002020, "Note: The Preceptor's Secret", false, "items/tools/note_the_preceptors_secret.png"},
	{0x40002021, "Weathered Map", false, "items/tools/weathered_map.png"},
	{0x40002312, "Sellia's Secret", false, "items/tools/sellias_secret.png"},

	// 9 About * (base) — gap fills in the 0x4000238C–0x400023C1 sequence
	{0x4000238D, "About Sorceries and Incantations", false, "items/info/about_sorceries_and_incantations.png"},
	{0x4000239B, "About Flask of Wondrous Physick", false, "items/info/about_flask_of_wondrous_physick.png"},
	{0x400023A0, "About Teardrop Scarabs", false, "items/info/about_teardrop_scarabs.png"},
	{0x400023B5, "About Great Runes", false, "items/info/about_great_runes.png"},
	{0x400023B6, "About the Cave of Knowledge", false, "items/info/about_the_cave_of_knowledge.png"},
	{0x400023BE, "About Duels", false, "items/info/about_duels.png"},
	{0x400023BF, "About United Combat and Combat Ordeals", false, "items/info/about_united_combat_and_combat_ordeals.png"},
	{0x400023C0, "About Combat with Spirit Ashes", false, "items/info/about_combat_with_spirit_ashes.png"},
	{0x400023C1, "About Marika's Effigy at the Roundtable", false, "items/info/about_marikas_effigy_at_the_roundtable.png"},

	// 1 SOTE info entry
	{0x401EA849, "About the Revered Spirit Ash Blessing", true, "items/info/about_the_revered_spirit_ash_blessing.png"},
}

func TestPhase2B3Batch1Present(t *testing.T) {
	if got, want := len(phase2b3Batch1Additions), 13; got != want {
		t.Fatalf("phase2b3Batch1Additions has %d entries, want exactly %d", got, want)
	}
	for _, w := range phase2b3Batch1Additions {
		item, ok := Information[w.ID]
		if !ok {
			t.Errorf("Information[0x%08X] (%s): missing entry", w.ID, w.Name)
			continue
		}
		if item.Name != w.Name {
			t.Errorf("Information[0x%08X] name = %q, want %q", w.ID, item.Name, w.Name)
		}
		if item.Category != "info" {
			t.Errorf("Information[0x%08X] (%s) category = %q, want %q",
				w.ID, w.Name, item.Category, "info")
		}
		if item.MaxInventory != 1 {
			t.Errorf("Information[0x%08X] (%s) MaxInventory = %d, want 1",
				w.ID, w.Name, item.MaxInventory)
		}
		if item.MaxStorage != 0 {
			t.Errorf("Information[0x%08X] (%s) MaxStorage = %d, want 0",
				w.ID, w.Name, item.MaxStorage)
		}
		if item.MaxUpgrade != 0 {
			t.Errorf("Information[0x%08X] (%s) MaxUpgrade = %d, want 0",
				w.ID, w.Name, item.MaxUpgrade)
		}
		if item.IconPath != w.IconPath {
			t.Errorf("Information[0x%08X] (%s) IconPath = %q, want %q",
				w.ID, w.Name, item.IconPath, w.IconPath)
		}
		hasDLC := false
		for _, f := range item.Flags {
			if f == "dlc" {
				hasDLC = true
				break
			}
		}
		if hasDLC != w.IsDLC {
			t.Errorf("Information[0x%08X] (%s) dlc flag = %v, want %v",
				w.ID, w.Name, hasDLC, w.IsDLC)
		}
	}
}

// TestPhase2B3Batch1SubCategoryAssignment confirms the auto-classifier in
// info_subcat.go places the 13 batch-1 entries in expected sub-categories:
//   - "About *"     → Mechanics / Locations Info
//   - "Note: *"     → Mechanics / Locations Info
//   - DLC "About *" → Mechanics / Locations Info (Mechanics takes precedence
//     over the DLC sub-bucket because the classifier checks name prefix first)
//   - Other        → Letters / Maps / Paintings (base) when no dlc flag
func TestPhase2B3Batch1SubCategoryAssignment(t *testing.T) {
	cases := []struct {
		id     uint32
		expect string
	}{
		// About * → Mechanics
		{0x4000238D, SubcatInfoMechanicsLocations},
		{0x4000239B, SubcatInfoMechanicsLocations},
		{0x400023A0, SubcatInfoMechanicsLocations},
		{0x400023B5, SubcatInfoMechanicsLocations},
		{0x400023B6, SubcatInfoMechanicsLocations},
		{0x400023BE, SubcatInfoMechanicsLocations},
		{0x400023BF, SubcatInfoMechanicsLocations},
		{0x400023C0, SubcatInfoMechanicsLocations},
		{0x400023C1, SubcatInfoMechanicsLocations},
		{0x401EA849, SubcatInfoMechanicsLocations},
		// Note: * → Mechanics
		{0x40002020, SubcatInfoMechanicsLocations},
		// Other base (no Note:/About prefix, no dlc) → Letters/Maps
		{0x40002021, SubcatInfoLettersMaps},
		{0x40002312, SubcatInfoLettersMaps},
	}
	for _, c := range cases {
		item := Information[c.id]
		if item.SubCategory != c.expect {
			t.Errorf("Information[0x%08X] (%s) SubCategory = %q, want %q",
				c.id, item.Name, item.SubCategory, c.expect)
		}
	}
}

// TestPhase2B3SetBDuplicatesAbsent verifies the 15 Set B Notes
// (0x4000222E–0x4000223F, with three gaps the regulation already skips)
// stay out of Information. These duplicate Set A names and have unfinished
// regulation params; info.go documents the omission at the comment block
// immediately above the Notes (base) section.
func TestPhase2B3SetBDuplicatesAbsent(t *testing.T) {
	setB := []uint32{
		0x4000222E, 0x4000222F, 0x40002230, 0x40002231, 0x40002232,
		0x40002233, 0x40002235, 0x40002236, 0x40002237, 0x40002238,
		0x40002239, 0x4000223A, 0x4000223B, 0x4000223D, 0x4000223F,
	}
	for _, id := range setB {
		if _, ok := Information[id]; ok {
			t.Errorf("Information[0x%08X] present — Set B duplicate must remain absent", id)
		}
	}
}

// TestPhase2B3SealedSpiritspringsCanonicalReplacement verifies the canonical
// shipped Note: Sealed Spiritsprings (0x401EA3DF) is present and the broken
// Set-B-equivalent (0x401EA443) is no longer exposed. The cut variant had
// goodsType=0 / iconId=0 / sortId=999999 in EquipParamGoods and rendered as
// an "ICON" placeholder under Tools tab; the shipped variant has goodsType=12,
// iconId=3861, sortId=453100 and slots naturally next to 0x401EA3D9.
func TestPhase2B3SealedSpiritspringsCanonicalReplacement(t *testing.T) {
	canonical, ok := Information[0x401EA3DF]
	if !ok {
		t.Fatalf("Information[0x401EA3DF] missing — shipped canonical Sealed " +
			"Spiritsprings note must be present")
	}
	if canonical.Name != "Note: Sealed Spiritsprings" {
		t.Errorf("Information[0x401EA3DF] name = %q, want %q",
			canonical.Name, "Note: Sealed Spiritsprings")
	}
	if canonical.Category != "info" {
		t.Errorf("Information[0x401EA3DF] category = %q, want %q",
			canonical.Category, "info")
	}
	if canonical.MaxInventory != 1 {
		t.Errorf("Information[0x401EA3DF] MaxInventory = %d, want 1",
			canonical.MaxInventory)
	}
	if canonical.MaxStorage != 0 {
		t.Errorf("Information[0x401EA3DF] MaxStorage = %d, want 0",
			canonical.MaxStorage)
	}
	if canonical.MaxUpgrade != 0 {
		t.Errorf("Information[0x401EA3DF] MaxUpgrade = %d, want 0",
			canonical.MaxUpgrade)
	}
	if canonical.IconPath != "items/tools/note_sealed_spiritsprings.png" {
		t.Errorf("Information[0x401EA3DF] IconPath = %q, want %q",
			canonical.IconPath, "items/tools/note_sealed_spiritsprings.png")
	}
	hasDLC := false
	for _, f := range canonical.Flags {
		if f == "dlc" {
			hasDLC = true
		}
		if f == "cut_content" {
			t.Errorf("Information[0x401EA3DF] must NOT carry cut_content flag — " +
				"this is the shipped canonical entry")
		}
		if f == "ban_risk" {
			t.Errorf("Information[0x401EA3DF] must NOT carry ban_risk flag — " +
				"this is the shipped canonical entry")
		}
	}
	if !hasDLC {
		t.Errorf("Information[0x401EA3DF] missing dlc flag, got %v", canonical.Flags)
	}
	if canonical.SubCategory != SubcatInfoMechanicsLocations {
		t.Errorf("Information[0x401EA3DF] SubCategory = %q, want %q (Note: prefix → Mechanics)",
			canonical.SubCategory, SubcatInfoMechanicsLocations)
	}

	if _, ok := Information[0x401EA443]; ok {
		t.Errorf("Information[0x401EA443] present — broken Set-B Sealed " +
			"Spiritsprings duplicate must remain absent after canonical replacement")
	}
}
