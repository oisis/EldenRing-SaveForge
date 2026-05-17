package main

import (
	"fmt"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
	"github.com/oisis/EldenRing-SaveForge/backend/editor"
)

// StartInventoryEditSession builds a read-only workspace snapshot for the
// given character slot and registers it as the active session for that
// character. If a session already exists for the same charIdx it is
// replaced.
//
// Phase 1 contract: the slot is not mutated, no undo is pushed, no rebuild
// is performed. Future phases will accept mutations on the returned
// session and only then call into the rebuild pipeline.
func (a *App) StartInventoryEditSession(charIdx int) (editor.InventoryWorkspaceSnapshot, error) {
	if a.save == nil {
		return editor.InventoryWorkspaceSnapshot{}, fmt.Errorf("no save loaded")
	}
	if charIdx < 0 || charIdx >= len(a.save.Slots) {
		return editor.InventoryWorkspaceSnapshot{}, fmt.Errorf("invalid character index %d", charIdx)
	}
	slot := &a.save.Slots[charIdx]
	if slot.Version == 0 {
		return editor.InventoryWorkspaceSnapshot{}, fmt.Errorf("slot %d is empty", charIdx)
	}

	sess, err := editor.StartSession(slot, charIdx)
	if err != nil {
		return editor.InventoryWorkspaceSnapshot{}, err
	}

	// Replace any existing session for this character.
	if oldID, ok := a.editSessionByChar[charIdx]; ok {
		delete(a.editSessions, oldID)
	}
	a.editSessions[sess.ID] = sess
	a.editSessionByChar[charIdx] = sess.ID

	return sess.Workspace, nil
}

// GetInventoryEditSession returns the current workspace snapshot for a
// session ID. Errors if the session is not active.
func (a *App) GetInventoryEditSession(sessionID string) (editor.InventoryWorkspaceSnapshot, error) {
	sess, ok := a.editSessions[sessionID]
	if !ok {
		return editor.InventoryWorkspaceSnapshot{}, fmt.Errorf("inventory edit session %q not found", sessionID)
	}
	return sess.Workspace, nil
}

// ValidateInventoryWorkspace re-runs dry-run validation on the active
// session's workspace and returns the report. The workspace itself is
// updated with the latest report so subsequent Get calls see it too.
func (a *App) ValidateInventoryWorkspace(sessionID string) (editor.WorkspaceValidationReport, error) {
	sess, ok := a.editSessions[sessionID]
	if !ok {
		return editor.WorkspaceValidationReport{}, fmt.Errorf("inventory edit session %q not found", sessionID)
	}
	rep := editor.Validate(sess.Workspace)
	sess.Workspace.Validation = rep
	return rep, nil
}

// MoveInventoryWorkspaceItem relocates an editable item inside the
// session's workspace. targetContainer must be "inventory" or "storage".
// The mutation lives only in RAM — slot.Data is not touched and no
// rebuild is triggered.
func (a *App) MoveInventoryWorkspaceItem(sessionID, itemUID, targetContainer string, targetPosition int) (editor.InventoryWorkspaceSnapshot, error) {
	sess, ok := a.editSessions[sessionID]
	if !ok {
		return editor.InventoryWorkspaceSnapshot{}, fmt.Errorf("inventory edit session %q not found", sessionID)
	}
	ck, err := parseContainerKind(targetContainer)
	if err != nil {
		return editor.InventoryWorkspaceSnapshot{}, err
	}
	if err := editor.MoveItem(&sess.Workspace, itemUID, ck, targetPosition); err != nil {
		return editor.InventoryWorkspaceSnapshot{}, err
	}
	return sess.Workspace, nil
}

// TransferInventoryWorkspaceItem is a convenience wrapper that moves an
// editable item to the end of the target container. Equivalent to
// MoveInventoryWorkspaceItem with targetPosition past the slice length
// (MoveItem clamps to append).
func (a *App) TransferInventoryWorkspaceItem(sessionID, itemUID, targetContainer string) (editor.InventoryWorkspaceSnapshot, error) {
	sess, ok := a.editSessions[sessionID]
	if !ok {
		return editor.InventoryWorkspaceSnapshot{}, fmt.Errorf("inventory edit session %q not found", sessionID)
	}
	ck, err := parseContainerKind(targetContainer)
	if err != nil {
		return editor.InventoryWorkspaceSnapshot{}, err
	}
	// Sentinel above any realistic slice length — MoveItem clamps to append.
	if err := editor.MoveItem(&sess.Workspace, itemUID, ck, 1<<30); err != nil {
		return editor.InventoryWorkspaceSnapshot{}, err
	}
	return sess.Workspace, nil
}

// AddInventoryWorkspaceItem inserts a new editable item into the
// session's workspace. The item carries Source=added, OriginalHandle=0,
// HasGaItem=false — real handle/GaItem allocation happens at Save time
// (Phase 3+). Slot binary is untouched.
func (a *App) AddInventoryWorkspaceItem(sessionID string, spec editor.AddItemSpec, targetContainer string, targetPosition int) (editor.InventoryWorkspaceSnapshot, error) {
	sess, ok := a.editSessions[sessionID]
	if !ok {
		return editor.InventoryWorkspaceSnapshot{}, fmt.Errorf("inventory edit session %q not found", sessionID)
	}
	ck, err := parseContainerKind(targetContainer)
	if err != nil {
		return editor.InventoryWorkspaceSnapshot{}, err
	}
	if err := editor.AddItem(&sess.Workspace, spec, ck, targetPosition); err != nil {
		return editor.InventoryWorkspaceSnapshot{}, err
	}
	return sess.Workspace, nil
}

// UpdateInventoryWorkspaceWeapon applies a RAM-only patch (upgrade,
// infusion, pending Ash of War) to a weapon-editable item in the
// session's workspace. The slot binary, GaItems map, and any handle
// tables are NOT touched — final encoding into the save runs at Save
// time (Phase 3+).
//
// Errors are returned for unknown session, unknown UID, non-weapon
// item, or any invalid patch field (upgrade out of range, unknown
// infusion, unknown / non-AoW ID).
func (a *App) UpdateInventoryWorkspaceWeapon(sessionID, itemUID string, patch editor.WeaponPatch) (editor.InventoryWorkspaceSnapshot, error) {
	sess, ok := a.editSessions[sessionID]
	if !ok {
		return editor.InventoryWorkspaceSnapshot{}, fmt.Errorf("inventory edit session %q not found", sessionID)
	}
	if err := editor.UpdateWeapon(&sess.Workspace, itemUID, patch); err != nil {
		return editor.InventoryWorkspaceSnapshot{}, err
	}
	return sess.Workspace, nil
}

// RemoveInventoryWorkspaceItem deletes an editable item from the
// session's workspace by UID. Pass-through (unsupported) records are
// unaffected.
func (a *App) RemoveInventoryWorkspaceItem(sessionID, itemUID string) (editor.InventoryWorkspaceSnapshot, error) {
	sess, ok := a.editSessions[sessionID]
	if !ok {
		return editor.InventoryWorkspaceSnapshot{}, fmt.Errorf("inventory edit session %q not found", sessionID)
	}
	if err := editor.RemoveItem(&sess.Workspace, itemUID); err != nil {
		return editor.InventoryWorkspaceSnapshot{}, err
	}
	return sess.Workspace, nil
}

// parseContainerKind validates the wire string and returns the typed
// ContainerKind. Used by every mutation endpoint to normalize input
// before reaching the editor package.
func parseContainerKind(s string) (editor.ContainerKind, error) {
	switch s {
	case string(editor.ContainerInventory):
		return editor.ContainerInventory, nil
	case string(editor.ContainerStorage):
		return editor.ContainerStorage, nil
	}
	return "", fmt.Errorf("invalid container %q (want 'inventory' or 'storage')", s)
}

// SaveInventoryWorkspaceChanges commits the workspace's RAM-only edits
// into slot.Data via editor.ApplyWorkspaceSave (Phase 3A — supports
// reorder + add + weapon upgrade/infusion + pass-through preservation).
//
// Failure / rejection contract:
//   - Validation errors, pending AoW, transfer, remove, or capacity
//     overflow → return error WITHOUT mutating slot.Data; session
//     stays Dirty=true so the user can revise the workspace.
//   - Mutation error after writes begin → roll back via
//     core.RestoreSlot; session stays Dirty=true.
//
// Success contract:
//   - slot.Data, slot.GaMap, slot.GaItems updated atomically.
//   - A fresh snapshot is built from the reparsed slot and replaces
//     sess.Workspace; Dirty=false; BaseRevision refreshed; baseline
//     handle map regenerated.
//   - pushUndo runs first so the user can revert via the existing undo
//     stack.
func (a *App) SaveInventoryWorkspaceChanges(sessionID string) (editor.InventoryWorkspaceSnapshot, error) {
	sess, ok := a.editSessions[sessionID]
	if !ok {
		return editor.InventoryWorkspaceSnapshot{}, fmt.Errorf("inventory edit session %q not found", sessionID)
	}
	if a.save == nil {
		return editor.InventoryWorkspaceSnapshot{}, fmt.Errorf("no save loaded")
	}
	if sess.CharacterIndex < 0 || sess.CharacterIndex >= len(a.save.Slots) {
		return editor.InventoryWorkspaceSnapshot{}, fmt.Errorf("session character index %d out of range", sess.CharacterIndex)
	}
	slot := &a.save.Slots[sess.CharacterIndex]

	// Atomic snapshot for partial-mutation rollback. Separate from
	// pushUndo (user-facing) so a mid-save failure doesn't bloat the
	// undo stack with a half-mutated state.
	rollback := core.SnapshotSlot(slot)

	// User-visible undo: push BEFORE any mutation so the user can
	// revert the entire save via the existing undo button.
	a.pushUndo(sess.CharacterIndex)

	_, err := editor.ApplyWorkspaceSave(slot, &sess.Workspace, sess.BaselineEditableHandles)
	if err != nil {
		// Determine whether to rollback. Rejection errors return
		// before any mutation; mutation errors may have partially
		// changed slot. Restore unconditionally to be safe — it's a
		// no-op for byte-identical data.
		core.RestoreSlot(slot, rollback)
		return editor.InventoryWorkspaceSnapshot{}, err
	}

	// Build a fresh snapshot from the reparsed slot.
	fresh, err := editor.BuildSnapshot(slot, sess.ID, sess.CharacterIndex)
	if err != nil {
		core.RestoreSlot(slot, rollback)
		return editor.InventoryWorkspaceSnapshot{}, fmt.Errorf("SaveInventoryWorkspaceChanges: rebuild snapshot: %w", err)
	}
	fresh.Validation = editor.Validate(fresh)
	sess.Workspace = fresh
	sess.BaseRevision = editor.ComputeBaseRevision(slot)

	// Regenerate baseline from the post-save state — subsequent edits
	// in the same session detect transfer/remove relative to NOW.
	sess.BaselineEditableHandles = make(map[uint32]editor.ContainerKind, len(fresh.InventoryItems)+len(fresh.StorageItems))
	for _, it := range fresh.InventoryItems {
		if it.Source == editor.ItemSourceOriginal && it.OriginalHandle != 0 {
			sess.BaselineEditableHandles[it.OriginalHandle] = editor.ContainerInventory
		}
	}
	for _, it := range fresh.StorageItems {
		if it.Source == editor.ItemSourceOriginal && it.OriginalHandle != 0 {
			sess.BaselineEditableHandles[it.OriginalHandle] = editor.ContainerStorage
		}
	}

	return sess.Workspace, nil
}

// DiscardInventoryEditSession deletes the session by ID. It is a no-op
// if the session does not exist (idempotent — frontends can call this
// during cleanup without checking existence first).
func (a *App) DiscardInventoryEditSession(sessionID string) error {
	sess, ok := a.editSessions[sessionID]
	if !ok {
		return nil
	}
	delete(a.editSessions, sessionID)
	if a.editSessionByChar[sess.CharacterIndex] == sessionID {
		delete(a.editSessionByChar, sess.CharacterIndex)
	}
	return nil
}

// _forceExportTypesInventorySession surfaces the editor package's DTOs to
// the Wails type generator. It is never called — the function signature
// alone teaches the generator about each type so TypeScript bindings
// include them. Kept here (instead of bundled into app.go's
// _forceExportTypes) to keep the Phase 1 patch small.
func (a *App) _forceExportTypesInventorySession() (
	editor.InventoryWorkspaceSnapshot,
	editor.EditableItem,
	editor.RawInventoryRecord,
	editor.WorkspaceValidationReport,
	editor.WorkspaceValidationIssue,
	editor.AddItemSpec,
	editor.WeaponPatch,
) {
	return editor.InventoryWorkspaceSnapshot{},
		editor.EditableItem{},
		editor.RawInventoryRecord{},
		editor.WorkspaceValidationReport{},
		editor.WorkspaceValidationIssue{},
		editor.AddItemSpec{},
		editor.WeaponPatch{}
}
