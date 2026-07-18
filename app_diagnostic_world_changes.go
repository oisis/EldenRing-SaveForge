package main

import (
	"fmt"
	"sort"
	"strconv"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
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

// worldMutationPlans keeps the stable World field order: Event Flags before
// unlocked-region membership. Future World writers append only fields they own
// to these groups, while the lifecycle helpers preserve global phase grouping.
type worldMutationPlans struct {
	flags   []worldFieldPlan
	regions []worldFieldPlan
}

func (p worldMutationPlans) records() []characterFieldChange {
	plans := append(append([]worldFieldPlan(nil), p.flags...), p.regions...)
	records := make([]characterFieldChange, len(plans))
	for i, p := range plans {
		records[i] = characterFieldChange{Field: p.field, Before: p.before, After: p.planned}
	}
	return records
}

func (p worldMutationPlans) finished(slot *core.SaveSlot) []characterFieldChange {
	plans := append(append([]worldFieldPlan(nil), p.flags...), p.regions...)
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
