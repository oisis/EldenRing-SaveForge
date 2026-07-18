package main

import (
	"strconv"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
	"github.com/oisis/EldenRing-SaveForge/backend/db/data"
)

type worldLifecycle struct {
	before   map[string]string
	planned  map[string]string
	finished map[string]string
	beforeIx []int
	planIx   []int
	finIx    []int
	outcome  string
	stage    string
}

func worldFlagsApp(debug bool) *App {
	app := gameItemAddApp(debug)
	withUnlockEventFlags(app, 0x200000)
	return app
}

func collectWorldLifecycle(t *testing.T, records []diagnosticRecord, action string) worldLifecycle {
	t.Helper()
	lc := worldLifecycle{
		before:   map[string]string{},
		planned:  map[string]string{},
		finished: map[string]string{},
	}
	for i, rec := range records {
		switch rec.Event {
		case eventWorldChangeBefore, eventWorldChangePlanned, eventWorldChangeFinished:
		default:
			continue
		}
		if got := operationField(rec, "action"); got != action {
			t.Errorf("%s action = %q, want %q", rec.Event, got, action)
		}
		if got := operationField(rec, "character_index"); got != "0" {
			t.Errorf("%s character_index = %q, want 0", rec.Event, got)
		}
		field := operationField(rec, "field")
		switch rec.Event {
		case eventWorldChangeBefore:
			if got := operationField(rec, "after"); got != "" {
				t.Errorf("before %s carries after=%q", field, got)
			}
			if got := operationField(rec, "outcome"); got != "" || operationField(rec, "stage") != "" {
				t.Errorf("before %s carries terminal fields", field)
			}
			lc.before[field] = operationField(rec, "before")
			lc.beforeIx = append(lc.beforeIx, i)
		case eventWorldChangePlanned:
			lc.planned[field] = operationField(rec, "after")
			lc.planIx = append(lc.planIx, i)
		case eventWorldChangeFinished:
			lc.finished[field] = operationField(rec, "after")
			lc.finIx = append(lc.finIx, i)
			lc.outcome = operationField(rec, "outcome")
			lc.stage = operationField(rec, "stage")
		}
	}
	return lc
}

func assertWorldSuccess(t *testing.T, lc worldLifecycle) {
	t.Helper()
	if len(lc.beforeIx) == 0 || len(lc.planIx) == 0 || len(lc.finIx) == 0 {
		t.Fatalf("missing world lifecycle phase: before=%d planned=%d finished=%d", len(lc.beforeIx), len(lc.planIx), len(lc.finIx))
	}
	if maxInt(lc.beforeIx) >= minInt(lc.planIx) || maxInt(lc.planIx) >= minInt(lc.finIx) {
		t.Errorf("world lifecycle phase grouping is not before -> planned -> finished")
	}
	if lc.outcome != string(characterChangeSuccess) || lc.stage != characterStageCompleted {
		t.Errorf("terminal = %s/%s, want success/%s", lc.outcome, lc.stage, characterStageCompleted)
	}
}

func assertWorldField(t *testing.T, lc worldLifecycle, field, before, after string) {
	t.Helper()
	if got := lc.before[field]; got != before {
		t.Errorf("before %s = %q, want %q", field, got, before)
	}
	if got := lc.planned[field]; got != after {
		t.Errorf("planned %s = %q, want %q", field, got, after)
	}
	if got := lc.finished[field]; got != after {
		t.Errorf("finished %s = %q, want %q", field, got, after)
	}
}

func TestWorldGraceDiagnosticLogsAllOwnedFlags(t *testing.T) {
	app := worldFlagsApp(true)
	const graceID = data.GatefrontGraceEventFlagID

	if err := app.SetGraceVisited(0, graceID, true); err != nil {
		t.Fatalf("SetGraceVisited: %v", err)
	}
	lc := collectWorldLifecycle(t, app.journal.Tail(), actionWorldSetGraceVisited)
	assertWorldSuccess(t, lc)
	for _, flagID := range worldGraceFlagIDs(graceID, true) {
		assertWorldField(t, lc, "event_flag_"+strconv.FormatUint(uint64(flagID), 10), "false", "true")
	}
}

func TestWorldBasicFlagDiagnostics(t *testing.T) {
	tests := []struct {
		name   string
		action string
		flagID uint32
		apply  func(*App) error
	}{
		{"boss", actionWorldSetBossDefeated, 10000800, func(app *App) error { return app.SetBossDefeated(0, 10000800, true) }},
		{"summoning_pool", actionWorldSetSummoningPoolActivated, 76100, func(app *App) error { return app.SetSummoningPoolActivated(0, 76100, true) }},
		{"map_flag", actionWorldSetMapFlag, 62010, func(app *App) error { return app.SetMapFlag(0, 62010, true) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := worldFlagsApp(true)
			if err := tt.apply(app); err != nil {
				t.Fatalf("apply: %v", err)
			}
			lc := collectWorldLifecycle(t, app.journal.Tail(), tt.action)
			assertWorldSuccess(t, lc)
			assertWorldField(t, lc, "event_flag_"+strconv.FormatUint(uint64(tt.flagID), 10), "false", "true")
		})
	}
}

func TestWorldColosseumDiagnosticLogsDerivativesAndGlobals(t *testing.T) {
	app := worldFlagsApp(true)
	const colosseumID = uint32(60360)
	if err := app.SetColosseumUnlocked(0, colosseumID, true); err != nil {
		t.Fatalf("SetColosseumUnlocked: %v", err)
	}
	lc := collectWorldLifecycle(t, app.journal.Tail(), actionWorldSetColosseumUnlocked)
	assertWorldSuccess(t, lc)
	for _, flagID := range worldColosseumFlagIDs(colosseumID, true) {
		assertWorldField(t, lc, "event_flag_"+strconv.FormatUint(uint64(flagID), 10), "false", "true")
	}
}

func enableWorldGestureData(slot *core.SaveSlot) {
	end := slot.StorageBoxOffset + core.DynStorageBox + core.DynStorageToGestures
	if len(slot.Data) < end {
		slot.Data = append(slot.Data, make([]byte, end-len(slot.Data))...)
	}
	slots := make([]uint32, 64)
	for i := range slots {
		slots[i] = data.GestureEmptySentinel
	}
	writeGestureSlots(slot.Data, slot.StorageBoxOffset+core.DynStorageBox, slots)
}

func TestWorldBulkGesturesDiagnosticLogsEveryChangedSlot(t *testing.T) {
	app := worldFlagsApp(true)
	enableWorldGestureData(&app.save.Slots[0])

	if err := app.BulkSetGesturesUnlocked(0, []uint32{1, 3}, true); err != nil {
		t.Fatalf("BulkSetGesturesUnlocked: %v", err)
	}
	lc := collectWorldLifecycle(t, app.journal.Tail(), actionWorldBulkSetGesturesUnlocked)
	assertWorldSuccess(t, lc)
	assertWorldField(t, lc, "gesture_slot_0_id", strconv.FormatUint(uint64(data.GestureEmptySentinel), 10), "1")
	assertWorldField(t, lc, "gesture_slot_1_id", strconv.FormatUint(uint64(data.GestureEmptySentinel), 10), "3")
	if len(lc.before) != 2 || len(lc.planned) != 2 || len(lc.finished) != 2 {
		t.Fatalf("gesture record fields = %d/%d/%d, want exactly 2", len(lc.before), len(lc.planned), len(lc.finished))
	}
}

func TestWorldGestureDiagnosticLogsSingleSlot(t *testing.T) {
	app := worldFlagsApp(true)
	enableWorldGestureData(&app.save.Slots[0])

	if err := app.SetGestureUnlocked(0, 1, true); err != nil {
		t.Fatalf("SetGestureUnlocked: %v", err)
	}
	lc := collectWorldLifecycle(t, app.journal.Tail(), actionWorldSetGestureUnlocked)
	assertWorldSuccess(t, lc)
	assertWorldField(t, lc, "gesture_slot_0_id", strconv.FormatUint(uint64(data.GestureEmptySentinel), 10), "1")
	if len(lc.before) != 1 || len(lc.planned) != 1 || len(lc.finished) != 1 {
		t.Fatalf("single gesture record fields = %d/%d/%d, want exactly 1", len(lc.before), len(lc.planned), len(lc.finished))
	}
}

func TestWorldFlagDiagnosticsNoopAndDebugOff(t *testing.T) {
	app := worldFlagsApp(true)
	if err := app.SetMapFlag(0, 62010, true); err != nil {
		t.Fatalf("seed SetMapFlag: %v", err)
	}
	app.journal = newInMemoryDiagnosticJournal()
	app.journal.SetDebugEnabled(true)
	if err := app.SetMapFlag(0, 62010, true); err != nil {
		t.Fatalf("noop SetMapFlag: %v", err)
	}
	if got := len(app.journal.Tail()); got != 0 {
		t.Fatalf("noop records = %d, want 0", got)
	}

	app = worldFlagsApp(false)
	if err := app.SetMapFlag(0, 62010, true); err != nil {
		t.Fatalf("debug-off SetMapFlag: %v", err)
	}
	if got := len(app.journal.Tail()); got != 0 {
		t.Fatalf("debug-off records = %d, want 0", got)
	}
}
