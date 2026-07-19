package main

import (
	"fmt"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
	"github.com/oisis/EldenRing-SaveForge/backend/editor"
)

// Repair apply outcome tags. One central result model shared by the loaded and
// external endpoints (Prompt 9: "loaded i external save mają ten sam model wyniku").
const (
	repairOutcomeApplied        = "applied"
	repairOutcomeSkipped        = "skipped"
	repairOutcomeFailed         = "failed"
	repairOutcomeNeedsUserInput = "needsUserInput"
)

// RepairApplyTarget is one issue + chosen action to apply. It carries the full
// IssueKey and the fingerprint captured at scan time so the endpoint can
// stale-check the record before dispatching a primitive. The frontend gets Key
// and Fingerprint verbatim from RepairIssueDTO, so no re-scan-to-resolve is
// needed.
type RepairApplyTarget struct {
	IssueID        string        `json:"issueID"`
	Key            core.IssueKey `json:"key"`
	Fingerprint    string        `json:"fingerprint"`
	SelectedAction string        `json:"selectedAction"`
	// AoWHandle is the replacement Ash of War GaItem handle for pick_aow
	// (attach_existing_aow). Zero → needsUserInput.
	AoWHandle uint32 `json:"aowHandle"`
	// AoWItemID overrides the copy source for create_new_aow_copy. Zero → the
	// endpoint derives it from the weapon's currently attached AoW.
	AoWItemID uint32 `json:"aowItemID"`
}

// RepairActionResult is the per-target outcome.
type RepairActionResult struct {
	IssueID   string `json:"issueID"`
	SlotIndex int    `json:"slotIndex"`
	Action    string `json:"action"`
	Outcome   string `json:"outcome"` // applied | skipped | failed | needsUserInput
	Message   string `json:"message"`
}

// RepairApplyReport is the batch result, identical shape for loaded and external.
type RepairApplyReport struct {
	Applied        int                  `json:"applied"`
	Skipped        int                  `json:"skipped"`
	Failed         int                  `json:"failed"`
	NeedsUserInput int                  `json:"needsUserInput"`
	Stopped        bool                 `json:"stopped"` // stopOnFirstFailure tripped
	Results        []RepairActionResult `json:"results"`
}

// ---- endpoints --------------------------------------------------------------

// ApplyRepairsLoaded applies a batch of repair actions to one loaded-save slot.
// A single pre-batch undo snapshot is pushed only if ≥1 action mutated, and the
// active workspace session for that character is invalidated on any successful
// mutation so its cached (now-stale) workspace is dropped.
func (a *App) ApplyRepairsLoaded(charIdx int, targets []RepairApplyTarget, stopOnFirstFailure bool) (RepairApplyReport, error) {
	// The operation lifecycle brackets the whole call. requested fires before any
	// lock or validation, so even an early rejection (no save / bad index / empty
	// slot) still leaves a requested/finished pair; finished fires on every exit
	// path. Both are debug-gated at the journal, so Debug off leaves no trace.
	a.journalToolsOperationRequested(actionToolsApplyRepairsLoaded)

	var empty RepairApplyReport
	a.saveMu.RLock()
	defer a.saveMu.RUnlock()
	if a.save == nil {
		a.journalToolsOperationFinished(actionToolsApplyRepairsLoaded, characterChangeError, toolsStageNoActiveSave)
		return empty, fmt.Errorf("ApplyRepairsLoaded: no save loaded")
	}
	if charIdx < 0 || charIdx >= maxCharacters {
		a.journalToolsOperationFinished(actionToolsApplyRepairsLoaded, characterChangeError, toolsStageInvalidCharacter)
		return empty, fmt.Errorf("ApplyRepairsLoaded: invalid character index %d", charIdx)
	}

	a.slotMu[charIdx].Lock()
	slot := &a.save.Slots[charIdx]
	if slot.Version == 0 {
		a.slotMu[charIdx].Unlock()
		a.journalToolsOperationFinished(actionToolsApplyRepairsLoaded, characterChangeError, toolsStageEmptySlot)
		return empty, fmt.Errorf("ApplyRepairsLoaded: slot %d is empty", charIdx)
	}

	// Capture the pre-mutation state for undo BEFORE the batch runs; push it
	// afterwards only if something actually applied (req 6).
	undoSnap := a.buildSlotSnapshotLocked(charIdx)

	// Debug Mode projects the exact physical save changes this batch will make by
	// replaying the identical batch on a throwaway clone (which never touches undo,
	// sessions, locks or the real slot), emitting all before then all planned
	// records before the real slot is touched. Debug off makes no clone and emits
	// no tools_change_* record; either way the real writer below runs exactly once
	// — applyRepairBatchToSlot is the single source of the mutation.
	debug := a.journal.debugEnabled()
	var plans gameItemMutationPlans
	if debug {
		clone := core.CloneSlot(slot)
		applyRepairBatchToSlot(clone, charIdx, targets, stopOnFirstFailure) // diagnostic replay; report intentionally ignored
		plans = planToolsRepairApply(slot, clone)
		a.journalToolsRepairApplyBefore(charIdx, plans)
	}

	rep := applyRepairBatchToSlot(slot, charIdx, targets, stopOnFirstFailure)
	if rep.Applied > 0 {
		a.pushUndoSnapshotLocked(charIdx, undoSnap)
	}

	// The finished phase reads the real slot exactly once after the batch, so a
	// partial batch reports the real state every applied target left (outcome
	// error, stage apply_repairs_loaded), never the clone. A no-op / all-skipped /
	// needsUserInput batch produced no plan, so it emits no field record — the
	// operation event stays its only status.
	if debug {
		outcome, stage := repairApplyOperationResult(rep)
		a.journalToolsRepairApplyFinished(charIdx, outcome, stage, plans, slot)
	}
	a.slotMu[charIdx].Unlock()

	// Invalidate the session AFTER releasing slotMu (lock order:
	// lifecycleMu → editSessionsMu → session; never under slotMu).
	if rep.Applied > 0 {
		a.invalidateSessionForChar(charIdx)
	}

	outcome, stage := repairApplyOperationResult(rep)
	a.journalToolsOperationFinished(actionToolsApplyRepairsLoaded, outcome, stage)
	return rep, nil
}

// ---- batch + single-action core --------------------------------------------

// applyRepairBatchToSlot applies every target to a single slot in order,
// honouring stopOnFirstFailure. Shared by ApplyRepairsLoaded and tests.
func applyRepairBatchToSlot(slot *core.SaveSlot, slotIndex int, targets []RepairApplyTarget, stopOnFirstFailure bool) RepairApplyReport {
	rep := RepairApplyReport{Results: make([]RepairActionResult, 0, len(targets))}
	for _, t := range targets {
		r := applyRepairActionToSlot(slot, slotIndex, t)
		if tallyRepairOutcome(&rep, r, stopOnFirstFailure) {
			return rep
		}
	}
	return rep
}

// tallyRepairOutcome appends r and updates the counters. Returns true when the
// batch must stop (stopOnFirstFailure && r failed).
func tallyRepairOutcome(rep *RepairApplyReport, r RepairActionResult, stopOnFirstFailure bool) bool {
	rep.Results = append(rep.Results, r)
	switch r.Outcome {
	case repairOutcomeApplied:
		rep.Applied++
	case repairOutcomeSkipped:
		rep.Skipped++
	case repairOutcomeNeedsUserInput:
		rep.NeedsUserInput++
	case repairOutcomeFailed:
		rep.Failed++
		if stopOnFirstFailure {
			rep.Stopped = true
			return true
		}
	}
	return false
}

// applyRepairActionToSlot runs one action atomically:
// fingerprint stale-check → SnapshotSlot → dispatch primitive → post-mutation
// re-scan validation → RestoreSlot on any failure after the snapshot. It never
// relies on primitive-level rollback (req 3): even ClearWeaponAoW /
// CreateWeaponAoWCopy are wrapped, because PatchWeaponAoW can theoretically
// fail after a partial allocation/rebuild.
func applyRepairActionToSlot(slot *core.SaveSlot, slotIndex int, t RepairApplyTarget) RepairActionResult {
	res := RepairActionResult{IssueID: t.IssueID, SlotIndex: slotIndex, Action: t.SelectedAction}

	// No-op / not-central actions are skipped, never failed (req: leave_unchanged
	// / report-only → skipped).
	if isSkipRepairAction(t.SelectedAction) {
		res.Outcome = repairOutcomeSkipped
		res.Message = fmt.Sprintf("action %q requires no central mutation", t.SelectedAction)
		return res
	}

	// pick_aow needs a user-chosen replacement handle.
	if t.SelectedAction == core.RepairActionPickAoW && t.AoWHandle == 0 {
		res.Outcome = repairOutcomeNeedsUserInput
		res.Message = "pick_aow requires a replacement Ash of War handle"
		return res
	}

	// Fingerprint stale-check for record-addressed scopes (req 2/4). AoW actions
	// address the weapon's inventory/storage row, so they are covered too.
	if scopeAddressesRecord(t.Key.Scope) {
		curFp, ok := core.FingerprintRecordAt(slot, t.Key.Scope, t.Key.Row)
		if !ok {
			res.Outcome = repairOutcomeFailed
			res.Message = fmt.Sprintf("row %d in scope %q no longer addressable", t.Key.Row, t.Key.Scope)
			return res
		}
		if curFp != t.Fingerprint {
			res.Outcome = repairOutcomeFailed
			res.Message = "stale: record changed since scan"
			return res
		}
	}

	// Atomic guard: snapshot before any mutation.
	snap := core.SnapshotSlot(slot)

	if err := dispatchRepairAction(slot, t); err != nil {
		core.RestoreSlot(slot, snap)
		res.Outcome = repairOutcomeFailed
		res.Message = err.Error()
		return res
	}

	// Post-mutation validation: the targeted issue must be gone. Force the scan
	// slot index into the key so the recomputed issueID lines up with the fresh
	// scan (which stamps Slot=slotIndex).
	issues := core.ScanRepairIssues(slotIndex, slot)
	wantKey := t.Key
	wantKey.Slot = slotIndex
	wantID := core.IssueKeyID(wantKey)
	for _, iss := range issues {
		if iss.IssueID == wantID {
			core.RestoreSlot(slot, snap)
			res.Outcome = repairOutcomeFailed
			res.Message = "post-validation: issue still present after repair"
			return res
		}
	}

	// Clamp-specific postcondition: IssueKey.Value carries the OLD effective
	// quantity, so a buggy partial clamp would change the value → a new IssueID
	// the original-ID check above cannot catch. Re-check the targeted row itself
	// for any lingering over-cap or zero-quantity defect. Row-scoped (not
	// value-scoped) deliberately — but NOT used for remove_record, where clearing
	// a compacted storage row shifts a different record into the same row and must
	// not look like a failure.
	if t.SelectedAction == core.RepairActionClampQuantity &&
		clampLeavesQuantityInvalid(issues, t.Key.Scope, t.Key.Row) {
		core.RestoreSlot(slot, snap)
		res.Outcome = repairOutcomeFailed
		res.Message = "post-validation: quantity still invalid at the clamped row"
		return res
	}
	if t.SelectedAction == RepairActionClampUpgrade {
		stillInvalid, err := clampUpgradeStillInvalid(slot, slotIndex, t)
		if err != nil {
			core.RestoreSlot(slot, snap)
			res.Outcome = repairOutcomeFailed
			res.Message = fmt.Sprintf("post-validation: %v", err)
			return res
		}
		if stillInvalid {
			core.RestoreSlot(slot, snap)
			res.Outcome = repairOutcomeFailed
			res.Message = "post-validation: upgrade still invalid after repair"
			return res
		}
	}

	res.Outcome = repairOutcomeApplied
	return res
}

// clampLeavesQuantityInvalid reports whether, after a clamp, the inventory record
// at scope+row still carries an over-cap or zero-quantity issue — the two defects
// a correct clamp must eliminate without introducing.
func clampLeavesQuantityInvalid(issues []core.RepairIssue, scope string, row int) bool {
	for _, iss := range issues {
		if iss.Key.Domain == "inventory" && iss.Key.Scope == scope && iss.Key.Row == row &&
			(iss.Key.Code == core.RepairCodeQuantityAboveMax || iss.Key.Code == core.RepairCodeQuantityZero) {
			return true
		}
	}
	return false
}

// dispatchRepairAction routes (domain, action) to the backing core primitive.
func dispatchRepairAction(slot *core.SaveSlot, t RepairApplyTarget) error {
	if t.SelectedAction == RepairActionClampUpgrade {
		return clampUpgradeAt(slot, t)
	}
	switch t.Key.Domain {
	case "inventory":
		switch t.SelectedAction {
		case core.RepairActionRemoveRecord:
			return core.RemoveInventoryRecordAt(slot, t.Key.Scope, t.Key.Row, t.Fingerprint)
		case core.RepairActionRepairIndex:
			_, err := core.AssignFreshInventoryIndex(slot, t.Key.Scope, t.Key.Row)
			return err
		case core.RepairActionCreateCopy:
			_, err := core.RehandleInventoryRecord(slot, t.Key.Scope, t.Key.Row)
			return err
		case core.RepairActionClampQuantity:
			_, err := core.ClampInventoryQuantityAt(slot, t.Key.Scope, t.Key.Row, t.Fingerprint)
			return err
		}
	case "aow":
		switch t.SelectedAction {
		case core.RepairActionClearAoW:
			return core.ClearWeaponAoW(slot, t.Key.Handle)
		case core.RepairActionPickAoW:
			return core.AttachExistingWeaponAoW(slot, t.Key.Handle, t.AoWHandle)
		case core.RepairActionCreateCopy:
			aowItemID := t.AoWItemID
			if aowItemID == 0 {
				id, ok := core.CurrentWeaponAoWItemID(slot, t.Key.Handle)
				if !ok {
					return fmt.Errorf("cannot derive current AoW itemID for weapon 0x%08X; supply aowItemID", t.Key.Handle)
				}
				aowItemID = id
			}
			return core.CreateWeaponAoWCopy(slot, t.Key.Handle, aowItemID)
		}
	}
	return fmt.Errorf("unsupported action %q for domain %q", t.SelectedAction, t.Key.Domain)
}

// clampUpgradeAt reuses the workspace weapon encoder, then applies only its
// resulting ItemID patch to the matching GaItem. Unlike a workspace save this
// does not rebuild inventory/storage layouts or touch their acquisition
// counters; a repair of an invalid upgrade must be limited to that weapon.
func clampUpgradeAt(slot *core.SaveSlot, t RepairApplyTarget) error {
	if t.Key.Domain != "inventory" || t.Key.Code != editor.CodeUpgradeOutOfRange {
		return fmt.Errorf("clamp_upgrade requires an inventory upgrade_out_of_range issue")
	}

	snap, err := editor.BuildSnapshot(slot, "", t.Key.Slot)
	if err != nil {
		return fmt.Errorf("build workspace snapshot: %w", err)
	}

	item, ok := findWorkspaceItemForRepair(&snap, t.Key)
	if !ok {
		return fmt.Errorf("target weapon at %s row %d (handle 0x%08X) not found",
			t.Key.Scope, t.Key.Row, t.Key.Handle)
	}
	oldItemID := item.ItemID
	if err := editor.AutoRepairWorkspaceItem(&snap, item.UID, editor.CodeUpgradeOutOfRange); err != nil {
		return err
	}
	repaired, ok := findWorkspaceItem(&snap, item.UID)
	if !ok {
		return fmt.Errorf("repaired weapon %q disappeared from workspace", item.UID)
	}
	if repaired.ItemID == oldItemID {
		return fmt.Errorf("clamp_upgrade produced no ItemID change for %q", item.Name)
	}
	return core.PatchWeaponItemID(slot, item.OriginalHandle, oldItemID, repaired.ItemID)
}

// findWorkspaceItemForRepair resolves a workspace item by its concrete binary
// location. Handle alone is insufficient because the same handle may exist in
// both containers, so scope and physical row are part of the identity.
func findWorkspaceItemForRepair(snap *editor.InventoryWorkspaceSnapshot, key core.IssueKey) (editor.EditableItem, bool) {
	var items []editor.EditableItem
	switch key.Scope {
	case "inventory_common":
		items = snap.InventoryItems
	case "storage_common":
		items = snap.StorageItems
	default:
		return editor.EditableItem{}, false
	}
	for _, item := range items {
		if item.OriginalSlotIndex == key.Row && item.OriginalHandle == key.Handle {
			return item, true
		}
	}
	return editor.EditableItem{}, false
}

// clampUpgradeStillInvalid re-runs workspace validation after the GaItem patch
// because core.ScanRepairIssues intentionally does not own editor-only upgrade
// validation.
func clampUpgradeStillInvalid(slot *core.SaveSlot, slotIndex int, t RepairApplyTarget) (bool, error) {
	snap, err := editor.BuildSnapshot(slot, "", slotIndex)
	if err != nil {
		return false, err
	}
	validation := editor.Validate(snap)
	for _, issue := range append(validation.Errors, validation.Warnings...) {
		if issue.Code != editor.CodeUpgradeOutOfRange {
			continue
		}
		item, ok := findWorkspaceItem(&snap, issue.UID)
		if ok && item.OriginalSlotIndex == t.Key.Row && item.OriginalHandle == t.Key.Handle &&
			containerToScope(item.Container) == t.Key.Scope {
			return true, nil
		}
	}
	return false, nil
}

// isSkipRepairAction reports actions the central endpoint intentionally does
// not mutate: explicit no-ops and stats actions handled by other flows. These
// are reported as skipped, never failed.
func isSkipRepairAction(action string) bool {
	switch action {
	case core.RepairActionNoAction,
		core.RepairActionFixLevel,
		RepairActionLeaveUnchanged,
		RepairActionReportOnly:
		return true
	}
	return false
}

// scopeAddressesRecord reports whether the scope names a concrete inventory /
// storage record (so a fingerprint stale-check applies).
func scopeAddressesRecord(scope string) bool {
	switch scope {
	case "inventory_common", "inventory_key", "storage_common":
		return true
	}
	return false
}

// invalidateSessionForChar evicts and drains the active inventory edit session
// for charIdx, if any. The frontend self-heals by starting a fresh session on
// its next call (which then reflects the post-repair slot). Lock order matches
// DiscardInventoryEditSession; MUST NOT be called while holding slotMu[charIdx].
func (a *App) invalidateSessionForChar(charIdx int) {
	if charIdx < 0 || charIdx >= maxCharacters {
		return
	}
	a.lifecycleMu[charIdx].Lock()
	defer a.lifecycleMu[charIdx].Unlock()

	a.editSessionsMu.Lock()
	id, ok := a.editSessionByChar[charIdx]
	if !ok {
		a.editSessionsMu.Unlock()
		return
	}
	sess := a.editSessions[id]
	delete(a.editSessions, id)
	delete(a.editSessionByChar, charIdx)
	a.editSessionsMu.Unlock()

	if sess != nil {
		closeSession(sess)
	}
}

// _forceExportTypesRepairApply surfaces the apply DTOs to the Wails type
// generator. Never called at runtime.
func (a *App) _forceExportTypesRepairApply() (RepairApplyTarget, RepairActionResult, RepairApplyReport) {
	return RepairApplyTarget{}, RepairActionResult{}, RepairApplyReport{}
}
