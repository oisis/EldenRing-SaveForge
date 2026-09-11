package deploy

import (
	"crypto/md5"
	"fmt"
	"io"
	"net"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// SSHManager handles SSH connections and remote operations for deploy targets.
type SSHManager struct {
	store *TargetStore
}

// NewSSHManager creates a new SSH manager backed by the given target store.
func NewSSHManager(store *TargetStore) *SSHManager {
	return &SSHManager{store: store}
}

// TestConnection verifies SSH connectivity to a target. Returns host info on success.
func (m *SSHManager) TestConnection(targetName string) (string, error) {
	t, ok := m.store.Get(targetName)
	if !ok {
		return "", fmt.Errorf("target %q not found", targetName)
	}
	client, err := m.dial(t)
	if err != nil {
		return "", err
	}
	defer client.Close()
	return fmt.Sprintf("Connected to %s@%s:%d", t.User, t.Host, t.Port), nil
}

// sizedReader feeds a transfer source to io.Copy without the source's own
// io.WriterTo. io.Copy prefers src.WriteTo over dst.ReadFrom, and os.File
// implements WriteTo, which would bypass sftp.File.ReadFrom and with it the
// concurrent-write path. Size lets ReadFrom size its concurrency up front.
type sizedReader struct {
	r    io.Reader
	size int64
}

func (s sizedReader) Read(p []byte) (int, error) { return s.r.Read(p) }

func (s sizedReader) Size() int64 { return s.size }

// copyStream streams src into dst through sizedReader, so io.Copy reaches
// sftp.File.ReadFrom instead of the source's own WriteTo. A known size (>= 0)
// is enforced: a stream that ends early is a truncated transfer, not a
// success. size < 0 means the size is unknown and no count check is possible.
func copyStream(dst io.Writer, src io.Reader, size int64) (int64, error) {
	n, err := io.Copy(dst, sizedReader{r: src, size: size})
	if err != nil {
		return n, err
	}
	if size >= 0 && n != size {
		return n, fmt.Errorf("copied %d of %d bytes: %w", n, size, io.ErrUnexpectedEOF)
	}
	return n, nil
}

// UploadSave uploads a local save file to the remote target.
// It creates a timestamped backup of the remote file before overwriting.
func (m *SSHManager) UploadSave(targetName string, localPath string) error {
	t, ok := m.store.Get(targetName)
	if !ok {
		return fmt.Errorf("target %q not found", targetName)
	}

	local, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("[local-read] cannot read local file: %w", err)
	}
	defer local.Close()

	localInfo, err := local.Stat()
	if err != nil {
		return fmt.Errorf("[local-read] cannot stat local file: %w", err)
	}
	localSize := localInfo.Size()

	client, err := m.dial(t)
	if err != nil {
		return err
	}
	defer client.Close()

	sftpClient, err := sftp.NewClient(client, sftp.UseConcurrentWrites(true))
	if err != nil {
		return fmt.Errorf("[sftp-init] SFTP session failed: %w", err)
	}
	defer sftpClient.Close()

	// Backup remote file if it exists (creates .bak + .json sidecar for Save Manager).
	if _, statErr := sftpClient.Stat(t.SavePath); statErr == nil {
		stamp := time.Now().Format("20060102_150405")
		backupPath := fmt.Sprintf("%s.%s.bak", t.SavePath, stamp)
		if existing, err := sftpClient.Open(t.SavePath); err == nil {
			// -1 keeps "unknown" distinct from "empty": pkg/sftp reads a
			// negative size as unknown and still uses full concurrency.
			existingSize := int64(-1)
			if info, statErr := existing.Stat(); statErr == nil {
				existingSize = info.Size()
			}
			if dst, err := sftpClient.Create(backupPath); err == nil {
				// Stream the remote save straight into the backup and hash it on
				// the way through, so neither copy is buffered in memory.
				digest := md5.New()
				_, copyErr := copyStream(dst, io.TeeReader(existing, digest), existingSize)
				closeErr := dst.Close()
				if copyErr != nil || closeErr != nil {
					// A half-written .bak would show up in ListBackups as a real
					// backup, so drop exactly the file this block just created.
					sftpClient.Remove(backupPath) //nolint:errcheck — best effort
				} else {
					meta := BackupMeta{
						MD5: fmt.Sprintf("%x", digest.Sum(nil)), Tags: []string{},
						Desc: "Auto-backup before deploy", CreatedAt: time.Now(),
					}
					if mf, err := sftpClient.Create(metaPath(backupPath)); err == nil {
						mf.Write(marshalMeta(meta)) //nolint:errcheck
						mf.Close()
					}
				}
			}
			existing.Close()
		}
	}

	// Ensure remote directory exists (use path.Dir for POSIX remote paths)
	remoteDir := path.Dir(t.SavePath)
	sftpClient.MkdirAll(remoteDir)

	// Upload
	dst, err := sftpClient.Create(t.SavePath)
	if err != nil {
		return fmt.Errorf("[remote-create] cannot create remote file %s: %w", t.SavePath, err)
	}

	n, err := copyStream(dst, local, localSize)
	if err != nil {
		dst.Close()
		return fmt.Errorf("[remote-write] upload write failed: %w", err)
	}

	// Close flushes the SFTP buffer — must check error
	if err := dst.Close(); err != nil {
		return fmt.Errorf("[remote-flush] upload flush failed: %w", err)
	}

	// Verify size via SFTP stat
	info, err := sftpClient.Stat(t.SavePath)
	if err != nil {
		return fmt.Errorf("[remote-verify] cannot verify remote file: %w", err)
	}
	if info.Size() != localSize {
		return fmt.Errorf("[remote-verify] size mismatch after upload: local=%d, remote=%d", localSize, info.Size())
	}
	if n != localSize {
		return fmt.Errorf("[remote-verify] write mismatch: wrote %d, expected %d", n, localSize)
	}

	return nil
}

// DownloadSave downloads the save file from the remote target to a local path.
func (m *SSHManager) DownloadSave(targetName string, localPath string) error {
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

	src, err := sftpClient.Open(t.SavePath)
	if err != nil {
		return fmt.Errorf("cannot open remote file: %w", err)
	}
	defer src.Close()

	// Ensure local directory exists
	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		return fmt.Errorf("cannot create local directory: %w", err)
	}

	dst, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("cannot create local file: %w", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("download failed: %w", err)
	}

	return nil
}

// LaunchGame executes the game start command on the remote target.
func (m *SSHManager) LaunchGame(targetName string) (string, error) {
	t, ok := m.store.Get(targetName)
	if !ok {
		return "", fmt.Errorf("target %q not found", targetName)
	}
	cmd := t.GameStartCmd
	if cmd == "" {
		cmd = DefaultStartCmd
	}
	return m.execRemote(t, cmd)
}

// CloseGame executes the game stop command on the remote target.
func (m *SSHManager) CloseGame(targetName string) (string, error) {
	t, ok := m.store.Get(targetName)
	if !ok {
		return "", fmt.Errorf("target %q not found", targetName)
	}
	cmd := t.GameStopCmd
	if cmd == "" {
		cmd = DefaultStopCmd
	}
	// Wrap pkill-style commands so they always return exit 0
	// (pkill returns 1 when no process found — not a real error)
	if strings.Contains(cmd, "pkill") || strings.Contains(cmd, "taskkill") {
		cmd = cmd + ` && echo "killed" || echo "not found"`
	}
	return m.execRemote(t, cmd)
}

// DeployAndLaunch performs the full workflow: close game → wait → upload → launch.
func (m *SSHManager) DeployAndLaunch(targetName string, localPath string) error {
	// Step 1: Close game (ignore errors — game might not be running)
	m.CloseGame(targetName)

	// Step 2: Wait for graceful shutdown
	time.Sleep(3 * time.Second)

	// Step 3: Upload save
	if err := m.UploadSave(targetName, localPath); err != nil {
		return fmt.Errorf("upload failed: %w", err)
	}

	// Step 4: Launch game
	if _, err := m.LaunchGame(targetName); err != nil {
		return fmt.Errorf("launch failed: %w", err)
	}

	return nil
}

// dial opens an SSH client to the target. Every failure is tagged with the
// stage it happened in so the UI console can show where the connection broke,
// not only that it broke. Stages map 1:1 to work this function actually does.
func (m *SSHManager) dial(t Target) (*ssh.Client, error) {
	keyPath := expandHome(t.KeyPath)
	if keyPath == "" {
		return nil, fmt.Errorf("[config] no SSH key configured for target %q", t.Name)
	}

	keyData, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("[key-read] cannot read SSH key %s: %w", keyPath, err)
	}
	signer, err := ssh.ParsePrivateKey(keyData)
	if err != nil {
		return nil, fmt.Errorf("[key-parse] cannot parse SSH key %s: %w", keyPath, err)
	}

	config := &ssh.ClientConfig{
		User:            t.User,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}

	addr := net.JoinHostPort(t.Host, strconv.Itoa(t.Port))
	// ssh.Dial folds the TCP connect and the SSH handshake into a single error.
	// Doing both steps here keeps the same semantics but lets "host unreachable"
	// and "authentication rejected" report as different stages.
	conn, err := net.DialTimeout("tcp", addr, config.Timeout)
	if err != nil {
		return nil, fmt.Errorf("[tcp-dial] cannot reach %s: %w", addr, err)
	}
	sshConn, chans, reqs, err := ssh.NewClientConn(conn, addr, config)
	if err != nil {
		conn.Close() //nolint:errcheck
		return nil, fmt.Errorf("[ssh-handshake] SSH handshake/authentication to %s as %q failed: %w", addr, t.User, err)
	}
	return ssh.NewClient(sshConn, chans, reqs), nil
}

func (m *SSHManager) execRemote(t Target, command string) (string, error) {
	client, err := m.dial(t)
	if err != nil {
		return "", err
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return "", fmt.Errorf("SSH session failed: %w", err)
	}
	defer session.Close()

	output, err := session.CombinedOutput(command)
	if err != nil {
		return string(output), fmt.Errorf("command failed: %w — output: %s", err, string(output))
	}
	return strings.TrimSpace(string(output)), nil
}

func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[2:])
	}
	return path
}
