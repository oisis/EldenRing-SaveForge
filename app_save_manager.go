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
	if a.deployStore == nil {
		return nil, fmt.Errorf("deploy not initialized")
	}
	if a.isLocalTarget(targetName) {
		return a.deployLocal.ListBackups(targetName)
	}
	return a.deploySSH.ListBackups(targetName)
}

// SetActiveBackup copies the named .bak file over ER0000.sl2 on the target.
func (a *App) SetActiveBackup(targetName, backupName string) error {
	if a.deployStore == nil {
		return fmt.Errorf("deploy not initialized")
	}
	if a.isLocalTarget(targetName) {
		return a.deployLocal.SetActiveBackup(targetName, backupName)
	}
	return a.deploySSH.SetActiveBackup(targetName, backupName)
}

// UnsetActiveBackup deletes ER0000.sl2 from the target.
// If no .bak with a matching md5 exists, the active file is first moved to a new .bak.
func (a *App) UnsetActiveBackup(targetName string) error {
	if a.deployStore == nil {
		return fmt.Errorf("deploy not initialized")
	}
	if a.isLocalTarget(targetName) {
		return a.deployLocal.UnsetActiveBackup(targetName)
	}
	return a.deploySSH.UnsetActiveBackup(targetName)
}

// DeleteSaveBackup deletes a .bak file from the target.
// Returns an error if the backup is currently active.
func (a *App) DeleteSaveBackup(targetName, backupName string) error {
	if a.deployStore == nil {
		return fmt.Errorf("deploy not initialized")
	}
	if a.isLocalTarget(targetName) {
		return a.deployLocal.DeleteSaveBackup(targetName, backupName)
	}
	return a.deploySSH.DeleteSaveBackup(targetName, backupName)
}

// CreateManualBackup copies ER0000.sl2 on the target to a new timestamped .bak file.
func (a *App) CreateManualBackup(targetName string) error {
	if a.deployStore == nil {
		return fmt.Errorf("deploy not initialized")
	}
	if a.isLocalTarget(targetName) {
		return a.deployLocal.CreateManualBackup(targetName)
	}
	return a.deploySSH.CreateManualBackup(targetName)
}

// UpdateBackupMeta rewrites the .json sidecar for a backup with new tags and description.
func (a *App) UpdateBackupMeta(targetName, backupName string, tags []string, desc string) error {
	if a.deployStore == nil {
		return fmt.Errorf("deploy not initialized")
	}
	if a.isLocalTarget(targetName) {
		return a.deployLocal.UpdateBackupMeta(targetName, backupName, tags, desc)
	}
	return a.deploySSH.UpdateBackupMeta(targetName, backupName, tags, desc)
}

// DownloadBackupFile opens a save-file dialog then downloads the named .bak to the chosen path.
// Returns the chosen local path (empty string if the dialog was cancelled).
func (a *App) DownloadBackupFile(targetName, backupName string) (string, error) {
	if a.deployStore == nil {
		return "", fmt.Errorf("deploy not initialized")
	}

	base := strings.TrimSuffix(backupName, ".bak")
	defaultName := base
	if !strings.HasSuffix(base, ".sl2") {
		defaultName = base + ".sl2"
	}
	localPath, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "Save Backup As",
		DefaultFilename: defaultName,
		Filters: []runtime.FileFilter{
			{DisplayName: "Elden Ring Save (*.sl2)", Pattern: "*.sl2"},
			{DisplayName: "All Files (*.*)", Pattern: "*.*"},
		},
	})
	if err != nil {
		return "", err
	}
	if localPath == "" {
		return "", nil
	}

	if a.isLocalTarget(targetName) {
		return localPath, a.deployLocal.DownloadBackup(targetName, backupName, localPath)
	}
	return localPath, a.deploySSH.DownloadBackup(targetName, backupName, localPath)
}

// LoadSaveFromPath loads a save file from a local path without opening a file dialog.
// Passing "" as localPath after the load means WriteSave will require the user to choose
// a destination (same behaviour as DownloadRemoteSave).
func (a *App) LoadSaveFromPath(localPath string) (string, error) {
	save, err := core.LoadSave(localPath)
	if err != nil {
		return "", fmt.Errorf("invalid save file: %w", err)
	}
	a.commitLoadedSave(save, "", loadOriginLocalPath)
	return string(save.Platform), nil
}
