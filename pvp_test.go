package main

import (
	"strings"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
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
