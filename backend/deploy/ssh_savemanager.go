package deploy

import (
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/sftp"
)

func remoteActiveMarkerPath(dir string) string {
	return path.Join(dir, activeMarkerFile)
}

// ListBackups returns all .bak files in the target's save directory.
func (m *SSHManager) ListBackups(targetName string) ([]SaveBackupEntry, error) {
	t, ok := m.store.Get(targetName)
	if !ok {
		return nil, fmt.Errorf("target %q not found", targetName)
	}

	client, err := m.dial(t)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	sftpClient, err := sftp.NewClient(client)
	if err != nil {
		return nil, fmt.Errorf("SFTP session failed: %w", err)
	}
	defer sftpClient.Close()

	dir := path.Dir(t.SavePath)

	// Read active-backup marker (written by SetActiveBackup / cleared by UnsetActiveBackup).
	activeName := ""
	if f, err := sftpClient.Open(remoteActiveMarkerPath(dir)); err == nil {
		data, _ := io.ReadAll(f)
		f.Close()
		activeName = strings.TrimSpace(string(data))
	}

	entries, err := sftpClient.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("cannot read remote directory: %w", err)
	}

	var result []SaveBackupEntry
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".bak") {
			continue
		}
		bakPath := path.Join(dir, entry.Name())

		var meta BackupMeta
		if f, err := sftpClient.Open(metaPath(bakPath)); err == nil {
			data, _ := io.ReadAll(f)
			f.Close()
			meta = readMeta(data)
		} else {
			meta.Tags = []string{}
		}

		result = append(result, SaveBackupEntry{
			Name:      entry.Name(),
			Timestamp: entry.ModTime().UTC().Format(time.RFC3339),
			Size:      entry.Size(),
			MD5:       meta.MD5,
			Tags:      meta.Tags,
			Desc:      meta.Desc,
			IsActive:  activeName != "" && entry.Name() == activeName,
		})
	}
	return result, nil
}

// SetActiveBackup copies the named .bak file over ER0000.sl2 and writes the active marker.
func (m *SSHManager) SetActiveBackup(targetName, backupName string) error {
	t, ok := m.store.Get(targetName)
	if !ok {
		return fmt.Errorf("target %q not found", targetName)
	}

	client, err := m.dial(t)
	if err != nil {
		return err
	}
	defer client.Close()

	sftpClient, err := sftp.NewClient(client)
	if err != nil {
		return fmt.Errorf("SFTP session failed: %w", err)
	}
	defer sftpClient.Close()

	dir := path.Dir(t.SavePath)
	bakPath := path.Join(dir, backupName)
	data, err := readRemoteFile(sftpClient, bakPath)
	if err != nil {
		return fmt.Errorf("cannot read backup: %w", err)
	}
	if err := writeRemoteFile(sftpClient, t.SavePath, data); err != nil {
		return err
	}
	writeRemoteFile(sftpClient, remoteActiveMarkerPath(dir), []byte(backupName)) //nolint:errcheck
	return nil
}

// UnsetActiveBackup removes ER0000.sl2 from the target.
// If no existing .bak has a matching md5, the active file is first moved to a new .bak.
func (m *SSHManager) UnsetActiveBackup(targetName string) error {
	t, ok := m.store.Get(targetName)
	if !ok {
		return fmt.Errorf("target %q not found", targetName)
	}

	client, err := m.dial(t)
	if err != nil {
		return err
	}
	defer client.Close()

	sftpClient, err := sftp.NewClient(client)
	if err != nil {
		return fmt.Errorf("SFTP session failed: %w", err)
	}
	defer sftpClient.Close()

	if _, statErr := sftpClient.Stat(t.SavePath); statErr != nil {
		return nil // nothing to unset
	}

	data, err := readRemoteFile(sftpClient, t.SavePath)
	if err != nil {
		return fmt.Errorf("cannot read active save: %w", err)
	}
	activeMD5 := computeMD5(data)
	dir := path.Dir(t.SavePath)

	// Check whether any .bak already covers this md5.
	entries, _ := sftpClient.ReadDir(dir)
	hasMatchingBak := false
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".bak") {
			continue
		}
		if f, err := sftpClient.Open(metaPath(path.Join(dir, entry.Name()))); err == nil {
			jsonData, _ := io.ReadAll(f)
			f.Close()
			if readMeta(jsonData).MD5 == activeMD5 {
				hasMatchingBak = true
				break
			}
		}
	}

	if !hasMatchingBak {
		stamp := time.Now().Format("20060102_150405")
		newBak := path.Join(dir, fmt.Sprintf("ER0000.sl2.%s.bak", stamp))
		if err := writeRemoteFile(sftpClient, newBak, data); err != nil {
			return fmt.Errorf("cannot save active to backup: %w", err)
		}
		meta := BackupMeta{
			MD5: activeMD5, Tags: []string{},
			Desc: "Auto-saved before unset", CreatedAt: time.Now(),
		}
		writeRemoteFile(sftpClient, metaPath(newBak), marshalMeta(meta)) //nolint:errcheck
	}

	sftpClient.Remove(remoteActiveMarkerPath(dir)) //nolint:errcheck
	return sftpClient.Remove(t.SavePath)
}

// DeleteSaveBackup deletes a .bak file (and its .json sidecar) from the target.
// Returns an error if the backup is currently active.
func (m *SSHManager) DeleteSaveBackup(targetName, backupName string) error {
	t, ok := m.store.Get(targetName)
	if !ok {
		return fmt.Errorf("target %q not found", targetName)
	}

	client, err := m.dial(t)
	if err != nil {
		return err
	}
	defer client.Close()

	sftpClient, err := sftp.NewClient(client)
	if err != nil {
		return fmt.Errorf("SFTP session failed: %w", err)
	}
	defer sftpClient.Close()

	dir := path.Dir(t.SavePath)
	bakPath := path.Join(dir, backupName)

	// Determine if this backup is currently active via the marker file.
	if f, err := sftpClient.Open(remoteActiveMarkerPath(dir)); err == nil {
		data, _ := io.ReadAll(f)
		f.Close()
		if strings.TrimSpace(string(data)) == backupName {
			return fmt.Errorf("cannot delete active backup: unset Active first")
		}
	}

	sftpClient.Remove(metaPath(bakPath)) //nolint:errcheck — best effort
	return sftpClient.Remove(bakPath)
}

// CreateManualBackup copies the active ER0000.sl2 to a new timestamped .bak file.
func (m *SSHManager) CreateManualBackup(targetName string) error {
	t, ok := m.store.Get(targetName)
	if !ok {
		return fmt.Errorf("target %q not found", targetName)
	}

	client, err := m.dial(t)
	if err != nil {
		return err
	}
	defer client.Close()

	sftpClient, err := sftp.NewClient(client)
	if err != nil {
		return fmt.Errorf("SFTP session failed: %w", err)
	}
	defer sftpClient.Close()

	data, err := readRemoteFile(sftpClient, t.SavePath)
	if err != nil {
		return fmt.Errorf("active save not found: %w", err)
	}

	stamp := time.Now().Format("20060102_150405")
	dir := path.Dir(t.SavePath)
	bakPath := path.Join(dir, fmt.Sprintf("ER0000.sl2.%s.bak", stamp))

	if err := writeRemoteFile(sftpClient, bakPath, data); err != nil {
		return fmt.Errorf("cannot write backup: %w", err)
	}
	meta := BackupMeta{
		MD5: computeMD5(data), Tags: []string{},
		CreatedAt: time.Now(),
	}
	writeRemoteFile(sftpClient, metaPath(bakPath), marshalMeta(meta)) //nolint:errcheck
	return nil
}

// UpdateBackupMeta rewrites the .json sidecar for a backup with new tags and description.
func (m *SSHManager) UpdateBackupMeta(targetName, backupName string, tags []string, desc string) error {
	t, ok := m.store.Get(targetName)
	if !ok {
		return fmt.Errorf("target %q not found", targetName)
	}

	client, err := m.dial(t)
	if err != nil {
		return err
	}
	defer client.Close()

	sftpClient, err := sftp.NewClient(client)
	if err != nil {
		return fmt.Errorf("SFTP session failed: %w", err)
	}
	defer sftpClient.Close()

	dir := path.Dir(t.SavePath)
	jsonPath := metaPath(path.Join(dir, backupName))

	var meta BackupMeta
	if f, err := sftpClient.Open(jsonPath); err == nil {
		data, _ := io.ReadAll(f)
		f.Close()
		meta = readMeta(data)
	}
	meta.Tags = tags
	meta.Desc = desc
	return writeRemoteFile(sftpClient, jsonPath, marshalMeta(meta))
}

// DownloadBackup copies a .bak file from the remote target to a local path.
func (m *SSHManager) DownloadBackup(targetName, backupName, localPath string) error {
	t, ok := m.store.Get(targetName)
	if !ok {
		return fmt.Errorf("target %q not found", targetName)
	}

	client, err := m.dial(t)
	if err != nil {
		return err
	}
	defer client.Close()

	sftpClient, err := sftp.NewClient(client)
	if err != nil {
		return fmt.Errorf("SFTP session failed: %w", err)
	}
	defer sftpClient.Close()

	src, err := sftpClient.Open(path.Join(path.Dir(t.SavePath), backupName))
	if err != nil {
		return fmt.Errorf("cannot open backup: %w", err)
	}
	defer src.Close()

	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		return fmt.Errorf("cannot create local directory: %w", err)
	}
	dst, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("cannot create local file: %w", err)
	}
	defer dst.Close()

	_, err = io.Copy(dst, src)
	return err
}

// readRemoteFile reads an entire remote file via SFTP.
func readRemoteFile(c *sftp.Client, remotePath string) ([]byte, error) {
	f, err := c.Open(remotePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(f)
}

// writeRemoteFile writes data to a remote path via SFTP, creating or truncating the file.
func writeRemoteFile(c *sftp.Client, remotePath string, data []byte) error {
	f, err := c.Create(remotePath)
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

