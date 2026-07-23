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
	// KeyItemsUsed/KeyItemsMax track Inventory.KeyItems separately from
	// Inventory.CommonItems: it is a distinct, independently-sized
	// (KeyItemCount) physical array (see addToKeyItems), not a subset of the
	// CommonItems budget. A native KeyItem add (addKindKeyItemStack) must be
	// checked against this limit, not InventoryUsed/InventoryMax.
	KeyItemsUsed int
	KeyItemsMax  int
}

// CountSlotUsage counts used entries in all slot containers by scanning binary data.
func CountSlotUsage(slot *SaveSlot) SlotUsage {
	u := SlotUsage{
		GaItemsMax:    len(slot.GaItems),
		GaItemDataMax: GaItemDataMaxCount,
		InventoryMax:  CommonItemCount,
		StorageMax:    StorageCommonCount,
		KeyItemsMax:   KeyItemCount,
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

	for _, item := range slot.Inventory.KeyItems {
		if item.GaItemHandle != GaHandleEmpty && item.GaItemHandle != GaHandleInvalid {
			u.KeyItemsUsed++
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

// ItemToAdd describes a single item intended for batch addition. How it consumes
// slot resources (merge vs per-copy records, GaItem or not) is derived from the
// item ID via classifyItemAdd — callers must not pre-classify it. ForceStackable
// is the one explicit override: it marks weapon-prefixed ammo (arrows) that stack.
type ItemToAdd struct {
	ItemID         uint32
	InvQty         int
	StorageQty     int
	ForceStackable bool
	// ForceCommonItems overrides nativeKeyItemFamily's native KeyItems routing
	// back to the classic CommonItems goods contract. See classifyItemAdd.
	ForceCommonItems bool
}

// CapacityReport describes why items don't fit.
type CapacityReport struct {
	CanFitAll   bool
	CapHit      string // "" | "inventory_full" | "storage_full" | "gaitem_full" | "gaitemdata_full"
	FreeInv     int
	FreeStorage int
	FreeGaItems int
	// FreeGaItemCursor is retained for API compatibility. The native hole
	// allocator has no monotonic cursor, so it mirrors FreeGaItems.
	FreeGaItemCursor int
	FreeGaItemData   int
	NeededInv        int
	NeededStorage    int
	NeededGaItems    int
	NeededGaItemData int
	// FreeKeyItems/NeededKeyItems track Inventory.KeyItems capacity
	// separately from FreeInv/NeededInv (CommonItems) — see
	// SlotUsage.KeyItemsUsed. Not exposed on the public AddResult contract:
	// a KeyItems shortfall still reports CapHit "inventory_full" (no new
	// UI-facing label), these two fields exist purely so the internal
	// CapHit decision can distinguish the two physical arrays correctly.
	FreeKeyItems   int
	NeededKeyItems int
}

// CheckAddCapacity verifies that ALL items can be added without exceeding any
// container limit. Returns a report indicating whether everything fits.
func CheckAddCapacity(slot *SaveSlot, items []ItemToAdd) CapacityReport {
	usage := CountSlotUsage(slot)

	freeGaItems := usage.GaItemsMax - usage.GaItemsUsed

	report := CapacityReport{
		FreeInv:          usage.InventoryMax - usage.InventoryUsed,
		FreeStorage:      usage.StorageMax - usage.StorageUsed,
		FreeGaItems:      freeGaItems,
		FreeGaItemCursor: freeGaItems,
		FreeGaItemData:   usage.GaItemDataMax - usage.GaItemDataUsed,
		FreeKeyItems:     usage.KeyItemsMax - usage.KeyItemsUsed,
	}

	existingHandles := make(map[uint32]bool)
	// existingItemIDs is a reverse index (item ID -> present) over the same
	// GaMap. Needed for addKindArrow: unlike goods/talismans, an arrow's
	// handle is counter-allocated by generateUniqueHandle, not a deterministic
	// function of the item ID, so it cannot be found by guessing
	// (id & mask) | prefix. This mirrors how the writer itself locates an
	// existing arrow stack (AddItemsToSlotBatch scans GaMap by value).
	existingItemIDs := make(map[uint32]bool, len(slot.GaMap))
	for h, id := range slot.GaMap {
		existingHandles[h] = true
		existingItemIDs[id] = true
	}

	// existingArrowInv / existingArrowStorage are PER-DESTINATION existence
	// indexes, keyed by item ID, built by resolving each physical record's
	// handle through GaMap. Arrows have real serialized GaItem records (T063),
	// so scanGaItems always populates GaMap for them on load — unlike goods/
	// talismans/Key Items. existingItemIDs alone is not enough here: an arrow
	// present in Inventory but not Storage (or vice versa) must still report a
	// needed record for the missing destination, even though the ID "exists"
	// somewhere. Fixes a preflight/writer mismatch where NeededStorage=0 was
	// reported for an Inventory-only arrow added to Storage, while
	// AddItemsToSlotBatch went on to create a real new Storage record.
	existingArrowInv := make(map[uint32]bool)
	for _, inv := range slot.Inventory.CommonItems {
		if inv.GaItemHandle == GaHandleEmpty || inv.GaItemHandle == GaHandleInvalid {
			continue
		}
		if id, ok := slot.GaMap[inv.GaItemHandle]; ok {
			existingArrowInv[id] = true
		}
	}
	existingArrowStorage := make(map[uint32]bool)
	{
		storageStart := slot.StorageBoxOffset + StorageHeaderSkip
		for i := 0; i < StorageCommonCount; i++ {
			off := storageStart + i*InvRecordLen
			if off+InvRecordLen > len(slot.Data) {
				break
			}
			h := binary.LittleEndian.Uint32(slot.Data[off:])
			if h == GaHandleEmpty || h == GaHandleInvalid {
				continue
			}
			if id, ok := slot.GaMap[h]; ok {
				existingArrowStorage[id] = true
			}
		}
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
					entryOff := arrayBase + i*GaItemDataActiveEntryLen
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
	neededKeyItemSlots := 0

	seenNewStackableInv := make(map[uint32]bool)
	seenNewStackableStorage := make(map[uint32]bool)
	seenNewArrowGaItem := make(map[uint32]bool)
	seenNewKeyItemInv := make(map[uint32]bool)

	for _, item := range items {
		kind := classifyItemAdd(item.ItemID, item.ForceStackable, item.ForceCommonItems)
		switch kind {
		case addKindTalisman:
			// Talismans: each copy is a distinct qty-1 physical record (see the
			// writer talisman path) and allocates NO serialized GaItem. Counting a
			// GaItem here caused false gaitem_full rejections; counting one slot for
			// N copies would risk a false accept that overflows inventory/storage.
			neededInvSlots += item.InvQty
			neededStorageSlots += item.StorageQty
			// T040: the FIRST physical copy of a new talisman ID also gets one
			// active GaItemData entry (flag 1); further copies of the same ID
			// (even across separate ItemToAdd entries in this batch) must not
			// double-count it — existingGaItemData is updated immediately below,
			// so the dedup is self-maintaining across the whole items loop.
			if (item.InvQty > 0 || item.StorageQty > 0) && !existingGaItemData[item.ItemID] {
				neededGaItemData++
				existingGaItemData[item.ItemID] = true
			}
			continue
		case addKindGaItem:
			// weapon/armor/AoW: each destination consumes its own GaItem record
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
			continue
		case addKindArrow:
			// Arrows/bolts share ONE handle across inv+storage like goods (see
			// writer.go's forceStackable branch), but that handle is
			// counter-allocated, not id-derived — hence the existingItemIDs
			// lookup instead of handlePrefixForStackable's formula. A genuinely
			// new ID additionally allocates one GaItem and (first time only)
			// one GaItemData entry; the writer used to skip GaItemData for
			// arrows entirely (T211), so this preflight must count exactly
			// what AddItemsToSlotBatch now writes.
			if stackableItemExistsInKeyItems(slot, item.ItemID) {
				continue
			}
			// hasExistingHandleAnywhere gates the GaItem/GaItemData need: the
			// writer allocates at most one of each per ID regardless of how
			// many destinations it lands in (see allocNewGaItem — it is called
			// once per new ID, not once per destination). Gated additionally by
			// seenNewArrowGaItem so a batch with two entries for the same new
			// ID (one per destination, or two entries for the same destination)
			// counts it only once — allocNewGaItem itself only runs on the
			// FIRST Phase 1 iteration that finds no existing handle.
			hasExistingHandleAnywhere := existingItemIDs[item.ItemID]
			if !hasExistingHandleAnywhere && (item.InvQty > 0 || item.StorageQty > 0) && !seenNewArrowGaItem[item.ItemID] {
				neededGaItems++
				seenNewArrowGaItem[item.ItemID] = true
				if needsGaItemData(item.ItemID) && !existingGaItemData[item.ItemID] {
					neededGaItemData++
					existingGaItemData[item.ItemID] = true
				}
			}
			// Inventory/Storage records are tracked PER DESTINATION: an arrow
			// already owned in Inventory but absent from Storage (or vice
			// versa) still needs a fresh record for the missing destination —
			// existingArrowInv/existingArrowStorage (built from real physical
			// scans, not existingItemIDs) capture that distinction that a
			// single "exists anywhere" bool cannot.
			if item.InvQty > 0 && !existingArrowInv[item.ItemID] && !seenNewStackableInv[item.ItemID] {
				neededInvSlots++
				seenNewStackableInv[item.ItemID] = true
			}
			if item.StorageQty > 0 && !existingArrowStorage[item.ItemID] && !seenNewStackableStorage[item.ItemID] {
				neededStorageSlots++
				seenNewStackableStorage[item.ItemID] = true
			}
			continue
		case addKindKeyItemStack:
			// T070/T071/T074/T090: Crafting Kit, cookbooks, Cracked Pot, and the
			// Physick package's Crimson Crystal Tear variant — same fungible,
			// handle-encoded, no-serialized-GaItem contract as addKindStack, but
			// the native destination is Inventory.KeyItems, not CommonItems —
			// a separate, independently-sized physical array (KeyItemCount),
			// so a genuinely new record must consume neededKeyItemSlots /
			// FreeKeyItems, never neededInvSlots / FreeInv (see writer.go's
			// addToKeyItems and the "legacy compat" case just below, where a
			// pre-existing CommonItems record is instead bumped in place and
			// consumes no new slot in EITHER budget).
			//
			// The existence scan checks both containers so a pre-existing
			// record in EITHER one (native KeyItems, or a legacy CommonItems
			// placement from a pre-fix save — see writer.go's
			// legacyInCommonItems) is recognized and a fresh add never
			// double-counts a slot in the wrong budget.
			testHandle := (item.ItemID & 0x0FFFFFFF) | handlePrefixForStackable(item.ItemID)
			hasExistingHandle := existingHandles[testHandle]

			if item.InvQty > 0 && !hasExistingHandle {
				legacyInCommonItems := false
				for _, inv := range slot.Inventory.CommonItems {
					if inv.GaItemHandle == testHandle {
						legacyInCommonItems = true
						break
					}
				}
				if legacyInCommonItems {
					// Mirrors writer.go: a legacy CommonItems record is bumped
					// in place there, not created anew — no slot consumed in
					// either budget.
				} else {
					alreadyInKeyItems := false
					for _, inv := range slot.Inventory.KeyItems {
						if inv.GaItemHandle == testHandle {
							alreadyInKeyItems = true
							break
						}
					}
					if !alreadyInKeyItems && !seenNewKeyItemInv[item.ItemID] {
						neededKeyItemSlots++
						seenNewKeyItemInv[item.ItemID] = true
					}
				}
			}

			if item.StorageQty > 0 {
				alreadyInStorage := false
				storageStart := slot.StorageBoxOffset + StorageHeaderSkip
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

			if (item.InvQty > 0 || item.StorageQty > 0) && !existingGaItemData[item.ItemID] {
				neededGaItemData++
				existingGaItemData[item.ItemID] = true
			}
			continue
		}

		// addKindStack: fungible goods — at most one physical record per
		// destination, no serialized GaItem (handle-encoded, id-derived).
		{
			if stackableItemExistsInKeyItems(slot, item.ItemID) {
				continue
			}
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

			// T050/T060/T062: the FIRST physical record of a new goods/
			// crafting-material/bolstering-material stack (either destination)
			// also gets one active GaItemData entry with flag 1; a quantity
			// bump to an already-existing stack must not add a second one.
			// existingGaItemData reflects real on-disk state, so an item that
			// already has a physical record here will already be marked here
			// too — this only fires for a genuinely new ID.
			if (item.InvQty > 0 || item.StorageQty > 0) && !existingGaItemData[item.ItemID] {
				neededGaItemData++
				existingGaItemData[item.ItemID] = true
			}
		}
	}

	report.NeededInv = neededInvSlots
	report.NeededStorage = neededStorageSlots
	report.NeededGaItems = neededGaItems
	report.NeededGaItemData = neededGaItemData
	report.NeededKeyItems = neededKeyItemSlots

	if neededGaItemData > report.FreeGaItemData {
		report.CapHit = "gaitemdata_full"
	} else if neededGaItems > report.FreeGaItems {
		report.CapHit = "gaitem_full"
	} else if neededInvSlots > report.FreeInv {
		report.CapHit = "inventory_full"
	} else if neededKeyItemSlots > report.FreeKeyItems {
		// A full Inventory.KeyItems array has no dedicated public CapHit label
		// (see CapacityReport.FreeKeyItems doc) — reuse "inventory_full" so the
		// existing UI contract needs no new case, matching how a full
		// CommonItems array is reported.
		report.CapHit = "inventory_full"
	} else if neededStorageSlots > report.FreeStorage {
		report.CapHit = "storage_full"
	}

	report.CanFitAll = report.CapHit == ""
	return report
}

// handlePrefixForStackable returns the handle prefix for a deterministic,
// id-derived stackable handle (goods). It is never called for addKindArrow —
// arrow handles are counter-allocated, not id-derived (see the addKindArrow
// switch case in CheckAddCapacity).
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
	// 0x00000000 weapon, 0x10000000 armor (T020: Chain Coif/Chain Armor both
	// get an active GaItemData entry, same as weapons), 0x80000000 AoW.
	return prefix == 0x00000000 || prefix == 0x10000000 || prefix == 0x80000000
}
