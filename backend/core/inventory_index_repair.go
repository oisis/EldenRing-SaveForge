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

// RepairDuplicateInventoryIndices reassigns the Index of every duplicate
// occurrence in Inventory.CommonItems + KeyItems so that all non-empty entries
// share a globally-unique Index. The first occurrence of each value is kept;
// every subsequent occurrence gets a fresh Index > all existing values, taken
// from a counter seeded at max(NextAcquisitionSortId, max(existing Index)+1).
//
// Updates both the in-memory InventoryItem and the matching uint32 in
// slot.Data so a subsequent WriteSave (or any direct raw read) sees the
// corrected Index. Also advances NextAcquisitionSortId / NextEquipIndex to
// stay > all assigned indices, with the matching slot.Data counters written
// back when their offsets are known.
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

	// Seed the new-index counter from the higher of (NextAcquisitionSortId,
	// max existing Index + 1). Both inputs may be stale on saves edited by
	// other tools, so take the max defensively. InvEquipReservedMax floor
	// matches addToInventory's clamp.
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
	nextFree := maxIdx + 1
	if slot.Inventory.NextAcquisitionSortId > nextFree {
		nextFree = slot.Inventory.NextAcquisitionSortId
	}
	if nextFree <= InvEquipReservedMax {
		nextFree = InvEquipReservedMax + 1
	}

	seen := make(map[uint32]bool)

	reassign := func(scope string, entryStart int, items []InventoryItem) error {
		for i := range items {
			it := items[i]
			if it.GaItemHandle == GaHandleEmpty || it.GaItemHandle == GaHandleInvalid {
				continue
			}
			if !seen[it.Index] {
				seen[it.Index] = true
				continue
			}
			newIdx := nextFree
			nextFree++
			// Belt-and-braces: skip values already seen (shouldn't happen
			// since nextFree starts > maxIdx, but guards against future
			// changes to the seeding logic).
			for seen[newIdx] {
				newIdx = nextFree
				nextFree++
			}
			seen[newIdx] = true

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

	// No reassignments → leave counters untouched (idempotent no-op). The
	// InvEquipReservedMax floor used above is only relevant when we actually
	// pulled fresh Indices for duplicate rows.
	if report.Changed == 0 {
		return report, nil
	}

	// Advance ONLY the acquisition/sort counter so future additions don't collide
	// with reassigned indices. NextEquipIndex is a SEPARATE equip-list counter, NOT
	// a visibility gate: genuine console saves keep it far below NextAcquisitionSortId.
	// Forcing it up here (the old behaviour) corrupted the slot (CE-108255-1) — so the
	// dedup renumbers duplicate indices but never touches NextEquipIndex.
	if nextFree > slot.Inventory.NextAcquisitionSortId {
		slot.Inventory.NextAcquisitionSortId = nextFree
		if slot.Inventory.nextAcqSortIdOff > 0 && slot.Inventory.nextAcqSortIdOff+4 <= len(slot.Data) {
			binary.LittleEndian.PutUint32(slot.Data[slot.Inventory.nextAcqSortIdOff:], slot.Inventory.NextAcquisitionSortId)
		}
	}

	return report, nil
}

// AssignFreshInventoryIndex assigns a new, safe acquisition index to exactly
// one inventory record identified by scope + row. The new index is:
//   - greater than all existing indices across inventory_common and inventory_key;
//   - greater than InvEquipReservedMax, as a conservative floor for newly
//     generated editor indices;
//   - unique within the combined index space.
//
// Both the in-memory InventoryItem and the raw slot.Data bytes are updated.
// NextAcquisitionSortId is advanced if the new index exceeds the current value.
// NextEquipIndex is never touched (see CE-108255-1).
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

	// Collect all existing indices across both scopes to guarantee uniqueness.
	seen := make(map[uint32]bool)
	var maxIdx uint32
	for _, item := range slot.Inventory.CommonItems {
		if item.GaItemHandle == GaHandleEmpty || item.GaItemHandle == GaHandleInvalid {
			continue
		}
		seen[item.Index] = true
		if item.Index > maxIdx {
			maxIdx = item.Index
		}
	}
	for _, item := range slot.Inventory.KeyItems {
		if item.GaItemHandle == GaHandleEmpty || item.GaItemHandle == GaHandleInvalid {
			continue
		}
		seen[item.Index] = true
		if item.Index > maxIdx {
			maxIdx = item.Index
		}
	}

	nextFree := maxIdx + 1
	if slot.Inventory.NextAcquisitionSortId > nextFree {
		nextFree = slot.Inventory.NextAcquisitionSortId
	}
	if nextFree <= InvEquipReservedMax {
		nextFree = InvEquipReservedMax + 1
	}
	for seen[nextFree] {
		nextFree++
	}
	newIdx := nextFree

	off := entryStart + row*InvRecordLen + 8
	if off < 0 || off+4 > len(slot.Data) {
		return zero, fmt.Errorf("AssignFreshInventoryIndex: %s row %d index byte offset %d out of bounds (data len %d)",
			scope, row, off, len(slot.Data))
	}
	binary.LittleEndian.PutUint32(slot.Data[off:], newIdx)
	items[row].Index = newIdx

	// Advance acquisition counter so future additions don't collide.
	// Never touch NextEquipIndex (CE-108255-1).
	if newIdx+1 > slot.Inventory.NextAcquisitionSortId {
		slot.Inventory.NextAcquisitionSortId = newIdx + 1
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
