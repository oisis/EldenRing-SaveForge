package main

import (
	"errors"
	"fmt"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
	"github.com/oisis/EldenRing-SaveForge/backend/editor"
)

// acquireSession looks up a session by ID under the registry mutex and
// then takes the per-session lock. The two locks are deliberately not
// held simultaneously: registry lock is short-lived (just the map probe)
// so concurrent StartInventoryEditSession for a different character is
// not blocked by a long-running Save on this one.
//
// A missing session ID and a closed session both map to the same
// "session ... not found" wire error: the frontend hook
// (useInventoryWorkspace.runSessionOp) self-heals on that exact phrase
// by restarting the session, which is the right recovery for both
// causes from the UI's perspective.
//
// On nil error the caller owns the session lock and MUST release it via
// sess.Unlock() (typically defer).
func (a *App) acquireSession(sessionID string) (*editor.InventoryEditSession, error) {
	a.editSessionsMu.Lock()
	sess, ok := a.editSessions[sessionID]
	a.editSessionsMu.Unlock()
	if !ok {
		return nil, fmt.Errorf("inventory edit session %q not found", sessionID)
	}
	if err := sess.Acquire(); err != nil {
		// Either ErrSessionClosed (Discard / clearAllEditSessions raced
		// us between the registry probe and Acquire) — surface the same
		// wire shape as "not found" for the frontend self-heal path.
		if errors.Is(err, editor.ErrSessionClosed) {
			return nil, fmt.Errorf("inventory edit session %q not found", sessionID)
		}
		return nil, err
	}
	return sess, nil
}

// StartInventoryEditSession builds a read-only workspace snapshot for the
// given character slot and registers it as the active session for that
// character. If a session already exists for the same charIdx it is
// replaced atomically.
//
// Phase 1 contract: the slot is not mutated, no undo is pushed, no rebuild
// is performed. Future phases will accept mutations on the returned
// session and only then call into the rebuild pipeline.
//
// Lifecycle concurrency contract:
//   - Holds lifecycleMu[charIdx] across the entire replacement sequence
//     so a concurrent Start, Discard, or clearAllEditSessions for the
//     same character is serialised. Different characters take different
//     lifecycle locks and run independently.
//   - The prior session for this charIdx (if any) is evicted from the
//     registry FIRST, then closeSession is awaited. closeSession blocks
//     on the per-session mutex, which a peer SaveInventoryWorkspaceChanges
//     holds for the entire apply → rebuild snapshot → regenerate baseline
//     sequence. As a result editor.StartSession (which reads
//     a.save.Slots[charIdx]) is invoked ONLY after the previous session
//     has finished mutating the same slot — no torn read on slot.Data /
//     slot.GaMap / slot.Inventory / slot.Storage.
//   - The new session is published under editSessionsMu (short
//     critical section, registry maps only) so the original
//     concurrent map writes crash signature is also covered.
//   - Replacement semantics are preserved: the call always returns a
//     fresh snapshot of the slot as it stands AFTER any prior Save has
//     drained, never the prior session's dirty workspace. SortOrderTab's
//     effect (re-runs on charIndex / inventoryVersion change) still gets
//     the post-event snapshot it depends on.
func (a *App) StartInventoryEditSession(charIdx int) (editor.InventoryWorkspaceSnapshot, error) {
	if a.save == nil {
		return editor.InventoryWorkspaceSnapshot{}, fmt.Errorf("no save loaded")
	}
	if charIdx < 0 || charIdx >= len(a.save.Slots) {
		return editor.InventoryWorkspaceSnapshot{}, fmt.Errorf("invalid character index %d", charIdx)
	}

	a.lifecycleMu[charIdx].Lock()
	defer a.lifecycleMu[charIdx].Unlock()

	slot := &a.save.Slots[charIdx]
	if slot.Version == 0 {
		return editor.InventoryWorkspaceSnapshot{}, fmt.Errorf("slot %d is empty", charIdx)
	}

	// Step 1 — evict any prior session for this character from the
	// registry. We deliberately delete BEFORE building the new snapshot
	// so peer endpoints holding only the prior session's ID see "not
	// found" immediately and the frontend self-heal triggers a fresh
	// Start (which then queues behind us on lifecycleMu).
	var prior *editor.InventoryEditSession
	a.editSessionsMu.Lock()
	if oldID, ok := a.editSessionByChar[charIdx]; ok {
		if p, ok2 := a.editSessions[oldID]; ok2 {
			prior = p
		}
		delete(a.editSessions, oldID)
		delete(a.editSessionByChar, charIdx)
	}
	a.editSessionsMu.Unlock()

	// Step 2 — drain the prior session BEFORE touching the slot.
	// closeSession blocks until any in-flight Save (or other mutator)
	// has released the per-session lock, so editor.StartSession below
	// reads a quiesced slot.
	if prior != nil {
		closeSession(prior)
	}

	// Step 3 — build the fresh snapshot off the now-quiet slot.
	sess, err := editor.StartSession(slot, charIdx)
	if err != nil {
		return editor.InventoryWorkspaceSnapshot{}, err
	}

	// Step 4 — publish under the registry lock.
	a.editSessionsMu.Lock()
	a.editSessions[sess.ID] = sess
	a.editSessionByChar[charIdx] = sess.ID
	a.editSessionsMu.Unlock()

	return sess.Workspace, nil
}

// closeSession waits for any in-flight mutator on a session to drain,
// then marks it closed so any peer that subsequently calls Acquire
// fails fast. Used by replacement (StartInventoryEditSession), explicit
// Discard, and the bulk clearAllEditSessions path.
func closeSession(s *editor.InventoryEditSession) {
	if err := s.Acquire(); err == nil {
		s.Close()
		s.Unlock()
	}
}

// GetInventoryEditSession returns the current workspace snapshot for a
// session ID. Errors if the session is not active.
//
// The snapshot is read under the session lock and returned by value, so
// the caller receives a self-contained copy that cannot tear under a
// concurrent mutator.
func (a *App) GetInventoryEditSession(sessionID string) (editor.InventoryWorkspaceSnapshot, error) {
	sess, err := a.acquireSession(sessionID)
	if err != nil {
		return editor.InventoryWorkspaceSnapshot{}, err
	}
	defer sess.Unlock()
	return sess.Workspace, nil
}

// ValidateInventoryWorkspace re-runs dry-run validation on the active
// session's workspace and returns the report. The workspace itself is
// updated with the latest report so subsequent Get calls see it too.
func (a *App) ValidateInventoryWorkspace(sessionID string) (editor.WorkspaceValidationReport, error) {
	sess, err := a.acquireSession(sessionID)
	if err != nil {
		return editor.WorkspaceValidationReport{}, err
	}
	defer sess.Unlock()
	rep := editor.Validate(sess.Workspace)
	sess.Workspace.Validation = rep
	return rep, nil
}

// MoveInventoryWorkspaceItem relocates an editable item inside the
// session's workspace. targetContainer must be "inventory" or "storage".
// The mutation lives only in RAM — slot.Data is not touched and no
// rebuild is triggered.
func (a *App) MoveInventoryWorkspaceItem(sessionID, itemUID, targetContainer string, targetPosition int) (editor.InventoryWorkspaceSnapshot, error) {
	sess, err := a.acquireSession(sessionID)
	if err != nil {
		return editor.InventoryWorkspaceSnapshot{}, err
	}
	defer sess.Unlock()
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
	sess, err := a.acquireSession(sessionID)
	if err != nil {
		return editor.InventoryWorkspaceSnapshot{}, err
	}
	defer sess.Unlock()
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
	sess, err := a.acquireSession(sessionID)
	if err != nil {
		return editor.InventoryWorkspaceSnapshot{}, err
	}
	defer sess.Unlock()
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
	sess, err := a.acquireSession(sessionID)
	if err != nil {
		return editor.InventoryWorkspaceSnapshot{}, err
	}
	defer sess.Unlock()
	if err := editor.UpdateWeapon(&sess.Workspace, itemUID, patch); err != nil {
		return editor.InventoryWorkspaceSnapshot{}, err
	}
	return sess.Workspace, nil
}

// RemoveInventoryWorkspaceItem deletes an editable item from the
// session's workspace by UID. Pass-through (unsupported) records are
// unaffected.
func (a *App) RemoveInventoryWorkspaceItem(sessionID, itemUID string) (editor.InventoryWorkspaceSnapshot, error) {
	sess, err := a.acquireSession(sessionID)
	if err != nil {
		return editor.InventoryWorkspaceSnapshot{}, err
	}
	defer sess.Unlock()
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
// into slot.Data via editor.ApplyWorkspaceSave (Phase 3B — supports
// reorder + add + transfer + remove + weapon upgrade/infusion +
// pass-through preservation; pending AoW still rejected for Phase 4).
//
// Failure / rejection contract:
//   - Validation errors, pending AoW, or capacity overflow → return
//     error WITHOUT mutating slot.Data; session stays Dirty=true so
//     the user can revise the workspace.
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
//
// Concurrency contract (this fix):
//   - The session lock is held across the entire flow (apply → rebuild
//     snapshot → regenerate baseline). No peer mutator can observe a
//     partially-replaced Workspace or a half-initialised
//     BaselineEditableHandles map — historically the rebuild loop at the
//     end could race a concurrent AddInventoryWorkspaceItem and tear
//     either the slice or the baseline map.
func (a *App) SaveInventoryWorkspaceChanges(sessionID string) (editor.InventoryWorkspaceSnapshot, error) {
	sess, err := a.acquireSession(sessionID)
	if err != nil {
		return editor.InventoryWorkspaceSnapshot{}, err
	}
	defer sess.Unlock()
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

	_, err = editor.ApplyWorkspaceSave(slot, &sess.Workspace, sess.BaselineEditableHandles)
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
//
// Lifecycle concurrency contract:
//   - First takes a short editSessionsMu probe to discover the session's
//     charIdx (sessionIDs are opaque to the caller).
//   - Then takes lifecycleMu[charIdx] so a concurrent StartInventoryEditSession
//     for the same character cannot publish a new session and call
//     editor.StartSession on the slot while we are still draining the
//     prior Save.
//   - Re-probes the registry under editSessionsMu: a parallel
//     StartInventoryEditSession (also lifecycle-locked, but it would
//     have run BEFORE us if it won the lifecycle lock) could have
//     already evicted the same session ID. In that case there is
//     nothing left for us to delete and Discard is a no-op — exactly
//     the idempotent behaviour the frontend cleanup path relies on.
//   - The registry rows are deleted under editSessionsMu, then the
//     session is closeSession-d outside it. closeSession waits for any
//     in-flight Save to release the per-session lock, so by the time
//     Discard returns no orphan goroutine is still mutating the slot.
//   - Lock order is identical to Start (lifecycleMu[charIdx] →
//     editSessionsMu → sess.Acquire()), so no reverse-cycle deadlock.
func (a *App) DiscardInventoryEditSession(sessionID string) error {
	// Probe charIdx so we know which lifecycle lock to take. Sessions
	// IDs are unique random hex (editor.NewSessionID), so the same ID
	// never re-appears under a different charIdx — even if the entry is
	// later replaced, the charIdx we read here is correct for the
	// session we eventually close.
	a.editSessionsMu.Lock()
	probe, ok := a.editSessions[sessionID]
	a.editSessionsMu.Unlock()
	if !ok {
		return nil
	}
	charIdx := probe.CharacterIndex
	if charIdx < 0 || charIdx >= maxCharacters {
		// Defensive: a session pointing at an out-of-range character
		// should be impossible — Start rejects such inputs — but we
		// would rather no-op than index out of bounds.
		return nil
	}

	a.lifecycleMu[charIdx].Lock()
	defer a.lifecycleMu[charIdx].Unlock()

	// Re-probe under editSessionsMu: a peer Start that won the
	// lifecycle lock before us would have already evicted this session
	// ID. In that case the prior session has already been close()-d by
	// Start and there's nothing left for us to do.
	a.editSessionsMu.Lock()
	sess, stillThere := a.editSessions[sessionID]
	if !stillThere {
		a.editSessionsMu.Unlock()
		return nil
	}
	delete(a.editSessions, sessionID)
	if a.editSessionByChar[sess.CharacterIndex] == sessionID {
		delete(a.editSessionByChar, sess.CharacterIndex)
	}
	a.editSessionsMu.Unlock()

	closeSession(sess)
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
