package core

import "testing"

// findContainerHandleExcluding returns the first non-empty handle in the given
// CommonItems binary block whose type prefix matches `prefix` but which is not
// `exclude`. Used to locate the freshly-minted rehandle instance on the
// destination side after a duplicate-handle transfer. Returns 0 when none.
func findContainerHandleExcluding(slot *SaveSlot, start, slots int, prefix, exclude uint32) uint32 {
	data := slot.Data
	for i := 0; i < slots; i++ {
		off := start + i*InvRecordLen
		if off+InvRecordLen > len(data) {
			break
		}
		h := uint32(data[off]) | uint32(data[off+1])<<8 | uint32(data[off+2])<<16 | uint32(data[off+3])<<24
		if h == GaHandleEmpty || h == GaHandleInvalid || h == exclude {
			continue
		}
		if h&GaHandleTypeMask == prefix {
			return h
		}
	}
	return 0
}

// TestTransfer_TalismanCollision_RehandleHandleOnly is the persistent in-memory
// integration test that CI runs without any tmp/ save file. It drives the full
// transferOne rehandle path (duplicate 0xA0 handle on the destination) through
// rebuildAfterAllocation -> RebuildSlotFull (real rebuild, Version!=0) ->
// parseFromData, and locks the native handle-only contract for talismans: a
// fresh 0xA0 handle is minted and registered in GaMap, but NO physical GaItem
// record is created and the armament cursor never advances.
func TestTransfer_TalismanCollision_RehandleHandleOnly(t *testing.T) {
	slot := fragmentedRepackRoundTripFixture(t).Slot

	const talisman = ItemTypeAccessory | 0x000000A5 // 0xA00000A5
	const talismanItemID = 0x20000000 | 0x000000A5  // 0x200000A5 (HandleToItemID)

	// Inject the same talisman handle into BOTH inventory (source) and storage
	// (dest) so transferOne sees a destination collision. Inventory slots 0..2
	// and storage slot 0 are occupied by the fixture; use the first free slot.
	invStart := slot.MagicOffset + InvStartFromMagic
	stoStart := slot.StorageBoxOffset + StorageHeaderSkip
	writeRecord(slot.Data, invStart, 3, talisman, 1, 103)
	writeRecord(slot.Data, stoStart, 1, talisman, 1, 201)
	if err := slot.parseFromData(); err != nil {
		t.Fatalf("parseFromData after injection: %v", err)
	}

	beforeArmament := slot.NextArmamentIndex

	res, err := MoveItemsBetweenContainers(slot, []uint32{talisman}, TransferToStorage, nil)
	if err != nil {
		t.Fatalf("MoveItemsBetweenContainers: %v", err)
	}
	if res.Moved != 1 || len(res.Skipped) != 0 {
		t.Fatalf("moved=%d skipped=%+v, want moved=1 skipped=0", res.Moved, res.Skipped)
	}

	// Offsets may have shifted after the rebuild/reparse — recompute.
	invStart = slot.MagicOffset + InvStartFromMagic
	stoStart = slot.StorageBoxOffset + StorageHeaderSkip

	if _, _, ok := scanRecord(slot.Data, invStart, CommonItemCount, talisman); ok {
		t.Errorf("source inventory still holds talisman 0x%08X (not cleared)", talisman)
	}
	if _, _, ok := scanRecord(slot.Data, stoStart, StorageCommonCount, talisman); !ok {
		t.Errorf("dest storage lost the original talisman 0x%08X", talisman)
	}
	newHandle := findContainerHandleExcluding(slot, stoStart, StorageCommonCount, ItemTypeAccessory, talisman)
	if newHandle == 0 {
		t.Fatalf("no fresh 0xA0 handle materialized in storage")
	}
	if got := slot.GaMap[newHandle]; got != talismanItemID {
		t.Errorf("GaMap[0x%08X] = 0x%08X, want 0x%08X after rebuild/reparse", newHandle, got, talismanItemID)
	}
	if n := countGaItemsWithPrefix(slot, ItemTypeAccessory); n != 0 {
		t.Errorf("talisman created %d physical 0xA0 GaItem record(s); want 0 (handle-only)", n)
	}
	if slot.NextArmamentIndex != beforeArmament {
		t.Errorf("NextArmamentIndex advanced %d -> %d for handle-only talisman", beforeArmament, slot.NextArmamentIndex)
	}
}

// TestTransfer_WeaponCollision_RehandlePhysical is the guard against
// over-correction: a duplicate weapon (0x80) handle on the destination must
// still materialize a PHYSICAL GaItem record and survive the rebuild/reparse.
func TestTransfer_WeaponCollision_RehandlePhysical(t *testing.T) {
	fx := fragmentedRepackRoundTripFixture(t)
	slot := fx.Slot
	weapon := fx.Handles.Weapon
	const weaponItemID = 0x000F4240 // Uchigatana, GaMap[handles.Weapon]

	// weapon already sits in inventory (fixture slot 0); inject a duplicate into
	// the first free storage slot to force the destination collision.
	stoStart := slot.StorageBoxOffset + StorageHeaderSkip
	writeRecord(slot.Data, stoStart, 1, weapon, 1, 202)
	if err := slot.parseFromData(); err != nil {
		t.Fatalf("parseFromData after injection: %v", err)
	}

	beforeArmament := slot.NextArmamentIndex

	res, err := MoveItemsBetweenContainers(slot, []uint32{weapon}, TransferToStorage, nil)
	if err != nil {
		t.Fatalf("MoveItemsBetweenContainers: %v", err)
	}
	if res.Moved != 1 || len(res.Skipped) != 0 {
		t.Fatalf("moved=%d skipped=%+v, want moved=1 skipped=0", res.Moved, res.Skipped)
	}

	stoStart = slot.StorageBoxOffset + StorageHeaderSkip
	if _, _, ok := scanRecord(slot.Data, stoStart, StorageCommonCount, weapon); !ok {
		t.Errorf("dest storage lost the original weapon 0x%08X", weapon)
	}
	newHandle := findContainerHandleExcluding(slot, stoStart, StorageCommonCount, ItemTypeWeapon, weapon)
	if newHandle == 0 {
		t.Fatalf("no fresh 0x80 handle materialized in storage")
	}
	if got := slot.GaMap[newHandle]; got != weaponItemID {
		t.Errorf("GaMap[0x%08X] = 0x%08X, want 0x%08X", newHandle, got, weaponItemID)
	}
	// A physical GaItem record for the new handle must exist after rebuild/reparse.
	found := false
	for i := range slot.GaItems {
		if slot.GaItems[i].Handle == newHandle {
			found = true
			if slot.GaItems[i].ItemID != weaponItemID {
				t.Errorf("physical GaItem for 0x%08X has ItemID 0x%08X, want 0x%08X",
					newHandle, slot.GaItems[i].ItemID, weaponItemID)
			}
		}
	}
	if !found {
		t.Errorf("no physical GaItem record for new weapon handle 0x%08X", newHandle)
	}
	if slot.NextArmamentIndex <= beforeArmament {
		t.Errorf("NextArmamentIndex %d -> %d, want advance (physical weapon record)", beforeArmament, slot.NextArmamentIndex)
	}
}
