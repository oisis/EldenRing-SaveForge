package main

import (
	"strings"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
	"github.com/oisis/EldenRing-SaveForge/backend/db"
)

// minimalSave returns an App with a single populated slot whose EventFlags area
// is large enough for all known colosseum and summoning-pool IDs (BST-resolved
// max ~14 KB; 20 KB provides headroom). It does NOT initialise dynamic offsets,
// so modules that call core.SetUnlockedRegions (MatchmakingRegions) or
// revealBaseMap/revealDLCMap (RevealMap) will fail — those are tested via
// roundtrip/integration tests that load real save files.
func minimalSave(slotIndex int) *App {
	const dataSize = 20_000
	app := NewApp()
	app.save = &core.SaveFile{}
	app.save.Slots[slotIndex].Version = 1
	app.save.Slots[slotIndex].EventFlagsOffset = 4
	app.save.Slots[slotIndex].Data = make([]byte, dataSize)
	return app
}

func TestApplyPvPPreparation_NoSave(t *testing.T) {
	app := NewApp() // save == nil
	_, err := app.ApplyPvPPreparation(0, PvPPreparationOptions{})
	if err == nil || !strings.Contains(err.Error(), "no save loaded") {
		t.Fatalf("want 'no save loaded' error, got %v", err)
	}
}

func TestApplyPvPPreparation_InvalidSlotIndex(t *testing.T) {
	app := minimalSave(0)
	for _, idx := range []int{-1, 10, 99} {
		_, err := app.ApplyPvPPreparation(idx, PvPPreparationOptions{})
		if err == nil || !strings.Contains(err.Error(), "invalid slot index") {
			t.Errorf("slot %d: want 'invalid slot index', got %v", idx, err)
		}
	}
}

func TestApplyPvPPreparation_EmptySlot(t *testing.T) {
	app := NewApp()
	app.save = &core.SaveFile{} // Slots[0].Version == 0
	_, err := app.ApplyPvPPreparation(0, PvPPreparationOptions{})
	if err == nil || !strings.Contains(err.Error(), "empty") {
		t.Fatalf("want 'empty' error for Version==0 slot, got %v", err)
	}
}

func TestApplyPvPPreparation_BadFlagsOffset(t *testing.T) {
	app := NewApp()
	app.save = &core.SaveFile{}
	app.save.Slots[0].Version = 1
	// EventFlagsOffset == 0 triggers "event flags offset not computed"
	app.save.Slots[0].Data = make([]byte, 100)
	_, err := app.ApplyPvPPreparation(0, PvPPreparationOptions{})
	if err == nil || !strings.Contains(err.Error(), "event flags offset") {
		t.Fatalf("want 'event flags offset' error, got %v", err)
	}
}

func TestApplyPvPPreparation_SitesOfGraceWarning(t *testing.T) {
	app := minimalSave(0)
	warnings, err := app.ApplyPvPPreparation(0, PvPPreparationOptions{SitesOfGrace: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "planned") {
		t.Fatalf("want one 'planned' warning, got %v", warnings)
	}
}

func TestApplyPvPPreparation_ColosseumWarning(t *testing.T) {
	app := minimalSave(0)
	warnings, err := app.ApplyPvPPreparation(0, PvPPreparationOptions{Colosseums: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, w := range warnings {
		if strings.Contains(w, "gates") {
			found = true
		}
	}
	if !found {
		t.Fatalf("want 'gates' in warnings, got %v", warnings)
	}
}

func TestApplyPvPPreparation_SummoningPoolsWarning(t *testing.T) {
	app := minimalSave(0)
	warnings, err := app.ApplyPvPPreparation(0, PvPPreparationOptions{SummoningPools: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, w := range warnings {
		if strings.Contains(w, "Bloody Finger") {
			found = true
		}
	}
	if !found {
		t.Fatalf("want 'Bloody Finger' in warnings, got %v", warnings)
	}
}

// TestApplyPvPPreparation_ColosseumsMutate verifies that the Colosseums module
// actually sets the Activate flags that GetColosseums reads back as Unlocked.
func TestApplyPvPPreparation_ColosseumsMutate(t *testing.T) {
	app := minimalSave(0)

	_, err := app.ApplyPvPPreparation(0, PvPPreparationOptions{Colosseums: true})
	if err != nil {
		t.Fatalf("ApplyPvPPreparation: %v", err)
	}

	entries, err := app.GetColosseums(0)
	if err != nil {
		t.Fatalf("GetColosseums: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("GetColosseums returned empty list")
	}
	for _, c := range entries {
		if !c.Unlocked {
			t.Errorf("colosseum %q (id=%d) not unlocked after Apply", c.Name, c.ID)
		}
	}
}

// TestApplyPvPPreparation_SummoningPoolsMutate verifies that the SummoningPools
// module actually sets flags that GetSummoningPools reads back as Activated.
func TestApplyPvPPreparation_SummoningPoolsMutate(t *testing.T) {
	app := minimalSave(0)

	_, err := app.ApplyPvPPreparation(0, PvPPreparationOptions{SummoningPools: true})
	if err != nil {
		t.Fatalf("ApplyPvPPreparation: %v", err)
	}

	pools, err := app.GetSummoningPools(0)
	if err != nil {
		t.Fatalf("GetSummoningPools: %v", err)
	}
	if len(pools) == 0 {
		t.Fatal("GetSummoningPools returned empty list")
	}
	notActivated := 0
	for _, p := range pools {
		if !p.Activated {
			notActivated++
		}
	}
	if notActivated > 0 {
		t.Errorf("%d/%d summoning pools not activated after Apply", notActivated, len(pools))
	}
}

// TestApplyPvPPreparation_EventFlagRoundtrip directly checks that the Activate
// flag for each colosseum is set in the underlying byte array after Apply,
// verifying the mutation independently of GetColosseums.
func TestApplyPvPPreparation_EventFlagRoundtrip(t *testing.T) {
	app := minimalSave(0)

	_, err := app.ApplyPvPPreparation(0, PvPPreparationOptions{Colosseums: true})
	if err != nil {
		t.Fatalf("ApplyPvPPreparation: %v", err)
	}

	slot := &app.save.Slots[0]
	flags := slot.Data[slot.EventFlagsOffset:]

	for _, c := range db.GetAllColosseums() {
		set, ferr := db.GetEventFlag(flags, c.ID)
		if ferr != nil {
			t.Errorf("colosseum %q activate flag %d: GetEventFlag error: %v", c.Name, c.ID, ferr)
			continue
		}
		if !set {
			t.Errorf("colosseum %q activate flag %d: NOT set after Apply", c.Name, c.ID)
		}
	}
}
