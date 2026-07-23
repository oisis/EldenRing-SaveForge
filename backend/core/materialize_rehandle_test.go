package core

import "testing"

// countGaItemsWithPrefix returns how many non-empty entries in slot.GaItems
// carry the given handle type prefix (e.g. ItemTypeAccessory).
func countGaItemsWithPrefix(slot *SaveSlot, prefix uint32) int {
	n := 0
	for i := range slot.GaItems {
		g := slot.GaItems[i]
		if g.IsEmpty() {
			continue
		}
		if g.Handle&GaHandleTypeMask == prefix {
			n++
		}
	}
	return n
}

func newRehandleTestSlot() *SaveSlot {
	return &SaveSlot{
		GaItems:           make([]GaItemFull, 100),
		GaMap:             map[uint32]uint32{},
		NextAoWIndex:      0,
		NextArmamentIndex: 10,
		NextGaItemHandle:  1,
		PartGaItemHandle:  0x80,
	}
}

// TestMaterializeRehandledInstance_TalismanIsHandleOnly locks the native
// contract: rehandling a talisman (0xA0) on a dest-handle collision must mint a
// fresh 0xA0 handle and a GaMap entry, but must NOT create a physical GaItem
// record — accessories are handle-only on native saves (E5b).
func TestMaterializeRehandledInstance_TalismanIsHandleOnly(t *testing.T) {
	slot := newRehandleTestSlot()
	oldHandle := uint32(0xA0000005) // talisman handle
	itemID := uint32(0x20000005)    // talisman item ID
	slot.GaMap[oldHandle] = itemID

	beforeArmament := slot.NextArmamentIndex

	newHandle, err := materializeRehandledInstance(slot, oldHandle)
	if err != nil {
		t.Fatalf("materializeRehandledInstance: %v", err)
	}

	if newHandle&GaHandleTypeMask != ItemTypeAccessory {
		t.Errorf("newHandle 0x%08X prefix != 0xA0", newHandle)
	}
	if newHandle == oldHandle {
		t.Errorf("newHandle equals oldHandle 0x%08X (no fresh handle minted)", oldHandle)
	}
	if got := slot.GaMap[newHandle]; got != itemID {
		t.Errorf("GaMap[0x%08X] = 0x%08X, want 0x%08X", newHandle, got, itemID)
	}
	// The core regression: no physical 0xA0 record may exist in GaItems.
	if n := countGaItemsWithPrefix(slot, ItemTypeAccessory); n != 0 {
		t.Errorf("talisman created %d physical 0xA0 GaItem record(s); want 0 (handle-only)", n)
	}
	// Handle-only path must not consume an armament slot.
	if slot.NextArmamentIndex != beforeArmament {
		t.Errorf("NextArmamentIndex advanced %d -> %d for handle-only talisman", beforeArmament, slot.NextArmamentIndex)
	}
}

// TestMaterializeRehandledInstance_TalismanFallbackNoGaMap covers the real
// field scenario: a freshly-loaded save whose talisman has no GaMap entry yet.
// The itemID must be derived from the handle, still with no physical record.
func TestMaterializeRehandledInstance_TalismanFallbackNoGaMap(t *testing.T) {
	slot := newRehandleTestSlot()
	oldHandle := uint32(0xA0000007) // talisman handle, deliberately NOT in GaMap
	wantItemID := uint32(0x20000007)

	newHandle, err := materializeRehandledInstance(slot, oldHandle)
	if err != nil {
		t.Fatalf("materializeRehandledInstance: %v", err)
	}
	if got := slot.GaMap[newHandle]; got != wantItemID {
		t.Errorf("GaMap[0x%08X] = 0x%08X, want 0x%08X (derived from handle)", newHandle, got, wantItemID)
	}
	if n := countGaItemsWithPrefix(slot, ItemTypeAccessory); n != 0 {
		t.Errorf("talisman fallback created %d physical 0xA0 record(s); want 0", n)
	}
}

// TestMaterializeRehandledInstance_WeaponStillPhysical guards against
// over-correction: weapons (0x80) MUST still get a physical GaItem record.
func TestMaterializeRehandledInstance_WeaponStillPhysical(t *testing.T) {
	slot := newRehandleTestSlot()
	oldHandle := uint32(ItemTypeWeapon | gaItemHandleValidBit | 3)
	itemID := uint32(0x000F4240) // Uchigatana (weapon nibble 0x0)
	slot.GaItems[3] = nativeTestRecord(oldHandle, itemID)
	slot.GaMap[oldHandle] = itemID
	refreshGaItemTracking(slot)
	beforeCount := countGaItemsWithPrefix(slot, ItemTypeWeapon)

	newHandle, err := materializeRehandledInstance(slot, oldHandle)
	if err != nil {
		t.Fatalf("materializeRehandledInstance: %v", err)
	}
	if newHandle&GaHandleTypeMask != ItemTypeWeapon {
		t.Errorf("newHandle 0x%08X prefix != 0x80", newHandle)
	}
	if got := slot.GaMap[newHandle]; got != itemID {
		t.Errorf("GaMap[0x%08X] = 0x%08X, want 0x%08X", newHandle, got, itemID)
	}
	if got := countGaItemsWithPrefix(slot, ItemTypeWeapon); got != beforeCount+1 {
		t.Fatalf("physical weapon records = %d, want %d", got, beforeCount+1)
	}
	found := false
	for _, record := range slot.GaItems {
		if record.Handle == newHandle && record.ItemID == itemID {
			found = true
		}
	}
	if !found {
		t.Errorf("new physical record {handle 0x%08X, id 0x%08X} not found", newHandle, itemID)
	}
}
