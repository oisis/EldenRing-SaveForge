package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
)

// TestLoadSaveFromPath_UnsupportedContainerDoesNotReplaceLoadedSave proves the
// strict-detection contract at the app boundary: when the input container is
// ambiguous/unsupported, the open fails and the currently loaded save is left
// untouched (never silently swapped for a wrongly-classified one).
func TestLoadSaveFromPath_UnsupportedContainerDoesNotReplaceLoadedSave(t *testing.T) {
	app := NewApp()

	// Sentinel "already loaded" save.
	loaded := &core.SaveFile{Platform: core.PlatformPS}
	app.save = loaded
	app.lastSavePath = "sentinel.dat"

	// Properly-sized file with unknown container magic (mimics an AES PC save).
	data := make([]byte, core.MinSaveFileSize)
	copy(data, []byte{0x81, 0xb4, 0x3a, 0xec})
	path := filepath.Join(t.TempDir(), "unsupported.dat")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	if _, err := app.LoadSaveFromPath(path); err == nil {
		t.Fatal("LoadSaveFromPath accepted an unsupported container")
	}
	if app.save != loaded {
		t.Fatal("unsupported load replaced the active save")
	}
	if app.lastSavePath != "sentinel.dat" {
		t.Fatalf("lastSavePath mutated by failed load: %q", app.lastSavePath)
	}
}
