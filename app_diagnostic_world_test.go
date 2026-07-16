package main

import "testing"

func TestRecordDiagnosticWorldActionUsesClosedSafeContract(t *testing.T) {
	app := withJournal(NewApp())

	app.RecordDiagnosticWorldAction("graces_unlock_all", "requested", 2, 17, 0, "")
	app.RecordDiagnosticWorldAction("graces_unlock_all", "succeeded", 2, 17, 17, "")
	app.RecordDiagnosticWorldAction("graces_unlock_all", "error", 2, 17, -1, "event_flags_unavailable")
	app.RecordDiagnosticWorldAction("untrusted_action", "succeeded", 2, 17, 17, "")
	app.RecordDiagnosticWorldAction("graces_unlock_all", "succeeded", 10, 17, 17, "")
	app.RecordDiagnosticWorldAction("graces_unlock_all", "error", 2, 17, -1, "raw filesystem path")

	records := app.journal.Tail()
	if len(records) != 3 {
		t.Fatalf("record count = %d, want 3", len(records))
	}
	if got := records[0].Event; got != "world_action_requested" {
		t.Errorf("first event = %q, want world_action_requested", got)
	}
	if got := records[1].Event; got != "world_action_finished" {
		t.Errorf("second event = %q, want world_action_finished", got)
	}
	if got := operationField(records[1], "completed_count"); got != "17" {
		t.Errorf("success completed_count = %q, want 17", got)
	}
	if got := records[2].Level; got != levelError {
		t.Errorf("failure level = %q, want error", got)
	}
	if got := operationField(records[2], "completed_count"); got != "unknown" {
		t.Errorf("failure completed_count = %q, want unknown", got)
	}
	if got := operationField(records[2], "reason"); got != "event_flags_unavailable" {
		t.Errorf("failure reason = %q, want event_flags_unavailable", got)
	}
}
