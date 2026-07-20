package core

import (
	"encoding/binary"
	"fmt"
	"io"

	"github.com/oisis/EldenRing-SaveForge/backend/db"
)

// upsertWeaponGaItemData ensures a weapon itemID is present in the active
// GaItemData list. The game resolves weapon properties from this list on load;
// omitting the corresponding entry causes EXCEPTION_ACCESS_VIOLATION.
//
// Current PC saves store each active record as eight bytes:
//
//	[0:8]  count (i64) — number of valid entries
//	[8+]   count × { itemID(u32), flag(u32) }
//
// Ordinary item IDs occupy the final ascending low-ID segment before the
// high-bit Ashes of War group. We reproduce the game's lower-bound insertion
// in that segment and shift only the active list, preserving every byte beyond
// the expanded active range.
func upsertWeaponGaItemData(slot *SaveSlot, itemID uint32) error {
	return upsertActiveGaItemData(slot, itemID, "upsertWeaponGaItemData", weaponGaItemDataInsertionIndex)
}

// upsertOrdinaryGaItemData is upsertWeaponGaItemData under a name that
// reflects its real scope. weaponGaItemDataInsertionIndex's lower_bound
// insertion only cares about ascending itemID order within the "ordinary"
// segment (everything before the AoW group, i.e. itemID>>28 != 8) — it has no
// weapon-specific logic. T040 (talismans), T050/T060/T062 (goods/crafting/
// bolstering materials), and T070/T071/T074/T090 (Key Items, cookbooks,
// containers, the Physick package) all confirm the game reorders existing
// active GaItemData entries the same ascending way when a new item in this ID
// range is added — so those families reuse this exact function rather than a
// parallel implementation.
func upsertOrdinaryGaItemData(slot *SaveSlot, itemID uint32) error {
	return upsertWeaponGaItemData(slot, itemID)
}

// upsertAoWGaItemData is the AoW counterpart of upsertWeaponGaItemData. On
// current saves the high-bit AoW group is ascending, so new AoWs are inserted
// with lower_bound. Older saves can retain an unsorted legacy group; appending
// to that group preserves its established order rather than rewriting it.
func upsertAoWGaItemData(slot *SaveSlot, itemID uint32) error {
	return upsertActiveGaItemData(slot, itemID, "upsertAoWGaItemData", aowGaItemDataInsertionIndex)
}

type gaItemDataInsertionIndex func(data []byte, arrayBase, count int, itemID uint32) int

func upsertActiveGaItemData(slot *SaveSlot, itemID uint32, operation string, insertionIndex gaItemDataInsertionIndex) error {
	off := slot.GaItemDataOffset
	if off <= 0 {
		return nil
	}
	sa := NewSlotAccessor(slot.Data)

	if err := sa.CheckBounds(off, GaItemDataArrayOff, operation+"/header"); err != nil {
		return fmt.Errorf("%s: header bounds check failed: %w", operation, err)
	}
	count := int(int32(binary.LittleEndian.Uint32(slot.Data[off:])))
	if count < 0 || count >= GaItemDataMaxCount {
		return fmt.Errorf("%s: count %d out of range [0, %d)", operation, count, GaItemDataMaxCount)
	}

	arrayBase := off + GaItemDataArrayOff
	for i := 0; i < count; i++ {
		entryOff := arrayBase + i*GaItemDataActiveEntryLen
		if err := sa.CheckBounds(entryOff, 4, operation+"/scan"); err != nil {
			return fmt.Errorf("%s: scan bounds check at entry %d: %w", operation, i, err)
		}
		if binary.LittleEndian.Uint32(slot.Data[entryOff:]) == itemID {
			return nil
		}
	}

	insertAt := insertionIndex(slot.Data, arrayBase, count, itemID)
	oldEnd := arrayBase + count*GaItemDataActiveEntryLen
	newEnd := oldEnd + GaItemDataActiveEntryLen
	if err := sa.CheckBounds(oldEnd, GaItemDataActiveEntryLen, operation+"/write"); err != nil {
		return fmt.Errorf("%s: write bounds check at count %d: %w", operation, count, err)
	}
	insertOff := arrayBase + insertAt*GaItemDataActiveEntryLen
	copy(slot.Data[insertOff+GaItemDataActiveEntryLen:newEnd], slot.Data[insertOff:oldEnd])
	binary.LittleEndian.PutUint32(slot.Data[insertOff:], itemID)
	binary.LittleEndian.PutUint32(slot.Data[insertOff+4:], 1)
	binary.LittleEndian.PutUint32(slot.Data[off:], uint32(count+1))
	return nil
}

func weaponGaItemDataInsertionIndex(data []byte, arrayBase, count int, itemID uint32) int {
	ordinaryEnd := count
	for i := 0; i < count; i++ {
		entryOff := arrayBase + i*GaItemDataActiveEntryLen
		if binary.LittleEndian.Uint32(data[entryOff:])>>28 == 8 {
			ordinaryEnd = i
			break
		}
	}
	ordinaryStart := 0
	for i := ordinaryEnd - 1; i > 0; i-- {
		previousOff := arrayBase + (i-1)*GaItemDataActiveEntryLen
		currentOff := arrayBase + i*GaItemDataActiveEntryLen
		if binary.LittleEndian.Uint32(data[previousOff:]) > binary.LittleEndian.Uint32(data[currentOff:]) {
			ordinaryStart = i
			break
		}
	}
	return gaItemDataLowerBound(data, arrayBase, ordinaryStart, ordinaryEnd, itemID)
}

func aowGaItemDataInsertionIndex(data []byte, arrayBase, count int, itemID uint32) int {
	aowStart := count
	for i := 0; i < count; i++ {
		entryOff := arrayBase + i*GaItemDataActiveEntryLen
		if binary.LittleEndian.Uint32(data[entryOff:])>>28 == 8 {
			aowStart = i
			break
		}
	}
	if aowStart == count {
		return count
	}
	aowEnd := aowStart
	for aowEnd < count {
		entryOff := arrayBase + aowEnd*GaItemDataActiveEntryLen
		if binary.LittleEndian.Uint32(data[entryOff:])>>28 != 8 {
			break
		}
		aowEnd++
	}
	for i := aowStart + 1; i < aowEnd; i++ {
		previousOff := arrayBase + (i-1)*GaItemDataActiveEntryLen
		currentOff := arrayBase + i*GaItemDataActiveEntryLen
		if binary.LittleEndian.Uint32(data[previousOff:]) > binary.LittleEndian.Uint32(data[currentOff:]) {
			return aowEnd
		}
	}
	return gaItemDataLowerBound(data, arrayBase, aowStart, aowEnd, itemID)
}

func gaItemDataLowerBound(data []byte, arrayBase, start, end int, itemID uint32) int {
	for i := start; i < end; i++ {
		entryOff := arrayBase + i*GaItemDataActiveEntryLen
		if binary.LittleEndian.Uint32(data[entryOff:]) >= itemID {
			return i
		}
	}
	return end
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

func stackableItemExistsInKeyItems(slot *SaveSlot, itemID uint32) bool {
	for _, ki := range slot.Inventory.KeyItems {
		if ki.Quantity > 0 && db.HandleToItemID(ki.GaItemHandle) == itemID {
			return true
		}
	}
	return false
}

// hasInventoryRecordWithHandle reports whether items already contains a
// physical record for handle. Used to decide "is this a genuinely new item"
// for the GaItemData contract (T040/T050/T060/T062/T070/T071/T074/T090):
// unlike GaMap, which scanGaItems only ever populates from serialized GaItem
// records (weapons/armor/AoW/arrows), goods/talismans/Key Items loaded from a
// real save have no such record, so their prior existence can only be read
// off the physical inventory/KeyItems lists themselves.
func hasInventoryRecordWithHandle(items []InventoryItem, handle uint32) bool {
	for _, it := range items {
		if it.GaItemHandle == handle {
			return true
		}
	}
	return false
}

// AddItemsToSlot adds multiple items to a specific save slot, applying the
// same invQty/storageQty/forceStackable to every id.
// invQty and storageQty control quantities: 0 = skip, -1 = use provided max from caller, >0 = exact qty.
// forceStackable treats items as stackable (reuse existing GaMap handle) regardless of type.
// Used for arrows/bolts which have weapon-like IDs but are stackable in inventory.
//
// A thin per-id wrapper around AddItemsToSlotBatch: both public entry points
// share one classification/allocation/GaItemData code path (see
// classifyItemAdd), so they can never disagree about what a given item ID
// needs — the exact bug requirement 1 (item-add-native-contracts) fixes.
// ForceCommonItems is set unconditionally: every existing AddItemsToSlot
// caller (container auto-sync, cookbook/whetblade/map-fragment flag unlocks,
// DLC map reveal, appearance items) is a flag-driven state-sync helper with
// its own established, independently tested "canonical CommonItems, legacy
// KeyItems compat" contract — not a modeled single-item native pickup — so
// none of them should shift onto the native KeyItems routing
// (nativeKeyItemFamily) that AddItemsToSlotBatch now applies for direct,
// explicit adds of Crafting Kit/cookbooks/Cracked Pot/Crimson Crystal Tear.
// The GaItemData contract itself (this fix's actual point) is unaffected by
// ForceCommonItems: addKindStack and addKindKeyItemStack create it
// identically, only the physical destination container differs.
func AddItemsToSlot(slot *SaveSlot, itemIDs []uint32, invQty, storageQty int, forceStackable bool) error {
	items := make([]ItemToAdd, len(itemIDs))
	for i, id := range itemIDs {
		items[i] = ItemToAdd{
			ItemID:           id,
			InvQty:           invQty,
			StorageQty:       storageQty,
			ForceStackable:   forceStackable,
			ForceCommonItems: true,
		}
	}
	return AddItemsToSlotBatch(slot, items)
}

// AddItemsToSlotBatch adds a batch of items with per-item qty/stackable settings.
// All GaItem allocations happen in Phase 1, then ONE RebuildSlotFull in Phase 2,
// then all inventory/storage writes in Phase 3. This is O(1) rebuilds instead of O(N).
func AddItemsToSlotBatch(slot *SaveSlot, items []ItemToAdd) error {
	type pendingInv struct {
		handle     uint32
		invQty     uint32
		storageQty uint32
		dup        bool // append a new physical record even if handle already exists (talismans)
		keyItem    bool // route invQty into Inventory.KeyItems instead of CommonItems (addKindKeyItemStack)
	}
	var pending []pendingInv
	gaModified := false
	// seenNewGaItemDataForID dedups the "first physical copy of a new ID gets
	// one active GaItemData entry" contract (T040/T050/T060/T062/T070/T071/
	// T074/T090) across the whole batch — mirrors CheckAddCapacity's
	// existingGaItemData/seenNew* bookkeeping so preflight and writer agree.
	seenNewGaItemDataForID := make(map[uint32]bool)

	allocNewGaItem := func(id, handlePrefix uint32) (uint32, error) {
		h, err := generateUniqueHandle(slot, handlePrefix)
		if err != nil {
			return 0, err
		}
		if err := allocateGaItem(slot, h, id); err != nil {
			return 0, err
		}
		slot.GaMap[h] = id
		// Arrows/bolts are weapon-prefixed and must get an active GaItemData
		// entry too (T211). Armor gets the same ordinary-segment treatment as
		// weapons (T020: Chain Coif/Chain Armor both get an active entry) —
		// upsertOrdinaryGaItemData is upsertWeaponGaItemData under its real,
		// non-weapon-specific name.
		if handlePrefix == ItemTypeWeapon || handlePrefix == ItemTypeArmor {
			if err := upsertOrdinaryGaItemData(slot, id); err != nil {
				return 0, err
			}
		} else if handlePrefix == ItemTypeAow {
			if err := upsertAoWGaItemData(slot, id); err != nil {
				return 0, err
			}
		}
		return h, nil
	}

	for _, item := range items {
		handlePrefix := db.ItemIDToHandlePrefix(item.ItemID)
		kind := classifyItemAdd(item.ItemID, item.ForceStackable, item.ForceCommonItems)

		// Talismans (0xA0) are handle-encoded like stackables (no serialized GaItem,
		// handle = 0xA0|itemID) but are NOT fungible stacks: each copy is a distinct
		// physical inventory record, qty 1. Merging by handle would collapse N copies
		// into one. Verified against real saves (PC ER0000.sl2, PS4 .dat): talismans
		// have no serialized GaItem record; they DO get an in-memory GaMap entry (the
		// id-derived handle below) so RebuildSlotFull can resolve the records.
		if kind == addKindTalisman {
			handle := (item.ItemID & 0x0FFFFFFF) | handlePrefix
			slot.GaMap[handle] = item.ItemID
			// T040: the FIRST physical copy of a new talisman ID also gets one
			// active GaItemData entry (flag 1); further copies of the same ID
			// must not create a second one. Existence is read off the physical
			// lists (GaMap is unreliable for talismans loaded from disk — see
			// hasInventoryRecordWithHandle), not off the map write above.
			if item.InvQty > 0 || item.StorageQty > 0 {
				alreadyPhysical := hasInventoryRecordWithHandle(slot.Inventory.CommonItems, handle) ||
					hasInventoryRecordWithHandle(slot.Storage.CommonItems, handle)
				if !alreadyPhysical && !seenNewGaItemDataForID[item.ItemID] {
					if err := upsertOrdinaryGaItemData(slot, item.ItemID); err != nil {
						return err
					}
					seenNewGaItemDataForID[item.ItemID] = true
				}
			}
			for n := 0; n < item.InvQty; n++ {
				pending = append(pending, pendingInv{handle: handle, invQty: 1, dup: true})
			}
			for n := 0; n < item.StorageQty; n++ {
				pending = append(pending, pendingInv{handle: handle, storageQty: 1, dup: true})
			}
			continue
		}

		if kind == addKindStack {
			handle := uint32(0)
			for h, id := range slot.GaMap {
				if id == item.ItemID {
					handle = h
					break
				}
			}
			if handle == 0 && stackableItemExistsInKeyItems(slot, item.ItemID) {
				continue // item already in KeyItems — skip to avoid duplicate
			}
			if handle == 0 {
				handle = (item.ItemID & 0x0FFFFFFF) | handlePrefix
				slot.GaMap[handle] = item.ItemID
			}
			// T050/T060/T062: the FIRST physical record of a new goods/
			// crafting-material/bolstering-material stack (either
			// destination) also gets one active GaItemData entry; a
			// quantity bump to an existing stack must not add a second one.
			if item.InvQty > 0 || item.StorageQty > 0 {
				alreadyPhysical := hasInventoryRecordWithHandle(slot.Inventory.CommonItems, handle) ||
					hasInventoryRecordWithHandle(slot.Storage.CommonItems, handle)
				if !alreadyPhysical && !seenNewGaItemDataForID[item.ItemID] {
					if err := upsertOrdinaryGaItemData(slot, item.ItemID); err != nil {
						return err
					}
					seenNewGaItemDataForID[item.ItemID] = true
				}
			}
			pending = append(pending, pendingInv{
				handle:     handle,
				invQty:     uint32(item.InvQty),
				storageQty: uint32(item.StorageQty),
			})
			continue
		}

		if kind == addKindKeyItemStack {
			// T070/T071/T074/T090: Crafting Kit, cookbooks, Cracked Pot, and the
			// Physick package's Crimson Crystal Tear variant — same id-derived,
			// handle-encoded fungible stack as addKindStack, no serialized
			// GaItem. handle is deterministic (not looked up via GaMap first)
			// because it must match whichever container a legacy record
			// already lives in.
			handle := (item.ItemID & 0x0FFFFFFF) | handlePrefix
			slot.GaMap[handle] = item.ItemID
			// Legacy compat: a record already sitting in CommonItems (e.g. an
			// app version predating this fix) is the canonical physical
			// location for THIS handle and must be bumped in place — never
			// duplicated into KeyItems. Only a genuinely new record, or one
			// already correctly in KeyItems, routes natively.
			legacyInCommonItems := hasInventoryRecordWithHandle(slot.Inventory.CommonItems, handle)
			if item.InvQty > 0 || item.StorageQty > 0 {
				alreadyPhysical := legacyInCommonItems ||
					hasInventoryRecordWithHandle(slot.Inventory.KeyItems, handle) ||
					hasInventoryRecordWithHandle(slot.Storage.CommonItems, handle)
				if !alreadyPhysical && !seenNewGaItemDataForID[item.ItemID] {
					if err := upsertOrdinaryGaItemData(slot, item.ItemID); err != nil {
						return err
					}
					seenNewGaItemDataForID[item.ItemID] = true
				}
			}
			pending = append(pending, pendingInv{
				handle:     handle,
				invQty:     uint32(item.InvQty),
				storageQty: uint32(item.StorageQty),
				keyItem:    !legacyInCommonItems,
			})
			continue
		}

		if kind == addKindArrow {
			handle := uint32(0)
			for h, id := range slot.GaMap {
				if id == item.ItemID {
					handle = h
					break
				}
			}
			if handle == 0 && stackableItemExistsInKeyItems(slot, item.ItemID) {
				continue // item already in KeyItems — skip to avoid duplicate
			}
			if handle == 0 {
				var err error
				handle, err = allocNewGaItem(item.ItemID, handlePrefix)
				if err != nil {
					return err
				}
				gaModified = true
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
			if p.keyItem {
				if err := addToKeyItems(slot, p.handle, p.invQty); err != nil {
					return err
				}
			} else if err := addToInventory(slot, p.handle, p.invQty, false, p.dup); err != nil {
				return err
			}
		}
		if p.storageQty != 0 {
			if err := addToInventory(slot, p.handle, p.storageQty, true, p.dup); err != nil {
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

	// Unk2/Unk3 default to 0 for weapon/armor-shaped records (confirmed native:
	// T020 armor Chain Coif/Chain Armor, T211/T063 Arrow — both -1 in the app's
	// old output). AoW records serialize as 8 bytes (GaRecordItem) and never
	// write these fields at all, so -1 here is inert but kept for continuity
	// with the pre-fix in-memory default.
	entry := GaItemFull{
		Handle:          handle,
		ItemID:          itemID,
		Unk2:            -1,
		Unk3:            -1,
		AoWGaItemHandle: NoCustomAoWHandle,
		Unk5:            0,
	}
	if !isAoW {
		entry.Unk2 = 0
		entry.Unk3 = 0
	}

	maxEntries := len(slot.GaItems)

	if isAoW {
		idx := slot.NextAoWIndex
		if idx >= maxEntries {
			return fmt.Errorf("allocateGaItem: AoW array full (index %d >= %d)", idx, maxEntries)
		}
		// AoW insertion unconditionally advances NextArmamentIndex (the
		// armament zone's right edge bookkeeping). If that edge is already
		// at the array limit, advancing it would push it past maxEntries
		// and trip ValidatePostMutation's "NextArmamentIndex > len(GaItems)"
		// check at commit time. Reject early with a clear, allocator-level
		// message instead of letting the post-mutation validator surface a
		// numeric-looking violation to the user. Saves with a non-empty
		// entry pinned at array position maxEntries-1 (observed on PS4
		// saves that reach this state via in-game placement) end up with
		// NextArmamentIndex == maxEntries on load, so any AoW add would
		// otherwise corrupt the index.
		if slot.NextArmamentIndex >= maxEntries {
			return fmt.Errorf("allocateGaItem: cannot insert AoW — armament zone at capacity (NextArmamentIndex %d == %d)", slot.NextArmamentIndex, maxEntries)
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
				keyStart := invStart + CommonItemCount*InvRecordLen + InvKeyCountHeader
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
// Editor-added goods use computed handle (e.g. 0xB0XXXXXX for 0x40XXXXXX items).
// Game-placed key items in the KeyItems section use the raw item ID as handle.
func RemoveItemByBaseID(slot *SaveSlot, itemID uint32) {
	handlePrefix := db.ItemIDToHandlePrefix(itemID)
	computedHandle := (itemID & 0x0FFFFFFF) | handlePrefix

	// CommonItems: editor-added goods use computed handle (0xB0XXXXXX for cookbooks)
	for _, item := range slot.Inventory.CommonItems {
		if item.GaItemHandle == computedHandle && item.Quantity > 0 {
			_ = RemoveItemFromSlot(slot, computedHandle, true, false)
			return
		}
	}
	// KeyItems: game-placed key items use raw item ID as handle (no GaItem record)
	for _, item := range slot.Inventory.KeyItems {
		if (item.GaItemHandle == itemID || item.GaItemHandle == computedHandle) && item.Quantity > 0 {
			_ = RemoveItemFromSlot(slot, item.GaItemHandle, true, false)
			return
		}
	}
}

// storageRecordOffset returns the binary offset of the storage CommonItems
// record whose GaItemHandle == handle, scanning the raw slot.Data array
// (StorageCommonCount slots from startOffset). Required because the in-memory
// Storage.CommonItems slice is COMPACTED (empty slots skipped on load — see
// spec/10-storage.md), so a slice index does not map to a binary position.
func storageRecordOffset(slot *SaveSlot, startOffset int, handle uint32) (int, error) {
	for i := 0; i < StorageCommonCount; i++ {
		off := startOffset + i*InvRecordLen
		if off+InvRecordLen > len(slot.Data) {
			break
		}
		if binary.LittleEndian.Uint32(slot.Data[off:]) == handle {
			return off, nil
		}
	}
	return 0, fmt.Errorf("storageRecordOffset: handle 0x%08X not found in storage CommonItems", handle)
}

// nextAcquisitionWriteIndex returns the next safe acquisition index for a
// newly written record. Elden Ring orders acquisition records by Index >> 1,
// so writes must use a parity-stable stride of two. Keeping the base even also
// keeps every consecutive write in a distinct game-side bucket.
func nextAcquisitionWriteIndex(next uint32) uint32 {
	if next <= InvEquipReservedMax {
		next = InvEquipReservedMax + 2
	}
	if next%2 != 0 {
		next++
	}
	return next
}

// allowDuplicate: when true, always append a NEW physical record even if a record
// with the same handle already exists. Used for talismans, where N copies are N
// separate records sharing the id-derived handle (not a merged quantity stack).
func addToInventory(slot *SaveSlot, handle uint32, qty uint32, isStorage bool, allowDuplicate bool) error {
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
		if allowDuplicate {
			break
		}
		if item.GaItemHandle == handle {
			(*items)[i].Quantity = qty
			// Binary position: held inventory CommonItems is a full 2688-entry array
			// (slice index == binary slot), so i*InvRecordLen is correct. Storage
			// CommonItems is a COMPACTED slice (empty slots skipped on load — see
			// spec/10-storage.md), so i is NOT the binary position; locate the real
			// record by scanning slot.Data for this handle.
			var off int
			if isStorage {
				binOff, err := storageRecordOffset(slot, startOffset, handle)
				if err != nil {
					return err
				}
				off = binOff + 4
			} else {
				off = startOffset + i*InvRecordLen + 4
			}
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

		// Acquisition order is keyed by Index >> 1 in-game. Start from the
		// acquisition counter (not NextEquipIndex), then keep the value above
		// existing storage records and on an even stride-2 boundary.
		nextListId := slot.Storage.NextAcquisitionSortId
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
		nextListId = nextAcquisitionWriteIndex(nextListId)

		newItem := InventoryItem{GaItemHandle: handle, Quantity: qty, Index: nextListId}
		off := startOffset + emptyIdx*InvRecordLen
		if err := sa.CheckBounds(off, InvRecordLen, "addToInventory/storage-insert"); err != nil {
			return err
		}
		binary.LittleEndian.PutUint32(slot.Data[off:], newItem.GaItemHandle)
		binary.LittleEndian.PutUint32(slot.Data[off+4:], newItem.Quantity)
		binary.LittleEndian.PutUint32(slot.Data[off+8:], newItem.Index)

		// NextEquipIndex is a separate game-owned counter. Acquisition order uses
		// NextAcquisitionSortId, so inserting a record must never synchronize or
		// advance NextEquipIndex (doing so causes an in-game load crash).
		slot.Storage.NextAcquisitionSortId = nextListId + 1
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

		// Elden Ring uses Index >> 1 as its acquisition-order key. Allocate a
		// fresh even value so consecutive writes advance by two and cannot share
		// a game-side sort bucket.
		acqIdx := nextAcquisitionWriteIndex(slot.Inventory.NextAcquisitionSortId)

		(*items)[emptyIdx] = InventoryItem{GaItemHandle: handle, Quantity: qty, Index: acqIdx}
		off := startOffset + emptyIdx*InvRecordLen
		if err := sa.CheckBounds(off, InvRecordLen, "addToInventory/inv-insert"); err != nil {
			return err
		}
		binary.LittleEndian.PutUint32(slot.Data[off:], handle)
		binary.LittleEndian.PutUint32(slot.Data[off+4:], qty)
		binary.LittleEndian.PutUint32(slot.Data[off+8:], acqIdx)

		// NextEquipIndex is a separate game-owned counter. Acquisition order uses
		// NextAcquisitionSortId, so inserting a record must never synchronize or
		// advance NextEquipIndex (doing so causes an in-game load crash).
		slot.Inventory.NextAcquisitionSortId = acqIdx + 1
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

// addToKeyItems is addToInventory's Storage branch (physical byte-scan, not
// in-memory-slice-index) retargeted at Inventory.KeyItems: a fully
// pre-allocated KeyItemCount-slot array on any real parsed slot (see
// EquipInventoryData.Read), found here by scanning slot.Data directly rather
// than assuming len(slot.Inventory.KeyItems) already matches — a caller-built
// test fixture may leave that slice short or nil even though the physical
// bytes at keyStart are correctly sized and zeroed. Shares
// slot.Inventory.NextAcquisitionSortId with CommonItems (T070 confirms a
// KeyItems add advances the identical counter a CommonItems add does) and
// never touches NextEquipIndex, matching addToInventory. There is no
// isStorage variant: no confirmed-native family here has evidence of a
// KeyItems-equivalent Storage placement, so StorageQty for these items keeps
// going through addToInventory's ordinary Storage.CommonItems path.
func addToKeyItems(slot *SaveSlot, handle uint32, qty uint32) error {
	sa := NewSlotAccessor(slot.Data)
	keyStart := slot.MagicOffset + InvStartFromMagic + CommonItemCount*InvRecordLen + InvKeyCountHeader

	for i := 0; i < KeyItemCount; i++ {
		off := keyStart + i*InvRecordLen
		if off+InvRecordLen > len(slot.Data) {
			break
		}
		if binary.LittleEndian.Uint32(slot.Data[off:]) != handle {
			continue
		}
		if err := sa.CheckBounds(off+4, 4, "addToKeyItems/update"); err != nil {
			return err
		}
		binary.LittleEndian.PutUint32(slot.Data[off+4:], qty)
		for idx := range slot.Inventory.KeyItems {
			if slot.Inventory.KeyItems[idx].GaItemHandle == handle {
				slot.Inventory.KeyItems[idx].Quantity = qty
			}
		}
		return nil
	}

	emptyIdx := -1
	for i := 0; i < KeyItemCount; i++ {
		off := keyStart + i*InvRecordLen
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
		return io.ErrShortBuffer // All KeyItems slots occupied
	}

	acqIdx := nextAcquisitionWriteIndex(slot.Inventory.NextAcquisitionSortId)
	off := keyStart + emptyIdx*InvRecordLen
	if err := sa.CheckBounds(off, InvRecordLen, "addToKeyItems/insert"); err != nil {
		return err
	}
	binary.LittleEndian.PutUint32(slot.Data[off:], handle)
	binary.LittleEndian.PutUint32(slot.Data[off+4:], qty)
	binary.LittleEndian.PutUint32(slot.Data[off+8:], acqIdx)

	slot.Inventory.NextAcquisitionSortId = acqIdx + 1
	if slot.Inventory.nextAcqSortIdOff > 0 {
		binary.LittleEndian.PutUint32(slot.Data[slot.Inventory.nextAcqSortIdOff:], slot.Inventory.NextAcquisitionSortId)
	}

	// key_count header, sitting immediately before the KeyItems array.
	// Confirmed by direct byte-level inspection of the T071/T074 read-only
	// lab artifacts (tmp/item-save-lab/WYNIKI_I_IMPLEMENTACJA.md, "Analiza
	// key_count" section): the header equals the physical non-empty KeyItems
	// count at every observed step, incrementing by exactly 1 on a genuinely
	// new physical record and staying flat across a quantity-only bump of an
	// existing record (task-074-containers 04-before-b -> 05-pickup-a-b:
	// Cracked Pot 1->2, key_count 1->1). Not a symmetry guess with
	// common_item_count — independently verified for this header specifically.
	countOff := keyStart - InvKeyCountHeader
	if err := sa.CheckBounds(countOff, 4, "addToKeyItems/key-count"); err == nil {
		currentCount := binary.LittleEndian.Uint32(slot.Data[countOff:])
		binary.LittleEndian.PutUint32(slot.Data[countOff:], currentCount+1)
	}

	newItem := InventoryItem{GaItemHandle: handle, Quantity: qty, Index: acqIdx}
	if emptyIdx < len(slot.Inventory.KeyItems) {
		// Real parsed slot: the in-memory slice already spans KeyItemCount,
		// slice index == physical row.
		slot.Inventory.KeyItems[emptyIdx] = newItem
	} else {
		// Test fixture with a short/nil KeyItems slice: append, mirroring the
		// physical position it was just written to (rows are filled in
		// ascending order, so len(slice) == emptyIdx here).
		slot.Inventory.KeyItems = append(slot.Inventory.KeyItems, newItem)
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
	return upsertWeaponGaItemData(slot, newItemID)
}

// PatchWeaponAoWHandle patches the AoWGaItemHandle field of a weapon GaItem in-place.
// newAoWHandle indicates no-custom-AoW (per IsNoCustomAoWHandle): removes AoW
// attachment. Always allowed; the canonical NoCustomAoWHandle value is
// written regardless of which sentinel the caller supplied — keeps disk
// output vanilla-aligned even when legacy callers still pass 0xFFFFFFFF.
// newAoWHandle is a valid 0xC0... handle: attaches an existing AoW GaItem —
// validates that the handle identifies an existing AoW GaItem and is not
// already referenced by a different weapon.
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

	if IsNoCustomAoWHandle(newAoWHandle) {
		// Canonicalize the sentinel: callers may still pass the legacy
		// 0xFFFFFFFF, but disk output should match the in-game vanilla
		// value (0x00000000) — see the constant docs in structures.go.
		newAoWHandle = NoCustomAoWHandle
	} else {
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
// newAoWItemID == 0: removes AoW — patches AoWGaItemHandle to the canonical
// NoCustomAoWHandle (0x00000000) in-place. No GaItem allocation, no RebuildSlotFull.
//
// newAoWItemID != 0: allocates a fresh AoW GaItem (never reuses an existing handle —
// sharing an AoW handle between two weapons causes EXCEPTION_ACCESS_VIOLATION),
// upserts GaItemData, calls RebuildSlotFull + parseFromData, then patches the
// weapon's AoWGaItemHandle field after the rebuild settles offsets.
//
// Old AoW GaItems are intentionally left in place — the game tolerates orphaned entries.
//
// This is the allocate path: it mints a fresh AoW GaItem rather than reusing an
// existing handle. It is invoked by the active workspace save flow
// (backend/editor/save.go) when an AoW change requires a new record; the in-place
// strict path (PatchWeaponAoWHandle) handles reuse of a pre-existing free copy.
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
		binary.LittleEndian.PutUint32(slot.Data[curr+16:], NoCustomAoWHandle)
		slot.GaItems[idx].AoWGaItemHandle = NoCustomAoWHandle
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

	if err := upsertAoWGaItemData(slot, newAoWItemID); err != nil {
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
