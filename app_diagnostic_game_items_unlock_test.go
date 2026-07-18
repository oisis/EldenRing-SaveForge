package main

import (
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
	"github.com/oisis/EldenRing-SaveForge/backend/db"
)

// Database-tab single-item unlock lifecycle coverage for SetCookbookUnlocked,
// SetBellBearingUnlocked and SetMapRegionFlags. Fixture-free: state is built and
// mutated only through the public App / db API, and every record is read back
// from the in-memory journal.

// withUnlockEventFlags carves a zeroed event-flags region past the end of the
// slot buffer and wires EventFlagsOffset to it, so a Database unlock can flip its
// flag and db.GetEventFlag can read it back. size is explicit because bell bearing
// flags (~11.1M) resolve to a far higher byte offset than cookbook/map flags.
func withUnlockEventFlags(app *App, size int) {
	slot := &app.save.Slots[0]
	slot.EventFlagsOffset = len(slot.Data)
	slot.Data = append(slot.Data, make([]byte, size)...)
}

// firstPopulatedInvRow returns the index of the first non-empty held-inventory
// CommonItems row and its handle, or -1 when the section is empty.
func firstPopulatedInvRow(slot *core.SaveSlot) (int, uint32) {
	for i, it := range slot.Inventory.CommonItems {
		if it.GaItemHandle != core.GaHandleEmpty && it.GaItemHandle != 0 {
			return i, it.GaItemHandle
		}
	}
	return -1, 0
}

// unlockLifecycle collects, per lifecycle phase, a field->value map plus the flat
// record index of every phase, and asserts the action tag and character index on
// every record it sees.
type unlockLifecycle struct {
	before   map[string]string
	planned  map[string]string
	finished map[string]string
	beforeIx []int
	planIx   []int
	finIx    []int
	outcome  string
	stage    string
	count    int
}

func collectUnlockLifecycle(t *testing.T, records []diagnosticRecord, action, charIdx string) unlockLifecycle {
	t.Helper()
	lc := unlockLifecycle{
		before:   map[string]string{},
		planned:  map[string]string{},
		finished: map[string]string{},
	}
	for i, rec := range records {
		switch rec.Event {
		case eventGameItemsChangeBefore, eventGameItemsChangePlanned, eventGameItemsChangeFinished:
		default:
			continue
		}
		lc.count++
		field := operationField(rec, "field")
		switch rec.Event {
		case eventGameItemsChangeBefore:
			lc.before[field] = operationField(rec, "before")
			lc.beforeIx = append(lc.beforeIx, i)
		case eventGameItemsChangePlanned:
			lc.planned[field] = operationField(rec, "after")
			lc.planIx = append(lc.planIx, i)
		case eventGameItemsChangeFinished:
			lc.finished[field] = operationField(rec, "after")
			lc.finIx = append(lc.finIx, i)
			lc.outcome = operationField(rec, "outcome")
			lc.stage = operationField(rec, "stage")
		}
		if got := operationField(rec, "action"); got != action {
			t.Errorf("%s action = %q, want %q", rec.Event, got, action)
		}
		if got := operationField(rec, "character_index"); got != charIdx {
			t.Errorf("%s character_index = %q, want %q", rec.Event, got, charIdx)
		}
	}
	return lc
}

// assertUnlockPhaseGrouping proves every before precedes every planned and every
// planned precedes every finished (one global phase grouping).
func assertUnlockPhaseGrouping(t *testing.T, lc unlockLifecycle) {
	t.Helper()
	if len(lc.beforeIx) == 0 || len(lc.planIx) == 0 || len(lc.finIx) == 0 {
		t.Fatalf("missing a lifecycle phase: before=%d planned=%d finished=%d",
			len(lc.beforeIx), len(lc.planIx), len(lc.finIx))
	}
	if maxInt(lc.beforeIx) >= minInt(lc.planIx) {
		t.Errorf("a before record does not precede all planned records")
	}
	if maxInt(lc.planIx) >= minInt(lc.finIx) {
		t.Errorf("a planned record does not precede all finished records")
	}
}

func assertUnlockSuccess(t *testing.T, lc unlockLifecycle) {
	t.Helper()
	assertUnlockPhaseGrouping(t, lc)
	if lc.outcome != string(characterChangeSuccess) {
		t.Errorf("outcome = %q, want success", lc.outcome)
	}
	if lc.stage != characterStageCompleted {
		t.Errorf("stage = %q, want completed", lc.stage)
	}
}

// assertUnlockLifecycle checks the full before -> planned -> finished triple for
// one field.
func assertUnlockLifecycle(t *testing.T, lc unlockLifecycle, field, before, planned, finished string) {
	t.Helper()
	if got, ok := lc.before[field]; !ok || got != before {
		t.Errorf("before %s = %q (ok=%v), want %q", field, got, ok, before)
	}
	if got, ok := lc.planned[field]; !ok || got != planned {
		t.Errorf("planned %s = %q (ok=%v), want %q", field, got, ok, planned)
	}
	if got, ok := lc.finished[field]; !ok || got != finished {
		t.Errorf("finished %s = %q (ok=%v), want %q", field, got, ok, finished)
	}
}

// assertFieldAbsent proves a field self-excluded across every phase, i.e. its
// before value already equalled its planned value.
func assertFieldAbsent(t *testing.T, lc unlockLifecycle, field string) {
	t.Helper()
	if _, ok := lc.before[field]; ok {
		t.Errorf("field %q emitted a before record but its state did not change", field)
	}
	if _, ok := lc.planned[field]; ok {
		t.Errorf("field %q emitted a planned record but its state did not change", field)
	}
	if _, ok := lc.finished[field]; ok {
		t.Errorf("field %q emitted a finished record but its state did not change", field)
	}
}

// A. Cookbook unlock: flips its event flag and adds the mapped key item.
func TestGameItemsUnlockCookbookLifecycle(t *testing.T) {
	const cookbookFlag = uint32(67000)
	const cookbookItem = uint32(0x40002454)

	app := gameItemAddApp(false)
	withUnlockEventFlags(app, itemEventFlagsRegionSize)

	journal := freshRemoveJournal(app, true)
	if err := app.SetCookbookUnlocked(0, cookbookFlag, true); err != nil {
		t.Fatalf("SetCookbookUnlocked: %v", err)
	}

	slot := &app.save.Slots[0]
	row, _ := firstPopulatedInvRow(slot)
	if row < 0 {
		t.Fatalf("cookbook unlock added no inventory row")
	}

	lc := collectUnlockLifecycle(t, journal.Tail(), actionGameItemsUnlockCookbook, "0")
	assertUnlockSuccess(t, lc)

	assertUnlockLifecycle(t, lc, "event_flag_"+giDec(cookbookFlag), "false", "true", "true")

	p := "inventory_common_row_" + giDec(uint32(row))
	assertUnlockLifecycle(t, lc, p+"_item_id", giAbsent, giHex(cookbookItem), giHex(cookbookItem))
	assertUnlockLifecycle(t, lc, "inventory_common_header_count", "0", "1", "1")

	// finished equals the real post-unlock slot.
	if got := readContainerFlag(slot, cookbookFlag); got != lc.finished["event_flag_"+giDec(cookbookFlag)] {
		t.Errorf("finished flag %q != real slot %q", lc.finished["event_flag_"+giDec(cookbookFlag)], got)
	}
	if got := readGameItemField(slot, giSecInventoryCommon, row, giItemID); got != lc.finished[p+"_item_id"] {
		t.Errorf("finished item_id %q != real slot %q", lc.finished[p+"_item_id"], got)
	}
	if got := readInventoryCommonHeaderCount(slot); got != lc.finished["inventory_common_header_count"] {
		t.Errorf("finished header %q != real slot %q", lc.finished["inventory_common_header_count"], got)
	}
}

// B. Bell bearing unlock: flips its acquisition flag and removes the seeded key
// item (giving the BB to the Twin Maidens consumes it).
func TestGameItemsUnlockBellBearingLifecycle(t *testing.T) {
	const bellFlag = uint32(11109710)
	const bellItem = uint32(0x400022CE)

	app := gameItemAddApp(false)
	// Bell bearing flag 11109710 resolves to byte ~1.39M; size the region past it.
	withUnlockEventFlags(app, 0x160000)

	// Seed the BB key item (Debug off, so no lifecycle records leak into the test).
	if _, err := app.AddItemsToCharacter(0, []uint32{bellItem}, 0, 0, 0, 0, 1, 0); err != nil {
		t.Fatalf("seed AddItemsToCharacter: %v", err)
	}
	slot := &app.save.Slots[0]
	row, handle := firstPopulatedInvRow(slot)
	if row < 0 {
		t.Fatalf("seed produced no inventory row")
	}
	beforeHandle := giHex(handle)
	beforeItemID := giResolveItemID(slot, handle)

	journal := freshRemoveJournal(app, true)
	if err := app.SetBellBearingUnlocked(0, bellFlag, true); err != nil {
		t.Fatalf("SetBellBearingUnlocked: %v", err)
	}

	lc := collectUnlockLifecycle(t, journal.Tail(), actionGameItemsUnlockBellBearing, "0")
	assertUnlockSuccess(t, lc)

	assertUnlockLifecycle(t, lc, "event_flag_"+giDec(bellFlag), "false", "true", "true")

	p := "inventory_common_row_" + giDec(uint32(row))
	assertUnlockLifecycle(t, lc, p+"_handle", beforeHandle, "0x00000000", "0x00000000")
	assertUnlockLifecycle(t, lc, p+"_item_id", beforeItemID, giAbsent, giAbsent)
	assertUnlockLifecycle(t, lc, "inventory_common_header_count", "1", "0", "0")

	// finished equals the real post-unlock slot.
	if got := readContainerFlag(slot, bellFlag); got != lc.finished["event_flag_"+giDec(bellFlag)] {
		t.Errorf("finished flag %q != real slot %q", lc.finished["event_flag_"+giDec(bellFlag)], got)
	}
	if got := giHex(slot.Inventory.CommonItems[row].GaItemHandle); got != lc.finished[p+"_handle"] {
		t.Errorf("finished handle %q != real slot %q", lc.finished[p+"_handle"], got)
	}
}

// C. Map fragment unlock: flips the visible flag and adds the map fragment item.
func TestGameItemsUnlockMapFragmentLifecycle(t *testing.T) {
	const mapFlag = uint32(62010)
	const mapItem = uint32(0x40002198)

	app := gameItemAddApp(false)
	withUnlockEventFlags(app, itemEventFlagsRegionSize)

	journal := freshRemoveJournal(app, true)
	if err := app.SetMapRegionFlags(0, mapFlag, true); err != nil {
		t.Fatalf("SetMapRegionFlags: %v", err)
	}

	slot := &app.save.Slots[0]
	row, _ := firstPopulatedInvRow(slot)
	if row < 0 {
		t.Fatalf("map fragment unlock added no inventory row")
	}

	lc := collectUnlockLifecycle(t, journal.Tail(), actionGameItemsUnlockMapFragment, "0")
	assertUnlockSuccess(t, lc)

	assertUnlockLifecycle(t, lc, "event_flag_"+giDec(mapFlag), "false", "true", "true")

	p := "inventory_common_row_" + giDec(uint32(row))
	assertUnlockLifecycle(t, lc, p+"_item_id", giAbsent, giHex(mapItem), giHex(mapItem))
	assertUnlockLifecycle(t, lc, "inventory_common_header_count", "0", "1", "1")

	if got := readContainerFlag(slot, mapFlag); got != lc.finished["event_flag_"+giDec(mapFlag)] {
		t.Errorf("finished flag %q != real slot %q", lc.finished["event_flag_"+giDec(mapFlag)], got)
	}
	if got := readGameItemField(slot, giSecInventoryCommon, row, giItemID); got != lc.finished[p+"_item_id"] {
		t.Errorf("finished item_id %q != real slot %q", lc.finished[p+"_item_id"], got)
	}
}

// D. Idempotent repeat: the cookbook flag is pre-set (only the flag, not the
// item), so the unlock leaves the flag unchanged (self-excluded) while the item
// add remains a real change with a full before -> planned -> finished lifecycle.
func TestGameItemsUnlockCookbookIdempotentFlag(t *testing.T) {
	const cookbookFlag = uint32(67000)
	const cookbookItem = uint32(0x40002454)

	app := gameItemAddApp(false)
	withUnlockEventFlags(app, itemEventFlagsRegionSize)

	slot := &app.save.Slots[0]
	// Pre-set ONLY the flag, leaving the item absent.
	if err := db.SetEventFlag(slot.Data[slot.EventFlagsOffset:], cookbookFlag, true); err != nil {
		t.Fatalf("pre-seed flag: %v", err)
	}

	journal := freshRemoveJournal(app, true)
	if err := app.SetCookbookUnlocked(0, cookbookFlag, true); err != nil {
		t.Fatalf("SetCookbookUnlocked: %v", err)
	}

	row, _ := firstPopulatedInvRow(slot)
	if row < 0 {
		t.Fatalf("cookbook unlock added no inventory row")
	}

	lc := collectUnlockLifecycle(t, journal.Tail(), actionGameItemsUnlockCookbook, "0")
	assertUnlockSuccess(t, lc)

	// The already-set flag self-excludes: no record in any phase.
	assertFieldAbsent(t, lc, "event_flag_"+giDec(cookbookFlag))

	// The item add is still a real change with a full lifecycle.
	p := "inventory_common_row_" + giDec(uint32(row))
	assertUnlockLifecycle(t, lc, p+"_item_id", giAbsent, giHex(cookbookItem), giHex(cookbookItem))
	assertUnlockLifecycle(t, lc, "inventory_common_header_count", "0", "1", "1")
}

// E. Debug off: the unlock still mutates the slot, but no lifecycle records are
// retained.
func TestGameItemsUnlockDebugOff(t *testing.T) {
	const mapFlag = uint32(62010)

	app := gameItemAddApp(false)
	withUnlockEventFlags(app, itemEventFlagsRegionSize)

	journal := freshRemoveJournal(app, false) // Debug Mode disabled
	if err := app.SetMapRegionFlags(0, mapFlag, true); err != nil {
		t.Fatalf("SetMapRegionFlags: %v", err)
	}

	slot := &app.save.Slots[0]
	// Mutation actually occurred: flag set and fragment added.
	if got := readContainerFlag(slot, mapFlag); got != "true" {
		t.Errorf("map flag not set: %q", got)
	}
	if row, _ := firstPopulatedInvRow(slot); row < 0 {
		t.Errorf("map fragment not added to inventory")
	}

	lc := collectUnlockLifecycle(t, journal.Tail(), actionGameItemsUnlockMapFragment, "0")
	if lc.count != 0 {
		t.Errorf("Debug-off unlock emitted %d game_items_change_* records, want 0", lc.count)
	}
}
