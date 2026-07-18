package main

import (
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/db"
	"github.com/oisis/EldenRing-SaveForge/backend/db/data"
)

func TestWorldQuestStepDiagnosticLifecycle(t *testing.T) {
	app := worldFlagsApp(true)
	const npc = "Gatekeeper Gostoc"
	const stepIndex = 0
	step := data.QuestData[npc][stepIndex]
	if len(step.Flags) != 1 {
		t.Fatalf("%s step %d flags = %d, want one stable test target", npc, stepIndex, len(step.Flags))
	}
	flag := step.Flags[0]

	if err := app.SetQuestStep(0, npc, stepIndex); err != nil {
		t.Fatalf("SetQuestStep: %v", err)
	}
	lc := collectWorldLifecycle(t, app.journal.Tail(), actionWorldSetQuestStep)
	assertWorldSuccess(t, lc)
	assertWorldField(t, lc, "event_flag_10009549", "false", "true")
	if got := readContainerFlag(&app.save.Slots[0], flag.ID); got != "true" {
		t.Errorf("finished quest flag = %q, want true", got)
	}
}

func TestWorldQuestActionsReuseMapFlagLifecycleForUnsetAndToggle(t *testing.T) {
	app := worldFlagsApp(true)
	const flagID = uint32(10009549)
	if err := db.SetEventFlag(app.save.Slots[0].Data[app.save.Slots[0].EventFlagsOffset:], flagID, true); err != nil {
		t.Fatalf("seed flag: %v", err)
	}

	if err := app.SetMapFlag(0, flagID, false); err != nil {
		t.Fatalf("quest unset via SetMapFlag: %v", err)
	}
	lc := collectWorldLifecycle(t, app.journal.Tail(), actionWorldSetMapFlag)
	assertWorldSuccess(t, lc)
	assertWorldField(t, lc, "event_flag_10009549", "true", "false")
}

func TestWorldQuestStepDiagnosticNoopAndDebugOff(t *testing.T) {
	const npc = "Gatekeeper Gostoc"
	const stepIndex = 0

	app := worldFlagsApp(true)
	if err := app.SetQuestStep(0, npc, stepIndex); err != nil {
		t.Fatalf("seed SetQuestStep: %v", err)
	}
	app.journal = newInMemoryDiagnosticJournal()
	app.journal.SetDebugEnabled(true)
	if err := app.SetQuestStep(0, npc, stepIndex); err != nil {
		t.Fatalf("noop SetQuestStep: %v", err)
	}
	if got := len(app.journal.Tail()); got != 0 {
		t.Fatalf("noop records = %d, want 0", got)
	}

	app = worldFlagsApp(false)
	if err := app.SetQuestStep(0, npc, stepIndex); err != nil {
		t.Fatalf("debug-off SetQuestStep: %v", err)
	}
	if got := len(app.journal.Tail()); got != 0 {
		t.Fatalf("debug-off records = %d, want 0", got)
	}
}
