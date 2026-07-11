package core

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// TestClassifyContainer covers the strict, unambiguous container detection that
// LoadSave relies on: only a native PC (raw BND4) or PS4 (raw PS4 magic) header
// is accepted. Anything else — crucially an AES-encrypted PC save whose BND4
// only appears AFTER decryption — must be reported as unsupported ("") and never
// guessed as a platform.
func TestClassifyContainer(t *testing.T) {
	rawBND4 := append([]byte("BND4"), bytes.Repeat([]byte{0x00}, 32)...)
	rawPS4 := append([]byte(nil), ps4HeaderTemplate[:]...)

	// A genuine AES-encrypted BND4 payload: the plaintext starts with "BND4",
	// but the on-disk container is IV||ciphertext and does NOT.
	plain := append([]byte("BND4"), bytes.Repeat([]byte{0x11}, 28)...) // 32 bytes, block-aligned
	iv := bytes.Repeat([]byte{0x42}, 16)
	encrypted, err := EncryptSave(plain, iv)
	if err != nil {
		t.Fatalf("EncryptSave: %v", err)
	}
	if bytes.HasPrefix(encrypted, []byte("BND4")) {
		t.Fatal("test setup: encrypted container unexpectedly starts with BND4")
	}

	cases := []struct {
		name string
		data []byte
		want Platform
	}{
		{"raw BND4 is PC", rawBND4, PlatformPC},
		{"raw PS4 magic is PS4", rawPS4, PlatformPS},
		{"AES-encrypted BND4 is unsupported", encrypted, ""},
		{"empty is unsupported", nil, ""},
		{"random bytes are unsupported", []byte{0x81, 0xb4, 0x3a, 0xec, 0x00}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ClassifyContainer(tc.data); got != tc.want {
				t.Errorf("ClassifyContainer = %q, want %q", got, tc.want)
			}
		})
	}

	// Explicit guard for the historical bug: encrypted BND4 must never be PC.
	if ClassifyContainer(encrypted) == PlatformPC {
		t.Fatal("regression: AES-encrypted BND4 classified as PC")
	}
}

// TestLoadSaveRejectsUnsupportedContainer proves LoadSave returns
// ErrUnsupportedContainer (rather than parsing/guessing) for a properly-sized
// file whose container magic is neither raw BND4 nor raw PS4. This is the load
// half of "no unsupported input ever reaches an opened state".
func TestLoadSaveRejectsUnsupportedContainer(t *testing.T) {
	data := make([]byte, MinSaveFileSize)
	// Unknown magic (mimics an AES-encrypted PC save's IV-first container).
	copy(data, []byte{0x81, 0xb4, 0x3a, 0xec})

	path := filepath.Join(t.TempDir(), "unsupported.dat")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	save, err := LoadSave(path)
	if !errors.Is(err, ErrUnsupportedContainer) {
		t.Fatalf("LoadSave error = %v, want ErrUnsupportedContainer", err)
	}
	if save != nil {
		t.Fatalf("LoadSave returned a non-nil save for unsupported input")
	}
}

// TestLoadSaveRejectsEncryptedBND4EndToEnd is the historical-bug regression at
// the LoadSave level: a genuinely AES-encrypted PC save (plaintext starts with
// BND4) must be rejected as unsupported, never decrypted-and-guessed as PC.
// Unlike the random-magic case, this proves the payload really decrypts back to
// BND4 — so rejection is a policy decision, not an accident of unknown bytes.
func TestLoadSaveRejectsEncryptedBND4EndToEnd(t *testing.T) {
	// Full-size plaintext BND4 container (MinSaveFileSize is 16-byte aligned).
	plain := make([]byte, MinSaveFileSize)
	copy(plain, []byte("BND4"))

	iv := bytes.Repeat([]byte{0x42}, 16)
	encrypted, err := EncryptSave(plain, iv)
	if err != nil {
		t.Fatalf("EncryptSave: %v", err)
	}
	if bytes.HasPrefix(encrypted, []byte("BND4")) {
		t.Fatal("test setup: encrypted container unexpectedly starts with BND4")
	}
	// Prove this is a real encrypted BND4, not just unknown bytes.
	dec, err := DecryptSave(encrypted)
	if err != nil {
		t.Fatalf("DecryptSave: %v", err)
	}
	if !bytes.HasPrefix(dec, []byte("BND4")) {
		t.Fatal("test setup: encrypted payload does not decrypt back to BND4")
	}

	path := filepath.Join(t.TempDir(), "encrypted.sl2")
	if err := os.WriteFile(path, encrypted, 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	save, err := LoadSave(path)
	if !errors.Is(err, ErrUnsupportedContainer) {
		t.Fatalf("LoadSave error = %v, want ErrUnsupportedContainer", err)
	}
	if save != nil {
		t.Fatal("LoadSave returned a non-nil save for an encrypted BND4 container")
	}
}
