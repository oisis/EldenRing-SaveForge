package main

import (
	"strconv"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
	"github.com/oisis/EldenRing-SaveForge/backend/vm"
)

// enableDebugJournal attaches an in-directory diagnostic journal with Debug Mode
// on to an existing App, so SaveCharacter's character_change_* records survive
// the journal's verbosity gate.
func enableDebugJournal(t *testing.T, app *App) {
	t.Helper()
	j, err := newDiagnosticJournalInDir(t.TempDir())
	if err != nil {
		t.Fatalf("newDiagnosticJournalInDir: %v", err)
	}
	j.SetDebugEnabled(true)
	t.Cleanup(func() { _ = j.Close() })
	app.journal = j
}

// saveCharacterPhases returns the before/planned/finished records for a single
// logical field, in lifecycle order.
func saveCharacterPhases(records []diagnosticRecord, field string) []diagnosticRecord {
	var out []diagnosticRecord
	for _, rec := range records {
		switch rec.Event {
		case eventCharacterChangeBefore, eventCharacterChangePlanned, eventCharacterChangeFinished:
			if operationField(rec, "field") == field {
				out = append(out, rec)
			}
		}
	}
	return out
}

// TestSaveCharacterDiagnosticSuccessLogsEveryChangedField drives a real
// SaveCharacter that changes every in-scope field (including the three
// normalized ones) and asserts exact before/planned/finished records.
func TestSaveCharacterDiagnosticSuccessAllFields(t *testing.T) {
	app := remembranceGameLimitsFixture()
	enableDebugJournal(t, app)

	charVM, err := app.GetCharacter(0)
	if err != nil {
		t.Fatalf("GetCharacter: %v", err)
	}
	// Fresh fixture slot starts at zero/empty for every field, so each of these
	// differs from the current value.
	charVM.Name = "TestHero"
	charVM.Class = 5
	charVM.Level = 100
	charVM.Souls = 500000
	charVM.SoulMemory = 1 // below the level-100 floor → normalized upward
	charVM.Vigor = 40
	charVM.Mind = 30
	charVM.Endurance = 25
	charVM.Strength = 50
	charVM.Dexterity = 45
	charVM.Intelligence = 20
	charVM.Faith = 15
	charVM.Arcane = 10
	charVM.TalismanSlots = 10 // clamps to 3
	charVM.ClearCount = 99    // clamps to 7

	if err := app.SaveCharacter(0, *charVM); err != nil {
		t.Fatalf("SaveCharacter: %v", err)
	}

	wantPlanned := map[string]string{
		"name":           "TestHero",
		"class":          "5",
		"level":          "100",
		"souls":          "500000",
		"soul_memory":    strconv.FormatUint(uint64(vm.NormalizeSoulMemory(100, 1)), 10),
		"vigor":          "40",
		"mind":           "30",
		"endurance":      "25",
		"strength":       "50",
		"dexterity":      "45",
		"intelligence":   "20",
		"faith":          "15",
		"arcane":         "10",
		"talisman_slots": "3",
		"clear_count":    "7",
	}

	records := characterRecords(app.journal.Tail())
	if len(records) != len(wantPlanned)*3 {
		t.Fatalf("record count = %d, want %d (3 phases x %d fields)", len(records), len(wantPlanned)*3, len(wantPlanned))
	}

	for field, planned := range wantPlanned {
		phases := saveCharacterPhases(records, field)
		if len(phases) != 3 {
			t.Fatalf("field %q: got %d records, want 3", field, len(phases))
		}
		wantEvents := []string{eventCharacterChangeBefore, eventCharacterChangePlanned, eventCharacterChangeFinished}
		for i, rec := range phases {
			if rec.Event != wantEvents[i] {
				t.Errorf("field %q phase %d: event = %q, want %q", field, i, rec.Event, wantEvents[i])
			}
			if got := operationField(rec, "action"); got != actionSaveCharacter {
				t.Errorf("field %q phase %d: action = %q, want %q", field, i, got, actionSaveCharacter)
			}
			if got := operationField(rec, "character_index"); got != "0" {
				t.Errorf("field %q phase %d: character_index = %q, want 0", field, i, got)
			}
		}

		// before phase carries the old value and omits after.
		wantBefore := "0"
		if field == "name" {
			wantBefore = ""
		}
		if got := operationField(phases[0], "before"); got != wantBefore {
			t.Errorf("field %q before = %q, want %q", field, got, wantBefore)
		}
		if got := operationField(phases[0], "after"); got != "" {
			t.Errorf("field %q before phase leaked after = %q", field, got)
		}
		// planned phase: old value + normalized target.
		if got := operationField(phases[1], "after"); got != planned {
			t.Errorf("field %q planned after = %q, want %q", field, got, planned)
		}
		// finished phase: actual applied value must equal the normalized target
		// (no drift) and report success/completed.
		finished := phases[2]
		if got := operationField(finished, "after"); got != planned {
			t.Errorf("field %q finished after = %q, want %q (planned/finished drift)", field, got, planned)
		}
		if got := operationField(finished, "outcome"); got != string(characterChangeSuccess) {
			t.Errorf("field %q finished outcome = %q, want success", field, got)
		}
		if got := operationField(finished, "stage"); got != characterStageCompleted {
			t.Errorf("field %q finished stage = %q, want %q", field, got, characterStageCompleted)
		}
	}
}

// TestSaveCharacterDiagnosticUnchangedFieldEmitsNothing changes exactly one
// field; every other in-scope field must stay silent.
func TestSaveCharacterDiagnosticUnchangedFieldEmitsNothing(t *testing.T) {
	app := remembranceGameLimitsFixture()
	enableDebugJournal(t, app)

	charVM, err := app.GetCharacter(0)
	if err != nil {
		t.Fatalf("GetCharacter: %v", err)
	}
	charVM.Vigor = 60 // the only change; everything else stays at its current value

	if err := app.SaveCharacter(0, *charVM); err != nil {
		t.Fatalf("SaveCharacter: %v", err)
	}

	records := characterRecords(app.journal.Tail())
	if len(records) != 3 {
		t.Fatalf("record count = %d, want 3 (only vigor)", len(records))
	}
	for _, rec := range records {
		if got := operationField(rec, "field"); got != "vigor" {
			t.Errorf("unexpected record for field %q, want only vigor", got)
		}
	}
	if got := operationField(saveCharacterPhases(records, "vigor")[2], "after"); got != "60" {
		t.Errorf("vigor finished after = %q, want 60", got)
	}
}

// TestSaveCharacterDiagnosticFailureReportsRealValues forces an ApplyVM failure
// (inventory write past the end of a too-small slot) and asserts every planned
// field gets a finished:error record with the real post-failure value and the
// closed apply_vm stage.
func TestSaveCharacterDiagnosticFailureReportsRealValues(t *testing.T) {
	const badHandle = uint32(0xB0001234)

	app := NewApp()
	enableDebugJournal(t, app)
	app.save = &core.SaveFile{}
	slot := &app.save.Slots[0]
	slot.Version = 1
	slot.MagicOffset = 16
	// commonStart = MagicOffset + 505 = 521; the first matching inventory row
	// writes qty at 521+4 = 525, one byte past this buffer → bounded write fails.
	slot.Data = make([]byte, 524)
	slot.GaMap = map[uint32]uint32{}
	slot.Inventory.CommonItems = []core.InventoryItem{{GaItemHandle: badHandle, Quantity: 1}}

	charVM := &vm.CharacterViewModel{
		Name:      "Doomed",
		Level:     50,
		Vigor:     33,
		Inventory: []vm.ItemViewModel{{Handle: badHandle, Quantity: 5}},
	}

	if err := app.SaveCharacter(0, *charVM); err == nil {
		t.Fatalf("SaveCharacter: expected failure, got nil")
	}

	records := characterRecords(app.journal.Tail())
	finishedByField := map[string]diagnosticRecord{}
	plannedFields := map[string]bool{}
	for _, rec := range records {
		field := operationField(rec, "field")
		switch rec.Event {
		case eventCharacterChangePlanned:
			plannedFields[field] = true
		case eventCharacterChangeFinished:
			finishedByField[field] = rec
		}
	}
	if len(plannedFields) == 0 {
		t.Fatal("no planned fields recorded")
	}
	// ApplyVMToParsedSlot applies the scalar fields before the inventory write
	// fails, so the real post-failure value equals the applied value.
	wantAfter := map[string]string{"name": "Doomed", "level": "50", "vigor": "33"}
	for field := range plannedFields {
		rec, ok := finishedByField[field]
		if !ok {
			t.Errorf("field %q: planned but no finished record", field)
			continue
		}
		if got := operationField(rec, "outcome"); got != string(characterChangeError) {
			t.Errorf("field %q finished outcome = %q, want error", field, got)
		}
		if got := operationField(rec, "stage"); got != characterStageApplyVM {
			t.Errorf("field %q finished stage = %q, want %q", field, got, characterStageApplyVM)
		}
		if want, known := wantAfter[field]; known {
			if got := operationField(rec, "after"); got != want {
				t.Errorf("field %q finished after = %q, want %q", field, got, want)
			}
		}
	}
	// soul_memory is normalized against level 50 even on the failure path.
	if _, ok := finishedByField["soul_memory"]; !ok {
		t.Error("soul_memory: expected a finished record (level floor bumps it above 0)")
	}
}
