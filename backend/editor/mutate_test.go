package editor

import "testing"

// mkItem returns a valid editable item suitable for mutation tests. Each
// item is a Dagger with a distinct UID/handle so duplicate-handle checks
// stay quiet and Validate returns OK.
func mkItem(uid string, handle uint32, container ContainerKind, acq uint32) EditableItem {
	return EditableItem{
		UID:              uid,
		Source:           ItemSourceOriginal,
		Container:        container,
		OriginalHandle:   handle,
		ItemID:           0x000F4240,
		BaseItemID:       0x000F4240,
		Name:             "Dagger",
		Category:         "melee_armaments",
		Quantity:         1,
		AcquisitionIndex: acq,
		MaxUpgrade:       25,
		HasGaItem:        true,
		IsWeapon:         true,
	}
}

// mkSnap builds a snapshot with N inventory + M storage items plus one
// pass-through record in each container. Positions are pre-assigned.
func mkSnap(invN, stoN int) *InventoryWorkspaceSnapshot {
	snap := &InventoryWorkspaceSnapshot{
		SessionID:      "ses-test",
		CharacterIndex: 0,
		InventoryItems: make([]EditableItem, invN),
		StorageItems:   make([]EditableItem, stoN),
		UnsupportedInventoryRecords: []RawInventoryRecord{
			{Container: ContainerInventory, SlotIndex: 99, Handle: 0xB0CAFE01, Reason: ReasonUnknownItem},
		},
		UnsupportedStorageRecords: []RawInventoryRecord{
			{Container: ContainerStorage, SlotIndex: 99, Handle: 0xB0CAFE02, Reason: ReasonUnknownItem},
		},
	}
	for i := 0; i < invN; i++ {
		uid := indexedUID("inv", i)
		snap.InventoryItems[i] = mkItem(uid, uint32(0x80810000)+uint32(i), ContainerInventory, uint32(1000+i*2))
		snap.InventoryItems[i].Position = i
	}
	for i := 0; i < stoN; i++ {
		uid := indexedUID("sto", i)
		snap.StorageItems[i] = mkItem(uid, uint32(0x80820000)+uint32(i), ContainerStorage, uint32(2000+i*2))
		snap.StorageItems[i].Position = i
	}
	return snap
}

func indexedUID(prefix string, i int) string {
	// Compact, deterministic UIDs for tests.
	letters := []byte{byte('A' + i)}
	return prefix + ":" + string(letters)
}

func uidsOf(items []EditableItem) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = it.UID
	}
	return out
}

func assertPositions(t *testing.T, items []EditableItem, label string) {
	t.Helper()
	for i, it := range items {
		if it.Position != i {
			t.Errorf("%s[%d].Position = %d, want %d", label, i, it.Position, i)
		}
	}
}

func TestMoveItem_WithinInventoryFromIndex2ToIndex0(t *testing.T) {
	snap := mkSnap(4, 0)
	if err := MoveItem(snap, "inv:C", ContainerInventory, 0); err != nil {
		t.Fatalf("MoveItem: %v", err)
	}
	want := []string{"inv:C", "inv:A", "inv:B", "inv:D"}
	if got := uidsOf(snap.InventoryItems); !equalStrs(got, want) {
		t.Errorf("order = %v, want %v", got, want)
	}
	assertPositions(t, snap.InventoryItems, "inv")
	if !snap.Dirty {
		t.Error("Dirty should be true after move")
	}
}

func TestMoveItem_WithinStorageFromIndex0ToIndex2(t *testing.T) {
	snap := mkSnap(0, 3)
	if err := MoveItem(snap, "sto:A", ContainerStorage, 2); err != nil {
		t.Fatalf("MoveItem: %v", err)
	}
	want := []string{"sto:B", "sto:C", "sto:A"}
	if got := uidsOf(snap.StorageItems); !equalStrs(got, want) {
		t.Errorf("order = %v, want %v", got, want)
	}
	assertPositions(t, snap.StorageItems, "sto")
}

func TestMoveItem_InventoryToStorageAtSpecificPosition(t *testing.T) {
	snap := mkSnap(3, 2)
	// Move inv:B (Position 1) to storage at position 1 (between sto:A and sto:B).
	if err := MoveItem(snap, "inv:B", ContainerStorage, 1); err != nil {
		t.Fatalf("MoveItem: %v", err)
	}
	if got := uidsOf(snap.InventoryItems); !equalStrs(got, []string{"inv:A", "inv:C"}) {
		t.Errorf("inv = %v", got)
	}
	if got := uidsOf(snap.StorageItems); !equalStrs(got, []string{"sto:A", "inv:B", "sto:B"}) {
		t.Errorf("sto = %v", got)
	}
	// Container field must be updated on the moved item.
	for _, it := range snap.StorageItems {
		if it.UID == "inv:B" && it.Container != ContainerStorage {
			t.Errorf("moved item Container = %q, want storage", it.Container)
		}
	}
	assertPositions(t, snap.InventoryItems, "inv")
	assertPositions(t, snap.StorageItems, "sto")
}

func TestMoveItem_StorageToInventoryAppend(t *testing.T) {
	snap := mkSnap(2, 2)
	// targetPosition past length → append.
	if err := MoveItem(snap, "sto:B", ContainerInventory, 999); err != nil {
		t.Fatalf("MoveItem: %v", err)
	}
	if got := uidsOf(snap.InventoryItems); !equalStrs(got, []string{"inv:A", "inv:B", "sto:B"}) {
		t.Errorf("inv = %v", got)
	}
	if got := uidsOf(snap.StorageItems); !equalStrs(got, []string{"sto:A"}) {
		t.Errorf("sto = %v", got)
	}
}

func TestMoveItem_NegativeClampedToZero(t *testing.T) {
	snap := mkSnap(3, 0)
	if err := MoveItem(snap, "inv:C", ContainerInventory, -42); err != nil {
		t.Fatalf("MoveItem: %v", err)
	}
	if snap.InventoryItems[0].UID != "inv:C" {
		t.Errorf("expected inv:C at index 0, got %v", uidsOf(snap.InventoryItems))
	}
}

func TestMoveItem_RecomputesPositionsAcrossBothContainers(t *testing.T) {
	snap := mkSnap(3, 3)
	if err := MoveItem(snap, "inv:A", ContainerStorage, 0); err != nil {
		t.Fatalf("MoveItem: %v", err)
	}
	assertPositions(t, snap.InventoryItems, "inv")
	assertPositions(t, snap.StorageItems, "sto")
}

func TestMoveItem_RefreshesValidation(t *testing.T) {
	snap := mkSnap(2, 0)
	// Pre-set stale validation.
	snap.Validation = WorkspaceValidationReport{OK: false}
	if err := MoveItem(snap, "inv:A", ContainerInventory, 1); err != nil {
		t.Fatalf("MoveItem: %v", err)
	}
	if !snap.Validation.OK {
		t.Errorf("Validation.OK should be true after clean move, errors=%+v", snap.Validation.Errors)
	}
	if snap.Validation.InventoryItemCount != 2 {
		t.Errorf("Validation.InventoryItemCount = %d, want 2", snap.Validation.InventoryItemCount)
	}
}

func TestMoveItem_UnknownUIDReturnsError(t *testing.T) {
	snap := mkSnap(2, 2)
	err := MoveItem(snap, "hnd:0xDEADBEEF", ContainerInventory, 0)
	if err == nil {
		t.Fatal("expected error for unknown UID")
	}
}

func TestMoveItem_InvalidContainerKindReturnsError(t *testing.T) {
	snap := mkSnap(1, 0)
	err := MoveItem(snap, "inv:A", ContainerKind("trash"), 0)
	if err == nil {
		t.Fatal("expected error for invalid container kind")
	}
}

func TestMoveItem_LeavesPassThroughUntouched(t *testing.T) {
	snap := mkSnap(2, 2)
	invBefore := append([]RawInventoryRecord(nil), snap.UnsupportedInventoryRecords...)
	stoBefore := append([]RawInventoryRecord(nil), snap.UnsupportedStorageRecords...)
	if err := MoveItem(snap, "inv:A", ContainerStorage, 0); err != nil {
		t.Fatalf("MoveItem: %v", err)
	}
	if !equalRawRecords(snap.UnsupportedInventoryRecords, invBefore) {
		t.Errorf("UnsupportedInventoryRecords changed: %+v", snap.UnsupportedInventoryRecords)
	}
	if !equalRawRecords(snap.UnsupportedStorageRecords, stoBefore) {
		t.Errorf("UnsupportedStorageRecords changed: %+v", snap.UnsupportedStorageRecords)
	}
}

func TestRemoveItem_FromInventory(t *testing.T) {
	snap := mkSnap(3, 0)
	if err := RemoveItem(snap, "inv:B"); err != nil {
		t.Fatalf("RemoveItem: %v", err)
	}
	if got := uidsOf(snap.InventoryItems); !equalStrs(got, []string{"inv:A", "inv:C"}) {
		t.Errorf("inv = %v", got)
	}
	assertPositions(t, snap.InventoryItems, "inv")
	if !snap.Dirty {
		t.Error("Dirty should be true after remove")
	}
}

func TestRemoveItem_FromStorage(t *testing.T) {
	snap := mkSnap(0, 3)
	if err := RemoveItem(snap, "sto:A"); err != nil {
		t.Fatalf("RemoveItem: %v", err)
	}
	if got := uidsOf(snap.StorageItems); !equalStrs(got, []string{"sto:B", "sto:C"}) {
		t.Errorf("sto = %v", got)
	}
}

func TestRemoveItem_UnknownUIDReturnsError(t *testing.T) {
	snap := mkSnap(1, 1)
	if err := RemoveItem(snap, "nope"); err == nil {
		t.Fatal("expected error for unknown UID")
	}
}

func TestRemoveItem_LeavesPassThroughUntouched(t *testing.T) {
	snap := mkSnap(2, 2)
	invBefore := append([]RawInventoryRecord(nil), snap.UnsupportedInventoryRecords...)
	if err := RemoveItem(snap, "inv:A"); err != nil {
		t.Fatalf("RemoveItem: %v", err)
	}
	if !equalRawRecords(snap.UnsupportedInventoryRecords, invBefore) {
		t.Errorf("pass-through mutated: %+v", snap.UnsupportedInventoryRecords)
	}
}

// equalStrs is a small helper rather than pulling in reflect.DeepEqual.
func equalStrs(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func equalRawRecords(a, b []RawInventoryRecord) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
