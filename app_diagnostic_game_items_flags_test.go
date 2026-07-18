package main

import (
	"strings"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
	"github.com/oisis/EldenRing-SaveForge/backend/db"
	"github.com/oisis/EldenRing-SaveForge/backend/db/data"
)

// itemEventFlagsRegionSize covers every direct item-driven flag these tests
// touch, including the Spectral Steed Whistle companion flag 710520 (byte
// 88815). Sized well past the highest so no SetEventFlag runs out of bounds even
// after a GaItem insert shifts EventFlagsOffset by a small delta.
const itemEventFlagsRegionSize = 0x20000

// withItemEventFlags carves a zeroed event-flags region past the end of the slot
// buffer and wires EventFlagsOffset to it, so the Database Add can set the AoW,
// world-pickup and companion flags and db.GetEventFlag can read them back.
func withItemEventFlags(app *App) {
	slot := &app.save.Slots[0]
	slot.EventFlagsOffset = len(slot.Data)
	slot.Data = append(slot.Data, make([]byte, itemEventFlagsRegionSize)...)
}

// assertGameItemAddSuccess asserts the whole-lifecycle contract shared by every
// successful Database Add: global all-before -> all-planned -> all-finished
// grouping, outcome success, and stage completed. It holds even when every
// scoped item-flag field self-excludes (idempotency, no Event Flags region),
// because the direct physical item add still emits its own lifecycle.
func assertGameItemAddSuccess(t *testing.T, lc gameItemLifecycle) {
	t.Helper()
	assertGameItemPhaseGrouping(t, lc)
	if lc.outcome != string(characterChangeSuccess) {
		t.Errorf("outcome = %q, want success", lc.outcome)
	}
	if lc.stage != characterStageCompleted {
		t.Errorf("stage = %q, want completed", lc.stage)
	}
}

// assertItemFlagLifecycle asserts one item-flag field carries the full
// false -> true -> true lifecycle and that its finished value matches the flag
// read live from the real post-add slot.
func assertItemFlagLifecycle(t *testing.T, app *App, lc gameItemLifecycle, field string, flagID uint32) {
	t.Helper()
	assertContainerLifecycle(t, lc, field, "false", "true", "true")
	if got := readContainerFlag(&app.save.Slots[0], flagID); got != lc.finished[field] {
		t.Errorf("finished %s = %q != real slot flag %q", field, lc.finished[field], got)
	}
}

// A. Ash of War mapping: Lion's Claw (0x80002710) maps to duplication flag 65820.
// Adding it via the GaItem-capable fixture flips exactly that flag. The fixture's
// parsed slot already carries a zeroed Event Flags region within SlotSize, so no
// region is appended here — a GaItem add rebuilds the slot and RebuildSlotFull
// requires the buffer stay exactly SlotSize.
func TestGameItemsAddAoWFlagLifecycle(t *testing.T) {
	app := gaItemAddApp(t, true)

	const lionsClaw = uint32(0x80002710)
	flagID := data.AoWItemToFlagID[lionsClaw]

	res, err := app.AddItemsToCharacter(0, []uint32{lionsClaw}, 0, 0, 0, 0, 1, 0)
	if err != nil {
		t.Fatalf("AddItemsToCharacter: %v", err)
	}
	if res.Added != 1 {
		t.Fatalf("res.Added = %d, want 1", res.Added)
	}

	lc := collectGameItemLifecycle(t, app.journal.Tail())
	assertGameItemAddSuccess(t, lc)

	field := "item_0x80002710_aow_flag_" + giDec(flagID)
	assertItemFlagLifecycle(t, app, lc, field, flagID)
}

// B. World pickup mapping: Note: Hidden Cave (0x400021FC) maps to pickup flag
// 69600. The Database Add accepts the goods note and flips exactly that flag.
func TestGameItemsAddWorldPickupFlagLifecycle(t *testing.T) {
	app := gameItemAddApp(true)
	withItemEventFlags(app)

	const noteID = uint32(0x400021FC)
	flagID := data.WorldPickupFlagID[noteID]

	res, err := app.AddItemsToCharacter(0, []uint32{noteID}, 0, 0, 0, 0, 1, 0)
	if err != nil {
		t.Fatalf("AddItemsToCharacter: %v", err)
	}
	if res.Added != 1 {
		t.Fatalf("res.Added = %d, want 1", res.Added)
	}

	lc := collectGameItemLifecycle(t, app.journal.Tail())
	assertGameItemAddSuccess(t, lc)

	field := "item_0x400021FC_world_pickup_flag_" + giDec(flagID)
	assertItemFlagLifecycle(t, app, lc, field, flagID)
}

// C. Companion mapping: Spectral Steed Whistle sets a non-empty companion flag
// list. Every flag the writer sets must carry the full lifecycle; no transient
// or unrelated flag is asserted.
func TestGameItemsAddCompanionFlagLifecycle(t *testing.T) {
	app := gameItemAddApp(true)
	withItemEventFlags(app)

	flags := data.CompanionEventFlagsForItem(data.ItemSpectralSteedWhistle)
	if len(flags) == 0 {
		t.Fatalf("Spectral Steed Whistle has no companion flags")
	}

	res, err := app.AddItemsToCharacter(0, []uint32{data.ItemSpectralSteedWhistle}, 0, 0, 0, 0, 1, 0)
	if err != nil {
		t.Fatalf("AddItemsToCharacter: %v", err)
	}
	if res.Added != 1 {
		t.Fatalf("res.Added = %d, want 1", res.Added)
	}

	lc := collectGameItemLifecycle(t, app.journal.Tail())
	assertGameItemAddSuccess(t, lc)

	for _, flagID := range flags {
		field := "item_0x40000082_companion_flag_" + giDec(flagID)
		assertItemFlagLifecycle(t, app, lc, field, flagID)
	}
}

// D. Idempotency: a mapped flag pre-set before the add self-excludes — its
// before value already equals the clone value, so no lifecycle field is emitted
// for it. The normal item add still succeeds.
func TestGameItemsAddFlagIdempotency(t *testing.T) {
	app := gameItemAddApp(true)
	withItemEventFlags(app)

	const noteID = uint32(0x400021FC)
	flagID := data.WorldPickupFlagID[noteID]

	// Pre-set the world-pickup flag so it is already true on entry.
	slot := &app.save.Slots[0]
	if err := db.SetEventFlag(slot.Data[slot.EventFlagsOffset:], flagID, true); err != nil {
		t.Fatalf("pre-set flag: %v", err)
	}

	res, err := app.AddItemsToCharacter(0, []uint32{noteID}, 0, 0, 0, 0, 1, 0)
	if err != nil {
		t.Fatalf("AddItemsToCharacter: %v", err)
	}
	if res.Added != 1 {
		t.Fatalf("res.Added = %d, want 1", res.Added)
	}

	lc := collectGameItemLifecycle(t, app.journal.Tail())
	// The pre-set target flag self-excludes...
	assertNoField(t, lc, "item_0x400021FC_world_pickup_flag_"+giDec(flagID))
	// ...but the direct physical item add still emits its full lifecycle.
	assertGameItemAddSuccess(t, lc)
}

// E. No Event Flags region: the fixture leaves EventFlagsOffset at 0, so the
// writer sets no flags. No aow/world_pickup/companion field may be emitted,
// while the Database Add itself still succeeds.
func TestGameItemsAddFlagNoEventFlagsRegion(t *testing.T) {
	app := gameItemAddApp(true) // no withItemEventFlags: EventFlagsOffset stays 0

	res, err := app.AddItemsToCharacter(0, []uint32{data.ItemSpectralSteedWhistle}, 0, 0, 0, 0, 1, 0)
	if err != nil {
		t.Fatalf("AddItemsToCharacter: %v", err)
	}
	if res.Added != 1 {
		t.Fatalf("res.Added = %d, want 1", res.Added)
	}

	// The item still reached the executor: the whistle is a valid GaItem-free
	// goods add, so ensure the real slot changed.
	if app.save.Slots[0].Inventory.CommonItems[0].GaItemHandle == core.GaHandleEmpty {
		t.Fatalf("add did not mutate the slot")
	}

	lc := collectGameItemLifecycle(t, app.journal.Tail())
	// Every scoped flag field self-excludes without a region...
	for _, phase := range []map[string]string{lc.before, lc.planned, lc.finished} {
		for field := range phase {
			if strings.Contains(field, "_aow_flag_") ||
				strings.Contains(field, "_world_pickup_flag_") ||
				strings.Contains(field, "_companion_flag_") {
				t.Errorf("item flag field emitted without an Event Flags region: %q", field)
			}
		}
	}
	// ...but the direct physical item add still emits its full lifecycle.
	assertGameItemAddSuccess(t, lc)
}
