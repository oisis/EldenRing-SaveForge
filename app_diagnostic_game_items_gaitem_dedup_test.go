package main

import (
	"fmt"
	"reflect"
	"strconv"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
)

// gaItemDedupPhases returns the before/planned/finished game_items_change_*
// records for one logical field, in lifecycle order.
func gaItemDedupPhases(records []diagnosticRecord, field string) []diagnosticRecord {
	var out []diagnosticRecord
	for _, rec := range gameItemRecords(records) {
		if operationField(rec, "field") == field {
			out = append(out, rec)
		}
	}
	return out
}

// assertDedupPhases asserts one field logs exactly before/planned/finished under
// the dedup action, with the given values, a clean before phase (no
// after/outcome/stage), and a success/completed finished phase read from the
// real slot for character index 0.
func assertDedupPhases(t *testing.T, records []diagnosticRecord, field, before, planned, final string) {
	t.Helper()
	phases := gaItemDedupPhases(records, field)
	if len(phases) != 3 {
		t.Fatalf("field %q: got %d records, want 3", field, len(phases))
	}
	wantEvents := []string{eventGameItemsChangeBefore, eventGameItemsChangePlanned, eventGameItemsChangeFinished}
	for i, rec := range phases {
		if rec.Event != wantEvents[i] {
			t.Errorf("field %q phase %d: event=%q want %q", field, i, rec.Event, wantEvents[i])
		}
		if got := operationField(rec, "before"); got != before {
			t.Errorf("field %q phase %d: before=%q want %q", field, i, got, before)
		}
		if got := operationField(rec, "action"); got != actionGameItemsGaItemDeduplicate {
			t.Errorf("field %q phase %d: action=%q want %q", field, i, got, actionGameItemsGaItemDeduplicate)
		}
		if got := operationField(rec, "character_index"); got != "0" {
			t.Errorf("field %q phase %d: character_index=%q want 0", field, i, got)
		}
	}
	// before phase carries only the before value.
	if got := operationField(phases[0], "after"); got != "" {
		t.Errorf("field %q before leaked after=%q", field, got)
	}
	if got := operationField(phases[0], "outcome"); got != "" {
		t.Errorf("field %q before leaked outcome=%q", field, got)
	}
	if got := operationField(phases[0], "stage"); got != "" {
		t.Errorf("field %q before leaked stage=%q", field, got)
	}
	if got := operationField(phases[1], "after"); got != planned {
		t.Errorf("field %q planned after=%q want %q", field, got, planned)
	}
	fin := phases[2]
	if got := operationField(fin, "after"); got != final {
		t.Errorf("field %q finished after=%q want %q (real slot)", field, got, final)
	}
	if got := operationField(fin, "outcome"); got != string(characterChangeSuccess) {
		t.Errorf("field %q finished outcome=%q want success", field, got)
	}
	if got := operationField(fin, "stage"); got != characterStageCompleted {
		t.Errorf("field %q finished stage=%q want completed", field, got)
	}
}

// runReadyDedup runs Analyze + Execute for the always-executable dedup fixture,
// asserting the success outcome, and returns the pre-repair snapshot, the removed
// index and the kept item ID for lifecycle assertions.
func runReadyDedup(t *testing.T, app *App, charIdx int, handle uint32, keepIndex int) (before *core.SaveSlot, removedIndex int, keptItemID uint32) {
	t.Helper()
	before = core.CloneSlot(&app.save.Slots[charIdx])

	analysisCore := core.AnalyzeGaItemDuplicate(before, handle)
	removedIndex = analysisCore.Candidates[1].Index
	if keepIndex != analysisCore.Candidates[0].Index {
		removedIndex = analysisCore.Candidates[0].Index
	}
	keptItemID = before.GaItems[keepIndex].ItemID

	analysis, err := app.AnalyzeGaItemDuplicate(charIdx, handle)
	if err != nil {
		t.Fatalf("AnalyzeGaItemDuplicate: %v", err)
	}
	if analysis.Outcome != "ready" || analysis.AnalysisToken == "" {
		t.Fatalf("analysis=%+v, want ready with token", analysis)
	}

	result, err := app.ExecuteGaItemDuplicateRepair(GaItemDuplicateExecuteRequest{
		CharacterIndex: charIdx, Handle: handle, KeepIndex: keepIndex, AnalysisToken: analysis.AnalysisToken,
	})
	if err != nil {
		t.Fatalf("ExecuteGaItemDuplicateRepair: %v", err)
	}
	if result.Outcome != "success" {
		t.Fatalf("result=%+v, want success", result)
	}
	return before, removedIndex, keptItemID
}

// TestGaItemDedupDiagnosticSuccessLifecycle asserts the full before/planned/
// finished lifecycle for a successful dedup: the removed physical row, the GaMap
// value change the fixture generates, and every changed allocation cursor, with
// unchanged cursors self-excluded and phase grouping preserved.
func TestGaItemDedupDiagnosticSuccessLifecycle(t *testing.T) {
	app, charIdx, handle, keepIndex := readyDedupApp(t)
	enableDebugJournal(t, app)

	before, removedIndex, keptItemID := runReadyDedup(t, app, charIdx, handle, keepIndex)
	post := &app.save.Slots[charIdx]
	records := gameItemRecords(app.journal.Tail())

	// A. the removed direct physical row's full lifecycle.
	afterHandle := giHex(post.GaItems[removedIndex].Handle)
	assertDedupPhases(t,
		records,
		fmt.Sprintf("gaitem_row_%d_handle", removedIndex),
		giHex(before.GaItems[removedIndex].Handle),
		afterHandle, afterHandle,
	)
	assertDedupPhases(t,
		records,
		fmt.Sprintf("gaitem_row_%d_item_id", removedIndex),
		giHex(before.GaItems[removedIndex].ItemID),
		giAbsent, giAbsent,
	)

	// B. the GaMap handle whose mapped item_id flips to the kept record.
	mapField := fmt.Sprintf("gaitem_map_handle_0x%08X_item_id", handle)
	if before.GaMap[handle] != keptItemID {
		assertDedupPhases(t, records, mapField, giHex(before.GaMap[handle]), giHex(keptItemID), giHex(post.GaMap[handle]))
	} else {
		t.Fatal("fixture did not generate a GaMap change; expected kept != prior mapping")
	}

	// C. every changed cursor logs; unchanged cursors self-exclude.
	cursors := []struct{ field, before, after string }{
		{"gaitem_next_aow_index", strconv.Itoa(before.NextAoWIndex), strconv.Itoa(post.NextAoWIndex)},
		{"gaitem_next_armament_index", strconv.Itoa(before.NextArmamentIndex), strconv.Itoa(post.NextArmamentIndex)},
		{"gaitem_next_handle", giHex(before.NextGaItemHandle), giHex(post.NextGaItemHandle)},
		{"gaitem_part_handle", giHex(uint32(before.PartGaItemHandle)), giHex(uint32(post.PartGaItemHandle))},
	}
	for _, c := range cursors {
		if c.before == c.after {
			if n := len(gaItemDedupPhases(records, c.field)); n != 0 {
				t.Errorf("unchanged cursor %q emitted %d records", c.field, n)
			}
			continue
		}
		assertDedupPhases(t, records, c.field, c.before, c.after, c.after)
	}

	// D. phase grouping: all-before, then all-planned, then all-finished.
	order := map[string]int{
		eventGameItemsChangeBefore:   0,
		eventGameItemsChangePlanned:  1,
		eventGameItemsChangeFinished: 2,
	}
	last := -1
	for _, rec := range records {
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

// TestPlanGameItemsGaMapRecords_AddRemoveAndReader exercises the planner in
// isolation: a removed entry (0x%08X -> absent), a changed value, and an added
// entry (absent -> 0x%08X) must all log, ordered by ascending handle, and the
// finished reader must resolve a missing handle as absent and a present one as
// its uppercase item_id.
func TestPlanGameItemsGaMapRecords_AddRemoveAndReader(t *testing.T) {
	const (
		removed = uint32(0x00000010)
		changed = uint32(0x00000020)
		added   = uint32(0x00000030)
	)
	before := &core.SaveSlot{GaMap: map[uint32]uint32{
		removed: 0x000000AA,
		changed: 0x000000BB,
	}}
	planned := &core.SaveSlot{GaMap: map[uint32]uint32{
		changed: 0x000000CC,
		added:   0x000000DD,
	}}

	plans := planGameItemsGaMapRecords(before, planned)
	want := []struct{ field, before, planned string }{
		{"gaitem_map_handle_0x00000010_item_id", giHex(0x000000AA), giAbsent},
		{"gaitem_map_handle_0x00000020_item_id", giHex(0x000000BB), giHex(0x000000CC)},
		{"gaitem_map_handle_0x00000030_item_id", giAbsent, giHex(0x000000DD)},
	}
	if len(plans) != len(want) {
		t.Fatalf("got %d plans, want %d", len(plans), len(want))
	}
	for i, w := range want {
		if plans[i].field != w.field {
			t.Errorf("plan %d field=%q want %q", i, plans[i].field, w.field)
		}
		if plans[i].before != w.before {
			t.Errorf("plan %d before=%q want %q", i, plans[i].before, w.before)
		}
		if plans[i].planned != w.planned {
			t.Errorf("plan %d planned=%q want %q", i, plans[i].planned, w.planned)
		}
	}

	// Finished reader against a real slot missing the removed handle and holding
	// the added one.
	realSlot := &core.SaveSlot{GaMap: map[uint32]uint32{
		changed: 0x000000CC,
		added:   0x000000DD,
	}}
	if got := plans[0].read(realSlot); got != giAbsent {
		t.Errorf("removed handle reader=%q want absent", got)
	}
	if got := plans[2].read(realSlot); got != giHex(0x000000DD) {
		t.Errorf("added handle reader=%q want %q", got, giHex(0x000000DD))
	}
}

// TestGaItemDedupDiagnosticDebugOffEmitsNothing confirms the repair still runs
// with Debug Mode off and emits zero lifecycle records.
func TestGaItemDedupDiagnosticDebugOffEmitsNothing(t *testing.T) {
	app, charIdx, handle, keepIndex := readyDedupApp(t)
	j, err := newDiagnosticJournalInDir(t.TempDir())
	if err != nil {
		t.Fatalf("newDiagnosticJournalInDir: %v", err)
	}
	j.SetDebugEnabled(false)
	t.Cleanup(func() { _ = j.Close() })
	app.journal = j

	runReadyDedup(t, app, charIdx, handle, keepIndex)

	if recs := gameItemRecords(app.journal.Tail()); len(recs) != 0 {
		t.Fatalf("debug off emitted %d records, want 0", len(recs))
	}
	// The repair still happened: exactly one physical record remains.
	slot := &app.save.Slots[charIdx]
	remaining := 0
	for i := range slot.GaItems {
		if !slot.GaItems[i].IsEmpty() && slot.GaItems[i].Handle == handle {
			remaining++
		}
	}
	if remaining != 1 {
		t.Fatalf("remaining records for handle = %d, want 1", remaining)
	}
}

// TestGaItemDedupDiagnosticStaleEmitsNothing confirms a stale (refused) execution
// emits zero lifecycle records and leaves the real slot untouched.
func TestGaItemDedupDiagnosticStaleEmitsNothing(t *testing.T) {
	app, charIdx, handle, keepIndex := readyDedupApp(t)
	enableDebugJournal(t, app)

	analysis, err := app.AnalyzeGaItemDuplicate(charIdx, handle)
	if err != nil {
		t.Fatalf("AnalyzeGaItemDuplicate: %v", err)
	}
	app.save.Slots[charIdx].Data[0] ^= 0x01 // invalidate the token snapshot
	changed := core.CloneSlot(&app.save.Slots[charIdx])

	result, err := app.ExecuteGaItemDuplicateRepair(GaItemDuplicateExecuteRequest{
		CharacterIndex: charIdx, Handle: handle, KeepIndex: keepIndex, AnalysisToken: analysis.AnalysisToken,
	})
	if err != nil {
		t.Fatalf("ExecuteGaItemDuplicateRepair: %v", err)
	}
	if result.Outcome != "could_not_start" || result.Failure == nil || result.Failure.Code != "analysis_stale" {
		t.Fatalf("result=%+v, want stale no-start", result)
	}
	if recs := gameItemRecords(app.journal.Tail()); len(recs) != 0 {
		t.Fatalf("stale execution emitted %d records, want 0", len(recs))
	}
	if !reflect.DeepEqual(&app.save.Slots[charIdx], changed) {
		t.Fatal("stale execution changed the active slot")
	}
}
