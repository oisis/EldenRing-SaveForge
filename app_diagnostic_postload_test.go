package main

import (
	"strings"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
)

func TestRecordDiagnosticPostLoadDiagnosticsModalShownLogsSafeSummary(t *testing.T) {
	app := withJournal(repairFixture(
		[]core.InventoryItem{
			{GaItemHandle: 0xB0000100, Quantity: 1, Index: 552},
			{GaItemHandle: 0xB0000101, Quantity: 1, Index: 552},
		},
		nil,
	))
	// The generic diagnostics scanner requires a full-size slot before it can
	// reach inventory validation. The fixture's meaningful inventory bytes stay
	// intact at the front of the extended buffer.
	slot := &app.save.Slots[0]
	slot.Data = append(slot.Data, make([]byte, core.SlotSize-len(slot.Data))...)

	app.RecordDiagnosticPostLoadDiagnosticsModalShown()

	rec := operationEvent(t, app.journal.Tail(), "post_load_diagnostics_modal_shown")
	if got := rec.Level; got != levelInfo {
		t.Errorf("level = %q, want info", got)
	}
	if got := operationField(rec, "affected_slots"); got != "1" {
		t.Errorf("affected_slots = %q, want 1", got)
	}
	if got := operationField(rec, "repairable_issue_count"); got == "0" {
		t.Error("repairable_issue_count = 0, want the duplicate inventory issue")
	}
	if got := operationField(rec, "issues"); !strings.Contains(got, "category=inventory,issue=duplicate_inventory_index") {
		t.Errorf("issues = %q, want safe inventory issue code", got)
	}

	allowed := map[string]bool{
		"affected_slots":         true,
		"issue_count":            true,
		"repairable_issue_count": true,
		"critical_count":         true,
		"warning_count":          true,
		"issues":                 true,
		"additional_issues":      true,
	}
	for _, f := range rec.Fields {
		if !allowed[f.Key] {
			t.Errorf("unexpected field %q=%q", f.Key, f.Value)
		}
		for _, forbidden := range []string{"0x", "b000", "552", "handle"} {
			if strings.Contains(strings.ToLower(f.Value), forbidden) {
				t.Errorf("field %q leaked %q in %q", f.Key, forbidden, f.Value)
			}
		}
	}
}

func TestDiagnosticPostLoadIssueFieldsAreBoundedAndRedacted(t *testing.T) {
	issues := make([]core.DiagnosticIssue, 0, diagnosticPostLoadItemsMax+1)
	for i := 0; i < diagnosticPostLoadItemsMax+1; i++ {
		issues = append(issues, core.DiagnosticIssue{
			Severity:    core.SeverityCritical,
			Category:    "gaitem",
			Description: "GaItem handle=0xB0000123 has unknown type prefix 0xF",
		})
	}
	fields := diagnosticPostLoadIssueFields(DiagnosticsReport{Slots: []SlotDiagResult{{SlotIndex: 3, Issues: issues}}})
	rec := diagnosticRecord{Fields: fields}

	if got := operationField(rec, "issue_count"); got != "21" {
		t.Errorf("issue_count = %q, want 21", got)
	}
	if got := operationField(rec, "additional_issues"); got != "1" {
		t.Errorf("additional_issues = %q, want 1", got)
	}
	got := operationField(rec, "issues")
	if count := strings.Count(got, ";") + 1; count != diagnosticPostLoadItemsMax {
		t.Errorf("issues entries = %d, want %d", count, diagnosticPostLoadItemsMax)
	}
	for _, forbidden := range []string{"0x", "b000", "prefix"} {
		if strings.Contains(strings.ToLower(got), forbidden) {
			t.Errorf("issues leaked %q in %q", forbidden, got)
		}
	}
}

func TestRepairAllLoadedSlotsLogsFailureLifecycle(t *testing.T) {
	app := withJournal(NewApp())

	if _, err := app.RepairAllLoadedSlots(); err == nil {
		t.Fatal("RepairAllLoadedSlots succeeded without a loaded save")
	}

	records := app.journal.Tail()
	requested := operationEvent(t, records, "post_load_diagnostics_repair_requested")
	finished := operationEvent(t, records, "post_load_diagnostics_repair_finished")
	if requested.Seq >= finished.Seq {
		t.Errorf("requested seq %d must precede finished seq %d", requested.Seq, finished.Seq)
	}
	if got := operationField(requested, "action"); got != "repair_all_detected_issues" {
		t.Errorf("requested action = %q, want repair_all_detected_issues", got)
	}
	if got := finished.Level; got != levelError {
		t.Errorf("finished level = %q, want error", got)
	}
	if got := operationField(finished, "outcome"); got != "error" {
		t.Errorf("outcome = %q, want error", got)
	}
	if got := operationField(finished, "stage"); got != "precondition" {
		t.Errorf("stage = %q, want precondition", got)
	}
	if got := operationField(finished, "fixed_actions"); got != "none" {
		t.Errorf("fixed_actions = %q, want none", got)
	}
	if got := operationField(finished, "skipped_reasons"); got != "none" {
		t.Errorf("skipped_reasons = %q, want none", got)
	}
}

func TestRepairAllLoadedSlotsLogsResolvedLifecycle(t *testing.T) {
	app := withJournal(repairFixture(
		[]core.InventoryItem{
			{GaItemHandle: 0xB0000100, Quantity: 1, Index: 552},
			{GaItemHandle: 0xB0000101, Quantity: 1, Index: 552},
		},
		nil,
	))

	report, err := app.RepairAllLoadedSlots()
	if err != nil {
		t.Fatalf("RepairAllLoadedSlots: %v", err)
	}
	if len(report.Fixed) == 0 {
		t.Fatal("expected the fixture to produce at least one applied repair")
	}

	records := app.journal.Tail()
	requested := operationEvent(t, records, "post_load_diagnostics_repair_requested")
	finished := operationEvent(t, records, "post_load_diagnostics_repair_finished")
	if requested.Seq >= finished.Seq {
		t.Errorf("requested seq %d must precede finished seq %d", requested.Seq, finished.Seq)
	}
	if got := finished.Level; got != levelInfo {
		t.Errorf("finished level = %q, want info", got)
	}
	if got := operationField(finished, "outcome"); got != "resolved" {
		t.Errorf("outcome = %q, want resolved", got)
	}
	if got := operationField(finished, "processed_slots"); got != "1" {
		t.Errorf("processed_slots = %q, want 1", got)
	}
	if got := operationField(finished, "fixed_actions"); got == "none" || strings.Contains(strings.ToLower(got), "0x") {
		t.Errorf("fixed_actions = %q, want safe applied action codes", got)
	}
}

func TestDiagnosticRepairActionCodeDoesNotCopyCoreMessages(t *testing.T) {
	if got := diagnosticRepairActionCode("duplicate inventory indices: handle=0xB0000100 failed", true); got != "duplicate_inventory_indices_unrepaired" {
		t.Errorf("skipped code = %q", got)
	}
	if got := diagnosticRepairActionCode("Level 999 → 713", false); got != "clamp_level" {
		t.Errorf("fixed code = %q", got)
	}
}
