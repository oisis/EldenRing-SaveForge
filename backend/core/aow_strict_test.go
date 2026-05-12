package core

import (
	"encoding/binary"
	"testing"
)

// makeStrictTestSlot builds a SaveSlot with serialized GaItems in slot.Data
// so that PatchWeaponAoWHandle can locate byte offsets correctly.
// numEntries controls the pre-allocated GaItems slice length.
func makeStrictTestSlot(numEntries int) *SaveSlot {
	slot := makeTestSlot(numEntries)
	// InventoryEnd will be updated after we populate GaItems.
	return slot
}

// flushTestGaItems serializes slot.GaItems into slot.Data and sets InventoryEnd.
func flushTestGaItems(slot *SaveSlot) {
	pos := GaItemsStart
	buf := slot.Data
	for i := range slot.GaItems {
		n := slot.GaItems[i].Serialize(buf[pos:])
		pos += n
	}
	slot.InventoryEnd = pos
}

// addTestAoW inserts an AoW GaItem at slot.GaItems[idx] and registers it in GaMap.
func addTestAoW(slot *SaveSlot, idx int, handle, itemID uint32) {
	slot.GaItems[idx] = GaItemFull{Handle: handle, ItemID: itemID, Unk2: -1, Unk3: -1, AoWGaItemHandle: 0xFFFFFFFF}
	slot.GaMap[handle] = itemID
}

// addTestWeapon inserts a weapon GaItem at slot.GaItems[idx] and registers it in GaMap.
func addTestWeapon(slot *SaveSlot, idx int, handle, itemID, aowHandle uint32) {
	slot.GaItems[idx] = GaItemFull{Handle: handle, ItemID: itemID, Unk2: -1, Unk3: -1, AoWGaItemHandle: aowHandle}
	slot.GaMap[handle] = itemID
}

// --- ScanAoWAvailability tests ---

func TestScanAoWAvailability_FreeAndUsedCopies(t *testing.T) {
	slot := makeStrictTestSlot(10)

	const aowItemID = uint32(0x80001000)
	const aowHandle1 = uint32(0xC0800001) // free
	const aowHandle2 = uint32(0xC0800002) // used by weapon1
	const wepHandle = uint32(0x80800001)
	const wepItemID = uint32(0x00001000)

	addTestAoW(slot, 0, aowHandle1, aowItemID)
	addTestAoW(slot, 1, aowHandle2, aowItemID)
	addTestWeapon(slot, 2, wepHandle, wepItemID, aowHandle2)

	copies := ScanAoWAvailability(slot)
	if len(copies) != 2 {
		t.Fatalf("expected 2 copies, got %d", len(copies))
	}

	free := 0
	used := 0
	for _, c := range copies {
		if c.ItemID != aowItemID {
			t.Errorf("unexpected itemID 0x%08X", c.ItemID)
		}
		if c.UsedByWeaponHandle == 0 {
			free++
		} else {
			used++
			if c.UsedByWeaponHandle != wepHandle {
				t.Errorf("used copy: expected weapon 0x%08X, got 0x%08X", wepHandle, c.UsedByWeaponHandle)
			}
		}
		if c.HasSharedHandleConflict {
			t.Errorf("unexpected conflict on copy 0x%08X", c.Handle)
		}
	}
	if free != 1 || used != 1 {
		t.Errorf("expected 1 free + 1 used, got %d free + %d used", free, used)
	}
}

func TestScanAoWAvailability_SharedHandleConflict(t *testing.T) {
	slot := makeStrictTestSlot(10)

	const aowItemID = uint32(0x80002000)
	const aowHandle = uint32(0xC0800010)
	const wep1Handle = uint32(0x80800011)
	const wep2Handle = uint32(0x80800012)
	const wepItemID = uint32(0x00002000)

	addTestAoW(slot, 0, aowHandle, aowItemID)
	addTestWeapon(slot, 1, wep1Handle, wepItemID, aowHandle)
	addTestWeapon(slot, 2, wep2Handle, wepItemID, aowHandle)

	copies := ScanAoWAvailability(slot)
	if len(copies) != 1 {
		t.Fatalf("expected 1 copy, got %d", len(copies))
	}
	if !copies[0].HasSharedHandleConflict {
		t.Error("expected HasSharedHandleConflict = true")
	}
	if copies[0].UsedByWeaponHandle == 0 {
		t.Error("expected UsedByWeaponHandle to be set")
	}
}

func TestScanAoWAvailability_EmptySlot(t *testing.T) {
	slot := makeStrictTestSlot(10)
	copies := ScanAoWAvailability(slot)
	if len(copies) != 0 {
		t.Errorf("expected 0 copies for empty slot, got %d", len(copies))
	}
}

// --- PatchWeaponAoWHandle tests ---

func TestPatchWeaponAoWHandle_NilSlot(t *testing.T) {
	err := PatchWeaponAoWHandle(nil, 0x80800001, 0xFFFFFFFF)
	if err == nil {
		t.Error("expected error for nil slot")
	}
}

func TestPatchWeaponAoWHandle_WeaponNotFound(t *testing.T) {
	slot := makeStrictTestSlot(10)
	addTestAoW(slot, 0, 0xC0800001, 0x80001000)
	flushTestGaItems(slot)

	err := PatchWeaponAoWHandle(slot, 0x80800099, 0xFFFFFFFF)
	if err == nil {
		t.Error("expected error for unknown weapon handle")
	}
}

func TestPatchWeaponAoWHandle_Remove(t *testing.T) {
	slot := makeStrictTestSlot(10)
	const aowHandle = uint32(0xC0800001)
	const wepHandle = uint32(0x80800001)
	addTestAoW(slot, 0, aowHandle, 0x80001000)
	addTestWeapon(slot, 1, wepHandle, 0x00001000, aowHandle)
	flushTestGaItems(slot)

	if err := PatchWeaponAoWHandle(slot, wepHandle, 0xFFFFFFFF); err != nil {
		t.Fatalf("remove AoW: %v", err)
	}
	if slot.GaItems[1].AoWGaItemHandle != 0xFFFFFFFF {
		t.Errorf("in-memory: expected AoWGaItemHandle=0xFFFFFFFF, got 0x%08X", slot.GaItems[1].AoWGaItemHandle)
	}
	// Verify byte patch in slot.Data.
	// weapon is at GaItemsStart + (AoW 8 bytes) = GaItemsStart+8, AoWGaItemHandle at +16.
	wepOff := GaItemsStart + 8 // AoW record is 8 bytes
	got := binary.LittleEndian.Uint32(slot.Data[wepOff+16:])
	if got != 0xFFFFFFFF {
		t.Errorf("slot.Data: expected 0xFFFFFFFF at [%d+16], got 0x%08X", wepOff, got)
	}
}

func TestPatchWeaponAoWHandle_AoWHandleNotFound(t *testing.T) {
	slot := makeStrictTestSlot(10)
	const wepHandle = uint32(0x80800001)
	addTestWeapon(slot, 0, wepHandle, 0x00001000, 0xFFFFFFFF)
	flushTestGaItems(slot)

	err := PatchWeaponAoWHandle(slot, wepHandle, 0xC0800099)
	if err == nil {
		t.Error("expected error for unknown AoW handle")
	}
}

func TestPatchWeaponAoWHandle_AoWAlreadyUsed(t *testing.T) {
	slot := makeStrictTestSlot(10)
	const aowHandle = uint32(0xC0800001)
	const wep1Handle = uint32(0x80800001)
	const wep2Handle = uint32(0x80800002)
	addTestAoW(slot, 0, aowHandle, 0x80001000)
	addTestWeapon(slot, 1, wep1Handle, 0x00001000, aowHandle) // already uses aowHandle
	addTestWeapon(slot, 2, wep2Handle, 0x00001001, 0xFFFFFFFF)
	flushTestGaItems(slot)

	err := PatchWeaponAoWHandle(slot, wep2Handle, aowHandle)
	if err == nil {
		t.Error("expected error: AoW handle already used by another weapon")
	}
}

func TestPatchWeaponAoWHandle_AttachFreeHandle(t *testing.T) {
	slot := makeStrictTestSlot(10)
	const aowHandle = uint32(0xC0800001)
	const wepHandle = uint32(0x80800001)
	addTestAoW(slot, 0, aowHandle, 0x80001000)
	addTestWeapon(slot, 1, wepHandle, 0x00001000, 0xFFFFFFFF)
	flushTestGaItems(slot)

	if err := PatchWeaponAoWHandle(slot, wepHandle, aowHandle); err != nil {
		t.Fatalf("attach free AoW: %v", err)
	}
	if slot.GaItems[1].AoWGaItemHandle != aowHandle {
		t.Errorf("expected AoWGaItemHandle=0x%08X, got 0x%08X", aowHandle, slot.GaItems[1].AoWGaItemHandle)
	}
}
