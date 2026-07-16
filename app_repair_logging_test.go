package main

import (
	"strings"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
)

// withJournal attaches a fresh in-memory journal to an App so the repair
// endpoint's durable events land in the observable tail.
func withJournal(app *App) *App {
	app.journal = newInMemoryDiagnosticJournal()
	return app
}

// safeRepairEventFields fails if any repair event carries a field key outside
// the approved safe-aggregate set, guarding against a future field leaking
// character names, item IDs, handles, or raw errors.
func safeRepairEventFields(t *testing.T, rec diagnosticRecord) {
	t.Helper()
	allowed := map[string]bool{
		"character_index":             true,
		"duplicate_inventory_entries": true,
		"duplicate_physick_entries":   true,
		"outcome":                     true,
		"changed_inventory_indices":   true,
		"attempted_actions":           true,
	}
	for _, f := range rec.Fields {
		if !allowed[f.Key] {
			t.Errorf("event %q leaked unapproved field %q=%q", rec.Event, f.Key, f.Value)
		}
	}
}

// assertRequestedIntentOnly fails unless the requested event carries exactly
// character_index and action: it is journalled before any repair lock, so it
// must describe intent only and never a pre-scan-derived result.
func assertRequestedIntentOnly(t *testing.T, rec diagnosticRecord) {
	t.Helper()
	if got := len(rec.Fields); got != 2 {
		t.Errorf("requested has %d fields, want exactly character_index + action", got)
	}
	if got := operationField(rec, "character_index"); got == "" {
		t.Errorf("requested missing character_index")
	}
	if got := operationField(rec, "action"); got != "repair_duplicates" {
		t.Errorf("action = %q, want repair_duplicates", got)
	}
	for _, f := range rec.Fields {
		if f.Key != "character_index" && f.Key != "action" {
			t.Errorf("requested leaked field %q=%q", f.Key, f.Value)
		}
	}
}

// requestedBeforeFinished fails unless the requested event precedes the
// finished event in journal order — the durable ordering that proves intent
// was recorded before the mutation.
func requestedBeforeFinished(t *testing.T, records []diagnosticRecord) {
	t.Helper()
	var reqSeq, finSeq uint64
	for _, r := range records {
		switch r.Event {
		case "inventory_integrity_repair_requested":
			reqSeq = r.Seq
		case "inventory_integrity_repair_finished":
			finSeq = r.Seq
		}
	}
	if reqSeq == 0 || finSeq == 0 {
		t.Fatalf("missing requested (%d) or finished (%d) event", reqSeq, finSeq)
	}
	if reqSeq >= finSeq {
		t.Errorf("requested seq %d must precede finished seq %d", reqSeq, finSeq)
	}
}

func TestRepairDuplicateInventoryIndices_LogsApplied(t *testing.T) {
	app := withJournal(repairFixture(
		[]core.InventoryItem{
			{GaItemHandle: 0xB0000334, Quantity: 99, Index: 552},
			{GaItemHandle: 0xB00003C0, Quantity: 99, Index: 552},
		},
		nil,
	))

	if _, err := app.RepairDuplicateInventoryIndices(0); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	records := app.journal.Tail()
	requestedBeforeFinished(t, records)
	requested := operationEvent(t, records, "inventory_integrity_repair_requested")
	assertRequestedIntentOnly(t, requested)
	finished := operationEvent(t, records, "inventory_integrity_repair_finished")
	if finished.Level != levelInfo {
		t.Errorf("level = %q, want info", finished.Level)
	}
	if got := operationField(finished, "outcome"); got != "applied" {
		t.Errorf("outcome = %q, want applied", got)
	}
	if got := operationField(finished, "character_index"); got != "0" {
		t.Errorf("character_index = %q, want 0", got)
	}
	if got := operationField(finished, "changed_inventory_indices"); got != "1" {
		t.Errorf("changed_inventory_indices = %q, want 1", got)
	}
	if got := operationField(finished, "duplicate_inventory_entries"); got == "" || got == "0" {
		t.Errorf("duplicate_inventory_entries = %q, want a detected count", got)
	}
	if got := operationField(finished, "attempted_actions"); got != "reassign_duplicate_inventory_indices,remove_duplicate_physick_entries" {
		t.Errorf("attempted_actions = %q, want both steps attempted", got)
	}
	safeRepairEventFields(t, finished)
}

func TestRepairDuplicateInventoryIndices_LogsNoChanges(t *testing.T) {
	app := withJournal(repairFixture(
		[]core.InventoryItem{
			{GaItemHandle: 0xB0000001, Quantity: 1, Index: 100},
			{GaItemHandle: 0xB0000002, Quantity: 1, Index: 101},
		},
		nil,
	))

	if _, err := app.RepairDuplicateInventoryIndices(0); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	records := app.journal.Tail()
	requestedBeforeFinished(t, records)
	assertRequestedIntentOnly(t, operationEvent(t, records, "inventory_integrity_repair_requested"))
	finished := operationEvent(t, records, "inventory_integrity_repair_finished")
	if finished.Level != levelInfo {
		t.Errorf("level = %q, want info", finished.Level)
	}
	if got := operationField(finished, "outcome"); got != "no_changes" {
		t.Errorf("outcome = %q, want no_changes", got)
	}
	if got := operationField(finished, "duplicate_inventory_entries"); got != "0" {
		t.Errorf("duplicate_inventory_entries = %q, want 0", got)
	}
	if got := operationField(finished, "changed_inventory_indices"); got != "0" {
		t.Errorf("changed_inventory_indices = %q, want 0", got)
	}
	if got := operationField(finished, "attempted_actions"); got != "none" {
		t.Errorf("attempted_actions = %q, want none", got)
	}
	safeRepairEventFields(t, finished)
}

func TestRepairDuplicateInventoryIndices_LogsErrorOnInactiveSave(t *testing.T) {
	app := withJournal(NewApp())

	if _, err := app.RepairDuplicateInventoryIndices(0); err == nil {
		t.Fatal("expected error on inactive save")
	}

	records := app.journal.Tail()
	requestedBeforeFinished(t, records)
	assertRequestedIntentOnly(t, operationEvent(t, records, "inventory_integrity_repair_requested"))
	finished := operationEvent(t, records, "inventory_integrity_repair_finished")
	if finished.Level != levelError {
		t.Errorf("level = %q, want error", finished.Level)
	}
	if got := operationField(finished, "outcome"); got != "error" {
		t.Errorf("outcome = %q, want error", got)
	}
	// The error fires before any core repair call is reached, so no step was
	// attempted.
	if got := operationField(finished, "attempted_actions"); got != "none" {
		t.Errorf("attempted_actions = %q, want none", got)
	}
	// The finished event must never carry the raw error text or any other
	// unapproved payload.
	if strings.Contains(finished.Message, "no save loaded") {
		t.Errorf("finished message leaked raw error: %q", finished.Message)
	}
	safeRepairEventFields(t, finished)
}

func TestRepairAttemptedActions(t *testing.T) {
	cases := []struct {
		name                       string
		attemptReassign, attPhysic bool
		want                       string
	}{
		{"none started", false, false, "none"},
		{"first step only (error after reassign)", true, false, "reassign_duplicate_inventory_indices"},
		{"both started", true, true, "reassign_duplicate_inventory_indices,remove_duplicate_physick_entries"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := repairAttemptedActions(tc.attemptReassign, tc.attPhysic); got != tc.want {
				t.Errorf("repairAttemptedActions(%v,%v) = %q, want %q", tc.attemptReassign, tc.attPhysic, got, tc.want)
			}
		})
	}
}
