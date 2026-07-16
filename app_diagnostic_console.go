package main

import "encoding/json"

// GetDiagnosticLogTail returns the already-sanitized, bounded tail as the
// journal's native JSON array. Returning the established JSONL record shape as
// one primitive keeps this read-only UI bridge independent of generated model
// classes: Wails updates only the method binding, not models.ts. The frontend
// validates this untrusted transport value before rendering it.
//
// The on-disk journal remains the durable source of truth. This endpoint is a
// live-console convenience limited by diagnosticTailMax and cannot read, alter,
// or reveal the session-file path.
func (a *App) GetDiagnosticLogTail() string {
	records := a.journal.Tail()
	if len(records) == 0 {
		return "[]"
	}
	data, err := json.Marshal(records)
	if err != nil {
		// diagnosticRecord contains concrete JSON-safe fields only, so this is
		// defensive. A valid empty JSON array keeps the UI safe if that ever
		// changes.
		return "[]"
	}
	return string(data)
}
