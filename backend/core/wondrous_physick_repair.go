package core

import (
	"encoding/binary"
	"fmt"

	"github.com/oisis/EldenRing-SaveForge/backend/db"
)

type WondrousPhysickOccurrence struct {
	Scope  string `json:"scope"`
	Row    int    `json:"row"`
	Handle uint32 `json:"handle"`
	ItemID uint32 `json:"itemId"`
}

func ScanDuplicateWondrousPhysick(slot *SaveSlot) []WondrousPhysickOccurrence {
	if slot == nil {
		return nil
	}
	var found []WondrousPhysickOccurrence
	for row, item := range slot.Inventory.CommonItems {
		if item.GaItemHandle == GaHandleEmpty || item.GaItemHandle == GaHandleInvalid {
			continue
		}
		itemID, ok := slot.GaMap[item.GaItemHandle]
		if !ok {
			itemID = db.HandleToItemID(item.GaItemHandle)
		}
		if db.IsWondrousPhysick(itemID) {
			found = append(found, WondrousPhysickOccurrence{
				Scope:  "inventory_common",
				Row:    row,
				Handle: item.GaItemHandle,
				ItemID: itemID,
			})
		}
	}
	if len(found) <= 1 {
		return nil
	}
	return found
}

func RepairDuplicateWondrousPhysick(slot *SaveSlot) (int, error) {
	occurrences := ScanDuplicateWondrousPhysick(slot)
	if len(occurrences) <= 1 {
		return 0, nil
	}

	keep := 0
	for i, occ := range occurrences {
		if occ.ItemID == db.ItemFlaskWondrousPhysickFilled {
			keep = i
			break
		}
	}

	commonStart := slot.MagicOffset + InvStartFromMagic
	if commonStart <= 0 {
		return 0, fmt.Errorf("RepairDuplicateWondrousPhysick: invalid common inventory offset %d", commonStart)
	}
	sa := NewSlotAccessor(slot.Data)
	removed := 0
	removedHandles := make([]uint32, 0, len(occurrences)-1)
	for i, occ := range occurrences {
		if i == keep {
			continue
		}
		if occ.Row < 0 || occ.Row >= len(slot.Inventory.CommonItems) {
			return removed, fmt.Errorf("RepairDuplicateWondrousPhysick: row %d out of range", occ.Row)
		}
		off := commonStart + occ.Row*InvRecordLen
		if err := sa.CheckBounds(off, InvRecordLen, "RepairDuplicateWondrousPhysick/common"); err != nil {
			return removed, err
		}
		slot.Inventory.CommonItems[occ.Row] = InventoryItem{GaItemHandle: 0, Quantity: 0, Index: uint32(occ.Row)}
		binary.LittleEndian.PutUint32(slot.Data[off:], 0)
		binary.LittleEndian.PutUint32(slot.Data[off+4:], 0)
		binary.LittleEndian.PutUint32(slot.Data[off+8:], uint32(occ.Row))
		removed++
		removedHandles = append(removedHandles, occ.Handle)
	}

	if removed > 0 {
		countOff := commonStart - 4
		if err := sa.CheckBounds(countOff, 4, "RepairDuplicateWondrousPhysick/inv-count"); err == nil {
			currentCount := binary.LittleEndian.Uint32(slot.Data[countOff:])
			if currentCount >= uint32(removed) {
				binary.LittleEndian.PutUint32(slot.Data[countOff:], currentCount-uint32(removed))
			}
		}
		for _, handle := range removedHandles {
			if !inventoryHandlePresent(slot, handle) {
				delete(slot.GaMap, handle)
				for i := range slot.GaItems {
					if slot.GaItems[i].Handle == handle {
						slot.GaItems[i] = GaItemFull{Unk2: -1, Unk3: -1, AoWGaItemHandle: 0xFFFFFFFF}
						break
					}
				}
			}
		}
	}

	return removed, nil
}

func inventoryHandlePresent(slot *SaveSlot, handle uint32) bool {
	for _, item := range slot.Inventory.CommonItems {
		if item.GaItemHandle == handle {
			return true
		}
	}
	for _, item := range slot.Inventory.KeyItems {
		if item.GaItemHandle == handle {
			return true
		}
	}
	for _, item := range slot.Storage.CommonItems {
		if item.GaItemHandle == handle {
			return true
		}
	}
	return false
}
