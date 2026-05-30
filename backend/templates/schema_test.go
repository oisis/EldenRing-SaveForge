package templates

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestSchemaRoundtrip marshals a fully-populated template, unmarshals it
// back and asserts every load-bearing field survives. The decoded copy
// must also pass ValidateBuildTemplate.
func TestSchemaRoundtrip(t *testing.T) {
	aow := uint32(0x80002710)
	tpl := &BuildTemplate{
		Schema:     SchemaKey,
		Version:    SchemaVersion,
		CreatedAt:  "2026-05-17T12:34:56Z",
		AppVersion: "0.15.0-beta",
		Metadata: &TemplateMetadata{
			Name:                 "Greatsword build",
			Description:          "RL150 quality",
			Author:               "OiSiS",
			Tags:                 []string{"pvp", "rl150"},
			SourceCharacterIndex: 2,
			SourceCharacterName:  "Tarnished",
		},
		Sections: TemplateSections{
			InventoryWorkspace: &InventoryWorkspaceSection{
				InventoryItems: []TemplateItem{
					{
						BaseItemID:   0x003D9700,
						Name:         "Greatsword",
						Category:     "melee_armaments",
						Quantity:     1,
						Upgrade:      25,
						InfusionName: "Heavy",
						AoWItemID:    &aow,
						Container:    ContainerInventory,
						Position:     0,
					},
				},
				StorageItems: []TemplateItem{
					{
						BaseItemID: 0x02FAF080,
						Name:       "Standard Arrow",
						Category:   "ammo",
						Quantity:   99,
						Container:  ContainerStorage,
						Position:   0,
					},
				},
			},
		},
	}

	data, err := json.Marshal(tpl)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var got BuildTemplate
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if got.Schema != SchemaKey || got.Version != SchemaVersion {
		t.Fatalf("schema/version drift: got %q v%d", got.Schema, got.Version)
	}
	if got.Sections.InventoryWorkspace == nil {
		t.Fatalf("inventory.workspace section lost in roundtrip")
	}
	if len(got.Sections.InventoryWorkspace.InventoryItems) != 1 {
		t.Fatalf("inventoryItems lost: %+v", got.Sections.InventoryWorkspace.InventoryItems)
	}
	roundtripItem := got.Sections.InventoryWorkspace.InventoryItems[0]
	if roundtripItem.AoWItemID == nil || *roundtripItem.AoWItemID != aow {
		t.Fatalf("AoWItemID lost in roundtrip: %v", roundtripItem.AoWItemID)
	}
	if err := ValidateBuildTemplate(&got); err != nil {
		t.Fatalf("decoded template fails validation: %v", err)
	}
}

// TestSchemaJSON_OmitsForbiddenFields builds a template with every
// optional field populated and asserts the resulting JSON does not leak
// any of the editor-side handle / session fields that are forbidden in
// templates by Phase A schema contract.
//
// This is the regression guard against future field additions in
// editor.EditableItem that might accidentally leak into the template if
// someone serializes EditableItem directly.
func TestSchemaJSON_OmitsForbiddenFields(t *testing.T) {
	aow := uint32(0x80002710)
	tpl := &BuildTemplate{
		Schema:     SchemaKey,
		Version:    SchemaVersion,
		CreatedAt:  "2026-05-17T12:34:56Z",
		AppVersion: "0.15.0-beta",
		Metadata: &TemplateMetadata{
			Name: "Test",
		},
		Sections: TemplateSections{
			InventoryWorkspace: &InventoryWorkspaceSection{
				InventoryItems: []TemplateItem{{
					BaseItemID:   0x003D9700,
					Name:         "Greatsword",
					Category:     "melee_armaments",
					Quantity:     1,
					Upgrade:      25,
					InfusionName: "Heavy",
					AoWItemID:    &aow,
					Container:    ContainerInventory,
					Position:     0,
				}},
				StorageItems: []TemplateItem{},
			},
		},
	}

	data, err := json.Marshal(tpl)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	body := string(data)

	forbidden := []string{
		"originalHandle",
		"currentAoWHandle",
		"\"uid\"",
		"acquisitionIndex",
		"pendingAoWItemID",
		"pendingAoWName",
		"pendingAoWClear",
		"hasGaItem",
		"hasCurrentAoW",
		"currentAoWShared",
		"currentAoWStatus",
		"hasPendingWeaponPatch",
		"isWeapon",
		"isArmor",
		"isTalisman",
		"maxUpgrade",
		"iconPath",
	}
	for _, key := range forbidden {
		if strings.Contains(body, key) {
			t.Errorf("forbidden field %q leaked into template JSON:\n%s", key, body)
		}
	}
}

// TestSchemaJSON_OmitsAoWWhenNil ensures the AoWItemID pointer + omitempty
// combination produces a JSON document where the field is absent for
// weapons without a custom AoW, rather than zero-valued.
func TestSchemaJSON_OmitsAoWWhenNil(t *testing.T) {
	tpl := &BuildTemplate{
		Schema:    SchemaKey,
		Version:   SchemaVersion,
		CreatedAt: "2026-05-17T12:34:56Z",
		Sections: TemplateSections{
			InventoryWorkspace: &InventoryWorkspaceSection{
				InventoryItems: []TemplateItem{{
					BaseItemID: 0x003D9700,
					Name:       "Greatsword (no AoW)",
					Quantity:   1,
					Container:  ContainerInventory,
					Position:   0,
				}},
				StorageItems: []TemplateItem{},
			},
		},
	}
	data, err := json.Marshal(tpl)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if strings.Contains(string(data), "aowItemID") {
		t.Errorf("aowItemID must be absent when nil, got:\n%s", string(data))
	}
}

func TestValidate_RejectsWrongSchema(t *testing.T) {
	tpl := minimalValidTemplate()
	tpl.Schema = "something-else"
	if err := ValidateBuildTemplate(tpl); err == nil {
		t.Fatal("expected error for wrong schema")
	}
}

func TestValidate_RejectsUnsupportedVersion(t *testing.T) {
	tpl := minimalValidTemplate()
	tpl.Version = MaxSchemaVersion + 1
	if err := ValidateBuildTemplate(tpl); err == nil {
		t.Fatal("expected error for unsupported version")
	}
}

func TestValidate_RejectsZeroVersion(t *testing.T) {
	tpl := minimalValidTemplate()
	tpl.Version = 0
	if err := ValidateBuildTemplate(tpl); err == nil {
		t.Fatal("expected error for version=0")
	}
}

func TestValidate_RejectsMissingInventoryWorkspaceSection(t *testing.T) {
	tpl := minimalValidTemplate()
	tpl.Sections.InventoryWorkspace = nil
	if err := ValidateBuildTemplate(tpl); err == nil {
		t.Fatal("expected error for missing section")
	}
}

func TestValidate_RejectsEmptySection(t *testing.T) {
	tpl := minimalValidTemplate()
	tpl.Sections.InventoryWorkspace.InventoryItems = []TemplateItem{}
	tpl.Sections.InventoryWorkspace.StorageItems = []TemplateItem{}
	if err := ValidateBuildTemplate(tpl); err == nil {
		t.Fatal("expected error for empty section")
	}
}

func TestValidate_RejectsZeroBaseItemID(t *testing.T) {
	tpl := minimalValidTemplate()
	tpl.Sections.InventoryWorkspace.InventoryItems[0].BaseItemID = 0
	if err := ValidateBuildTemplate(tpl); err == nil {
		t.Fatal("expected error for baseItemID=0")
	}
}

func TestValidate_RejectsZeroQuantity(t *testing.T) {
	tpl := minimalValidTemplate()
	tpl.Sections.InventoryWorkspace.InventoryItems[0].Quantity = 0
	if err := ValidateBuildTemplate(tpl); err == nil {
		t.Fatal("expected error for quantity=0")
	}
}

func TestValidate_RejectsContainerMismatch(t *testing.T) {
	tpl := minimalValidTemplate()
	tpl.Sections.InventoryWorkspace.InventoryItems[0].Container = ContainerStorage
	if err := ValidateBuildTemplate(tpl); err == nil {
		t.Fatal("expected error for container mismatch")
	}
}

func TestValidate_RejectsNilTemplate(t *testing.T) {
	if err := ValidateBuildTemplate(nil); err == nil {
		t.Fatal("expected error for nil template")
	}
}

func TestValidate_AcceptsMinimalValid(t *testing.T) {
	tpl := minimalValidTemplate()
	if err := ValidateBuildTemplate(tpl); err != nil {
		t.Fatalf("expected minimal template to validate: %v", err)
	}
}

func minimalValidTemplate() *BuildTemplate {
	return &BuildTemplate{
		Schema:    SchemaKey,
		Version:   SchemaVersion,
		CreatedAt: "2026-05-17T12:34:56Z",
		Sections: TemplateSections{
			InventoryWorkspace: &InventoryWorkspaceSection{
				InventoryItems: []TemplateItem{{
					BaseItemID: 0x003D9700,
					Quantity:   1,
					Container:  ContainerInventory,
					Position:   0,
				}},
				StorageItems: []TemplateItem{},
			},
		},
	}
}

// ─── Phase 3A — schema v2 structural tests ───────────────────────────────

func u32p(v uint32) *uint32 { return &v }
func u8p(v uint8) *uint8    { return &v }
func strp(v string) *string { return &v }
func boolp(v bool) *bool    { return &v }

// minimalValidV2 returns a v2 template with one selected section. The
// `which` argument controls which section is present + selected so
// individual tests can cover each path without writing fresh fixtures.
func minimalValidV2(which string) *BuildTemplate {
	tpl := &BuildTemplate{
		Schema:    SchemaKey,
		Version:   2,
		CreatedAt: "2026-05-17T12:34:56Z",
		Selection: &TemplateSelection{},
		Sections:  TemplateSections{},
	}
	switch which {
	case "profile":
		tpl.Selection.Profile = &SectionSelection{All: true}
		tpl.Sections.Profile = &ProfileSection{
			Level: u32p(150),
			Runes: u32p(0),
		}
	case "stats":
		tpl.Selection.Stats = &SectionSelection{All: true}
		tpl.Sections.Stats = &StatsSection{
			Vigor:        u32p(60),
			Mind:         u32p(25),
			Endurance:    u32p(25),
			Strength:     u32p(12),
			Dexterity:    u32p(18),
			Intelligence: u32p(80),
			Faith:        u32p(9),
			Arcane:       u32p(7),
		}
	case "inventory":
		tpl.Selection.InventoryWorkspace = &SectionSelection{All: true}
		tpl.Sections.InventoryWorkspace = &InventoryWorkspaceSection{
			InventoryItems: []TemplateItem{{
				BaseItemID: 0x003D9700,
				Quantity:   1,
				Container:  ContainerInventory,
				Position:   0,
			}},
			StorageItems: []TemplateItem{},
		}
	}
	return tpl
}

// TestSchema_ExporterEmitsV1 anchors the Phase 3A invariant that
// SchemaVersion (the constant the v1 exporter emits) does not drift up
// when MaxSchemaVersion is bumped.
func TestSchema_ExporterEmitsV1(t *testing.T) {
	if SchemaVersion != 1 {
		t.Fatalf("SchemaVersion must remain 1 — v1 exporter would otherwise emit a wrong version; got %d", SchemaVersion)
	}
	if MaxSchemaVersion < SchemaVersion {
		t.Fatalf("MaxSchemaVersion (%d) must be >= SchemaVersion (%d)", MaxSchemaVersion, SchemaVersion)
	}
	if MaxSchemaVersion != 2 {
		t.Fatalf("MaxSchemaVersion must be 2 in Phase 3A; got %d", MaxSchemaVersion)
	}
}

func TestValidate_RejectsBeyondMaxSchemaVersion(t *testing.T) {
	tpl := minimalValidV2("profile")
	tpl.Version = MaxSchemaVersion + 1
	if err := ValidateBuildTemplate(tpl); err == nil {
		t.Fatalf("expected error for version=%d", tpl.Version)
	}
}

func TestValidate_AcceptsV2_ProfileOnly(t *testing.T) {
	tpl := minimalValidV2("profile")
	if err := ValidateBuildTemplate(tpl); err != nil {
		t.Fatalf("expected v2 profile-only to validate: %v", err)
	}
}

func TestValidate_AcceptsV2_StatsOnly(t *testing.T) {
	tpl := minimalValidV2("stats")
	if err := ValidateBuildTemplate(tpl); err != nil {
		t.Fatalf("expected v2 stats-only to validate: %v", err)
	}
}

func TestValidate_AcceptsV2_StatsBooleanShortcut(t *testing.T) {
	tpl := minimalValidV2("stats")
	// Already All: true via helper; assert HasAny is true and explicitly
	// rebuild via boolean payload through JSON to exercise the decoder.
	data, err := json.Marshal(tpl)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if !strings.Contains(string(data), `"stats":true`) {
		t.Fatalf("expected selection.stats to be emitted as boolean shortcut, got:\n%s", data)
	}
	var got BuildTemplate
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if err := ValidateBuildTemplate(&got); err != nil {
		t.Fatalf("expected round-tripped boolean-shortcut stats to validate: %v", err)
	}
	if !got.Selection.Stats.All {
		t.Fatalf("selection.stats.All lost on round-trip")
	}
}

func TestValidate_AcceptsV2_InventoryWorkspaceShortcut(t *testing.T) {
	tpl := minimalValidV2("inventory")
	if err := ValidateBuildTemplate(tpl); err != nil {
		t.Fatalf("expected v2 inventory.workspace shortcut to validate: %v", err)
	}
}

func TestValidate_AcceptsV2_ProfileAndStats(t *testing.T) {
	tpl := minimalValidV2("profile")
	statsTpl := minimalValidV2("stats")
	tpl.Selection.Stats = statsTpl.Selection.Stats
	tpl.Sections.Stats = statsTpl.Sections.Stats
	if err := ValidateBuildTemplate(tpl); err != nil {
		t.Fatalf("expected v2 profile+stats to validate: %v", err)
	}
}

func TestValidate_AcceptsV2_ProfileWithoutInventoryWorkspace(t *testing.T) {
	tpl := minimalValidV2("profile")
	if tpl.Sections.InventoryWorkspace != nil {
		t.Fatalf("fixture leak: inventory.workspace must be nil here")
	}
	if err := ValidateBuildTemplate(tpl); err != nil {
		t.Fatalf("expected v2 with only profile (no inventory.workspace) to validate: %v", err)
	}
}

// TestValidate_AcceptsV2_SectionPresentNotSelected documents the
// decision that selection is the source of truth: an unrelated section
// may sit in `sections` without being elected by `selection`, and the
// validator must accept it (the section is still validated for its own
// structural invariants).
func TestValidate_AcceptsV2_SectionPresentNotSelected(t *testing.T) {
	tpl := minimalValidV2("stats")
	tpl.Sections.Profile = &ProfileSection{
		Level: u32p(120),
	}
	if err := ValidateBuildTemplate(tpl); err != nil {
		t.Fatalf("expected unselected-but-present profile section to be accepted: %v", err)
	}
}

func TestValidate_AcceptsV2_PerFieldStatsSelection(t *testing.T) {
	tpl := minimalValidV2("stats")
	tpl.Selection.Stats = &SectionSelection{
		Fields: map[string]bool{
			"vigor":        true,
			"intelligence": true,
		},
	}
	if err := ValidateBuildTemplate(tpl); err != nil {
		t.Fatalf("expected per-field stats selection to validate: %v", err)
	}
}

func TestValidate_RejectsV2_NoSelection(t *testing.T) {
	tpl := minimalValidV2("profile")
	tpl.Selection = nil
	if err := ValidateBuildTemplate(tpl); err == nil {
		t.Fatal("expected error for v2 without selection")
	}
}

func TestValidate_RejectsV2_EmptySelection(t *testing.T) {
	tpl := minimalValidV2("profile")
	tpl.Selection = &TemplateSelection{}
	if err := ValidateBuildTemplate(tpl); err == nil {
		t.Fatal("expected error for v2 with empty selection")
	}

	tpl2 := minimalValidV2("profile")
	tpl2.Selection = &TemplateSelection{Profile: &SectionSelection{All: false}}
	if err := ValidateBuildTemplate(tpl2); err == nil {
		t.Fatal("expected error for v2 with selection.profile=false")
	}

	tpl3 := minimalValidV2("profile")
	tpl3.Selection = &TemplateSelection{Profile: &SectionSelection{
		Fields: map[string]bool{"level": false, "runes": false},
	}}
	if err := ValidateBuildTemplate(tpl3); err == nil {
		t.Fatal("expected error for v2 with all-false fields map")
	}
}

func TestValidate_RejectsV2_SelectedProfileMissingSection(t *testing.T) {
	tpl := minimalValidV2("profile")
	tpl.Sections.Profile = nil
	if err := ValidateBuildTemplate(tpl); err == nil {
		t.Fatal("expected error for selected profile without sections.profile")
	}
}

func TestValidate_RejectsV2_SelectedStatsMissingSection(t *testing.T) {
	tpl := minimalValidV2("stats")
	tpl.Sections.Stats = nil
	if err := ValidateBuildTemplate(tpl); err == nil {
		t.Fatal("expected error for selected stats without sections.stats")
	}
}

func TestValidate_RejectsV2_SelectedInventoryMissingSection(t *testing.T) {
	tpl := minimalValidV2("inventory")
	tpl.Sections.InventoryWorkspace = nil
	if err := ValidateBuildTemplate(tpl); err == nil {
		t.Fatal("expected error for selected inventory.workspace without the section")
	}
}

func TestValidate_RejectsV2_ProfileLevelZero(t *testing.T) {
	tpl := minimalValidV2("profile")
	tpl.Sections.Profile.Level = u32p(0)
	if err := ValidateBuildTemplate(tpl); err == nil {
		t.Fatal("expected error for profile.level=0")
	}
}

func TestValidate_RejectsV2_ProfileLevelTooHigh(t *testing.T) {
	tpl := minimalValidV2("profile")
	tpl.Sections.Profile.Level = u32p(MaxProfileLevel + 1)
	if err := ValidateBuildTemplate(tpl); err == nil {
		t.Fatalf("expected error for profile.level=%d", MaxProfileLevel+1)
	}
}

func TestValidate_RejectsV2_ProfileTalismanSlotsTooHigh(t *testing.T) {
	tpl := minimalValidV2("profile")
	tpl.Sections.Profile.TalismanSlots = u8p(4)
	if err := ValidateBuildTemplate(tpl); err == nil {
		t.Fatal("expected error for profile.talismanSlots=4")
	}
}

func TestValidate_RejectsV2_ProfileNameTooLong(t *testing.T) {
	tpl := minimalValidV2("profile")
	tpl.Sections.Profile.Name = strp(strings.Repeat("A", 17))
	if err := ValidateBuildTemplate(tpl); err == nil {
		t.Fatal("expected error for profile.name longer than 16 UTF-16 units")
	}
}

func TestValidate_RejectsV2_ProfileNameEmpty(t *testing.T) {
	tpl := minimalValidV2("profile")
	tpl.Sections.Profile.Name = strp("")
	if err := ValidateBuildTemplate(tpl); err == nil {
		t.Fatal("expected error for empty profile.name")
	}
}

func TestValidate_RejectsV2_ProfileClearCountTooHigh(t *testing.T) {
	tpl := minimalValidV2("profile")
	tpl.Sections.Profile.ClearCount = u32p(MaxProfileClearCount + 1)
	if err := ValidateBuildTemplate(tpl); err == nil {
		t.Fatal("expected error for clearCount > cap")
	}
}

func TestValidate_RejectsV2_ProfileScadutreeTooHigh(t *testing.T) {
	tpl := minimalValidV2("profile")
	tpl.Sections.Profile.ScadutreeBlessing = u8p(MaxProfileScadutreeBlessing + 1)
	if err := ValidateBuildTemplate(tpl); err == nil {
		t.Fatal("expected error for scadutreeBlessing > cap")
	}
}

func TestValidate_RejectsV2_ProfileShadowRealmTooHigh(t *testing.T) {
	tpl := minimalValidV2("profile")
	tpl.Sections.Profile.ShadowRealmBlessing = u8p(MaxProfileShadowRealmBlessing + 1)
	if err := ValidateBuildTemplate(tpl); err == nil {
		t.Fatal("expected error for shadowRealmBlessing > cap")
	}
}

func TestValidate_RejectsV2_StatsVigorZero(t *testing.T) {
	tpl := minimalValidV2("stats")
	tpl.Sections.Stats.Vigor = u32p(0)
	if err := ValidateBuildTemplate(tpl); err == nil {
		t.Fatal("expected error for stats.vigor=0")
	}
}

func TestValidate_RejectsV2_StatsArcaneTooHigh(t *testing.T) {
	tpl := minimalValidV2("stats")
	tpl.Sections.Stats.Arcane = u32p(MaxStatValue + 1)
	if err := ValidateBuildTemplate(tpl); err == nil {
		t.Fatal("expected error for stats.arcane > cap")
	}
}

func TestValidate_RejectsV2_InvalidInventoryItem(t *testing.T) {
	tpl := minimalValidV2("inventory")
	tpl.Sections.InventoryWorkspace.InventoryItems[0].BaseItemID = 0
	if err := ValidateBuildTemplate(tpl); err == nil {
		t.Fatal("expected v2 to reject inventory item with baseItemID=0")
	}
}

func TestValidate_RejectsV2_UnknownProfileSelectionKey(t *testing.T) {
	tpl := minimalValidV2("profile")
	tpl.Selection.Profile = &SectionSelection{
		Fields: map[string]bool{
			"level":      true,
			"bogusField": true,
		},
	}
	if err := ValidateBuildTemplate(tpl); err == nil {
		t.Fatal("expected error for unknown selection.profile field key")
	}
}

func TestValidate_RejectsV2_UnknownStatsSelectionKey(t *testing.T) {
	tpl := minimalValidV2("stats")
	tpl.Selection.Stats = &SectionSelection{
		Fields: map[string]bool{
			"vigor":   true,
			"luckily": true,
		},
	}
	if err := ValidateBuildTemplate(tpl); err == nil {
		t.Fatal("expected error for unknown selection.stats field key")
	}
}

func TestValidate_RejectsV2_InventoryWorkspaceSelectionAsMap(t *testing.T) {
	tpl := minimalValidV2("inventory")
	tpl.Selection.InventoryWorkspace = &SectionSelection{
		Fields: map[string]bool{
			"inventoryItems": true,
		},
	}
	if err := ValidateBuildTemplate(tpl); err == nil {
		t.Fatal("expected error for non-boolean inventory.workspace selection")
	}
}

// TestSchemaV2_RoundtripJSON marshals a populated v2 template (profile
// + stats + inventory.workspace with explicit per-field selection on
// profile, boolean shortcut on stats and boolean shortcut on
// inventory.workspace), unmarshals it, and asserts every field
// survives. The decoded copy must also pass ValidateBuildTemplate.
func TestSchemaV2_RoundtripJSON(t *testing.T) {
	tpl := &BuildTemplate{
		Schema:     SchemaKey,
		Version:    2,
		CreatedAt:  "2026-05-30T12:00:00Z",
		AppVersion: "1.0.0-beta1",
		Metadata: &TemplateMetadata{
			Name: "RL150 INT",
			Tags: []string{"int", "cold"},
		},
		Selection: &TemplateSelection{
			Profile: &SectionSelection{
				Fields: map[string]bool{
					"level":         true,
					"runes":         true,
					"talismanSlots": true,
				},
			},
			Stats:              &SectionSelection{All: true},
			InventoryWorkspace: &SectionSelection{All: true},
		},
		Sections: TemplateSections{
			Profile: &ProfileSection{
				Level:         u32p(150),
				Runes:         u32p(0),
				TalismanSlots: u8p(2),
			},
			Stats: &StatsSection{
				Vigor:        u32p(60),
				Intelligence: u32p(80),
			},
			InventoryWorkspace: &InventoryWorkspaceSection{
				InventoryItems: []TemplateItem{{
					BaseItemID: 0x003D9700,
					Quantity:   1,
					Container:  ContainerInventory,
					Position:   0,
				}},
				StorageItems: []TemplateItem{},
			},
		},
	}
	data, err := json.Marshal(tpl)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var got BuildTemplate
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if err := ValidateBuildTemplate(&got); err != nil {
		t.Fatalf("decoded v2 fails validation: %v\njson:\n%s", err, data)
	}
	if got.Version != 2 {
		t.Fatalf("version drift: got %d", got.Version)
	}
	if got.Selection == nil || got.Selection.Profile == nil {
		t.Fatalf("selection.profile lost: %+v", got.Selection)
	}
	if !got.Selection.Profile.Selected("level") || got.Selection.Profile.Selected("name") {
		t.Fatalf("per-field selection drift on profile: %+v", got.Selection.Profile)
	}
	if !got.Selection.Stats.All {
		t.Fatalf("stats boolean shortcut lost: %+v", got.Selection.Stats)
	}
	if got.Sections.Profile == nil || got.Sections.Profile.Level == nil || *got.Sections.Profile.Level != 150 {
		t.Fatalf("profile.level lost: %+v", got.Sections.Profile)
	}
	if got.Sections.Stats == nil || got.Sections.Stats.Intelligence == nil || *got.Sections.Stats.Intelligence != 80 {
		t.Fatalf("stats.intelligence lost: %+v", got.Sections.Stats)
	}
}
