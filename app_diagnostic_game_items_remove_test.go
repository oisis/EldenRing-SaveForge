package main

import (
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
)

// freshRemoveJournal replaces an app's journal with a fresh in-memory journal at
// the requested debug verbosity, so items seeded with Debug Mode off leave no
// records and only the removal under test is captured.
func freshRemoveJournal(app *App, debug bool) *DiagnosticJournal {
	journal := newInMemoryDiagnosticJournal()
	journal.SetDebugEnabled(debug)
	app.journal = journal
	return journal
}

// removeLifecycle is the remove-side counterpart of gameItemLifecycle: it
// collects per-phase field maps for the remove_items action and asserts the
// action tag and character index on every record.
type removeLifecycle struct {
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

func collectRemoveLifecycle(t *testing.T, records []diagnosticRecord, charIdx string) removeLifecycle {
	t.Helper()
	lc := removeLifecycle{
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
		if got := operationField(rec, "action"); got != actionGameItemsRemove {
			t.Errorf("%s action = %q, want %q", rec.Event, got, actionGameItemsRemove)
		}
		if got := operationField(rec, "character_index"); got != charIdx {
			t.Errorf("%s character_index = %q, want %q", rec.Event, got, charIdx)
		}
	}
	return lc
}

// assertRemovePhaseGrouping proves every before precedes every planned and every
// planned precedes every finished (one global phase grouping).
func assertRemovePhaseGrouping(t *testing.T, lc removeLifecycle) {
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

// assertRemoveLifecycle checks the full before -> planned -> finished triple for
// one field.
func assertRemoveLifecycle(t *testing.T, lc removeLifecycle, field, before, planned, finished string) {
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

// invRowForHandle returns the physical Inventory.CommonItems row holding handle.
func invRowForHandle(slot *core.SaveSlot, handle uint32) int {
	for i, it := range slot.Inventory.CommonItems {
		if it.GaItemHandle == handle {
			return i
		}
	}
	return -1
}

// A. Inventory CommonItems removal.
func TestGameItemsRemoveDiagnosticInventory(t *testing.T) {
	app := gameItemAddApp(false) // seed with Debug Mode off
	if _, err := app.AddItemsToCharacter(0, []uint32{storageHeaderItemID}, 0, 0, 0, 0, 2, 0); err != nil {
		t.Fatalf("seed AddItemsToCharacter: %v", err)
	}
	slot := &app.save.Slots[0]
	row := -1
	var handle uint32
	for i, it := range slot.Inventory.CommonItems {
		if it.GaItemHandle != core.GaHandleEmpty && it.GaItemHandle != 0 {
			row = i
			handle = it.GaItemHandle
			break
		}
	}
	if row < 0 {
		t.Fatalf("seed produced no inventory row")
	}
	// Capture pre-removal values that the removal will zero.
	beforeHandle := giHex(handle)
	beforeItemID := giResolveItemID(slot, handle)
	beforeQty := giDec(slot.Inventory.CommonItems[row].Quantity)
	beforeIndex := giDec(slot.Inventory.CommonItems[row].Index)

	journal := freshRemoveJournal(app, true)
	if err := app.RemoveItemsFromCharacter(0, []uint32{handle}, true, false); err != nil {
		t.Fatalf("RemoveItemsFromCharacter: %v", err)
	}

	lc := collectRemoveLifecycle(t, journal.Tail(), "0")
	assertRemovePhaseGrouping(t, lc)
	if lc.outcome != string(characterChangeSuccess) {
		t.Errorf("outcome = %q, want success", lc.outcome)
	}
	if lc.stage != characterStageCompleted {
		t.Errorf("stage = %q, want completed", lc.stage)
	}

	p := "inventory_common_row_" + giDec(uint32(row))
	// RemoveItemFromSlot zeroes handle/quantity but preserves Index = row.
	assertRemoveLifecycle(t, lc, p+"_handle", beforeHandle, "0x00000000", "0x00000000")
	assertRemoveLifecycle(t, lc, p+"_item_id", beforeItemID, giAbsent, giAbsent)
	assertRemoveLifecycle(t, lc, p+"_quantity", beforeQty, "0", "0")
	if beforeIndex != giDec(uint32(row)) {
		assertRemoveLifecycle(t, lc, p+"_index", beforeIndex, giDec(uint32(row)), giDec(uint32(row)))
	}
	// Header 1 -> 0 -> 0.
	assertRemoveLifecycle(t, lc, "inventory_common_header_count", "1", "0", "0")

	// finished equals the real post-removal slot.
	if got := giHex(slot.Inventory.CommonItems[row].GaItemHandle); got != lc.finished[p+"_handle"] {
		t.Errorf("finished handle %q != real slot %q", lc.finished[p+"_handle"], got)
	}
	if got := readInventoryCommonHeaderCount(slot); got != lc.finished["inventory_common_header_count"] {
		t.Errorf("finished header %q != real slot %q", lc.finished["inventory_common_header_count"], got)
	}
}

// B. Storage CommonItems removal.
func TestGameItemsRemoveDiagnosticStorage(t *testing.T) {
	app := gameItemAddApp(false)
	if _, err := app.AddItemsToCharacter(0, []uint32{storageHeaderItemID}, 0, 0, 0, 0, 0, 1); err != nil {
		t.Fatalf("seed AddItemsToCharacter: %v", err)
	}
	slot := &app.save.Slots[0]
	row := -1
	var handle uint32
	for i, it := range slot.Storage.CommonItems {
		if it.GaItemHandle != core.GaHandleEmpty && it.GaItemHandle != 0 {
			row = i
			handle = it.GaItemHandle
			break
		}
	}
	if row < 0 {
		t.Fatalf("seed produced no storage row")
	}
	beforeHandle := giHex(handle)
	beforeItemID := giResolveItemID(slot, handle)
	beforeQty := giDec(slot.Storage.CommonItems[row].Quantity)

	journal := freshRemoveJournal(app, true)
	if err := app.RemoveItemsFromCharacter(0, []uint32{handle}, false, true); err != nil {
		t.Fatalf("RemoveItemsFromCharacter: %v", err)
	}

	lc := collectRemoveLifecycle(t, journal.Tail(), "0")
	assertRemovePhaseGrouping(t, lc)
	if lc.outcome != string(characterChangeSuccess) {
		t.Errorf("outcome = %q, want success", lc.outcome)
	}
	if lc.stage != characterStageCompleted {
		t.Errorf("stage = %q, want completed", lc.stage)
	}

	p := "storage_common_row_" + giDec(uint32(row))
	assertRemoveLifecycle(t, lc, p+"_handle", beforeHandle, "0x00000000", "0x00000000")
	assertRemoveLifecycle(t, lc, p+"_item_id", beforeItemID, giAbsent, giAbsent)
	assertRemoveLifecycle(t, lc, p+"_quantity", beforeQty, "0", "0")
	// Header 1 -> 0 -> 0.
	assertRemoveLifecycle(t, lc, "storage_header_count", "1", "0", "0")

	if got := giHex(slot.Storage.CommonItems[row].GaItemHandle); got != lc.finished[p+"_handle"] {
		t.Errorf("finished handle %q != real slot %q", lc.finished[p+"_handle"], got)
	}
	if got := readStorageHeaderCount(slot); got != lc.finished["storage_header_count"] {
		t.Errorf("finished header %q != real slot %q", lc.finished["storage_header_count"], got)
	}
}

// C. GaItem-backed removal (non-stackable weapon).
func TestGameItemsRemoveDiagnosticGaItem(t *testing.T) {
	app := gaItemAddApp(t, false) // parsed in-memory GaItem fixture, Debug off
	const weaponID = uint32(0x00895440)
	if _, err := app.AddItemsToCharacter(0, []uint32{weaponID}, 0, 0, 0, 0, 1, 0); err != nil {
		t.Fatalf("seed AddItemsToCharacter: %v", err)
	}
	slot := &app.save.Slots[0]
	gaRow := -1
	for i, g := range slot.GaItems {
		if !g.IsEmpty() && g.ItemID == weaponID {
			gaRow = i
			break
		}
	}
	if gaRow < 0 {
		t.Fatalf("no serialized GaItem created for weapon 0x%08X", weaponID)
	}
	handle := slot.GaItems[gaRow].Handle
	invRow := invRowForHandle(slot, handle)
	if invRow < 0 {
		t.Fatalf("weapon handle 0x%08X not present in inventory", handle)
	}
	beforeGaHandle := giHex(handle)
	beforeInvHandle := giHex(handle)
	beforeInvItemID := giResolveItemID(slot, handle)

	journal := freshRemoveJournal(app, true)
	if err := app.RemoveItemsFromCharacter(0, []uint32{handle}, true, false); err != nil {
		t.Fatalf("RemoveItemsFromCharacter: %v", err)
	}

	lc := collectRemoveLifecycle(t, journal.Tail(), "0")
	assertRemovePhaseGrouping(t, lc)
	if lc.outcome != string(characterChangeSuccess) {
		t.Errorf("outcome = %q, want success", lc.outcome)
	}

	gaHandleField := "gaitem_row_" + giDec(uint32(gaRow)) + "_handle"
	gaItemField := "gaitem_row_" + giDec(uint32(gaRow)) + "_item_id"
	// RemoveItemFromSlot clears the serialized GaItem entry: handle -> 0, id -> absent.
	assertRemoveLifecycle(t, lc, gaHandleField, beforeGaHandle, "0x00000000", "0x00000000")
	assertRemoveLifecycle(t, lc, gaItemField, giHex(weaponID), giAbsent, giAbsent)

	invP := "inventory_common_row_" + giDec(uint32(invRow))
	assertRemoveLifecycle(t, lc, invP+"_handle", beforeInvHandle, "0x00000000", "0x00000000")
	assertRemoveLifecycle(t, lc, invP+"_item_id", beforeInvItemID, giAbsent, giAbsent)

	// finished equals the real post-removal slot.
	if got := giHex(slot.GaItems[gaRow].Handle); got != lc.finished[gaHandleField] {
		t.Errorf("finished %s = %q, want real slot %q", gaHandleField, lc.finished[gaHandleField], got)
	}
	if got := readGameItemField(slot, giSecGaItem, gaRow, giItemID); got != lc.finished[gaItemField] {
		t.Errorf("finished %s = %q, want real slot %q", gaItemField, lc.finished[gaItemField], got)
	}
}

// D. No-op handle: an unknown handle mutates nothing and emits no records.
func TestGameItemsRemoveDiagnosticNoOp(t *testing.T) {
	app := gameItemAddApp(false)
	if _, err := app.AddItemsToCharacter(0, []uint32{storageHeaderItemID}, 0, 0, 0, 0, 2, 0); err != nil {
		t.Fatalf("seed AddItemsToCharacter: %v", err)
	}
	slot := &app.save.Slots[0]
	row := -1
	var keepHandle uint32
	var keepQty uint32
	for i, it := range slot.Inventory.CommonItems {
		if it.GaItemHandle != core.GaHandleEmpty && it.GaItemHandle != 0 {
			row = i
			keepHandle = it.GaItemHandle
			keepQty = it.Quantity
			break
		}
	}
	if row < 0 {
		t.Fatalf("seed produced no inventory row")
	}
	headerBefore := readInventoryCommonHeaderCount(slot)

	journal := freshRemoveJournal(app, true)
	const unknown = uint32(0xDEADBEEF)
	if err := app.RemoveItemsFromCharacter(0, []uint32{unknown}, true, true); err != nil {
		t.Fatalf("RemoveItemsFromCharacter: %v", err)
	}

	lc := collectRemoveLifecycle(t, journal.Tail(), "0")
	if lc.count != 0 {
		t.Errorf("no-op removal emitted %d game_items_change_* records, want 0", lc.count)
	}
	// Seeded row is untouched.
	if got := slot.Inventory.CommonItems[row].GaItemHandle; got != keepHandle {
		t.Errorf("seeded handle mutated: got 0x%08X, want 0x%08X", got, keepHandle)
	}
	if got := slot.Inventory.CommonItems[row].Quantity; got != keepQty {
		t.Errorf("seeded quantity mutated: got %d, want %d", got, keepQty)
	}
	if got := readInventoryCommonHeaderCount(slot); got != headerBefore {
		t.Errorf("inventory header mutated: got %q, want %q", got, headerBefore)
	}
}

// F. Failed Remove lifecycle: core.RemoveItemFromSlot zeroes the matching
// in-memory inventory row, then fails its physical CheckBounds, so the public
// RemoveItemsFromCharacter returns an error with the real slot left partially
// mutated and no rollback. Positioning MagicOffset so invStart lands at the end
// of slot.Data makes the row write fail deterministically while the count header
// at invStart-4 stays bounds-valid.
func TestGameItemsRemoveDiagnosticInventoryError(t *testing.T) {
	app := gameItemAddApp(false) // seed with Debug Mode off
	if _, err := app.AddItemsToCharacter(0, []uint32{storageHeaderItemID}, 0, 0, 0, 0, 3, 0); err != nil {
		t.Fatalf("seed AddItemsToCharacter: %v", err)
	}
	slot := &app.save.Slots[0]
	row := -1
	var handle uint32
	for i, it := range slot.Inventory.CommonItems {
		if it.GaItemHandle != core.GaHandleEmpty && it.GaItemHandle != 0 {
			row = i
			handle = it.GaItemHandle
			break
		}
	}
	if row < 0 {
		t.Fatalf("seed produced no inventory row")
	}
	beforeHandle := giHex(handle)
	beforeItemID := giResolveItemID(slot, handle)
	beforeQty := giDec(slot.Inventory.CommonItems[row].Quantity)
	beforeIndex := giDec(slot.Inventory.CommonItems[row].Index)

	// Move invStart to the very end of the buffer: the seeded row's physical
	// write CheckBounds (off = invStart + row*InvRecordLen) now overflows and
	// fails, but MagicOffset stays positive and the header at invStart-4 remains
	// in bounds. RemoveItemFromSlot mutates the in-memory row before that check.
	slot.MagicOffset = len(slot.Data) - core.InvStartFromMagic
	if slot.MagicOffset <= 0 {
		t.Fatalf("computed MagicOffset %d is not positive", slot.MagicOffset)
	}

	journal := freshRemoveJournal(app, true)
	err := app.RemoveItemsFromCharacter(0, []uint32{handle}, true, false)
	if err == nil {
		t.Fatalf("RemoveItemsFromCharacter: want error, got nil")
	}

	// No rollback: the real in-memory row is left exactly as the partial write
	// left it (handle/quantity zeroed, index = row).
	if got := slot.Inventory.CommonItems[row].GaItemHandle; got != 0 {
		t.Errorf("row %d handle not partially mutated: got 0x%08X, want 0", row, got)
	}
	if got := slot.Inventory.CommonItems[row].Quantity; got != 0 {
		t.Errorf("row %d quantity not partially mutated: got %d, want 0", row, got)
	}

	lc := collectRemoveLifecycle(t, journal.Tail(), "0")
	assertRemovePhaseGrouping(t, lc)
	if lc.outcome != string(characterChangeError) {
		t.Errorf("outcome = %q, want error", lc.outcome)
	}
	if lc.stage != stageGameItemsRemoveItem {
		t.Errorf("stage = %q, want %q", lc.stage, stageGameItemsRemoveItem)
	}

	p := "inventory_common_row_" + giDec(uint32(row))
	// planned reflects the clone's identical partial mutation; finished re-reads
	// the real partially mutated slot. Both zero the handle/quantity, resolve the
	// item_id to absent, and keep index = row.
	assertRemoveLifecycle(t, lc, p+"_handle", beforeHandle, "0x00000000", "0x00000000")
	assertRemoveLifecycle(t, lc, p+"_item_id", beforeItemID, giAbsent, giAbsent)
	assertRemoveLifecycle(t, lc, p+"_quantity", beforeQty, "0", "0")
	if beforeIndex != giDec(uint32(row)) {
		assertRemoveLifecycle(t, lc, p+"_index", beforeIndex, giDec(uint32(row)), giDec(uint32(row)))
	}

	// finished equals the real partially mutated slot, field by field.
	if got := giHex(slot.Inventory.CommonItems[row].GaItemHandle); got != lc.finished[p+"_handle"] {
		t.Errorf("finished handle %q != real slot %q", lc.finished[p+"_handle"], got)
	}
	if got := giResolveItemID(slot, slot.Inventory.CommonItems[row].GaItemHandle); got != lc.finished[p+"_item_id"] {
		t.Errorf("finished item_id %q != real slot %q", lc.finished[p+"_item_id"], got)
	}
	if got := giDec(slot.Inventory.CommonItems[row].Quantity); got != lc.finished[p+"_quantity"] {
		t.Errorf("finished quantity %q != real slot %q", lc.finished[p+"_quantity"], got)
	}
}

// E. Debug off: removal still happens, no records retained.
func TestGameItemsRemoveDiagnosticDebugOff(t *testing.T) {
	app := gameItemAddApp(false)
	if _, err := app.AddItemsToCharacter(0, []uint32{storageHeaderItemID}, 0, 0, 0, 0, 2, 0); err != nil {
		t.Fatalf("seed AddItemsToCharacter: %v", err)
	}
	slot := &app.save.Slots[0]
	row := -1
	var handle uint32
	for i, it := range slot.Inventory.CommonItems {
		if it.GaItemHandle != core.GaHandleEmpty && it.GaItemHandle != 0 {
			row = i
			handle = it.GaItemHandle
			break
		}
	}
	if row < 0 {
		t.Fatalf("seed produced no inventory row")
	}

	journal := freshRemoveJournal(app, false) // Debug Mode disabled
	if err := app.RemoveItemsFromCharacter(0, []uint32{handle}, true, false); err != nil {
		t.Fatalf("RemoveItemsFromCharacter: %v", err)
	}

	// Removal actually occurred.
	if got := slot.Inventory.CommonItems[row].GaItemHandle; got != 0 {
		t.Errorf("row %d not cleared: handle = 0x%08X", row, got)
	}
	// No lifecycle records.
	lc := collectRemoveLifecycle(t, journal.Tail(), "0")
	if lc.count != 0 {
		t.Errorf("Debug-off removal emitted %d game_items_change_* records, want 0", lc.count)
	}
}
