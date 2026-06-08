package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
	"github.com/oisis/EldenRing-SaveForge/backend/editor"
	"github.com/oisis/EldenRing-SaveForge/backend/templates"
)

// Phase 8A removed the public JSON template exchange. Tests for the
// removed App methods (ExportBuildTemplateJSON / ExportBuildTemplateToFile
// / PreviewBuildTemplateImportJSON / PreviewBuildTemplateImportFromFile /
// ApplyBuildTemplateToWorkspaceFromFile) are dropped along with the
// helpers they covered. Remaining tests target the internal helpers that
// underpin ApplyBuildTemplateFromLibrary (v1 library apply via canonical
// JSON), the marshalBuildTemplate canonical encoder used by the v2 apply
// pipeline, and the file-name / cancelled-preview helpers still in use.

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
	if !strings.Contains(string(data), "\n  ") {
		t.Errorf("expected indented JSON, got:\n%s", data)
	}
}

func TestCancelledPreviewReport_Shape(t *testing.T) {
	// Cancelled file dialog produces a report that the UI treats as a
	// no-op: not OK, no errors, no items. The contract is documented
	// in the YAML preview endpoint comments — this test guards it.
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

// ─── Internal apply helper: applyBuildTemplateToWorkspaceFromJSON ──────

// applyTemplateFixture spins up an App with one editable weapon already
// in the slot, builds a canonical JSON template from the workspace via
// the internal helpers, then feeds it back through the internal apply
// helper. Returns the app, source session ID, and the canonical JSON.
//
// Phase 8A: replaces the previous ExportBuildTemplateJSON-based fixture.
// The canonical JSON path stays exercised because ApplyBuildTemplateFromLibrary
// (still public) marshals library entries through the same helper.
func applyTemplateFixture(t *testing.T) (*App, string, string) {
	t.Helper()
	app := inventoryOrderFixture(testWeapons)
	snap, err := app.StartInventoryEditSession(0)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	tpl, _, err := app.buildAndValidateTemplate(snap.SessionID, BuildTemplateExportOptions{
		IncludeInventory: true,
	})
	if err != nil {
		t.Fatalf("buildAndValidateTemplate: %v", err)
	}
	data, err := marshalBuildTemplate(tpl)
	if err != nil {
		t.Fatalf("marshalBuildTemplate: %v", err)
	}
	return app, snap.SessionID, string(data)
}

func TestApplyTemplate_HappyPath_AppendsInventoryItems(t *testing.T) {
	app, sessionID, jsonText := applyTemplateFixture(t)
	beforeInv := len(app.editSessions[sessionID].Workspace.InventoryItems)

	res, err := app.applyBuildTemplateToWorkspaceFromJSON(sessionID, jsonText, ApplyTemplateOptions{Mode: "append"})
	if err != nil {
		t.Fatalf("applyBuildTemplateToWorkspaceFromJSON: %v", err)
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

	res, err := app.applyBuildTemplateToWorkspaceFromJSON(sessionID, jsonText, ApplyTemplateOptions{})
	if err != nil || !res.Applied {
		t.Fatalf("apply failed: %v / %+v", err, res.Preview)
	}

	inv := app.editSessions[sessionID].Workspace.InventoryItems
	if inv[0].BaseItemID != existingFirst {
		t.Errorf("existing first item shifted: got 0x%X, want 0x%X", inv[0].BaseItemID, existingFirst)
	}
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
	res, err := app.applyBuildTemplateToWorkspaceFromJSON(sessionID, jsonText, ApplyTemplateOptions{})
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

	if _, err := app.applyBuildTemplateToWorkspaceFromJSON(sessionID, jsonText, ApplyTemplateOptions{}); err != nil {
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

	res, err := app.applyBuildTemplateToWorkspaceFromJSON(snap.SessionID, templateJSON, ApplyTemplateOptions{})
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

	res, err := app.applyBuildTemplateToWorkspaceFromJSON(snap.SessionID, badJSON, ApplyTemplateOptions{})
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

	res, err := app.applyBuildTemplateToWorkspaceFromJSON(snap.SessionID, jsonText, ApplyTemplateOptions{})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if res.Applied {
		t.Fatal("Applied must be false on capacity overflow")
	}
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
	if got := len(app.editSessions[snap.SessionID].Workspace.InventoryItems); got != core.CommonItemCount {
		t.Errorf("workspace mutated on capacity reject: got %d, want %d", got, core.CommonItemCount)
	}
}

func TestApplyTemplate_InvalidModeReturnsError(t *testing.T) {
	app, sessionID, jsonText := applyTemplateFixture(t)
	beforeInv := len(app.editSessions[sessionID].Workspace.InventoryItems)
	if _, err := app.applyBuildTemplateToWorkspaceFromJSON(sessionID, jsonText, ApplyTemplateOptions{Mode: "replace-all"}); err == nil {
		t.Fatal("expected error for unsupported mode")
	}
	if got := len(app.editSessions[sessionID].Workspace.InventoryItems); got != beforeInv {
		t.Errorf("unsupported mode must not mutate workspace, got %d != %d", got, beforeInv)
	}
}

func TestApplyTemplate_UnknownSessionReturnsError(t *testing.T) {
	app := inventoryOrderFixture(testWeapons)
	_, err := app.applyBuildTemplateToWorkspaceFromJSON("ses-not-real", "{}", ApplyTemplateOptions{})
	if err == nil {
		t.Fatal("expected error for unknown session")
	}
}

func TestApplyTemplate_InvalidJSONReturnsStructureErrorNotGoError(t *testing.T) {
	app, sessionID, _ := applyTemplateFixture(t)
	res, err := app.applyBuildTemplateToWorkspaceFromJSON(sessionID, "not valid json", ApplyTemplateOptions{})
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

func TestApplyTemplate_SchemaV2ProfileOnlyIsRejectedNotPanicked(t *testing.T) {
	// v2 profile-only templates parse and validate successfully under the
	// Phase 3A structural schema but carry no sections.inventory.workspace.
	// The v1 apply path dereferences sec.InventoryItems unconditionally;
	// without the v2 guard this call would panic. Assert: structured error,
	// no panic, no workspace mutation.
	app, sessionID, _ := applyTemplateFixture(t)
	beforeInv := len(app.editSessions[sessionID].Workspace.InventoryItems)
	beforeStorage := len(app.editSessions[sessionID].Workspace.StorageItems)
	beforeDirty := app.editSessions[sessionID].Workspace.Dirty

	v2JSON := `{
  "schema": "saveforge.build-template",
  "version": 2,
  "createdAt": "2026-05-30T12:00:00Z",
  "selection": { "profile": true },
  "sections": {
    "profile": { "level": 50 }
  }
}`

	res, err := app.applyBuildTemplateToWorkspaceFromJSON(sessionID, v2JSON, ApplyTemplateOptions{})
	if err != nil {
		t.Fatalf("v2 apply must surface preview errors via report, not Go error; got %v", err)
	}
	if res.Applied {
		t.Fatal("Applied must be false for v2 templates in this phase")
	}
	if res.Preview.OK {
		t.Error("Preview.OK must be false when apply is blocked")
	}
	if len(res.Preview.Errors) != 1 {
		t.Fatalf("expected exactly one error, got %+v", res.Preview.Errors)
	}
	if res.Preview.Errors[0].Code != templates.IssueCodeStructureInvalid {
		t.Errorf("expected structure_invalid code, got %q", res.Preview.Errors[0].Code)
	}
	if !strings.Contains(res.Preview.Errors[0].Message, "schema v2") {
		t.Errorf("error message must mention schema v2; got %q", res.Preview.Errors[0].Message)
	}
	if !strings.Contains(res.Preview.Errors[0].Message, "not yet supported") {
		t.Errorf("error message must mark v2 apply as unsupported; got %q", res.Preview.Errors[0].Message)
	}

	if got := len(app.editSessions[sessionID].Workspace.InventoryItems); got != beforeInv {
		t.Errorf("workspace inventory mutated despite v2 block: %d -> %d", beforeInv, got)
	}
	if got := len(app.editSessions[sessionID].Workspace.StorageItems); got != beforeStorage {
		t.Errorf("workspace storage mutated despite v2 block: %d -> %d", beforeStorage, got)
	}
	if app.editSessions[sessionID].Workspace.Dirty != beforeDirty {
		t.Errorf("workspace.Dirty flipped despite v2 block: %v -> %v",
			beforeDirty, app.editSessions[sessionID].Workspace.Dirty)
	}
}

func TestApplyTemplate_RoundTripThroughSavePersistsImportedItems(t *testing.T) {
	app, charIdx := realSaveAppForSave(t)
	snap, err := app.StartInventoryEditSession(charIdx)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

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
	res, err := app.applyBuildTemplateToWorkspaceFromJSON(snap.SessionID, jsonText, ApplyTemplateOptions{})
	if err != nil || !res.Applied {
		t.Fatalf("apply: err=%v preview=%+v", err, res.Preview)
	}

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
