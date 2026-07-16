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

func TestRecordDiagnosticClientErrorBoundsMessageAndNormalizesKind(t *testing.T) {
	app := NewApp()
	app.journal = newInMemoryDiagnosticJournal()
	app.RecordDiagnosticClientError("unexpected", "Error", strings.Repeat("x", maxClientDiagnosticMessageRunes+1))

	record := operationEvent(t, app.journal.Tail(), "frontend_unknown_error")
	if got := []rune(operationField(record, "message")); len(got) != maxClientDiagnosticMessageRunes+1 || got[len(got)-1] != '…' {
		t.Errorf("bounded message length/tail = %d/%q, want %d and ellipsis", len(got), string(got[len(got)-1:]), maxClientDiagnosticMessageRunes+1)
	}
}
