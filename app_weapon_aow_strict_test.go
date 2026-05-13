package main

import (
	"encoding/binary"
	"strings"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
)

// weaponAoWStrictFixture returns an App with one free AoW copy (Sword Dance, 0x80003070)
// and one Dagger weapon (0x000F4240) with no AoW attached, both serialized into slot.Data.
//
// Layout in slot.Data from GaItemsStart (0x20 = 32):
//
//	[32:40]  AoW GaItem    (8 bytes):  handle | itemID
//	[40:61]  Weapon GaItem (21 bytes): handle | itemID | Unk2 | Unk3 | AoWHandle | Unk5
//
// slot.InventoryEnd = 61. slot.GaItemDataOffset = 0 (upsertGaItemData no-op).
func weaponAoWStrictFixture() *App {
	const (
		wepHandle = uint32(0x80800001) // ItemTypeWeapon prefix 0x8
		wepItemID = uint32(0x000F4240) // Standard Dagger +0
		aowHandle = uint32(0xC0800001) // ItemTypeAow prefix 0xC
		aowItemID = uint32(0x80003070) // Sword Dance
		gaStart   = core.GaItemsStart // 0x20 = 32
		gaAoWSize = core.GaRecordAoW  // 8
		gaWepSize = core.GaRecordWeapon // 21
		bufSize   = 512
	)

	app := NewApp()
	app.save = &core.SaveFile{}
	slot := &app.save.Slots[0]

	slot.GaItems = []core.GaItemFull{
		// AoW copy — free (not referenced by any weapon)
		{Handle: aowHandle, ItemID: aowItemID, Unk2: -1, Unk3: -1, AoWGaItemHandle: 0xFFFFFFFF},
		// Weapon — no AoW currently attached
		{Handle: wepHandle, ItemID: wepItemID, Unk2: -1, Unk3: -1, AoWGaItemHandle: 0xFFFFFFFF, Unk5: 0},
	}
	slot.GaMap = map[uint32]uint32{
		aowHandle: aowItemID,
		wepHandle: wepItemID,
	}

	slot.Data = make([]byte, bufSize)

	// Serialize AoW at GaItemsStart.
	off := gaStart
	binary.LittleEndian.PutUint32(slot.Data[off:], aowHandle)
	binary.LittleEndian.PutUint32(slot.Data[off+4:], aowItemID)

	// Serialize weapon immediately after AoW.
	off += gaAoWSize
	binary.LittleEndian.PutUint32(slot.Data[off:], wepHandle)
	binary.LittleEndian.PutUint32(slot.Data[off+4:], wepItemID)
	binary.LittleEndian.PutUint32(slot.Data[off+8:], 0xFFFFFFFF)  // Unk2
	binary.LittleEndian.PutUint32(slot.Data[off+12:], 0xFFFFFFFF) // Unk3
	binary.LittleEndian.PutUint32(slot.Data[off+16:], 0xFFFFFFFF) // AoWGaItemHandle (none)
	slot.Data[off+20] = 0                                          // Unk5

	slot.InventoryEnd = gaStart + gaAoWSize + gaWepSize // 61
	slot.GaItemDataOffset = 0

	return app
}

// ─── Guard tests ────────────────────────────────────────────────────────────────

func TestApplyWeaponAoWStrict_NoSave(t *testing.T) {
	app := NewApp()
	err := app.ApplyWeaponAoWStrict(0, 0x80800001, 0x80003070)
	if err == nil || !strings.Contains(err.Error(), "no save loaded") {
		t.Fatalf("want 'no save loaded', got %v", err)
	}
}

func TestApplyWeaponAoWStrict_InvalidCharIdx(t *testing.T) {
	app := weaponAoWStrictFixture()
	for _, idx := range []int{-1, 10, 99} {
		err := app.ApplyWeaponAoWStrict(idx, 0x80800001, 0x80003070)
		if err == nil || !strings.Contains(err.Error(), "invalid character index") {
			t.Errorf("idx %d: want 'invalid character index', got %v", idx, err)
		}
	}
}

func TestApplyWeaponAoWStrict_HandleNotFound(t *testing.T) {
	app := weaponAoWStrictFixture()
	err := app.ApplyWeaponAoWStrict(0, 0xDEADBEEF, 0x80003070)
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("want 'not found', got %v", err)
	}
}

// ─── Scenario A: happy path — attach free AoW copy to weapon ───────────────────

func TestApplyWeaponAoWStrict_AttachFreeAoW(t *testing.T) {
	const (
		wepHandle = uint32(0x80800001)
		wepItemID = uint32(0x000F4240)
		aowHandle = uint32(0xC0800001)
		aowItemID = uint32(0x80003070)
		gaStart   = core.GaItemsStart
		gaAoWSize = core.GaRecordAoW
	)

	app := weaponAoWStrictFixture()
	slot := &app.save.Slots[0]

	err := app.ApplyWeaponAoWStrict(0, wepHandle, aowItemID)
	if err != nil {
		t.Fatalf("ApplyWeaponAoWStrict: unexpected error: %v", err)
	}

	// AoWGaItemHandle updated in GaItems in-memory (weapon is GaItems[1]).
	if slot.GaItems[1].AoWGaItemHandle != aowHandle {
		t.Errorf("GaItems[1].AoWGaItemHandle = 0x%08X, want 0x%08X", slot.GaItems[1].AoWGaItemHandle, aowHandle)
	}

	// AoWGaItemHandle patched in slot.Data at weaponByteOff+16.
	weaponByteOff := gaStart + gaAoWSize // 32 + 8 = 40
	dataAoWHandle := binary.LittleEndian.Uint32(slot.Data[weaponByteOff+16:])
	if dataAoWHandle != aowHandle {
		t.Errorf("slot.Data AoWHandle = 0x%08X, want 0x%08X", dataAoWHandle, aowHandle)
	}

	// Weapon ItemID and handle unchanged.
	if slot.GaItems[1].ItemID != wepItemID {
		t.Errorf("GaItems[1].ItemID changed: 0x%08X", slot.GaItems[1].ItemID)
	}
	if slot.GaItems[1].Handle != wepHandle {
		t.Errorf("GaItems[1].Handle changed: 0x%08X", slot.GaItems[1].Handle)
	}

	// AoW GaItem itself unchanged.
	if slot.GaItems[0].Handle != aowHandle || slot.GaItems[0].ItemID != aowItemID {
		t.Errorf("AoW GaItem mutated: handle=0x%08X itemID=0x%08X", slot.GaItems[0].Handle, slot.GaItems[0].ItemID)
	}
}

// ─── Scenario B: remove AoW ─────────────────────────────────────────────────────

func TestApplyWeaponAoWStrict_RemoveAoW(t *testing.T) {
	const (
		wepHandle = uint32(0x80800001)
		aowHandle = uint32(0xC0800001)
		gaStart   = core.GaItemsStart
		gaAoWSize = core.GaRecordAoW
	)

	app := weaponAoWStrictFixture()
	slot := &app.save.Slots[0]

	// Pre-attach: weapon currently has AoW.
	weaponByteOff := gaStart + gaAoWSize // 40
	slot.GaItems[1].AoWGaItemHandle = aowHandle
	binary.LittleEndian.PutUint32(slot.Data[weaponByteOff+16:], aowHandle)

	err := app.ApplyWeaponAoWStrict(0, wepHandle, 0)
	if err != nil {
		t.Fatalf("ApplyWeaponAoWStrict (remove): unexpected error: %v", err)
	}

	// AoWGaItemHandle reset to 0xFFFFFFFF in GaItems.
	if slot.GaItems[1].AoWGaItemHandle != 0xFFFFFFFF {
		t.Errorf("GaItems[1].AoWGaItemHandle = 0x%08X, want 0xFFFFFFFF", slot.GaItems[1].AoWGaItemHandle)
	}

	// AoWGaItemHandle reset to 0xFFFFFFFF in slot.Data.
	dataAoWHandle := binary.LittleEndian.Uint32(slot.Data[weaponByteOff+16:])
	if dataAoWHandle != 0xFFFFFFFF {
		t.Errorf("slot.Data AoWHandle = 0x%08X, want 0xFFFFFFFF", dataAoWHandle)
	}
}

// ─── Scenario C: AoW not present in save ────────────────────────────────────────

func TestApplyWeaponAoWStrict_AoWNotInSave(t *testing.T) {
	const (
		wepHandle = uint32(0x80800001)
		wepItemID = uint32(0x000F4240)
		aowItemID = uint32(0x80003070)
		bufSize   = 512
	)

	// Weapon-only save — no AoW GaItem present.
	app := NewApp()
	app.save = &core.SaveFile{}
	slot := &app.save.Slots[0]
	slot.GaItems = []core.GaItemFull{
		{Handle: wepHandle, ItemID: wepItemID, Unk2: -1, Unk3: -1, AoWGaItemHandle: 0xFFFFFFFF},
	}
	slot.GaMap = map[uint32]uint32{wepHandle: wepItemID}
	slot.Data = make([]byte, bufSize)
	off := core.GaItemsStart
	binary.LittleEndian.PutUint32(slot.Data[off:], wepHandle)
	binary.LittleEndian.PutUint32(slot.Data[off+4:], wepItemID)
	binary.LittleEndian.PutUint32(slot.Data[off+8:], 0xFFFFFFFF)
	binary.LittleEndian.PutUint32(slot.Data[off+12:], 0xFFFFFFFF)
	binary.LittleEndian.PutUint32(slot.Data[off+16:], 0xFFFFFFFF)
	slot.Data[off+20] = 0
	slot.InventoryEnd = core.GaItemsStart + core.GaRecordWeapon
	slot.GaItemDataOffset = 0

	err := app.ApplyWeaponAoWStrict(0, wepHandle, aowItemID)
	if err == nil || !strings.Contains(err.Error(), "not present in save") {
		t.Fatalf("want 'not present in save', got %v", err)
	}

	// Weapon AoWGaItemHandle must remain 0xFFFFFFFF.
	if slot.GaItems[0].AoWGaItemHandle != 0xFFFFFFFF {
		t.Errorf("weapon AoWGaItemHandle changed: 0x%08X", slot.GaItems[0].AoWGaItemHandle)
	}
}

// ─── Scenario D: AoW already used by another weapon ─────────────────────────────

func TestApplyWeaponAoWStrict_AoWAlreadyUsed(t *testing.T) {
	const (
		aowHandle  = uint32(0xC0800001)
		aowItemID  = uint32(0x80003070) // Sword Dance
		wep1Handle = uint32(0x80800001) // holds the only AoW copy
		wep2Handle = uint32(0x80800002) // tries to claim it — should fail
		wepItemID  = uint32(0x000F4240) // Standard Dagger +0
		bufSize    = 512
		gaStart    = core.GaItemsStart
		gaAoWSize  = core.GaRecordAoW
		gaWepSize  = core.GaRecordWeapon
	)

	app := NewApp()
	app.save = &core.SaveFile{}
	slot := &app.save.Slots[0]

	slot.GaItems = []core.GaItemFull{
		{Handle: aowHandle, ItemID: aowItemID, Unk2: -1, Unk3: -1, AoWGaItemHandle: 0xFFFFFFFF},
		{Handle: wep1Handle, ItemID: wepItemID, Unk2: -1, Unk3: -1, AoWGaItemHandle: aowHandle},  // uses AoW
		{Handle: wep2Handle, ItemID: wepItemID, Unk2: -1, Unk3: -1, AoWGaItemHandle: 0xFFFFFFFF}, // wants AoW
	}
	slot.GaMap = map[uint32]uint32{
		aowHandle:  aowItemID,
		wep1Handle: wepItemID,
		wep2Handle: wepItemID,
	}

	slot.Data = make([]byte, bufSize)
	off := gaStart
	// AoW
	binary.LittleEndian.PutUint32(slot.Data[off:], aowHandle)
	binary.LittleEndian.PutUint32(slot.Data[off+4:], aowItemID)
	off += gaAoWSize
	// Weapon1 (AoW attached)
	binary.LittleEndian.PutUint32(slot.Data[off:], wep1Handle)
	binary.LittleEndian.PutUint32(slot.Data[off+4:], wepItemID)
	binary.LittleEndian.PutUint32(slot.Data[off+8:], 0xFFFFFFFF)
	binary.LittleEndian.PutUint32(slot.Data[off+12:], 0xFFFFFFFF)
	binary.LittleEndian.PutUint32(slot.Data[off+16:], aowHandle) // used
	slot.Data[off+20] = 0
	off += gaWepSize
	// Weapon2 (no AoW)
	binary.LittleEndian.PutUint32(slot.Data[off:], wep2Handle)
	binary.LittleEndian.PutUint32(slot.Data[off+4:], wepItemID)
	binary.LittleEndian.PutUint32(slot.Data[off+8:], 0xFFFFFFFF)
	binary.LittleEndian.PutUint32(slot.Data[off+12:], 0xFFFFFFFF)
	binary.LittleEndian.PutUint32(slot.Data[off+16:], 0xFFFFFFFF)
	slot.Data[off+20] = 0
	off += gaWepSize

	slot.InventoryEnd = off // 32 + 8 + 21 + 21 = 82
	slot.GaItemDataOffset = 0

	// Weapon2 tries to claim the only copy — already used by Weapon1.
	err := app.ApplyWeaponAoWStrict(0, wep2Handle, aowItemID)
	if err == nil || !strings.Contains(err.Error(), "no free copy") {
		t.Fatalf("want 'no free copy', got %v", err)
	}

	// Weapon2 AoWGaItemHandle must remain unchanged.
	if slot.GaItems[2].AoWGaItemHandle != 0xFFFFFFFF {
		t.Errorf("weapon2 AoWGaItemHandle changed: 0x%08X", slot.GaItems[2].AoWGaItemHandle)
	}
}
