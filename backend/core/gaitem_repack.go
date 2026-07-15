package core

import "fmt"

// GaItemCapacity describes the usable GaItem allocation capacity at one point
// in time. PhysicalEmpty is the number of empty records in the table, while
// CursorRoom is what the current allocator cursor can reach. Usable is always
// the smaller of those two values.
type GaItemCapacity struct {
	PhysicalEmpty int
	CursorRoom    int
	Usable        int
}

// GaItemRepackAnalysis is a read-only forecast of stable GaItem compaction.
// It deliberately makes no safety decision: validation/refusal belongs to the
// repack pre-flight. The caller can use this report to decide whether there is
// capacity to recover before any mutation is attempted.
type GaItemRepackAnalysis struct {
	Before          GaItemCapacity
	ProjectedAfter  GaItemCapacity
	Recovered       int
	NonEmptyRecords int
}

// GaItemRepackPlan is the deterministic in-memory layout produced by stable
// compaction. It is valid only for the exact, already preflighted slot from
// which it was built; the transaction layer is responsible for checking
// freshness before applying it to a candidate slot.
type GaItemRepackPlan struct {
	GaItems         []GaItemFull
	NonEmptyRecords int
	Changes         bool
}

// AnalyzeGaItemRepack calculates the capacity effect of the canonical stable
// compaction without modifying slot. Non-empty records preserve their physical
// order; after compaction scanGaItems would place the max-counter record at its
// compacted position, which determines the projected allocator cursor.
func AnalyzeGaItemRepack(slot *SaveSlot) GaItemRepackAnalysis {
	if slot == nil {
		return GaItemRepackAnalysis{}
	}

	n := len(slot.GaItems)
	nonEmpty := 0
	maxCounter := uint32(0)
	projectedMaxCounterIndex := 0

	for _, record := range slot.GaItems {
		if record.IsEmpty() {
			continue
		}
		if counter := record.Handle & 0xFFFF; counter >= maxCounter {
			maxCounter = counter
			projectedMaxCounterIndex = nonEmpty
		}
		nonEmpty++
	}

	projectedNextArmamentIndex := projectedMaxCounterIndex + 1
	before := gaItemCapacity(n, nonEmpty, slot.NextArmamentIndex)
	projectedAfter := gaItemCapacity(n, nonEmpty, projectedNextArmamentIndex)
	return GaItemRepackAnalysis{
		Before:          before,
		ProjectedAfter:  projectedAfter,
		Recovered:       max(0, projectedAfter.Usable-before.Usable),
		NonEmptyRecords: nonEmpty,
	}
}

// BuildGaItemRepackPlan creates the canonical stable-compaction layout without
// modifying slot. It preserves every field and the relative order of non-empty
// records, then leaves a zero-value empty suffix of the original table length.
// Callers must run PreflightGaItemRepack before using the plan.
func BuildGaItemRepackPlan(slot *SaveSlot) GaItemRepackPlan {
	if slot == nil {
		return GaItemRepackPlan{}
	}

	planned := make([]GaItemFull, len(slot.GaItems))
	nonEmpty := 0
	for _, record := range slot.GaItems {
		if record.IsEmpty() {
			continue
		}
		planned[nonEmpty] = record
		nonEmpty++
	}

	changed := false
	for i := range planned {
		if planned[i] != slot.GaItems[i] {
			changed = true
			break
		}
	}
	return GaItemRepackPlan{
		GaItems:         planned,
		NonEmptyRecords: nonEmpty,
		Changes:         changed,
	}
}

// ApplyGaItemRepackPlan replaces only slot.GaItems with a private copy of the
// planned stable layout. It intentionally does not rebuild bytes, reparse,
// update cursors, or touch GaMap; those atomic transaction steps belong to the
// caller. A no-op plan does not replace the existing slice.
func ApplyGaItemRepackPlan(slot *SaveSlot, plan GaItemRepackPlan) error {
	if slot == nil {
		return fmt.Errorf("ApplyGaItemRepackPlan: nil slot")
	}
	if len(plan.GaItems) != len(slot.GaItems) {
		return fmt.Errorf("ApplyGaItemRepackPlan: plan length %d != GaItems length %d", len(plan.GaItems), len(slot.GaItems))
	}
	if !plan.Changes {
		return nil
	}
	slot.GaItems = append([]GaItemFull(nil), plan.GaItems...)
	return nil
}

func gaItemCapacity(tableSize, nonEmpty, nextArmamentIndex int) GaItemCapacity {
	physicalEmpty := tableSize - nonEmpty
	cursorRoom := tableSize - nextArmamentIndex
	if cursorRoom < 0 {
		cursorRoom = 0
	}

	return GaItemCapacity{
		PhysicalEmpty: physicalEmpty,
		CursorRoom:    cursorRoom,
		Usable:        min(physicalEmpty, cursorRoom),
	}
}
