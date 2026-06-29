package deploy

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const activeMarkerFile = ".active_backup"

func localActiveMarkerPath(dir string) string {
	return filepath.Join(dir, activeMarkerFile)
}

// ListBackups returns all .bak files in the local target's save directory.
func (m *LocalManager) ListBackups(targetName string) ([]SaveBackupEntry, error) {
	t, ok := m.store.Get(targetName)
	if !ok {
		return nil, fmt.Errorf("target %q not found", targetName)
	}

	savePath := expandHome(t.SavePath)
	dir := filepath.Dir(savePath)

	// Read active-backup marker (written by SetActiveBackup / cleared by UnsetActiveBackup).
	activeName := ""
	if data, err := os.ReadFile(localActiveMarkerPath(dir)); err == nil {
		activeName = strings.TrimSpace(string(data))
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("cannot read directory: %w", err)
	}

	var result []SaveBackupEntry
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".bak") {
			continue
		}
		bakPath := filepath.Join(dir, entry.Name())
		info, _ := entry.Info()

		var meta BackupMeta
		if data, err := os.ReadFile(metaPath(bakPath)); err == nil {
			meta = readMeta(data)
		} else {
			meta.Tags = []string{}
		}

		ts := ""
		var size int64
		if info != nil {
			ts = info.ModTime().UTC().Format(time.RFC3339)
			size = info.Size()
		}

		result = append(result, SaveBackupEntry{
			Name:      entry.Name(),
			Timestamp: ts,
			Size:      size,
			MD5:       meta.MD5,
			Tags:      meta.Tags,
			Desc:      meta.Desc,
			IsActive:  activeName != "" && entry.Name() == activeName,
		})
	}
	return result, nil
}

// SetActiveBackup copies the named .bak file over ER0000.sl2 and writes the active marker.
func (m *LocalManager) SetActiveBackup(targetName, backupName string) error {
	t, ok := m.store.Get(targetName)
	if !ok {
		return fmt.Errorf("target %q not found", targetName)
	}
	savePath := expandHome(t.SavePath)
	dir := filepath.Dir(savePath)
	bakPath := filepath.Join(dir, backupName)
	if err := copyFile(bakPath, savePath); err != nil {
		return err
	}
	os.WriteFile(localActiveMarkerPath(dir), []byte(backupName), 0644) //nolint:errcheck
	return nil
}

// UnsetActiveBackup removes ER0000.sl2 from the local target.
// If no existing .bak has a matching md5, the active file is first moved to a new .bak.
func (m *LocalManager) UnsetActiveBackup(targetName string) error {
	t, ok := m.store.Get(targetName)
	if !ok {
		return fmt.Errorf("target %q not found", targetName)
	}

	savePath := expandHome(t.SavePath)
	if _, err := os.Stat(savePath); err != nil {
		return nil // nothing to unset
	}

	data, err := os.ReadFile(savePath)
	if err != nil {
		return fmt.Errorf("cannot read active save: %w", err)
	}
	activeMD5 := computeMD5(data)
	dir := filepath.Dir(savePath)

	entries, _ := os.ReadDir(dir)
	hasMatchingBak := false
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".bak") {
			continue
		}
		metaData, err := os.ReadFile(metaPath(filepath.Join(dir, entry.Name())))
		if err == nil && readMeta(metaData).MD5 == activeMD5 {
			hasMatchingBak = true
			break
		}
	}

	if !hasMatchingBak {
		stamp := time.Now().Format("20060102_150405")
		newBak := filepath.Join(dir, fmt.Sprintf("ER0000.sl2.%s.bak", stamp))
		if err := copyFile(savePath, newBak); err != nil {
			return fmt.Errorf("cannot save active to backup: %w", err)
		}
		meta := BackupMeta{
			MD5: activeMD5, Tags: []string{},
			Desc: "Auto-saved before unset", CreatedAt: time.Now(),
		}
		os.WriteFile(metaPath(newBak), marshalMeta(meta), 0644) //nolint:errcheck
	}

	os.Remove(localActiveMarkerPath(dir)) //nolint:errcheck
	return os.Remove(savePath)
}

// DeleteSaveBackup deletes a .bak file (and its .json sidecar).
// Returns an error if the backup is currently active.
func (m *LocalManager) DeleteSaveBackup(targetName, backupName string) error {
	t, ok := m.store.Get(targetName)
	if !ok {
		return fmt.Errorf("target %q not found", targetName)
	}

	savePath := expandHome(t.SavePath)
	dir := filepath.Dir(savePath)
	bakPath := filepath.Join(dir, backupName)

	// Determine if this backup is currently active via the marker file.
	if data, err := os.ReadFile(localActiveMarkerPath(dir)); err == nil {
		if strings.TrimSpace(string(data)) == backupName {
			return fmt.Errorf("cannot delete active backup: unset Active first")
		}
	}

	os.Remove(metaPath(bakPath)) //nolint:errcheck — best effort
	return os.Remove(bakPath)
}

// CreateManualBackup copies the active ER0000.sl2 to a new timestamped .bak file.
func (m *LocalManager) CreateManualBackup(targetName string) error {
	t, ok := m.store.Get(targetName)
	if !ok {
		return fmt.Errorf("target %q not found", targetName)
	}

	savePath := expandHome(t.SavePath)
	data, err := os.ReadFile(savePath)
	if err != nil {
		return fmt.Errorf("active save not found: %w", err)
	}

	stamp := time.Now().Format("20060102_150405")
	dir := filepath.Dir(savePath)
	bakPath := filepath.Join(dir, fmt.Sprintf("ER0000.sl2.%s.bak", stamp))

	if err := copyFile(savePath, bakPath); err != nil {
		return fmt.Errorf("cannot write backup: %w", err)
	}
	meta := BackupMeta{
		MD5: computeMD5(data), Tags: []string{},
		CreatedAt: time.Now(),
	}
	os.WriteFile(metaPath(bakPath), marshalMeta(meta), 0644) //nolint:errcheck
	return nil
}

// UpdateBackupMeta rewrites the .json sidecar for a backup with new tags and description.
func (m *LocalManager) UpdateBackupMeta(targetName, backupName string, tags []string, desc string) error {
	t, ok := m.store.Get(targetName)
	if !ok {
		return fmt.Errorf("target %q not found", targetName)
	}

	savePath := expandHome(t.SavePath)
	jsonPath := metaPath(filepath.Join(filepath.Dir(savePath), backupName))

	var meta BackupMeta
	if data, err := os.ReadFile(jsonPath); err == nil {
		meta = readMeta(data)
	}
	meta.Tags = tags
	meta.Desc = desc
	return os.WriteFile(jsonPath, marshalMeta(meta), 0644)
}

// DownloadBackup copies a .bak file to a local destination path.
func (m *LocalManager) DownloadBackup(targetName, backupName, localPath string) error {
	t, ok := m.store.Get(targetName)
	if !ok {
		return fmt.Errorf("target %q not found", targetName)
	}

	savePath := expandHome(t.SavePath)
	bakPath := filepath.Join(filepath.Dir(savePath), backupName)

	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		return fmt.Errorf("cannot create destination directory: %w", err)
	}
	return copyFile(bakPath, localPath)
}
