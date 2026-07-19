package main

import (
	"fmt"
	"strings"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
	"github.com/oisis/EldenRing-SaveForge/backend/deploy"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// ListSaveBackups returns all .bak backups from the named deploy target.
func (a *App) ListSaveBackups(targetName string) ([]deploy.SaveBackupEntry, error) {
	a.journalToolsOperationRequested(actionToolsListSaveBackups)
	if a.deployStore == nil {
		a.journalToolsOperationFinished(actionToolsListSaveBackups, characterChangeError, toolsStageConfiguration)
		return nil, fmt.Errorf("deploy not initialized")
	}
	var backups []deploy.SaveBackupEntry
	var err error
	if a.isLocalTarget(targetName) {
		backups, err = a.deployLocal.ListBackups(targetName)
	} else {
		backups, err = a.deploySSH.ListBackups(targetName)
	}
	if err != nil {
		a.journalToolsOperationFinished(actionToolsListSaveBackups, characterChangeError, toolsStageListBackups)
		return nil, err
	}
	a.journalToolsOperationFinished(actionToolsListSaveBackups, characterChangeSuccess, toolsStageCompleted)
	return backups, nil
}

// SetActiveBackup copies the named .bak file over ER0000.sl2 on the target.
func (a *App) SetActiveBackup(targetName, backupName string) error {
	a.journalToolsOperationRequested(actionToolsSetActiveBackup)
	if a.deployStore == nil {
		a.journalToolsOperationFinished(actionToolsSetActiveBackup, characterChangeError, toolsStageConfiguration)
		return fmt.Errorf("deploy not initialized")
	}
	var err error
	if a.isLocalTarget(targetName) {
		err = a.deployLocal.SetActiveBackup(targetName, backupName)
	} else {
		err = a.deploySSH.SetActiveBackup(targetName, backupName)
	}
	if err != nil {
		a.journalToolsOperationFinished(actionToolsSetActiveBackup, characterChangeError, toolsStageSetActiveBackup)
		return err
	}
	a.journalToolsOperationFinished(actionToolsSetActiveBackup, characterChangeSuccess, toolsStageCompleted)
	return nil
}

// UnsetActiveBackup deletes ER0000.sl2 from the target.
// If no .bak with a matching md5 exists, the active file is first moved to a new .bak.
func (a *App) UnsetActiveBackup(targetName string) error {
	a.journalToolsOperationRequested(actionToolsUnsetActiveBackup)
	if a.deployStore == nil {
		a.journalToolsOperationFinished(actionToolsUnsetActiveBackup, characterChangeError, toolsStageConfiguration)
		return fmt.Errorf("deploy not initialized")
	}
	var err error
	if a.isLocalTarget(targetName) {
		err = a.deployLocal.UnsetActiveBackup(targetName)
	} else {
		err = a.deploySSH.UnsetActiveBackup(targetName)
	}
	if err != nil {
		a.journalToolsOperationFinished(actionToolsUnsetActiveBackup, characterChangeError, toolsStageUnsetActiveBackup)
		return err
	}
	a.journalToolsOperationFinished(actionToolsUnsetActiveBackup, characterChangeSuccess, toolsStageCompleted)
	return nil
}

// DeleteSaveBackup deletes a .bak file from the target.
// Returns an error if the backup is currently active.
func (a *App) DeleteSaveBackup(targetName, backupName string) error {
	a.journalToolsOperationRequested(actionToolsDeleteSaveBackup)
	if a.deployStore == nil {
		a.journalToolsOperationFinished(actionToolsDeleteSaveBackup, characterChangeError, toolsStageConfiguration)
		return fmt.Errorf("deploy not initialized")
	}
	var err error
	if a.isLocalTarget(targetName) {
		err = a.deployLocal.DeleteSaveBackup(targetName, backupName)
	} else {
		err = a.deploySSH.DeleteSaveBackup(targetName, backupName)
	}
	if err != nil {
		a.journalToolsOperationFinished(actionToolsDeleteSaveBackup, characterChangeError, toolsStageDeleteBackup)
		return err
	}
	a.journalToolsOperationFinished(actionToolsDeleteSaveBackup, characterChangeSuccess, toolsStageCompleted)
	return nil
}

// CreateManualBackup copies ER0000.sl2 on the target to a new timestamped .bak file.
func (a *App) CreateManualBackup(targetName string) error {
	a.journalToolsOperationRequested(actionToolsCreateManualBackup)
	if a.deployStore == nil {
		a.journalToolsOperationFinished(actionToolsCreateManualBackup, characterChangeError, toolsStageConfiguration)
		return fmt.Errorf("deploy not initialized")
	}
	var err error
	if a.isLocalTarget(targetName) {
		err = a.deployLocal.CreateManualBackup(targetName)
	} else {
		err = a.deploySSH.CreateManualBackup(targetName)
	}
	if err != nil {
		a.journalToolsOperationFinished(actionToolsCreateManualBackup, characterChangeError, toolsStageCreateManualBackup)
		return err
	}
	a.journalToolsOperationFinished(actionToolsCreateManualBackup, characterChangeSuccess, toolsStageCompleted)
	return nil
}

// UpdateBackupMeta rewrites the .json sidecar for a backup with new tags and description.
func (a *App) UpdateBackupMeta(targetName, backupName string, tags []string, desc string) error {
	a.journalToolsOperationRequested(actionToolsUpdateBackupMeta)
	if a.deployStore == nil {
		a.journalToolsOperationFinished(actionToolsUpdateBackupMeta, characterChangeError, toolsStageConfiguration)
		return fmt.Errorf("deploy not initialized")
	}
	var err error
	if a.isLocalTarget(targetName) {
		err = a.deployLocal.UpdateBackupMeta(targetName, backupName, tags, desc)
	} else {
		err = a.deploySSH.UpdateBackupMeta(targetName, backupName, tags, desc)
	}
	if err != nil {
		a.journalToolsOperationFinished(actionToolsUpdateBackupMeta, characterChangeError, toolsStageUpdateBackupMeta)
		return err
	}
	a.journalToolsOperationFinished(actionToolsUpdateBackupMeta, characterChangeSuccess, toolsStageCompleted)
	return nil
}

// DownloadBackupFile opens a save-file dialog then downloads the named .bak to the chosen path.
// Returns the chosen local path (empty string if the dialog was cancelled).
func (a *App) DownloadBackupFile(targetName, backupName string) (string, error) {
	return a.downloadBackupFile(targetName, backupName, func() (string, error) {
		base := strings.TrimSuffix(backupName, ".bak")
		defaultName := base
		if !strings.HasSuffix(base, ".sl2") {
			defaultName = base + ".sl2"
		}
		return runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
			Title:           "Save Backup As",
			DefaultFilename: defaultName,
			Filters: []runtime.FileFilter{
				{DisplayName: "Elden Ring Save (*.sl2)", Pattern: "*.sl2"},
				{DisplayName: "All Files (*.*)", Pattern: "*.*"},
			},
		})
	})
}

// downloadBackupFile is the context-free driver behind DownloadBackupFile.
// chooseDestination returns an empty string when the user cancels. The operation
// lifecycle deliberately records only closed status metadata: target, backup and
// local destination names are never journalled.
func (a *App) downloadBackupFile(targetName, backupName string, chooseDestination func() (string, error)) (string, error) {
	a.journalToolsOperationRequested(actionToolsDownloadBackupFile)
	if a.deployStore == nil {
		a.journalToolsOperationFinished(actionToolsDownloadBackupFile, characterChangeError, toolsStageConfiguration)
		return "", fmt.Errorf("deploy not initialized")
	}
	localPath, err := chooseDestination()
	if err != nil {
		a.journalToolsOperationFinished(actionToolsDownloadBackupFile, characterChangeError, toolsStageDialog)
		return "", err
	}
	if localPath == "" {
		a.journalToolsOperationFinished(actionToolsDownloadBackupFile, characterChangeSuccess, toolsStageCancelled)
		return "", nil
	}

	var downloadErr error
	if a.isLocalTarget(targetName) {
		downloadErr = a.deployLocal.DownloadBackup(targetName, backupName, localPath)
	} else {
		downloadErr = a.deploySSH.DownloadBackup(targetName, backupName, localPath)
	}
	if downloadErr != nil {
		a.journalToolsOperationFinished(actionToolsDownloadBackupFile, characterChangeError, toolsStageDownloadBackup)
		return localPath, downloadErr
	}
	a.journalToolsOperationFinished(actionToolsDownloadBackupFile, characterChangeSuccess, toolsStageCompleted)
	return localPath, nil
}

// LoadSaveFromPath loads a save file from a local path without opening a file dialog.
// Passing "" as localPath after the load means WriteSave will require the user to choose
// a destination (same behaviour as DownloadRemoteSave).
func (a *App) LoadSaveFromPath(localPath string) (string, error) {
	a.journalToolsOperationRequested(actionToolsLoadSaveFromPath)
	a.journalDebug("save_load_requested", "save load requested", field("origin", string(loadOriginLocalPath)))
	save, err := core.LoadSave(localPath)
	if err != nil {
		a.journalDebug("save_load_failed", "save load failed", field("origin", string(loadOriginLocalPath)), field("stage", "parse"))
		a.journalToolsOperationFinished(actionToolsLoadSaveFromPath, characterChangeError, toolsStageParse)
		return "", fmt.Errorf("invalid save file: %w", err)
	}
	a.journalDebug("save_load_parsed", "save load parsed", field("origin", string(loadOriginLocalPath)), field("platform", string(save.Platform)))
	a.commitLoadedSave(save, "", loadOriginLocalPath)
	a.journalToolsOperationFinished(actionToolsLoadSaveFromPath, characterChangeSuccess, toolsStageCompleted)
	return string(save.Platform), nil
}
