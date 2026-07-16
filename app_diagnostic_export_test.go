package main

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
)

// newTestAppWithJournal returns an App wired to a real session journal in a
// throwaway directory, so scope-selection can be driven end-to-end without
// touching the real user-config directory.
func newTestAppWithJournal(t *testing.T) (*App, string) {
	t.Helper()
	dir := t.TempDir()
	j, err := newDiagnosticJournalInDir(dir)
	if err != nil {
		t.Fatalf("newDiagnosticJournalInDir: %v", err)
	}
	t.Cleanup(func() { j.Close() })
	app := NewApp()
	app.journal = j
	return app, dir
}

func eventNames(records []diagnosticRecord) []string {
	out := make([]string, len(records))
	for i, r := range records {
		out[i] = r.Event
	}
	return out
}

func containsEvent(records []diagnosticRecord, event string) bool {
	return hasEvent(records, event)
}

// --- Scope selection ---------------------------------------------------------

func TestSelectCurrentSessionReturnsAllRecords(t *testing.T) {
	app, _ := newTestAppWithJournal(t)
	app.journalLog(levelInfo, "op_one", "first")
	app.journalLog(levelWarn, "op_two", "second")

	records, err := app.selectDiagnosticRecords(exportScopeCurrentSession)
	if err != nil {
		t.Fatalf("selectDiagnosticRecords: %v", err)
	}
	want := []string{"session_started", "op_one", "op_two"}
	if got := eventNames(records); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("events = %v, want %v", got, want)
	}
}

func TestSelectCurrentSaveErrorsBeforeLoad(t *testing.T) {
	app, _ := newTestAppWithJournal(t)
	app.journalLog(levelInfo, "op", "no save yet")

	if _, err := app.selectDiagnosticRecords(exportScopeCurrentSave); err == nil {
		t.Fatal("current_save must error when no save has been loaded")
	}
}

func TestSelectCurrentSaveUsesNewestLoadMarker(t *testing.T) {
	app, _ := newTestAppWithJournal(t)

	app.commitLoadedSave(&core.SaveFile{Platform: "PC"}, "/first/secret/path.sl2", loadOriginFileDialog)
	app.journalLog(levelInfo, "marker_a", "work under first save")
	app.commitLoadedSave(&core.SaveFile{Platform: "PS4"}, "/second/secret/path.dat", loadOriginLocalPath)
	app.journalLog(levelInfo, "marker_b", "work under second save")

	records, err := app.selectDiagnosticRecords(exportScopeCurrentSave)
	if err != nil {
		t.Fatalf("selectDiagnosticRecords: %v", err)
	}
	if records[0].Event != eventSaveLoaded {
		t.Fatalf("first record = %q, want %q", records[0].Event, eventSaveLoaded)
	}
	// The newest load replaces the prior scope: only marker_b is in range.
	if containsEvent(records, "marker_a") {
		t.Error("current_save leaked records from the replaced (older) save scope")
	}
	if !containsEvent(records, "marker_b") {
		t.Error("current_save missing records from the active save scope")
	}
	// The active scope opener must carry the second load's origin.
	if got := records[0].Fields; len(got) == 0 || got[0].Value != string(loadOriginLocalPath) {
		t.Errorf("scope opener origin = %+v, want %q", got, loadOriginLocalPath)
	}
}

func TestCurrentSaveScopeClearedAfterCloseSave(t *testing.T) {
	app, _ := newTestAppWithJournal(t)

	app.commitLoadedSave(&core.SaveFile{Platform: "PC"}, "", loadOriginFileDialog)
	if _, err := app.selectDiagnosticRecords(exportScopeCurrentSave); err != nil {
		t.Fatalf("current_save should succeed while a save is loaded: %v", err)
	}

	if err := app.CloseSave(); err != nil {
		t.Fatalf("CloseSave: %v", err)
	}
	if _, err := app.selectDiagnosticRecords(exportScopeCurrentSave); err == nil {
		t.Fatal("current_save must error after CloseSave clears the scope")
	}
}

func TestSelectInvalidScopeErrors(t *testing.T) {
	app, _ := newTestAppWithJournal(t)
	if _, err := app.selectDiagnosticRecords("bogus_scope"); err == nil {
		t.Fatal("invalid scope must return an error")
	}
}

// --- Previous-unclosed recovery ---------------------------------------------

func TestFindNewestUnclosedSessionExcludesCurrentAndClosed(t *testing.T) {
	dir := t.TempDir()

	// The currently open journal: unclosed, but must never be a recovery target.
	cur, err := newDiagnosticJournalInDir(dir)
	if err != nil {
		t.Fatalf("open current: %v", err)
	}
	defer cur.Close()

	mk := func(events ...string) string {
		j, err := newDiagnosticJournalInDir(dir)
		if err != nil {
			t.Fatalf("open session: %v", err)
		}
		for _, e := range events {
			if err := j.Log(levelInfo, sourceApp, e, "x"); err != nil {
				t.Fatalf("log: %v", err)
			}
		}
		return j.Path() // deliberately left open (crash simulation)
	}

	oldUnclosed := mk("old_work")
	newerUnclosed := mk("newer_work")

	// A cleanly closed prior session, made the newest on disk to prove closure —
	// not recency — is why it is skipped.
	closed, err := newDiagnosticJournalInDir(dir)
	if err != nil {
		t.Fatalf("open closed: %v", err)
	}
	closedPath := closed.Path()
	if err := closed.Close(); err != nil {
		t.Fatalf("close closed: %v", err)
	}

	// Deterministic modtimes: closed newest, then current, then the two unclosed
	// with newerUnclosed after oldUnclosed.
	base := time.Now()
	must := func(p string, d time.Duration) {
		ts := base.Add(d)
		if err := os.Chtimes(p, ts, ts); err != nil {
			t.Fatalf("chtimes: %v", err)
		}
	}
	must(oldUnclosed, 1*time.Minute)
	must(newerUnclosed, 2*time.Minute)
	must(cur.Path(), 3*time.Minute)
	must(closedPath, 4*time.Minute)

	path, records, err := findNewestUnclosedSession(dir, cur.Path())
	if err != nil {
		t.Fatalf("findNewestUnclosedSession: %v", err)
	}
	if !sameFile(path, newerUnclosed) {
		t.Fatalf("recovery target = %q, want the newest unclosed %q", path, newerUnclosed)
	}
	if len(records) == 0 || !containsEvent(records, "newer_work") {
		t.Errorf("recovery records missing expected content: %v", eventNames(records))
	}
}

func TestFindNewestUnclosedSessionNoneWhenAllClosed(t *testing.T) {
	dir := t.TempDir()

	cur, err := newDiagnosticJournalInDir(dir)
	if err != nil {
		t.Fatalf("open current: %v", err)
	}
	defer cur.Close()

	prior, err := newDiagnosticJournalInDir(dir)
	if err != nil {
		t.Fatalf("open prior: %v", err)
	}
	if err := prior.Close(); err != nil { // clean close → not a recovery candidate
		t.Fatalf("close prior: %v", err)
	}

	path, _, err := findNewestUnclosedSession(dir, cur.Path())
	if err != nil {
		t.Fatalf("findNewestUnclosedSession: %v", err)
	}
	if path != "" {
		t.Fatalf("a cleanly closed prior session was offered for recovery: %q", path)
	}
}

func TestDiagnosticRecoveryStatusNilJournalIsSafe(t *testing.T) {
	app := NewApp() // no journal attached
	status, err := app.DiagnosticRecoveryStatus()
	if err != nil {
		t.Fatalf("DiagnosticRecoveryStatus: %v", err)
	}
	if status.HasUnclosedSession {
		t.Error("no journal should report no unclosed session")
	}
}

// --- ZIP artifact ------------------------------------------------------------

func readZipFiles(t *testing.T, path string) map[string]string {
	t.Helper()
	zr, err := zip.OpenReader(path)
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}
	defer zr.Close()
	out := make(map[string]string)
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open zip entry %q: %v", f.Name, err)
		}
		b, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			t.Fatalf("read zip entry %q: %v", f.Name, err)
		}
		out[f.Name] = string(b)
	}
	return out
}

func TestBuildDiagnosticZipContentsAndOrder(t *testing.T) {
	records := []diagnosticRecord{
		{SchemaVersion: 1, Seq: 1, Timestamp: "2026-01-01T00:00:00Z", Level: levelInfo, Source: sourceApp, Event: "session_started", Message: "started"},
		{SchemaVersion: 1, Seq: 2, Timestamp: "2026-01-01T00:00:01Z", Level: levelWarn, Source: sourceApp, Event: "op", Message: "safe items=42"},
		{SchemaVersion: 1, Seq: 3, Timestamp: "2026-01-01T00:00:02Z", Level: levelInfo, Source: sourceApp, Event: "op2", Message: "more"},
	}
	dest := filepath.Join(t.TempDir(), "out.zip")
	if err := buildDiagnosticZip(dest, exportScopeCurrentSession, records); err != nil {
		t.Fatalf("buildDiagnosticZip: %v", err)
	}

	files := readZipFiles(t, dest)
	if len(files) != 2 {
		t.Fatalf("zip has %d files, want exactly 2 (summary.txt, events.jsonl)", len(files))
	}

	summary, ok := files["summary.txt"]
	if !ok {
		t.Fatal("missing summary.txt")
	}
	for _, frag := range []string{"Scope: current_session", "Records: 3", "Schema version: 1", "2026-01-01T00:00:00Z .. 2026-01-01T00:00:02Z", "Privacy:"} {
		if !strings.Contains(summary, frag) {
			t.Errorf("summary.txt missing %q\n---\n%s", frag, summary)
		}
	}

	events, ok := files["events.jsonl"]
	if !ok {
		t.Fatal("missing events.jsonl")
	}
	lines := strings.Split(strings.TrimRight(events, "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("events.jsonl has %d lines, want 3", len(lines))
	}
	// Original sequence order preserved: seq 1 → 2 → 3.
	for i, want := range []string{`"seq":1`, `"seq":2`, `"seq":3`} {
		if !strings.Contains(lines[i], want) {
			t.Errorf("line %d = %q, want to contain %q", i, lines[i], want)
		}
	}
}

func TestBuildDiagnosticZipCarriesNoRawSecrets(t *testing.T) {
	// End-to-end: sanitize at the journal boundary, then export. The archive
	// must inherit the redaction — no raw path or Steam ID survives.
	app, _ := newTestAppWithJournal(t)
	app.journalLog(levelInfo, "op", "loading /Users/alice/private/ER0000.sl2 steam 76561198000000000")

	records, err := app.selectDiagnosticRecords(exportScopeCurrentSession)
	if err != nil {
		t.Fatalf("select: %v", err)
	}
	dest := filepath.Join(t.TempDir(), "secrets.zip")
	if err := buildDiagnosticZip(dest, exportScopeCurrentSession, records); err != nil {
		t.Fatalf("buildDiagnosticZip: %v", err)
	}
	events := readZipFiles(t, dest)["events.jsonl"]
	for _, leak := range []string{"ER0000.sl2", "76561198000000000", "alice"} {
		if strings.Contains(events, leak) {
			t.Errorf("events.jsonl leaked sanitized value %q", leak)
		}
	}
	if !strings.Contains(events, "[redacted") {
		t.Error("expected redaction markers in exported events")
	}
}

func TestFinishDiagnosticExportCancellation(t *testing.T) {
	res, err := finishDiagnosticExport(exportScopeCurrentSession, "", nil)
	if err != nil {
		t.Fatalf("cancellation must not error: %v", err)
	}
	if !res.Cancelled {
		t.Error("empty destination must yield a cancelled result")
	}
	if res.Path != "" {
		t.Errorf("cancelled result path = %q, want empty", res.Path)
	}
}

func TestBuildDiagnosticZipCleansUpTempOnFailure(t *testing.T) {
	dir := t.TempDir()
	// Destination is an existing directory: os.Rename onto it fails, forcing the
	// atomic build to clean up its temp file.
	dest := filepath.Join(dir, "adir")
	if err := os.Mkdir(dest, 0o700); err != nil {
		t.Fatalf("mkdir dest: %v", err)
	}
	records := []diagnosticRecord{{SchemaVersion: 1, Seq: 1, Timestamp: "2026-01-01T00:00:00Z", Event: "e"}}
	if err := buildDiagnosticZip(dest, exportScopeCurrentSession, records); err == nil {
		t.Fatal("expected buildDiagnosticZip to fail when dest is a directory")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".saveforge-diag-") {
			t.Errorf("leftover temp export file not cleaned up: %q", e.Name())
		}
	}
}

// --- Fix 1: scope-marker linearity under concurrency -------------------------

func scopeMarkerField(rec diagnosticRecord, key string) string {
	for _, f := range rec.Fields {
		if f.Key == key {
			return f.Value
		}
	}
	return ""
}

// TestScopeMarkerLinearityUnderConcurrency fires many concurrent loads and
// closes at one journal and asserts that, once quiesced, the final a.save state
// and the LAST scope marker agree. Because each transition holds
// diagnosticScopeMu across both its state change and its marker append, the
// transition that runs last sets both — so they can never diverge. No sleeps.
func TestScopeMarkerLinearityUnderConcurrency(t *testing.T) {
	app, _ := newTestAppWithJournal(t)

	const workers = 24
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if i%3 == 0 {
				_ = app.CloseSave()
				return
			}
			plat := core.Platform(fmt.Sprintf("P%02d", i))
			app.commitLoadedSave(&core.SaveFile{Platform: plat}, "", loadOriginFileDialog)
		}(i)
	}
	wg.Wait()

	app.saveMu.RLock()
	hasSave := app.save != nil
	finalPlat := ""
	if hasSave {
		finalPlat = string(app.save.Platform)
	}
	app.saveMu.RUnlock()

	records, err := readJournalFile(app.journal.Path())
	if err != nil {
		t.Fatalf("readJournalFile: %v", err)
	}
	var last diagnosticRecord
	found := false
	for _, r := range records {
		if r.Event == eventSaveLoaded || r.Event == eventSaveClosed {
			last, found = r, true
		}
	}
	if !found {
		t.Fatal("no scope marker was written")
	}
	if hasSave {
		if last.Event != eventSaveLoaded {
			t.Fatalf("final save present but last marker is %q", last.Event)
		}
		if got := scopeMarkerField(last, "platform"); got != finalPlat {
			t.Fatalf("last marker platform %q != final save platform %q", got, finalPlat)
		}
	} else {
		if last.Event != eventSaveClosed {
			t.Fatalf("final save is nil but last marker is %q", last.Event)
		}
	}

	// current_save selection must also agree with the final state.
	_, selErr := app.selectDiagnosticRecords(exportScopeCurrentSave)
	if hasSave && selErr != nil {
		t.Errorf("current_save errored though a save is active: %v", selErr)
	}
	if !hasSave && selErr == nil {
		t.Error("current_save succeeded though no save is active")
	}
}

// TestCurrentSaveSelectionWaitsForScopeMu proves current_save selection cannot
// observe the window between an in-flight transition mutating a.save and that
// transition appending its scope marker. The selection takes diagnosticScopeMu,
// so while a simulated transition holds it the read blocks; once released the
// read sees the marker written under the lock — never the prior save's scope.
// Deterministic: the read is parked on the lock we hold and can only proceed
// after Unlock, so it can never complete before the marker is durable. No
// sleeps in the code under test; the timeout only asserts the read is blocked.
func TestCurrentSaveSelectionWaitsForScopeMu(t *testing.T) {
	app, _ := newTestAppWithJournal(t)
	app.commitLoadedSave(&core.SaveFile{Platform: "PC"}, "", loadOriginFileDialog)

	// Reproduce the exact mid-transition state commitLoadedSave passes through:
	// hold the outermost scope lock, install the new active save under saveMu
	// via the same helper the real load path uses, release saveMu — but do NOT
	// yet append the save_loaded marker. a.save is now PS4 while the journal's
	// last marker still points at PC.
	app.diagnosticScopeMu.Lock()
	app.saveMu.Lock()
	app.installLoadedSave(&core.SaveFile{Platform: "PS4"}, "")
	app.saveMu.Unlock()

	type result struct {
		recs []diagnosticRecord
		err  error
	}
	done := make(chan result, 1)
	go func() {
		recs, err := app.selectDiagnosticRecords(exportScopeCurrentSave)
		done <- result{recs, err}
	}()

	// The selection must block on diagnosticScopeMu while we hold it. If it
	// returns, it read the journal mid-transition and would resolve the PC
	// scope while a.save is already PS4 — the exact bug this guards against.
	select {
	case <-done:
		app.diagnosticScopeMu.Unlock()
		t.Fatal("current_save selection did not wait for diagnosticScopeMu")
	case <-time.After(time.Second):
	}

	// Complete the transition: append the PS4 marker, still under the lock,
	// then release — matching commitLoadedSave's (mutate, marker) sequence.
	app.journalLog(levelInfo, eventSaveLoaded, "second save loaded", field("platform", "PS4"))
	app.diagnosticScopeMu.Unlock()

	res := <-done
	if res.err != nil {
		t.Fatalf("selection after unlock: %v", res.err)
	}
	if len(res.recs) == 0 {
		t.Fatal("selection returned no records")
	}
	// Serialised after the marker: the scope must open at the PS4 marker, and
	// that must agree with the now-active a.save — never the prior PC scope.
	if got := res.recs[0]; got.Event != eventSaveLoaded || scopeMarkerField(got, "platform") != "PS4" {
		t.Fatalf("scope opener = %+v, want the PS4 marker appended under the lock", got)
	}
	app.saveMu.RLock()
	activePlat := ""
	if app.save != nil {
		activePlat = string(app.save.Platform)
	}
	app.saveMu.RUnlock()
	if activePlat != "PS4" {
		t.Fatalf("active save platform = %q, want PS4", activePlat)
	}
}

// --- Fix 2: JSONL integrity --------------------------------------------------

func mustRecordJSON(t *testing.T, seq uint64, event string) string {
	t.Helper()
	b, err := json.Marshal(diagnosticRecord{
		SchemaVersion: 1, Seq: seq, Timestamp: "2026-01-01T00:00:00Z",
		Level: levelInfo, Source: sourceApp, Event: event, Message: "m",
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(b)
}

func TestDecodeJournalToleratesTornTrailingFragment(t *testing.T) {
	r1 := mustRecordJSON(t, 1, "a")
	r2 := mustRecordJSON(t, 2, "b")
	// Two complete lines, then a truncated final fragment with no newline.
	data := r1 + "\n" + r2 + "\n" + `{"schema_version":1,"seq":3,"event":"c`
	recs, err := decodeJournalRecords(strings.NewReader(data))
	if err != nil {
		t.Fatalf("torn trailing fragment must not error: %v", err)
	}
	if len(recs) != 2 {
		t.Fatalf("got %d records, want 2 (both complete lines preserved)", len(recs))
	}
	if recs[0].Event != "a" || recs[1].Event != "b" {
		t.Errorf("records reordered/dropped: %v", eventNames(recs))
	}
}

func TestDecodeJournalErrorsOnCorruptMiddleLine(t *testing.T) {
	r1 := mustRecordJSON(t, 1, "a")
	r2 := mustRecordJSON(t, 2, "b")
	corrupt := "{not valid json}"
	data := r1 + "\n" + corrupt + "\n" + r2 + "\n"
	_, err := decodeJournalRecords(strings.NewReader(data))
	if err == nil {
		t.Fatal("corrupt middle line must return an error")
	}
	if strings.Contains(err.Error(), corrupt) {
		t.Errorf("error leaked corrupt record content: %v", err)
	}
}

func TestDecodeJournalErrorsOnCorruptTerminatedLastLine(t *testing.T) {
	r1 := mustRecordJSON(t, 1, "a")
	// A corrupt but newline-terminated final line is real corruption, not a
	// torn fragment: it must error.
	data := r1 + "\n" + "{broken\n"
	_, err := decodeJournalRecords(strings.NewReader(data))
	if err == nil {
		t.Fatal("corrupt terminated last line must return an error")
	}
	if strings.Contains(err.Error(), "broken") {
		t.Errorf("error leaked corrupt record content: %v", err)
	}
}

// --- Fix 3: failed journal export safety -------------------------------------

func TestExportScopesFailForFailedJournal(t *testing.T) {
	app, _ := newTestAppWithJournal(t)

	// Force the journal into the permanent failed state: close the underlying
	// file out from under it, then drive one failing append.
	if err := app.journal.f.Close(); err != nil {
		t.Fatalf("pre-close file: %v", err)
	}
	if err := app.journal.Log(levelInfo, sourceApp, "x", "y"); err == nil {
		t.Fatal("expected the forcing Log to fail")
	}

	for _, scope := range []string{exportScopeCurrentSession, exportScopeCurrentSave} {
		_, err := app.selectDiagnosticRecords(scope)
		if err == nil {
			t.Errorf("scope %q must return a safe error on a failed journal", scope)
			continue
		}
		if strings.Contains(err.Error(), app.journal.path) {
			t.Errorf("scope %q error leaked the session path: %v", scope, err)
		}
	}
}
