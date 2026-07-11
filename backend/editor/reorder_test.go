package editor

import "testing"

func TestReorderItems_BothContainers(t *testing.T) {
	snap := mkSnap(3, 3)
	invWant := []string{"inv:C", "inv:A", "inv:B"}
	stoWant := []string{"sto:B", "sto:C", "sto:A"}
	if err := ReorderItems(snap, invWant, stoWant); err != nil {
		t.Fatalf("ReorderItems: %v", err)
	}
	if got := uidsOf(snap.InventoryItems); !equalStrs(got, invWant) {
		t.Errorf("inv order = %v, want %v", got, invWant)
	}
	if got := uidsOf(snap.StorageItems); !equalStrs(got, stoWant) {
		t.Errorf("sto order = %v, want %v", got, stoWant)
	}
	// Positions recomputed for both.
	assertPositions(t, snap.InventoryItems, "inv")
	assertPositions(t, snap.StorageItems, "sto")
	if !snap.Dirty {
		t.Error("Dirty should be true after reorder")
	}
}

func TestReorderItems_PassThroughRecordsUnchanged(t *testing.T) {
	snap := mkSnap(2, 2)
	invRaw := append([]RawInventoryRecord(nil), snap.UnsupportedInventoryRecords...)
	stoRaw := append([]RawInventoryRecord(nil), snap.UnsupportedStorageRecords...)
	if err := ReorderItems(snap, []string{"inv:B", "inv:A"}, []string{"sto:B", "sto:A"}); err != nil {
		t.Fatalf("ReorderItems: %v", err)
	}
	if len(snap.UnsupportedInventoryRecords) != len(invRaw) || snap.UnsupportedInventoryRecords[0].Handle != invRaw[0].Handle {
		t.Errorf("inventory pass-through mutated: %+v", snap.UnsupportedInventoryRecords)
	}
	if len(snap.UnsupportedStorageRecords) != len(stoRaw) || snap.UnsupportedStorageRecords[0].Handle != stoRaw[0].Handle {
		t.Errorf("storage pass-through mutated: %+v", snap.UnsupportedStorageRecords)
	}
}

// assertUnchanged verifies both editable orders match the original mkSnap
// layout — used to prove every rejected reorder is atomic.
func assertUnchanged(t *testing.T, snap *InventoryWorkspaceSnapshot, invN, stoN int) {
	t.Helper()
	for i := 0; i < invN; i++ {
		if snap.InventoryItems[i].UID != indexedUID("inv", i) {
			t.Fatalf("inventory mutated by rejected reorder: %v", uidsOf(snap.InventoryItems))
		}
	}
	for i := 0; i < stoN; i++ {
		if snap.StorageItems[i].UID != indexedUID("sto", i) {
			t.Fatalf("storage mutated by rejected reorder: %v", uidsOf(snap.StorageItems))
		}
	}
	if snap.Dirty {
		t.Error("Dirty should stay false after a rejected reorder")
	}
}

func TestReorderItems_RejectsAndStaysAtomic(t *testing.T) {
	cases := []struct {
		name    string
		invUIDs []string
		stoUIDs []string
	}{
		{"missing uid", []string{"inv:A", "inv:B"}, []string{"sto:A", "sto:B", "sto:C"}}, // storage drops one
		{"wrong length", []string{"inv:A"}, []string{"sto:A", "sto:B", "sto:C"}},
		{"duplicate uid", []string{"inv:A", "inv:A", "inv:C"}, []string{"sto:A", "sto:B", "sto:C"}},
		{"foreign uid", []string{"inv:A", "inv:B", "hnd:0xDEAD"}, []string{"sto:A", "sto:B", "sto:C"}},
		{"wrong container uid", []string{"inv:A", "inv:B", "sto:A"}, []string{"sto:A", "sto:B", "sto:C"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			snap := mkSnap(3, 3)
			if err := ReorderItems(snap, tc.invUIDs, tc.stoUIDs); err == nil {
				t.Fatalf("expected error for %s, got nil", tc.name)
			}
			assertUnchanged(t, snap, 3, 3)
		})
	}
}

// A valid inventory list must not commit if the storage list is invalid:
// both are validated before either is mutated.
func TestReorderItems_ValidInventoryStillRejectedOnBadStorage(t *testing.T) {
	snap := mkSnap(3, 3)
	if err := ReorderItems(snap, []string{"inv:C", "inv:B", "inv:A"}, []string{"sto:A", "sto:A", "sto:C"}); err == nil {
		t.Fatal("expected error when storage list has a duplicate")
	}
	assertUnchanged(t, snap, 3, 3)
}
