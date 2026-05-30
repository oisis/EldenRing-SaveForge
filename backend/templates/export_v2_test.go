package templates

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/oisis/EldenRing-SaveForge/backend/editor"
)

func fixedNow() time.Time {
	return time.Date(2026, 5, 30, 12, 34, 56, 0, time.UTC)
}

func TestBuildV2Template_EmitsVersion2(t *testing.T) {
	tpl, err := BuildV2Template(ExportV2Options{
		AppVersion: "1.0.0-test",
		Now:        fixedNow(),
		Profile:    &ProfileSource{Level: u32p(50)},
		Selection:  &TemplateSelection{Profile: &SectionSelection{Fields: map[string]bool{"level": true}}},
	})
	if err != nil {
		t.Fatalf("BuildV2Template: %v", err)
	}
	if tpl.Schema != SchemaKey {
		t.Errorf("Schema = %q, want %q", tpl.Schema, SchemaKey)
	}
	if tpl.Version != 2 {
		t.Errorf("Version = %d, want 2", tpl.Version)
	}
	if want := fixedNow().UTC().Format(time.RFC3339); tpl.CreatedAt != want {
		t.Errorf("CreatedAt = %q, want %q", tpl.CreatedAt, want)
	}
	if tpl.AppVersion != "1.0.0-test" {
		t.Errorf("AppVersion = %q, want \"1.0.0-test\"", tpl.AppVersion)
	}
}

func TestBuildV2Template_ProfileOnly(t *testing.T) {
	tpl, err := BuildV2Template(ExportV2Options{
		Now:       fixedNow(),
		Profile:   &ProfileSource{Level: u32p(60), Runes: u32p(123456)},
		Selection: &TemplateSelection{Profile: &SectionSelection{All: true}},
	})
	if err != nil {
		t.Fatalf("BuildV2Template: %v", err)
	}
	if tpl.Sections.Profile == nil {
		t.Fatal("expected Sections.Profile to be set")
	}
	if tpl.Sections.Stats != nil {
		t.Error("expected Sections.Stats to be nil for profile-only build")
	}
	if tpl.Sections.InventoryWorkspace != nil {
		t.Error("expected Sections.InventoryWorkspace to be nil")
	}
	if err := ValidateBuildTemplate(tpl); err != nil {
		t.Errorf("output failed validation: %v", err)
	}
}

func TestBuildV2Template_StatsOnly(t *testing.T) {
	tpl, err := BuildV2Template(ExportV2Options{
		Now:       fixedNow(),
		Stats:     &StatsSource{Vigor: u32p(40), Mind: u32p(20)},
		Selection: &TemplateSelection{Stats: &SectionSelection{All: true}},
	})
	if err != nil {
		t.Fatalf("BuildV2Template: %v", err)
	}
	if tpl.Sections.Stats == nil {
		t.Fatal("expected Sections.Stats to be set")
	}
	if tpl.Sections.Profile != nil {
		t.Error("expected Sections.Profile to be nil for stats-only build")
	}
	if err := ValidateBuildTemplate(tpl); err != nil {
		t.Errorf("output failed validation: %v", err)
	}
}

func TestBuildV2Template_StatsBooleanShortcut(t *testing.T) {
	src := &StatsSource{
		Vigor: u32p(50), Mind: u32p(40), Endurance: u32p(30), Strength: u32p(20),
		Dexterity: u32p(15), Intelligence: u32p(10), Faith: u32p(8), Arcane: u32p(7),
	}
	tpl, err := BuildV2Template(ExportV2Options{
		Now:       fixedNow(),
		Stats:     src,
		Selection: &TemplateSelection{Stats: &SectionSelection{All: true}},
	})
	if err != nil {
		t.Fatalf("BuildV2Template: %v", err)
	}
	if tpl.Selection == nil || tpl.Selection.Stats == nil {
		t.Fatal("expected output Selection.Stats")
	}
	if !tpl.Selection.Stats.All {
		t.Error("expected Selection.Stats.All=true (shortcut survives)")
	}
	if tpl.Selection.Stats.Fields != nil {
		t.Errorf("expected Selection.Stats.Fields=nil for shortcut, got %v", tpl.Selection.Stats.Fields)
	}
	s := tpl.Sections.Stats
	if s == nil {
		t.Fatal("expected Sections.Stats")
	}
	checks := []struct {
		name string
		got  *uint32
		want uint32
	}{
		{"vigor", s.Vigor, 50}, {"mind", s.Mind, 40}, {"endurance", s.Endurance, 30},
		{"strength", s.Strength, 20}, {"dexterity", s.Dexterity, 15},
		{"intelligence", s.Intelligence, 10}, {"faith", s.Faith, 8}, {"arcane", s.Arcane, 7},
	}
	for _, c := range checks {
		if c.got == nil || *c.got != c.want {
			t.Errorf("stat %s = %v, want %d", c.name, c.got, c.want)
		}
	}
}

func TestBuildV2Template_PerFieldProfileSelection(t *testing.T) {
	tpl, err := BuildV2Template(ExportV2Options{
		Now:     fixedNow(),
		Profile: &ProfileSource{Name: "Hero", Level: u32p(50), Runes: u32p(1000)},
		Selection: &TemplateSelection{Profile: &SectionSelection{Fields: map[string]bool{
			"level": true, "runes": true,
		}}},
	})
	if err != nil {
		t.Fatalf("BuildV2Template: %v", err)
	}
	p := tpl.Sections.Profile
	if p == nil {
		t.Fatal("expected Sections.Profile")
	}
	if p.Level == nil || *p.Level != 50 {
		t.Errorf("Level = %v, want *50", p.Level)
	}
	if p.Runes == nil || *p.Runes != 1000 {
		t.Errorf("Runes = %v, want *1000", p.Runes)
	}
	if p.Name != nil {
		t.Errorf("Name should be omitted, got %q", *p.Name)
	}
	if tpl.Selection == nil || tpl.Selection.Profile == nil {
		t.Fatal("expected Selection.Profile")
	}
	want := map[string]bool{"level": true, "runes": true}
	if !reflect.DeepEqual(tpl.Selection.Profile.Fields, want) {
		t.Errorf("Selection.Profile.Fields = %v, want %v", tpl.Selection.Profile.Fields, want)
	}
	if tpl.Selection.Profile.All {
		t.Error("Selection.Profile.All must be false for per-field selection")
	}
}

func TestBuildV2Template_DropsSelectedButUnsetField(t *testing.T) {
	tpl, err := BuildV2Template(ExportV2Options{
		Now:     fixedNow(),
		Profile: &ProfileSource{Level: u32p(50)},
		Selection: &TemplateSelection{Profile: &SectionSelection{Fields: map[string]bool{
			"level": true, "runes": true,
		}}},
	})
	if err != nil {
		t.Fatalf("BuildV2Template: %v", err)
	}
	p := tpl.Sections.Profile
	if p == nil {
		t.Fatal("expected Sections.Profile")
	}
	if p.Level == nil || *p.Level != 50 {
		t.Errorf("Level = %v, want *50", p.Level)
	}
	if p.Runes != nil {
		t.Errorf("Runes should be omitted (source nil), got %v", *p.Runes)
	}
	want := map[string]bool{"level": true}
	if !reflect.DeepEqual(tpl.Selection.Profile.Fields, want) {
		t.Errorf("Selection.Profile.Fields = %v, want %v (runes dropped)", tpl.Selection.Profile.Fields, want)
	}
}

func TestBuildV2Template_OutputValidates(t *testing.T) {
	tpl, err := BuildV2Template(ExportV2Options{
		Now:     fixedNow(),
		Profile: &ProfileSource{Level: u32p(50), ClassName: "Vagabond"},
		Stats:   &StatsSource{Vigor: u32p(40)},
		Selection: &TemplateSelection{
			Profile: &SectionSelection{Fields: map[string]bool{"level": true, "class": true}},
			Stats:   &SectionSelection{All: true},
		},
	})
	if err != nil {
		t.Fatalf("BuildV2Template: %v", err)
	}
	if err := ValidateBuildTemplate(tpl); err != nil {
		t.Errorf("ValidateBuildTemplate: %v", err)
	}
}

func TestBuildV2Template_YAMLRoundTrip(t *testing.T) {
	tpl, err := BuildV2Template(ExportV2Options{
		AppVersion: "1.0.0-test",
		Now:        fixedNow(),
		Profile:    &ProfileSource{Name: "Hero", Level: u32p(50), ClassName: "Vagabond"},
		Stats:      &StatsSource{Vigor: u32p(40), Mind: u32p(20)},
		Selection: &TemplateSelection{
			Profile: &SectionSelection{All: true},
			Stats:   &SectionSelection{Fields: map[string]bool{"vigor": true, "mind": true}},
		},
	})
	if err != nil {
		t.Fatalf("BuildV2Template: %v", err)
	}
	data, err := MarshalBuildTemplateYAML(tpl)
	if err != nil {
		t.Fatalf("MarshalBuildTemplateYAML: %v", err)
	}
	roundtrip, err := ParseBuildTemplateYAML(data)
	if err != nil {
		t.Fatalf("ParseBuildTemplateYAML: %v\nYAML:\n%s", err, data)
	}
	if roundtrip.Version != 2 {
		t.Errorf("round-trip Version = %d, want 2", roundtrip.Version)
	}
	if roundtrip.Sections.Profile == nil || roundtrip.Sections.Profile.Name == nil || *roundtrip.Sections.Profile.Name != "Hero" {
		t.Errorf("round-trip lost profile.name")
	}
	if roundtrip.Sections.Profile.Class == nil || *roundtrip.Sections.Profile.Class != "Vagabond" {
		t.Errorf("round-trip lost profile.class")
	}
	if roundtrip.Sections.Stats == nil || roundtrip.Sections.Stats.Vigor == nil || *roundtrip.Sections.Stats.Vigor != 40 {
		t.Errorf("round-trip lost stats.vigor")
	}
	if roundtrip.Selection == nil || roundtrip.Selection.Profile == nil || !roundtrip.Selection.Profile.All {
		t.Errorf("round-trip lost selection.profile shortcut")
	}
}

func TestBuildV2Template_NoForbiddenTechFields(t *testing.T) {
	tpl, err := BuildV2Template(ExportV2Options{
		Now:     fixedNow(),
		Profile: &ProfileSource{Name: "Hero", Level: u32p(50), ClassName: "Vagabond"},
		Stats:   &StatsSource{Vigor: u32p(40)},
		Selection: &TemplateSelection{
			Profile: &SectionSelection{All: true},
			Stats:   &SectionSelection{All: true},
		},
	})
	if err != nil {
		t.Fatalf("BuildV2Template: %v", err)
	}
	data, err := MarshalBuildTemplateYAML(tpl)
	if err != nil {
		t.Fatalf("MarshalBuildTemplateYAML: %v", err)
	}
	body := strings.ToLower(string(data))
	forbidden := []string{
		"handle", "acquisitionindex", "eventflag", "facedata",
		"saveblob", "gaitem", "originalhandle", "uid", "steamid",
	}
	for _, kw := range forbidden {
		if strings.Contains(body, kw) {
			t.Errorf("YAML payload contains forbidden tech field substring %q\n---\n%s", kw, data)
		}
	}
}

func TestBuildV2Template_RejectsMissingSelection(t *testing.T) {
	_, err := BuildV2Template(ExportV2Options{
		Now:     fixedNow(),
		Profile: &ProfileSource{Level: u32p(50)},
	})
	if err == nil {
		t.Fatal("expected error when Selection is nil")
	}
	if !strings.Contains(err.Error(), "selection is required") {
		t.Errorf("error message = %q, want hint about required selection", err.Error())
	}
}

func TestBuildV2Template_RejectsEmptySelection(t *testing.T) {
	_, err := BuildV2Template(ExportV2Options{
		Now:       fixedNow(),
		Profile:   &ProfileSource{Level: u32p(50)},
		Selection: &TemplateSelection{},
	})
	if err == nil {
		t.Fatal("expected error when Selection has no selected fields")
	}
	if !strings.Contains(err.Error(), "no selected fields") {
		t.Errorf("error message = %q, want hint about empty selection", err.Error())
	}
}

func TestBuildV2Template_RejectsSelectedSectionWithoutSource(t *testing.T) {
	t.Run("profile selected, Profile nil", func(t *testing.T) {
		_, err := BuildV2Template(ExportV2Options{
			Now:       fixedNow(),
			Selection: &TemplateSelection{Profile: &SectionSelection{All: true}},
		})
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "selection.profile") {
			t.Errorf("error = %q, want hint about selection.profile", err.Error())
		}
	})
	t.Run("stats selected, Stats nil", func(t *testing.T) {
		_, err := BuildV2Template(ExportV2Options{
			Now:       fixedNow(),
			Selection: &TemplateSelection{Stats: &SectionSelection{All: true}},
		})
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "selection.stats") {
			t.Errorf("error = %q, want hint about selection.stats", err.Error())
		}
	})
	t.Run("inventory.workspace selected, InventoryWorkspace nil", func(t *testing.T) {
		_, err := BuildV2Template(ExportV2Options{
			Now:       fixedNow(),
			Selection: &TemplateSelection{InventoryWorkspace: &SectionSelection{All: true}},
		})
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "selection.inventory.workspace") {
			t.Errorf("error = %q, want hint about selection.inventory.workspace", err.Error())
		}
	})
}

func TestBuildV2Template_PreservesInventoryWorkspaceWhenPassed(t *testing.T) {
	inv := &InventoryWorkspaceSection{
		InventoryItems: []TemplateItem{{
			BaseItemID: 0x000F4240, Quantity: 1,
			Container: ContainerInventory, Position: 0,
		}},
		StorageItems: []TemplateItem{},
	}
	tpl, err := BuildV2Template(ExportV2Options{
		Now:                fixedNow(),
		InventoryWorkspace: inv,
		Selection:          &TemplateSelection{InventoryWorkspace: &SectionSelection{All: true}},
	})
	if err != nil {
		t.Fatalf("BuildV2Template: %v", err)
	}
	if tpl.Sections.InventoryWorkspace == nil {
		t.Fatal("expected Sections.InventoryWorkspace")
	}
	if len(tpl.Sections.InventoryWorkspace.InventoryItems) != 1 {
		t.Errorf("inventory item count = %d, want 1", len(tpl.Sections.InventoryWorkspace.InventoryItems))
	}
	if tpl.Sections.InventoryWorkspace.InventoryItems[0].BaseItemID != 0x000F4240 {
		t.Errorf("baseItemID = 0x%X, want 0x000F4240", tpl.Sections.InventoryWorkspace.InventoryItems[0].BaseItemID)
	}
	if tpl.Selection == nil || tpl.Selection.InventoryWorkspace == nil || !tpl.Selection.InventoryWorkspace.All {
		t.Error("expected Selection.InventoryWorkspace.All=true")
	}
}

func TestBuildV2Template_MetadataPassthrough(t *testing.T) {
	meta := &TemplateMetadata{
		Name:                 "My Build",
		Description:          "Test description",
		Author:               "Tester",
		Tags:                 []string{"v2", "test"},
		SourceCharacterIndex: 3,
		SourceCharacterName:  "Hero",
	}
	tpl, err := BuildV2Template(ExportV2Options{
		AppVersion: "1.0.0-test",
		Metadata:   meta,
		Now:        fixedNow(),
		Profile:    &ProfileSource{Level: u32p(50)},
		Selection:  &TemplateSelection{Profile: &SectionSelection{Fields: map[string]bool{"level": true}}},
	})
	if err != nil {
		t.Fatalf("BuildV2Template: %v", err)
	}
	if tpl.Metadata == nil {
		t.Fatal("expected Metadata to be set")
	}
	if !reflect.DeepEqual(tpl.Metadata, meta) {
		t.Errorf("Metadata = %+v, want %+v", tpl.Metadata, meta)
	}
	if tpl.AppVersion != "1.0.0-test" {
		t.Errorf("AppVersion = %q, want \"1.0.0-test\"", tpl.AppVersion)
	}
}

func TestBuildV2Template_ClassNamePassthrough(t *testing.T) {
	tpl, err := BuildV2Template(ExportV2Options{
		Now:     fixedNow(),
		Profile: &ProfileSource{ClassName: "Vagabond"},
		Selection: &TemplateSelection{Profile: &SectionSelection{
			Fields: map[string]bool{"class": true},
		}},
	})
	if err != nil {
		t.Fatalf("BuildV2Template: %v", err)
	}
	if tpl.Sections.Profile == nil || tpl.Sections.Profile.Class == nil {
		t.Fatal("expected Sections.Profile.Class to be set")
	}
	if *tpl.Sections.Profile.Class != "Vagabond" {
		t.Errorf("Class = %q, want \"Vagabond\"", *tpl.Sections.Profile.Class)
	}
}

func TestBuildV2Template_V1ExporterUnchanged(t *testing.T) {
	snap := editor.InventoryWorkspaceSnapshot{
		InventoryItems: []editor.EditableItem{{
			BaseItemID: 0x000F4240,
			Name:       "Dagger",
			Container:  editor.ContainerInventory,
			Quantity:   1,
			Position:   0,
		}},
	}
	tpl, _, err := BuildTemplateFromSnapshot(snap, ExportOptions{
		IncludeInventory: true,
		Now:              fixedNow(),
	})
	if err != nil {
		t.Fatalf("BuildTemplateFromSnapshot: %v", err)
	}
	if tpl.Version != 1 {
		t.Errorf("v1 builder Version = %d, want 1", tpl.Version)
	}
	if tpl.Version != SchemaVersion {
		t.Errorf("v1 builder Version=%d does not match SchemaVersion=%d", tpl.Version, SchemaVersion)
	}
}

func TestBuildV2Template_AllShortcutWithPartialSource(t *testing.T) {
	tpl, err := BuildV2Template(ExportV2Options{
		Now:       fixedNow(),
		Profile:   &ProfileSource{Name: "Hero", Level: u32p(50)},
		Selection: &TemplateSelection{Profile: &SectionSelection{All: true}},
	})
	if err != nil {
		t.Fatalf("BuildV2Template: %v", err)
	}
	if tpl.Selection == nil || tpl.Selection.Profile == nil || !tpl.Selection.Profile.All {
		t.Fatal("expected Selection.Profile.All=true to survive normalization")
	}
	if tpl.Selection.Profile.Fields != nil {
		t.Errorf("shortcut output should not carry Fields map, got %v", tpl.Selection.Profile.Fields)
	}
	p := tpl.Sections.Profile
	if p == nil {
		t.Fatal("expected Sections.Profile")
	}
	if p.Name == nil || *p.Name != "Hero" {
		t.Errorf("Name = %v, want *Hero", p.Name)
	}
	if p.Level == nil || *p.Level != 50 {
		t.Errorf("Level = %v, want *50", p.Level)
	}
	if p.Runes != nil {
		t.Errorf("Runes should be omitted (source nil), got %v", *p.Runes)
	}
	if p.Class != nil {
		t.Errorf("Class should be omitted (source empty), got %v", *p.Class)
	}
	if err := ValidateBuildTemplate(tpl); err != nil {
		t.Errorf("output failed validation: %v", err)
	}
}
