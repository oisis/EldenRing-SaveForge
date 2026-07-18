package main

// Lifecycle event names for a single Game Items field mutation. Like the
// Character lifecycle, a future Game Items mutation endpoint emits, per changed
// field, one record for each phase in this order: before -> planned -> finished.
// These are the on-disk contract for a Game Items save mutation; this file is
// the foundation only and is not yet wired into any mutation.
const (
	eventGameItemsChangeBefore   = "game_items_change_before"
	eventGameItemsChangePlanned  = "game_items_change_planned"
	eventGameItemsChangeFinished = "game_items_change_finished"
)

// Game Items lifecycle reuses the Character change-field and outcome types on
// purpose: the journal contract (action, character_index, field, before, after,
// outcome, stage) is identical, so a diagnostics reader treats a Game Items
// mutation exactly like a Character one, and there is no parallel unrestricted
// outcome vocabulary to special-case. The three helpers below route through the
// same journalChangeRecords loop as their Character counterparts.

// journalGameItemChangeBefore records the pre-change state of each Game Items
// field, one debug record per field, before any new value is computed. Only
// "before" is meaningful at this phase, so "after" is omitted.
func (a *App) journalGameItemChangeBefore(action string, characterIndex int, changes []characterFieldChange) {
	a.journalChangeRecords(eventGameItemsChangeBefore, "game items change before", action, characterIndex, changes, nil)
}

// journalGameItemChangePlanned records the intended new value of each Game Items
// field, one debug record per field, after it is computed but before it is applied.
func (a *App) journalGameItemChangePlanned(action string, characterIndex int, changes []characterFieldChange) {
	a.journalChangeRecords(eventGameItemsChangePlanned, "game items change planned", action, characterIndex, changes, changePlannedTail)
}

// journalGameItemChangeFinished records the terminal state of each Game Items
// field, one debug record per field, once the mutation has run. outcome and
// stage report how and where it ended; After holds the actual applied (or
// attempted) value.
func (a *App) journalGameItemChangeFinished(action string, characterIndex int, outcome characterChangeOutcome, stage string, changes []characterFieldChange) {
	a.journalChangeRecords(eventGameItemsChangeFinished, "game items change finished", action, characterIndex, changes, changeFinishedTail(outcome, stage))
}
