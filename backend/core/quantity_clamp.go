package core

import (
	"encoding/binary"
	"fmt"
)

// QuantityClampChange describes one ClampInventoryQuantityAt result. OldQuantity
// and NewQuantity are the RAW quantity fields (high bit preserved), so a caller
// can tell exactly what changed on the wire.
type QuantityClampChange struct {
	Scope       string `json:"scope"`
	Row         int    `json:"row"`
	Handle      uint32 `json:"handle"`
	OldQuantity uint32 `json:"oldQuantity"`
	NewQuantity uint32 `json:"newQuantity"`
	Cap         uint64 `json:"cap"`
}

// ClampInventoryQuantityAt clamps the quantity of the single record at scope+row
// down to its authoritative effective cap, recomputed from the item DB at apply
// time. It never accepts a caller-supplied cap.
//
// The record is re-resolved, its fingerprint stale-checked against the scan-time
// value, and the raw/in-memory state verified for consistency before any write.
// Records without an applicable cap, with a zero cap (item not permitted in the
// container — removal, not clamping, is the correct repair), or already at/below
// the cap are rejected with no mutation. The high quantity bit (0x80000000) is
// preserved.
//
// Every fallible check runs before the single write, so no failure is possible
// afterwards and the primitive needs no snapshot of its own (the App repair
// wrapper still snapshots for defense in depth). Exactly one 4-byte raw quantity
// field and its in-memory counterpart change; handle, acquisition index,
// headers, GaItems and every other record are left untouched.
//
// scope is one of the repairScope* constants; row is the index into the matching
// in-memory list exactly as the scanner produced it.
func ClampInventoryQuantityAt(slot *SaveSlot, scope string, row int, fingerprint string) (QuantityClampChange, error) {
	var zero QuantityClampChange
	if slot == nil {
		return zero, fmt.Errorf("ClampInventoryQuantityAt: nil slot")
	}

	list, err := clampScopeList(slot, scope)
	if err != nil {
		return zero, err
	}
	if row < 0 || row >= len(list) {
		return zero, fmt.Errorf("ClampInventoryQuantityAt: %s row %d out of range [0,%d)", scope, row, len(list))
	}
	item := list[row]
	if item.GaItemHandle == GaHandleEmpty || item.GaItemHandle == GaHandleInvalid {
		return zero, fmt.Errorf("ClampInventoryQuantityAt: %s row %d has empty/invalid handle", scope, row)
	}
	if got := fingerprintInventoryItem(item); got != fingerprint {
		return zero, fmt.Errorf("ClampInventoryQuantityAt: fingerprint mismatch at %s row %d (record changed since scan)", scope, row)
	}

	// Resolve the write offset and verify raw/in-memory consistency BEFORE any
	// mutation. Storage is compacted, so its physical byte offset is found by
	// scanning (storageBinaryOff, which also revalidates the handle); inventory is
	// pre-allocated, so the row maps directly to a binary slot.
	off, err := clampRecordOffset(slot, scope, row, item.GaItemHandle)
	if err != nil {
		return zero, err
	}
	if binary.LittleEndian.Uint32(slot.Data[off:]) != item.GaItemHandle ||
		binary.LittleEndian.Uint32(slot.Data[off+4:]) != item.Quantity ||
		binary.LittleEndian.Uint32(slot.Data[off+8:]) != item.Index {
		return zero, fmt.Errorf("ClampInventoryQuantityAt: %s row %d binary/list drift", scope, row)
	}

	// Recompute the authoritative cap — never trust a caller value.
	rec := ResolveRecord(slot, scope, row, item.GaItemHandle, item.Quantity, item.Index)
	if rec.Resolution != ResolutionKnownDB {
		return zero, fmt.Errorf("ClampInventoryQuantityAt: %s row %d did not resolve to a DB entry (%s)", scope, row, rec.Resolution)
	}
	limit, applies := EffectiveQuantityCap(rec, slot.Player.ClearCount)
	if !applies {
		return zero, fmt.Errorf("ClampInventoryQuantityAt: %s row %d has no applicable cap", scope, row)
	}
	if limit == 0 {
		return zero, fmt.Errorf("ClampInventoryQuantityAt: %s row %d cap is 0 (item not permitted in container); use remove_record", scope, row)
	}
	if limit > 0x7FFFFFFF {
		return zero, fmt.Errorf("ClampInventoryQuantityAt: %s row %d cap %d exceeds the 31-bit quantity field", scope, row, limit)
	}

	oldRaw := item.Quantity
	flag := oldRaw & 0x80000000
	effective := oldRaw & 0x7FFFFFFF
	if uint64(effective) <= limit {
		return zero, fmt.Errorf("ClampInventoryQuantityAt: %s row %d effective quantity %d already within cap %d", scope, row, effective, limit)
	}
	newRaw := flag | uint32(limit)

	binary.LittleEndian.PutUint32(slot.Data[off+4:], newRaw)
	list[row].Quantity = newRaw

	return QuantityClampChange{
		Scope:       scope,
		Row:         row,
		Handle:      item.GaItemHandle,
		OldQuantity: oldRaw,
		NewQuantity: newRaw,
		Cap:         limit,
	}, nil
}

// clampScopeList returns the in-memory record list for a supported scope.
func clampScopeList(slot *SaveSlot, scope string) ([]InventoryItem, error) {
	switch scope {
	case repairScopeInventoryCommon:
		return slot.Inventory.CommonItems, nil
	case repairScopeInventoryKey:
		return slot.Inventory.KeyItems, nil
	case repairScopeStorageCommon:
		return slot.Storage.CommonItems, nil
	default:
		return nil, fmt.Errorf("ClampInventoryQuantityAt: unsupported scope %q", scope)
	}
}

// clampRecordOffset returns the raw byte offset of the record at scope+row.
// Inventory scopes are pre-allocated (row maps directly); storage is compacted,
// so its offset is resolved by scanning via storageBinaryOff (which also
// revalidates the expected handle). The returned offset is bounds-checked to hold
// a full InvRecordLen record.
func clampRecordOffset(slot *SaveSlot, scope string, row int, handle uint32) (int, error) {
	switch scope {
	case repairScopeInventoryCommon:
		return clampInvOffset(slot, slot.MagicOffset+InvStartFromMagic, row)
	case repairScopeInventoryKey:
		base := slot.MagicOffset + InvStartFromMagic + CommonItemCount*InvRecordLen + InvKeyCountHeader
		return clampInvOffset(slot, base, row)
	case repairScopeStorageCommon:
		if slot.StorageBoxOffset <= 0 {
			return 0, fmt.Errorf("ClampInventoryQuantityAt: slot has no storage box")
		}
		return storageBinaryOff(slot, row, handle)
	default:
		return 0, fmt.Errorf("ClampInventoryQuantityAt: unsupported scope %q", scope)
	}
}

func clampInvOffset(slot *SaveSlot, base, row int) (int, error) {
	off := base + row*InvRecordLen
	if off < 0 || off+InvRecordLen > len(slot.Data) {
		return 0, fmt.Errorf("ClampInventoryQuantityAt: record offset %d out of bounds (data len %d)", off, len(slot.Data))
	}
	return off, nil
}
