package templates

import (
	"encoding/json"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// spellTpl wraps a SpellsSection in a minimal valid v2 BuildTemplate so
// validators can run end-to-end. Mirrors equipTpl in
// schema_equipment_test.go.
func spellTpl(sec *SpellsSection, sel *SectionSelection) *BuildTemplate {
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

// ─── struct / serialization ─────────────────────────────────────────────

func TestSpellsSection_JSONRoundTrip(t *testing.T) {
	in := SpellsSection{
		Spell1:  &SpellSlotRef{BaseItemID: 0x40001770, Name: "Catch Flame"},
		Spell5:  &SpellSlotRef{BaseItemID: 0x40000FA0, Name: "Glintstone Pebble"},
		Spell14: &SpellSlotRef{BaseItemID: 0, Name: "explicit clear"},
	}

	data, err := json.Marshal(&in)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	// Slots 2/3/4/6..13 are nil — must be omitted entirely.
	if strings.Contains(string(data), `"spell2"`) {
		t.Errorf("nil slot spell2 should be omitted, got %s", string(data))
	}
	if !strings.Contains(string(data), `"spell1"`) ||
		!strings.Contains(string(data), `"spell5"`) ||
		!strings.Contains(string(data), `"spell14"`) {
		t.Errorf("populated slots missing from JSON: %s", string(data))
	}

	var out SpellsSection
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if out.Spell1 == nil || out.Spell1.BaseItemID != 0x40001770 || out.Spell1.Name != "Catch Flame" {
		t.Errorf("spell1 lost: %+v", out.Spell1)
	}
	if out.Spell5 == nil || out.Spell5.BaseItemID != 0x40000FA0 {
		t.Errorf("spell5 lost: %+v", out.Spell5)
	}
	if out.Spell14 == nil || out.Spell14.BaseItemID != 0 {
		t.Errorf("spell14 (explicit clear) lost: %+v", out.Spell14)
	}
	if out.Spell2 != nil {
		t.Errorf("spell2 should round-trip as nil, got %+v", out.Spell2)
	}
}

func TestSpellsSection_YAMLRoundTrip(t *testing.T) {
	in := SpellsSection{
		Spell1: &SpellSlotRef{BaseItemID: 0x40001770, Name: "Catch Flame"},
		Spell7: &SpellSlotRef{BaseItemID: 0x40000FA0},
	}
	data, err := yaml.Marshal(&in)
	if err != nil {
		t.Fatalf("yaml.Marshal: %v", err)
	}

	var out SpellsSection
	if err := yaml.Unmarshal(data, &out); err != nil {
		t.Fatalf("yaml.Unmarshal: %v", err)
	}
	if out.Spell1 == nil || out.Spell1.BaseItemID != 0x40001770 || out.Spell1.Name != "Catch Flame" {
		t.Errorf("spell1 lost: %+v", out.Spell1)
	}
	if out.Spell7 == nil || out.Spell7.BaseItemID != 0x40000FA0 {
		t.Errorf("spell7 lost: %+v", out.Spell7)
	}
}

// ─── backward compatibility ─────────────────────────────────────────────

// TestValidate_AcceptsV2_NoSpells re-asserts that a v2 template without
// any spells section continues to validate. Existing v2 templates must
// remain valid after the Phase 7d.1 schema additions.
func TestValidate_AcceptsV2_NoSpells(t *testing.T) {
	for _, which := range []string{"profile", "stats"} {
		t.Run(which, func(t *testing.T) {
			tpl := minimalValidV2(which)
			if err := ValidateBuildTemplate(tpl); err != nil {
				t.Errorf("v2 %s-only must remain valid without spells: %v", which, err)
			}
			// And the Spells fields must default to nil — i.e. no surprise
			// section materialization.
			if tpl.Selection.Spells != nil {
				t.Errorf("Selection.Spells should default to nil, got %+v", tpl.Selection.Spells)
			}
			if tpl.Sections.Spells != nil {
				t.Errorf("Sections.Spells should default to nil, got %+v", tpl.Sections.Spells)
			}
		})
	}
}

// ─── HasAny ─────────────────────────────────────────────────────────────

func TestSpellsSelection_HasAny_Nil(t *testing.T) {
	var sel *SectionSelection
	if sel.HasAny() {
		t.Error("nil SectionSelection should report HasAny()==false")
	}
}

func TestSpellsSelection_HasAny_EmptyZeroValue(t *testing.T) {
	sel := &SectionSelection{}
	if sel.HasAny() {
		t.Error("zero-value SectionSelection should report HasAny()==false")
	}
}

func TestSpellsSelection_HasAny_All(t *testing.T) {
	sel := &SectionSelection{All: true}
	if !sel.HasAny() {
		t.Error("All=true should report HasAny()==true")
	}
}

func TestSpellsSelection_HasAny_PerField(t *testing.T) {
	sel := &SectionSelection{Fields: map[string]bool{"spell1": true}}
	if !sel.HasAny() {
		t.Error("Fields with spell1=true should report HasAny()==true")
	}
}

func TestHasAnySelected_PicksUpSpells(t *testing.T) {
	t.Run("All shortcut", func(t *testing.T) {
		ts := &TemplateSelection{Spells: &SectionSelection{All: true}}
		if !ts.HasAnySelected() {
			t.Error("TemplateSelection with Spells.All=true must report HasAnySelected()==true")
		}
	})
	t.Run("per-field", func(t *testing.T) {
		ts := &TemplateSelection{Spells: &SectionSelection{Fields: map[string]bool{"spell14": true}}}
		if !ts.HasAnySelected() {
			t.Error("TemplateSelection with Spells.Fields[spell14]=true must report HasAnySelected()==true")
		}
	})
	t.Run("nil spells does not flip selection", func(t *testing.T) {
		ts := &TemplateSelection{}
		if ts.HasAnySelected() {
			t.Error("empty TemplateSelection must not report HasAnySelected()==true")
		}
	})
}

// ─── happy-path validation ──────────────────────────────────────────────

func TestValidate_AcceptsV2_SpellsBooleanShortcut(t *testing.T) {
	tpl := spellTpl(
		&SpellsSection{Spell1: &SpellSlotRef{BaseItemID: 0x40001770}},
		&SectionSelection{All: true},
	)
	if err := ValidateBuildTemplate(tpl); err != nil {
		t.Errorf("v2 spells with All=true must validate: %v", err)
	}
}

func TestValidate_AcceptsV2_SpellsPerFieldSelection(t *testing.T) {
	tpl := spellTpl(
		&SpellsSection{
			Spell1:  &SpellSlotRef{BaseItemID: 0x40001770, Name: "Catch Flame"},
			Spell14: &SpellSlotRef{BaseItemID: 0x40000FA0, Name: "Glintstone Pebble"},
		},
		&SectionSelection{Fields: map[string]bool{"spell1": true, "spell14": true}},
	)
	if err := ValidateBuildTemplate(tpl); err != nil {
		t.Errorf("v2 spells per-field selection must validate: %v", err)
	}
}

func TestValidate_AcceptsV2_SpellsAllFourteenSlots(t *testing.T) {
	sec := &SpellsSection{
		Spell1:  &SpellSlotRef{BaseItemID: 0x40000FA0},
		Spell2:  &SpellSlotRef{BaseItemID: 0x40000FA1},
		Spell3:  &SpellSlotRef{BaseItemID: 0x40000FA2},
		Spell4:  &SpellSlotRef{BaseItemID: 0x40000FA3},
		Spell5:  &SpellSlotRef{BaseItemID: 0x40000FA4},
		Spell6:  &SpellSlotRef{BaseItemID: 0x40000FA5},
		Spell7:  &SpellSlotRef{BaseItemID: 0x40000FA6},
		Spell8:  &SpellSlotRef{BaseItemID: 0x40000FA7},
		Spell9:  &SpellSlotRef{BaseItemID: 0x40000FA8},
		Spell10: &SpellSlotRef{BaseItemID: 0x40001770},
		Spell11: &SpellSlotRef{BaseItemID: 0x40001771},
		Spell12: &SpellSlotRef{BaseItemID: 0x40001772},
		Spell13: &SpellSlotRef{BaseItemID: 0x40001773},
		Spell14: &SpellSlotRef{BaseItemID: 0x40001774},
	}
	tpl := spellTpl(sec, &SectionSelection{All: true})
	if err := ValidateBuildTemplate(tpl); err != nil {
		t.Errorf("all 14 valid spells must validate: %v", err)
	}
}

func TestValidate_AcceptsV2_SpellsExplicitClear(t *testing.T) {
	// BaseItemID == 0 means "clear this slot" and must be accepted
	// without prefix checks. Mirrors the EquipmentSection convention.
	tpl := spellTpl(
		&SpellsSection{Spell3: &SpellSlotRef{BaseItemID: 0, Name: "explicit clear"}},
		&SectionSelection{All: true},
	)
	if err := ValidateBuildTemplate(tpl); err != nil {
		t.Errorf("explicit clear (BaseItemID=0) must validate: %v", err)
	}
}

func TestValidate_AcceptsV2_SpellsAllSlotsNil(t *testing.T) {
	// SpellsSection present but every slot nil — semantically equivalent
	// to "no spell slots targeted." Must validate when selection is also
	// non-empty (boolean shortcut here).
	tpl := spellTpl(&SpellsSection{}, &SectionSelection{All: true})
	if err := ValidateBuildTemplate(tpl); err != nil {
		t.Errorf("empty SpellsSection must validate: %v", err)
	}
}

// ─── rejection paths ────────────────────────────────────────────────────

func TestValidate_RejectsSpellItemID_WrongPrefix(t *testing.T) {
	cases := []struct {
		name string
		id   uint32
	}{
		{"weapon prefix 0x00", 0x00001770},
		{"armor prefix 0x10", 0x10001770},
		{"talisman prefix 0x20", 0x20001770},
		{"incantation legacy 0x60 (must reject — DB uses 0x40)", 0x60001770},
		{"aow prefix 0x80", 0x80001770},
		{"raw MagicParam ID without prefix", 0x00000001},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tpl := spellTpl(
				&SpellsSection{Spell1: &SpellSlotRef{BaseItemID: tc.id}},
				&SectionSelection{All: true},
			)
			err := ValidateBuildTemplate(tpl)
			if err == nil {
				t.Fatalf("expected error for id 0x%08X, got nil", tc.id)
			}
			if !strings.Contains(err.Error(), "wrong prefix") {
				t.Errorf("error %q does not mention 'wrong prefix'", err)
			}
			if !strings.Contains(err.Error(), "spell1") {
				t.Errorf("error %q does not mention the offending slot key", err)
			}
		})
	}
}

func TestValidate_RejectsSpellSelection_UnknownSlotKey(t *testing.T) {
	cases := []string{"spell0", "spell15", "spell99", "spellX", "memory1"}
	for _, key := range cases {
		t.Run(key, func(t *testing.T) {
			tpl := spellTpl(
				&SpellsSection{Spell1: &SpellSlotRef{BaseItemID: 0x40001770}},
				&SectionSelection{Fields: map[string]bool{key: true}},
			)
			err := ValidateBuildTemplate(tpl)
			if err == nil {
				t.Fatalf("expected error for unknown slot key %q, got nil", key)
			}
			if !strings.Contains(err.Error(), "unknown slot") {
				t.Errorf("error %q does not mention 'unknown slot'", err)
			}
			if !strings.Contains(err.Error(), key) {
				t.Errorf("error %q does not mention the offending key %q", err, key)
			}
		})
	}
}

func TestValidate_RejectsSpellsSelection_SelectedButSectionMissing(t *testing.T) {
	tpl := &BuildTemplate{
		Schema:    SchemaKey,
		Version:   2,
		CreatedAt: "2026-06-02T00:00:00Z",
		Selection: &TemplateSelection{Spells: &SectionSelection{All: true}},
		Sections:  TemplateSections{}, // sections.spells deliberately missing
	}
	err := ValidateBuildTemplate(tpl)
	if err == nil {
		t.Fatal("expected error when selection.spells is selected but sections.spells is missing, got nil")
	}
	if !strings.Contains(err.Error(), "selection.spells is selected but sections.spells is missing") {
		t.Errorf("unexpected error message: %v", err)
	}
}

// ─── per-helper direct exercise ─────────────────────────────────────────

func TestValidateSpellSlotRef_DirectPrefixHandling(t *testing.T) {
	if err := validateSpellSlotRef("spell1", &SpellSlotRef{BaseItemID: 0}); err != nil {
		t.Errorf("explicit clear must pass: %v", err)
	}
	if err := validateSpellSlotRef("spell1", &SpellSlotRef{BaseItemID: 0x40001770}); err != nil {
		t.Errorf("0x40-prefixed id must pass: %v", err)
	}
	if err := validateSpellSlotRef("spell1", &SpellSlotRef{BaseItemID: 0x60001770}); err == nil {
		t.Error("0x60-prefixed id must fail (DB uses 0x40 for both sorceries and incantations)")
	}
}

func TestSpellSlotOrder_HasFourteenUniqueKeys(t *testing.T) {
	if len(SpellSlotOrder) != SpellSlotCount {
		t.Fatalf("SpellSlotOrder length = %d, want %d", len(SpellSlotOrder), SpellSlotCount)
	}
	seen := make(map[string]bool, SpellSlotCount)
	for _, key := range SpellSlotOrder {
		if seen[key] {
			t.Errorf("duplicate key in SpellSlotOrder: %q", key)
		}
		if !spellsSelectionFields[key] {
			t.Errorf("SpellSlotOrder key %q missing from spellsSelectionFields allowlist", key)
		}
		seen[key] = true
	}
}

func TestSpellSlotRef_GetterCoversAllSlots(t *testing.T) {
	// Each slot key must round-trip a known sentinel through the
	// private spellSlotRef getter — guards against a forgotten case
	// branch when the slot count is bumped (it shouldn't be, but the
	// guard is cheap).
	s := &SpellsSection{
		Spell1: &SpellSlotRef{BaseItemID: 0x40000001},
		Spell2: &SpellSlotRef{BaseItemID: 0x40000002},
		Spell3: &SpellSlotRef{BaseItemID: 0x40000003},
		Spell4: &SpellSlotRef{BaseItemID: 0x40000004},
		Spell5: &SpellSlotRef{BaseItemID: 0x40000005},
		Spell6: &SpellSlotRef{BaseItemID: 0x40000006},
		Spell7: &SpellSlotRef{BaseItemID: 0x40000007},
		Spell8: &SpellSlotRef{BaseItemID: 0x40000008},
		Spell9: &SpellSlotRef{BaseItemID: 0x40000009},
		Spell10: &SpellSlotRef{BaseItemID: 0x4000000A},
		Spell11: &SpellSlotRef{BaseItemID: 0x4000000B},
		Spell12: &SpellSlotRef{BaseItemID: 0x4000000C},
		Spell13: &SpellSlotRef{BaseItemID: 0x4000000D},
		Spell14: &SpellSlotRef{BaseItemID: 0x4000000E},
	}
	for i, key := range SpellSlotOrder {
		ref := spellSlotRef(s, key)
		want := uint32(0x40000001 + i)
		if ref == nil || ref.BaseItemID != want {
			t.Errorf("spellSlotRef(%q) = %+v, want BaseItemID=0x%08X", key, ref, want)
		}
	}
	if got := spellSlotRef(s, "spell0"); got != nil {
		t.Errorf("spellSlotRef(spell0) should be nil, got %+v", got)
	}
	if got := spellSlotRef(s, "spell15"); got != nil {
		t.Errorf("spellSlotRef(spell15) should be nil, got %+v", got)
	}
	if got := spellSlotRef(nil, "spell1"); got != nil {
		t.Errorf("spellSlotRef(nil, ...) should be nil, got %+v", got)
	}
}
