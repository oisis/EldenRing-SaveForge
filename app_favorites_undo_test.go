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

// TestRemoveFavorite_EmptySlotIsNoOp proves that removing an empty Mirror slot
// changes nothing and creates no undo step.
func TestRemoveFavorite_EmptySlotIsNoOp(t *testing.T) {
	app, _ := realSaveAppForSave(t)

	// Find an empty slot (no FACE magic).
	favLen := core.FavSlotCount * core.FavSlotSize
	ud := app.save.UserData10.Data
	emptyIdx := -1
	for s := 0; s < core.FavSlotCount; s++ {
		off := core.FavBaseOffset + s*core.FavSlotSize
		if string(ud[off+core.FavOffMagic:off+core.FavOffMagic+4]) != "FACE" {
			emptyIdx = s
			break
		}
	}
	if emptyIdx < 0 {
		t.Skip("no empty Mirror slot available in fixture")
	}

	before := make([]byte, favLen)
	copy(before, ud[core.FavBaseOffset:core.FavBaseOffset+favLen])

	if err := app.RemoveFavoritePreset(emptyIdx); err != nil {
		t.Fatalf("RemoveFavoritePreset on empty slot: %v", err)
	}

	after := app.save.UserData10.Data[core.FavBaseOffset : core.FavBaseOffset+favLen]
	if !bytes.Equal(before, after) {
		t.Fatal("removing an empty slot changed the favorites bytes")
	}
	if got := app.GetFavoritesUndoDepth(); got != 0 {
		t.Fatalf("undo depth = %d after empty-slot remove, want 0", got)
	}
}

// TestRevertFavorites_AddRemoveIsChronological proves that Add and Remove share
// one chronological undo history: Add→Remove→Undo restores the removed entry
// (exact bytes + name), a second Undo reverts the earlier Add, and the stack
// depth walks 0 → 1 → 2 → 1 → 0.
func TestRevertFavorites_AddRemoveIsChronological(t *testing.T) {
	app, idx := realSaveAppForSave(t)

	const testName = "A2b Add-Remove Chronological Preset"

	orig := data.Presets
	data.Presets = append(append([]data.AppearancePreset{}, orig...), data.AppearancePreset{
		Name:      testName,
		BodyType:  1,
		FaceModel: 6,
	})
	t.Cleanup(func() { data.Presets = orig })

	favLen := core.FavSlotCount * core.FavSlotSize
	region := func() []byte {
		return app.save.UserData10.Data[core.FavBaseOffset : core.FavBaseOffset+favLen]
	}

	// State 0: before any change.
	if got := app.GetFavoritesUndoDepth(); got != 0 {
		t.Fatalf("undo depth = %d at start, want 0", got)
	}
	beforeAdd := append([]byte{}, region()...)

	// Add → depth 1.
	written, err := app.WriteSelectedToFavorites(idx, []string{testName})
	if err != nil {
		t.Fatalf("WriteSelectedToFavorites: %v", err)
	}
	if written != 1 {
		t.Fatalf("WriteSelectedToFavorites wrote %d presets, want 1", written)
	}
	if got := app.GetFavoritesUndoDepth(); got != 1 {
		t.Fatalf("undo depth = %d after Add, want 1", got)
	}

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
	afterAdd := append([]byte{}, region()...)

	// Remove → depth 2.
	if err := app.RemoveFavoritePreset(slotIdx); err != nil {
		t.Fatalf("RemoveFavoritePreset: %v", err)
	}
	if got := app.GetFavoritesUndoDepth(); got != 2 {
		t.Fatalf("undo depth = %d after Remove, want 2", got)
	}
	if _, ok := app.favSlotNames[slotIdx]; ok {
		t.Fatal("name still present right after Remove")
	}

	// Undo #1 → restores the removed entry exactly; depth 1.
	if err := app.RevertFavorites(); err != nil {
		t.Fatalf("RevertFavorites #1: %v", err)
	}
	if got := app.GetFavoritesUndoDepth(); got != 1 {
		t.Fatalf("undo depth = %d after Undo #1, want 1", got)
	}
	if !bytes.Equal(afterAdd, region()) {
		t.Fatal("Undo #1 did not restore the exact pre-Remove bytes")
	}
	if app.favSlotNames[slotIdx] != testName {
		t.Fatalf("Undo #1 did not restore the entry name: got %q", app.favSlotNames[slotIdx])
	}

	// Undo #2 → reverts the earlier Add; depth 0.
	if err := app.RevertFavorites(); err != nil {
		t.Fatalf("RevertFavorites #2: %v", err)
	}
	if got := app.GetFavoritesUndoDepth(); got != 0 {
		t.Fatalf("undo depth = %d after Undo #2, want 0", got)
	}
	if !bytes.Equal(beforeAdd, region()) {
		t.Fatal("Undo #2 did not restore the pre-Add bytes")
	}
	if _, ok := app.favSlotNames[slotIdx]; ok {
		t.Fatal("Undo #2 left the added name in place")
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
