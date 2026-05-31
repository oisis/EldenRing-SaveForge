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
	writes, warnings, err := resolveEquipmentWritesFromItems(items, sel, sec, 4)
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
	writes, warnings, err := resolveEquipmentWritesFromItems(items, sel, sec, 4)
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
	writes, warnings, err := resolveEquipmentWritesFromItems(items, sel, sec, 4)
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
	writes, warnings, err := resolveEquipmentWritesFromItems(items, sel, sec, 4)
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
	writes, _, err := resolveEquipmentWritesFromItems(items, sel, sec, 4)
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
	writes, warnings, err := resolveEquipmentWritesFromItems(items, sel, sec, 4)
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
	writes, _, err := resolveEquipmentWritesFromItems(items, sel, sec, 4)
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
	// sections outside the allowlist when equipment is also selected.
	// Phase 7c added talisman1..5 to the allowlist; we now use a still-
	// unsupported key (equippedSpell1) to exercise the unknown-slot path.
	app := equipApplyFixture(t)
	const badJSON = `{
		"schema": "saveforge.build-template",
		"version": 2,
		"createdAt": "2026-06-01T00:00:00Z",
		"selection": {"equipment": {"equippedSpell1": true}},
		"sections": {"equipment": {}}
	}`
	res, err := app.ApplyBuildTemplateV2ToCharacterJSON(0, badJSON, ApplyTemplateV2Options{})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if res.Applied {
		t.Error("apply should reject unknown slot in selection")
	}
	if !strings.Contains(strings.ToLower(stringifyPreviewErrors(res.Preview)), "equippedspell1") {
		t.Errorf("expected error to mention equippedSpell1, got %v", res.Preview.Errors)
	}
}

func stringifyPreviewErrors(rep templates.ImportPreviewReport) string {
	var parts []string
	for _, e := range rep.Errors {
		parts = append(parts, e.Code+" "+e.Message)
	}
	return strings.Join(parts, " | ")
}

// ─── Phase 7c — talisman apply tests ────────────────────────────────────

// ─── resolveEquipmentWritesFromItems talisman unit tests ───

func TestResolveEquipmentWrites_TalismanWithPouchOK(t *testing.T) {
	items := []editor.EditableItem{
		{BaseItemID: 0x200003E8, ItemID: 0x200003E8, OriginalHandle: 0xA0000041, IsTalisman: true},
	}
	sec := &templates.EquipmentSection{
		Talisman1: &templates.EquipmentItemRef{BaseItemID: 0x200003E8},
	}
	sel := &templates.SectionSelection{All: true}
	writes, warnings, err := resolveEquipmentWritesFromItems(items, sel, sec, 4)
	if err != nil {
		t.Fatalf("resolver: %v", err)
	}
	if len(warnings) != 0 {
		t.Errorf("expected no warnings, got %v", warnings)
	}
	if len(writes) != 1 || writes[0].Slot != core.EquipSlotTalisman1 || writes[0].Handle != 0xA0000041 {
		t.Errorf("unexpected writes: %+v", writes)
	}
}

func TestResolveEquipmentWrites_TalismanBeyondPouchWarnsAndSkips(t *testing.T) {
	items := []editor.EditableItem{
		{BaseItemID: 0x200003E8, ItemID: 0x200003E8, OriginalHandle: 0xA0000041, IsTalisman: true},
		{BaseItemID: 0x200003F2, ItemID: 0x200003F2, OriginalHandle: 0xA0000042, IsTalisman: true},
		{BaseItemID: 0x200003FC, ItemID: 0x200003FC, OriginalHandle: 0xA0000043, IsTalisman: true},
		{BaseItemID: 0x20000406, ItemID: 0x20000406, OriginalHandle: 0xA0000044, IsTalisman: true},
	}
	sec := &templates.EquipmentSection{
		Talisman1: &templates.EquipmentItemRef{BaseItemID: 0x200003E8},
		Talisman2: &templates.EquipmentItemRef{BaseItemID: 0x200003F2},
		Talisman3: &templates.EquipmentItemRef{BaseItemID: 0x200003FC},
		Talisman4: &templates.EquipmentItemRef{BaseItemID: 0x20000406},
	}
	sel := &templates.SectionSelection{All: true}
	// activeTalismanSlots = 1 (TalismanSlots == 0)
	writes, warnings, err := resolveEquipmentWritesFromItems(items, sel, sec, 1)
	if err != nil {
		t.Fatalf("resolver: %v", err)
	}
	// Only talisman1 should be written.
	if len(writes) != 1 || writes[0].Slot != core.EquipSlotTalisman1 {
		t.Errorf("expected single talisman1 write, got %+v", writes)
	}
	// 3 warnings for talisman2/3/4.
	pouchWarnings := 0
	for _, w := range warnings {
		if w.Code == templates.IssueCodeTalismanSlotPouchInsufficient {
			pouchWarnings++
		}
	}
	if pouchWarnings != 3 {
		t.Errorf("expected 3 pouch warnings, got %d (warnings=%v)", pouchWarnings, warnings)
	}
}

func TestResolveEquipmentWrites_Talisman5AlwaysWarnsWhenPopulated(t *testing.T) {
	items := []editor.EditableItem{
		{BaseItemID: 0x200003E9, ItemID: 0x200003E9, OriginalHandle: 0xA0000045, IsTalisman: true},
	}
	sec := &templates.EquipmentSection{
		Talisman5: &templates.EquipmentItemRef{BaseItemID: 0x200003E9},
	}
	sel := &templates.SectionSelection{All: true}
	// Even at max pouch (activeTalismanSlots = 4), talisman5 still warns.
	writes, warnings, err := resolveEquipmentWritesFromItems(items, sel, sec, 4)
	if err != nil {
		t.Fatalf("resolver: %v", err)
	}
	if len(writes) != 0 {
		t.Errorf("talisman5 should never write a non-empty ref, got %+v", writes)
	}
	if len(warnings) != 1 || warnings[0].Code != templates.IssueCodeTalismanSlotPouchInsufficient {
		t.Errorf("expected single pouch warning for talisman5, got %v", warnings)
	}
}

func TestResolveEquipmentWrites_Talisman5ClearAllowed(t *testing.T) {
	sec := &templates.EquipmentSection{
		Talisman5: &templates.EquipmentItemRef{BaseItemID: 0}, // explicit clear
	}
	sel := &templates.SectionSelection{All: true}
	writes, warnings, err := resolveEquipmentWritesFromItems(nil, sel, sec, 4)
	if err != nil {
		t.Fatalf("resolver: %v", err)
	}
	if len(warnings) != 0 {
		t.Errorf("talisman5 explicit clear should not warn, got %v", warnings)
	}
	if len(writes) != 1 || writes[0].Slot != core.EquipSlotTalisman5 || writes[0].Handle != 0 {
		t.Errorf("expected single clear write for talisman5, got %+v", writes)
	}
}

func TestResolveEquipmentWrites_TalismanClearBeyondPouchAllowed(t *testing.T) {
	// Even when pouch is small (activeTalismanSlots=1), an explicit clear
	// for talisman4 should still emit a write — clearing a slot that is
	// already unreachable is a safe no-op the writer will accept.
	sec := &templates.EquipmentSection{
		Talisman4: &templates.EquipmentItemRef{BaseItemID: 0},
	}
	sel := &templates.SectionSelection{All: true}
	writes, warnings, err := resolveEquipmentWritesFromItems(nil, sel, sec, 1)
	if err != nil {
		t.Fatalf("resolver: %v", err)
	}
	if len(warnings) != 0 {
		t.Errorf("clear-only should not warn, got %v", warnings)
	}
	if len(writes) != 1 || writes[0].Slot != core.EquipSlotTalisman4 || writes[0].Handle != 0 {
		t.Errorf("expected single clear write for talisman4, got %+v", writes)
	}
}

func TestResolveEquipmentWrites_TalismanMissingItemPouchOK(t *testing.T) {
	// Item missing from inventory but slot is within pouch capacity:
	// only equipment_item_not_in_inventory should fire, NOT a pouch
	// warning.
	sec := &templates.EquipmentSection{
		Talisman1: &templates.EquipmentItemRef{BaseItemID: 0xDEADBEEF},
	}
	sel := &templates.SectionSelection{All: true}
	writes, warnings, err := resolveEquipmentWritesFromItems(nil, sel, sec, 4)
	if err != nil {
		t.Fatalf("resolver: %v", err)
	}
	if len(writes) != 0 {
		t.Errorf("expected zero writes, got %+v", writes)
	}
	if len(warnings) != 1 || warnings[0].Code != templates.IssueCodeEquipmentItemNotInInventory {
		t.Errorf("expected equipment_item_not_in_inventory, got %v", warnings)
	}
}

// ─── computeActiveTalismanSlots tests ───

func TestComputeActiveTalismanSlots_UsesSlotValueWhenNoProfile(t *testing.T) {
	slot := &core.SaveSlot{}
	slot.Player.TalismanSlots = 2
	tpl := &templates.BuildTemplate{
		Selection: &templates.TemplateSelection{Equipment: &templates.SectionSelection{All: true}},
	}
	got := computeActiveTalismanSlots(slot, tpl)
	if got != 3 {
		t.Errorf("expected 1 + 2 = 3, got %d", got)
	}
}

func TestComputeActiveTalismanSlots_TemplateOverridesSlot(t *testing.T) {
	slot := &core.SaveSlot{}
	slot.Player.TalismanSlots = 0
	bump := uint8(3)
	tpl := &templates.BuildTemplate{
		Selection: &templates.TemplateSelection{
			Profile:   &templates.SectionSelection{All: true},
			Equipment: &templates.SectionSelection{All: true},
		},
		Sections: templates.TemplateSections{
			Profile: &templates.ProfileSection{TalismanSlots: &bump},
		},
	}
	got := computeActiveTalismanSlots(slot, tpl)
	if got != 4 {
		t.Errorf("expected 1 + 3 = 4, got %d", got)
	}
}

func TestComputeActiveTalismanSlots_TemplateProfileNotSelectedIgnored(t *testing.T) {
	slot := &core.SaveSlot{}
	slot.Player.TalismanSlots = 1
	bump := uint8(3)
	tpl := &templates.BuildTemplate{
		// Profile section present but selection.profile not set →
		// template value must be ignored.
		Selection: &templates.TemplateSelection{Equipment: &templates.SectionSelection{All: true}},
		Sections:  templates.TemplateSections{Profile: &templates.ProfileSection{TalismanSlots: &bump}},
	}
	got := computeActiveTalismanSlots(slot, tpl)
	if got != 2 {
		t.Errorf("expected 1 + 1 = 2 (slot wins), got %d", got)
	}
}

func TestComputeActiveTalismanSlots_ClampsSlotValue(t *testing.T) {
	slot := &core.SaveSlot{}
	slot.Player.TalismanSlots = 7 // garbage; should clamp to 3.
	tpl := &templates.BuildTemplate{}
	got := computeActiveTalismanSlots(slot, tpl)
	if got != 4 {
		t.Errorf("expected clamp to 1 + 3 = 4, got %d", got)
	}
}

// ─── resolver + writer integration (bypasses BuildSnapshot) ────────────
//
// These tests wire resolveEquipmentWritesFromItems directly into
// SaveSlot.WriteEquipment to exercise the full talisman write path —
// resolver → batch → ChrAsmEquipment bytes → hash 8 — without standing
// up the full BuildSnapshot / Inventory parsing pipeline. They mirror
// what ApplyBuildTemplateV2ToCharacterJSON does at the talisman-touching
// step (see app_templates_v2_apply.go: resolveEquipmentWrites +
// WriteEquipment) so a future regression in either layer surfaces here.

// makeTalismanWriteSlot builds a minimal SaveSlot with empty
// ChrAsmEquipment, sized large enough to hold the hash block. Mirrors
// the backend/core/equipment_writer_test.go fixture but local so the
// main package test compiles without importing a sibling _test file.
func makeTalismanWriteSlot() *core.SaveSlot {
	data := make([]byte, core.SlotSize)
	equipOff := 0x10000
	for i := 0; i < core.ChrAsmFieldCount; i++ {
		binary.LittleEndian.PutUint32(data[equipOff+i*4:], 0xFFFFFFFF)
	}
	return &core.SaveSlot{
		Data:               data,
		EquipItemsIDOffset: equipOff,
		GaMap:              map[uint32]uint32{},
	}
}

func talismanReadSlot(s *core.SaveSlot, idx int) uint32 {
	return binary.LittleEndian.Uint32(s.Data[s.EquipItemsIDOffset+idx*4:])
}

func TestTalismanResolveAndWrite_HappyPath4Slots(t *testing.T) {
	slot := makeTalismanWriteSlot()
	// Synthetic talisman editable items with handle prefix 0xA0 and itemID
	// prefix 0x20 — same encoding the runtime produces from GaMap.
	slot.GaMap[0xA0000041] = 0x20100001
	slot.GaMap[0xA0000042] = 0x20100002
	slot.GaMap[0xA0000043] = 0x20100003
	slot.GaMap[0xA0000044] = 0x20100004
	items := []editor.EditableItem{
		{BaseItemID: 0x20100001, ItemID: 0x20100001, OriginalHandle: 0xA0000041, IsTalisman: true},
		{BaseItemID: 0x20100002, ItemID: 0x20100002, OriginalHandle: 0xA0000042, IsTalisman: true},
		{BaseItemID: 0x20100003, ItemID: 0x20100003, OriginalHandle: 0xA0000043, IsTalisman: true},
		{BaseItemID: 0x20100004, ItemID: 0x20100004, OriginalHandle: 0xA0000044, IsTalisman: true},
	}
	sec := &templates.EquipmentSection{
		Talisman1: &templates.EquipmentItemRef{BaseItemID: 0x20100001},
		Talisman2: &templates.EquipmentItemRef{BaseItemID: 0x20100002},
		Talisman3: &templates.EquipmentItemRef{BaseItemID: 0x20100003},
		Talisman4: &templates.EquipmentItemRef{BaseItemID: 0x20100004},
	}
	sel := &templates.SectionSelection{All: true}
	writes, warnings, err := resolveEquipmentWritesFromItems(items, sel, sec, 4)
	if err != nil {
		t.Fatalf("resolver: %v", err)
	}
	if len(warnings) != 0 {
		t.Errorf("expected no warnings, got %v", warnings)
	}
	if len(writes) != 4 {
		t.Fatalf("expected 4 writes, got %d", len(writes))
	}
	if err := slot.WriteEquipment(writes); err != nil {
		t.Fatalf("WriteEquipment: %v", err)
	}
	if talismanReadSlot(slot, 17) != 0x20100001 || talismanReadSlot(slot, 18) != 0x20100002 ||
		talismanReadSlot(slot, 19) != 0x20100003 || talismanReadSlot(slot, 20) != 0x20100004 {
		t.Errorf("talisman bytes mismatch after write: %08X %08X %08X %08X",
			talismanReadSlot(slot, 17), talismanReadSlot(slot, 18), talismanReadSlot(slot, 19), talismanReadSlot(slot, 20))
	}
}

func TestTalismanResolveAndWrite_PouchInsufficientSkipsSlots(t *testing.T) {
	slot := makeTalismanWriteSlot()
	slot.GaMap[0xA0000041] = 0x20100001
	slot.GaMap[0xA0000042] = 0x20100002
	slot.GaMap[0xA0000043] = 0x20100003
	slot.GaMap[0xA0000044] = 0x20100004
	items := []editor.EditableItem{
		{BaseItemID: 0x20100001, ItemID: 0x20100001, OriginalHandle: 0xA0000041, IsTalisman: true},
		{BaseItemID: 0x20100002, ItemID: 0x20100002, OriginalHandle: 0xA0000042, IsTalisman: true},
		{BaseItemID: 0x20100003, ItemID: 0x20100003, OriginalHandle: 0xA0000043, IsTalisman: true},
		{BaseItemID: 0x20100004, ItemID: 0x20100004, OriginalHandle: 0xA0000044, IsTalisman: true},
	}
	sec := &templates.EquipmentSection{
		Talisman1: &templates.EquipmentItemRef{BaseItemID: 0x20100001},
		Talisman2: &templates.EquipmentItemRef{BaseItemID: 0x20100002},
		Talisman3: &templates.EquipmentItemRef{BaseItemID: 0x20100003},
		Talisman4: &templates.EquipmentItemRef{BaseItemID: 0x20100004},
	}
	sel := &templates.SectionSelection{All: true}
	// activeTalismanSlots = 1 simulates TalismanSlots=0 (1 active slot only).
	writes, warnings, err := resolveEquipmentWritesFromItems(items, sel, sec, 1)
	if err != nil {
		t.Fatalf("resolver: %v", err)
	}
	if len(writes) != 1 {
		t.Fatalf("expected only talisman1 write, got %d", len(writes))
	}
	pouch := 0
	for _, w := range warnings {
		if w.Code == templates.IssueCodeTalismanSlotPouchInsufficient {
			pouch++
		}
	}
	if pouch != 3 {
		t.Errorf("expected 3 pouch warnings, got %d (%v)", pouch, warnings)
	}
	if err := slot.WriteEquipment(writes); err != nil {
		t.Fatalf("WriteEquipment: %v", err)
	}
	if talismanReadSlot(slot, 17) != 0x20100001 {
		t.Errorf("talisman1 not written: %08X", talismanReadSlot(slot, 17))
	}
	for _, idx := range []int{18, 19, 20} {
		if talismanReadSlot(slot, idx) != 0xFFFFFFFF {
			t.Errorf("slot %d should remain empty (resolver skipped), got %08X", idx, talismanReadSlot(slot, idx))
		}
	}
}

func TestTalismanResolveAndWrite_Talisman5AlwaysWarnsButClearWorks(t *testing.T) {
	slot := makeTalismanWriteSlot()
	// Pre-seed slot 21 with a non-empty talisman so we can confirm explicit clear.
	binary.LittleEndian.PutUint32(slot.Data[slot.EquipItemsIDOffset+21*4:], 0x20100099)

	sec := &templates.EquipmentSection{
		Talisman5: &templates.EquipmentItemRef{BaseItemID: 0}, // explicit clear
	}
	sel := &templates.SectionSelection{All: true}
	writes, warnings, err := resolveEquipmentWritesFromItems(nil, sel, sec, 4)
	if err != nil {
		t.Fatalf("resolver: %v", err)
	}
	if len(warnings) != 0 {
		t.Errorf("clear-only talisman5 should not warn, got %v", warnings)
	}
	if len(writes) != 1 || writes[0].Slot != core.EquipSlotTalisman5 {
		t.Fatalf("expected single clear write for talisman5, got %+v", writes)
	}
	if err := slot.WriteEquipment(writes); err != nil {
		t.Fatalf("WriteEquipment: %v", err)
	}
	if talismanReadSlot(slot, 21) != 0xFFFFFFFF {
		t.Errorf("talisman5 not cleared: %08X", talismanReadSlot(slot, 21))
	}
}
