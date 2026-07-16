package main

import "testing"

// countEvents returns how many tail records carry the given event name.
func countEvents(records []diagnosticRecord, event string) int {
	n := 0
	for _, rec := range records {
		if rec.Event == event {
			n++
		}
	}
	return n
}

func debugModeApp() *App {
	app := NewApp()
	app.journal = newInMemoryDiagnosticJournal()
	return app
}

// Two identical true syncs (e.g. React Strict Mode) must emit exactly one
// change event: the first configures the level, the second is a no-op repeat.
func TestSetDiagnosticDebugMode_RepeatedTrue_EmitsOneEvent(t *testing.T) {
	app := debugModeApp()

	app.SetDiagnosticDebugMode(true)
	app.SetDiagnosticDebugMode(true)

	if got := countEvents(app.journal.Tail(), "diagnostic_debug_mode_changed"); got != 1 {
		t.Fatalf("diagnostic_debug_mode_changed count = %d, want 1", got)
	}
}

// A real true→false transition emits two events.
func TestSetDiagnosticDebugMode_RealToggle_EmitsTwoEvents(t *testing.T) {
	app := debugModeApp()

	app.SetDiagnosticDebugMode(true)
	app.SetDiagnosticDebugMode(false)

	records := app.journal.Tail()
	if got := countEvents(records, "diagnostic_debug_mode_changed"); got != 2 {
		t.Fatalf("diagnostic_debug_mode_changed count = %d, want 2", got)
	}
	// The two records must carry the distinct configured values, in order.
	var enabledVals []string
	for _, rec := range records {
		if rec.Event == "diagnostic_debug_mode_changed" {
			enabledVals = append(enabledVals, operationField(rec, "enabled"))
		}
	}
	if len(enabledVals) != 2 || enabledVals[0] != "true" || enabledVals[1] != "false" {
		t.Fatalf("enabled values = %v, want [true false]", enabledVals)
	}
}

// The first configuration is durable even when it is false, so the session log
// records its initial verbosity.
func TestSetDiagnosticDebugMode_FirstFalse_EmitsOneEvent(t *testing.T) {
	app := debugModeApp()

	app.SetDiagnosticDebugMode(false)

	records := app.journal.Tail()
	if got := countEvents(records, "diagnostic_debug_mode_changed"); got != 1 {
		t.Fatalf("diagnostic_debug_mode_changed count = %d, want 1", got)
	}
	rec := operationEvent(t, records, "diagnostic_debug_mode_changed")
	if got := operationField(rec, "enabled"); got != "false" {
		t.Fatalf("enabled = %q, want false", got)
	}
}
