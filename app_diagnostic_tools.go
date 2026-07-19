package main

// Tools-tab diagnostics reuse the same before -> planned -> finished lifecycle as
// Character, Game Items, World and Advanced, through the shared
// journalChangeRecords loop, characterFieldChange type and change*Tail helpers in
// app_diagnostic_character.go. This file only supplies the Tools-specific event
// names, thin phase helpers and the privacy-safe Steam ID planner/mapper; the
// sanitizer and per-field emission are never re-implemented.
//
// The one Tools mutation with a permanent effect on a save is SetSteamIDFromString.
// A Steam ID is private account data, so it is NEVER journalled raw, partial,
// hashed or encoded. Instead each steam_id field value is reduced to exactly one
// of two literals — "absent" (id 0) or "[redacted]" (any non-zero id) — before it
// ever reaches a characterFieldChange. The raw uint64 ids are compared only inside
// the planner and mapper and never stored, so no record can leak the number, its
// length, or a fragment of it.

// actionToolsSetSteamID is the action tag every Steam ID Tools diagnostic record
// carries. The string is closed and backend-owned — never derived from renderer
// input.
const actionToolsSetSteamID = "tools_set_steam_id"

// stageToolsSetSteamID is the closed technical stage a Steam ID mutation reports
// on a failed finished record; a successful write reports the shared
// characterStageCompleted, so the finished stage stays a small typed vocabulary.
const stageToolsSetSteamID = "set_steam_id"

// toolsFieldSteamID is the single logical field name a Steam ID change emits. It
// names the field, never the value — the value is always a redacted literal.
const toolsFieldSteamID = "steam_id"

// toolsCharacterIndex is the character_index every Tools record carries. A Steam ID
// is metadata of the whole save (SaveFile.SteamID), not the state of any one
// character, so the slot index is a fixed "no character" -1.
const toolsCharacterIndex = -1

// Redacted representations of a Steam ID. absent and redacted are the ONLY two
// values a steam_id field may ever hold in the journal: absent distinguishes an
// unset id (0) from a present one, redacted stands in for every non-zero id without
// revealing its value, length or any digit of it.
const (
	steamIDAbsent   = "absent"
	steamIDRedacted = "[redacted]"
)

// Lifecycle event names for a single Tools field mutation. Per changed field one
// record is emitted for each phase in this order: before -> planned -> finished.
const (
	eventToolsChangeBefore   = "tools_change_before"
	eventToolsChangePlanned  = "tools_change_planned"
	eventToolsChangeFinished = "tools_change_finished"
)

// steamIDState reduces a raw Steam ID to its journal-safe redacted representation:
// "absent" for 0, "[redacted]" for any non-zero id. This is the single privacy
// boundary for the value — the raw uint64 never leaves the planner/mapper.
func steamIDState(id uint64) string {
	if id == 0 {
		return steamIDAbsent
	}
	return steamIDRedacted
}

// journalToolsChangeBefore records the pre-change state of each field, one debug
// record per field, before any new value is applied. Only "before" is meaningful at
// this phase, so "after"/outcome/stage are omitted.
func (a *App) journalToolsChangeBefore(action string, changes []characterFieldChange) {
	a.journalChangeRecords(eventToolsChangeBefore, "tools change before", action, toolsCharacterIndex, changes, nil)
}

// journalToolsChangePlanned records the intended new value of each field, one debug
// record per field, after it is computed but before it is applied.
func (a *App) journalToolsChangePlanned(action string, changes []characterFieldChange) {
	a.journalChangeRecords(eventToolsChangePlanned, "tools change planned", action, toolsCharacterIndex, changes, changePlannedTail)
}

// journalToolsChangeFinished records the terminal state of each field, one debug
// record per field, once the mutation has run. outcome and stage report how and
// where it ended; After holds the actual applied (or attempted) value.
func (a *App) journalToolsChangeFinished(action string, outcome characterChangeOutcome, stage string, changes []characterFieldChange) {
	a.journalChangeRecords(eventToolsChangeFinished, "tools change finished", action, toolsCharacterIndex, changes, changeFinishedTail(outcome, stage))
}

// toolsSteamIDPlan is the Steam ID field change when the planned id differs from the
// before id. before/planned hold ONLY the redacted representation (absent /
// [redacted]); the raw ids are compared in planToolsSteamIDChange and never stored,
// so the plan itself cannot leak them. For a non-zero -> non-zero replacement both
// fields are "[redacted]", yet the plan still exists because the compared raw ids
// differ — the lifecycle stays intact while the value stays hidden.
type toolsSteamIDPlan struct {
	before  string
	planned string
}

// planToolsSteamIDChange compares the real uint64 ids. Identical ids -> no plan
// (noop, and therefore no records). A change -> exactly one plan for the single
// steam_id logical field. The comparison is on the raw uint64s, not the redacted
// strings, so a non-zero -> non-zero replacement (both redacted to "[redacted]") is
// still detected as a change rather than collapsing into a false noop.
func planToolsSteamIDChange(before, planned uint64) []toolsSteamIDPlan {
	if before == planned {
		return nil
	}
	return []toolsSteamIDPlan{{before: steamIDState(before), planned: steamIDState(planned)}}
}

// toolsSteamIDPlannedRecords maps plans to before/planned records: the before phase
// ignores After, the planned phase uses it, mirroring advancedPlannedRecords.
func toolsSteamIDPlannedRecords(plans []toolsSteamIDPlan) []characterFieldChange {
	out := make([]characterFieldChange, len(plans))
	for i, p := range plans {
		out[i] = characterFieldChange{Field: toolsFieldSteamID, Before: p.before, After: p.planned}
	}
	return out
}

// toolsSteamIDFinishedRecords maps plans to finished records, reducing the actual
// post-operation id to its redacted representation so the After column reflects the
// real state that landed (absent vs [redacted]) without revealing the number. On a
// failed write the caller passes the unchanged id, so finished can still tell an
// untouched "absent" apart from a "[redacted]".
func toolsSteamIDFinishedRecords(plans []toolsSteamIDPlan, actual uint64) []characterFieldChange {
	out := make([]characterFieldChange, len(plans))
	for i, p := range plans {
		out[i] = characterFieldChange{Field: toolsFieldSteamID, Before: p.before, After: steamIDState(actual)}
	}
	return out
}
