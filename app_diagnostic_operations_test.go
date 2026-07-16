package main

import (
	"strings"
	"testing"
)

func newDebugOperationApp(t *testing.T) *App {
	t.Helper()
	j, err := newDiagnosticJournalInDir(t.TempDir())
	if err != nil {
		t.Fatalf("newDiagnosticJournalInDir: %v", err)
	}
	j.SetDebugEnabled(true)
	t.Cleanup(func() { _ = j.Close() })
	app := NewApp()
	app.journal = j
	return app
}

func operationEvent(t *testing.T, records []diagnosticRecord, event string) diagnosticRecord {
	t.Helper()
	for _, rec := range records {
		if rec.Event == event {
			return rec
		}
	}
	t.Fatalf("missing event %q in %v", event, eventsOf(records))
	return diagnosticRecord{}
}

func operationField(rec diagnosticRecord, key string) string {
	for _, f := range rec.Fields {
		if f.Key == key {
			return f.Value
		}
	}
	return ""
}

func TestDebugOperationEventsAreSafeAndStructured(t *testing.T) {
	app := newDebugOperationApp(t)

	// Deliberately use values that must never escape into the journal.
	_, _ = app.LoadSaveFromPath("/Users/alice/private/ER0000.sl2")
	_ = app.WriteSave() // no active save: avoids a runtime file dialog in the test.
	_, _ = app.DeploySave("internal-host.example")
	_, _ = app.AddItemsToCharacter(3, []uint32{100, 200}, 0, 0, 0, 0, 1, 0)

	records := app.journal.Tail()
	loadFailed := operationEvent(t, records, "save_load_failed")
	if got := operationField(loadFailed, "stage"); got != "parse" {
		t.Errorf("save_load_failed stage = %q, want parse", got)
	}
	writeFailed := operationEvent(t, records, "save_write_failed")
	if got := operationField(writeFailed, "stage"); got != "no_active_save" {
		t.Errorf("save_write_failed stage = %q, want no_active_save", got)
	}
	deployFailed := operationEvent(t, records, "deploy_save_failed")
	if got := operationField(deployFailed, "stage"); got != "configuration" {
		t.Errorf("deploy_save_failed stage = %q, want configuration", got)
	}
	itemsFinished := operationEvent(t, records, "items_add_finished")
	if got := operationField(itemsFinished, "outcome"); got != "error" {
		t.Errorf("items_add_finished outcome = %q, want error", got)
	}
	if got := operationField(itemsFinished, "requested"); got != "" {
		t.Errorf("items_add_finished must not duplicate request payload, got %q", got)
	}

	for _, rec := range records {
		serialized := rec.Event + " " + rec.Message
		for _, f := range rec.Fields {
			serialized += " " + f.Key + "=" + f.Value
		}
		for _, forbidden := range []string{"alice", "ER0000.sl2", "internal-host.example"} {
			if strings.Contains(serialized, forbidden) {
				t.Errorf("event %q leaked %q: %s", rec.Event, forbidden, serialized)
			}
		}
	}
}
