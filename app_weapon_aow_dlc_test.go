package main

import (
	"encoding/binary"
	"strings"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
)

// weaponAoWDLCFixture builds an App with one free AoW copy (Sword Dance, 0x80003070)
// and one Dragon Towershield weapon (0x01E84800, wepType=69, GemMountType=2).
// wepType 69 is absent from WepTypeToCanMountBit, so IsAshOfWarCompatibleWithWeapon
// returns known=false — the compatibility passthrough case.
//
// Layout in slot.Data from GaItemsStart (0x20 = 32):
//
//	[32:40]  AoW GaItem    (8 bytes):  handle | itemID
//	[40:61]  Weapon GaItem (21 bytes): handle | itemID | Unk2 | Unk3 | AoWHandle | Unk5
func weaponAoWDLCFixture() *App {
	const (
		wepHandle = uint32(0x80800001)
		wepItemID = uint32(0x01E84800) // Dragon Towershield +0 — wepType=69, GemMountType=2
		aowHandle = uint32(0xC0800001)
		aowItemID = uint32(0x80003070) // Sword Dance — has non-zero bitmask, wepType 69 not in map
		gaStart   = core.GaItemsStart
		gaAoWSize = core.GaRecordAoW    // 8
		gaWepSize = core.GaRecordWeapon // 21
		bufSize   = 512
	)

	app := NewApp()
	app.save = &core.SaveFile{}
	slot := &app.save.Slots[0]

	slot.GaItems = []core.GaItemFull{
		{Handle: aowHandle, ItemID: aowItemID, Unk2: -1, Unk3: -1, AoWGaItemHandle: 0xFFFFFFFF},
		{Handle: wepHandle, ItemID: wepItemID, Unk2: -1, Unk3: -1, AoWGaItemHandle: 0xFFFFFFFF, Unk5: 0},
	}
	slot.GaMap = map[uint32]uint32{
		aowHandle: aowItemID,
		wepHandle: wepItemID,
	}

	slot.Data = make([]byte, bufSize)
	off := gaStart
	binary.LittleEndian.PutUint32(slot.Data[off:], aowHandle)
	binary.LittleEndian.PutUint32(slot.Data[off+4:], aowItemID)
	off += gaAoWSize
	binary.LittleEndian.PutUint32(slot.Data[off:], wepHandle)
	binary.LittleEndian.PutUint32(slot.Data[off+4:], wepItemID)
	binary.LittleEndian.PutUint32(slot.Data[off+8:], 0xFFFFFFFF)  // Unk2
	binary.LittleEndian.PutUint32(slot.Data[off+12:], 0xFFFFFFFF) // Unk3
	binary.LittleEndian.PutUint32(slot.Data[off+16:], 0xFFFFFFFF) // AoWGaItemHandle (none)
	slot.Data[off+20] = 0                                         // Unk5

	slot.InventoryEnd = gaStart + gaAoWSize + gaWepSize // 61
	slot.GaItemDataOffset = 0

	return app
}

// ─── DLC unknown-compat passthrough — ApplyWeaponAoW (editor) ───────────────────

// buildDLCEditorSlot creates a full-size editor slot for a DLC weapon + free AoW copy.
// Uses the same setup as buildEditorSetSlot (Version=100, SlotSize, SectionMap) so
// PatchWeaponAoW can run RebuildSlotFull without failing on the verbatim-copy guard.
func buildDLCEditorSlot(wepHandle, wepItemID, aowHandle, aowItemID uint32) *App {
	entries := []core.GaItemFull{
		{Handle: aowHandle, ItemID: aowItemID, Unk2: -1, Unk3: -1, AoWGaItemHandle: 0xFFFFFFFF},
		{Handle: wepHandle, ItemID: wepItemID, Unk2: -1, Unk3: -1, AoWGaItemHandle: 0xFFFFFFFF},
	}
	gaData := serializeGaItemsEditor(entries)
	inventoryEnd := core.GaItemsStart + len(gaData)

	gaItems := make([]core.GaItemFull, 5120)
	copy(gaItems[:len(entries)], entries)

	gaMap := map[uint32]uint32{aowHandle: aowItemID, wepHandle: wepItemID}

	return buildEditorSetSlot(gaData, gaItems, gaMap, inventoryEnd,
		1, 2, 0x80, 2,
	)
}

// TestApplyWeaponAoW_DLCUnmappedWepType_Allows verifies that a weapon with an unmapped
// DLC wepType (wepType=69, Dragon Towershield, no entry in WepTypeToCanMountBit) and
// GemMountType=2 can have an Ash of War applied even though
// IsAshOfWarCompatibleWithWeapon returns known=false.
func TestApplyWeaponAoW_DLCUnmappedWepType_Allows(t *testing.T) {
	const (
		wepHandle = uint32(0x80800001)
		wepItemID = uint32(0x01E84800) // Dragon Towershield +0 — wepType=69, GemMountType=2
		aowHandle = uint32(0xC0800001)
		aowItemID = uint32(0x80003070) // Sword Dance
	)

	app := buildDLCEditorSlot(wepHandle, wepItemID, aowHandle, aowItemID)
	err := app.ApplyWeaponAoW(0, wepHandle, aowItemID)
	if err != nil {
		t.Fatalf("ApplyWeaponAoW with DLC wepType=69: unexpected error: %v", err)
	}
}

// TestApplyWeaponAoW_DLCGreatKatana_Allows tests wepType=94 (Great Katana, DLC).
func TestApplyWeaponAoW_DLCGreatKatana_Allows(t *testing.T) {
	const (
		wepHandle = uint32(0x80800001)
		wepItemID = uint32(0x03F6B5A0) // Great Katana +0 — wepType=94, GemMountType=2
		aowHandle = uint32(0xC0800001)
		aowItemID = uint32(0x80003070) // Sword Dance
	)

	app := buildDLCEditorSlot(wepHandle, wepItemID, aowHandle, aowItemID)
	err := app.ApplyWeaponAoW(0, wepHandle, aowItemID)
	if err != nil {
		t.Fatalf("ApplyWeaponAoW with DLC wepType=94: unexpected error: %v", err)
	}
}

// ─── DLC unknown-compat passthrough — ApplyWeaponAoWStrict ──────────────────────

// TestApplyWeaponAoWStrict_DLCUnmappedWepType_Allows verifies that strict mode also allows
// the operation when known=false for a DLC weapon — the free copy is linked in-place.
func TestApplyWeaponAoWStrict_DLCUnmappedWepType_Allows(t *testing.T) {
	const (
		wepHandle = uint32(0x80800001)
		aowItemID = uint32(0x80003070)
	)

	app := weaponAoWDLCFixture()
	err := app.ApplyWeaponAoWStrict(0, wepHandle, aowItemID)
	if err != nil {
		t.Fatalf("ApplyWeaponAoWStrict with DLC wepType=69: unexpected error: %v", err)
	}
}

// ─── known==true, incompatible — still blocked ──────────────────────────────────

// TestApplyWeaponAoW_KnownIncompatible_Blocks ensures that known=true + compatible=false
// is still rejected after the passthrough change.
// Dagger (wepType=1) + shield-only AoW (0x80007530, bits 32-34 only) → incompatible.
func TestApplyWeaponAoW_KnownIncompatible_Blocks(t *testing.T) {
	const (
		wepHandle       = uint32(0x80800001)
		wepItemID       = uint32(0x000F4240) // Standard Dagger +0, wepType=1
		aowHandle       = uint32(0xC0800001)
		shieldOnlyAoWID = uint32(0x80007530) // compatible with ShieldSmall/Normal/Large only
		gaStart         = core.GaItemsStart
		gaAoWSize       = core.GaRecordAoW
		gaWepSize       = core.GaRecordWeapon
		bufSize         = 512
	)

	app := NewApp()
	app.save = &core.SaveFile{}
	slot := &app.save.Slots[0]
	slot.GaItems = []core.GaItemFull{
		{Handle: aowHandle, ItemID: shieldOnlyAoWID, Unk2: -1, Unk3: -1, AoWGaItemHandle: 0xFFFFFFFF},
		{Handle: wepHandle, ItemID: wepItemID, Unk2: -1, Unk3: -1, AoWGaItemHandle: 0xFFFFFFFF},
	}
	slot.GaMap = map[uint32]uint32{aowHandle: shieldOnlyAoWID, wepHandle: wepItemID}
	slot.Data = make([]byte, bufSize)
	off := gaStart
	binary.LittleEndian.PutUint32(slot.Data[off:], aowHandle)
	binary.LittleEndian.PutUint32(slot.Data[off+4:], shieldOnlyAoWID)
	off += gaAoWSize
	binary.LittleEndian.PutUint32(slot.Data[off:], wepHandle)
	binary.LittleEndian.PutUint32(slot.Data[off+4:], wepItemID)
	binary.LittleEndian.PutUint32(slot.Data[off+8:], 0xFFFFFFFF)
	binary.LittleEndian.PutUint32(slot.Data[off+12:], 0xFFFFFFFF)
	binary.LittleEndian.PutUint32(slot.Data[off+16:], 0xFFFFFFFF)
	slot.Data[off+20] = 0
	slot.InventoryEnd = gaStart + gaAoWSize + gaWepSize
	slot.GaItemDataOffset = 0

	err := app.ApplyWeaponAoW(0, wepHandle, shieldOnlyAoWID)
	if err == nil || !strings.Contains(err.Error(), "not compatible") {
		t.Fatalf("want 'not compatible', got %v", err)
	}
}

// ─── Non-mountable weapon — still blocked ────────────────────────────────────────

// TestApplyWeaponAoW_NonMountableWeapon_Blocks ensures that weapons without AoW support
// (GemMountType != 2, e.g. unique/somber weapons not in weapon_gem_mount) are blocked
// before the compatibility check, unaffected by the passthrough change.
// Moonveil (0x008A3EA0) is in the DB but has no weapon_gem_mount entry → GemMountType=0.
func TestApplyWeaponAoW_NonMountableWeapon_Blocks(t *testing.T) {
	const (
		wepHandle = uint32(0x80800001)
		wepItemID = uint32(0x008A3EA0) // Moonveil — in DB, GemMountType=0 (not in weapon_gem_mount)
		aowHandle = uint32(0xC0800001)
		aowItemID = uint32(0x80003070)
		gaStart   = core.GaItemsStart
		gaAoWSize = core.GaRecordAoW
		gaWepSize = core.GaRecordWeapon
		bufSize   = 512
	)

	app := NewApp()
	app.save = &core.SaveFile{}
	slot := &app.save.Slots[0]
	slot.GaItems = []core.GaItemFull{
		{Handle: aowHandle, ItemID: aowItemID, Unk2: -1, Unk3: -1, AoWGaItemHandle: 0xFFFFFFFF},
		{Handle: wepHandle, ItemID: wepItemID, Unk2: -1, Unk3: -1, AoWGaItemHandle: 0xFFFFFFFF},
	}
	slot.GaMap = map[uint32]uint32{aowHandle: aowItemID, wepHandle: wepItemID}
	slot.Data = make([]byte, bufSize)
	off := gaStart
	binary.LittleEndian.PutUint32(slot.Data[off:], aowHandle)
	binary.LittleEndian.PutUint32(slot.Data[off+4:], aowItemID)
	off += gaAoWSize
	binary.LittleEndian.PutUint32(slot.Data[off:], wepHandle)
	binary.LittleEndian.PutUint32(slot.Data[off+4:], wepItemID)
	binary.LittleEndian.PutUint32(slot.Data[off+8:], 0xFFFFFFFF)
	binary.LittleEndian.PutUint32(slot.Data[off+12:], 0xFFFFFFFF)
	binary.LittleEndian.PutUint32(slot.Data[off+16:], 0xFFFFFFFF)
	slot.Data[off+20] = 0
	slot.InventoryEnd = gaStart + gaAoWSize + gaWepSize
	slot.GaItemDataOffset = 0

	err := app.ApplyWeaponAoW(0, wepHandle, aowItemID)
	if err == nil || !strings.Contains(err.Error(), "does not support Ash of War") {
		t.Fatalf("want 'does not support Ash of War', got %v", err)
	}
}

// ─── Remove AoW — always allowed ────────────────────────────────────────────────

// TestApplyWeaponAoW_RemoveAlwaysAllowed verifies that newAoWItemID==0 (remove) skips
// the compatibility check entirely and always succeeds for mountable weapons.
func TestApplyWeaponAoW_RemoveAlwaysAllowed(t *testing.T) {
	const (
		aowHandle = uint32(0xC0800001)
		aowItemID = uint32(0x80003070)
		wepHandle = uint32(0x80800001)
		gaStart   = core.GaItemsStart
		gaAoWSize = core.GaRecordAoW
		gaWepSize = core.GaRecordWeapon
	)

	app := weaponAoWDLCFixture()
	slot := &app.save.Slots[0]

	// Pre-attach: weapon has AoW
	weaponByteOff := gaStart + gaAoWSize
	slot.GaItems[1].AoWGaItemHandle = aowHandle
	binary.LittleEndian.PutUint32(slot.Data[weaponByteOff+16:], aowHandle)

	err := app.ApplyWeaponAoW(0, wepHandle, 0)
	if err != nil {
		t.Fatalf("ApplyWeaponAoW remove: unexpected error: %v", err)
	}

	// AoWGaItemHandle should be canonical NoCustomAoWHandle after remove (vanilla-aligned).
	if slot.GaItems[1].AoWGaItemHandle != core.NoCustomAoWHandle {
		t.Errorf("after remove: AoWGaItemHandle = 0x%08X, want 0x%08X", slot.GaItems[1].AoWGaItemHandle, core.NoCustomAoWHandle)
	}
}

// TestApplyWeaponAoWStrict_RemoveAlwaysAllowed mirrors the remove test for strict mode.
func TestApplyWeaponAoWStrict_RemoveAlwaysAllowed(t *testing.T) {
	const (
		aowHandle = uint32(0xC0800001)
		wepHandle = uint32(0x80800001)
		gaStart   = core.GaItemsStart
		gaAoWSize = core.GaRecordAoW
	)

	app := weaponAoWDLCFixture()
	slot := &app.save.Slots[0]

	weaponByteOff := gaStart + gaAoWSize
	slot.GaItems[1].AoWGaItemHandle = aowHandle
	binary.LittleEndian.PutUint32(slot.Data[weaponByteOff+16:], aowHandle)

	err := app.ApplyWeaponAoWStrict(0, wepHandle, 0)
	if err != nil {
		t.Fatalf("ApplyWeaponAoWStrict remove: unexpected error: %v", err)
	}

	if slot.GaItems[1].AoWGaItemHandle != core.NoCustomAoWHandle {
		t.Errorf("after remove: AoWGaItemHandle = 0x%08X, want 0x%08X", slot.GaItems[1].AoWGaItemHandle, core.NoCustomAoWHandle)
	}
}
