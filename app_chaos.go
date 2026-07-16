package main

import (
	"fmt"
	"os"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
)

// BackupCurrentSave creates a timestamped .bak copy of the currently loaded
// save file. Used by Chaos Mode's opt-in autobackup: Chaos edits are
// irreversible in-place, so a restore point is the only recovery path.
// Returns the backup path. Fails if no save is loaded or the file is gone.
func (a *App) BackupCurrentSave() (string, error) {
	a.journalLog(levelInfo, "save_backup_requested", "manual save backup requested")
	a.saveMu.RLock()
	path := a.lastSavePath
	loaded := a.save != nil
	a.saveMu.RUnlock()

	if !loaded || path == "" {
		a.journalLog(levelError, "save_backup_failed", "manual save backup failed", field("stage", "no_active_save"))
		return "", fmt.Errorf("no save loaded")
	}
	if _, err := os.Stat(path); err != nil {
		a.journalLog(levelError, "save_backup_failed", "manual save backup failed", field("stage", "source_missing"))
		return "", fmt.Errorf("save file not found: %w", err)
	}

	backupPath, err := core.CreateBackup(path)
	if err != nil {
		a.journalLog(levelError, "save_backup_failed", "manual save backup failed", field("stage", "create"))
		return "", err
	}
	if err := core.PruneBackups(path, 10); err != nil {
		fmt.Printf("Warning: failed to prune old backups: %v\n", err)
	}
	fields := []diagnosticField{field("outcome", "success")}
	if name := safeSaveFileName(path); name != "" {
		fields = append(fields, field("save_file", name))
	}
	a.journalLog(levelInfo, "save_backup_finished", "manual save backup finished", fields...)
	return backupPath, nil
}
