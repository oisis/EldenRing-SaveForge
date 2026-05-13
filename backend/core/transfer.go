package core

import (
	"encoding/binary"
	"fmt"
)

// TransferDirection identifies the source→destination direction of a move
// between Inventory.CommonItems and Storage.CommonItems.
type TransferDirection int

const (
	TransferToStorage   TransferDirection = iota // Inventory.CommonItems → Storage.CommonItems
	TransferToInventory                          // Storage.CommonItems → Inventory.CommonItems
)

// TransferSkip describes a single handle outcome that was not (fully) moved.
// MovedQty and RemainingQty are populated only for partial transfers (reason
// "dest_at_cap" with non-zero MovedQty) — other reasons leave them at 0.
type TransferSkip struct {
	Handle       uint32 `json:"handle"`
	Reason       string `json:"reason"`             // see SkipReason* constants
	MovedQty     uint32 `json:"movedQty,omitempty"`  // qty actually transferred (partial-cap only)
	RemainingQty uint32 `json:"remainingQty,omitempty"` // qty left in source (partial-cap only)
}

// Skip reason constants.
const (
	SkipReasonInvalidHandle     = "invalid_handle"
	SkipReasonNotFound          = "not_found"
	SkipReasonEquipped          = "equipped"
	SkipReasonDestFull          = "dest_full"
	SkipReasonDestAtCap         = "dest_at_cap"
	SkipReasonMissingCap        = "missing_cap"
	SkipReasonHandleAllocFailed = "handle_alloc_failed"

	// SkipReasonDestDuplicate is reserved. Instance-move handles that
	// encounter the same handle on the destination side trigger a rehandle
	// path (materializeRehandledInstance) and do NOT return this reason. It
	// is kept as a public constant for callers that want to surface a
	// duplicate-handle condition in future direct-write APIs (e.g. preset
	// import) and for backwards compatibility with the prior transfer
	// semantics. The transfer core no longer emits it.
	SkipReasonDestDuplicate = "dest_duplicate"
)

// TransferResult is the outcome of a batch transfer.
type TransferResult struct {
	Moved   int            `json:"moved"`
	Skipped []TransferSkip `json:"skipped"`
}

// TransferOptions carries optional parameters for MoveItemsBetweenContainers.
// DestCaps provides per-handle quantity caps for the destination container
// (e.g. MaxStorage for to-storage direction). When a stackable handle is
// missing from DestCaps or maps to 0, the move is rejected with
// SkipReasonMissingCap rather than silently exceeding the in-game cap.
// Non-stackable items ignore the cap (qty=1 per record).
type TransferOptions struct {
	DestCaps map[uint32]uint32
}

// MoveItemsBetweenContainers physically relocates item records between
// Inventory.CommonItems and Storage.CommonItems. The GaItem array and GaMap
// are NOT touched — the handle stays alive on the destination side and the
// underlying GaItem record (with all its metadata: upgrade level, infusion,
// attached AoW gem) is preserved across the transfer.
//
// For non-stackable items (handle prefixes 0x80 Weapon, 0x90 Armor, 0xC0 AoW):
// the source record is cleared and an empty destination slot receives the same
// handle, the original quantity, and a freshly-assigned Index (per add-like
// rules for the dest side). Caps are ignored (records hold qty=1).
//
// For stackable items (handle prefixes 0xA0 Accessory/Talisman and 0xB0
// Goods), and for any case where the destination already has a record with the
// same handle: the move is cap-aware. The destination cap is read from
// opts.DestCaps[handle]; if missing or 0, the handle is skipped with reason
// SkipReasonMissingCap (no silent unbounded merge). When the source quantity
// exceeds the available space (cap - existing_dst_qty), the move is partial:
// destination is filled to cap, source retains the remainder, the handle is
// reported in both Moved (>0 qty moved) and Skipped (reason
// SkipReasonDestAtCap with MovedQty/RemainingQty populated). When the
// destination is already at cap, no movement happens and the skip reason is
// SkipReasonDestAtCap with MovedQty=0.
//
// Equipped items (handles referenced by ChrAsmEquipment) are rejected when
// moving Inventory → Storage with skip reason SkipReasonEquipped. No equipped
// check is performed for Storage → Inventory.
//
// The function never partial-fails the batch: invalid handles are recorded in
// Skipped, valid handles are processed independently. After the batch, both
// common_item_count headers are reconciled defensively, and
// slot.Storage.CommonItems is rebuilt from binary to drop stale entries.
func MoveItemsBetweenContainers(slot *SaveSlot, handles []uint32, direction TransferDirection, opts *TransferOptions) (TransferResult, error) {
	res := TransferResult{}
	if slot == nil {
		return res, fmt.Errorf("MoveItemsBetweenContainers: nil slot")
	}
	if slot.Version == 0 {
		return res, fmt.Errorf("MoveItemsBetweenContainers: empty slot")
	}
	if direction != TransferToStorage && direction != TransferToInventory {
		return res, fmt.Errorf("MoveItemsBetweenContainers: invalid direction %d", direction)
	}

	var caps map[uint32]uint32
	if opts != nil {
		caps = opts.DestCaps
	}

	for _, h := range handles {
		if h == GaHandleEmpty || h == GaHandleInvalid {
			res.Skipped = append(res.Skipped, TransferSkip{Handle: h, Reason: SkipReasonInvalidHandle})
			continue
		}
		moved, skip := transferOne(slot, h, direction, caps)
		if moved {
			res.Moved++
		}
		if skip != nil {
			res.Skipped = append(res.Skipped, *skip)
		}
	}

	// Defensive reconcile: regardless of which path each handle took, headers
	// must reflect the actual non-empty record count. Both calls scan binary
	// and rewrite the counter — idempotent when already correct.
	ReconcileInventoryHeader(slot)
	ReconcileStorageHeader(slot)

	// Rebuild in-memory CommonItems lists from the binary so callers (e.g.
	// vm.MapParsedSlotToVM) observe the dest writes. The transfer mutates
	// binary directly via writeRecord; positional updates on the source side
	// are sufficient for source clears, but DEST writes (especially the
	// rehandle path that runs after a slot rebuild) need a full refresh to
	// expose the new record.
	rescanInventoryList(slot)
	rescanStorageList(slot)

	return res, nil
}

// Handle classification for transfer semantics.
//
// Items used in Sort Order tabs (weapons / talismans / armor) and AoW gems
// are instance-transfer: each record represents a discrete equippable unit
// regardless of whether the item ID is duplicated elsewhere. The move
// transplants the source record verbatim onto an empty destination slot —
// no cap is consulted, no merge.
//
// Goods (0xB0) are real quantity stacks: dst qty is summed with src qty up
// to the destination cap; partial merges may leave a remainder on source.
//
// Notes on talismans (0xA0): the game encodes handle == itemID|prefix, so
// adding the same talisman to both Inventory and Storage via AddItemsToSlot
// produces TWO records sharing one handle. Treating that as a cap collision
// (the old behaviour) is wrong — the records are independent inventory
// instances and the user expects to move the source slot away regardless of
// whether dst happens to hold an unrelated copy. We surface that edge case
// as SkipReasonDestDuplicate so callers can distinguish it from a real cap
// hit; the actual move is then declined to avoid creating two records with
// the same handle inside a single container (game treats them as the same
// physical item — see AddItemsToSlot comments at writer.go:176-198).
func isInstanceMoveHandle(handle uint32) bool {
	switch handle & GaHandleTypeMask {
	case ItemTypeWeapon, ItemTypeArmor, ItemTypeAccessory, ItemTypeAow:
		return true
	default:
		return false
	}
}

func isQuantityMergeHandle(handle uint32) bool {
	return handle&GaHandleTypeMask == ItemTypeItem
}

// transferOne moves a single handle. Returns (moved, skip):
//   - moved=true means at least one unit of qty was transferred.
//   - skip != nil records a skip/partial reason; with SkipReasonDestAtCap and
//     moved=true the handle was partially transferred.
func transferOne(slot *SaveSlot, handle uint32, direction TransferDirection, caps map[uint32]uint32) (bool, *TransferSkip) {
	srcStart, srcSlots := containerBinary(slot, sourceContainer(direction))
	dstStart, dstSlots := containerBinary(slot, destContainer(direction))
	if srcStart <= 0 || dstStart <= 0 {
		return false, &TransferSkip{Handle: handle, Reason: SkipReasonNotFound}
	}

	srcIdx, srcQty, ok := scanRecord(slot.Data, srcStart, srcSlots, handle)
	if !ok {
		return false, &TransferSkip{Handle: handle, Reason: SkipReasonNotFound}
	}

	// Equipped check (only Inventory → Storage).
	if direction == TransferToStorage && IsHandleEquipped(slot, handle) {
		return false, &TransferSkip{Handle: handle, Reason: SkipReasonEquipped}
	}

	// ── Instance-move path (weapons, armor, talismans, AoW) ──────────────
	if isInstanceMoveHandle(handle) {
		// Duplicate-handle on dest: common for handle-encoded talismans
		// (0xA0; AddItemsToSlot stores the same handle in both containers
		// when the UI adds an item to inventory and storage simultaneously)
		// and theoretically possible after save edits for weapons/armor.
		// The user wants the transfer to materialize a separate instance on
		// the destination side rather than blocking — we allocate a fresh
		// unique handle for the moved record and let the original dst entry
		// remain untouched.
		if _, _, dstExists := scanRecord(slot.Data, dstStart, dstSlots, handle); dstExists {
			newHandle, err := materializeRehandledInstance(slot, handle)
			if err != nil {
				return false, &TransferSkip{Handle: handle, Reason: SkipReasonHandleAllocFailed}
			}
			if err := rebuildAfterAllocation(slot); err != nil {
				return false, &TransferSkip{Handle: handle, Reason: SkipReasonHandleAllocFailed}
			}
			// parseFromData refreshed slot.Inventory/Storage layout and all
			// dynamic offsets; recompute start positions and re-scan source.
			srcStart, srcSlots = containerBinary(slot, sourceContainer(direction))
			dstStart, dstSlots = containerBinary(slot, destContainer(direction))
			if srcStart <= 0 || dstStart <= 0 {
				return false, &TransferSkip{Handle: handle, Reason: SkipReasonNotFound}
			}
			rescanIdx, rescanQty, rescanOk := scanRecord(slot.Data, srcStart, srcSlots, handle)
			if !rescanOk {
				return false, &TransferSkip{Handle: handle, Reason: SkipReasonNotFound}
			}
			srcIdx = rescanIdx
			srcQty = rescanQty
			dstEmptyIdx, hasEmpty := scanEmpty(slot.Data, dstStart, dstSlots)
			if !hasEmpty {
				return false, &TransferSkip{Handle: handle, Reason: SkipReasonDestFull}
			}
			newIndex := assignDestIndex(slot, destContainer(direction))
			writeRecord(slot.Data, dstStart, dstEmptyIdx, newHandle, srcQty, newIndex)
			advanceDestCounters(slot, destContainer(direction), newIndex)
			clearRecord(slot.Data, srcStart, srcIdx, sourceContainer(direction))
			updateInventoryListAfterClear(slot, sourceContainer(direction), srcIdx)
			return true, nil
		}
		dstEmptyIdx, hasEmpty := scanEmpty(slot.Data, dstStart, dstSlots)
		if !hasEmpty {
			return false, &TransferSkip{Handle: handle, Reason: SkipReasonDestFull}
		}
		newIndex := assignDestIndex(slot, destContainer(direction))
		writeRecord(slot.Data, dstStart, dstEmptyIdx, handle, srcQty, newIndex)
		advanceDestCounters(slot, destContainer(direction), newIndex)
		clearRecord(slot.Data, srcStart, srcIdx, sourceContainer(direction))
		updateInventoryListAfterClear(slot, sourceContainer(direction), srcIdx)
		return true, nil
	}

	// ── Quantity-merge path (goods 0xB0; default for unknown prefixes) ───
	cap, hasCap := caps[handle]
	if !hasCap || cap == 0 {
		return false, &TransferSkip{Handle: handle, Reason: SkipReasonMissingCap}
	}
	dstExistingIdx, dstExistingQty, dstExists := scanRecord(slot.Data, dstStart, dstSlots, handle)
	if dstExists {
		if dstExistingQty >= cap {
			return false, &TransferSkip{Handle: handle, Reason: SkipReasonDestAtCap}
		}
		available := cap - dstExistingQty
		transferQty := srcQty
		if transferQty > available {
			transferQty = available
		}
		writeRecordQty(slot.Data, dstStart, dstExistingIdx, dstExistingQty+transferQty)
		remaining := srcQty - transferQty
		if remaining == 0 {
			clearRecord(slot.Data, srcStart, srcIdx, sourceContainer(direction))
			updateInventoryListAfterClear(slot, sourceContainer(direction), srcIdx)
		} else {
			writeRecordQty(slot.Data, srcStart, srcIdx, remaining)
			updateInventoryListQty(slot, sourceContainer(direction), srcIdx, remaining)
		}
		if remaining > 0 {
			return true, &TransferSkip{
				Handle:       handle,
				Reason:       SkipReasonDestAtCap,
				MovedQty:     transferQty,
				RemainingQty: remaining,
			}
		}
		return true, nil
	}
	// Quantity-merge, dest empty: create new record with cap-clamped qty.
	dstEmptyIdx, hasEmpty := scanEmpty(slot.Data, dstStart, dstSlots)
	if !hasEmpty {
		return false, &TransferSkip{Handle: handle, Reason: SkipReasonDestFull}
	}
	transferQty := srcQty
	if transferQty > cap {
		transferQty = cap
	}
	newIndex := assignDestIndex(slot, destContainer(direction))
	writeRecord(slot.Data, dstStart, dstEmptyIdx, handle, transferQty, newIndex)
	advanceDestCounters(slot, destContainer(direction), newIndex)
	remaining := srcQty - transferQty
	if remaining == 0 {
		clearRecord(slot.Data, srcStart, srcIdx, sourceContainer(direction))
		updateInventoryListAfterClear(slot, sourceContainer(direction), srcIdx)
	} else {
		writeRecordQty(slot.Data, srcStart, srcIdx, remaining)
		updateInventoryListQty(slot, sourceContainer(direction), srcIdx, remaining)
	}
	_ = dstExistingIdx
	if remaining > 0 {
		return true, &TransferSkip{
			Handle:       handle,
			Reason:       SkipReasonDestAtCap,
			MovedQty:     transferQty,
			RemainingQty: remaining,
		}
	}
	return true, nil
}

// ── container helpers ────────────────────────────────────────────────────────

type containerKind int

const (
	containerInventory containerKind = iota
	containerStorage
)

func sourceContainer(d TransferDirection) containerKind {
	if d == TransferToStorage {
		return containerInventory
	}
	return containerStorage
}

func destContainer(d TransferDirection) containerKind {
	if d == TransferToStorage {
		return containerStorage
	}
	return containerInventory
}

// containerBinary returns the absolute start offset of the CommonItems block
// and the number of pre-allocated record slots for the given container.
func containerBinary(slot *SaveSlot, c containerKind) (int, int) {
	switch c {
	case containerInventory:
		if slot.MagicOffset <= 0 {
			return 0, 0
		}
		return slot.MagicOffset + InvStartFromMagic, CommonItemCount
	case containerStorage:
		if slot.StorageBoxOffset <= 0 {
			return 0, 0
		}
		return slot.StorageBoxOffset + StorageHeaderSkip, StorageCommonCount
	}
	return 0, 0
}

// ── binary record helpers ────────────────────────────────────────────────────

// scanRecord linearly scans the CommonItems block for a record with the given
// handle. Returns (slot_index, quantity, true) on match.
func scanRecord(data []byte, start, slots int, handle uint32) (int, uint32, bool) {
	for i := 0; i < slots; i++ {
		off := start + i*InvRecordLen
		if off+InvRecordLen > len(data) {
			break
		}
		h := binary.LittleEndian.Uint32(data[off:])
		if h == handle {
			qty := binary.LittleEndian.Uint32(data[off+4:])
			return i, qty, true
		}
	}
	return -1, 0, false
}

// scanEmpty linearly scans for the first empty slot (handle == 0 or 0xFFFFFFFF).
func scanEmpty(data []byte, start, slots int) (int, bool) {
	for i := 0; i < slots; i++ {
		off := start + i*InvRecordLen
		if off+InvRecordLen > len(data) {
			break
		}
		h := binary.LittleEndian.Uint32(data[off:])
		if h == GaHandleEmpty || h == GaHandleInvalid {
			return i, true
		}
	}
	return -1, false
}

// writeRecord writes a full (handle, qty, index) record at slot i.
func writeRecord(data []byte, start, i int, handle, qty, idx uint32) {
	off := start + i*InvRecordLen
	binary.LittleEndian.PutUint32(data[off:], handle)
	binary.LittleEndian.PutUint32(data[off+4:], qty)
	binary.LittleEndian.PutUint32(data[off+8:], idx)
}

// writeRecordQty rewrites only the quantity field at slot i.
func writeRecordQty(data []byte, start, i int, qty uint32) {
	off := start + i*InvRecordLen + 4
	binary.LittleEndian.PutUint32(data[off:], qty)
}

// clearRecord zeroes the handle and qty fields at slot i. The index field is
// preserved on the Inventory side (matches existing RemoveItemFromSlot
// convention of writing the slot position as Index). Storage uses 0.
func clearRecord(data []byte, start, i int, c containerKind) {
	off := start + i*InvRecordLen
	binary.LittleEndian.PutUint32(data[off:], 0)
	binary.LittleEndian.PutUint32(data[off+4:], 0)
	if c == containerInventory {
		binary.LittleEndian.PutUint32(data[off+8:], uint32(i))
	} else {
		binary.LittleEndian.PutUint32(data[off+8:], 0)
	}
}

// ── index / counter helpers (mirrors addToInventory in writer.go) ────────────

// assignDestIndex returns the Index value to write at the destination slot,
// applying the same clamps as addToInventory for the corresponding side.
func assignDestIndex(slot *SaveSlot, c containerKind) uint32 {
	if c == containerInventory {
		acq := slot.Inventory.NextAcquisitionSortId
		if acq <= InvEquipReservedMax {
			acq = InvEquipReservedMax + 1
		}
		return acq
	}
	// Storage: use next_equip_index clamped above InvEquipReservedMax and above
	// max existing Index of any valid record (matches writer.go:766-790).
	nextListId := slot.Storage.NextEquipIndex
	if nextListId <= InvEquipReservedMax {
		nextListId = InvEquipReservedMax + 1
	}
	storageStart, slots := containerBinary(slot, containerStorage)
	if storageStart > 0 {
		for i := 0; i < slots; i++ {
			off := storageStart + i*InvRecordLen
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
	}
	return nextListId
}

// advanceDestCounters updates NextEquipIndex / NextAcquisitionSortId on the
// destination side after a fresh record write, mirroring the corresponding
// branches of addToInventory.
func advanceDestCounters(slot *SaveSlot, c containerKind, writtenIndex uint32) {
	if c == containerInventory {
		// NextEquipIndex must stay strictly > the just-written Index. If
		// NextAcquisitionSortId jumped ahead, lift NextEquipIndex to acqIdx+1.
		if writtenIndex >= slot.Inventory.NextEquipIndex {
			slot.Inventory.NextEquipIndex = writtenIndex + 1
		} else {
			slot.Inventory.NextEquipIndex++
		}
		// Ensure NextAcquisitionSortId moves forward past the value we just used.
		if slot.Inventory.NextAcquisitionSortId <= writtenIndex {
			slot.Inventory.NextAcquisitionSortId = writtenIndex + 1
		} else {
			slot.Inventory.NextAcquisitionSortId++
		}
		if slot.Inventory.nextEquipIndexOff > 0 {
			binary.LittleEndian.PutUint32(slot.Data[slot.Inventory.nextEquipIndexOff:], slot.Inventory.NextEquipIndex)
		}
		if slot.Inventory.nextAcqSortIdOff > 0 {
			binary.LittleEndian.PutUint32(slot.Data[slot.Inventory.nextAcqSortIdOff:], slot.Inventory.NextAcquisitionSortId)
		}
		return
	}
	// Storage
	slot.Storage.NextEquipIndex = writtenIndex + 1
	slot.Storage.NextAcquisitionSortId++
	if slot.Storage.nextEquipIndexOff > 0 {
		binary.LittleEndian.PutUint32(slot.Data[slot.Storage.nextEquipIndexOff:], slot.Storage.NextEquipIndex)
	}
	if slot.Storage.nextAcqSortIdOff > 0 {
		binary.LittleEndian.PutUint32(slot.Data[slot.Storage.nextAcqSortIdOff:], slot.Storage.NextAcquisitionSortId)
	}
}

// ── in-memory list helpers ───────────────────────────────────────────────────

// updateInventoryListAfterClear keeps slot.Inventory.CommonItems in sync with
// binary after a source record is cleared. The Inventory in-memory list is
// fully pre-allocated (index i maps to slot i), so positional update is safe.
// For Storage we rebuild the sparse list from binary at batch end via
// rescanStorageList.
func updateInventoryListAfterClear(slot *SaveSlot, c containerKind, idx int) {
	if c != containerInventory {
		return
	}
	if idx < 0 || idx >= len(slot.Inventory.CommonItems) {
		return
	}
	slot.Inventory.CommonItems[idx] = InventoryItem{GaItemHandle: 0, Quantity: 0, Index: uint32(idx)}
}

// updateInventoryListQty keeps slot.Inventory.CommonItems[idx].Quantity in sync
// with the binary after a partial transfer reduces the source quantity in
// place. Storage's sparse list is rebuilt via rescanStorageList.
func updateInventoryListQty(slot *SaveSlot, c containerKind, idx int, qty uint32) {
	if c != containerInventory {
		return
	}
	if idx < 0 || idx >= len(slot.Inventory.CommonItems) {
		return
	}
	slot.Inventory.CommonItems[idx].Quantity = qty
}

// rescanStorageList rebuilds slot.Storage.CommonItems from the binary CommonItems
// block, preserving the same sparse semantics as ReadStorage (skip empty entries).
func rescanStorageList(slot *SaveSlot) {
	start, slots := containerBinary(slot, containerStorage)
	if start <= 0 {
		return
	}
	list := slot.Storage.CommonItems[:0]
	for i := 0; i < slots; i++ {
		off := start + i*InvRecordLen
		if off+InvRecordLen > len(slot.Data) {
			break
		}
		h := binary.LittleEndian.Uint32(slot.Data[off:])
		if h == GaHandleEmpty || h == GaHandleInvalid {
			continue
		}
		qty := binary.LittleEndian.Uint32(slot.Data[off+4:])
		idx := binary.LittleEndian.Uint32(slot.Data[off+8:])
		list = append(list, InventoryItem{GaItemHandle: h, Quantity: qty, Index: idx})
	}
	slot.Storage.CommonItems = list
}

// rescanInventoryList rebuilds slot.Inventory.CommonItems from the binary
// CommonItems block. Inventory is fully pre-allocated (length = CommonItemCount,
// empty entries kept as zero-valued records), matching EquipInventoryData.Read
// semantics. Called at the end of a transfer batch so direct binary writes made
// after a rebuildAfterAllocation (or on non-rebuild paths) propagate to the
// in-memory list that vm.MapParsedSlotToVM reads.
func rescanInventoryList(slot *SaveSlot) {
	start, slots := containerBinary(slot, containerInventory)
	if start <= 0 {
		return
	}
	if len(slot.Inventory.CommonItems) != slots {
		slot.Inventory.CommonItems = make([]InventoryItem, slots)
	}
	for i := 0; i < slots; i++ {
		off := start + i*InvRecordLen
		if off+InvRecordLen > len(slot.Data) {
			break
		}
		h := binary.LittleEndian.Uint32(slot.Data[off:])
		q := binary.LittleEndian.Uint32(slot.Data[off+4:])
		idx := binary.LittleEndian.Uint32(slot.Data[off+8:])
		slot.Inventory.CommonItems[i] = InventoryItem{GaItemHandle: h, Quantity: q, Index: idx}
	}
}

// ── equipped check ───────────────────────────────────────────────────────────

// IsHandleEquipped reports whether the given handle is referenced by any slot
// of ChrAsmEquipment (equipped weapons / armor / talismans / arrows). The
// equipped block stores item-form IDs, so the check matches against multiple
// candidate representations of the handle:
//   - handle itself (defensive — some saves may store handles directly)
//   - GaMap[handle] (true item ID for weapons, armor, AoW)
//   - lower 28 bits with item-ID prefix (talismans 0xA0→0x20, goods 0xB0→0x40)
//
// Returns false when EquipItemsIDOffset is not parsed.
func IsHandleEquipped(slot *SaveSlot, handle uint32) bool {
	if slot == nil || slot.EquipItemsIDOffset <= 0 {
		return false
	}
	if slot.EquipItemsIDOffset+ChrAsmEquipmentSize > len(slot.Data) {
		return false
	}

	// ChrAsmEquipment uses an encoded item-ID form: for weapons/armor/AoW the
	// value is `itemID | 0x80000000` (the item ID resolved via GaMap with a
	// 0x80 high-bit flag). For talismans the value is the bare lower 28 bits
	// of the handle. Match against every plausible representation to keep the
	// check robust across upgrade levels and infusions.
	candidates := map[uint32]struct{}{
		handle:                                {},
		handle & 0x0FFFFFFF:                   {}, // bare lower bits (talismans)
		(handle & 0x0FFFFFFF) | 0x80000000:    {}, // bare lower bits + 0x80 flag
	}
	if id, ok := slot.GaMap[handle]; ok && id != 0 && id != GaHandleInvalid {
		candidates[id] = struct{}{}
		candidates[id|0x80000000] = struct{}{} // weapon/armor/AoW equipped form
	}
	prefix := handle & GaHandleTypeMask
	lower := handle & 0x0FFFFFFF
	switch prefix {
	case ItemTypeAccessory: // talisman handle 0xA0XXXXXX ↔ item ID 0x20XXXXXX
		candidates[lower|0x20000000] = struct{}{}
	case ItemTypeItem: // goods handle 0xB0XXXXXX ↔ item ID 0x40XXXXXX
		candidates[lower|0x40000000] = struct{}{}
	}

	for i := 0; i < ChrAsmFieldCount; i++ {
		off := slot.EquipItemsIDOffset + i*4
		v := binary.LittleEndian.Uint32(slot.Data[off:])
		if v == 0 || v == 0xFFFFFFFF {
			continue
		}
		if _, hit := candidates[v]; hit {
			return true
		}
	}
	return false
}

// ── rehandle / GaItem materialization ────────────────────────────────────────

// materializeRehandledInstance allocates a fresh unique handle and a GaItem
// record so the caller can write a separate inventory record for an instance
// that duplicates an existing handle on the destination side.
//
// The original record (at oldHandle) is NOT touched here. The caller must run
// rebuildAfterAllocation immediately afterwards so the new GaItem entry is
// reflected in binary, all dynamic offsets refresh, and slot.GaMap is
// repopulated. After rebuild, write the destination inventory record with the
// returned newHandle.
//
// itemID is resolved from slot.GaMap[oldHandle] first; for talisman (0xA0) and
// goods (0xB0) handle-encoded records that may not yet have a GaMap entry on
// freshly-loaded saves, we fall back to deriving itemID from the lower 28 bits
// via the prefix swap used by HandleToItemID.
func materializeRehandledInstance(slot *SaveSlot, oldHandle uint32) (uint32, error) {
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
			return 0, fmt.Errorf("rehandle: itemID for handle 0x%08X not in GaMap", oldHandle)
		}
	}
	newHandle, err := generateUniqueHandle(slot, prefix)
	if err != nil {
		return 0, err
	}
	if err := allocateGaItem(slot, newHandle, itemID); err != nil {
		return 0, err
	}
	slot.GaMap[newHandle] = itemID
	return newHandle, nil
}

// rebuildAfterAllocation re-serializes slot binary so the in-memory GaItem
// changes made by allocateGaItem are persisted, then re-parses the result so
// downstream offsets (inventory/storage record starts, counter positions)
// refresh. Mirrors Phase 2 of AddItemsToSlot.
//
// parseFromData rebuilds GaMap purely from the GaItem array — stackable
// handle-encoded entries that have no backing GaItem record are dropped on
// re-parse. We snapshot the GaMap before rebuild and merge the dropped
// entries back afterwards, matching the protective pattern in
// AddItemsToSlot (writer.go:207-250).
func rebuildAfterAllocation(slot *SaveSlot) error {
	savedGaMap := make(map[uint32]uint32, len(slot.GaMap))
	for k, v := range slot.GaMap {
		savedGaMap[k] = v
	}
	savedNextAoW := slot.NextAoWIndex
	savedNextArmament := slot.NextArmamentIndex
	savedNextHandle := slot.NextGaItemHandle

	rebuilt, err := RebuildSlotFull(slot)
	if err != nil {
		return fmt.Errorf("rebuildAfterAllocation: rebuild: %w", err)
	}
	copy(slot.Data, rebuilt)
	if err := slot.parseFromData(); err != nil {
		return fmt.Errorf("rebuildAfterAllocation: re-parse: %w", err)
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
	return nil
}
