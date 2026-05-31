package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"strings"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
	"github.com/oisis/EldenRing-SaveForge/backend/editor"
	"github.com/oisis/EldenRing-SaveForge/backend/templates"
)

// equipApplyFixture builds an App fixture wired up so the Phase 7b.1
// apply path can exercise resolveEquipmentWritesFromItems + WriteEquipment
// without a real save file. The slot has an empty ChrAsmEquipment
// section and a populated GaMap; tests then patch slot.Inventory.CommonItems
// directly (resolver path tests use resolveEquipmentWritesFromItems
// instead and skip the App layer entirely).
func equipApplyFixture(t *testing.T) *App {
	t.Helper()
	app := applyV2Fixture()
	slot := &app.save.Slots[0]
	slot.Data = make([]byte, core.SlotSize)
	slot.EquipItemsIDOffset = 0x10000
	for i := 0; i < core.ChrAsmFieldCount; i++ {
		binary.LittleEndian.PutUint32(slot.Data[slot.EquipItemsIDOffset+i*4:], 0xFFFFFFFF)
	}
	slot.GaMap = map[uint32]uint32{
		0x80000010: 0x00100020, // weapon
		0x90000020: 0x10100040, // armor
		0xB0000030: 0x40100050, // goods/arrows
	}
	return app
}

func equipReadSlot(slot *core.SaveSlot, idx int) uint32 {
	return binary.LittleEndian.Uint32(slot.Data[slot.EquipItemsIDOffset+idx*4:])
}

// ─── resolveEquipmentWritesFromItems unit tests (pure logic) ───

func TestResolveEquipmentWrites_MatchByBaseID(t *testing.T) {
	items := []editor.EditableItem{
		{BaseItemID: 0x100000, OriginalHandle: 0x80000010, IsWeapon: true, CurrentUpgrade: 25},
	}
	sec := &templates.EquipmentSection{WeaponRightHand1: &templates.EquipmentItemRef{BaseItemID: 0x100000}}
	sel := &templates.SectionSelection{All: true}
	writes, warnings, err := resolveEquipmentWritesFromItems(items, sel, sec)
	if err != nil {
		t.Fatalf("resolver: %v", err)
	}
	if len(warnings) != 0 {
		t.Errorf("expected no warnings, got %v", warnings)
	}
	if len(writes) != 1 || writes[0].Slot != core.EquipSlotRightHandArmament1 || writes[0].Handle != 0x80000010 {
		t.Errorf("unexpected writes: %+v", writes)
	}
}

func TestResolveEquipmentWrites_MissingItemEmitsWarning(t *testing.T) {
	items := []editor.EditableItem{}
	sec := &templates.EquipmentSection{WeaponRightHand1: &templates.EquipmentItemRef{BaseItemID: 0x999999}}
	sel := &templates.SectionSelection{All: true}
	writes, warnings, err := resolveEquipmentWritesFromItems(items, sel, sec)
	if err != nil {
		t.Fatalf("resolver: %v", err)
	}
	if len(writes) != 0 {
		t.Errorf("expected zero writes for missing item, got %+v", writes)
	}
	if len(warnings) != 1 || warnings[0].Code != templates.IssueCodeEquipmentItemNotInInventory {
		t.Errorf("expected equipment_item_not_in_inventory warning, got %v", warnings)
	}
}

func TestResolveEquipmentWrites_AmbiguousMatchFirstWinsPlusWarning(t *testing.T) {
	items := []editor.EditableItem{
		{BaseItemID: 0x100000, OriginalHandle: 0x80000010, IsWeapon: true, CurrentUpgrade: 0},
		{BaseItemID: 0x100000, OriginalHandle: 0x80000011, IsWeapon: true, CurrentUpgrade: 0},
	}
	sec := &templates.EquipmentSection{WeaponRightHand1: &templates.EquipmentItemRef{BaseItemID: 0x100000}}
	sel := &templates.SectionSelection{All: true}
	writes, warnings, err := resolveEquipmentWritesFromItems(items, sel, sec)
	if err != nil {
		t.Fatalf("resolver: %v", err)
	}
	if len(writes) != 1 || writes[0].Handle != 0x80000010 {
		t.Errorf("first match should win, got %+v", writes)
	}
	if len(warnings) != 1 || warnings[0].Code != templates.IssueCodeEquipmentItemAmbiguous {
		t.Errorf("expected equipment_item_ambiguous warning, got %v", warnings)
	}
}

func TestResolveEquipmentWrites_UpgradeDisambiguator(t *testing.T) {
	items := []editor.EditableItem{
		{BaseItemID: 0x100000, OriginalHandle: 0x80000010, IsWeapon: true, CurrentUpgrade: 10},
		{BaseItemID: 0x100000, OriginalHandle: 0x80000011, IsWeapon: true, CurrentUpgrade: 25},
	}
	up := 25
	sec := &templates.EquipmentSection{WeaponRightHand1: &templates.EquipmentItemRef{BaseItemID: 0x100000, Upgrade: &up}}
	sel := &templates.SectionSelection{All: true}
	writes, warnings, err := resolveEquipmentWritesFromItems(items, sel, sec)
	if err != nil {
		t.Fatalf("resolver: %v", err)
	}
	if len(warnings) != 0 {
		t.Errorf("upgrade disambiguator should resolve uniquely, got %v", warnings)
	}
	if len(writes) != 1 || writes[0].Handle != 0x80000011 {
		t.Errorf("upgrade should pick the +25 item, got %+v", writes)
	}
}

func TestResolveEquipmentWrites_InfusionDisambiguator(t *testing.T) {
	items := []editor.EditableItem{
		{BaseItemID: 0x100000, OriginalHandle: 0x80000010, IsWeapon: true, InfusionName: "Heavy"},
		{BaseItemID: 0x100000, OriginalHandle: 0x80000011, IsWeapon: true, InfusionName: "Cold"},
	}
	sec := &templates.EquipmentSection{WeaponRightHand1: &templates.EquipmentItemRef{BaseItemID: 0x100000, InfusionName: "Cold"}}
	sel := &templates.SectionSelection{All: true}
	writes, _, err := resolveEquipmentWritesFromItems(items, sel, sec)
	if err != nil {
		t.Fatalf("resolver: %v", err)
	}
	if len(writes) != 1 || writes[0].Handle != 0x80000011 {
		t.Errorf("infusion should pick the Cold item, got %+v", writes)
	}
}

func TestResolveEquipmentWrites_ExplicitClear(t *testing.T) {
	items := []editor.EditableItem{}
	sec := &templates.EquipmentSection{WeaponRightHand1: &templates.EquipmentItemRef{BaseItemID: 0}}
	sel := &templates.SectionSelection{All: true}
	writes, warnings, err := resolveEquipmentWritesFromItems(items, sel, sec)
	if err != nil {
		t.Fatalf("resolver: %v", err)
	}
	if len(warnings) != 0 {
		t.Errorf("explicit clear should not warn, got %v", warnings)
	}
	if len(writes) != 1 || writes[0].Handle != 0 {
		t.Errorf("expected single clear write, got %+v", writes)
	}
}

func TestResolveEquipmentWrites_PerFieldSelection(t *testing.T) {
	items := []editor.EditableItem{
		{BaseItemID: 0x100000, OriginalHandle: 0x80000010, IsWeapon: true},
		{BaseItemID: 0x10100040, OriginalHandle: 0x90000020, IsArmor: true},
	}
	sec := &templates.EquipmentSection{
		WeaponRightHand1: &templates.EquipmentItemRef{BaseItemID: 0x100000},
		ArmorHead:        &templates.EquipmentItemRef{BaseItemID: 0x10100040},
	}
	// Only WeaponRightHand1 selected — armor must be skipped.
	sel := &templates.SectionSelection{Fields: map[string]bool{"weaponRightHand1": true}}
	writes, _, err := resolveEquipmentWritesFromItems(items, sel, sec)
	if err != nil {
		t.Fatalf("resolver: %v", err)
	}
	if len(writes) != 1 || writes[0].Slot != core.EquipSlotRightHandArmament1 {
		t.Errorf("per-field selection should yield 1 weapon write, got %+v", writes)
	}
}

// ─── full apply path tests via ApplyBuildTemplateV2ToCharacterJSON ───

func TestApplyBuildTemplateV2_EquipmentInventoryComboRejected(t *testing.T) {
	app := equipApplyFixture(t)
	tpl := templates.BuildTemplate{
		Schema:    templates.SchemaKey,
		Version:   2,
		CreatedAt: "2026-06-01T00:00:00Z",
		Selection: &templates.TemplateSelection{
			Equipment:          &templates.SectionSelection{All: true},
			InventoryWorkspace: &templates.SectionSelection{All: true},
		},
		Sections: templates.TemplateSections{
			Equipment:          &templates.EquipmentSection{WeaponRightHand1: &templates.EquipmentItemRef{BaseItemID: 0x100000}},
			InventoryWorkspace: &templates.InventoryWorkspaceSection{InventoryItems: []templates.TemplateItem{}, StorageItems: []templates.TemplateItem{}},
		},
	}
	data, _ := json.Marshal(&tpl)

	// snapshot ChrAsmEquipment to confirm zero side effects
	before := make([]byte, core.ChrAsmEquipmentSize)
	copy(before, app.save.Slots[0].Data[0x10000:0x10000+core.ChrAsmEquipmentSize])

	res, err := app.ApplyBuildTemplateV2ToCharacterJSON(0, string(data), ApplyTemplateV2Options{})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if res.Applied {
		t.Error("combo should reject apply")
	}
	combo := false
	for _, e := range res.Preview.Errors {
		if e.Code == templates.IssueCodeEquipmentInventoryComboUnsupported {
			combo = true
		}
	}
	if !combo {
		t.Errorf("expected equipment_inventory_combo_unsupported error, got %v", res.Preview.Errors)
	}
	after := app.save.Slots[0].Data[0x10000 : 0x10000+core.ChrAsmEquipmentSize]
	if !bytes.Equal(before, after) {
		t.Error("ChrAsmEquipment mutated despite combo rejection")
	}
}

func TestApplyBuildTemplateV2_EquipmentSessionIDIgnoredWhenNoInventory(t *testing.T) {
	app := equipApplyFixture(t)
	// craft a template with equipment only
	tpl := templates.BuildTemplate{
		Schema:    templates.SchemaKey,
		Version:   2,
		CreatedAt: "2026-06-01T00:00:00Z",
		Selection: &templates.TemplateSelection{Equipment: &templates.SectionSelection{All: true}},
		Sections: templates.TemplateSections{
			Equipment: &templates.EquipmentSection{WeaponRightHand1: &templates.EquipmentItemRef{BaseItemID: 0x999999}},
		},
	}
	data, _ := json.Marshal(&tpl)

	res, err := app.ApplyBuildTemplateV2ToCharacterJSON(0, string(data), ApplyTemplateV2Options{SessionID: "ignored-session-id"})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	// The unknown item warning should appear and applied should be false (no
	// successful writes); SessionID was silently ignored so we do NOT see
	// inventory_session_invalid.
	for _, e := range res.Preview.Errors {
		if e.Code == templates.IssueCodeInventorySessionInvalid || e.Code == templates.IssueCodeInventorySessionRequired {
			t.Errorf("session error surfaced on equipment-only apply: %v", e)
		}
	}
}

func TestApplyBuildTemplateV2_EquipmentMissingItemWarningSkipped(t *testing.T) {
	app := equipApplyFixture(t)
	tpl := templates.BuildTemplate{
		Schema:    templates.SchemaKey,
		Version:   2,
		CreatedAt: "2026-06-01T00:00:00Z",
		Selection: &templates.TemplateSelection{Equipment: &templates.SectionSelection{All: true}},
		Sections: templates.TemplateSections{
			Equipment: &templates.EquipmentSection{WeaponRightHand1: &templates.EquipmentItemRef{BaseItemID: 0xABCDEF}},
		},
	}
	data, _ := json.Marshal(&tpl)
	res, err := app.ApplyBuildTemplateV2ToCharacterJSON(0, string(data), ApplyTemplateV2Options{})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	// No actual write → applied should be false (no other sections selected).
	if res.Applied {
		t.Error("applied should be false when only equipment is selected and item is missing")
	}
	found := false
	for _, w := range res.Preview.Warnings {
		if w.Code == templates.IssueCodeEquipmentItemNotInInventory {
			found = true
		}
	}
	if !found {
		t.Errorf("expected equipment_item_not_in_inventory warning, got warnings: %v", res.Preview.Warnings)
	}
	if equipReadSlot(&app.save.Slots[0], 1) != 0xFFFFFFFF {
		t.Error("slot mutated despite skipped item")
	}
	if res.EquipmentSlotsApplied != 0 {
		t.Errorf("EquipmentSlotsApplied should stay 0, got %d", res.EquipmentSlotsApplied)
	}
}

func TestApplyBuildTemplateV2_EquipmentRejectsExtraSections(t *testing.T) {
	// Confirm that the existing scope rejection logic still works for
	// sections outside the Phase 7b.1 allowlist when equipment is also
	// selected. (No new section to test here; the validator catches
	// unknown keys at parse time. This is a smoke test that the apply
	// path's hasEquipment branch did not regress the schema validator's
	// behaviour.)
	app := equipApplyFixture(t)
	const badJSON = `{
		"schema": "saveforge.build-template",
		"version": 2,
		"createdAt": "2026-06-01T00:00:00Z",
		"selection": {"equipment": {"talisman1": true}},
		"sections": {"equipment": {}}
	}`
	res, err := app.ApplyBuildTemplateV2ToCharacterJSON(0, badJSON, ApplyTemplateV2Options{})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if res.Applied {
		t.Error("apply should reject unknown slot in selection")
	}
	if !strings.Contains(strings.ToLower(stringifyPreviewErrors(res.Preview)), "talisman1") {
		t.Errorf("expected error to mention talisman1, got %v", res.Preview.Errors)
	}
}

func stringifyPreviewErrors(rep templates.ImportPreviewReport) string {
	var parts []string
	for _, e := range rep.Errors {
		parts = append(parts, e.Code+" "+e.Message)
	}
	return strings.Join(parts, " | ")
}
