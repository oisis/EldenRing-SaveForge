package deploy

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// LocalManager handles local file copy and game launch/stop operations.
type LocalManager struct {
	store *TargetStore
}

// NewLocalManager creates a new local operations manager.
func NewLocalManager(store *TargetStore) *LocalManager {
	return &LocalManager{store: store}
}

// TestConnection verifies the local save path is accessible.
func (m *LocalManager) TestConnection(targetName string) (string, error) {
	t, ok := m.store.Get(targetName)
	if !ok {
		return "", fmt.Errorf("target %q not found", targetName)
	}

	path := expandHome(t.SavePath)
	dir := filepath.Dir(path)
	info, err := os.Stat(dir)
	if err != nil {
		return "", fmt.Errorf("directory not accessible: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%s is not a directory", dir)
	}

	// Check if save file exists
	if _, err := os.Stat(path); err == nil {
		return fmt.Sprintf("Local path OK — save file exists at %s", path), nil
	}
	return fmt.Sprintf("Local path OK — directory exists, save file not found yet (%s)", path), nil
}

// UploadSave copies a local save file to the target path.
// Creates a timestamped backup before overwriting.
func (m *LocalManager) UploadSave(targetName string, localPath string) error {
	t, ok := m.store.Get(targetName)
	if !ok {
		return fmt.Errorf("target %q not found", targetName)
	}

	destPath := expandHome(t.SavePath)

	// Read source
	srcData, err := os.ReadFile(localPath)
	if err != nil {
		return fmt.Errorf("cannot read source file: %w", err)
	}

	// Backup existing file (creates .bak + .json sidecar for Save Manager).
	if existingData, statErr := os.ReadFile(destPath); statErr == nil {
		stamp := time.Now().Format("20060102_150405")
		backupPath := fmt.Sprintf("%s.%s.bak", destPath, stamp)
		if err := copyFile(destPath, backupPath); err != nil {
			return fmt.Errorf("backup failed: %w", err)
		}
		meta := BackupMeta{
			MD5: computeMD5(existingData), Tags: []string{},
			Desc: "Auto-backup before deploy", CreatedAt: time.Now(),
		}
		os.WriteFile(metaPath(backupPath), marshalMeta(meta), 0644) //nolint:errcheck
	}

	// Ensure destination directory exists
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return fmt.Errorf("cannot create destination directory: %w", err)
	}

	// Write
	if err := os.WriteFile(destPath, srcData, 0644); err != nil {
		return fmt.Errorf("write failed: %w", err)
	}

	// Verify size
	info, err := os.Stat(destPath)
	if err != nil {
		return fmt.Errorf("cannot verify written file: %w", err)
	}
	if info.Size() != int64(len(srcData)) {
		return fmt.Errorf("size mismatch: wrote %d, file is %d", len(srcData), info.Size())
	}

	return nil
}

// DownloadSave copies the save file from the target path to a local path.
func (m *LocalManager) DownloadSave(targetName string, localPath string) error {
	t, ok := m.store.Get(targetName)
	if !ok {
		return fmt.Errorf("target %q not found", targetName)
	}

	srcPath := expandHome(t.SavePath)
	if _, err := os.Stat(srcPath); err != nil {
		return fmt.Errorf("save file not found at %s: %w", srcPath, err)
	}

	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		return fmt.Errorf("cannot create local directory: %w", err)
	}

	return copyFile(srcPath, localPath)
}

// LaunchGame starts the game using the configured command or platform default.
func (m *LocalManager) LaunchGame(targetName string) (string, error) {
	t, ok := m.store.Get(targetName)
	if !ok {
		return "", fmt.Errorf("target %q not found", targetName)
	}

	cmd := t.GameStartCmd
	if cmd == "" {
		cmd = defaultLocalStartCmd()
	}

	return runLocalCmd(cmd)
}

// CloseGame stops the game using the configured command or platform default.
func (m *LocalManager) CloseGame(targetName string) (string, error) {
	t, ok := m.store.Get(targetName)
	if !ok {
		return "", fmt.Errorf("target %q not found", targetName)
	}

	cmd := t.GameStopCmd
	if cmd == "" {
		cmd = defaultLocalStopCmd()
	}

	// Wrap kill commands so they always exit 0
	if strings.Contains(cmd, "pkill") || strings.Contains(cmd, "taskkill") {
		cmd = cmd + ` && echo "killed" || echo "not found"`
	}

	return runLocalCmd(cmd)
}

// DeployAndLaunch performs: close game → wait → copy save → launch.
func (m *LocalManager) DeployAndLaunch(targetName string, localPath string) error {
	m.CloseGame(targetName)
	time.Sleep(3 * time.Second)

	if err := m.UploadSave(targetName, localPath); err != nil {
		return fmt.Errorf("copy failed: %w", err)
	}

	if _, err := m.LaunchGame(targetName); err != nil {
		return fmt.Errorf("launch failed: %w", err)
	}
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}

func defaultLocalStartCmd() string {
	switch runtime.GOOS {
	case "windows":
		return `cmd /C start steam://rungameid/1245620`
	case "darwin":
		return `open steam://rungameid/1245620`
	default: // linux
		return `steam steam://rungameid/1245620`
	}
}

func defaultLocalStopCmd() string {
	switch runtime.GOOS {
	case "windows":
		return `taskkill /IM eldenring.exe`
	default: // linux, darwin
		return `pkill -TERM -f eldenring.exe`
	}
}

func runLocalCmd(command string) (string, error) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/C", command)
	default:
		cmd = exec.Command("sh", "-c", command)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("command failed: %w — output: %s", err, string(output))
	}
	return string(output), nil
}
