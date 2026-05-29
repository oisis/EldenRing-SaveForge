package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/templates"
)

// libraryFixture wires inventoryOrderFixture with a TemplateLibrary
// rooted at t.TempDir() so tests never touch the real user config dir.
// Returns the app, an active session ID, and a freshly-saved template's
// ID so each test can pick what it wants to exercise.
func libraryFixture(t *testing.T) (*App, string, string) {
	t.Helper()
	app := inventoryOrderFixture(testWeapons)
	app.templateLibrary = templates.NewTemplateLibrary(t.TempDir())
	snap, err := app.StartInventoryEditSession(0)
	if err != nil {
		t.Fatalf("StartInventoryEditSession: %v", err)
	}
	entry, err := app.SaveBuildTemplateToLibrary(snap.SessionID, BuildTemplateExportOptions{
		IncludeInventory: true,
		Name:             "fixture template",
		Description:      "set up by test",
		Tags:             []string{"unit"},
	})
	if err != nil {
		t.Fatalf("SaveBuildTemplateToLibrary: %v", err)
	}
	return app, snap.SessionID, entry.ID
}

func TestSaveBuildTemplateToLibrary_CreatesEntry(t *testing.T) {
	app, sessionID, _ := libraryFixture(t)
	entry, err := app.SaveBuildTemplateToLibrary(sessionID, BuildTemplateExportOptions{
		IncludeInventory: true,
		Name:             "another",
	})
	if err != nil {
		t.Fatalf("SaveBuildTemplateToLibrary: %v", err)
	}
	if entry.ID == "" {
		t.Errorf("ID empty")
	}
	if entry.Name != "another" {
		t.Errorf("Name=%q", entry.Name)
	}
}

func TestSaveBuildTemplateToLibrary_RejectsUnknownSession(t *testing.T) {
	app, _, _ := libraryFixture(t)
	if _, err := app.SaveBuildTemplateToLibrary("does-not-exist", BuildTemplateExportOptions{
		IncludeInventory: true,
	}); err == nil {
		t.Fatal("expected error for unknown session")
	}
}

func TestSaveBuildTemplateToLibrary_DoesNotMutateWorkspace(t *testing.T) {
	app, sessionID, _ := libraryFixture(t)
	before := len(app.editSessions[sessionID].Workspace.InventoryItems)
	dirtyBefore := app.editSessions[sessionID].Workspace.Dirty
	if _, err := app.SaveBuildTemplateToLibrary(sessionID, BuildTemplateExportOptions{
		IncludeInventory: true,
	}); err != nil {
		t.Fatalf("save: %v", err)
	}
	after := len(app.editSessions[sessionID].Workspace.InventoryItems)
	if after != before {
		t.Errorf("workspace mutated: %d -> %d", before, after)
	}
	if app.editSessions[sessionID].Workspace.Dirty != dirtyBefore {
		t.Errorf("dirty flag flipped")
	}
}

func TestListBuildTemplateLibrary_ReturnsSavedEntries(t *testing.T) {
	app, _, fixtureID := libraryFixture(t)
	entries, err := app.ListBuildTemplateLibrary()
	if err != nil {
		t.Fatalf("ListBuildTemplateLibrary: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("want 1 entry, got %d", len(entries))
	}
	if entries[0].ID != fixtureID {
		t.Errorf("entry ID mismatch: got %q want %q", entries[0].ID, fixtureID)
	}
}

func TestPreviewBuildTemplateFromLibrary_ReturnsOKReport(t *testing.T) {
	app, _, id := libraryFixture(t)
	preview, err := app.PreviewBuildTemplateFromLibrary(id)
	if err != nil {
		t.Fatalf("PreviewBuildTemplateFromLibrary: %v", err)
	}
	if !preview.Report.OK {
		t.Errorf("preview not OK: %+v", preview.Report)
	}
	if preview.JSON == "" {
		t.Errorf("preview JSON should be populated for the Apply round-trip")
	}
	if preview.Path != id {
		t.Errorf("preview Path should echo ID for the library path: got %q want %q", preview.Path, id)
	}
}

func TestPreviewBuildTemplateFromLibrary_UnknownIDErrors(t *testing.T) {
	app, _, _ := libraryFixture(t)
	if _, err := app.PreviewBuildTemplateFromLibrary("not-an-id"); err == nil {
		t.Fatal("expected error for unknown id")
	}
}

func TestApplyBuildTemplateFromLibrary_MutatesWorkspaceOnly(t *testing.T) {
	app, sessionID, id := libraryFixture(t)
	beforeInv := len(app.editSessions[sessionID].Workspace.InventoryItems)
	slotDataBefore := append([]byte(nil), app.save.Slots[0].Data...)

	res, err := app.ApplyBuildTemplateFromLibrary(sessionID, id, ApplyTemplateOptions{Mode: "append"})
	if err != nil {
		t.Fatalf("ApplyBuildTemplateFromLibrary: %v", err)
	}
	if !res.Applied {
		t.Fatalf("not applied: %+v", res.Preview)
	}
	if !app.editSessions[sessionID].Workspace.Dirty {
		t.Error("workspace should be dirty after apply")
	}
	afterInv := len(app.editSessions[sessionID].Workspace.InventoryItems)
	if afterInv <= beforeInv {
		t.Errorf("inventory not grown: %d -> %d", beforeInv, afterInv)
	}
	// slot.Data must remain untouched — apply is RAM-only.
	if len(slotDataBefore) != len(app.save.Slots[0].Data) {
		t.Errorf("slot.Data length changed; apply must not touch the save")
	}
	for i := range slotDataBefore {
		if slotDataBefore[i] != app.save.Slots[0].Data[i] {
			t.Errorf("slot.Data byte %d changed; apply must not touch the save", i)
			break
		}
	}
}

func TestApplyBuildTemplateFromLibrary_UnknownIDErrors(t *testing.T) {
	app, sessionID, _ := libraryFixture(t)
	if _, err := app.ApplyBuildTemplateFromLibrary(sessionID, "nope", ApplyTemplateOptions{}); err == nil {
		t.Fatal("expected error for unknown id")
	}
}

func TestDeleteBuildTemplateFromLibrary_RemovesEntry(t *testing.T) {
	app, _, id := libraryFixture(t)
	if err := app.DeleteBuildTemplateFromLibrary(id); err != nil {
		t.Fatalf("DeleteBuildTemplateFromLibrary: %v", err)
	}
	entries, err := app.ListBuildTemplateLibrary()
	if err != nil {
		t.Fatalf("ListBuildTemplateLibrary: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("want 0 entries after delete, got %d", len(entries))
	}
	if err := app.DeleteBuildTemplateFromLibrary(id); err == nil {
		t.Errorf("expected error on second delete (entry already gone)")
	}
}

func TestRenameBuildTemplateInLibrary_UpdatesEntry(t *testing.T) {
	app, _, id := libraryFixture(t)
	updated, err := app.RenameBuildTemplateInLibrary(id, "renamed", "new desc", []string{"x", "y"})
	if err != nil {
		t.Fatalf("RenameBuildTemplateInLibrary: %v", err)
	}
	if updated.Name != "renamed" || updated.Description != "new desc" {
		t.Errorf("rename did not stick: %+v", updated)
	}
	if len(updated.Tags) != 2 {
		t.Errorf("tags lost: %+v", updated.Tags)
	}
	// Confirm rename persists through a fresh List.
	entries, _ := app.ListBuildTemplateLibrary()
	if len(entries) != 1 || entries[0].Name != "renamed" {
		t.Errorf("rename did not survive list refresh: %+v", entries)
	}
}

func TestRenameBuildTemplateInLibrary_AcceptsNilTags(t *testing.T) {
	app, _, id := libraryFixture(t)
	updated, err := app.RenameBuildTemplateInLibrary(id, "no tags", "", nil)
	if err != nil {
		t.Fatalf("rename: %v", err)
	}
	if len(updated.Tags) != 0 {
		t.Errorf("nil tags should normalise to empty: %+v", updated.Tags)
	}
}

func TestExportLibraryBuildTemplateAsYAMLToFile_ViaLibraryHelper(t *testing.T) {
	// Bypass runtime.SaveFileDialog (needs Wails ctx) and exercise the
	// underlying library helper directly. Matches the JSON twin's
	// test pattern.
	app, _, id := libraryFixture(t)
	dest := filepath.Join(t.TempDir(), "exported.yaml")
	if err := app.templateLibrary.ExportTemplateToYAMLFile(id, dest); err != nil {
		t.Fatalf("ExportTemplateToYAMLFile: %v", err)
	}
	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read exported: %v", err)
	}
	tpl, err := templates.ParseBuildTemplateYAML(data)
	if err != nil {
		t.Fatalf("ParseBuildTemplateYAML: %v\nyaml:\n%s", err, data)
	}
	if tpl.Schema != templates.SchemaKey {
		t.Errorf("exported YAML has wrong schema: %q", tpl.Schema)
	}
}

func TestDefaultTemplateFilenameYAML_EndsInYaml(t *testing.T) {
	tpl := &templates.BuildTemplate{
		Schema: templates.SchemaKey, Version: templates.SchemaVersion,
		Metadata: &templates.TemplateMetadata{Name: "Greatsword RL150"},
	}
	got := defaultTemplateFilenameYAML(tpl)
	if !strings.HasSuffix(got, ".yaml") {
		t.Errorf("default YAML filename does not end with .yaml: %q", got)
	}
	if strings.Contains(got, ".json") {
		t.Errorf("default YAML filename still contains .json: %q", got)
	}

	// Empty metadata fallback also lands on .yaml.
	fallback := defaultTemplateFilenameYAML(&templates.BuildTemplate{})
	if !strings.HasSuffix(fallback, ".yaml") {
		t.Errorf("fallback YAML filename does not end with .yaml: %q", fallback)
	}
}

func TestPreviewYAMLPayload_HappyPath(t *testing.T) {
	// End-to-end: export workspace as JSON, transcode to YAML, then
	// run the YAML preview helper. Mirrors the JSON happy-path test.
	app := inventoryOrderFixture(testWeapons)
	snap, err := app.StartInventoryEditSession(0)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	exp, err := app.ExportBuildTemplateJSON(snap.SessionID, BuildTemplateExportOptions{IncludeInventory: true})
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	tpl, err := templates.ParseBuildTemplateJSON([]byte(exp.JSON))
	if err != nil {
		t.Fatalf("parse JSON: %v", err)
	}
	yamlBytes, err := templates.MarshalBuildTemplateYAML(tpl)
	if err != nil {
		t.Fatalf("MarshalBuildTemplateYAML: %v", err)
	}

	bundle := previewYAMLPayload(yamlBytes, "/fake/path.yaml")
	if !bundle.Report.OK {
		t.Fatalf("preview not OK: %+v", bundle.Report.Errors)
	}
	if bundle.JSON == "" {
		t.Fatal("previewYAMLPayload must return canonical JSON for anti-TOCTOU")
	}
	if bundle.Path != "/fake/path.yaml" {
		t.Errorf("path not echoed: %q", bundle.Path)
	}
	// Canonical JSON must round-trip through the JSON parser cleanly.
	if _, err := templates.ParseBuildTemplateJSON([]byte(bundle.JSON)); err != nil {
		t.Errorf("canonical JSON does not re-parse: %v", err)
	}
}

func TestPreviewYAMLPayload_MalformedReturnsStructureInvalid(t *testing.T) {
	bundle := previewYAMLPayload([]byte("not: [valid"), "/x.yaml")
	if bundle.Report.OK {
		t.Fatal("expected NOT-OK report for malformed YAML")
	}
	if len(bundle.Report.Errors) != 1 || bundle.Report.Errors[0].Code != templates.IssueCodeStructureInvalid {
		t.Errorf("expected single structure_invalid error, got %+v", bundle.Report.Errors)
	}
	if bundle.JSON != "" {
		t.Errorf("malformed YAML must not produce canonical JSON, got %q", bundle.JSON)
	}
}

func TestPreviewYAMLPayload_RejectsUnknownField(t *testing.T) {
	payload := []byte(`schema: saveforge.build-template
version: 1
createdAt: "2026-05-17T12:34:56Z"
extraGarbage: true
sections:
  inventory.workspace:
    inventoryItems:
      - baseItemID: 1000000
        quantity: 1
        container: inventory
        position: 0
    storageItems: []
`)
	bundle := previewYAMLPayload(payload, "")
	if bundle.Report.OK {
		t.Fatal("expected NOT-OK report for unknown field")
	}
	if bundle.Report.Errors[0].Code != templates.IssueCodeStructureInvalid {
		t.Errorf("expected structure_invalid, got %+v", bundle.Report.Errors)
	}
}

// TestPreviewYAMLPayload_RejectsMultiDocument confirms the YAML preview
// helper surfaces multi-document rejection through the existing
// malformed-payload UX (a NOT-OK report with a single structure_invalid
// error). Anti-confused-deputy guarantee must hold end-to-end, not just
// at the codec layer.
func TestPreviewYAMLPayload_RejectsMultiDocument(t *testing.T) {
	payload := []byte(`schema: saveforge.build-template
version: 1
createdAt: "2026-05-17T12:34:56Z"
sections:
  inventory.workspace:
    inventoryItems:
      - baseItemID: 1000000
        quantity: 1
        container: inventory
        position: 0
    storageItems: []
---
schema: hidden-second-document
version: 42
`)
	bundle := previewYAMLPayload(payload, "/x.yaml")
	if bundle.Report.OK {
		t.Fatal("expected NOT-OK report for multi-document YAML")
	}
	if len(bundle.Report.Errors) != 1 || bundle.Report.Errors[0].Code != templates.IssueCodeStructureInvalid {
		t.Errorf("expected single structure_invalid error, got %+v", bundle.Report.Errors)
	}
	if !strings.Contains(bundle.Report.Errors[0].Message, "multi-document YAML payloads are not supported") {
		t.Errorf("error message must surface multi-document rejection, got %q", bundle.Report.Errors[0].Message)
	}
	if bundle.JSON != "" {
		t.Errorf("multi-document YAML must not produce canonical JSON, got %q", bundle.JSON)
	}
}

func TestReadYAMLFileCapped_RejectsOversize(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "big.yaml")
	oversize := make([]byte, maxYAMLImportBytes+10)
	for i := range oversize {
		oversize[i] = 'a'
	}
	if err := os.WriteFile(path, oversize, 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if _, err := readYAMLFileCapped(path); err == nil {
		t.Fatal("expected error for oversize YAML")
	}
}

func TestReadYAMLFileCapped_AcceptsAtLimit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ok.yaml")
	atLimit := make([]byte, maxYAMLImportBytes)
	for i := range atLimit {
		atLimit[i] = 'b'
	}
	if err := os.WriteFile(path, atLimit, 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	data, err := readYAMLFileCapped(path)
	if err != nil {
		t.Fatalf("readYAMLFileCapped at limit failed: %v", err)
	}
	if int64(len(data)) != maxYAMLImportBytes {
		t.Errorf("returned %d bytes, want %d", len(data), maxYAMLImportBytes)
	}
}

func TestSaveImportedBuildTemplateJSONToLibrary_PersistsAsJSON(t *testing.T) {
	app, _, _ := libraryFixture(t)
	// Use a synthetic library-only template (no session needed). Use a
	// real, DB-resolvable item so PreviewBuildTemplateImport returns OK.
	src := &templates.BuildTemplate{
		Schema:    templates.SchemaKey,
		Version:   templates.SchemaVersion,
		CreatedAt: "2026-05-17T12:34:56Z",
		Metadata: &templates.TemplateMetadata{
			Name:        "imported",
			Description: "via SaveImportedBuildTemplateJSONToLibrary",
		},
		Sections: templates.TemplateSections{
			InventoryWorkspace: &templates.InventoryWorkspaceSection{
				InventoryItems: []templates.TemplateItem{{
					BaseItemID: testWeapons[0].itemID,
					Quantity:   1,
					Container:  templates.ContainerInventory,
					Position:   0,
				}},
				StorageItems: []templates.TemplateItem{},
			},
		},
	}
	canonicalJSON, err := json.MarshalIndent(src, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	entry, err := app.SaveImportedBuildTemplateJSONToLibrary(string(canonicalJSON))
	if err != nil {
		t.Fatalf("SaveImportedBuildTemplateJSONToLibrary: %v", err)
	}
	if entry.ID == "" {
		t.Fatal("entry.ID empty")
	}
	if entry.Name != "imported" {
		t.Errorf("Name=%q want imported", entry.Name)
	}

	// Library now has the fixture + the imported entry; both must be
	// loadable through the existing library load flow as JSON.
	listing, err := app.ListBuildTemplateLibrary()
	if err != nil {
		t.Fatalf("ListBuildTemplateLibrary: %v", err)
	}
	if len(listing) != 2 {
		t.Fatalf("want 2 entries (fixture + imported), got %d", len(listing))
	}
	loaded, err := app.templateLibrary.LoadTemplate(entry.ID)
	if err != nil {
		t.Fatalf("LoadTemplate: %v", err)
	}
	if loaded.Schema != templates.SchemaKey {
		t.Errorf("loaded back wrong schema: %q", loaded.Schema)
	}

	// File on disk must end with .json — library storage stays JSON-internal.
	if !strings.HasSuffix(entry.Filename, ".json") {
		t.Errorf("library file should end with .json, got %q", entry.Filename)
	}
}

func TestSaveImportedBuildTemplateJSONToLibrary_RejectsInvalidJSON(t *testing.T) {
	app, _, _ := libraryFixture(t)
	if _, err := app.SaveImportedBuildTemplateJSONToLibrary("not json"); err == nil {
		t.Fatal("expected error for malformed JSON")
	}
}

func TestSaveImportedBuildTemplateJSONToLibrary_RejectsWrongSchema(t *testing.T) {
	app, _, _ := libraryFixture(t)
	bad := `{"schema":"wrong","version":1,"createdAt":"2026-05-17T12:34:56Z","sections":{"inventory.workspace":{"inventoryItems":[{"baseItemID":15990336,"quantity":1,"container":"inventory","position":0}],"storageItems":[]}}}`
	if _, err := app.SaveImportedBuildTemplateJSONToLibrary(bad); err == nil {
		t.Fatal("expected error for wrong schema")
	}
}

func TestSaveImportedBuildTemplateJSONToLibrary_RejectsBlockingPreview(t *testing.T) {
	// A structurally valid template that references an unknown item ID
	// should be refused — PreviewBuildTemplateImport flags unknown_item.
	app, _, _ := libraryFixture(t)
	src := &templates.BuildTemplate{
		Schema:    templates.SchemaKey,
		Version:   templates.SchemaVersion,
		CreatedAt: "2026-05-17T12:34:56Z",
		Metadata:  &templates.TemplateMetadata{Name: "bad item"},
		Sections: templates.TemplateSections{
			InventoryWorkspace: &templates.InventoryWorkspaceSection{
				InventoryItems: []templates.TemplateItem{{
					BaseItemID: 0xDEADBEEF, // not a real item
					Quantity:   1,
					Container:  templates.ContainerInventory,
					Position:   0,
				}},
				StorageItems: []templates.TemplateItem{},
			},
		},
	}
	canonicalJSON, _ := json.MarshalIndent(src, "", "  ")
	if _, err := app.SaveImportedBuildTemplateJSONToLibrary(string(canonicalJSON)); err == nil {
		t.Fatal("expected error for template with blocking preview issues")
	}
}

func TestSaveImportedBuildTemplateJSONToLibrary_NoSessionRequired(t *testing.T) {
	// Build a fresh app with NO sessions started. The endpoint must
	// still work — saving to library is workspace-independent.
	app := inventoryOrderFixture(testWeapons)
	app.templateLibrary = templates.NewTemplateLibrary(t.TempDir())

	src := &templates.BuildTemplate{
		Schema:    templates.SchemaKey,
		Version:   templates.SchemaVersion,
		CreatedAt: "2026-05-17T12:34:56Z",
		Metadata:  &templates.TemplateMetadata{Name: "no-session"},
		Sections: templates.TemplateSections{
			InventoryWorkspace: &templates.InventoryWorkspaceSection{
				InventoryItems: []templates.TemplateItem{{
					BaseItemID: testWeapons[0].itemID,
					Quantity:   1,
					Container:  templates.ContainerInventory,
					Position:   0,
				}},
				StorageItems: []templates.TemplateItem{},
			},
		},
	}
	canonicalJSON, _ := json.MarshalIndent(src, "", "  ")
	entry, err := app.SaveImportedBuildTemplateJSONToLibrary(string(canonicalJSON))
	if err != nil {
		t.Fatalf("SaveImportedBuildTemplateJSONToLibrary without session: %v", err)
	}
	if entry.Name != "no-session" {
		t.Errorf("Name=%q want no-session", entry.Name)
	}
}

func TestExportLibraryBuildTemplateToFile_ViaLibraryHelper(t *testing.T) {
	// We bypass the runtime SaveFileDialog (which requires a Wails ctx)
	// and exercise the underlying library helper directly. This matches
	// the existing writeBuildTemplateFile-style test pattern in Phase B.
	app, _, id := libraryFixture(t)
	dest := filepath.Join(t.TempDir(), "exported.json")
	if err := app.templateLibrary.ExportTemplateToFile(id, dest); err != nil {
		t.Fatalf("ExportTemplateToFile: %v", err)
	}
	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read exported: %v", err)
	}
	var tpl templates.BuildTemplate
	if err := json.Unmarshal(data, &tpl); err != nil {
		t.Fatalf("unmarshal exported: %v", err)
	}
	if tpl.Schema != templates.SchemaKey {
		t.Errorf("exported payload has wrong schema: %q", tpl.Schema)
	}
}

func TestRebuildBuildTemplateLibraryIndex_PicksUpManuallyDroppedFiles(t *testing.T) {
	app, _, _ := libraryFixture(t)
	libDir := app.templateLibrary.RootDir()

	// Drop a hand-written valid template file into the library
	// without going through SaveTemplate, then verify the index
	// catches it after Rebuild.
	dropped := &templates.BuildTemplate{
		Schema:    templates.SchemaKey,
		Version:   templates.SchemaVersion,
		CreatedAt: "2026-05-17T12:00:00Z",
		Metadata: &templates.TemplateMetadata{
			Name:        "manually dropped",
			Description: "copied from another machine",
		},
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
	data, err := json.MarshalIndent(dropped, "", "  ")
	if err != nil {
		t.Fatalf("marshal dropped: %v", err)
	}
	if err := os.WriteFile(filepath.Join(libDir, "hand-written.json"), data, 0644); err != nil {
		t.Fatalf("write dropped: %v", err)
	}

	beforeList, _ := app.ListBuildTemplateLibrary()
	if len(beforeList) != 1 {
		t.Fatalf("baseline list should have 1 entry (fixture); got %d", len(beforeList))
	}

	rebuilt, err := app.RebuildBuildTemplateLibraryIndex()
	if err != nil {
		t.Fatalf("RebuildBuildTemplateLibraryIndex: %v", err)
	}
	if len(rebuilt) != 2 {
		t.Fatalf("after rebuild want 2 entries, got %d (%+v)", len(rebuilt), rebuilt)
	}
	found := false
	for _, e := range rebuilt {
		if e.Name == "manually dropped" {
			found = true
		}
	}
	if !found {
		t.Errorf("dropped template not picked up by rebuild")
	}
}

func TestRebuildBuildTemplateLibraryIndex_SkipsCorruptFiles(t *testing.T) {
	app, _, _ := libraryFixture(t)
	libDir := app.templateLibrary.RootDir()

	if err := os.WriteFile(filepath.Join(libDir, "garbage.json"), []byte("{not json"), 0644); err != nil {
		t.Fatalf("write garbage: %v", err)
	}

	entries, err := app.RebuildBuildTemplateLibraryIndex()
	if err != nil {
		t.Fatalf("RebuildBuildTemplateLibraryIndex: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("garbage file leaked into index; want 1 entry, got %d", len(entries))
	}
}

func TestRebuildBuildTemplateLibraryIndex_DoesNotTouchSaveOrWorkspace(t *testing.T) {
	app, sessionID, _ := libraryFixture(t)
	wsBefore := len(app.editSessions[sessionID].Workspace.InventoryItems)
	dirtyBefore := app.editSessions[sessionID].Workspace.Dirty
	slotBefore := append([]byte(nil), app.save.Slots[0].Data...)

	if _, err := app.RebuildBuildTemplateLibraryIndex(); err != nil {
		t.Fatalf("RebuildBuildTemplateLibraryIndex: %v", err)
	}

	if got := len(app.editSessions[sessionID].Workspace.InventoryItems); got != wsBefore {
		t.Errorf("workspace inventory mutated: %d -> %d", wsBefore, got)
	}
	if app.editSessions[sessionID].Workspace.Dirty != dirtyBefore {
		t.Errorf("dirty flag flipped during rebuild")
	}
	for i := range slotBefore {
		if slotBefore[i] != app.save.Slots[0].Data[i] {
			t.Errorf("slot.Data byte %d changed during rebuild", i)
			break
		}
	}
}

func TestGetBuildTemplateLibraryPath_ReturnsRootDir(t *testing.T) {
	app, _, _ := libraryFixture(t)
	path, err := app.GetBuildTemplateLibraryPath()
	if err != nil {
		t.Fatalf("GetBuildTemplateLibraryPath: %v", err)
	}
	if path == "" {
		t.Errorf("library path is empty")
	}
	if path != app.templateLibrary.RootDir() {
		t.Errorf("returned path %q does not match RootDir %q", path, app.templateLibrary.RootDir())
	}
}

func TestEnsureTemplateLibrary_IsLazyAndCached(t *testing.T) {
	app := inventoryOrderFixture(testWeapons)
	if app.templateLibrary != nil {
		t.Fatal("templateLibrary should be nil before first endpoint call")
	}
	// Pre-seed to avoid hitting the real $UserConfigDir during tests.
	app.templateLibrary = templates.NewTemplateLibrary(t.TempDir())
	first, err := app.ensureTemplateLibrary()
	if err != nil {
		t.Fatalf("ensureTemplateLibrary: %v", err)
	}
	second, err := app.ensureTemplateLibrary()
	if err != nil {
		t.Fatalf("ensureTemplateLibrary again: %v", err)
	}
	if first != second {
		t.Errorf("ensureTemplateLibrary should return the cached handle")
	}
}
