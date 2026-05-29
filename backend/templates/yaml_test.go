package templates

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// yamlFixtureTemplate is the fully-populated reference template used by
// the YAML round-trip tests. Mirrors the shape of schema_test.go's
// roundtrip fixture so any divergence between the JSON and YAML codecs
// shows up immediately.
func yamlFixtureTemplate() *BuildTemplate {
	aow := uint32(0x80002710)
	return &BuildTemplate{
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
}

func TestYAML_RoundTrip(t *testing.T) {
	tpl := yamlFixtureTemplate()
	data, err := MarshalBuildTemplateYAML(tpl)
	if err != nil {
		t.Fatalf("MarshalBuildTemplateYAML: %v", err)
	}
	got, err := ParseBuildTemplateYAML(data)
	if err != nil {
		t.Fatalf("ParseBuildTemplateYAML: %v\nyaml:\n%s", err, data)
	}
	if !reflect.DeepEqual(tpl, got) {
		t.Fatalf("round-trip lost data\nwant: %+v\ngot:  %+v\nyaml:\n%s", tpl, got, data)
	}
}

// TestYAML_JSONYAMLEquivalence asserts the JSON v1 codec and the YAML
// v1 codec produce semantically equivalent BuildTemplate values. This
// is the cross-format contract: a public YAML payload must carry the
// same information as the corresponding JSON payload.
func TestYAML_JSONYAMLEquivalence(t *testing.T) {
	tpl := yamlFixtureTemplate()

	jsonBytes, err := json.Marshal(tpl)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	fromJSON, err := ParseBuildTemplateJSON(jsonBytes)
	if err != nil {
		t.Fatalf("ParseBuildTemplateJSON: %v", err)
	}
	yamlBytes, err := MarshalBuildTemplateYAML(fromJSON)
	if err != nil {
		t.Fatalf("MarshalBuildTemplateYAML: %v", err)
	}
	fromYAML, err := ParseBuildTemplateYAML(yamlBytes)
	if err != nil {
		t.Fatalf("ParseBuildTemplateYAML: %v\nyaml:\n%s", err, yamlBytes)
	}
	if !reflect.DeepEqual(fromJSON, fromYAML) {
		t.Fatalf("JSON ↔ YAML drift\nfromJSON: %+v\nfromYAML: %+v", fromJSON, fromYAML)
	}
}

func TestYAML_PreservesSchemaAndKeys(t *testing.T) {
	tpl := yamlFixtureTemplate()
	data, err := MarshalBuildTemplateYAML(tpl)
	if err != nil {
		t.Fatalf("MarshalBuildTemplateYAML: %v", err)
	}
	body := string(data)
	mustContain := []string{
		"schema: " + SchemaKey,
		"version: 1",
		"inventory.workspace:",
		"inventoryItems:",
		"storageItems:",
		"baseItemID:",
		"quantity:",
		"container: " + ContainerInventory,
	}
	for _, needle := range mustContain {
		if !strings.Contains(body, needle) {
			t.Errorf("YAML missing expected substring %q\nyaml:\n%s", needle, body)
		}
	}
}

func TestYAML_AoWOmittedWhenNil(t *testing.T) {
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
	data, err := MarshalBuildTemplateYAML(tpl)
	if err != nil {
		t.Fatalf("MarshalBuildTemplateYAML: %v", err)
	}
	if strings.Contains(string(data), "aowItemID") {
		t.Errorf("aowItemID must be absent when nil, got:\n%s", string(data))
	}
}

func TestYAML_RejectsMalformed(t *testing.T) {
	bad := []byte("schema: saveforge.build-template\nversion: [oops")
	if _, err := ParseBuildTemplateYAML(bad); err == nil {
		t.Fatal("expected error for malformed YAML")
	}
}

func TestYAML_RejectsEmpty(t *testing.T) {
	for _, payload := range [][]byte{nil, []byte(""), []byte("   \n\t  ")} {
		if _, err := ParseBuildTemplateYAML(payload); err == nil {
			t.Errorf("expected error for empty payload %q", payload)
		}
	}
}

func TestYAML_RejectsUnknownField(t *testing.T) {
	// Strict decode (KnownFields(true)) must refuse a YAML doc that
	// carries an extra top-level field not declared on BuildTemplate.
	payload := []byte(`schema: saveforge.build-template
version: 1
createdAt: "2026-05-17T12:34:56Z"
totallyUnknown: hello
sections:
  inventory.workspace:
    inventoryItems:
      - baseItemID: 4032256
        quantity: 1
        container: inventory
        position: 0
    storageItems: []
`)
	if _, err := ParseBuildTemplateYAML(payload); err == nil {
		t.Fatal("expected strict decode to reject unknown field")
	}
}

func TestYAML_RejectsUnknownItemField(t *testing.T) {
	// Same protection at the item level — a typo'd item field must
	// fail closed instead of being silently dropped.
	payload := []byte(`schema: saveforge.build-template
version: 1
createdAt: "2026-05-17T12:34:56Z"
sections:
  inventory.workspace:
    inventoryItems:
      - baseItemID: 4032256
        quantity: 1
        container: inventory
        position: 0
        bogusItemField: hello
    storageItems: []
`)
	if _, err := ParseBuildTemplateYAML(payload); err == nil {
		t.Fatal("expected strict decode to reject unknown item-level field")
	}
}

func TestYAML_RejectsWrongSchema(t *testing.T) {
	tpl := yamlFixtureTemplate()
	data, err := MarshalBuildTemplateYAML(tpl)
	if err != nil {
		t.Fatalf("MarshalBuildTemplateYAML: %v", err)
	}
	bad := strings.Replace(string(data), "schema: "+SchemaKey, "schema: not-a-saveforge-template", 1)
	if _, err := ParseBuildTemplateYAML([]byte(bad)); err == nil {
		t.Fatal("expected error for wrong schema")
	}
}

func TestYAML_RejectsUnsupportedVersion(t *testing.T) {
	tpl := yamlFixtureTemplate()
	tpl.Version = SchemaVersion + 1
	// Marshal goes through validator → must reject directly.
	if _, err := MarshalBuildTemplateYAML(tpl); err == nil {
		t.Fatal("expected MarshalBuildTemplateYAML to refuse unsupported version")
	}

	// Also exercise the parse path on a hand-crafted document so we
	// know the reader-side gate is wired.
	payload := []byte(`schema: saveforge.build-template
version: 99
createdAt: "2026-05-17T12:34:56Z"
sections:
  inventory.workspace:
    inventoryItems:
      - baseItemID: 4032256
        quantity: 1
        container: inventory
        position: 0
    storageItems: []
`)
	if _, err := ParseBuildTemplateYAML(payload); err == nil {
		t.Fatal("expected ParseBuildTemplateYAML to refuse unsupported version")
	}
}

// TestYAML_RejectsMultiDocument asserts that a YAML payload carrying a
// well-formed Build Template v1 followed by a second YAML document (any
// content) is refused as a whole. The public template format is
// exactly one document per file; a second document — even empty — is a
// confused-deputy hazard because some YAML consumers would silently
// drop it.
func TestYAML_RejectsMultiDocument(t *testing.T) {
	payload := []byte(`schema: saveforge.build-template
version: 1
createdAt: "2026-05-17T12:34:56Z"
sections:
  inventory.workspace:
    inventoryItems:
      - baseItemID: 4032256
        quantity: 1
        container: inventory
        position: 0
    storageItems: []
---
schema: completely-different
version: 999
malicious: payload
`)
	_, err := ParseBuildTemplateYAML(payload)
	if err == nil {
		t.Fatal("expected error for multi-document YAML payload")
	}
	if !strings.Contains(err.Error(), "multi-document YAML payloads are not supported") {
		t.Errorf("error message must clearly indicate multi-document rejection, got: %v", err)
	}

	// Variant: a second empty document after `---` must also be refused
	// — the rule is "exactly one document", not "the second one must
	// look harmless".
	emptySecond := []byte(`schema: saveforge.build-template
version: 1
createdAt: "2026-05-17T12:34:56Z"
sections:
  inventory.workspace:
    inventoryItems:
      - baseItemID: 4032256
        quantity: 1
        container: inventory
        position: 0
    storageItems: []
---
`)
	if _, err := ParseBuildTemplateYAML(emptySecond); err == nil {
		t.Error("expected error for trailing empty second document")
	}
}

// TestYAML_OmitsForbiddenFields is the YAML twin of
// TestSchemaJSON_OmitsForbiddenFields. The YAML encoder must not leak
// any editor-side handle / session fields.
func TestYAML_OmitsForbiddenFields(t *testing.T) {
	tpl := yamlFixtureTemplate()
	data, err := MarshalBuildTemplateYAML(tpl)
	if err != nil {
		t.Fatalf("MarshalBuildTemplateYAML: %v", err)
	}
	body := string(data)
	forbidden := []string{
		"originalHandle",
		"currentAoWHandle",
		"uid:",
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
			t.Errorf("forbidden field %q leaked into template YAML:\n%s", key, body)
		}
	}
}
