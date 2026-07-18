package main

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strconv"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
	"github.com/oisis/EldenRing-SaveForge/backend/db"
	"github.com/oisis/EldenRing-SaveForge/backend/db/data"
	"github.com/oisis/EldenRing-SaveForge/backend/vm"
)

// actionSaveCharacter is the action tag every SaveCharacter-originated Character
// diagnostic record carries.
const actionSaveCharacter = "save_character"

// actionApplyAppearancePreset is the action tag every ApplyPresetToCharacter
// Appearance diagnostic record carries. It reuses the character_change_*
// lifecycle events (before -> planned -> finished) so a diagnostics reader
// treats an appearance write like any other Character mutation.
const actionApplyAppearancePreset = "apply_appearance_preset"

// actionSetCharacterGender is the action tag every SetCharacterGender Appearance
// diagnostic record carries. It reuses the same character_change_* lifecycle and
// planApplyAppearance planner as actionApplyAppearancePreset — the only
// difference is the action label, since a gender switch is an appearance write of
// the target gender's default preset.
const actionSetCharacterGender = "set_character_gender"

// actionApplyMirrorFavorite is the action tag every ApplyMirrorFavoriteToCharacter
// Appearance diagnostic record carries. It reuses the character_change_* lifecycle
// and appearanceFieldPlan mappers, but plans from the raw Mirror Favorites slot
// bytes via planApplyMirrorFavorite instead of a resolvedAppearance — the Mirror
// writer copies raw model/hex data verbatim and never touches voice_type.
const actionApplyMirrorFavorite = "apply_mirror_favorite"

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
// what the writer persists. NG+ flags and ProfileSummary are handled by the
// dedicated side-effect planner below; the remaining operational side effects
// (Memory Stones inventory, containers, quantities, appearance, favorites) are
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
// hasEventFlagsRegion, maxMemoryStones) live in app_appearance.go next to
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
	if hasEventFlagsRegion(slot) {
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
	if hasEventFlagsRegion(slot) {
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

// sideEffectPlan is one Clear Count side effect (an NG+ event flag or a
// ProfileSummary field) whose target differs from the current value. Unlike the
// scalar/Memory Stones plans, the finished value can live in the slot's flag
// buffer or the save's ProfileSummary, so read is a self-contained closure that
// pulls the field's live post-operation value from wherever it belongs.
type sideEffectPlan struct {
	field   string
	before  string
	planned string
	read    func() string
}

// finalCharacterName reports the exact truncated name SaveCharacter will persist
// for a ProfileSummary plan, without mutating the slot first. It reuses the
// writer's own vm.NormalizeCharacterName so the plan can never drift from what
// ApplyVMToParsedSlot writes, then decodes the fixed-width UTF-16 buffer back to
// a string the same way the summary reader does.
func finalCharacterName(name string) string {
	buf := vm.NormalizeCharacterName(name)
	return core.UTF16ToString(buf[:])
}

// readNGPlusFlag reports NG+ event flag flagID as the scalar "true"/"false",
// reading live from the slot's Event Flags region. It uses the same
// availability guard as the writer and, like the existing flag readers, treats
// an unavailable region or a read error as an unset flag.
func (a *App) readNGPlusFlag(index int, flagID uint32) string {
	slot := &a.save.Slots[index]
	if hasEventFlagsRegion(slot) {
		if val, err := db.GetEventFlag(slot.Data[slot.EventFlagsOffset:], flagID); err == nil && val {
			return "true"
		}
	}
	return "false"
}

// planClearCountSideEffects builds the plans for SaveCharacter's Clear Count
// side effects: NG+ event flags 50-57 and the ProfileSummary level/name mirror.
// Everything is planned against the pre-mutation state; each read closure pulls
// the field's live value back after the operation so the finished phase reports
// what actually landed (or, on failure, the unchanged pre-error value). Only
// fields whose target differs from the current value are returned.
func (a *App) planClearCountSideEffects(index int, submitted *vm.CharacterViewModel) []sideEffectPlan {
	slot := &a.save.Slots[index]
	var plans []sideEffectPlan

	// NG+ flags: exactly the flag matching the normalized clear count is true.
	// Mutable only when the slot exposes a valid Event Flags region — the same
	// guard the writer uses; otherwise emit nothing.
	if hasEventFlagsRegion(slot) {
		normalized := vm.NormalizeClearCount(submitted.ClearCount)
		for i := uint32(0); i <= 7; i++ {
			flagID := 50 + i
			target := strconv.FormatBool(i == normalized)
			before := a.readNGPlusFlag(index, flagID)
			if before == target {
				continue
			}
			plans = append(plans, sideEffectPlan{
				field:   "ng_plus_flag_" + strconv.FormatUint(uint64(flagID), 10),
				before:  before,
				planned: target,
				read:    func() string { return a.readNGPlusFlag(index, flagID) },
			})
		}
	}

	// ProfileSummary mirrors the final character level and the final persisted
	// (truncated) name SaveCharacter assigns.
	ps := &a.save.ProfileSummaries[index]
	if levelBefore, levelPlanned := strconv.FormatUint(uint64(ps.Level), 10), strconv.FormatUint(uint64(submitted.Level), 10); levelBefore != levelPlanned {
		plans = append(plans, sideEffectPlan{
			field:   "profile_summary_level",
			before:  levelBefore,
			planned: levelPlanned,
			read:    func() string { return strconv.FormatUint(uint64(a.save.ProfileSummaries[index].Level), 10) },
		})
	}
	if nameBefore, namePlanned := core.UTF16ToString(ps.CharacterName[:]), finalCharacterName(submitted.Name); nameBefore != namePlanned {
		plans = append(plans, sideEffectPlan{
			field:   "profile_summary_name",
			before:  nameBefore,
			planned: namePlanned,
			read:    func() string { return core.UTF16ToString(a.save.ProfileSummaries[index].CharacterName[:]) },
		})
	}
	return plans
}

// sideEffectPlannedRecords maps side-effect plans to before/planned records (the
// before phase ignores After, the planned phase uses it), mirroring the scalar
// and Memory Stones record mappers.
func sideEffectPlannedRecords(plans []sideEffectPlan) []characterFieldChange {
	out := make([]characterFieldChange, len(plans))
	for i, p := range plans {
		out[i] = characterFieldChange{Field: p.field, Before: p.before, After: p.planned}
	}
	return out
}

// sideEffectFinishedRecords maps side-effect plans to finished records, invoking
// each plan's read closure to capture the field's actual post-operation value.
func sideEffectFinishedRecords(plans []sideEffectPlan) []characterFieldChange {
	out := make([]characterFieldChange, len(plans))
	for i, p := range plans {
		out[i] = characterFieldChange{Field: p.field, Before: p.before, After: p.read()}
	}
	return out
}

// itemQuantityPlan is one physical Inventory/Storage item row whose normalized
// submitted quantity differs from its current physical Quantity. read pulls the
// same row's live physical Quantity back out of the post-operation slot so the
// finished phase reports what actually landed (or, on error, the unchanged
// pre-error value). The field name embeds the physical row and handle so two
// rows sharing a handle produce distinct, non-colliding records.
type itemQuantityPlan struct {
	field   string
	before  string
	planned string
	read    func(*core.SaveSlot) string
}

// itemVMByHandle indexes submitted items by Handle exactly as updateItemsAndSync
// does (last write wins on a duplicate handle), so the planner resolves the same
// ItemViewModel the writer will apply to each physical row.
func itemVMByHandle(items []vm.ItemViewModel) map[uint32]vm.ItemViewModel {
	m := make(map[uint32]vm.ItemViewModel, len(items))
	for _, it := range items {
		m[it.Handle] = it
	}
	return m
}

// planItemSection builds the quantity plans for one physical section (Inventory
// Common/Key or Storage Common). It mirrors the writer's scope: only live rows
// mapped by a submitted item, skipping empty/invalid handles and the Memory
// Stone stack (which has its own memory_stones* logging), and only rows whose
// normalized planned quantity differs from the current physical Quantity. read
// re-selects the same section+row from the post-operation slot.
func planItemSection(records []core.InventoryItem, submitted map[uint32]vm.ItemViewModel, isStorage bool, prefix string, section func(*core.SaveSlot) []core.InventoryItem) []itemQuantityPlan {
	var plans []itemQuantityPlan
	for row := range records {
		rec := records[row]
		handle := rec.GaItemHandle
		if handle == core.GaHandleEmpty || handle == core.GaHandleInvalid || handle == memoryStonesHandle {
			continue
		}
		vmItem, ok := submitted[handle]
		if !ok {
			continue
		}
		planned := vm.NormalizeItemQuantity(vmItem, isStorage)
		if planned == rec.Quantity {
			continue
		}
		row := row
		plans = append(plans, itemQuantityPlan{
			field:   fmt.Sprintf("%s_row_%d_handle_0x%08X_quantity", prefix, row, handle),
			before:  strconv.FormatUint(uint64(rec.Quantity), 10),
			planned: strconv.FormatUint(uint64(planned), 10),
			read: func(s *core.SaveSlot) string {
				sec := section(s)
				if row < len(sec) {
					return strconv.FormatUint(uint64(sec[row].Quantity), 10)
				}
				return strconv.FormatUint(uint64(rec.Quantity), 10)
			},
		})
	}
	return plans
}

// planItemQuantityChanges builds the ordered quantity plans for the three
// physical sections ApplyVMToParsedSlot writes directly — Inventory Common,
// Inventory Key and Storage Common — against the pre-mutation slot. Container
// auto-sync and Event Flags are deliberately out of scope here.
func planItemQuantityChanges(slot *core.SaveSlot, submitted *vm.CharacterViewModel) []itemQuantityPlan {
	invMap := itemVMByHandle(submitted.Inventory)
	storMap := itemVMByHandle(submitted.Storage)

	var plans []itemQuantityPlan
	plans = append(plans, planItemSection(slot.Inventory.CommonItems, invMap, false, "inventory_common",
		func(s *core.SaveSlot) []core.InventoryItem { return s.Inventory.CommonItems })...)
	plans = append(plans, planItemSection(slot.Inventory.KeyItems, invMap, false, "inventory_key",
		func(s *core.SaveSlot) []core.InventoryItem { return s.Inventory.KeyItems })...)
	plans = append(plans, planItemSection(slot.Storage.CommonItems, storMap, true, "storage_common",
		func(s *core.SaveSlot) []core.InventoryItem { return s.Storage.CommonItems })...)
	return plans
}

// itemQuantityPlannedRecords maps quantity plans to before/planned records (the
// before phase ignores After, the planned phase uses it), mirroring the scalar,
// Memory Stones and side-effect record mappers.
func itemQuantityPlannedRecords(plans []itemQuantityPlan) []characterFieldChange {
	out := make([]characterFieldChange, len(plans))
	for i, p := range plans {
		out[i] = characterFieldChange{Field: p.field, Before: p.before, After: p.planned}
	}
	return out
}

// itemQuantityFinishedRecords maps quantity plans to finished records, reading
// each row's actual physical Quantity back out of the post-operation slot.
func itemQuantityFinishedRecords(plans []itemQuantityPlan, slot *core.SaveSlot) []characterFieldChange {
	out := make([]characterFieldChange, len(plans))
	for i, p := range plans {
		out[i] = characterFieldChange{Field: p.field, Before: p.before, After: p.read(slot)}
	}
	return out
}

// Container auto-sync (Cracked/Ritual/Hefty Pot, Perfume Bottle) is a
// SaveCharacter side effect driven off the edited Inventory/Storage quantities,
// not a field the caller submits. The three semantic values logged per touched
// container are the container quantity and each pickup/vendor event flag the
// writer sets — never raw slot bytes or flag buffers.
const containerAbsent = "absent" // container_*_quantity when no inventory record holds the container

// containerItemHandle maps a canonical container item ID to its inventory goods
// handle exactly as inventoryContainerQuantity / upsertInventoryContainerQuantity do.
func containerItemHandle(itemID uint32) uint32 {
	return db.ItemIDToHandlePrefix(itemID) | (itemID & 0x0FFFFFFF)
}

// readContainerQuantity reports the container's physical quantity, or "absent"
// when no Inventory record (canonical CommonItems or legacy KeyItems) holds it.
// It mirrors inventoryContainerQuantity's max-across-records reading but keeps
// the absent case distinct from a real zero.
func readContainerQuantity(slot *core.SaveSlot, itemID uint32) string {
	handle := containerItemHandle(itemID)
	found := false
	qty := uint32(0)
	for _, it := range slot.Inventory.CommonItems {
		if it.GaItemHandle == handle {
			found = true
			if q := it.Quantity & 0x7FFFFFFF; q > qty {
				qty = q
			}
		}
	}
	for _, it := range slot.Inventory.KeyItems {
		if it.GaItemHandle == handle {
			found = true
			if q := it.Quantity & 0x7FFFFFFF; q > qty {
				qty = q
			}
		}
	}
	if !found {
		return containerAbsent
	}
	return strconv.FormatUint(uint64(qty), 10)
}

// readContainerFlag reports event flag flagID as the scalar "true"/"false",
// reading live from the slot's Event Flags region. Like the other flag readers,
// an unavailable region or a read error is treated as an unset flag.
func readContainerFlag(slot *core.SaveSlot, flagID uint32) string {
	if hasEventFlagsRegion(slot) {
		if val, err := db.GetEventFlag(slot.Data[slot.EventFlagsOffset:], flagID); err == nil && val {
			return "true"
		}
	}
	return "false"
}

// containerFieldPlan is one container-sync side effect (a container quantity or
// one pickup/vendor flag) whose target differs from the current value. read
// pulls the field's live value back out of the post-operation real slot so the
// finished phase reports what actually landed (or, on rollback, the restored
// pre-error value).
type containerFieldPlan struct {
	field   string
	before  string
	planned string
	read    func(*core.SaveSlot) string
}

// planContainerSideEffects predicts the container auto-sync SaveCharacter will
// perform. It projects the submitted VM onto a deep copy of the slot via the
// writer's own ApplyVMToParsedSlot, then feeds that projection to the shared
// planContainerSync, so the prediction sees exactly the post-edit inventory the
// real writer will reconcile — without touching the real slot, charVM, undo, or
// the write order. If the projection fails ApplyVMToParsedSlot the real save
// aborts at apply_vm before container sync runs, so no container plans are
// emitted. before/finished values are read from the REAL slot; only the set of
// planned actions comes from the projection. A flag whose target already matches
// its current state self-excludes, and no flag records are emitted when the slot
// has no Event Flags region (planContainerSync leaves the flag lists empty).
func (a *App) planContainerSideEffects(index int, submitted *vm.CharacterViewModel) []containerFieldPlan {
	real := &a.save.Slots[index]

	clone := core.CloneSlot(real)
	proj := *submitted // value copy: ApplyVMToParsedSlot writes back normalized scalars
	if err := vm.ApplyVMToParsedSlot(&proj, clone); err != nil {
		return nil
	}

	var plans []containerFieldPlan
	for _, act := range planContainerSync(clone) {
		itemID := act.itemID
		// The quantity record is logged only when the real container quantity
		// actually differs from the synced final value. A VM that lowers a
		// container which the sync then raises straight back to its starting
		// quantity nets zero semantic change (before == planned == finished), so
		// it self-excludes here; the direct physical row still logs the writer's
		// full 3 -> 1 -> 3 attempt via the Task 4C.1 quantity plan.
		if before, planned := readContainerQuantity(real, itemID), strconv.Itoa(act.finalQty); before != planned {
			plans = append(plans, containerFieldPlan{
				field:   fmt.Sprintf("container_0x%08X_quantity", itemID),
				before:  before,
				planned: planned,
				read:    func(s *core.SaveSlot) string { return readContainerQuantity(s, itemID) },
			})
		}
		for _, f := range act.pickupFlags {
			plans = appendContainerFlagPlan(plans, real, itemID, f, "pickup")
		}
		for _, f := range act.vendorFlags {
			plans = appendContainerFlagPlan(plans, real, itemID, f, "vendor")
		}
	}
	return plans
}

// appendContainerFlagPlan appends a container flag plan when the writer will
// flip the flag (planned "true" differs from the current state); a flag already
// set self-excludes, so only genuine changes are logged.
func appendContainerFlagPlan(plans []containerFieldPlan, real *core.SaveSlot, itemID, flagID uint32, kind string) []containerFieldPlan {
	before := readContainerFlag(real, flagID)
	if before == "true" {
		return plans
	}
	return append(plans, containerFieldPlan{
		field:   fmt.Sprintf("container_0x%08X_%s_flag_%d", itemID, kind, flagID),
		before:  before,
		planned: "true",
		read:    func(s *core.SaveSlot) string { return readContainerFlag(s, flagID) },
	})
}

// containerPlannedRecords maps container plans to before/planned records (the
// before phase ignores After, the planned phase uses it), mirroring the scalar,
// Memory Stones, side-effect and quantity record mappers.
func containerPlannedRecords(plans []containerFieldPlan) []characterFieldChange {
	out := make([]characterFieldChange, len(plans))
	for i, p := range plans {
		out[i] = characterFieldChange{Field: p.field, Before: p.before, After: p.planned}
	}
	return out
}

// containerFinishedRecords maps container plans to finished records, reading each
// field's actual value back out of the post-operation real slot.
func containerFinishedRecords(plans []containerFieldPlan, slot *core.SaveSlot) []characterFieldChange {
	out := make([]characterFieldChange, len(plans))
	for i, p := range plans {
		out[i] = characterFieldChange{Field: p.field, Before: p.before, After: p.read(slot)}
	}
	return out
}

// appearanceFieldPlan is one Appearance value ApplyPresetToCharacter changes: an
// eight-model raw PartsId, one of the three fixed-width FaceData hex blocks, the
// two sex-flag bytes applyResolvedAppearance zeroes, or the gender/voice scalars.
// before/planned are captured from the pre-write slot against the resolved
// payload; read pulls the field's live value back out of the FaceData blob (fd)
// so the finished phase reports what applyResolvedAppearance actually wrote.
type appearanceFieldPlan struct {
	field   string
	before  string
	planned string
	read    func(slot *core.SaveSlot, fd int) string
}

// readAppearanceModel reads a model PartsId back as decimal from the FaceData
// blob, matching applyResolvedAppearance's uint32-LE write at fd+offset.
func readAppearanceModel(slot *core.SaveSlot, fd, offset int) string {
	return strconv.FormatUint(uint64(binary.LittleEndian.Uint32(slot.Data[fd+offset:])), 10)
}

// planApplyAppearance builds the ordered list of Appearance fields whose resolved
// target differs from the current slot value. Every value is a scalar decimal or
// a lowercase, unprefixed hex string — never a raw []byte, map, or interface — so
// only privacy-safe technical values reach the journal. planned values come from
// the same resolvedAppearance applyResolvedAppearance writes, so a plan can never
// drift from what the writer persists; the two sex-flag bytes are always zeroed
// by the writer, so their planned value is a fixed "0000". Only fields that
// actually change are returned, so re-applying an identical preset emits nothing.
func planApplyAppearance(slot *core.SaveSlot, fd int, r resolvedAppearance) []appearanceFieldPlan {
	dec := func(v uint32) string { return strconv.FormatUint(uint64(v), 10) }

	var plans []appearanceFieldPlan

	modelSpecs := []struct {
		field  string
		offset int
		id     uint8
	}{
		{"appearance_face_model", core.FDOffFaceModel, r.Models[0]},
		{"appearance_hair_model", core.FDOffHairModel, r.Models[1]},
		{"appearance_eye_model", core.FDOffEyeModel, r.Models[2]},
		{"appearance_eyebrow_model", core.FDOffEyebrowModel, r.Models[3]},
		{"appearance_beard_model", core.FDOffBeardModel, r.Models[4]},
		{"appearance_eyepatch_model", core.FDOffEyepatchModel, r.Models[5]},
		{"appearance_decal_model", core.FDOffDecalModel, r.Models[6]},
		{"appearance_eyelash_model", core.FDOffEyelashModel, r.Models[7]},
	}
	for _, s := range modelSpecs {
		offset := s.offset
		before := readAppearanceModel(slot, fd, offset)
		planned := dec(uint32(s.id))
		if before == planned {
			continue
		}
		plans = append(plans, appearanceFieldPlan{
			field:   s.field,
			before:  before,
			planned: planned,
			read:    func(sl *core.SaveSlot, f int) string { return readAppearanceModel(sl, f, offset) },
		})
	}

	hexSpecs := []struct {
		field   string
		offset  int
		length  int
		planned string
	}{
		{"appearance_face_shape_hex", core.FDOffFaceShape, 64, hex.EncodeToString(r.FaceShape[:])},
		{"appearance_body_proportions_hex", core.FDOffHead, 7, hex.EncodeToString(r.Body[:])},
		{"appearance_skin_cosmetics_hex", core.FDOffSkinR, 91, hex.EncodeToString(r.Skin[:])},
	}
	for _, s := range hexSpecs {
		offset, length := s.offset, s.length
		before := hex.EncodeToString(slot.Data[fd+offset : fd+offset+length])
		if before == s.planned {
			continue
		}
		plans = append(plans, appearanceFieldPlan{
			field:   s.field,
			before:  before,
			planned: s.planned,
			read:    func(sl *core.SaveSlot, f int) string { return hex.EncodeToString(sl.Data[f+offset : f+offset+length]) },
		})
	}

	// Sex-flag bytes at fd+0x125..fd+0x126: applyResolvedAppearance always zeroes
	// them, so the planned value is fixed and the field self-excludes only when the
	// slot already holds two zero bytes.
	if before := hex.EncodeToString(slot.Data[fd+0x125 : fd+0x127]); before != "0000" {
		plans = append(plans, appearanceFieldPlan{
			field:   "appearance_apply_flags_hex",
			before:  before,
			planned: "0000",
			read:    func(sl *core.SaveSlot, f int) string { return hex.EncodeToString(sl.Data[f+0x125 : f+0x127]) },
		})
	}

	if before, planned := dec(uint32(slot.Player.Gender)), dec(uint32(r.BodyType)); before != planned {
		plans = append(plans, appearanceFieldPlan{
			field:   "gender",
			before:  before,
			planned: planned,
			read:    func(sl *core.SaveSlot, _ int) string { return dec(uint32(sl.Player.Gender)) },
		})
	}
	if before, planned := dec(uint32(slot.Player.VoiceType)), dec(uint32(r.VoiceType)); before != planned {
		plans = append(plans, appearanceFieldPlan{
			field:   "voice_type",
			before:  before,
			planned: planned,
			read:    func(sl *core.SaveSlot, _ int) string { return dec(uint32(sl.Player.VoiceType)) },
		})
	}

	return plans
}

// planApplyMirrorFavorite builds the ordered list of Appearance fields
// ApplyMirrorFavoriteToCharacter changes, planned from the raw Mirror Favorites
// slot bytes (mirror = the FavSlotSize-byte slot) rather than a resolvedAppearance.
// It reuses appearanceFieldPlan, readAppearanceModel and the shared before/planned/
// finished mappers, so it only supplies the planned source. Unlike planApplyAppearance
// the eight models are raw uint32 LE straight from the Mirror slot (not clamped to
// uint8), the three fixed-width hex blocks are copied verbatim, the writer always
// zeroes the two apply-flag bytes, gender comes from the Mirror body-type inversion
// (FavOffBodyType==0 -> Gender 1, else Gender 0), and voice_type is never planned
// because the writer leaves it untouched. Only fields whose target differs from the
// current slot value are returned, so re-applying an identical Mirror slot emits nothing.
func planApplyMirrorFavorite(slot *core.SaveSlot, fd int, mirror []byte) []appearanceFieldPlan {
	var plans []appearanceFieldPlan

	modelSpecs := []struct {
		field  string
		offset int
	}{
		{"appearance_face_model", core.FDOffFaceModel},
		{"appearance_hair_model", core.FDOffHairModel},
		{"appearance_eye_model", core.FDOffEyeModel},
		{"appearance_eyebrow_model", core.FDOffEyebrowModel},
		{"appearance_beard_model", core.FDOffBeardModel},
		{"appearance_eyepatch_model", core.FDOffEyepatchModel},
		{"appearance_decal_model", core.FDOffDecalModel},
		{"appearance_eyelash_model", core.FDOffEyelashModel},
	}
	for i, s := range modelSpecs {
		offset := s.offset
		before := readAppearanceModel(slot, fd, offset)
		planned := strconv.FormatUint(uint64(binary.LittleEndian.Uint32(mirror[core.FavOffModelIDs+i*4:])), 10)
		if before == planned {
			continue
		}
		plans = append(plans, appearanceFieldPlan{
			field:   s.field,
			before:  before,
			planned: planned,
			read:    func(sl *core.SaveSlot, f int) string { return readAppearanceModel(sl, f, offset) },
		})
	}

	hexSpecs := []struct {
		field  string
		offset int
		src    int
		length int
	}{
		{"appearance_face_shape_hex", core.FDOffFaceShape, core.FavOffFaceShape, 64},
		{"appearance_body_proportions_hex", core.FDOffHead, core.FavOffBody, 7},
		{"appearance_skin_cosmetics_hex", core.FDOffSkinR, core.FavOffSkin, 91},
	}
	for _, s := range hexSpecs {
		offset, length := s.offset, s.length
		before := hex.EncodeToString(slot.Data[fd+offset : fd+offset+length])
		planned := hex.EncodeToString(mirror[s.src : s.src+length])
		if before == planned {
			continue
		}
		plans = append(plans, appearanceFieldPlan{
			field:   s.field,
			before:  before,
			planned: planned,
			read:    func(sl *core.SaveSlot, f int) string { return hex.EncodeToString(sl.Data[f+offset : f+offset+length]) },
		})
	}

	// Apply-flag bytes at fd+0x125..fd+0x126: the writer always zeroes them, so the
	// planned value is fixed and the field self-excludes only when both are already zero.
	if before := hex.EncodeToString(slot.Data[fd+0x125 : fd+0x127]); before != "0000" {
		plans = append(plans, appearanceFieldPlan{
			field:   "appearance_apply_flags_hex",
			before:  before,
			planned: "0000",
			read:    func(sl *core.SaveSlot, f int) string { return hex.EncodeToString(sl.Data[f+0x125 : f+0x127]) },
		})
	}

	// Gender from the Mirror body-type inversion (mirrors the writer's own logic).
	planned := "0"
	if mirror[core.FavOffBodyType] == 0 {
		planned = "1"
	}
	if before := strconv.FormatUint(uint64(slot.Player.Gender), 10); before != planned {
		plans = append(plans, appearanceFieldPlan{
			field:   "gender",
			before:  before,
			planned: planned,
			read:    func(sl *core.SaveSlot, _ int) string { return strconv.FormatUint(uint64(sl.Player.Gender), 10) },
		})
	}

	return plans
}

// appearancePlannedRecords maps Appearance plans to before/planned records (the
// before phase ignores After, the planned phase uses it), mirroring the scalar,
// Memory Stones, side-effect, quantity and container record mappers.
func appearancePlannedRecords(plans []appearanceFieldPlan) []characterFieldChange {
	out := make([]characterFieldChange, len(plans))
	for i, p := range plans {
		out[i] = characterFieldChange{Field: p.field, Before: p.before, After: p.planned}
	}
	return out
}

// appearanceFinishedRecords maps Appearance plans to finished records, reading
// each field's actual value back out of the post-write FaceData blob (fd).
func appearanceFinishedRecords(plans []appearanceFieldPlan, slot *core.SaveSlot, fd int) []characterFieldChange {
	out := make([]characterFieldChange, len(plans))
	for i, p := range plans {
		out[i] = characterFieldChange{Field: p.field, Before: p.before, After: p.read(slot, fd)}
	}
	return out
}
