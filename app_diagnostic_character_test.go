package main

import "testing"

// characterRecords returns only the character_change_* records from a tail, in
// their original order, filtering out session_started and any other noise.
func characterRecords(records []diagnosticRecord) []diagnosticRecord {
	var out []diagnosticRecord
	for _, rec := range records {
		switch rec.Event {
		case eventCharacterChangeBefore, eventCharacterChangePlanned, eventCharacterChangeFinished:
			out = append(out, rec)
		}
	}
	return out
}

func TestCharacterDiagnosticLifecycleEmitsRecordPerFieldPerPhase(t *testing.T) {
	app := newDebugOperationApp(t)

	changes := []characterFieldChange{
		{Field: "vigor", Before: "10", After: "60"},
		{Field: "mind", Before: "12", After: "40"},
	}

	app.journalCharacterChangeBefore("set_stats", 3, changes)
	app.journalCharacterChangePlanned("set_stats", 3, changes)
	app.journalCharacterChangeFinished("set_stats", 3, characterChangeSuccess, "write", changes)

	records := characterRecords(app.journal.Tail())
	if len(records) != 6 {
		t.Fatalf("character record count = %d, want 6 (3 phases x 2 fields)", len(records))
	}

	// Each field must carry exactly its three phases in lifecycle order.
	for _, want := range []struct {
		field, before, after string
	}{
		{"vigor", "10", "60"},
		{"mind", "12", "40"},
	} {
		var phases []diagnosticRecord
		for _, rec := range records {
			if operationField(rec, "field") == want.field {
				phases = append(phases, rec)
			}
		}
		if len(phases) != 3 {
			t.Fatalf("field %q: got %d records, want 3", want.field, len(phases))
		}
		wantEvents := []string{eventCharacterChangeBefore, eventCharacterChangePlanned, eventCharacterChangeFinished}
		for i, rec := range phases {
			if rec.Event != wantEvents[i] {
				t.Errorf("field %q phase %d: event = %q, want %q", want.field, i, rec.Event, wantEvents[i])
			}
			if got := operationField(rec, "action"); got != "set_stats" {
				t.Errorf("field %q phase %d: action = %q, want set_stats", want.field, i, got)
			}
			if got := operationField(rec, "character_index"); got != "3" {
				t.Errorf("field %q phase %d: character_index = %q, want 3", want.field, i, got)
			}
			if got := operationField(rec, "before"); got != want.before {
				t.Errorf("field %q phase %d: before = %q, want %q", want.field, i, got, want.before)
			}
		}

		// "before" phase omits after; planned and finished carry it.
		if got := operationField(phases[0], "after"); got != "" {
			t.Errorf("field %q before phase leaked after = %q", want.field, got)
		}
		if got := operationField(phases[1], "after"); got != want.after {
			t.Errorf("field %q planned after = %q, want %q", want.field, got, want.after)
		}

		finished := phases[2]
		if got := operationField(finished, "after"); got != want.after {
			t.Errorf("field %q finished after = %q, want %q", want.field, got, want.after)
		}
		if got := operationField(finished, "outcome"); got != string(characterChangeSuccess) {
			t.Errorf("field %q finished outcome = %q, want success", want.field, got)
		}
		if got := operationField(finished, "stage"); got != "write" {
			t.Errorf("field %q finished stage = %q, want write", want.field, got)
		}
	}
}

func TestCharacterDiagnosticDroppedWhenDebugDisabled(t *testing.T) {
	app := withJournal(NewApp()) // in-memory journal, Debug Mode off by default

	changes := []characterFieldChange{{Field: "vigor", Before: "10", After: "60"}}
	app.journalCharacterChangeBefore("set_stats", 0, changes)
	app.journalCharacterChangePlanned("set_stats", 0, changes)
	app.journalCharacterChangeFinished("set_stats", 0, characterChangeSuccess, "write", changes)

	if got := characterRecords(app.journal.Tail()); len(got) != 0 {
		t.Fatalf("Debug Mode disabled retained %d character records, want 0", len(got))
	}
}

func TestCharacterDiagnosticRedactsAccountIdentifier(t *testing.T) {
	app := newDebugOperationApp(t)

	changes := []characterFieldChange{{Field: "name", Before: "psn_online_id=user4242", After: "hero"}}
	app.journalCharacterChangeBefore("rename", 1, changes)

	records := characterRecords(app.journal.Tail())
	if len(records) != 1 {
		t.Fatalf("character record count = %d, want 1", len(records))
	}
	if got := operationField(records[0], "before"); got != "psn_online_id=[redacted]" {
		t.Errorf("before = %q, want central sanitizer to redact the account identifier", got)
	}
}
