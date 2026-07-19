package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
)

// backupApp builds an App wired to a file-backed journal with a preset save and
// lastSavePath. save == nil / path == "" models "no active save". No real user
// save is touched: the save is an in-memory core.SaveFile and path points into a
// deliberately sensitive-looking t.TempDir() tree so the leak scans have real
// path fragments to look for.
func backupApp(t *testing.T, debug bool, save *core.SaveFile, path string) *App {
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
	app.lastSavePath = path
	return app
}

// operationRecords returns only the tools_operation_* records, the sole family
// BackupCurrentSave may emit. Serializing exactly these (and nothing else) is the
// surface the path-leak scan runs against.
func operationRecords(records []diagnosticRecord) []diagnosticRecord {
	var out []diagnosticRecord
	for _, rec := range records {
		if rec.Event == eventToolsOperationRequested || rec.Event == eventToolsOperationFinished {
			out = append(out, rec)
		}
	}
	return out
}

// The sensitive, alphabetic-only path tokens the tests invent. They are decoupled
// from t.TempDir() on purpose: t.TempDir() embeds the running test's name (which
// legitimately contains "Backup"/"Current"/"Success"), so scanning the temp prefix
// would false-positive against the operation vocab. Scanning exactly these
// self-chosen tokens proves any real directory/filename leak while staying clear of
// the harness-injected prefix.
const (
	secretDirName   = "SuperSecretPlayerProfile"
	secretFileName  = "MyPrivateEldenRingCharacter.sltwo"
	secretSourceDir = "SuperSecretPlayerDirectory"
)

// assertNoPathLeak marshals ONLY the given records and asserts that neither each
// full path (whole-string) nor any non-numeric contiguous fragment of length >= 5
// of the sensitive tokens appears. All-digit fragments are skipped: the only digit
// runs in a backup path come from its timestamp suffix (never sensitive), while
// every sensitive token here is alphabetic.
func assertNoPathLeak(t *testing.T, records []diagnosticRecord, fullPaths []string, tokens ...string) {
	t.Helper()
	blob, err := json.Marshal(records)
	if err != nil {
		t.Fatalf("marshal operation records: %v", err)
	}
	hay := string(blob)
	for _, p := range fullPaths {
		if p != "" && strings.Contains(hay, p) {
			t.Errorf("full path %q leaked into operation records", p)
		}
	}
	for _, tok := range tokens {
		for start := 0; start+5 <= len(tok); start++ {
			for end := start + 5; end <= len(tok); end++ {
				frag := tok[start:end]
				if isAllDigits(frag) {
					continue
				}
				if strings.Contains(hay, frag) {
					t.Errorf("path fragment %q leaked into operation records", frag)
				}
			}
		}
	}
}

func isAllDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return len(s) > 0
}

// assertBackupOperationRequested checks the single requested record carries only
// the backup action.
func assertBackupOperationRequested(t *testing.T, records []diagnosticRecord) {
	t.Helper()
	if got := countEvent(records, eventToolsOperationRequested); got != 1 {
		t.Fatalf("requested count = %d, want 1", got)
	}
	req := operationEvent(t, records, eventToolsOperationRequested)
	if got := operationField(req, "action"); got != actionToolsBackupCurrentSave {
		t.Errorf("requested action = %q, want %q", got, actionToolsBackupCurrentSave)
	}
	for _, forbidden := range []string{"outcome", "stage", "before", "after", "field"} {
		if got := operationField(req, forbidden); got != "" {
			t.Errorf("requested carries %s=%q", forbidden, got)
		}
	}
}

// assertBackupOperationFinished checks the single finished record carries exactly
// action/outcome/stage and nothing value-bearing.
func assertBackupOperationFinished(t *testing.T, records []diagnosticRecord, outcome, stage string) {
	t.Helper()
	if got := countEvent(records, eventToolsOperationFinished); got != 1 {
		t.Fatalf("finished count = %d, want 1", got)
	}
	fin := operationEvent(t, records, eventToolsOperationFinished)
	if got := operationField(fin, "action"); got != actionToolsBackupCurrentSave {
		t.Errorf("finished action = %q, want %q", got, actionToolsBackupCurrentSave)
	}
	if got := operationField(fin, "outcome"); got != outcome {
		t.Errorf("finished outcome = %q, want %q", got, outcome)
	}
	if got := operationField(fin, "stage"); got != stage {
		t.Errorf("finished stage = %q, want %q", got, stage)
	}
	for _, forbidden := range []string{"before", "after", "field", "save_file", "path"} {
		if got := operationField(fin, forbidden); got != "" {
			t.Errorf("finished carries %s=%q", forbidden, got)
		}
	}
}

// sensitiveSavePath builds an alphabetic, deliberately private-looking source path
// inside dir. Alphabetic-only so assertNoPathFragments can ignore digit-only
// timestamp fragments while still catching every real token.
func sensitiveSavePath(t *testing.T, dir string) string {
	t.Helper()
	secretDir := filepath.Join(dir, secretDirName)
	if err := os.MkdirAll(secretDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	return filepath.Join(secretDir, secretFileName)
}

func TestBackupCurrentSaveDebugSuccess(t *testing.T) {
	dir := t.TempDir()
	path := sensitiveSavePath(t, dir)
	const content = "eldenring-save-payload-bytes"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile source: %v", err)
	}
	app := backupApp(t, true, &core.SaveFile{}, path)

	backupPath, err := app.BackupCurrentSave()
	if err != nil {
		t.Fatalf("BackupCurrentSave: %v", err)
	}
	got, err := os.ReadFile(backupPath)
	if err != nil {
		t.Fatalf("read backup: %v", err)
	}
	if string(got) != content {
		t.Errorf("backup content = %q, want %q", got, content)
	}
	if !strings.HasSuffix(backupPath, ".bak") {
		t.Errorf("backup path %q missing .bak suffix", backupPath)
	}

	tail := app.journal.Tail()
	assertBackupOperationRequested(t, tail)
	assertBackupOperationFinished(t, tail, string(characterChangeSuccess), toolsStageCompleted)
	assertNoFieldRecords(t, tail)

	// Order: requested strictly before finished.
	reqIx := eventIndex(t, tail, eventToolsOperationRequested)
	finIx := eventIndex(t, tail, eventToolsOperationFinished)
	if reqIx >= finIx {
		t.Errorf("requested (%d) not before finished (%d)", reqIx, finIx)
	}

	// Serialize ONLY the operation records and prove no path survives: neither the
	// full source/backup path, nor any fragment of the sensitive dir/file tokens.
	assertNoPathLeak(t, operationRecords(tail), []string{path, backupPath}, secretDirName, secretFileName, filepath.Base(backupPath))
}

func TestBackupCurrentSaveDebugNoActiveSave(t *testing.T) {
	app := backupApp(t, true, nil, "")

	_, err := app.BackupCurrentSave()
	if err == nil || err.Error() != "no save loaded" {
		t.Fatalf("error = %v, want \"no save loaded\"", err)
	}

	tail := app.journal.Tail()
	ops := operationRecords(tail)
	assertBackupOperationRequested(t, ops)
	assertBackupOperationFinished(t, ops, string(characterChangeError), toolsStageNoActiveSave)
	assertNoFieldRecords(t, tail)
	assertNoPathLeak(t, ops, nil)
}

func TestBackupCurrentSaveDebugSourceMissing(t *testing.T) {
	dir := t.TempDir()
	// a.save present, lastSavePath points at a file that does not exist.
	path := sensitiveSavePath(t, dir) // creates the dir, not the file
	app := backupApp(t, true, &core.SaveFile{}, path)

	_, err := app.BackupCurrentSave()
	if err == nil || !strings.Contains(err.Error(), "save file not found") {
		t.Fatalf("error = %v, want it to wrap \"save file not found\"", err)
	}

	tail := app.journal.Tail()
	ops := operationRecords(tail)
	assertBackupOperationRequested(t, ops)
	assertBackupOperationFinished(t, ops, string(characterChangeError), toolsStageSourceMissing)
	assertNoFieldRecords(t, tail)
	assertNoPathLeak(t, ops, []string{path}, secretDirName, secretFileName)
}

func TestBackupCurrentSaveDebugCreateBackupError(t *testing.T) {
	dir := t.TempDir()
	// Deterministic CreateBackup failure: the source is a directory, so os.Stat
	// passes but core.CreateBackup's io.Copy read of a directory fd errors.
	path := filepath.Join(dir, secretSourceDir)
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	app := backupApp(t, true, &core.SaveFile{}, path)

	_, err := app.BackupCurrentSave()
	if err == nil {
		t.Fatalf("BackupCurrentSave error = nil, want a CreateBackup failure")
	}

	tail := app.journal.Tail()
	ops := operationRecords(tail)
	assertBackupOperationRequested(t, ops)
	assertBackupOperationFinished(t, ops, string(characterChangeError), toolsStageCreateBackup)
	assertNoFieldRecords(t, tail)
	assertNoPathLeak(t, ops, []string{path}, secretSourceDir)
}

func TestBackupCurrentSaveDebugOff(t *testing.T) {
	dir := t.TempDir()
	path := sensitiveSavePath(t, dir)
	if err := os.WriteFile(path, []byte("payload"), 0o644); err != nil {
		t.Fatalf("WriteFile source: %v", err)
	}
	app := backupApp(t, false, &core.SaveFile{}, path)

	backupPath, err := app.BackupCurrentSave()
	if err != nil {
		t.Fatalf("BackupCurrentSave: %v", err)
	}
	if _, err := os.Stat(backupPath); err != nil {
		t.Fatalf("backup not created with debug off: %v", err)
	}

	// Only the backup operation family must be absent; the always-on save_backup_*
	// events remain, so the whole journal is deliberately not asserted empty.
	tail := app.journal.Tail()
	for _, ev := range []string{eventToolsOperationRequested, eventToolsOperationFinished} {
		if n := countEvent(tail, ev); n != 0 {
			t.Errorf("debug-off %s = %d, want 0", ev, n)
		}
	}
}
