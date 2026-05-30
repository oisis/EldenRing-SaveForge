package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
	"github.com/oisis/EldenRing-SaveForge/backend/templates"
)

// profileStatsFixture returns an App with slot 0 populated with stable
// profile/stats values used by Phase 3C.1 tests. The fixture uses
// Class = 0 (Vagabond) so vm.MapParsedSlotToVM resolves ClassName via
// db.GetClassStats rather than falling through to the "Unknown (N)"
// branch. Tests that need the Unknown path bump Player.Class directly
// afterwards.
//
// No filesystem touch, no real save load — this is in-memory only.
func profileStatsFixture() *App {
	app := NewApp()
	app.save = &core.SaveFile{}
	slot := &app.save.Slots[0]
	slot.Version = 1
	slot.Player.Class = 0
	slot.Player.Level = 50
	slot.Player.Souls = 1000
	slot.Player.SoulMemory = 50000
	slot.Player.Vigor = 20
	slot.Player.Mind = 15
	slot.Player.Endurance = 18
	slot.Player.Strength = 16
	slot.Player.Dexterity = 12
	slot.Player.Intelligence = 9
	slot.Player.Faith = 9
	slot.Player.Arcane = 7
	slot.Player.ClearCount = 1
	slot.Player.ScadutreeBlessing = 5
	slot.Player.ShadowRealmBlessing = 3
	slot.Player.TalismanSlots = 2

	name := utf16.Encode([]rune("Tester"))
	copy(slot.Player.CharacterName[:], name)
	return app
}

func TestExportBuildTemplateV2JSONFromCharacter_ProfileStatsAll(t *testing.T) {
	app := profileStatsFixture()
	sel := `{"profile":true,"stats":true}`
	out, err := app.ExportBuildTemplateV2JSONFromCharacter(0, sel, BuildTemplateV2ExportOptions{
		Name:        "Test V2",
		Description: "phase 3C.1 happy path",
	})
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	if out == "" {
		t.Fatal("output JSON must be non-empty")
	}
	var tpl templates.BuildTemplate
	if err := json.Unmarshal([]byte(out), &tpl); err != nil {
		t.Fatalf("output JSON does not decode: %v\npayload: %s", err, out)
	}
	if tpl.Version != 2 {
		t.Errorf("Version = %d, want 2", tpl.Version)
	}
	if tpl.Schema != templates.SchemaKey {
		t.Errorf("Schema = %q, want %q", tpl.Schema, templates.SchemaKey)
	}
	if tpl.AppVersion == "" {
		t.Error("AppVersion must be populated from main package")
	}
	if tpl.Metadata == nil {
		t.Fatal("Metadata missing")
	}
	if tpl.Metadata.SourceCharacterIndex != 0 {
		t.Errorf("SourceCharacterIndex = %d, want 0", tpl.Metadata.SourceCharacterIndex)
	}
	if tpl.Metadata.SourceCharacterName != "Tester" {
		t.Errorf("SourceCharacterName = %q, want %q", tpl.Metadata.SourceCharacterName, "Tester")
	}
	if tpl.Sections.Profile == nil {
		t.Fatal("profile section missing")
	}
	if tpl.Sections.Profile.Level == nil || *tpl.Sections.Profile.Level != 50 {
		t.Errorf("profile.level wrong: %+v", tpl.Sections.Profile.Level)
	}
	if tpl.Sections.Profile.Runes == nil || *tpl.Sections.Profile.Runes != 1000 {
		t.Errorf("profile.runes wrong (vm.Souls→Runes mapping): %+v", tpl.Sections.Profile.Runes)
	}
	if tpl.Sections.Stats == nil {
		t.Fatal("stats section missing")
	}
	if tpl.Sections.Stats.Vigor == nil || *tpl.Sections.Stats.Vigor != 20 {
		t.Errorf("stats.vigor wrong: %+v", tpl.Sections.Stats.Vigor)
	}
	if tpl.Sections.Stats.Mind == nil || *tpl.Sections.Stats.Mind != 15 {
		t.Errorf("stats.mind wrong: %+v", tpl.Sections.Stats.Mind)
	}
	if tpl.Selection == nil || tpl.Selection.Profile == nil || !tpl.Selection.Profile.All {
		t.Errorf("Selection.Profile.All not set: %+v", tpl.Selection)
	}
	if tpl.Selection.Stats == nil || !tpl.Selection.Stats.All {
		t.Errorf("Selection.Stats.All not set: %+v", tpl.Selection)
	}
}

func TestPreviewBuildTemplateV2FromCharacter_ProfileStatsAll(t *testing.T) {
	app := profileStatsFixture()
	sel := `{"profile":true,"stats":true}`
	res, err := app.PreviewBuildTemplateV2FromCharacter(0, sel, BuildTemplateV2ExportOptions{})
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}
	if !res.Report.OK {
		t.Errorf("Report.OK = false, want true: %+v", res.Report.Errors)
	}
	if res.Report.Summary.Version != 2 {
		t.Errorf("Summary.Version = %d, want 2", res.Report.Summary.Version)
	}
	want := []string{"profile", "stats"}
	got := res.Report.Summary.SelectedSections
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("SelectedSections = %v, want %v", got, want)
	}
	if len(res.Report.Summary.ProfileFieldsPresent) == 0 {
		t.Error("ProfileFieldsPresent empty")
	}
	if len(res.Report.Summary.StatFieldsPresent) == 0 {
		t.Error("StatFieldsPresent empty")
	}
	if res.JSON == "" {
		t.Fatal("JSON missing in LoadedTemplatePreview")
	}
	var tpl templates.BuildTemplate
	if err := json.Unmarshal([]byte(res.JSON), &tpl); err != nil {
		t.Fatalf("Preview.JSON does not parse: %v", err)
	}
	if tpl.Version != 2 {
		t.Errorf("Preview.JSON Version = %d, want 2", tpl.Version)
	}
}

func TestExportBuildTemplateV2JSONFromCharacter_SubsetProfile(t *testing.T) {
	app := profileStatsFixture()
	sel := `{"profile":{"level":true,"runes":true}}`
	out, err := app.ExportBuildTemplateV2JSONFromCharacter(0, sel, BuildTemplateV2ExportOptions{})
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	var tpl templates.BuildTemplate
	if err := json.Unmarshal([]byte(out), &tpl); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if tpl.Sections.Profile == nil {
		t.Fatal("profile section missing")
	}
	if tpl.Sections.Profile.Level == nil || *tpl.Sections.Profile.Level != 50 {
		t.Errorf("profile.level missing")
	}
	if tpl.Sections.Profile.Runes == nil || *tpl.Sections.Profile.Runes != 1000 {
		t.Errorf("profile.runes missing")
	}
	if tpl.Sections.Profile.Name != nil {
		t.Errorf("profile.name should be absent for subset selection, got %q", *tpl.Sections.Profile.Name)
	}
	if tpl.Sections.Stats != nil {
		t.Errorf("stats section should be absent, got %+v", tpl.Sections.Stats)
	}
}

func TestExportBuildTemplateV2JSONFromCharacter_MalformedSelectionRejected(t *testing.T) {
	app := profileStatsFixture()
	_, err := app.ExportBuildTemplateV2JSONFromCharacter(0, "not json", BuildTemplateV2ExportOptions{})
	if err == nil {
		t.Fatal("expected error for malformed selection JSON")
	}
	if !strings.Contains(err.Error(), "parse selection") {
		t.Errorf("error must mention parse selection, got %q", err.Error())
	}
}

func TestExportBuildTemplateV2JSONFromCharacter_UnknownTopLevelSelectionRejected(t *testing.T) {
	app := profileStatsFixture()
	_, err := app.ExportBuildTemplateV2JSONFromCharacter(0, `{"profiel":true}`, BuildTemplateV2ExportOptions{})
	if err == nil {
		t.Fatal("expected error for unknown top-level selection key")
	}
	if !strings.Contains(err.Error(), "parse selection") {
		t.Errorf("error must surface via parse selection, got %q", err.Error())
	}
}

func TestExportBuildTemplateV2JSONFromCharacter_InvalidCharIndexRejected(t *testing.T) {
	app := profileStatsFixture()
	for _, idx := range []int{-1, 10} {
		idx := idx
		t.Run(fmt.Sprintf("idx=%d", idx), func(t *testing.T) {
			_, err := app.ExportBuildTemplateV2JSONFromCharacter(idx, `{"profile":true}`, BuildTemplateV2ExportOptions{})
			if err == nil {
				t.Fatalf("expected error for invalid charIndex %d", idx)
			}
			if !strings.Contains(err.Error(), "invalid slot index") {
				t.Errorf("error must mention invalid slot index, got %q", err.Error())
			}
		})
	}
}

func TestExportBuildTemplateV2JSONFromCharacter_NoSaveLoadedRejected(t *testing.T) {
	app := NewApp()
	_, err := app.ExportBuildTemplateV2JSONFromCharacter(0, `{"profile":true}`, BuildTemplateV2ExportOptions{})
	if err == nil {
		t.Fatal("expected error when no save loaded")
	}
	if !strings.Contains(err.Error(), "no save loaded") {
		t.Errorf("error must mention no save loaded, got %q", err.Error())
	}
}

func TestExportBuildTemplateV2JSONFromCharacter_ClassUnknownDropped(t *testing.T) {
	app := profileStatsFixture()
	// Class 99 is outside the DB class table → vm.ClassName = "Unknown (99)".
	// Mapper must drop the field so the template does not ship a literal
	// "Unknown (99)" string.
	app.save.Slots[0].Player.Class = 99
	sel := `{"profile":{"class":true,"level":true}}`
	out, err := app.ExportBuildTemplateV2JSONFromCharacter(0, sel, BuildTemplateV2ExportOptions{})
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	var tpl templates.BuildTemplate
	if err := json.Unmarshal([]byte(out), &tpl); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if tpl.Sections.Profile == nil {
		t.Fatal("profile section missing")
	}
	if tpl.Sections.Profile.Class != nil {
		t.Errorf("profile.class must be dropped for Unknown class, got %q", *tpl.Sections.Profile.Class)
	}
	if tpl.Sections.Profile.Level == nil || *tpl.Sections.Profile.Level != 50 {
		t.Errorf("profile.level should remain")
	}
	// Builder normalisation also drops the unset field from per-field
	// selection so the v2 invariant "selection ⟺ data present" holds.
	if tpl.Selection == nil || tpl.Selection.Profile == nil {
		t.Fatal("selection.profile missing")
	}
	if tpl.Selection.Profile.Fields["class"] {
		t.Errorf("Selection.Profile.Fields[class] must be dropped: %v", tpl.Selection.Profile.Fields)
	}
	if !tpl.Selection.Profile.Fields["level"] {
		t.Errorf("Selection.Profile.Fields[level] must remain: %v", tpl.Selection.Profile.Fields)
	}
}

func TestExportBuildTemplateV2JSONFromCharacter_StatsOutOfRangeRejected(t *testing.T) {
	app := profileStatsFixture()
	app.save.Slots[0].Player.Vigor = 100 // > MaxStatValue = 99
	_, err := app.ExportBuildTemplateV2JSONFromCharacter(0, `{"stats":true}`, BuildTemplateV2ExportOptions{})
	if err == nil {
		t.Fatal("expected error for stats out of range")
	}
	if !strings.Contains(err.Error(), "build v2 template") {
		t.Errorf("error must wrap builder failure, got %q", err.Error())
	}
}

func TestExportBuildTemplateV2JSONFromCharacter_EmptySelectionRejected(t *testing.T) {
	app := profileStatsFixture()
	_, err := app.ExportBuildTemplateV2JSONFromCharacter(0, `{}`, BuildTemplateV2ExportOptions{})
	if err == nil {
		t.Fatal("expected error for empty selection")
	}
}

func TestPreviewBuildTemplateV2FromCharacter_JSONMatchesExport(t *testing.T) {
	// Anti-TOCTOU contract: Preview returns the same canonical JSON bytes
	// the Export endpoint would produce for the same inputs, so a future
	// "save to library" call (Phase 3C.2) can reuse them verbatim.
	app := profileStatsFixture()
	sel := `{"profile":true,"stats":true}`
	opts := BuildTemplateV2ExportOptions{Name: "anti-TOCTOU"}
	exported, err := app.ExportBuildTemplateV2JSONFromCharacter(0, sel, opts)
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	preview, err := app.PreviewBuildTemplateV2FromCharacter(0, sel, opts)
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}
	if preview.JSON != exported {
		t.Errorf("preview.JSON differs from export output\nexport:  %s\npreview: %s", exported, preview.JSON)
	}
}
