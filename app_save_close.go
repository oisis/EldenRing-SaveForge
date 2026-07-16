package main

// CloseSave drops the currently-loaded active save without touching the file
// on disk. It is the explicit "Close save" path for the load-time inventory
// integrity gate: when the user opens a problematic save and chooses NOT to
// run repair, the backend cannot continue to serve a.save (every subsequent
// endpoint would operate on the known-bad state). A frontend-only state
// reset is insufficient because the Wails App keeps a.save in memory and
// would happily honour follow-up writes.
//
// Scope (deliberately narrow):
//   - Replaces a.save with nil.
//   - Clears every piece of derived state that installLoadedSave wires up
//     for a freshly-loaded file: lastSavePath, favSlotNames, undo stacks
//     and inventory edit sessions.
//   - Does NOT touch a.sourceSave — the source save is an independent
//     read-only handle owned by SelectAndOpenSourceSave / Character
//     Importer and is unrelated to the active editable save.
//   - Does NOT write the file or any backup.
//
// Lock strategy mirrors SelectAndOpenSave / DownloadRemoteSave: take
// a.saveMu.Lock exclusively for the entire reset so no in-flight
// reader/writer of the old a.save can observe a half-cleared state.
// clearAllEditSessions internally takes its own lifecycleMu + editSessionsMu
// in the documented order; that nested lock acquisition is safe because
// saveMu sits above both in the project-wide order (see app.go:117–128).
//
// Idempotent: calling CloseSave with no active save is a no-op and
// returns nil — consistent with the "drop without complaining" semantics
// the modal needs.
func (a *App) CloseSave() error {
	// diagnosticScopeMu (outermost, level 0) serialises this whole close
	// transition against concurrent loads so the reset + save_closed marker can
	// never be reordered relative to a later load's install + save_loaded. See
	// commitLoadedSave and the lock-order note in app.go.
	a.diagnosticScopeMu.Lock()
	defer a.diagnosticScopeMu.Unlock()

	a.saveMu.Lock()
	if a.save == nil {
		a.saveMu.Unlock()
		return nil // no-op: nothing to close, no scope marker to write
	}
	a.save = nil
	a.lastSavePath = ""
	a.saveGeneration++
	a.slotRevisions = [maxCharacters]uint64{}
	a.gaItemRepackTokens = make(map[string]gaItemRepackToken)
	a.favSlotNames = make(map[int]string)
	a.clearAllUndoStacks()
	a.clearAllEditSessions()
	a.saveMu.Unlock()

	// Ending the current-save scope: append save_closed AFTER the reset and
	// after releasing saveMu (journal Sync must never run under saveMu).
	// journalLog swallows any journal error, so a logging failure never fails
	// CloseSave.
	a.journalLog(levelInfo, eventSaveClosed, "active save closed")
	return nil
}
