package main

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
	"github.com/oisis/EldenRing-SaveForge/backend/db"
	"github.com/oisis/EldenRing-SaveForge/backend/db/data"
)

// goldenSeedID is a database-valid bolstering material with a non-empty
// data.BolsteringPickupFlags list (43 world-pickup flags). Its default storage
// cap is 0, so the mixed inventory+storage case drives the game-limits add path
// (GameMaxStorage 600) to make storage quantity actually apply.
const goldenSeedID = uint32(0x4000271A)

// sortedBolsteringFlags returns an ascending copy of an item's bolstering pickup
// flag list, mirroring the sort the writer (applyItemAddMutationPlan) and the
// diagnostic planner both apply before consuming the `set` counter.
func sortedBolsteringFlags(t *testing.T, itemID uint32) []uint32 {
	t.Helper()
	flags, ok := data.BolsteringPickupFlags[itemID]
	if !ok || len(flags) == 0 {
		t.Fatalf("item 0x%08X has no bolstering pickup flags", itemID)
	}
	sorted := make([]uint32, len(flags))
	copy(sorted, flags)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	return sorted
}

// withFullEventFlags carves a real-size zeroed event-flags region past the slot
// buffer and wires EventFlagsOffset to it. Golden Seed's pickup flags span the
// whole event-flags block (their IDs resolve through the BST, not a small linear
// range), so the region must be the true save size for every SetEventFlag to
// land in bounds.
func withFullEventFlags(app *App) {
	slot := &app.save.Slots[0]
	slot.EventFlagsOffset = len(slot.Data)
	slot.Data = append(slot.Data, make([]byte, core.EventFlagsByteCount)...)
}

// bolsteringField is the physical lifecycle field name for one bolstering pickup
// flag of an item.
func bolsteringField(itemID, flagID uint32) string {
	return fmt.Sprintf("item_0x%08X_bolstering_pickup_flag_%d", itemID, flagID)
}

// A. Mixed inventory + storage quantity: adding Golden Seed with inventory 2 and
// storage 1 gives the writer a `set` budget of 3, so the first three
// initially-false sorted flags each flip false -> true -> true, and each
// finished value matches the flag read live from the real post-add slot.
func TestGameItemsAddBolsteringMixedQuantity(t *testing.T) {
	app := gameItemAddApp(true)
	withFullEventFlags(app)

	sorted := sortedBolsteringFlags(t, goldenSeedID)

	// Game limits make Golden Seed's storage cap 600, so storage quantity applies
	// (its default storage cap is 0). qty = actualInv(2) + actualStorage(1) = 3.
	res, err := app.AddItemsToCharacterWithGameLimits(0, []uint32{goldenSeedID}, 0, 0, 0, 0, 2, 1)
	if err != nil {
		t.Fatalf("AddItemsToCharacterWithGameLimits: %v", err)
	}
	if res.Added != 1 {
		t.Fatalf("res.Added = %d, want 1", res.Added)
	}

	lc := collectGameItemLifecycle(t, app.journal.Tail())
	assertGameItemAddSuccess(t, lc)

	for i := 0; i < 3; i++ {
		field := bolsteringField(goldenSeedID, sorted[i])
		assertContainerLifecycle(t, lc, field, "false", "true", "true")
		if got := readContainerFlag(&app.save.Slots[0], sorted[i]); got != lc.finished[field] {
			t.Errorf("finished %s = %q != real slot flag %q", field, lc.finished[field], got)
		}
	}
	// The fourth sorted flag is beyond the set budget, so it must stay absent.
	assertNoField(t, lc, bolsteringField(goldenSeedID, sorted[3]))
}

// B. Already-set flag does not consume quantity: with the first sorted flag
// pre-set, a total quantity of 2 flips the next two initially-false sorted flags,
// not the first two flag IDs. This proves the planner tracks the writer's `set`
// counter rather than blindly taking the first N flags.
func TestGameItemsAddBolsteringSkipsPresetFlag(t *testing.T) {
	app := gameItemAddApp(true)
	withFullEventFlags(app)

	sorted := sortedBolsteringFlags(t, goldenSeedID)

	// Pre-set the first sorted flag so the writer skips it without spending budget.
	slot := &app.save.Slots[0]
	if err := db.SetEventFlag(slot.Data[slot.EventFlagsOffset:], sorted[0], true); err != nil {
		t.Fatalf("pre-set flag: %v", err)
	}

	res, err := app.AddItemsToCharacter(0, []uint32{goldenSeedID}, 0, 0, 0, 0, 2, 0)
	if err != nil {
		t.Fatalf("AddItemsToCharacter: %v", err)
	}
	if res.Added != 1 {
		t.Fatalf("res.Added = %d, want 1", res.Added)
	}

	lc := collectGameItemLifecycle(t, app.journal.Tail())
	assertGameItemAddSuccess(t, lc)

	// The pre-set first flag self-excludes (before == planned == true).
	assertNoField(t, lc, bolsteringField(goldenSeedID, sorted[0]))
	// The next two initially-false flags each carry the full lifecycle.
	for i := 1; i <= 2; i++ {
		assertContainerLifecycle(t, lc, bolsteringField(goldenSeedID, sorted[i]), "false", "true", "true")
	}
	// The budget of 2 stops there: the fourth sorted flag stays absent.
	assertNoField(t, lc, bolsteringField(goldenSeedID, sorted[3]))
}

// C. Duplicate submitted ID deduplication: submitting Golden Seed twice in one
// call must not double any bolstering field. Every emitted bolstering field
// appears exactly once per phase (three raw records total per changed field).
func TestGameItemsAddBolsteringDeduplicatesSubmittedID(t *testing.T) {
	app := gameItemAddApp(true)
	withFullEventFlags(app)

	res, err := app.AddItemsToCharacter(0, []uint32{goldenSeedID, goldenSeedID}, 0, 0, 0, 0, 3, 0)
	if err != nil {
		t.Fatalf("AddItemsToCharacter: %v", err)
	}
	if res.Added < 1 {
		t.Fatalf("res.Added = %d, want >= 1", res.Added)
	}

	// Count raw records per (event, field) straight from the journal tail so a
	// duplicated field record is visible even though the field->value maps would
	// collapse it.
	type key struct {
		event string
		field string
	}
	counts := map[key]int{}
	for _, rec := range app.journal.Tail() {
		field := operationField(rec, "field")
		if !strings.Contains(field, "_bolstering_pickup_flag_") {
			continue
		}
		switch rec.Event {
		case eventGameItemsChangeBefore, eventGameItemsChangePlanned, eventGameItemsChangeFinished:
			counts[key{rec.Event, field}]++
		}
	}
	if len(counts) == 0 {
		t.Fatal("no bolstering pickup flag records emitted")
	}
	for k, n := range counts {
		if n != 1 {
			t.Errorf("field %q in %s emitted %d times, want exactly 1", k.field, k.event, n)
		}
	}

	// Sanity: the add still produced a success lifecycle with global grouping.
	lc := collectGameItemLifecycle(t, app.journal.Tail())
	assertGameItemAddSuccess(t, lc)
}

// D. No Event Flags region: without a region the writer sets no flags and the
// clone leaves every flag unset, so no bolstering field is emitted — while the
// direct physical item add still emits its full success lifecycle.
func TestGameItemsAddBolsteringNoEventFlagsRegion(t *testing.T) {
	app := gameItemAddApp(true) // no region: EventFlagsOffset stays 0

	res, err := app.AddItemsToCharacter(0, []uint32{goldenSeedID}, 0, 0, 0, 0, 2, 0)
	if err != nil {
		t.Fatalf("AddItemsToCharacter: %v", err)
	}
	if res.Added != 1 {
		t.Fatalf("res.Added = %d, want 1", res.Added)
	}

	lc := collectGameItemLifecycle(t, app.journal.Tail())
	for _, phase := range []map[string]string{lc.before, lc.planned, lc.finished} {
		for field := range phase {
			if strings.Contains(field, "_bolstering_pickup_flag_") {
				t.Errorf("bolstering field emitted without an Event Flags region: %q", field)
			}
		}
	}
	assertGameItemAddSuccess(t, lc)
}
