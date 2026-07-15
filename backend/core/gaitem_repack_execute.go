package core

import (
	"bytes"
	"fmt"
	"reflect"
	"strings"
)

// GaItemRepackResult describes a completed repack attempt. Repack never
// writes a save file; it only updates the passed in-memory candidate slot.
type GaItemRepackResult struct {
	Before    GaItemCapacity
	After     GaItemCapacity
	Recovered int
	Changed   bool
}

// RepackGaItems applies the canonical stable compaction to slot as one
// transaction. A caller that must keep its live slot untouched should pass a
// CloneSlot candidate and publish that candidate only after success.
//
// RepackGaItems refuses unsafe input before mutating it. Any error after the
// first mutation restores the complete SlotSnapshot, so the passed slot is
// left unchanged. It does not save a file or create a backup.
func RepackGaItems(slot *SaveSlot) (GaItemRepackResult, error) {
	if slot == nil {
		return GaItemRepackResult{}, fmt.Errorf("RepackGaItems: nil slot")
	}

	preflight := PreflightGaItemRepack(slot)
	if len(preflight.Blockers) != 0 {
		return GaItemRepackResult{}, fmt.Errorf("RepackGaItems: refused: %s", formatGaItemRepackBlockers(preflight.Blockers))
	}

	analysis := preflight.Analysis
	result := GaItemRepackResult{
		Before:    analysis.Before,
		After:     analysis.Before,
		Recovered: 0,
	}
	if analysis.NonEmptyRecords == 0 || analysis.Recovered == 0 {
		return result, nil
	}

	snapshot := SnapshotSlot(slot)
	plan := BuildGaItemRepackPlan(slot)
	rollback := func(cause error) (GaItemRepackResult, error) {
		RestoreSlot(slot, snapshot)
		return GaItemRepackResult{}, fmt.Errorf("RepackGaItems: %w", cause)
	}

	if err := ApplyGaItemRepackPlan(slot, plan); err != nil {
		return rollback(err)
	}
	rebuilt, err := RebuildSlotFull(slot)
	if err != nil {
		return rollback(fmt.Errorf("rebuild: %w", err))
	}
	slot.Data = rebuilt
	if err := slot.parseFromData(); err != nil {
		return rollback(fmt.Errorf("reparse: %w", err))
	}
	remergeMissingGaMapEntries(slot, snapshot.GaMap)

	post, err := validateGaItemRepackPostconditions(slot, snapshot, plan)
	if err != nil {
		return rollback(err)
	}
	return GaItemRepackResult{
		Before:    analysis.Before,
		After:     post.Analysis.Before,
		Recovered: post.Analysis.Before.Usable - analysis.Before.Usable,
		Changed:   true,
	}, nil
}

func formatGaItemRepackBlockers(blockers []GaItemRepackBlocker) string {
	parts := make([]string, len(blockers))
	for i, blocker := range blockers {
		parts[i] = blocker.Code + ": " + blocker.Message
	}
	return strings.Join(parts, "; ")
}

// remergeMissingGaMapEntries restores the known stackable entries that
// scanGaItems cannot reconstruct because they have no physical GaItem record.
// PreflightGaItemRepack has already rejected every other orphaned map entry.
func remergeMissingGaMapEntries(slot *SaveSlot, before map[uint32]uint32) {
	for handle, itemID := range before {
		if _, exists := slot.GaMap[handle]; !exists {
			slot.GaMap[handle] = itemID
		}
	}
}

func validateGaItemRepackPostconditions(slot *SaveSlot, snapshot SlotSnapshot, plan GaItemRepackPlan) (GaItemRepackPreflight, error) {
	if !sameGaItemPrefix(slot.GaItems, plan.GaItems, plan.NonEmptyRecords) {
		return GaItemRepackPreflight{}, fmt.Errorf("postcondition: non-empty GaItem records changed or lost their stable order")
	}
	for i := plan.NonEmptyRecords; i < len(slot.GaItems); i++ {
		if !slot.GaItems[i].IsEmpty() {
			return GaItemRepackPreflight{}, fmt.Errorf("postcondition: GaItem[%d] is not empty after compacted prefix", i)
		}
	}
	if slot.NextArmamentIndex > plan.NonEmptyRecords {
		return GaItemRepackPreflight{}, fmt.Errorf("postcondition: NextArmamentIndex %d > non-empty records %d", slot.NextArmamentIndex, plan.NonEmptyRecords)
	}
	if slot.NextAoWIndex > slot.NextArmamentIndex || slot.NextArmamentIndex > len(slot.GaItems) {
		return GaItemRepackPreflight{}, fmt.Errorf("postcondition: invalid GaItem cursors AoW=%d Armament=%d len=%d", slot.NextAoWIndex, slot.NextArmamentIndex, len(slot.GaItems))
	}
	if slot.NextGaItemHandle != snapshot.NextGaItemHandle {
		return GaItemRepackPreflight{}, fmt.Errorf("postcondition: NextGaItemHandle changed from %d to %d", snapshot.NextGaItemHandle, slot.NextGaItemHandle)
	}
	if slot.PartGaItemHandle != snapshot.PartGaItemHandle {
		return GaItemRepackPreflight{}, fmt.Errorf("postcondition: PartGaItemHandle changed from 0x%02X to 0x%02X", snapshot.PartGaItemHandle, slot.PartGaItemHandle)
	}
	if !reflect.DeepEqual(slot.GaMap, snapshot.GaMap) {
		return GaItemRepackPreflight{}, fmt.Errorf("postcondition: GaMap changed after reparse")
	}
	if err := verifyNonGaItemBytes(snapshot.Data, slot.Data, snapshot.GaItems); err != nil {
		return GaItemRepackPreflight{}, err
	}
	if violations := validateGaItemRepackPostMutation(slot); len(violations) != 0 {
		return GaItemRepackPreflight{}, fmt.Errorf("postcondition: repack integrity: %v", violations)
	}

	post := PreflightGaItemRepack(slot)
	if len(post.Blockers) != 0 {
		return GaItemRepackPreflight{}, fmt.Errorf("postcondition: preflight failed: %s", formatGaItemRepackBlockers(post.Blockers))
	}
	if post.Analysis.Recovered != 0 {
		return GaItemRepackPreflight{}, fmt.Errorf("postcondition: recovered capacity remains %d", post.Analysis.Recovered)
	}
	return post, nil
}

// validateGaItemRepackPostMutation keeps the generic integrity checks that
// cover data repack can affect. A raw InventoryItem.Index duplicate is omitted:
// stable GaItem compaction preserves Inventory and Storage byte-for-byte, and
// Elden Ring can legitimately write such duplicates after a container move.
func validateGaItemRepackPostMutation(slot *SaveSlot) []IntegrityError {
	all := ValidatePostMutation(slot)
	filtered := make([]IntegrityError, 0, len(all))
	for _, violation := range all {
		if violation.Check == "duplicate_index" {
			continue
		}
		filtered = append(filtered, violation)
	}
	return filtered
}

func sameGaItemPrefix(actual, planned []GaItemFull, count int) bool {
	if count < 0 || count > len(actual) || count > len(planned) {
		return false
	}
	for i := 0; i < count; i++ {
		if actual[i] != planned[i] {
			return false
		}
	}
	return true
}

func verifyNonGaItemBytes(before, after []byte, gaItems []GaItemFull) error {
	if len(before) != SlotSize || len(after) != SlotSize {
		return fmt.Errorf("postcondition: slot data length changed (%d -> %d)", len(before), len(after))
	}
	gaItemsEnd := GaItemsStart
	for i := range gaItems {
		gaItemsEnd += gaItems[i].ByteSize()
	}
	if gaItemsEnd > len(before) {
		return fmt.Errorf("postcondition: GaItems end 0x%X is outside slot data", gaItemsEnd)
	}
	if !bytes.Equal(before[:GaItemsStart], after[:GaItemsStart]) || !bytes.Equal(before[gaItemsEnd:], after[gaItemsEnd:]) {
		return fmt.Errorf("postcondition: bytes outside GaItems changed")
	}
	return nil
}
