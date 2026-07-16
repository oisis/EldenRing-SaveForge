package main

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"
)

// readSessionRecords reads every JSONL line from the single session file
// in dir and decodes it into diagnosticRecord values, in file order.
func readSessionRecords(t *testing.T, dir string) []diagnosticRecord {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	var path string
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".jsonl" {
			if path != "" {
				t.Fatalf("expected exactly one session file, found extra %q", e.Name())
			}
			path = filepath.Join(dir, e.Name())
		}
	}
	if path == "" {
		t.Fatal("no .jsonl session file found")
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open session file: %v", err)
	}
	defer f.Close()

	var recs []diagnosticRecord
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		var rec diagnosticRecord
		if err := json.Unmarshal(sc.Bytes(), &rec); err != nil {
			t.Fatalf("decode record %q: %v", sc.Text(), err)
		}
		recs = append(recs, rec)
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("scan: %v", err)
	}
	return recs
}

// readSessionRaw returns the raw bytes of the single session file in dir.
func readSessionRaw(t *testing.T, dir string) string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".jsonl" {
			b, err := os.ReadFile(filepath.Join(dir, e.Name()))
			if err != nil {
				t.Fatalf("read session file: %v", err)
			}
			return string(b)
		}
	}
	t.Fatal("no .jsonl session file found")
	return ""
}

func TestNewDiagnosticJournalWritesSessionStarted(t *testing.T) {
	dir := t.TempDir()
	j, err := newDiagnosticJournalInDir(dir)
	if err != nil {
		t.Fatalf("newDiagnosticJournalInDir: %v", err)
	}
	defer j.Close()

	recs := readSessionRecords(t, dir)
	if len(recs) != 1 {
		t.Fatalf("expected 1 record after open, got %d", len(recs))
	}
	got := recs[0]
	if got.Event != "session_started" {
		t.Errorf("event = %q, want session_started", got.Event)
	}
	if got.Source != sourceApp {
		t.Errorf("source = %q, want %q", got.Source, sourceApp)
	}
	if got.SchemaVersion != diagnosticJournalSchemaVersion {
		t.Errorf("schema = %d, want %d", got.SchemaVersion, diagnosticJournalSchemaVersion)
	}
	if got.Seq != 1 {
		t.Errorf("seq = %d, want 1", got.Seq)
	}
	if got.Level != levelInfo {
		t.Errorf("level = %q, want %q", got.Level, levelInfo)
	}
	if got.Timestamp == "" {
		t.Error("timestamp must be set")
	}
}

func TestDiagnosticJournalRecordShapeAndOrder(t *testing.T) {
	dir := t.TempDir()
	j, err := newDiagnosticJournalInDir(dir)
	if err != nil {
		t.Fatalf("newDiagnosticJournalInDir: %v", err)
	}

	if err := j.Log(levelWarn, sourceApp, "op_a", "first", field("k", "v")); err != nil {
		t.Fatalf("Log op_a: %v", err)
	}
	if err := j.Log(levelError, sourceApp, "op_b", "second"); err != nil {
		t.Fatalf("Log op_b: %v", err)
	}
	if err := j.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	recs := readSessionRecords(t, dir)
	wantEvents := []string{"session_started", "op_a", "op_b", "session_closed"}
	if len(recs) != len(wantEvents) {
		t.Fatalf("got %d records, want %d", len(recs), len(wantEvents))
	}
	for i, rec := range recs {
		if rec.Event != wantEvents[i] {
			t.Errorf("record %d event = %q, want %q", i, rec.Event, wantEvents[i])
		}
		if rec.Seq != uint64(i+1) {
			t.Errorf("record %d seq = %d, want %d", i, rec.Seq, i+1)
		}
	}
	if len(recs[1].Fields) != 1 || recs[1].Fields[0] != field("k", "v") {
		t.Errorf("op_a fields = %+v, want one {k,v}", recs[1].Fields)
	}
	if recs[2].Level != levelError {
		t.Errorf("op_b level = %q, want %q", recs[2].Level, levelError)
	}
}

func TestDiagnosticJournalCloseIsIdempotentAndClosesFile(t *testing.T) {
	dir := t.TempDir()
	j, err := newDiagnosticJournalInDir(dir)
	if err != nil {
		t.Fatalf("newDiagnosticJournalInDir: %v", err)
	}
	if err := j.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	// Second close must be a safe no-op, not a double-close error.
	if err := j.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
	// A closed journal must not append further records: session_closed
	// stays the last line so a reader can trust it marks a clean exit.
	if err := j.Log(levelInfo, sourceApp, "after_close", "ignored"); err != nil {
		t.Fatalf("Log after close: %v", err)
	}
	recs := readSessionRecords(t, dir)
	if len(recs) != 2 {
		t.Fatalf("got %d records, want 2 (started, closed)", len(recs))
	}
	if recs[len(recs)-1].Event != "session_closed" {
		t.Errorf("last event = %q, want session_closed", recs[len(recs)-1].Event)
	}
}

func TestDiagnosticJournalCrashLeavesNoSessionClosed(t *testing.T) {
	dir := t.TempDir()
	j, err := newDiagnosticJournalInDir(dir)
	if err != nil {
		t.Fatalf("newDiagnosticJournalInDir: %v", err)
	}
	if err := j.Log(levelInfo, sourceApp, "work", "in progress"); err != nil {
		t.Fatalf("Log: %v", err)
	}
	// Simulate a crash: the process dies without Close. Because every
	// record is fsync'd, the durable file holds the pre-crash records
	// and, crucially, no session_closed marker.
	recs := readSessionRecords(t, dir)
	for _, rec := range recs {
		if rec.Event == "session_closed" {
			t.Fatal("session_closed present without a clean Close")
		}
	}
	if len(recs) != 2 {
		t.Fatalf("got %d records, want 2 (started, work)", len(recs))
	}
}

func TestDiagnosticJournalConcurrentAppendsPreserveOrder(t *testing.T) {
	dir := t.TempDir()
	j, err := newDiagnosticJournalInDir(dir)
	if err != nil {
		t.Fatalf("newDiagnosticJournalInDir: %v", err)
	}

	const goroutines = 8
	const perG = 50
	var wg sync.WaitGroup
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < perG; i++ {
				if err := j.Log(levelInfo, sourceApp, "concurrent", "x"); err != nil {
					t.Errorf("Log: %v", err)
					return
				}
			}
		}()
	}
	wg.Wait()
	if err := j.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	recs := readSessionRecords(t, dir)
	// session_started + N concurrent + session_closed.
	wantTotal := 1 + goroutines*perG + 1
	if len(recs) != wantTotal {
		t.Fatalf("got %d records, want %d", len(recs), wantTotal)
	}
	// Sequence numbers must form one gap-free 1..N total order and lines
	// must never be interleaved (each line decoded cleanly above).
	for i, rec := range recs {
		if rec.Seq != uint64(i+1) {
			t.Fatalf("record %d seq = %d, want %d (order or atomicity broken)", i, rec.Seq, i+1)
		}
	}
}

func TestDiagnosticJournalTailIsBounded(t *testing.T) {
	j := newInMemoryDiagnosticJournal()
	j.tailMax = 3
	for i := 0; i < 10; i++ {
		if err := j.Log(levelInfo, sourceApp, "e", "m"); err != nil {
			t.Fatalf("Log: %v", err)
		}
	}
	tail := j.Tail()
	if len(tail) != 3 {
		t.Fatalf("tail len = %d, want 3", len(tail))
	}
	// The ring keeps the newest records: seq 8, 9, 10.
	for i, rec := range tail {
		wantSeq := uint64(8 + i)
		if rec.Seq != wantSeq {
			t.Errorf("tail[%d] seq = %d, want %d", i, rec.Seq, wantSeq)
		}
	}
}

func TestWailsJournalLoggerMapsLevels(t *testing.T) {
	dir := t.TempDir()
	j, err := newDiagnosticJournalInDir(dir)
	if err != nil {
		t.Fatalf("newDiagnosticJournalInDir: %v", err)
	}
	sink := newWailsJournalLogger(j)

	sink.Trace("t")
	sink.Debug("d")
	sink.Info("i")
	sink.Warning("w")
	sink.Error("e")
	sink.Print("p")
	sink.Fatal("f")
	if err := j.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	recs := readSessionRecords(t, dir)
	// session_started + 7 sink calls + session_closed.
	type want struct {
		event string
		level diagnosticLevel
	}
	wants := []want{
		{"trace", levelTrace},
		{"debug", levelDebug},
		{"info", levelInfo},
		{"warning", levelWarn},
		{"error", levelError},
		{"print", levelInfo},
		{"fatal", levelError},
	}
	if len(recs) != 1+len(wants)+1 {
		t.Fatalf("got %d records, want %d", len(recs), 1+len(wants)+1)
	}
	for i, w := range wants {
		rec := recs[i+1] // skip session_started
		if rec.Source != sourceWails {
			t.Errorf("%s source = %q, want %q", w.event, rec.Source, sourceWails)
		}
		if rec.Event != w.event {
			t.Errorf("record %d event = %q, want %q", i, rec.Event, w.event)
		}
		if rec.Level != w.level {
			t.Errorf("%s level = %q, want %q", w.event, rec.Level, w.level)
		}
	}
}

func TestWailsJournalLoggerSurvivesJournalWriteFailure(t *testing.T) {
	dir := t.TempDir()
	j, err := newDiagnosticJournalInDir(dir)
	if err != nil {
		t.Fatalf("newDiagnosticJournalInDir: %v", err)
	}
	// Force every subsequent write to fail by closing the underlying file
	// out from under the journal, without going through Close (which would
	// nil the handle). The sink must swallow the error and not panic.
	if err := j.f.Close(); err != nil {
		t.Fatalf("pre-close file: %v", err)
	}
	sink := newWailsJournalLogger(j)
	sink.Info("this write will fail")

	// The direct Log caller must observe the failure as a returned error.
	if err := j.Log(levelInfo, sourceApp, "direct", "also fails"); err == nil {
		t.Error("expected Log to return an error on a closed file")
	}
}

func TestDiagnosticJournalWriteFailureDoesNotPublishTail(t *testing.T) {
	dir := t.TempDir()
	j, err := newDiagnosticJournalInDir(dir)
	if err != nil {
		t.Fatalf("newDiagnosticJournalInDir: %v", err)
	}
	// Tail holds only session_started at this point.
	if got := len(j.Tail()); got != 1 {
		t.Fatalf("tail len after open = %d, want 1", got)
	}
	// Close the file out from under the journal so the next write fails.
	if err := j.f.Close(); err != nil {
		t.Fatalf("pre-close file: %v", err)
	}
	if err := j.Log(levelInfo, sourceApp, "will_fail", "nope"); err == nil {
		t.Fatal("expected Log to return an error after the file was closed")
	}
	// A failed append must not change the tail: no partial record leaks out.
	tail := j.Tail()
	if len(tail) != 1 {
		t.Fatalf("tail len after failed append = %d, want 1 (unchanged)", len(tail))
	}
	if tail[0].Event != "session_started" {
		t.Errorf("tail[0] event = %q, want session_started", tail[0].Event)
	}
}

func TestDiagnosticJournalFailedCannotAppendSessionClosed(t *testing.T) {
	dir := t.TempDir()
	j, err := newDiagnosticJournalInDir(dir)
	if err != nil {
		t.Fatalf("newDiagnosticJournalInDir: %v", err)
	}
	// Force the journal into the failed state.
	if err := j.f.Close(); err != nil {
		t.Fatalf("pre-close file: %v", err)
	}
	if err := j.Log(levelInfo, sourceApp, "will_fail", "nope"); err == nil {
		t.Fatal("expected the forcing Log to fail")
	}
	// Close on an already-failed journal surfaces the preserved cause (not a
	// false clean success) and must not append session_closed.
	if err := j.Close(); err == nil {
		t.Error("Close on failed journal returned nil, want the preserved failure")
	}
	recs := readSessionRecords(t, dir)
	for _, rec := range recs {
		if rec.Event == "session_closed" {
			t.Fatal("session_closed present on a failed journal")
		}
	}
	if len(recs) != 1 || recs[0].Event != "session_started" {
		t.Fatalf("records = %+v, want only session_started", recs)
	}
}

func TestDiagnosticJournalPersistedAndTailShareSeq(t *testing.T) {
	dir := t.TempDir()
	j, err := newDiagnosticJournalInDir(dir)
	if err != nil {
		t.Fatalf("newDiagnosticJournalInDir: %v", err)
	}
	for i := 0; i < 5; i++ {
		if err := j.Log(levelInfo, sourceApp, "op", "m"); err != nil {
			t.Fatalf("Log: %v", err)
		}
	}
	if err := j.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	disk := readSessionRecords(t, dir)
	tail := j.Tail()
	if len(disk) != len(tail) {
		t.Fatalf("disk %d records, tail %d records", len(disk), len(tail))
	}
	for i := range disk {
		if disk[i].Seq != tail[i].Seq {
			t.Errorf("record %d: disk seq %d != tail seq %d", i, disk[i].Seq, tail[i].Seq)
		}
		if disk[i].Event != tail[i].Event {
			t.Errorf("record %d: disk event %q != tail event %q", i, disk[i].Event, tail[i].Event)
		}
	}
}

func TestDiagnosticJournalSanitizesSensitiveValues(t *testing.T) {
	dir := t.TempDir()
	j, err := newDiagnosticJournalInDir(dir)
	if err != nil {
		t.Fatalf("newDiagnosticJournalInDir: %v", err)
	}
	sensitive := []string{
		"/Users/alice/private/ER0000.sl2",
		`C:\Users\Alice\ER0000.sl2`,
		`\\server\share\ER0000.sl2`,
		"76561198000000000",
		"token=secret-value",
		"ssh://alice:password@example.com/save",
	}
	for i, msg := range sensitive {
		if err := j.Log(levelInfo, sourceApp, "op", msg); err != nil {
			t.Fatalf("Log %d: %v", i, err)
		}
	}
	// Sensitive values in structured-field keys must be redacted too.
	if err := j.Log(levelInfo, sourceApp, "op", "with fields",
		field("path", "/Users/alice/private/ER0000.sl2"),
		field("steam", "76561198000000000"),
		field("token", "secret-value"),
	); err != nil {
		t.Fatalf("Log fields: %v", err)
	}
	if err := j.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	raw := readSessionRaw(t, dir)
	// Exact representative values, plus escaping-agnostic fragments (JSON
	// escapes backslashes, so also assert filename/host fragments are gone).
	mustBeAbsent := append([]string{}, sensitive...)
	mustBeAbsent = append(mustBeAbsent,
		"ER0000.sl2", "76561198000000000", "secret-value",
		"alice:password", "example.com", "server", "Alice",
	)
	for _, v := range mustBeAbsent {
		if strings.Contains(raw, v) {
			t.Errorf("session file leaked sensitive value %q", v)
		}
	}
}

func TestDiagnosticJournalPreservesSafeValues(t *testing.T) {
	dir := t.TempDir()
	j, err := newDiagnosticJournalInDir(dir)
	if err != nil {
		t.Fatalf("newDiagnosticJournalInDir: %v", err)
	}
	if err := j.Log(levelWarn, sourceApp, "save_load",
		"operation save_load items=42 handle=0x80000123 error: unexpected EOF at offset 512"); err != nil {
		t.Fatalf("Log: %v", err)
	}
	if err := j.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	raw := readSessionRaw(t, dir)
	for _, v := range []string{"save_load", "items=42", "handle=0x80000123", "unexpected EOF", "offset 512"} {
		if !strings.Contains(raw, v) {
			t.Errorf("safe value %q was redacted but should remain", v)
		}
	}
}

func TestDiagnosticJournalCloseDoesNotPanicWhenAppendFails(t *testing.T) {
	dir := t.TempDir()
	j, err := newDiagnosticJournalInDir(dir)
	if err != nil {
		t.Fatalf("newDiagnosticJournalInDir: %v", err)
	}
	// Close the file out from under the still-open journal so the
	// session_closed append fails. fail() then clears j.f; Close must not
	// double-close it (would panic / act on a nil handle).
	if err := j.f.Close(); err != nil {
		t.Fatalf("pre-close file: %v", err)
	}
	// Must not panic and must return the append failure, not nil.
	if err := j.Close(); err == nil {
		t.Fatal("Close returned nil, want the session_closed append failure")
	}
	// No session_closed on disk...
	for _, rec := range readSessionRecords(t, dir) {
		if rec.Event == "session_closed" {
			t.Fatal("session_closed persisted despite append failure")
		}
	}
	// ...and none published to the tail.
	for _, rec := range j.Tail() {
		if rec.Event == "session_closed" {
			t.Fatal("session_closed published to tail despite append failure")
		}
	}
}

func TestDiagnosticJournalRedactsAuthorizationAndAPIKey(t *testing.T) {
	dir := t.TempDir()
	j, err := newDiagnosticJournalInDir(dir)
	if err != nil {
		t.Fatalf("newDiagnosticJournalInDir: %v", err)
	}
	if err := j.Log(levelInfo, sourceApp, "op", "Authorization: Bearer super-secret-token"); err != nil {
		t.Fatalf("Log auth: %v", err)
	}
	// api_key as a structured key with a value that has no secret-looking
	// prefix must still be redacted.
	if err := j.Log(levelInfo, sourceApp, "op", "creds", field("api_key", "plain-api-secret")); err != nil {
		t.Fatalf("Log field: %v", err)
	}
	if err := j.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	raw := readSessionRaw(t, dir)
	for _, v := range []string{
		"Authorization: Bearer super-secret-token",
		"super-secret-token",
		"plain-api-secret",
	} {
		if strings.Contains(raw, v) {
			t.Errorf("session file leaked %q", v)
		}
	}
}

func TestWailsJournalLoggerRedactsAuthorizationAndAPIKey(t *testing.T) {
	dir := t.TempDir()
	j, err := newDiagnosticJournalInDir(dir)
	if err != nil {
		t.Fatalf("newDiagnosticJournalInDir: %v", err)
	}
	// The Wails sink must pass through the same central sanitizer.
	sink := newWailsJournalLogger(j)
	sink.Info("Authorization: Bearer super-secret-token")
	sink.Error("api_key=plain-api-secret rejected")
	if err := j.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	raw := readSessionRaw(t, dir)
	for _, v := range []string{"super-secret-token", "plain-api-secret"} {
		if strings.Contains(raw, v) {
			t.Errorf("wails sink leaked %q", v)
		}
	}
	// Ordinary error text mentioning "authorization" (no header value) stays.
	if !strings.Contains(raw, "rejected") {
		t.Error("safe trailing text was redacted")
	}
}

func TestEnsureDiagnosticsDirIsOwnerOnly(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("file modes are not meaningful on this platform")
	}
	dir := filepath.Join(t.TempDir(), "diagnostics")
	// Pre-create it group/other-accessible to prove tightening happens.
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatalf("pre-create: %v", err)
	}
	if err := ensureDiagnosticsDir(dir); err != nil {
		t.Fatalf("ensureDiagnosticsDir: %v", err)
	}
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm&0o077 != 0 {
		t.Errorf("dir perm = %o, want 0700 (no group/other bits)", perm)
	}
}

func TestNilJournalIsSafe(t *testing.T) {
	var j *DiagnosticJournal
	if err := j.Log(levelInfo, sourceApp, "e", "m"); err != nil {
		t.Errorf("nil Log err = %v, want nil", err)
	}
	if tail := j.Tail(); tail != nil {
		t.Errorf("nil Tail = %v, want nil", tail)
	}
	if err := j.Close(); err != nil {
		t.Errorf("nil Close err = %v, want nil", err)
	}
	// App helpers must no-op with no journal attached (existing fixtures).
	app := NewApp()
	app.journalLog(levelInfo, "e", "m")
	app.shutdown(nil)
}

// TestDiagnosticRecordHasNoUntypedFields is the privacy boundary in
// test form: it walks the record model and fails if any field can carry
// interface{}, a raw byte slice, or a map — the shapes that could smuggle
// unsanitized or raw-byte payloads into the journal.
func TestDiagnosticRecordHasNoUntypedFields(t *testing.T) {
	assertNoUntyped(t, reflect.TypeOf(diagnosticRecord{}))
	assertNoUntyped(t, reflect.TypeOf(diagnosticField{}))
}

func assertNoUntyped(t *testing.T, typ reflect.Type) {
	t.Helper()
	for i := 0; i < typ.NumField(); i++ {
		f := typ.Field(i)
		checkFieldType(t, typ.Name()+"."+f.Name, f.Type)
	}
}

func checkFieldType(t *testing.T, name string, ft reflect.Type) {
	t.Helper()
	switch ft.Kind() {
	case reflect.Interface:
		t.Errorf("%s is an interface type (%s); untyped payloads are forbidden", name, ft)
	case reflect.Map:
		t.Errorf("%s is a map type (%s); arbitrary untyped data is forbidden", name, ft)
	case reflect.Slice, reflect.Array:
		if ft.Elem().Kind() == reflect.Uint8 {
			t.Errorf("%s is a raw byte payload (%s); raw bytes are forbidden", name, ft)
		}
		if ft.Elem().Kind() == reflect.Struct {
			for i := 0; i < ft.Elem().NumField(); i++ {
				sf := ft.Elem().Field(i)
				checkFieldType(t, name+"[]."+sf.Name, sf.Type)
			}
		} else {
			checkFieldType(t, name+"[]", ft.Elem())
		}
	}
}
