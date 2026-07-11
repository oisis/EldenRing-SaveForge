package editor

import "fmt"

// MoveItem relocates an editable item inside the workspace.
//
// The item is located by UID across both InventoryItems and StorageItems,
// removed from its current slice, has its Container field updated, and
// is re-inserted into the target slice at the requested position.
// Pass-through (unsupported) records are never touched.
//
// targetPosition is clamped — values below 0 land at 0, values past the
// (post-removal) target length append at the end. Positions are
// recomputed for both editable slices after the move so the snapshot
// remains consistent. The snapshot is marked Dirty and Validate is
// re-run; the resulting report is stored on snap.Validation.
//
// Returns an error if the snapshot is nil, the target container is not
// "inventory"/"storage", or the UID does not match any editable item.
func MoveItem(snap *InventoryWorkspaceSnapshot, uid string, targetContainer ContainerKind, targetPosition int) error {
	if snap == nil {
		return fmt.Errorf("MoveItem: nil snapshot")
	}
	if targetContainer != ContainerInventory && targetContainer != ContainerStorage {
		return fmt.Errorf("MoveItem: invalid target container %q (want %q or %q)",
			targetContainer, ContainerInventory, ContainerStorage)
	}

	srcKind, srcIdx, found := findEditable(snap, uid)
	if !found {
		return fmt.Errorf("MoveItem: item %q not found in workspace", uid)
	}

	// Capture by value before slice mutation: source and target may be
	// the same slice header, and `append` can reuse the underlying array.
	srcSlice := sliceFor(snap, srcKind)
	item := (*srcSlice)[srcIdx]
	*srcSlice = append((*srcSlice)[:srcIdx], (*srcSlice)[srcIdx+1:]...)

	item.Container = targetContainer

	dstSlice := sliceFor(snap, targetContainer)
	if targetPosition < 0 {
		targetPosition = 0
	}
	if targetPosition > len(*dstSlice) {
		targetPosition = len(*dstSlice)
	}

	// Insert at targetPosition. Two appends keep semantics readable.
	tail := append([]EditableItem{item}, (*dstSlice)[targetPosition:]...)
	*dstSlice = append((*dstSlice)[:targetPosition], tail...)

	recomputePositions(snap.InventoryItems)
	recomputePositions(snap.StorageItems)

	snap.Dirty = true
	snap.Validation = Validate(*snap)
	return nil
}

// ReorderItems atomically replaces the ordering of BOTH editable
// containers from full desired UID lists.
//
// Each list must be an exact permutation of the editable items currently
// in that container: same length, no duplicates, no missing UID, no
// foreign UID, and no UID belonging to the other container. Both lists
// are validated in full before anything is mutated, so a rejected request
// leaves both containers byte-for-byte unchanged. Pass-through
// (unsupported) records, quantities and item data are never touched.
//
// On success the editable Inventory and Storage slices are rebuilt in the
// requested order, positions are recomputed, Dirty is set, and Validate
// runs exactly once.
func ReorderItems(snap *InventoryWorkspaceSnapshot, inventoryUIDs, storageUIDs []string) error {
	if snap == nil {
		return fmt.Errorf("ReorderItems: nil snapshot")
	}
	invOrder, err := reorderPermutation(snap.InventoryItems, inventoryUIDs, ContainerInventory)
	if err != nil {
		return err
	}
	stoOrder, err := reorderPermutation(snap.StorageItems, storageUIDs, ContainerStorage)
	if err != nil {
		return err
	}

	// Both lists validated — safe to mutate now.
	snap.InventoryItems = invOrder
	snap.StorageItems = stoOrder
	recomputePositions(snap.InventoryItems)
	recomputePositions(snap.StorageItems)

	snap.Dirty = true
	snap.Validation = Validate(*snap)
	return nil
}

// reorderPermutation validates that wantUIDs is an exact permutation of
// the UIDs in items and returns the reordered slice. It mutates nothing on
// error so the caller can validate both containers before touching either.
func reorderPermutation(items []EditableItem, wantUIDs []string, container ContainerKind) ([]EditableItem, error) {
	if len(wantUIDs) != len(items) {
		return nil, fmt.Errorf("ReorderItems: %s expects %d uids, got %d", container, len(items), len(wantUIDs))
	}
	byUID := make(map[string]EditableItem, len(items))
	for _, it := range items {
		byUID[it.UID] = it
	}
	out := make([]EditableItem, 0, len(items))
	seen := make(map[string]bool, len(items))
	for _, uid := range wantUIDs {
		if seen[uid] {
			return nil, fmt.Errorf("ReorderItems: %s has duplicate uid %q", container, uid)
		}
		it, ok := byUID[uid]
		if !ok {
			return nil, fmt.Errorf("ReorderItems: %s has unknown uid %q", container, uid)
		}
		seen[uid] = true
		out = append(out, it)
	}
	return out, nil
}

// RemoveItem deletes an editable item from the workspace by UID.
//
// Pass-through records are never affected. Positions are recomputed for
// both editable slices after the removal. Dirty is set and Validate is
// re-run. Returns an error if the snapshot is nil or the UID is unknown.
func RemoveItem(snap *InventoryWorkspaceSnapshot, uid string) error {
	if snap == nil {
		return fmt.Errorf("RemoveItem: nil snapshot")
	}
	srcKind, idx, found := findEditable(snap, uid)
	if !found {
		return fmt.Errorf("RemoveItem: item %q not found in workspace", uid)
	}
	slice := sliceFor(snap, srcKind)
	*slice = append((*slice)[:idx], (*slice)[idx+1:]...)

	recomputePositions(snap.InventoryItems)
	recomputePositions(snap.StorageItems)

	snap.Dirty = true
	snap.Validation = Validate(*snap)
	return nil
}

// findEditable locates an editable item by UID. Inventory is searched
// before Storage; the first match wins.
func findEditable(snap *InventoryWorkspaceSnapshot, uid string) (ContainerKind, int, bool) {
	for i := range snap.InventoryItems {
		if snap.InventoryItems[i].UID == uid {
			return ContainerInventory, i, true
		}
	}
	for i := range snap.StorageItems {
		if snap.StorageItems[i].UID == uid {
			return ContainerStorage, i, true
		}
	}
	return "", 0, false
}

// sliceFor returns a pointer to the snapshot slice for a container kind.
// Caller is responsible for verifying the kind is valid first.
func sliceFor(snap *InventoryWorkspaceSnapshot, kind ContainerKind) *[]EditableItem {
	if kind == ContainerInventory {
		return &snap.InventoryItems
	}
	return &snap.StorageItems
}

// recomputePositions reassigns each EditableItem.Position to its current
// 0-based index in the slice.
func recomputePositions(items []EditableItem) {
	for i := range items {
		items[i].Position = i
	}
}
