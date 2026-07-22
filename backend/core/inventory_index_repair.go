package core

import (
	"encoding/binary"
	"fmt"
)

// InventoryIndexRepairChange describes one Index reassignment performed by
// RepairDuplicateInventoryIndices. NewIndex is guaranteed unique across the
// combined Inventory.CommonItems + KeyItems set after the repair completes.
type InventoryIndexRepairChange struct {
	Scope    string `json:"scope"` // "inventory_common" | "inventory_key"
	Row      int    `json:"row"`
	Handle   uint32 `json:"handle"`
	OldIndex uint32 `json:"oldIndex"`
	NewIndex uint32 `json:"newIndex"`
}

// InventoryIndexRepairReport is the outcome of one repair invocation.
// Changed == 0 means the slot was already clean (no-op, idempotent).
type InventoryIndexRepairReport struct {
	Changed int                          `json:"changed"`
	Changes []InventoryIndexRepairChange `json:"changes"`
}

// RepairDuplicateInventoryIndices reassigns the Index of every record that
// shares an acquisition-order bucket (Index>>1) with an earlier record in
// Inventory.CommonItems + KeyItems, so that every non-empty entry occupies a
// distinct bucket. The first record in a bucket is kept; every subsequent one
// is moved to a fresh bucket.
//
// Elden Ring keys "Order of Acquisition" by Index>>1 (spec 52), so fresh
// indices MUST follow the native stride-2 pattern — an even mark, the record's
// Index = mark+1, then mark += 2 — otherwise two renumbered records land in the
// same bucket and revert on restart. The mark is seeded at
// max(NextAcquisitionSortId, max(existing Index)+1), rounded up to an even value
// > InvEquipReservedMax via nextAcquisitionWriteIndex.
//
// Updates both the in-memory InventoryItem and the matching uint32 in
// slot.Data so a subsequent WriteSave (or any direct raw read) sees the
// corrected Index. After the series NextAcquisitionSortId is advanced to at
// least the final mark; NextEquipIndex is never touched (CE-108255-1).
//
// Read scope is identical to ScanDuplicateInventoryIndices: empty / invalid
// handles are ignored, storage is not touched.
//
// Idempotent: a second call on the repaired slot returns Changed=0.
func RepairDuplicateInventoryIndices(slot *SaveSlot) (InventoryIndexRepairReport, error) {
	var report InventoryIndexRepairReport
	if slot == nil {
		return report, fmt.Errorf("RepairDuplicateInventoryIndices: nil slot")
	}

	commonStart := slot.MagicOffset + InvStartFromMagic
	keyStart := commonStart + CommonItemCount*InvRecordLen + InvKeyCountHeader

	// Seed the stride-2 mark from the higher of (NextAcquisitionSortId,
	// max existing Index + 1). Both inputs may be stale on saves edited by
	// other tools, so take the max defensively, then round up to an even mark
	// above InvEquipReservedMax (nextAcquisitionWriteIndex mirrors addToInventory).
	var maxIdx uint32
	for _, it := range slot.Inventory.CommonItems {
		if it.GaItemHandle == GaHandleEmpty || it.GaItemHandle == GaHandleInvalid {
			continue
		}
		if it.Index > maxIdx {
			maxIdx = it.Index
		}
	}
	for _, it := range slot.Inventory.KeyItems {
		if it.GaItemHandle == GaHandleEmpty || it.GaItemHandle == GaHandleInvalid {
			continue
		}
		if it.Index > maxIdx {
			maxIdx = it.Index
		}
	}
	seed := maxIdx + 1
	if slot.Inventory.NextAcquisitionSortId > seed {
		seed = slot.Inventory.NextAcquisitionSortId
	}
	mark := nextAcquisitionWriteIndex(seed, InvEquipReservedMax+2)

	// seenBucket tracks Index>>1 keys already claimed (kept records + freshly
	// assigned ones) so no reassignment reuses a bucket.
	seenBucket := make(map[uint32]bool)

	reassign := func(scope string, entryStart int, items []InventoryItem) error {
		for i := range items {
			it := items[i]
			if it.GaItemHandle == GaHandleEmpty || it.GaItemHandle == GaHandleInvalid {
				continue
			}
			bucket := it.Index >> 1
			if !seenBucket[bucket] {
				seenBucket[bucket] = true
				continue
			}
			// Collision: allocate the next free stride-2 bucket. mark is even,
			// so newIdx = mark+1 lands in bucket mark>>1; mark += 2 moves to the
			// next bucket. Belt-and-braces skip if a bucket is somehow taken.
			newBucket := mark >> 1
			for seenBucket[newBucket] {
				mark += 2
				newBucket = mark >> 1
			}
			newIdx := mark + 1
			mark += 2
			seenBucket[newBucket] = true

			off := entryStart + i*InvRecordLen + 8
			if off < 0 || off+4 > len(slot.Data) {
				return fmt.Errorf("RepairDuplicateInventoryIndices: %s row %d index byte offset %d out of bounds (data len %d)",
					scope, i, off, len(slot.Data))
			}
			binary.LittleEndian.PutUint32(slot.Data[off:], newIdx)
			items[i].Index = newIdx

			report.Changes = append(report.Changes, InventoryIndexRepairChange{
				Scope:    scope,
				Row:      i,
				Handle:   it.GaItemHandle,
				OldIndex: it.Index,
				NewIndex: newIdx,
			})
		}
		return nil
	}

	if err := reassign("inventory_common", commonStart, slot.Inventory.CommonItems); err != nil {
		return report, err
	}
	if err := reassign("inventory_key", keyStart, slot.Inventory.KeyItems); err != nil {
		return report, err
	}

	report.Changed = len(report.Changes)

	// No reassignments → leave counters untouched (idempotent no-op).
	if report.Changed == 0 {
		return report, nil
	}

	// Advance ONLY the acquisition/sort counter to at least the final mark so
	// future additions don't collide with reassigned indices. NextEquipIndex is
	// a SEPARATE equip-list counter, NOT a visibility gate: genuine console saves
	// keep it far below NextAcquisitionSortId. Forcing it up (the old behaviour)
	// corrupted the slot (CE-108255-1) — so the repair renumbers colliding
	// buckets but never touches NextEquipIndex.
	if mark > slot.Inventory.NextAcquisitionSortId {
		slot.Inventory.NextAcquisitionSortId = mark
		if slot.Inventory.nextAcqSortIdOff > 0 && slot.Inventory.nextAcqSortIdOff+4 <= len(slot.Data) {
			binary.LittleEndian.PutUint32(slot.Data[slot.Inventory.nextAcqSortIdOff:], slot.Inventory.NextAcquisitionSortId)
		}
	}

	return report, nil
}

// AssignFreshInventoryIndex assigns a new, safe acquisition index to exactly
// one inventory record identified by scope + row. The new index follows the
// native stride-2 pattern (even mark, Index = mark+1) so its bucket (Index>>1)
// is:
//   - distinct from every existing record's bucket across inventory_common and
//     inventory_key — Elden Ring keys Order of Acquisition by Index>>1 (spec 52),
//     so a bucket-unique index, not merely a value-unique one, is what keeps the
//     record from swapping/reverting in-game;
//   - above InvEquipReservedMax, a conservative floor for editor-generated indices.
//
// Both the in-memory InventoryItem and the raw slot.Data bytes are updated.
// NextAcquisitionSortId is advanced to at least the final mark; NextEquipIndex is
// never touched (see CE-108255-1).
//
// Scope must be "inventory_common" or "inventory_key".
// This primitive is the building block for both duplicate-index repair and
// single-record index repair; the batch RepairDuplicateInventoryIndices uses
// its own counter loop for efficiency.
func AssignFreshInventoryIndex(slot *SaveSlot, scope string, row int) (InventoryIndexRepairChange, error) {
	var zero InventoryIndexRepairChange
	if slot == nil {
		return zero, fmt.Errorf("AssignFreshInventoryIndex: nil slot")
	}

	commonStart := slot.MagicOffset + InvStartFromMagic
	keyStart := commonStart + CommonItemCount*InvRecordLen + InvKeyCountHeader

	var items []InventoryItem
	var entryStart int
	switch scope {
	case "inventory_common":
		items = slot.Inventory.CommonItems
		entryStart = commonStart
	case "inventory_key":
		items = slot.Inventory.KeyItems
		entryStart = keyStart
	default:
		return zero, fmt.Errorf("AssignFreshInventoryIndex: unknown scope %q", scope)
	}

	if row < 0 || row >= len(items) {
		return zero, fmt.Errorf("AssignFreshInventoryIndex: row %d out of bounds (len=%d)", row, len(items))
	}
	it := items[row]
	if it.GaItemHandle == GaHandleEmpty || it.GaItemHandle == GaHandleInvalid {
		return zero, fmt.Errorf("AssignFreshInventoryIndex: row %d has empty/invalid handle", row)
	}

	// Collect every existing bucket (Index>>1) across both scopes so the fresh
	// index lands in an unclaimed bucket, plus the max Index to seed the mark.
	seenBucket := make(map[uint32]bool)
	var maxIdx uint32
	for _, item := range slot.Inventory.CommonItems {
		if item.GaItemHandle == GaHandleEmpty || item.GaItemHandle == GaHandleInvalid {
			continue
		}
		seenBucket[item.Index>>1] = true
		if item.Index > maxIdx {
			maxIdx = item.Index
		}
	}
	for _, item := range slot.Inventory.KeyItems {
		if item.GaItemHandle == GaHandleEmpty || item.GaItemHandle == GaHandleInvalid {
			continue
		}
		seenBucket[item.Index>>1] = true
		if item.Index > maxIdx {
			maxIdx = item.Index
		}
	}

	seed := maxIdx + 1
	if slot.Inventory.NextAcquisitionSortId > seed {
		seed = slot.Inventory.NextAcquisitionSortId
	}
	mark := nextAcquisitionWriteIndex(seed, InvEquipReservedMax+2)
	for seenBucket[mark>>1] {
		mark += 2
	}
	newIdx := mark + 1

	off := entryStart + row*InvRecordLen + 8
	if off < 0 || off+4 > len(slot.Data) {
		return zero, fmt.Errorf("AssignFreshInventoryIndex: %s row %d index byte offset %d out of bounds (data len %d)",
			scope, row, off, len(slot.Data))
	}
	binary.LittleEndian.PutUint32(slot.Data[off:], newIdx)
	items[row].Index = newIdx

	// Advance acquisition counter to at least the final mark so future additions
	// don't collide. Never touch NextEquipIndex (CE-108255-1).
	if mark+2 > slot.Inventory.NextAcquisitionSortId {
		slot.Inventory.NextAcquisitionSortId = mark + 2
		if slot.Inventory.nextAcqSortIdOff > 0 && slot.Inventory.nextAcqSortIdOff+4 <= len(slot.Data) {
			binary.LittleEndian.PutUint32(slot.Data[slot.Inventory.nextAcqSortIdOff:], slot.Inventory.NextAcquisitionSortId)
		}
	}

	return InventoryIndexRepairChange{
		Scope:    scope,
		Row:      row,
		Handle:   it.GaItemHandle,
		OldIndex: it.Index,
		NewIndex: newIdx,
	}, nil
}
