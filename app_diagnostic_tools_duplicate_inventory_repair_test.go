package main

import (
	"encoding/binary"
	"strconv"
	"strings"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
)

// toolsRepairPhases returns the before/planned/finished tools_change_* records for
// one logical field of the duplicate-inventory-index repair, in lifecycle order.
func toolsRepairPhases(records []diagnosticRecord, field string) []diagnosticRecord {
	var out []diagnosticRecord
	for _, rec := range records {
		switch rec.Event {
		case eventToolsChangeBefore, eventToolsChangePlanned, eventToolsChangeFinished:
		default:
			continue
		}
		if operationField(rec, "action") != actionToolsRepairDuplicateInventoryIndices {
			continue
		}
		if operationField(rec, "field") == field {
			out = append(out, rec)
		}
	}
	return out
}

// toolsRepairRecordCount counts every tools_change_* record carrying the repair
// action, regardless of field — used to assert the no-op / Debug-off silence.
func toolsRepairRecordCount(records []diagnosticRecord) int {
	n := 0
	for _, rec := range records {
		switch rec.Event {
		case eventToolsChangeBefore, eventToolsChangePlanned, eventToolsChangeFinished:
			if operationField(rec, "action") == actionToolsRepairDuplicateInventoryIndices {
				n++
			}
		}
	}
	return n
}

// assertToolsRepairPhases asserts one field logs exactly before/planned/finished
// under the repair action, with the given values, the real character index, a
// clean before phase (no after/outcome/stage), a planned phase carrying only
// after, and a finished phase read from the real slot with the given
// outcome/stage. planned == final == the value read from the real slot proves the
// clone projection matched what actually landed.
func assertToolsRepairPhases(t *testing.T, records []diagnosticRecord, charIdx int, field, before, planned, final string, outcome characterChangeOutcome, stage string) {
	t.Helper()
	phases := toolsRepairPhases(records, field)
	if len(phases) != 3 {
		t.Fatalf("field %q: got %d records, want 3", field, len(phases))
	}
	wantEvents := []string{eventToolsChangeBefore, eventToolsChangePlanned, eventToolsChangeFinished}
	for i, rec := range phases {
		if rec.Event != wantEvents[i] {
			t.Errorf("field %q phase %d: event=%q want %q", field, i, rec.Event, wantEvents[i])
		}
		if got := operationField(rec, "before"); got != before {
			t.Errorf("field %q phase %d: before=%q want %q", field, i, got, before)
		}
		if got := operationField(rec, "action"); got != actionToolsRepairDuplicateInventoryIndices {
			t.Errorf("field %q phase %d: action=%q want %q", field, i, got, actionToolsRepairDuplicateInventoryIndices)
		}
		if got := operationField(rec, "character_index"); got != strconv.Itoa(charIdx) {
			t.Errorf("field %q phase %d: character_index=%q want %d", field, i, got, charIdx)
		}
	}
	// before phase carries only the before value.
	if got := operationField(phases[0], "after"); got != "" {
		t.Errorf("field %q before leaked after=%q", field, got)
	}
	if operationField(phases[0], "outcome") != "" || operationField(phases[0], "stage") != "" {
		t.Errorf("field %q before leaked terminal fields", field)
	}
	// planned phase carries after but no terminal fields.
	if got := operationField(phases[1], "after"); got != planned {
		t.Errorf("field %q planned after=%q want %q", field, got, planned)
	}
	if operationField(phases[1], "outcome") != "" || operationField(phases[1], "stage") != "" {
		t.Errorf("field %q planned leaked terminal fields", field)
	}
	// finished phase carries the real post-mutation value plus outcome/stage.
	fin := phases[2]
	if got := operationField(fin, "after"); got != final {
		t.Errorf("field %q finished after=%q want %q (real slot)", field, got, final)
	}
	if got := operationField(fin, "outcome"); got != string(outcome) {
		t.Errorf("field %q finished outcome=%q want %q", field, got, outcome)
	}
	if got := operationField(fin, "stage"); got != stage {
		t.Errorf("field %q finished stage=%q want %q", field, got, stage)
	}
}

// assertToolsRepairGrouping asserts the global phase grouping before(all) ->
// planned(all) -> finished(all) across every repair-action tools_change_* record.
func assertToolsRepairGrouping(t *testing.T, records []diagnosticRecord) {
	t.Helper()
	order := map[string]int{
		eventToolsChangeBefore:   0,
		eventToolsChangePlanned:  1,
		eventToolsChangeFinished: 2,
	}
	last := -1
	for _, rec := range records {
		if operationField(rec, "action") != actionToolsRepairDuplicateInventoryIndices {
			continue
		}
		phase, ok := order[rec.Event]
		if !ok {
			continue
		}
		if phase < last {
			t.Fatalf("phase grouping violated: %q after phase %d", rec.Event, last)
		}
		last = phase
	}
}

// assertFieldUnchanged fails if a field the repair never touches emitted any
// tools_change_* record — the self-exclusion contract.
func assertFieldUnchanged(t *testing.T, records []diagnosticRecord, field string) {
	t.Helper()
	if n := len(toolsRepairPhases(records, field)); n != 0 {
		t.Errorf("unchanged field %q emitted %d records, want 0", field, n)
	}
}

// A. Duplicate Common Item: the physically changed common row index and the
// advanced acquisition counter both log a full before/planned/finished lifecycle
// with planned == finished == the real slot; the untouched sibling row does not.
func TestToolsDuplicateRepairCommonLifecycle(t *testing.T) {
	app := repairFixture(
		[]core.InventoryItem{
			{GaItemHandle: 0xB0000334, Quantity: 99, Index: 552},
			{GaItemHandle: 0xB00003C0, Quantity: 99, Index: 552},
		},
		nil,
	)
	enableDebugJournal(t, app)
	before := core.CloneSlot(&app.save.Slots[0])

	report, err := app.RepairDuplicateInventoryIndices(0)
	if err != nil {
		t.Fatalf("RepairDuplicateInventoryIndices: %v", err)
	}
	if report.Changed != 1 {
		t.Fatalf("report.Changed = %d, want 1", report.Changed)
	}

	post := &app.save.Slots[0]
	records := app.journal.Tail()

	rowAfter := strconv.Itoa(int(post.Inventory.CommonItems[1].Index))
	if strconv.Itoa(int(report.Changes[0].NewIndex)) != rowAfter {
		t.Fatalf("report NewIndex %d != real slot row1 index %s", report.Changes[0].NewIndex, rowAfter)
	}
	assertToolsRepairPhases(t, records, 0, "inventory_common_row_1_index",
		"552", rowAfter, rowAfter, characterChangeSuccess, toolsStageCompleted)

	acqBefore := strconv.Itoa(int(before.Inventory.NextAcquisitionSortId))
	acqAfter := strconv.Itoa(int(post.Inventory.NextAcquisitionSortId))
	if acqBefore == acqAfter {
		t.Fatalf("acquisition counter did not advance (%s), fixture cannot exercise its lifecycle", acqAfter)
	}
	assertToolsRepairPhases(t, records, 0, "inventory_next_acquisition_sort_id",
		acqBefore, acqAfter, acqAfter, characterChangeSuccess, toolsStageCompleted)

	assertToolsRepairGrouping(t, records)
	// The preserved first occurrence and its untouched scalars never log.
	assertFieldUnchanged(t, records, "inventory_common_row_0_index")
	assertFieldUnchanged(t, records, "inventory_common_row_1_handle")
	assertFieldUnchanged(t, records, "inventory_common_row_1_quantity")
}

// B. Duplicate Key Item: the key row index logs its full lifecycle; the clean
// common row is never emitted.
func TestToolsDuplicateRepairKeyLifecycle(t *testing.T) {
	app := repairFixture(
		[]core.InventoryItem{
			{GaItemHandle: 0xB0000001, Quantity: 1, Index: 100},
		},
		[]core.InventoryItem{
			{GaItemHandle: 0xC0000001, Quantity: 1, Index: 200},
			{GaItemHandle: 0xC0000002, Quantity: 1, Index: 200},
		},
	)
	enableDebugJournal(t, app)

	report, err := app.RepairDuplicateInventoryIndices(0)
	if err != nil {
		t.Fatalf("RepairDuplicateInventoryIndices: %v", err)
	}
	if report.Changed != 1 || report.Changes[0].Scope != "inventory_key" {
		t.Fatalf("report = %+v, want a single inventory_key change", report)
	}

	post := &app.save.Slots[0]
	records := app.journal.Tail()

	keyAfter := strconv.Itoa(int(post.Inventory.KeyItems[1].Index))
	assertToolsRepairPhases(t, records, 0, "inventory_key_row_1_index",
		"200", keyAfter, keyAfter, characterChangeSuccess, toolsStageCompleted)

	assertToolsRepairGrouping(t, records)
	// The clean common row and the untouched first key row never log.
	assertFieldUnchanged(t, records, "inventory_common_row_0_index")
	assertFieldUnchanged(t, records, "inventory_common_row_0_handle")
	assertFieldUnchanged(t, records, "inventory_key_row_0_index")
}

// physickRepairFixture builds a slot whose two Wondrous Physick common rows are
// deduplicated by RepairDuplicateWondrousPhysick: the surplus row is zeroed, the
// held-inventory count header decremented, and the removed handle's GaMap entry
// and GaItem record cleared. MagicOffset is deliberately non-zero so the header
// reader (which self-disables at MagicOffset<=0) is active. Indices are distinct,
// so the index-repair step is a no-op and the physick removal is the only change.
func physickRepairFixture(t *testing.T) *App {
	t.Helper()
	const (
		magicOff    = 16
		keptHandle  = uint32(0xB00000FA) // resolves to filled Wondrous Physick
		dropHandle  = uint32(0xB00000FB) // resolves to empty Wondrous Physick (surplus)
		filledID    = uint32(0x400000FA)
		emptyID     = uint32(0x400000FB)
		headerCount = uint32(2)
	)
	commonStart := magicOff + core.InvStartFromMagic
	keyStart := commonStart + core.CommonItemCount*core.InvRecordLen + core.InvKeyCountHeader
	nextEquipIdxOff := keyStart + core.KeyItemCount*core.InvRecordLen
	nextAcqSortIdOff := nextEquipIdxOff + 4
	bufSize := nextAcqSortIdOff + 4 + 64

	common := []core.InventoryItem{
		{GaItemHandle: keptHandle, Quantity: 1, Index: 100},
		{GaItemHandle: dropHandle, Quantity: 1, Index: 102}, // stride-2 → distinct bucket; isolates the physick-removal path
	}

	app := NewApp()
	app.save = &core.SaveFile{}
	slot := &app.save.Slots[0]
	slot.Version = 1
	slot.MagicOffset = magicOff
	slot.Data = make([]byte, bufSize)
	for i, it := range common {
		off := commonStart + i*core.InvRecordLen
		binary.LittleEndian.PutUint32(slot.Data[off:], it.GaItemHandle)
		binary.LittleEndian.PutUint32(slot.Data[off+4:], it.Quantity)
		binary.LittleEndian.PutUint32(slot.Data[off+8:], it.Index)
	}
	slot.Inventory.CommonItems = append([]core.InventoryItem(nil), common...)
	slot.Inventory.NextAcquisitionSortId = 102
	slot.Inventory.NextEquipIndex = 102
	binary.LittleEndian.PutUint32(slot.Data[nextEquipIdxOff:], slot.Inventory.NextEquipIndex)
	binary.LittleEndian.PutUint32(slot.Data[nextAcqSortIdOff:], slot.Inventory.NextAcquisitionSortId)
	// Held-inventory CommonItems count header at commonStart-4; the physick removal
	// decrements it in place.
	binary.LittleEndian.PutUint32(slot.Data[commonStart-4:], headerCount)
	slot.GaMap = map[uint32]uint32{keptHandle: filledID, dropHandle: emptyID}
	slot.GaItems = []core.GaItemFull{
		{Handle: keptHandle, ItemID: filledID},
		{Handle: dropHandle, ItemID: emptyID},
	}

	if got := core.ScanDuplicateWondrousPhysick(slot); len(got) != 2 {
		t.Fatalf("fixture scan found %d physick occurrences, want 2", len(got))
	}
	return app
}

// C. Duplicate Wondrous Physick: the zeroed surplus row (handle/item_id/quantity/
// index), the decremented held-inventory header, the removed handle's GaMap entry
// and its cleared GaItem record all log a full lifecycle; the kept row and the
// allocation cursors never do.
func TestToolsDuplicateRepairWondrousPhysickLifecycle(t *testing.T) {
	app := physickRepairFixture(t)
	enableDebugJournal(t, app)

	if _, err := app.RepairDuplicateInventoryIndices(0); err != nil {
		t.Fatalf("RepairDuplicateInventoryIndices: %v", err)
	}

	post := &app.save.Slots[0]
	records := app.journal.Tail()

	// The surplus physical row is zeroed: handle -> empty, item_id -> absent,
	// quantity -> 0, index -> its own row number.
	assertToolsRepairPhases(t, records, 0, "inventory_common_row_1_handle",
		giHex(0xB00000FB), giHex(core.GaHandleEmpty), giHex(post.Inventory.CommonItems[1].GaItemHandle),
		characterChangeSuccess, toolsStageCompleted)
	assertToolsRepairPhases(t, records, 0, "inventory_common_row_1_item_id",
		giHex(0x400000FB), giAbsent, giAbsent, characterChangeSuccess, toolsStageCompleted)
	assertToolsRepairPhases(t, records, 0, "inventory_common_row_1_quantity",
		"1", "0", "0", characterChangeSuccess, toolsStageCompleted)
	assertToolsRepairPhases(t, records, 0, "inventory_common_row_1_index",
		"102", "1", strconv.Itoa(int(post.Inventory.CommonItems[1].Index)),
		characterChangeSuccess, toolsStageCompleted)

	// The held-inventory count header the physick removal rewrites.
	assertToolsRepairPhases(t, records, 0, "inventory_common_header_count",
		"2", "1", "1", characterChangeSuccess, toolsStageCompleted)

	// The removed handle's GaMap entry and its GaItem record.
	assertToolsRepairPhases(t, records, 0, "gaitem_map_handle_0xB00000FB_item_id",
		giHex(0x400000FB), giAbsent, giAbsent, characterChangeSuccess, toolsStageCompleted)
	assertToolsRepairPhases(t, records, 0, "gaitem_row_1_handle",
		giHex(0xB00000FB), giHex(core.GaHandleEmpty), giHex(post.GaItems[1].Handle),
		characterChangeSuccess, toolsStageCompleted)
	assertToolsRepairPhases(t, records, 0, "gaitem_row_1_item_id",
		giHex(0x400000FB), giAbsent, giAbsent, characterChangeSuccess, toolsStageCompleted)

	assertToolsRepairGrouping(t, records)

	// The kept physick row, the kept handle's GaMap entry, the allocation cursors
	// and the untouched acquisition counter never log — the writer does not move
	// them, so the projection must not either.
	for _, f := range []string{
		"inventory_common_row_0_handle", "inventory_common_row_0_index",
		"gaitem_row_0_handle", "gaitem_row_0_item_id",
		"gaitem_map_handle_0xB00000FA_item_id",
		"gaitem_next_aow_index", "gaitem_next_armament_index",
		"gaitem_next_handle", "gaitem_part_handle",
		"inventory_next_acquisition_sort_id",
	} {
		assertFieldUnchanged(t, records, f)
	}
}

// boundsErrorRepairFixture builds a slot whose common duplicate can be repaired in
// bounds but whose key duplicate write falls past the end of slot.Data, so
// RepairDuplicateInventoryIndices fixes the common row and then fails on the key
// row — a real partial mutation with no rollback.
func boundsErrorRepairFixture(t *testing.T) *App {
	t.Helper()
	const magicOff = 16
	commonStart := magicOff + core.InvStartFromMagic
	keyStart := commonStart + core.CommonItemCount*core.InvRecordLen + core.InvKeyCountHeader

	common := []core.InventoryItem{
		{GaItemHandle: 0xB0000010, Quantity: 1, Index: 500},
		{GaItemHandle: 0xB0000020, Quantity: 1, Index: 500},
	}
	key := []core.InventoryItem{
		{GaItemHandle: 0xC0000030, Quantity: 1, Index: 600},
		{GaItemHandle: 0xC0000040, Quantity: 1, Index: 600},
	}

	app := NewApp()
	app.save = &core.SaveFile{}
	slot := &app.save.Slots[0]
	slot.Version = 1
	slot.MagicOffset = magicOff
	// Data covers the common section but stops at keyStart, so any key index write
	// is out of bounds while the common writes succeed.
	slot.Data = make([]byte, keyStart)
	for i, it := range common {
		off := commonStart + i*core.InvRecordLen
		binary.LittleEndian.PutUint32(slot.Data[off:], it.GaItemHandle)
		binary.LittleEndian.PutUint32(slot.Data[off+4:], it.Quantity)
		binary.LittleEndian.PutUint32(slot.Data[off+8:], it.Index)
	}
	slot.Inventory.CommonItems = append([]core.InventoryItem(nil), common...)
	slot.Inventory.KeyItems = append([]core.InventoryItem(nil), key...)
	slot.Inventory.NextAcquisitionSortId = 601
	slot.Inventory.NextEquipIndex = 601
	slot.GaMap = make(map[uint32]uint32)
	return app
}

// D. Error after partial mutation: the endpoint returns the error without a
// rollback, the finished phase reads the real partially-changed slot with
// outcome=error and the repair stage, and the high-level repair events keep their
// contract.
func TestToolsDuplicateRepairErrorAfterPartialMutation(t *testing.T) {
	app := boundsErrorRepairFixture(t)
	enableDebugJournal(t, app)

	report, err := app.RepairDuplicateInventoryIndices(0)
	if err == nil || !strings.Contains(err.Error(), "out of bounds") {
		t.Fatalf("err = %v, want an out-of-bounds repair error", err)
	}
	if report.Changed != 0 {
		t.Fatalf("report.Changed = %d, want 0 (error discards the report)", report.Changed)
	}

	post := &app.save.Slots[0]
	// No rollback: the common row was really reassigned before the key error.
	if post.Inventory.CommonItems[1].Index == 500 {
		t.Fatalf("common row 1 index unchanged (%d); expected a real partial mutation", post.Inventory.CommonItems[1].Index)
	}
	rowAfter := strconv.Itoa(int(post.Inventory.CommonItems[1].Index))

	records := app.journal.Tail()
	// finished reads the real partially-changed slot, not the planned/snapshot.
	assertToolsRepairPhases(t, records, 0, "inventory_common_row_1_index",
		"500", rowAfter, rowAfter, characterChangeError, stageToolsRepairDuplicateInventoryIndices)
	assertToolsRepairGrouping(t, records)
	// The key write never landed and the counter advance was never reached.
	assertFieldUnchanged(t, records, "inventory_key_row_1_index")
	assertFieldUnchanged(t, records, "inventory_next_acquisition_sort_id")

	// The existing high-level repair events keep their contract.
	finished := operationEvent(t, records, "inventory_integrity_repair_finished")
	if finished.Level != levelError {
		t.Errorf("high-level finished level = %q, want error", finished.Level)
	}
	if got := operationField(finished, "outcome"); got != "error" {
		t.Errorf("high-level finished outcome = %q, want error", got)
	}
	if got := operationField(finished, "attempted_actions"); got != "reassign_duplicate_inventory_indices" {
		t.Errorf("attempted_actions = %q, want reassign only (physick never reached)", got)
	}
}

// E1. A clean slot is a true no-op: the repair changes nothing and emits no
// tools_change_* record even with Debug Mode on.
func TestToolsDuplicateRepairNoOpEmitsNoFieldRecords(t *testing.T) {
	app := repairFixture(
		[]core.InventoryItem{
			{GaItemHandle: 0xB0000001, Quantity: 1, Index: 100},
			{GaItemHandle: 0xB0000002, Quantity: 1, Index: 102}, // stride-2 → distinct bucket → genuine no-op
		},
		nil,
	)
	enableDebugJournal(t, app)

	report, err := app.RepairDuplicateInventoryIndices(0)
	if err != nil {
		t.Fatalf("RepairDuplicateInventoryIndices: %v", err)
	}
	if report.Changed != 0 {
		t.Fatalf("report.Changed = %d, want 0", report.Changed)
	}
	if n := toolsRepairRecordCount(app.journal.Tail()); n != 0 {
		t.Fatalf("no-op emitted %d tools_change records, want 0", n)
	}
}

// E2. With Debug Mode off the repair still runs, but no tools_change_* record is
// emitted and the real writer runs exactly once.
func TestToolsDuplicateRepairDebugOffEmitsNoFieldRecords(t *testing.T) {
	app := repairFixture(
		[]core.InventoryItem{
			{GaItemHandle: 0xB0000334, Quantity: 99, Index: 552},
			{GaItemHandle: 0xB00003C0, Quantity: 99, Index: 552},
		},
		nil,
	)
	j, err := newDiagnosticJournalInDir(t.TempDir())
	if err != nil {
		t.Fatalf("newDiagnosticJournalInDir: %v", err)
	}
	j.SetDebugEnabled(false)
	t.Cleanup(func() { _ = j.Close() })
	app.journal = j

	report, err := app.RepairDuplicateInventoryIndices(0)
	if err != nil {
		t.Fatalf("RepairDuplicateInventoryIndices: %v", err)
	}
	if report.Changed != 1 {
		t.Fatalf("report.Changed = %d, want 1 (repair must still run)", report.Changed)
	}
	if issues := core.ScanDuplicateInventoryIndices(&app.save.Slots[0]); len(issues) != 0 {
		t.Fatalf("repair left %d duplicate(s) with Debug off", len(issues))
	}
	if n := toolsRepairRecordCount(app.journal.Tail()); n != 0 {
		t.Fatalf("Debug-off emitted %d tools_change records, want 0", n)
	}
}
