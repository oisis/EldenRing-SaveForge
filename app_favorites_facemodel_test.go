package main

import (
	"encoding/binary"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
	"github.com/oisis/EldenRing-SaveForge/backend/db/data"
)

// TestWriteSelectedToFavorites_PersistsFaceModel is the Task A1 regression
// gate: a non-zero FaceModel must be written to Model ID index 0 through the
// same UI->PartsId (UI-1) encoding as every other Type A model. Before the fix
// WriteSelectedToFavorites never wrote index 0, so FaceModel silently became 0
// in every slot. The assertion guards against a regression back to 0.
func TestWriteSelectedToFavorites_PersistsFaceModel(t *testing.T) {
	app, idx := realSaveAppForSave(t)

	const testName = "A1 FaceModel Regression Preset"
	const wantFaceModel uint8 = 6       // preset UI value, non-zero on purpose
	const wantSavedFaceModel uint32 = 5 // UI-1 encoding, same as other Type A models

	// Register a throwaway Type A preset so it flows through the normal
	// findPresetByName -> WriteSelectedToFavorites path. Restore afterwards.
	orig := data.Presets
	data.Presets = append(append([]data.AppearancePreset{}, orig...), data.AppearancePreset{
		Name:      testName,
		BodyType:  1, // Type A (male) — the path that writes Model IDs
		FaceModel: wantFaceModel,
	})
	t.Cleanup(func() { data.Presets = orig })

	written, err := app.WriteSelectedToFavorites(idx, []string{testName})
	if err != nil {
		t.Fatalf("WriteSelectedToFavorites: %v", err)
	}
	if written != 1 {
		t.Fatalf("WriteSelectedToFavorites wrote %d presets, want 1", written)
	}

	// Locate the slot the write landed in via the name bookkeeping.
	slotIdx := -1
	for i, name := range app.favSlotNames {
		if name == testName {
			slotIdx = i
			break
		}
	}
	if slotIdx < 0 {
		t.Fatal("written preset not found in favSlotNames")
	}

	// Read the saved entry back and confirm the UI-1 encoded FaceModel value.
	ud := app.save.UserData10.Data
	off := core.FavBaseOffset + slotIdx*core.FavSlotSize
	gotFaceModel := binary.LittleEndian.Uint32(ud[off+core.FavOffModelIDs:])
	if gotFaceModel != wantSavedFaceModel {
		t.Fatalf("saved FaceModel = %d, want %d", gotFaceModel, wantSavedFaceModel)
	}
}
