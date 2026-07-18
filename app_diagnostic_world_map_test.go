package main

import (
	"strings"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
	"github.com/oisis/EldenRing-SaveForge/backend/db"
)

func TestWorldResetMapExplorationDiagnostic(t *testing.T) {
	app := worldFlagsApp(false)
	const visibleFlag = uint32(62010)
	const acquiredFlag = uint32(63010)
	const fragmentItem = uint32(0x40002198)
	if err := app.SetMapRegionFlags(0, visibleFlag, true); err != nil {
		t.Fatalf("seed SetMapRegionFlags: %v", err)
	}
	if err := db.SetEventFlag(app.save.Slots[0].Data[app.save.Slots[0].EventFlagsOffset:], acquiredFlag, true); err != nil {
		t.Fatalf("seed acquired flag: %v", err)
	}
	app.journal = newInMemoryDiagnosticJournal()
	app.journal.SetDebugEnabled(true)

	if err := app.ResetMapExploration(0); err != nil {
		t.Fatalf("ResetMapExploration: %v", err)
	}
	lc := collectWorldLifecycle(t, app.journal.Tail(), actionWorldResetMapExploration)
	assertWorldSuccess(t, lc)
	assertWorldField(t, lc, "event_flag_62010", "true", "false")
	assertWorldField(t, lc, "event_flag_63010", "true", "false")
	assertWorldField(t, lc, "map_fragment_0x40002198_owned", "true", "false")
	if got := readWorldMapFragmentOwned(&app.save.Slots[0], fragmentItem); got != "false" {
		t.Errorf("real fragment ownership = %q, want false", got)
	}
}

func TestWorldRemoveFogOfWarDiagnostic(t *testing.T) {
	app := worldFlagsApp(true)
	slot := &app.save.Slots[0]
	afterRegs, err := resolveAfterRegs(slot)
	if err != nil {
		t.Fatalf("resolveAfterRegs: %v", err)
	}
	start := afterRegs + core.FoWBlobStart
	end := afterRegs + core.FoWBlobEnd
	if end >= len(slot.Data)-0x80 {
		t.Fatalf("test fixture fog range is out of bounds")
	}
	slot.Data[start] = 0x12
	slot.Data[end] = 0x34
	before := readWorldFogOfWar(slot)

	if err := app.RemoveFogOfWar(0); err != nil {
		t.Fatalf("RemoveFogOfWar: %v", err)
	}
	lc := collectWorldLifecycle(t, app.journal.Tail(), actionWorldRemoveFogOfWar)
	assertWorldSuccess(t, lc)
	after := strings.Repeat("ff", end-start+1)
	assertWorldField(t, lc, "fog_of_war_hex", before, after)
	if got := readWorldFogOfWar(slot); got != after {
		t.Errorf("real fog data does not match finished lifecycle value")
	}
}

func TestWorldRevealAllMapDiagnostic(t *testing.T) {
	app := worldFlagsApp(true)
	// Reveal All changes hundreds of semantic World fields; retain the complete
	// three-phase lifecycle instead of the production journal's short support
	// tail so this test can assert global phase grouping.
	app.journal.tailMax = 2048
	if err := app.RevealAllMap(0); err != nil {
		t.Fatalf("RevealAllMap: %v", err)
	}
	lc := collectWorldLifecycle(t, app.journal.Tail(), actionWorldRevealAllMap)
	assertWorldSuccess(t, lc)
	assertWorldField(t, lc, "event_flag_62000", "false", "true")
	assertWorldField(t, lc, "event_flag_62010", "false", "true")
	assertWorldField(t, lc, "map_fragment_0x40002198_owned", "false", "true")
	if got := lc.planned["map_dlc_tiles_hex"]; got == "" || got == "unavailable" {
		t.Fatalf("DLC tiles planned value = %q, want changed hex", got)
	}
	if got := lc.planned["map_dlc_tiles_hex"]; got != lc.finished["map_dlc_tiles_hex"] {
		t.Errorf("DLC tiles planned/finished differ")
	}
}

func TestWorldMapExplorationDebugOff(t *testing.T) {
	app := worldFlagsApp(false)
	if err := app.RemoveFogOfWar(0); err != nil {
		t.Fatalf("RemoveFogOfWar: %v", err)
	}
	if got := len(app.journal.Tail()); got != 0 {
		t.Fatalf("debug-off records = %d, want 0", got)
	}
}
