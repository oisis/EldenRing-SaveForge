package main

import (
	"strings"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/templates"
)

// Phase 7a.2 — apply-time weapon level override on the v2 apply path.
// All tests in this file exercise ApplyBuildTemplateV2ToCharacterJSON
// with ApplyTemplateV2Options.WeaponLevelOverride set. They reuse the
// inventory-edit-session fixture from app_templates_v2_apply_inventory_test.go
// (startSessionForFixture) and the v1 override fixture helpers
// (intPtr, idStandard*, idSomber*, idUnupgradeableArm, findAddedWeapon,
// hasIssueCode) from app_templates_weapon_override_test.go.

// templateWithInventoryItems is a thin wrapper around templateWithInventory
// that builds the smallest legal v2 template for a single-weapon import.
// upgrade lets the caller seed an upgrade level the override will then
// overwrite; quantity is fixed at 1 because the override semantics do not
// care about stacks.
func templateWithInventoryWeapon(baseItemID uint32, templateUpgrade int) *templates.BuildTemplate {
	return templateWithInventory([]templates.TemplateItem{
		{
			BaseItemID: baseItemID,
			Name:       "weapon",
			Quantity:   1,
			Upgrade:    templateUpgrade,
			Container:  templates.ContainerInventory,
			Position:   0,
		},
	})
}

// templateWithInventoryWeapons builds a v2 template with multiple weapons
// in the inventory section. Mirrors overrideTemplateJSONMultiple from the
// v1 override test file.
func templateWithInventoryWeapons(ids ...uint32) *templates.BuildTemplate {
	items := make([]templates.TemplateItem, 0, len(ids))
	for _, id := range ids {
		items = append(items, templates.TemplateItem{
			BaseItemID: id,
			Name:       "weapon",
			Quantity:   1,
			Container:  templates.ContainerInventory,
			Position:   0,
		})
	}
	return templateWithInventory(items)
}

// ─── Nil / disabled / no-op paths ──────────────────────────────────────

func TestApplyBuildTemplateV2_Inventory_OverrideNil_NoOp(t *testing.T) {
	app, sessionID := startSessionForFixture(t)
	jsonText := mustMarshalTpl(t, templateWithInventoryWeapon(idStandardDagger, 3))

	res, err := app.ApplyBuildTemplateV2ToCharacterJSON(0, jsonText, ApplyTemplateV2Options{SessionID: sessionID})
	if err != nil || !res.Applied {
		t.Fatalf("apply: err=%v applied=%v preview=%+v", err, res.Applied, res.Preview)
	}
	got := findAddedWeapon(t, *res.Workspace, idStandardDagger)
	if got.CurrentUpgrade != 3 {
		t.Errorf("nil override: template upgrade should pass through, got +%d want +3", got.CurrentUpgrade)
	}
	if hasIssueCode(res.Preview.Warnings, templates.IssueCodeWeaponLevelClamped) {
		t.Errorf("nil override emitted clamp warning: %+v", res.Preview.Warnings)
	}
}

func TestApplyBuildTemplateV2_Inventory_OverrideEnabledFalse_NoOp(t *testing.T) {
	app, sessionID := startSessionForFixture(t)
	jsonText := mustMarshalTpl(t, templateWithInventoryWeapon(idStandardDagger, 3))

	res, err := app.ApplyBuildTemplateV2ToCharacterJSON(0, jsonText, ApplyTemplateV2Options{
		SessionID: sessionID,
		WeaponLevelOverride: &WeaponLevelOverride{
			Enabled:       false,
			StandardLevel: intPtr(25),
		},
	})
	if err != nil || !res.Applied {
		t.Fatalf("apply: err=%v applied=%v", err, res.Applied)
	}
	got := findAddedWeapon(t, *res.Workspace, idStandardDagger)
	if got.CurrentUpgrade != 3 {
		t.Errorf("disabled override must be a no-op: CurrentUpgrade=+%d want +3", got.CurrentUpgrade)
	}
}

// ─── Standard / somber happy paths ─────────────────────────────────────

func TestApplyBuildTemplateV2_Inventory_StandardLevelOnly(t *testing.T) {
	app, sessionID := startSessionForFixture(t)
	jsonText := mustMarshalTpl(t, templateWithInventoryWeapons(idStandardDagger, idSomberGolemBow))

	res, err := app.ApplyBuildTemplateV2ToCharacterJSON(0, jsonText, ApplyTemplateV2Options{
		SessionID: sessionID,
		WeaponLevelOverride: &WeaponLevelOverride{
			Enabled:       true,
			StandardLevel: intPtr(15),
		},
	})
	if err != nil || !res.Applied {
		t.Fatalf("apply: err=%v applied=%v", err, res.Applied)
	}
	standard := findAddedWeapon(t, *res.Workspace, idStandardDagger)
	somber := findAddedWeapon(t, *res.Workspace, idSomberGolemBow)
	if standard.CurrentUpgrade != 15 {
		t.Errorf("standard weapon should be +15, got +%d", standard.CurrentUpgrade)
	}
	if somber.CurrentUpgrade != 0 {
		t.Errorf("somber weapon should be untouched, got +%d", somber.CurrentUpgrade)
	}
}

func TestApplyBuildTemplateV2_Inventory_SomberLevelOnly(t *testing.T) {
	app, sessionID := startSessionForFixture(t)
	jsonText := mustMarshalTpl(t, templateWithInventoryWeapons(idStandardDagger, idSomberGolemBow, idSomberMaraisSword))

	res, err := app.ApplyBuildTemplateV2ToCharacterJSON(0, jsonText, ApplyTemplateV2Options{
		SessionID: sessionID,
		WeaponLevelOverride: &WeaponLevelOverride{
			Enabled:     true,
			SomberLevel: intPtr(7),
		},
	})
	if err != nil || !res.Applied {
		t.Fatalf("apply: err=%v applied=%v", err, res.Applied)
	}
	standard := findAddedWeapon(t, *res.Workspace, idStandardDagger)
	if standard.CurrentUpgrade != 0 {
		t.Errorf("standard weapon should be untouched, got +%d", standard.CurrentUpgrade)
	}
	for _, id := range []uint32{idSomberGolemBow, idSomberMaraisSword} {
		got := findAddedWeapon(t, *res.Workspace, id)
		if got.CurrentUpgrade != 7 {
			t.Errorf("somber weapon 0x%08X should be +7, got +%d", id, got.CurrentUpgrade)
		}
	}
}

func TestApplyBuildTemplateV2_Inventory_BothLevels(t *testing.T) {
	app, sessionID := startSessionForFixture(t)
	jsonText := mustMarshalTpl(t, templateWithInventoryWeapons(idStandardClaymore, idSomberGolemBow))

	res, err := app.ApplyBuildTemplateV2ToCharacterJSON(0, jsonText, ApplyTemplateV2Options{
		SessionID: sessionID,
		WeaponLevelOverride: &WeaponLevelOverride{
			Enabled:       true,
			StandardLevel: intPtr(25),
			SomberLevel:   intPtr(10),
		},
	})
	if err != nil || !res.Applied {
		t.Fatalf("apply: err=%v applied=%v", err, res.Applied)
	}
	standard := findAddedWeapon(t, *res.Workspace, idStandardClaymore)
	somber := findAddedWeapon(t, *res.Workspace, idSomberGolemBow)
	if standard.CurrentUpgrade != 25 {
		t.Errorf("standard +25 expected, got +%d", standard.CurrentUpgrade)
	}
	if somber.CurrentUpgrade != 10 {
		t.Errorf("somber +10 expected, got +%d", somber.CurrentUpgrade)
	}
	if hasIssueCode(res.Preview.Warnings, templates.IssueCodeWeaponLevelClamped) {
		t.Errorf("exact-boundary override must not warn: %+v", res.Preview.Warnings)
	}
}

// ─── Clamping warnings ─────────────────────────────────────────────────

func TestApplyBuildTemplateV2_Inventory_ClampedStandard(t *testing.T) {
	app, sessionID := startSessionForFixture(t)
	jsonText := mustMarshalTpl(t, templateWithInventoryWeapon(idStandardDagger, 0))

	res, err := app.ApplyBuildTemplateV2ToCharacterJSON(0, jsonText, ApplyTemplateV2Options{
		SessionID: sessionID,
		WeaponLevelOverride: &WeaponLevelOverride{
			Enabled:       true,
			StandardLevel: intPtr(99),
		},
	})
	if err != nil || !res.Applied {
		t.Fatalf("apply: err=%v applied=%v", err, res.Applied)
	}
	got := findAddedWeapon(t, *res.Workspace, idStandardDagger)
	if got.CurrentUpgrade != 25 {
		t.Errorf("standard 99 should clamp to +25, got +%d", got.CurrentUpgrade)
	}
	if !hasIssueCode(res.Preview.Warnings, templates.IssueCodeWeaponLevelClamped) {
		t.Errorf("expected %q warning, got %+v", templates.IssueCodeWeaponLevelClamped, res.Preview.Warnings)
	}
}

func TestApplyBuildTemplateV2_Inventory_ClampedSomber(t *testing.T) {
	app, sessionID := startSessionForFixture(t)
	jsonText := mustMarshalTpl(t, templateWithInventoryWeapon(idSomberGolemBow, 0))

	res, err := app.ApplyBuildTemplateV2ToCharacterJSON(0, jsonText, ApplyTemplateV2Options{
		SessionID: sessionID,
		WeaponLevelOverride: &WeaponLevelOverride{
			Enabled:     true,
			SomberLevel: intPtr(99),
		},
	})
	if err != nil || !res.Applied {
		t.Fatalf("apply: err=%v applied=%v", err, res.Applied)
	}
	got := findAddedWeapon(t, *res.Workspace, idSomberGolemBow)
	if got.CurrentUpgrade != 10 {
		t.Errorf("somber 99 should clamp to +10, got +%d", got.CurrentUpgrade)
	}
	if !hasIssueCode(res.Preview.Warnings, templates.IssueCodeWeaponLevelClamped) {
		t.Errorf("expected clamp warning, got %+v", res.Preview.Warnings)
	}
}

func TestApplyBuildTemplateV2_Inventory_Unupgradeable_Warning(t *testing.T) {
	app, sessionID := startSessionForFixture(t)
	jsonText := mustMarshalTpl(t, templateWithInventoryWeapon(idUnupgradeableArm, 0))

	res, err := app.ApplyBuildTemplateV2ToCharacterJSON(0, jsonText, ApplyTemplateV2Options{
		SessionID: sessionID,
		WeaponLevelOverride: &WeaponLevelOverride{
			Enabled:       true,
			StandardLevel: intPtr(10),
		},
	})
	if err != nil || !res.Applied {
		t.Fatalf("apply: err=%v applied=%v preview=%+v", err, res.Applied, res.Preview)
	}
	got := findAddedWeapon(t, *res.Workspace, idUnupgradeableArm)
	if got.CurrentUpgrade != 0 {
		t.Errorf("unupgradeable weapon must stay at +0, got +%d", got.CurrentUpgrade)
	}
	if !hasIssueCode(res.Preview.Warnings, templates.IssueCodeWeaponUnupgradeable) {
		t.Errorf("expected %q warning, got %+v", templates.IssueCodeWeaponUnupgradeable, res.Preview.Warnings)
	}
}

// ─── Validation rejects (no mutation) ──────────────────────────────────

func TestApplyBuildTemplateV2_Inventory_OverrideEnabled_BothNil_Rejected(t *testing.T) {
	app, sessionID := startSessionForFixture(t)
	jsonText := mustMarshalTpl(t, templateWithInventoryWeapon(idStandardDagger, 0))
	preSlotLen := len(app.save.Slots[0].Data)
	preWorkspace, err := app.GetInventoryEditSession(sessionID)
	if err != nil {
		t.Fatalf("GetInventoryEditSession: %v", err)
	}
	preInventoryLen := len(preWorkspace.InventoryItems)

	res, err := app.ApplyBuildTemplateV2ToCharacterJSON(0, jsonText, ApplyTemplateV2Options{
		SessionID:           sessionID,
		WeaponLevelOverride: &WeaponLevelOverride{Enabled: true},
	})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if res.Applied {
		t.Fatal("Applied=true for invalid override shape")
	}
	if len(res.Preview.Errors) == 0 || res.Preview.Errors[0].Code != templates.IssueCodeStructureInvalid {
		t.Errorf("expected IssueCodeStructureInvalid, got %+v", res.Preview.Errors)
	}
	if !strings.Contains(res.Preview.Errors[0].Message, "requires at least one of standardLevel") {
		t.Errorf("error message should reference the invalid options shape, got %q", res.Preview.Errors[0].Message)
	}
	if got := len(app.save.Slots[0].Data); got != preSlotLen {
		t.Errorf("slot.Data length changed despite rejection: pre=%d post=%d", preSlotLen, got)
	}
	post, err := app.GetInventoryEditSession(sessionID)
	if err != nil {
		t.Fatalf("GetInventoryEditSession post: %v", err)
	}
	if len(post.InventoryItems) != preInventoryLen {
		t.Errorf("workspace mutated despite override rejection: pre=%d post=%d", preInventoryLen, len(post.InventoryItems))
	}
	if post.Dirty {
		t.Error("workspace Dirty flag flipped despite override rejection")
	}
}

func TestApplyBuildTemplateV2_Inventory_NegativeStandard_Rejected(t *testing.T) {
	app, sessionID := startSessionForFixture(t)
	jsonText := mustMarshalTpl(t, templateWithInventoryWeapon(idStandardDagger, 0))
	preWorkspace, err := app.GetInventoryEditSession(sessionID)
	if err != nil {
		t.Fatalf("GetInventoryEditSession: %v", err)
	}
	preInventoryLen := len(preWorkspace.InventoryItems)

	res, err := app.ApplyBuildTemplateV2ToCharacterJSON(0, jsonText, ApplyTemplateV2Options{
		SessionID: sessionID,
		WeaponLevelOverride: &WeaponLevelOverride{
			Enabled:       true,
			StandardLevel: intPtr(-1),
		},
	})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if res.Applied {
		t.Fatal("Applied=true for negative standardLevel")
	}
	if len(res.Preview.Errors) == 0 || res.Preview.Errors[0].Code != templates.IssueCodeStructureInvalid {
		t.Errorf("expected IssueCodeStructureInvalid, got %+v", res.Preview.Errors)
	}
	if !strings.Contains(res.Preview.Errors[0].Message, "standardLevel") {
		t.Errorf("error message should name the offending field, got %q", res.Preview.Errors[0].Message)
	}
	post, err := app.GetInventoryEditSession(sessionID)
	if err != nil {
		t.Fatalf("GetInventoryEditSession post: %v", err)
	}
	if len(post.InventoryItems) != preInventoryLen {
		t.Errorf("workspace mutated despite negative-level rejection")
	}
}

// ─── Mixed apply paths ─────────────────────────────────────────────────

func TestApplyBuildTemplateV2_Mixed_ProfileStatsInventory_OverrideHappyPath(t *testing.T) {
	app, sessionID := startSessionForFixture(t)

	tpl := &templates.BuildTemplate{
		Schema:     templates.SchemaKey,
		Version:    2,
		AppVersion: "test",
		CreatedAt:  "2026-05-31T12:00:00Z",
		Selection: &templates.TemplateSelection{
			Profile:            &templates.SectionSelection{Fields: map[string]bool{"level": true}},
			Stats:              &templates.SectionSelection{Fields: map[string]bool{"vigor": true}},
			InventoryWorkspace: &templates.SectionSelection{All: true},
		},
		Sections: templates.TemplateSections{
			Profile: &templates.ProfileSection{Level: u32(99)},
			Stats:   &templates.StatsSection{Vigor: u32(42)},
			InventoryWorkspace: &templates.InventoryWorkspaceSection{
				InventoryItems: []templates.TemplateItem{
					{BaseItemID: idStandardDagger, Name: "Dagger", Quantity: 1, Container: templates.ContainerInventory, Position: 0},
				},
				StorageItems: []templates.TemplateItem{},
			},
		},
	}
	jsonText := mustMarshalTpl(t, tpl)

	res, err := app.ApplyBuildTemplateV2ToCharacterJSON(0, jsonText, ApplyTemplateV2Options{
		SessionID: sessionID,
		WeaponLevelOverride: &WeaponLevelOverride{
			Enabled:       true,
			StandardLevel: intPtr(20),
		},
	})
	if err != nil || !res.Applied {
		t.Fatalf("apply: err=%v applied=%v preview=%+v", err, res.Applied, res.Preview)
	}
	if app.save.Slots[0].Player.Level != 99 {
		t.Errorf("Player.Level = %d, want 99", app.save.Slots[0].Player.Level)
	}
	if app.save.Slots[0].Player.Vigor != 42 {
		t.Errorf("Player.Vigor = %d, want 42", app.save.Slots[0].Player.Vigor)
	}
	got := findAddedWeapon(t, *res.Workspace, idStandardDagger)
	if got.CurrentUpgrade != 20 {
		t.Errorf("override should set the freshly-imported weapon to +20, got +%d", got.CurrentUpgrade)
	}
}

// Override validation runs BEFORE acquireSession + snapshot. Even on a
// mixed template, an invalid override shape rejects the request without
// touching slot bytes or workspace state.
func TestApplyBuildTemplateV2_Mixed_OverrideValidationFailure_Rollback(t *testing.T) {
	app, sessionID := startSessionForFixture(t)

	tpl := &templates.BuildTemplate{
		Schema:     templates.SchemaKey,
		Version:    2,
		AppVersion: "test",
		CreatedAt:  "2026-05-31T12:00:00Z",
		Selection: &templates.TemplateSelection{
			Profile:            &templates.SectionSelection{Fields: map[string]bool{"level": true}},
			Stats:              &templates.SectionSelection{Fields: map[string]bool{"vigor": true}},
			InventoryWorkspace: &templates.SectionSelection{All: true},
		},
		Sections: templates.TemplateSections{
			Profile: &templates.ProfileSection{Level: u32(99)},
			Stats:   &templates.StatsSection{Vigor: u32(42)},
			InventoryWorkspace: &templates.InventoryWorkspaceSection{
				InventoryItems: []templates.TemplateItem{
					{BaseItemID: idStandardDagger, Name: "Dagger", Quantity: 1, Container: templates.ContainerInventory, Position: 0},
				},
				StorageItems: []templates.TemplateItem{},
			},
		},
	}
	jsonText := mustMarshalTpl(t, tpl)

	preLevel := app.save.Slots[0].Player.Level
	preVigor := app.save.Slots[0].Player.Vigor
	preSlotLen := len(app.save.Slots[0].Data)
	preWorkspace, err := app.GetInventoryEditSession(sessionID)
	if err != nil {
		t.Fatalf("GetInventoryEditSession: %v", err)
	}
	preInventoryLen := len(preWorkspace.InventoryItems)

	res, err := app.ApplyBuildTemplateV2ToCharacterJSON(0, jsonText, ApplyTemplateV2Options{
		SessionID:           sessionID,
		WeaponLevelOverride: &WeaponLevelOverride{Enabled: true},
	})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if res.Applied {
		t.Fatal("Applied=true despite invalid override on a mixed template")
	}
	if app.save.Slots[0].Player.Level != preLevel {
		t.Errorf("Player.Level mutated: pre=%d post=%d", preLevel, app.save.Slots[0].Player.Level)
	}
	if app.save.Slots[0].Player.Vigor != preVigor {
		t.Errorf("Player.Vigor mutated: pre=%d post=%d", preVigor, app.save.Slots[0].Player.Vigor)
	}
	if got := len(app.save.Slots[0].Data); got != preSlotLen {
		t.Errorf("slot.Data length changed: pre=%d post=%d", preSlotLen, got)
	}
	post, err := app.GetInventoryEditSession(sessionID)
	if err != nil {
		t.Fatalf("GetInventoryEditSession post: %v", err)
	}
	if len(post.InventoryItems) != preInventoryLen {
		t.Errorf("workspace mutated: pre=%d post=%d", preInventoryLen, len(post.InventoryItems))
	}
	if post.Dirty {
		t.Error("workspace Dirty flag flipped on validation failure")
	}
}

// ─── Profile/stats-only + override ─────────────────────────────────────

// A structurally valid override paired with a profile/stats-only template
// is silently ignored: the override never finds an inventory.workspace
// section to act on, so the apply proceeds as a vanilla profile/stats
// apply and emits no override warnings.
func TestApplyBuildTemplateV2_ProfileStatsOnly_WithOverride_Ignored(t *testing.T) {
	app, _ := startSessionForFixture(t)

	tpl := &templates.BuildTemplate{
		Schema:     templates.SchemaKey,
		Version:    2,
		AppVersion: "test",
		CreatedAt:  "2026-05-31T12:00:00Z",
		Selection: &templates.TemplateSelection{
			Profile: &templates.SectionSelection{Fields: map[string]bool{"level": true}},
			Stats:   &templates.SectionSelection{Fields: map[string]bool{"vigor": true}},
		},
		Sections: templates.TemplateSections{
			Profile: &templates.ProfileSection{Level: u32(77)},
			Stats:   &templates.StatsSection{Vigor: u32(33)},
		},
	}
	jsonText := mustMarshalTpl(t, tpl)

	// Profile/stats-only apply still rejects when an edit session conflicts;
	// close the fixture's session so the v2 path can run.
	app.editSessionsMu.Lock()
	for id, sess := range app.editSessions {
		if sess.CharacterIndex == 0 {
			delete(app.editSessions, id)
			delete(app.editSessionByChar, 0)
		}
	}
	app.editSessionsMu.Unlock()

	res, err := app.ApplyBuildTemplateV2ToCharacterJSON(0, jsonText, ApplyTemplateV2Options{
		WeaponLevelOverride: &WeaponLevelOverride{
			Enabled:       true,
			StandardLevel: intPtr(15),
		},
	})
	if err != nil || !res.Applied {
		t.Fatalf("apply: err=%v applied=%v preview=%+v", err, res.Applied, res.Preview)
	}
	if app.save.Slots[0].Player.Level != 77 {
		t.Errorf("Player.Level = %d, want 77", app.save.Slots[0].Player.Level)
	}
	if app.save.Slots[0].Player.Vigor != 33 {
		t.Errorf("Player.Vigor = %d, want 33", app.save.Slots[0].Player.Vigor)
	}
	if hasIssueCode(res.Preview.Warnings, templates.IssueCodeWeaponLevelClamped) ||
		hasIssueCode(res.Preview.Warnings, templates.IssueCodeWeaponUnupgradeable) {
		t.Errorf("profile/stats-only apply must not emit override warnings, got %+v", res.Preview.Warnings)
	}
	if res.InventoryItemsApplied != 0 || res.StorageItemsApplied != 0 {
		t.Errorf("inventory counters non-zero on profile/stats-only apply: inv=%d sto=%d", res.InventoryItemsApplied, res.StorageItemsApplied)
	}
}
