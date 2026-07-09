package main

import (
	"fmt"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
	"github.com/oisis/EldenRing-SaveForge/backend/editor"
)

// InventoryIssuesScanReport is returned by ScanInventoryIssues.
// It merges binary (core) and workspace (editor) findings into one
// report, and carries the session ID started during the scan so the
// frontend can pass it directly to Repair* endpoints.
type InventoryIssuesScanReport struct {
	CharIdx         int                    `json:"charIdx"`
	SessionID       string                 `json:"sessionID"`
	HasIssues       bool                   `json:"hasIssues"`
	BinaryIssues    []core.DiagnosticIssue `json:"binaryIssues"`
	WorkspaceIssues []WorkspaceIssueDetail `json:"workspaceIssues"`
}

// WorkspaceIssueDetail is editor.WorkspaceValidationIssue enriched with
// repair metadata so the frontend can render checkboxes and repair
// descriptions without a second round-trip.
type WorkspaceIssueDetail struct {
	Code       string `json:"code"`
	Severity   string `json:"severity"`
	Message    string `json:"message"`
	UID        string `json:"uid"`
	Handle     uint32 `json:"handle"`
	ItemName   string `json:"itemName"`
	CanRepair  bool   `json:"canRepair"`
	RepairDesc string `json:"repairDesc"`
}

// WorkspaceRepairSpec identifies a single workspace issue to repair.
type WorkspaceRepairSpec struct {
	UID  string `json:"uid"`
	Code string `json:"code"`
}

// ScanInventoryIssues starts an inventory edit session for charIdx
// (replacing any prior session for that character), runs the binary
// corruption scan and workspace validation, and returns a unified report.
// The returned SessionID can be handed to RepairInventoryWorkspaceItem /
// RepairInventoryWorkspaceItems.
func (a *App) ScanInventoryIssues(charIdx int) (InventoryIssuesScanReport, error) {
	var empty InventoryIssuesScanReport

	// Start (or restart) the session — this also runs editor.Validate
	// internally (session.go). All locks released when this returns.
	snap, err := a.StartInventoryEditSession(charIdx)
	if err != nil {
		return empty, fmt.Errorf("ScanInventoryIssues: %w", err)
	}

	// Binary scan — needs slot lock.
	a.saveMu.RLock()
	defer a.saveMu.RUnlock()
	if a.save == nil {
		return empty, fmt.Errorf("ScanInventoryIssues: no save loaded")
	}
	if charIdx < 0 || charIdx >= maxCharacters {
		return empty, fmt.Errorf("ScanInventoryIssues: invalid charIdx %d", charIdx)
	}
	a.slotMu[charIdx].Lock()
	diag := core.DiagnoseSaveCorruption(&a.save.Slots[charIdx], charIdx)
	a.slotMu[charIdx].Unlock()

	// Build workspace issue details from the session's validation report.
	allIssues := append(snap.Validation.Errors, snap.Validation.Warnings...)
	details := make([]WorkspaceIssueDetail, 0, len(allIssues))
	for _, iss := range allIssues {
		canRepair := editor.CanRepairCode(iss.Code)
		d := WorkspaceIssueDetail{
			Code:      iss.Code,
			Severity:  iss.Severity,
			Message:   iss.Message,
			UID:       iss.UID,
			Handle:    iss.Handle,
			ItemName:  resolveItemName(&snap, iss.UID),
			CanRepair: canRepair,
		}
		if canRepair {
			d.RepairDesc = editor.RepairDescForIssue(&snap, iss)
		}
		details = append(details, d)
	}

	binaryIssues := diag.Issues
	if binaryIssues == nil {
		binaryIssues = []core.DiagnosticIssue{}
	}

	hasIssues := len(binaryIssues) > 0 || len(details) > 0
	return InventoryIssuesScanReport{
		CharIdx:         charIdx,
		SessionID:       snap.SessionID,
		HasIssues:       hasIssues,
		BinaryIssues:    binaryIssues,
		WorkspaceIssues: details,
	}, nil
}

// resolveItemName finds an item by UID in the snapshot and returns its name.
func resolveItemName(snap *editor.InventoryWorkspaceSnapshot, uid string) string {
	for _, it := range snap.InventoryItems {
		if it.UID == uid {
			return it.Name
		}
	}
	for _, it := range snap.StorageItems {
		if it.UID == uid {
			return it.Name
		}
	}
	return ""
}

// RepairInventoryWorkspaceItem applies an automatic fix for a single
// workspace issue (identified by uid+code) and commits it as a workspace
// save (SaveInventoryWorkspaceChanges). The returned snapshot reflects the
// post-repair state. Compatibility path: new repair UI should prefer
// ApplyRepairsLoaded.
func (a *App) RepairInventoryWorkspaceItem(sessionID, uid, code string) (editor.InventoryWorkspaceSnapshot, error) {
	var empty editor.InventoryWorkspaceSnapshot
	{
		sess, err := a.acquireSession(sessionID)
		if err != nil {
			return empty, err
		}
		repairErr := editor.AutoRepairWorkspaceItem(&sess.Workspace, uid, code)
		sess.Unlock()
		if repairErr != nil {
			return empty, fmt.Errorf("RepairInventoryWorkspaceItem: %w", repairErr)
		}
	}
	snap, err := a.SaveInventoryWorkspaceChanges(sessionID)
	if err != nil {
		return empty, fmt.Errorf("RepairInventoryWorkspaceItem: workspace save: %w", err)
	}
	return snap, nil
}

// RepairInventoryWorkspaceItems applies automatic fixes for all provided
// specs in one batch, then commits a single workspace save. Best-effort:
// all repairable specs are attempted even if some fail. Returns the
// post-repair snapshot. Compatibility path: kept for SortOrderTab and older
// workspace-specific callers while central repair apply becomes primary.
func (a *App) RepairInventoryWorkspaceItems(sessionID string, repairs []WorkspaceRepairSpec) (editor.InventoryWorkspaceSnapshot, error) {
	var empty editor.InventoryWorkspaceSnapshot
	{
		sess, err := a.acquireSession(sessionID)
		if err != nil {
			return empty, err
		}
		for _, r := range repairs {
			// Best-effort: ignore per-item errors so the rest still apply.
			_ = editor.AutoRepairWorkspaceItem(&sess.Workspace, r.UID, r.Code)
		}
		sess.Unlock()
	}
	snap, err := a.SaveInventoryWorkspaceChanges(sessionID)
	if err != nil {
		return empty, fmt.Errorf("RepairInventoryWorkspaceItems: workspace save: %w", err)
	}
	return snap, nil
}

// RepairAllWeaponIssues scans the current workspace for auto-repairable
// weapon issues (upgrade_out_of_range, pending AoW) and applies all fixes in
// one batch. Returns a RepairReport with fixed/skipped counts.
// The caller must still WriteSave to persist changes to disk.
func (a *App) RepairAllWeaponIssues(charIdx int) (RepairReport, error) {
	scan, err := a.ScanInventoryIssues(charIdx)
	if err != nil {
		return RepairReport{}, fmt.Errorf("RepairAllWeaponIssues: %w", err)
	}

	var specs []WorkspaceRepairSpec
	for _, iss := range scan.WorkspaceIssues {
		if iss.CanRepair {
			specs = append(specs, WorkspaceRepairSpec{UID: iss.UID, Code: iss.Code})
		}
	}
	if len(specs) == 0 {
		return RepairReport{Fixed: []string{}, Skipped: []string{}}, nil
	}

	if _, err := a.RepairInventoryWorkspaceItems(scan.SessionID, specs); err != nil {
		return RepairReport{}, fmt.Errorf("RepairAllWeaponIssues: %w", err)
	}

	fixed := make([]string, len(specs))
	for i, s := range specs {
		fixed[i] = s.UID
	}
	return RepairReport{Fixed: fixed, Skipped: []string{}}, nil
}

// _forceExportTypesInventoryIssues surfaces the new DTOs to the Wails
// type generator. Never called.
func (a *App) _forceExportTypesInventoryIssues() (InventoryIssuesScanReport, WorkspaceIssueDetail, WorkspaceRepairSpec) {
	return InventoryIssuesScanReport{}, WorkspaceIssueDetail{}, WorkspaceRepairSpec{}
}
