package main

import (
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
)

// TestCloseSave_NoSave_NoOp documents the idempotent contract: calling
// CloseSave when nothing is loaded must succeed silently — the modal that
// triggers this endpoint may fire under racy conditions, and reporting a
// spurious error would force the UI to surface a fake failure.
func TestCloseSave_NoSave_NoOp(t *testing.T) {
	app := NewApp()
	if err := app.CloseSave(); err != nil {
		t.Errorf("CloseSave on empty App should be no-op, got %v", err)
	}
	if app.save != nil {
		t.Errorf("app.save should remain nil after no-op CloseSave, got %v", app.save)
	}
}

// TestCloseSave_DropsLoadedState verifies that a freshly-loaded save is
// fully released by CloseSave: the active pointer is cleared and the last
// path metadata is gone. The subsequent integrity scan must then report
// "no save loaded" so downstream endpoints cannot operate on stale data.
func TestCloseSave_DropsLoadedState(t *testing.T) {
	app := NewApp()
	app.save = &core.SaveFile{}
	app.save.Slots[0].Version = 1
	app.lastSavePath = "/tmp/whatever.sl2"

	if err := app.CloseSave(); err != nil {
		t.Fatalf("CloseSave: %v", err)
	}
	if app.save != nil {
		t.Errorf("app.save should be nil after CloseSave, got %v", app.save)
	}
	if app.lastSavePath != "" {
		t.Errorf("lastSavePath should be cleared, got %q", app.lastSavePath)
	}

	// Downstream integrity scan must now refuse — otherwise the modal could
	// keep blocking the UI on cached data from the dropped save.
	if _, err := app.GetSaveInventoryIntegrityReport(); err == nil {
		t.Errorf("GetSaveInventoryIntegrityReport should error after CloseSave")
	}
}

// TestCloseSave_ClearsUndoStacks pins that per-slot undo history does not
// outlive the active save. If it did, a follow-up Open Save could surface
// undo entries that belong to a completely different file.
func TestCloseSave_ClearsUndoStacks(t *testing.T) {
	app := NewApp()
	app.save = &core.SaveFile{}
	app.save.Slots[0].Version = 1
	app.undoStacks[0] = []slotSnapshot{{}}
	app.undoStacks[4] = []slotSnapshot{{}, {}}

	if err := app.CloseSave(); err != nil {
		t.Fatalf("CloseSave: %v", err)
	}
	for i := 0; i < 10; i++ {
		if depth := len(app.undoStacks[i]); depth != 0 {
			t.Errorf("undoStacks[%d] should be empty after CloseSave, got depth %d", i, depth)
		}
	}
}

// TestCloseSave_ClearsFavSlotNames keeps the favorites cache scoped to the
// active save: a phantom name from the previous file must not bleed into
// the next load.
func TestCloseSave_ClearsFavSlotNames(t *testing.T) {
	app := NewApp()
	app.save = &core.SaveFile{}
	app.favSlotNames[1] = "Stale Preset"
	app.favSlotNames[7] = "Other Preset"

	if err := app.CloseSave(); err != nil {
		t.Fatalf("CloseSave: %v", err)
	}
	if len(app.favSlotNames) != 0 {
		t.Errorf("favSlotNames should be reset after CloseSave, got %+v", app.favSlotNames)
	}
}

// TestCloseSave_DoesNotTouchSourceSave guards the explicit scope decision:
// the source save loaded for Character Importer is independent of the
// active editable save and must survive a CloseSave on the main file.
func TestCloseSave_DoesNotTouchSourceSave(t *testing.T) {
	app := NewApp()
	app.save = &core.SaveFile{}
	source := &core.SaveFile{}
	app.sourceSave = source

	if err := app.CloseSave(); err != nil {
		t.Fatalf("CloseSave: %v", err)
	}
	if app.sourceSave != source {
		t.Errorf("CloseSave must not touch a.sourceSave, got %v want %v", app.sourceSave, source)
	}
}
