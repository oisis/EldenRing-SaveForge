package main

import "testing"

// gameItemRecords returns only the game_items_change_* records from a tail, in
// their original order, filtering out session_started and any other noise.
func gameItemRecords(records []diagnosticRecord) []diagnosticRecord {
	var out []diagnosticRecord
	for _, rec := range records {
		switch rec.Event {
		case eventGameItemsChangeBefore, eventGameItemsChangePlanned, eventGameItemsChangeFinished:
			out = append(out, rec)
		}
	}
	return out
}

func TestGameItemDiagnosticLifecycleEmitsRecordPerFieldPerPhase(t *testing.T) {
	app := newDebugOperationApp(t)

	changes := []characterFieldChange{
		{Field: "runes", Before: "100", After: "999"},
		{Field: "smithing_stone_1", Before: "3", After: "50"},
	}

	app.journalGameItemChangeBefore("add_items", 2, changes)
	app.journalGameItemChangePlanned("add_items", 2, changes)
	app.journalGameItemChangeFinished("add_items", 2, characterChangeSuccess, "write", changes)

	records := gameItemRecords(app.journal.Tail())
	if len(records) != 6 {
		t.Fatalf("game item record count = %d, want 6 (3 phases x 2 fields)", len(records))
	}

	// Phase grouping: all before records first (in caller order), then all
	// planned, then all finished — each group preserving the caller's field order.
	wantOrder := []struct {
		event, field string
	}{
		{eventGameItemsChangeBefore, "runes"},
		{eventGameItemsChangeBefore, "smithing_stone_1"},
		{eventGameItemsChangePlanned, "runes"},
		{eventGameItemsChangePlanned, "smithing_stone_1"},
		{eventGameItemsChangeFinished, "runes"},
		{eventGameItemsChangeFinished, "smithing_stone_1"},
	}
	for i, want := range wantOrder {
		if records[i].Event != want.event {
			t.Errorf("record %d: event = %q, want %q", i, records[i].Event, want.event)
		}
		if got := operationField(records[i], "field"); got != want.field {
			t.Errorf("record %d: field = %q, want %q", i, got, want.field)
		}
	}

	// Exact field contract per phase, verified on the "runes" field.
	byPhase := map[string]diagnosticRecord{}
	for _, rec := range records {
		if operationField(rec, "field") == "runes" {
			byPhase[rec.Event] = rec
		}
	}

	before := byPhase[eventGameItemsChangeBefore]
	if got := operationField(before, "action"); got != "add_items" {
		t.Errorf("before action = %q, want add_items", got)
	}
	if got := operationField(before, "character_index"); got != "2" {
		t.Errorf("before character_index = %q, want 2", got)
	}
	if got := operationField(before, "before"); got != "100" {
		t.Errorf("before before = %q, want 100", got)
	}
	if got := operationField(before, "after"); got != "" {
		t.Errorf("before phase leaked after = %q", got)
	}
	if got := operationField(before, "outcome"); got != "" {
		t.Errorf("before phase leaked outcome = %q", got)
	}
	if got := operationField(before, "stage"); got != "" {
		t.Errorf("before phase leaked stage = %q", got)
	}

	planned := byPhase[eventGameItemsChangePlanned]
	if got := operationField(planned, "after"); got != "999" {
		t.Errorf("planned after = %q, want 999", got)
	}
	if got := operationField(planned, "outcome"); got != "" {
		t.Errorf("planned phase leaked outcome = %q", got)
	}

	finished := byPhase[eventGameItemsChangeFinished]
	if got := operationField(finished, "after"); got != "999" {
		t.Errorf("finished after = %q, want 999", got)
	}
	if got := operationField(finished, "outcome"); got != string(characterChangeSuccess) {
		t.Errorf("finished outcome = %q, want success", got)
	}
	if got := operationField(finished, "stage"); got != "write" {
		t.Errorf("finished stage = %q, want write", got)
	}
}

func TestGameItemDiagnosticDroppedWhenDebugDisabled(t *testing.T) {
	app := withJournal(NewApp()) // in-memory journal, Debug Mode off by default

	changes := []characterFieldChange{{Field: "runes", Before: "100", After: "999"}}
	app.journalGameItemChangeBefore("add_items", 0, changes)
	app.journalGameItemChangePlanned("add_items", 0, changes)
	app.journalGameItemChangeFinished("add_items", 0, characterChangeSuccess, "write", changes)

	if got := gameItemRecords(app.journal.Tail()); len(got) != 0 {
		t.Fatalf("Debug Mode disabled retained %d game item records, want 0", len(got))
	}
}

func TestGameItemDiagnosticRedactsAccountIdentifier(t *testing.T) {
	app := newDebugOperationApp(t)

	changes := []characterFieldChange{{Field: "note", Before: "psn_online_id=user4242", After: "clean"}}
	app.journalGameItemChangeBefore("add_items", 1, changes)

	records := gameItemRecords(app.journal.Tail())
	if len(records) != 1 {
		t.Fatalf("game item record count = %d, want 1", len(records))
	}
	if got := operationField(records[0], "before"); got != "psn_online_id=[redacted]" {
		t.Errorf("before = %q, want central sanitizer to redact the account identifier", got)
	}
}
