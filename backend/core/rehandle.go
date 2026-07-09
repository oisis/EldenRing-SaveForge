package core

import (
	"encoding/binary"
	"fmt"
)

// RehandleChange describes the outcome of one RehandleInventoryRecord call.
type RehandleChange struct {
	Scope     string `json:"scope"`
	Row       int    `json:"row"`
	OldHandle uint32 `json:"oldHandle"`
	NewHandle uint32 `json:"newHandle"`
	ItemID    uint32 `json:"itemID"`
}

// RehandleInventoryRecord assigns a fresh unique handle to the record at
// scope+row, leaving the record in its container. ItemID, quantity, and
// acquisition index are preserved.
//
// Weapons (0x80), armor (0x90), and AoW (0xC0): a new GaItem entry is
// allocated. If GaItems is full, the function returns an error with no
// partial mutation — slot.Data and the in-memory list are unchanged.
// Talismans (0xA0) and goods (0xB0): no GaItem is allocated; only GaMap
// and the binary handle field are updated.
//
// GaMap gains an entry for the new handle. The old handle's GaMap entry
// is left in place — other records may still reference it.
//
// Scope: "inventory_common" | "inventory_key" | "storage_common".
func RehandleInventoryRecord(slot *SaveSlot, scope string, row int) (RehandleChange, error) {
	var zero RehandleChange
	if slot == nil {
		return zero, fmt.Errorf("RehandleInventoryRecord: nil slot")
	}

	items, entryStart, err := rehandleScopeItems(slot, scope)
	if err != nil {
		return zero, fmt.Errorf("RehandleInventoryRecord: %w", err)
	}
	if row < 0 || row >= len(items) {
		return zero, fmt.Errorf("RehandleInventoryRecord: row %d out of bounds (len=%d)", row, len(items))
	}
	oldHandle := items[row].GaItemHandle
	if oldHandle == GaHandleEmpty || oldHandle == GaHandleInvalid {
		return zero, fmt.Errorf("RehandleInventoryRecord: row %d has empty/invalid handle", row)
	}

	prefix := oldHandle & GaHandleTypeMask
	itemID, ok := slot.GaMap[oldHandle]
	if !ok {
		lower := oldHandle & 0x0FFFFFFF
		switch prefix {
		case ItemTypeAccessory:
			itemID = lower | 0x20000000
		case ItemTypeItem:
			itemID = lower | 0x40000000
		default:
			return zero, fmt.Errorf("RehandleInventoryRecord: itemID for handle 0x%08X not in GaMap", oldHandle)
		}
	}

	needsGaItem := prefix == ItemTypeWeapon || prefix == ItemTypeArmor || prefix == ItemTypeAow

	// Snapshot before the first mutation (generateUniqueHandle advances NextGaItemHandle).
	// Every error path after this point must call RestoreSlot before returning.
	snap := SnapshotSlot(slot)
	restore := func(cause error) (RehandleChange, error) {
		RestoreSlot(slot, snap)
		return zero, cause
	}

	newHandle, err := generateUniqueHandle(slot, prefix)
	if err != nil {
		return restore(fmt.Errorf("RehandleInventoryRecord: %w", err))
	}

	if needsGaItem {
		if err := allocateGaItem(slot, newHandle, itemID); err != nil {
			return restore(fmt.Errorf("RehandleInventoryRecord: %w", err))
		}
		slot.GaMap[newHandle] = itemID

		if err := rebuildAfterAllocation(slot); err != nil {
			return restore(fmt.Errorf("RehandleInventoryRecord: %w", err))
		}
		// Re-resolve after rebuild — offsets and in-memory slices refreshed.
		items, entryStart, err = rehandleScopeItems(slot, scope)
		if err != nil {
			return restore(fmt.Errorf("RehandleInventoryRecord: post-rebuild: %w", err))
		}
	} else {
		slot.GaMap[newHandle] = itemID
	}

	var writeOff int
	if scope == "storage_common" {
		writeOff, err = storageBinaryOff(slot, row, oldHandle)
		if err != nil {
			return restore(fmt.Errorf("RehandleInventoryRecord: %w", err))
		}
	} else {
		writeOff = entryStart + row*InvRecordLen
		if writeOff < 0 || writeOff+4 > len(slot.Data) {
			return restore(fmt.Errorf("RehandleInventoryRecord: %s row %d offset %d out of bounds", scope, row, writeOff))
		}
	}
	binary.LittleEndian.PutUint32(slot.Data[writeOff:], newHandle)
	items[row].GaItemHandle = newHandle

	return RehandleChange{
		Scope:     scope,
		Row:       row,
		OldHandle: oldHandle,
		NewHandle: newHandle,
		ItemID:    itemID,
	}, nil
}

// rehandleScopeItems returns the in-memory item slice and the binary base
// offset for the given scope. For "storage_common", the per-row byte offset
// must be resolved via storageBinaryOff (storage array is pre-allocated and
// the sparse in-memory index does not map 1:1 to binary slot index).
func rehandleScopeItems(slot *SaveSlot, scope string) ([]InventoryItem, int, error) {
	switch scope {
	case "inventory_common":
		return slot.Inventory.CommonItems, slot.MagicOffset + InvStartFromMagic, nil
	case "inventory_key":
		base := slot.MagicOffset + InvStartFromMagic + CommonItemCount*InvRecordLen + InvKeyCountHeader
		return slot.Inventory.KeyItems, base, nil
	case "storage_common":
		return slot.Storage.CommonItems, slot.StorageBoxOffset + StorageHeaderSkip, nil
	default:
		return nil, 0, fmt.Errorf("unknown scope %q", scope)
	}
}

// storageBinaryOff returns the byte offset of the row'th non-empty storage
// record in slot.Data and validates that it holds expectedHandle.
// Required because the sparse in-memory CommonItems index does not map 1:1
// to the binary pre-allocated slot index.
func storageBinaryOff(slot *SaveSlot, row int, expectedHandle uint32) (int, error) {
	start := slot.StorageBoxOffset + StorageHeaderSkip
	sparseIdx := 0
	for i := 0; i < StorageCommonCount; i++ {
		off := start + i*InvRecordLen
		if off+InvRecordLen > len(slot.Data) {
			break
		}
		h := binary.LittleEndian.Uint32(slot.Data[off:])
		if h == GaHandleEmpty || h == GaHandleInvalid {
			continue
		}
		if sparseIdx == row {
			if h != expectedHandle {
				return 0, fmt.Errorf("storage row %d: expected handle 0x%08X, got 0x%08X",
					row, expectedHandle, h)
			}
			return off, nil
		}
		sparseIdx++
	}
	return 0, fmt.Errorf("storage row %d not found", row)
}
