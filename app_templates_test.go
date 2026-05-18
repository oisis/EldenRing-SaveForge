package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
	"github.com/oisis/EldenRing-SaveForge/backend/editor"
	"github.com/oisis/EldenRing-SaveForge/backend/templates"
)

func TestExportBuildTemplateJSON_HappyPath_Both(t *testing.T) {
	app := inventoryOrderFixture(testWeapons)
	snap, err := app.StartInventoryEditSession(0)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	res, err := app.ExportBuildTemplateJSON(snap.SessionID, BuildTemplateExportOptions{
		IncludeInventory: true,
		IncludeStorage:   true,
		Name:             "Test Build",
		Description:      "Phase B happy path",
		Author:           "OiSiS",
		Tags:             []string{"test", "phase-b"},
	})
	if err != nil {
		t.Fatalf("ExportBuildTemplateJSON: %v", err)
	}
	if res.Path != "" {
		t.Errorf("Path must be empty for JSON-only export, got %q", res.Path)
	}
	if res.JSON == "" {
		t.Fatal("JSON payload must be non-empty")
	}

	var tpl templates.BuildTemplate
	if err := json.Unmarshal([]byte(res.JSON), &tpl); err != nil {
		t.Fatalf("returned JSON does not decode: %v\npayload: %s", err, res.JSON)
	}
	if tpl.Schema != templates.SchemaKey || tpl.Version != templates.SchemaVersion {
		t.Errorf("schema header wrong: %q v%d", tpl.Schema, tpl.Version)
	}
	if tpl.Metadata == nil || tpl.Metadata.Name != "Test Build" || tpl.Metadata.Author != "OiSiS" {
		t.Errorf("metadata not propagated: %+v", tpl.Metadata)
	}
	if tpl.Metadata.SourceCharacterIndex != 0 {
		t.Errorf("SourceCharacterIndex = %d, want 0", tpl.Metadata.SourceCharacterIndex)
	}
	if tpl.AppVersion == "" {
		t.Error("AppVersion must be populated from main package")
	}
	if tpl.Sections.InventoryWorkspace == nil {
		t.Fatal("inventory.workspace section missing")
	}
	if len(tpl.Sections.InventoryWorkspace.InventoryItems) != len(testWeapons) {
		t.Errorf("inventoryItems = %d, want %d", len(tpl.Sections.InventoryWorkspace.InventoryItems), len(testWeapons))
	}
	if err := templates.ValidateBuildTemplate(&tpl); err != nil {
		t.Errorf("exported template fails validation: %v", err)
	}
}

func TestExportBuildTemplateJSON_InventoryOnly(t *testing.T) {
	app := inventoryOrderFixture(testWeapons)
	snap, err := app.StartInventoryEditSession(0)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	res, err := app.ExportBuildTemplateJSON(snap.SessionID, BuildTemplateExportOptions{
		IncludeInventory: true,
		IncludeStorage:   false,
	})
	if err != nil {
		t.Fatalf("ExportBuildTemplateJSON: %v", err)
	}

	var tpl templates.BuildTemplate
	if err := json.Unmarshal([]byte(res.JSON), &tpl); err != nil {
		t.Fatalf("decode: %v", err)
	}
	sec := tpl.Sections.InventoryWorkspace
	if len(sec.InventoryItems) == 0 {
		t.Error("inventoryItems must be populated")
	}
	if len(sec.StorageItems) != 0 {
		t.Errorf("storageItems must be empty for inventory-only, got %d", len(sec.StorageItems))
	}
}

func TestExportBuildTemplateJSON_StorageOnly(t *testing.T) {
	app := inventoryOrderFixture(testWeapons)
	snap, err := app.StartInventoryEditSession(0)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Storage is empty in this fixture, so an export including storage
	// alone yields an empty payload that ValidateBuildTemplate rejects.
	// That rejection is the correct user-facing behaviour and the test
	// asserts on it.
	_, err = app.ExportBuildTemplateJSON(snap.SessionID, BuildTemplateExportOptions{
		IncludeInventory: false,
		IncludeStorage:   true,
	})
	if err == nil {
		t.Fatal("expected validation error for empty template (storage-only on inventory-only fixture)")
	}
	if !strings.Contains(err.Error(), "empty") && !strings.Contains(err.Error(), "validate") {
		t.Errorf("error should mention empty/validate, got: %v", err)
	}
}

func TestExportBuildTemplateJSON_UnknownSession(t *testing.T) {
	app := inventoryOrderFixture(testWeapons)
	_, err := app.ExportBuildTemplateJSON("ses-not-real", BuildTemplateExportOptions{
		IncludeInventory: true,
	})
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("want 'not found' error, got %v", err)
	}
}

func TestExportBuildTemplateJSON_BothExcluded(t *testing.T) {
	app := inventoryOrderFixture(testWeapons)
	snap, err := app.StartInventoryEditSession(0)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	_, err = app.ExportBuildTemplateJSON(snap.SessionID, BuildTemplateExportOptions{
		IncludeInventory: false,
		IncludeStorage:   false,
	})
	if err == nil {
		t.Fatal("expected error when both containers excluded")
	}
}

// TestExportBuildTemplateJSON_DirtyWorkspaceExports verifies that an
// active session with dirty=true can still produce a template — this is
// the whole point of exporting before Save changes.
func TestExportBuildTemplateJSON_DirtyWorkspaceExports(t *testing.T) {
	app := inventoryOrderFixture(testWeapons)
	snap, err := app.StartInventoryEditSession(0)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	// Force-mark the session dirty without going through a real mutation —
	// the export path does not care HOW dirty became true, only that it
	// does not block on it.
	app.editSessions[snap.SessionID].Workspace.Dirty = true

	res, err := app.ExportBuildTemplateJSON(snap.SessionID, BuildTemplateExportOptions{
		IncludeInventory: true,
	})
	if err != nil {
		t.Fatalf("dirty workspace export must not error: %v", err)
	}
	if res.JSON == "" {
		t.Fatal("JSON must be produced for dirty workspace")
	}

	// Sanity: dirty flag was not silently cleared by the export.
	if !app.editSessions[snap.SessionID].Workspace.Dirty {
		t.Error("export must not mutate workspace dirty flag")
	}
}

// TestExportBuildTemplateJSON_OmitsHandlesAndSessionFields is the
// regression guard at the App-method boundary: even when going through
// the full Wails-level encoder, none of the forbidden save-local fields
// from EditableItem may leak into the produced JSON.
func TestExportBuildTemplateJSON_OmitsHandlesAndSessionFields(t *testing.T) {
	app := inventoryOrderFixture(testWeapons)
	snap, err := app.StartInventoryEditSession(0)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	res, err := app.ExportBuildTemplateJSON(snap.SessionID, BuildTemplateExportOptions{
		IncludeInventory: true,
	})
	if err != nil {
		t.Fatalf("export: %v", err)
	}

	forbidden := []string{
		"originalHandle",
		"currentAoWHandle",
		"\"uid\"",
		"acquisitionIndex",
		"pendingAoWItemID",
		"pendingAoWName",
		"pendingAoWClear",
		"hasGaItem",
		"hasCurrentAoW",
		"currentAoWStatus",
		"hasPendingWeaponPatch",
		"isWeapon",
		"isArmor",
		"isTalisman",
		"maxUpgrade",
	}
	for _, key := range forbidden {
		if strings.Contains(res.JSON, key) {
			t.Errorf("forbidden field %q leaked into App-level JSON:\n%s", key, res.JSON)
		}
	}
}

func TestExportBuildTemplateJSON_PropagatesWarnings(t *testing.T) {
	app := inventoryOrderFixture(testWeapons)
	snap, err := app.StartInventoryEditSession(0)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Reach into the session and inject a "missing" AoW state on the
	// first inventory weapon — the BuildTemplateFromSnapshot path must
	// surface this as an aow_missing_skipped warning via the App-level
	// result.
	sess := app.editSessions[snap.SessionID]
	if len(sess.Workspace.InventoryItems) == 0 {
		t.Fatal("fixture must have inventory items")
	}
	sess.Workspace.InventoryItems[0].IsWeapon = true
	sess.Workspace.InventoryItems[0].CurrentAoWStatus = "missing"

	res, err := app.ExportBuildTemplateJSON(snap.SessionID, BuildTemplateExportOptions{
		IncludeInventory: true,
	})
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if len(res.Warnings) == 0 {
		t.Fatal("expected at least one warning for missing AoW")
	}
	found := false
	for _, w := range res.Warnings {
		if w.Code == templates.WarnCodeAoWMissingSkipped {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected aow_missing_skipped warning, got %+v", res.Warnings)
	}
}

func TestWriteBuildTemplateFile_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test-template.json")
	data := []byte(`{"schema":"saveforge.build-template","version":1}`)
	if err := writeBuildTemplateFile(path, data); err != nil {
		t.Fatalf("writeBuildTemplateFile: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != string(data) {
		t.Errorf("payload mismatch:\nwant %s\ngot  %s", data, got)
	}
	stat, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if mode := stat.Mode().Perm(); mode != 0644 {
		t.Errorf("file mode = %o, want 0644", mode)
	}
}

func TestWriteBuildTemplateFile_RejectsEmptyPath(t *testing.T) {
	if err := writeBuildTemplateFile("", []byte("{}")); err == nil {
		t.Fatal("expected error for empty path")
	}
}

func TestMarshalBuildTemplate_IsIndented(t *testing.T) {
	tpl := &templates.BuildTemplate{
		Schema:    templates.SchemaKey,
		Version:   templates.SchemaVersion,
		CreatedAt: "2026-05-17T12:34:56Z",
		Sections: templates.TemplateSections{
			InventoryWorkspace: &templates.InventoryWorkspaceSection{
				InventoryItems: []templates.TemplateItem{{
					BaseItemID: 0x000F4240,
					Quantity:   1,
					Container:  templates.ContainerInventory,
					Position:   0,
				}},
				StorageItems: []templates.TemplateItem{},
			},
		},
	}
	data, err := marshalBuildTemplate(tpl)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	// Indented output must contain newlines and two-space indentation —
	// the file is meant to be human-readable.
	if !strings.Contains(string(data), "\n  ") {
		t.Errorf("expected indented JSON, got:\n%s", data)
	}
}

// ─── Phase C: PreviewBuildTemplateImportJSON ────────────────────────────

func TestPreviewBuildTemplateImportJSON_HappyPath(t *testing.T) {
	// Export a real workspace as JSON, then preview that JSON. End-to-end
	// round trip — guarantees the App-level export and import contracts
	// agree on the wire format.
	app := inventoryOrderFixture(testWeapons)
	snap, err := app.StartInventoryEditSession(0)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	exp, err := app.ExportBuildTemplateJSON(snap.SessionID, BuildTemplateExportOptions{
		IncludeInventory: true,
	})
	if err != nil {
		t.Fatalf("export: %v", err)
	}

	rep, err := app.PreviewBuildTemplateImportJSON(exp.JSON)
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if !rep.OK {
		t.Fatalf("expected OK, got errors %+v", rep.Errors)
	}
	if rep.Summary.Weapons != len(testWeapons) {
		t.Errorf("Summary.Weapons = %d, want %d", rep.Summary.Weapons, len(testWeapons))
	}
}

func TestPreviewBuildTemplateImportJSON_InvalidJSON(t *testing.T) {
	app := inventoryOrderFixture(testWeapons)
	rep, err := app.PreviewBuildTemplateImportJSON("not json at all")
	if err != nil {
		t.Fatalf("preview must not error on malformed JSON, returned %v", err)
	}
	if rep.OK {
		t.Fatal("malformed JSON must produce a NOT-OK report")
	}
	if len(rep.Errors) != 1 || rep.Errors[0].Code != templates.IssueCodeStructureInvalid {
		t.Errorf("expected one structure_invalid error, got %+v", rep.Errors)
	}
}

func TestPreviewBuildTemplateImportJSON_WrongSchema(t *testing.T) {
	app := inventoryOrderFixture(testWeapons)
	bad := `{"schema":"wrong","version":1,"createdAt":"2026-05-17T12:34:56Z","sections":{"inventory.workspace":{"inventoryItems":[{"baseItemID":15990336,"quantity":1,"container":"inventory","position":0}],"storageItems":[]}}}`
	rep, err := app.PreviewBuildTemplateImportJSON(bad)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rep.OK {
		t.Fatal("wrong schema must produce NOT-OK")
	}
	if len(rep.Errors) != 1 || rep.Errors[0].Code != templates.IssueCodeStructureInvalid {
		t.Errorf("expected structure_invalid, got %+v", rep.Errors)
	}
}

func TestPreviewBuildTemplateImportJSON_DoesNotMutateSession(t *testing.T) {
	app := inventoryOrderFixture(testWeapons)
	snap, err := app.StartInventoryEditSession(0)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	exp, err := app.ExportBuildTemplateJSON(snap.SessionID, BuildTemplateExportOptions{
		IncludeInventory: true,
	})
	if err != nil {
		t.Fatalf("export: %v", err)
	}

	dirtyBefore := app.editSessions[snap.SessionID].Workspace.Dirty
	invCountBefore := len(app.editSessions[snap.SessionID].Workspace.InventoryItems)

	if _, err := app.PreviewBuildTemplateImportJSON(exp.JSON); err != nil {
		t.Fatalf("preview: %v", err)
	}

	if app.editSessions[snap.SessionID].Workspace.Dirty != dirtyBefore {
		t.Errorf("preview mutated workspace.Dirty (before=%v after=%v)",
			dirtyBefore, app.editSessions[snap.SessionID].Workspace.Dirty)
	}
	if got := len(app.editSessions[snap.SessionID].Workspace.InventoryItems); got != invCountBefore {
		t.Errorf("preview changed inventory size (before=%d after=%d)", invCountBefore, got)
	}
}

func TestCancelledPreviewReport_Shape(t *testing.T) {
	// Cancelled file dialog produces a report that the UI treats as a
	// no-op: not OK, no errors, no items. The contract is documented
	// in the file dialog endpoint comments — this test guards it.
	rep := cancelledPreviewReport()
	if rep.OK {
		t.Error("cancelled report must be OK=false")
	}
	if len(rep.Errors) != 0 || len(rep.Warnings) != 0 {
		t.Errorf("cancelled report must carry no issues, got %+v", rep)
	}
	if rep.Summary.InventoryItems != 0 || rep.Summary.StorageItems != 0 {
		t.Errorf("cancelled report must carry no items, got %+v", rep.Summary)
	}
}

// ─── Phase D: ApplyBuildTemplateToWorkspaceJSON ──────────────────────────

// applyTemplateFixture spins up an App with one editable weapon already
// in the slot, exports the workspace as JSON, then resets the workspace
// state so apply has a clean target. Returns the app, source session ID,
// and the exported JSON ready to feed back through Apply.
func applyTemplateFixture(t *testing.T) (*App, string, string) {
	t.Helper()
	app := inventoryOrderFixture(testWeapons)
	snap, err := app.StartInventoryEditSession(0)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	exp, err := app.ExportBuildTemplateJSON(snap.SessionID, BuildTemplateExportOptions{
		IncludeInventory: true,
	})
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	return app, snap.SessionID, exp.JSON
}

func TestApplyTemplate_HappyPath_AppendsInventoryItems(t *testing.T) {
	app, sessionID, jsonText := applyTemplateFixture(t)
	beforeInv := len(app.editSessions[sessionID].Workspace.InventoryItems)

	res, err := app.ApplyBuildTemplateToWorkspaceJSON(sessionID, jsonText, ApplyTemplateOptions{Mode: "append"})
	if err != nil {
		t.Fatalf("ApplyBuildTemplateToWorkspaceJSON: %v", err)
	}
	if !res.Applied {
		t.Fatalf("Applied=false; preview=%+v", res.Preview)
	}
	if !res.Preview.OK {
		t.Errorf("Preview should be OK on happy path, got %+v", res.Preview)
	}

	afterInv := len(app.editSessions[sessionID].Workspace.InventoryItems)
	if afterInv != beforeInv+len(testWeapons) {
		t.Errorf("inventory size = %d, want %d + %d added", afterInv, beforeInv, len(testWeapons))
	}
}

func TestApplyTemplate_PreservesRelativeOrderAndAppendsAfterExisting(t *testing.T) {
	app, sessionID, jsonText := applyTemplateFixture(t)
	existingFirst := app.editSessions[sessionID].Workspace.InventoryItems[0].BaseItemID
	beforeCount := len(app.editSessions[sessionID].Workspace.InventoryItems)

	res, err := app.ApplyBuildTemplateToWorkspaceJSON(sessionID, jsonText, ApplyTemplateOptions{})
	if err != nil || !res.Applied {
		t.Fatalf("apply failed: %v / %+v", err, res.Preview)
	}

	inv := app.editSessions[sessionID].Workspace.InventoryItems
	if inv[0].BaseItemID != existingFirst {
		t.Errorf("existing first item shifted: got 0x%X, want 0x%X", inv[0].BaseItemID, existingFirst)
	}
	// Imported items follow in the same order as testWeapons.
	for i, w := range testWeapons {
		appended := inv[beforeCount+i]
		if appended.BaseItemID != w.itemID {
			t.Errorf("appended item %d baseItemID = 0x%X, want 0x%X", i, appended.BaseItemID, w.itemID)
		}
		if appended.Source != "added" {
			t.Errorf("appended item %d source = %q, want \"added\"", i, appended.Source)
		}
	}
}

func TestApplyTemplate_SetsWorkspaceDirty(t *testing.T) {
	app, sessionID, jsonText := applyTemplateFixture(t)
	res, err := app.ApplyBuildTemplateToWorkspaceJSON(sessionID, jsonText, ApplyTemplateOptions{})
	if err != nil || !res.Applied {
		t.Fatalf("apply failed: %v / %+v", err, res.Preview)
	}
	if !app.editSessions[sessionID].Workspace.Dirty {
		t.Error("workspace.Dirty must be true after apply")
	}
	if !res.Workspace.Dirty {
		t.Error("returned snapshot must reflect Dirty=true")
	}
}

func TestApplyTemplate_DoesNotMutateSlotData(t *testing.T) {
	app, sessionID, jsonText := applyTemplateFixture(t)
	slot := &app.save.Slots[0]
	dataBefore := append([]byte(nil), slot.Data...)

	if _, err := app.ApplyBuildTemplateToWorkspaceJSON(sessionID, jsonText, ApplyTemplateOptions{}); err != nil {
		t.Fatalf("apply: %v", err)
	}

	if len(slot.Data) != len(dataBefore) {
		t.Fatalf("slot.Data length changed (before=%d after=%d)", len(dataBefore), len(slot.Data))
	}
	for i := range dataBefore {
		if slot.Data[i] != dataBefore[i] {
			t.Fatalf("slot.Data mutated at offset %d", i)
		}
	}
}

func TestApplyTemplate_WeaponUpgradeInfusionAoWAppliedAsPendingState(t *testing.T) {
	app := inventoryOrderFixture(testWeapons)
	snap, err := app.StartInventoryEditSession(0)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Hand-craft a template with one upgraded + infused + AoW-assigned
	// dagger. Dagger + Sword Dance is known-compatible per
	// backend/db/compat_test.go.
	aow := uint32(0x80003070)
	templateJSON := `{
  "schema": "saveforge.build-template",
  "version": 1,
  "createdAt": "2026-05-17T12:34:56Z",
  "sections": {
    "inventory.workspace": {
      "inventoryItems": [{
        "baseItemID": 1000000,
        "quantity": 1,
        "upgrade": 5,
        "infusionName": "Heavy",
        "aowItemID": ` + fmt.Sprintf("%d", aow) + `,
        "container": "inventory",
        "position": 0
      }],
      "storageItems": []
    }
  }
}`

	res, err := app.ApplyBuildTemplateToWorkspaceJSON(snap.SessionID, templateJSON, ApplyTemplateOptions{})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if !res.Applied {
		t.Fatalf("Apply failed: %+v", res.Preview)
	}

	inv := app.editSessions[snap.SessionID].Workspace.InventoryItems
	added := inv[len(inv)-1]
	if added.CurrentUpgrade != 5 {
		t.Errorf("CurrentUpgrade = %d, want 5", added.CurrentUpgrade)
	}
	if added.InfusionName != "Heavy" {
		t.Errorf("InfusionName = %q, want \"Heavy\"", added.InfusionName)
	}
	if added.PendingAoWItemID != aow {
		t.Errorf("PendingAoWItemID = 0x%X, want 0x%X", added.PendingAoWItemID, aow)
	}
	if !added.HasPendingWeaponPatch {
		t.Error("HasPendingWeaponPatch must be true after AoW patch")
	}
}

func TestApplyTemplate_PreviewErrorsBlockApplyAndKeepWorkspaceClean(t *testing.T) {
	app := inventoryOrderFixture(testWeapons)
	snap, err := app.StartInventoryEditSession(0)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	beforeInv := len(app.editSessions[snap.SessionID].Workspace.InventoryItems)
	beforeDirty := app.editSessions[snap.SessionID].Workspace.Dirty

	// Template references an unknown item ID — preview will fail.
	badJSON := `{
  "schema": "saveforge.build-template",
  "version": 1,
  "createdAt": "2026-05-17T12:34:56Z",
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

	res, err := app.ApplyBuildTemplateToWorkspaceJSON(snap.SessionID, badJSON, ApplyTemplateOptions{})
	if err != nil {
		t.Fatalf("apply must surface preview errors via report, not Go error; got %v", err)
	}
	if res.Applied {
		t.Fatal("Applied must be false when preview reports errors")
	}
	if res.Preview.OK {
		t.Error("Preview.OK must be false")
	}
	if got := len(app.editSessions[snap.SessionID].Workspace.InventoryItems); got != beforeInv {
		t.Errorf("workspace mutated despite preview errors: inv %d -> %d", beforeInv, got)
	}
	if app.editSessions[snap.SessionID].Workspace.Dirty != beforeDirty {
		t.Errorf("workspace.Dirty changed despite preview errors: %v -> %v",
			beforeDirty, app.editSessions[snap.SessionID].Workspace.Dirty)
	}
}

func TestApplyTemplate_CapacityOverflowBlocksApplyAndIsReportedAsError(t *testing.T) {
	app := inventoryOrderFixture(testWeapons)
	snap, err := app.StartInventoryEditSession(0)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Forge a workspace that already sits at the inventory cap. The
	// capacity preflight must reject the import before any AddItem
	// runs.
	sess := app.editSessions[snap.SessionID]
	sess.Workspace.InventoryItems = make([]editor.EditableItem, core.CommonItemCount)

	jsonText := `{
  "schema": "saveforge.build-template",
  "version": 1,
  "createdAt": "2026-05-17T12:34:56Z",
  "sections": {
    "inventory.workspace": {
      "inventoryItems": [{
        "baseItemID": 1000000,
        "quantity": 1,
        "container": "inventory",
        "position": 0
      }],
      "storageItems": []
    }
  }
}`

	res, err := app.ApplyBuildTemplateToWorkspaceJSON(snap.SessionID, jsonText, ApplyTemplateOptions{})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if res.Applied {
		t.Fatal("Applied must be false on capacity overflow")
	}
	// Confirm capacity_exceeded is the surfaced reason.
	found := false
	for _, e := range res.Preview.Errors {
		if e.Code == templates.IssueCodeCapacityExceeded {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected capacity_exceeded error, got %+v", res.Preview.Errors)
	}
	// Workspace must still hold exactly the pre-apply inventory.
	if got := len(app.editSessions[snap.SessionID].Workspace.InventoryItems); got != core.CommonItemCount {
		t.Errorf("workspace mutated on capacity reject: got %d, want %d", got, core.CommonItemCount)
	}
}

func TestApplyTemplate_InvalidModeReturnsError(t *testing.T) {
	app, sessionID, jsonText := applyTemplateFixture(t)
	beforeInv := len(app.editSessions[sessionID].Workspace.InventoryItems)
	if _, err := app.ApplyBuildTemplateToWorkspaceJSON(sessionID, jsonText, ApplyTemplateOptions{Mode: "replace-all"}); err == nil {
		t.Fatal("expected error for unsupported mode")
	}
	if got := len(app.editSessions[sessionID].Workspace.InventoryItems); got != beforeInv {
		t.Errorf("unsupported mode must not mutate workspace, got %d != %d", got, beforeInv)
	}
}

func TestApplyTemplate_UnknownSessionReturnsError(t *testing.T) {
	app := inventoryOrderFixture(testWeapons)
	_, err := app.ApplyBuildTemplateToWorkspaceJSON("ses-not-real", "{}", ApplyTemplateOptions{})
	if err == nil {
		t.Fatal("expected error for unknown session")
	}
}

func TestApplyTemplate_InvalidJSONReturnsStructureErrorNotGoError(t *testing.T) {
	app, sessionID, _ := applyTemplateFixture(t)
	res, err := app.ApplyBuildTemplateToWorkspaceJSON(sessionID, "not valid json", ApplyTemplateOptions{})
	if err != nil {
		t.Fatalf("malformed JSON must not produce Go error, got %v", err)
	}
	if res.Applied {
		t.Fatal("Applied must be false for malformed JSON")
	}
	if len(res.Preview.Errors) != 1 || res.Preview.Errors[0].Code != templates.IssueCodeStructureInvalid {
		t.Errorf("expected one structure_invalid issue, got %+v", res.Preview.Errors)
	}
}

func TestApplyTemplate_RoundTripThroughSavePersistsImportedItems(t *testing.T) {
	// End-to-end: export → apply → SaveInventoryWorkspaceChanges →
	// re-snapshot. Imported items must persist with fresh non-zero
	// handles assigned by the existing handle allocator. The minimal
	// inventoryOrderFixture does not provision a GaItems array large
	// enough for new allocations, so this test rides the same real
	// save fixture as the other Save tests and skips when absent.
	app, charIdx := realSaveAppForSave(t)
	snap, err := app.StartInventoryEditSession(charIdx)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	// One-item template (Dagger 0x000F4240) — narrow assertions.
	jsonText := `{
  "schema": "saveforge.build-template",
  "version": 1,
  "createdAt": "2026-05-17T12:34:56Z",
  "sections": {
    "inventory.workspace": {
      "inventoryItems": [{
        "baseItemID": 1000000,
        "quantity": 1,
        "container": "inventory",
        "position": 0
      }],
      "storageItems": []
    }
  }
}`
	res, err := app.ApplyBuildTemplateToWorkspaceJSON(snap.SessionID, jsonText, ApplyTemplateOptions{})
	if err != nil || !res.Applied {
		t.Fatalf("apply: err=%v preview=%+v", err, res.Preview)
	}

	// All imported items currently have OriginalHandle=0 — Save must
	// assign fresh non-zero handles, NOT reuse anything from the
	// source. The shape check captures the invariant.
	for _, it := range app.editSessions[snap.SessionID].Workspace.InventoryItems {
		if it.Source == "added" && it.OriginalHandle != 0 {
			t.Errorf("imported item has non-zero OriginalHandle pre-save: %+v", it)
		}
	}

	saved, err := app.SaveInventoryWorkspaceChanges(snap.SessionID)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if saved.Dirty {
		t.Error("post-save snapshot should not be dirty")
	}
	// Imported Dagger must now be present with a non-zero handle.
	found := false
	for _, it := range saved.InventoryItems {
		if it.BaseItemID == 0x000F4240 && it.OriginalHandle != 0 {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("imported dagger missing or has zero handle after save")
	}
}

func TestApplyTemplate_FromFileCancelledReturnsSentinelResult(t *testing.T) {
	// Drive the cancellation path via the internal helper rather than
	// invoking the GUI dialog. Mirrors how cancelledPreviewReport is
	// tested.
	app := inventoryOrderFixture(testWeapons)
	snap, err := app.StartInventoryEditSession(0)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	res := cancelledApplyResult(app, snap.SessionID)
	if res.Applied {
		t.Error("cancelled apply must be Applied=false")
	}
	if res.Workspace.SessionID != snap.SessionID {
		t.Errorf("cancelled result must echo current workspace, got %q want %q", res.Workspace.SessionID, snap.SessionID)
	}
	if len(res.Preview.Errors) != 0 || len(res.Preview.Warnings) != 0 {
		t.Errorf("cancelled apply must carry no preview issues, got %+v", res.Preview)
	}
}

func TestDefaultTemplateFilename(t *testing.T) {
	cases := []struct {
		desc string
		tpl  *templates.BuildTemplate
		want string
	}{
		{
			desc: "nil template falls back to generic name",
			tpl:  nil,
			want: "saveforge-build-template.json",
		},
		{
			desc: "metadata.Name is preferred when set",
			tpl: &templates.BuildTemplate{Metadata: &templates.TemplateMetadata{
				Name: "RL150 Quality",
			}},
			want: "RL150-Quality.json",
		},
		{
			desc: "source character name when no template name",
			tpl: &templates.BuildTemplate{Metadata: &templates.TemplateMetadata{
				SourceCharacterName: "Tarnished",
			}},
			want: "Tarnished-build.json",
		},
		{
			desc: "unsafe characters are sanitised",
			tpl: &templates.BuildTemplate{Metadata: &templates.TemplateMetadata{
				Name: "build / with: weird*chars",
			}},
			want: "build-with-weird-chars.json",
		},
		{
			desc: "all-unsafe name falls back to generic",
			tpl: &templates.BuildTemplate{Metadata: &templates.TemplateMetadata{
				Name: "///***",
			}},
			want: "saveforge-build-template.json",
		},
		{
			desc: "empty metadata falls back to generic",
			tpl:  &templates.BuildTemplate{Metadata: &templates.TemplateMetadata{}},
			want: "saveforge-build-template.json",
		},
	}
	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			got := defaultTemplateFilename(tc.tpl)
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}
