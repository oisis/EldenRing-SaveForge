package deploy

import (
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// storeWith builds an in-memory target store so the test never touches the
// user's real targets.json.
func storeWith(t Target) *TargetStore {
	return &TargetStore{targets: []Target{t}}
}

// TestConnection must name the stage that failed and keep the underlying
// system cause, instead of collapsing everything into one generic sentence.
func TestTestConnectionReportsStageAndCause(t *testing.T) {
	dir := t.TempDir()

	garbageKey := filepath.Join(dir, "garbage.key")
	if err := os.WriteFile(garbageKey, []byte("not a private key"), 0600); err != nil {
		t.Fatalf("write garbage key: %v", err)
	}

	// A closed local port gives a deterministic TCP-dial failure without
	// touching any configured remote host.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	closedPort := ln.Addr().(*net.TCPAddr).Port
	ln.Close()

	realKey := filepath.Join(dir, "id_ed25519")
	if err := os.WriteFile(realKey, []byte(testEd25519Key), 0600); err != nil {
		t.Fatalf("write key: %v", err)
	}

	cases := []struct {
		name   string
		target Target
		stage  string
		cause  string
	}{
		{
			name:   "missing key configuration",
			target: Target{Name: "t", Host: "127.0.0.1", Port: closedPort, User: "u"},
			stage:  "[config]",
			cause:  `no SSH key configured for target "t"`,
		},
		{
			name:   "unreadable key file",
			target: Target{Name: "t", Host: "127.0.0.1", Port: closedPort, User: "u", KeyPath: filepath.Join(dir, "absent.key")},
			stage:  "[key-read]",
			cause:  "no such file or directory",
		},
		{
			name:   "unparsable key file",
			target: Target{Name: "t", Host: "127.0.0.1", Port: closedPort, User: "u", KeyPath: garbageKey},
			stage:  "[key-parse]",
			cause:  "ssh: no key found",
		},
		{
			name:   "unreachable host",
			target: Target{Name: "t", Host: "127.0.0.1", Port: closedPort, User: "u", KeyPath: realKey},
			stage:  "[tcp-dial]",
			cause:  "connect: connection refused",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewSSHManager(storeWith(tc.target)).TestConnection("t")
			if err == nil {
				t.Fatal("expected an error")
			}
			if !strings.Contains(err.Error(), tc.stage) {
				t.Errorf("missing stage %s in %q", tc.stage, err)
			}
			if !strings.Contains(err.Error(), tc.cause) {
				t.Errorf("missing original cause %q in %q", tc.cause, err)
			}
			if strings.Contains(err.Error(), "PRIVATE KEY") || strings.Contains(err.Error(), "not a private key") {
				t.Errorf("error leaks key material: %q", err)
			}
		})
	}
}

// UploadSave must report the local-read stage before any connection attempt.
func TestUploadSaveReportsLocalReadStage(t *testing.T) {
	target := Target{Name: "t", Host: "127.0.0.1", Port: 1, User: "u", SavePath: "/remote/ER0000.sl2"}
	err := NewSSHManager(storeWith(target)).UploadSave("t", filepath.Join(t.TempDir(), "absent.sl2"))
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "[local-read]") || !strings.Contains(err.Error(), "no such file or directory") {
		t.Errorf("want stage and cause, got %q", err)
	}
}

// Unencrypted throwaway key, generated for this test only. It is never used
// against a real host: every case here fails before or at the TCP dial.
const testEd25519Key = `-----BEGIN OPENSSH PRIVATE KEY-----
b3BlbnNzaC1rZXktdjEAAAAABG5vbmUAAAAEbm9uZQAAAAAAAAABAAAAMwAAAAtzc2gtZW
QyNTUxOQAAACArkCplBSMbki0b4kxnD183UefLfEqtW3a9qj57ceg0NQAAAJgAeqgFAHqo
BQAAAAtzc2gtZWQyNTUxOQAAACArkCplBSMbki0b4kxnD183UefLfEqtW3a9qj57ceg0NQ
AAAEDupq7NygRE74s0uPz0xBA45P7XxiYKDigK9z9uycjgPSuQKmUFIxuSLRviTGcPXzdR
58t8Sq1bdr2qPntx6DQ1AAAADnNhdmVmb3JnZS10ZXN0AQIDBAUGBw==
-----END OPENSSH PRIVATE KEY-----
`
