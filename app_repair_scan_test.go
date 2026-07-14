package main

import (
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
	"github.com/oisis/EldenRing-SaveForge/backend/editor"
)

// ---- repairActionsForCode ---------------------------------------------------

// TestRepairActionsForCode_QuantityZero confirms that quantity_zero offers both
// remove_record and leave_unchanged, with remove_record as default.
func TestRepairActionsForCode_QuantityZero(t *testing.T) {
	actions, def := repairActionsForCode(core.RepairCodeQuantityZero)

	if def != core.RepairActionRemoveRecord {
		t.Errorf("default action = %q, want %q", def, core.RepairActionRemoveRecord)
	}

	hasRemove, hasLeave := false, false
	for _, a := range actions {
		switch a.ID {
		case core.RepairActionRemoveRecord:
			hasRemove = true
		case RepairActionLeaveUnchanged:
			hasLeave = true
		}
	}
	if !hasRemove {
		t.Error("quantity_zero actions must include remove_record")
	}
	if !hasLeave {
		t.Error("quantity_zero actions must include leave_unchanged")
	}
}

// TestRepairActionsForCode_QuantityAboveMax_Clamp confirms a positive-cap
// over-quantity offers a clamp with leave_unchanged, defaulting to the clamp.
func TestRepairActionsForCode_QuantityAboveMax_Clamp(t *testing.T) {
	actions, def := repairActionsForCode(core.RepairCodeQuantityAboveMax)

	if def != core.RepairActionClampQuantity {
		t.Errorf("default action = %q, want %q", def, core.RepairActionClampQuantity)
	}
	if len(actions) != 2 || actions[0].ID != core.RepairActionClampQuantity || actions[1].ID != RepairActionLeaveUnchanged {
		t.Errorf("quantity_above_max must offer [clamp_quantity, leave_unchanged], got %+v", actions)
	}
	if actions[0].Label != "Clamp quantity to allowed maximum" {
		t.Errorf("clamp label = %q", actions[0].Label)
	}
}

// TestRepairActionsForCode_ItemNotAllowed offers removal without selecting it by
// default (removal is destructive), plus leave_unchanged.
func TestRepairActionsForCode_ItemNotAllowed(t *testing.T) {
	actions, def := repairActionsForCode(core.RepairCodeItemNotAllowedInContainer)

	if def != RepairActionLeaveUnchanged {
		t.Errorf("default action = %q, want %q", def, RepairActionLeaveUnchanged)
	}
	if len(actions) != 2 || actions[0].ID != core.RepairActionRemoveRecord || actions[1].ID != RepairActionLeaveUnchanged {
		t.Errorf("item_not_allowed must offer [remove_record, leave_unchanged], got %+v", actions)
	}
}

func TestRepairActionsForCode_DuplicateHandleCanBeLeftUnchanged(t *testing.T) {
	actions, def := repairActionsForCode(core.RepairCodeDuplicateHandle)

	if def != core.RepairActionCreateCopy {
		t.Errorf("default action = %q, want %q", def, core.RepairActionCreateCopy)
	}
	hasCreateCopy, hasLeave := false, false
	for _, a := range actions {
		switch a.ID {
		case core.RepairActionCreateCopy:
			hasCreateCopy = true
		case RepairActionLeaveUnchanged:
			hasLeave = true
		}
	}
	if !hasCreateCopy {
		t.Error("duplicate_handle actions must include create_copy")
	}
	if !hasLeave {
		t.Error("duplicate_handle actions must include leave_unchanged")
	}
}

// ---- workspaceIssueToDTO with Record ----------------------------------------

// TestWorkspaceIssueToDTO_UpgradeOutOfRange_HasRecord confirms that a workspace
// issue with a UID resolves to a Record carrying Name and CurrentUpgrade.
func TestWorkspaceIssueToDTO_UpgradeOutOfRange_HasRecord(t *testing.T) {
	const (
		// Dagger (melee_armaments, max upgrade 25)
		weapHandle = uint32(0x80800001)
		weapItemID = uint32(0x000F4240)
		testUID    = "uid-dagger-001"
	)

	snap := editor.InventoryWorkspaceSnapshot{
		InventoryItems: []editor.EditableItem{
			{
				UID:               testUID,
				Container:         editor.ContainerInventory,
				OriginalSlotIndex: 0,
				OriginalHandle:    weapHandle,
				ItemID:            weapItemID,
				Name:              "Dagger",
				Category:          "melee_armaments",
				Quantity:          1,
				CurrentUpgrade:    99, // out of range
				InfusionName:      "",
			},
		},
	}

	issue := editor.WorkspaceValidationIssue{
		Severity: editor.SeverityError,
		Code:     editor.CodeUpgradeOutOfRange,
		Message:  "upgrade 99 exceeds max for Dagger",
		UID:      testUID,
		Handle:   weapHandle,
	}

	dto := workspaceIssueToDTO(0, issue, &snap)

	if dto.Record == nil {
		t.Fatal("DTO Record is nil; expected enriched record with item context")
	}
	if dto.Record.Name != "Dagger" {
		t.Errorf("Record.Name = %q, want %q", dto.Record.Name, "Dagger")
	}
	if dto.Record.CurrentUpgrade != 99 {
		t.Errorf("Record.CurrentUpgrade = %d, want 99", dto.Record.CurrentUpgrade)
	}
	if dto.Record.Scope != "inventory_common" {
		t.Errorf("Record.Scope = %q, want %q", dto.Record.Scope, "inventory_common")
	}
	if dto.Key.Scope != "inventory_common" {
		t.Errorf("IssueKey.Scope = %q, want %q", dto.Key.Scope, "inventory_common")
	}
	if dto.Key.Row != 0 {
		t.Errorf("IssueKey.Row = %d, want 0", dto.Key.Row)
	}
}

// TestWorkspaceIssueToDTO_NoUID_FallsBackToWorkspaceScope confirms that an issue
// without UID (e.g. a global AoW conflict) keeps scope="workspace" and nil Record.
func TestWorkspaceIssueToDTO_NoUID_FallsBackToWorkspaceScope(t *testing.T) {
	issue := editor.WorkspaceValidationIssue{
		Severity: editor.SeverityWarning,
		Code:     editor.CodeSharedAoWConflict,
		Message:  "AoW shared across weapons",
		Handle:   0xC0001234,
	}

	dto := workspaceIssueToDTO(0, issue, nil)

	if dto.Key.Scope != "workspace" {
		t.Errorf("scope = %q, want %q", dto.Key.Scope, "workspace")
	}
	if dto.Record != nil {
		t.Error("expected nil Record for issue without UID")
	}
}

// ---- dedup by IssueID not code ----------------------------------------------

// TestBuildRepairIssueReport_SameCodeDifferentHandle confirms that a workspace
// issue with the same code as a core issue, but for a different handle, is NOT
// dropped by deduplication.
func TestBuildRepairIssueReport_SameCodeDifferentHandle(t *testing.T) {
	// Build a slot with one duplicate-handle pair so core emits duplicate_handle.
	h := uint32(0x80000002)
	slot := &core.SaveSlot{
		GaMap: map[uint32]uint32{h: 0x00400110},
		Inventory: core.EquipInventoryData{
			CommonItems: []core.InventoryItem{
				{GaItemHandle: h, Quantity: 1, Index: 500},
				{GaItemHandle: h, Quantity: 1, Index: 501}, // duplicate → core emits duplicate_handle for h
			},
		},
	}

	// A workspace issue with the same code but a different handle (h2).
	h2 := uint32(0x80000003)
	wsValidation := &editor.WorkspaceValidationReport{
		Errors: []editor.WorkspaceValidationIssue{
			{
				Severity: editor.SeverityError,
				Code:     core.RepairCodeDuplicateHandle,
				Message:  "duplicate handle 0x80000003",
				Handle:   h2,
				// no UID → scope stays "workspace", row=-1
			},
		},
		Warnings: []editor.WorkspaceValidationIssue{},
	}

	report := buildRepairIssueReport(0, "Test", slot, wsValidation, nil)

	foundH, foundH2 := false, false
	for _, dto := range report.Issues {
		if dto.Key.Code == core.RepairCodeDuplicateHandle {
			switch dto.Key.Handle {
			case h:
				foundH = true
			case h2:
				foundH2 = true
			}
		}
	}
	if !foundH {
		t.Error("expected duplicate_handle issue for handle h (core), not found")
	}
	if !foundH2 {
		t.Error("expected duplicate_handle issue for handle h2 (workspace), was incorrectly deduplicated")
	}
}

// ---- ScanRepairIssuesLoaded is read-only w.r.t. Inventory Workspaces --------

// scanRepackApp builds an App around the synthetic fragmentedRepackSlot at slot
// 0, so the diagnostics/workspace regressions run deterministically without the
// on-disk tmp/save fixture.
func scanRepackApp(t *testing.T) (*App, int) {
	t.Helper()
	app := NewApp()
	app.save = &core.SaveFile{}
	app.save.Slots[0] = *fragmentedRepackSlot(t)
	app.saveGeneration = 1
	return app, 0
}

// TestScanRepairIssuesLoaded_DoesNotCreateSession confirms a diagnostic scan on
// a slot with no active workspace leaves the session registry empty — it must
// not publish an Inventory Edit Session.
func TestScanRepairIssuesLoaded_DoesNotCreateSession(t *testing.T) {
	app, charIdx := scanRepackApp(t)

	report, err := app.ScanRepairIssuesLoaded(charIdx)
	if err != nil {
		t.Fatalf("ScanRepairIssuesLoaded: %v", err)
	}
	if report.SlotIndex != charIdx {
		t.Fatalf("report SlotIndex=%d, want %d", report.SlotIndex, charIdx)
	}

	app.editSessionsMu.Lock()
	nSessions, nByChar := len(app.editSessions), len(app.editSessionByChar)
	app.editSessionsMu.Unlock()
	if nSessions != 0 || nByChar != 0 {
		t.Fatalf("scan created a session: editSessions=%d editSessionByChar=%d, want 0/0", nSessions, nByChar)
	}
}

// TestScanRepairIssuesLoaded_PreservesExistingSession confirms that scanning a
// slot with a live Inventory Workspace leaves that exact session intact — same
// ID, same object (so same pending state), neither replaced nor discarded.
func TestScanRepairIssuesLoaded_PreservesExistingSession(t *testing.T) {
	app, charIdx := scanRepackApp(t)

	snap, err := app.StartInventoryEditSession(charIdx)
	if err != nil {
		t.Fatalf("StartInventoryEditSession: %v", err)
	}
	wantID := snap.SessionID

	app.editSessionsMu.Lock()
	wantSess := app.editSessions[app.editSessionByChar[charIdx]]
	app.editSessionsMu.Unlock()

	if _, err := app.ScanRepairIssuesLoaded(charIdx); err != nil {
		t.Fatalf("ScanRepairIssuesLoaded: %v", err)
	}

	app.editSessionsMu.Lock()
	gotID := app.editSessionByChar[charIdx]
	gotSess := app.editSessions[gotID]
	nSessions := len(app.editSessions)
	app.editSessionsMu.Unlock()

	if gotID != wantID {
		t.Fatalf("session ID changed: %q -> %q", wantID, gotID)
	}
	if gotSess != wantSess {
		t.Fatal("session object was replaced by the scan")
	}
	if nSessions != 1 {
		t.Fatalf("editSessions size=%d, want 1", nSessions)
	}
}

// TestScanRepairIssuesLoaded_ThenRepackNotRejected is the regression for the
// original defect: a diagnostic scan must not leave a session that makes the
// GaItem optimizer refuse with inventory_edit_session_active.
func TestScanRepairIssuesLoaded_ThenRepackNotRejected(t *testing.T) {
	app, charIdx := scanRepackApp(t)

	if _, err := app.ScanRepairIssuesLoaded(charIdx); err != nil {
		t.Fatalf("ScanRepairIssuesLoaded: %v", err)
	}

	analysis, err := app.AnalyzeGaItemRepack(charIdx)
	if err != nil {
		t.Fatalf("AnalyzeGaItemRepack: %v", err)
	}
	if analysis.Failure != nil && analysis.Failure.Code == "inventory_edit_session_active" {
		t.Fatalf("repack rejected after diagnostics scan: %+v", analysis.Failure)
	}
	if analysis.Outcome != "ready" {
		t.Fatalf("analysis outcome=%q, want ready", analysis.Outcome)
	}
}

// TestScanRepairIssuesLoaded_EmptySlotErrors guards the pre-refactor behavior:
// an empty slot (Version == 0) must still return an error, not an empty report.
func TestScanRepairIssuesLoaded_EmptySlotErrors(t *testing.T) {
	app := NewApp()
	app.save = &core.SaveFile{} // Slots[0].Version == 0
	app.saveGeneration = 1

	if _, err := app.ScanRepairIssuesLoaded(0); err == nil {
		t.Fatal("ScanRepairIssuesLoaded on an empty slot returned nil error, want failure")
	}
}
