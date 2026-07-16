package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestGetDiagnosticLogTailReturnsSanitizedRecords(t *testing.T) {
	app := NewApp()
	journal := newInMemoryDiagnosticJournal()
	app.journal = journal
	if err := journal.Log(levelError, sourceApp, "save_load_failed", "failed to load /Users/alice/private/ER0000.sl2"); err != nil {
		t.Fatalf("journal.Log: %v", err)
	}

	encoded := app.GetDiagnosticLogTail()
	if strings.Contains(encoded, "alice") || strings.Contains(encoded, "ER0000.sl2") {
		t.Fatalf("console tail leaked a path: %s", encoded)
	}
	var records []diagnosticRecord
	if err := json.Unmarshal([]byte(encoded), &records); err != nil {
		t.Fatalf("GetDiagnosticLogTail returned invalid JSON: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("record count = %d, want 1", len(records))
	}
	if got := records[0].Level; got != levelError {
		t.Errorf("level = %q, want %q", got, levelError)
	}
	if got := records[0].Event; got != "save_load_failed" {
		t.Errorf("event = %q, want save_load_failed", got)
	}
}

func TestGetDiagnosticLogTailNilJournalReturnsEmptyArray(t *testing.T) {
	if got := NewApp().GetDiagnosticLogTail(); got != "[]" {
		t.Errorf("nil journal tail = %q, want empty JSON array", got)
	}
}
