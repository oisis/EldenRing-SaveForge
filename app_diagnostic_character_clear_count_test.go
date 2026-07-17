package main

import (
	"strconv"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
	"github.com/oisis/EldenRing-SaveForge/backend/db"
	"github.com/oisis/EldenRing-SaveForge/backend/vm"
)

// seedNGPlusFlag sets a single NG+ event flag in a slot's Event Flags region so
// a pre-seeded flag state can be built for the Clear Count diagnostics tests.
func seedNGPlusFlag(t *testing.T, slot *core.SaveSlot, flagID uint32, value bool) {
	t.Helper()
	if err := db.SetEventFlag(slot.Data[slot.EventFlagsOffset:], flagID, value); err != nil {
		t.Fatalf("SetEventFlag(%d): %v", flagID, err)
	}
}

// assertSideEffectPhases asserts a single logical field emits the exact
// before/planned/finished lifecycle with the given values and terminal
// outcome/stage.
func assertSideEffectPhases(t *testing.T, records []diagnosticRecord, field, before, planned, final string, outcome characterChangeOutcome, stage string) {
	t.Helper()
	phases := saveCharacterPhases(records, field)
	if len(phases) != 3 {
		t.Fatalf("field %q: got %d records, want 3", field, len(phases))
	}
	wantEvents := []string{eventCharacterChangeBefore, eventCharacterChangePlanned, eventCharacterChangeFinished}
	for i, rec := range phases {
		if rec.Event != wantEvents[i] {
			t.Errorf("field %q phase %d: event = %q, want %q", field, i, rec.Event, wantEvents[i])
		}
		if got := operationField(rec, "before"); got != before {
			t.Errorf("field %q phase %d: before = %q, want %q", field, i, got, before)
		}
	}
	if got := operationField(phases[0], "after"); got != "" {
		t.Errorf("field %q before phase leaked after = %q", field, got)
	}
	if got := operationField(phases[1], "after"); got != planned {
		t.Errorf("field %q planned after = %q, want %q", field, got, planned)
	}
	finished := phases[2]
	if got := operationField(finished, "after"); got != final {
		t.Errorf("field %q finished after = %q, want %q (real final)", field, got, final)
	}
	if got := operationField(finished, "outcome"); got != string(outcome) {
		t.Errorf("field %q finished outcome = %q, want %q", field, got, outcome)
	}
	if got := operationField(finished, "stage"); got != stage {
		t.Errorf("field %q finished stage = %q, want %q", field, got, stage)
	}
}

// TestSaveCharacterClearCountDiagnosticFlagsChanged raises the clear count on a
// slot whose NG+ flags reflect an earlier cycle and asserts every changed NG+
// flag logs through before/planned/finished with exact true/false values and
// real final state, while unchanged flags stay silent.
func TestSaveCharacterClearCountDiagnosticFlagsChanged(t *testing.T) {
	app := remembranceGameLimitsFixture()
	slot := &app.save.Slots[0]
	withEventFlagsRegion(slot)
	// Pre-seed the canonical NG cycle 0 flag, then move to cycle 3: flag 50 must
	// clear and flag 53 must set; every other flag already matches its target.
	seedNGPlusFlag(t, slot, 50, true)
	enableDebugJournal(t, app)

	charVM, err := app.GetCharacter(0)
	if err != nil {
		t.Fatalf("GetCharacter: %v", err)
	}
	charVM.ClearCount = 3

	if err := app.SaveCharacter(0, *charVM); err != nil {
		t.Fatalf("SaveCharacter: %v", err)
	}

	records := characterRecords(app.journal.Tail())

	assertSideEffectPhases(t, records, "ng_plus_flag_50", "true", "false", "false", characterChangeSuccess, characterStageCompleted)
	assertSideEffectPhases(t, records, "ng_plus_flag_53", "false", "true", "true", characterChangeSuccess, characterStageCompleted)

	// Flags whose target already matched the current state emit nothing.
	for _, field := range []string{"ng_plus_flag_51", "ng_plus_flag_52", "ng_plus_flag_54", "ng_plus_flag_55", "ng_plus_flag_56", "ng_plus_flag_57"} {
		if got := saveCharacterPhases(records, field); len(got) != 0 {
			t.Errorf("field %q emitted %d records, want 0 (unchanged)", field, len(got))
		}
	}

	assertPhaseGrouping(t, records)
}

// TestSaveCharacterClearCountDiagnosticFlagsAlreadyMatch re-saves a slot whose
// NG+ flags already match its clear count: no NG+ flag records may be emitted.
func TestSaveCharacterClearCountDiagnosticFlagsAlreadyMatch(t *testing.T) {
	app := remembranceGameLimitsFixture()
	slot := &app.save.Slots[0]
	withEventFlagsRegion(slot)
	// Clear count 0 with flag 50 already set is canonical: nothing changes.
	seedNGPlusFlag(t, slot, 50, true)
	enableDebugJournal(t, app)

	charVM, err := app.GetCharacter(0)
	if err != nil {
		t.Fatalf("GetCharacter: %v", err)
	}
	charVM.ClearCount = 0 // unchanged, already canonical

	if err := app.SaveCharacter(0, *charVM); err != nil {
		t.Fatalf("SaveCharacter: %v", err)
	}

	records := characterRecords(app.journal.Tail())
	for i := uint32(50); i <= 57; i++ {
		field := "ng_plus_flag_" + itoaU32(i)
		if got := saveCharacterPhases(records, field); len(got) != 0 {
			t.Errorf("field %q emitted %d records, want 0 (flags already match)", field, len(got))
		}
	}
}

// TestSaveCharacterClearCountDiagnosticNoEventFlagsRegion changes the clear
// count on a slot with no valid Event Flags region: the writer cannot touch NG+
// flags, so none may be logged (no record may claim an unapplied flag change).
func TestSaveCharacterClearCountDiagnosticNoEventFlagsRegion(t *testing.T) {
	app := remembranceGameLimitsFixture()
	slot := &app.save.Slots[0]
	// Deliberately no withEventFlagsRegion: EventFlagsOffset stays 0 (invalid).
	if hasEventFlagsRegion(slot) {
		t.Fatal("fixture unexpectedly exposes a valid Event Flags region")
	}
	enableDebugJournal(t, app)

	charVM, err := app.GetCharacter(0)
	if err != nil {
		t.Fatalf("GetCharacter: %v", err)
	}
	charVM.ClearCount = 3 // changes the clear count, but flags cannot be written

	if err := app.SaveCharacter(0, *charVM); err != nil {
		t.Fatalf("SaveCharacter: %v", err)
	}

	records := characterRecords(app.journal.Tail())
	for i := uint32(50); i <= 57; i++ {
		field := "ng_plus_flag_" + itoaU32(i)
		if got := saveCharacterPhases(records, field); len(got) != 0 {
			t.Errorf("field %q emitted %d records, want 0 (no valid Event Flags region)", field, len(got))
		}
	}
}

// TestSaveCharacterClearCountDiagnosticProfileSummary changes level and name and
// asserts the ProfileSummary mirror fields log exact before/planned/finished
// values reflecting what SaveCharacter assigns.
func TestSaveCharacterClearCountDiagnosticProfileSummary(t *testing.T) {
	app := remembranceGameLimitsFixture()
	enableDebugJournal(t, app)

	charVM, err := app.GetCharacter(0)
	if err != nil {
		t.Fatalf("GetCharacter: %v", err)
	}
	charVM.Level = 120
	charVM.Name = "Tarnished"

	if err := app.SaveCharacter(0, *charVM); err != nil {
		t.Fatalf("SaveCharacter: %v", err)
	}

	records := characterRecords(app.journal.Tail())
	assertSideEffectPhases(t, records, "profile_summary_level", "0", "120", "120", characterChangeSuccess, characterStageCompleted)
	assertSideEffectPhases(t, records, "profile_summary_name", "", "Tarnished", "Tarnished", characterChangeSuccess, characterStageCompleted)

	// The ProfileSummary itself must actually carry the assigned values.
	if got := app.save.ProfileSummaries[0].Level; got != 120 {
		t.Errorf("ProfileSummary level = %d, want 120", got)
	}
	if got := core.UTF16ToString(app.save.ProfileSummaries[0].CharacterName[:]); got != "Tarnished" {
		t.Errorf("ProfileSummary name = %q, want Tarnished", got)
	}
}

// TestSaveCharacterClearCountDiagnosticForcedFailure forces an ApplyVM failure
// before the NG+ flag / ProfileSummary writers run and asserts those planned
// fields finish with outcome=error and their real (unchanged) post-error values
// — never claiming the side effects were applied.
func TestSaveCharacterClearCountDiagnosticForcedFailure(t *testing.T) {
	const badHandle = uint32(0xB0001234)

	app := NewApp()
	enableDebugJournal(t, app)
	app.save = &core.SaveFile{}
	slot := &app.save.Slots[0]
	slot.Version = 1
	slot.MagicOffset = 16
	// commonStart = MagicOffset + 505 = 521; the inventory row writes qty at 525,
	// one byte past this buffer → ApplyVMToParsedSlot fails before the NG+ flag
	// and ProfileSummary writers run.
	slot.Data = make([]byte, 524)
	slot.GaMap = map[uint32]uint32{}
	// Valid Event Flags region (zeroed) so NG+ flags stay in scope; the write
	// fails before any flag is set, so every flag stays false through the failure.
	slot.EventFlagsOffset = 256
	slot.Inventory.CommonItems = []core.InventoryItem{{GaItemHandle: badHandle, Quantity: 1}}

	charVM := &vm.CharacterViewModel{
		Name:       "Doomed",
		Level:      50,
		ClearCount: 3, // would set NG+ flag 53
		Inventory:  []vm.ItemViewModel{{Handle: badHandle, Quantity: 5}},
	}

	if err := app.SaveCharacter(0, *charVM); err == nil {
		t.Fatalf("SaveCharacter: expected failure, got nil")
	}

	records := characterRecords(app.journal.Tail())

	// NG+ flag 53 was planned to set but the writer never ran → still false.
	assertSideEffectPhases(t, records, "ng_plus_flag_53", "false", "true", "false", characterChangeError, characterStageApplyVM)
	// ProfileSummary never updated on the failure path → real post-error values
	// equal the unchanged before values.
	assertSideEffectPhases(t, records, "profile_summary_level", "0", "50", "0", characterChangeError, characterStageApplyVM)
	assertSideEffectPhases(t, records, "profile_summary_name", "", "Doomed", "", characterChangeError, characterStageApplyVM)

	// The ProfileSummary must not have been mutated.
	if got := app.save.ProfileSummaries[0].Level; got != 0 {
		t.Errorf("ProfileSummary level = %d, want 0 (never applied)", got)
	}
}

// itoaU32 renders a uint32 as base-10, matching the field-name suffix format.
func itoaU32(v uint32) string {
	return strconv.FormatUint(uint64(v), 10)
}
