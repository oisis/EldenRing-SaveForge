package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
)

// TestBackupCurrentSave_NoSave verifies the fail-closed contract: with nothing
// loaded, Chaos Mode's autobackup must error rather than silently succeed.
func TestBackupCurrentSave_NoSave(t *testing.T) {
	app := NewApp()
	if _, err := app.BackupCurrentSave(); err == nil || !strings.Contains(err.Error(), "no save loaded") {
		t.Fatalf("expected 'no save loaded', got %v", err)
	}
}

// TestBackupCurrentSave_CreatesBak verifies that a loaded save with a real file
// on disk produces a timestamped .bak copy with identical contents.
func TestBackupCurrentSave_CreatesBak(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ER0000.sl2")
	content := []byte("save-bytes")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	app := NewApp()
	app.save = &core.SaveFile{}
	app.lastSavePath = path

	backupPath, err := app.BackupCurrentSave()
	if err != nil {
		t.Fatalf("BackupCurrentSave: %v", err)
	}
	if !strings.HasSuffix(backupPath, ".bak") {
		t.Fatalf("backup path %q does not end in .bak", backupPath)
	}
	got, err := os.ReadFile(backupPath)
	if err != nil {
		t.Fatalf("read backup: %v", err)
	}
	if string(got) != string(content) {
		t.Fatalf("backup contents = %q, want %q", got, content)
	}
}
