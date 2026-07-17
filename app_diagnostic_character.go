package main

import "strconv"

// characterChangeOutcome is the closed set of terminal outcomes a Character
// mutation may report on character_change_finished. Keeping it a small typed
// vocabulary stops a future caller from inventing free-text outcomes that a
// diagnostics reader would then have to special-case.
type characterChangeOutcome string

const (
	characterChangeSuccess characterChangeOutcome = "success"
	characterChangeError   characterChangeOutcome = "error"
	characterChangeNoop    characterChangeOutcome = "noop"
)

// Lifecycle event names for a single Character field mutation. A future
// mutation endpoint emits, per changed field, one record for each phase in
// this order: before -> planned -> finished.
const (
	eventCharacterChangeBefore   = "character_change_before"
	eventCharacterChangePlanned  = "character_change_planned"
	eventCharacterChangeFinished = "character_change_finished"
)

// characterFieldChange describes one scalar field a Character mutation touches.
// All three members are plain strings by deliberate design: the helpers below
// pass them straight through the central journal sanitizer, so a caller can
// never smuggle a map, interface, byte slice, raw slot data, or arbitrary
// frontend payload into the diagnostic log — only already-scalar technical
// values reach the journal.
type characterFieldChange struct {
	Field  string // logical field name, e.g. "vigor", "name"
	Before string // value before the change
	After  string // planned value (planned phase) or actual value (finished phase)
}

// journalCharacterChangeBefore records the pre-change state of each field, one
// debug record per field, before any new value is computed. Only "before" is
// meaningful at this phase, so "after" is omitted.
func (a *App) journalCharacterChangeBefore(action string, characterIndex int, changes []characterFieldChange) {
	for _, ch := range changes {
		a.journalDebug(eventCharacterChangeBefore, "character change before",
			field("action", action),
			field("character_index", strconv.Itoa(characterIndex)),
			field("field", ch.Field),
			field("before", ch.Before),
		)
	}
}

// journalCharacterChangePlanned records the intended new value of each field,
// one debug record per field, after it is computed but before it is applied.
func (a *App) journalCharacterChangePlanned(action string, characterIndex int, changes []characterFieldChange) {
	for _, ch := range changes {
		a.journalDebug(eventCharacterChangePlanned, "character change planned",
			field("action", action),
			field("character_index", strconv.Itoa(characterIndex)),
			field("field", ch.Field),
			field("before", ch.Before),
			field("after", ch.After),
		)
	}
}

// journalCharacterChangeFinished records the terminal state of each field, one
// debug record per field, once the mutation has run. outcome and stage report
// how and where it ended; After holds the actual applied (or attempted) value.
func (a *App) journalCharacterChangeFinished(action string, characterIndex int, outcome characterChangeOutcome, stage string, changes []characterFieldChange) {
	for _, ch := range changes {
		a.journalDebug(eventCharacterChangeFinished, "character change finished",
			field("action", action),
			field("character_index", strconv.Itoa(characterIndex)),
			field("field", ch.Field),
			field("before", ch.Before),
			field("after", ch.After),
			field("outcome", string(outcome)),
			field("stage", stage),
		)
	}
}
