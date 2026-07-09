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
	a.saveMu.RLock()
	path := a.lastSavePath
	loaded := a.save != nil
	a.saveMu.RUnlock()

	if !loaded || path == "" {
		return "", fmt.Errorf("no save loaded")
	}
	if _, err := os.Stat(path); err != nil {
		return "", fmt.Errorf("save file not found: %w", err)
	}

	backupPath, err := core.CreateBackup(path)
	if err != nil {
		return "", err
	}
	if err := core.PruneBackups(path, 10); err != nil {
		fmt.Printf("Warning: failed to prune old backups: %v\n", err)
	}
	return backupPath, nil
}
