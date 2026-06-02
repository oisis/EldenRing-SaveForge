package templates

import (
	"reflect"
	"strings"
	"testing"
)

// spellsImportTpl builds a v2 BuildTemplate carrying a SpellsSection plus
// matching selection. Mirrors the small helpers in schema_spells_test.go
// but stays self-contained to keep this file readable.
func spellsImportTpl(sec *SpellsSection, sel *SectionSelection) *BuildTemplate {
	if sel == nil {
		sel = &SectionSelection{All: true}
	}
	return &BuildTemplate{
		Schema:    SchemaKey,
		Version:   2,
		CreatedAt: "2026-06-02T00:00:00Z",
		Selection: &TemplateSelection{Spells: sel},
		Sections:  TemplateSections{Spells: sec},
	}
}

// TestPreview_V2_Spells_SummaryListsPresentSlots covers the happy path:
// a populated SpellsSection should surface in SpellSlotsPresent in
// canonical (SpellSlotOrder) order.
func TestPreview_V2_Spells_SummaryListsPresentSlots(t *testing.T) {
	tpl := spellsImportTpl(
		&SpellsSection{
			Spell14: &SpellSlotRef{BaseItemID: 0x40000FA0, Name: "Glintstone Pebble"},
			Spell1:  &SpellSlotRef{BaseItemID: 0x40001770, Name: "Catch Flame"},
			Spell5:  &SpellSlotRef{BaseItemID: 0, Name: "explicit clear"},
		},
		&SectionSelection{All: true},
	)

	rep := PreviewBuildTemplateImport(tpl, ImportPreviewOptions{})
	if !rep.OK {
		t.Fatalf("expected OK, got errors: %+v", rep.Errors)
	}

	want := []string{"spell1", "spell5", "spell14"}
	if !reflect.DeepEqual(rep.Summary.SpellSlotsPresent, want) {
		t.Errorf("SpellSlotsPresent = %v, want %v (canonical SpellSlotOrder)",
			rep.Summary.SpellSlotsPresent, want)
	}

	// Belt-and-suspenders: explicit-clear refs (BaseItemID == 0) must
	// still appear in SpellSlotsPresent — they are "present in the
	// template," and the UI's job is to render them as clears.
	found := false
	for _, k := range rep.Summary.SpellSlotsPresent {
		if k == "spell5" {
			found = true
		}
	}
	if !found {
		t.Error("explicit-clear slot (BaseItemID=0) must still appear in SpellSlotsPresent")
	}
}

// TestPreview_V2_Spells_SelectedSectionsIncludesSpells confirms that the
// 'spells' key shows up in the canonical SelectedSections list when
// Selection.Spells is non-empty.
func TestPreview_V2_Spells_SelectedSectionsIncludesSpells(t *testing.T) {
	t.Run("All shortcut", func(t *testing.T) {
		tpl := spellsImportTpl(
			&SpellsSection{Spell1: &SpellSlotRef{BaseItemID: 0x40001770}},
			&SectionSelection{All: true},
		)
		rep := PreviewBuildTemplateImport(tpl, ImportPreviewOptions{})
		if len(rep.Errors) != 0 {
			t.Fatalf("unexpected errors: %+v", rep.Errors)
		}
		if !containsString(rep.Summary.SelectedSections, "spells") {
			t.Errorf("SelectedSections = %v, want it to include 'spells'", rep.Summary.SelectedSections)
		}
	})
	t.Run("per-field selection", func(t *testing.T) {
		tpl := spellsImportTpl(
			&SpellsSection{Spell14: &SpellSlotRef{BaseItemID: 0x40000FA0}},
			&SectionSelection{Fields: map[string]bool{"spell14": true}},
		)
		rep := PreviewBuildTemplateImport(tpl, ImportPreviewOptions{})
		if len(rep.Errors) != 0 {
			t.Fatalf("unexpected errors: %+v", rep.Errors)
		}
		if !containsString(rep.Summary.SelectedSections, "spells") {
			t.Errorf("SelectedSections = %v, want it to include 'spells'", rep.Summary.SelectedSections)
		}
	})
}

// TestPreview_V2_NoSpells_BackwardCompat ensures templates that don't
// touch spells continue to produce nil SpellSlotsPresent and an
// 'spells'-free SelectedSections list. Existing v2 callers must see no
// behavioural change.
func TestPreview_V2_NoSpells_BackwardCompat(t *testing.T) {
	tpl := &BuildTemplate{
		Schema:    SchemaKey,
		Version:   2,
		CreatedAt: "2026-06-02T00:00:00Z",
		Selection: &TemplateSelection{
			Profile: &SectionSelection{All: true},
		},
		Sections: TemplateSections{
			Profile: &ProfileSection{Level: u32p(150)},
		},
	}
	rep := PreviewBuildTemplateImport(tpl, ImportPreviewOptions{})
	if len(rep.Errors) != 0 {
		t.Fatalf("unexpected errors: %+v", rep.Errors)
	}
	if rep.Summary.SpellSlotsPresent != nil {
		t.Errorf("SpellSlotsPresent should be nil when template has no spells, got %v",
			rep.Summary.SpellSlotsPresent)
	}
	if containsString(rep.Summary.SelectedSections, "spells") {
		t.Errorf("SelectedSections must not include 'spells' for a spells-free template; got %v",
			rep.Summary.SelectedSections)
	}
}

// TestPreview_V2_SpellsSelectedButSectionMissing_StillRejected re-asserts
// that the Phase 7d.1 validator's "selection.spells selected but
// sections.spells missing" rule still fires through the preview entry
// point — preview is not allowed to silently accept what
// ValidateBuildTemplate rejects.
func TestPreview_V2_SpellsSelectedButSectionMissing_StillRejected(t *testing.T) {
	tpl := &BuildTemplate{
		Schema:    SchemaKey,
		Version:   2,
		CreatedAt: "2026-06-02T00:00:00Z",
		Selection: &TemplateSelection{Spells: &SectionSelection{All: true}},
		Sections:  TemplateSections{}, // sections.spells deliberately missing
	}
	rep := PreviewBuildTemplateImport(tpl, ImportPreviewOptions{})
	if len(rep.Errors) == 0 {
		t.Fatal("expected validation error, got clean preview")
	}
	found := false
	for _, e := range rep.Errors {
		if strings.Contains(e.Message, "selection.spells is selected but sections.spells is missing") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected the Phase 7d.1 missing-section error; got: %+v", rep.Errors)
	}
}

// TestPreview_V2_Spells_InvalidPrefixStillRejected pins that an export
// with a bad spell prefix is rejected by preview just like by direct
// validation.
func TestPreview_V2_Spells_InvalidPrefixStillRejected(t *testing.T) {
	tpl := spellsImportTpl(
		&SpellsSection{Spell1: &SpellSlotRef{BaseItemID: 0x60001770}}, // 0x60 not allowed
		&SectionSelection{All: true},
	)
	rep := PreviewBuildTemplateImport(tpl, ImportPreviewOptions{})
	if len(rep.Errors) == 0 {
		t.Fatal("expected validation error for 0x60-prefixed spell, got clean preview")
	}
	found := false
	for _, e := range rep.Errors {
		if strings.Contains(e.Message, "wrong prefix") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected wrong-prefix error; got: %+v", rep.Errors)
	}
}

// TestPreview_V2_Spells_AllNilSlotsYieldsEmptyPresent covers the edge
// where Selection.Spells is non-empty but the section was emitted with
// every slot nil (semantically "no slots present").
func TestPreview_V2_Spells_AllNilSlotsYieldsEmptyPresent(t *testing.T) {
	tpl := spellsImportTpl(&SpellsSection{}, &SectionSelection{All: true})
	rep := PreviewBuildTemplateImport(tpl, ImportPreviewOptions{})
	if len(rep.Errors) != 0 {
		t.Fatalf("unexpected errors: %+v", rep.Errors)
	}
	if rep.Summary.SpellSlotsPresent != nil {
		t.Errorf("SpellSlotsPresent should be nil when no slot pointers are set, got %v",
			rep.Summary.SpellSlotsPresent)
	}
	// SelectedSections still mentions spells — the user selected it,
	// the section just happens to carry no populated slots.
	if !containsString(rep.Summary.SelectedSections, "spells") {
		t.Errorf("SelectedSections = %v, want it to include 'spells' (selection drives this, not slot count)",
			rep.Summary.SelectedSections)
	}
}

// containsString is a tiny local helper to avoid pulling in slices.Contains
// just for this file; the broader templates package targets the same Go
// version as the rest of the repo and other tests use plain loops too.
func containsString(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}
