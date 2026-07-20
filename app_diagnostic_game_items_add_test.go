package main

import (
	"encoding/binary"
	"strings"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
)

// gameItemAddApp builds the fixture-free remembrance app with an in-memory
// journal at the requested debug verbosity.
func gameItemAddApp(debug bool) *App {
	app := remembranceGameLimitsFixture()
	journal := newInMemoryDiagnosticJournal()
	journal.SetDebugEnabled(debug)
	app.journal = journal
	return app
}

// gameItemLifecycleFields collects, per lifecycle event, a field->After map for
// the add_items action, plus the flat index of every record so phase ordering
// can be checked against the journal tail.
type gameItemLifecycle struct {
	before   map[string]string // field -> before value
	planned  map[string]string // field -> planned After value
	finished map[string]string // field -> finished After value
	beforeIx []int
	planIx   []int
	finIx    []int
	outcome  string
	stage    string
}

func collectGameItemLifecycle(t *testing.T, records []diagnosticRecord) gameItemLifecycle {
	t.Helper()
	lc := gameItemLifecycle{
		before:   map[string]string{},
		planned:  map[string]string{},
		finished: map[string]string{},
	}
	for i, rec := range records {
		field := operationField(rec, "field")
		switch rec.Event {
		case eventGameItemsChangeBefore:
			lc.before[field] = operationField(rec, "before")
			lc.beforeIx = append(lc.beforeIx, i)
		case eventGameItemsChangePlanned:
			lc.planned[field] = operationField(rec, "after")
			lc.planIx = append(lc.planIx, i)
		case eventGameItemsChangeFinished:
			lc.finished[field] = operationField(rec, "after")
			lc.finIx = append(lc.finIx, i)
			lc.outcome = operationField(rec, "outcome")
			lc.stage = operationField(rec, "stage")
		default:
			continue
		}
		if got := operationField(rec, "action"); got != actionGameItemsAdd {
			t.Errorf("%s action = %q, want %q", rec.Event, got, actionGameItemsAdd)
		}
		if got := operationField(rec, "character_index"); got != "0" {
			t.Errorf("%s character_index = %q, want 0", rec.Event, got)
		}
	}
	return lc
}

func maxInt(xs []int) int {
	m := xs[0]
	for _, x := range xs {
		if x > m {
			m = x
		}
	}
	return m
}

func minInt(xs []int) int {
	m := xs[0]
	for _, x := range xs {
		if x < m {
			m = x
		}
	}
	return m
}

func TestGameItemsAddLifecycleSuccess(t *testing.T) {
	app := gameItemAddApp(true)

	res, err := app.AddItemsToCharacter(0, []uint32{firePotID}, 0, 0, 0, 0, 2, 0)
	if err != nil {
		t.Fatalf("AddItemsToCharacter: %v", err)
	}
	if res.Added != 1 {
		t.Fatalf("res.Added = %d, want 1", res.Added)
	}

	lc := collectGameItemLifecycle(t, app.journal.Tail())

	if len(lc.beforeIx) == 0 || len(lc.planIx) == 0 || len(lc.finIx) == 0 {
		t.Fatalf("missing a lifecycle phase: before=%d planned=%d finished=%d",
			len(lc.beforeIx), len(lc.planIx), len(lc.finIx))
	}

	// Phase ordering: every before precedes every planned, every planned precedes
	// every finished.
	if maxInt(lc.beforeIx) >= minInt(lc.planIx) {
		t.Errorf("a before record does not precede all planned records")
	}
	if maxInt(lc.planIx) >= minInt(lc.finIx) {
		t.Errorf("a planned record does not precede all finished records")
	}

	// Every changed field has the full before -> planned -> finished lifecycle; on
	// success finished equals the planned (clone) value.
	if len(lc.planned) == 0 {
		t.Fatalf("no planned fields emitted")
	}
	for field, planned := range lc.planned {
		if _, ok := lc.before[field]; !ok {
			t.Errorf("field %q has planned but no before record", field)
		}
		fin, ok := lc.finished[field]
		if !ok {
			t.Errorf("field %q has planned but no finished record", field)
			continue
		}
		if fin != planned {
			t.Errorf("field %q finished %q != planned %q", field, fin, planned)
		}
	}

	if lc.outcome != string(characterChangeSuccess) {
		t.Errorf("outcome = %q, want success", lc.outcome)
	}
	if lc.stage != characterStageCompleted {
		t.Errorf("stage = %q, want completed", lc.stage)
	}

	// Representative direct fields with the required names and formats.
	wantPlanned := map[string]string{
		"inventory_common_row_0_handle":      "0xB000012C",
		"inventory_common_row_0_item_id":     "0x4000012C",
		"inventory_common_row_0_quantity":    "2",
		"inventory_common_row_0_index":       "1001",
		"inventory_next_acquisition_sort_id": "1004",
	}
	for field, want := range wantPlanned {
		if got := lc.planned[field]; got != want {
			t.Errorf("planned %s = %q, want %q", field, got, want)
		}
	}

	// Task 2B non-goal: the Cracked Pot container raised as a side effect of the
	// gated Fire Pot add must NOT appear as a direct row. Prove no lifecycle field
	// carries the Cracked Pot handle (0xB000251C) or item ID (0x4000251C).
	for _, phase := range []map[string]string{lc.before, lc.planned, lc.finished} {
		for field, val := range phase {
			if val == "0xB000251C" || val == "0x4000251C" {
				t.Errorf("derived container leaked into direct record %s = %q", field, val)
			}
		}
	}

	// Regression: the reordered plan build still performs the real add. Verify the
	// actual physical slot, not only the journal.
	slot := &app.save.Slots[0]
	if got := giHex(slot.Inventory.CommonItems[0].GaItemHandle); got != "0xB000012C" {
		t.Errorf("physical row 0 handle = %q, want 0xB000012C", got)
	}
	if got := slot.Inventory.CommonItems[0].Quantity; got != 2 {
		t.Errorf("physical row 0 quantity = %d, want 2", got)
	}
	if got := lc.finished["inventory_common_row_0_index"]; got != giDec(slot.Inventory.CommonItems[0].Index) {
		t.Errorf("finished index %q != real slot index %d", got, slot.Inventory.CommonItems[0].Index)
	}
	if got := giDec(slot.Inventory.NextAcquisitionSortId); got != lc.finished["inventory_next_acquisition_sort_id"] {
		t.Errorf("finished counter %q != real slot %q", lc.finished["inventory_next_acquisition_sort_id"], got)
	}
}

func TestGameItemsAddNewPhysicalRecordFieldsSeparate(t *testing.T) {
	app := gameItemAddApp(true)

	if _, err := app.AddItemsToCharacter(0, []uint32{firePotID}, 0, 0, 0, 0, 2, 0); err != nil {
		t.Fatalf("AddItemsToCharacter: %v", err)
	}
	lc := collectGameItemLifecycle(t, app.journal.Tail())

	// An empty physical row became populated: handle, item_id, quantity and index
	// are each represented as a separate field with the before empty-row values.
	empty := map[string]string{
		"inventory_common_row_0_handle":   "0x00000000",
		"inventory_common_row_0_item_id":  giAbsent,
		"inventory_common_row_0_quantity": "0",
		"inventory_common_row_0_index":    "0",
	}
	for field, wantBefore := range empty {
		before, ok := lc.before[field]
		if !ok {
			t.Errorf("missing before record for %q", field)
			continue
		}
		if before != wantBefore {
			t.Errorf("before %s = %q, want %q (empty row)", field, before, wantBefore)
		}
		if _, ok := lc.planned[field]; !ok {
			t.Errorf("missing planned record for %q", field)
		}
	}

	// No field is emitted for an unchanged scalar: rows the add never touched must
	// not appear.
	for field := range lc.planned {
		if field == "inventory_common_row_2_handle" || field == "inventory_common_row_100_handle" {
			t.Errorf("unchanged row emitted a field: %q", field)
		}
	}
	if _, ok := lc.planned["storage_next_acquisition_sort_id"]; ok {
		t.Errorf("storage counter changed by an inventory-only add: unexpected field")
	}
}

func TestGameItemsAddNoChangeEmitsNoLifecycle(t *testing.T) {
	app := gameItemAddApp(true)

	// A semantic no-change: an empty request executes no item-add mutation plan.
	res, err := app.AddItemsToCharacter(0, []uint32{}, 0, 0, 0, 0, 1, 0)
	if err != nil {
		t.Fatalf("AddItemsToCharacter: %v", err)
	}
	if res.Added != 0 {
		t.Errorf("res.Added = %d, want 0 (no-change contract)", res.Added)
	}

	for _, rec := range app.journal.Tail() {
		switch rec.Event {
		case eventGameItemsChangeBefore, eventGameItemsChangePlanned, eventGameItemsChangeFinished:
			t.Errorf("no-change path emitted a lifecycle record: %s", rec.Event)
		}
	}
}

func TestGameItemsAddDebugDisabledEmitsNoLifecycle(t *testing.T) {
	app := gameItemAddApp(false)

	if _, err := app.AddItemsToCharacter(0, []uint32{firePotID}, 0, 0, 0, 0, 2, 0); err != nil {
		t.Fatalf("AddItemsToCharacter: %v", err)
	}
	for _, rec := range app.journal.Tail() {
		switch rec.Event {
		case eventGameItemsChangeBefore, eventGameItemsChangePlanned, eventGameItemsChangeFinished:
			t.Errorf("debug-disabled run emitted a lifecycle record: %s", rec.Event)
		}
	}
	// The real add still happened.
	if got := app.save.Slots[0].Inventory.CommonItems[0].GaItemHandle; got == core.GaHandleEmpty {
		t.Errorf("debug-disabled add did not mutate the slot")
	}
}

// gaItemAddApp builds the smallest valid in-memory slot that survives a full
// Database Add of a non-stackable weapon: a zero-filled SlotSize buffer with the
// magic anchor placed after an all-empty GaItems table. projectile and unlocked-
// region counts are zero, so the whole dynamic offset chain is fixed-size and
// parses deterministically. No save file is used. The weapon add allocates a real
// serialized GaItem and runs RebuildSlotFull end-to-end.
func gaItemAddApp(t *testing.T, debug bool) *App {
	t.Helper()
	version := uint32(core.GaItemVersionBreak + 1) // > break => GaItemCountNew empty entries
	gaBytes := core.GaItemCountNew * core.GaRecordItem
	magicOffset := core.GaItemsStart + gaBytes + core.DynPlayerData - 1

	data := make([]byte, core.SlotSize)
	binary.LittleEndian.PutUint32(data, version)
	copy(data[magicOffset:], core.MagicPattern)

	var slot core.SaveSlot
	if err := slot.Read(core.NewReader(data), ""); err != nil {
		t.Fatalf("build GaItem fixture: slot.Read: %v", err)
	}

	app := NewApp()
	app.save = &core.SaveFile{}
	app.save.Slots[0] = slot
	journal := newInMemoryDiagnosticJournal()
	journal.SetDebugEnabled(debug)
	app.journal = journal
	return app
}

func TestGameItemsAddGaItemLifecycleIntegration(t *testing.T) {
	app := gaItemAddApp(t, true)

	// Icerind Hatchet (0x00895440) — a non-stackable weapon, so the add path
	// allocates a serialized GaItem record and rebuilds the slot.
	const weaponID = uint32(0x00895440)
	res, err := app.AddItemsToCharacter(0, []uint32{weaponID}, 0, 0, 0, 0, 1, 0)
	if err != nil {
		t.Fatalf("AddItemsToCharacter: %v", err)
	}
	if res.Added != 1 {
		t.Fatalf("res.Added = %d, want 1", res.Added)
	}
	if res.CapHit != "" {
		t.Fatalf("res.CapHit = %q, want empty", res.CapHit)
	}

	// The writer created a real serialized GaItem for the weapon.
	slot := &app.save.Slots[0]
	gaRow := -1
	for i, g := range slot.GaItems {
		if !g.IsEmpty() && g.ItemID == weaponID {
			gaRow = i
			break
		}
	}
	if gaRow < 0 {
		t.Fatalf("no serialized GaItem created for weapon 0x%08X", weaponID)
	}

	lc := collectGameItemLifecycle(t, app.journal.Tail())

	handleField := "gaitem_row_" + giDec(uint32(gaRow)) + "_handle"
	itemField := "gaitem_row_" + giDec(uint32(gaRow)) + "_item_id"

	for _, field := range []string{handleField, itemField} {
		if _, ok := lc.before[field]; !ok {
			t.Errorf("missing before record for %q", field)
		}
		if _, ok := lc.planned[field]; !ok {
			t.Errorf("missing planned record for %q", field)
		}
		if _, ok := lc.finished[field]; !ok {
			t.Errorf("missing finished record for %q", field)
		}
	}

	// finished equals the real post-add slot value.
	if got, want := lc.finished[handleField], giHex(slot.GaItems[gaRow].Handle); got != want {
		t.Errorf("finished %s = %q, want real slot %q", handleField, got, want)
	}
	if got, want := lc.finished[itemField], giHex(slot.GaItems[gaRow].ItemID); got != want {
		t.Errorf("finished %s = %q, want real slot %q", itemField, got, want)
	}
	if got := lc.planned[itemField]; got != giHex(weaponID) {
		t.Errorf("planned %s = %q, want 0x%08X", itemField, got, weaponID)
	}

	if lc.outcome != string(characterChangeSuccess) {
		t.Errorf("outcome = %q, want success", lc.outcome)
	}
	if lc.stage != characterStageCompleted {
		t.Errorf("stage = %q, want completed", lc.stage)
	}
}

// legacyKeyItemCrackedPotApp builds the remembrance fixture with a parse-
// consistent legacy Inventory.KeyItems Cracked Pot record (handle in both the
// KeyItems slice and its binary backing region), so the container sync raises the
// legacy KeyItems row instead of creating a canonical CommonItems container. It
// returns the app and the physical KeyItems row holding Cracked Pot.
func legacyKeyItemCrackedPotApp() (*App, int) {
	app := gameItemAddApp(true)
	slot := &app.save.Slots[0]

	slot.Inventory.KeyItems = make([]core.InventoryItem, core.KeyItemCount)
	for i := range slot.Inventory.KeyItems {
		slot.Inventory.KeyItems[i].GaItemHandle = core.GaHandleEmpty
	}

	const keyRow = 0
	invKeyStart := slot.MagicOffset + core.InvStartFromMagic +
		core.CommonItemCount*core.InvRecordLen + core.InvKeyCountHeader
	off := invKeyStart + keyRow*core.InvRecordLen
	binary.LittleEndian.PutUint32(slot.Data[off:], crackedPotHandle)
	binary.LittleEndian.PutUint32(slot.Data[off+4:], 1)   // start qty 1 so the sync must raise it
	binary.LittleEndian.PutUint32(slot.Data[off+8:], 700) // acquisition index
	slot.Inventory.KeyItems[keyRow] = core.InventoryItem{GaItemHandle: crackedPotHandle, Quantity: 1, Index: 700}

	return app, keyRow
}

// TestGameItemsAddExcludesLegacyKeyItemsContainer proves the derived-container
// exclusion covers the legacy KeyItems container location: adding Fire Pot raises
// the pre-existing Cracked Pot KeyItems row as a gated side effect, and that
// physical row must not surface in the direct-record lifecycle.
func TestGameItemsAddExcludesLegacyKeyItemsContainer(t *testing.T) {
	app, keyRow := legacyKeyItemCrackedPotApp()

	res, err := app.AddItemsToCharacter(0, []uint32{firePotID}, 0, 0, 0, 0, 2, 0)
	if err != nil {
		t.Fatalf("AddItemsToCharacter: %v", err)
	}
	if res.Added != 1 {
		t.Fatalf("res.Added = %d, want 1", res.Added)
	}

	// Prove the legacy KeyItems container path really ran: the row was raised in
	// place (1 -> 2), not relocated to CommonItems.
	slot := &app.save.Slots[0]
	if row, qty := findKeyItem(slot, crackedPotHandle); row != keyRow || qty != 2 {
		t.Fatalf("legacy KeyItems Cracked Pot row=%d qty=%d, want row=%d qty=2", row, qty, keyRow)
	}
	if row, _ := findCommonItem(slot, crackedPotHandle); row >= 0 {
		t.Fatalf("Cracked Pot leaked into CommonItems row %d; legacy KeyItems path not exercised", row)
	}

	lc := collectGameItemLifecycle(t, app.journal.Tail())

	// No direct record for any legacy KeyItems row — the only KeyItems mutation
	// here is the excluded derived container.
	for _, phase := range []map[string]string{lc.before, lc.planned, lc.finished} {
		for field := range phase {
			if strings.HasPrefix(field, "inventory_key_row_") {
				t.Errorf("derived legacy KeyItems container leaked into direct record %q", field)
			}
		}
	}

	// The primary Fire Pot lifecycle is still emitted in full.
	const firePotItemField = "inventory_common_row_0_item_id"
	for _, phase := range []struct {
		name string
		m    map[string]string
	}{{"before", lc.before}, {"planned", lc.planned}, {"finished", lc.finished}} {
		if _, ok := phase.m[firePotItemField]; !ok {
			t.Errorf("missing %s record for primary Fire Pot field %q", phase.name, firePotItemField)
		}
	}
	if got := lc.planned[firePotItemField]; got != giHex(firePotID) {
		t.Errorf("planned %s = %q, want 0x%08X", firePotItemField, got, firePotID)
	}
}

// TestGameItemsAddKeepsExplicitContainerAsPrimary proves a container that is
// explicitly present in plan.batch is NOT filtered as a derived side effect, even
// when the same add makes it a used container. Fire Pot makes Cracked Pot a gated
// (derived) container, while Cracked Pot is also requested directly; its physical
// KeyItems row (native destination per T074 — see nativeKeyItemFamily) must
// keep the full direct-record lifecycle.
func TestGameItemsAddKeepsExplicitContainerAsPrimary(t *testing.T) {
	app := gameItemAddApp(true)

	const crackedPotID = uint32(0x4000251C)
	res, err := app.AddItemsToCharacter(0, []uint32{firePotID, crackedPotID}, 0, 0, 0, 0, 2, 0)
	if err != nil {
		t.Fatalf("AddItemsToCharacter: %v", err)
	}
	if res.Added != 2 {
		t.Fatalf("res.Added = %d, want 2", res.Added)
	}

	// Locate the real physical KeyItems row Cracked Pot landed in.
	slot := &app.save.Slots[0]
	row, qty := findKeyItem(slot, crackedPotHandle)
	if row < 0 {
		t.Fatalf("Cracked Pot not written to KeyItems")
	}

	lc := collectGameItemLifecycle(t, app.journal.Tail())

	prefix := "inventory_key_row_" + giDec(uint32(row)) + "_"
	fields := map[string]string{
		prefix + "handle":   giHex(crackedPotHandle),
		prefix + "item_id":  giHex(crackedPotID),
		prefix + "quantity": giDec(qty),
		prefix + "index":    giDec(slot.Inventory.KeyItems[row].Index),
	}
	for field, want := range fields {
		if _, ok := lc.before[field]; !ok {
			t.Errorf("missing before record for explicit container field %q", field)
		}
		planned, ok := lc.planned[field]
		if !ok {
			t.Errorf("missing planned record for explicit container field %q", field)
		} else if planned != want {
			t.Errorf("planned %s = %q, want %q (real mutation)", field, planned, want)
		}
		finished, ok := lc.finished[field]
		if !ok {
			t.Errorf("missing finished record for explicit container field %q", field)
		} else if finished != want {
			t.Errorf("finished %s = %q, want %q (real mutation)", field, finished, want)
		}
	}

	if lc.outcome != string(characterChangeSuccess) {
		t.Errorf("outcome = %q, want success", lc.outcome)
	}
	if lc.stage != characterStageCompleted {
		t.Errorf("stage = %q, want completed", lc.stage)
	}
}
