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
	tpl.Version = SchemaVersion + 1
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
