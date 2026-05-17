package editor

import (
	"encoding/binary"
	"fmt"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
	"github.com/oisis/EldenRing-SaveForge/backend/db"
)

// HandlesByUID maps editor UIDs to allocated GaItem handles after a
// successful ApplyWorkspaceSave. UID format for existing items is
// "hnd:0x%08X" (handle never changes — kept stable across transfers and
// reorders) and "new:N" for added items (handle freshly minted by
// AddItemsToSlotBatch). Removed items are absent from this map.
// Callers use this to follow the same item through subsequent edits.
type HandlesByUID map[string]uint32

// ApplyWorkspaceSave is the Phase 3B commit path. It validates the
// workspace, rejects changes still out of scope (pending AoW), then
// writes a reorder + add + transfer + remove + weapon-upgrade plan
// into slot.Data via the wipe-and-replay layout.
//
// In scope for Phase 3B (extends Phase 3A):
//   - reorder existing editable items within their original container
//   - transfer existing editable items between Inventory and Storage —
//     OriginalHandle is preserved, the GaItem stays in place, and only
//     the inventory record is moved; old record is wiped by the layout
//     rebuild
//   - remove existing editable items from the workspace — their record
//     is absent from both containers post-save; GaItem stays in
//     slot.GaItems / slot.GaMap (conservative no-GC policy — see notes
//     under "GaItem GC policy" below)
//   - add new editable items (Source=Added) — allocates real handles
//     and GaItem entries via core.AddItemsToSlotBatch
//   - patch weapon ItemID for existing items with upgrade/infusion
//     changes via core.PatchWeaponItemID (works correctly for
//     transferred weapons too — patch keys on handle, not container)
//   - preserve unsupported/pass-through records at their original
//     physical SlotIndex
//
// Still rejected with clear errors (slot.Data left untouched):
//   - workspace validation errors
//   - any EditableItem with PendingAoWItemID != 0 (Phase 4)
//   - missing baseline data (session created without baseline tracking)
//   - inventory or storage capacity exceeded by the final layout
//   - pass-through SlotIndex collisions
//
// GaItem GC policy (Phase 3B):
//   - Removed items leave their GaItem record orphaned in slot.GaItems
//     and slot.GaMap. No record in slot.Data references them after the
//     layout rebuild, so they do not show up in any container.
//   - We do NOT call RepairOrphanedGaItems automatically. The trade-off
//     is a small amount of wasted GaItem array space versus the safety
//     risk of an under-tested GC sweep mutating shared state.
//   - Future Phase 4+ may add explicit GC when AoW work lands and the
//     ownership model is fully understood.
//
// Atomicity contract:
//   - Callers MUST snapshot slot via core.SnapshotSlot BEFORE calling
//     this function and call core.RestoreSlot on a non-nil error to
//     roll back partial state.
//   - This function does NOT manage its own undo. It only guarantees
//     all rejection checks run BEFORE any mutation; if a check fails,
//     slot.Data is byte-identical to the input.
//   - Once writes begin (after the rejection block), an error means
//     slot.Data has been partially mutated. Caller MUST roll back.
func ApplyWorkspaceSave(slot *core.SaveSlot, snap *InventoryWorkspaceSnapshot, baseline map[uint32]ContainerKind) (HandlesByUID, error) {
	if slot == nil || snap == nil {
		return nil, fmt.Errorf("ApplyWorkspaceSave: nil slot or snapshot")
	}

	// ── Pre-flight rejection checks (no mutation) ─────────────────
	rep := Validate(*snap)
	if !rep.OK {
		return nil, fmt.Errorf("ApplyWorkspaceSave: workspace fails validation: %d error(s) (first: %s)",
			len(rep.Errors), rep.Errors[0].Message)
	}

	if err := rejectPendingAoW(snap); err != nil {
		return nil, err
	}
	if baseline == nil {
		return nil, fmt.Errorf("ApplyWorkspaceSave: session missing baseline handle map (Phase 3B requires a session created with baseline tracking)")
	}
	// Note: transfer and remove are now in-scope. We still compute the
	// plans up-front so any future per-action validation has a single
	// well-tested entry point. The wipe-and-replay layout below
	// naturally realises these plans without needing per-record patches:
	// removed items don't appear in either container's editable list,
	// so they're not re-emitted; transferred items appear in their new
	// container's editable list, so they're emitted there with their
	// original handle preserved.
	_ = detectRemovedEditableHandles(snap, baseline)
	_ = detectTransferredEditableItems(snap, baseline)

	// Capacity pre-check (also runs in writeContainerLayout, but doing
	// it up-front keeps slot.Data untouched on rejection).
	if invTotal := len(snap.InventoryItems) + len(snap.UnsupportedInventoryRecords); invTotal > core.CommonItemCount {
		return nil, fmt.Errorf("ApplyWorkspaceSave: inventory capacity exceeded: %d > %d",
			invTotal, core.CommonItemCount)
	}
	if stoTotal := len(snap.StorageItems) + len(snap.UnsupportedStorageRecords); stoTotal > core.StorageCommonCount {
		return nil, fmt.Errorf("ApplyWorkspaceSave: storage capacity exceeded: %d > %d",
			stoTotal, core.StorageCommonCount)
	}

	// Pass-through SlotIndex uniqueness check (before any writes).
	if err := validatePassThroughIndices(snap.UnsupportedInventoryRecords, core.CommonItemCount, "inventory"); err != nil {
		return nil, err
	}
	if err := validatePassThroughIndices(snap.UnsupportedStorageRecords, core.StorageCommonCount, "storage"); err != nil {
		return nil, err
	}

	// ── Step 1: add new editable items ────────────────────────────
	handles := HandlesByUID{}
	if err := executeAdds(slot, snap, handles); err != nil {
		return nil, fmt.Errorf("ApplyWorkspaceSave: %w", err)
	}

	// Populate handles map for existing items (so callers always have
	// a UID → handle entry per editable record).
	for _, it := range snap.InventoryItems {
		if it.Source == ItemSourceOriginal && it.OriginalHandle != 0 {
			handles[it.UID] = it.OriginalHandle
		}
	}
	for _, it := range snap.StorageItems {
		if it.Source == ItemSourceOriginal && it.OriginalHandle != 0 {
			handles[it.UID] = it.OriginalHandle
		}
	}

	// ── Step 2: patch weapon ItemID for upgrade/infusion changes ──
	if err := executeWeaponPatches(slot, snap.InventoryItems); err != nil {
		return nil, fmt.Errorf("ApplyWorkspaceSave: %w", err)
	}
	if err := executeWeaponPatches(slot, snap.StorageItems); err != nil {
		return nil, fmt.Errorf("ApplyWorkspaceSave: %w", err)
	}

	// ── Step 3: wipe + replay record layouts in both containers ───
	if err := writeContainerLayout(slot, snap, ContainerInventory); err != nil {
		return nil, fmt.Errorf("ApplyWorkspaceSave: write inventory: %w", err)
	}
	if err := writeContainerLayout(slot, snap, ContainerStorage); err != nil {
		return nil, fmt.Errorf("ApplyWorkspaceSave: write storage: %w", err)
	}

	// Header reconciliation — mirrors what legacy ReorderStorage does.
	core.ReconcileInventoryHeader(slot)
	core.ReconcileStorageHeader(slot)

	return handles, nil
}

// rejectPendingAoW returns an error if any editable item carries a
// pending AoW request. Phase 3B still cannot allocate AoW handles —
// pending AoW save lands in Phase 4 alongside compatibility checks.
func rejectPendingAoW(snap *InventoryWorkspaceSnapshot) error {
	for _, list := range [][]EditableItem{snap.InventoryItems, snap.StorageItems} {
		for _, it := range list {
			if it.PendingAoWItemID != 0 {
				return fmt.Errorf("ApplyWorkspaceSave: pending AoW save not implemented (item %s, UID %s, pending AoW 0x%08X)",
					it.Name, it.UID, it.PendingAoWItemID)
			}
		}
	}
	return nil
}

// currentEditableContainerMap collapses both editable container slices
// into a (handle → container) map for Source=Original items with a real
// handle. Added items and zero handles are skipped — they're not
// represented in the baseline. Shared by both detection helpers.
func currentEditableContainerMap(snap *InventoryWorkspaceSnapshot) map[uint32]ContainerKind {
	out := make(map[uint32]ContainerKind, len(snap.InventoryItems)+len(snap.StorageItems))
	for _, it := range snap.InventoryItems {
		if it.Source == ItemSourceOriginal && it.OriginalHandle != 0 {
			out[it.OriginalHandle] = ContainerInventory
		}
	}
	for _, it := range snap.StorageItems {
		if it.Source == ItemSourceOriginal && it.OriginalHandle != 0 {
			out[it.OriginalHandle] = ContainerStorage
		}
	}
	return out
}

// detectRemovedEditableHandles returns the baseline handles that no
// longer appear in the workspace's editable lists. Added items are not
// considered — they're not in the baseline to begin with. Used by
// callers that want to know which removals are about to be committed
// (e.g., for future GC bookkeeping or telemetry); the actual record
// removal happens implicitly via the wipe-and-replay layout, which
// only emits items present in the workspace.
func detectRemovedEditableHandles(snap *InventoryWorkspaceSnapshot, baseline map[uint32]ContainerKind) []uint32 {
	if baseline == nil {
		return nil
	}
	current := currentEditableContainerMap(snap)
	var removed []uint32
	for h := range baseline {
		if _, present := current[h]; !present {
			removed = append(removed, h)
		}
	}
	return removed
}

// detectTransferredEditableItems returns the baseline handles whose
// container differs in the current workspace, mapped to their new
// container. Added items are ignored (no baseline entry). Reorder
// within the same container is not flagged. Used the same way as
// detectRemovedEditableHandles — informational; the wipe-and-replay
// layout realises the move by emitting the item in its new container.
func detectTransferredEditableItems(snap *InventoryWorkspaceSnapshot, baseline map[uint32]ContainerKind) map[uint32]ContainerKind {
	if baseline == nil {
		return nil
	}
	current := currentEditableContainerMap(snap)
	out := map[uint32]ContainerKind{}
	for h, orig := range baseline {
		cur, present := current[h]
		if !present {
			continue
		}
		if cur != orig {
			out[h] = cur
		}
	}
	return out
}

// validatePassThroughIndices ensures the pass-through SlotIndices fit
// the container and don't collide. Runs before any binary writes.
func validatePassThroughIndices(records []RawInventoryRecord, capacity int, kindName string) error {
	seen := make(map[int]uint32, len(records))
	for _, p := range records {
		if p.SlotIndex < 0 || p.SlotIndex >= capacity {
			return fmt.Errorf("ApplyWorkspaceSave: %s pass-through SlotIndex %d out of range [0,%d)",
				kindName, p.SlotIndex, capacity)
		}
		if other, dup := seen[p.SlotIndex]; dup {
			return fmt.Errorf("ApplyWorkspaceSave: %s pass-through SlotIndex %d duplicated (handles 0x%08X and 0x%08X)",
				kindName, p.SlotIndex, other, p.Handle)
		}
		seen[p.SlotIndex] = p.Handle
	}
	return nil
}

// executeAdds materialises each Source=Added EditableItem in slot by
// calling core.AddItemsToSlotBatch one item at a time. The diff against
// pre-call GaMap yields the freshly minted handle for that item.
//
// Single-item batching makes handle attribution unambiguous: two
// concurrently-added weapons of the same itemID would otherwise produce
// two new GaMap entries we couldn't tell apart by content alone.
//
// For stackable items (talisman / goods) that already have a synthetic
// handle in GaMap, we reuse it directly and skip AddItemsToSlotBatch.
// The wipe-and-replay step then writes a record with that handle at
// our chosen position with the workspace's quantity.
func executeAdds(slot *core.SaveSlot, snap *InventoryWorkspaceSnapshot, handles HandlesByUID) error {
	// Gather pointers so we can mutate OriginalHandle in place.
	added := []*EditableItem{}
	for i := range snap.InventoryItems {
		if snap.InventoryItems[i].Source == ItemSourceAdded {
			added = append(added, &snap.InventoryItems[i])
		}
	}
	for i := range snap.StorageItems {
		if snap.StorageItems[i].Source == ItemSourceAdded {
			added = append(added, &snap.StorageItems[i])
		}
	}

	for _, it := range added {
		handlePrefix := db.ItemIDToHandlePrefix(it.ItemID)
		isStackable := handlePrefix == core.ItemTypeItem || handlePrefix == core.ItemTypeAccessory

		// Stackable + already in GaMap: reuse existing handle, no allocation.
		if isStackable {
			if reused, ok := findHandleForItemID(slot.GaMap, it.ItemID); ok {
				it.OriginalHandle = reused
				handles[it.UID] = reused
				continue
			}
		}

		gaMapBefore := snapshotGaMap(slot.GaMap)
		req := core.ItemToAdd{ItemID: it.ItemID}
		if it.Container == ContainerInventory {
			req.InvQty = int(it.Quantity)
		} else {
			req.StorageQty = int(it.Quantity)
		}
		if err := core.AddItemsToSlotBatch(slot, []core.ItemToAdd{req}); err != nil {
			return fmt.Errorf("add %s (0x%08X): %w", it.Name, it.ItemID, err)
		}
		newH, err := pickNewHandle(slot.GaMap, gaMapBefore, it.ItemID)
		if err != nil {
			return fmt.Errorf("identify new handle for %s: %w", it.Name, err)
		}
		it.OriginalHandle = newH
		handles[it.UID] = newH
	}
	return nil
}

// executeWeaponPatches applies upgrade / infusion changes for existing
// (Source=Original) weapon items whose workspace ItemID has diverged
// from the current slot.GaMap binding. Non-weapons are skipped silently
// (Phase 1.7 only supports weapon edits anyway).
func executeWeaponPatches(slot *core.SaveSlot, items []EditableItem) error {
	for _, it := range items {
		if it.Source != ItemSourceOriginal || it.OriginalHandle == 0 {
			continue
		}
		if !it.IsWeapon {
			continue
		}
		currentID, ok := slot.GaMap[it.OriginalHandle]
		if !ok {
			continue
		}
		if currentID == it.ItemID {
			continue
		}
		if err := core.PatchWeaponItemID(slot, it.OriginalHandle, currentID, it.ItemID); err != nil {
			return fmt.Errorf("patch weapon 0x%08X (%s) %X→%X: %w",
				it.OriginalHandle, it.Name, currentID, it.ItemID, err)
		}
	}
	return nil
}

// writeContainerLayout wipes the entire CommonItems region of one
// container in slot.Data and rewrites it as:
//   - pass-through records pinned at their original SlotIndex with
//     their original AcquisitionIndex
//   - editable records placed at the next free physical slot in
//     workspace Position order, with fresh stride-2 AcquisitionIndex
//     values starting just above the current NextAcquisitionSortId
//
// Pre-conditions enforced before reaching this function (in
// ApplyWorkspaceSave): pass-through SlotIndex uniqueness, capacity.
//
// In-memory state:
//   - Inventory uses a pre-sized CommonItems array of length
//     CommonItemCount; entries at non-occupied slots remain zero
//     (handle = GaHandleEmpty).
//   - Storage uses a compacted CommonItems list (only non-empty
//     entries, in physical slot order) — matching ReadStorage's parse
//     convention.
func writeContainerLayout(slot *core.SaveSlot, snap *InventoryWorkspaceSnapshot, kind ContainerKind) error {
	var (
		editables   []EditableItem
		passthrough []RawInventoryRecord
		startOff    int
		capacity    int
		equip       *core.EquipInventoryData
	)
	if kind == ContainerInventory {
		editables = snap.InventoryItems
		passthrough = snap.UnsupportedInventoryRecords
		if slot.MagicOffset <= 0 {
			if len(editables) == 0 && len(passthrough) == 0 {
				return nil
			}
			return fmt.Errorf("inventory MagicOffset=0 but workspace has %d editable + %d pass-through",
				len(editables), len(passthrough))
		}
		startOff = slot.MagicOffset + core.InvStartFromMagic
		capacity = core.CommonItemCount
		equip = &slot.Inventory
	} else {
		editables = snap.StorageItems
		passthrough = snap.UnsupportedStorageRecords
		if slot.StorageBoxOffset == 0 {
			if len(editables) == 0 && len(passthrough) == 0 {
				return nil
			}
			return fmt.Errorf("storage offset=0 but workspace has %d editable + %d pass-through",
				len(editables), len(passthrough))
		}
		startOff = slot.StorageBoxOffset + core.StorageHeaderSkip
		capacity = core.StorageCommonCount
		equip = &slot.Storage
	}

	// Build occupied (pass-through pinned positions) and reserved
	// acquisition indices so the stride-2 base doesn't collide.
	occupied := make(map[int]bool, len(passthrough))
	reservedAcq := make(map[uint32]bool, len(passthrough))
	for _, p := range passthrough {
		occupied[p.SlotIndex] = true
		if p.Handle != core.GaHandleEmpty && p.Handle != core.GaHandleInvalid {
			reservedAcq[p.AcquisitionIndex] = true
		}
	}

	// Compute stride-2 base for editable items.
	baseAcq := equip.NextAcquisitionSortId
	if baseAcq <= core.InvEquipReservedMax {
		baseAcq = core.InvEquipReservedMax + 1
	}
	if baseAcq%2 != 0 {
		baseAcq++
	}
	for {
		collision := false
		for i := 0; i < len(editables); i++ {
			if reservedAcq[baseAcq+uint32(i*2)] {
				collision = true
				break
			}
		}
		if !collision {
			break
		}
		baseAcq += 2
	}

	// Verify slot.Data is large enough.
	endOff := startOff + capacity*core.InvRecordLen
	if endOff > len(slot.Data) {
		return fmt.Errorf("slot.Data too short for %s container (%d < %d)", kind, len(slot.Data), endOff)
	}

	// Wipe all CommonItems bytes.
	for i := startOff; i < endOff; i++ {
		slot.Data[i] = 0
	}

	// Write pass-through records.
	for _, p := range passthrough {
		off := startOff + p.SlotIndex*core.InvRecordLen
		binary.LittleEndian.PutUint32(slot.Data[off:], p.Handle)
		binary.LittleEndian.PutUint32(slot.Data[off+4:], p.Quantity)
		binary.LittleEndian.PutUint32(slot.Data[off+8:], p.AcquisitionIndex)
	}

	// Write editable records at next free physical slot.
	maxAcq := uint32(0)
	for _, p := range passthrough {
		if p.AcquisitionIndex > maxAcq {
			maxAcq = p.AcquisitionIndex
		}
	}
	nextFree := 0
	for pos, it := range editables {
		for nextFree < capacity && occupied[nextFree] {
			nextFree++
		}
		if nextFree >= capacity {
			return fmt.Errorf("%s container ran out of free slots after placing pass-through", kind)
		}
		acq := baseAcq + uint32(pos*2)
		off := startOff + nextFree*core.InvRecordLen
		binary.LittleEndian.PutUint32(slot.Data[off:], it.OriginalHandle)
		binary.LittleEndian.PutUint32(slot.Data[off+4:], it.Quantity)
		binary.LittleEndian.PutUint32(slot.Data[off+8:], acq)
		if acq > maxAcq {
			maxAcq = acq
		}
		nextFree++
	}

	// Rebuild in-memory CommonItems from binary.
	rebuildInMemoryCommonItems(slot, startOff, capacity, equip, kind == ContainerInventory)

	// Update counters in-memory and in binary.
	newNext := maxAcq + 1
	equip.NextEquipIndex = newNext
	equip.NextAcquisitionSortId = newNext
	if off := equip.NextEquipIndexOff(); off > 0 && off+8 <= len(slot.Data) {
		binary.LittleEndian.PutUint32(slot.Data[off:], newNext)   // NextEquipIndex
		binary.LittleEndian.PutUint32(slot.Data[off+4:], newNext) // NextAcquisitionSortId (adjacent u32)
	}

	return nil
}

// rebuildInMemoryCommonItems syncs equip.CommonItems with the freshly
// written slot.Data records. Inventory keeps a fixed-size array (one
// entry per physical slot, empties as zero handles); storage uses the
// compacted ReadStorage convention.
func rebuildInMemoryCommonItems(slot *core.SaveSlot, startOff, capacity int, equip *core.EquipInventoryData, fullSize bool) {
	if fullSize {
		out := make([]core.InventoryItem, capacity)
		for i := 0; i < capacity; i++ {
			off := startOff + i*core.InvRecordLen
			out[i] = core.InventoryItem{
				GaItemHandle: binary.LittleEndian.Uint32(slot.Data[off:]),
				Quantity:     binary.LittleEndian.Uint32(slot.Data[off+4:]),
				Index:        binary.LittleEndian.Uint32(slot.Data[off+8:]),
			}
		}
		equip.CommonItems = out
		return
	}
	out := []core.InventoryItem{}
	for i := 0; i < capacity; i++ {
		off := startOff + i*core.InvRecordLen
		h := binary.LittleEndian.Uint32(slot.Data[off:])
		if h == core.GaHandleEmpty || h == core.GaHandleInvalid {
			continue
		}
		out = append(out, core.InventoryItem{
			GaItemHandle: h,
			Quantity:     binary.LittleEndian.Uint32(slot.Data[off+4:]),
			Index:        binary.LittleEndian.Uint32(slot.Data[off+8:]),
		})
	}
	equip.CommonItems = out
}

// snapshotGaMap captures a shallow copy of slot.GaMap before
// AddItemsToSlotBatch. The diff after lets executeAdds identify the
// fresh handle minted for the added EditableItem.
func snapshotGaMap(m map[uint32]uint32) map[uint32]uint32 {
	out := make(map[uint32]uint32, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// pickNewHandle returns the handle present in `after` but not `before`,
// where after[h] == wantID. Errors on zero or multiple candidates so
// the caller never silently grabs the wrong handle.
func pickNewHandle(after, before map[uint32]uint32, wantID uint32) (uint32, error) {
	var candidates []uint32
	for h, id := range after {
		if _, hadBefore := before[h]; hadBefore {
			continue
		}
		if id != wantID {
			continue
		}
		candidates = append(candidates, h)
	}
	if len(candidates) == 0 {
		return 0, fmt.Errorf("no new handle minted for itemID 0x%08X", wantID)
	}
	if len(candidates) > 1 {
		return 0, fmt.Errorf("ambiguous new handles for itemID 0x%08X: %d candidates", wantID, len(candidates))
	}
	return candidates[0], nil
}

// findHandleForItemID looks up a stackable handle already present in
// GaMap for the requested itemID. Used for stackable adds (talisman /
// goods) so we reuse the existing synthetic handle instead of letting
// AddItemsToSlotBatch attempt a fresh allocation.
func findHandleForItemID(m map[uint32]uint32, itemID uint32) (uint32, bool) {
	for h, id := range m {
		if id == itemID {
			return h, true
		}
	}
	return 0, false
}
