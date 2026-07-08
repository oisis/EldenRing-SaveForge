package editor

import (
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
)

// Blocker-2 regression: classifyRecord's duplicate-handle protection must key
// off the shared resolver's identity class (only IdentityInstanceBacked), not a
// hard-coded talisman-prefix exception. Handle-encoded goods, stackable ammo and
// talismans legitimately share a handle across copies, so a repeated handle is
// normal save state — never ReasonDuplicateHandle. Only per-instance weapon /
// armor / AoW handles may collide.

// countReason tallies pass-through records carrying a given reason.
func countReason(recs []RawInventoryRecord, reason string) int {
	n := 0
	for _, r := range recs {
		if r.Reason == reason {
			n++
		}
	}
	return n
}

// (goods) Two identical goods records in one container: both preserved, neither
// flagged as a duplicate handle (they are unsupported-category pass-through).
func TestClassifyRecord_DuplicateGoods_NotDuplicateHandle(t *testing.T) {
	const goodsHandle = uint32(0xB0002774) // handle-encoded goods (→ itemID 0x40002774)
	slot, invStart, _ := fixtureSlot(t)
	fakeRecord(slot.Data, invStart+0*core.InvRecordLen, goodsHandle, 5, 1000)
	fakeRecord(slot.Data, invStart+1*core.InvRecordLen, goodsHandle, 5, 1001)

	snap, err := BuildSnapshot(slot, "ses-goods", 0)
	if err != nil {
		t.Fatalf("BuildSnapshot: %v", err)
	}
	if got := countReason(snap.UnsupportedInventoryRecords, ReasonDuplicateHandle); got != 0 {
		t.Errorf("goods must never be duplicate_handle, got %d such records: %+v", got, snap.UnsupportedInventoryRecords)
	}
	if got := countReason(snap.UnsupportedInventoryRecords, ReasonUnsupportedCategory); got != 2 {
		t.Errorf("expected both goods copies classified as unsupported_category, got %d: %+v", got, snap.UnsupportedInventoryRecords)
	}
}

// (ammo) Two identical arrow records in one container: stackable ammo shares a
// handle by design — both preserved, neither a duplicate handle.
func TestClassifyRecord_DuplicateAmmo_NotDuplicateHandle(t *testing.T) {
	const ammoHandle = uint32(0x80800AA1) // weapon-prefix handle backing an arrow GaItem
	const ammoItemID = uint32(0x02FAF080) // arrow (IsArrowID) → stackable ammo
	slot, invStart, _ := fixtureSlot(t)
	slot.GaMap[ammoHandle] = ammoItemID
	fakeRecord(slot.Data, invStart+0*core.InvRecordLen, ammoHandle, 600, 1000)
	fakeRecord(slot.Data, invStart+1*core.InvRecordLen, ammoHandle, 600, 1001)

	snap, err := BuildSnapshot(slot, "ses-ammo", 0)
	if err != nil {
		t.Fatalf("BuildSnapshot: %v", err)
	}
	if got := countReason(snap.UnsupportedInventoryRecords, ReasonDuplicateHandle); got != 0 {
		t.Errorf("stackable ammo must never be duplicate_handle, got %d: %+v", got, snap.UnsupportedInventoryRecords)
	}
	if len(snap.UnsupportedInventoryRecords) != 2 {
		t.Errorf("expected both arrow copies preserved, got %d: %+v", len(snap.UnsupportedInventoryRecords), snap.UnsupportedInventoryRecords)
	}
}

// (control) Two identical weapon records DO collide — instance-backed handles
// are per-instance unique, so the second copy is forced to ReasonDuplicateHandle.
func TestClassifyRecord_DuplicateWeapon_IsDuplicateHandle(t *testing.T) {
	const weaponHandle = uint32(0x80000002)
	const daggerItemID = uint32(0x000F4240) // Dagger (melee_armaments, editable)
	slot, invStart, _ := fixtureSlot(t)
	slot.GaMap[weaponHandle] = daggerItemID
	fakeRecord(slot.Data, invStart+0*core.InvRecordLen, weaponHandle, 1, 1000)
	fakeRecord(slot.Data, invStart+1*core.InvRecordLen, weaponHandle, 1, 1001)

	snap, err := BuildSnapshot(slot, "ses-weapon", 0)
	if err != nil {
		t.Fatalf("BuildSnapshot: %v", err)
	}
	if len(snap.InventoryItems) != 1 {
		t.Errorf("expected exactly one editable weapon (first copy), got %d", len(snap.InventoryItems))
	}
	if got := countReason(snap.UnsupportedInventoryRecords, ReasonDuplicateHandle); got != 1 {
		t.Errorf("expected the second weapon copy flagged duplicate_handle, got %d such records: %+v", got, snap.UnsupportedInventoryRecords)
	}
}
