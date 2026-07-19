package main

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"
)

// toolsApp builds a journal-only App: the Tools planner and phase helpers touch
// only a.journal, so no save fixture (and no tmp/save) is required.
func toolsApp(debug bool) *App {
	app := NewApp()
	journal := newInMemoryDiagnosticJournal()
	journal.SetDebugEnabled(debug)
	app.journal = journal
	return app
}

// emitToolsSteamIDLifecycle drives the three phase helpers exactly as the future
// SetSteamIDFromString wiring will: plan once from the raw ids, emit before(all),
// planned(all), finished(all). before/planned come from the plan; finished re-reads
// from the actual post-write id.
func emitToolsSteamIDLifecycle(app *App, before, planned, actual uint64, outcome characterChangeOutcome, stage string) []toolsSteamIDPlan {
	plans := planToolsSteamIDChange(before, planned)
	records := toolsSteamIDPlannedRecords(plans)
	app.journalToolsChangeBefore(actionToolsSetSteamID, records)
	app.journalToolsChangePlanned(actionToolsSetSteamID, records)
	app.journalToolsChangeFinished(actionToolsSetSteamID, outcome, stage, toolsSteamIDFinishedRecords(plans, actual))
	return plans
}

// The two raw Steam IDs used across the leak-scan tests. They share no long common
// substring with each other or with any journal noise (event names, "absent",
// "[redacted]", "-1", the action/stage/field labels carry no 5-digit runs), so a
// fragment scan of length >= 5 cannot false-positive.
const (
	rawSteamIDOne = uint64(76561197960287930)
	rawSteamIDTwo = uint64(76561198088776655)
)

type toolsLifecycle struct {
	beforeIx []int
	planIx   []int
	finIx    []int
	before   string
	planned  string
	finished string
	outcome  string
	stage    string
}

func collectToolsLifecycle(t *testing.T, records []diagnosticRecord) toolsLifecycle {
	t.Helper()
	var lc toolsLifecycle
	for i, rec := range records {
		switch rec.Event {
		case eventToolsChangeBefore, eventToolsChangePlanned, eventToolsChangeFinished:
		default:
			continue
		}
		if got := operationField(rec, "action"); got != actionToolsSetSteamID {
			t.Errorf("%s action = %q, want %q", rec.Event, got, actionToolsSetSteamID)
		}
		if got := operationField(rec, "character_index"); got != "-1" {
			t.Errorf("%s character_index = %q, want -1", rec.Event, got)
		}
		if got := operationField(rec, "field"); got != toolsFieldSteamID {
			t.Errorf("%s field = %q, want %q", rec.Event, got, toolsFieldSteamID)
		}
		switch rec.Event {
		case eventToolsChangeBefore:
			// Phase contract: before carries neither after nor terminal fields.
			if got := operationField(rec, "after"); got != "" {
				t.Errorf("before carries after=%q", got)
			}
			if operationField(rec, "outcome") != "" || operationField(rec, "stage") != "" {
				t.Errorf("before carries terminal fields")
			}
			lc.before = operationField(rec, "before")
			lc.beforeIx = append(lc.beforeIx, i)
		case eventToolsChangePlanned:
			// Phase contract: planned carries after but no terminal fields.
			if operationField(rec, "outcome") != "" || operationField(rec, "stage") != "" {
				t.Errorf("planned carries terminal fields")
			}
			lc.planned = operationField(rec, "after")
			lc.planIx = append(lc.planIx, i)
		case eventToolsChangeFinished:
			lc.finished = operationField(rec, "after")
			lc.outcome = operationField(rec, "outcome")
			lc.stage = operationField(rec, "stage")
			lc.finIx = append(lc.finIx, i)
		}
	}
	return lc
}

// assertNoRawSteamID marshals the whole journal tail to JSON (its on-disk JSONL
// form) and asserts that neither raw id, nor any contiguous fragment of length >= 5
// of either, appears anywhere in the records or their fields.
func assertNoRawSteamID(t *testing.T, records []diagnosticRecord, ids ...uint64) {
	t.Helper()
	blob, err := json.Marshal(records)
	if err != nil {
		t.Fatalf("marshal records: %v", err)
	}
	assertNoRawSteamIDInString(t, string(blob), ids...)
}

// assertNoRawSteamIDInString asserts that neither raw id, nor any contiguous
// fragment of length >= 5 of either, appears anywhere in haystack. Shared by the
// in-memory (marshaled Tail) and file-backed (raw JSONL) leak scans.
func assertNoRawSteamIDInString(t *testing.T, haystack string, ids ...uint64) {
	t.Helper()
	for _, id := range ids {
		s := strconv.FormatUint(id, 10)
		for start := 0; start+5 <= len(s); start++ {
			for end := start + 5; end <= len(s); end++ {
				frag := s[start:end]
				if strings.Contains(haystack, frag) {
					t.Errorf("raw Steam ID fragment %q leaked into journal", frag)
				}
			}
		}
	}
}

func TestToolsDiagnosticSteamIDFullLifecycle(t *testing.T) {
	app := toolsApp(true)
	// absent -> [redacted] -> [redacted]: unset id set to a real non-zero id.
	emitToolsSteamIDLifecycle(app, 0, rawSteamIDOne, rawSteamIDOne, characterChangeSuccess, characterStageCompleted)

	tail := app.journal.Tail()
	lc := collectToolsLifecycle(t, tail)
	if len(lc.beforeIx) != 1 || len(lc.planIx) != 1 || len(lc.finIx) != 1 {
		t.Fatalf("phase counts = %d/%d/%d, want 1/1/1", len(lc.beforeIx), len(lc.planIx), len(lc.finIx))
	}
	// Strict grouping before(all) -> planned(all) -> finished(all).
	if maxInt(lc.beforeIx) >= minInt(lc.planIx) || maxInt(lc.planIx) >= minInt(lc.finIx) {
		t.Errorf("phase grouping is not before -> planned -> finished")
	}
	if lc.before != steamIDAbsent {
		t.Errorf("before = %q, want %q", lc.before, steamIDAbsent)
	}
	if lc.planned != steamIDRedacted || lc.finished != steamIDRedacted {
		t.Errorf("planned/finished = %q/%q, want %q/%q", lc.planned, lc.finished, steamIDRedacted, steamIDRedacted)
	}
	if lc.outcome != string(characterChangeSuccess) || lc.stage != characterStageCompleted {
		t.Errorf("terminal = %s/%s, want success/%s", lc.outcome, lc.stage, characterStageCompleted)
	}
	assertNoRawSteamID(t, tail, rawSteamIDOne)
}

func TestToolsDiagnosticSteamIDReplacement(t *testing.T) {
	app := toolsApp(true)
	// non-zero -> non-zero: both ends redact to [redacted], but the lifecycle must
	// still exist because the raw ids differ.
	emitToolsSteamIDLifecycle(app, rawSteamIDOne, rawSteamIDTwo, rawSteamIDTwo, characterChangeSuccess, characterStageCompleted)

	tail := app.journal.Tail()
	lc := collectToolsLifecycle(t, tail)
	if len(lc.beforeIx) != 1 || len(lc.planIx) != 1 || len(lc.finIx) != 1 {
		t.Fatalf("phase counts = %d/%d/%d, want 1/1/1", len(lc.beforeIx), len(lc.planIx), len(lc.finIx))
	}
	if lc.before != steamIDRedacted || lc.planned != steamIDRedacted || lc.finished != steamIDRedacted {
		t.Errorf("before/planned/finished = %q/%q/%q, want all %q", lc.before, lc.planned, lc.finished, steamIDRedacted)
	}
	// Neither the old nor the new raw id (nor any fragment) may appear.
	assertNoRawSteamID(t, tail, rawSteamIDOne, rawSteamIDTwo)
}

func TestToolsDiagnosticSteamIDNoop(t *testing.T) {
	app := toolsApp(true)
	emitToolsSteamIDLifecycle(app, rawSteamIDOne, rawSteamIDOne, rawSteamIDOne, characterChangeSuccess, characterStageCompleted)
	if got := len(app.journal.Tail()); got != 0 {
		t.Fatalf("noop records = %d, want 0", got)
	}
}

func TestToolsDiagnosticSteamIDDebugOff(t *testing.T) {
	app := toolsApp(false)
	emitToolsSteamIDLifecycle(app, 0, rawSteamIDOne, rawSteamIDOne, characterChangeSuccess, characterStageCompleted)
	if got := len(app.journal.Tail()); got != 0 {
		t.Fatalf("debug-off records = %d, want 0", got)
	}
}

func TestToolsDiagnosticSteamIDFinishedError(t *testing.T) {
	app := toolsApp(true)
	// A failed write leaves the id unchanged at 0; finished reports the real
	// post-error state (absent) with the error outcome and set_steam_id stage,
	// distinguishing an untouched absent from a landed [redacted].
	emitToolsSteamIDLifecycle(app, 0, rawSteamIDOne, 0, characterChangeError, stageToolsSetSteamID)

	tail := app.journal.Tail()
	lc := collectToolsLifecycle(t, tail)
	if lc.outcome != string(characterChangeError) || lc.stage != stageToolsSetSteamID {
		t.Errorf("terminal = %s/%s, want error/%s", lc.outcome, lc.stage, stageToolsSetSteamID)
	}
	if lc.planned != steamIDRedacted {
		t.Errorf("planned = %q, want %q", lc.planned, steamIDRedacted)
	}
	if lc.finished != steamIDAbsent {
		t.Errorf("finished = %q, want %q (rolled back, absent != [redacted])", lc.finished, steamIDAbsent)
	}
	assertNoRawSteamID(t, tail, rawSteamIDOne)
}

// TestToolsDiagnosticSteamIDNoRawInRawJSONL exercises the whole path through a real
// file-backed journal (not just the in-memory Tail): a non-zero -> non-zero
// replacement is written to an actual .jsonl session file, and neither raw id nor
// any contiguous fragment of length >= 5 may appear in the bytes on disk.
func TestToolsDiagnosticSteamIDNoRawInRawJSONL(t *testing.T) {
	dir := t.TempDir()
	journal, err := newDiagnosticJournalInDir(dir)
	if err != nil {
		t.Fatalf("newDiagnosticJournalInDir: %v", err)
	}
	journal.SetDebugEnabled(true)
	app := NewApp()
	app.journal = journal

	emitToolsSteamIDLifecycle(app, rawSteamIDOne, rawSteamIDTwo, rawSteamIDTwo, characterChangeSuccess, characterStageCompleted)

	// Same in-memory Tail contract as the pure-redaction test.
	tail := app.journal.Tail()
	lc := collectToolsLifecycle(t, tail)
	if len(lc.beforeIx) != 1 || len(lc.planIx) != 1 || len(lc.finIx) != 1 {
		t.Fatalf("phase counts = %d/%d/%d, want 1/1/1", len(lc.beforeIx), len(lc.planIx), len(lc.finIx))
	}
	if lc.before != steamIDRedacted || lc.planned != steamIDRedacted || lc.finished != steamIDRedacted {
		t.Errorf("before/planned/finished = %q/%q/%q, want all %q", lc.before, lc.planned, lc.finished, steamIDRedacted)
	}
	assertNoRawSteamID(t, tail, rawSteamIDOne, rawSteamIDTwo)

	// And the authoritative check: the raw bytes actually persisted to disk.
	assertNoRawSteamIDInString(t, readSessionRaw(t, dir), rawSteamIDOne, rawSteamIDTwo)
}
