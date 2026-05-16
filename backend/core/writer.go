package core

import (
	"encoding/binary"
	"fmt"
	"io"

	"github.com/oisis/EldenRing-SaveForge/backend/db"
)

// upsertGaItemData ensures itemID is present in the GaitemGameData section.
// The game looks up weapon/AoW properties from this list on load. Adding a
// weapon without a corresponding entry causes EXCEPTION_ACCESS_VIOLATION.
//
// Layout (at slot.GaItemDataOffset = start of GaitemGameData):
//
//	[0:8]  count (i64) — number of valid entries
//	[8+]   GaitemGameDataEntry array: 7000 × 16 bytes
//	        per entry: id(4) + unk0x4(1) + pad(3) + nextItemID(4) + unk0xc(1) + pad(3)
func upsertGaItemData(slot *SaveSlot, itemID uint32) error {
	off := slot.GaItemDataOffset
	if off <= 0 {
		return nil
	}
	sa := NewSlotAccessor(slot.Data)

	if err := sa.CheckBounds(off, GaItemDataArrayOff, "upsertGaItemData/header"); err != nil {
		return fmt.Errorf("upsertGaItemData: header bounds check failed: %w", err)
	}
	count := int(int32(binary.LittleEndian.Uint32(slot.Data[off:])))
	if count < 0 || count >= GaItemDataMaxCount {
		return fmt.Errorf("upsertGaItemData: count %d out of range [0, %d)", count, GaItemDataMaxCount)
	}

	arrayBase := off + GaItemDataArrayOff
	for i := 0; i < count; i++ {
		entryOff := arrayBase + i*GaItemDataEntryLen
		if err := sa.CheckBounds(entryOff, 4, "upsertGaItemData/scan"); err != nil {
			return fmt.Errorf("upsertGaItemData: scan bounds check at entry %d: %w", i, err)
		}
		if binary.LittleEndian.Uint32(slot.Data[entryOff:]) == itemID {
			return nil
		}
	}

	newEntryOff := arrayBase + count*GaItemDataEntryLen
	if err := sa.CheckBounds(newEntryOff, GaItemDataEntryLen, "upsertGaItemData/write"); err != nil {
		return fmt.Errorf("upsertGaItemData: write bounds check at count %d: %w", count, err)
	}
	binary.LittleEndian.PutUint32(slot.Data[newEntryOff:], itemID)
	slot.Data[newEntryOff+4] = 1
	slot.Data[newEntryOff+5] = 0
	slot.Data[newEntryOff+6] = 0
	slot.Data[newEntryOff+7] = 0
	binary.LittleEndian.PutUint32(slot.Data[newEntryOff+8:], itemID)
	slot.Data[newEntryOff+12] = 1
	slot.Data[newEntryOff+13] = 0
	slot.Data[newEntryOff+14] = 0
	slot.Data[newEntryOff+15] = 0

	binary.LittleEndian.PutUint32(slot.Data[off:], uint32(count+1))

	return nil
}

type Writer struct {
	w io.Writer
}

func NewWriter(w io.Writer) *Writer {
	return &Writer{w: w}
}

func (w *Writer) WriteU8(v uint8) error {
	return binary.Write(w.w, binary.LittleEndian, v)
}

func (w *Writer) WriteU16(v uint16) error {
	return binary.Write(w.w, binary.LittleEndian, v)
}

func (w *Writer) WriteU32(v uint32) error {
	return binary.Write(w.w, binary.LittleEndian, v)
}

func (w *Writer) WriteI32(v int32) error {
	return binary.Write(w.w, binary.LittleEndian, v)
}

func (w *Writer) WriteU64(v uint64) error {
	return binary.Write(w.w, binary.LittleEndian, v)
}

func (w *Writer) WriteBytes(v []byte) error {
	_, err := w.w.Write(v)
	return err
}

// AddItemsToSlot adds multiple items to a specific save slot.
// invQty and storageQty control quantities: 0 = skip, -1 = use provided max from caller, >0 = exact qty.
// forceStackable treats items as stackable (reuse existing GaMap handle) regardless of type.
// Used for arrows/bolts which have weapon-like IDs but are stackable in inventory.
//
// Algorithm (Plan D — GaItems Section Re-serialization):
//
//	Phase 1: Allocate GaItem entries in-memory array + write GaItemData at old offsets.
//	Phase 2: FlushGaItems — serialize array, compute size delta, single shift, update offsets.
//	Phase 3: Add to inventory/storage (offsets now correct after flush).
func AddItemsToSlot(slot *SaveSlot, itemIDs []uint32, invQty, storageQty int, forceStackable bool) error {
	type pendingInv struct {
		handle     uint32
		invQty     uint32
		storageQty uint32
	}
	var pending []pendingInv
	gaModified := false

	// allocNewGaItem allocates a new GaItem record + handle for a non-stackable
	// item, registers it in GaMap, and updates GaItemData when needed. Returns
	// the new handle.
	allocNewGaItem := func(id, handlePrefix uint32) (uint32, error) {
		h, err := generateUniqueHandle(slot, handlePrefix)
		if err != nil {
			return 0, err
		}
		if err := allocateGaItem(slot, h, id); err != nil {
			return 0, err
		}
		slot.GaMap[h] = id
		if (handlePrefix == ItemTypeWeapon && !db.IsArrowID(id)) || handlePrefix == ItemTypeAow {
			if err := upsertGaItemData(slot, id); err != nil {
				return 0, err
			}
		}
		return h, nil
	}

	// Phase 1: allocate GaItem entries in-memory + GaItemData at old offsets.
	for _, id := range itemIDs {
		handlePrefix := db.ItemIDToHandlePrefix(id)
		isStackable := handlePrefix == ItemTypeItem || handlePrefix == ItemTypeAccessory

		if isStackable || forceStackable {
			// Stackable / arrow path: shared handle across inv+storage is correct
			// because the game resolves stackable handles directly (no GaItem
			// record) and treats stacks of the same item ID as fungible.
			handle := uint32(0)
			for h, i := range slot.GaMap {
				if i == id {
					handle = h
					break
				}
			}
			if handle == 0 {
				if isStackable {
					handle = (id & 0x0FFFFFFF) | handlePrefix
					slot.GaMap[handle] = id
				} else {
					// forceStackable arrow without existing GaItem — allocate one.
					var err error
					handle, err = allocNewGaItem(id, handlePrefix)
					if err != nil {
						return err
					}
					gaModified = true
				}
			}
			pending = append(pending, pendingInv{
				handle:     handle,
				invQty:     uint32(invQty),
				storageQty: uint32(storageQty),
			})
			continue
		}

		// Non-stackable path (weapon / armor / AoW): each destination requires
		// its own GaItem record. Sharing one handle between inventory and
		// storage causes the game to treat both list entries as the same
		// physical item — equipping an AoW on one weapon then propagates to the
		// duplicate list entry, and on next save cycle the game collides them
		// (observed: same handle in inventory at two positions, items pushed to
		// invalid Index >= next_equip_index, becoming invisible in-game).
		if invQty != 0 {
			h, err := allocNewGaItem(id, handlePrefix)
			if err != nil {
				return err
			}
			gaModified = true
			pending = append(pending, pendingInv{handle: h, invQty: uint32(invQty), storageQty: 0})
		}
		if storageQty != 0 {
			h, err := allocNewGaItem(id, handlePrefix)
			if err != nil {
				return err
			}
			gaModified = true
			pending = append(pending, pendingInv{handle: h, invQty: 0, storageQty: uint32(storageQty)})
		}
	}

	// Phase 2: rebuild slot from scratch with new GaItems (no in-place shift).
	// FlushGaItems used to shift slot.Data right by `delta` bytes, which
	// overwrote the last `delta` bytes of the slot (DLC + Hash regions) when
	// delta > 0x132. RebuildSlotFull writes a fresh SlotSize buffer with DLC
	// and Hash placed via typed Write paths at their correct fixed positions.
	if gaModified {
		// Snapshot GaMap before rebuild — parseFromData rebuilds it from GaItem
		// records only via scanGaItems(), dropping stackable handles (talismans,
		// goods) which aren't backed by GaItem records. We re-merge them after.
		savedGaMap := make(map[uint32]uint32, len(slot.GaMap))
		for k, v := range slot.GaMap {
			savedGaMap[k] = v
		}
		// Snapshot type-segregation indices — same reason: parseFromData re-scans
		// these from GaItem records, but Phase 3 / future calls need the values
		// we computed in Phase 1 (after allocateGaItem mutations).
		savedNextAoW := slot.NextAoWIndex
		savedNextArmament := slot.NextArmamentIndex
		savedNextHandle := slot.NextGaItemHandle

		rebuilt, err := RebuildSlotFull(slot)
		if err != nil {
			return fmt.Errorf("AddItemsToSlot: rebuild: %w", err)
		}
		copy(slot.Data, rebuilt)
		// Re-parse all derived state from the new layout. parseFromData mirrors
		// the post-ReadBytes flow of slot.Read — re-finds MagicPattern (whose
		// absolute position shifted with GaItems growth), re-scans GaItems,
		// recomputes dynamic offsets, and refreshes inventory counter offsets
		// that Phase 3 will write to.
		if err := slot.parseFromData(); err != nil {
			return fmt.Errorf("AddItemsToSlot: re-parse after rebuild: %w", err)
		}

		// Re-merge stackable handles dropped by GaMap rebuild.
		for h, id := range savedGaMap {
			if _, ok := slot.GaMap[h]; !ok {
				slot.GaMap[h] = id
			}
		}
		// Restore tracked indices that Phase 1 advanced past the rescanned values.
		if savedNextAoW > slot.NextAoWIndex {
			slot.NextAoWIndex = savedNextAoW
		}
		if savedNextArmament > slot.NextArmamentIndex {
			slot.NextArmamentIndex = savedNextArmament
		}
		if savedNextHandle > slot.NextGaItemHandle {
			slot.NextGaItemHandle = savedNextHandle
		}
	}

	// Phase 3: add to inventory/storage (offsets are now correct).
	for _, p := range pending {
		if p.invQty != 0 {
			if err := addToInventory(slot, p.handle, p.invQty, false); err != nil {
				return err
			}
		}
		if p.storageQty != 0 {
			if err := addToInventory(slot, p.handle, p.storageQty, true); err != nil {
				return err
			}
		}
	}

	return nil
}

// AddItemsToSlotBatch adds a batch of items with per-item qty/stackable settings.
// All GaItem allocations happen in Phase 1, then ONE RebuildSlotFull in Phase 2,
// then all inventory/storage writes in Phase 3. This is O(1) rebuilds instead of O(N).
func AddItemsToSlotBatch(slot *SaveSlot, items []ItemToAdd) error {
	type pendingInv struct {
		handle     uint32
		invQty     uint32
		storageQty uint32
	}
	var pending []pendingInv
	gaModified := false

	allocNewGaItem := func(id, handlePrefix uint32) (uint32, error) {
		h, err := generateUniqueHandle(slot, handlePrefix)
		if err != nil {
			return 0, err
		}
		if err := allocateGaItem(slot, h, id); err != nil {
			return 0, err
		}
		slot.GaMap[h] = id
		if (handlePrefix == ItemTypeWeapon && !db.IsArrowID(id)) || handlePrefix == ItemTypeAow {
			if err := upsertGaItemData(slot, id); err != nil {
				return 0, err
			}
		}
		return h, nil
	}

	for _, item := range items {
		handlePrefix := db.ItemIDToHandlePrefix(item.ItemID)
		isStackable := handlePrefix == ItemTypeItem || handlePrefix == ItemTypeAccessory

		if isStackable || item.ForceStackable {
			handle := uint32(0)
			for h, id := range slot.GaMap {
				if id == item.ItemID {
					handle = h
					break
				}
			}
			if handle == 0 {
				if isStackable {
					handle = (item.ItemID & 0x0FFFFFFF) | handlePrefix
					slot.GaMap[handle] = item.ItemID
				} else {
					var err error
					handle, err = allocNewGaItem(item.ItemID, handlePrefix)
					if err != nil {
						return err
					}
					gaModified = true
				}
			}
			pending = append(pending, pendingInv{
				handle:     handle,
				invQty:     uint32(item.InvQty),
				storageQty: uint32(item.StorageQty),
			})
			continue
		}

		// Non-stackable: separate GaItem per destination — see AddItemsToSlot
		// for the explanation of why sharing a handle corrupts the save.
		if item.InvQty != 0 {
			h, err := allocNewGaItem(item.ItemID, handlePrefix)
			if err != nil {
				return err
			}
			gaModified = true
			pending = append(pending, pendingInv{handle: h, invQty: uint32(item.InvQty), storageQty: 0})
		}
		if item.StorageQty != 0 {
			h, err := allocNewGaItem(item.ItemID, handlePrefix)
			if err != nil {
				return err
			}
			gaModified = true
			pending = append(pending, pendingInv{handle: h, invQty: 0, storageQty: uint32(item.StorageQty)})
		}
	}

	if gaModified {
		savedGaMap := make(map[uint32]uint32, len(slot.GaMap))
		for k, v := range slot.GaMap {
			savedGaMap[k] = v
		}
		savedNextAoW := slot.NextAoWIndex
		savedNextArmament := slot.NextArmamentIndex
		savedNextHandle := slot.NextGaItemHandle

		rebuilt, err := RebuildSlotFull(slot)
		if err != nil {
			return fmt.Errorf("AddItemsToSlotBatch: rebuild: %w", err)
		}
		copy(slot.Data, rebuilt)
		if err := slot.parseFromData(); err != nil {
			return fmt.Errorf("AddItemsToSlotBatch: re-parse after rebuild: %w", err)
		}

		for h, id := range savedGaMap {
			if _, ok := slot.GaMap[h]; !ok {
				slot.GaMap[h] = id
			}
		}
		if savedNextAoW > slot.NextAoWIndex {
			slot.NextAoWIndex = savedNextAoW
		}
		if savedNextArmament > slot.NextArmamentIndex {
			slot.NextArmamentIndex = savedNextArmament
		}
		if savedNextHandle > slot.NextGaItemHandle {
			slot.NextGaItemHandle = savedNextHandle
		}
	}

	for _, p := range pending {
		if p.invQty != 0 {
			if err := addToInventory(slot, p.handle, p.invQty, false); err != nil {
				return err
			}
		}
		if p.storageQty != 0 {
			if err := addToInventory(slot, p.handle, p.storageQty, true); err != nil {
				return err
			}
		}
	}

	return nil
}

func generateUniqueHandle(slot *SaveSlot, prefix uint32) (uint32, error) {
	// Use global counter from tracked indices (matching Rust ER-Save-Editor).
	// Handle format: 0xTTPPCCCC where TT=type, PP=part_id, CCCC=global counter.
	// The game regenerates all handles on load, so the counter just needs to be unique.
	partID := uint32(slot.PartGaItemHandle)
	counter := slot.NextGaItemHandle

	h := prefix | (partID << 16) | counter
	for i := 0; i < MaxHandleAttempts; i++ {
		if _, ok := slot.GaMap[h]; !ok {
			slot.NextGaItemHandle = counter + 1
			return h, nil
		}
		counter++
		h = prefix | (partID << 16) | counter
	}
	return 0, fmt.Errorf("failed to generate unique handle after %d attempts (prefix 0x%X)",
		MaxHandleAttempts, prefix)
}

// allocateGaItem places a new item in the GaItems array at the correct
// type-segregated position. The game expects AoW entries at low indices,
// armor/weapons at higher indices. Matching Rust ER-Save-Editor's add_gaitem().
//
// AoW → placed at slot.NextAoWIndex, then NextAoWIndex++ and NextArmamentIndex++
// Weapon/Armor → placed at slot.NextArmamentIndex, then NextArmamentIndex++
//
// Does NOT write to binary data — call FlushGaItems after all allocations.
func allocateGaItem(slot *SaveSlot, handle, itemID uint32) error {
	handleType := handle & GaHandleTypeMask
	isAoW := handleType == ItemTypeAow

	entry := GaItemFull{
		Handle:          handle,
		ItemID:          itemID,
		Unk2:            -1,
		Unk3:            -1,
		AoWGaItemHandle: 0xFFFFFFFF,
		Unk5:            0,
	}

	maxEntries := len(slot.GaItems)

	if isAoW {
		idx := slot.NextAoWIndex
		if idx >= maxEntries {
			return fmt.Errorf("allocateGaItem: AoW array full (index %d >= %d)", idx, maxEntries)
		}
		// If position is occupied, shift entries right to make room
		if !slot.GaItems[idx].IsEmpty() {
			// Find the end of used entries to shift into
			shiftEnd := idx
			for shiftEnd < maxEntries && !slot.GaItems[shiftEnd].IsEmpty() {
				shiftEnd++
			}
			if shiftEnd >= maxEntries {
				return fmt.Errorf("allocateGaItem: no room to insert AoW at index %d", idx)
			}
			// Shift right by 1
			copy(slot.GaItems[idx+1:shiftEnd+1], slot.GaItems[idx:shiftEnd])
		}
		slot.GaItems[idx] = entry
		slot.NextAoWIndex++
		slot.NextArmamentIndex++ // AoW insertion shifts armament zone right
	} else {
		idx := slot.NextArmamentIndex
		if idx >= maxEntries {
			return fmt.Errorf("allocateGaItem: armament/armor array full (index %d >= %d)", idx, maxEntries)
		}
		// If position is occupied, shift entries right to make room
		if !slot.GaItems[idx].IsEmpty() {
			shiftEnd := idx
			for shiftEnd < maxEntries && !slot.GaItems[shiftEnd].IsEmpty() {
				shiftEnd++
			}
			if shiftEnd >= maxEntries {
				return fmt.Errorf("allocateGaItem: no room to insert weapon/armor at index %d", idx)
			}
			copy(slot.GaItems[idx+1:shiftEnd+1], slot.GaItems[idx:shiftEnd])
		}
		slot.GaItems[idx] = entry
		slot.NextArmamentIndex++
	}

	return nil
}

// FlushGaItems serializes the entire in-memory GaItems array back to slot.Data.
// If the total byte size changed (e.g. empty 8B slot replaced by 21B weapon),
// shifts all data after the GaItems section and updates all downstream offsets.
//
// Deprecated: this function uses an in-place data shift that overwrites the
// last `delta` bytes of slot.Data — including DLC section (50 B at SlotSize-0xB2)
// and PlayerGameDataHash (128 B at SlotSize-0x80) — whenever delta > 0x132.
// Use RebuildSlotFull (called from AddItemsToSlot Phase 2) instead. Kept for
// backward compatibility with any external callers; will be removed once no
// callers remain. See CHANGELOG entry for the FlushGaItems DLC+Hash overwrite
// post-mortem.
func FlushGaItems(slot *SaveSlot) error {
	// 1. Serialize all GaItem entries into a temporary buffer.
	// Max possible size: all entries as weapons (21B each).
	maxBuf := len(slot.GaItems) * GaRecordWeapon
	buf := make([]byte, maxBuf)
	pos := 0
	for i := range slot.GaItems {
		n := slot.GaItems[i].Serialize(buf[pos:])
		pos += n
	}
	newGaBytes := buf[:pos]
	newGaSize := len(newGaBytes)

	// 2. Compute old section size and delta.
	oldGaLimit := slot.MagicOffset - DynPlayerData + 1
	if oldGaLimit < GaItemsStart {
		oldGaLimit = GaItemsStart
	}
	oldGaSize := oldGaLimit - GaItemsStart
	delta := newGaSize - oldGaSize

	// 3. Safety check: new section must fit within slot.
	if GaItemsStart+newGaSize+DynPlayerData > SlotSize {
		return fmt.Errorf("FlushGaItems: section too large (%d bytes, max %d)",
			newGaSize, SlotSize-GaItemsStart-DynPlayerData)
	}

	// 4. Shift data after GaItems section if size changed.
	if delta != 0 {
		if delta > 0 {
			// Section grew — shift right (loses trailing padding bytes at end of slot).
			copy(slot.Data[oldGaLimit+delta:SlotSize], slot.Data[oldGaLimit:SlotSize-delta])
		} else {
			// Section shrank — shift left (frees bytes at end of slot).
			absDelta := -delta
			copy(slot.Data[oldGaLimit-absDelta:SlotSize-absDelta], slot.Data[oldGaLimit:SlotSize])
			// Zero the freed space at the end.
			for i := SlotSize - absDelta; i < SlotSize; i++ {
				slot.Data[i] = 0
			}
		}

		// Update ALL downstream offsets by delta (single update for entire batch).
		slot.MagicOffset += delta
		slot.PlayerDataOffset += delta
		slot.FaceDataOffset += delta
		slot.StorageBoxOffset += delta
		slot.GaItemDataOffset += delta
		if slot.TutorialDataOffset > 0 {
			slot.TutorialDataOffset += delta
		}
		slot.IngameTimerOffset += delta
		if slot.EventFlagsOffset > 0 {
			slot.EventFlagsOffset += delta
		}
		if slot.Inventory.nextEquipIndexOff >= oldGaLimit {
			slot.Inventory.nextEquipIndexOff += delta
		}
		if slot.Inventory.nextAcqSortIdOff >= oldGaLimit {
			slot.Inventory.nextAcqSortIdOff += delta
		}
		if slot.Storage.nextEquipIndexOff >= oldGaLimit {
			slot.Storage.nextEquipIndexOff += delta
		}
		if slot.Storage.nextAcqSortIdOff >= oldGaLimit {
			slot.Storage.nextAcqSortIdOff += delta
		}
	}

	// 5. Write serialized GaItems into slot.Data (covers [GaItemsStart, GaItemsStart+newGaSize)).
	copy(slot.Data[GaItemsStart:], newGaBytes)

	// 6. Update InventoryEnd to match the new section end.
	slot.InventoryEnd = GaItemsStart + newGaSize

	return nil
}

// RemoveItemFromSlot zeroes out inventory/storage slots for the given handle.
// Inventory: fixed pre-allocated array — zero the matching slot(s).
// Storage: dynamic list — zero the matching slot(s); game stops reading at handle==0.
// GaMap entry is removed only when the handle is absent from both lists after removal.
func RemoveItemFromSlot(slot *SaveSlot, handle uint32, fromInventory, fromStorage bool) error {
	sa := NewSlotAccessor(slot.Data)

	if fromInventory {
		invStart := slot.MagicOffset + InvStartFromMagic
		removedFromInv := 0
		for i, item := range slot.Inventory.CommonItems {
			if item.GaItemHandle == handle {
				slot.Inventory.CommonItems[i] = InventoryItem{GaItemHandle: 0, Quantity: 0, Index: uint32(i)}
				off := invStart + i*InvRecordLen
				if err := sa.CheckBounds(off, InvRecordLen, "RemoveItemFromSlot/common"); err != nil {
					return err
				}
				binary.LittleEndian.PutUint32(slot.Data[off:], 0)
				binary.LittleEndian.PutUint32(slot.Data[off+4:], 0)
				binary.LittleEndian.PutUint32(slot.Data[off+8:], uint32(i))
				removedFromInv++
			}
		}
		for i, item := range slot.Inventory.KeyItems {
			if item.GaItemHandle == handle {
				keyStart := invStart + CommonItemCount*InvRecordLen
				slot.Inventory.KeyItems[i] = InventoryItem{GaItemHandle: 0, Quantity: 0, Index: uint32(i)}
				off := keyStart + i*InvRecordLen
				if err := sa.CheckBounds(off, InvRecordLen, "RemoveItemFromSlot/key"); err != nil {
					return err
				}
				binary.LittleEndian.PutUint32(slot.Data[off:], 0)
				binary.LittleEndian.PutUint32(slot.Data[off+4:], 0)
				binary.LittleEndian.PutUint32(slot.Data[off+8:], uint32(i))
			}
		}
		// Decrement common_item_count header at invStart-4 (mirrors the increment in addToInventory).
		if removedFromInv > 0 {
			countOff := invStart - 4
			if err := sa.CheckBounds(countOff, 4, "RemoveItemFromSlot/inv-count"); err == nil {
				currentCount := binary.LittleEndian.Uint32(slot.Data[countOff:])
				if currentCount >= uint32(removedFromInv) {
					binary.LittleEndian.PutUint32(slot.Data[countOff:], currentCount-uint32(removedFromInv))
				}
			}
		}
	}
	if fromStorage {
		// Scan the physical pre-allocated storage array (1920 slots) to find and zero the handle.
		// Cannot use in-memory list index because ReadStorage skips empty slots (sparse).
		storageStart := slot.StorageBoxOffset + StorageHeaderSkip
		removed := 0
		for i := 0; i < StorageCommonCount; i++ {
			off := storageStart + i*InvRecordLen
			if err := sa.CheckBounds(off, InvRecordLen, "RemoveItemFromSlot/storage"); err != nil {
				break
			}
			h := binary.LittleEndian.Uint32(slot.Data[off:])
			if h == handle {
				binary.LittleEndian.PutUint32(slot.Data[off:], 0)
				binary.LittleEndian.PutUint32(slot.Data[off+4:], 0)
				binary.LittleEndian.PutUint32(slot.Data[off+8:], 0)
				removed++
			}
		}
		// Decrement common_inventory_items_distinct_count header
		if removed > 0 {
			countOff := slot.StorageBoxOffset
			if err := sa.CheckBounds(countOff, 4, "RemoveItemFromSlot/storage-count"); err == nil {
				currentCount := binary.LittleEndian.Uint32(slot.Data[countOff:])
				if currentCount >= uint32(removed) {
					binary.LittleEndian.PutUint32(slot.Data[countOff:], currentCount-uint32(removed))
				}
			}
		}
		// Update in-memory list
		for i, item := range slot.Storage.CommonItems {
			if item.GaItemHandle == handle {
				slot.Storage.CommonItems[i] = InventoryItem{GaItemHandle: 0, Quantity: 0, Index: 0}
			}
		}
	}
	// Remove from GaMap only if the handle is now absent from both lists.
	stillPresent := false
	for _, item := range slot.Inventory.CommonItems {
		if item.GaItemHandle == handle {
			stillPresent = true
			break
		}
	}
	if !stillPresent {
		for _, item := range slot.Inventory.KeyItems {
			if item.GaItemHandle == handle {
				stillPresent = true
				break
			}
		}
	}
	if !stillPresent {
		for _, item := range slot.Storage.CommonItems {
			if item.GaItemHandle == handle {
				stillPresent = true
				break
			}
		}
	}
	if !stillPresent {
		delete(slot.GaMap, handle)
		// Clear the GaItem in-memory entry so the next RebuildSlotFull doesn't
		// re-serialize it. Without this, scanGaItems() on re-parse re-adds the
		// handle to GaMap as an "orphaned" entry, accumulating over sessions.
		for i := range slot.GaItems {
			if slot.GaItems[i].Handle == handle {
				slot.GaItems[i] = GaItemFull{Unk2: -1, Unk3: -1, AoWGaItemHandle: 0xFFFFFFFF}
				break
			}
		}
	}
	return nil
}

// RemoveItemByBaseID removes an item from inventory by its base item ID (e.g. 0x40002198).
// For stackable items (tools, key items), GaItemHandle == item ID directly.
func RemoveItemByBaseID(slot *SaveSlot, itemID uint32) {
	// Find the handle in inventory (for stackable items, handle == itemID)
	for _, item := range slot.Inventory.CommonItems {
		if item.GaItemHandle == itemID && item.Quantity > 0 {
			_ = RemoveItemFromSlot(slot, itemID, true, false)
			return
		}
	}
	for _, item := range slot.Inventory.KeyItems {
		if item.GaItemHandle == itemID && item.Quantity > 0 {
			_ = RemoveItemFromSlot(slot, itemID, true, false)
			return
		}
	}
}

func addToInventory(slot *SaveSlot, handle uint32, qty uint32, isStorage bool) error {
	sa := NewSlotAccessor(slot.Data)
	var items *[]InventoryItem
	var startOffset int

	if isStorage {
		items = &slot.Storage.CommonItems
		startOffset = slot.StorageBoxOffset + StorageHeaderSkip
	} else {
		items = &slot.Inventory.CommonItems
		startOffset = slot.MagicOffset + InvStartFromMagic
	}

	// Check if already in inventory (for stackable items).
	// SET quantity to the desired value (not ADD) — qty represents the target total,
	// not a delta. Prevents 10 existing + 99 max = 109 instead of 99.
	for i, item := range *items {
		if item.GaItemHandle == handle {
			(*items)[i].Quantity = qty
			off := startOffset + i*InvRecordLen + 4
			if err := sa.CheckBounds(off, 4, "addToInventory/update"); err != nil {
				return err
			}
			binary.LittleEndian.PutUint32(slot.Data[off:], qty)
			return nil
		}
	}

	if isStorage {
		// Storage is pre-allocated (StorageCommonCount=1920 slots), same as held inventory.
		// Find first empty slot by scanning the binary data directly (the in-memory list
		// only contains non-empty items due to ReadStorage skipping gaps).
		storageCapacity := StorageCommonCount
		emptyIdx := -1
		for i := 0; i < storageCapacity; i++ {
			off := startOffset + i*InvRecordLen
			if off+InvRecordLen > len(slot.Data) {
				break
			}
			h := binary.LittleEndian.Uint32(slot.Data[off:])
			if h == GaHandleEmpty || h == GaHandleInvalid {
				emptyIdx = i
				break
			}
		}
		if emptyIdx < 0 {
			return io.ErrShortBuffer // All storage slots occupied
		}

		// Use next_equip_index as the Index value (matching Rust ER-Save-Editor behavior).
		// Clamp to be > InvEquipReservedMax and > max existing index to prevent collisions.
		nextListId := slot.Storage.NextEquipIndex
		if nextListId <= InvEquipReservedMax {
			nextListId = InvEquipReservedMax + 1
		}
		for i := 0; i < storageCapacity; i++ {
			off := startOffset + i*InvRecordLen
			if off+InvRecordLen > len(slot.Data) {
				break
			}
			h := binary.LittleEndian.Uint32(slot.Data[off:])
			if h == GaHandleEmpty || h == GaHandleInvalid {
				continue
			}
			typeBits := h & GaHandleTypeMask
			if typeBits != ItemTypeWeapon && typeBits != ItemTypeArmor &&
				typeBits != ItemTypeAccessory && typeBits != ItemTypeItem && typeBits != ItemTypeAow {
				continue
			}
			idx := binary.LittleEndian.Uint32(slot.Data[off+8:])
			if idx > InvEquipReservedMax && idx < 50000 && idx >= nextListId {
				nextListId = idx + 1
			}
		}

		newItem := InventoryItem{GaItemHandle: handle, Quantity: qty, Index: nextListId}
		off := startOffset + emptyIdx*InvRecordLen
		if err := sa.CheckBounds(off, InvRecordLen, "addToInventory/storage-insert"); err != nil {
			return err
		}
		binary.LittleEndian.PutUint32(slot.Data[off:], newItem.GaItemHandle)
		binary.LittleEndian.PutUint32(slot.Data[off+4:], newItem.Quantity)
		binary.LittleEndian.PutUint32(slot.Data[off+8:], newItem.Index)

		// Advance counters and write back.
		// NextEquipIndex must stay > all per-item Index values (validity gate).
		// NextAcquisitionSortId is an independent sort counter — increment by 1 only.
		slot.Storage.NextEquipIndex = nextListId + 1
		slot.Storage.NextAcquisitionSortId++
		if slot.Storage.nextEquipIndexOff > 0 {
			binary.LittleEndian.PutUint32(slot.Data[slot.Storage.nextEquipIndexOff:], slot.Storage.NextEquipIndex)
		}
		if slot.Storage.nextAcqSortIdOff > 0 {
			binary.LittleEndian.PutUint32(slot.Data[slot.Storage.nextAcqSortIdOff:], slot.Storage.NextAcquisitionSortId)
		}

		// Update common_inventory_items_distinct_count header.
		// The game uses this count to determine how many storage items to load.
		// Without this update, added items are invisible in-game (count stays 0).
		// Source: Rust ER-Save-Editor add_to_storage_common_items() increments common_item_count.
		countOff := slot.StorageBoxOffset // header is at StorageBoxOffset (before items)
		if err := sa.CheckBounds(countOff, 4, "addToInventory/storage-count"); err == nil {
			currentCount := binary.LittleEndian.Uint32(slot.Data[countOff:])
			binary.LittleEndian.PutUint32(slot.Data[countOff:], currentCount+1)
		}

		// Update in-memory list
		*items = append(*items, newItem)
	} else {
		// Inventory is fully pre-allocated — find first empty slot (handle == 0 or 0xFFFFFFFF)
		emptyIdx := -1
		for i, item := range *items {
			if item.GaItemHandle == GaHandleEmpty || item.GaItemHandle == GaHandleInvalid {
				emptyIdx = i
				break
			}
		}
		if emptyIdx < 0 {
			return io.ErrShortBuffer // All slots occupied
		}

		// Per-item acquisition index = current NextAcquisitionSortId (before increment).
		// mapInventory() reconciles this value on load so it is always > all existing
		// item indices — no per-call scan needed here.
		acqIdx := slot.Inventory.NextAcquisitionSortId
		if acqIdx <= InvEquipReservedMax {
			acqIdx = InvEquipReservedMax + 1
			slot.Inventory.NextAcquisitionSortId = acqIdx
		}

		(*items)[emptyIdx] = InventoryItem{GaItemHandle: handle, Quantity: qty, Index: acqIdx}
		off := startOffset + emptyIdx*InvRecordLen
		if err := sa.CheckBounds(off, InvRecordLen, "addToInventory/inv-insert"); err != nil {
			return err
		}
		binary.LittleEndian.PutUint32(slot.Data[off:], handle)
		binary.LittleEndian.PutUint32(slot.Data[off+4:], qty)
		binary.LittleEndian.PutUint32(slot.Data[off+8:], acqIdx)

		// NextEquipIndex: validity gate — items with Index >= this value are invisible to the
		// game. Must stay strictly greater than the item.Index we just wrote (acqIdx).
		// If NextAcquisitionSortId jumped ahead of NextEquipIndex (e.g. InvEquipReservedMax
		// clamp or a save edited by an external tool), a plain ++ leaves the gate behind and
		// the new item becomes invisible. Use max(NextEquipIndex, acqIdx) + 1 instead.
		if acqIdx >= slot.Inventory.NextEquipIndex {
			slot.Inventory.NextEquipIndex = acqIdx + 1
		} else {
			slot.Inventory.NextEquipIndex++
		}
		// NextAcquisitionSortId: sort order; its pre-increment value is the per-item Index.
		slot.Inventory.NextAcquisitionSortId++
		if slot.Inventory.nextEquipIndexOff > 0 {
			binary.LittleEndian.PutUint32(slot.Data[slot.Inventory.nextEquipIndexOff:], slot.Inventory.NextEquipIndex)
		}
		if slot.Inventory.nextAcqSortIdOff > 0 {
			binary.LittleEndian.PutUint32(slot.Data[slot.Inventory.nextAcqSortIdOff:], slot.Inventory.NextAcquisitionSortId)
		}

		// Increment common_item_count header at invStart-4.
		// The game uses this as the insertion point and full check (== CommonItemCount → full).
		// Source: er-save-manager inventory.common_item_count += 1,
		//         Rust ER-Save-Editor storage.common_item_count += 1.
		countOff := startOffset - 4
		if err := sa.CheckBounds(countOff, 4, "addToInventory/inv-count"); err == nil {
			currentCount := binary.LittleEndian.Uint32(slot.Data[countOff:])
			binary.LittleEndian.PutUint32(slot.Data[countOff:], currentCount+1)
		}
	}

	return nil
}

// SetUnlockedRegions replaces the slot's unlocked-regions list with the
// given IDs (deduplicated, sorted ascending), rebuilds the slot data via
// RebuildSlot, and refreshes dynamic offsets so subsequent reads/writes see
// the new layout.
//
// This is the write entry point for the Invasion Regions feature. The
// rebuild path means the call always succeeds regardless of how much
// "slack" exists at the slot tail (full struct rebuild + tail zero pad).
func SetUnlockedRegions(slot *SaveSlot, ids []uint32) error {
	if slot == nil {
		return fmt.Errorf("SetUnlockedRegions: nil slot")
	}
	if slot.Version == 0 {
		return fmt.Errorf("SetUnlockedRegions: cannot modify empty slot")
	}

	// Refresh dynamic offsets and SectionMap from the current slot.Data.
	// Other writers (AddItemsToSlot, FlushGaItems, revealDLCMap, …) mutate
	// slot.Data without updating slot.UnlockedRegionsOffset or SectionMap, so
	// without this refresh we would rebuild from stale boundaries and produce
	// a corrupted save (observed when the user added an item, then revealed
	// the map, then unlocked regions — slot 4 of ER0000.sl2 corrupted at the
	// regCount offset).
	if err := slot.calculateDynamicOffsets(); err != nil {
		return fmt.Errorf("SetUnlockedRegions: refresh offsets: %w", err)
	}
	if err := slot.buildSectionMap(); err != nil {
		return fmt.Errorf("SetUnlockedRegions: refresh section map: %w", err)
	}

	// Dedup + sort ascending — matches er-save-manager invariant and ensures
	// stable output across platforms.
	seen := make(map[uint32]struct{}, len(ids))
	deduped := make([]uint32, 0, len(ids))
	for _, id := range ids {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		deduped = append(deduped, id)
	}
	sortUint32Slice(deduped)

	prev := slot.UnlockedRegions
	prevData := slot.Data
	slot.UnlockedRegions = deduped

	rebuilt, err := RebuildSlot(slot)
	if err != nil {
		slot.UnlockedRegions = prev
		slot.Data = prevData
		return fmt.Errorf("SetUnlockedRegions: %w", err)
	}
	slot.Data = rebuilt

	// Refresh offsets and section map. Failures here roll back the mutation.
	if err := slot.calculateDynamicOffsets(); err != nil {
		slot.UnlockedRegions = prev
		slot.Data = prevData
		return fmt.Errorf("SetUnlockedRegions: re-calc offsets: %w", err)
	}
	if err := slot.buildSectionMap(); err != nil {
		// SectionMap rebuild failure is non-fatal — surface as a warning.
		slot.Warnings = append(slot.Warnings, "SetUnlockedRegions: "+err.Error())
	}
	return nil
}

// ReconcileStorageHeader sets the storage header count to the actual number
// of non-empty items in the storage array. Fixes mismatch from blind +1
// increments in addToInventory when the header was already wrong.
func ReconcileStorageHeader(slot *SaveSlot) {
	if slot.StorageBoxOffset <= 0 || slot.StorageBoxOffset+4 >= len(slot.Data) {
		return
	}
	actualCount := uint32(0)
	storageStart := slot.StorageBoxOffset + StorageHeaderSkip
	for i := 0; i < StorageCommonCount; i++ {
		off := storageStart + i*InvRecordLen
		if off+InvRecordLen > len(slot.Data) {
			break
		}
		h := binary.LittleEndian.Uint32(slot.Data[off:])
		if h != GaHandleEmpty && h != GaHandleInvalid {
			actualCount++
		}
	}
	binary.LittleEndian.PutUint32(slot.Data[slot.StorageBoxOffset:], actualCount)
}

// ReconcileInventoryHeader sets the held-inventory common_item_count header to the
// actual number of non-empty common item slots. Mirrors ReconcileStorageHeader.
// Call this after loading a save that was edited by another tool (er-save-manager,
// Rust ER-Save-Editor) to guarantee the counter matches what the game will see.
func ReconcileInventoryHeader(slot *SaveSlot) {
	if slot.MagicOffset <= 0 {
		return
	}
	invStart := slot.MagicOffset + InvStartFromMagic
	countOff := invStart - 4
	if countOff < 0 || countOff+4 >= len(slot.Data) {
		return
	}
	actualCount := uint32(0)
	for _, item := range slot.Inventory.CommonItems {
		if item.GaItemHandle != GaHandleEmpty && item.GaItemHandle != GaHandleInvalid {
			actualCount++
		}
	}
	binary.LittleEndian.PutUint32(slot.Data[countOff:], actualCount)
}

// RepairOrphanedGaItems clears GaItem records whose handles are not present in
// either held inventory or storage. These orphans accumulate when RemoveItemFromSlot
// zeros the inventory slot but does not clear the backing GaItem binary record —
// scanGaItems() then re-adds them to GaMap on every load.
//
// Returns the number of entries cleared.
func RepairOrphanedGaItems(slot *SaveSlot) int {
	// Build set of live handles from inventory and storage.
	live := make(map[uint32]bool, len(slot.Inventory.CommonItems)+len(slot.Inventory.KeyItems)+len(slot.Storage.CommonItems))
	for _, item := range slot.Inventory.CommonItems {
		if item.GaItemHandle != GaHandleEmpty && item.GaItemHandle != GaHandleInvalid {
			live[item.GaItemHandle] = true
		}
	}
	for _, item := range slot.Inventory.KeyItems {
		if item.GaItemHandle != GaHandleEmpty && item.GaItemHandle != GaHandleInvalid {
			live[item.GaItemHandle] = true
		}
	}
	for _, item := range slot.Storage.CommonItems {
		if item.GaItemHandle != GaHandleEmpty && item.GaItemHandle != GaHandleInvalid {
			live[item.GaItemHandle] = true
		}
	}

	cleared := 0
	for i := range slot.GaItems {
		g := &slot.GaItems[i]
		if g.IsEmpty() {
			continue
		}
		if !live[g.Handle] {
			delete(slot.GaMap, g.Handle)
			*g = GaItemFull{Unk2: -1, Unk3: -1, AoWGaItemHandle: 0xFFFFFFFF}
			cleared++
		}
	}
	return cleared
}

// sortUint32Slice sorts a []uint32 ascending in place.
func sortUint32Slice(s []uint32) {
	for i := 1; i < len(s); i++ {
		v := s[i]
		j := i - 1
		for ; j >= 0 && s[j] > v; j-- {
			s[j+1] = s[j]
		}
		s[j+1] = v
	}
}

// PatchWeaponItemID changes the ItemID of a single weapon GaItem in-place.
// Both IDs must be in the weapon range (prefix 0x00000000 → record size 21 B).
// Since the record size doesn't change, no RebuildSlotFull is needed — we
// overwrite exactly 4 bytes at the ItemID field and update derived state.
//
// Caller guarantees:
//   - handle identifies one weapon instance in slot.GaItems / slot.GaMap
//   - expectedCurrentItemID matches what the slot currently stores (stale-data guard)
//   - newItemID encodes the same base weapon + same upgrade level, different infusion
func PatchWeaponItemID(slot *SaveSlot, handle, expectedCurrentItemID, newItemID uint32) error {
	// Both IDs must be weapons: upper nibble 0x0.
	if expectedCurrentItemID>>28 != 0 {
		return fmt.Errorf("PatchWeaponItemID: expectedCurrentItemID 0x%08X is not a weapon ID", expectedCurrentItemID)
	}
	if newItemID>>28 != 0 {
		return fmt.Errorf("PatchWeaponItemID: newItemID 0x%08X is not a weapon ID", newItemID)
	}
	if expectedCurrentItemID == newItemID {
		return nil
	}

	// Locate the GaItem by handle; simultaneously compute its byte offset in slot.Data.
	// Real entries are stored contiguously from GaItemsStart; synthetic empty entries
	// (those beyond slot.InventoryEnd) have no backing bytes in slot.Data.
	curr := GaItemsStart
	found := -1
	for i := range slot.GaItems {
		if curr >= slot.InventoryEnd {
			break // beyond real data
		}
		g := &slot.GaItems[i]
		if !g.IsEmpty() && g.Handle == handle {
			found = i
			break
		}
		curr += GaItemRecordSize(g.ItemID)
	}
	if found == -1 {
		return fmt.Errorf("PatchWeaponItemID: handle 0x%08X not found in GaItems", handle)
	}

	if slot.GaItems[found].ItemID != expectedCurrentItemID {
		return fmt.Errorf("PatchWeaponItemID: handle 0x%08X: expected ItemID 0x%08X, got 0x%08X (stale data?)",
			handle, expectedCurrentItemID, slot.GaItems[found].ItemID)
	}

	// Overwrite ItemID field (bytes [curr+4, curr+8]) in slot.Data.
	sa := NewSlotAccessor(slot.Data)
	if err := sa.CheckBounds(curr+4, 4, "PatchWeaponItemID/itemID"); err != nil {
		return err
	}
	binary.LittleEndian.PutUint32(slot.Data[curr+4:], newItemID)

	// Update in-memory derived state.
	slot.GaItems[found].ItemID = newItemID
	slot.GaMap[handle] = newItemID

	// Ensure new weapon ID is present in GaitemGameData section.
	// We intentionally leave the old entry in GaItemData — the game tolerates
	// extra entries (same behaviour as when a weapon is removed from inventory
	// but AddItemsToSlot already registered it in GaItemData).
	return upsertGaItemData(slot, newItemID)
}

// PatchWeaponAoWHandle patches the AoWGaItemHandle field of a weapon GaItem in-place.
// newAoWHandle == 0xFFFFFFFF: removes AoW attachment (always allowed).
// newAoWHandle != 0xFFFFFFFF: attaches an existing AoW GaItem — validates that the handle
// identifies an existing AoW GaItem and is not already referenced by a different weapon.
// No GaItem allocation, no RebuildSlotFull — exactly 4 bytes at [weaponOff+16] are overwritten.
func PatchWeaponAoWHandle(slot *SaveSlot, weaponHandle uint32, newAoWHandle uint32) error {
	if slot == nil {
		return fmt.Errorf("PatchWeaponAoWHandle: slot is nil")
	}

	// Locate the weapon GaItem and its byte offset in slot.Data.
	curr := GaItemsStart
	weaponIdx := -1
	weaponByteOff := 0
	for i := range slot.GaItems {
		if curr >= slot.InventoryEnd {
			break
		}
		g := &slot.GaItems[i]
		if !g.IsEmpty() && g.Handle == weaponHandle {
			if g.Handle&GaHandleTypeMask != ItemTypeWeapon {
				return fmt.Errorf("PatchWeaponAoWHandle: handle 0x%08X is not a weapon handle", weaponHandle)
			}
			weaponIdx = i
			weaponByteOff = curr
			break
		}
		curr += GaItemRecordSize(g.ItemID)
	}
	if weaponIdx == -1 {
		return fmt.Errorf("PatchWeaponAoWHandle: weapon handle 0x%08X not found in GaItems", weaponHandle)
	}

	if newAoWHandle != 0xFFFFFFFF {
		if newAoWHandle&GaHandleTypeMask != ItemTypeAow {
			return fmt.Errorf("PatchWeaponAoWHandle: newAoWHandle 0x%08X is not an AoW handle (expected prefix 0xC0000000)", newAoWHandle)
		}
		aowFound := false
		for i := range slot.GaItems {
			g := &slot.GaItems[i]
			if !g.IsEmpty() && g.Handle == newAoWHandle {
				aowFound = true
				break
			}
		}
		if !aowFound {
			return fmt.Errorf("PatchWeaponAoWHandle: AoW GaItem with handle 0x%08X not found in slot", newAoWHandle)
		}
		// Ensure no OTHER weapon already references this AoW handle — sharing causes EXCEPTION_ACCESS_VIOLATION.
		for i := range slot.GaItems {
			g := &slot.GaItems[i]
			if g.IsEmpty() || g.Handle == weaponHandle {
				continue
			}
			if g.Handle&GaHandleTypeMask == ItemTypeWeapon && g.AoWGaItemHandle == newAoWHandle {
				return fmt.Errorf("PatchWeaponAoWHandle: AoW handle 0x%08X is already used by weapon 0x%08X", newAoWHandle, g.Handle)
			}
		}
	}

	sa := NewSlotAccessor(slot.Data)
	if err := sa.CheckBounds(weaponByteOff+16, 4, "PatchWeaponAoWHandle"); err != nil {
		return err
	}
	binary.LittleEndian.PutUint32(slot.Data[weaponByteOff+16:], newAoWHandle)
	slot.GaItems[weaponIdx].AoWGaItemHandle = newAoWHandle
	return nil
}

// PatchWeaponAoW sets or removes the Ash of War attached to a weapon GaItem.
//
// newAoWItemID == 0: removes AoW — patches AoWGaItemHandle to 0xFFFFFFFF in-place.
// No GaItem allocation, no RebuildSlotFull.
//
// newAoWItemID != 0: allocates a fresh AoW GaItem (never reuses an existing handle —
// sharing an AoW handle between two weapons causes EXCEPTION_ACCESS_VIOLATION),
// upserts GaItemData, calls RebuildSlotFull + parseFromData, then patches the
// weapon's AoWGaItemHandle field after the rebuild settles offsets.
//
// Old AoW GaItems are intentionally left in place — the game tolerates orphaned entries.
//
// NOTE: currently invoked only via App.ApplyWeaponAoW, which itself is reachable only from
// the hidden legacy Weapon Edit tab. Do not remove without updating tests and the cleanup plan.
// Sort Order's modal uses the strict path (PatchWeaponAoWHandle) instead.
func PatchWeaponAoW(slot *SaveSlot, weaponHandle, newAoWItemID uint32) error {
	if slot == nil {
		return fmt.Errorf("PatchWeaponAoW: slot is nil")
	}

	// findWeapon locates the weapon GaItem by handle, returning (index, byteOffset).
	// Must be called fresh after RebuildSlotFull because byte offsets shift.
	findWeapon := func() (int, int, error) {
		curr := GaItemsStart
		for i := range slot.GaItems {
			if curr >= slot.InventoryEnd {
				break
			}
			g := &slot.GaItems[i]
			if !g.IsEmpty() && g.Handle == weaponHandle {
				if GaItemRecordSize(g.ItemID) != GaRecordWeapon {
					return -1, -1, fmt.Errorf("PatchWeaponAoW: handle 0x%08X is not a weapon record (itemID 0x%08X)", weaponHandle, g.ItemID)
				}
				return i, curr, nil
			}
			curr += GaItemRecordSize(g.ItemID)
		}
		return -1, -1, fmt.Errorf("PatchWeaponAoW: weapon handle 0x%08X not found in GaItems", weaponHandle)
	}

	if newAoWItemID == 0 {
		// Remove AoW — patch in-place, no rebuild needed.
		idx, curr, err := findWeapon()
		if err != nil {
			return err
		}
		sa := NewSlotAccessor(slot.Data)
		if err := sa.CheckBounds(curr+16, 4, "PatchWeaponAoW/remove"); err != nil {
			return err
		}
		binary.LittleEndian.PutUint32(slot.Data[curr+16:], 0xFFFFFFFF)
		slot.GaItems[idx].AoWGaItemHandle = 0xFFFFFFFF
		return nil
	}

	// newAoWItemID must be in the Ash of War item ID range: upper nibble 0x8.
	if newAoWItemID>>28 != 8 {
		return fmt.Errorf("PatchWeaponAoW: newAoWItemID 0x%08X is not an Ash of War item ID", newAoWItemID)
	}

	// Allocate a fresh AoW GaItem. Never reuse an existing AoW handle —
	// shared handles between weapons cause EXCEPTION_ACCESS_VIOLATION on game load.
	newAoWHandle, err := generateUniqueHandle(slot, ItemTypeAow)
	if err != nil {
		return fmt.Errorf("PatchWeaponAoW: %w", err)
	}
	if err := allocateGaItem(slot, newAoWHandle, newAoWItemID); err != nil {
		return fmt.Errorf("PatchWeaponAoW: %w", err)
	}
	slot.GaMap[newAoWHandle] = newAoWItemID

	if err := upsertGaItemData(slot, newAoWItemID); err != nil {
		return fmt.Errorf("PatchWeaponAoW: %w", err)
	}

	// Save indices advanced by allocateGaItem; parseFromData may underscan them.
	savedNextAoW := slot.NextAoWIndex
	savedNextArmament := slot.NextArmamentIndex
	savedNextHandle := slot.NextGaItemHandle

	// Rebuild — GaItems section grew by 8B (one AoW record). All byte offsets
	// derived from GaItems are now stale; parseFromData refreshes them.
	rebuilt, err := RebuildSlotFull(slot)
	if err != nil {
		return fmt.Errorf("PatchWeaponAoW: rebuild: %w", err)
	}
	copy(slot.Data, rebuilt)
	if err := slot.parseFromData(); err != nil {
		return fmt.Errorf("PatchWeaponAoW: re-parse: %w", err)
	}

	// Restore indices if parseFromData undershot.
	if savedNextAoW > slot.NextAoWIndex {
		slot.NextAoWIndex = savedNextAoW
	}
	if savedNextArmament > slot.NextArmamentIndex {
		slot.NextArmamentIndex = savedNextArmament
	}
	if savedNextHandle > slot.NextGaItemHandle {
		slot.NextGaItemHandle = savedNextHandle
	}

	// Re-locate the weapon after rebuild — its byte offset may have shifted.
	idx, curr, err := findWeapon()
	if err != nil {
		return fmt.Errorf("PatchWeaponAoW: post-rebuild weapon lookup: %w", err)
	}

	sa := NewSlotAccessor(slot.Data)
	if err := sa.CheckBounds(curr+16, 4, "PatchWeaponAoW/set"); err != nil {
		return err
	}
	binary.LittleEndian.PutUint32(slot.Data[curr+16:], newAoWHandle)
	slot.GaItems[idx].AoWGaItemHandle = newAoWHandle

	return nil
}
