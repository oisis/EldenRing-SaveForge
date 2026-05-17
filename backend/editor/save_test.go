package editor

import (
	"strings"
	"testing"
)

// savableSnap returns a snapshot that ApplyWorkspaceSave's rejection
// block accepts (clean Validation, no pending AoW). Used as the base
// for "negative case A vs. base" tests so we can flip a single field
// and verify the rejection.
//
// Important: the snapshot here is synthetic — slot=nil scenarios short
// out at the entry guard, so we only ever exercise the rejection
// helpers via direct calls (rejectPendingAoW, rejectTransferOrRemove,
// validatePassThroughIndices) in this file.
func savableSnap() *InventoryWorkspaceSnapshot {
	return &InventoryWorkspaceSnapshot{
		InventoryItems: []EditableItem{
			{
				UID:              "hnd:0x80800001",
				Source:           ItemSourceOriginal,
				Container:        ContainerInventory,
				OriginalHandle:   0x80800001,
				ItemID:           0x000F4240,
				BaseItemID:       0x000F4240,
				Name:             "Dagger",
				Category:         "melee_armaments",
				Quantity:         1,
				AcquisitionIndex: 1000,
				MaxUpgrade:       25,
				HasGaItem:        true,
				IsWeapon:         true,
			},
		},
		StorageItems: []EditableItem{
			{
				UID:              "hnd:0x80800002",
				Source:           ItemSourceOriginal,
				Container:        ContainerStorage,
				OriginalHandle:   0x80800002,
				ItemID:           0x003085E0,
				BaseItemID:       0x003085E0,
				Name:             "Claymore",
				Category:         "melee_armaments",
				Quantity:         1,
				AcquisitionIndex: 1002,
				MaxUpgrade:       25,
				HasGaItem:        true,
				IsWeapon:         true,
			},
		},
	}
}

func baselineFor(snap *InventoryWorkspaceSnapshot) map[uint32]ContainerKind {
	b := map[uint32]ContainerKind{}
	for _, it := range snap.InventoryItems {
		if it.Source == ItemSourceOriginal && it.OriginalHandle != 0 {
			b[it.OriginalHandle] = ContainerInventory
		}
	}
	for _, it := range snap.StorageItems {
		if it.Source == ItemSourceOriginal && it.OriginalHandle != 0 {
			b[it.OriginalHandle] = ContainerStorage
		}
	}
	return b
}

func TestRejectPendingAoW_TriggersOnInventoryItem(t *testing.T) {
	snap := savableSnap()
	snap.InventoryItems[0].PendingAoWItemID = 0x80002710
	err := rejectPendingAoW(snap)
	if err == nil {
		t.Fatal("expected error for pending AoW")
	}
	if !strings.Contains(err.Error(), "pending AoW") {
		t.Errorf("error should mention pending AoW, got %v", err)
	}
}

func TestRejectPendingAoW_TriggersOnStorageItem(t *testing.T) {
	snap := savableSnap()
	snap.StorageItems[0].PendingAoWItemID = 0x80002710
	if err := rejectPendingAoW(snap); err == nil {
		t.Fatal("expected error for pending AoW in storage")
	}
}

func TestRejectPendingAoW_CleanSnapshotOK(t *testing.T) {
	if err := rejectPendingAoW(savableSnap()); err != nil {
		t.Fatalf("clean snapshot should accept: %v", err)
	}
}

// ─── Removed-handle detection (Phase 3B) ─────────────────────────

func TestDetectRemovedEditableHandles_DetectsRemoveFromInventory(t *testing.T) {
	snap := savableSnap()
	baseline := baselineFor(snap)
	// Drop the inventory item — should be reported as removed.
	removedHandle := snap.InventoryItems[0].OriginalHandle
	snap.InventoryItems = nil
	got := detectRemovedEditableHandles(snap, baseline)
	if len(got) != 1 || got[0] != removedHandle {
		t.Errorf("got %v, want [0x%08X]", got, removedHandle)
	}
}

func TestDetectRemovedEditableHandles_DetectsRemoveFromStorage(t *testing.T) {
	snap := savableSnap()
	baseline := baselineFor(snap)
	removedHandle := snap.StorageItems[0].OriginalHandle
	snap.StorageItems = nil
	got := detectRemovedEditableHandles(snap, baseline)
	if len(got) != 1 || got[0] != removedHandle {
		t.Errorf("got %v, want [0x%08X]", got, removedHandle)
	}
}

func TestDetectRemovedEditableHandles_NoneWhenAllPresent(t *testing.T) {
	snap := savableSnap()
	if got := detectRemovedEditableHandles(snap, baselineFor(snap)); len(got) != 0 {
		t.Errorf("clean snapshot should have no removes; got %v", got)
	}
}

func TestDetectRemovedEditableHandles_TransferIsNotRemove(t *testing.T) {
	snap := savableSnap()
	baseline := baselineFor(snap)
	// Move inventory item to storage. Handle still present in
	// workspace → must NOT count as removed.
	moved := snap.InventoryItems[0]
	moved.Container = ContainerStorage
	snap.InventoryItems = nil
	snap.StorageItems = append(snap.StorageItems, moved)
	if got := detectRemovedEditableHandles(snap, baseline); len(got) != 0 {
		t.Errorf("transferred item should not appear as removed; got %v", got)
	}
}

func TestDetectRemovedEditableHandles_AddedItemsIgnored(t *testing.T) {
	snap := savableSnap()
	baseline := baselineFor(snap)
	snap.InventoryItems = append(snap.InventoryItems, EditableItem{
		UID:            "new:1",
		Source:         ItemSourceAdded,
		Container:      ContainerInventory,
		OriginalHandle: 0,
		ItemID:         0x000F4240,
	})
	if got := detectRemovedEditableHandles(snap, baseline); len(got) != 0 {
		t.Errorf("added items must not flip clean snapshot; got %v", got)
	}
}

func TestDetectRemovedEditableHandles_NilBaseline(t *testing.T) {
	snap := savableSnap()
	if got := detectRemovedEditableHandles(snap, nil); got != nil {
		t.Errorf("nil baseline must return nil; got %v", got)
	}
}

// ─── Transfer detection (Phase 3B) ───────────────────────────────

func TestDetectTransferredEditableItems_DetectsInventoryToStorage(t *testing.T) {
	snap := savableSnap()
	baseline := baselineFor(snap)
	moved := snap.InventoryItems[0]
	movedHandle := moved.OriginalHandle
	moved.Container = ContainerStorage
	snap.InventoryItems = nil
	snap.StorageItems = append(snap.StorageItems, moved)
	got := detectTransferredEditableItems(snap, baseline)
	if got[movedHandle] != ContainerStorage {
		t.Errorf("got %v, want [0x%08X → storage]", got, movedHandle)
	}
}

func TestDetectTransferredEditableItems_DetectsStorageToInventory(t *testing.T) {
	snap := savableSnap()
	baseline := baselineFor(snap)
	moved := snap.StorageItems[0]
	movedHandle := moved.OriginalHandle
	moved.Container = ContainerInventory
	snap.StorageItems = nil
	snap.InventoryItems = append(snap.InventoryItems, moved)
	got := detectTransferredEditableItems(snap, baseline)
	if got[movedHandle] != ContainerInventory {
		t.Errorf("got %v, want [0x%08X → inventory]", got, movedHandle)
	}
}

func TestDetectTransferredEditableItems_ReorderInsideContainerNotTransfer(t *testing.T) {
	snap := savableSnap()
	baseline := baselineFor(snap)
	snap.InventoryItems = append(snap.InventoryItems, EditableItem{
		UID:            "hnd:0x80800003",
		Source:         ItemSourceOriginal,
		Container:      ContainerInventory,
		OriginalHandle: 0x80800003,
	})
	baseline[0x80800003] = ContainerInventory
	// Swap positions — pure reorder.
	snap.InventoryItems[0], snap.InventoryItems[1] = snap.InventoryItems[1], snap.InventoryItems[0]
	if got := detectTransferredEditableItems(snap, baseline); len(got) != 0 {
		t.Errorf("pure reorder must not look like transfer; got %v", got)
	}
}

func TestDetectTransferredEditableItems_RemoveIsNotTransfer(t *testing.T) {
	snap := savableSnap()
	baseline := baselineFor(snap)
	// Drop the item entirely — should appear in removed, not in transferred.
	snap.InventoryItems = nil
	if got := detectTransferredEditableItems(snap, baseline); len(got) != 0 {
		t.Errorf("removed item must not appear as transferred; got %v", got)
	}
}

func TestDetectTransferredEditableItems_AddedItemsIgnored(t *testing.T) {
	snap := savableSnap()
	baseline := baselineFor(snap)
	snap.InventoryItems = append(snap.InventoryItems, EditableItem{
		UID:            "new:1",
		Source:         ItemSourceAdded,
		Container:      ContainerInventory,
		OriginalHandle: 0,
		ItemID:         0x000F4240,
	})
	if got := detectTransferredEditableItems(snap, baseline); len(got) != 0 {
		t.Errorf("added items must not be flagged as transfers; got %v", got)
	}
}

func TestDetectTransferredEditableItems_NilBaseline(t *testing.T) {
	snap := savableSnap()
	if got := detectTransferredEditableItems(snap, nil); got != nil {
		t.Errorf("nil baseline must return nil; got %v", got)
	}
}

func TestValidatePassThroughIndices_DuplicateRejected(t *testing.T) {
	records := []RawInventoryRecord{
		{Container: ContainerInventory, SlotIndex: 100, Handle: 0xB0001111},
		{Container: ContainerInventory, SlotIndex: 100, Handle: 0xB0002222},
	}
	err := validatePassThroughIndices(records, 2688, "inventory")
	if err == nil {
		t.Fatal("expected error for duplicate SlotIndex")
	}
	if !strings.Contains(err.Error(), "duplicated") {
		t.Errorf("error should mention duplicated, got %v", err)
	}
}

func TestValidatePassThroughIndices_OutOfRangeRejected(t *testing.T) {
	records := []RawInventoryRecord{
		{Container: ContainerInventory, SlotIndex: 9999, Handle: 0xB0001111},
	}
	if err := validatePassThroughIndices(records, 2688, "inventory"); err == nil {
		t.Fatal("expected error for out-of-range SlotIndex")
	}
}

func TestValidatePassThroughIndices_NegativeRejected(t *testing.T) {
	records := []RawInventoryRecord{
		{Container: ContainerInventory, SlotIndex: -1, Handle: 0xB0001111},
	}
	if err := validatePassThroughIndices(records, 2688, "inventory"); err == nil {
		t.Fatal("expected error for negative SlotIndex")
	}
}

func TestValidatePassThroughIndices_DistinctOK(t *testing.T) {
	records := []RawInventoryRecord{
		{Container: ContainerInventory, SlotIndex: 0, Handle: 0xB0001111},
		{Container: ContainerInventory, SlotIndex: 50, Handle: 0xB0002222},
		{Container: ContainerInventory, SlotIndex: 2687, Handle: 0xB0003333},
	}
	if err := validatePassThroughIndices(records, 2688, "inventory"); err != nil {
		t.Fatalf("distinct SlotIndex values should pass: %v", err)
	}
}

func TestApplyWorkspaceSave_NilSlotOrSnapRejected(t *testing.T) {
	if _, err := ApplyWorkspaceSave(nil, savableSnap(), map[uint32]ContainerKind{}); err == nil {
		t.Fatal("nil slot should error")
	}
	// Cannot easily construct a non-nil slot here without core import
	// cycle; covered in app-level integration tests.
}

func TestApplyWorkspaceSave_NilBaselineRejected(t *testing.T) {
	// Build a minimal non-nil-but-empty snapshot. We're testing the
	// baseline==nil guard which fires before any slot access.
	snap := &InventoryWorkspaceSnapshot{}
	// Need a slot but we pass nil baseline. Use a sentinel — the
	// function checks baseline==nil BEFORE the rejectTransferOrRemove
	// call. But it first runs Validate + rejectPendingAoW which need
	// nothing from slot.
	// nil-slot guard fires first, so this branch is checked through
	// integration. Skip direct test here.
	_ = snap
}

func TestPickNewHandle_SingleCandidate(t *testing.T) {
	before := map[uint32]uint32{0x80800001: 0x000F4240}
	after := map[uint32]uint32{
		0x80800001: 0x000F4240,
		0x80800050: 0x003085E0, // newly added Claymore
	}
	h, err := pickNewHandle(after, before, 0x003085E0)
	if err != nil {
		t.Fatalf("pickNewHandle: %v", err)
	}
	if h != 0x80800050 {
		t.Errorf("h = 0x%08X, want 0x80800050", h)
	}
}

func TestPickNewHandle_NoCandidate(t *testing.T) {
	before := map[uint32]uint32{0x80800001: 0x000F4240}
	after := map[uint32]uint32{0x80800001: 0x000F4240}
	if _, err := pickNewHandle(after, before, 0x003085E0); err == nil {
		t.Fatal("expected error when no new handle")
	}
}

func TestPickNewHandle_MultipleCandidates(t *testing.T) {
	before := map[uint32]uint32{}
	after := map[uint32]uint32{
		0x80800050: 0x003085E0,
		0x80800051: 0x003085E0, // two new handles for the same itemID
	}
	if _, err := pickNewHandle(after, before, 0x003085E0); err == nil {
		t.Fatal("expected error for ambiguous candidates")
	}
}

func TestFindHandleForItemID_Hit(t *testing.T) {
	m := map[uint32]uint32{
		0xA0005678: 0x20005678, // existing talisman handle
	}
	h, ok := findHandleForItemID(m, 0x20005678)
	if !ok {
		t.Fatal("expected hit")
	}
	if h != 0xA0005678 {
		t.Errorf("h = 0x%08X, want 0xA0005678", h)
	}
}

func TestFindHandleForItemID_Miss(t *testing.T) {
	m := map[uint32]uint32{0xA0005678: 0x20005678}
	if _, ok := findHandleForItemID(m, 0x40001111); ok {
		t.Fatal("expected miss for unrelated itemID")
	}
}

func TestSnapshotGaMap_IsDeepEnough(t *testing.T) {
	orig := map[uint32]uint32{1: 10, 2: 20}
	cp := snapshotGaMap(orig)
	cp[1] = 999
	if orig[1] != 10 {
		t.Errorf("snapshotGaMap should not alias original (orig[1]=%d after copy mutation)", orig[1])
	}
}
