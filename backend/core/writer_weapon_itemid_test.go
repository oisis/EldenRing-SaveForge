package core

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// These tests characterize the low-level contract of PatchWeaponItemID,
// independent of the App.ApplyWeapon{UpgradeLevel,Infusion} endpoints that
// currently call it. The writer is also reached by the active workspace save
// path (backend/editor/save.go via ApplyWorkspaceSave), so its contract must
// stay covered even after the legacy public endpoints (and their app-level
// tests) are removed.
//
// Scope is deliberately limited to what PatchWeaponItemID itself implements:
// locate-by-handle, the expected-current stale-data guard, and the in-place
// 4-byte ItemID overwrite. Business rules that live in the App layer (upgrade
// clamp, infusion-offset derivation, base-ID validation, somber/standard
// rules) are NOT part of this primitive's contract and are not asserted here.

const (
	itemidWepHandle  = uint32(0x80800001)
	itemidOtherWep   = uint32(0x80800002)
	itemidCurrentID  = uint32(0x00001000)
	itemidNewID      = uint32(0x00001005) // same base weapon, different encoded suffix — input to the writer
	itemidStaleID    = uint32(0x00002000) // a different weapon ID the slot does NOT store
	itemidWriterOff  = GaItemsStart + 4   // weapon record at index 0 → ItemID field at [GaItemsStart+4]
	itemidMissingHnd = uint32(0x80800099)
)

// TC-01 — success: patches only the matching weapon's ItemID, exactly 4 bytes.
func TestPatchWeaponItemID_Success_PatchesOnlyMatchingItemID(t *testing.T) {
	slot := makeStrictTestSlot(10)
	addTestWeapon(slot, 0, itemidWepHandle, itemidCurrentID, NoCustomAoWHandle)
	addTestWeapon(slot, 1, itemidOtherWep, itemidCurrentID, NoCustomAoWHandle)
	flushTestGaItems(slot)

	before := append([]byte(nil), slot.Data...)

	if err := PatchWeaponItemID(slot, itemidWepHandle, itemidCurrentID, itemidNewID); err != nil {
		t.Fatalf("PatchWeaponItemID: unexpected error: %v", err)
	}

	// In-memory derived state updated for the matching record.
	if slot.GaItems[0].ItemID != itemidNewID {
		t.Errorf("GaItems[0].ItemID: expected 0x%08X, got 0x%08X", itemidNewID, slot.GaItems[0].ItemID)
	}
	if slot.GaMap[itemidWepHandle] != itemidNewID {
		t.Errorf("GaMap[handle]: expected 0x%08X, got 0x%08X", itemidNewID, slot.GaMap[itemidWepHandle])
	}
	// Handle itself is untouched.
	if slot.GaItems[0].Handle != itemidWepHandle {
		t.Errorf("GaItems[0].Handle changed: expected 0x%08X, got 0x%08X", itemidWepHandle, slot.GaItems[0].Handle)
	}
	// The unrelated second weapon is untouched.
	if slot.GaItems[1].ItemID != itemidCurrentID || slot.GaMap[itemidOtherWep] != itemidCurrentID {
		t.Errorf("second weapon mutated: ItemID=0x%08X, GaMap=0x%08X", slot.GaItems[1].ItemID, slot.GaMap[itemidOtherWep])
	}

	// Raw bytes: exactly the 4 ItemID bytes of the target record changed.
	for i := range slot.Data {
		if i >= itemidWriterOff && i < itemidWriterOff+4 {
			continue
		}
		if slot.Data[i] != before[i] {
			t.Fatalf("byte at offset %d changed unexpectedly (0x%02X → 0x%02X)", i, before[i], slot.Data[i])
		}
	}
	if got := binary.LittleEndian.Uint32(slot.Data[itemidWriterOff:]); got != itemidNewID {
		t.Errorf("slot.Data ItemID field: expected 0x%08X, got 0x%08X", itemidNewID, got)
	}
}

// TC-02 — stale-data guard: a mismatched expectedCurrentItemID errors and never mutates.
func TestPatchWeaponItemID_StaleExpectedID_ErrorsWithoutMutation(t *testing.T) {
	slot := makeStrictTestSlot(10)
	addTestWeapon(slot, 0, itemidWepHandle, itemidCurrentID, NoCustomAoWHandle)
	flushTestGaItems(slot)

	before := append([]byte(nil), slot.Data...)

	// expected != new (so the writer does not early-return on equality) and
	// expected != stored (so it reaches and trips the stale-data guard).
	err := PatchWeaponItemID(slot, itemidWepHandle, itemidStaleID, itemidNewID)
	if err == nil {
		t.Fatal("expected stale-data error, got nil")
	}

	if slot.GaItems[0].ItemID != itemidCurrentID {
		t.Errorf("GaItems[0].ItemID mutated on stale guard: got 0x%08X", slot.GaItems[0].ItemID)
	}
	if slot.GaMap[itemidWepHandle] != itemidCurrentID {
		t.Errorf("GaMap mutated on stale guard: got 0x%08X", slot.GaMap[itemidWepHandle])
	}
	if !bytes.Equal(slot.Data, before) {
		t.Error("slot.Data mutated despite stale-data error")
	}
}

// TC-03 — unknown handle: errors and never mutates.
func TestPatchWeaponItemID_UnknownHandle_ErrorsWithoutMutation(t *testing.T) {
	slot := makeStrictTestSlot(10)
	addTestWeapon(slot, 0, itemidWepHandle, itemidCurrentID, NoCustomAoWHandle)
	flushTestGaItems(slot)

	before := append([]byte(nil), slot.Data...)

	err := PatchWeaponItemID(slot, itemidMissingHnd, itemidCurrentID, itemidNewID)
	if err == nil {
		t.Fatal("expected unknown-handle error, got nil")
	}

	if slot.GaItems[0].ItemID != itemidCurrentID {
		t.Errorf("GaItems[0].ItemID mutated on unknown handle: got 0x%08X", slot.GaItems[0].ItemID)
	}
	if !bytes.Equal(slot.Data, before) {
		t.Error("slot.Data mutated despite unknown-handle error")
	}
}
