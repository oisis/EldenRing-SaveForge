package main

import (
	"encoding/binary"
	"strings"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
)

// weaponInfusionFixture returns an App with a single fully wired weapon GaItem
// in slot 0. The weapon is a Standard Dagger +0 (baseID 0x000F4240, infuseOffset=0).
//
// Layout in slot.Data starting at GaItemsStart (0x20):
//
//	[0x20:0x35]  GaItemFull (21 bytes): Handle | ItemID | Unk2 | Unk3 | AoWGaItemHandle | Unk5
//
// slot.InventoryEnd is set to GaItemsStart + GaRecordWeapon so PatchWeaponItemID
// stops scanning at the right boundary. GaItemDataOffset is left zero, so
// upsertGaItemData is a no-op.
func weaponInfusionFixture() *App {
	const (
		wepHandle    = uint32(0x00400001)
		wepItemID    = uint32(0x000F4240) // Standard Dagger +0
		aowHandle    = uint32(0xFFFFFFFF) // no AoW
		bufSize      = 512
		gaStart      = core.GaItemsStart    // 0x20 = 32
		gaWepSize    = core.GaRecordWeapon  // 21
	)

	app := NewApp()
	app.save = &core.SaveFile{}
	slot := &app.save.Slots[0]

	// Populate in-memory GaItems / GaMap.
	slot.GaItems = []core.GaItemFull{
		{
			Handle:          wepHandle,
			ItemID:          wepItemID,
			Unk2:            -1,
			Unk3:            -1,
			AoWGaItemHandle: aowHandle,
			Unk5:            0,
		},
	}
	slot.GaMap = map[uint32]uint32{wepHandle: wepItemID}

	// Allocate slot.Data and serialize the weapon at GaItemsStart.
	slot.Data = make([]byte, bufSize)
	off := gaStart
	binary.LittleEndian.PutUint32(slot.Data[off:], wepHandle)
	binary.LittleEndian.PutUint32(slot.Data[off+4:], wepItemID)
	binary.LittleEndian.PutUint32(slot.Data[off+8:], 0xFFFFFFFF)  // Unk2
	binary.LittleEndian.PutUint32(slot.Data[off+12:], 0xFFFFFFFF) // Unk3
	binary.LittleEndian.PutUint32(slot.Data[off+16:], aowHandle)
	slot.Data[off+20] = 0 // Unk5

	// PatchWeaponItemID stops scanning when curr >= slot.InventoryEnd.
	slot.InventoryEnd = gaStart + gaWepSize

	// GaItemDataOffset == 0 → upsertGaItemData returns nil (no-op).
	slot.GaItemDataOffset = 0

	return app
}

// ─── Guard tests ───────────────────────────────────────────────────────────────

func TestApplyWeaponInfusion_NoSave(t *testing.T) {
	app := NewApp()
	err := app.ApplyWeaponInfusion(0, 0x00400001, 0x000F4240, 0x000F42A4)
	if err == nil || !strings.Contains(err.Error(), "no save loaded") {
		t.Fatalf("want 'no save loaded', got %v", err)
	}
}

func TestApplyWeaponInfusion_InvalidCharIdx(t *testing.T) {
	app := weaponInfusionFixture()
	for _, idx := range []int{-1, 10, 99} {
		err := app.ApplyWeaponInfusion(idx, 0x00400001, 0x000F4240, 0x000F42A4)
		if err == nil || !strings.Contains(err.Error(), "invalid character index") {
			t.Errorf("idx %d: want 'invalid character index', got %v", idx, err)
		}
	}
}

func TestApplyWeaponInfusion_HandleNotFound(t *testing.T) {
	app := weaponInfusionFixture()
	// Use a handle that doesn't exist in GaItems.
	err := app.ApplyWeaponInfusion(0, 0xDEADBEEF, 0x000F4240, 0x000F42A4)
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("want 'not found', got %v", err)
	}
}

func TestApplyWeaponInfusion_ItemIDMismatch(t *testing.T) {
	app := weaponInfusionFixture()
	// expectedCurrentItemID doesn't match actual ItemID in GaItems.
	// GaItems has 0x000F4240 (Standard Dagger +0); we claim it's 0x000F4241 (+1).
	err := app.ApplyWeaponInfusion(0, 0x00400001, 0x000F4241, 0x000F42A5)
	if err == nil || !strings.Contains(err.Error(), "stale") {
		t.Fatalf("want 'stale' error, got %v", err)
	}
}

// ─── Happy path ────────────────────────────────────────────────────────────────

func TestApplyWeaponInfusion_StandardToHeavy(t *testing.T) {
	// Standard Dagger +0 (0x000F4240) → Heavy Dagger +0 (0x000F42A4).
	// Infusion offset: 0 (Standard) → 100 (Heavy). Upgrade level: 0 unchanged.
	const (
		wepHandle    = uint32(0x00400001)
		standardID   = uint32(0x000F4240) // Standard Dagger +0
		heavyID      = uint32(0x000F42A4) // Heavy   Dagger +0  (base + 100)
		aowHandle    = uint32(0xFFFFFFFF)
		gaStart      = core.GaItemsStart
	)

	app := weaponInfusionFixture()
	slot := &app.save.Slots[0]
	gaCountBefore := len(slot.GaItems)

	err := app.ApplyWeaponInfusion(0, wepHandle, standardID, heavyID)
	if err != nil {
		t.Fatalf("ApplyWeaponInfusion: unexpected error: %v", err)
	}

	// ItemID updated in GaItems in-memory.
	if slot.GaItems[0].ItemID != heavyID {
		t.Errorf("GaItems[0].ItemID = 0x%08X, want 0x%08X", slot.GaItems[0].ItemID, heavyID)
	}

	// Handle unchanged.
	if slot.GaItems[0].Handle != wepHandle {
		t.Errorf("GaItems[0].Handle = 0x%08X, want 0x%08X", slot.GaItems[0].Handle, wepHandle)
	}

	// AoWGaItemHandle not touched.
	if slot.GaItems[0].AoWGaItemHandle != aowHandle {
		t.Errorf("AoWGaItemHandle = 0x%08X, want 0x%08X", slot.GaItems[0].AoWGaItemHandle, aowHandle)
	}

	// GaMap updated.
	if slot.GaMap[wepHandle] != heavyID {
		t.Errorf("GaMap[handle] = 0x%08X, want 0x%08X", slot.GaMap[wepHandle], heavyID)
	}

	// ItemID patched in slot.Data at GaItemsStart+4.
	dataID := binary.LittleEndian.Uint32(slot.Data[gaStart+4:])
	if dataID != heavyID {
		t.Errorf("slot.Data ItemID = 0x%08X, want 0x%08X", dataID, heavyID)
	}

	// No new GaItems allocated.
	if len(slot.GaItems) != gaCountBefore {
		t.Errorf("GaItems count changed: %d → %d", gaCountBefore, len(slot.GaItems))
	}
}
