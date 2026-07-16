package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// diagnosticJournalSchemaVersion is the on-disk record schema version.
// Bump it only when the JSONL record shape changes in a way readers must
// branch on; it is written into every record so future tooling can
// migrate old session files.
const diagnosticJournalSchemaVersion = 1

// diagnosticTailMax bounds the in-memory ring of the most recent records
// kept for a future diagnostics UI. It is a memory cap only — the JSONL
// file on disk is the durable source of truth and is never trimmed here.
const diagnosticTailMax = 500

// diagnosticLevel is the severity of a journal record. The set mirrors
// the levels the Wails logger emits (its WARNING collapses to warn) so a
// single vocabulary covers both app- and wails-sourced records.
type diagnosticLevel string

const (
	levelTrace diagnosticLevel = "trace"
	levelDebug diagnosticLevel = "debug"
	levelInfo  diagnosticLevel = "info"
	levelWarn  diagnosticLevel = "warn"
	levelError diagnosticLevel = "error"
)

// Record source constants for the two producers wired today: the app
// itself and the Wails logger sink. Log takes source as a plain string and
// does not enforce this set, so a reader should treat these as the current
// convention rather than a closed enum the API guarantees.
const (
	sourceApp   = "app"
	sourceWails = "wails"
)

// Central sanitization boundary. Every record's message and field values
// pass through sanitize/sanitizeFields before persistence and before the
// tail can observe them, so no producer (app or the Wails sink) can leak a
// filesystem path, Steam ID, token/password/secret/credential, or
// credential-bearing URL into the durable log. The patterns are deliberately
// broad in the redact direction: it is safer to over-redact a stray path
// than to persist a real one.
var (
	// Credential-bearing URLs (any scheme with userinfo, e.g.
	// ssh://user:pass@host/...) and bare ssh:// URLs.
	reCredURL = regexp.MustCompile(`[a-zA-Z][a-zA-Z0-9+.\-]*://[^\s]*@[^\s]*`)
	reSSHURL  = regexp.MustCompile(`ssh://[^\s]+`)
	// General URLs can expose an internal host in a browser/network error.
	// Diagnostic events never need the destination, so redact the whole URL.
	reURL = regexp.MustCompile(`[a-zA-Z][a-zA-Z0-9+.\-]*://[^\s]+`)
	// Authorization headers: the value is a whitespace-separated scheme plus
	// token (e.g. "Authorization: Bearer <token>"), so redact both words, not
	// just the scheme. Runs before reSecretKV, which only takes one token.
	reAuthHeader = regexp.MustCompile(`(?i)\b(authorization)\s*[:=]\s*\S+(?:\s+\S+)?`)
	// key=value / key: value pairs whose key names a secret.
	reSecretKV = regexp.MustCompile(`(?i)\b(token|password|passwd|pwd|secret|credential|authorization|api[_\-]?key)\s*[:=]\s*\S+`)
	// UNC (\\server\share\...), Windows drive-letter, and POSIX absolute
	// paths (two or more segments so a lone '/' in prose is left alone).
	reUNCPath   = regexp.MustCompile(`\\\\[^\s]+`)
	reWinPath   = regexp.MustCompile(`[A-Za-z]:\\[^\s]*`)
	rePosixPath = regexp.MustCompile(`/[\w.\-]+(?:/[\w.\-]+)+`)
	// 17-digit Steam IDs (far longer than any item ID or counter we log).
	reSteamID = regexp.MustCompile(`\b\d{17}\b`)
)

// sensitiveFieldKeys names structured-field keys whose value is redacted
// wholesale. Matched case-insensitively as a substring so "save_path",
// "steam_id", or "ip_address" are caught too.
var sensitiveFieldKeys = []string{
	"path", "steam", "token", "password", "secret",
	"credential", "authorization", "host", "address",
	"api_key", "api-key", "apikey",
}

// sanitize redacts sensitive substrings from a free-text value. URLs are
// handled first because they embed the '@', '/', and ':' the later patterns
// key on; paths and Steam IDs follow. Safe diagnostic data — operation
// names, item IDs, counters, ordinary error text — matches none of these
// and passes through unchanged.
func sanitize(s string) string {
	if s == "" {
		return s
	}
	s = reCredURL.ReplaceAllString(s, "[redacted-url]")
	s = reSSHURL.ReplaceAllString(s, "[redacted-url]")
	s = reURL.ReplaceAllString(s, "[redacted-url]")
	s = reAuthHeader.ReplaceAllString(s, "$1=[redacted]")
	s = reSecretKV.ReplaceAllString(s, "$1=[redacted]")
	s = reUNCPath.ReplaceAllString(s, "[redacted-path]")
	s = reWinPath.ReplaceAllString(s, "[redacted-path]")
	s = rePosixPath.ReplaceAllString(s, "[redacted-path]")
	s = reSteamID.ReplaceAllString(s, "[redacted-id]")
	return s
}

// sanitizeFields returns a sanitized copy of fields: a sensitive-keyed field
// keeps its key but has its value replaced, and any other field has its
// value run through sanitize. The caller's slice is never mutated.
func sanitizeFields(fields []diagnosticField) []diagnosticField {
	if len(fields) == 0 {
		return fields
	}
	out := make([]diagnosticField, len(fields))
	for i, f := range fields {
		if isSensitiveKey(f.Key) {
			out[i] = diagnosticField{Key: f.Key, Value: "[redacted]"}
			continue
		}
		// Narrow, deliberate exception: a strictly validated item-icon path
		// (items/<category>/<file>.png) is kept verbatim so an asset_load_failed
		// record stays diagnostic — the general path sanitizer would otherwise
		// redact it. Any other "asset" value, or any non-matching syntax, still
		// falls through to sanitize below.
		if f.Key == "asset" && isValidIconAsset(f.Value) {
			out[i] = f
			continue
		}
		out[i] = diagnosticField{Key: f.Key, Value: sanitize(f.Value)}
	}
	return out
}

func isSensitiveKey(key string) bool {
	k := strings.ToLower(key)
	for _, s := range sensitiveFieldKeys {
		if strings.Contains(k, s) {
			return true
		}
	}
	return false
}

// diagnosticField is one structured attribute attached to a record. Both
// members are strings by deliberate design: callers pass already-sanitized
// technical values, and a string-only field can never smuggle raw save
// bytes, a map, or an arbitrary untyped payload into the journal. This is
// the privacy boundary in type form — see TestDiagnosticRecordHasNoUntypedFields.
type diagnosticField struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// field is the terse constructor for a structured field.
func field(key, value string) diagnosticField {
	return diagnosticField{Key: key, Value: value}
}

// diagnosticRecord is one JSONL line. Field order/tags are the on-disk
// contract; every member is a concrete scalar or a slice of concrete
// scalars, never interface{} or []byte, so the record cannot serialize
// untyped or raw-byte data.
type diagnosticRecord struct {
	SchemaVersion int               `json:"schema_version"`
	Seq           uint64            `json:"seq"`
	Timestamp     string            `json:"ts"`
	Level         diagnosticLevel   `json:"level"`
	Source        string            `json:"source"`
	Event         string            `json:"event"`
	Message       string            `json:"message"`
	Fields        []diagnosticField `json:"fields,omitempty"`
}

// DiagnosticJournal is a durable, append-only, crash-safe record of
// technical diagnostic events for one process session. It is the single
// source of truth for diagnostic logs: a record is appended (and fsync'd)
// to the session JSONL file before it is observable anywhere else.
//
// Every operation is serialized by mu so the file preserves one total
// sequence order and each record is written as exactly one line. This
// first implementation fsyncs after every record and deliberately has no
// timers, background workers, retention, rotation, or queue.
//
// The journal never calls the Wails runtime logger, so a Wails logger
// sink can feed it without risking recursion.
type DiagnosticJournal struct {
	mu      sync.Mutex
	f       *os.File // nil once closed, or for an in-memory journal
	path    string
	seq     uint64
	tail    []diagnosticRecord
	tailMax int
	// failed marks a disk-backed journal that hit a write/sync error: its
	// file is closed and no further record (including session_closed) may be
	// appended, so a partially written JSONL line can never be followed.
	failed bool
	err    error // cause preserved for callers once failed
	// debug, when true, admits debug-level records; trace is always dropped
	// either way (debug mode is not trace mode). This is the single source of
	// truth for journal verbosity — App holds no parallel policy — and is
	// read/written only under mu.
	debug bool
	// debugConfigured records whether SetDebugEnabled has run at least once this
	// session. It lets that method distinguish the first configuration of the
	// level (always worth journalling, even for false) from a later repeat of an
	// unchanged value, so the debug-mode change event is not duplicated when the
	// frontend re-syncs the same value (e.g. React Strict Mode).
	debugConfigured bool
}

// DefaultDiagnosticsDir returns the per-user diagnostics directory,
// $UserConfigDir/EldenRing-SaveEditor/diagnostics, creating it with
// owner-only (0700) permissions if missing. It sits next to the existing
// templates store (see templates.DefaultTemplateLibraryDir) — never the
// repository, tmp/, or the save-file directory.
func DefaultDiagnosticsDir() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("DefaultDiagnosticsDir: %w", err)
	}
	dir := filepath.Join(configDir, "EldenRing-SaveEditor", "diagnostics")
	if err := ensureDiagnosticsDir(dir); err != nil {
		return "", fmt.Errorf("DefaultDiagnosticsDir: %w", err)
	}
	return dir, nil
}

// ensureDiagnosticsDir creates dir with owner-only (0700) permissions if it
// is missing and tightens an already-existing directory to owner-only.
// MkdirAll leaves an existing directory's mode untouched, so the explicit
// Chmod strips any group/other bits a prior run (or another tool) may have
// left. Best-effort: on platforms that do not honour POSIX modes the Chmod
// is a harmless no-op, so its error is not fatal.
func ensureDiagnosticsDir(dir string) error {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	_ = os.Chmod(dir, 0700)
	return nil
}

// sessionFileName builds a collision-safe JSONL file name for one process
// session: a UTC timestamp plus the pid and a random suffix so two
// processes starting in the same second still get distinct files.
func sessionFileName() (string, error) {
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("sessionFileName: %w", err)
	}
	stamp := time.Now().UTC().Format("20060102T150405Z")
	return fmt.Sprintf("session-%s-%d-%s.jsonl", stamp, os.Getpid(), hex.EncodeToString(b[:])), nil
}

// NewSessionDiagnosticJournal opens a fresh session journal in the
// default per-user diagnostics directory and writes the session_started
// record. Used by the production app before wails.Run.
func NewSessionDiagnosticJournal() (*DiagnosticJournal, error) {
	dir, err := DefaultDiagnosticsDir()
	if err != nil {
		return nil, err
	}
	return newDiagnosticJournalInDir(dir)
}

// newDiagnosticJournalInDir opens a session journal inside dir. Tests
// pass a t.TempDir() so they never touch the real user directory. The
// file is created O_EXCL (collision-safe) and 0600 (private to the user
// where the OS honours file modes), and session_started is appended
// immediately so an empty session file can never exist.
func newDiagnosticJournalInDir(dir string) (*DiagnosticJournal, error) {
	name, err := sessionFileName()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(dir, name)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_EXCL|os.O_APPEND, 0600)
	if err != nil {
		return nil, fmt.Errorf("newDiagnosticJournalInDir: %w", err)
	}
	j := &DiagnosticJournal{f: f, path: path, tailMax: diagnosticTailMax}
	// app_version is stamped here, at journal creation before wails.Run, so a
	// crash in early startup still leaves the running version on record.
	if err := j.append(levelInfo, sourceApp, "session_started", "diagnostic session started", []diagnosticField{field("app_version", appVersion)}); err != nil {
		f.Close()
		return nil, err
	}
	return j, nil
}

// newInMemoryDiagnosticJournal returns a journal with no backing file. It
// keeps the bounded tail but writes nothing to disk — a seam for tests
// that need a journal instance without creating a session file.
func newInMemoryDiagnosticJournal() *DiagnosticJournal {
	return &DiagnosticJournal{tailMax: diagnosticTailMax}
}

// SetDebugEnabled sets journal verbosity: true admits debug records, false
// drops them. Trace is always dropped regardless. It is the sole writer of
// the journal's verbosity policy. A nil receiver is a safe no-op so headless
// callers (tests, journal-unavailable startup) need no guard.
//
// It returns true when this call is a state-defining event worth journalling:
// the first configuration of the session (whatever the value) or a real
// true↔false transition. A repeat of the already-configured value returns
// false, so callers do not emit a duplicate change event. A nil receiver
// returns false. The decision is made atomically under mu against the same
// policy state it mutates, so it is the single source of truth.
func (j *DiagnosticJournal) SetDebugEnabled(enabled bool) bool {
	if j == nil {
		return false
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	changed := !j.debugConfigured || j.debug != enabled
	j.debugConfigured = true
	j.debug = enabled
	return changed
}

// recordable reports whether a record at level passes the current verbosity
// policy: info/warn/error always pass; debug passes only in debug mode; trace
// is always dropped. Callers hold j.mu.
func (j *DiagnosticJournal) recordable(level diagnosticLevel) bool {
	switch level {
	case levelTrace:
		return false
	case levelDebug:
		return j.debug
	default:
		return true
	}
}

// Log appends one record. The returned error (nil on success) lets the
// direct caller observe a write/fsync failure; the app never crashes on a
// journal error. A nil receiver is a safe no-op so callers that may not
// have a journal (tests, journal-unavailable startup) need no guard.
func (j *DiagnosticJournal) Log(level diagnosticLevel, source, event, message string, fields ...diagnosticField) error {
	if j == nil {
		return nil
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.append(level, source, event, message, fields)
}

// append assigns the next sequence number, sanitizes the record, then — for
// a disk-backed journal — writes and fsyncs one complete JSONL line before
// the record is published to the tail. Callers hold j.mu, so sequence
// numbers and file lines share one total order.
//
// Durability is the invariant: the tail can only ever contain records that
// are already on disk, so a failed append leaves the tail unchanged. A
// write or sync error marks the journal unusable (see fail): its file is
// closed and every later append — including session_closed — short-circuits,
// so no record can follow a partially written line.
//
// The in-memory seam (j.f == nil, not failed) publishes straight to the tail
// with no disk I/O; it exists only for tests that need a journal without a
// session file.
func (j *DiagnosticJournal) append(level diagnosticLevel, source, event, message string, fields []diagnosticField) error {
	if j.failed {
		return fmt.Errorf("diagnostic journal unusable: %w", j.err)
	}
	if !j.recordable(level) {
		// Filtered before seq++, so a dropped record touches neither the
		// sequence, the file, Sync, nor the tail — and leaves no seq gap.
		return nil
	}
	j.seq++
	rec := diagnosticRecord{
		SchemaVersion: diagnosticJournalSchemaVersion,
		Seq:           j.seq,
		Timestamp:     time.Now().UTC().Format(time.RFC3339Nano),
		Level:         level,
		Source:        source,
		Event:         event,
		Message:       sanitize(message),
		Fields:        sanitizeFields(fields),
	}

	if j.f == nil {
		// In-memory seam: tail-only, always succeeds.
		j.pushTail(rec)
		return nil
	}

	line, err := json.Marshal(rec)
	if err != nil {
		return j.fail(fmt.Errorf("diagnostic journal marshal: %w", err))
	}
	line = append(line, '\n')
	if _, err := j.f.Write(line); err != nil {
		return j.fail(fmt.Errorf("diagnostic journal write: %w", err))
	}
	if err := j.f.Sync(); err != nil {
		return j.fail(fmt.Errorf("diagnostic journal sync: %w", err))
	}
	// Durable now: only after a complete, fsync'd line does the record
	// become observable in the tail.
	j.pushTail(rec)
	return nil
}

// fail marks a disk-backed journal permanently unusable after a write/sync
// error: it stores the cause, closes the file so no further partial line can
// be appended, and returns err for the direct caller to observe.
func (j *DiagnosticJournal) fail(err error) error {
	j.failed = true
	j.err = err
	if j.f != nil {
		j.f.Close()
		j.f = nil
	}
	return err
}

// pushTail appends to the bounded in-memory ring, dropping the oldest
// record once the cap is reached.
func (j *DiagnosticJournal) pushTail(rec diagnosticRecord) {
	j.tail = append(j.tail, rec)
	if len(j.tail) > j.tailMax {
		// Copy forward so the dropped record can be GC'd rather than
		// pinned by a slice header retaining the backing array head.
		j.tail = append(j.tail[:0:0], j.tail[len(j.tail)-j.tailMax:]...)
	}
}

// Tail returns a copy of the bounded in-memory record ring, oldest first.
// It exists for a future diagnostics UI; no Wails endpoint exposes it yet.
func (j *DiagnosticJournal) Tail() []diagnosticRecord {
	if j == nil {
		return nil
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	return append([]diagnosticRecord(nil), j.tail...)
}

// Path returns the absolute path of the backing session file, or "" for a
// nil or in-memory journal. The diagnostic export reads this file directly
// (the on-disk JSONL is the durable source of truth, not the bounded tail).
func (j *DiagnosticJournal) Path() string {
	if j == nil {
		return ""
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.path
}

// usableSessionPath returns the backing session file path only when the
// journal is present, disk-backed, and has NOT failed; otherwise "". It lets
// the export layer treat an absent OR permanently failed journal identically —
// both yield "" — without exposing the path or the OS failure cause. Path()
// still returns the raw path after a failure (recovery scans need it to
// exclude the current file); this stricter accessor gates the current-session
// and current-save export scopes.
func (j *DiagnosticJournal) usableSessionPath() string {
	if j == nil {
		return ""
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.failed {
		return ""
	}
	return j.path
}

// Close writes the session_closed record, then fsyncs and closes the
// file. A crash skips this path, so the absence of session_closed marks
// an unclean shutdown — no recovery logic lives here. Close is idempotent
// and a nil receiver is a safe no-op.
//
// If the session_closed append itself fails, append calls fail(), which
// closes the file and clears j.f; Close must not close it again (that would
// operate on a nil handle) and returns the append error, leaving the journal
// failed. A journal that was already failed returns its preserved cause
// rather than a false clean success, and never appends session_closed.
func (j *DiagnosticJournal) Close() error {
	if j == nil {
		return nil
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.failed {
		return j.err
	}
	if j.f == nil {
		return nil
	}
	// append clears j.f via fail() on error, so bail out before double-close.
	if err := j.append(levelInfo, sourceApp, "session_closed", "diagnostic session closed", nil); err != nil {
		return err
	}
	closeErr := j.f.Close()
	j.f = nil
	return closeErr
}

// wailsJournalLogger adapts the Wails logger.Logger interface onto a
// DiagnosticJournal, tagging every message with source "wails". Wails
// applies its own LogLevel filtering before calling these methods (see
// logger.New in the Wails runtime), so this sink adds no filtering of its
// own and does not change the production log-level policy.
//
// On a journal write failure it falls back to stderr — never to the Wails
// runtime logger — so a logging error can neither crash the app nor
// recurse back into this sink. The sink also omits known, high-volume asset
// load chatter that carries no diagnostic value for a user-reported issue.
type wailsJournalLogger struct {
	journal *DiagnosticJournal
}

// newWailsJournalLogger wires a Wails logger sink to journal.
func newWailsJournalLogger(journal *DiagnosticJournal) *wailsJournalLogger {
	return &wailsJournalLogger{journal: journal}
}

// record appends one wails-sourced entry, degrading to stderr on failure.
func (w *wailsJournalLogger) record(level diagnosticLevel, event, message string) {
	if err := w.journal.Log(level, sourceWails, event, message); err != nil {
		fmt.Fprintf(os.Stderr, "diagnostic journal (wails %s): %v\n", event, err)
	}
}

// isNoisyWailsDebug reports whether a Wails debug line is asset-handler
// fallback chatter with no diagnostic value. Both the internal and external
// asset handlers emit a line per served asset (Loading / not found, serving),
// producing dozens of redacted, path-shaped entries that never prove an icon
// failed to reach the user. Only Debug() consults this — Warning/Error/Fatal
// are always recorded.
func isNoisyWailsDebug(message string) bool {
	return strings.Contains(message, "[ExternalAssetHandler]") ||
		strings.Contains(message, "[AssetHandler]")
}

// reIconAsset matches the sole public, relative item-icon path the diagnostics
// layer trusts verbatim: items/<category>/<file>.png, lowercase letters,
// digits, '_', '-', '.' only. No leading slash, scheme, host, query string,
// or absolute path can match. The explicit ".." check in isValidIconAsset
// rejects traversal that the character class alone would admit.
var reIconAsset = regexp.MustCompile(`^items/[a-z0-9._-]+/[a-z0-9._-]+\.png$`)

// isValidIconAsset reports whether asset is a safe, public item-icon path.
// It is the single validation rule shared by the client endpoint (which drops
// an invalid asset without logging) and the journal's narrow sanitizer
// exception (which keeps a valid asset readable). Anything else is untrusted.
func isValidIconAsset(asset string) bool {
	if strings.Contains(asset, "..") {
		return false
	}
	return reIconAsset.MatchString(asset)
}

func (w *wailsJournalLogger) Print(message string) { w.record(levelInfo, "print", message) }
func (w *wailsJournalLogger) Trace(message string) { w.record(levelTrace, "trace", message) }
func (w *wailsJournalLogger) Debug(message string) {
	if isNoisyWailsDebug(message) {
		return
	}
	w.record(levelDebug, "debug", message)
}
func (w *wailsJournalLogger) Info(message string)    { w.record(levelInfo, "info", message) }
func (w *wailsJournalLogger) Warning(message string) { w.record(levelWarn, "warning", message) }
func (w *wailsJournalLogger) Error(message string)   { w.record(levelError, "error", message) }
func (w *wailsJournalLogger) Fatal(message string)   { w.record(levelError, "fatal", message) }
