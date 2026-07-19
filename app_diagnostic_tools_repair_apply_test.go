package main

import (
	"bytes"
	"strconv"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
)

// toolsApplyPhases returns the before/planned/finished tools_change_* records for
// one logical field of an ApplyRepairsLoaded batch, in lifecycle order.
func toolsApplyPhases(records []diagnosticRecord, field string) []diagnosticRecord {
	var out []diagnosticRecord
	for _, rec := range records {
		switch rec.Event {
		case eventToolsChangeBefore, eventToolsChangePlanned, eventToolsChangeFinished:
		default:
			continue
		}
		if operationField(rec, "action") != actionToolsApplyRepairsLoaded {
			continue
		}
		if operationField(rec, "field") == field {
			out = append(out, rec)
		}
	}
	return out
}

// toolsApplyChangeCount counts every tools_change_* record carrying the apply
// action, regardless of field — used to assert the no-op / early-error / Debug-off
// silence.
func toolsApplyChangeCount(records []diagnosticRecord) int {
	n := 0
	for _, rec := range records {
		switch rec.Event {
		case eventToolsChangeBefore, eventToolsChangePlanned, eventToolsChangeFinished:
			if operationField(rec, "action") == actionToolsApplyRepairsLoaded {
				n++
			}
		}
	}
	return n
}

// toolsApplyOperationCount counts the tools_operation_* records carrying the apply
// action — Debug off must emit zero, an early error exactly two (requested +
// finished).
func toolsApplyOperationCount(records []diagnosticRecord) int {
	n := 0
	for _, rec := range records {
		switch rec.Event {
		case eventToolsOperationRequested, eventToolsOperationFinished:
			if operationField(rec, "action") == actionToolsApplyRepairsLoaded {
				n++
			}
		}
	}
	return n
}

// assertToolsApplyPhases asserts one field logs exactly before/planned/finished
// under the apply action with the given values, the real character index, a clean
// before phase (no after/outcome/stage), a planned phase carrying only after, and
// a finished phase read from the real slot with the given outcome/stage. planned ==
// final == the value read from the real slot proves the clone projection matched
// what actually landed.
func assertToolsApplyPhases(t *testing.T, records []diagnosticRecord, charIdx int, field, before, planned, final string, outcome characterChangeOutcome, stage string) {
	t.Helper()
	phases := toolsApplyPhases(records, field)
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
		if got := operationField(rec, "action"); got != actionToolsApplyRepairsLoaded {
			t.Errorf("field %q phase %d: action=%q want %q", field, i, got, actionToolsApplyRepairsLoaded)
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
	// finished phase carries the real post-batch value plus outcome/stage.
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

// assertToolsApplyFieldUnchanged fails if a field the batch never touches emitted
// any tools_change_* record — the self-exclusion contract.
func assertToolsApplyFieldUnchanged(t *testing.T, records []diagnosticRecord, field string) {
	t.Helper()
	if n := len(toolsApplyPhases(records, field)); n != 0 {
		t.Errorf("unchanged field %q emitted %d records, want 0", field, n)
	}
}

// assertToolsApplyGlobalOrder asserts the strict global order across every
// apply-action record: tools_operation_requested -> before(all) -> planned(all) ->
// finished(all) -> tools_operation_finished, and that the operation bracket is
// present.
func assertToolsApplyGlobalOrder(t *testing.T, records []diagnosticRecord) {
	t.Helper()
	rank := map[string]int{
		eventToolsOperationRequested: 0,
		eventToolsChangeBefore:       1,
		eventToolsChangePlanned:      2,
		eventToolsChangeFinished:     3,
		eventToolsOperationFinished:  4,
	}
	last := -1
	var sawRequested, sawFinished bool
	for _, rec := range records {
		if operationField(rec, "action") != actionToolsApplyRepairsLoaded {
			continue
		}
		r, ok := rank[rec.Event]
		if !ok {
			continue
		}
		if r < last {
			t.Fatalf("global order violated: %q (rank %d) after rank %d", rec.Event, r, last)
		}
		last = r
		switch rec.Event {
		case eventToolsOperationRequested:
			sawRequested = true
		case eventToolsOperationFinished:
			sawFinished = true
		}
	}
	if !sawRequested || !sawFinished {
		t.Fatalf("missing operation bracket: requested=%v finished=%v", sawRequested, sawFinished)
	}
}

// newDebugApplyApp returns an App with a Debug-on journal and no save loaded, for
// the early-validation cases.
func newDebugApplyApp(t *testing.T) *App {
	t.Helper()
	app := NewApp()
	enableDebugJournal(t, app)
	return app
}

// A. A successful clamp_quantity repair: the one changed field logs a full
// before/planned/finished lifecycle with planned == finished == the real slot, the
// action, real character index, success/completed, and the strict global order.
func TestToolsRepairApply_ClampQuantityLifecycle(t *testing.T) {
	app := NewApp()
	app.save = &core.SaveFile{}
	app.save.Slots[0] = *buildApplyInvFixture(t, []core.InventoryItem{
		{GaItemHandle: smithingStoneHandleQty, Quantity: 1500, Index: 500},
	})
	enableDebugJournal(t, app)
	slot := &app.save.Slots[0]

	rep, err := app.ApplyRepairsLoaded(0, []RepairApplyTarget{clampTarget(slot, 0)}, false)
	if err != nil {
		t.Fatalf("ApplyRepairsLoaded: %v", err)
	}
	if rep.Applied != 1 || rep.Failed != 0 {
		t.Fatalf("want applied=1 failed=0, got %+v", rep)
	}
	post := &app.save.Slots[0]
	if post.Inventory.CommonItems[0].Quantity != 999 {
		t.Fatalf("quantity = %d, want clamped 999", post.Inventory.CommonItems[0].Quantity)
	}
	records := app.journal.Tail()

	finalQty := strconv.Itoa(int(post.Inventory.CommonItems[0].Quantity))
	assertToolsApplyPhases(t, records, 0, "inventory_common_row_0_quantity",
		"1500", finalQty, finalQty, characterChangeSuccess, toolsStageCompleted)

	req := operationEvent(t, records, eventToolsOperationRequested)
	if got := operationField(req, "action"); got != actionToolsApplyRepairsLoaded {
		t.Errorf("requested action=%q, want %q", got, actionToolsApplyRepairsLoaded)
	}
	fin := operationEvent(t, records, eventToolsOperationFinished)
	if operationField(fin, "outcome") != string(characterChangeSuccess) || operationField(fin, "stage") != toolsStageCompleted {
		t.Errorf("operation finished = %q/%q, want success/completed",
			operationField(fin, "outcome"), operationField(fin, "stage"))
	}
	assertToolsApplyGlobalOrder(t, records)
	// A clamp touches only the quantity; the sibling scalars and the header never log.
	assertToolsApplyFieldUnchanged(t, records, "inventory_common_row_0_handle")
	assertToolsApplyFieldUnchanged(t, records, "inventory_common_row_0_index")
	assertToolsApplyFieldUnchanged(t, records, "inventory_common_header_count")
}

// B. A remove_record repair zeroes the physical row (handle -> empty, index -> its
// own row number) and rewrites the held-inventory count header; both log full
// lifecycles, the untouched sibling row does not. The fixture carries no GaMap, so
// item_id stays absent on both sides and self-excludes.
func TestToolsRepairApply_RemoveRecordLifecycle(t *testing.T) {
	app := NewApp()
	app.save = &core.SaveFile{}
	app.save.Slots[0] = *twoZeroQtySlot(t)
	enableDebugJournal(t, app)
	slot := &app.save.Slots[0]

	rep, err := app.ApplyRepairsLoaded(0, []RepairApplyTarget{quantityZeroTarget(slot, 0)}, false)
	if err != nil {
		t.Fatalf("ApplyRepairsLoaded: %v", err)
	}
	if rep.Applied != 1 || rep.Failed != 0 {
		t.Fatalf("want applied=1 failed=0, got %+v", rep)
	}
	post := &app.save.Slots[0]
	if post.Inventory.CommonItems[0].GaItemHandle != 0 {
		t.Fatalf("row 0 not removed: %+v", post.Inventory.CommonItems[0])
	}
	records := app.journal.Tail()

	assertToolsApplyPhases(t, records, 0, "inventory_common_row_0_handle",
		giHex(0xA00017B6), giHex(core.GaHandleEmpty), giHex(post.Inventory.CommonItems[0].GaItemHandle),
		characterChangeSuccess, toolsStageCompleted)
	assertToolsApplyPhases(t, records, 0, "inventory_common_row_0_index",
		"500", "0", strconv.Itoa(int(post.Inventory.CommonItems[0].Index)),
		characterChangeSuccess, toolsStageCompleted)
	assertToolsApplyPhases(t, records, 0, "inventory_common_header_count",
		"2", "1", "1", characterChangeSuccess, toolsStageCompleted)

	assertToolsApplyGlobalOrder(t, records)
	// The untouched sibling row never logs.
	assertToolsApplyFieldUnchanged(t, records, "inventory_common_row_1_handle")
	assertToolsApplyFieldUnchanged(t, records, "inventory_common_row_1_index")
}

// C. A partial batch: the first target removes row 1 (applied), the second is a
// deterministically stale row-0 target (failed, no mutation). The applied target's
// real changes are logged, finished read from the real slot, and both the terminal
// tools_change_finished and the tools_operation_finished carry error/apply_repairs
// even though the endpoint returns a nil error. The stale row leaves no change, so
// it never logs.
func TestToolsRepairApply_PartialBatchLifecycle(t *testing.T) {
	app := NewApp()
	app.save = &core.SaveFile{}
	app.save.Slots[0] = *twoZeroQtySlot(t)
	enableDebugJournal(t, app)
	slot := &app.save.Slots[0]

	good := quantityZeroTarget(slot, 1)
	stale := quantityZeroTarget(slot, 0)
	stale.Fingerprint = "deadbeefdeadbeef" // deterministic failure

	rep, err := app.ApplyRepairsLoaded(0, []RepairApplyTarget{good, stale}, false)
	if err != nil {
		t.Fatalf("ApplyRepairsLoaded: %v", err)
	}
	if rep.Applied != 1 || rep.Failed != 1 || rep.Stopped {
		t.Fatalf("want applied=1 failed=1 not-stopped, got %+v", rep)
	}
	post := &app.save.Slots[0]
	if post.Inventory.CommonItems[1].GaItemHandle != 0 {
		t.Fatalf("row 1 not removed: %+v", post.Inventory.CommonItems[1])
	}
	if post.Inventory.CommonItems[0].GaItemHandle == 0 {
		t.Fatal("stale row 0 was mutated despite the fingerprint mismatch")
	}
	records := app.journal.Tail()

	assertToolsApplyPhases(t, records, 0, "inventory_common_row_1_handle",
		giHex(0xA00017B6), giHex(core.GaHandleEmpty), giHex(post.Inventory.CommonItems[1].GaItemHandle),
		characterChangeError, stageToolsApplyRepairsLoaded)
	assertToolsApplyPhases(t, records, 0, "inventory_common_row_1_index",
		"600", "1", strconv.Itoa(int(post.Inventory.CommonItems[1].Index)),
		characterChangeError, stageToolsApplyRepairsLoaded)
	assertToolsApplyPhases(t, records, 0, "inventory_common_header_count",
		"2", "1", "1", characterChangeError, stageToolsApplyRepairsLoaded)

	fin := operationEvent(t, records, eventToolsOperationFinished)
	if operationField(fin, "outcome") != string(characterChangeError) || operationField(fin, "stage") != stageToolsApplyRepairsLoaded {
		t.Errorf("operation finished = %q/%q, want error/%s",
			operationField(fin, "outcome"), operationField(fin, "stage"), stageToolsApplyRepairsLoaded)
	}
	assertToolsApplyGlobalOrder(t, records)
	// The stale target rolled back to no change, so its row never logs.
	assertToolsApplyFieldUnchanged(t, records, "inventory_common_row_0_handle")
	assertToolsApplyFieldUnchanged(t, records, "inventory_common_row_0_index")
}

// D. needsUserInput: pick_aow without an AoWHandle blocks before any mutation. The
// report carries needsUserInput, the slot is untouched, no field record is emitted,
// and the operation event reports error/needs_user_input — not a false success.
func TestToolsRepairApply_NeedsUserInput(t *testing.T) {
	app := NewApp()
	app.save = &core.SaveFile{}
	app.save.Slots[0] = *buildApplyInvFixture(t, []core.InventoryItem{
		{GaItemHandle: smithingStoneHandleQty, Quantity: 1, Index: 500},
	})
	enableDebugJournal(t, app)
	before := append([]byte(nil), app.save.Slots[0].Data...)

	key := core.IssueKey{Slot: 0, Domain: "aow", Handle: 0x80800021}
	target := RepairApplyTarget{
		IssueID:        core.IssueKeyID(key),
		Key:            key,
		SelectedAction: core.RepairActionPickAoW, // AoWHandle 0 -> needsUserInput
	}

	rep, err := app.ApplyRepairsLoaded(0, []RepairApplyTarget{target}, false)
	if err != nil {
		t.Fatalf("ApplyRepairsLoaded: %v", err)
	}
	if rep.NeedsUserInput != 1 || rep.Applied != 0 || rep.Failed != 0 {
		t.Fatalf("want needsUserInput=1 only, got %+v", rep)
	}
	if !bytes.Equal(before, app.save.Slots[0].Data) {
		t.Fatal("needsUserInput mutated the slot")
	}
	records := app.journal.Tail()
	if n := toolsApplyChangeCount(records); n != 0 {
		t.Fatalf("needsUserInput emitted %d field records, want 0", n)
	}
	fin := operationEvent(t, records, eventToolsOperationFinished)
	if operationField(fin, "outcome") != string(characterChangeError) || operationField(fin, "stage") != toolsStageNeedsUserInput {
		t.Errorf("operation finished = %q/%q, want error/needs_user_input",
			operationField(fin, "outcome"), operationField(fin, "stage"))
	}
	assertToolsApplyGlobalOrder(t, records)
}

// E. With Debug Mode off the real repair still runs exactly once, but no
// tools_operation_* and no tools_change_* record is emitted.
func TestToolsRepairApply_DebugOffEmitsNothing(t *testing.T) {
	app := NewApp()
	app.save = &core.SaveFile{}
	app.save.Slots[0] = *buildApplyInvFixture(t, []core.InventoryItem{
		{GaItemHandle: smithingStoneHandleQty, Quantity: 1500, Index: 500},
	})
	j, err := newDiagnosticJournalInDir(t.TempDir())
	if err != nil {
		t.Fatalf("newDiagnosticJournalInDir: %v", err)
	}
	j.SetDebugEnabled(false)
	t.Cleanup(func() { _ = j.Close() })
	app.journal = j
	slot := &app.save.Slots[0]

	rep, err := app.ApplyRepairsLoaded(0, []RepairApplyTarget{clampTarget(slot, 0)}, false)
	if err != nil {
		t.Fatalf("ApplyRepairsLoaded: %v", err)
	}
	if rep.Applied != 1 {
		t.Fatalf("repair must still run with Debug off: %+v", rep)
	}
	if slot.Inventory.CommonItems[0].Quantity != 999 {
		t.Fatalf("quantity = %d, want clamped 999", slot.Inventory.CommonItems[0].Quantity)
	}
	records := app.journal.Tail()
	if n := toolsApplyChangeCount(records); n != 0 {
		t.Fatalf("Debug-off emitted %d field records, want 0", n)
	}
	if n := toolsApplyOperationCount(records); n != 0 {
		t.Fatalf("Debug-off emitted %d operation records, want 0", n)
	}
}

// F1. no-save: the endpoint rejects with its verbatim error, emitting exactly a
// requested + finished operation pair (error/no_active_save) and no field record.
func TestToolsRepairApply_NoActiveSave(t *testing.T) {
	app := newDebugApplyApp(t)

	rep, err := app.ApplyRepairsLoaded(0, nil, false)
	if err == nil || err.Error() != "ApplyRepairsLoaded: no save loaded" {
		t.Fatalf("err = %v, want the endpoint no-save error verbatim", err)
	}
	if rep.Applied != 0 || len(rep.Results) != 0 {
		t.Fatalf("want empty report, got %+v", rep)
	}
	records := app.journal.Tail()
	if n := toolsApplyChangeCount(records); n != 0 {
		t.Fatalf("early error emitted %d field records, want 0", n)
	}
	assertEarlyApplyOperation(t, records, toolsStageNoActiveSave)
}

// F2. invalid charIdx: same contract with stage invalid_character.
func TestToolsRepairApply_InvalidCharacter(t *testing.T) {
	app := newDebugApplyApp(t)
	app.save = &core.SaveFile{}

	rep, err := app.ApplyRepairsLoaded(-1, nil, false)
	if err == nil || err.Error() != "ApplyRepairsLoaded: invalid character index -1" {
		t.Fatalf("err = %v, want the endpoint invalid-index error verbatim", err)
	}
	if len(rep.Results) != 0 {
		t.Fatalf("want empty report, got %+v", rep)
	}
	records := app.journal.Tail()
	if n := toolsApplyChangeCount(records); n != 0 {
		t.Fatalf("early error emitted %d field records, want 0", n)
	}
	assertEarlyApplyOperation(t, records, toolsStageInvalidCharacter)
}

// F3. empty slot: same contract with stage empty_slot.
func TestToolsRepairApply_EmptySlot(t *testing.T) {
	app := newDebugApplyApp(t)
	app.save = &core.SaveFile{} // slot 0 Version == 0

	_, err := app.ApplyRepairsLoaded(0, nil, false)
	if err == nil || err.Error() != "ApplyRepairsLoaded: slot 0 is empty" {
		t.Fatalf("err = %v, want the endpoint empty-slot error verbatim", err)
	}
	records := app.journal.Tail()
	if n := toolsApplyChangeCount(records); n != 0 {
		t.Fatalf("early error emitted %d field records, want 0", n)
	}
	assertEarlyApplyOperation(t, records, toolsStageEmptySlot)
}

// assertToolsApplyCleanSuccess asserts a batch that mutated nothing (empty or
// all-skipped) emitted no field record and exactly the requested → finished
// operation pair: requested carries only the action, finished only
// action/outcome=success/stage=completed, in order.
func assertToolsApplyCleanSuccess(t *testing.T, records []diagnosticRecord) {
	t.Helper()
	if n := toolsApplyChangeCount(records); n != 0 {
		t.Fatalf("clean batch emitted %d field records, want 0", n)
	}
	if got := toolsApplyOperationCount(records); got != 2 {
		t.Fatalf("want exactly requested+finished (2), got %d operation records", got)
	}
	req := operationEvent(t, records, eventToolsOperationRequested)
	if got := operationField(req, "action"); got != actionToolsApplyRepairsLoaded {
		t.Errorf("requested action=%q, want %q", got, actionToolsApplyRepairsLoaded)
	}
	if keys := fieldKeys(req); len(keys) != 1 || keys[0] != "action" {
		t.Errorf("requested fields=%v, want only [action]", keys)
	}
	fin := operationEvent(t, records, eventToolsOperationFinished)
	if got := operationField(fin, "action"); got != actionToolsApplyRepairsLoaded {
		t.Errorf("finished action=%q, want %q", got, actionToolsApplyRepairsLoaded)
	}
	if got := operationField(fin, "outcome"); got != string(characterChangeSuccess) {
		t.Errorf("finished outcome=%q, want success", got)
	}
	if got := operationField(fin, "stage"); got != toolsStageCompleted {
		t.Errorf("finished stage=%q, want %q", got, toolsStageCompleted)
	}
	assertFieldKeys(t, "finished", fin, "action", "outcome", "stage")
	assertToolsApplyGlobalOrder(t, records)
}

// fieldKeys returns the record's field keys in order.
func fieldKeys(rec diagnosticRecord) []string {
	keys := make([]string, 0, len(rec.Fields))
	for _, f := range rec.Fields {
		keys = append(keys, f.Key)
	}
	return keys
}

// assertFieldKeys fails unless rec carries exactly want, in any order.
func assertFieldKeys(t *testing.T, label string, rec diagnosticRecord, want ...string) {
	t.Helper()
	got := fieldKeys(rec)
	if len(got) != len(want) {
		t.Errorf("%s fields=%v, want %v", label, got, want)
		return
	}
	seen := map[string]bool{}
	for _, k := range got {
		seen[k] = true
	}
	for _, k := range want {
		if !seen[k] {
			t.Errorf("%s fields=%v, missing %q", label, got, k)
		}
	}
}

// G. Empty batch (Debug on): a valid non-empty slot with no targets mutates
// nothing. The slot stays byte-identical, no tools_change_* record is emitted,
// and only the requested → finished(success/completed) operation pair remains.
func TestToolsRepairApply_EmptyBatchLifecycle(t *testing.T) {
	app := NewApp()
	app.save = &core.SaveFile{}
	app.save.Slots[0] = *buildApplyInvFixture(t, []core.InventoryItem{
		{GaItemHandle: smithingStoneHandleQty, Quantity: 1, Index: 500},
	})
	enableDebugJournal(t, app)
	before := append([]byte(nil), app.save.Slots[0].Data...)

	rep, err := app.ApplyRepairsLoaded(0, nil, false)
	if err != nil {
		t.Fatalf("ApplyRepairsLoaded: %v", err)
	}
	if rep.Applied != 0 || rep.Skipped != 0 || rep.Failed != 0 || rep.NeedsUserInput != 0 || len(rep.Results) != 0 {
		t.Fatalf("want empty report, got %+v", rep)
	}
	if !bytes.Equal(before, app.save.Slots[0].Data) {
		t.Fatal("empty batch mutated the slot")
	}
	assertToolsApplyCleanSuccess(t, app.journal.Tail())
}

// H. All-skipped batch (Debug on): a single no_action target reports skipped
// without touching the slot. Same journal contract as the empty batch — zero
// field records, exactly requested → finished(success/completed).
func TestToolsRepairApply_SkippedOnlyLifecycle(t *testing.T) {
	app := NewApp()
	app.save = &core.SaveFile{}
	app.save.Slots[0] = *buildApplyInvFixture(t, []core.InventoryItem{
		{GaItemHandle: smithingStoneHandleQty, Quantity: 1, Index: 500},
	})
	enableDebugJournal(t, app)
	before := append([]byte(nil), app.save.Slots[0].Data...)

	target := RepairApplyTarget{SelectedAction: core.RepairActionNoAction}
	rep, err := app.ApplyRepairsLoaded(0, []RepairApplyTarget{target}, false)
	if err != nil {
		t.Fatalf("ApplyRepairsLoaded: %v", err)
	}
	if rep.Skipped != 1 || rep.Applied != 0 || rep.Failed != 0 || rep.NeedsUserInput != 0 {
		t.Fatalf("want skipped=1 only, got %+v", rep)
	}
	if !bytes.Equal(before, app.save.Slots[0].Data) {
		t.Fatal("skipped batch mutated the slot")
	}
	assertToolsApplyCleanSuccess(t, app.journal.Tail())
}

// assertEarlyApplyOperation asserts an early-exit path emitted exactly the
// requested + finished operation pair with error and the given stage, and nothing
// else, in order.
func assertEarlyApplyOperation(t *testing.T, records []diagnosticRecord, stage string) {
	t.Helper()
	if got := toolsApplyOperationCount(records); got != 2 {
		t.Fatalf("want exactly requested+finished (2), got %d operation records", got)
	}
	req := operationEvent(t, records, eventToolsOperationRequested)
	if got := operationField(req, "action"); got != actionToolsApplyRepairsLoaded {
		t.Errorf("requested action=%q, want %q", got, actionToolsApplyRepairsLoaded)
	}
	fin := operationEvent(t, records, eventToolsOperationFinished)
	if got := operationField(fin, "outcome"); got != string(characterChangeError) {
		t.Errorf("finished outcome=%q, want error", got)
	}
	if got := operationField(fin, "stage"); got != stage {
		t.Errorf("finished stage=%q, want %q", got, stage)
	}
	assertToolsApplyGlobalOrder(t, records)
}
