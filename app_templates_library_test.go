package main

import (
	"encoding/json"
	"os"
	"path/filepath"
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
