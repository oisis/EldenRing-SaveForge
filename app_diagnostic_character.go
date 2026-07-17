package main

import (
	"strconv"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
	"github.com/oisis/EldenRing-SaveForge/backend/db"
	"github.com/oisis/EldenRing-SaveForge/backend/db/data"
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

// Memory Stones live outside the scalar profile fields: they are an inventory
// stack whose count is capped by the game and mirrored into world pickup event
// flags. SaveCharacter journals the three semantic values below, never raw slot
// bytes or flag buffers. The shared clamp/guard helpers (normalizeMemoryStones,
// memoryStonesFlagsAvailable, maxMemoryStones) live in app_appearance.go next to
// the writer that owns that behavior.
const (
	memoryStonesItemID = 0x4000272E // item ID (key into BolsteringPickupFlags)
	memoryStonesHandle = 0xB000272E // GaItemHandle of the Memory Stone stack in a parsed slot
	memoryStonesAbsent = "absent"   // memory_stones_common_quantity when no Common Items record holds the stone
)

// memoryStonesEffective mirrors character_vm.go: the count the Character VM
// shows is the Common Items stack quantity, falling back to Key Items only when
// no live Common Items stack is present.
func memoryStonesEffective(slot *core.SaveSlot) uint32 {
	var eff uint32
	for _, item := range slot.Inventory.CommonItems {
		if item.GaItemHandle == memoryStonesHandle {
			eff = item.Quantity & 0x7FFFFFFF
			break
		}
	}
	if eff == 0 {
		for _, item := range slot.Inventory.KeyItems {
			if item.GaItemHandle == memoryStonesHandle {
				eff = item.Quantity & 0x7FFFFFFF
				break
			}
		}
	}
	return eff
}

// readMemoryStonesEffective is the journal reader for the effective count.
func readMemoryStonesEffective(slot *core.SaveSlot) string {
	return strconv.FormatUint(uint64(memoryStonesEffective(slot)), 10)
}

// readMemoryStonesCommonQuantity reports the physical Common Items stack
// quantity, or "absent" when no Common Items record holds the stone.
func readMemoryStonesCommonQuantity(slot *core.SaveSlot) string {
	for _, item := range slot.Inventory.CommonItems {
		if item.GaItemHandle == memoryStonesHandle {
			return strconv.FormatUint(uint64(item.Quantity&0x7FFFFFFF), 10)
		}
	}
	return memoryStonesAbsent
}

// readMemoryStonesPickupFlagsSet reports how many Memory Stones pickup event
// flags are currently set — a semantic count only, never the raw flag bytes.
func readMemoryStonesPickupFlagsSet(slot *core.SaveSlot) string {
	count := 0
	if memoryStonesFlagsAvailable(slot) {
		flags := slot.Data[slot.EventFlagsOffset:]
		for _, f := range data.BolsteringPickupFlags[memoryStonesItemID] {
			if val, err := db.GetEventFlag(flags, f); err == nil && val {
				count++
			}
		}
	}
	return strconv.Itoa(count)
}

// memoryStonesFieldPlan is one in-scope Memory Stones field whose normalized
// intended value differs from the current slot value. read pulls the field's
// live value back out of the post-operation slot for the finished phase.
type memoryStonesFieldPlan struct {
	field   string
	before  string
	planned string
	read    func(*core.SaveSlot) string
}

// planMemoryStonesSaveChanges builds the ordered list of Memory Stones fields
// whose normalized target differs from the pre-mutation slot. planned values
// reuse the same clamp applyMemoryStonesToSlot applies, so a plan can never
// drift from what the writer persists. Only fields that actually change are
// returned, so a canonical, unchanged state emits nothing.
func planMemoryStonesSaveChanges(slot *core.SaveSlot, requested uint32) []memoryStonesFieldPlan {
	normalized := normalizeMemoryStones(requested)
	count := strconv.FormatUint(uint64(normalized), 10)

	// The writer creates or updates a Common Items stack only when it ends up
	// non-empty (desired > 0) or a stack already exists; otherwise the stone
	// stays absent from Common Items.
	commonExists := readMemoryStonesCommonQuantity(slot) != memoryStonesAbsent
	plannedCommon := memoryStonesAbsent
	if commonExists || normalized > 0 {
		plannedCommon = count
	}

	// Pickup flags are only mutable when the slot exposes a valid Event Flags
	// region; otherwise the writer leaves them untouched, so planning the
	// requested count would emit a phantom "planned" change whose finished value
	// never moves. When the region is unavailable, fall the planned value back to
	// the current readable count so the field self-excludes (before == planned).
	plannedPickup := readMemoryStonesPickupFlagsSet(slot)
	if memoryStonesFlagsAvailable(slot) {
		plannedPickup = count
	}

	specs := []struct {
		field   string
		planned string
		read    func(*core.SaveSlot) string
	}{
		{"memory_stones", count, readMemoryStonesEffective},
		{"memory_stones_common_quantity", plannedCommon, readMemoryStonesCommonQuantity},
		{"memory_stones_pickup_flags_set", plannedPickup, readMemoryStonesPickupFlagsSet},
	}

	var plans []memoryStonesFieldPlan
	for _, s := range specs {
		before := s.read(slot)
		if before == s.planned {
			continue
		}
		plans = append(plans, memoryStonesFieldPlan{field: s.field, before: before, planned: s.planned, read: s.read})
	}
	return plans
}

// memoryStonesPlannedRecords maps Memory Stones plans to before/planned records
// (the before phase ignores After, the planned phase uses it), mirroring
// plannedChangeRecords for the scalar plans.
func memoryStonesPlannedRecords(plans []memoryStonesFieldPlan) []characterFieldChange {
	out := make([]characterFieldChange, len(plans))
	for i, p := range plans {
		out[i] = characterFieldChange{Field: p.field, Before: p.before, After: p.planned}
	}
	return out
}

// memoryStonesFinishedRecords maps Memory Stones plans to finished records,
// reading each field's actual value back out of the post-operation slot.
func memoryStonesFinishedRecords(plans []memoryStonesFieldPlan, slot *core.SaveSlot) []characterFieldChange {
	out := make([]characterFieldChange, len(plans))
	for i, p := range plans {
		out[i] = characterFieldChange{Field: p.field, Before: p.before, After: p.read(slot)}
	}
	return out
}
