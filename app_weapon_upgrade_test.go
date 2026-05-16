package main

import (
	"encoding/binary"
	"strings"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
)

// weaponUpgradeFixture returns an App with a single fully wired weapon GaItem
// in slot 0 with the given initial ItemID. Mirrors weaponInfusionFixture but
// parametrised so tests can start from any infusion offset / level.
//
// The weapon is laid out at GaItemsStart (0x20). slot.InventoryEnd is set so
// PatchWeaponItemID stops scanning at the right boundary. GaItemDataOffset is
// left zero, so upsertGaItemData is a no-op.
func weaponUpgradeFixture(initialItemID uint32) *App {
	const (
		wepHandle = uint32(0x00400001)
		aowHandle = uint32(0xFFFFFFFF)
		bufSize   = 512
		gaStart   = core.GaItemsStart
		gaWepSize = core.GaRecordWeapon
	)

	app := NewApp()
	app.save = &core.SaveFile{}
	slot := &app.save.Slots[0]

	slot.GaItems = []core.GaItemFull{
		{
			Handle:          wepHandle,
			ItemID:          initialItemID,
			Unk2:            -1,
			Unk3:            -1,
			AoWGaItemHandle: aowHandle,
			Unk5:            0,
		},
	}
	slot.GaMap = map[uint32]uint32{wepHandle: initialItemID}

	slot.Data = make([]byte, bufSize)
	off := gaStart
	binary.LittleEndian.PutUint32(slot.Data[off:], wepHandle)
	binary.LittleEndian.PutUint32(slot.Data[off+4:], initialItemID)
	binary.LittleEndian.PutUint32(slot.Data[off+8:], 0xFFFFFFFF)
	binary.LittleEndian.PutUint32(slot.Data[off+12:], 0xFFFFFFFF)
	binary.LittleEndian.PutUint32(slot.Data[off+16:], aowHandle)
	slot.Data[off+20] = 0

	slot.InventoryEnd = gaStart + gaWepSize
	slot.GaItemDataOffset = 0
	return app
}

// ─── Guard tests ───────────────────────────────────────────────────────────────

func TestApplyWeaponUpgradeLevel_NoSave(t *testing.T) {
	app := NewApp()
	err := app.ApplyWeaponUpgradeLevel(0, 0x00400001, 0x000F4240, 0x000F4245)
	if err == nil || !strings.Contains(err.Error(), "no save loaded") {
		t.Fatalf("want 'no save loaded', got %v", err)
	}
}

func TestApplyWeaponUpgradeLevel_InvalidCharIdx(t *testing.T) {
	app := weaponUpgradeFixture(0x000F4240)
	for _, idx := range []int{-1, 10, 99} {
		err := app.ApplyWeaponUpgradeLevel(idx, 0x00400001, 0x000F4240, 0x000F4245)
		if err == nil || !strings.Contains(err.Error(), "invalid character index") {
			t.Errorf("idx %d: want 'invalid character index', got %v", idx, err)
		}
	}
}

func TestApplyWeaponUpgradeLevel_HandleNotFound(t *testing.T) {
	app := weaponUpgradeFixture(0x000F4240)
	err := app.ApplyWeaponUpgradeLevel(0, 0xDEADBEEF, 0x000F4240, 0x000F4245)
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("want 'not found', got %v", err)
	}
}

func TestApplyWeaponUpgradeLevel_ItemIDMismatch(t *testing.T) {
	// Slot has +0, frontend claims +1 was loaded.
	app := weaponUpgradeFixture(0x000F4240)
	err := app.ApplyWeaponUpgradeLevel(0, 0x00400001, 0x000F4241, 0x000F4245)
	if err == nil || !strings.Contains(err.Error(), "stale") {
		t.Fatalf("want stale-data error, got %v", err)
	}
}

func TestApplyWeaponUpgradeLevel_NonWeaponID(t *testing.T) {
	app := weaponUpgradeFixture(0x000F4240)
	// Talisman ID (upper nibble 0x2) is not a weapon.
	err := app.ApplyWeaponUpgradeLevel(0, 0x00400001, 0x2000082A, 0x000F4245)
	if err == nil || !strings.Contains(err.Error(), "not a weapon") {
		t.Fatalf("want 'not a weapon' error, got %v", err)
	}
	err = app.ApplyWeaponUpgradeLevel(0, 0x00400001, 0x000F4240, 0x2000082A)
	if err == nil || !strings.Contains(err.Error(), "not a weapon") {
		t.Fatalf("want 'not a weapon' error, got %v", err)
	}
}

// ─── Happy path: smithing-stone weapon (MaxUpgrade=25) ─────────────────────────

func TestApplyWeaponUpgradeLevel_StandardDagger_Plus0ToPlus5(t *testing.T) {
	const (
		wepHandle = uint32(0x00400001)
		fromID    = uint32(0x000F4240) // Dagger +0  (Standard)
		toID      = uint32(0x000F4245) // Dagger +5  (Standard)
		gaStart   = core.GaItemsStart
	)
	app := weaponUpgradeFixture(fromID)
	slot := &app.save.Slots[0]
	gaCountBefore := len(slot.GaItems)

	if err := app.ApplyWeaponUpgradeLevel(0, wepHandle, fromID, toID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if slot.GaItems[0].ItemID != toID {
		t.Errorf("GaItems[0].ItemID = 0x%08X, want 0x%08X", slot.GaItems[0].ItemID, toID)
	}
	if slot.GaItems[0].Handle != wepHandle {
		t.Errorf("Handle changed: 0x%08X", slot.GaItems[0].Handle)
	}
	if slot.GaItems[0].AoWGaItemHandle != 0xFFFFFFFF {
		t.Errorf("AoWGaItemHandle changed: 0x%08X", slot.GaItems[0].AoWGaItemHandle)
	}
	if slot.GaMap[wepHandle] != toID {
		t.Errorf("GaMap[handle] = 0x%08X, want 0x%08X", slot.GaMap[wepHandle], toID)
	}
	dataID := binary.LittleEndian.Uint32(slot.Data[gaStart+4:])
	if dataID != toID {
		t.Errorf("slot.Data ItemID = 0x%08X, want 0x%08X", dataID, toID)
	}
	if len(slot.GaItems) != gaCountBefore {
		t.Errorf("GaItems count changed: %d → %d", gaCountBefore, len(slot.GaItems))
	}
}

// Preserves the infusion offset when upgrading.
func TestApplyWeaponUpgradeLevel_PreservesInfusion(t *testing.T) {
	const (
		wepHandle = uint32(0x00400001)
		fromID    = uint32(0x000F42A4) // Heavy Dagger +0 (infuseOffset=100)
		toID      = uint32(0x000F42A9) // Heavy Dagger +5
	)
	app := weaponUpgradeFixture(fromID)
	if err := app.ApplyWeaponUpgradeLevel(0, wepHandle, fromID, toID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if app.save.Slots[0].GaItems[0].ItemID != toID {
		t.Errorf("ItemID = 0x%08X, want 0x%08X", app.save.Slots[0].GaItems[0].ItemID, toID)
	}
}

// Setting level exactly to MaxUpgrade (25 for smithing-stone) is allowed.
func TestApplyWeaponUpgradeLevel_AtMaxAllowed(t *testing.T) {
	const (
		wepHandle = uint32(0x00400001)
		fromID    = uint32(0x000F4240) // Dagger +0
		toID      = uint32(0x000F4259) // Dagger +25 (0x000F4240 + 0x19)
	)
	app := weaponUpgradeFixture(fromID)
	if err := app.ApplyWeaponUpgradeLevel(0, wepHandle, fromID, toID); err != nil {
		t.Fatalf("unexpected error at max: %v", err)
	}
}

// ─── Happy path: somber weapon (MaxUpgrade=10) ─────────────────────────────────

func TestApplyWeaponUpgradeLevel_SomberWeapon_AtMax(t *testing.T) {
	const (
		wepHandle = uint32(0x00400001)
		fromID    = uint32(0x000F6950) // Black Knife +0  (MaxUpgrade=10, somber)
		toID      = uint32(0x000F695A) // Black Knife +10
	)
	app := weaponUpgradeFixture(fromID)
	if err := app.ApplyWeaponUpgradeLevel(0, wepHandle, fromID, toID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if app.save.Slots[0].GaItems[0].ItemID != toID {
		t.Errorf("ItemID = 0x%08X, want 0x%08X", app.save.Slots[0].GaItems[0].ItemID, toID)
	}
}

func TestApplyWeaponUpgradeLevel_SomberWeapon_AboveMaxRejected(t *testing.T) {
	const (
		wepHandle = uint32(0x00400001)
		fromID    = uint32(0x000F6950) // Black Knife +0  (MaxUpgrade=10)
		toID      = uint32(0x000F695B) // +11 — out of range
	)
	app := weaponUpgradeFixture(fromID)
	err := app.ApplyWeaponUpgradeLevel(0, wepHandle, fromID, toID)
	if err == nil || !strings.Contains(err.Error(), "out of range") {
		t.Fatalf("want 'out of range', got %v", err)
	}
}

// ─── Rejection tests ───────────────────────────────────────────────────────────

func TestApplyWeaponUpgradeLevel_AboveMaxRejected(t *testing.T) {
	const (
		wepHandle = uint32(0x00400001)
		fromID    = uint32(0x000F4240) // Dagger +0 (MaxUpgrade=25)
		toID      = uint32(0x000F425A) // +26
	)
	app := weaponUpgradeFixture(fromID)
	err := app.ApplyWeaponUpgradeLevel(0, wepHandle, fromID, toID)
	if err == nil || !strings.Contains(err.Error(), "out of range") {
		t.Fatalf("want 'out of range', got %v", err)
	}
}

func TestApplyWeaponUpgradeLevel_InfusionChangeRejected(t *testing.T) {
	const (
		wepHandle = uint32(0x00400001)
		fromID    = uint32(0x000F4240) // Standard Dagger +0
		toID      = uint32(0x000F42A5) // Heavy Dagger +1 (infusion AND level differ)
	)
	app := weaponUpgradeFixture(fromID)
	err := app.ApplyWeaponUpgradeLevel(0, wepHandle, fromID, toID)
	if err == nil || !strings.Contains(err.Error(), "infusion") {
		t.Fatalf("want infusion-change error, got %v", err)
	}
}

func TestApplyWeaponUpgradeLevel_InfusionOnlyChangeRejected(t *testing.T) {
	const (
		wepHandle = uint32(0x00400001)
		fromID    = uint32(0x000F4240) // Standard Dagger +0
		toID      = uint32(0x000F42A4) // Heavy Dagger +0 (infusion only)
	)
	app := weaponUpgradeFixture(fromID)
	err := app.ApplyWeaponUpgradeLevel(0, wepHandle, fromID, toID)
	if err == nil || !strings.Contains(err.Error(), "infusion") {
		t.Fatalf("want infusion-change error, got %v", err)
	}
}

func TestApplyWeaponUpgradeLevel_BaseIDChangeRejected(t *testing.T) {
	const (
		wepHandle = uint32(0x00400001)
		fromID    = uint32(0x000F4240) // Dagger +0
		toID      = uint32(0x000F9060) // Parrying Dagger +0 (different baseID)
	)
	app := weaponUpgradeFixture(fromID)
	err := app.ApplyWeaponUpgradeLevel(0, wepHandle, fromID, toID)
	if err == nil || !strings.Contains(err.Error(), "base ID") {
		t.Fatalf("want base-ID change error, got %v", err)
	}
}

func TestApplyWeaponUpgradeLevel_UnarmedRejected(t *testing.T) {
	// Unarmed (0x0001ADB0) has MaxUpgrade=0 — cannot be upgraded.
	const (
		wepHandle = uint32(0x00400001)
		fromID    = uint32(0x0001ADB0)
		toID      = uint32(0x0001ADB1)
	)
	app := weaponUpgradeFixture(fromID)
	err := app.ApplyWeaponUpgradeLevel(0, wepHandle, fromID, toID)
	if err == nil || !strings.Contains(err.Error(), "cannot be upgraded") {
		t.Fatalf("want 'cannot be upgraded', got %v", err)
	}
}

// ─── No-op: same ItemID is accepted (delegates to PatchWeaponItemID's early return). ──

func TestApplyWeaponUpgradeLevel_SameLevelNoOp(t *testing.T) {
	const (
		wepHandle = uint32(0x00400001)
		id        = uint32(0x000F4245) // Dagger +5
	)
	app := weaponUpgradeFixture(id)
	if err := app.ApplyWeaponUpgradeLevel(0, wepHandle, id, id); err != nil {
		t.Fatalf("same-level call should be no-op, got %v", err)
	}
	if app.save.Slots[0].GaItems[0].ItemID != id {
		t.Errorf("ItemID changed unexpectedly: 0x%08X", app.save.Slots[0].GaItems[0].ItemID)
	}
}
