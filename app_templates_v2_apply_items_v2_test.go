package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
	"github.com/oisis/EldenRing-SaveForge/backend/editor"
	"github.com/oisis/EldenRing-SaveForge/backend/templates"
)

// Phase 8D.1 — app-level tests for the v2 sections.items addMissing
// apply path. The fixture helpers (startSessionForFixture,
// freshOverrideFixture, mustMarshalTpl, intPtr, idStandardDagger /
// idSomberGolemBow / idStandardClaymore, findAddedWeapon) live in
// app_templates_v2_apply_inventory_test.go and
// app_templates_weapon_override_test.go — same package.

// templateWithV2Items builds the smallest legal v2 template that
// selects sections.items and carries the supplied entries. Profile /
// stats / inventory.workspace stay absent so tests isolate the items
// path.
func templateWithV2Items(entries []templates.TemplateItemEntryV2) *templates.BuildTemplate {
	return &templates.BuildTemplate{
		Schema:     templates.SchemaKey,
		Version:    2,
		AppVersion: "test",
		CreatedAt:  "2026-06-08T00:00:00Z",
		Selection: &templates.TemplateSelection{
			Items: &templates.SectionSelection{All: true},
		},
		Sections: templates.TemplateSections{
			Items: &templates.ItemsSection{Entries: entries},
		},
	}
}

func u8ptr(v uint8) *uint8 { return &v }

// freshItemsFixture spins up an empty-workspace App with slot 0 marked
// active so the v2 apply path passes its inactive-slot gate. Mirrors
// freshOverrideFixture but adds the ActiveSlots flag the v2 path
// requires (v1 apply skips that check, so freshOverrideFixture works
// for the existing v1 weapon-override tests but not for items apply).
func freshItemsFixture(t *testing.T) (*App, string) {
	t.Helper()
	app := inventoryOrderFixture(nil)
	app.save.ActiveSlots[0] = true
	snap, err := app.StartInventoryEditSession(0)
	if err != nil {
		t.Fatalf("StartInventoryEditSession: %v", err)
	}
	return app, snap.SessionID
}

// entry is a tiny constructor for the common "weapon, standard kind"
// shape used across the addMissing tests.
func entry(entryID string, baseItemID uint32, category, location string, qty uint32, kind string, level *uint8, infusion string, aow *uint32) templates.TemplateItemEntryV2 {
	return templates.TemplateItemEntryV2{
		EntryID:        entryID,
		ItemID:         baseItemID,
		Category:       category,
		Quantity:       qty,
		Location:       location,
		UpgradeKind:    kind,
		UpgradeLevel:   level,
		InfusionName:   infusion,
		AshOfWarItemID: aow,
	}
}

func standardWeaponEntry(entryID string, baseItemID uint32, level uint8) templates.TemplateItemEntryV2 {
	return entry(entryID, baseItemID, templates.ItemCategoryMeleeArmaments, templates.ItemLocationInventory, 1, templates.UpgradeKindStandard, u8ptr(level), "", nil)
}

func somberWeaponEntry(entryID string, baseItemID uint32, level uint8, category string) templates.TemplateItemEntryV2 {
	return entry(entryID, baseItemID, category, templates.ItemLocationInventory, 1, templates.UpgradeKindSomber, u8ptr(level), "", nil)
}

func hasIssue(issues []templates.ImportPreviewIssue, code string) bool {
	for _, i := range issues {
		if i.Code == code {
			return true
		}
	}
	return false
}

func countIssues(issues []templates.ImportPreviewIssue, code string) int {
	n := 0
	for _, i := range issues {
		if i.Code == code {
			n++
		}
	}
	return n
}

// ─── Session gating ────────────────────────────────────────────────────

// Items selection + empty SessionID → IssueCodeInventorySessionRequired,
// no mutation, no committed changes.
func TestApplyBuildTemplateV2_Items_MissingSessionRejected(t *testing.T) {
	app, _ := startSessionForFixture(t)
	tpl := templateWithV2Items([]templates.TemplateItemEntryV2{
		standardWeaponEntry("inv_0000", idStandardDagger, 0),
	})
	jsonText := mustMarshalTpl(t, tpl)
	preSlotLen := len(app.save.Slots[0].Data)

	res, err := app.ApplyBuildTemplateV2ToCharacterJSON(0, jsonText, ApplyTemplateV2Options{})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if res.Applied {
		t.Fatalf("Applied=true, want false")
	}
	if !hasIssue(res.Preview.Errors, templates.IssueCodeInventorySessionRequired) {
		t.Errorf("missing IssueCodeInventorySessionRequired in errors: %+v", res.Preview.Errors)
	}
	if got := len(app.save.Slots[0].Data); got != preSlotLen {
		t.Errorf("slot.Data pre=%d post=%d", preSlotLen, got)
	}
}

// Items selection + bogus session ID → IssueCodeInventorySessionInvalid,
// no mutation.
func TestApplyBuildTemplateV2_Items_BogusSessionRejected(t *testing.T) {
	app, _ := startSessionForFixture(t)
	tpl := templateWithV2Items([]templates.TemplateItemEntryV2{
		standardWeaponEntry("inv_0000", idStandardDagger, 0),
	})
	jsonText := mustMarshalTpl(t, tpl)

	res, err := app.ApplyBuildTemplateV2ToCharacterJSON(0, jsonText, ApplyTemplateV2Options{SessionID: "does-not-exist"})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if res.Applied {
		t.Fatalf("Applied=true, want false")
	}
	if !hasIssue(res.Preview.Errors, templates.IssueCodeInventorySessionInvalid) {
		t.Errorf("missing IssueCodeInventorySessionInvalid in errors: %+v", res.Preview.Errors)
	}
}

// ─── Mode gating ───────────────────────────────────────────────────────

func applyWithItemsMode(t *testing.T, mode string) ApplyTemplateV2Result {
	t.Helper()
	app, sessionID := freshItemsFixture(t)
	tpl := templateWithV2Items([]templates.TemplateItemEntryV2{
		standardWeaponEntry("inv_0000", idStandardDagger, 0),
	})
	tpl.ApplyOptions = &templates.ApplyOptions{
		Items: &templates.ItemApplyOptions{Mode: mode},
	}
	jsonText := mustMarshalTpl(t, tpl)
	res, err := app.ApplyBuildTemplateV2ToCharacterJSON(0, jsonText, ApplyTemplateV2Options{SessionID: sessionID})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	return res
}

func TestApplyBuildTemplateV2_Items_MergeMode_Rejected(t *testing.T) {
	res := applyWithItemsMode(t, templates.ItemApplyModeMerge)
	if res.Applied {
		t.Fatal("Applied=true, want false")
	}
	if !hasIssue(res.Preview.Errors, templates.IssueCodeItemsModeUnsupported) {
		t.Errorf("missing items_mode_unsupported error: %+v", res.Preview.Errors)
	}
}

func TestApplyBuildTemplateV2_Items_UpdateExistingMode_Rejected(t *testing.T) {
	res := applyWithItemsMode(t, templates.ItemApplyModeUpdateExisting)
	if res.Applied {
		t.Fatal("Applied=true, want false")
	}
	if !hasIssue(res.Preview.Errors, templates.IssueCodeItemsModeUnsupported) {
		t.Errorf("missing items_mode_unsupported error: %+v", res.Preview.Errors)
	}
}

func TestApplyBuildTemplateV2_Items_ReplaceMode_Rejected(t *testing.T) {
	res := applyWithItemsMode(t, templates.ItemApplyModeReplace)
	if res.Applied {
		t.Fatal("Applied=true, want false")
	}
	if !hasIssue(res.Preview.Errors, templates.IssueCodeItemsModeUnsupported) {
		t.Errorf("missing items_mode_unsupported error: %+v", res.Preview.Errors)
	}
}

// ─── addMissing happy path ─────────────────────────────────────────────

// addMissing on an empty inventory adds the requested entry. The
// workspace flips to Dirty=true and the result counter reflects the
// inventory addition.
func TestApplyBuildTemplateV2_Items_AddMissing_AddsMissingInventoryItem(t *testing.T) {
	app, sessionID := freshItemsFixture(t)
	tpl := templateWithV2Items([]templates.TemplateItemEntryV2{
		standardWeaponEntry("inv_0000", idStandardDagger, 0),
	})
	jsonText := mustMarshalTpl(t, tpl)

	res, err := app.ApplyBuildTemplateV2ToCharacterJSON(0, jsonText, ApplyTemplateV2Options{SessionID: sessionID})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if !res.Applied {
		t.Fatalf("Applied=false, errors=%+v", res.Preview.Errors)
	}
	if res.InventoryItemsApplied != 1 || res.StorageItemsApplied != 0 {
		t.Errorf("counters inv=%d sto=%d, want 1/0", res.InventoryItemsApplied, res.StorageItemsApplied)
	}
	if !containsString(res.AppliedFields, "items") {
		t.Errorf("AppliedFields missing 'items': %+v", res.AppliedFields)
	}
	if res.Workspace == nil || !res.Workspace.Dirty {
		t.Errorf("Workspace nil or not dirty: %+v", res.Workspace)
	}
	added := findAddedWeapon(t, *res.Workspace, idStandardDagger)
	if added.CurrentUpgrade != 0 {
		t.Errorf("CurrentUpgrade=%d, want 0", added.CurrentUpgrade)
	}
}

// addMissing skips an entry whose live identity tuple is already
// present in the workspace. Counter stays 0; warning is emitted.
func TestApplyBuildTemplateV2_Items_AddMissing_SkipsExistingItem(t *testing.T) {
	app, sessionID := startSessionForFixture(t)
	// testWeapons fixture pre-seeds the inventory with idStandardDagger
	// at upgrade 0, no infusion, no AoW. addMissing must skip a
	// matching template entry.
	tpl := templateWithV2Items([]templates.TemplateItemEntryV2{
		standardWeaponEntry("inv_0000", idStandardDagger, 0),
	})
	jsonText := mustMarshalTpl(t, tpl)

	preWS, err := app.GetInventoryEditSession(sessionID)
	if err != nil {
		t.Fatalf("GetInventoryEditSession: %v", err)
	}
	preCount := len(preWS.InventoryItems)

	res, err := app.ApplyBuildTemplateV2ToCharacterJSON(0, jsonText, ApplyTemplateV2Options{SessionID: sessionID})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if res.Applied {
		t.Fatalf("Applied=true on no-op skip, want false")
	}
	if !hasIssue(res.Preview.Warnings, templates.IssueCodeItemsAlreadyPresent) {
		t.Errorf("missing items_already_present warning: %+v", res.Preview.Warnings)
	}
	postWS, _ := app.GetInventoryEditSession(sessionID)
	if len(postWS.InventoryItems) != preCount {
		t.Errorf("inventory count changed: pre=%d post=%d", preCount, len(postWS.InventoryItems))
	}
}

// Two template entries with the same identity tuple → second one
// collides against the planned-so-far set and is skipped with a
// warning. First add still lands.
func TestApplyBuildTemplateV2_Items_AddMissing_DuplicateInTemplate_SecondSkipped(t *testing.T) {
	app, sessionID := freshItemsFixture(t)
	tpl := templateWithV2Items([]templates.TemplateItemEntryV2{
		standardWeaponEntry("inv_0000", idStandardDagger, 0),
		standardWeaponEntry("inv_0001", idStandardDagger, 0),
	})
	jsonText := mustMarshalTpl(t, tpl)

	res, err := app.ApplyBuildTemplateV2ToCharacterJSON(0, jsonText, ApplyTemplateV2Options{SessionID: sessionID})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if res.InventoryItemsApplied != 1 {
		t.Errorf("InventoryItemsApplied=%d, want 1 (second duplicate must be skipped)", res.InventoryItemsApplied)
	}
	if countIssues(res.Preview.Warnings, templates.IssueCodeItemsAlreadyPresent) != 1 {
		t.Errorf("expected exactly 1 already_present warning, got: %+v", res.Preview.Warnings)
	}
}

// Two entries with same baseItemID but different upgrade levels are
// distinct via the match tuple. Both add successfully.
func TestApplyBuildTemplateV2_Items_AddMissing_DistinctUpgradeLevels_BothAdded(t *testing.T) {
	app, sessionID := freshItemsFixture(t)
	tpl := templateWithV2Items([]templates.TemplateItemEntryV2{
		standardWeaponEntry("inv_0000", idStandardDagger, 0),
		standardWeaponEntry("inv_0001", idStandardDagger, 5),
	})
	jsonText := mustMarshalTpl(t, tpl)

	res, err := app.ApplyBuildTemplateV2ToCharacterJSON(0, jsonText, ApplyTemplateV2Options{SessionID: sessionID})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if !res.Applied {
		t.Fatalf("Applied=false, errors=%+v", res.Preview.Errors)
	}
	if res.InventoryItemsApplied != 2 {
		t.Errorf("InventoryItemsApplied=%d, want 2", res.InventoryItemsApplied)
	}
	upgrades := []int{}
	for _, it := range res.Workspace.InventoryItems {
		if it.BaseItemID == idStandardDagger {
			upgrades = append(upgrades, it.CurrentUpgrade)
		}
	}
	if len(upgrades) != 2 {
		t.Fatalf("found %d daggers, want 2 (upgrades=%v)", len(upgrades), upgrades)
	}
	saw0, saw5 := false, false
	for _, u := range upgrades {
		if u == 0 {
			saw0 = true
		}
		if u == 5 {
			saw5 = true
		}
	}
	if !saw0 || !saw5 {
		t.Errorf("expected +0 and +5 daggers, got %v", upgrades)
	}
}

// ─── Unsupported category ──────────────────────────────────────────────

// A v2 entry whose category is outside editor.SupportedCategories is
// skipped with a warning. Other entries continue.
func TestApplyBuildTemplateV2_Items_AddMissing_UnsupportedCategory_Skipped(t *testing.T) {
	app, sessionID := freshItemsFixture(t)
	// Sorceries is in the v2 schema allowlist but NOT in
	// editor.SupportedCategories — it must be skipped at apply time.
	// The ItemID is intentionally fake; planning skips before AddItem.
	tpl := templateWithV2Items([]templates.TemplateItemEntryV2{
		{
			EntryID:  "inv_0000",
			ItemID:   0x40000001, // fake — never reaches DB
			Category: templates.ItemCategorySorceries,
			Quantity: 1,
			Location: templates.ItemLocationInventory,
		},
		standardWeaponEntry("inv_0001", idStandardDagger, 0),
	})
	jsonText := mustMarshalTpl(t, tpl)

	res, err := app.ApplyBuildTemplateV2ToCharacterJSON(0, jsonText, ApplyTemplateV2Options{SessionID: sessionID})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if !res.Applied {
		t.Fatalf("Applied=false, errors=%+v", res.Preview.Errors)
	}
	if res.InventoryItemsApplied != 1 {
		t.Errorf("InventoryItemsApplied=%d, want 1 (sorceries skipped, dagger added)", res.InventoryItemsApplied)
	}
	if !hasIssue(res.Preview.Warnings, templates.IssueCodeUnsupportedCategory) {
		t.Errorf("missing unsupported_category warning: %+v", res.Preview.Warnings)
	}
}

// ─── Layout sections / template-override warnings ──────────────────────

// Template carries an inventoryLayout section + items section. Layout
// is ignored with a warning; items still apply.
// Phase 8E.1 supersedes the previous "layout is ignored" warning. With
// the reorderOnly default the combo items+inventoryLayout applies BOTH
// — items addMissing runs first, then the layout reorders the workspace
// (including the freshly added entry). No items_layout_ignored warning
// is emitted.
func TestApplyBuildTemplateV2_Items_LayoutSection_AppliedInReorderOnly(t *testing.T) {
	app, sessionID := freshItemsFixture(t)
	tpl := templateWithV2Items([]templates.TemplateItemEntryV2{
		standardWeaponEntry("inv_0000", idStandardDagger, 0),
	})
	tpl.Selection.InventoryLayout = &templates.SectionSelection{All: true}
	tpl.Sections.InventoryLayout = &templates.InventoryLayoutSection{
		Entries: []templates.LayoutEntry{
			{EntryRef: "inv_0000", Position: 0},
		},
	}
	jsonText := mustMarshalTpl(t, tpl)

	res, err := app.ApplyBuildTemplateV2ToCharacterJSON(0, jsonText, ApplyTemplateV2Options{SessionID: sessionID})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if !res.Applied {
		t.Fatalf("Applied=false, errors=%+v", res.Preview.Errors)
	}
	if res.InventoryItemsApplied != 1 {
		t.Errorf("InventoryItemsApplied=%d, want 1", res.InventoryItemsApplied)
	}
	if res.LayoutInventoryEntriesApplied != 1 {
		t.Errorf("LayoutInventoryEntriesApplied=%d, want 1", res.LayoutInventoryEntriesApplied)
	}
	if hasIssue(res.Preview.Warnings, templates.IssueCodeItemsLayoutIgnored) {
		t.Errorf("Phase 8E.1 should not emit items_layout_ignored when layout applies: %+v", res.Preview.Warnings)
	}
	if hasIssue(res.Preview.Warnings, templates.IssueCodeLayoutModeUnsupported) {
		t.Errorf("Phase 8E.1 should not reject reorderOnly default: %+v", res.Preview.Warnings)
	}
}

// Template carries applyOptions.weaponLevelOverride — Phase 8D.1 honours
// only the runtime override, so the template option must trigger a
// warning while items still apply.
func TestApplyBuildTemplateV2_Items_TemplateOverride_IgnoredWithWarning(t *testing.T) {
	app, sessionID := freshItemsFixture(t)
	tpl := templateWithV2Items([]templates.TemplateItemEntryV2{
		standardWeaponEntry("inv_0000", idStandardDagger, 3),
	})
	tpl.ApplyOptions = &templates.ApplyOptions{
		WeaponLevelOverride: &templates.WeaponLevelOverride{
			UseTemplateLevels: false,
			StandardOverride:  templatesU8(25),
		},
	}
	jsonText := mustMarshalTpl(t, tpl)

	res, err := app.ApplyBuildTemplateV2ToCharacterJSON(0, jsonText, ApplyTemplateV2Options{SessionID: sessionID})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if !res.Applied {
		t.Fatalf("Applied=false, errors=%+v", res.Preview.Errors)
	}
	if !hasIssue(res.Preview.Warnings, templates.IssueCodeItemsTemplateOverrideIgnored) {
		t.Errorf("missing items_template_override_ignored warning: %+v", res.Preview.Warnings)
	}
	// Template upgrade=3 honored, override IGNORED → final +3, not +25.
	added := findAddedWeapon(t, *res.Workspace, idStandardDagger)
	if added.CurrentUpgrade != 3 {
		t.Errorf("CurrentUpgrade=%d, want 3 (template override should NOT have applied)", added.CurrentUpgrade)
	}
}

func templatesU8(v uint8) *uint8 { return &v }

// ─── Weapon level override (runtime opts) ──────────────────────────────

// Standard weapon entry + runtime StandardLevel=25 override → final
// upgrade clamped at 25.
func TestApplyBuildTemplateV2_Items_RuntimeStandardOverride_Applied(t *testing.T) {
	app, sessionID := freshItemsFixture(t)
	tpl := templateWithV2Items([]templates.TemplateItemEntryV2{
		standardWeaponEntry("inv_0000", idStandardDagger, 0),
	})
	jsonText := mustMarshalTpl(t, tpl)

	res, err := app.ApplyBuildTemplateV2ToCharacterJSON(0, jsonText, ApplyTemplateV2Options{
		SessionID: sessionID,
		WeaponLevelOverride: &WeaponLevelOverride{
			Enabled:       true,
			StandardLevel: intPtr(25),
		},
	})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if !res.Applied {
		t.Fatalf("Applied=false, errors=%+v", res.Preview.Errors)
	}
	added := findAddedWeapon(t, *res.Workspace, idStandardDagger)
	if added.CurrentUpgrade != 25 {
		t.Errorf("CurrentUpgrade=%d, want 25", added.CurrentUpgrade)
	}
}

// Somber weapon entry + runtime SomberLevel=10 override → final upgrade
// clamped at 10 (somber cap).
func TestApplyBuildTemplateV2_Items_RuntimeSomberOverride_Applied(t *testing.T) {
	app, sessionID := freshItemsFixture(t)
	tpl := templateWithV2Items([]templates.TemplateItemEntryV2{
		somberWeaponEntry("inv_0000", idSomberGolemBow, 0, templates.ItemCategoryRangedAndCatalysts),
	})
	jsonText := mustMarshalTpl(t, tpl)

	res, err := app.ApplyBuildTemplateV2ToCharacterJSON(0, jsonText, ApplyTemplateV2Options{
		SessionID: sessionID,
		WeaponLevelOverride: &WeaponLevelOverride{
			Enabled:     true,
			SomberLevel: intPtr(10),
		},
	})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if !res.Applied {
		t.Fatalf("Applied=false, errors=%+v", res.Preview.Errors)
	}
	added := findAddedWeapon(t, *res.Workspace, idSomberGolemBow)
	if added.CurrentUpgrade != 10 {
		t.Errorf("CurrentUpgrade=%d, want 10", added.CurrentUpgrade)
	}
}

// Invalid runtime override (Enabled=true with both pointers nil) is
// rejected by validateWeaponLevelOverride BEFORE the workspace is
// touched. No mutation, error surfaces from ApplyBuildTemplateV2ToCharacterJSON.
func TestApplyBuildTemplateV2_Items_InvalidOverride_Rejected(t *testing.T) {
	app, sessionID := freshItemsFixture(t)
	tpl := templateWithV2Items([]templates.TemplateItemEntryV2{
		standardWeaponEntry("inv_0000", idStandardDagger, 0),
	})
	jsonText := mustMarshalTpl(t, tpl)
	preSlotLen := len(app.save.Slots[0].Data)

	res, err := app.ApplyBuildTemplateV2ToCharacterJSON(0, jsonText, ApplyTemplateV2Options{
		SessionID:           sessionID,
		WeaponLevelOverride: &WeaponLevelOverride{Enabled: true},
	})
	if err != nil {
		t.Fatalf("apply: unexpected Go error %v", err)
	}
	if res.Applied {
		t.Fatalf("Applied=true on invalid override, want false")
	}
	if !hasIssue(res.Preview.Errors, templates.IssueCodeStructureInvalid) {
		t.Errorf("expected structure_invalid error from validateWeaponLevelOverride: %+v", res.Preview.Errors)
	}
	overrideMentioned := false
	for _, e := range res.Preview.Errors {
		if strings.Contains(strings.ToLower(e.Message), "weaponleveloverride") || strings.Contains(strings.ToLower(e.Message), "weapon level override") {
			overrideMentioned = true
			break
		}
	}
	if !overrideMentioned {
		t.Errorf("no error message mentions weapon level override: %+v", res.Preview.Errors)
	}
	if got := len(app.save.Slots[0].Data); got != preSlotLen {
		t.Errorf("slot.Data pre=%d post=%d (no mutation expected)", preSlotLen, got)
	}
	postWS, _ := app.GetInventoryEditSession(sessionID)
	if postWS.Dirty {
		t.Error("workspace should not be dirty on validation reject")
	}
}

// ─── Capacity preflight ────────────────────────────────────────────────

// Capacity preflight rejects the apply when the planned additions would
// overflow the inventory CommonItemCount cap. Slot stays untouched.
func TestApplyBuildTemplateV2_Items_CapacityExceeded_NoMutation(t *testing.T) {
	app, sessionID := freshItemsFixture(t)

	// Saturate the workspace's unsupported-records counter to the
	// inventory cap so a single planned addition overflows preflight.
	sess := app.editSessions[sessionID]
	sess.Workspace.UnsupportedInventoryRecords = make([]editor.RawInventoryRecord, core.CommonItemCount)

	tpl := templateWithV2Items([]templates.TemplateItemEntryV2{
		standardWeaponEntry("inv_0000", idStandardDagger, 0),
	})
	jsonText := mustMarshalTpl(t, tpl)
	preSlotLen := len(app.save.Slots[0].Data)

	res, err := app.ApplyBuildTemplateV2ToCharacterJSON(0, jsonText, ApplyTemplateV2Options{SessionID: sessionID})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if res.Applied {
		t.Fatalf("Applied=true, want false")
	}
	if !hasIssue(res.Preview.Errors, templates.IssueCodeCapacityExceeded) {
		t.Errorf("missing capacity_exceeded error: %+v", res.Preview.Errors)
	}
	if got := len(app.save.Slots[0].Data); got != preSlotLen {
		t.Errorf("slot.Data pre=%d post=%d", preSlotLen, got)
	}
	postWS, _ := app.GetInventoryEditSession(sessionID)
	if postWS.Dirty {
		t.Error("workspace should not be dirty on capacity reject")
	}
	if len(postWS.InventoryItems) != 0 {
		t.Errorf("InventoryItems = %d, want 0 (no mutation)", len(postWS.InventoryItems))
	}
}

// ─── Rollback on mid-flight failure ────────────────────────────────────

// An unknown-DB item ID passes preflight (category=melee_armaments,
// inside the editable allow-list) but trips editor.AddItem's DB
// resolution check at apply time → executor returns an error → main
// dispatch rolls back. Workspace and slot stay byte-identical to the
// pre-apply state even though a prior valid entry was already added.
func TestApplyBuildTemplateV2_Items_AddItemFailure_RollsBack(t *testing.T) {
	app, sessionID := freshItemsFixture(t)
	preWS, err := app.GetInventoryEditSession(sessionID)
	if err != nil {
		t.Fatalf("GetInventoryEditSession: %v", err)
	}
	preInvLen := len(preWS.InventoryItems)
	preSlotLen := len(app.save.Slots[0].Data)

	tpl := templateWithV2Items([]templates.TemplateItemEntryV2{
		standardWeaponEntry("inv_0000", idStandardDagger, 0),
		standardWeaponEntry("inv_0001", 0x77777777, 0), // unknown DB item
	})
	jsonText := mustMarshalTpl(t, tpl)

	res, err := app.ApplyBuildTemplateV2ToCharacterJSON(0, jsonText, ApplyTemplateV2Options{SessionID: sessionID})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if res.Applied {
		t.Fatalf("Applied=true on mid-flight failure, want false")
	}
	if !hasIssue(res.Preview.Errors, templates.IssueCodeUnsupportedCategory) {
		// buildApplyErrorReport rewrites the rollback error under
		// IssueCodeUnsupportedCategory. Defensive — accept either it or
		// the structure_invalid bucket so an exporter change doesn't
		// silently break this test.
		if !hasIssue(res.Preview.Errors, templates.IssueCodeStructureInvalid) {
			t.Errorf("expected rollback error code in errors: %+v", res.Preview.Errors)
		}
	}
	postWS, _ := app.GetInventoryEditSession(sessionID)
	if len(postWS.InventoryItems) != preInvLen {
		t.Errorf("InventoryItems pre=%d post=%d (rollback failed)", preInvLen, len(postWS.InventoryItems))
	}
	if postWS.Dirty {
		t.Error("workspace must not be dirty after rollback")
	}
	if got := len(app.save.Slots[0].Data); got != preSlotLen {
		t.Errorf("slot.Data pre=%d post=%d", preSlotLen, got)
	}
}

// ─── Storage / both ────────────────────────────────────────────────────

// Storage-only entry adds to storage (not inventory). Counters reflect
// the storage addition.
func TestApplyBuildTemplateV2_Items_AddMissing_StorageOnly_AddsToStorage(t *testing.T) {
	app, sessionID := freshItemsFixture(t)
	tpl := templateWithV2Items([]templates.TemplateItemEntryV2{
		entry("sto_0000", idStandardDagger, templates.ItemCategoryMeleeArmaments, templates.ItemLocationStorage, 1, templates.UpgradeKindStandard, u8ptr(0), "", nil),
	})
	jsonText := mustMarshalTpl(t, tpl)

	res, err := app.ApplyBuildTemplateV2ToCharacterJSON(0, jsonText, ApplyTemplateV2Options{SessionID: sessionID})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if !res.Applied {
		t.Fatalf("Applied=false, errors=%+v", res.Preview.Errors)
	}
	if res.InventoryItemsApplied != 0 || res.StorageItemsApplied != 1 {
		t.Errorf("counters inv=%d sto=%d, want 0/1", res.InventoryItemsApplied, res.StorageItemsApplied)
	}
	if len(res.Workspace.StorageItems) != 1 {
		t.Errorf("StorageItems len=%d, want 1", len(res.Workspace.StorageItems))
	}
	if len(res.Workspace.InventoryItems) != 0 {
		t.Errorf("InventoryItems len=%d, want 0", len(res.Workspace.InventoryItems))
	}
}

// Location="both" splits into two AddItem calls — one per container.
// Both counters tick up by 1.
func TestApplyBuildTemplateV2_Items_AddMissing_LocationBoth_AddsToBothContainers(t *testing.T) {
	app, sessionID := freshItemsFixture(t)
	tpl := templateWithV2Items([]templates.TemplateItemEntryV2{
		entry("any_0000", idStandardDagger, templates.ItemCategoryMeleeArmaments, templates.ItemLocationBoth, 1, templates.UpgradeKindStandard, u8ptr(0), "", nil),
	})
	jsonText := mustMarshalTpl(t, tpl)

	res, err := app.ApplyBuildTemplateV2ToCharacterJSON(0, jsonText, ApplyTemplateV2Options{SessionID: sessionID})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if !res.Applied {
		t.Fatalf("Applied=false, errors=%+v", res.Preview.Errors)
	}
	if res.InventoryItemsApplied != 1 || res.StorageItemsApplied != 1 {
		t.Errorf("counters inv=%d sto=%d, want 1/1", res.InventoryItemsApplied, res.StorageItemsApplied)
	}
	invSeen := false
	for _, it := range res.Workspace.InventoryItems {
		if it.BaseItemID == idStandardDagger {
			invSeen = true
		}
	}
	stoSeen := false
	for _, it := range res.Workspace.StorageItems {
		if it.BaseItemID == idStandardDagger {
			stoSeen = true
		}
	}
	if !invSeen || !stoSeen {
		t.Errorf("dagger missing in one container — inv=%v sto=%v", invSeen, stoSeen)
	}
}

// ─── v1 + v2 mix ───────────────────────────────────────────────────────

// A template that selects both v1 inventory.workspace AND v2 items is
// fail-closed before any mutation.
func TestApplyBuildTemplateV2_Items_V1V2Mix_Rejected(t *testing.T) {
	app, sessionID := freshItemsFixture(t)
	tpl := templateWithV2Items([]templates.TemplateItemEntryV2{
		standardWeaponEntry("inv_0000", idStandardDagger, 0),
	})
	tpl.Selection.InventoryWorkspace = &templates.SectionSelection{All: true}
	tpl.Sections.InventoryWorkspace = &templates.InventoryWorkspaceSection{
		InventoryItems: []templates.TemplateItem{
			{BaseItemID: idStandardClaymore, Name: "Claymore", Quantity: 1, Container: templates.ContainerInventory, Position: 0},
		},
		StorageItems: []templates.TemplateItem{},
	}
	jsonText := mustMarshalTpl(t, tpl)
	preSlotLen := len(app.save.Slots[0].Data)

	res, err := app.ApplyBuildTemplateV2ToCharacterJSON(0, jsonText, ApplyTemplateV2Options{SessionID: sessionID})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if res.Applied {
		t.Fatalf("Applied=true on v1+v2 mix, want false")
	}
	if !hasIssue(res.Preview.Errors, templates.IssueCodeItemsV1V2Mix) {
		t.Errorf("missing items_v1_v2_mix error: %+v", res.Preview.Errors)
	}
	if got := len(app.save.Slots[0].Data); got != preSlotLen {
		t.Errorf("slot.Data pre=%d post=%d", preSlotLen, got)
	}
}

// ─── Round-trip of plan/apply through JSON ─────────────────────────────

// Smoke check: the v2 items template still round-trips through the
// canonical JSON encoding used by ExportBuildTemplateV2JSONFromCharacter.
// Catches accidental drift in the schema or marshalling rules.
func TestApplyBuildTemplateV2_Items_TemplateRoundTripsThroughJSON(t *testing.T) {
	tpl := templateWithV2Items([]templates.TemplateItemEntryV2{
		standardWeaponEntry("inv_0000", idStandardDagger, 3),
	})
	data, err := json.Marshal(tpl)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var back templates.BuildTemplate
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if err := templates.ValidateBuildTemplate(&back); err != nil {
		t.Fatalf("validate post-roundtrip: %v", err)
	}
	if back.Sections.Items == nil || len(back.Sections.Items.Entries) != 1 {
		t.Fatalf("items section missing after round-trip: %+v", back.Sections)
	}
	if back.Sections.Items.Entries[0].ItemID != idStandardDagger {
		t.Errorf("ItemID drift: got 0x%08X, want 0x%08X", back.Sections.Items.Entries[0].ItemID, idStandardDagger)
	}
}
