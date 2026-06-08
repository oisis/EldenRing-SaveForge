package main

import (
	"encoding/binary"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
	"github.com/oisis/EldenRing-SaveForge/backend/editor"
	"github.com/oisis/EldenRing-SaveForge/backend/templates"
)

// Phase 8E.1 — backend tests for v2 inventoryLayout / storageLayout
// reorderOnly apply. Shares fixtures with the items_v2 / inventory test
// files (same package): inventoryOrderFixture, testWeapons,
// freshItemsFixture, startSessionForFixture, mustMarshalTpl,
// standardWeaponEntry, idStandard*. Layout apply mutates only
// EditableItem.Position values; the slot binary itself is untouched
// until SaveInventoryWorkspaceChanges runs, so these tests assert
// against the workspace snapshot returned in ApplyTemplateV2Result.

// ─── helpers ─────────────────────────────────────────────────────────────

// matchingInvItemsTemplate builds a v2 template that selects items + the
// requested layout sections, with one TemplateItemEntryV2 per testWeapons
// row so an inventoryLayout entryRef can match the live workspace.
//
// All entries are standard kind, level 0 (matches the freshly inserted
// items in inventoryOrderFixture, which have CurrentUpgrade=0 because
// the test handles point at the unfused, +0 base ItemID).
func entriesForTestWeapons() []templates.TemplateItemEntryV2 {
	return []templates.TemplateItemEntryV2{
		standardWeaponEntry("inv_0000", testWeapons[0].itemID, 0), // Dagger
		standardWeaponEntry("inv_0001", testWeapons[1].itemID, 0), // Claymore
		standardWeaponEntry("inv_0002", testWeapons[2].itemID, 0), // Parrying Dagger
		standardWeaponEntry("inv_0003", testWeapons[3].itemID, 0), // Bloodstained Dagger
	}
}

// layoutTpl wraps the smallest legal layout-only template (no items
// selection — only inventoryLayout and/or storageLayout).
func layoutTpl(items []templates.TemplateItemEntryV2, inv *templates.InventoryLayoutSection, sto *templates.StorageLayoutSection) *templates.BuildTemplate {
	tpl := &templates.BuildTemplate{
		Schema:     templates.SchemaKey,
		Version:    2,
		AppVersion: "test",
		CreatedAt:  "2026-06-08T00:00:00Z",
		Selection:  &templates.TemplateSelection{},
		Sections: templates.TemplateSections{
			Items: &templates.ItemsSection{Entries: items},
		},
	}
	if inv != nil {
		tpl.Selection.InventoryLayout = &templates.SectionSelection{All: true}
		tpl.Sections.InventoryLayout = inv
	}
	if sto != nil {
		tpl.Selection.StorageLayout = &templates.SectionSelection{All: true}
		tpl.Sections.StorageLayout = sto
	}
	return tpl
}

// positionsByBaseID returns a map of baseItemID → Position from the
// workspace inventory slice (post-apply read).
func positionsByBaseID(items []editor.EditableItem) map[uint32]int {
	out := map[uint32]int{}
	for _, it := range items {
		out[it.BaseItemID] = it.Position
	}
	return out
}

// ─── 1) session gating ──────────────────────────────────────────────────

func TestApplyBuildTemplateV2_Layout_MissingSession_NoMutation(t *testing.T) {
	app := inventoryOrderFixture(testWeapons)
	app.save.ActiveSlots[0] = true

	tpl := layoutTpl(
		entriesForTestWeapons(),
		&templates.InventoryLayoutSection{
			Entries: []templates.LayoutEntry{
				{EntryRef: "inv_0000", Position: 0},
				{EntryRef: "inv_0001", Position: 1},
			},
		},
		nil,
	)
	jsonText := mustMarshalTpl(t, tpl)
	res, err := app.ApplyBuildTemplateV2ToCharacterJSON(0, jsonText, ApplyTemplateV2Options{})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if res.Applied {
		t.Fatalf("Applied=true, want false (no session)")
	}
	if !hasIssue(res.Preview.Errors, templates.IssueCodeInventorySessionRequired) {
		t.Errorf("missing inventory_session_required: %+v", res.Preview.Errors)
	}
	if res.LayoutInventoryEntriesApplied != 0 {
		t.Errorf("LayoutInventoryEntriesApplied=%d, want 0", res.LayoutInventoryEntriesApplied)
	}
}

func TestApplyBuildTemplateV2_Layout_WrongCharSession_NoMutation(t *testing.T) {
	app, sessionID := startSessionForFixture(t)
	// craft a layout template targeting slot 1 with a session that owns slot 0
	tpl := layoutTpl(
		entriesForTestWeapons(),
		&templates.InventoryLayoutSection{
			Entries: []templates.LayoutEntry{{EntryRef: "inv_0000", Position: 0}},
		},
		nil,
	)
	jsonText := mustMarshalTpl(t, tpl)
	app.save.ActiveSlots[1] = true
	res, err := app.ApplyBuildTemplateV2ToCharacterJSON(1, jsonText, ApplyTemplateV2Options{SessionID: sessionID})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if res.Applied {
		t.Fatalf("Applied=true, want false (wrong char)")
	}
	if !hasIssue(res.Preview.Errors, templates.IssueCodeInventorySessionInvalid) {
		t.Errorf("missing inventory_session_invalid: %+v", res.Preview.Errors)
	}
}

// ─── 2) inventory layout reorders existing items ────────────────────────

func TestApplyBuildTemplateV2_Layout_InventoryReorderOnly_ReordersExistingItems(t *testing.T) {
	app, sessionID := startSessionForFixture(t)
	// Reverse-order template: inv_0003 first, then inv_0002, inv_0001, inv_0000.
	entries := entriesForTestWeapons()
	tpl := layoutTpl(
		entries,
		&templates.InventoryLayoutSection{
			Entries: []templates.LayoutEntry{
				{EntryRef: "inv_0003", Position: 0},
				{EntryRef: "inv_0002", Position: 1},
				{EntryRef: "inv_0001", Position: 2},
				{EntryRef: "inv_0000", Position: 3},
			},
		},
		nil,
	)
	jsonText := mustMarshalTpl(t, tpl)
	res, err := app.ApplyBuildTemplateV2ToCharacterJSON(0, jsonText, ApplyTemplateV2Options{SessionID: sessionID})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if !res.Applied {
		t.Fatalf("Applied=false, errors=%+v", res.Preview.Errors)
	}
	if res.LayoutInventoryEntriesApplied != 4 {
		t.Errorf("LayoutInventoryEntriesApplied=%d, want 4", res.LayoutInventoryEntriesApplied)
	}
	if res.LayoutInventoryEntriesMissing != 0 {
		t.Errorf("LayoutInventoryEntriesMissing=%d, want 0", res.LayoutInventoryEntriesMissing)
	}
	if res.LayoutInventoryExtrasPreserved != 0 {
		t.Errorf("LayoutInventoryExtrasPreserved=%d, want 0", res.LayoutInventoryExtrasPreserved)
	}
	if res.Workspace == nil {
		t.Fatalf("Workspace nil — needsSession=true must echo workspace")
	}
	if !res.Workspace.Dirty {
		t.Errorf("Workspace.Dirty=false, want true after layout reorder")
	}
	got := positionsByBaseID(res.Workspace.InventoryItems)
	wantOrder := map[uint32]int{
		testWeapons[3].itemID: 0, // Bloodstained Dagger
		testWeapons[2].itemID: 1, // Parrying Dagger
		testWeapons[1].itemID: 2, // Claymore
		testWeapons[0].itemID: 3, // Dagger
	}
	for itemID, wantPos := range wantOrder {
		if got[itemID] != wantPos {
			t.Errorf("itemID=0x%08X Position=%d, want %d", itemID, got[itemID], wantPos)
		}
	}
}

// ─── 3) extras preserved + appended after template ──────────────────────

func TestApplyBuildTemplateV2_Layout_ExtrasPreservedAfterTemplate(t *testing.T) {
	app, sessionID := startSessionForFixture(t)
	// Layout mentions only 2 of the 4 weapons; the other 2 should appear
	// after the template-ordered block in their existing order.
	entries := entriesForTestWeapons()
	tpl := layoutTpl(
		entries,
		&templates.InventoryLayoutSection{
			Entries: []templates.LayoutEntry{
				{EntryRef: "inv_0002", Position: 0}, // Parrying Dagger first
				{EntryRef: "inv_0000", Position: 1}, // Dagger second
			},
		},
		nil,
	)
	jsonText := mustMarshalTpl(t, tpl)
	res, err := app.ApplyBuildTemplateV2ToCharacterJSON(0, jsonText, ApplyTemplateV2Options{SessionID: sessionID})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if !res.Applied {
		t.Fatalf("Applied=false, errors=%+v", res.Preview.Errors)
	}
	if res.LayoutInventoryEntriesApplied != 2 {
		t.Errorf("matched=%d, want 2", res.LayoutInventoryEntriesApplied)
	}
	if res.LayoutInventoryExtrasPreserved != 2 {
		t.Errorf("extras=%d, want 2", res.LayoutInventoryExtrasPreserved)
	}
	if !hasIssue(res.Preview.Warnings, templates.IssueCodeLayoutExtraItemsPreserved) {
		t.Errorf("missing layout_extra_items_preserved warning: %+v", res.Preview.Warnings)
	}
	got := positionsByBaseID(res.Workspace.InventoryItems)
	// Expected: 0=Parrying Dagger, 1=Dagger, 2=Claymore (extra), 3=Bloodstained Dagger (extra)
	// Original order before layout was: 0=Dagger, 1=Claymore, 2=Parrying Dagger, 3=Bloodstained Dagger
	// Extras (Claymore at pos 1, BD at pos 3) preserved in that relative order → Claymore=2, BD=3.
	if got[testWeapons[2].itemID] != 0 {
		t.Errorf("Parrying Dagger Position=%d, want 0", got[testWeapons[2].itemID])
	}
	if got[testWeapons[0].itemID] != 1 {
		t.Errorf("Dagger Position=%d, want 1", got[testWeapons[0].itemID])
	}
	if got[testWeapons[1].itemID] != 2 {
		t.Errorf("Claymore Position=%d (extra), want 2", got[testWeapons[1].itemID])
	}
	if got[testWeapons[3].itemID] != 3 {
		t.Errorf("Bloodstained Dagger Position=%d (extra), want 3", got[testWeapons[3].itemID])
	}
}

// ─── 4) missing layout entry → warning + skip ───────────────────────────

func TestApplyBuildTemplateV2_Layout_MissingLiveItem_WarnsAndSkips(t *testing.T) {
	app, sessionID := startSessionForFixture(t)
	// Add a template entry that DOESN'T match any live item (different itemID).
	entries := entriesForTestWeapons()
	ghostItemID := uint32(0x00100000) // bogus, won't match anything in fixture
	entries = append(entries, standardWeaponEntry("inv_ghost", ghostItemID, 0))
	tpl := layoutTpl(
		entries,
		&templates.InventoryLayoutSection{
			Entries: []templates.LayoutEntry{
				{EntryRef: "inv_0000", Position: 0},
				{EntryRef: "inv_ghost", Position: 1}, // no live match
				{EntryRef: "inv_0001", Position: 2},
			},
		},
		nil,
	)
	jsonText := mustMarshalTpl(t, tpl)
	res, err := app.ApplyBuildTemplateV2ToCharacterJSON(0, jsonText, ApplyTemplateV2Options{SessionID: sessionID})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if !res.Applied {
		t.Fatalf("Applied=false, errors=%+v", res.Preview.Errors)
	}
	if res.LayoutInventoryEntriesApplied != 2 {
		t.Errorf("matched=%d, want 2 (ghost not resolved)", res.LayoutInventoryEntriesApplied)
	}
	if res.LayoutInventoryEntriesMissing != 1 {
		t.Errorf("missing=%d, want 1", res.LayoutInventoryEntriesMissing)
	}
	if countIssues(res.Preview.Warnings, templates.IssueCodeLayoutEntryMissing) != 1 {
		t.Errorf("expected 1 layout_entry_missing warning, got %d: %+v",
			countIssues(res.Preview.Warnings, templates.IssueCodeLayoutEntryMissing), res.Preview.Warnings)
	}
}

// ─── 5) ambiguous duplicate same tuple → first-by-Position + warning ────

func TestApplyBuildTemplateV2_Layout_AmbiguousDuplicate_PicksFirstByPosition(t *testing.T) {
	// Fixture with two identical Daggers (same itemID, different handles).
	weapons := []testInvWeapon{
		{0x80800001, 0x000F4240, 2000}, // Dagger A — acqIdx 2000 → Position 0
		{0x80800002, 0x003085E0, 2002}, // Claymore  → Position 1
		{0x80800003, 0x000F4240, 2004}, // Dagger B — same itemID, lower acqIdx → Position 2
	}
	app := inventoryOrderFixture(weapons)
	app.save.ActiveSlots[0] = true
	snap, err := app.StartInventoryEditSession(0)
	if err != nil {
		t.Fatalf("StartInventoryEditSession: %v", err)
	}
	sessionID := snap.SessionID

	tpl := layoutTpl(
		[]templates.TemplateItemEntryV2{
			standardWeaponEntry("inv_0000", 0x000F4240, 0), // matches BOTH Daggers
			standardWeaponEntry("inv_0001", 0x003085E0, 0), // matches Claymore
		},
		&templates.InventoryLayoutSection{
			Entries: []templates.LayoutEntry{
				{EntryRef: "inv_0000", Position: 0}, // ambiguous: 2 candidates
				{EntryRef: "inv_0001", Position: 1},
			},
		},
		nil,
	)
	jsonText := mustMarshalTpl(t, tpl)
	res, err := app.ApplyBuildTemplateV2ToCharacterJSON(0, jsonText, ApplyTemplateV2Options{SessionID: sessionID})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if !res.Applied {
		t.Fatalf("Applied=false, errors=%+v", res.Preview.Errors)
	}
	if res.LayoutInventoryEntriesApplied != 2 {
		t.Errorf("matched=%d, want 2", res.LayoutInventoryEntriesApplied)
	}
	if res.LayoutInventoryExtrasPreserved != 1 {
		t.Errorf("extras=%d, want 1 (the second Dagger)", res.LayoutInventoryExtrasPreserved)
	}
	if !hasIssue(res.Preview.Warnings, templates.IssueCodeLayoutEntryAmbiguous) {
		t.Errorf("missing layout_entry_ambiguous warning: %+v", res.Preview.Warnings)
	}
	// Dagger A (handle 0x80800001) had the lowest pre-apply Position (0) so
	// it must end up matched at new Position 0; Dagger B becomes the extra.
	var posA, posB int = -1, -1
	for _, it := range res.Workspace.InventoryItems {
		switch it.OriginalHandle {
		case 0x80800001:
			posA = it.Position
		case 0x80800003:
			posB = it.Position
		}
	}
	if posA != 0 {
		t.Errorf("Dagger A Position=%d, want 0 (first-by-pos)", posA)
	}
	if posB < 2 {
		t.Errorf("Dagger B Position=%d, want >=2 (extra, appended)", posB)
	}
}

// ─── 6) distinct tuples disambiguate duplicate ItemIDs ──────────────────

func TestApplyBuildTemplateV2_Layout_DistinctTuples_MatchesCorrectInstance(t *testing.T) {
	// Two Daggers, one base (+0) and one with a higher itemID encoding an
	// upgrade level. We use raw +5 standard encoding: itemID = baseID + 5.
	// In inventoryOrderFixture handle→itemID is set via GaMap; we emit the
	// real upgraded itemID so decodeWeaponUpgradeInfusion produces level=5.
	weapons := []testInvWeapon{
		{0x80800001, 0x000F4240, 2000}, // Dagger +0
		{0x80800002, 0x000F4245, 2002}, // Dagger +5 (standard upgrade encoding)
	}
	app := inventoryOrderFixture(weapons)
	app.save.ActiveSlots[0] = true
	snap, err := app.StartInventoryEditSession(0)
	if err != nil {
		t.Fatalf("StartInventoryEditSession: %v", err)
	}
	sessionID := snap.SessionID

	tpl := layoutTpl(
		[]templates.TemplateItemEntryV2{
			standardWeaponEntry("inv_plus0", 0x000F4240, 0),
			standardWeaponEntry("inv_plus5", 0x000F4240, 5), // tuple includes upgrade=5
		},
		&templates.InventoryLayoutSection{
			Entries: []templates.LayoutEntry{
				{EntryRef: "inv_plus5", Position: 0}, // upgraded first
				{EntryRef: "inv_plus0", Position: 1}, // base second
			},
		},
		nil,
	)
	jsonText := mustMarshalTpl(t, tpl)
	res, err := app.ApplyBuildTemplateV2ToCharacterJSON(0, jsonText, ApplyTemplateV2Options{SessionID: sessionID})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if !res.Applied {
		t.Fatalf("Applied=false, errors=%+v", res.Preview.Errors)
	}
	if res.LayoutInventoryEntriesApplied != 2 {
		t.Errorf("matched=%d, want 2", res.LayoutInventoryEntriesApplied)
	}
	if hasIssue(res.Preview.Warnings, templates.IssueCodeLayoutEntryAmbiguous) {
		t.Errorf("unexpected ambiguous warning when tuples differ: %+v", res.Preview.Warnings)
	}
	var posPlus0, posPlus5 int = -1, -1
	for _, it := range res.Workspace.InventoryItems {
		switch it.OriginalHandle {
		case 0x80800001:
			posPlus0 = it.Position
		case 0x80800002:
			posPlus5 = it.Position
		}
	}
	if posPlus5 != 0 {
		t.Errorf("+5 Position=%d, want 0", posPlus5)
	}
	if posPlus0 != 1 {
		t.Errorf("+0 Position=%d, want 1", posPlus0)
	}
}

// ─── 7) sparse positions normalised + warning ───────────────────────────

func TestApplyBuildTemplateV2_Layout_SparsePositions_NormalisedWithWarning(t *testing.T) {
	app, sessionID := startSessionForFixture(t)
	tpl := layoutTpl(
		entriesForTestWeapons(),
		&templates.InventoryLayoutSection{
			Entries: []templates.LayoutEntry{
				{EntryRef: "inv_0000", Position: 0},
				{EntryRef: "inv_0001", Position: 10},
				{EntryRef: "inv_0002", Position: 100},
				{EntryRef: "inv_0003", Position: 999},
			},
		},
		nil,
	)
	jsonText := mustMarshalTpl(t, tpl)
	res, err := app.ApplyBuildTemplateV2ToCharacterJSON(0, jsonText, ApplyTemplateV2Options{SessionID: sessionID})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if !res.Applied {
		t.Fatalf("Applied=false, errors=%+v", res.Preview.Errors)
	}
	if res.LayoutInventoryEntriesApplied != 4 {
		t.Errorf("matched=%d, want 4", res.LayoutInventoryEntriesApplied)
	}
	if !hasIssue(res.Preview.Warnings, templates.IssueCodeLayoutSparseNormalized) {
		t.Errorf("missing layout_sparse_normalized: %+v", res.Preview.Warnings)
	}
	got := positionsByBaseID(res.Workspace.InventoryItems)
	want := map[uint32]int{
		testWeapons[0].itemID: 0,
		testWeapons[1].itemID: 1,
		testWeapons[2].itemID: 2,
		testWeapons[3].itemID: 3,
	}
	for id, p := range want {
		if got[id] != p {
			t.Errorf("itemID=0x%08X Position=%d, want %d (normalised)", id, got[id], p)
		}
	}
}

// ─── 8) storage layout reorderOnly ──────────────────────────────────────

// weaponsAsInvItems converts a []testInvWeapon to the []testInvItem
// shape expected by the shared storageOrderFixture in app_storage_order_test.go.
func weaponsAsInvItems(ws []testInvWeapon) []testInvItem {
	out := make([]testInvItem, len(ws))
	for i, w := range ws {
		out[i] = testInvItem{handle: w.handle, itemID: w.itemID, acqIdx: w.acqIdx}
	}
	return out
}

// storageWeaponFixture wraps storageOrderFixture (shared) and marks the
// slot active so the v2 apply path passes its gate.
func storageWeaponFixture(t *testing.T, stoWeapons []testInvWeapon, invWeapons []testInvWeapon) *App {
	t.Helper()
	app := storageOrderFixture(weaponsAsInvItems(stoWeapons), weaponsAsInvItems(invWeapons))
	app.save.ActiveSlots[0] = true
	return app
}

func TestApplyBuildTemplateV2_Layout_StorageReorderOnly_ReordersStorage(t *testing.T) {
	app := storageWeaponFixture(t, testWeapons, nil)
	snap, err := app.StartInventoryEditSession(0)
	if err != nil {
		t.Fatalf("StartInventoryEditSession: %v", err)
	}
	sessionID := snap.SessionID

	// Entries use storage location so the items section round-trips
	// through validateItemsSection without complaining; layout-only
	// apply (no items selection) won't act on them.
	entries := []templates.TemplateItemEntryV2{
		entry("sto_0000", testWeapons[0].itemID, templates.ItemCategoryMeleeArmaments,
			templates.ItemLocationStorage, 1, templates.UpgradeKindStandard, u8ptr(0), "", nil),
		entry("sto_0001", testWeapons[1].itemID, templates.ItemCategoryMeleeArmaments,
			templates.ItemLocationStorage, 1, templates.UpgradeKindStandard, u8ptr(0), "", nil),
		entry("sto_0002", testWeapons[2].itemID, templates.ItemCategoryMeleeArmaments,
			templates.ItemLocationStorage, 1, templates.UpgradeKindStandard, u8ptr(0), "", nil),
		entry("sto_0003", testWeapons[3].itemID, templates.ItemCategoryMeleeArmaments,
			templates.ItemLocationStorage, 1, templates.UpgradeKindStandard, u8ptr(0), "", nil),
	}
	tpl := layoutTpl(
		entries,
		nil,
		&templates.StorageLayoutSection{
			Entries: []templates.LayoutEntry{
				{EntryRef: "sto_0003", Position: 0},
				{EntryRef: "sto_0002", Position: 1},
				{EntryRef: "sto_0001", Position: 2},
				{EntryRef: "sto_0000", Position: 3},
			},
		},
	)
	jsonText := mustMarshalTpl(t, tpl)
	res, err := app.ApplyBuildTemplateV2ToCharacterJSON(0, jsonText, ApplyTemplateV2Options{SessionID: sessionID})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if !res.Applied {
		t.Fatalf("Applied=false, errors=%+v", res.Preview.Errors)
	}
	if res.LayoutStorageEntriesApplied != 4 {
		t.Errorf("storage matched=%d, want 4", res.LayoutStorageEntriesApplied)
	}
	if res.LayoutInventoryEntriesApplied != 0 {
		t.Errorf("inventory matched=%d, want 0", res.LayoutInventoryEntriesApplied)
	}
	got := positionsByBaseID(res.Workspace.StorageItems)
	want := map[uint32]int{
		testWeapons[3].itemID: 0,
		testWeapons[2].itemID: 1,
		testWeapons[1].itemID: 2,
		testWeapons[0].itemID: 3,
	}
	for id, p := range want {
		if got[id] != p {
			t.Errorf("storage itemID=0x%08X Position=%d, want %d", id, got[id], p)
		}
	}
}

// ─── 9) layout mode unsupported (replace / append) ──────────────────────

func TestApplyBuildTemplateV2_Layout_AppendMode_Unsupported_NoMutation(t *testing.T) {
	app, sessionID := startSessionForFixture(t)
	tpl := layoutTpl(
		entriesForTestWeapons(),
		&templates.InventoryLayoutSection{
			Entries: []templates.LayoutEntry{
				{EntryRef: "inv_0003", Position: 0},
				{EntryRef: "inv_0000", Position: 1},
			},
		},
		nil,
	)
	tpl.ApplyOptions = &templates.ApplyOptions{
		InventoryLayout: &templates.LayoutApplyOptions{Mode: templates.LayoutApplyModeAppend},
	}
	jsonText := mustMarshalTpl(t, tpl)

	preWS, _ := app.GetInventoryEditSession(sessionID)
	prePositions := positionsByBaseID(preWS.InventoryItems)

	res, err := app.ApplyBuildTemplateV2ToCharacterJSON(0, jsonText, ApplyTemplateV2Options{SessionID: sessionID})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if res.Applied {
		// Applied=false because no other section was nominated and layout was rejected.
		t.Logf("Applied=false (expected — only section was rejected layout)")
	}
	if !hasIssue(res.Preview.Warnings, templates.IssueCodeLayoutModeUnsupported) {
		t.Errorf("missing layout_mode_unsupported: %+v", res.Preview.Warnings)
	}
	if res.LayoutInventoryEntriesApplied != 0 {
		t.Errorf("matched=%d, want 0 (rejected)", res.LayoutInventoryEntriesApplied)
	}
	postWS, _ := app.GetInventoryEditSession(sessionID)
	postPositions := positionsByBaseID(postWS.InventoryItems)
	for id, p := range prePositions {
		if postPositions[id] != p {
			t.Errorf("itemID=0x%08X mutated: pre=%d post=%d (mode=append rejected → no mutation)", id, p, postPositions[id])
		}
	}
}

func TestApplyBuildTemplateV2_Layout_ReplaceMode_Unsupported(t *testing.T) {
	app, sessionID := startSessionForFixture(t)
	tpl := layoutTpl(
		entriesForTestWeapons(),
		&templates.InventoryLayoutSection{
			Entries: []templates.LayoutEntry{{EntryRef: "inv_0000", Position: 0}},
		},
		nil,
	)
	tpl.ApplyOptions = &templates.ApplyOptions{
		InventoryLayout: &templates.LayoutApplyOptions{Mode: templates.LayoutApplyModeReplace},
	}
	jsonText := mustMarshalTpl(t, tpl)
	res, err := app.ApplyBuildTemplateV2ToCharacterJSON(0, jsonText, ApplyTemplateV2Options{SessionID: sessionID})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if !hasIssue(res.Preview.Warnings, templates.IssueCodeLayoutModeUnsupported) {
		t.Errorf("missing layout_mode_unsupported for replace: %+v", res.Preview.Warnings)
	}
	if res.LayoutInventoryEntriesApplied != 0 {
		t.Errorf("matched=%d, want 0", res.LayoutInventoryEntriesApplied)
	}
}

// ─── 10) layout mode = ignore → silent skip ─────────────────────────────

func TestApplyBuildTemplateV2_Layout_IgnoreMode_SilentSkip(t *testing.T) {
	app, sessionID := startSessionForFixture(t)
	tpl := layoutTpl(
		entriesForTestWeapons(),
		&templates.InventoryLayoutSection{
			Entries: []templates.LayoutEntry{
				{EntryRef: "inv_0003", Position: 0},
				{EntryRef: "inv_0000", Position: 1},
			},
		},
		nil,
	)
	tpl.ApplyOptions = &templates.ApplyOptions{
		InventoryLayout: &templates.LayoutApplyOptions{Mode: templates.LayoutApplyModeIgnore},
	}
	jsonText := mustMarshalTpl(t, tpl)
	res, err := app.ApplyBuildTemplateV2ToCharacterJSON(0, jsonText, ApplyTemplateV2Options{SessionID: sessionID})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if hasIssue(res.Preview.Warnings, templates.IssueCodeLayoutModeUnsupported) {
		t.Errorf("ignore mode must NOT emit layout_mode_unsupported: %+v", res.Preview.Warnings)
	}
	if res.LayoutInventoryEntriesApplied != 0 {
		t.Errorf("matched=%d, want 0 (ignore)", res.LayoutInventoryEntriesApplied)
	}
}

// ─── 11) inventory + storage in one template ────────────────────────────

func TestApplyBuildTemplateV2_Layout_BothInventoryAndStorage(t *testing.T) {
	// Reuse storage fixture which also gives an empty inventory.
	app := storageWeaponFixture(t, testWeapons, nil)
	// Add a couple of inventory items so inventoryLayout has something to
	// chew on.
	app.save.Slots[0].Inventory.CommonItems = []core.InventoryItem{}
	startOff := app.save.Slots[0].MagicOffset + core.InvStartFromMagic
	invWeapons := []testInvWeapon{
		{0x80808001, testWeapons[0].itemID, 1000},
		{0x80808002, testWeapons[1].itemID, 1002},
	}
	for i, w := range invWeapons {
		off := startOff + i*core.InvRecordLen
		binary.LittleEndian.PutUint32(app.save.Slots[0].Data[off:], w.handle)
		binary.LittleEndian.PutUint32(app.save.Slots[0].Data[off+4:], 1)
		binary.LittleEndian.PutUint32(app.save.Slots[0].Data[off+8:], w.acqIdx)
		app.save.Slots[0].GaMap[w.handle] = w.itemID
		app.save.Slots[0].Inventory.CommonItems = append(app.save.Slots[0].Inventory.CommonItems, core.InventoryItem{
			GaItemHandle: w.handle, Quantity: 1, Index: w.acqIdx,
		})
	}
	snap, err := app.StartInventoryEditSession(0)
	if err != nil {
		t.Fatalf("StartInventoryEditSession: %v", err)
	}
	sessionID := snap.SessionID

	entries := []templates.TemplateItemEntryV2{
		entry("inv_0000", testWeapons[0].itemID, templates.ItemCategoryMeleeArmaments,
			templates.ItemLocationInventory, 1, templates.UpgradeKindStandard, u8ptr(0), "", nil),
		entry("inv_0001", testWeapons[1].itemID, templates.ItemCategoryMeleeArmaments,
			templates.ItemLocationInventory, 1, templates.UpgradeKindStandard, u8ptr(0), "", nil),
		entry("sto_0000", testWeapons[0].itemID, templates.ItemCategoryMeleeArmaments,
			templates.ItemLocationStorage, 1, templates.UpgradeKindStandard, u8ptr(0), "", nil),
		entry("sto_0001", testWeapons[1].itemID, templates.ItemCategoryMeleeArmaments,
			templates.ItemLocationStorage, 1, templates.UpgradeKindStandard, u8ptr(0), "", nil),
		entry("sto_0002", testWeapons[2].itemID, templates.ItemCategoryMeleeArmaments,
			templates.ItemLocationStorage, 1, templates.UpgradeKindStandard, u8ptr(0), "", nil),
		entry("sto_0003", testWeapons[3].itemID, templates.ItemCategoryMeleeArmaments,
			templates.ItemLocationStorage, 1, templates.UpgradeKindStandard, u8ptr(0), "", nil),
	}
	tpl := layoutTpl(
		entries,
		&templates.InventoryLayoutSection{
			Entries: []templates.LayoutEntry{
				{EntryRef: "inv_0001", Position: 0}, // swap inventory
				{EntryRef: "inv_0000", Position: 1},
			},
		},
		&templates.StorageLayoutSection{
			Entries: []templates.LayoutEntry{
				{EntryRef: "sto_0003", Position: 0}, // reverse storage
				{EntryRef: "sto_0002", Position: 1},
				{EntryRef: "sto_0001", Position: 2},
				{EntryRef: "sto_0000", Position: 3},
			},
		},
	)
	jsonText := mustMarshalTpl(t, tpl)
	res, err := app.ApplyBuildTemplateV2ToCharacterJSON(0, jsonText, ApplyTemplateV2Options{SessionID: sessionID})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if !res.Applied {
		t.Fatalf("Applied=false, errors=%+v", res.Preview.Errors)
	}
	if res.LayoutInventoryEntriesApplied != 2 {
		t.Errorf("inv matched=%d, want 2", res.LayoutInventoryEntriesApplied)
	}
	if res.LayoutStorageEntriesApplied != 4 {
		t.Errorf("sto matched=%d, want 4", res.LayoutStorageEntriesApplied)
	}
}

// ─── 12) layout-only (no items selected) still runs ─────────────────────

func TestApplyBuildTemplateV2_Layout_OnlySelection_NoItemsSelection(t *testing.T) {
	// Confirms Phase 8E.1 accepts a template that selects ONLY layout.
	// (items section must still be present so entryRefs resolve, but
	// selection.items is absent.)
	app, sessionID := startSessionForFixture(t)
	tpl := layoutTpl(
		entriesForTestWeapons(),
		&templates.InventoryLayoutSection{
			Entries: []templates.LayoutEntry{
				{EntryRef: "inv_0000", Position: 0},
				{EntryRef: "inv_0001", Position: 1},
				{EntryRef: "inv_0002", Position: 2},
				{EntryRef: "inv_0003", Position: 3},
			},
		},
		nil,
	)
	jsonText := mustMarshalTpl(t, tpl)
	res, err := app.ApplyBuildTemplateV2ToCharacterJSON(0, jsonText, ApplyTemplateV2Options{SessionID: sessionID})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if !res.Applied {
		t.Fatalf("Applied=false (layout-only must apply), errors=%+v", res.Preview.Errors)
	}
	if res.LayoutInventoryEntriesApplied != 4 {
		t.Errorf("matched=%d, want 4", res.LayoutInventoryEntriesApplied)
	}
	if res.InventoryItemsApplied != 0 {
		t.Errorf("InventoryItemsApplied=%d, want 0 (no items selection)", res.InventoryItemsApplied)
	}
}

// ─── 13) items + layout combined: addMissing first, then reorder ────────

func TestApplyBuildTemplateV2_ItemsThenLayout_OrdersFreshlyAddedItems(t *testing.T) {
	// Start with an empty workspace; addMissing inserts 2 items, then
	// layout reorders them.
	app, sessionID := freshItemsFixture(t)
	entries := []templates.TemplateItemEntryV2{
		standardWeaponEntry("inv_first", idStandardDagger, 0),
		standardWeaponEntry("inv_second", idStandardClaymore, 0),
	}
	tpl := &templates.BuildTemplate{
		Schema:     templates.SchemaKey,
		Version:    2,
		AppVersion: "test",
		CreatedAt:  "2026-06-08T00:00:00Z",
		Selection: &templates.TemplateSelection{
			Items:           &templates.SectionSelection{All: true},
			InventoryLayout: &templates.SectionSelection{All: true},
		},
		Sections: templates.TemplateSections{
			Items: &templates.ItemsSection{Entries: entries},
			InventoryLayout: &templates.InventoryLayoutSection{
				Entries: []templates.LayoutEntry{
					{EntryRef: "inv_second", Position: 0}, // Claymore first
					{EntryRef: "inv_first", Position: 1},  // Dagger second
				},
			},
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
	if res.InventoryItemsApplied != 2 {
		t.Errorf("items applied=%d, want 2", res.InventoryItemsApplied)
	}
	if res.LayoutInventoryEntriesApplied != 2 {
		t.Errorf("layout matched=%d, want 2 (items addMissing ran before layout)", res.LayoutInventoryEntriesApplied)
	}
	got := positionsByBaseID(res.Workspace.InventoryItems)
	if got[idStandardClaymore] != 0 {
		t.Errorf("Claymore Position=%d, want 0", got[idStandardClaymore])
	}
	if got[idStandardDagger] != 1 {
		t.Errorf("Dagger Position=%d, want 1", got[idStandardDagger])
	}
}

// ─── 14) no deletion of extras / no cross-container movement ────────────

func TestApplyBuildTemplateV2_Layout_NeverDeletesOrMovesItems(t *testing.T) {
	app, sessionID := startSessionForFixture(t)
	preWS, _ := app.GetInventoryEditSession(sessionID)
	preInvCount := len(preWS.InventoryItems)
	preStoCount := len(preWS.StorageItems)

	// Layout that mentions only 1 item; the other 3 must stay (as extras).
	tpl := layoutTpl(
		entriesForTestWeapons(),
		&templates.InventoryLayoutSection{
			Entries: []templates.LayoutEntry{{EntryRef: "inv_0000", Position: 0}},
		},
		nil,
	)
	jsonText := mustMarshalTpl(t, tpl)
	res, err := app.ApplyBuildTemplateV2ToCharacterJSON(0, jsonText, ApplyTemplateV2Options{SessionID: sessionID})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if !res.Applied {
		t.Fatalf("Applied=false, errors=%+v", res.Preview.Errors)
	}
	postWS, _ := app.GetInventoryEditSession(sessionID)
	if len(postWS.InventoryItems) != preInvCount {
		t.Errorf("inventory count changed: pre=%d post=%d (no deletions allowed)", preInvCount, len(postWS.InventoryItems))
	}
	if len(postWS.StorageItems) != preStoCount {
		t.Errorf("storage count changed: pre=%d post=%d (no cross-container moves)", preStoCount, len(postWS.StorageItems))
	}
}
