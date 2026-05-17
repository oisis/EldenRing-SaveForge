package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

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
