package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
)

// steamIDApp builds an App wired to a file-backed journal (so tests can also scan
// the raw JSONL on disk) with a save whose SteamID is preset. save == nil models
// "no active save". No fixture and no tmp/save is touched: the save is an in-memory
// core.SaveFile carrying only the one field the endpoint reads and writes.
func steamIDApp(t *testing.T, debug bool, save *core.SaveFile) *App {
	t.Helper()
	journal, err := newDiagnosticJournalInDir(t.TempDir())
	if err != nil {
		t.Fatalf("newDiagnosticJournalInDir: %v", err)
	}
	journal.SetDebugEnabled(debug)
	t.Cleanup(func() { _ = journal.Close() })
	app := NewApp()
	app.journal = journal
	app.save = save
	return app
}

func eventIndex(t *testing.T, records []diagnosticRecord, event string) int {
	t.Helper()
	for i, rec := range records {
		if rec.Event == event {
			return i
		}
	}
	t.Fatalf("missing event %q", event)
	return -1
}

func countEvent(records []diagnosticRecord, event string) int {
	n := 0
	for _, rec := range records {
		if rec.Event == event {
			n++
		}
	}
	return n
}

// assertOperationFinished checks the single tools_operation_finished record carries
// exactly action/outcome/stage and nothing that could leak a value.
func assertOperationFinished(t *testing.T, records []diagnosticRecord, outcome, stage string) {
	t.Helper()
	fin := operationEvent(t, records, eventToolsOperationFinished)
	if got := operationField(fin, "action"); got != actionToolsSetSteamID {
		t.Errorf("finished action = %q, want %q", got, actionToolsSetSteamID)
	}
	if got := operationField(fin, "outcome"); got != outcome {
		t.Errorf("finished outcome = %q, want %q", got, outcome)
	}
	if got := operationField(fin, "stage"); got != stage {
		t.Errorf("finished stage = %q, want %q", got, stage)
	}
	for _, forbidden := range []string{"before", "after", "field"} {
		if got := operationField(fin, forbidden); got != "" {
			t.Errorf("finished carries %s=%q", forbidden, got)
		}
	}
}

// assertOperationRequested checks the requested record carries only the action.
func assertOperationRequested(t *testing.T, records []diagnosticRecord) {
	t.Helper()
	req := operationEvent(t, records, eventToolsOperationRequested)
	if got := operationField(req, "action"); got != actionToolsSetSteamID {
		t.Errorf("requested action = %q, want %q", got, actionToolsSetSteamID)
	}
	for _, forbidden := range []string{"outcome", "stage", "before", "after", "field"} {
		if got := operationField(req, forbidden); got != "" {
			t.Errorf("requested carries %s=%q", forbidden, got)
		}
	}
}

func assertNoFieldRecords(t *testing.T, records []diagnosticRecord) {
	t.Helper()
	for _, ev := range []string{eventToolsChangeBefore, eventToolsChangePlanned, eventToolsChangeFinished} {
		if n := countEvent(records, ev); n != 0 {
			t.Errorf("%s emitted %d records, want 0", ev, n)
		}
	}
}

// assertStrictOrder verifies operation requested -> field before(all) -> field
// planned(all) -> field finished(all) -> operation finished.
func assertStrictOrder(t *testing.T, records []diagnosticRecord, lc toolsLifecycle) {
	t.Helper()
	reqIx := eventIndex(t, records, eventToolsOperationRequested)
	finIx := eventIndex(t, records, eventToolsOperationFinished)
	if reqIx >= minInt(lc.beforeIx) {
		t.Errorf("operation requested (%d) not before field before phase (%d)", reqIx, minInt(lc.beforeIx))
	}
	if maxInt(lc.beforeIx) >= minInt(lc.planIx) || maxInt(lc.planIx) >= minInt(lc.finIx) {
		t.Errorf("field grouping is not before -> planned -> finished")
	}
	if maxInt(lc.finIx) >= finIx {
		t.Errorf("operation finished (%d) not after field finished phase (%d)", finIx, maxInt(lc.finIx))
	}
}

func TestSetSteamIDFromStringSuccessAbsentToPresent(t *testing.T) {
	dir := t.TempDir()
	journal, err := newDiagnosticJournalInDir(dir)
	if err != nil {
		t.Fatalf("newDiagnosticJournalInDir: %v", err)
	}
	journal.SetDebugEnabled(true)
	t.Cleanup(func() { _ = journal.Close() })
	app := NewApp()
	app.journal = journal
	app.save = &core.SaveFile{SteamID: 0}

	if err := app.SetSteamIDFromString("76561197960287930"); err != nil {
		t.Fatalf("SetSteamIDFromString: %v", err)
	}
	if app.save.SteamID != rawSteamIDOne {
		t.Fatalf("save.SteamID = %d, want %d", app.save.SteamID, rawSteamIDOne)
	}

	tail := app.journal.Tail()
	assertOperationRequested(t, tail)
	assertOperationFinished(t, tail, string(characterChangeSuccess), toolsStageCompleted)

	lc := collectToolsLifecycle(t, tail)
	if len(lc.beforeIx) != 1 || len(lc.planIx) != 1 || len(lc.finIx) != 1 {
		t.Fatalf("phase counts = %d/%d/%d, want 1/1/1", len(lc.beforeIx), len(lc.planIx), len(lc.finIx))
	}
	if lc.before != steamIDAbsent {
		t.Errorf("before = %q, want %q", lc.before, steamIDAbsent)
	}
	if lc.planned != steamIDRedacted || lc.finished != steamIDRedacted {
		t.Errorf("planned/finished = %q/%q, want %q", lc.planned, lc.finished, steamIDRedacted)
	}
	assertStrictOrder(t, tail, lc)

	// Privacy: neither the marshaled tail nor the raw JSONL on disk may carry the
	// id or any contiguous fragment of length >= 5.
	assertNoRawSteamID(t, tail, rawSteamIDOne)
	assertNoRawSteamIDInString(t, readSessionRaw(t, dir), rawSteamIDOne)
}

func TestSetSteamIDFromStringReplacementNonZero(t *testing.T) {
	dir := t.TempDir()
	journal, err := newDiagnosticJournalInDir(dir)
	if err != nil {
		t.Fatalf("newDiagnosticJournalInDir: %v", err)
	}
	journal.SetDebugEnabled(true)
	t.Cleanup(func() { _ = journal.Close() })
	app := NewApp()
	app.journal = journal
	app.save = &core.SaveFile{SteamID: rawSteamIDOne}

	if err := app.SetSteamIDFromString("76561198088776655"); err != nil {
		t.Fatalf("SetSteamIDFromString: %v", err)
	}
	if app.save.SteamID != rawSteamIDTwo {
		t.Fatalf("save.SteamID = %d, want %d", app.save.SteamID, rawSteamIDTwo)
	}

	tail := app.journal.Tail()
	assertOperationRequested(t, tail)
	assertOperationFinished(t, tail, string(characterChangeSuccess), toolsStageCompleted)

	lc := collectToolsLifecycle(t, tail)
	if len(lc.beforeIx) != 1 || len(lc.planIx) != 1 || len(lc.finIx) != 1 {
		t.Fatalf("phase counts = %d/%d/%d, want 1/1/1 (redaction must not collapse the lifecycle)", len(lc.beforeIx), len(lc.planIx), len(lc.finIx))
	}
	if lc.before != steamIDRedacted || lc.planned != steamIDRedacted || lc.finished != steamIDRedacted {
		t.Errorf("before/planned/finished = %q/%q/%q, want all %q", lc.before, lc.planned, lc.finished, steamIDRedacted)
	}
	assertStrictOrder(t, tail, lc)

	// Both the replaced and the new id (and every >= 5 fragment) must be absent.
	assertNoRawSteamID(t, tail, rawSteamIDOne, rawSteamIDTwo)
	assertNoRawSteamIDInString(t, readSessionRaw(t, dir), rawSteamIDOne, rawSteamIDTwo)
}

func TestSetSteamIDFromStringNoop(t *testing.T) {
	app := steamIDApp(t, true, &core.SaveFile{SteamID: rawSteamIDOne})

	if err := app.SetSteamIDFromString("76561197960287930"); err != nil {
		t.Fatalf("SetSteamIDFromString: %v", err)
	}
	if app.save.SteamID != rawSteamIDOne {
		t.Fatalf("save.SteamID = %d, want %d", app.save.SteamID, rawSteamIDOne)
	}

	tail := app.journal.Tail()
	assertOperationRequested(t, tail)
	assertOperationFinished(t, tail, string(characterChangeSuccess), toolsStageCompleted)
	assertNoFieldRecords(t, tail)
	assertNoRawSteamID(t, tail, rawSteamIDOne)
}

func TestSetSteamIDFromStringNoActiveSave(t *testing.T) {
	app := steamIDApp(t, true, nil)

	err := app.SetSteamIDFromString("76561197960287930")
	if err == nil || err.Error() != "no save loaded" {
		t.Fatalf("error = %v, want \"no save loaded\"", err)
	}

	tail := app.journal.Tail()
	if got := countEvent(tail, eventToolsOperationRequested); got != 1 {
		t.Errorf("requested count = %d, want 1", got)
	}
	assertOperationFinished(t, tail, string(characterChangeError), toolsStageNoActiveSave)
	assertNoFieldRecords(t, tail)
}

func TestSetSteamIDFromStringInvalidInput(t *testing.T) {
	app := steamIDApp(t, true, &core.SaveFile{SteamID: 0})

	// An all-digit value that overflows uint64: if the parser error text or the
	// input ever leaked, its digits would appear in the journal.
	const overflow = "999999999999999999999999"
	err := app.SetSteamIDFromString(overflow)
	if err == nil || !strings.Contains(err.Error(), "invalid SteamID") {
		t.Fatalf("error = %v, want it to wrap \"invalid SteamID\"", err)
	}
	if app.save.SteamID != 0 {
		t.Fatalf("save.SteamID = %d, want 0 (unchanged)", app.save.SteamID)
	}

	tail := app.journal.Tail()
	if got := countEvent(tail, eventToolsOperationRequested); got != 1 {
		t.Errorf("requested count = %d, want 1", got)
	}
	assertOperationFinished(t, tail, string(characterChangeError), toolsStageParse)
	assertNoFieldRecords(t, tail)

	blob, marshalErr := json.Marshal(tail)
	if marshalErr != nil {
		t.Fatalf("marshal tail: %v", marshalErr)
	}
	if strings.Contains(string(blob), "99999") {
		t.Errorf("rejected input digits leaked into journal")
	}
}

func TestSetSteamIDFromStringDebugOff(t *testing.T) {
	app := steamIDApp(t, false, &core.SaveFile{SteamID: 0})

	if err := app.SetSteamIDFromString("76561197960287930"); err != nil {
		t.Fatalf("SetSteamIDFromString: %v", err)
	}
	if app.save.SteamID != rawSteamIDOne {
		t.Fatalf("save.SteamID = %d, want %d", app.save.SteamID, rawSteamIDOne)
	}

	// The session_started record is always present; only the two Tools event
	// families must be absent when Debug Mode is off.
	tail := app.journal.Tail()
	for _, ev := range []string{eventToolsOperationRequested, eventToolsOperationFinished} {
		if n := countEvent(tail, ev); n != 0 {
			t.Errorf("debug-off %s = %d, want 0", ev, n)
		}
	}
	assertNoFieldRecords(t, tail)
}
