package templates

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// TestBuildV2Template_EquipmentBooleanShortcut copies the whole equipment
// section verbatim when selection.equipment is All=true.
func TestBuildV2Template_EquipmentBooleanShortcut(t *testing.T) {
	src := &EquipmentSection{
		WeaponRightHand1: &EquipmentItemRef{BaseItemID: 0x100000, Name: "Uchigatana", Upgrade: intPtr(25)},
		ArmorHead:        &EquipmentItemRef{BaseItemID: 0x1010000},
	}
	tpl, err := BuildV2Template(ExportV2Options{
		Now:       time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		Equipment: src,
		Selection: &TemplateSelection{Equipment: &SectionSelection{All: true}},
	})
	if err != nil {
		t.Fatalf("BuildV2Template: %v", err)
	}
	if tpl.Sections.Equipment == nil {
		t.Fatal("sections.equipment should be populated")
	}
	if tpl.Sections.Equipment.WeaponRightHand1 == nil || tpl.Sections.Equipment.WeaponRightHand1.BaseItemID != 0x100000 {
		t.Errorf("weaponRightHand1 lost: %+v", tpl.Sections.Equipment.WeaponRightHand1)
	}
	if tpl.Selection.Equipment == nil || !tpl.Selection.Equipment.All {
		t.Errorf("selection.equipment should be All=true, got %+v", tpl.Selection.Equipment)
	}
}

func TestBuildV2Template_EquipmentPerFieldDropsUnsupplied(t *testing.T) {
	// Source has only WeaponRightHand1 populated, but selection asks for
	// armor too. Per-field normalisation should keep only the slots the
	// source actually supplies.
	src := &EquipmentSection{WeaponRightHand1: &EquipmentItemRef{BaseItemID: 0x100000}}
	tpl, err := BuildV2Template(ExportV2Options{
		Equipment: src,
		Selection: &TemplateSelection{Equipment: &SectionSelection{Fields: map[string]bool{
			"weaponRightHand1": true,
			"armorHead":        true,
		}}},
	})
	if err != nil {
		t.Fatalf("BuildV2Template: %v", err)
	}
	if _, ok := tpl.Selection.Equipment.Fields["armorHead"]; ok {
		t.Errorf("armorHead should be dropped from selection (source had no value)")
	}
	if !tpl.Selection.Equipment.Fields["weaponRightHand1"] {
		t.Errorf("weaponRightHand1 should remain selected")
	}
	if tpl.Sections.Equipment.ArmorHead != nil {
		t.Errorf("armorHead should be nil in section after normalisation")
	}
}

func TestBuildV2Template_EquipmentDeepClonePointer(t *testing.T) {
	up := 15
	aow := uint32(0xDEADBEEF)
	src := &EquipmentSection{WeaponRightHand1: &EquipmentItemRef{BaseItemID: 1, Upgrade: &up, AoWItemID: &aow}}
	tpl, err := BuildV2Template(ExportV2Options{
		Equipment: src,
		Selection: &TemplateSelection{Equipment: &SectionSelection{All: true}},
	})
	if err != nil {
		t.Fatalf("BuildV2Template: %v", err)
	}
	// mutate the source to confirm the template was deep-cloned
	*src.WeaponRightHand1.Upgrade = 99
	*src.WeaponRightHand1.AoWItemID = 0
	if *tpl.Sections.Equipment.WeaponRightHand1.Upgrade != 15 {
		t.Errorf("equipment upgrade aliased the source (got %d, want 15)", *tpl.Sections.Equipment.WeaponRightHand1.Upgrade)
	}
	if *tpl.Sections.Equipment.WeaponRightHand1.AoWItemID != 0xDEADBEEF {
		t.Errorf("equipment AoW aliased the source")
	}
}

func TestBuildV2Template_EquipmentExplicitClearKept(t *testing.T) {
	// baseItemID==0 is a meaningful sentinel; the builder must keep it.
	src := &EquipmentSection{WeaponRightHand1: &EquipmentItemRef{BaseItemID: 0}}
	tpl, err := BuildV2Template(ExportV2Options{
		Equipment: src,
		Selection: &TemplateSelection{Equipment: &SectionSelection{All: true}},
	})
	if err != nil {
		t.Fatalf("BuildV2Template: %v", err)
	}
	if tpl.Sections.Equipment.WeaponRightHand1 == nil || tpl.Sections.Equipment.WeaponRightHand1.BaseItemID != 0 {
		t.Errorf("explicit-clear sentinel lost in build path")
	}
}

func TestBuildV2Template_EquipmentSelectedButNoSourceProvided(t *testing.T) {
	_, err := BuildV2Template(ExportV2Options{
		Selection: &TemplateSelection{Equipment: &SectionSelection{All: true}},
		// Equipment intentionally nil
	})
	if err == nil || !strings.Contains(err.Error(), "selection.equipment is selected but no Equipment source") {
		t.Errorf("expected error about missing equipment source, got %v", err)
	}
}

func TestBuildV2Template_EquipmentJSONShape(t *testing.T) {
	src := &EquipmentSection{
		WeaponRightHand1: &EquipmentItemRef{BaseItemID: 0x100000, Upgrade: intPtr(25)},
		ArmorLegs:        &EquipmentItemRef{BaseItemID: 0}, // explicit clear
	}
	tpl, err := BuildV2Template(ExportV2Options{
		Equipment: src,
		Selection: &TemplateSelection{Equipment: &SectionSelection{All: true}},
	})
	if err != nil {
		t.Fatalf("BuildV2Template: %v", err)
	}
	data, err := json.Marshal(tpl)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	js := string(data)
	if !strings.Contains(js, `"equipment":`) {
		t.Errorf("JSON should contain equipment section: %s", js)
	}
	if !strings.Contains(js, `"weaponRightHand1":`) {
		t.Errorf("JSON should contain weaponRightHand1: %s", js)
	}
	if !strings.Contains(js, `"armorLegs":`) {
		t.Errorf("JSON should contain armorLegs (explicit clear): %s", js)
	}
}

func TestBuildV2Template_EquipmentWithProfile_BothLand(t *testing.T) {
	profile := &ProfileSource{Name: "Tarnished", Level: equipU32Ptr(150)}
	equipment := &EquipmentSection{WeaponRightHand1: &EquipmentItemRef{BaseItemID: 0x100000}}
	tpl, err := BuildV2Template(ExportV2Options{
		Profile:   profile,
		Equipment: equipment,
		Selection: &TemplateSelection{
			Profile:   &SectionSelection{All: true},
			Equipment: &SectionSelection{All: true},
		},
	})
	if err != nil {
		t.Fatalf("BuildV2Template: %v", err)
	}
	if tpl.Sections.Profile == nil || tpl.Sections.Equipment == nil {
		t.Errorf("both sections should land")
	}
	if len(rep_summary_sections(tpl)) != 2 {
		t.Errorf("expected 2 selected sections (profile + equipment), got %v", rep_summary_sections(tpl))
	}
}

// helper — local instead of running PreviewBuildTemplateImport because we
// only need the selectedSections list.
func rep_summary_sections(tpl *BuildTemplate) []string {
	return selectedSectionsForTemplate(tpl)
}

// ─── Phase 7c — talisman export tests ───────────────────────────────────

func TestBuildV2Template_EquipmentTalismansShipped(t *testing.T) {
	src := &EquipmentSection{
		Talisman1: &EquipmentItemRef{BaseItemID: 0x20100001, Name: "Radagon's Soreseal"},
		Talisman2: &EquipmentItemRef{BaseItemID: 0x20100002},
		Talisman5: &EquipmentItemRef{BaseItemID: 0}, // explicit clear
	}
	tpl, err := BuildV2Template(ExportV2Options{
		Equipment: src,
		Selection: &TemplateSelection{Equipment: &SectionSelection{All: true}},
	})
	if err != nil {
		t.Fatalf("BuildV2Template: %v", err)
	}
	if tpl.Sections.Equipment == nil {
		t.Fatal("section nil")
	}
	if tpl.Sections.Equipment.Talisman1 == nil || tpl.Sections.Equipment.Talisman1.BaseItemID != 0x20100001 {
		t.Errorf("talisman1 lost: %+v", tpl.Sections.Equipment.Talisman1)
	}
	if tpl.Sections.Equipment.Talisman2 == nil || tpl.Sections.Equipment.Talisman2.BaseItemID != 0x20100002 {
		t.Errorf("talisman2 lost: %+v", tpl.Sections.Equipment.Talisman2)
	}
	if tpl.Sections.Equipment.Talisman5 == nil || tpl.Sections.Equipment.Talisman5.BaseItemID != 0 {
		t.Errorf("talisman5 explicit clear lost")
	}
	if tpl.Sections.Equipment.Talisman3 != nil || tpl.Sections.Equipment.Talisman4 != nil {
		t.Errorf("absent talisman slots should stay nil")
	}
}

func TestBuildV2Template_EquipmentTalismanPerFieldSelection(t *testing.T) {
	src := &EquipmentSection{
		Talisman1: &EquipmentItemRef{BaseItemID: 0x20100001},
		Talisman4: &EquipmentItemRef{BaseItemID: 0x20100004},
	}
	tpl, err := BuildV2Template(ExportV2Options{
		Equipment: src,
		Selection: &TemplateSelection{Equipment: &SectionSelection{Fields: map[string]bool{
			"talisman1": true,
			"talisman2": true,
			"talisman4": true,
		}}},
	})
	if err != nil {
		t.Fatalf("BuildV2Template: %v", err)
	}
	if _, ok := tpl.Selection.Equipment.Fields["talisman2"]; ok {
		t.Errorf("talisman2 should be dropped (source had no value)")
	}
	if !tpl.Selection.Equipment.Fields["talisman1"] {
		t.Error("talisman1 should remain selected")
	}
	if !tpl.Selection.Equipment.Fields["talisman4"] {
		t.Error("talisman4 should remain selected")
	}
}

func TestBuildV2Template_EquipmentTalismanJSONShape(t *testing.T) {
	src := &EquipmentSection{
		Talisman1: &EquipmentItemRef{BaseItemID: 0x20100001, Name: "Radagon's Soreseal"},
	}
	tpl, err := BuildV2Template(ExportV2Options{
		Equipment: src,
		Selection: &TemplateSelection{Equipment: &SectionSelection{All: true}},
	})
	if err != nil {
		t.Fatalf("BuildV2Template: %v", err)
	}
	data, err := json.Marshal(tpl)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	js := string(data)
	if !strings.Contains(js, `"talisman1":`) {
		t.Errorf("JSON should contain talisman1 key: %s", js)
	}
	if !strings.Contains(js, `"Radagon's Soreseal"`) {
		t.Errorf("JSON should contain talisman name: %s", js)
	}
}
