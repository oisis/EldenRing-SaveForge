package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
)

// loadFixtureSave parses the canonical tmp/save/ER0000.sl2 fixture
// into a *core.SaveFile, skipping the test if the gitignored save is
// not checked in locally. Returned save is independent of the App
// instance — callers can use it as the candidate for installLoadedSave
// or as a writeSaveCore "expected" snapshot.
func loadFixtureSave(t *testing.T) *core.SaveFile {
	t.Helper()
	const savePath = "tmp/save/ER0000.sl2"
	if _, err := os.Stat(savePath); os.IsNotExist(err) {
		t.Skipf("test save not found: %s", savePath)
	}
	save, err := core.LoadSave(savePath)
	if err != nil {
		t.Fatalf("LoadSave: %v", err)
	}
	return save
}

// TestSaveLifecycle_InstallLoadedSaveBlocksOnActiveReader proves the
// whole-save commit phase (installLoadedSave under saveMu.Lock) drains
// in-flight readers before swapping the a.save pointer. We park a
// reader via saveMu.RLock and assert the writer cannot complete until
// we release.
func TestSaveLifecycle_InstallLoadedSaveBlocksOnActiveReader(t *testing.T) {
	app, _ := realSaveAppForSave(t)
	candidate := loadFixtureSave(t)

	// Pre-populate derived state so we can prove the install reset it.
	app.favSlotNames[3] = "sentinel"
	app.undoStacks[0] = append(app.undoStacks[0], slotSnapshot{Data: []byte{0xDE, 0xAD}})

	app.saveMu.RLock()
	released := false
	release := func() {
		if !released {
			released = true
			app.saveMu.RUnlock()
		}
	}
	defer release()

	done := make(chan struct{})
	go func() {
		defer close(done)
		app.saveMu.Lock()
		app.installLoadedSave(candidate, "test/path/install.sl2")
		app.saveMu.Unlock()
	}()

	select {
	case <-done:
		t.Fatal("installLoadedSave completed while an active saveMu.RLock reader was held — saveMu drain contract broken")
	case <-time.After(50 * time.Millisecond):
		// expected
	}

	release()

	select {
	case <-done:
		// good
	case <-time.After(5 * time.Second):
		t.Fatal("installLoadedSave did not complete after RLock release — possible deadlock")
	}

	if app.save != candidate {
		t.Error("a.save was not swapped to the candidate after install")
	}
	if app.lastSavePath != "test/path/install.sl2" {
		t.Errorf("lastSavePath = %q, want test/path/install.sl2", app.lastSavePath)
	}
	if len(app.favSlotNames) != 0 {
		t.Errorf("favSlotNames not reset by install: got %d entries", len(app.favSlotNames))
	}
	for i, stack := range app.undoStacks {
		if len(stack) != 0 {
			t.Errorf("undoStacks[%d] not reset by install: got %d snapshots", i, len(stack))
		}
	}
}

// TestWriteSave_AbortsWhenActiveSaveChanged is the WriteSave identity
// guard test: if a concurrent SelectAndOpenSave / DownloadRemoteSave
// replaces a.save while the user is in the save dialog, the eventual
// SaveFile must NOT happen — otherwise save B would be written to a
// path picked for save A. The guard is enforced inside writeSaveCore
// by comparing `a.save != expected` under saveMu.Lock and erroring
// before any mutation or disk I/O.
func TestWriteSave_AbortsWhenActiveSaveChanged(t *testing.T) {
	app, _ := realSaveAppForSave(t)

	// Snapshot the active save A and capture pre-mutation metadata.
	app.saveMu.RLock()
	expected := app.save
	app.saveMu.RUnlock()
	origPlatform := expected.Platform
	origEncrypted := expected.Encrypted
	origIV := append([]byte(nil), expected.IV...)
	origLastPath := app.lastSavePath

	// Install a fresh save B under the commit-phase contract — same
	// path SelectAndOpenSave / DownloadRemoteSave use. installLoadedSave
	// runs clearAllUndoStacks as part of derived-state reset, so any
	// "undo not cleared by aborted write" check must seed AFTER the
	// install, not before.
	saveB := loadFixtureSave(t)
	if saveB == expected {
		t.Fatal("loadFixtureSave returned the same pointer as the App's active save; cannot simulate a swap")
	}
	app.saveMu.Lock()
	app.installLoadedSave(saveB, "")
	app.saveMu.Unlock()

	// Seed the undo stack AFTER install so we can prove a rejected
	// write does NOT clear it (clearAllUndoStacks runs inside
	// writeSaveCore only on the happy path, after SaveFile succeeds).
	app.undoStacks[0] = append(app.undoStacks[0], slotSnapshot{Data: []byte{0x55}})

	// Target path must NOT exist before the write attempt and must
	// remain non-existent afterwards — the guard errors before any
	// SaveFile call.
	targetPath := filepath.Join(t.TempDir(), "write-aborted.sl2")
	if _, err := os.Stat(targetPath); !os.IsNotExist(err) {
		t.Fatalf("precondition: target path %q already exists", targetPath)
	}

	err := app.writeSaveCore(targetPath, string(saveB.Platform), expected)
	if err == nil {
		t.Fatal("writeSaveCore returned nil after a.save was swapped; expected identity-guard error")
	}
	if !strings.Contains(err.Error(), "active save changed") {
		t.Errorf("writeSaveCore error: want substring %q, got %q", "active save changed", err.Error())
	}

	if _, statErr := os.Stat(targetPath); !os.IsNotExist(statErr) {
		t.Errorf("target file should not exist after aborted write, stat=%v", statErr)
	}

	// SaveB metadata must be unchanged — the guard returns before
	// the Platform / IV / Encrypted rewrite block.
	if saveB.Platform != origPlatform {
		t.Errorf("saveB.Platform mutated by aborted write: got %q, want %q", saveB.Platform, origPlatform)
	}
	if saveB.Encrypted != origEncrypted {
		t.Errorf("saveB.Encrypted mutated by aborted write: got %v, want %v", saveB.Encrypted, origEncrypted)
	}
	if !bytes.Equal(saveB.IV, origIV) {
		t.Errorf("saveB.IV mutated by aborted write")
	}
	if app.lastSavePath != origLastPath {
		t.Errorf("app.lastSavePath mutated by aborted write: got %q, want %q", app.lastSavePath, origLastPath)
	}

	// Undo stack must NOT have been cleared — clearAllUndoStacks
	// runs only after a successful SaveFile.
	if len(app.undoStacks[0]) == 0 {
		t.Error("undo stack was cleared by aborted write; clearAllUndoStacks should only run on success")
	}
}
