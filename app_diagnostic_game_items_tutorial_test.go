package main

import (
	"encoding/binary"
	"strings"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
	"github.com/oisis/EldenRing-SaveForge/backend/db/data"
)

// aboutItemCraftingID is a database-valid Goods item mapped in
// data.AboutTutorialID. Adding it drives core.AppendTutorialID to register the
// mapped TutorialParam row ID.
const aboutItemCraftingID = uint32(0x40002399)

// writeTutorialData lays a TutorialData header (size + count + ids) into the
// fixture's parsed TutorialData block. With a non-zero size it produces a valid,
// readable list the Database Add can append to. A size of 0 with a non-zero id
// count is a deliberately malformed (unreadable) block: core.ReadTutorialIDs
// rejects the count (count > (size-4)/4), so semantic membership always reads
// false — independent of whatever raw bytes core.AppendTutorialID may write into
// the malformed header (its size limit is enforced only when size > 0).
func writeTutorialData(t *testing.T, app *App, size uint32, ids ...uint32) {
	t.Helper()
	slot := &app.save.Slots[0]
	off := slot.TutorialDataOffset
	if off <= 0 || off+core.TutorialDataIDsOff+len(ids)*4 > len(slot.Data) {
		t.Fatalf("invalid TutorialDataOffset 0x%X for %d ids", off, len(ids))
	}
	binary.LittleEndian.PutUint32(slot.Data[off+4:], size)
	binary.LittleEndian.PutUint32(slot.Data[off+core.TutorialDataCountOff:], uint32(len(ids)))
	for i, id := range ids {
		binary.LittleEndian.PutUint32(slot.Data[off+core.TutorialDataIDsOff+i*4:], id)
	}
}

// tutorialField is the physical lifecycle field name for one TutorialData ID.
func tutorialField(tutorialID uint32) string {
	return "tutorial_id_" + giDec(tutorialID)
}

// A. Successful append: from an empty valid TutorialData list, adding About Item
// Crafting registers the mapped tutorial ID. Its field flips false -> true ->
// true, and the finished value matches membership read live from the real
// post-add slot.
func TestGameItemsAddTutorialAppend(t *testing.T) {
	app := gaItemAddApp(t, true)
	writeTutorialData(t, app, 0x400) // valid empty list (size 0x400, count 0)

	tutorialID, ok := data.AboutTutorialID[aboutItemCraftingID]
	if !ok {
		t.Fatalf("0x%08X not mapped in AboutTutorialID", aboutItemCraftingID)
	}

	res, err := app.AddItemsToCharacter(0, []uint32{aboutItemCraftingID}, 0, 0, 0, 0, 1, 0)
	if err != nil {
		t.Fatalf("AddItemsToCharacter: %v", err)
	}
	if res.Added != 1 {
		t.Fatalf("res.Added = %d, want 1", res.Added)
	}

	lc := collectGameItemLifecycle(t, app.journal.Tail())
	assertGameItemAddSuccess(t, lc)

	field := tutorialField(tutorialID)
	assertContainerLifecycle(t, lc, field, "false", "true", "true")

	// finished must equal live membership from the real post-add slot, via both
	// exported core read paths.
	has, hErr := core.HasTutorialID(&app.save.Slots[0], tutorialID)
	if hErr != nil {
		t.Fatalf("HasTutorialID: %v", hErr)
	}
	if want := boolFlagString(has); lc.finished[field] != want {
		t.Errorf("finished %s = %q, want %q (HasTutorialID)", field, lc.finished[field], want)
	}
	ids, rErr := core.ReadTutorialIDs(&app.save.Slots[0])
	if rErr != nil {
		t.Fatalf("ReadTutorialIDs: %v", rErr)
	}
	if !containsUint32(ids, tutorialID) {
		t.Errorf("ReadTutorialIDs = %v, missing appended id %d", ids, tutorialID)
	}
}

// B. Idempotency: with the mapped tutorial ID already seeded in a valid list, the
// add re-registers nothing, so no lifecycle field is emitted for it — while the
// direct physical item add still emits its full success lifecycle.
func TestGameItemsAddTutorialIdempotency(t *testing.T) {
	app := gaItemAddApp(t, true)

	tutorialID := data.AboutTutorialID[aboutItemCraftingID]
	writeTutorialData(t, app, 0x400, tutorialID) // valid list already containing the id

	res, err := app.AddItemsToCharacter(0, []uint32{aboutItemCraftingID}, 0, 0, 0, 0, 1, 0)
	if err != nil {
		t.Fatalf("AddItemsToCharacter: %v", err)
	}
	if res.Added != 1 {
		t.Fatalf("res.Added = %d, want 1", res.Added)
	}

	lc := collectGameItemLifecycle(t, app.journal.Tail())
	// The already-registered tutorial ID self-excludes (before == planned == true).
	assertNoField(t, lc, tutorialField(tutorialID))
	// ...but the direct physical item add still emits its full lifecycle.
	assertGameItemAddSuccess(t, lc)
}

// C. Unreadable/malformed TutorialData: a block whose header cannot be parsed
// (size 0 with a non-zero count) has no readable semantic membership, so
// core.ReadTutorialIDs fails both before and after the add. There is thus no
// observable membership transition and no tutorial field is emitted — while the
// Database Add itself still succeeds with its normal grouped lifecycle. The test
// asserts only the semantic membership contract, never that the malformed raw
// bytes stayed untouched.
func TestGameItemsAddTutorialMalformed(t *testing.T) {
	app := gaItemAddApp(t, true)
	// size 0 with a non-zero count is unreadable to core.ReadTutorialIDs, so
	// semantic membership reads false regardless of any raw bytes the writer emits.
	writeTutorialData(t, app, 0, 0)

	tutorialID := data.AboutTutorialID[aboutItemCraftingID]

	// The block is unreadable before the add.
	if _, err := core.ReadTutorialIDs(&app.save.Slots[0]); err == nil {
		t.Fatalf("ReadTutorialIDs unexpectedly succeeded before add on malformed block")
	}

	res, err := app.AddItemsToCharacter(0, []uint32{aboutItemCraftingID}, 0, 0, 0, 0, 1, 0)
	if err != nil {
		t.Fatalf("AddItemsToCharacter: %v", err)
	}
	if res.Added != 1 {
		t.Fatalf("res.Added = %d, want 1", res.Added)
	}

	// The block is still unreadable after the add: no observable membership exists.
	if _, err := core.ReadTutorialIDs(&app.save.Slots[0]); err == nil {
		t.Fatalf("ReadTutorialIDs unexpectedly succeeded after add on malformed block")
	}

	lc := collectGameItemLifecycle(t, app.journal.Tail())
	// No tutorial field in any phase.
	for _, phase := range []map[string]string{lc.before, lc.planned, lc.finished} {
		for field := range phase {
			if strings.Contains(field, "tutorial_id_") {
				t.Errorf("tutorial field emitted for malformed TutorialData: %q", field)
			}
		}
	}
	// Explicitly assert the mapped field is absent, and the add still succeeded.
	assertNoField(t, lc, tutorialField(tutorialID))
	assertGameItemAddSuccess(t, lc)
}

func boolFlagString(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func containsUint32(xs []uint32, v uint32) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}
