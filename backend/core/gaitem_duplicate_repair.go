package core

import (
	"fmt"
	"strings"
)

// GaItemDuplicateCandidate is one physical GaItem record that shares a handle
// with another physical record. The two candidates always carry different
// ItemID values; which one to keep is a decision only the user can make.
type GaItemDuplicateCandidate struct {
	Index  int
	ItemID uint32
}

// GaItemDuplicateAnalysis is the read-only preflight for one requested duplicate
// physical GaItem handle. Repairable is true only when every safety condition
// holds; the caller must still pick which physical index to keep. This analysis
// never mutates the slot and never picks a candidate on the user's behalf.
type GaItemDuplicateAnalysis struct {
	Handle      uint32
	Candidates  [2]GaItemDuplicateCandidate
	Repairable  bool
	RefusalCode string
	RefusalMsg  string
}

// AnalyzeGaItemDuplicate inspects one requested handle and decides whether a
// single, unambiguous physical duplicate pair can be safely deduplicated. It is
// fail-closed: any missing condition or ambiguity returns Repairable=false with
// a stable RefusalCode and never mutates slot. It deliberately makes no choice
// between the two candidates.
func AnalyzeGaItemDuplicate(slot *SaveSlot, handle uint32) GaItemDuplicateAnalysis {
	res := GaItemDuplicateAnalysis{Handle: handle}
	if slot == nil {
		return refuseDuplicate(res, "no_slot", "nil slot")
	}

	// 1. Exactly two non-empty physical records use the handle.
	var idxs []int
	for i := range slot.GaItems {
		if slot.GaItems[i].IsEmpty() {
			continue
		}
		if slot.GaItems[i].Handle == handle {
			idxs = append(idxs, i)
		}
	}
	if len(idxs) != 2 {
		return refuseDuplicate(res, "not_exactly_two_records",
			fmt.Sprintf("handle 0x%08X has %d non-empty physical records, want exactly 2", handle, len(idxs)))
	}
	a := slot.GaItems[idxs[0]]
	b := slot.GaItems[idxs[1]]
	res.Candidates = [2]GaItemDuplicateCandidate{
		{Index: idxs[0], ItemID: a.ItemID},
		{Index: idxs[1], ItemID: b.ItemID},
	}

	// 2. Their ItemID values differ (identical records are not a user choice).
	if a.ItemID == b.ItemID {
		return refuseDuplicate(res, "identical_item_id",
			fmt.Sprintf("both physical records for handle 0x%08X share itemID 0x%08X", handle, a.ItemID))
	}

	// 3. Exactly one container record references the handle.
	if refs := gaItemHandleContainerRefs(slot, handle); refs != 1 {
		return refuseDuplicate(res, "reference_count",
			fmt.Sprintf("handle 0x%08X is referenced by %d container records, want exactly 1", handle, refs))
	}

	// 4. Equipment must be inspectable, and neither candidate may be equipped.
	// An unreadable equipment section is fail-closed: we refuse rather than
	// assume the candidates are unequipped.
	switch gaItemDuplicateEquipState(slot, handle, a, b) {
	case equipStateUnavailable:
		return refuseDuplicate(res, "equipment_state_unavailable",
			fmt.Sprintf("handle 0x%08X: equipment section is unreadable, cannot confirm the candidates are unequipped", handle))
	case equipStateEquipped:
		return refuseDuplicate(res, "candidate_equipped",
			fmt.Sprintf("a candidate for handle 0x%08X is currently equipped", handle))
	}

	// 5. Neither candidate is referenced as an Ash of War by another weapon.
	if gaItemHandleReferencedAsAoW(slot, handle) {
		return refuseDuplicate(res, "aow_reference",
			fmt.Sprintf("handle 0x%08X is referenced as an Ash of War by a weapon", handle))
	}

	// 6. No GaItem repack blocker exists apart from this exact duplicate — for
	// EITHER possible keep choice, so whichever index the user later picks is safe.
	if blockers := gaItemDuplicateOtherBlockers(slot, handle, idxs, a.ItemID, b.ItemID); len(blockers) != 0 {
		return refuseDuplicate(res, "other_preflight_blocker",
			fmt.Sprintf("handle 0x%08X cannot be repaired while other issues exist: %s", handle, formatGaItemRepackBlockers(blockers)))
	}

	res.Repairable = true
	return res
}

// RepairGaItemDuplicate removes only the unselected physical GaItem record for a
// verified duplicate pair, preserving the user-selected record unchanged. It runs
// as one transaction: any failed postcondition restores the complete slot, so a
// refused or failed call leaves the slot byte-for-byte and structurally unchanged.
// It never saves a file, never creates a backup, and never runs a repack.
func RepairGaItemDuplicate(slot *SaveSlot, handle uint32, keepIndex int) error {
	analysis := AnalyzeGaItemDuplicate(slot, handle)
	if !analysis.Repairable {
		return fmt.Errorf("RepairGaItemDuplicate: refused: %s: %s", analysis.RefusalCode, analysis.RefusalMsg)
	}

	c0, c1 := analysis.Candidates[0], analysis.Candidates[1]
	var removeIndex int
	switch keepIndex {
	case c0.Index:
		removeIndex = c1.Index
	case c1.Index:
		removeIndex = c0.Index
	default:
		return fmt.Errorf("RepairGaItemDuplicate: keepIndex %d is not one of the duplicate candidate indexes %d/%d", keepIndex, c0.Index, c1.Index)
	}
	keptRecord := slot.GaItems[keepIndex]
	keptItemID := keptRecord.ItemID
	removedRecord := slot.GaItems[removeIndex]

	snapshot := SnapshotSlot(slot)
	rollback := func(cause error) error {
		RestoreSlot(slot, snapshot)
		return fmt.Errorf("RepairGaItemDuplicate: %w", cause)
	}

	slot.GaItems[removeIndex] = GaItemFull{}
	// Removing an AoW that sat before a later AoW leaves an empty slot inside the
	// AoW block. RebuildSlotFull serializes slot.GaItems linearly, so the table
	// must already be the fixed point of the native CSGaitem::WriteArray stable
	// partition (AoW records first, then everything else — empties included — in
	// relative order) or the rebuilt bytes diverge from what the game emits.
	canonicalizeAoWPartition(slot.GaItems)
	rebuilt, err := RebuildSlotFull(slot)
	if err != nil {
		return rollback(fmt.Errorf("rebuild: %w", err))
	}
	slot.Data = rebuilt
	if err := slot.parseFromData(); err != nil {
		return rollback(fmt.Errorf("reparse: %w", err))
	}
	remergeMissingGaMapEntries(slot, snapshot.GaMap)

	if err := validateGaItemDuplicatePostconditions(slot, snapshot, handle, keptRecord, removedRecord, keptItemID); err != nil {
		return rollback(err)
	}
	return nil
}

// isAoWGaItem reports whether a record is a non-empty Ash of War entry, using
// the same field the native CSGaitem::WriteArray partitions on — the itemID's
// high nibble (0x8), not the handle. Empty slots are never AoW.
func isAoWGaItem(g GaItemFull) bool {
	return !g.IsEmpty() && g.ItemID>>28 == 8
}

// canonicalizeAoWPartition reorders items in place into the layout the native
// CSGaitem::WriteArray emits: pass 1 keeps every non-empty AoW record in its
// existing relative order, pass 2 appends every remaining entry (non-AoW records
// and empty slots) in its existing relative order. A table that already
// satisfies the partition is left unchanged, so a duplicate repair with no AoW
// hole performs no reorder.
func canonicalizeAoWPartition(items []GaItemFull) {
	reordered := make([]GaItemFull, 0, len(items))
	for _, g := range items {
		if isAoWGaItem(g) {
			reordered = append(reordered, g)
		}
	}
	for _, g := range items {
		if !isAoWGaItem(g) {
			reordered = append(reordered, g)
		}
	}
	copy(items, reordered)
}

func validateGaItemDuplicatePostconditions(slot *SaveSlot, snapshot SlotSnapshot, handle uint32, keptRecord, removedRecord GaItemFull, keptItemID uint32) error {
	if len(slot.GaItems) != len(snapshot.GaItems) {
		return fmt.Errorf("postcondition: GaItem table length changed (%d -> %d)", len(snapshot.GaItems), len(slot.GaItems))
	}
	// Record preservation under reorder. The AoW-first canonicalization may move
	// records, so identity is a multiset property, not a per-index one: the
	// post-repair non-empty records must equal the pre-repair non-empty records
	// with exactly one occurrence of the removed duplicate gone. (Empty slots are
	// ignored here; the length check above pins their count.)
	before := gaItemRecordCounts(snapshot.GaItems)
	if before[removedRecord] == 0 {
		return fmt.Errorf("postcondition: removed record for handle 0x%08X absent from pre-repair table", handle)
	}
	if before[removedRecord] == 1 {
		delete(before, removedRecord)
	} else {
		before[removedRecord]--
	}
	if after := gaItemRecordCounts(slot.GaItems); !sameRecordCounts(before, after) {
		return fmt.Errorf("postcondition: GaItem records changed beyond the single removed duplicate")
	}
	// The table is a fixed point of the native AoW-first partition: no non-empty
	// AoW record follows a non-AoW-or-empty slot.
	seenNonAoW := false
	for i := range slot.GaItems {
		if isAoWGaItem(slot.GaItems[i]) {
			if seenNonAoW {
				return fmt.Errorf("postcondition: AoW record at index %d follows a non-AoW slot (partition violated)", i)
			}
			continue
		}
		seenNonAoW = true
	}
	// Exactly one physical record remains for the handle, and it is the kept one.
	remaining := 0
	var survivor GaItemFull
	for i := range slot.GaItems {
		if !slot.GaItems[i].IsEmpty() && slot.GaItems[i].Handle == handle {
			remaining++
			survivor = slot.GaItems[i]
		}
	}
	if remaining != 1 {
		return fmt.Errorf("postcondition: handle 0x%08X has %d physical records after repair, want 1", handle, remaining)
	}
	if survivor != keptRecord {
		return fmt.Errorf("postcondition: surviving record for handle 0x%08X differs from the kept record", handle)
	}
	if mapped, ok := slot.GaMap[handle]; !ok || mapped != keptItemID {
		return fmt.Errorf("postcondition: GaMap[0x%08X]=0x%08X (ok=%v), want kept itemID 0x%08X", handle, mapped, ok, keptItemID)
	}
	// The sole container reference — and every other reference — is unchanged.
	if !sameInventoryItems(slot.Inventory.CommonItems, snapshot.Inventory.CommonItems) ||
		!sameInventoryItems(slot.Inventory.KeyItems, snapshot.Inventory.KeyItems) ||
		!sameInventoryItems(slot.Storage.CommonItems, snapshot.Storage.CommonItems) {
		return fmt.Errorf("postcondition: an inventory/storage reference changed")
	}
	if slot.Player != snapshot.Player {
		return fmt.Errorf("postcondition: player data changed")
	}
	if violations := ValidatePostMutation(slot); len(violations) != 0 {
		return fmt.Errorf("postcondition: ValidatePostMutation: %v", violations)
	}
	post := PreflightGaItemRepack(slot)
	if len(post.Blockers) != 0 {
		return fmt.Errorf("postcondition: preflight still blocked: %s", formatGaItemRepackBlockers(post.Blockers))
	}
	return nil
}

// gaItemDuplicateOtherBlockers returns every repack blocker that is NOT the
// expected duplicate_handle for handle, plus any blocker that would remain after
// removing either candidate. An empty result means the only defect is this exact
// duplicate and both keep choices are safe.
func gaItemDuplicateOtherBlockers(slot *SaveSlot, handle uint32, idxs []int, itemIDLow, itemIDHigh uint32) []GaItemRepackBlocker {
	handleTag := fmt.Sprintf("handle 0x%08X", handle)
	var other []GaItemRepackBlocker
	for _, b := range PreflightGaItemRepack(slot).Blockers {
		if b.Code == "duplicate_handle" && strings.Contains(b.Message, handleTag) {
			continue
		}
		other = append(other, b)
	}

	// Simulate removing each candidate in memory (no byte rebuild — preflight
	// reads GaItems/GaMap/containers, all unaffected by byte layout) and confirm
	// the remainder is clean for whichever record the user keeps.
	keeps := []struct {
		keep, remove int
		keepItemID   uint32
	}{
		{idxs[0], idxs[1], itemIDLow},
		{idxs[1], idxs[0], itemIDHigh},
	}
	for _, k := range keeps {
		clone := CloneSlot(slot)
		clone.GaItems[k.remove] = GaItemFull{}
		clone.GaMap[handle] = k.keepItemID
		other = append(other, PreflightGaItemRepack(clone).Blockers...)
	}
	return other
}

func gaItemHandleContainerRefs(slot *SaveSlot, handle uint32) int {
	n := 0
	for _, list := range [][]InventoryItem{slot.Inventory.CommonItems, slot.Inventory.KeyItems, slot.Storage.CommonItems} {
		for _, it := range list {
			if it.GaItemHandle == handle {
				n++
			}
		}
	}
	return n
}

func gaItemHandleReferencedAsAoW(slot *SaveSlot, handle uint32) bool {
	for i := range slot.GaItems {
		g := slot.GaItems[i]
		if g.IsEmpty() || g.Handle&GaHandleTypeMask != ItemTypeWeapon {
			continue
		}
		if IsNoCustomAoWHandle(g.AoWGaItemHandle) {
			continue
		}
		if g.AoWGaItemHandle == handle {
			return true
		}
	}
	return false
}

type gaItemEquipState int

const (
	// equipStateReadable means the equipment section was inspected and neither
	// candidate is equipped.
	equipStateReadable gaItemEquipState = iota
	// equipStateUnavailable means the equipment section could not be read, so
	// the equipped/unequipped status is unknown and repair must refuse.
	equipStateUnavailable
	// equipStateEquipped means a candidate is currently equipped.
	equipStateEquipped
)

// gaItemDuplicateEquipState inspects the ChrAsm equipment section for either
// candidate. A missing or out-of-bounds section returns equipStateUnavailable
// (never "unequipped") so unreadable equipment fails closed. Equipment stores
// itemID-derived values (weapon and armor as itemID|ItemTypeWeapon, goods and
// talismans as the bare itemID), so each candidate's signature is derived from
// the shared handle's type.
func gaItemDuplicateEquipState(slot *SaveSlot, handle uint32, a, b GaItemFull) gaItemEquipState {
	if slot.EquipItemsIDOffset <= 0 || slot.EquipItemsIDOffset+ChrAsmEquipmentSize > len(slot.Data) {
		return equipStateUnavailable
	}
	equipped := make(map[uint32]bool)
	for _, v := range readEquipSection(slot.Data, slot.EquipItemsIDOffset) {
		if v != GaHandleEmpty && v != GaHandleInvalid {
			equipped[v] = true
		}
	}
	handleType := handle & GaHandleTypeMask
	for _, itemID := range []uint32{a.ItemID, b.ItemID} {
		for _, sig := range gaItemEquipSignatures(itemID, handleType) {
			if equipped[sig] {
				return equipStateEquipped
			}
		}
	}
	return equipStateReadable
}

func gaItemEquipSignatures(itemID, handleType uint32) []uint32 {
	switch handleType {
	case ItemTypeWeapon, ItemTypeArmor:
		return []uint32{itemID | ItemTypeWeapon}
	case ItemTypeAccessory, ItemTypeItem:
		return []uint32{itemID}
	default:
		return nil
	}
}

// gaItemRecordCounts is the multiset of non-empty GaItem records. Empty slots
// are excluded so differing empty-marker forms (zeroed vs. reparsed 0xFFFFFFFF)
// never register as a record change.
func gaItemRecordCounts(items []GaItemFull) map[GaItemFull]int {
	counts := make(map[GaItemFull]int)
	for _, g := range items {
		if g.IsEmpty() {
			continue
		}
		counts[g]++
	}
	return counts
}

func sameRecordCounts(a, b map[GaItemFull]int) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

func sameInventoryItems(a, b []InventoryItem) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func refuseDuplicate(res GaItemDuplicateAnalysis, code, msg string) GaItemDuplicateAnalysis {
	res.Repairable = false
	res.RefusalCode = code
	res.RefusalMsg = msg
	return res
}
