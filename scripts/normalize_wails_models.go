package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

const (
	defaultModelsPath = "frontend/wailsjs/go/models.ts"
	defaultRuntimeDir = "frontend/wailsjs/runtime"
)

func main() {
	path := flag.String("path", defaultModelsPath, "path to generated Wails models.ts")
	runtimeDir := flag.String("runtime-dir", defaultRuntimeDir, "directory of generated Wails runtime files")
	flag.Parse()

	if err := normalizeWailsModels(*path); err != nil {
		fmt.Fprintf(os.Stderr, "normalize Wails models: %v\n", err)
		os.Exit(1)
	}
	if err := normalizeWailsRuntimeModes(*runtimeDir); err != nil {
		fmt.Fprintf(os.Stderr, "normalize Wails runtime modes: %v\n", err)
		os.Exit(1)
	}
}

// normalizeWailsModels removes only trailing spaces and tabs from each line of
// Wails' generated models.ts while preserving all other bytes and the existing
// file mode. Wails v2 prefixes every scanned model line with a tab, including
// empty lines; this makes generated blank lines fail git diff --check.
func normalizeWailsModels(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	normalized := trimLineTrailingWhitespace(data)
	if bytes.Equal(data, normalized) {
		return nil
	}

	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	return os.WriteFile(path, normalized, info.Mode().Perm())
}

func trimLineTrailingWhitespace(data []byte) []byte {
	var out bytes.Buffer
	for start := 0; start < len(data); {
		end := bytes.IndexByte(data[start:], '\n')
		if end < 0 {
			out.Write(bytes.TrimRight(data[start:], " \t"))
			break
		}
		end += start
		out.Write(bytes.TrimRight(data[start:end], " \t"))
		out.WriteByte('\n')
		start = end + 1
	}
	return out.Bytes()
}

// normalizeWailsRuntimeModes removes executable bits from Wails runtime files.
// The Wails generator recreates this directory as 0755 even though these are
// JavaScript/package artifacts, which otherwise produces a mode-only Git diff.
func normalizeWailsRuntimeModes(dir string) error {
	return filepath.WalkDir(dir, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !entry.Type().IsRegular() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		mode := info.Mode().Perm()
		normalized := mode &^ 0o111
		if normalized == mode {
			return nil
		}
		return os.Chmod(path, normalized)
	})
}
