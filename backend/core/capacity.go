package core

import (
	"encoding/binary"
)

// SlotUsage holds the count of used vs max slots for each container.
type SlotUsage struct {
	GaItemsUsed    int
	GaItemsMax     int
	GaItemDataUsed int
	GaItemDataMax  int
	InventoryUsed  int
	InventoryMax   int
	StorageUsed    int
	StorageMax     int
}

// CountSlotUsage counts used entries in all slot containers by scanning binary data.
func CountSlotUsage(slot *SaveSlot) SlotUsage {
	u := SlotUsage{
		GaItemsMax:    len(slot.GaItems),
		GaItemDataMax: GaItemDataMaxCount,
		InventoryMax:  CommonItemCount,
		StorageMax:    StorageCommonCount,
	}

	for _, g := range slot.GaItems {
		if !g.IsEmpty() {
			u.GaItemsUsed++
		}
	}

	if slot.GaItemDataOffset > 0 && slot.GaItemDataOffset+4 <= len(slot.Data) {
		count := int(binary.LittleEndian.Uint32(slot.Data[slot.GaItemDataOffset:]))
		if count >= 0 && count <= GaItemDataMaxCount {
			u.GaItemDataUsed = count
		}
	}

	for _, item := range slot.Inventory.CommonItems {
		if item.GaItemHandle != GaHandleEmpty && item.GaItemHandle != GaHandleInvalid {
			u.InventoryUsed++
		}
	}

	storageStart := slot.StorageBoxOffset + StorageHeaderSkip
	for i := 0; i < StorageCommonCount; i++ {
		off := storageStart + i*InvRecordLen
		if off+InvRecordLen > len(slot.Data) {
			break
		}
		h := binary.LittleEndian.Uint32(slot.Data[off:])
		if h != GaHandleEmpty && h != GaHandleInvalid {
			u.StorageUsed++
		}
	}

	return u
}

// ItemToAdd describes a single item intended for batch addition.
type ItemToAdd struct {
	ItemID         uint32
	InvQty         int
	StorageQty     int
	ForceStackable bool
	IsStackable    bool
}

// CapacityReport describes why items don't fit.
type CapacityReport struct {
	CanFitAll        bool
	CapHit           string // "" | "inventory_full" | "storage_full" | "gaitem_full" | "gaitemdata_full"
	FreeInv          int
	FreeStorage      int
	FreeGaItems      int
	FreeGaItemData   int
	NeededInv        int
	NeededStorage    int
	NeededGaItems    int
	NeededGaItemData int
}

// CheckAddCapacity verifies that ALL items can be added without exceeding any
// container limit. Returns a report indicating whether everything fits.
func CheckAddCapacity(slot *SaveSlot, items []ItemToAdd) CapacityReport {
	usage := CountSlotUsage(slot)

	report := CapacityReport{
		FreeInv:        usage.InventoryMax - usage.InventoryUsed,
		FreeStorage:    usage.StorageMax - usage.StorageUsed,
		FreeGaItems:    usage.GaItemsMax - usage.GaItemsUsed,
		FreeGaItemData: usage.GaItemDataMax - usage.GaItemDataUsed,
	}

	existingHandles := make(map[uint32]bool)
	for h := range slot.GaMap {
		existingHandles[h] = true
	}

	existingGaItemData := make(map[uint32]bool)
	if slot.GaItemDataOffset > 0 {
		off := slot.GaItemDataOffset
		sa := NewSlotAccessor(slot.Data)
		if sa.CheckBounds(off, GaItemDataArrayOff, "cap/header") == nil {
			count := int(binary.LittleEndian.Uint32(slot.Data[off:]))
			if count > 0 && count <= GaItemDataMaxCount {
				arrayBase := off + GaItemDataArrayOff
				for i := 0; i < count; i++ {
					entryOff := arrayBase + i*GaItemDataEntryLen
					if entryOff+4 > len(slot.Data) {
						break
					}
					id := binary.LittleEndian.Uint32(slot.Data[entryOff:])
					existingGaItemData[id] = true
				}
			}
		}
	}

	neededInvSlots := 0
	neededStorageSlots := 0
	neededGaItems := 0
	neededGaItemData := 0

	seenNewStackableInv := make(map[uint32]bool)
	seenNewStackableStorage := make(map[uint32]bool)

	for _, item := range items {
		isStackable := item.IsStackable || item.ForceStackable

		if isStackable {
			hasExistingHandle := false
			for _, id := range []uint32{item.ItemID} {
				testHandle := (id & 0x0FFFFFFF) | handlePrefixForStackable(id)
				if existingHandles[testHandle] {
					hasExistingHandle = true
					break
				}
			}

			if item.InvQty > 0 {
				if !hasExistingHandle {
					alreadyInInv := false
					for _, inv := range slot.Inventory.CommonItems {
						if inv.GaItemHandle != GaHandleEmpty && inv.GaItemHandle != GaHandleInvalid {
							testHandle := (item.ItemID & 0x0FFFFFFF) | handlePrefixForStackable(item.ItemID)
							if inv.GaItemHandle == testHandle {
								alreadyInInv = true
								break
							}
						}
					}
					if !alreadyInInv && !seenNewStackableInv[item.ItemID] {
						neededInvSlots++
						seenNewStackableInv[item.ItemID] = true
					}
				}
			}

			if item.StorageQty > 0 {
				alreadyInStorage := false
				storageStart := slot.StorageBoxOffset + StorageHeaderSkip
				testHandle := (item.ItemID & 0x0FFFFFFF) | handlePrefixForStackable(item.ItemID)
				for i := 0; i < StorageCommonCount; i++ {
					off := storageStart + i*InvRecordLen
					if off+InvRecordLen > len(slot.Data) {
						break
					}
					h := binary.LittleEndian.Uint32(slot.Data[off:])
					if h == testHandle {
						alreadyInStorage = true
						break
					}
				}
				if !alreadyInStorage && !seenNewStackableStorage[item.ItemID] {
					neededStorageSlots++
					seenNewStackableStorage[item.ItemID] = true
				}
			}
		} else {
			// Non-stackable: each destination consumes its own GaItem record
			// because sharing a handle between inv and storage corrupts the
			// save (see writer.go::AddItemsToSlot non-stackable path).
			if item.InvQty > 0 {
				neededInvSlots++
				neededGaItems++
				if needsGaItemData(item.ItemID) && !existingGaItemData[item.ItemID] {
					neededGaItemData++
					existingGaItemData[item.ItemID] = true
				}
			}
			if item.StorageQty > 0 {
				neededStorageSlots++
				neededGaItems++
				if needsGaItemData(item.ItemID) && !existingGaItemData[item.ItemID] {
					neededGaItemData++
					existingGaItemData[item.ItemID] = true
				}
			}
		}
	}

	report.NeededInv = neededInvSlots
	report.NeededStorage = neededStorageSlots
	report.NeededGaItems = neededGaItems
	report.NeededGaItemData = neededGaItemData

	if neededGaItemData > report.FreeGaItemData {
		report.CapHit = "gaitemdata_full"
	} else if neededGaItems > report.FreeGaItems {
		report.CapHit = "gaitem_full"
	} else if neededInvSlots > report.FreeInv {
		report.CapHit = "inventory_full"
	} else if neededStorageSlots > report.FreeStorage {
		report.CapHit = "storage_full"
	}

	report.CanFitAll = report.CapHit == ""
	return report
}

func handlePrefixForStackable(itemID uint32) uint32 {
	switch itemID & 0xF0000000 {
	case 0x20000000:
		return ItemTypeAccessory
	case 0x40000000:
		return ItemTypeItem
	default:
		return ItemTypeItem
	}
}

func needsGaItemData(itemID uint32) bool {
	prefix := itemID & 0xF0000000
	return prefix == 0x00000000 || prefix == 0x80000000
}
