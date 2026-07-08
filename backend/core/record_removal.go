package core

import (
	"encoding/binary"
	"fmt"
)

// RemoveInventoryRecordAt deletes exactly ONE inventory or storage record,
// identified structurally by scope + row (matching the RepairIssue IssueKey)
// plus a fingerprint stale-check.
//
// Unlike RemoveItemFromSlot — which zeroes EVERY record sharing a handle — this
// targets a single record by position. Duplicate talisman copies (0xA0) and any
// other record that happens to share the same handle in a different row are left
// untouched.
//
// scope is one of the repairScope* constants (inventory_common, inventory_key,
// storage_common). row is the index into the matching in-memory list exactly as
// the scanner produced it. fingerprint must equal fingerprintInventoryItem of
// the record currently at that row; a mismatch means the slot changed since the
// scan, so the removal is refused with no mutation.
//
// GaItem records are intentionally NOT garbage-collected: another record may
// still reference the same handle/GaItem. Call RepairOrphanedGaItems separately
// when GC is explicitly wanted.
func RemoveInventoryRecordAt(slot *SaveSlot, scope string, row int, fingerprint string) error {
	if slot == nil {
		return fmt.Errorf("RemoveInventoryRecordAt: nil slot")
	}
	switch scope {
	case repairScopeInventoryCommon:
		commonStart := slot.MagicOffset + InvStartFromMagic
		if err := removeInventoryRow(slot, slot.Inventory.CommonItems, commonStart, row, fingerprint); err != nil {
			return err
		}
		ReconcileInventoryHeader(slot)
		return nil
	case repairScopeInventoryKey:
		keyStart := slot.MagicOffset + InvStartFromMagic + CommonItemCount*InvRecordLen + InvKeyCountHeader
		return removeInventoryRow(slot, slot.Inventory.KeyItems, keyStart, row, fingerprint)
	case repairScopeStorageCommon:
		return removeStorageRow(slot, row, fingerprint)
	default:
		return fmt.Errorf("RemoveInventoryRecordAt: unsupported scope %q", scope)
	}
}

// removeInventoryRow clears one record in a fully pre-allocated inventory block
// (held CommonItems / KeyItems), where the in-memory slice index equals the
// physical binary slot. The index field is preserved (matches RemoveItemFromSlot
// convention of writing the slot position back).
func removeInventoryRow(slot *SaveSlot, list []InventoryItem, blockStart, row int, fingerprint string) error {
	if row < 0 || row >= len(list) {
		return fmt.Errorf("RemoveInventoryRecordAt: row %d out of range [0,%d)", row, len(list))
	}
	item := list[row]
	if item.GaItemHandle == GaHandleEmpty || item.GaItemHandle == GaHandleInvalid {
		return fmt.Errorf("RemoveInventoryRecordAt: row %d is already empty", row)
	}
	if got := fingerprintInventoryItem(item); got != fingerprint {
		return fmt.Errorf("RemoveInventoryRecordAt: fingerprint mismatch at row %d (record changed since scan)", row)
	}
	off := blockStart + row*InvRecordLen
	sa := NewSlotAccessor(slot.Data)
	if err := sa.CheckBounds(off, InvRecordLen, "RemoveInventoryRecordAt/inventory"); err != nil {
		return err
	}
	binary.LittleEndian.PutUint32(slot.Data[off:], 0)
	binary.LittleEndian.PutUint32(slot.Data[off+4:], 0)
	binary.LittleEndian.PutUint32(slot.Data[off+8:], uint32(row))
	list[row] = InventoryItem{GaItemHandle: 0, Quantity: 0, Index: uint32(row)}
	return nil
}

// removeStorageRow clears one record in the COMPACTED storage CommonItems list.
// The in-memory slice skips empty physical slots (ReadStorage), so slice row N
// maps to the N-th non-empty physical record — located by scanning the raw
// array. The physical record is verified against the fingerprinted in-memory
// item before clearing, then dropped from the slice and the header reconciled.
func removeStorageRow(slot *SaveSlot, row int, fingerprint string) error {
	list := slot.Storage.CommonItems
	if row < 0 || row >= len(list) {
		return fmt.Errorf("RemoveInventoryRecordAt: storage row %d out of range [0,%d)", row, len(list))
	}
	item := list[row]
	if got := fingerprintInventoryItem(item); got != fingerprint {
		return fmt.Errorf("RemoveInventoryRecordAt: fingerprint mismatch at storage row %d (record changed since scan)", row)
	}
	if slot.StorageBoxOffset <= 0 {
		return fmt.Errorf("RemoveInventoryRecordAt: slot has no storage box")
	}
	storageStart := slot.StorageBoxOffset + StorageHeaderSkip

	physical := -1
	seen := 0
	for i := 0; i < StorageCommonCount; i++ {
		off := storageStart + i*InvRecordLen
		if off+InvRecordLen > len(slot.Data) {
			break
		}
		h := binary.LittleEndian.Uint32(slot.Data[off:])
		if h == GaHandleEmpty || h == GaHandleInvalid {
			continue
		}
		if seen == row {
			physical = off
			break
		}
		seen++
	}
	if physical < 0 {
		return fmt.Errorf("RemoveInventoryRecordAt: storage row %d not found in physical array", row)
	}
	// Guard against list/binary drift: the located physical record must match the
	// fingerprinted in-memory item exactly before we zero it.
	if binary.LittleEndian.Uint32(slot.Data[physical:]) != item.GaItemHandle ||
		binary.LittleEndian.Uint32(slot.Data[physical+4:]) != item.Quantity ||
		binary.LittleEndian.Uint32(slot.Data[physical+8:]) != item.Index {
		return fmt.Errorf("RemoveInventoryRecordAt: storage row %d binary/list drift", row)
	}
	binary.LittleEndian.PutUint32(slot.Data[physical:], 0)
	binary.LittleEndian.PutUint32(slot.Data[physical+4:], 0)
	binary.LittleEndian.PutUint32(slot.Data[physical+8:], 0)
	// Storage list is compacted — drop the row rather than leaving an empty entry.
	slot.Storage.CommonItems = append(list[:row], list[row+1:]...)
	reconcileStorageHeader(slot)
	return nil
}

// reconcileStorageHeader rewrites the 4-byte storage distinct-item count at
// StorageBoxOffset to the actual number of non-empty physical records, matching
// the invariant asserted by SlotDiagnostics.checkStorageHeader.
func reconcileStorageHeader(slot *SaveSlot) {
	if slot.StorageBoxOffset <= 0 || slot.StorageBoxOffset+4 > len(slot.Data) {
		return
	}
	storageStart := slot.StorageBoxOffset + StorageHeaderSkip
	count := uint32(0)
	for i := 0; i < StorageCommonCount; i++ {
		off := storageStart + i*InvRecordLen
		if off+InvRecordLen > len(slot.Data) {
			break
		}
		h := binary.LittleEndian.Uint32(slot.Data[off:])
		if h != GaHandleEmpty && h != GaHandleInvalid {
			count++
		}
	}
	binary.LittleEndian.PutUint32(slot.Data[slot.StorageBoxOffset:], count)
}
