package main

import (
	"encoding/binary"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
	"github.com/oisis/EldenRing-SaveForge/backend/db"
)

// worldRegionsApp builds the minimum full-size active slot required by
// core.SetUnlockedRegions. The core writer recalculates the dynamic offsets and
// rebuilds this buffer, so lifecycle tests exercise the public World API rather
// than a simulated region writer.
func worldRegionsApp(debug bool) *App {
	app := NewApp()
	app.save = &core.SaveFile{}
	slot := &app.save.Slots[0]
	slot.Version = 1
	slot.MagicOffset = 0
	slot.Data = make([]byte, core.SlotSize)
	binary.LittleEndian.PutUint32(slot.Data, 1)
	journal := newInMemoryDiagnosticJournal()
	journal.SetDebugEnabled(debug)
	app.journal = journal
	return app
}

func TestWorldRegionDiagnosticSingleLifecycle(t *testing.T) {
	app := worldRegionsApp(true)
	entries := db.GetAllRegions()
	if len(entries) == 0 {
		t.Fatal("region database is empty")
	}
	regionID := entries[0].ID

	if err := app.SetRegionUnlocked(0, regionID, true); err != nil {
		t.Fatalf("SetRegionUnlocked: %v", err)
	}
	lc := collectWorldLifecycle(t, app.journal.Tail(), actionWorldSetRegionUnlocked)
	assertWorldSuccess(t, lc)
	assertWorldField(t, lc, "unlocked_region_"+giDec(regionID), "false", "true")
	if got := readWorldUnlockedRegion(&app.save.Slots[0], regionID); got != "true" {
		t.Errorf("real slot region state = %q, want true", got)
	}
}

func TestWorldRegionDiagnosticBulkPreservesRawUnknownIDs(t *testing.T) {
	app := worldRegionsApp(true)
	entries := db.GetAllRegions()
	if len(entries) < 2 {
		t.Fatal("region database has fewer than two entries")
	}
	const rawUnknown = uint32(0xDEAD0001)
	if err := core.SetUnlockedRegions(&app.save.Slots[0], []uint32{entries[0].ID, rawUnknown}); err != nil {
		t.Fatalf("seed SetUnlockedRegions: %v", err)
	}
	app.journal = newInMemoryDiagnosticJournal()
	app.journal.SetDebugEnabled(true)

	if err := app.BulkSetUnlockedRegions(0, []uint32{entries[1].ID}); err != nil {
		t.Fatalf("BulkSetUnlockedRegions: %v", err)
	}
	lc := collectWorldLifecycle(t, app.journal.Tail(), actionWorldBulkSetUnlockedRegions)
	assertWorldSuccess(t, lc)
	assertWorldField(t, lc, "unlocked_region_"+giDec(entries[0].ID), "true", "false")
	assertWorldField(t, lc, "unlocked_region_"+giDec(entries[1].ID), "false", "true")
	if got := readWorldUnlockedRegion(&app.save.Slots[0], rawUnknown); got != "true" {
		t.Errorf("raw unknown region state = %q, want preserved true", got)
	}
}

func TestWorldRegionDiagnosticDebugOff(t *testing.T) {
	app := worldRegionsApp(false)
	entries := db.GetAllRegions()
	if err := app.SetRegionUnlocked(0, entries[0].ID, true); err != nil {
		t.Fatalf("SetRegionUnlocked: %v", err)
	}
	if got := len(app.journal.Tail()); got != 0 {
		t.Fatalf("debug-off records = %d, want 0", got)
	}
}
