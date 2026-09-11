package deploy

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

// sizedReader is what routes remote transfers onto sftp.File.ReadFrom and its
// concurrent-write path. That only happens when the reader handed to io.Copy
// hides the source's own io.WriterTo and still advertises the transfer size.
func TestSizedReaderStreamsWithoutSourceWriterTo(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ER0000.sl2")
	payload := bytes.Repeat([]byte("save"), 4096)
	if err := os.WriteFile(path, payload, 0600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer f.Close()

	if _, ok := io.Reader(f).(io.WriterTo); !ok {
		t.Skip("os.File no longer implements io.WriterTo; the wrapper is moot")
	}

	src := sizedReader{r: f, size: int64(len(payload))}
	if _, ok := io.Reader(src).(io.WriterTo); ok {
		t.Fatal("sizedReader leaks io.WriterTo, io.Copy would bypass sftp.File.ReadFrom")
	}
	if got := src.Size(); got != int64(len(payload)) {
		t.Fatalf("Size() = %d, want %d", got, len(payload))
	}

	var dst bytes.Buffer
	n, err := io.Copy(&dst, src)
	if err != nil {
		t.Fatalf("copy: %v", err)
	}
	if n != int64(len(payload)) {
		t.Fatalf("copied %d bytes, want %d", n, len(payload))
	}
	if !bytes.Equal(dst.Bytes(), payload) {
		t.Fatal("copied content differs from source")
	}
}

// copyStream is the shared transfer contract for the upload and its backup:
// a full copy of a known size succeeds, anything short of it is an error, and
// an unknown size (-1) simply copies the whole stream.
func TestCopyStream(t *testing.T) {
	boom := errors.New("connection reset")

	t.Run("full copy of a known size", func(t *testing.T) {
		payload := bytes.Repeat([]byte("save"), 4096)
		var dst bytes.Buffer
		n, err := copyStream(&dst, bytes.NewReader(payload), int64(len(payload)))
		if err != nil {
			t.Fatalf("copyStream: %v", err)
		}
		if n != int64(len(payload)) {
			t.Fatalf("copied %d bytes, want %d", n, len(payload))
		}
		if !bytes.Equal(dst.Bytes(), payload) {
			t.Fatal("copied content differs from source")
		}
	})

	t.Run("source error keeps the real byte count", func(t *testing.T) {
		src := io.MultiReader(bytes.NewReader([]byte("half")), errReader{boom})
		var dst bytes.Buffer
		n, err := copyStream(&dst, src, 64)
		if !errors.Is(err, boom) {
			t.Fatalf("err = %v, want %v", err, boom)
		}
		if n != 4 {
			t.Fatalf("copied %d bytes, want 4", n)
		}
	})

	t.Run("short stream is an incomplete copy", func(t *testing.T) {
		var dst bytes.Buffer
		n, err := copyStream(&dst, bytes.NewReader([]byte("half")), 64)
		if !errors.Is(err, io.ErrUnexpectedEOF) {
			t.Fatalf("err = %v, want io.ErrUnexpectedEOF", err)
		}
		if n != 4 {
			t.Fatalf("copied %d bytes, want 4", n)
		}
	})

	t.Run("unknown size copies everything without a mismatch", func(t *testing.T) {
		payload := []byte("whole stream")
		var dst bytes.Buffer
		n, err := copyStream(&dst, bytes.NewReader(payload), -1)
		if err != nil {
			t.Fatalf("copyStream: %v", err)
		}
		if n != int64(len(payload)) || !bytes.Equal(dst.Bytes(), payload) {
			t.Fatalf("copied %d bytes %q, want %d bytes %q", n, dst.Bytes(), len(payload), payload)
		}
	})
}

type errReader struct{ err error }

func (r errReader) Read([]byte) (int, error) { return 0, r.err }
