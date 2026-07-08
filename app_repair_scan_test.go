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

// TestRepairActionsForCode_QuantityAboveMax_ReportOnly confirms the NG-aware
// over-cap issue stays report-only: no reviewed mutating clamp primitive exists
// yet, so the only offered action is report_only.
func TestRepairActionsForCode_QuantityAboveMax_ReportOnly(t *testing.T) {
	actions, def := repairActionsForCode(core.RepairCodeQuantityAboveMax)

	if def != RepairActionReportOnly {
		t.Errorf("default action = %q, want %q", def, RepairActionReportOnly)
	}
	if len(actions) != 1 || actions[0].ID != RepairActionReportOnly {
		t.Errorf("quantity_above_max must offer only report_only, got %+v", actions)
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
