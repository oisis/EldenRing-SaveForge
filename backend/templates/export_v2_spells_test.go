package templates

import (
	"encoding/json"
	"strings"
	"testing"
)

// emptySpellLoadout returns 14 raw IDs all set to the save's empty-slot
// sentinel — a vanilla "no spells equipped" save state.
func emptySpellLoadout() []uint32 {
	out := make([]uint32, SpellSlotCount)
	for i := range out {
		out[i] = 0xFFFFFFFF
	}
	return out
}

// TestBuildV2Template_Spells_BooleanShortcut_ConvertsRawToFullID verifies
// the All=true happy path: every slot lands in the output, raw MagicParam
// IDs are OR'd with 0x40000000, and the output Selection echoes All=true.
func TestBuildV2Template_Spells_BooleanShortcut_ConvertsRawToFullID(t *testing.T) {
	raw := emptySpellLoadout()
	raw[0] = 0x00001770  // Catch Flame
	raw[5] = 0x00000FA0  // Glintstone Pebble
	raw[13] = 0x00001234 // arbitrary

	tpl, err := BuildV2Template(ExportV2Options{
		Now:               fixedNow(),
		EquippedSpellsRaw: raw,
		Selection:         &TemplateSelection{Spells: &SectionSelection{All: true}},
	})
	if err != nil {
		t.Fatalf("BuildV2Template: %v", err)
	}

	if tpl.Sections.Spells == nil {
		t.Fatal("expected Sections.Spells to be populated")
	}
	if tpl.Selection == nil || tpl.Selection.Spells == nil || !tpl.Selection.Spells.All {
		t.Fatal("expected Selection.Spells.All=true to survive normalization")
	}
	if tpl.Selection.Spells.Fields != nil {
		t.Errorf("All=true shortcut should not carry Fields map, got %v", tpl.Selection.Spells.Fields)
	}

	if tpl.Sections.Spells.Spell1 == nil || tpl.Sections.Spells.Spell1.BaseItemID != 0x40001770 {
		t.Errorf("spell1 = %+v, want BaseItemID=0x40001770", tpl.Sections.Spells.Spell1)
	}
	if tpl.Sections.Spells.Spell6 == nil || tpl.Sections.Spells.Spell6.BaseItemID != 0x40000FA0 {
		t.Errorf("spell6 = %+v, want BaseItemID=0x40000FA0", tpl.Sections.Spells.Spell6)
	}
	if tpl.Sections.Spells.Spell14 == nil || tpl.Sections.Spells.Spell14.BaseItemID != 0x40001234 {
		t.Errorf("spell14 = %+v, want BaseItemID=0x40001234", tpl.Sections.Spells.Spell14)
	}

	// The output must validate cleanly under the Phase 7d.1 schema
	// rules. This guards the prefix conversion above against drift.
	if err := ValidateBuildTemplate(tpl); err != nil {
		t.Errorf("converted spells output failed schema validation: %v", err)
	}
}

// TestBuildV2Template_Spells_EmptySentinelToExplicitClear pins the rule
// that the save-format sentinel 0xFFFFFFFF must NEVER appear in the
// public template — it is translated to BaseItemID == 0 (explicit clear).
func TestBuildV2Template_Spells_EmptySentinelToExplicitClear(t *testing.T) {
	raw := emptySpellLoadout()
	raw[2] = 0x00001770 // one occupied slot mixed with empties

	tpl, err := BuildV2Template(ExportV2Options{
		Now:               fixedNow(),
		EquippedSpellsRaw: raw,
		Selection:         &TemplateSelection{Spells: &SectionSelection{All: true}},
	})
	if err != nil {
		t.Fatalf("BuildV2Template: %v", err)
	}

	// Pointer must be non-nil (slot was selected) but BaseItemID must
	// be the explicit-clear value 0 — NOT 0xFFFFFFFF.
	if tpl.Sections.Spells.Spell1 == nil {
		t.Fatal("spell1 should be present as explicit clear")
	}
	if tpl.Sections.Spells.Spell1.BaseItemID != 0 {
		t.Errorf("spell1.BaseItemID = 0x%08X, want 0 (explicit clear)", tpl.Sections.Spells.Spell1.BaseItemID)
	}
	if tpl.Sections.Spells.Spell14.BaseItemID != 0 {
		t.Errorf("spell14.BaseItemID = 0x%08X, want 0 (explicit clear)", tpl.Sections.Spells.Spell14.BaseItemID)
	}

	// Belt-and-suspenders: serialise the whole template and assert the
	// raw save sentinel string never appears in the JSON output.
	data, err := json.Marshal(tpl)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	for _, forbidden := range []string{"4294967295", "0xFFFFFFFF", "0xffffffff"} {
		if strings.Contains(string(data), forbidden) {
			t.Errorf("template JSON leaked save sentinel %q: %s", forbidden, string(data))
		}
	}
}

// TestBuildV2Template_Spells_PerFieldSelection asserts only the requested
// slot keys land in the output, and the output Selection.Fields mirrors
// what was emitted (and nothing more).
func TestBuildV2Template_Spells_PerFieldSelection(t *testing.T) {
	raw := emptySpellLoadout()
	raw[0] = 0x00001770   // spell1: Catch Flame
	raw[6] = 0x00002328   // spell7
	raw[13] = 0x000011A1 // spell14

	tpl, err := BuildV2Template(ExportV2Options{
		Now:               fixedNow(),
		EquippedSpellsRaw: raw,
		Selection: &TemplateSelection{Spells: &SectionSelection{
			Fields: map[string]bool{"spell1": true, "spell14": true},
		}},
	})
	if err != nil {
		t.Fatalf("BuildV2Template: %v", err)
	}
	if tpl.Sections.Spells == nil {
		t.Fatal("expected Sections.Spells to be populated")
	}
	if tpl.Sections.Spells.Spell1 == nil || tpl.Sections.Spells.Spell1.BaseItemID != 0x40001770 {
		t.Errorf("spell1 = %+v, want BaseItemID=0x40001770", tpl.Sections.Spells.Spell1)
	}
	// spell7 was NOT selected → must remain nil even though source had a value.
	if tpl.Sections.Spells.Spell7 != nil {
		t.Errorf("spell7 was not selected, must be nil; got %+v", tpl.Sections.Spells.Spell7)
	}
	if tpl.Sections.Spells.Spell14 == nil || tpl.Sections.Spells.Spell14.BaseItemID != 0x400011A1 {
		t.Errorf("spell14 = %+v, want BaseItemID=0x400011A1", tpl.Sections.Spells.Spell14)
	}

	if tpl.Selection.Spells == nil {
		t.Fatal("expected Selection.Spells")
	}
	if tpl.Selection.Spells.All {
		t.Error("per-field selection should not be normalized to All=true")
	}
	if !tpl.Selection.Spells.Fields["spell1"] || !tpl.Selection.Spells.Fields["spell14"] {
		t.Errorf("Selection.Spells.Fields = %+v, want {spell1,spell14}", tpl.Selection.Spells.Fields)
	}
	if tpl.Selection.Spells.Fields["spell7"] {
		t.Error("Selection.Spells.Fields must not echo unselected slot spell7")
	}
}

// TestBuildV2Template_Spells_SelectedButNoSource rejects exports where
// the caller asked for spells but forgot to wire up the source.
func TestBuildV2Template_Spells_SelectedButNoSource(t *testing.T) {
	_, err := BuildV2Template(ExportV2Options{
		Now:       fixedNow(),
		Selection: &TemplateSelection{Spells: &SectionSelection{All: true}},
	})
	if err == nil {
		t.Fatal("expected error when spells is selected but EquippedSpellsRaw is nil")
	}
	if !strings.Contains(err.Error(), "selection.spells") || !strings.Contains(err.Error(), "EquippedSpellsRaw") {
		t.Errorf("error %q should mention both selection.spells and EquippedSpellsRaw", err.Error())
	}
}

// TestBuildV2Template_Spells_RawLengthMismatch refuses to silently
// truncate or pad. A wrong-length slice is always a producer bug because
// slot indices would mis-align with spell IDs.
func TestBuildV2Template_Spells_RawLengthMismatch(t *testing.T) {
	for _, n := range []int{0, 1, 13, 15, 28} {
		raw := make([]uint32, n)
		for i := range raw {
			raw[i] = 0xFFFFFFFF
		}
		_, err := BuildV2Template(ExportV2Options{
			Now:               fixedNow(),
			EquippedSpellsRaw: raw,
			Selection:         &TemplateSelection{Spells: &SectionSelection{All: true}},
		})
		if err == nil {
			t.Errorf("len=%d: expected length-mismatch error, got nil", n)
			continue
		}
		if !strings.Contains(err.Error(), "EquippedSpellsRaw length") {
			t.Errorf("len=%d: error %q does not mention EquippedSpellsRaw length", n, err.Error())
		}
	}
}

// TestBuildV2Template_Spells_NoSourceWhenNotSelected guards the
// no-regressions story: producers that DON'T select spells continue to
// work fine without supplying EquippedSpellsRaw.
func TestBuildV2Template_Spells_NoSourceWhenNotSelected(t *testing.T) {
	tpl, err := BuildV2Template(ExportV2Options{
		Now:       fixedNow(),
		Profile:   &ProfileSource{Name: "Hero", Level: u32p(50)},
		Selection: &TemplateSelection{Profile: &SectionSelection{All: true}},
	})
	if err != nil {
		t.Fatalf("BuildV2Template: %v", err)
	}
	if tpl.Sections.Spells != nil {
		t.Errorf("Sections.Spells should be nil when spells not selected, got %+v", tpl.Sections.Spells)
	}
	if tpl.Selection.Spells != nil {
		t.Errorf("Selection.Spells should be nil when spells not selected, got %+v", tpl.Selection.Spells)
	}
}

// TestBuildV2Template_Spells_AllShortcutFullyEmptyLoadout asserts that
// "user asked for every spell slot, none are equipped in the save" round
// trips as 14 explicit-clear slots. This is the documented vanilla state
// and must NOT collapse to an empty section.
func TestBuildV2Template_Spells_AllShortcutFullyEmptyLoadout(t *testing.T) {
	tpl, err := BuildV2Template(ExportV2Options{
		Now:               fixedNow(),
		EquippedSpellsRaw: emptySpellLoadout(),
		Selection:         &TemplateSelection{Spells: &SectionSelection{All: true}},
	})
	if err != nil {
		t.Fatalf("BuildV2Template: %v", err)
	}
	if tpl.Sections.Spells == nil {
		t.Fatal("All=true must always emit Sections.Spells, even when every slot is empty")
	}
	for _, key := range SpellSlotOrder {
		ref := spellSlotRef(tpl.Sections.Spells, key)
		if ref == nil {
			t.Errorf("%s: expected non-nil explicit-clear ref, got nil", key)
			continue
		}
		if ref.BaseItemID != 0 {
			t.Errorf("%s.BaseItemID = 0x%08X, want 0 (explicit clear)", key, ref.BaseItemID)
		}
	}
}

// TestBuildV2Template_Spells_OutputValidatesSchemaInvariants gives the
// validator one more shot: feed it a per-field export with mixed
// occupied/empty slots and confirm the round-trips clean.
func TestBuildV2Template_Spells_OutputValidatesSchemaInvariants(t *testing.T) {
	raw := emptySpellLoadout()
	raw[3] = 0x00001770
	raw[10] = 0x00000FA0

	tpl, err := BuildV2Template(ExportV2Options{
		Now:               fixedNow(),
		EquippedSpellsRaw: raw,
		Selection: &TemplateSelection{Spells: &SectionSelection{
			Fields: map[string]bool{"spell4": true, "spell11": true, "spell14": true},
		}},
	})
	if err != nil {
		t.Fatalf("BuildV2Template: %v", err)
	}
	if err := ValidateBuildTemplate(tpl); err != nil {
		t.Errorf("output failed schema validation: %v", err)
	}
}

// TestBuildV2Template_Spells_CoexistsWithProfile guards against
// cross-section interference — running spells + profile together must
// leave both intact.
func TestBuildV2Template_Spells_CoexistsWithProfile(t *testing.T) {
	raw := emptySpellLoadout()
	raw[0] = 0x00001770

	tpl, err := BuildV2Template(ExportV2Options{
		Now:               fixedNow(),
		Profile:           &ProfileSource{Name: "Hero", Level: u32p(50)},
		EquippedSpellsRaw: raw,
		Selection: &TemplateSelection{
			Profile: &SectionSelection{All: true},
			Spells:  &SectionSelection{All: true},
		},
	})
	if err != nil {
		t.Fatalf("BuildV2Template: %v", err)
	}
	if tpl.Sections.Profile == nil || tpl.Sections.Profile.Name == nil || *tpl.Sections.Profile.Name != "Hero" {
		t.Errorf("profile lost: %+v", tpl.Sections.Profile)
	}
	if tpl.Sections.Spells == nil || tpl.Sections.Spells.Spell1 == nil || tpl.Sections.Spells.Spell1.BaseItemID != 0x40001770 {
		t.Errorf("spells lost: %+v", tpl.Sections.Spells)
	}
}
