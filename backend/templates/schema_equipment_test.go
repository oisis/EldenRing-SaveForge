package templates

import (
	"encoding/json"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// helper builders for tests
func intPtr(v int) *int          { return &v }
func equipU32Ptr(v uint32) *uint32 { return &v }

// minimal v2 template with an equipment selection + section for round-trip
// and validator tests. Selection defaults to All=true unless overridden.
func equipTpl(sec *EquipmentSection, sel *SectionSelection) *BuildTemplate {
	if sel == nil {
		sel = &SectionSelection{All: true}
	}
	return &BuildTemplate{
		Schema:    SchemaKey,
		Version:   2,
		CreatedAt: "2026-06-01T00:00:00Z",
		Selection: &TemplateSelection{Equipment: sel},
		Sections:  TemplateSections{Equipment: sec},
	}
}

func TestEquipmentSection_JSONRoundTrip(t *testing.T) {
	in := EquipmentSection{
		WeaponRightHand1: &EquipmentItemRef{BaseItemID: 0x100000, Name: "Uchigatana", Upgrade: intPtr(25)},
		ArmorHead:        &EquipmentItemRef{BaseItemID: 0x1010000, Upgrade: nil, InfusionName: "Heavy"},
		Arrows1:          &EquipmentItemRef{BaseItemID: 0x40100050},
		ArmorLegs:        &EquipmentItemRef{BaseItemID: 0, Name: "explicit clear"},
	}
	data, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out EquipmentSection
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.WeaponRightHand1 == nil || out.WeaponRightHand1.BaseItemID != 0x100000 {
		t.Errorf("weaponRightHand1 lost: %+v", out.WeaponRightHand1)
	}
	if out.WeaponRightHand1.Upgrade == nil || *out.WeaponRightHand1.Upgrade != 25 {
		t.Errorf("upgrade pointer lost")
	}
	if out.ArmorHead == nil || out.ArmorHead.Upgrade != nil {
		t.Errorf("nil upgrade should stay nil after round-trip")
	}
	if out.ArmorHead.InfusionName != "Heavy" {
		t.Errorf("infusion lost: %q", out.ArmorHead.InfusionName)
	}
	if out.Arrows1 == nil || out.Arrows1.BaseItemID != 0x40100050 {
		t.Errorf("arrows1 lost: %+v", out.Arrows1)
	}
	if out.ArmorLegs == nil || out.ArmorLegs.BaseItemID != 0 {
		t.Errorf("explicit-clear (baseItemID=0) lost: %+v", out.ArmorLegs)
	}
	if out.WeaponLeftHand1 != nil || out.WeaponLeftHand2 != nil {
		t.Errorf("absent fields should stay nil")
	}
}

func TestEquipmentSection_YAMLRoundTrip(t *testing.T) {
	in := EquipmentSection{
		WeaponLeftHand1: &EquipmentItemRef{BaseItemID: 0x222222, Upgrade: intPtr(10)},
		Bolts2:          &EquipmentItemRef{BaseItemID: 0x40100070},
	}
	data, err := yaml.Marshal(in)
	if err != nil {
		t.Fatalf("yaml marshal: %v", err)
	}
	var out EquipmentSection
	if err := yaml.Unmarshal(data, &out); err != nil {
		t.Fatalf("yaml unmarshal: %v", err)
	}
	if out.WeaponLeftHand1 == nil || out.WeaponLeftHand1.BaseItemID != 0x222222 {
		t.Errorf("weaponLeftHand1 lost: %+v", out.WeaponLeftHand1)
	}
	if out.WeaponLeftHand1.Upgrade == nil || *out.WeaponLeftHand1.Upgrade != 10 {
		t.Errorf("upgrade lost")
	}
	if out.Bolts2 == nil || out.Bolts2.BaseItemID != 0x40100070 {
		t.Errorf("bolts2 lost")
	}
}

func TestEquipmentSelection_All_AcceptsAllSlots(t *testing.T) {
	tpl := equipTpl(&EquipmentSection{WeaponRightHand1: &EquipmentItemRef{BaseItemID: 1}}, &SectionSelection{All: true})
	if err := ValidateBuildTemplate(tpl); err != nil {
		t.Errorf("All=true selection should validate: %v", err)
	}
}

func TestEquipmentSelection_PerFieldAllowlistAcceptsAll(t *testing.T) {
	fields := map[string]bool{}
	for _, k := range EquipmentSlotOrder {
		fields[k] = true
	}
	tpl := equipTpl(&EquipmentSection{WeaponRightHand1: &EquipmentItemRef{BaseItemID: 1}}, &SectionSelection{Fields: fields})
	if err := ValidateBuildTemplate(tpl); err != nil {
		t.Errorf("per-field selection with all canonical keys should validate: %v", err)
	}
	// Phase 7b.1 shipped 14 slots; Phase 7c extends with 5 talisman slots.
	if len(EquipmentSlotOrder) != 19 {
		t.Errorf("EquipmentSlotOrder length: expected 19, got %d", len(EquipmentSlotOrder))
	}
}

func TestEquipmentSelection_RejectsUnknownSlotKey(t *testing.T) {
	tpl := equipTpl(&EquipmentSection{WeaponRightHand1: &EquipmentItemRef{BaseItemID: 1}}, &SectionSelection{Fields: map[string]bool{"equippedSpell1": true}})
	err := ValidateBuildTemplate(tpl)
	if err == nil {
		t.Fatal("expected error on unknown selection key, got nil")
	}
	if !strings.Contains(err.Error(), "equippedSpell1") {
		t.Errorf("error should mention the bad key: %v", err)
	}
}

func TestEquipmentSelection_RejectsGreatRuneSlot(t *testing.T) {
	// EquippedGreatRune is intentionally not in the allowlist; users
	// should not be able to ship it under sections.equipment.
	tpl := equipTpl(&EquipmentSection{WeaponRightHand1: &EquipmentItemRef{BaseItemID: 1}}, &SectionSelection{Fields: map[string]bool{"equippedGreatRune": true}})
	err := ValidateBuildTemplate(tpl)
	if err == nil || !strings.Contains(err.Error(), "equippedGreatRune") {
		t.Errorf("expected unknown-slot error for equippedGreatRune, got %v", err)
	}
}

func TestEquipmentSection_ExplicitClearAccepted(t *testing.T) {
	tpl := equipTpl(&EquipmentSection{
		WeaponRightHand1: &EquipmentItemRef{BaseItemID: 0}, // explicit clear
		ArmorHead:        &EquipmentItemRef{BaseItemID: 0},
	}, nil)
	if err := ValidateBuildTemplate(tpl); err != nil {
		t.Errorf("explicit-clear (baseItemID=0) should validate: %v", err)
	}
}

func TestEquipmentSection_UpgradeNilAccepted(t *testing.T) {
	tpl := equipTpl(&EquipmentSection{WeaponRightHand1: &EquipmentItemRef{BaseItemID: 1}}, nil)
	if err := ValidateBuildTemplate(tpl); err != nil {
		t.Errorf("nil upgrade should validate: %v", err)
	}
}

func TestEquipmentSection_RejectsNegativeUpgrade(t *testing.T) {
	tpl := equipTpl(&EquipmentSection{WeaponRightHand1: &EquipmentItemRef{BaseItemID: 1, Upgrade: intPtr(-1)}}, nil)
	err := ValidateBuildTemplate(tpl)
	if err == nil || !strings.Contains(err.Error(), "negative") {
		t.Errorf("expected negative-upgrade error, got %v", err)
	}
}

func TestEquipmentSection_RejectsUpgradeAboveCap(t *testing.T) {
	tpl := equipTpl(&EquipmentSection{WeaponRightHand1: &EquipmentItemRef{BaseItemID: 1, Upgrade: intPtr(26)}}, nil)
	err := ValidateBuildTemplate(tpl)
	if err == nil || !strings.Contains(err.Error(), "out of range") {
		t.Errorf("expected upgrade-out-of-range error, got %v", err)
	}
}

func TestEquipmentSection_RejectsZeroAoWItemID(t *testing.T) {
	tpl := equipTpl(&EquipmentSection{WeaponRightHand1: &EquipmentItemRef{BaseItemID: 1, AoWItemID: equipU32Ptr(0)}}, nil)
	err := ValidateBuildTemplate(tpl)
	if err == nil || !strings.Contains(err.Error(), "aowItemID=0") {
		t.Errorf("expected aowItemID=0 rejection, got %v", err)
	}
}

func TestEquipmentSelection_SelectedButSectionMissing(t *testing.T) {
	tpl := &BuildTemplate{
		Schema:    SchemaKey,
		Version:   2,
		CreatedAt: "2026-06-01T00:00:00Z",
		Selection: &TemplateSelection{Equipment: &SectionSelection{All: true}},
		Sections:  TemplateSections{}, // Equipment missing
	}
	err := ValidateBuildTemplate(tpl)
	if err == nil || !strings.Contains(err.Error(), "sections.equipment is missing") {
		t.Errorf("expected sections.equipment missing error, got %v", err)
	}
}

func TestEquipmentSelection_HasAnySelected_PicksUpEquipment(t *testing.T) {
	sel := &TemplateSelection{Equipment: &SectionSelection{All: true}}
	if !sel.HasAnySelected() {
		t.Error("HasAnySelected should be true when only Equipment is selected")
	}
}

func TestEquipmentSelectionSummary_PresentInSelectedSections(t *testing.T) {
	tpl := equipTpl(&EquipmentSection{WeaponRightHand1: &EquipmentItemRef{BaseItemID: 0x100000}}, nil)
	rep := PreviewBuildTemplateImport(tpl, ImportPreviewOptions{})
	found := false
	for _, s := range rep.Summary.SelectedSections {
		if s == "equipment" {
			found = true
		}
	}
	if !found {
		t.Errorf("equipment should appear in selectedSections, got %v", rep.Summary.SelectedSections)
	}
}

func TestEquipmentSection_PreviewListsPresentSlots(t *testing.T) {
	tpl := equipTpl(&EquipmentSection{
		WeaponRightHand1: &EquipmentItemRef{BaseItemID: 0x100000},
		ArmorHead:        &EquipmentItemRef{BaseItemID: 0x1010000},
	}, nil)
	rep := PreviewBuildTemplateImport(tpl, ImportPreviewOptions{})
	if len(rep.Summary.EquipmentSlotsPresent) != 2 {
		t.Errorf("expected 2 equipment slots present, got %v", rep.Summary.EquipmentSlotsPresent)
	}
	if rep.Summary.EquipmentSlotsPresent[0] != "weaponRightHand1" {
		t.Errorf("slot order should follow canonical order, got %v", rep.Summary.EquipmentSlotsPresent)
	}
}

func TestEquipmentComboGuard_RejectsEquipmentPlusInventoryWorkspace(t *testing.T) {
	tpl := &BuildTemplate{
		Schema:    SchemaKey,
		Version:   2,
		CreatedAt: "2026-06-01T00:00:00Z",
		Selection: &TemplateSelection{
			Equipment:          &SectionSelection{All: true},
			InventoryWorkspace: &SectionSelection{All: true},
		},
		Sections: TemplateSections{
			Equipment:          &EquipmentSection{WeaponRightHand1: &EquipmentItemRef{BaseItemID: 0x100000}},
			InventoryWorkspace: &InventoryWorkspaceSection{InventoryItems: []TemplateItem{}, StorageItems: []TemplateItem{}},
		},
	}
	rep := PreviewBuildTemplateImport(tpl, ImportPreviewOptions{})
	if rep.OK {
		t.Fatal("preview should reject equipment+inventory.workspace combo, got OK=true")
	}
	found := false
	for _, e := range rep.Errors {
		if e.Code == IssueCodeEquipmentInventoryComboUnsupported {
			found = true
		}
	}
	if !found {
		t.Errorf("expected %s error in preview, got %v", IssueCodeEquipmentInventoryComboUnsupported, rep.Errors)
	}
}

func TestEquipmentSlotRefHelpers(t *testing.T) {
	eq := &EquipmentSection{}
	if EquipmentSlotRef(eq, "weaponRightHand1") != nil {
		t.Error("expected nil for empty slot")
	}
	SetEquipmentSlotRef(eq, "weaponRightHand1", &EquipmentItemRef{BaseItemID: 0x1234})
	got := EquipmentSlotRef(eq, "weaponRightHand1")
	if got == nil || got.BaseItemID != 0x1234 {
		t.Errorf("set/get mismatch: %+v", got)
	}
	// unknown key is a no-op (defensive)
	SetEquipmentSlotRef(eq, "fakeSlot", &EquipmentItemRef{BaseItemID: 0xFFFF})
	if EquipmentSlotRef(eq, "fakeSlot") != nil {
		t.Error("unknown slot should still return nil")
	}
}

// ─── Phase 7c — talisman slot tests ─────────────────────────────────────

func TestEquipmentSection_TalismanJSONRoundTrip(t *testing.T) {
	in := EquipmentSection{
		Talisman1: &EquipmentItemRef{BaseItemID: 0x20100001, Name: "Radagon's Soreseal"},
		Talisman2: &EquipmentItemRef{BaseItemID: 0x20100002},
		Talisman3: &EquipmentItemRef{BaseItemID: 0}, // explicit clear
		Talisman4: nil,
		Talisman5: &EquipmentItemRef{BaseItemID: 0},
	}
	data, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out EquipmentSection
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.Talisman1 == nil || out.Talisman1.BaseItemID != 0x20100001 {
		t.Errorf("talisman1 lost: %+v", out.Talisman1)
	}
	if out.Talisman1.Name != "Radagon's Soreseal" {
		t.Errorf("talisman1 name lost: %q", out.Talisman1.Name)
	}
	if out.Talisman2 == nil || out.Talisman2.BaseItemID != 0x20100002 {
		t.Errorf("talisman2 lost: %+v", out.Talisman2)
	}
	if out.Talisman3 == nil || out.Talisman3.BaseItemID != 0 {
		t.Errorf("talisman3 explicit clear lost: %+v", out.Talisman3)
	}
	if out.Talisman4 != nil {
		t.Errorf("talisman4 should stay nil after round-trip")
	}
	if out.Talisman5 == nil || out.Talisman5.BaseItemID != 0 {
		t.Errorf("talisman5 explicit clear lost: %+v", out.Talisman5)
	}
}

func TestEquipmentSection_TalismanYAMLRoundTrip(t *testing.T) {
	in := EquipmentSection{
		Talisman1: &EquipmentItemRef{BaseItemID: 0x20200005},
		Talisman4: &EquipmentItemRef{BaseItemID: 0x20200008, Name: "Old Lord's Talisman"},
	}
	data, err := yaml.Marshal(in)
	if err != nil {
		t.Fatalf("yaml marshal: %v", err)
	}
	var out EquipmentSection
	if err := yaml.Unmarshal(data, &out); err != nil {
		t.Fatalf("yaml unmarshal: %v", err)
	}
	if out.Talisman1 == nil || out.Talisman1.BaseItemID != 0x20200005 {
		t.Errorf("talisman1 lost: %+v", out.Talisman1)
	}
	if out.Talisman4 == nil || out.Talisman4.BaseItemID != 0x20200008 {
		t.Errorf("talisman4 lost: %+v", out.Talisman4)
	}
	if out.Talisman4.Name != "Old Lord's Talisman" {
		t.Errorf("talisman4 name lost")
	}
}

func TestEquipmentSlotOrder_HasTalismansLast(t *testing.T) {
	want := []string{"talisman1", "talisman2", "talisman3", "talisman4", "talisman5"}
	got := EquipmentSlotOrder[len(EquipmentSlotOrder)-5:]
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("EquipmentSlotOrder tail mismatch at %d: got %q want %q", i, got[i], want[i])
		}
	}
}

func TestEquipmentSelection_AcceptsTalismanFields(t *testing.T) {
	tpl := equipTpl(
		&EquipmentSection{Talisman1: &EquipmentItemRef{BaseItemID: 0x20100001}},
		&SectionSelection{Fields: map[string]bool{
			"talisman1": true,
			"talisman2": true,
			"talisman3": true,
			"talisman4": true,
			"talisman5": true,
		}},
	)
	if err := ValidateBuildTemplate(tpl); err != nil {
		t.Errorf("talisman1..5 per-field selection should validate: %v", err)
	}
}

func TestEquipmentSection_TalismanExplicitClearAccepted(t *testing.T) {
	tpl := equipTpl(&EquipmentSection{
		Talisman1: &EquipmentItemRef{BaseItemID: 0},
		Talisman5: &EquipmentItemRef{BaseItemID: 0},
	}, nil)
	if err := ValidateBuildTemplate(tpl); err != nil {
		t.Errorf("talisman explicit clear should validate: %v", err)
	}
}

func TestEquipmentSection_TalismanSlotRefHelpers(t *testing.T) {
	eq := &EquipmentSection{}
	SetEquipmentSlotRef(eq, "talisman3", &EquipmentItemRef{BaseItemID: 0xAB})
	got := EquipmentSlotRef(eq, "talisman3")
	if got == nil || got.BaseItemID != 0xAB {
		t.Errorf("talisman3 set/get mismatch: %+v", got)
	}
	SetEquipmentSlotRef(eq, "talisman5", &EquipmentItemRef{BaseItemID: 0})
	if EquipmentSlotRef(eq, "talisman5") == nil {
		t.Error("talisman5 explicit clear should round-trip")
	}
	if EquipmentSlotRef(eq, "talisman1") != nil {
		t.Error("talisman1 should remain nil")
	}
}

func TestEquipmentSection_TalismanPreviewListsPresentSlots(t *testing.T) {
	tpl := equipTpl(&EquipmentSection{
		Talisman1: &EquipmentItemRef{BaseItemID: 0x20100001},
		Talisman4: &EquipmentItemRef{BaseItemID: 0x20100004},
	}, nil)
	rep := PreviewBuildTemplateImport(tpl, ImportPreviewOptions{})
	want := []string{"talisman1", "talisman4"}
	if len(rep.Summary.EquipmentSlotsPresent) != len(want) {
		t.Errorf("expected %d slots present, got %v", len(want), rep.Summary.EquipmentSlotsPresent)
	}
	for i, k := range want {
		if rep.Summary.EquipmentSlotsPresent[i] != k {
			t.Errorf("slot[%d]: got %q want %q", i, rep.Summary.EquipmentSlotsPresent[i], k)
		}
	}
}

// TestEquipmentSection_TalismanComboGuard verifies the equipment+inventory.workspace
// hard reject still fires when only talisman slots populate the equipment section.
func TestEquipmentSection_TalismanComboGuard(t *testing.T) {
	tpl := &BuildTemplate{
		Schema:    SchemaKey,
		Version:   2,
		CreatedAt: "2026-06-01T00:00:00Z",
		Selection: &TemplateSelection{
			Equipment:          &SectionSelection{All: true},
			InventoryWorkspace: &SectionSelection{All: true},
		},
		Sections: TemplateSections{
			Equipment:          &EquipmentSection{Talisman1: &EquipmentItemRef{BaseItemID: 0x20100001}},
			InventoryWorkspace: &InventoryWorkspaceSection{InventoryItems: []TemplateItem{}, StorageItems: []TemplateItem{}},
		},
	}
	rep := PreviewBuildTemplateImport(tpl, ImportPreviewOptions{})
	if rep.OK {
		t.Fatal("preview should reject equipment(talisman)+inventory.workspace combo, got OK=true")
	}
	found := false
	for _, e := range rep.Errors {
		if e.Code == IssueCodeEquipmentInventoryComboUnsupported {
			found = true
		}
	}
	if !found {
		t.Errorf("expected %s error, got %v", IssueCodeEquipmentInventoryComboUnsupported, rep.Errors)
	}
}

// TestIssueCodeTalismanSlotPouchInsufficient verifies the stable string.
func TestIssueCodeTalismanSlotPouchInsufficient(t *testing.T) {
	if IssueCodeTalismanSlotPouchInsufficient != "talisman_slot_pouch_insufficient" {
		t.Errorf("unexpected stable string: %q", IssueCodeTalismanSlotPouchInsufficient)
	}
}
