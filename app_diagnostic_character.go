package main

import (
	"strconv"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
	"github.com/oisis/EldenRing-SaveForge/backend/vm"
)

// actionSaveCharacter is the action tag every SaveCharacter-originated Character
// diagnostic record carries.
const actionSaveCharacter = "save_character"

// Closed technical stages a SaveCharacter mutation can terminate at. A finished
// record reports exactly one of these so a diagnostics reader never has to parse
// free-text stage names. completed is the only success stage.
const (
	characterStageApplyVM        = "apply_vm"
	characterStageSyncContainers = "sync_containers"
	characterStageMemoryStones   = "memory_stones"
	characterStageCompleted      = "completed"
)

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

// characterFieldPlan is one in-scope direct profile/attribute field that a
// SaveCharacter mutation intends to change. before/planned are captured before
// any mutation; read pulls the field's live value back out of a slot so the
// finished phase reports what actually landed instead of the request.
type characterFieldPlan struct {
	field   string
	before  string
	planned string
	read    func(core.PlayerGameData) string
}

// planCharacterSaveChanges builds the ordered list of in-scope direct fields
// whose normalized submitted target differs from the current slot value.
// Normalization for soul_memory / talisman_slots / clear_count reuses the same
// vm helpers ApplyVMToParsedSlot applies, so planned values cannot drift from
// what the writer persists. Operational side effects (Memory Stones inventory,
// NG+ flags, containers, quantities, ProfileSummary, appearance, favorites) are
// deliberately out of scope here.
func planCharacterSaveChanges(cur core.PlayerGameData, submitted *vm.CharacterViewModel) []characterFieldPlan {
	u32 := func(v uint32) string { return strconv.FormatUint(uint64(v), 10) }
	u8 := func(v uint8) string { return strconv.FormatUint(uint64(v), 10) }
	name := func(p core.PlayerGameData) string { return core.UTF16ToString(p.CharacterName[:]) }

	specs := []struct {
		field   string
		planned string
		read    func(core.PlayerGameData) string
	}{
		{"name", submitted.Name, name},
		{"class", u8(submitted.Class), func(p core.PlayerGameData) string { return u8(p.Class) }},
		{"level", u32(submitted.Level), func(p core.PlayerGameData) string { return u32(p.Level) }},
		{"souls", u32(submitted.Souls), func(p core.PlayerGameData) string { return u32(p.Souls) }},
		{"soul_memory", u32(vm.NormalizeSoulMemory(submitted.Level, submitted.SoulMemory)), func(p core.PlayerGameData) string { return u32(p.SoulMemory) }},
		{"vigor", u32(submitted.Vigor), func(p core.PlayerGameData) string { return u32(p.Vigor) }},
		{"mind", u32(submitted.Mind), func(p core.PlayerGameData) string { return u32(p.Mind) }},
		{"endurance", u32(submitted.Endurance), func(p core.PlayerGameData) string { return u32(p.Endurance) }},
		{"strength", u32(submitted.Strength), func(p core.PlayerGameData) string { return u32(p.Strength) }},
		{"dexterity", u32(submitted.Dexterity), func(p core.PlayerGameData) string { return u32(p.Dexterity) }},
		{"intelligence", u32(submitted.Intelligence), func(p core.PlayerGameData) string { return u32(p.Intelligence) }},
		{"faith", u32(submitted.Faith), func(p core.PlayerGameData) string { return u32(p.Faith) }},
		{"arcane", u32(submitted.Arcane), func(p core.PlayerGameData) string { return u32(p.Arcane) }},
		{"talisman_slots", u8(vm.NormalizeTalismanSlots(submitted.TalismanSlots)), func(p core.PlayerGameData) string { return u8(p.TalismanSlots) }},
		{"clear_count", u32(vm.NormalizeClearCount(submitted.ClearCount)), func(p core.PlayerGameData) string { return u32(p.ClearCount) }},
	}

	var plans []characterFieldPlan
	for _, s := range specs {
		before := s.read(cur)
		if before == s.planned {
			continue
		}
		plans = append(plans, characterFieldPlan{field: s.field, before: before, planned: s.planned, read: s.read})
	}
	return plans
}

// plannedChangeRecords maps plans to before/planned records. The before phase
// journal ignores After; the planned phase uses it — both share this list.
func plannedChangeRecords(plans []characterFieldPlan) []characterFieldChange {
	out := make([]characterFieldChange, len(plans))
	for i, p := range plans {
		out[i] = characterFieldChange{Field: p.field, Before: p.before, After: p.planned}
	}
	return out
}

// finishedChangeRecords maps plans to finished records, reading each field's
// actual value back out of the post-operation slot so the After column reflects
// what really landed rather than the requested value.
func finishedChangeRecords(plans []characterFieldPlan, post core.PlayerGameData) []characterFieldChange {
	out := make([]characterFieldChange, len(plans))
	for i, p := range plans {
		out[i] = characterFieldChange{Field: p.field, Before: p.before, After: p.read(post)}
	}
	return out
}
