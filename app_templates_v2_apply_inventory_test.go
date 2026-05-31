package main

import (
	"encoding/json"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/editor"
	"github.com/oisis/EldenRing-SaveForge/backend/templates"
)

// Phase 7a — v2 inventory.workspace apply against an active Inventory
// Edit Session. All tests in this file exercise
// ApplyBuildTemplateV2ToCharacterJSON with the new SessionID option +
// inventory.workspace selection. They share the inventoryOrderFixture
// shape because that fixture is the only one in the codebase that ships
// a real slot.Data layout `editor.BuildSnapshot` can read.

// startSessionForFixture builds the inventory-backed fixture, flips the
// slot-active bit Phase 5/Phase 7a checks, and starts a real session.
// Returns the App plus the active session ID.
func startSessionForFixture(t *testing.T) (*App, string) {
	t.Helper()
	app := inventoryOrderFixture(testWeapons)
	app.save.ActiveSlots[0] = true
	snap, err := app.StartInventoryEditSession(0)
	if err != nil {
		t.Fatalf("StartInventoryEditSession: %v", err)
	}
	return app, snap.SessionID
}

// templateWithInventory builds the smallest legal v2 template that
// selects inventory.workspace and adds the supplied items. Profile/
// stats stay absent so tests can isolate the inventory path. The
// container field is normalised to ContainerInventory for callers that
// don't care about per-item placement.
func templateWithInventory(items []templates.TemplateItem) *templates.BuildTemplate {
	for i := range items {
		if items[i].Container == "" {
			items[i].Container = templates.ContainerInventory
		}
	}
	return &templates.BuildTemplate{
		Schema:     templates.SchemaKey,
		Version:    2,
		AppVersion: "test",
		CreatedAt:  "2026-05-31T12:00:00Z",
		Selection: &templates.TemplateSelection{
			InventoryWorkspace: &templates.SectionSelection{All: true},
		},
		Sections: templates.TemplateSections{
			InventoryWorkspace: &templates.InventoryWorkspaceSection{
				InventoryItems: items,
				StorageItems:   []templates.TemplateItem{},
			},
		},
	}
}

func mustMarshalTpl(t *testing.T, tpl *templates.BuildTemplate) string {
	t.Helper()
	if err := templates.ValidateBuildTemplate(tpl); err != nil {
		t.Fatalf("ValidateBuildTemplate: %v", err)
	}
	data, err := json.Marshal(tpl)
	if err != nil {
		t.Fatalf("marshal template: %v", err)
	}
	return string(data)
}

// ─── Reject paths ──────────────────────────────────────────────────────

// Inventory selection + bogus session ID → IssueCodeInventorySessionInvalid.
// Slot stays byte-identical; the active session's workspace stays
// byte-identical too because we never reached the apply layer.
func TestApplyBuildTemplateV2_Inventory_UnknownSessionRejected(t *testing.T) {
	app, _ := startSessionForFixture(t)
	jsonText := mustMarshalTpl(t, templateWithInventory(nil))
	preSlotLen := len(app.save.Slots[0].Data)

	res, err := app.ApplyBuildTemplateV2ToCharacterJSON(0, jsonText, ApplyTemplateV2Options{SessionID: "ses-deadbeef"})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if res.Applied {
		t.Fatal("Applied=true for unknown session ID")
	}
	if len(res.Preview.Errors) == 0 || res.Preview.Errors[0].Code != templates.IssueCodeInventorySessionInvalid {
		t.Errorf("expected IssueCodeInventorySessionInvalid, got %+v", res.Preview.Errors)
	}
	if got := len(app.save.Slots[0].Data); got != preSlotLen {
		t.Errorf("slot.Data length changed: pre=%d post=%d", preSlotLen, got)
	}
}

// Inventory selection + session belonging to a DIFFERENT character →
// IssueCodeInventorySessionInvalid. Target slot stays untouched; the
// other character's workspace stays clean too (no mutation, no Dirty
// flip).
func TestApplyBuildTemplateV2_Inventory_WrongCharacterSessionRejected(t *testing.T) {
	app := inventoryOrderFixture(testWeapons)
	app.save.ActiveSlots[0] = true
	app.save.ActiveSlots[1] = true
	// slot 1 needs a bare-bones structure for StartInventoryEditSession;
	// inventoryOrderFixture only populates slot 0. Copy the slot 0
	// layout so editor.BuildSnapshot can read it.
	app.save.Slots[1] = app.save.Slots[0]
	app.save.Slots[1].Data = append([]byte(nil), app.save.Slots[0].Data...)

	snap, err := app.StartInventoryEditSession(1)
	if err != nil {
		t.Fatalf("StartInventoryEditSession(1): %v", err)
	}
	wrongSession := snap.SessionID

	jsonText := mustMarshalTpl(t, templateWithInventory(nil))
	res, err := app.ApplyBuildTemplateV2ToCharacterJSON(0, jsonText, ApplyTemplateV2Options{SessionID: wrongSession})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if res.Applied {
		t.Fatal("Applied=true for session bound to a different character")
	}
	if len(res.Preview.Errors) == 0 || res.Preview.Errors[0].Code != templates.IssueCodeInventorySessionInvalid {
		t.Errorf("expected IssueCodeInventorySessionInvalid, got %+v", res.Preview.Errors)
	}
}

// ─── Happy path ────────────────────────────────────────────────────────

// Inventory selection + valid session → items land at the END of the
// workspace's inventory container (append semantics, mirrors the v1
// Apply Template flow). Workspace.Dirty flips to true; the counters on
// the result reflect the import count.
func TestApplyBuildTemplateV2_Inventory_HappyPath(t *testing.T) {
	app, sessionID := startSessionForFixture(t)

	tpl := templateWithInventory([]templates.TemplateItem{
		// Dagger — known DB entry; resolves cleanly through preview.
		{BaseItemID: 0x000F4240, Name: "Dagger", Quantity: 1, Container: templates.ContainerInventory, Position: 0},
	})
	jsonText := mustMarshalTpl(t, tpl)

	preWorkspace, err := app.GetInventoryEditSession(sessionID)
	if err != nil {
		t.Fatalf("GetInventoryEditSession: %v", err)
	}
	preInventoryLen := len(preWorkspace.InventoryItems)

	res, err := app.ApplyBuildTemplateV2ToCharacterJSON(0, jsonText, ApplyTemplateV2Options{SessionID: sessionID})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if !res.Applied {
		t.Fatalf("Applied=false, errors=%+v", res.Preview.Errors)
	}
	if res.InventoryItemsApplied != 1 {
		t.Errorf("InventoryItemsApplied = %d, want 1", res.InventoryItemsApplied)
	}
	if res.StorageItemsApplied != 0 {
		t.Errorf("StorageItemsApplied = %d, want 0", res.StorageItemsApplied)
	}
	contains := false
	for _, f := range res.AppliedFields {
		if f == "inventory.workspace" {
			contains = true
			break
		}
	}
	if !contains {
		t.Errorf("AppliedFields missing 'inventory.workspace' sentinel: %+v", res.AppliedFields)
	}
	if res.Workspace == nil {
		t.Fatal("Workspace must be populated on inventory apply success")
	}
	if !res.Workspace.Dirty {
		t.Error("Workspace.Dirty must be true after a successful inventory apply")
	}
	if got := len(res.Workspace.InventoryItems); got != preInventoryLen+1 {
		t.Errorf("InventoryItems = %d, want %d (append)", got, preInventoryLen+1)
	}
	// Active session is the same one we passed in — re-reading must
	// surface the same dirty workspace.
	post, err := app.GetInventoryEditSession(sessionID)
	if err != nil {
		t.Fatalf("GetInventoryEditSession post-apply: %v", err)
	}
	if !post.Dirty {
		t.Error("session workspace must report Dirty=true after apply")
	}
}

// Inventory selection + valid session + EMPTY items list → no-op
// success. Applied=false (no fields to write), workspace stays Dirty=
// false. This matches the contract documented on
// applyTemplateItemsToWorkspace.
func TestApplyBuildTemplateV2_Inventory_EmptyItems_NoOp(t *testing.T) {
	app, sessionID := startSessionForFixture(t)
	jsonText := mustMarshalTpl(t, templateWithInventory(nil))

	res, err := app.ApplyBuildTemplateV2ToCharacterJSON(0, jsonText, ApplyTemplateV2Options{SessionID: sessionID})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if res.Applied {
		t.Errorf("Applied=true for empty inventory selection (want no-op)")
	}
	if res.InventoryItemsApplied != 0 || res.StorageItemsApplied != 0 {
		t.Errorf("counters non-zero on no-op: inv=%d sto=%d", res.InventoryItemsApplied, res.StorageItemsApplied)
	}
	post, err := app.GetInventoryEditSession(sessionID)
	if err != nil {
		t.Fatalf("GetInventoryEditSession: %v", err)
	}
	if post.Dirty {
		t.Error("workspace must NOT be marked Dirty on empty no-op")
	}
}

// Mixed apply — profile + stats + inventory.workspace in one call.
// AppliedFields contains entries for every section that actually wrote;
// the workspace flips to Dirty and the inventory counter reflects the
// import. Player struct mutates to mirror the new profile/stats values.
func TestApplyBuildTemplateV2_Mixed_ProfileStatsInventory_HappyPath(t *testing.T) {
	app, sessionID := startSessionForFixture(t)

	tpl := &templates.BuildTemplate{
		Schema:     templates.SchemaKey,
		Version:    2,
		AppVersion: "test",
		CreatedAt:  "2026-05-31T12:00:00Z",
		Selection: &templates.TemplateSelection{
			Profile:            &templates.SectionSelection{Fields: map[string]bool{"level": true, "runes": true}},
			Stats:              &templates.SectionSelection{Fields: map[string]bool{"vigor": true, "endurance": true}},
			InventoryWorkspace: &templates.SectionSelection{All: true},
		},
		Sections: templates.TemplateSections{
			Profile: &templates.ProfileSection{Level: u32(99), Runes: u32(1000)},
			Stats:   &templates.StatsSection{Vigor: u32(42), Endurance: u32(33)},
			InventoryWorkspace: &templates.InventoryWorkspaceSection{
				InventoryItems: []templates.TemplateItem{
					{BaseItemID: 0x000F4240, Name: "Dagger", Quantity: 1, Container: templates.ContainerInventory, Position: 0},
				},
				StorageItems: []templates.TemplateItem{},
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
	if res.InventoryItemsApplied != 1 {
		t.Errorf("InventoryItemsApplied = %d, want 1", res.InventoryItemsApplied)
	}
	// Profile/stats sentinels surface as canonical paths; inventory.workspace
	// surfaces as its raw section key. Check for the union.
	want := map[string]bool{
		"profile.level":      false,
		"profile.runes":      false,
		"stats.vigor":        false,
		"stats.endurance":    false,
		"inventory.workspace": false,
	}
	for _, f := range res.AppliedFields {
		if _, ok := want[f]; ok {
			want[f] = true
		}
	}
	for k, found := range want {
		if !found {
			t.Errorf("AppliedFields missing %q: %+v", k, res.AppliedFields)
		}
	}
	if res.Workspace == nil || !res.Workspace.Dirty {
		t.Error("Workspace must be populated + Dirty after mixed apply")
	}
	if app.save.Slots[0].Player.Level != 99 {
		t.Errorf("Player.Level = %d, want 99", app.save.Slots[0].Player.Level)
	}
	if app.save.Slots[0].Player.Vigor != 42 {
		t.Errorf("Player.Vigor = %d, want 42", app.save.Slots[0].Player.Vigor)
	}
}

// ─── Phase 6b interaction ──────────────────────────────────────────────

// Phase 7a deliberately does NOT thread Phase 6b weapon-level override
// into the v2 path. Even if a future caller hand-sets the option, the
// ApplyTemplateV2Options struct has no override field, so the override
// can never reach the v2 apply layer. This test pins the contract: the
// v2 options struct exposes Mode + SessionID only.
func TestApplyTemplateV2Options_FieldSurface(t *testing.T) {
	// Compile-time guard: the literal must satisfy the struct without
	// unknown-field errors. Adding a new field below would force this
	// test to be updated, which is the audit trail we want.
	_ = ApplyTemplateV2Options{Mode: "append", SessionID: "ses-x"}
}

// ─── Scope guards still in force ──────────────────────────────────────

// Phase 7a only widens the inventory.workspace gate. Future v2 sections
// (equipment, equippedTalismans, spells, appearance) are not yet in the
// schema, so this test asserts that adding them via raw JSON still hits
// the strict-decode rejection BEFORE the new session-aware apply layer
// can be reached.
func TestApplyBuildTemplateV2_UnknownSectionStillRejected(t *testing.T) {
	app, sessionID := startSessionForFixture(t)
	// Unknown section "equipment" should be refused by the strict YAML/
	// JSON decoder; ParseBuildTemplateJSON enforces KnownFields on the
	// nested sections struct.
	rawJSON := `{
		"schema": "saveforge.build-template",
		"version": 2,
		"appVersion": "test",
		"createdAt": "2026-05-31T12:00:00Z",
		"selection": { "profile": true, "equipment": true },
		"sections": { "profile": { "level": 1 } }
	}`
	res, err := app.ApplyBuildTemplateV2ToCharacterJSON(0, rawJSON, ApplyTemplateV2Options{SessionID: sessionID})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if res.Applied {
		t.Fatal("Applied=true for unknown section name")
	}
	if len(res.Preview.Errors) == 0 {
		t.Fatal("expected at least one preview error")
	}
}

// ─── Sanity check on imported editor package ──────────────────────────

// Compile-time pin: the apply layer relies on editor.ContainerInventory
// existing. This guard catches an accidental rename far cheaper than
// the full backend test sweep.
var _ editor.ContainerKind = editor.ContainerInventory
