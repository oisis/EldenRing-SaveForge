package templates

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// makeTemplate returns a minimal but valid BuildTemplate the library
// can accept. Tests pass distinct names so filename collision logic
// stays observable.
func makeTemplate(name string) *BuildTemplate {
	return &BuildTemplate{
		Schema:     SchemaKey,
		Version:    SchemaVersion,
		CreatedAt:  time.Date(2026, time.May, 17, 10, 0, 0, 0, time.UTC).Format(time.RFC3339),
		AppVersion: "0.15.0-beta",
		Metadata: &TemplateMetadata{
			Name:        name,
			Description: "test template",
			Tags:        []string{"unit"},
		},
		Sections: TemplateSections{
			InventoryWorkspace: &InventoryWorkspaceSection{
				InventoryItems: []TemplateItem{{
					BaseItemID: 0x000F4240,
					Name:       "Greatsword",
					Category:   "melee_armaments",
					Quantity:   1,
					Upgrade:    10,
					Container:  ContainerInventory,
					Position:   0,
				}},
				StorageItems: []TemplateItem{},
			},
		},
	}
}

func TestLibrary_SaveTemplate_CreatesFileAndIndexEntry(t *testing.T) {
	lib := NewTemplateLibrary(t.TempDir())
	entry, err := lib.SaveTemplate(makeTemplate("RL150 Greatsword"))
	if err != nil {
		t.Fatalf("SaveTemplate: %v", err)
	}
	if entry.ID == "" {
		t.Fatal("entry.ID is empty")
	}
	if entry.Filename == "" {
		t.Fatal("entry.Filename is empty")
	}
	if entry.Name != "RL150 Greatsword" {
		t.Errorf("Name=%q want RL150 Greatsword", entry.Name)
	}
	if entry.InventoryItems != 1 {
		t.Errorf("InventoryItems=%d want 1", entry.InventoryItems)
	}
	if entry.CreatedAt == "" || entry.UpdatedAt == "" {
		t.Errorf("missing timestamps: created=%q updated=%q", entry.CreatedAt, entry.UpdatedAt)
	}

	if _, err := os.Stat(filepath.Join(lib.RootDir(), entry.Filename)); err != nil {
		t.Errorf("template file not on disk: %v", err)
	}
	if _, err := os.Stat(filepath.Join(lib.RootDir(), LibraryIndexFile)); err != nil {
		t.Errorf("index file not on disk: %v", err)
	}
}

func TestLibrary_ListTemplates_ReturnsSavedEntries(t *testing.T) {
	lib := NewTemplateLibrary(t.TempDir())
	if _, err := lib.SaveTemplate(makeTemplate("alpha")); err != nil {
		t.Fatalf("SaveTemplate alpha: %v", err)
	}
	// Force a second-resolution gap so the newer entry sorts first.
	time.Sleep(1100 * time.Millisecond)
	if _, err := lib.SaveTemplate(makeTemplate("beta")); err != nil {
		t.Fatalf("SaveTemplate beta: %v", err)
	}
	entries, err := lib.ListTemplates()
	if err != nil {
		t.Fatalf("ListTemplates: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("want 2 entries, got %d", len(entries))
	}
	if entries[0].Name != "beta" {
		t.Errorf("newest first: want beta, got %q", entries[0].Name)
	}
}

func TestLibrary_LoadTemplate_RoundTripsContents(t *testing.T) {
	lib := NewTemplateLibrary(t.TempDir())
	original := makeTemplate("roundtrip")
	entry, err := lib.SaveTemplate(original)
	if err != nil {
		t.Fatalf("SaveTemplate: %v", err)
	}
	loaded, err := lib.LoadTemplate(entry.ID)
	if err != nil {
		t.Fatalf("LoadTemplate: %v", err)
	}
	if loaded.Schema != original.Schema || loaded.Version != original.Version {
		t.Errorf("schema/version drift: %s/%d", loaded.Schema, loaded.Version)
	}
	if loaded.Metadata == nil || loaded.Metadata.Name != "roundtrip" {
		t.Errorf("metadata name lost: %#v", loaded.Metadata)
	}
	if loaded.Sections.InventoryWorkspace == nil || len(loaded.Sections.InventoryWorkspace.InventoryItems) != 1 {
		t.Errorf("inventory items lost")
	}
}

func TestLibrary_DeleteTemplate_RemovesFileAndEntry(t *testing.T) {
	lib := NewTemplateLibrary(t.TempDir())
	entry, err := lib.SaveTemplate(makeTemplate("to-delete"))
	if err != nil {
		t.Fatalf("SaveTemplate: %v", err)
	}
	if err := lib.DeleteTemplate(entry.ID); err != nil {
		t.Fatalf("DeleteTemplate: %v", err)
	}
	if _, err := os.Stat(filepath.Join(lib.RootDir(), entry.Filename)); !os.IsNotExist(err) {
		t.Errorf("file should be gone, stat err=%v", err)
	}
	entries, err := lib.ListTemplates()
	if err != nil {
		t.Fatalf("ListTemplates: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("want 0 entries, got %d", len(entries))
	}
	if err := lib.DeleteTemplate(entry.ID); err == nil {
		t.Errorf("second delete should error (already gone)")
	}
}

func TestLibrary_RenameTemplate_UpdatesFileAndIndex(t *testing.T) {
	lib := NewTemplateLibrary(t.TempDir())
	entry, err := lib.SaveTemplate(makeTemplate("old name"))
	if err != nil {
		t.Fatalf("SaveTemplate: %v", err)
	}
	updated, err := lib.RenameTemplate(entry.ID, "new name", "new desc", []string{"a", "b"})
	if err != nil {
		t.Fatalf("RenameTemplate: %v", err)
	}
	if updated.Name != "new name" || updated.Description != "new desc" {
		t.Errorf("entry not updated: %+v", updated)
	}
	if len(updated.Tags) != 2 || updated.Tags[0] != "a" {
		t.Errorf("tags not propagated: %+v", updated.Tags)
	}
	if updated.UpdatedAt == entry.UpdatedAt {
		t.Errorf("UpdatedAt not bumped: %s", updated.UpdatedAt)
	}

	tpl, err := lib.LoadTemplate(entry.ID)
	if err != nil {
		t.Fatalf("LoadTemplate after rename: %v", err)
	}
	if tpl.Metadata == nil || tpl.Metadata.Name != "new name" || tpl.Metadata.Description != "new desc" {
		t.Errorf("template file metadata not rewritten: %#v", tpl.Metadata)
	}
}

func TestLibrary_RebuildIndex_RecoversFromMissingIndex(t *testing.T) {
	lib := NewTemplateLibrary(t.TempDir())
	e1, err := lib.SaveTemplate(makeTemplate("survivor"))
	if err != nil {
		t.Fatalf("SaveTemplate: %v", err)
	}
	if err := os.Remove(filepath.Join(lib.RootDir(), LibraryIndexFile)); err != nil {
		t.Fatalf("remove index: %v", err)
	}
	if err := lib.RebuildIndex(); err != nil {
		t.Fatalf("RebuildIndex: %v", err)
	}
	entries, err := lib.ListTemplates()
	if err != nil {
		t.Fatalf("ListTemplates: %v", err)
	}
	if len(entries) != 1 || entries[0].Filename != e1.Filename {
		t.Errorf("rebuilt index lost entry; got %+v", entries)
	}
}

func TestLibrary_RebuildIndex_SkipsCorruptTemplateFiles(t *testing.T) {
	lib := NewTemplateLibrary(t.TempDir())
	if _, err := lib.SaveTemplate(makeTemplate("good")); err != nil {
		t.Fatalf("SaveTemplate good: %v", err)
	}
	// Drop a malformed JSON in the dir — should be ignored, not crash.
	if err := os.WriteFile(filepath.Join(lib.RootDir(), "garbage.json"), []byte("{not json"), 0644); err != nil {
		t.Fatalf("write garbage: %v", err)
	}
	if err := os.Remove(filepath.Join(lib.RootDir(), LibraryIndexFile)); err != nil {
		t.Fatalf("remove index: %v", err)
	}
	if err := lib.RebuildIndex(); err != nil {
		t.Fatalf("RebuildIndex: %v", err)
	}
	entries, err := lib.ListTemplates()
	if err != nil {
		t.Fatalf("ListTemplates: %v", err)
	}
	if len(entries) != 1 || entries[0].Name != "good" {
		t.Errorf("garbage file leaked into index: %+v", entries)
	}
}

func TestLibrary_ReadIndex_RebuildsCorruptIndex(t *testing.T) {
	lib := NewTemplateLibrary(t.TempDir())
	if _, err := lib.SaveTemplate(makeTemplate("survivor")); err != nil {
		t.Fatalf("SaveTemplate: %v", err)
	}
	if err := os.WriteFile(filepath.Join(lib.RootDir(), LibraryIndexFile), []byte("this is not json"), 0644); err != nil {
		t.Fatalf("corrupt index: %v", err)
	}
	entries, err := lib.ListTemplates()
	if err != nil {
		t.Fatalf("ListTemplates after corrupt index: %v", err)
	}
	if len(entries) != 1 || entries[0].Name != "survivor" {
		t.Errorf("rebuild-on-read failed: %+v", entries)
	}
}

func TestLibrary_SaveTemplate_PicksUniqueFilenameOnCollision(t *testing.T) {
	lib := NewTemplateLibrary(t.TempDir())
	e1, err := lib.SaveTemplate(makeTemplate("same"))
	if err != nil {
		t.Fatalf("SaveTemplate first: %v", err)
	}
	e2, err := lib.SaveTemplate(makeTemplate("same"))
	if err != nil {
		t.Fatalf("SaveTemplate second: %v", err)
	}
	if e1.Filename == e2.Filename {
		t.Fatalf("collision: %q == %q", e1.Filename, e2.Filename)
	}
	if e1.ID == e2.ID {
		t.Errorf("IDs collided: %q", e1.ID)
	}
}

func TestLibrary_SaveTemplate_RejectsInvalidTemplate(t *testing.T) {
	lib := NewTemplateLibrary(t.TempDir())
	bad := makeTemplate("bad")
	bad.Schema = "not-a-real-schema"
	if _, err := lib.SaveTemplate(bad); err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if _, err := lib.SaveTemplate(nil); err == nil {
		t.Fatal("expected error for nil template")
	}
}

func TestLibrary_LoadTemplate_UnknownIDErrors(t *testing.T) {
	lib := NewTemplateLibrary(t.TempDir())
	if _, err := lib.LoadTemplate("does-not-exist"); err == nil {
		t.Fatal("expected error for unknown id")
	}
}

func TestLibrary_ExportTemplateToFile_WritesIdenticalPayload(t *testing.T) {
	lib := NewTemplateLibrary(t.TempDir())
	entry, err := lib.SaveTemplate(makeTemplate("share me"))
	if err != nil {
		t.Fatalf("SaveTemplate: %v", err)
	}
	dest := filepath.Join(t.TempDir(), "exported.json")
	if err := lib.ExportTemplateToFile(entry.ID, dest); err != nil {
		t.Fatalf("ExportTemplateToFile: %v", err)
	}
	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read exported file: %v", err)
	}
	var tpl BuildTemplate
	if err := json.Unmarshal(data, &tpl); err != nil {
		t.Fatalf("unmarshal exported: %v", err)
	}
	if tpl.Metadata == nil || tpl.Metadata.Name != "share me" {
		t.Errorf("exported template metadata lost: %#v", tpl.Metadata)
	}
	if err := lib.ExportTemplateToFile("missing-id", dest); err == nil {
		t.Errorf("expected error for unknown id")
	}
	if err := lib.ExportTemplateToFile(entry.ID, ""); err == nil {
		t.Errorf("expected error for empty path")
	}
}

func TestLibrary_AtomicWrite_LeavesNoTempFileOnSuccess(t *testing.T) {
	lib := NewTemplateLibrary(t.TempDir())
	if _, err := lib.SaveTemplate(makeTemplate("atomic")); err != nil {
		t.Fatalf("SaveTemplate: %v", err)
	}
	entries, err := os.ReadDir(lib.RootDir())
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".saveforge-tmp-") {
			t.Errorf("temp file leaked: %q", e.Name())
		}
	}
}

func TestDefaultTemplateLibraryDir_ReturnsPathUnderConfigDir(t *testing.T) {
	dir, err := DefaultTemplateLibraryDir()
	if err != nil {
		t.Fatalf("DefaultTemplateLibraryDir: %v", err)
	}
	if !strings.Contains(dir, "EldenRing-SaveEditor") {
		t.Errorf("path %q does not contain app dir segment", dir)
	}
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if !info.IsDir() {
		t.Errorf("path is not a directory")
	}
}
