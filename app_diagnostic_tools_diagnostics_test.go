package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
	"github.com/oisis/EldenRing-SaveForge/backend/editor"
)

// These tests cover the private Debug Mode lifecycle for the two read-only Tools
// actions ScanRepairIssuesLoaded and ExportDiagnosticLog. Both emit ONLY the
// operation lifecycle (tools_operation_requested -> tools_operation_finished) and
// never the per-field tools_change_*; they mutate no save. Helpers operationRecords,
// assertNoPathLeak, operationField, operationEvent, countEvent, eventIndex and
// assertNoFieldRecords are shared with the other Tools diagnostics tests.

// privateCharName is a deliberately private, alphabetic 16-char name (the full
// CharacterName width) so a leak scan of the operation records has a real token to
// look for. It must never appear in any operation record.
const privateCharName = "PrivateHeroXyzzy"

func setPrivateCharName(slot *core.SaveSlot) {
	for i, r := range privateCharName {
		if i >= len(slot.Player.CharacterName) {
			break
		}
		slot.Player.CharacterName[i] = uint16(r)
	}
}

// scanDiagApp wires an App to a file-backed journal (debug configurable) with a
// healthy, non-empty slot 0 carrying the private character name. No on-disk user
// save is touched — the slot is the synthetic native-hole fixture.
func scanDiagApp(t *testing.T, debug bool) (*App, int) {
	t.Helper()
	journal, err := newDiagnosticJournalInDir(t.TempDir())
	if err != nil {
		t.Fatalf("newDiagnosticJournalInDir: %v", err)
	}
	journal.SetDebugEnabled(debug)
	t.Cleanup(func() { _ = journal.Close() })
	app := NewApp()
	app.journal = journal
	app.save = &core.SaveFile{}
	app.save.Slots[0] = *nativeHoleSlotFixture(t)
	setPrivateCharName(&app.save.Slots[0])
	app.saveGeneration = 1
	return app, 0
}

// assertScanOperationPair asserts exactly one requested and one finished record for
// the scan action, requested before finished, with the given closed outcome/stage,
// and no per-field tools_change_* records anywhere.
func assertScanOperationPair(t *testing.T, records []diagnosticRecord, outcome, stage string) {
	t.Helper()
	ops := operationRecords(records)
	if got := countEvent(ops, eventToolsOperationRequested); got != 1 {
		t.Fatalf("requested count = %d, want 1", got)
	}
	if got := countEvent(ops, eventToolsOperationFinished); got != 1 {
		t.Fatalf("finished count = %d, want 1", got)
	}
	req := operationEvent(t, ops, eventToolsOperationRequested)
	if got := operationField(req, "action"); got != actionToolsScanRepairIssuesLoaded {
		t.Errorf("requested action = %q, want %q", got, actionToolsScanRepairIssuesLoaded)
	}
	assertFieldKeys(t, "requested", req, "action")
	fin := operationEvent(t, ops, eventToolsOperationFinished)
	if got := operationField(fin, "action"); got != actionToolsScanRepairIssuesLoaded {
		t.Errorf("finished action = %q, want %q", got, actionToolsScanRepairIssuesLoaded)
	}
	if got := operationField(fin, "outcome"); got != outcome {
		t.Errorf("finished outcome = %q, want %q", got, outcome)
	}
	if got := operationField(fin, "stage"); got != stage {
		t.Errorf("finished stage = %q, want %q", got, stage)
	}
	assertFieldKeys(t, "finished", fin, "action", "outcome", "stage")
	if reqIx, finIx := eventIndex(t, records, eventToolsOperationRequested), eventIndex(t, records, eventToolsOperationFinished); reqIx >= finIx {
		t.Errorf("requested (%d) not before finished (%d)", reqIx, finIx)
	}
	assertNoFieldRecords(t, records)
}

// TestToolsScanRepairIssuesLoadedDiagnosticsSuccess: a debug scan reports issues,
// leaves the slot and the live workspace session untouched, emits exactly
// requested + finished success/completed and no tools_change_*, and leaks neither
// the character name nor report data into the operation records.
func TestToolsScanRepairIssuesLoadedDiagnosticsSuccess(t *testing.T) {
	app, charIdx := scanDiagApp(t, true)

	// Establish a live Inventory Workspace so the scan's read-only-ness w.r.t. an
	// existing session can be verified (same ID, same object, count unchanged).
	if _, err := app.StartInventoryEditSession(charIdx); err != nil {
		t.Fatalf("StartInventoryEditSession: %v", err)
	}
	app.editSessionsMu.Lock()
	wantID := app.editSessionByChar[charIdx]
	wantSess := app.editSessions[wantID]
	app.editSessionsMu.Unlock()

	slot := &app.save.Slots[charIdx]
	before := core.CloneSlot(slot)

	report, err := app.ScanRepairIssuesLoaded(charIdx)
	if err != nil {
		t.Fatalf("ScanRepairIssuesLoaded: %v", err)
	}
	if report.SlotIndex != charIdx {
		t.Fatalf("report SlotIndex = %d, want %d", report.SlotIndex, charIdx)
	}
	if report.CharName != privateCharName {
		t.Fatalf("report CharName = %q, want %q (test fixture wrong)", report.CharName, privateCharName)
	}

	// Slot untouched.
	if !reflect.DeepEqual(*before, app.save.Slots[charIdx]) {
		t.Error("scan mutated the slot; it must be read-only")
	}
	// Workspace session untouched: same ID, same object, still exactly one.
	app.editSessionsMu.Lock()
	gotID := app.editSessionByChar[charIdx]
	gotSess := app.editSessions[gotID]
	nSessions := len(app.editSessions)
	app.editSessionsMu.Unlock()
	if gotID != wantID || gotSess != wantSess || nSessions != 1 {
		t.Errorf("scan disturbed the workspace session: id %q->%q, replaced=%v, count=%d", wantID, gotID, gotSess != wantSess, nSessions)
	}

	tail := app.journal.Tail()
	assertScanOperationPair(t, tail, string(characterChangeSuccess), toolsStageCompleted)

	// The operation records must not carry the character name (nor any >=5 fragment).
	assertNoPathLeak(t, operationRecords(tail), nil, privateCharName)
}

// TestToolsScanRepairIssuesLoadedDiagnosticsFailureTable exercises every scan
// rejection: no active save, invalid index, empty slot and a deterministic
// build-snapshot error. Each preserves the exact public error and reports
// requested + finished error/<stage> with no tools_change_* and no error text.
func TestToolsScanRepairIssuesLoadedDiagnosticsFailureTable(t *testing.T) {
	// A distinctive synthetic build-snapshot error whose text must never reach the
	// operation records.
	const buildErrToken = "SyntheticBuildSnapshotFailureSecret"

	cases := []struct {
		name      string
		setup     func(t *testing.T) (*App, int)
		wantErr   string
		wantStage string
	}{
		{
			name: "no_active_save",
			setup: func(t *testing.T) (*App, int) {
				app, idx := scanDiagApp(t, true)
				app.save = nil
				return app, idx
			},
			wantErr:   "ScanRepairIssuesLoaded: no save loaded",
			wantStage: toolsStageNoActiveSave,
		},
		{
			name: "invalid_character",
			setup: func(t *testing.T) (*App, int) {
				app, _ := scanDiagApp(t, true)
				return app, -1
			},
			wantErr:   "ScanRepairIssuesLoaded: invalid charIdx -1",
			wantStage: toolsStageInvalidCharacter,
		},
		{
			name: "empty_slot",
			setup: func(t *testing.T) (*App, int) {
				app, idx := scanDiagApp(t, true)
				app.save.Slots[idx] = core.SaveSlot{} // Version == 0
				return app, idx
			},
			wantErr:   "ScanRepairIssuesLoaded: slot 0 is empty",
			wantStage: toolsStageEmptySlot,
		},
		{
			name: "build_snapshot",
			setup: func(t *testing.T) (*App, int) {
				app, idx := scanDiagApp(t, true)
				orig := buildValidationSnapshot
				buildValidationSnapshot = func(*core.SaveSlot, string, int) (editor.InventoryWorkspaceSnapshot, error) {
					return editor.InventoryWorkspaceSnapshot{}, errors.New(buildErrToken)
				}
				t.Cleanup(func() { buildValidationSnapshot = orig })
				return app, idx
			},
			wantErr:   "ScanRepairIssuesLoaded: " + buildErrToken,
			wantStage: toolsStageBuildSnapshot,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app, idx := tc.setup(t)
			_, err := app.ScanRepairIssuesLoaded(idx)
			if err == nil || err.Error() != tc.wantErr {
				t.Fatalf("error = %v, want %q", err, tc.wantErr)
			}

			tail := app.journal.Tail()
			assertScanOperationPair(t, tail, string(characterChangeError), tc.wantStage)

			// No error text (nor the char name) may leak into the operation records.
			blob, err := json.Marshal(operationRecords(tail))
			if err != nil {
				t.Fatalf("marshal operation records: %v", err)
			}
			for _, secret := range []string{buildErrToken, privateCharName} {
				if strings.Contains(string(blob), secret) {
					t.Errorf("operation records leaked %q", secret)
				}
			}
		})
	}
}

// TestToolsScanRepairIssuesLoadedDiagnosticsDebugOff: with Debug off the scan still
// works and emits zero tools_operation_* records.
func TestToolsScanRepairIssuesLoadedDiagnosticsDebugOff(t *testing.T) {
	app, charIdx := scanDiagApp(t, false)

	report, err := app.ScanRepairIssuesLoaded(charIdx)
	if err != nil {
		t.Fatalf("ScanRepairIssuesLoaded: %v", err)
	}
	if report.SlotIndex != charIdx {
		t.Fatalf("report SlotIndex = %d, want %d", report.SlotIndex, charIdx)
	}

	tail := app.journal.Tail()
	for _, ev := range []string{eventToolsOperationRequested, eventToolsOperationFinished} {
		if n := countEvent(tail, ev); n != 0 {
			t.Errorf("debug-off %s = %d, want 0", ev, n)
		}
	}
}

// ---- ExportDiagnosticLog ----------------------------------------------------

// exportDiagApp wires an App to a file-backed journal (debug configurable). The
// session file always holds at least the session_started record, so the
// current_session scope resolves to a non-empty record set.
func exportDiagApp(t *testing.T, debug bool) *App {
	t.Helper()
	journal, err := newDiagnosticJournalInDir(t.TempDir())
	if err != nil {
		t.Fatalf("newDiagnosticJournalInDir: %v", err)
	}
	journal.SetDebugEnabled(debug)
	t.Cleanup(func() { _ = journal.Close() })
	app := NewApp()
	app.journal = journal
	return app
}

// secretExportDest builds a private-looking .zip destination inside a fresh temp
// tree so a leak scan has real path tokens to hunt. The parent dir is created so
// the ZIP write succeeds.
func secretExportDest(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), secretDirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	return filepath.Join(dir, "PrivateArchiveBlob.zip")
}

// assertExportOperationPair asserts exactly one requested + one finished record for
// the export action with the given outcome/stage, requested before finished, and no
// tools_change_* anywhere.
func assertExportOperationPair(t *testing.T, records []diagnosticRecord, outcome, stage string) {
	t.Helper()
	ops := operationRecords(records)
	if got := countEvent(ops, eventToolsOperationRequested); got != 1 {
		t.Fatalf("requested count = %d, want 1", got)
	}
	if got := countEvent(ops, eventToolsOperationFinished); got != 1 {
		t.Fatalf("finished count = %d, want 1", got)
	}
	req := operationEvent(t, ops, eventToolsOperationRequested)
	if got := operationField(req, "action"); got != actionToolsExportDiagnosticLog {
		t.Errorf("requested action = %q, want %q", got, actionToolsExportDiagnosticLog)
	}
	assertFieldKeys(t, "requested", req, "action")
	fin := operationEvent(t, ops, eventToolsOperationFinished)
	if got := operationField(fin, "action"); got != actionToolsExportDiagnosticLog {
		t.Errorf("finished action = %q, want %q", got, actionToolsExportDiagnosticLog)
	}
	if got := operationField(fin, "outcome"); got != outcome {
		t.Errorf("finished outcome = %q, want %q", got, outcome)
	}
	if got := operationField(fin, "stage"); got != stage {
		t.Errorf("finished stage = %q, want %q", got, stage)
	}
	assertFieldKeys(t, "finished", fin, "action", "outcome", "stage")
	if reqIx, finIx := eventIndex(t, records, eventToolsOperationRequested), eventIndex(t, records, eventToolsOperationFinished); reqIx >= finIx {
		t.Errorf("requested (%d) not before finished (%d)", reqIx, finIx)
	}
	assertNoFieldRecords(t, records)
}

// TestToolsDiagnosticsExportSuccess: a debug export through the private driver writes
// the ZIP, emits requested + finished success/completed and no tools_change_*, and
// leaks neither the scope, the full destination path, nor any sensitive path
// fragment into the operation records.
func TestToolsDiagnosticsExportSuccess(t *testing.T) {
	app := exportDiagApp(t, true)
	dest := secretExportDest(t)

	result, err := app.exportDiagnosticLog(exportScopeCurrentSession, func() (string, error) {
		return dest, nil
	})
	if err != nil {
		t.Fatalf("exportDiagnosticLog: %v", err)
	}
	if result.Cancelled || result.Path != dest || result.RecordCount <= 0 {
		t.Fatalf("result = %+v, want a completed export at %q with records", result, dest)
	}
	if _, err := os.Stat(dest); err != nil {
		t.Fatalf("export ZIP not written: %v", err)
	}

	tail := app.journal.Tail()
	assertExportOperationPair(t, tail, string(characterChangeSuccess), toolsStageCompleted)

	// Serialize ONLY the operation records and prove no scope, no full destination,
	// and no sensitive path fragment (>=5) survives.
	assertNoPathLeak(t, operationRecords(tail), []string{dest, exportScopeCurrentSession}, secretDirName, "PrivateArchiveBlob")
}

// TestToolsDiagnosticsExportTerminalPaths drives every terminal export path through
// the private driver: invalid scope, select-scope error, dialog error, cancelled and
// write error. Each asserts the exact closed outcome/stage, no private value in the
// operation records, and no tools_change_*.
func TestToolsDiagnosticsExportTerminalPaths(t *testing.T) {
	cancelDest := func() (string, error) { return "", nil }
	dialogErr := func() (string, error) { return "", errors.New("native dialog exploded") }

	cases := []struct {
		name       string
		scope      string
		chooseDest func() (string, error)
		wantOut    string
		wantStage  string
		wantErr    bool
		// leakTokens are private strings that must never appear in the operation records.
		leakTokens []string
	}{
		{
			name:       "invalid_scope",
			scope:      "totally-bogus-scope",
			chooseDest: func() (string, error) { t.Helper(); t.Fatal("dialog must not open for invalid scope"); return "", nil },
			wantOut:    string(characterChangeError),
			wantStage:  toolsStageInvalidScope,
			wantErr:    true,
			leakTokens: []string{"totally-bogus-scope"},
		},
		{
			name:       "select_scope",
			scope:      exportScopeCurrentSave, // no save_loaded marker -> selection error
			chooseDest: func() (string, error) { t.Helper(); t.Fatal("dialog must not open on select error"); return "", nil },
			wantOut:    string(characterChangeError),
			wantStage:  toolsStageSelectScope,
			wantErr:    true,
			leakTokens: []string{exportScopeCurrentSave},
		},
		{
			name:       "dialog",
			scope:      exportScopeCurrentSession,
			chooseDest: dialogErr,
			wantOut:    string(characterChangeError),
			wantStage:  toolsStageDialog,
			wantErr:    true,
			leakTokens: []string{exportScopeCurrentSession, "exploded"},
		},
		{
			name:       "cancelled",
			scope:      exportScopeCurrentSession,
			chooseDest: cancelDest,
			wantOut:    string(characterChangeSuccess),
			wantStage:  toolsStageCancelled,
			wantErr:    false,
			leakTokens: []string{exportScopeCurrentSession},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app := exportDiagApp(t, true)
			result, err := app.exportDiagnosticLog(tc.scope, tc.chooseDest)
			if tc.wantErr != (err != nil) {
				t.Fatalf("err = %v, wantErr = %v", err, tc.wantErr)
			}
			if tc.name == "cancelled" && !result.Cancelled {
				t.Errorf("cancelled path result = %+v, want Cancelled", result)
			}

			tail := app.journal.Tail()
			assertExportOperationPair(t, tail, tc.wantOut, tc.wantStage)
			assertNoOpsLeak(t, operationRecords(tail), tc.leakTokens...)
		})
	}

	// Write error: a destination whose parent directory does not exist makes the
	// atomic ZIP write fail deterministically at CreateTemp.
	t.Run("write", func(t *testing.T) {
		app := exportDiagApp(t, true)
		dest := filepath.Join(t.TempDir(), "NoSuchSubdir", secretDirName, "PrivateArchiveTarget.zip")
		_, err := app.exportDiagnosticLog(exportScopeCurrentSession, func() (string, error) {
			return dest, nil
		})
		if err == nil {
			t.Fatal("exportDiagnosticLog error = nil, want a ZIP write failure")
		}

		tail := app.journal.Tail()
		assertExportOperationPair(t, tail, string(characterChangeError), toolsStageWrite)
		assertNoPathLeak(t, operationRecords(tail), []string{dest, exportScopeCurrentSession}, secretDirName, "PrivateArchiveTarget")
	})
}

// assertNoOpsLeak marshals the operation records and asserts none of the private
// tokens appears verbatim.
func assertNoOpsLeak(t *testing.T, records []diagnosticRecord, tokens ...string) {
	t.Helper()
	blob, err := json.Marshal(records)
	if err != nil {
		t.Fatalf("marshal operation records: %v", err)
	}
	for _, tok := range tokens {
		if tok != "" && strings.Contains(string(blob), tok) {
			t.Errorf("operation records leaked %q", tok)
		}
	}
}

// TestToolsDiagnosticsExportDebugOff: with Debug off the driver still writes the ZIP
// and emits zero tools_operation_*, while the always-on diagnostic_export_* events
// remain intact.
func TestToolsDiagnosticsExportDebugOff(t *testing.T) {
	app := exportDiagApp(t, false)
	dest := secretExportDest(t)

	result, err := app.exportDiagnosticLog(exportScopeCurrentSession, func() (string, error) {
		return dest, nil
	})
	if err != nil {
		t.Fatalf("exportDiagnosticLog: %v", err)
	}
	if result.Cancelled || result.Path != dest {
		t.Fatalf("result = %+v, want a completed export at %q", result, dest)
	}
	if _, err := os.Stat(dest); err != nil {
		t.Fatalf("export ZIP not written with debug off: %v", err)
	}

	tail := app.journal.Tail()
	for _, ev := range []string{eventToolsOperationRequested, eventToolsOperationFinished} {
		if n := countEvent(tail, ev); n != 0 {
			t.Errorf("debug-off %s = %d, want 0", ev, n)
		}
	}
	// The normal, always-on export events must remain.
	for _, ev := range []string{"diagnostic_export_requested", "diagnostic_export_finished"} {
		if n := countEvent(tail, ev); n < 1 {
			t.Errorf("normal %s = %d, want >=1", ev, n)
		}
	}
}
