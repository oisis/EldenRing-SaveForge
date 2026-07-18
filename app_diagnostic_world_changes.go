package main

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"sort"
	"strconv"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
	"github.com/oisis/EldenRing-SaveForge/backend/db"
	"github.com/oisis/EldenRing-SaveForge/backend/db/data"
)

const (
	actionWorldSetGraceVisited           = "set_grace_visited"
	actionWorldSetBossDefeated           = "set_boss_defeated"
	actionWorldSetSummoningPoolActivated = "set_summoning_pool_activated"
	actionWorldSetColosseumUnlocked      = "set_colosseum_unlocked"
	actionWorldSetGestureUnlocked        = "set_gesture_unlocked"
	actionWorldBulkSetGesturesUnlocked   = "bulk_set_gestures_unlocked"
	actionWorldSetMapFlag                = "set_map_flag"
	actionWorldSetRegionUnlocked         = "set_region_unlocked"
	actionWorldBulkSetUnlockedRegions    = "bulk_set_unlocked_regions"
	actionWorldSetQuestStep              = "set_quest_step"
	actionWorldRevealAllMap              = "reveal_all_map"
	actionWorldResetMapExploration       = "reset_map_exploration"
	actionWorldRemoveFogOfWar            = "remove_fog_of_war"

	stageWorldApply = "apply_world"
)

// Lifecycle event names for World-tab field mutations. Their record contract is
// intentionally identical to Character and Game Items: action,
// character_index, field, before, after, outcome, and stage.
const (
	eventWorldChangeBefore   = "world_change_before"
	eventWorldChangePlanned  = "world_change_planned"
	eventWorldChangeFinished = "world_change_finished"
)

// worldFieldPlan is one semantic World value whose real post-mutation state can
// be read from a SaveSlot. World operations own Event Flags and the unlocked
// region membership list, rather than raw byte ranges.
type worldFieldPlan struct {
	field   string
	before  string
	planned string
	read    func(*core.SaveSlot) string
}

// worldMutationPlans keeps the stable World field order: Event Flags, map
// fragments, exploration blocks, gesture slots, then unlocked-region
// membership. Future World writers append only fields they own to these groups,
// while the lifecycle helpers preserve global phase grouping.
type worldMutationPlans struct {
	flags        []worldFieldPlan
	mapFragments []worldFieldPlan
	exploration  []worldFieldPlan
	gestures     []worldFieldPlan
	regions      []worldFieldPlan
}

func (p worldMutationPlans) records() []characterFieldChange {
	plans := append(append([]worldFieldPlan(nil), p.flags...), p.mapFragments...)
	plans = append(plans, p.exploration...)
	plans = append(plans, p.gestures...)
	plans = append(plans, p.regions...)
	records := make([]characterFieldChange, len(plans))
	for i, p := range plans {
		records[i] = characterFieldChange{Field: p.field, Before: p.before, After: p.planned}
	}
	return records
}

func (p worldMutationPlans) finished(slot *core.SaveSlot) []characterFieldChange {
	plans := append(append([]worldFieldPlan(nil), p.flags...), p.mapFragments...)
	plans = append(plans, p.exploration...)
	plans = append(plans, p.gestures...)
	plans = append(plans, p.regions...)
	records := make([]characterFieldChange, len(plans))
	for i, p := range plans {
		records[i] = characterFieldChange{Field: p.field, Before: p.before, After: p.read(slot)}
	}
	return records
}

// planWorldEventFlags projects only the declared flags that actually differ
// after applying a World writer. IDs are sorted and deduplicated so bulk input
// order never leaks into the journal; an unavailable Event Flags region reads
// false on both slots and therefore self-excludes.
func planWorldEventFlags(before, planned *core.SaveSlot, flagIDs []uint32) []worldFieldPlan {
	ids := append([]uint32(nil), flagIDs...)
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	plans := make([]worldFieldPlan, 0, len(ids))
	for i, flagID := range ids {
		if i > 0 && flagID == ids[i-1] {
			continue
		}
		b := readContainerFlag(before, flagID)
		p := readContainerFlag(planned, flagID)
		if b == p {
			continue
		}
		readFlagID := flagID
		plans = append(plans, worldFieldPlan{
			field:   fmt.Sprintf("event_flag_%d", flagID),
			before:  b,
			planned: p,
			read:    func(s *core.SaveSlot) string { return readContainerFlag(s, readFlagID) },
		})
	}
	return plans
}

func readWorldGestureSlot(slot *core.SaveSlot, index int) string {
	off := slot.StorageBoxOffset + core.DynStorageBox + index*4
	if index < 0 || off < 0 || off+4 > len(slot.Data) {
		return "unavailable"
	}
	return strconv.FormatUint(uint64(binary.LittleEndian.Uint32(slot.Data[off:off+4])), 10)
}

// planWorldGestureSlots reports every physical GestureGameData slot changed by
// the writer, including purged legacy entries. The raw u32 is deliberately
// scalar and stable: it distinguishes the empty sentinel from an empty zero.
func planWorldGestureSlots(before, planned *core.SaveSlot) []worldFieldPlan {
	plans := make([]worldFieldPlan, 0, 64)
	for index := 0; index < 64; index++ {
		b := readWorldGestureSlot(before, index)
		p := readWorldGestureSlot(planned, index)
		if b == p {
			continue
		}
		readIndex := index
		plans = append(plans, worldFieldPlan{
			field:   "gesture_slot_" + strconv.Itoa(index) + "_id",
			before:  b,
			planned: p,
			read:    func(s *core.SaveSlot) string { return readWorldGestureSlot(s, readIndex) },
		})
	}
	return plans
}

func readWorldMapFragmentOwned(slot *core.SaveSlot, itemID uint32) string {
	hasItem := func(items []core.InventoryItem) bool {
		for _, item := range items {
			if item.Quantity > 0 && db.HandleToItemID(item.GaItemHandle) == itemID {
				return true
			}
		}
		return false
	}
	if hasItem(slot.Inventory.CommonItems) || hasItem(slot.Inventory.KeyItems) || hasItem(slot.Storage.CommonItems) || hasItem(slot.Storage.KeyItems) {
		return "true"
	}
	return "false"
}

func mapFragmentFlagIDs() []uint32 {
	ids := make([]uint32, 0, len(data.MapFragmentItems))
	for flagID := range data.MapFragmentItems {
		ids = append(ids, flagID)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

// planWorldMapFragments records semantic ownership of each map fragment the
// action may add or remove. Physical rows may move during inventory writes;
// item identity is therefore the stable World-side diagnostic field.
func planWorldMapFragments(before, planned *core.SaveSlot, visibleFlagIDs []uint32) []worldFieldPlan {
	flagIDs := append([]uint32(nil), visibleFlagIDs...)
	sort.Slice(flagIDs, func(i, j int) bool { return flagIDs[i] < flagIDs[j] })
	plans := make([]worldFieldPlan, 0, len(flagIDs))
	for i, flagID := range flagIDs {
		if i > 0 && flagID == flagIDs[i-1] {
			continue
		}
		itemID, ok := data.MapFragmentItems[flagID]
		if !ok {
			continue
		}
		b := readWorldMapFragmentOwned(before, itemID)
		p := readWorldMapFragmentOwned(planned, itemID)
		if b == p {
			continue
		}
		readItemID := itemID
		plans = append(plans, worldFieldPlan{
			field:   fmt.Sprintf("map_fragment_0x%08X_owned", itemID),
			before:  b,
			planned: p,
			read:    func(s *core.SaveSlot) string { return readWorldMapFragmentOwned(s, readItemID) },
		})
	}
	return plans
}

func readWorldExplorationRange(slot *core.SaveSlot, start, end int) string {
	if start < 0 || end < start || end >= len(slot.Data)-0x80 {
		return "unavailable"
	}
	return hex.EncodeToString(slot.Data[start : end+1])
}

func readWorldFogOfWar(slot *core.SaveSlot) string {
	afterRegs, err := resolveAfterRegs(slot)
	if err != nil {
		return "unavailable"
	}
	return readWorldExplorationRange(slot, afterRegs+core.FoWBlobStart, afterRegs+core.FoWBlobEnd)
}

func readWorldDLCTiles(slot *core.SaveSlot) string {
	afterRegs, err := resolveAfterRegs(slot)
	if err != nil {
		return "unavailable"
	}
	return readWorldExplorationRange(slot, afterRegs+core.DLCTileZeroStart, afterRegs+core.DLCTileZeroEnd-1)
}

func planWorldExplorationField(before, planned *core.SaveSlot, field string, read func(*core.SaveSlot) string) []worldFieldPlan {
	b := read(before)
	p := read(planned)
	if b == p {
		return nil
	}
	return []worldFieldPlan{{field: field, before: b, planned: p, read: read}}
}

func readWorldUnlockedRegion(slot *core.SaveSlot, regionID uint32) string {
	for _, id := range slot.UnlockedRegions {
		if id == regionID {
			return "true"
		}
	}
	return "false"
}

// planWorldUnlockedRegions projects membership in the semantic unlocked-region
// list. Like Event Flags, it sorts and deduplicates candidates and omits
// unchanged entries. Callers may supply a single region or every target of a
// bulk operation; reset/rebuild callers can supply the union they own.
func planWorldUnlockedRegions(before, planned *core.SaveSlot, regionIDs []uint32) []worldFieldPlan {
	ids := append([]uint32(nil), regionIDs...)
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	plans := make([]worldFieldPlan, 0, len(ids))
	for i, regionID := range ids {
		if i > 0 && regionID == ids[i-1] {
			continue
		}
		b := readWorldUnlockedRegion(before, regionID)
		p := readWorldUnlockedRegion(planned, regionID)
		if b == p {
			continue
		}
		readRegionID := regionID
		plans = append(plans, worldFieldPlan{
			field:   "unlocked_region_" + strconv.FormatUint(uint64(regionID), 10),
			before:  b,
			planned: p,
			read:    func(s *core.SaveSlot) string { return readWorldUnlockedRegion(s, readRegionID) },
		})
	}
	return plans
}

func (a *App) journalWorldMutationBefore(action string, charIdx int, plans worldMutationPlans) {
	records := plans.records()
	a.journalChangeRecords(eventWorldChangeBefore, "world change before", action, charIdx, records, nil)
	a.journalChangeRecords(eventWorldChangePlanned, "world change planned", action, charIdx, records, changePlannedTail)
}

func (a *App) journalWorldMutationFinished(action string, charIdx int, outcome characterChangeOutcome, stage string, plans worldMutationPlans, slot *core.SaveSlot) {
	a.journalChangeRecords(eventWorldChangeFinished, "world change finished", action, charIdx, plans.finished(slot), changeFinishedTail(outcome, stage))
}

// journalWorldSlotMutation applies one World writer to an independent clone for
// its planned projection and exactly once to the real slot. The writer itself is
// shared, so the journal cannot model behavior that differs from the mutation.
// Callers hold the slot lock, have already taken their undo snapshot, and have
// completed operation-specific bounds validation.
func (a *App) journalWorldSlotMutation(action string, charIdx int, slot *core.SaveSlot, apply func(*core.SaveSlot) error, plan func(before, planned *core.SaveSlot) worldMutationPlans) error {
	if !a.journal.debugEnabled() {
		return apply(slot)
	}

	clone := core.CloneSlot(slot)
	_ = apply(clone)
	plans := plan(slot, clone)
	a.journalWorldMutationBefore(action, charIdx, plans)

	if err := apply(slot); err != nil {
		a.journalWorldMutationFinished(action, charIdx, characterChangeError, stageWorldApply, plans, slot)
		return err
	}
	a.journalWorldMutationFinished(action, charIdx, characterChangeSuccess, characterStageCompleted, plans, slot)
	return nil
}

func worldGraceFlagIDs(graceID uint32, visited bool) []uint32 {
	flagIDs := []uint32{graceID}
	if grace, ok := data.Graces[graceID]; ok && grace.DoorFlag != 0 {
		flagIDs = append(flagIDs, grace.DoorFlag)
	}
	if visited {
		flagIDs = append(flagIDs, data.CompanionEventFlagsForGrace(graceID)...)
	}
	return flagIDs
}

func worldColosseumFlagIDs(colosseumID uint32, unlocked bool) []uint32 {
	flagSet, ok := data.ColosseumFlagSets[colosseumID]
	if !ok {
		flagSet = data.ColosseumFlagSet{Activate: colosseumID}
	}
	flagIDs := flagSet.AllFlags()
	if unlocked {
		flagIDs = append(flagIDs, data.ColosseumGlobalFlags...)
	}
	return flagIDs
}

func worldKnownRegionIDs() []uint32 {
	entries := db.GetAllRegions()
	ids := make([]uint32, len(entries))
	for i, entry := range entries {
		ids[i] = entry.ID
	}
	return ids
}
