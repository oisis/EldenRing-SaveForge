package main

import (
	"bytes"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
	"github.com/oisis/EldenRing-SaveForge/backend/db/data"
)

// TestRevertFavorites_UndoesAdd is the Task A2 (Undo for Add) regression gate:
// a WriteSelectedToFavorites Add followed by RevertFavorites must return the
// Mirror bytes to their exact pre-Add state and drop the written name from
// favSlotNames.
func TestRevertFavorites_UndoesAdd(t *testing.T) {
	app, idx := realSaveAppForSave(t)

	const testName = "A2 Undo Regression Preset"

	orig := data.Presets
	data.Presets = append(append([]data.AppearancePreset{}, orig...), data.AppearancePreset{
		Name:      testName,
		BodyType:  1, // Type A (male)
		FaceModel: 6,
	})
	t.Cleanup(func() { data.Presets = orig })

	// Seed a sentinel name in an unrelated slot so we can prove the WHOLE
	// favSlotNames map is restored, not just the newly added entry.
	const sentinelSlot = 14
	const sentinelName = "sentinel — pre-existing"
	app.favSlotNames[sentinelSlot] = sentinelName

	// Snapshot the full favorites region before the Add.
	favLen := core.FavSlotCount * core.FavSlotSize
	before := make([]byte, favLen)
	copy(before, app.save.UserData10.Data[core.FavBaseOffset:core.FavBaseOffset+favLen])

	written, err := app.WriteSelectedToFavorites(idx, []string{testName})
	if err != nil {
		t.Fatalf("WriteSelectedToFavorites: %v", err)
	}
	if written != 1 {
		t.Fatalf("WriteSelectedToFavorites wrote %d presets, want 1", written)
	}

	// The Add must have changed the region and registered the name.
	after := app.save.UserData10.Data[core.FavBaseOffset : core.FavBaseOffset+favLen]
	if bytes.Equal(before, after) {
		t.Fatal("Add did not mutate the favorites region")
	}
	foundName := false
	for _, name := range app.favSlotNames {
		if name == testName {
			foundName = true
			break
		}
	}
	if !foundName {
		t.Fatal("Add did not register the preset name")
	}

	// Undo.
	if err := app.RevertFavorites(); err != nil {
		t.Fatalf("RevertFavorites: %v", err)
	}

	// Bytes must match the pre-Add snapshot exactly.
	reverted := app.save.UserData10.Data[core.FavBaseOffset : core.FavBaseOffset+favLen]
	if !bytes.Equal(before, reverted) {
		t.Fatal("favorites region did not revert to pre-Add bytes")
	}
	// Added name must be gone.
	for slot, name := range app.favSlotNames {
		if name == testName {
			t.Fatalf("preset name still present after undo (slot %d)", slot)
		}
	}
	// The sentinel — part of favSlotNames before the Add — must survive the
	// undo, proving the full map state is restored (not just the new entry).
	if app.favSlotNames[sentinelSlot] != sentinelName {
		t.Fatalf("sentinel name lost after undo: got %q", app.favSlotNames[sentinelSlot])
	}
	// The stack must be empty after popping the single Add.
	if len(app.favUndoStack) != 0 {
		t.Fatalf("favUndoStack depth = %d after undo, want 0", len(app.favUndoStack))
	}
}

// TestRemoveFavorite_InvalidatesAddUndo proves that removing a Mirror slot
// invalidates a pending Add-undo snapshot: replaying it could otherwise
// resurrect the removed entry. Remove itself is not undoable (Task A2).
func TestRemoveFavorite_InvalidatesAddUndo(t *testing.T) {
	app, idx := realSaveAppForSave(t)

	const testName = "A2 Remove Invalidates Undo Preset"

	orig := data.Presets
	data.Presets = append(append([]data.AppearancePreset{}, orig...), data.AppearancePreset{
		Name:      testName,
		BodyType:  1,
		FaceModel: 6,
	})
	t.Cleanup(func() { data.Presets = orig })

	written, err := app.WriteSelectedToFavorites(idx, []string{testName})
	if err != nil {
		t.Fatalf("WriteSelectedToFavorites: %v", err)
	}
	if written != 1 {
		t.Fatalf("WriteSelectedToFavorites wrote %d presets, want 1", written)
	}
	if got := app.GetFavoritesUndoDepth(); got != 1 {
		t.Fatalf("favorites undo depth = %d after Add, want 1", got)
	}

	// Locate and remove the slot the Add landed in.
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
	if err := app.RemoveFavoritePreset(slotIdx); err != nil {
		t.Fatalf("RemoveFavoritePreset: %v", err)
	}

	// Remove must have invalidated the Add-undo snapshot.
	if got := app.GetFavoritesUndoDepth(); got != 0 {
		t.Fatalf("favorites undo depth = %d after remove, want 0", got)
	}

	// RevertFavorites must refuse — the removed entry stays removed.
	err = app.RevertFavorites()
	if err == nil {
		t.Fatal("RevertFavorites succeeded after remove, want error")
	}
	if err.Error() != "nothing to undo for favorites" {
		t.Fatalf("RevertFavorites error = %q, want %q", err.Error(), "nothing to undo for favorites")
	}
	if app.favSlotNames[slotIdx] == testName {
		t.Fatal("removed entry was resurrected after failed undo")
	}
}

// TestRevertFavorites_MultiPresetIsOneUndo proves that a single
// WriteSelectedToFavorites adding several presets forms exactly one logical
// undo step, and one RevertFavorites removes all of them.
func TestRevertFavorites_MultiPresetIsOneUndo(t *testing.T) {
	app, idx := realSaveAppForSave(t)

	const nameA = "A2 Multi Preset A"
	const nameB = "A2 Multi Preset B"

	orig := data.Presets
	data.Presets = append(append([]data.AppearancePreset{}, orig...),
		data.AppearancePreset{Name: nameA, BodyType: 1, FaceModel: 6},
		data.AppearancePreset{Name: nameB, BodyType: 1, FaceModel: 3},
	)
	t.Cleanup(func() { data.Presets = orig })

	favLen := core.FavSlotCount * core.FavSlotSize
	before := make([]byte, favLen)
	copy(before, app.save.UserData10.Data[core.FavBaseOffset:core.FavBaseOffset+favLen])

	written, err := app.WriteSelectedToFavorites(idx, []string{nameA, nameB})
	if err != nil {
		t.Fatalf("WriteSelectedToFavorites: %v", err)
	}
	if written != 2 {
		t.Fatalf("WriteSelectedToFavorites wrote %d presets, want 2", written)
	}

	// Two presets, but exactly ONE undo entry.
	if got := app.GetFavoritesUndoDepth(); got != 1 {
		t.Fatalf("favorites undo depth = %d after multi-preset Add, want 1", got)
	}

	// One undo reverts both.
	if err := app.RevertFavorites(); err != nil {
		t.Fatalf("RevertFavorites: %v", err)
	}
	for slot, name := range app.favSlotNames {
		if name == nameA || name == nameB {
			t.Fatalf("preset %q still present after single undo (slot %d)", name, slot)
		}
	}
	reverted := app.save.UserData10.Data[core.FavBaseOffset : core.FavBaseOffset+favLen]
	if !bytes.Equal(before, reverted) {
		t.Fatal("favorites region did not revert both presets in one undo")
	}
	if got := app.GetFavoritesUndoDepth(); got != 0 {
		t.Fatalf("favorites undo depth = %d after undo, want 0", got)
	}
}
