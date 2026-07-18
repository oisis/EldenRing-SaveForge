package main

import (
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
	"github.com/oisis/EldenRing-SaveForge/backend/db"
)

func worldChangeRecords(records []diagnosticRecord) []diagnosticRecord {
	var out []diagnosticRecord
	for _, rec := range records {
		switch rec.Event {
		case eventWorldChangeBefore, eventWorldChangePlanned, eventWorldChangeFinished:
			out = append(out, rec)
		}
	}
	return out
}

func TestWorldDiagnosticLifecycleUsesStableSemanticPlans(t *testing.T) {
	app := newDebugOperationApp(t)
	before := &core.SaveSlot{Data: make([]byte, 128), EventFlagsOffset: 16, UnlockedRegions: []uint32{20}}
	planned := core.CloneSlot(before)
	if err := db.SetEventFlag(planned.Data[planned.EventFlagsOffset:], 100, true); err != nil {
		t.Fatalf("SetEventFlag: %v", err)
	}
	planned.UnlockedRegions = []uint32{20, 5}
	plans := worldMutationPlans{
		flags:   planWorldEventFlags(before, planned, []uint32{100, 100}),
		regions: planWorldUnlockedRegions(before, planned, []uint32{5, 20, 5}),
	}

	app.journalWorldMutationBefore("world_test", 2, plans)
	app.journalWorldMutationFinished("world_test", 2, characterChangeSuccess, characterStageCompleted, plans, planned)
	records := worldChangeRecords(app.journal.Tail())
	if len(records) != 6 {
		t.Fatalf("record count=%d, want 6", len(records))
	}
	want := []struct{ event, field, before, after string }{
		{eventWorldChangeBefore, "event_flag_100", "false", ""},
		{eventWorldChangeBefore, "unlocked_region_5", "false", ""},
		{eventWorldChangePlanned, "event_flag_100", "false", "true"},
		{eventWorldChangePlanned, "unlocked_region_5", "false", "true"},
		{eventWorldChangeFinished, "event_flag_100", "false", "true"},
		{eventWorldChangeFinished, "unlocked_region_5", "false", "true"},
	}
	for i, want := range want {
		rec := records[i]
		if rec.Event != want.event || operationField(rec, "field") != want.field {
			t.Errorf("record %d=%q/%q, want %q/%q", i, rec.Event, operationField(rec, "field"), want.event, want.field)
		}
		if got := operationField(rec, "action"); got != "world_test" {
			t.Errorf("record %d action=%q, want world_test", i, got)
		}
		if got := operationField(rec, "character_index"); got != "2" {
			t.Errorf("record %d character_index=%q, want 2", i, got)
		}
		if got := operationField(rec, "before"); got != want.before {
			t.Errorf("record %d before=%q, want %q", i, got, want.before)
		}
		if got := operationField(rec, "after"); got != want.after {
			t.Errorf("record %d after=%q, want %q", i, got, want.after)
		}
	}
	if got := operationField(records[0], "outcome"); got != "" {
		t.Errorf("before outcome=%q, want empty", got)
	}
	if got := operationField(records[4], "outcome"); got != string(characterChangeSuccess) {
		t.Errorf("finished outcome=%q, want success", got)
	}
	if got := operationField(records[4], "stage"); got != characterStageCompleted {
		t.Errorf("finished stage=%q, want completed", got)
	}
}

func TestWorldDiagnosticLifecycleDebugOffEmitsNothing(t *testing.T) {
	app := NewApp()
	app.journal = newInMemoryDiagnosticJournal()
	app.journal.SetDebugEnabled(false)
	plans := worldMutationPlans{flags: []worldFieldPlan{{
		field: "event_flag_100", before: "false", planned: "true", read: func(*core.SaveSlot) string { return "true" },
	}}}
	app.journalWorldMutationBefore("world_test", 0, plans)
	app.journalWorldMutationFinished("world_test", 0, characterChangeSuccess, characterStageCompleted, plans, &core.SaveSlot{})
	if got := len(worldChangeRecords(app.journal.Tail())); got != 0 {
		t.Fatalf("debug-off records=%d, want 0", got)
	}
}
