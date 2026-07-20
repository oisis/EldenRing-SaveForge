package main

import (
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
	"github.com/oisis/EldenRing-SaveForge/backend/db"
)

// TestAddItemsToCharacter_CrimsonCrystalTearPickerCreatesConfirmedBundle is
// the real application-path test for the T090 bundle policy (not just
// core.AddItemsToSlotBatch): driving the public App.AddItemsToCharacter with
// the picker's only reachable ID (0x40002AFB — 0x40002AFA is "no_database"
// and never surfaced by db.GetItemsByCategory) must produce exactly the
// three confirmed records plus TutorialData 1590, never the canonical picker
// handle, never the empty Physick variant, and never the unconfirmed world
// flag 65030 (T090 explicitly shows it does not change for this pickup).
func TestAddItemsToCharacter_CrimsonCrystalTearPickerCreatesConfirmedBundle(t *testing.T) {
	app := gaItemAddApp(t, false)
	withContainerEventFlags(app)
	slot := &app.save.Slots[0]
	writeTutorialData(t, app, 0x400) // valid empty list — see app_diagnostic_game_items_tutorial_test.go
	usageBefore := core.CountSlotUsage(slot)

	res, err := app.AddItemsToCharacter(0, []uint32{crimsonCrystalTearPickerID}, 0, 0, 0, 0, 1, 0)
	if err != nil {
		t.Fatalf("AddItemsToCharacter: %v", err)
	}
	if res.Added != 1 {
		t.Fatalf("Added = %d, want 1", res.Added)
	}

	variantHandle := stackableHandle(crimsonCrystalTearVariantID)
	if !hasInventoryHandle(slot.Inventory.KeyItems, variantHandle) {
		t.Error("Crimson Crystal Tear variant (0x40002AFA) not written to KeyItems")
	}

	filledHandle := stackableHandle(db.ItemFlaskWondrousPhysickFilled)
	if !hasInventoryHandle(slot.Inventory.CommonItems, filledHandle) {
		t.Error("filled Flask of Wondrous Physick (0x400000FA) not written to CommonItems")
	}

	infoHandle := stackableHandle(aboutWondrousPhysickInfoItemID)
	if !hasInventoryHandle(slot.Inventory.CommonItems, infoHandle) {
		t.Error("About Flask of Wondrous Physick Info Item (0x4000239B) not written to CommonItems")
	}

	// Never the canonical picker handle, anywhere.
	canonicalHandle := stackableHandle(crimsonCrystalTearPickerID)
	if hasInventoryHandle(slot.Inventory.KeyItems, canonicalHandle) || hasInventoryHandle(slot.Inventory.CommonItems, canonicalHandle) {
		t.Error("standalone canonical Crimson Crystal Tear (0x40002AFB) leaked into the save")
	}

	// Never the empty Physick variant — the bundle must use the native filled one.
	emptyHandle := stackableHandle(db.ItemFlaskWondrousPhysickEmpty)
	if hasInventoryHandle(slot.Inventory.CommonItems, emptyHandle) {
		t.Error("empty Flask of Wondrous Physick (0x400000FB) written instead of the native filled variant")
	}

	hasTutorial, tErr := core.HasTutorialID(slot, crimsonCrystalTearBundleTutorialID)
	if tErr != nil {
		t.Fatalf("HasTutorialID: %v", tErr)
	}
	if !hasTutorial {
		t.Error("TutorialData 1590 not appended")
	}

	flags := slot.Data[slot.EventFlagsOffset:]
	if set, fErr := db.GetEventFlag(flags, 65030); fErr != nil {
		t.Fatalf("GetEventFlag(65030): %v", fErr)
	} else if set {
		t.Error("event flag 65030 was set automatically — T090 shows it does not change for this pickup")
	}

	// Each of the three records gets exactly one active GaItemData entry
	// (T090: three new active records, all flag 1) and none of them allocate
	// a serialized GaItem.
	usageAfter := core.CountSlotUsage(slot)
	if got, want := usageAfter.GaItemDataUsed-usageBefore.GaItemDataUsed, 3; got != want {
		t.Errorf("active GaItemData delta = %d, want %d", got, want)
	}
	if got, want := usageAfter.GaItemsUsed-usageBefore.GaItemsUsed, 0; got != want {
		t.Errorf("GaItems delta = %d, want %d (none of the three bundle records allocate a GaItem)", got, want)
	}
}

// TestAddItemsToCharacter_CrimsonCrystalTearBundleReAddIsSafeNoOp proves the
// "safe re-add" half of the contract: requesting the picker ID again after
// the bundle is already complete must not duplicate any record and must not
// error — it is reported as an already-satisfied skip.
func TestAddItemsToCharacter_CrimsonCrystalTearBundleReAddIsSafeNoOp(t *testing.T) {
	app := gaItemAddApp(t, false)
	withContainerEventFlags(app)
	slot := &app.save.Slots[0]

	if _, err := app.AddItemsToCharacter(0, []uint32{crimsonCrystalTearPickerID}, 0, 0, 0, 0, 1, 0); err != nil {
		t.Fatalf("first add: %v", err)
	}
	usageAfterFirst := core.CountSlotUsage(slot)

	res, err := app.AddItemsToCharacter(0, []uint32{crimsonCrystalTearPickerID}, 0, 0, 0, 0, 1, 0)
	if err != nil {
		t.Fatalf("second add (already complete): %v", err)
	}
	if res.Added != 0 {
		t.Errorf("Added = %d, want 0 (already-complete bundle is a no-op)", res.Added)
	}
	if len(res.SkippedExisting) != 1 || res.SkippedExisting[0].ItemID != crimsonCrystalTearPickerID {
		t.Errorf("SkippedExisting = %+v, want one entry for 0x%08X", res.SkippedExisting, crimsonCrystalTearPickerID)
	}

	usageAfterSecond := core.CountSlotUsage(slot)
	if usageAfterSecond != usageAfterFirst {
		t.Errorf("slot usage changed on re-add: before=%+v after=%+v", usageAfterFirst, usageAfterSecond)
	}
}

// TestAddItemsToCharacter_CrimsonCrystalTearBundlePartialStateErrors proves
// the "no silent merge" half of the contract: a partially-existing bundle
// (here, only the Info Item already present) has no lab evidence for how to
// reconcile, so the add must return a clear error and change nothing.
func TestAddItemsToCharacter_CrimsonCrystalTearBundlePartialStateErrors(t *testing.T) {
	app := gaItemAddApp(t, false)
	withContainerEventFlags(app)
	slot := &app.save.Slots[0]

	if err := core.AddItemsToSlotBatch(slot, []core.ItemToAdd{{ItemID: aboutWondrousPhysickInfoItemID, InvQty: 1}}); err != nil {
		t.Fatalf("seed Info Item: %v", err)
	}
	usageBefore := core.CountSlotUsage(slot)

	_, err := app.AddItemsToCharacter(0, []uint32{crimsonCrystalTearPickerID}, 0, 0, 0, 0, 1, 0)
	if err == nil {
		t.Fatal("AddItemsToCharacter succeeded on a partial bundle state, want a clear error")
	}

	usageAfter := core.CountSlotUsage(slot)
	if usageAfter != usageBefore {
		t.Errorf("slot usage changed despite the refused add: before=%+v after=%+v", usageBefore, usageAfter)
	}
}

// TestAddItemsToCharacter_CrimsonCrystalTearBundleConflictsWithStandalonePhysick
// proves the bundle refuses to run alongside an explicit standalone Flask of
// Wondrous Physick request in the same call — the bundle already creates its
// own confirmed Physick record, so mixing the two would create a duplicate
// logical flask this task has no evidence for.
func TestAddItemsToCharacter_CrimsonCrystalTearBundleConflictsWithStandalonePhysick(t *testing.T) {
	app := gaItemAddApp(t, false)
	withContainerEventFlags(app)
	slot := &app.save.Slots[0]
	usageBefore := core.CountSlotUsage(slot)

	_, err := app.AddItemsToCharacter(0, []uint32{crimsonCrystalTearPickerID, db.ItemFlaskWondrousPhysickEmpty}, 0, 0, 0, 0, 1, 0)
	if err == nil {
		t.Fatal("AddItemsToCharacter succeeded mixing the bundle with a standalone Physick request, want a clear error")
	}

	usageAfter := core.CountSlotUsage(slot)
	if usageAfter != usageBefore {
		t.Errorf("slot usage changed despite the refused add: before=%+v after=%+v", usageBefore, usageAfter)
	}
}
