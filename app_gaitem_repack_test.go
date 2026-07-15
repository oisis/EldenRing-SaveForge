package main

import (
	"reflect"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
)

func TestGaItemRepack_AnalyzeExecuteAndUndo(t *testing.T) {
	app, charIdx := readyGaItemRepackApp(t)
	before := core.CloneSlot(&app.save.Slots[charIdx])

	analysis, err := app.AnalyzeGaItemRepack(charIdx)
	if err != nil {
		t.Fatalf("AnalyzeGaItemRepack: %v", err)
	}
	if analysis.Outcome != "ready" || analysis.AnalysisToken == "" || analysis.ProjectedAfter == nil || analysis.Recovered <= 0 {
		t.Fatalf("analysis=%+v, want ready result with token and recovery", analysis)
	}
	if !reflect.DeepEqual(&app.save.Slots[charIdx], before) {
		t.Fatal("AnalyzeGaItemRepack mutated the active slot")
	}

	result, err := app.ExecuteGaItemRepack(GaItemRepackExecuteRequest{CharacterIndex: charIdx, AnalysisToken: analysis.AnalysisToken})
	if err != nil {
		t.Fatalf("ExecuteGaItemRepack: %v", err)
	}
	if result.Outcome != "success" || result.After == nil || result.Recovered != analysis.Recovered {
		t.Fatalf("result=%+v, want successful approved repack", result)
	}
	if result.After.Usable != result.Before.Usable+result.Recovered {
		t.Errorf("usable capacity %d -> %d does not match recovery %d", result.Before.Usable, result.After.Usable, result.Recovered)
	}
	if depth := app.GetUndoDepth(charIdx); depth != 1 {
		t.Fatalf("undo depth=%d, want 1 after successful repack", depth)
	}

	if err := app.RevertSlot(charIdx); err != nil {
		t.Fatalf("RevertSlot: %v", err)
	}
	if !reflect.DeepEqual(&app.save.Slots[charIdx], before) {
		t.Fatal("undo did not restore the complete pre-repack slot")
	}
}

func TestGaItemRepack_StaleTokenDoesNotMutate(t *testing.T) {
	app, charIdx := readyGaItemRepackApp(t)
	analysis, err := app.AnalyzeGaItemRepack(charIdx)
	if err != nil {
		t.Fatalf("AnalyzeGaItemRepack: %v", err)
	}
	before := core.CloneSlot(&app.save.Slots[charIdx])
	app.save.Slots[charIdx].Data[0] ^= 0x01
	changed := core.CloneSlot(&app.save.Slots[charIdx])

	result, err := app.ExecuteGaItemRepack(GaItemRepackExecuteRequest{CharacterIndex: charIdx, AnalysisToken: analysis.AnalysisToken})
	if err != nil {
		t.Fatalf("ExecuteGaItemRepack: %v", err)
	}
	if result.Outcome != "could_not_start" || result.Failure == nil || result.Failure.Code != "analysis_stale" {
		t.Fatalf("result=%+v, want stale-token no-start", result)
	}
	if !reflect.DeepEqual(&app.save.Slots[charIdx], changed) {
		t.Fatal("stale execution changed the active slot")
	}
	if reflect.DeepEqual(&app.save.Slots[charIdx], before) {
		t.Fatal("test setup did not change the slot")
	}
	if depth := app.GetUndoDepth(charIdx); depth != 0 {
		t.Fatalf("undo depth=%d, want 0 after stale no-start", depth)
	}
}

func TestGaItemRepack_ActiveWorkspaceIsUnavailable(t *testing.T) {
	app, charIdx := readyGaItemRepackApp(t)
	app.editSessionByChar[charIdx] = "active-workspace"

	analysis, err := app.AnalyzeGaItemRepack(charIdx)

	if err != nil {
		t.Fatalf("AnalyzeGaItemRepack: %v", err)
	}
	if analysis.Outcome != "unavailable" || analysis.Failure == nil || analysis.Failure.Code != "inventory_edit_session_active" {
		t.Fatalf("analysis=%+v, want workspace unavailable", analysis)
	}
}

func readyGaItemRepackApp(t *testing.T) (*App, int) {
	t.Helper()
	app := NewApp()
	app.save = loadFixtureSave(t)
	app.saveGeneration = 1

	for charIdx := range app.save.Slots {
		slot := &app.save.Slots[charIdx]
		if slot.Version == 0 || len(core.PreflightGaItemRepack(slot).Blockers) != 0 {
			continue
		}
		if fragmentGaItemSlot(slot) {
			return app, charIdx
		}
	}
	t.Skip("no healthy slot with a suitable GaItem gap")
	return nil, 0
}

func fragmentGaItemSlot(slot *core.SaveSlot) bool {
	firstNonEmpty := -1
	for i, record := range slot.GaItems {
		if !record.IsEmpty() {
			firstNonEmpty = i
			break
		}
	}
	if firstNonEmpty < 0 {
		return false
	}

	emptyIndex, recordIndex := -1, -1
	for i := firstNonEmpty + 1; i < len(slot.GaItems); i++ {
		if emptyIndex < 0 && slot.GaItems[i].IsEmpty() {
			emptyIndex = i
			continue
		}
		if emptyIndex >= 0 && !slot.GaItems[i].IsEmpty() {
			recordIndex = i
			break
		}
	}
	if emptyIndex < 0 || recordIndex < 0 {
		return false
	}

	slot.GaItems[emptyIndex] = slot.GaItems[recordIndex]
	slot.GaItems[recordIndex] = core.GaItemFull{}
	slot.NextArmamentIndex = len(slot.GaItems)
	preflight := core.PreflightGaItemRepack(slot)
	return len(preflight.Blockers) == 0 && preflight.Analysis.Recovered > 0
}
