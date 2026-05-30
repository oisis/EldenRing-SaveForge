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

// ─── Phase 3C.2 — YAML string export + Save to Library ──────────────────

// libraryV2Fixture builds on profileStatsFixture and injects a fresh
// per-test library directory through Go's t.TempDir so the system config
// dir is never touched. Mirrors the pattern used in
// app_templates_library_test.go.
func libraryV2Fixture(t *testing.T) *App {
	t.Helper()
	app := profileStatsFixture()
	app.templateLibrary = templates.NewTemplateLibrary(t.TempDir())
	return app
}

func TestExportBuildTemplateV2YAMLFromCharacter_ProfileStatsAll(t *testing.T) {
	app := profileStatsFixture()
	sel := `{"profile":true,"stats":true}`
	yamlText, err := app.ExportBuildTemplateV2YAMLFromCharacter(0, sel, BuildTemplateV2ExportOptions{
		Name: "YAML v2",
	})
	if err != nil {
		t.Fatalf("YAML Export: %v", err)
	}
	if yamlText == "" {
		t.Fatal("YAML output must be non-empty")
	}
	for _, want := range []string{"schema:", "version: 2", "selection:", "sections:"} {
		if !strings.Contains(yamlText, want) {
			t.Errorf("YAML missing %q\n%s", want, yamlText)
		}
	}
	tpl, err := templates.ParseBuildTemplateYAML([]byte(yamlText))
	if err != nil {
		t.Fatalf("ParseBuildTemplateYAML: %v\n%s", err, yamlText)
	}
	if tpl.Version != 2 {
		t.Errorf("Version = %d, want 2", tpl.Version)
	}
	if tpl.Sections.Profile == nil || tpl.Sections.Profile.Level == nil || *tpl.Sections.Profile.Level != 50 {
		t.Errorf("Sections.Profile.Level lost across YAML round-trip: %+v", tpl.Sections.Profile)
	}
	if tpl.Sections.Stats == nil || tpl.Sections.Stats.Vigor == nil || *tpl.Sections.Stats.Vigor != 20 {
		t.Errorf("Sections.Stats.Vigor lost across YAML round-trip: %+v", tpl.Sections.Stats)
	}
}

func TestExportBuildTemplateV2YAMLFromCharacter_YAMLAndJSONSemanticallyEquivalent(t *testing.T) {
	app := profileStatsFixture()
	sel := `{"profile":true,"stats":true}`
	opts := BuildTemplateV2ExportOptions{Name: "parity"}

	jsonText, err := app.ExportBuildTemplateV2JSONFromCharacter(0, sel, opts)
	if err != nil {
		t.Fatalf("JSON Export: %v", err)
	}
	yamlText, err := app.ExportBuildTemplateV2YAMLFromCharacter(0, sel, opts)
	if err != nil {
		t.Fatalf("YAML Export: %v", err)
	}

	tplFromJSON, err := templates.ParseBuildTemplateJSON([]byte(jsonText))
	if err != nil {
		t.Fatalf("ParseBuildTemplateJSON: %v", err)
	}
	tplFromYAML, err := templates.ParseBuildTemplateYAML([]byte(yamlText))
	if err != nil {
		t.Fatalf("ParseBuildTemplateYAML: %v", err)
	}

	if tplFromJSON.Version != tplFromYAML.Version {
		t.Errorf("Version mismatch: json=%d yaml=%d", tplFromJSON.Version, tplFromYAML.Version)
	}
	if tplFromJSON.Metadata == nil || tplFromYAML.Metadata == nil {
		t.Fatal("Metadata missing in one of the encodings")
	}
	if tplFromJSON.Metadata.SourceCharacterName != tplFromYAML.Metadata.SourceCharacterName {
		t.Errorf("SourceCharacterName mismatch: json=%q yaml=%q",
			tplFromJSON.Metadata.SourceCharacterName, tplFromYAML.Metadata.SourceCharacterName)
	}
	if tplFromJSON.Selection.Profile.All != tplFromYAML.Selection.Profile.All {
		t.Errorf("Selection.Profile.All mismatch: json=%v yaml=%v",
			tplFromJSON.Selection.Profile.All, tplFromYAML.Selection.Profile.All)
	}
	if tplFromJSON.Selection.Stats.All != tplFromYAML.Selection.Stats.All {
		t.Errorf("Selection.Stats.All mismatch: json=%v yaml=%v",
			tplFromJSON.Selection.Stats.All, tplFromYAML.Selection.Stats.All)
	}
	if *tplFromJSON.Sections.Profile.Level != *tplFromYAML.Sections.Profile.Level {
		t.Errorf("Sections.Profile.Level mismatch")
	}
	if *tplFromJSON.Sections.Profile.Runes != *tplFromYAML.Sections.Profile.Runes {
		t.Errorf("Sections.Profile.Runes mismatch")
	}
	if *tplFromJSON.Sections.Stats.Vigor != *tplFromYAML.Sections.Stats.Vigor {
		t.Errorf("Sections.Stats.Vigor mismatch")
	}
	if *tplFromJSON.Sections.Stats.Mind != *tplFromYAML.Sections.Stats.Mind {
		t.Errorf("Sections.Stats.Mind mismatch")
	}
}

func TestExportBuildTemplateV2YAMLFromCharacter_MalformedSelectionRejected(t *testing.T) {
	app := profileStatsFixture()
	_, err := app.ExportBuildTemplateV2YAMLFromCharacter(0, "not json", BuildTemplateV2ExportOptions{})
	if err == nil {
		t.Fatal("expected error for malformed selection JSON")
	}
	if !strings.Contains(err.Error(), "parse selection") {
		t.Errorf("error must mention parse selection, got %q", err.Error())
	}
}

func TestExportBuildTemplateV2YAMLFromCharacter_NoSaveLoadedRejected(t *testing.T) {
	app := NewApp()
	_, err := app.ExportBuildTemplateV2YAMLFromCharacter(0, `{"profile":true}`, BuildTemplateV2ExportOptions{})
	if err == nil {
		t.Fatal("expected error when no save loaded")
	}
	if !strings.Contains(err.Error(), "no save loaded") {
		t.Errorf("error must mention no save loaded, got %q", err.Error())
	}
}

func TestExportBuildTemplateV2YAMLFromCharacter_StatsOutOfRangeRejected(t *testing.T) {
	app := profileStatsFixture()
	app.save.Slots[0].Player.Vigor = 100
	_, err := app.ExportBuildTemplateV2YAMLFromCharacter(0, `{"stats":true}`, BuildTemplateV2ExportOptions{})
	if err == nil {
		t.Fatal("expected error for stats out of range")
	}
	if !strings.Contains(err.Error(), "build v2 template") {
		t.Errorf("error must wrap builder failure, got %q", err.Error())
	}
}

func TestSaveBuildTemplateV2FromCharacterToLibrary_ProfileStats(t *testing.T) {
	app := libraryV2Fixture(t)
	entry, err := app.SaveBuildTemplateV2FromCharacterToLibrary(0,
		`{"profile":true,"stats":true}`,
		BuildTemplateV2ExportOptions{Name: "L1 v2"},
	)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if entry.Version != 2 {
		t.Errorf("entry.Version = %d, want 2", entry.Version)
	}
	want := []string{"profile", "stats"}
	if len(entry.SelectedSections) != 2 ||
		entry.SelectedSections[0] != want[0] ||
		entry.SelectedSections[1] != want[1] {
		t.Errorf("SelectedSections = %v, want %v", entry.SelectedSections, want)
	}
	if entry.InventoryItems != 0 || entry.StorageItems != 0 {
		t.Errorf("inventory/storage should remain 0 for v2 profile/stats-only, got inv=%d sto=%d",
			entry.InventoryItems, entry.StorageItems)
	}
	if entry.Name != "L1 v2" {
		t.Errorf("entry.Name = %q, want %q", entry.Name, "L1 v2")
	}
	if entry.ID == "" {
		t.Error("entry.ID must be non-empty")
	}

	list, err := app.ListBuildTemplateLibrary()
	if err != nil {
		t.Fatalf("ListBuildTemplateLibrary: %v", err)
	}
	if len(list) != 1 || list[0].ID != entry.ID {
		t.Errorf("library list does not contain saved entry: %+v", list)
	}
}

func TestSaveBuildTemplateV2FromCharacterToLibrary_MetadataPreserved(t *testing.T) {
	app := libraryV2Fixture(t)
	entry, err := app.SaveBuildTemplateV2FromCharacterToLibrary(0,
		`{"profile":true}`,
		BuildTemplateV2ExportOptions{
			Name:        "metadata test",
			Description: "description body",
			Author:      "OiSiS",
			Tags:        []string{"v2", "profile"},
		},
	)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	loaded, err := app.templateLibrary.LoadTemplate(entry.ID)
	if err != nil {
		t.Fatalf("LoadTemplate: %v", err)
	}
	if loaded.Metadata == nil {
		t.Fatal("loaded.Metadata missing")
	}
	if loaded.Metadata.Name != "metadata test" {
		t.Errorf("Metadata.Name = %q", loaded.Metadata.Name)
	}
	if loaded.Metadata.Description != "description body" {
		t.Errorf("Metadata.Description = %q", loaded.Metadata.Description)
	}
	if loaded.Metadata.Author != "OiSiS" {
		t.Errorf("Metadata.Author = %q", loaded.Metadata.Author)
	}
	if len(loaded.Metadata.Tags) != 2 || loaded.Metadata.Tags[0] != "v2" || loaded.Metadata.Tags[1] != "profile" {
		t.Errorf("Metadata.Tags = %v", loaded.Metadata.Tags)
	}
	if loaded.Metadata.SourceCharacterIndex != 0 {
		t.Errorf("SourceCharacterIndex = %d", loaded.Metadata.SourceCharacterIndex)
	}
	if loaded.Metadata.SourceCharacterName != "Tester" {
		t.Errorf("SourceCharacterName = %q", loaded.Metadata.SourceCharacterName)
	}
}

func TestSaveBuildTemplateV2FromCharacterToLibrary_MalformedSelectionRejected(t *testing.T) {
	app := libraryV2Fixture(t)
	_, err := app.SaveBuildTemplateV2FromCharacterToLibrary(0, "not json", BuildTemplateV2ExportOptions{})
	if err == nil {
		t.Fatal("expected error for malformed selection JSON")
	}
	list, listErr := app.ListBuildTemplateLibrary()
	if listErr != nil {
		t.Fatalf("ListBuildTemplateLibrary: %v", listErr)
	}
	if len(list) != 0 {
		t.Errorf("library must remain empty after rejected save, got %d entries", len(list))
	}
}

func TestSaveBuildTemplateV2FromCharacterToLibrary_InvalidCharIndexRejected(t *testing.T) {
	app := libraryV2Fixture(t)
	for _, idx := range []int{-1, 10} {
		idx := idx
		t.Run(fmt.Sprintf("idx=%d", idx), func(t *testing.T) {
			_, err := app.SaveBuildTemplateV2FromCharacterToLibrary(idx, `{"profile":true}`, BuildTemplateV2ExportOptions{})
			if err == nil {
				t.Fatalf("expected error for invalid charIndex %d", idx)
			}
			if !strings.Contains(err.Error(), "invalid slot index") {
				t.Errorf("error must mention invalid slot index, got %q", err.Error())
			}
		})
	}
}

func TestSaveBuildTemplateV2FromCharacterToLibrary_NoSaveLoadedRejected(t *testing.T) {
	app := NewApp()
	app.templateLibrary = templates.NewTemplateLibrary(t.TempDir())
	_, err := app.SaveBuildTemplateV2FromCharacterToLibrary(0, `{"profile":true}`, BuildTemplateV2ExportOptions{})
	if err == nil {
		t.Fatal("expected error when no save loaded")
	}
	if !strings.Contains(err.Error(), "no save loaded") {
		t.Errorf("error must mention no save loaded, got %q", err.Error())
	}
}

func TestSaveBuildTemplateV2FromCharacterToLibrary_LibraryRoundTrip(t *testing.T) {
	app := libraryV2Fixture(t)
	entry, err := app.SaveBuildTemplateV2FromCharacterToLibrary(0,
		`{"profile":true,"stats":true}`,
		BuildTemplateV2ExportOptions{Name: "round-trip"},
	)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	loaded, err := app.templateLibrary.LoadTemplate(entry.ID)
	if err != nil {
		t.Fatalf("LoadTemplate: %v", err)
	}
	if loaded.Version != 2 {
		t.Errorf("loaded.Version = %d, want 2", loaded.Version)
	}
	if loaded.Sections.Profile == nil || loaded.Sections.Profile.Level == nil || *loaded.Sections.Profile.Level != 50 {
		t.Errorf("loaded profile.level lost: %+v", loaded.Sections.Profile)
	}
	if loaded.Selection == nil || loaded.Selection.Profile == nil || !loaded.Selection.Profile.All {
		t.Errorf("loaded Selection.Profile.All lost: %+v", loaded.Selection)
	}
}

func TestSaveBuildTemplateV2FromCharacterToLibrary_EmptyNameDefaultsFilename(t *testing.T) {
	app := libraryV2Fixture(t)
	entry, err := app.SaveBuildTemplateV2FromCharacterToLibrary(0,
		`{"profile":true}`,
		BuildTemplateV2ExportOptions{}, // Name == ""
	)
	if err != nil {
		t.Fatalf("Save with empty name: %v", err)
	}
	if !strings.HasPrefix(entry.Filename, "template-") {
		t.Errorf("Filename should fall back to template- prefix, got %q", entry.Filename)
	}
	if !strings.HasSuffix(entry.Filename, ".json") {
		t.Errorf("Filename should end with .json, got %q", entry.Filename)
	}
}
