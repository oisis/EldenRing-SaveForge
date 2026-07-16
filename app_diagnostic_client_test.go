package main

import (
	"strings"
	"testing"
)

func TestRecordDiagnosticClientErrorIsDurableAndSanitized(t *testing.T) {
	app := NewApp()
	app.journal = newInMemoryDiagnosticJournal()
	app.RecordDiagnosticClientError("unhandled_rejection", "TypeError", "request to https://internal.example/private failed token=secret-value")

	records := app.journal.Tail()
	if len(records) != 1 {
		t.Fatalf("record count = %d, want 1", len(records))
	}
	record := records[0]
	if record.Level != levelError || record.Source != "frontend" || record.Event != "frontend_unhandled_rejection" {
		t.Errorf("unexpected record: level=%q source=%q event=%q", record.Level, record.Source, record.Event)
	}
	if got := operationField(record, "error_type"); got != "TypeError" {
		t.Errorf("error_type = %q, want TypeError", got)
	}
	message := operationField(record, "message")
	for _, forbidden := range []string{"internal.example", "secret-value"} {
		if strings.Contains(message, forbidden) {
			t.Errorf("sanitized client message leaked %q: %q", forbidden, message)
		}
	}
}

func TestRecordDiagnosticClientAssetLoadFailureKeepsValidatedAssetVisible(t *testing.T) {
	app := NewApp()
	app.journal = newInMemoryDiagnosticJournal()
	app.RecordDiagnosticClientAssetLoadFailure("items/tools/fire_pot.png")

	record := operationEvent(t, app.journal.Tail(), "asset_load_failed")
	if record.Level != levelWarn || record.Source != "frontend" {
		t.Errorf("unexpected record: level=%q source=%q", record.Level, record.Source)
	}
	if record.Message != "item icon failed to load" {
		t.Errorf("message = %q, want fixed message", record.Message)
	}
	// The general path sanitizer would redact items/tools/fire_pot.png; the
	// narrow asset exception must keep the validated value readable.
	if got := operationField(record, "asset"); got != "items/tools/fire_pot.png" {
		t.Errorf("asset = %q, want items/tools/fire_pot.png (must survive sanitizer)", got)
	}
}

func TestRecordDiagnosticClientAssetLoadFailureRejectsUnsafeAssets(t *testing.T) {
	unsafe := []string{
		"https://internal.example/items/tools/fire_pot.png", // URL / host
		"/items/tools/fire_pot.png",                         // absolute path
		"items/../secrets/key.png",                          // traversal
		"items/tools/fire_pot.png?token=secret",             // query string
		"items/tools/fire_pot.exe",                          // wrong extension
		"items/tools/Fire_Pot.png",                          // uppercase
		"",                                                  // empty
	}
	for _, asset := range unsafe {
		app := NewApp()
		app.journal = newInMemoryDiagnosticJournal()
		app.RecordDiagnosticClientAssetLoadFailure(asset)
		if got := len(app.journal.Tail()); got != 0 {
			t.Errorf("asset %q produced %d records, want 0", asset, got)
		}
	}
}

func TestAssetSanitizerExceptionDoesNotWeakenPathRedaction(t *testing.T) {
	// A non-asset field carrying an icon-shaped path is still redacted, and an
	// "asset" field whose value fails validation falls through to sanitize.
	fields := sanitizeFields([]diagnosticField{
		field("detail", "items/tools/fire_pot.png"),
		field("asset", "/Users/alice/items/tools/fire_pot.png"),
	})
	if got := fields[0].Value; strings.Contains(got, "fire_pot.png") {
		t.Errorf("non-asset path not redacted: %q", got)
	}
	if got := fields[1].Value; strings.Contains(got, "alice") {
		t.Errorf("invalid asset value not redacted: %q", got)
	}
}

func TestRecordDiagnosticClientErrorBoundsMessageAndNormalizesKind(t *testing.T) {
	app := NewApp()
	app.journal = newInMemoryDiagnosticJournal()
	app.RecordDiagnosticClientError("unexpected", "Error", strings.Repeat("x", maxClientDiagnosticMessageRunes+1))

	record := operationEvent(t, app.journal.Tail(), "frontend_unknown_error")
	if got := []rune(operationField(record, "message")); len(got) != maxClientDiagnosticMessageRunes+1 || got[len(got)-1] != '…' {
		t.Errorf("bounded message length/tail = %d/%q, want %d and ellipsis", len(got), string(got[len(got)-1:]), maxClientDiagnosticMessageRunes+1)
	}
}

func TestRecordDiagnosticClientNavigationIsWhitelistedAndDebugOnly(t *testing.T) {
	app := NewApp()
	journal := newInMemoryDiagnosticJournal()
	app.journal = journal

	app.RecordDiagnosticClientNavigation("main_tab", "character", "tools")
	if got := len(journal.Tail()); got != 0 {
		t.Fatalf("navigation with debug disabled produced %d records, want 0", got)
	}

	journal.SetDebugEnabled(true)
	app.RecordDiagnosticClientNavigation("main_tab", "character", "tools")
	app.RecordDiagnosticClientNavigation("main_tab", "tools", "tools")
	app.RecordDiagnosticClientNavigation("main_tab", "tools", "untrusted input")

	records := journal.Tail()
	if len(records) != 1 {
		t.Fatalf("navigation records = %d, want 1", len(records))
	}
	record := records[0]
	if record.Source != "frontend" || record.Event != "navigation_changed" {
		t.Errorf("record source/event = %q/%q, want frontend/navigation_changed", record.Source, record.Event)
	}
	if got := operationField(record, "scope"); got != "main_tab" {
		t.Errorf("scope = %q, want main_tab", got)
	}
	if got := operationField(record, "from"); got != "character" {
		t.Errorf("from = %q, want character", got)
	}
	if got := operationField(record, "to"); got != "tools" {
		t.Errorf("to = %q, want tools", got)
	}
}
