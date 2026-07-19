package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/deploy"
)

const (
	saveManagerTargetName = "PrivateTargetOrion"
	saveManagerBackupName = "VaultNebulaCopy.bak"
	saveManagerTag        = "PrivateTagAurora"
	saveManagerDesc       = "AuroraMemoBorealis"
)

// saveManagerTestApp creates a local-only deploy target inside a temporary HOME
// and filesystem. It never reads a user's target configuration, real save or SSH
// credentials. The target and every filename deliberately look private so the
// operation-record leak checks have meaningful values to reject.
func saveManagerTestApp(t *testing.T, debug bool) (*App, deploy.Target, string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "config"))
	store, err := deploy.NewTargetStore()
	if err != nil {
		t.Fatalf("NewTargetStore: %v", err)
	}
	dir := t.TempDir()
	savePath := filepath.Join(dir, "PrivateLiveSaveOmega.sl2")
	if err := os.WriteFile(savePath, []byte("private-active-save"), 0o600); err != nil {
		t.Fatalf("write active save: %v", err)
	}
	target := deploy.Target{Type: deploy.TargetTypeLocal, Name: saveManagerTargetName, SavePath: savePath}
	if err := store.Save(target); err != nil {
		t.Fatalf("store.Save: %v", err)
	}
	journal, err := newDiagnosticJournalInDir(t.TempDir())
	if err != nil {
		t.Fatalf("newDiagnosticJournalInDir: %v", err)
	}
	journal.SetDebugEnabled(debug)
	t.Cleanup(func() { _ = journal.Close() })
	app := NewApp()
	app.journal = journal
	app.deployStore = store
	app.deployLocal = deploy.NewLocalManager(store)
	app.deploySSH = deploy.NewSSHManager(store)
	return app, target, dir
}

func seedSaveManagerBackup(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("seed backup %q: %v", name, err)
	}
	return path
}

func saveManagerOperationRecords(records []diagnosticRecord, action string) []diagnosticRecord {
	var out []diagnosticRecord
	for _, record := range records {
		if (record.Event == eventToolsOperationRequested || record.Event == eventToolsOperationFinished) &&
			operationField(record, "action") == action {
			out = append(out, record)
		}
	}
	return out
}

func saveManagerChangeCount(records []diagnosticRecord, action string) int {
	n := 0
	for _, record := range records {
		switch record.Event {
		case eventToolsChangeBefore, eventToolsChangePlanned, eventToolsChangeFinished:
			if operationField(record, "action") == action {
				n++
			}
		}
	}
	return n
}

func assertSaveManagerOperation(t *testing.T, records []diagnosticRecord, action, outcome, stage string) {
	t.Helper()
	ops := saveManagerOperationRecords(records, action)
	if len(ops) != 2 {
		t.Fatalf("%s operation records = %d, want requested + finished", action, len(ops))
	}
	if ops[0].Event != eventToolsOperationRequested || ops[1].Event != eventToolsOperationFinished {
		t.Fatalf("%s event order = %q, %q", action, ops[0].Event, ops[1].Event)
	}
	assertFieldKeys(t, action+" requested", ops[0], "action")
	assertFieldKeys(t, action+" finished", ops[1], "action", "outcome", "stage")
	if got := operationField(ops[1], "outcome"); got != outcome {
		t.Errorf("%s outcome = %q, want %q", action, got, outcome)
	}
	if got := operationField(ops[1], "stage"); got != stage {
		t.Errorf("%s stage = %q, want %q", action, got, stage)
	}
	if n := saveManagerChangeCount(records, action); n != 0 {
		t.Errorf("%s emitted %d tools_change records, want 0", action, n)
	}
}

func assertSaveManagerPrivate(t *testing.T, records []diagnosticRecord, action string, secrets ...string) {
	t.Helper()
	blob, err := json.Marshal(saveManagerOperationRecords(records, action))
	if err != nil {
		t.Fatalf("marshal %s records: %v", action, err)
	}
	haystack := string(blob)
	for _, secret := range secrets {
		if secret == "" {
			continue
		}
		if strings.Contains(haystack, secret) {
			t.Errorf("%s operation records leaked %q", action, secret)
		}
		// TempDir embeds the test function name, which legitimately overlaps with
		// closed action/stage vocabulary (for example "backup" or "download").
		// A complete path is still checked above; fragments are deliberately taken
		// only from its private basename.
		fragmentSource := secret
		if base := filepath.Base(secret); base != secret {
			fragmentSource = base
		}
		for start := 0; start+5 <= len(fragmentSource); start++ {
			for end := start + 5; end <= len(fragmentSource); end++ {
				if strings.Contains(haystack, fragmentSource[start:end]) {
					t.Errorf("%s operation records leaked fragment %q", action, fragmentSource[start:end])
				}
			}
		}
	}
}

func TestToolsSaveManagerListBackupsLifecycle(t *testing.T) {
	app, target, dir := saveManagerTestApp(t, true)
	backupPath := seedSaveManagerBackup(t, dir, saveManagerBackupName, "private-backup")

	entries, err := app.ListSaveBackups(target.Name)
	if err != nil {
		t.Fatalf("ListSaveBackups: %v", err)
	}
	if len(entries) != 1 || entries[0].Name != saveManagerBackupName {
		t.Fatalf("entries = %+v, want private backup", entries)
	}
	tail := app.journal.Tail()
	assertSaveManagerOperation(t, tail, actionToolsListSaveBackups, string(characterChangeSuccess), toolsStageCompleted)
	assertSaveManagerPrivate(t, tail, actionToolsListSaveBackups, target.Name, backupPath, saveManagerBackupName)
}

func TestToolsSaveManagerBackupMutationsLifecycle(t *testing.T) {
	app, target, dir := saveManagerTestApp(t, true)
	deletablePath := seedSaveManagerBackup(t, dir, saveManagerBackupName, "deletable-backup")

	if err := app.CreateManualBackup(target.Name); err != nil {
		t.Fatalf("CreateManualBackup: %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	activeName := ""
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".bak") && entry.Name() != saveManagerBackupName {
			activeName = entry.Name()
			break
		}
	}
	if activeName == "" {
		t.Fatal("CreateManualBackup did not create a new .bak")
	}
	if err := app.SetActiveBackup(target.Name, activeName); err != nil {
		t.Fatalf("SetActiveBackup: %v", err)
	}
	marker, err := os.ReadFile(filepath.Join(dir, ".active_backup"))
	if err != nil || strings.TrimSpace(string(marker)) != activeName {
		t.Fatalf("active marker = %q, err=%v, want %q", marker, err, activeName)
	}
	if err := app.UnsetActiveBackup(target.Name); err != nil {
		t.Fatalf("UnsetActiveBackup: %v", err)
	}
	if _, err := os.Stat(target.SavePath); !os.IsNotExist(err) {
		t.Fatalf("active save still exists after unset: %v", err)
	}
	if err := app.DeleteSaveBackup(target.Name, saveManagerBackupName); err != nil {
		t.Fatalf("DeleteSaveBackup: %v", err)
	}
	if _, err := os.Stat(deletablePath); !os.IsNotExist(err) {
		t.Fatalf("deletable backup still exists: %v", err)
	}

	tail := app.journal.Tail()
	for _, action := range []string{actionToolsCreateManualBackup, actionToolsSetActiveBackup, actionToolsUnsetActiveBackup, actionToolsDeleteSaveBackup} {
		assertSaveManagerOperation(t, tail, action, string(characterChangeSuccess), toolsStageCompleted)
		assertSaveManagerPrivate(t, tail, action, target.Name, activeName, saveManagerBackupName, target.SavePath)
	}
}

func TestToolsSaveManagerUpdateMetaLifecycle(t *testing.T) {
	app, target, dir := saveManagerTestApp(t, true)
	backupPath := seedSaveManagerBackup(t, dir, saveManagerBackupName, "metadata-backup")

	if err := app.UpdateBackupMeta(target.Name, saveManagerBackupName, []string{saveManagerTag}, saveManagerDesc); err != nil {
		t.Fatalf("UpdateBackupMeta: %v", err)
	}
	meta, err := os.ReadFile(backupPath + ".json")
	if err != nil {
		t.Fatalf("read metadata: %v", err)
	}
	if !strings.Contains(string(meta), saveManagerTag) || !strings.Contains(string(meta), saveManagerDesc) {
		t.Fatalf("metadata did not contain supplied private values: %s", meta)
	}
	tail := app.journal.Tail()
	assertSaveManagerOperation(t, tail, actionToolsUpdateBackupMeta, string(characterChangeSuccess), toolsStageCompleted)
	assertSaveManagerPrivate(t, tail, actionToolsUpdateBackupMeta, target.Name, saveManagerBackupName, saveManagerTag, saveManagerDesc, backupPath)
}

func TestToolsSaveManagerDownloadLifecycle(t *testing.T) {
	cases := []struct {
		name     string
		backup   string
		choose   func(string) func() (string, error)
		outcome  characterChangeOutcome
		stage    string
		wantErr  bool
		wantFile bool
	}{
		{
			name:     "success",
			backup:   saveManagerBackupName,
			choose:   func(dest string) func() (string, error) { return func() (string, error) { return dest, nil } },
			outcome:  characterChangeSuccess,
			stage:    toolsStageCompleted,
			wantFile: true,
		},
		{
			name:    "cancelled",
			backup:  saveManagerBackupName,
			choose:  func(string) func() (string, error) { return func() (string, error) { return "", nil } },
			outcome: characterChangeSuccess,
			stage:   toolsStageCancelled,
		},
		{
			name:   "dialog_error",
			backup: saveManagerBackupName,
			choose: func(string) func() (string, error) {
				return func() (string, error) { return "", errors.New("NebulaFaultQuasar") }
			},
			outcome: characterChangeError,
			stage:   toolsStageDialog,
			wantErr: true,
		},
		{
			name:    "download_error",
			backup:  "VaultMissingNebula.bak",
			choose:  func(dest string) func() (string, error) { return func() (string, error) { return dest, nil } },
			outcome: characterChangeError,
			stage:   toolsStageDownloadBackup,
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app, target, dir := saveManagerTestApp(t, true)
			seedSaveManagerBackup(t, dir, saveManagerBackupName, "downloadable-backup")
			dest := filepath.Join(t.TempDir(), "VaultOutputNebula.sl2")
			path, err := app.downloadBackupFile(target.Name, tc.backup, tc.choose(dest))
			if (err != nil) != tc.wantErr {
				t.Fatalf("downloadBackupFile error = %v, wantErr=%v", err, tc.wantErr)
			}
			if tc.name == "success" && path != dest {
				t.Errorf("success path = %q, want %q", path, dest)
			}
			if tc.wantFile {
				if data, readErr := os.ReadFile(dest); readErr != nil || string(data) != "downloadable-backup" {
					t.Fatalf("downloaded data=%q err=%v", data, readErr)
				}
			}
			tail := app.journal.Tail()
			assertSaveManagerOperation(t, tail, actionToolsDownloadBackupFile, string(tc.outcome), tc.stage)
			assertSaveManagerPrivate(t, tail, actionToolsDownloadBackupFile, target.Name, tc.backup, dest, target.SavePath, "NebulaFaultQuasar")
		})
	}
}

func TestToolsSaveManagerConfigurationErrors(t *testing.T) {
	app := NewApp()
	enableDebugJournal(t, app)
	if _, err := app.ListSaveBackups(saveManagerTargetName); err == nil || err.Error() != "deploy not initialized" {
		t.Fatalf("ListSaveBackups error = %v", err)
	}
	if err := app.CreateManualBackup(saveManagerTargetName); err == nil || err.Error() != "deploy not initialized" {
		t.Fatalf("CreateManualBackup error = %v", err)
	}
	tail := app.journal.Tail()
	for _, action := range []string{actionToolsListSaveBackups, actionToolsCreateManualBackup} {
		assertSaveManagerOperation(t, tail, action, string(characterChangeError), toolsStageConfiguration)
		assertSaveManagerPrivate(t, tail, action, saveManagerTargetName, "deploy not initialized")
	}
}

func TestToolsSaveManagerDebugOffEmitsNothing(t *testing.T) {
	app, target, dir := saveManagerTestApp(t, false)
	backupPath := seedSaveManagerBackup(t, dir, saveManagerBackupName, "debug-off-delete")
	if err := app.CreateManualBackup(target.Name); err != nil {
		t.Fatalf("CreateManualBackup: %v", err)
	}
	if err := app.DeleteSaveBackup(target.Name, saveManagerBackupName); err != nil {
		t.Fatalf("DeleteSaveBackup: %v", err)
	}
	if _, err := os.Stat(backupPath); !os.IsNotExist(err) {
		t.Fatalf("DeleteSaveBackup did not delete backup: %v", err)
	}
	tail := app.journal.Tail()
	for _, action := range []string{actionToolsCreateManualBackup, actionToolsDeleteSaveBackup} {
		if got := len(saveManagerOperationRecords(tail, action)); got != 0 {
			t.Errorf("debug-off %s operation records = %d, want 0", action, got)
		}
	}
}
