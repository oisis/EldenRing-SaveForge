package tests

import (
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
	"github.com/oisis/EldenRing-SaveForge/backend/db"
)

// snapshotOtherItems captures every NON-EMPTY CommonItems / KeyItems entry
// EXCEPT the Talisman Pouch (handle 0xB0002738). Empty slots are skipped:
// AddItemsToSlot / RemoveItemFromSlot legitimately rewrite the placeholder
// Index of empty slots they reuse, so including them would yield false
// positives without proving any real item was disturbed.
func snapshotOtherItems(slot *core.SaveSlot) (common, key []core.InventoryItem) {
	common = make([]core.InventoryItem, 0, len(slot.Inventory.CommonItems))
	for _, it := range slot.Inventory.CommonItems {
		if it.GaItemHandle == 0 || it.GaItemHandle == 0xFFFFFFFF {
			continue
		}
		if it.GaItemHandle == core.TalismanPouchHandle {
			continue
		}
		common = append(common, it)
	}
	key = make([]core.InventoryItem, 0, len(slot.Inventory.KeyItems))
	for _, it := range slot.Inventory.KeyItems {
		if it.GaItemHandle == 0 || it.GaItemHandle == 0xFFFFFFFF {
			continue
		}
		if it.GaItemHandle == core.TalismanPouchHandle {
			continue
		}
		key = append(key, it)
	}
	return
}

// equalItemSets compares two slices of InventoryItem as multisets — order does
// not matter, but every (handle, qty, index) triple in `a` must appear in `b`
// and vice versa.
func equalItemSets(a, b []core.InventoryItem) bool {
	if len(a) != len(b) {
		return false
	}
	type k struct {
		h, q, i uint32
	}
	bag := make(map[k]int, len(a))
	for _, it := range a {
		bag[k{it.GaItemHandle, it.Quantity, it.Index}]++
	}
	for _, it := range b {
		key := k{it.GaItemHandle, it.Quantity, it.Index}
		if bag[key] == 0 {
			return false
		}
		bag[key]--
	}
	return true
}

// countTalismanPouchEntries returns how many CommonItems records currently
// hold the Talisman Pouch handle, and the total Quantity across them.
func countTalismanPouchEntries(slot *core.SaveSlot) (entries int, totalQty uint32) {
	for _, it := range slot.Inventory.CommonItems {
		if it.GaItemHandle == core.TalismanPouchHandle {
			entries++
			totalQty += it.Quantity
		}
	}
	return
}

// pouchFlagState returns the current value of the three "Obtained Talisman
// Pouch N" event flags (60500/60510/60520).
func pouchFlagState(t *testing.T, slot *core.SaveSlot) (f0, f1, f2 bool) {
	t.Helper()
	flags := slot.Data[slot.EventFlagsOffset:]
	var err error
	if f0, err = db.GetEventFlag(flags, core.TalismanPouchFlag0); err != nil {
		t.Fatalf("GetEventFlag(60500): %v", err)
	}
	if f1, err = db.GetEventFlag(flags, core.TalismanPouchFlag1); err != nil {
		t.Fatalf("GetEventFlag(60510): %v", err)
	}
	if f2, err = db.GetEventFlag(flags, core.TalismanPouchFlag2); err != nil {
		t.Fatalf("GetEventFlag(60520): %v", err)
	}
	return
}

// assertPouchState asserts the full sync invariant for a given expected count.
func assertPouchState(t *testing.T, slot *core.SaveSlot, expectedCount int) {
	t.Helper()

	entries, qty := countTalismanPouchEntries(slot)
	if expectedCount == 0 {
		if entries != 0 {
			t.Fatalf("expected 0 Talisman Pouch entries, got %d (qty=%d)", entries, qty)
		}
	} else {
		if entries != 1 {
			t.Fatalf("expected exactly 1 Talisman Pouch entry, got %d (qty=%d)", entries, qty)
		}
		if qty != uint32(expectedCount) {
			t.Fatalf("expected Talisman Pouch quantity=%d, got %d", expectedCount, qty)
		}
	}

	wantF0 := expectedCount >= 1
	wantF1 := expectedCount >= 2
	wantF2 := expectedCount >= 3
	f0, f1, f2 := pouchFlagState(t, slot)
	if f0 != wantF0 || f1 != wantF1 || f2 != wantF2 {
		t.Fatalf("flag state (60500,60510,60520) = (%v,%v,%v), want (%v,%v,%v)",
			f0, f1, f2, wantF0, wantF1, wantF2)
	}
}

// freshSlotResetToZero loads a fresh PC test save and brings the Talisman
// Pouch state down to a known zero baseline so each test starts from the same
// point regardless of what the on-disk fixture contains.
func freshSlotResetToZero(t *testing.T) *core.SaveSlot {
	t.Helper()
	save, slotIdx := loadBulkTestSave(t)
	slot := &save.Slots[slotIdx]
	if slot.EventFlagsOffset <= 0 || slot.EventFlagsOffset >= len(slot.Data) {
		t.Skip("EventFlagsOffset not set in test save slot")
	}
	if err := core.SyncTalismanPouchCount(slot, 0); err != nil {
		t.Fatalf("baseline reset to 0 failed: %v", err)
	}
	return slot
}

func TestSyncTalismanPouch_Count0_RemovesAllAndClearsFlags(t *testing.T) {
	slot := freshSlotResetToZero(t)

	// Pre-populate so removal has work to do.
	if err := core.SyncTalismanPouchCount(slot, 3); err != nil {
		t.Fatalf("pre-populate count=3: %v", err)
	}
	otherCommon, otherKey := snapshotOtherItems(slot)

	if err := core.SyncTalismanPouchCount(slot, 0); err != nil {
		t.Fatalf("SyncTalismanPouchCount(0): %v", err)
	}
	assertPouchState(t, slot, 0)

	postCommon, postKey := snapshotOtherItems(slot)
	if !equalItemSets(otherCommon, postCommon) {
		t.Errorf("CommonItems (excluding Talisman Pouch) changed during count=0 sync")
	}
	if !equalItemSets(otherKey, postKey) {
		t.Errorf("KeyItems changed during count=0 sync")
	}
}

func TestSyncTalismanPouch_Count1_SetsOnePouchAndFlag60500Only(t *testing.T) {
	slot := freshSlotResetToZero(t)
	if err := core.SyncTalismanPouchCount(slot, 1); err != nil {
		t.Fatalf("SyncTalismanPouchCount(1): %v", err)
	}
	assertPouchState(t, slot, 1)
}

func TestSyncTalismanPouch_Count2_SetsTwoPouchesAndFlags60500_60510(t *testing.T) {
	slot := freshSlotResetToZero(t)
	if err := core.SyncTalismanPouchCount(slot, 2); err != nil {
		t.Fatalf("SyncTalismanPouchCount(2): %v", err)
	}
	assertPouchState(t, slot, 2)
}

func TestSyncTalismanPouch_Count3_SetsThreePouchesAndAllFlags(t *testing.T) {
	slot := freshSlotResetToZero(t)
	if err := core.SyncTalismanPouchCount(slot, 3); err != nil {
		t.Fatalf("SyncTalismanPouchCount(3): %v", err)
	}
	assertPouchState(t, slot, 3)
}

func TestSyncTalismanPouch_Idempotent_DoubleApplyCount3(t *testing.T) {
	slot := freshSlotResetToZero(t)

	if err := core.SyncTalismanPouchCount(slot, 3); err != nil {
		t.Fatalf("first apply count=3: %v", err)
	}
	assertPouchState(t, slot, 3)

	// Second apply with the same value must NOT create a second pouch entry
	// nor bump quantity beyond 3.
	if err := core.SyncTalismanPouchCount(slot, 3); err != nil {
		t.Fatalf("second apply count=3: %v", err)
	}
	assertPouchState(t, slot, 3)
}

func TestSyncTalismanPouch_Downsync_From3To1_LeavesOnePouchAnd60500Only(t *testing.T) {
	slot := freshSlotResetToZero(t)

	if err := core.SyncTalismanPouchCount(slot, 3); err != nil {
		t.Fatalf("setup count=3: %v", err)
	}
	otherCommon, otherKey := snapshotOtherItems(slot)

	if err := core.SyncTalismanPouchCount(slot, 1); err != nil {
		t.Fatalf("downsync count=1: %v", err)
	}
	assertPouchState(t, slot, 1)

	postCommon, postKey := snapshotOtherItems(slot)
	if !equalItemSets(otherCommon, postCommon) {
		t.Errorf("CommonItems (excluding Talisman Pouch) changed during downsync")
	}
	if !equalItemSets(otherKey, postKey) {
		t.Errorf("KeyItems changed during downsync")
	}
}

func TestSyncTalismanPouch_DoesNotTouchOtherKeyItems(t *testing.T) {
	slot := freshSlotResetToZero(t)
	otherCommonBefore, otherKeyBefore := snapshotOtherItems(slot)

	// Run the full count sequence.
	for _, n := range []int{1, 2, 3, 0, 2, 3, 1, 0} {
		if err := core.SyncTalismanPouchCount(slot, n); err != nil {
			t.Fatalf("SyncTalismanPouchCount(%d): %v", n, err)
		}
	}
	assertPouchState(t, slot, 0)

	otherCommonAfter, otherKeyAfter := snapshotOtherItems(slot)
	if !equalItemSets(otherCommonBefore, otherCommonAfter) {
		t.Errorf("CommonItems (excluding Talisman Pouch) drifted across sequence")
	}
	if !equalItemSets(otherKeyBefore, otherKeyAfter) {
		t.Errorf("KeyItems drifted across sequence")
	}
}

func TestSyncTalismanPouch_ClampOverThreeDownToThree(t *testing.T) {
	slot := freshSlotResetToZero(t)

	// Anything above the vanilla cap must clamp to 3 and never flip 60530+.
	flags := slot.Data[slot.EventFlagsOffset:]
	beforeF530, err := db.GetEventFlag(flags, 60530)
	if err != nil {
		t.Fatalf("baseline GetEventFlag(60530): %v", err)
	}

	if err := core.SyncTalismanPouchCount(slot, 99); err != nil {
		t.Fatalf("SyncTalismanPouchCount(99): %v", err)
	}
	assertPouchState(t, slot, 3)

	afterF530, err := db.GetEventFlag(flags, 60530)
	if err != nil {
		t.Fatalf("post GetEventFlag(60530): %v", err)
	}
	if afterF530 != beforeF530 {
		t.Errorf("flag 60530 changed during over-cap sync (before=%v, after=%v) — only 60500/60510/60520 may be touched",
			beforeF530, afterF530)
	}
}
