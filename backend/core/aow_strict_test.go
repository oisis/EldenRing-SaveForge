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

	// Caller may still pass the legacy sentinel — writer must accept it
	// and canonicalize to NoCustomAoWHandle on disk.
	if err := PatchWeaponAoWHandle(slot, wepHandle, LegacyNoCustomAoWHandle); err != nil {
		t.Fatalf("remove AoW: %v", err)
	}
	if slot.GaItems[1].AoWGaItemHandle != NoCustomAoWHandle {
		t.Errorf("in-memory: expected AoWGaItemHandle=0x%08X, got 0x%08X", NoCustomAoWHandle, slot.GaItems[1].AoWGaItemHandle)
	}
	// Verify byte patch in slot.Data.
	// weapon is at GaItemsStart + (AoW 8 bytes) = GaItemsStart+8, AoWGaItemHandle at +16.
	wepOff := GaItemsStart + 8 // AoW record is 8 bytes
	got := binary.LittleEndian.Uint32(slot.Data[wepOff+16:])
	if got != NoCustomAoWHandle {
		t.Errorf("slot.Data: expected 0x%08X at [%d+16], got 0x%08X", NoCustomAoWHandle, wepOff, got)
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

// --- vanilla-aligned sentinel regression tests ---

// TestPatchWeaponAoWHandle_RemoveWritesZeroSentinel asserts that the strict
// remove path emits the canonical vanilla sentinel (0x00000000) regardless
// of whether the caller passes the legacy or canonical sentinel. This is
// the load-bearing test that keeps SaveForge output matching what the
// game itself writes for weapons without a custom Ash of War attached.
func TestPatchWeaponAoWHandle_RemoveWritesZeroSentinel(t *testing.T) {
	cases := []struct {
		name  string
		input uint32
	}{
		{"caller_passes_canonical", NoCustomAoWHandle},
		{"caller_passes_legacy", LegacyNoCustomAoWHandle},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			slot := makeStrictTestSlot(10)
			const aowHandle = uint32(0xC0800001)
			const wepHandle = uint32(0x80800001)
			const wepItemID = uint32(0x00001000)
			addTestAoW(slot, 0, aowHandle, 0x80001000)
			addTestWeapon(slot, 1, wepHandle, wepItemID, aowHandle)
			flushTestGaItems(slot)

			if err := PatchWeaponAoWHandle(slot, wepHandle, tc.input); err != nil {
				t.Fatalf("remove AoW: %v", err)
			}
			if got := slot.GaItems[1].AoWGaItemHandle; got != NoCustomAoWHandle {
				t.Errorf("in-memory: expected 0x%08X, got 0x%08X", NoCustomAoWHandle, got)
			}
			wepOff := GaItemsStart + 8 // AoW record is 8 bytes
			got := binary.LittleEndian.Uint32(slot.Data[wepOff+16:])
			if got != NoCustomAoWHandle {
				t.Errorf("slot.Data: expected 0x%08X at [%d+16], got 0x%08X", NoCustomAoWHandle, wepOff, got)
			}
			// Default skill semantics: weapon ItemID must survive remove
			// untouched so the game can fall back to EquipParamWeapon.swordArtsParamId.
			if slot.GaItems[1].ItemID != wepItemID {
				t.Errorf("weapon ItemID changed by remove: expected 0x%08X, got 0x%08X", wepItemID, slot.GaItems[1].ItemID)
			}
		})
	}
}

// TestPatchWeaponAoWHandle_RemovePreservesWeaponItemID asserts the remove
// path never touches the weapon's own ItemID — the default skill is
// resolved from EquipParamWeapon.swordArtsParamId by Weapon.ItemID, so
// corrupting it would break the game's fallback after remove.
func TestPatchWeaponAoWHandle_RemovePreservesWeaponItemID(t *testing.T) {
	slot := makeStrictTestSlot(10)
	const aowHandle = uint32(0xC0800001)
	const wepHandle = uint32(0x80800001)
	const wepItemID = uint32(0x001F20C0) // Lordsworn's Straight Sword
	addTestAoW(slot, 0, aowHandle, 0x80001000)
	addTestWeapon(slot, 1, wepHandle, wepItemID, aowHandle)
	flushTestGaItems(slot)

	if err := PatchWeaponAoWHandle(slot, wepHandle, NoCustomAoWHandle); err != nil {
		t.Fatalf("remove: %v", err)
	}
	wepOff := GaItemsStart + 8
	gotID := binary.LittleEndian.Uint32(slot.Data[wepOff+4:])
	if gotID != wepItemID {
		t.Errorf("slot.Data weapon ItemID: expected 0x%08X, got 0x%08X", wepItemID, gotID)
	}
	if slot.GaItems[1].ItemID != wepItemID {
		t.Errorf("in-memory weapon ItemID: expected 0x%08X, got 0x%08X", wepItemID, slot.GaItems[1].ItemID)
	}
}

// TestPatchWeaponAoW_LegacyRemoveWritesZeroSentinel covers the legacy
// PatchWeaponAoW(slot, weaponHandle, 0) remove path.
func TestPatchWeaponAoW_LegacyRemoveWritesZeroSentinel(t *testing.T) {
	slot := makeStrictTestSlot(10)
	const aowHandle = uint32(0xC0800001)
	const wepHandle = uint32(0x80800001)
	const wepItemID = uint32(0x00001000)
	addTestAoW(slot, 0, aowHandle, 0x80001000)
	addTestWeapon(slot, 1, wepHandle, wepItemID, aowHandle)
	flushTestGaItems(slot)

	if err := PatchWeaponAoW(slot, wepHandle, 0); err != nil {
		t.Fatalf("PatchWeaponAoW remove: %v", err)
	}
	if got := slot.GaItems[1].AoWGaItemHandle; got != NoCustomAoWHandle {
		t.Errorf("in-memory: expected 0x%08X, got 0x%08X", NoCustomAoWHandle, got)
	}
	wepOff := GaItemsStart + 8
	got := binary.LittleEndian.Uint32(slot.Data[wepOff+16:])
	if got != NoCustomAoWHandle {
		t.Errorf("slot.Data: expected 0x%08X at [%d+16], got 0x%08X", NoCustomAoWHandle, wepOff, got)
	}
}

// TestAllocateGaItem_NewWeaponUsesZeroSentinel pins down the new-weapon
// path: every weapon GaItem that SaveForge allocates fresh starts with
// the canonical NoCustomAoWHandle, matching vanilla saves.
func TestAllocateGaItem_NewWeaponUsesZeroSentinel(t *testing.T) {
	slot := makeTestSlot(20)
	slot.NextAoWIndex = 0
	slot.NextArmamentIndex = 0

	const wepHandle = uint32(ItemTypeWeapon | 0x00800001)
	const wepItemID = uint32(0x00100000)
	if err := allocateGaItem(slot, wepHandle, wepItemID); err != nil {
		t.Fatalf("allocateGaItem: %v", err)
	}
	g := slot.GaItems[0]
	if g.Handle != wepHandle {
		t.Fatalf("expected weapon at index 0, got handle 0x%08X", g.Handle)
	}
	if g.AoWGaItemHandle != NoCustomAoWHandle {
		t.Errorf("new weapon AoWGaItemHandle: expected canonical 0x%08X, got 0x%08X",
			NoCustomAoWHandle, g.AoWGaItemHandle)
	}
}

// TestScanAoWAvailability_ZeroSentinelNotCounted asserts the canonical
// sentinel doesn't accidentally appear as a weapon reference (which
// would create a fake `weaponRefs[0]` entry).
func TestScanAoWAvailability_ZeroSentinelNotCounted(t *testing.T) {
	slot := makeStrictTestSlot(10)
	const aowHandle = uint32(0xC0800001)
	const aowItemID = uint32(0x80001000)
	addTestAoW(slot, 0, aowHandle, aowItemID)
	// Weapon with no custom AoW — vanilla sentinel.
	addTestWeapon(slot, 1, 0x80800001, 0x00001000, NoCustomAoWHandle)
	flushTestGaItems(slot)

	copies := ScanAoWAvailability(slot)
	if len(copies) != 1 {
		t.Fatalf("expected 1 AoW copy, got %d", len(copies))
	}
	if copies[0].UsedByWeaponHandle != 0 {
		t.Errorf("AoW copy should be free; got UsedByWeaponHandle=0x%08X", copies[0].UsedByWeaponHandle)
	}
	if copies[0].HasSharedHandleConflict {
		t.Error("unexpected conflict flagged for zero-sentinel weapon")
	}
}

// TestScanAoWAvailability_LegacyFFFFFFFFSentinelNotCounted asserts the
// legacy sentinel is still recognized as "no custom AoW" so saves
// produced by older SaveForge releases keep classifying correctly.
func TestScanAoWAvailability_LegacyFFFFFFFFSentinelNotCounted(t *testing.T) {
	slot := makeStrictTestSlot(10)
	const aowHandle = uint32(0xC0800001)
	addTestAoW(slot, 0, aowHandle, 0x80001000)
	addTestWeapon(slot, 1, 0x80800001, 0x00001000, LegacyNoCustomAoWHandle)
	flushTestGaItems(slot)

	copies := ScanAoWAvailability(slot)
	if len(copies) != 1 {
		t.Fatalf("expected 1 AoW copy, got %d", len(copies))
	}
	if copies[0].UsedByWeaponHandle != 0 {
		t.Errorf("AoW copy should be free; got UsedByWeaponHandle=0x%08X", copies[0].UsedByWeaponHandle)
	}
	if copies[0].HasSharedHandleConflict {
		t.Error("unexpected conflict flagged for legacy-sentinel weapon")
	}
}

// TestIsNoCustomAoWHandle covers the dual-sentinel helper directly.
func TestIsNoCustomAoWHandle(t *testing.T) {
	if !IsNoCustomAoWHandle(NoCustomAoWHandle) {
		t.Error("NoCustomAoWHandle should be recognized")
	}
	if !IsNoCustomAoWHandle(LegacyNoCustomAoWHandle) {
		t.Error("LegacyNoCustomAoWHandle should be recognized")
	}
	if IsNoCustomAoWHandle(0xC0800001) {
		t.Error("valid AoW handle 0xC0800001 must NOT be classified as no-custom")
	}
	if IsNoCustomAoWHandle(0x80800001) {
		t.Error("valid weapon handle 0x80800001 must NOT be classified as no-custom")
	}
}
