package main

import (
	"encoding/binary"
	"fmt"
	"reflect"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
)

// diagnosticRepackApp builds a serializable, fragmented GaItem table entirely
// in memory. Reading the raw slot through the public core API makes this an app
// level fixture: ExecuteGaItemRepack still rebuilds and reparses the candidate
// exactly as it does for a loaded save, without depending on tmp/save.
func diagnosticRepackApp(t *testing.T) *App {
	t.Helper()

	const (
		weaponHandle = uint32(core.ItemTypeWeapon | 0x0002)
		armorHandle  = uint32(core.ItemTypeArmor | 0x0004)
	)
	records := make([]core.GaItemFull, core.GaItemCountNew)
	records[2] = core.GaItemFull{
		Handle:          weaponHandle,
		ItemID:          0x000F4240,
		Unk2:            -7,
		Unk3:            9,
		AoWGaItemHandle: core.NoCustomAoWHandle,
		Unk5:            5,
	}
	records[4] = core.GaItemFull{
		Handle: armorHandle,
		ItemID: 0x10000001,
		Unk2:   11,
		Unk3:   12,
	}

	gaBytes := 0
	for i := range records {
		gaBytes += records[i].ByteSize()
	}
	data := make([]byte, core.SlotSize)
	binary.LittleEndian.PutUint32(data, core.GaItemVersionBreak+1)
	magicOffset := core.GaItemsStart + gaBytes + core.DynPlayerData - 1
	copy(data[magicOffset:], core.MagicPattern)
	pos := core.GaItemsStart
	for i := range records {
		pos += records[i].Serialize(data[pos:])
	}
	if pos != magicOffset-core.DynPlayerData+1 {
		t.Fatalf("GaItem fixture end=0x%X, want 0x%X", pos, magicOffset-core.DynPlayerData+1)
	}

	var slot core.SaveSlot
	if err := slot.Read(core.NewReader(data), string(core.PlatformPC)); err != nil {
		t.Fatalf("SaveSlot.Read: %v", err)
	}
	if preflight := core.PreflightGaItemRepack(&slot); len(preflight.Blockers) != 0 || preflight.Analysis.Recovered == 0 {
		t.Fatalf("fixture preflight=%+v, want safe positive recovery", preflight)
	}

	app := NewApp()
	app.save = &core.SaveFile{}
	app.save.Slots[0] = slot
	app.saveGeneration = 1
	return app
}

func runDiagnosticRepack(t *testing.T, app *App) *core.SaveSlot {
	t.Helper()
	before := core.CloneSlot(&app.save.Slots[0])
	analysis, err := app.AnalyzeGaItemRepack(0)
	if err != nil {
		t.Fatalf("AnalyzeGaItemRepack: %v", err)
	}
	if analysis.Outcome != "ready" || analysis.AnalysisToken == "" || analysis.Recovered <= 0 {
		t.Fatalf("analysis=%+v, want ready token with recovery", analysis)
	}
	result, err := app.ExecuteGaItemRepack(GaItemRepackExecuteRequest{CharacterIndex: 0, AnalysisToken: analysis.AnalysisToken})
	if err != nil {
		t.Fatalf("ExecuteGaItemRepack: %v", err)
	}
	if result.Outcome != "success" || result.After == nil || result.Recovered != analysis.Recovered {
		t.Fatalf("result=%+v, want successful approved repack", result)
	}
	return before
}

func gaItemRepackPhases(records []diagnosticRecord, field string) []diagnosticRecord {
	var out []diagnosticRecord
	for _, rec := range gameItemRecords(records) {
		if operationField(rec, "field") == field {
			out = append(out, rec)
		}
	}
	return out
}

func assertRepackPhases(t *testing.T, records []diagnosticRecord, field, before, planned, final string) {
	t.Helper()
	phases := gaItemRepackPhases(records, field)
	if len(phases) != 3 {
		t.Fatalf("field %q: got %d records, want 3", field, len(phases))
	}
	wantEvents := []string{eventGameItemsChangeBefore, eventGameItemsChangePlanned, eventGameItemsChangeFinished}
	for i, rec := range phases {
		if rec.Event != wantEvents[i] {
			t.Errorf("field %q phase %d: event=%q want %q", field, i, rec.Event, wantEvents[i])
		}
		if got := operationField(rec, "action"); got != actionGameItemsGaItemRepack {
			t.Errorf("field %q phase %d: action=%q want %q", field, i, got, actionGameItemsGaItemRepack)
		}
		if got := operationField(rec, "character_index"); got != "0" {
			t.Errorf("field %q phase %d: character_index=%q want 0", field, i, got)
		}
		if got := operationField(rec, "before"); got != before {
			t.Errorf("field %q phase %d: before=%q want %q", field, i, got, before)
		}
	}
	if got := operationField(phases[0], "after"); got != "" {
		t.Errorf("field %q before leaked after=%q", field, got)
	}
	if got := operationField(phases[0], "outcome"); got != "" {
		t.Errorf("field %q before leaked outcome=%q", field, got)
	}
	if got := operationField(phases[0], "stage"); got != "" {
		t.Errorf("field %q before leaked stage=%q", field, got)
	}
	if got := operationField(phases[1], "after"); got != planned {
		t.Errorf("field %q planned after=%q want %q", field, got, planned)
	}
	if got := operationField(phases[2], "after"); got != final {
		t.Errorf("field %q finished after=%q want %q", field, got, final)
	}
	if got := operationField(phases[2], "outcome"); got != string(characterChangeSuccess) {
		t.Errorf("field %q finished outcome=%q want success", field, got)
	}
	if got := operationField(phases[2], "stage"); got != characterStageCompleted {
		t.Errorf("field %q finished stage=%q want completed", field, got)
	}
}

func TestGaItemRepackDiagnosticSuccessLifecycle(t *testing.T) {
	app := diagnosticRepackApp(t)
	enableDebugJournal(t, app)
	before := runDiagnosticRepack(t, app)
	post := &app.save.Slots[0]
	records := gameItemRecords(app.journal.Tail())

	// The weapon moves from its fragmented row to the compacted prefix. This
	// exercises every serialized GaItemFull field, including signed values and
	// the weapon-only AoW handle / byte field.
	for _, row := range []int{0, 2} {
		for _, field := range []string{"handle", "item_id", "unk2", "unk3", "aow_gaitem_handle", "unk5"} {
			name := fmt.Sprintf("gaitem_row_%d_%s", row, field)
			assertRepackPhases(t, records, name,
				readGameItemField(before, giSecGaItem, row, gaItemKindForField(field)),
				readGameItemField(post, giSecGaItem, row, gaItemKindForField(field)),
				readGameItemField(post, giSecGaItem, row, gaItemKindForField(field)),
			)
		}
	}

	// Repacking preserves GaMap, next-handle, and part-handle. Only an actually
	// moved allocation cursor may emit, so unchanged semantic values self-exclude.
	for _, field := range []string{"gaitem_next_handle", "gaitem_part_handle"} {
		if got := len(gaItemRepackPhases(records, field)); got != 0 {
			t.Errorf("unchanged field %q emitted %d records", field, got)
		}
	}

	last := -1
	phaseOrder := map[string]int{eventGameItemsChangeBefore: 0, eventGameItemsChangePlanned: 1, eventGameItemsChangeFinished: 2}
	for _, rec := range records {
		phase := phaseOrder[rec.Event]
		if phase < last {
			t.Fatalf("phase grouping violated: %q after phase %d", rec.Event, last)
		}
		last = phase
	}
}

func TestGaItemRepackDiagnosticDebugOffEmitsNothing(t *testing.T) {
	app := diagnosticRepackApp(t)
	j, err := newDiagnosticJournalInDir(t.TempDir())
	if err != nil {
		t.Fatalf("newDiagnosticJournalInDir: %v", err)
	}
	j.SetDebugEnabled(false)
	t.Cleanup(func() { _ = j.Close() })
	app.journal = j

	runDiagnosticRepack(t, app)
	if recs := gameItemRecords(app.journal.Tail()); len(recs) != 0 {
		t.Fatalf("debug off emitted %d records, want 0", len(recs))
	}
	if app.save.Slots[0].GaItems[0].IsEmpty() {
		t.Fatal("debug-off repack did not compact the first GaItem row")
	}
}

func TestGaItemRepackDiagnosticStaleEmitsNothing(t *testing.T) {
	app := diagnosticRepackApp(t)
	enableDebugJournal(t, app)
	analysis, err := app.AnalyzeGaItemRepack(0)
	if err != nil {
		t.Fatalf("AnalyzeGaItemRepack: %v", err)
	}
	app.save.Slots[0].Data[0] ^= 0x01
	changed := core.SnapshotSlot(&app.save.Slots[0])

	result, err := app.ExecuteGaItemRepack(GaItemRepackExecuteRequest{CharacterIndex: 0, AnalysisToken: analysis.AnalysisToken})
	if err != nil {
		t.Fatalf("ExecuteGaItemRepack: %v", err)
	}
	if result.Outcome != "could_not_start" || result.Failure == nil || result.Failure.Code != "analysis_stale" {
		t.Fatalf("result=%+v, want stale no-start", result)
	}
	if recs := gameItemRecords(app.journal.Tail()); len(recs) != 0 {
		t.Fatalf("stale execution emitted %d records, want 0", len(recs))
	}
	if !reflect.DeepEqual(core.SnapshotSlot(&app.save.Slots[0]), changed) {
		t.Fatal("stale execution changed the active slot")
	}
}

func gaItemKindForField(field string) giKind {
	switch field {
	case "handle":
		return giHandle
	case "item_id":
		return giItemID
	case "unk2":
		return giGaItemUnk2
	case "unk3":
		return giGaItemUnk3
	case "aow_gaitem_handle":
		return giGaItemAoWHandle
	case "unk5":
		return giGaItemUnk5
	default:
		panic("unknown GaItem field: " + field)
	}
}
