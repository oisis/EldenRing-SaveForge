package main

import (
	"sort"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
	"github.com/oisis/EldenRing-SaveForge/backend/db"
	"github.com/oisis/EldenRing-SaveForge/backend/db/data"
	"github.com/oisis/EldenRing-SaveForge/backend/vm"
)

// memoryStonesPhases returns the before/planned/finished records for a single
// Memory Stones field, in lifecycle order.
func memoryStonesPhases(records []diagnosticRecord, field string) []diagnosticRecord {
	return saveCharacterPhases(records, field)
}

// withEventFlagsRegion grows a fixture slot's Data with a fresh event-flags
// region big enough to hold every Memory Stones pickup flag (max byte 1309),
// so applyMemoryStonesToSlot can actually set the pickup flags in-place.
func withEventFlagsRegion(slot *core.SaveSlot) {
	slot.EventFlagsOffset = len(slot.Data)
	slot.Data = append(slot.Data, make([]byte, 2048)...)
}

// setMemoryStonePickupFlags sets the first n Memory Stones pickup flags in a
// slot's event-flags region, ascending — mirroring how applyMemoryStonesToSlot
// fills them so a pre-seeded canonical state can be built.
func setMemoryStonePickupFlags(t *testing.T, slot *core.SaveSlot, n int) {
	t.Helper()
	flags := slot.Data[slot.EventFlagsOffset:]
	list := data.BolsteringPickupFlags[memoryStonesItemID]
	sorted := make([]uint32, len(list))
	copy(sorted, list)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	for i := 0; i < n && i < len(sorted); i++ {
		if err := db.SetEventFlag(flags, sorted[i], true); err != nil {
			t.Fatalf("SetEventFlag(%d): %v", sorted[i], err)
		}
	}
}

// TestSaveCharacterMemoryStonesDiagnosticSuccess drives a real SaveCharacter
// that raises Memory Stones from an empty slot to 5 and asserts the before ->
// planned -> finished lifecycle for every applicable in-scope field, with
// normalized planned values and real final values.
func TestSaveCharacterMemoryStonesDiagnosticSuccess(t *testing.T) {
	app := remembranceGameLimitsFixture()
	withEventFlagsRegion(&app.save.Slots[0])
	enableDebugJournal(t, app)

	charVM, err := app.GetCharacter(0)
	if err != nil {
		t.Fatalf("GetCharacter: %v", err)
	}
	charVM.MemoryStones = 5

	if err := app.SaveCharacter(0, *charVM); err != nil {
		t.Fatalf("SaveCharacter: %v", err)
	}

	records := characterRecords(app.journal.Tail())

	// before/planned/finished, three fields — every field changes from an empty
	// slot to a count of 5.
	want := map[string]struct{ before, planned, final string }{
		"memory_stones":                  {"0", "5", "5"},
		"memory_stones_common_quantity":  {memoryStonesAbsent, "5", "5"},
		"memory_stones_pickup_flags_set": {"0", "5", "5"},
	}
	for field, w := range want {
		phases := memoryStonesPhases(records, field)
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
			if got := operationField(rec, "before"); got != w.before {
				t.Errorf("field %q phase %d: before = %q, want %q", field, i, got, w.before)
			}
		}
		if got := operationField(phases[0], "after"); got != "" {
			t.Errorf("field %q before phase leaked after = %q", field, got)
		}
		if got := operationField(phases[1], "after"); got != w.planned {
			t.Errorf("field %q planned after = %q, want %q (normalized)", field, got, w.planned)
		}
		finished := phases[2]
		if got := operationField(finished, "after"); got != w.final {
			t.Errorf("field %q finished after = %q, want %q (real final)", field, got, w.final)
		}
		if got := operationField(finished, "outcome"); got != string(characterChangeSuccess) {
			t.Errorf("field %q finished outcome = %q, want success", field, got)
		}
		if got := operationField(finished, "stage"); got != characterStageCompleted {
			t.Errorf("field %q finished stage = %q, want %q", field, got, characterStageCompleted)
		}
	}

	// All before records must precede all planned records, which must precede
	// all finished records (phase grouping across scalar + Memory Stones plans).
	assertPhaseGrouping(t, records)
}

// TestSaveCharacterMemoryStonesDiagnosticNormalizesAboveMax requests more than
// the game maximum and asserts the planned value is clamped to 8 by the same
// helper the writer uses, with the real final value matching.
func TestSaveCharacterMemoryStonesDiagnosticNormalizesAboveMax(t *testing.T) {
	app := remembranceGameLimitsFixture()
	withEventFlagsRegion(&app.save.Slots[0])
	enableDebugJournal(t, app)

	charVM, err := app.GetCharacter(0)
	if err != nil {
		t.Fatalf("GetCharacter: %v", err)
	}
	charVM.MemoryStones = 99 // clamps to 8

	if err := app.SaveCharacter(0, *charVM); err != nil {
		t.Fatalf("SaveCharacter: %v", err)
	}

	records := characterRecords(app.journal.Tail())
	phases := memoryStonesPhases(records, "memory_stones")
	if len(phases) != 3 {
		t.Fatalf("memory_stones: got %d records, want 3", len(phases))
	}
	if got := operationField(phases[1], "after"); got != "8" {
		t.Errorf("memory_stones planned after = %q, want 8 (clamped)", got)
	}
	if got := operationField(phases[2], "after"); got != "8" {
		t.Errorf("memory_stones finished after = %q, want 8", got)
	}
}

// TestSaveCharacterMemoryStonesDiagnosticExistingCommonRecord starts from a
// canonical count of 3 (Common Items stack + 3 pickup flags) and lowers it to
// 1, asserting the Common Items quantity and pickup-flag count update correctly
// in the logs.
func TestSaveCharacterMemoryStonesDiagnosticExistingCommonRecord(t *testing.T) {
	app := remembranceGameLimitsFixture()
	slot := &app.save.Slots[0]
	withEventFlagsRegion(slot)
	// Seed a canonical count of 3: a Common Items stack + 3 pickup flags.
	slot.Inventory.CommonItems[0].GaItemHandle = memoryStonesHandle
	slot.Inventory.CommonItems[0].Quantity = 3
	setMemoryStonePickupFlags(t, slot, 3)
	enableDebugJournal(t, app)

	charVM, err := app.GetCharacter(0)
	if err != nil {
		t.Fatalf("GetCharacter: %v", err)
	}
	if charVM.MemoryStones != 3 {
		t.Fatalf("seed: effective MemoryStones = %d, want 3", charVM.MemoryStones)
	}
	charVM.MemoryStones = 1

	if err := app.SaveCharacter(0, *charVM); err != nil {
		t.Fatalf("SaveCharacter: %v", err)
	}

	records := characterRecords(app.journal.Tail())
	want := map[string]struct{ before, planned, final string }{
		"memory_stones":                  {"3", "1", "1"},
		"memory_stones_common_quantity":  {"3", "1", "1"},
		"memory_stones_pickup_flags_set": {"3", "1", "1"},
	}
	for field, w := range want {
		phases := memoryStonesPhases(records, field)
		if len(phases) != 3 {
			t.Fatalf("field %q: got %d records, want 3", field, len(phases))
		}
		if got := operationField(phases[0], "before"); got != w.before {
			t.Errorf("field %q before = %q, want %q", field, got, w.before)
		}
		if got := operationField(phases[1], "after"); got != w.planned {
			t.Errorf("field %q planned = %q, want %q", field, got, w.planned)
		}
		if got := operationField(phases[2], "after"); got != w.final {
			t.Errorf("field %q finished = %q, want %q", field, got, w.final)
		}
	}
}

// TestSaveCharacterMemoryStonesDiagnosticLegacyKeyItemsCreatesCommon covers a
// legacy state where the stone lives outside the Common Items stack: the writer
// creates a Common Items record, so memory_stones_common_quantity must go from
// absent to the real created quantity.
func TestSaveCharacterMemoryStonesDiagnosticLegacyKeyItemsCreatesCommon(t *testing.T) {
	app := remembranceGameLimitsFixture()
	slot := &app.save.Slots[0]
	withEventFlagsRegion(slot)
	// Live legacy state: a non-zero Memory Stones Key Items stack, with the
	// stackable handle registered in GaMap exactly as a parsed save carries it.
	// The writer only inspects Common Items before adding, so it creates a fresh
	// Common Items stack and leaves the legacy Key Items record untouched.
	const legacyKeyQty = uint32(5)
	slot.GaMap[memoryStonesHandle] = memoryStonesItemID
	slot.Inventory.KeyItems = []core.InventoryItem{{GaItemHandle: memoryStonesHandle, Quantity: legacyKeyQty}}
	enableDebugJournal(t, app)

	charVM, err := app.GetCharacter(0)
	if err != nil {
		t.Fatalf("GetCharacter: %v", err)
	}
	if charVM.MemoryStones != legacyKeyQty {
		t.Fatalf("seed: effective MemoryStones = %d, want %d (from Key Items)", charVM.MemoryStones, legacyKeyQty)
	}
	charVM.MemoryStones = 4

	if err := app.SaveCharacter(0, *charVM); err != nil {
		t.Fatalf("SaveCharacter: %v", err)
	}

	// The legacy Key Items stack must be preserved verbatim.
	if len(slot.Inventory.KeyItems) != 1 || slot.Inventory.KeyItems[0].GaItemHandle != memoryStonesHandle ||
		slot.Inventory.KeyItems[0].Quantity != legacyKeyQty {
		t.Fatalf("Key Items stack changed: %+v, want single stone qty %d", slot.Inventory.KeyItems, legacyKeyQty)
	}
	// The writer must have created a Common Items stack at the requested count,
	// and the effective count now reflects it (Common Items wins over Key Items).
	if got := readMemoryStonesCommonQuantity(slot); got != "4" {
		t.Fatalf("Common Items quantity = %q, want 4 (record created)", got)
	}
	if got := memoryStonesEffective(slot); got != 4 {
		t.Errorf("effective memory_stones = %d, want 4", got)
	}

	records := characterRecords(app.journal.Tail())
	phases := memoryStonesPhases(records, "memory_stones_common_quantity")
	if len(phases) != 3 {
		t.Fatalf("memory_stones_common_quantity: got %d records, want 3", len(phases))
	}
	if got := operationField(phases[0], "before"); got != memoryStonesAbsent {
		t.Errorf("common_quantity before = %q, want %q", got, memoryStonesAbsent)
	}
	if got := operationField(phases[1], "after"); got != "4" {
		t.Errorf("common_quantity planned = %q, want 4", got)
	}
	if got := operationField(phases[2], "after"); got != "4" {
		t.Errorf("common_quantity finished = %q, want 4 (record actually created)", got)
	}
	if got := operationField(phases[2], "outcome"); got != string(characterChangeSuccess) {
		t.Errorf("common_quantity finished outcome = %q, want success", got)
	}

	// Effective memory_stones lifecycle: 5 (Key Items) -> 4 (created Common stack).
	eff := memoryStonesPhases(records, "memory_stones")
	if len(eff) != 3 {
		t.Fatalf("memory_stones: got %d records, want 3", len(eff))
	}
	if got := operationField(eff[0], "before"); got != "5" {
		t.Errorf("memory_stones before = %q, want 5", got)
	}
	if got := operationField(eff[2], "after"); got != "4" {
		t.Errorf("memory_stones finished = %q, want 4", got)
	}
}

// TestSaveCharacterMemoryStonesDiagnosticNoEventFlagsRegion drives a successful
// Memory Stones update on a slot with no valid Event Flags region: the writer
// cannot touch pickup flags, so memory_stones and Common Items records must
// still be correct while memory_stones_pickup_flags_set emits nothing (no
// success record may claim an unapplied flag-count change).
func TestSaveCharacterMemoryStonesDiagnosticNoEventFlagsRegion(t *testing.T) {
	app := remembranceGameLimitsFixture()
	slot := &app.save.Slots[0]
	// Deliberately no withEventFlagsRegion: EventFlagsOffset stays 0 (invalid).
	if memoryStonesFlagsAvailable(slot) {
		t.Fatal("fixture unexpectedly exposes a valid Event Flags region")
	}
	enableDebugJournal(t, app)

	charVM, err := app.GetCharacter(0)
	if err != nil {
		t.Fatalf("GetCharacter: %v", err)
	}
	charVM.MemoryStones = 5

	if err := app.SaveCharacter(0, *charVM); err != nil {
		t.Fatalf("SaveCharacter: %v", err)
	}

	records := characterRecords(app.journal.Tail())

	// memory_stones and Common Items quantity still lifecycle correctly.
	want := map[string]struct{ before, planned, final string }{
		"memory_stones":                 {"0", "5", "5"},
		"memory_stones_common_quantity": {memoryStonesAbsent, "5", "5"},
	}
	for field, w := range want {
		phases := memoryStonesPhases(records, field)
		if len(phases) != 3 {
			t.Fatalf("field %q: got %d records, want 3", field, len(phases))
		}
		if got := operationField(phases[0], "before"); got != w.before {
			t.Errorf("field %q before = %q, want %q", field, got, w.before)
		}
		if got := operationField(phases[1], "after"); got != w.planned {
			t.Errorf("field %q planned = %q, want %q", field, got, w.planned)
		}
		if got := operationField(phases[2], "after"); got != w.final {
			t.Errorf("field %q finished = %q, want %q", field, got, w.final)
		}
	}

	// pickup flags cannot change without a valid region → no records at all, so
	// no success record can claim an unapplied flag-count change.
	if got := memoryStonesPhases(records, "memory_stones_pickup_flags_set"); len(got) != 0 {
		t.Errorf("memory_stones_pickup_flags_set emitted %d records, want 0 (no valid Event Flags region)", len(got))
	}
}

// TestSaveCharacterMemoryStonesDiagnosticUnchangedEmitsNothing asserts that an
// already-canonical Memory Stones state (Common stack + matching pickup flags,
// re-saved at the same count) emits no Memory Stones records.
func TestSaveCharacterMemoryStonesDiagnosticUnchangedEmitsNothing(t *testing.T) {
	app := remembranceGameLimitsFixture()
	slot := &app.save.Slots[0]
	withEventFlagsRegion(slot)
	slot.Inventory.CommonItems[0].GaItemHandle = memoryStonesHandle
	slot.Inventory.CommonItems[0].Quantity = 6
	setMemoryStonePickupFlags(t, slot, 6)
	enableDebugJournal(t, app)

	charVM, err := app.GetCharacter(0)
	if err != nil {
		t.Fatalf("GetCharacter: %v", err)
	}
	charVM.MemoryStones = 6 // unchanged, already canonical

	if err := app.SaveCharacter(0, *charVM); err != nil {
		t.Fatalf("SaveCharacter: %v", err)
	}

	records := characterRecords(app.journal.Tail())
	for _, field := range []string{"memory_stones", "memory_stones_common_quantity", "memory_stones_pickup_flags_set"} {
		if got := memoryStonesPhases(records, field); len(got) != 0 {
			t.Errorf("field %q emitted %d records, want 0 (canonical, unchanged)", field, len(got))
		}
	}
}

// TestSaveCharacterMemoryStonesDiagnosticForcedFailure forces a Memory Stones
// write failure (inventory full, no stone stack, desired > 0) and asserts the
// Memory Stones fields finish with outcome=error, stage=memory_stones, and
// their actual post-error values.
func TestSaveCharacterMemoryStonesDiagnosticForcedFailure(t *testing.T) {
	app := NewApp()
	enableDebugJournal(t, app)
	app.save = &core.SaveFile{}
	slot := &app.save.Slots[0]
	slot.Version = 1
	slot.MagicOffset = 16
	slot.Data = make([]byte, 4096)
	slot.GaMap = map[uint32]uint32{}
	// Valid Event Flags region so the pickup-flag field stays in scope; the
	// bytes are zero, so no pickup flag is set. The write fails before the flag
	// block, so the count stays 0 through the failure.
	slot.EventFlagsOffset = 2048
	// Inventory Common Items is full with a single non-stone, non-container item
	// and no empty slot: adding the Memory Stone stack must fail with a short
	// buffer inside applyMemoryStonesToSlot.
	slot.Inventory.CommonItems = []core.InventoryItem{{GaItemHandle: 0x90001234, Quantity: 1}}

	charVM := &vm.CharacterViewModel{MemoryStones: 3}

	if err := app.SaveCharacter(0, *charVM); err == nil {
		t.Fatalf("SaveCharacter: expected Memory Stones failure, got nil")
	}

	records := characterRecords(app.journal.Tail())
	// Post-error the stone stack was never created and no pickup flags were set.
	want := map[string]string{
		"memory_stones":                  "0",
		"memory_stones_common_quantity":  memoryStonesAbsent,
		"memory_stones_pickup_flags_set": "0",
	}
	for field, wantFinal := range want {
		phases := memoryStonesPhases(records, field)
		if len(phases) != 3 {
			t.Fatalf("field %q: got %d records, want 3", field, len(phases))
		}
		finished := phases[2]
		if got := operationField(finished, "outcome"); got != string(characterChangeError) {
			t.Errorf("field %q finished outcome = %q, want error", field, got)
		}
		if got := operationField(finished, "stage"); got != characterStageMemoryStones {
			t.Errorf("field %q finished stage = %q, want %q", field, got, characterStageMemoryStones)
		}
		if got := operationField(finished, "after"); got != wantFinal {
			t.Errorf("field %q finished after = %q, want %q (actual post-error)", field, got, wantFinal)
		}
	}
}

// assertPhaseGrouping verifies every before record precedes every planned
// record, which precedes every finished record.
func assertPhaseGrouping(t *testing.T, records []diagnosticRecord) {
	t.Helper()
	phaseRank := map[string]int{
		eventCharacterChangeBefore:   0,
		eventCharacterChangePlanned:  1,
		eventCharacterChangeFinished: 2,
	}
	last := -1
	for _, rec := range records {
		rank, ok := phaseRank[rec.Event]
		if !ok {
			continue
		}
		if rank < last {
			t.Fatalf("phase ordering violated: %q appeared after a later phase", rec.Event)
		}
		last = rank
	}
}
