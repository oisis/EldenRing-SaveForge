package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/editor"
	"github.com/oisis/EldenRing-SaveForge/backend/templates"
)

// Phase 6b — apply-time weapon level override for v1 inventory.workspace
// apply. The override mutates the active edit session in-memory only;
// slot.Data is untouched and persistence still requires Save changes.

// Test weapon IDs used across this file. Pulled from the same DB the
// existing apply / clamp tests use, so a DB drift would surface here as
// "unknown_item" before any override logic runs.
const (
	idStandardDagger    = uint32(0x000F4240) // melee_armaments, MaxUpgrade 25
	idStandardClaymore  = uint32(0x003085E0) // melee_armaments, MaxUpgrade 25
	idSomberGolemBow    = uint32(0x02810590) // ranged_armaments, somber, MaxUpgrade 10
	idSomberMaraisSword = uint32(0x003010B0) // melee_armaments, somber, MaxUpgrade 10
	idUnupgradeableArm  = uint32(0x0001ADB0) // Unarmed, MaxUpgrade 0, MaxInventory 1
)

// intPtr is a tiny helper. WeaponLevelOverride uses *int so callers can
// distinguish "unset" from "set to 0", which is the entire reason that
// shape exists.
func intPtr(v int) *int { return &v }

// overrideTemplateJSON hand-builds a v1 template carrying a single
// inventory weapon. Mirrors the JSON shape used by the existing apply
// tests so the schema validator stays in the loop.
func overrideTemplateJSON(baseItemID uint32, templateUpgrade int) string {
	return fmt.Sprintf(`{
  "schema": "saveforge.build-template",
  "version": 1,
  "createdAt": "2026-05-31T00:00:00Z",
  "sections": {
    "inventory.workspace": {
      "inventoryItems": [{
        "baseItemID": %d,
        "quantity": 1,
        "upgrade": %d,
        "container": "inventory",
        "position": 0
      }],
      "storageItems": []
    }
  }
}`, baseItemID, templateUpgrade)
}

// overrideTemplateJSONMultiple builds a v1 template carrying any number
// of weapons in the inventory section. Useful for "mixed standard +
// somber" override tests.
func overrideTemplateJSONMultiple(items ...uint32) string {
	var entries []string
	for _, id := range items {
		entries = append(entries, fmt.Sprintf(`{
        "baseItemID": %d,
        "quantity": 1,
        "container": "inventory",
        "position": 0
      }`, id))
	}
	return fmt.Sprintf(`{
  "schema": "saveforge.build-template",
  "version": 1,
  "createdAt": "2026-05-31T00:00:00Z",
  "sections": {
    "inventory.workspace": {
      "inventoryItems": [%s],
      "storageItems": []
    }
  }
}`, strings.Join(entries, ","))
}

// freshOverrideFixture spins up an empty-workspace app: no pre-existing
// weapons in the slot, so Apply's append behaviour drops the imported
// items at deterministic positions 0..N-1 of the inventory list. The
// preview/capacity layer still runs, just with extra headroom.
func freshOverrideFixture(t *testing.T) (*App, string) {
	t.Helper()
	app := inventoryOrderFixture(nil)
	snap, err := app.StartInventoryEditSession(0)
	if err != nil {
		t.Fatalf("StartInventoryEditSession: %v", err)
	}
	return app, snap.SessionID
}

// findAddedWeapon walks the post-apply inventory list and returns the
// first item that matches the given baseItemID. Apply appends, so the
// last hit is the freshly-added one; we return the last match for
// determinism.
func findAddedWeapon(t *testing.T, snap editor.InventoryWorkspaceSnapshot, baseItemID uint32) editor.EditableItem {
	t.Helper()
	for i := len(snap.InventoryItems) - 1; i >= 0; i-- {
		if snap.InventoryItems[i].BaseItemID == baseItemID {
			return snap.InventoryItems[i]
		}
	}
	t.Fatalf("weapon with baseItemID 0x%08X not found in workspace inventory", baseItemID)
	return editor.EditableItem{}
}

func TestValidateWeaponLevelOverride_NilAndDisabledAccepted(t *testing.T) {
	if err := validateWeaponLevelOverride(nil); err != nil {
		t.Errorf("nil override should validate, got %v", err)
	}
	if err := validateWeaponLevelOverride(&WeaponLevelOverride{Enabled: false, StandardLevel: intPtr(99)}); err != nil {
		t.Errorf("disabled override should validate regardless of fields, got %v", err)
	}
}

func TestValidateWeaponLevelOverride_EnabledButEmptyRejected(t *testing.T) {
	o := &WeaponLevelOverride{Enabled: true}
	err := validateWeaponLevelOverride(o)
	if err == nil {
		t.Fatal("expected error for enabled override with no levels")
	}
	if !strings.Contains(err.Error(), "requires at least one of standardLevel") {
		t.Errorf("error message should name both selectors, got %v", err)
	}
}

func TestValidateWeaponLevelOverride_NegativeLevelsRejected(t *testing.T) {
	cases := []struct {
		name    string
		o       *WeaponLevelOverride
		message string
	}{
		{"negative standard", &WeaponLevelOverride{Enabled: true, StandardLevel: intPtr(-1)}, "standardLevel"},
		{"negative somber", &WeaponLevelOverride{Enabled: true, SomberLevel: intPtr(-3)}, "somberLevel"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateWeaponLevelOverride(tc.o)
			if err == nil {
				t.Fatalf("expected error for %s", tc.name)
			}
			if !strings.Contains(err.Error(), tc.message) {
				t.Errorf("error %q should mention %q", err, tc.message)
			}
		})
	}
}

func TestApplyTemplate_OverrideNil_DoesNotChangeUpgrade(t *testing.T) {
	app, sessionID := freshOverrideFixture(t)
	jsonText := overrideTemplateJSON(idStandardDagger, 3)
	res, err := app.ApplyBuildTemplateToWorkspaceJSON(sessionID, jsonText, ApplyTemplateOptions{})
	if err != nil || !res.Applied {
		t.Fatalf("apply: err=%v applied=%v preview=%+v", err, res.Applied, res.Preview)
	}
	got := findAddedWeapon(t, res.Workspace, idStandardDagger)
	if got.CurrentUpgrade != 3 {
		t.Errorf("template upgrade should pass through unchanged: CurrentUpgrade=%d want 3", got.CurrentUpgrade)
	}
}

func TestApplyTemplate_OverrideDisabled_BehavesLikeKeep(t *testing.T) {
	app, sessionID := freshOverrideFixture(t)
	jsonText := overrideTemplateJSON(idStandardDagger, 3)
	res, err := app.ApplyBuildTemplateToWorkspaceJSON(sessionID, jsonText, ApplyTemplateOptions{
		WeaponLevelOverride: &WeaponLevelOverride{Enabled: false, StandardLevel: intPtr(20)},
	})
	if err != nil || !res.Applied {
		t.Fatalf("apply: err=%v applied=%v", err, res.Applied)
	}
	got := findAddedWeapon(t, res.Workspace, idStandardDagger)
	if got.CurrentUpgrade != 3 {
		t.Errorf("disabled override must be a no-op: CurrentUpgrade=%d want 3", got.CurrentUpgrade)
	}
}

func TestApplyTemplate_OverrideEnabledButEmpty_RejectedBeforeApply(t *testing.T) {
	app, sessionID := freshOverrideFixture(t)
	jsonText := overrideTemplateJSON(idStandardDagger, 0)
	_, err := app.ApplyBuildTemplateToWorkspaceJSON(sessionID, jsonText, ApplyTemplateOptions{
		WeaponLevelOverride: &WeaponLevelOverride{Enabled: true},
	})
	if err == nil {
		t.Fatal("expected hard error for invalid override shape")
	}
	if !strings.Contains(err.Error(), "weaponLevelOverride.enabled=true requires at least one") {
		t.Errorf("error %q should reference the invalid options shape", err)
	}
	// Workspace must be unchanged.
	if len(app.editSessions[sessionID].Workspace.InventoryItems) != 0 {
		t.Errorf("workspace mutated despite invalid options, items=%d", len(app.editSessions[sessionID].Workspace.InventoryItems))
	}
}

func TestApplyTemplate_OverrideStandardOnly_TouchesStandardWeapons(t *testing.T) {
	app, sessionID := freshOverrideFixture(t)
	jsonText := overrideTemplateJSONMultiple(idStandardDagger, idSomberGolemBow)
	res, err := app.ApplyBuildTemplateToWorkspaceJSON(sessionID, jsonText, ApplyTemplateOptions{
		WeaponLevelOverride: &WeaponLevelOverride{Enabled: true, StandardLevel: intPtr(15)},
	})
	if err != nil || !res.Applied {
		t.Fatalf("apply: err=%v applied=%v", err, res.Applied)
	}
	standard := findAddedWeapon(t, res.Workspace, idStandardDagger)
	somber := findAddedWeapon(t, res.Workspace, idSomberGolemBow)
	if standard.CurrentUpgrade != 15 {
		t.Errorf("standard weapon should be set to +15, got +%d", standard.CurrentUpgrade)
	}
	if somber.CurrentUpgrade != 0 {
		t.Errorf("somber weapon should be untouched when only standardLevel set, got +%d", somber.CurrentUpgrade)
	}
}

func TestApplyTemplate_OverrideSomberOnly_TouchesSomberWeapons(t *testing.T) {
	app, sessionID := freshOverrideFixture(t)
	jsonText := overrideTemplateJSONMultiple(idStandardDagger, idSomberGolemBow, idSomberMaraisSword)
	res, err := app.ApplyBuildTemplateToWorkspaceJSON(sessionID, jsonText, ApplyTemplateOptions{
		WeaponLevelOverride: &WeaponLevelOverride{Enabled: true, SomberLevel: intPtr(7)},
	})
	if err != nil || !res.Applied {
		t.Fatalf("apply: err=%v applied=%v", err, res.Applied)
	}
	standard := findAddedWeapon(t, res.Workspace, idStandardDagger)
	if standard.CurrentUpgrade != 0 {
		t.Errorf("standard weapon should be untouched when only somberLevel set, got +%d", standard.CurrentUpgrade)
	}
	for _, id := range []uint32{idSomberGolemBow, idSomberMaraisSword} {
		got := findAddedWeapon(t, res.Workspace, id)
		if got.CurrentUpgrade != 7 {
			t.Errorf("somber weapon 0x%08X should be +7, got +%d", id, got.CurrentUpgrade)
		}
	}
}

func TestApplyTemplate_OverrideBothLevels_TouchesEachClassSeparately(t *testing.T) {
	app, sessionID := freshOverrideFixture(t)
	jsonText := overrideTemplateJSONMultiple(idStandardClaymore, idSomberGolemBow)
	res, err := app.ApplyBuildTemplateToWorkspaceJSON(sessionID, jsonText, ApplyTemplateOptions{
		WeaponLevelOverride: &WeaponLevelOverride{
			Enabled:       true,
			StandardLevel: intPtr(25),
			SomberLevel:   intPtr(10),
		},
	})
	if err != nil || !res.Applied {
		t.Fatalf("apply: err=%v applied=%v", err, res.Applied)
	}
	standard := findAddedWeapon(t, res.Workspace, idStandardClaymore)
	somber := findAddedWeapon(t, res.Workspace, idSomberGolemBow)
	if standard.CurrentUpgrade != 25 {
		t.Errorf("standard +25 expected, got +%d", standard.CurrentUpgrade)
	}
	if somber.CurrentUpgrade != 10 {
		t.Errorf("somber +10 expected, got +%d", somber.CurrentUpgrade)
	}
}

func TestApplyTemplate_OverrideStandardOverMax_ClampedWithWarning(t *testing.T) {
	app, sessionID := freshOverrideFixture(t)
	jsonText := overrideTemplateJSON(idStandardDagger, 0)
	res, err := app.ApplyBuildTemplateToWorkspaceJSON(sessionID, jsonText, ApplyTemplateOptions{
		WeaponLevelOverride: &WeaponLevelOverride{Enabled: true, StandardLevel: intPtr(99)},
	})
	if err != nil || !res.Applied {
		t.Fatalf("apply: err=%v applied=%v", err, res.Applied)
	}
	got := findAddedWeapon(t, res.Workspace, idStandardDagger)
	if got.CurrentUpgrade != 25 {
		t.Errorf("standard 99 should clamp to +25, got +%d", got.CurrentUpgrade)
	}
	if !hasIssueCode(res.Preview.Warnings, templates.IssueCodeWeaponLevelClamped) {
		t.Errorf("expected warning %q in preview.warnings, got %+v",
			templates.IssueCodeWeaponLevelClamped, res.Preview.Warnings)
	}
}

func TestApplyTemplate_OverrideSomberOverMax_ClampedWithWarning(t *testing.T) {
	app, sessionID := freshOverrideFixture(t)
	jsonText := overrideTemplateJSON(idSomberGolemBow, 0)
	res, err := app.ApplyBuildTemplateToWorkspaceJSON(sessionID, jsonText, ApplyTemplateOptions{
		WeaponLevelOverride: &WeaponLevelOverride{Enabled: true, SomberLevel: intPtr(99)},
	})
	if err != nil || !res.Applied {
		t.Fatalf("apply: err=%v applied=%v", err, res.Applied)
	}
	got := findAddedWeapon(t, res.Workspace, idSomberGolemBow)
	if got.CurrentUpgrade != 10 {
		t.Errorf("somber 99 should clamp to +10, got +%d", got.CurrentUpgrade)
	}
	if !hasIssueCode(res.Preview.Warnings, templates.IssueCodeWeaponLevelClamped) {
		t.Errorf("expected weapon_level_clamped warning, got %+v", res.Preview.Warnings)
	}
}

func TestApplyTemplate_OverrideAtMaxBoundary_NoWarning(t *testing.T) {
	app, sessionID := freshOverrideFixture(t)
	jsonText := overrideTemplateJSON(idStandardDagger, 0)
	res, err := app.ApplyBuildTemplateToWorkspaceJSON(sessionID, jsonText, ApplyTemplateOptions{
		WeaponLevelOverride: &WeaponLevelOverride{Enabled: true, StandardLevel: intPtr(25)},
	})
	if err != nil || !res.Applied {
		t.Fatalf("apply: err=%v applied=%v", err, res.Applied)
	}
	if hasIssueCode(res.Preview.Warnings, templates.IssueCodeWeaponLevelClamped) {
		t.Errorf("no clamp warning expected at exact-max boundary, got %+v", res.Preview.Warnings)
	}
	got := findAddedWeapon(t, res.Workspace, idStandardDagger)
	if got.CurrentUpgrade != 25 {
		t.Errorf("expected +25, got +%d", got.CurrentUpgrade)
	}
}

func TestApplyTemplate_OverrideUnupgradeable_SkippedWithWarning(t *testing.T) {
	app, sessionID := freshOverrideFixture(t)
	jsonText := overrideTemplateJSON(idUnupgradeableArm, 0)
	res, err := app.ApplyBuildTemplateToWorkspaceJSON(sessionID, jsonText, ApplyTemplateOptions{
		WeaponLevelOverride: &WeaponLevelOverride{Enabled: true, StandardLevel: intPtr(10)},
	})
	if err != nil || !res.Applied {
		t.Fatalf("apply: err=%v applied=%v preview=%+v", err, res.Applied, res.Preview)
	}
	got := findAddedWeapon(t, res.Workspace, idUnupgradeableArm)
	if got.CurrentUpgrade != 0 {
		t.Errorf("unupgradeable weapon must stay at +0, got +%d", got.CurrentUpgrade)
	}
	if !hasIssueCode(res.Preview.Warnings, templates.IssueCodeWeaponUnupgradeable) {
		t.Errorf("expected weapon_unupgradeable warning, got %+v", res.Preview.Warnings)
	}
}

func TestApplyTemplate_OverrideDoesNotMutateSlotData(t *testing.T) {
	app, sessionID := freshOverrideFixture(t)
	slot := &app.save.Slots[0]
	before := append([]byte(nil), slot.Data...)

	jsonText := overrideTemplateJSON(idStandardDagger, 0)
	_, err := app.ApplyBuildTemplateToWorkspaceJSON(sessionID, jsonText, ApplyTemplateOptions{
		WeaponLevelOverride: &WeaponLevelOverride{Enabled: true, StandardLevel: intPtr(20)},
	})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if len(slot.Data) != len(before) {
		t.Fatalf("slot.Data length changed: before=%d after=%d", len(before), len(slot.Data))
	}
	for i := range before {
		if slot.Data[i] != before[i] {
			t.Fatalf("slot.Data mutated at offset %d despite RAM-only apply", i)
		}
	}
}

func TestApplyTemplate_OverrideOnPreviewError_NoMutation(t *testing.T) {
	app, sessionID := freshOverrideFixture(t)
	// Unknown weapon id → preview fails. Override must not bypass that.
	badJSON := `{
  "schema": "saveforge.build-template",
  "version": 1,
  "createdAt": "2026-05-31T00:00:00Z",
  "sections": {
    "inventory.workspace": {
      "inventoryItems": [{
        "baseItemID": 3735928559,
        "quantity": 1,
        "container": "inventory",
        "position": 0
      }],
      "storageItems": []
    }
  }
}`
	res, err := app.ApplyBuildTemplateToWorkspaceJSON(sessionID, badJSON, ApplyTemplateOptions{
		WeaponLevelOverride: &WeaponLevelOverride{Enabled: true, StandardLevel: intPtr(15)},
	})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if res.Applied {
		t.Fatal("override must not bypass preview rejection")
	}
	if len(app.editSessions[sessionID].Workspace.InventoryItems) != 0 {
		t.Error("workspace mutated despite preview rejection")
	}
}

func TestApplyTemplate_OverrideClampZeroRequest_NoWarning(t *testing.T) {
	// requested == clamped == 0 must NOT emit a "clamped" warning, but
	// the weapon should still be patched to +0 (mirroring keep semantics
	// for that class).
	app, sessionID := freshOverrideFixture(t)
	jsonText := overrideTemplateJSON(idStandardDagger, 5)
	res, err := app.ApplyBuildTemplateToWorkspaceJSON(sessionID, jsonText, ApplyTemplateOptions{
		WeaponLevelOverride: &WeaponLevelOverride{Enabled: true, StandardLevel: intPtr(0)},
	})
	if err != nil || !res.Applied {
		t.Fatalf("apply: err=%v applied=%v", err, res.Applied)
	}
	if hasIssueCode(res.Preview.Warnings, templates.IssueCodeWeaponLevelClamped) {
		t.Errorf("clamp warning unexpected for 0→0 override, got %+v", res.Preview.Warnings)
	}
	got := findAddedWeapon(t, res.Workspace, idStandardDagger)
	if got.CurrentUpgrade != 0 {
		t.Errorf("override 0 must overwrite template +5, got +%d", got.CurrentUpgrade)
	}
}

// hasIssueCode is a tiny helper used by override tests to grep the
// emitted warning slice for a specific stable code. Kept out of
// (and not exported from) the apply path to keep the test file
// self-contained — the apply path emits structured issues, not
// formatted strings, so direct slice scans are the natural read.
func hasIssueCode(issues []templates.ImportPreviewIssue, code string) bool {
	for _, i := range issues {
		if i.Code == code {
			return true
		}
	}
	return false
}
