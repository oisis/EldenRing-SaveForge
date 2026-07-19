package main

import "github.com/oisis/EldenRing-SaveForge/backend/core"

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

// Operation-level lifecycle event names bracketing a whole Tools endpoint call.
// Unlike the per-field tools_change_* records these fire even when a request is
// rejected before any field can change (no active save, unparseable input), so a
// diagnostics reader always sees a requested/finished pair. Both are debug-only
// and carry no input: requested has only the action, finished adds a closed
// outcome and stage. A Steam ID value, the raw input string and any parser error
// text are NEVER among their fields.
const (
	eventToolsOperationRequested = "tools_operation_requested"
	eventToolsOperationFinished  = "tools_operation_finished"
)

// Closed technical stages a tools_operation_finished may report. completed reuses
// the shared success stage; no_active_save and parse mark the two rejection points
// before a mutation. The vocabulary is small, typed and value-free.
const (
	toolsStageNoActiveSave = "no_active_save"
	toolsStageParse        = "parse"
	toolsStageCompleted    = characterStageCompleted
)

// journalToolsOperationRequested opens the operation lifecycle before any
// validation runs. The only field is the closed action tag — never the input.
func (a *App) journalToolsOperationRequested(action string) {
	a.journalDebug(eventToolsOperationRequested, "tools operation requested", field("action", action))
}

// journalToolsOperationFinished closes the operation lifecycle on every exit path.
// outcome and stage come from the closed vocabularies above; no value or error
// text is ever attached.
func (a *App) journalToolsOperationFinished(action string, outcome characterChangeOutcome, stage string) {
	a.journalDebug(eventToolsOperationFinished, "tools operation finished",
		field("action", action),
		field("outcome", string(outcome)),
		field("stage", stage))
}

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

// actionToolsRepairDuplicateInventoryIndices tags every tools_change_* record
// emitted for RepairDuplicateInventoryIndices. Closed and backend-owned — never
// derived from renderer input.
const actionToolsRepairDuplicateInventoryIndices = "tools_repair_duplicate_inventory_indices"

// stageToolsRepairDuplicateInventoryIndices is the closed terminal stage a failed
// duplicate-inventory-index repair reports on its finished records; a successful
// repair reuses the shared toolsStageCompleted, so the finished stage stays a
// small typed vocabulary.
const stageToolsRepairDuplicateInventoryIndices = "repair_duplicate_inventory_indices"

// planToolsInventoryRepair projects exactly the physical save changes the
// duplicate-inventory-index repair makes, reusing the Game Items physical readers
// and semantic GaMap/cursor/header readers without duplicating a scanner. It
// covers, in the same stable order the Game Items lifecycle uses:
//   - every changed inventory common/key row index, plus any other physically
//     changed inventory/GaItem record (including a zeroed surplus Wondrous
//     Physick row and its now-empty GaItem), via the shared direct scanner;
//   - the held-inventory CommonItems count header, when a Physick removal
//     rewrites it in place;
//   - the GaMap value changes and GaItem allocation cursors, when they actually
//     move.
//
// The direct scanner also reads storage rows, the storage counters and (through
// the unused flag/storage-header planners, which this projection deliberately
// omits) other families, but every field self-excludes when before == planned,
// so the records this repair never touches — storage, event flags — never reach
// the journal. Only physically changed fields are emitted.
func planToolsInventoryRepair(before, planned *core.SaveSlot) gameItemMutationPlans {
	return gameItemMutationPlans{
		direct:      planGameItemsDirectRecords(before, planned, nil),
		invHeader:   planGameItemsAddInventoryHeaderRecords(before, planned),
		gaItemState: planGameItemsGaItemState(before, planned),
	}
}

// journalToolsInventoryRepairBefore emits the before(all) then planned(all)
// tools_change_* records for a duplicate-inventory-index repair. Unlike the
// save-wide Steam ID lifecycle it carries the real character_index, because the
// repair mutates one character's inventory. It routes through the same shared
// journalChangeRecords loop and change tails as every other lifecycle, so a
// diagnostics reader treats these records identically.
func (a *App) journalToolsInventoryRepairBefore(charIdx int, plans gameItemMutationPlans) {
	records := plans.records()
	a.journalChangeRecords(eventToolsChangeBefore, "tools change before", actionToolsRepairDuplicateInventoryIndices, charIdx, records, nil)
	a.journalChangeRecords(eventToolsChangePlanned, "tools change planned", actionToolsRepairDuplicateInventoryIndices, charIdx, records, changePlannedTail)
}

// journalToolsInventoryRepairFinished emits the finished(all) tools_change_*
// records, re-reading each field from the real post-mutation slot so After
// reflects what actually landed — on a mid-repair error, the real partially
// changed state, not the planned projection or a snapshot.
func (a *App) journalToolsInventoryRepairFinished(charIdx int, outcome characterChangeOutcome, stage string, plans gameItemMutationPlans, slot *core.SaveSlot) {
	a.journalChangeRecords(eventToolsChangeFinished, "tools change finished", actionToolsRepairDuplicateInventoryIndices, charIdx, plans.finished(slot), changeFinishedTail(outcome, stage))
}
