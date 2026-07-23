package main

import (
	"reflect"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
)

func TestGaItemRepack_AnalyzeReportsNativeHoleNoOp(t *testing.T) {
	app, charIdx := readyGaItemRepackApp(t)
	before := core.CloneSlot(&app.save.Slots[charIdx])

	analysis, err := app.AnalyzeGaItemRepack(charIdx)
	if err != nil {
		t.Fatalf("AnalyzeGaItemRepack: %v", err)
	}
	if analysis.Outcome != "no_op" || analysis.AnalysisToken != "" || analysis.ProjectedAfter == nil || analysis.Recovered != 0 {
		t.Fatalf("analysis=%+v, want native-hole no-op without token", analysis)
	}
	if !reflect.DeepEqual(&app.save.Slots[charIdx], before) {
		t.Fatal("AnalyzeGaItemRepack mutated the active slot")
	}

	if analysis.Before != *analysis.ProjectedAfter || analysis.Before.Usable != analysis.Before.PhysicalEmpty {
		t.Fatalf("capacity=%+v projected=%+v, want every physical hole usable", analysis.Before, *analysis.ProjectedAfter)
	}
	if depth := app.GetUndoDepth(charIdx); depth != 0 {
		t.Fatalf("undo depth=%d, want 0 after no-op analysis", depth)
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
		if slot.Version == 0 {
			continue
		}
		if _, err := core.NativeGaItemCapacity(slot); err == nil {
			return app, charIdx
		}
	}
	t.Skip("no slot with a native GaItem layout")
	return nil, 0
}
