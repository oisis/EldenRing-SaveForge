package main

import (
	"archive/zip"
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// Durable scope-marker event names. save_loaded opens the current-save scope
// (one per successful active-save load); save_closed clears it when the active
// save is dropped. session_closed marks a clean process exit (written by
// DiagnosticJournal.Close) — its absence in a prior session file is what makes
// that session "previous unclosed".
const (
	eventSaveLoaded    = "save_loaded"
	eventSaveClosed    = "save_closed"
	eventSessionClosed = "session_closed"
)

// Validated export scopes. current_session is the whole running process
// session; current_save is everything since the most recent save_loaded that
// has not been cleared by a later save_closed; previous_unclosed is the newest
// prior session file that lacks a session_closed marker.
const (
	exportScopeCurrentSession   = "current_session"
	exportScopeCurrentSave      = "current_save"
	exportScopePreviousUnclosed = "previous_unclosed"
)

// saveLoadOrigin is the safe load-origin enum recorded with each save_loaded
// marker. It carries no path, hostname, or credential — only how the save was
// obtained.
type saveLoadOrigin string

const (
	loadOriginFileDialog     saveLoadOrigin = "file_dialog"
	loadOriginLocalPath      saveLoadOrigin = "local_path"
	loadOriginRemoteDownload saveLoadOrigin = "remote_download"
)

// DiagnosticExportResult is the Wails-facing return of ExportDiagnosticLog.
// Path is the destination the user chose in the native dialog (empty when
// cancelled). It carries no diagnostics-internal session path.
type DiagnosticExportResult struct {
	Scope       string `json:"scope"`
	Cancelled   bool   `json:"cancelled"`
	Path        string `json:"path,omitempty"`
	RecordCount int    `json:"recordCount"`
}

// DiagnosticRecoveryStatus reports whether a newest prior session ended without
// a clean-close marker, so a later UI can offer to export it. It exposes only
// safe metadata — never an absolute path. HasUnclosedSession=true means
// "previous session without a clean-close marker", NOT a confirmed crash.
type DiagnosticRecoveryStatus struct {
	HasUnclosedSession bool   `json:"hasUnclosedSession"`
	Timestamp          string `json:"timestamp,omitempty"`
	RecordCount        int    `json:"recordCount"`
}

// commitLoadedSave installs candidate as the active save under saveMu and,
// only after releasing the lock, appends the durable save_loaded scope marker.
// It is the single post-load commit path shared by every active-save load
// endpoint (file dialog, local path, remote download) so the marker is emitted
// identically without duplicating logic. The append happens outside saveMu
// because journal Sync must never run while the whole-save lock is held; a
// journal failure degrades to stderr and never fails the load.
//
// diagnosticScopeMu wraps the ENTIRE transition (install under saveMu, release
// saveMu, append save_loaded) so two concurrent loads — or a concurrent load
// and close — can never write their scope markers in an order that disagrees
// with the final a.save. It is the outermost lock and is always taken before
// saveMu (see the lock-order note in app.go).
func (a *App) commitLoadedSave(candidate *core.SaveFile, path string, origin saveLoadOrigin) {
	a.diagnosticScopeMu.Lock()
	defer a.diagnosticScopeMu.Unlock()

	a.saveMu.Lock()
	a.installLoadedSave(candidate, path)
	platform := string(candidate.Platform)
	snapshot := diagnosticSaveSnapshot(candidate, a.saveGeneration)
	a.saveMu.Unlock()

	fields := []diagnosticField{
		field("origin", string(origin)),
		field("platform", platform),
	}
	if name := safeSaveFileName(path); name != "" {
		fields = append(fields, field("save_file", name))
	}
	a.journalLog(levelInfo, eventSaveLoaded, "active save loaded", fields...)
	a.journalDebug(eventSaveStateLoaded, "privacy-safe save state captured after load", snapshot...)
}

// safeSaveFileName reduces a local save path to its bare basename when — and
// only when — that basename is a plausible Elden Ring save file name, else "".
// It never returns a directory, path separator, traversal, empty, or over-long
// value, and only accepts the .sl2/.dat/.txt suffixes a save uses, so the
// save_loaded record can name the loaded file without leaking the directory,
// URL, host, or Steam ID around it. It does not weaken the journal's global
// path sanitizer: a bare basename simply does not match those patterns, so no
// exception for the "path" key is introduced.
func safeSaveFileName(path string) string {
	// Reject traversal on the raw input; a legitimate save path never contains
	// "..". filepath.Base is host-separator only (it would keep a Windows path
	// whole on POSIX), so split on both separators to reduce a path from either
	// platform to its trailing segment.
	if path == "" || strings.Contains(path, "..") {
		return ""
	}
	name := path
	if i := strings.LastIndexAny(name, `/\`); i >= 0 {
		name = name[i+1:]
	}
	if name == "" || name == "." || len(name) > 128 || strings.ContainsAny(name, `/\`) {
		return ""
	}
	switch strings.ToLower(filepath.Ext(name)) {
	case ".sl2", ".dat", ".txt":
		return name
	default:
		return ""
	}
}

// journalDir returns the directory holding the current session file, or "" when
// no journal is attached.
func (a *App) journalDir() string {
	p := a.journal.Path()
	if p == "" {
		return ""
	}
	return filepath.Dir(p)
}

// DiagnosticRecoveryStatus reports whether a prior session ended without a
// clean-close marker. Safe to call with no journal (returns a zeroed status)
// and never panics. It never repairs, deletes, or modifies any session file.
func (a *App) DiagnosticRecoveryStatus() (*DiagnosticRecoveryStatus, error) {
	dir := a.journalDir()
	if dir == "" {
		return &DiagnosticRecoveryStatus{}, nil
	}
	path, records, err := findNewestUnclosedSession(dir, a.journal.Path())
	if err != nil {
		return nil, fmt.Errorf("diagnostic recovery status: %w", err)
	}
	if path == "" {
		return &DiagnosticRecoveryStatus{}, nil
	}
	return &DiagnosticRecoveryStatus{
		HasUnclosedSession: true,
		Timestamp:          lastTimestamp(records),
		RecordCount:        len(records),
	}, nil
}

// ExportDiagnosticLog gathers the records for scope, prompts for a destination
// with the native save dialog, and writes a single ZIP there. A cancelled
// dialog returns a typed cancelled result with a nil error. An unavailable or
// failed journal returns a safe diagnostic error, never a panic.
func (a *App) ExportDiagnosticLog(scope string) (DiagnosticExportResult, error) {
	records, err := a.selectDiagnosticRecords(scope)
	if err != nil {
		return DiagnosticExportResult{Scope: scope}, err
	}

	defaultName := fmt.Sprintf("saveforge-diagnostics-%s.zip", time.Now().UTC().Format("20060102T150405Z"))
	dest, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "Export Diagnostic Log",
		DefaultFilename: defaultName,
		Filters: []runtime.FileFilter{
			{DisplayName: "Zip Archive (*.zip)", Pattern: "*.zip"},
			{DisplayName: "All Files (*.*)", Pattern: "*.*"},
		},
	})
	if err != nil {
		return DiagnosticExportResult{Scope: scope}, err
	}
	return finishDiagnosticExport(scope, dest, records)
}

// finishDiagnosticExport turns a chosen destination into a result. An empty
// dest (dialog cancelled) yields a typed cancelled result with a nil error;
// otherwise it builds the ZIP atomically. Context-free so the cancellation and
// build paths can be unit-tested without a Wails context.
func finishDiagnosticExport(scope, dest string, records []diagnosticRecord) (DiagnosticExportResult, error) {
	if dest == "" {
		return DiagnosticExportResult{Scope: scope, Cancelled: true}, nil
	}
	if err := buildDiagnosticZip(dest, scope, records); err != nil {
		return DiagnosticExportResult{Scope: scope}, fmt.Errorf("export diagnostics: %w", err)
	}
	return DiagnosticExportResult{Scope: scope, Path: dest, RecordCount: len(records)}, nil
}

// selectDiagnosticRecords resolves the record slice for a validated scope. It
// reads the durable on-disk JSONL (not the bounded tail) so an export is
// complete. Invalid scopes and unavailable journals return safe errors.
func (a *App) selectDiagnosticRecords(scope string) ([]diagnosticRecord, error) {
	switch scope {
	case exportScopeCurrentSession:
		return a.readUsableSessionRecords()

	case exportScopeCurrentSave:
		// diagnosticScopeMu (outermost, level 0) wraps path resolution, journal
		// read, AND the scope walk so this selection can never observe the window
		// between a load/close mutating a.save and that transition appending its
		// save_loaded/save_closed marker. Without it a read landing mid-transition
		// would resolve the PREVIOUS save's scope (the journal's last marker still
		// points at it while a.save has already moved on). commitLoadedSave and
		// CloseSave hold this same lock across their whole (mutate, marker)
		// sequence, so once acquired here the marker and a.save agree. Journal
		// reads take no saveMu, so this never blocks whole-save work.
		a.diagnosticScopeMu.Lock()
		defer a.diagnosticScopeMu.Unlock()
		records, err := a.readUsableSessionRecords()
		if err != nil {
			return nil, err
		}
		return selectCurrentSaveRecords(records)

	case exportScopePreviousUnclosed:
		dir := a.journalDir()
		if dir == "" {
			return nil, fmt.Errorf("diagnostics unavailable: no active session journal")
		}
		path, records, err := findNewestUnclosedSession(dir, a.journal.Path())
		if err != nil {
			return nil, err
		}
		if path == "" {
			return nil, fmt.Errorf("no previous unclosed diagnostic session found")
		}
		return records, nil

	default:
		return nil, fmt.Errorf("invalid diagnostic export scope %q", scope)
	}
}

// readUsableSessionRecords resolves the current session file path and decodes
// its records. usableSessionPath returns "" for an absent OR permanently failed
// journal, so a failed session cannot masquerade as a valid export.
func (a *App) readUsableSessionRecords() ([]diagnosticRecord, error) {
	path := a.journal.usableSessionPath()
	if path == "" {
		return nil, fmt.Errorf("diagnostics unavailable: no active session journal")
	}
	return readJournalFile(path)
}

// selectCurrentSaveRecords returns records from the most recent save_loaded
// marker to the end. A later save_closed clears the scope, and a newer
// save_loaded replaces a prior one — so the walk simply tracks the last opener.
// When no active-save scope is open it returns a clear, safe error.
func selectCurrentSaveRecords(records []diagnosticRecord) ([]diagnosticRecord, error) {
	start := -1
	for i, rec := range records {
		switch rec.Event {
		case eventSaveLoaded:
			start = i
		case eventSaveClosed:
			start = -1
		}
	}
	if start < 0 {
		return nil, fmt.Errorf("no active save is loaded; the current-save diagnostic scope is empty")
	}
	return records[start:], nil
}

// findNewestUnclosedSession scans dir for session .jsonl files other than
// excludePath (the currently open journal) and returns the newest one that
// lacks a session_closed marker, along with its records. It never modifies,
// deletes, or repairs any file. Returns ("", nil, nil) when none qualifies.
func findNewestUnclosedSession(dir, excludePath string) (string, []diagnosticRecord, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", nil, fmt.Errorf("scan diagnostics dir: %w", err)
	}
	var bestPath string
	var bestRecords []diagnosticRecord
	var bestMod time.Time
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".jsonl" {
			continue
		}
		p := filepath.Join(dir, e.Name())
		if sameFile(p, excludePath) {
			continue // never scan the currently open journal
		}
		records, err := readJournalFile(p)
		if err != nil || len(records) == 0 {
			continue
		}
		if hasEvent(records, eventSessionClosed) {
			continue // cleanly closed — not a recovery candidate
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if bestPath == "" || info.ModTime().After(bestMod) {
			bestPath, bestRecords, bestMod = p, records, info.ModTime()
		}
	}
	return bestPath, bestRecords, nil
}

// buildDiagnosticZip writes a diagnostics ZIP atomically to destPath. It first
// writes a private temp file in destPath's directory, closes it, then renames
// it into place; on any failure only that temp file is removed. The archive
// holds exactly summary.txt and events.jsonl (already-sanitized records in
// original order) and uses only the Go standard library. It is Wails-context
// free so it can be unit-tested with t.TempDir().
func buildDiagnosticZip(destPath, scope string, records []diagnosticRecord) (err error) {
	tmp, err := os.CreateTemp(filepath.Dir(destPath), ".saveforge-diag-*.zip")
	if err != nil {
		return fmt.Errorf("create temp export: %w", err)
	}
	tmpPath := tmp.Name()
	defer func() {
		if err != nil {
			tmp.Close()
			os.Remove(tmpPath)
		}
	}()

	zw := zip.NewWriter(tmp)
	sw, err := zw.Create("summary.txt")
	if err != nil {
		return fmt.Errorf("zip summary: %w", err)
	}
	if _, err = io.WriteString(sw, diagnosticExportSummary(scope, records)); err != nil {
		return fmt.Errorf("write summary: %w", err)
	}

	ew, err := zw.Create("events.jsonl")
	if err != nil {
		return fmt.Errorf("zip events: %w", err)
	}
	for _, rec := range records {
		line, marshalErr := json.Marshal(rec)
		if marshalErr != nil {
			err = fmt.Errorf("marshal record: %w", marshalErr)
			return err
		}
		line = append(line, '\n')
		if _, err = ew.Write(line); err != nil {
			return fmt.Errorf("write events: %w", err)
		}
	}

	if err = zw.Close(); err != nil {
		return fmt.Errorf("finalize zip: %w", err)
	}
	if err = tmp.Close(); err != nil {
		return fmt.Errorf("close temp export: %w", err)
	}
	if err = os.Rename(tmpPath, destPath); err != nil {
		return fmt.Errorf("finalize export: %w", err)
	}
	return nil
}

// diagnosticExportSummary renders summary.txt: scope, record count, time range,
// schema version, and a short privacy statement.
func diagnosticExportSummary(scope string, records []diagnosticRecord) string {
	first, last := "", ""
	if len(records) > 0 {
		first = records[0].Timestamp
		last = records[len(records)-1].Timestamp
	}
	var b strings.Builder
	fmt.Fprintf(&b, "SaveForge diagnostic export\n")
	fmt.Fprintf(&b, "Scope: %s\n", scope)
	fmt.Fprintf(&b, "Records: %d\n", len(records))
	fmt.Fprintf(&b, "Time range: %s .. %s\n", first, last)
	fmt.Fprintf(&b, "Schema version: %d\n", diagnosticJournalSchemaVersion)
	fmt.Fprintf(&b, "\nPrivacy: events are sanitized technical diagnostics only. "+
		"They contain no save file paths, character names, Steam IDs, raw save "+
		"data, hostnames, or credentials.\n")
	return b.String()
}

// readJournalFile decodes every JSONL record in path, in file order. The
// open-error message is intentionally generic: it must not surface the OS path
// or error cause to the user (see decodeJournalRecords for the same rule).
func readJournalFile(path string) ([]diagnosticRecord, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("diagnostic journal unavailable")
	}
	defer f.Close()
	return decodeJournalRecords(f)
}

// decodeJournalRecords decodes JSONL records from r in order. It distinguishes
// a fully-written line (terminated by '\n') from an unterminated trailing
// fragment: only the latter — a record possibly caught mid-append on the live
// session file — is tolerated when it fails to decode. A newline-terminated
// line that does not decode is real corruption and returns a safe error that
// names neither the file path nor the offending bytes, so mid-journal damage is
// surfaced instead of silently dropped. Valid records keep their order and are
// never hidden.
func decodeJournalRecords(r io.Reader) ([]diagnosticRecord, error) {
	var recs []diagnosticRecord
	br := bufio.NewReader(r)
	lineNo := 0
	for {
		line, readErr := br.ReadBytes('\n')
		terminated := readErr == nil // a complete line ending in '\n'
		if terminated || len(line) > 0 {
			lineNo++
		}
		if len(bytes.TrimSpace(line)) > 0 {
			var rec diagnosticRecord
			if decErr := json.Unmarshal(line, &rec); decErr != nil {
				if terminated {
					// A fully-written line that will not decode is corruption.
					// Report position only — never the path or the raw bytes.
					return nil, fmt.Errorf("diagnostic journal corrupt at line %d", lineNo)
				}
				// Unterminated final fragment: skip it (torn concurrent append).
			} else {
				recs = append(recs, rec)
			}
		}
		if readErr != nil {
			if readErr == io.EOF {
				break
			}
			return nil, fmt.Errorf("diagnostic journal read failed")
		}
	}
	return recs, nil
}

func hasEvent(records []diagnosticRecord, event string) bool {
	for _, r := range records {
		if r.Event == event {
			return true
		}
	}
	return false
}

func lastTimestamp(records []diagnosticRecord) string {
	if len(records) == 0 {
		return ""
	}
	return records[len(records)-1].Timestamp
}

func sameFile(a, b string) bool {
	return filepath.Clean(a) == filepath.Clean(b)
}
