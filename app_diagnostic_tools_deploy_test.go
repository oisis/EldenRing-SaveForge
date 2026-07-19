package main

import (
	"path/filepath"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/deploy"
)

const (
	deployPrivateTarget = "CinderNodeVesper"
	deployPrivateHost   = "HiddenHostNebula"
	deployPrivateUser   = "SecretUserAurora"
	deployPrivateKey    = "PrivateKeyComet.pem"
	deployPrivateStart  = "ignite-cinder-run"
	deployPrivateStop   = "halt-cinder-run"
	deployPrivatePath   = "CinderVaultSaveOrion.sl2"
)

func deployOperationRecords(records []diagnosticRecord, action string) []diagnosticRecord {
	return saveManagerOperationRecords(records, action)
}

func assertDeployOperation(t *testing.T, records []diagnosticRecord, action, outcome, stage string) {
	t.Helper()
	ops := deployOperationRecords(records, action)
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

func privateDeployTarget(dir string) deploy.Target {
	return deploy.Target{
		Type:         deploy.TargetTypeLocal,
		Name:         deployPrivateTarget,
		Host:         deployPrivateHost,
		User:         deployPrivateUser,
		KeyPath:      filepath.Join(dir, deployPrivateKey),
		SavePath:     filepath.Join(dir, deployPrivatePath),
		GameStartCmd: deployPrivateStart,
		GameStopCmd:  deployPrivateStop,
	}
}

func assertDeployPrivate(t *testing.T, records []diagnosticRecord, action string, target deploy.Target, extra ...string) {
	t.Helper()
	secrets := []string{target.Name, target.Host, target.User, target.KeyPath, target.SavePath, target.GameStartCmd, target.GameStopCmd}
	secrets = append(secrets, extra...)
	assertSaveManagerPrivate(t, records, action, secrets...)
}

func TestToolsDeployConfigurationFailuresLifecycle(t *testing.T) {
	app := steamIDApp(t, true, nil)
	target := privateDeployTarget(t.TempDir())

	tests := []struct {
		action string
		call   func() error
	}{
		{actionToolsSaveDeployTarget, func() error { return app.SaveDeployTarget(target) }},
		{actionToolsDeleteDeployTarget, func() error { return app.DeleteDeployTarget(target.Name) }},
		{actionToolsTestDeployConnection, func() error { _, err := app.TestSSHConnection(target.Name); return err }},
		{actionToolsDeploySave, func() error { _, err := app.DeploySave(target.Name); return err }},
		{actionToolsDownloadRemoteSave, func() error { _, err := app.DownloadRemoteSave(target.Name); return err }},
		{actionToolsLaunchRemoteGame, func() error { _, err := app.LaunchRemoteGame(target.Name); return err }},
		{actionToolsCloseRemoteGame, func() error { _, err := app.CloseRemoteGame(target.Name); return err }},
		{actionToolsDeployAndLaunch, func() error { return app.DeployAndLaunch(target.Name) }},
		{actionToolsCloseAndDownload, func() error { _, err := app.CloseAndDownload(target.Name); return err }},
	}

	for _, test := range tests {
		if err := test.call(); err == nil {
			t.Errorf("%s unexpectedly succeeded", test.action)
		}
	}
	for _, test := range tests {
		assertDeployOperation(t, app.journal.Tail(), test.action, string(characterChangeError), toolsStageConfiguration)
		assertDeployPrivate(t, app.journal.Tail(), test.action, target)
	}
}

func TestToolsDeployTargetMutationsAndConnectionLifecycle(t *testing.T) {
	app, _, dir := saveManagerTestApp(t, true)
	target := privateDeployTarget(dir)

	if err := app.SaveDeployTarget(target); err != nil {
		t.Fatalf("SaveDeployTarget: %v", err)
	}
	if _, ok := app.deployStore.Get(target.Name); !ok {
		t.Fatal("SaveDeployTarget did not persist target")
	}
	assertDeployOperation(t, app.journal.Tail(), actionToolsSaveDeployTarget, string(characterChangeSuccess), toolsStageCompleted)
	assertDeployPrivate(t, app.journal.Tail(), actionToolsSaveDeployTarget, target)

	message, err := app.TestSSHConnection(target.Name)
	if err != nil {
		t.Fatalf("TestSSHConnection: %v", err)
	}
	if message == "" {
		t.Fatal("TestSSHConnection returned an empty success message")
	}
	assertDeployOperation(t, app.journal.Tail(), actionToolsTestDeployConnection, string(characterChangeSuccess), toolsStageCompleted)
	assertDeployPrivate(t, app.journal.Tail(), actionToolsTestDeployConnection, target, message)

	if err := app.DeleteDeployTarget(target.Name); err != nil {
		t.Fatalf("DeleteDeployTarget: %v", err)
	}
	if _, ok := app.deployStore.Get(target.Name); ok {
		t.Fatal("DeleteDeployTarget left target configured")
	}
	assertDeployOperation(t, app.journal.Tail(), actionToolsDeleteDeployTarget, string(characterChangeSuccess), toolsStageCompleted)
	assertDeployPrivate(t, app.journal.Tail(), actionToolsDeleteDeployTarget, target)
}

func TestToolsDeployNoActiveSaveLifecycle(t *testing.T) {
	app, target, _ := saveManagerTestApp(t, true)

	if _, err := app.DeploySave(target.Name); err == nil {
		t.Fatal("DeploySave unexpectedly succeeded without an active save")
	}
	if err := app.DeployAndLaunch(target.Name); err == nil {
		t.Fatal("DeployAndLaunch unexpectedly succeeded without an active save")
	}
	assertDeployOperation(t, app.journal.Tail(), actionToolsDeploySave, string(characterChangeError), toolsStageNoActiveSave)
	assertDeployOperation(t, app.journal.Tail(), actionToolsDeployAndLaunch, string(characterChangeError), toolsStageNoActiveSave)
	assertDeployPrivate(t, app.journal.Tail(), actionToolsDeploySave, target)
	assertDeployPrivate(t, app.journal.Tail(), actionToolsDeployAndLaunch, target)
}

func TestToolsLoadSaveFromPathParseLifecycle(t *testing.T) {
	app := steamIDApp(t, true, nil)
	privatePath := filepath.Join(t.TempDir(), "PrivateLocalLoadOrion.sl2")

	if _, err := app.LoadSaveFromPath(privatePath); err == nil {
		t.Fatal("LoadSaveFromPath unexpectedly parsed a missing file")
	}
	assertDeployOperation(t, app.journal.Tail(), actionToolsLoadSaveFromPath, string(characterChangeError), toolsStageParse)
	assertSaveManagerPrivate(t, app.journal.Tail(), actionToolsLoadSaveFromPath, privatePath)
}

func TestToolsDeployDebugOffEmitsNothing(t *testing.T) {
	app, _, dir := saveManagerTestApp(t, false)
	target := privateDeployTarget(dir)

	if err := app.SaveDeployTarget(target); err != nil {
		t.Fatalf("SaveDeployTarget: %v", err)
	}
	if _, err := app.TestSSHConnection(target.Name); err != nil {
		t.Fatalf("TestSSHConnection: %v", err)
	}
	if err := app.DeleteDeployTarget(target.Name); err != nil {
		t.Fatalf("DeleteDeployTarget: %v", err)
	}
	for _, record := range app.journal.Tail() {
		if record.Event == eventToolsOperationRequested || record.Event == eventToolsOperationFinished ||
			record.Event == eventToolsChangeBefore || record.Event == eventToolsChangePlanned || record.Event == eventToolsChangeFinished {
			t.Fatalf("Debug-off operation emitted diagnostic event %q", record.Event)
		}
	}
}
