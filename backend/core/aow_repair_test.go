package core

import (
	"encoding/binary"
	"os"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/db"
)

// Real DB IDs probed from the item DB:
//
//	Dagger weapon      0x000F4240 (WepType 1, GemMountType 2)
//	Sword Dance AoW    0x80003070 (compatible with WepType 1)
//	Shield Bash AoW    0x80007530 (incompatible with WepType 1 — shield-only)
const (
	testDaggerItemID     = uint32(0x000F4240)
	testCompatAoWItemID  = uint32(0x80003070)
	testIncompatAoWItmID = uint32(0x80007530)
)

// ---- incompatible AoW is rejected before any mutation ----------------------

func TestCreateWeaponAoWCopy_IncompatibleRejectedNoMutation(t *testing.T) {
	slot := makeStrictTestSlot(10)
	const wepHandle = uint32(0x80800001)
	addTestWeapon(slot, 0, wepHandle, testDaggerItemID, LegacyNoCustomAoWHandle)
	flushTestGaItems(slot)

	before := slot.GaItems[0].AoWGaItemHandle
	usedBefore := CountSlotUsage(slot).GaItemsUsed

	err := CreateWeaponAoWCopy(slot, wepHandle, testIncompatAoWItmID)
	if err == nil {
		t.Fatal("expected incompatibility error, got nil")
	}
	if slot.GaItems[0].AoWGaItemHandle != before {
		t.Fatalf("weapon AoW handle mutated on rejection: 0x%08X -> 0x%08X", before, slot.GaItems[0].AoWGaItemHandle)
	}
	if got := CountSlotUsage(slot).GaItemsUsed; got != usedBefore {
		t.Fatalf("GaItems used changed on rejection: %d -> %d", usedBefore, got)
	}
}

func TestAttachExistingWeaponAoW_IncompatibleRejectedNoMutation(t *testing.T) {
	slot := makeStrictTestSlot(10)
	const wepHandle = uint32(0x80800001)
	const aowHandle = uint32(0xC0800001)
	addTestWeapon(slot, 0, wepHandle, testDaggerItemID, LegacyNoCustomAoWHandle)
	addTestAoW(slot, 1, aowHandle, testIncompatAoWItmID) // Shield Bash — incompatible with dagger
	flushTestGaItems(slot)

	before := slot.GaItems[0].AoWGaItemHandle
	if err := AttachExistingWeaponAoW(slot, wepHandle, aowHandle); err == nil {
		t.Fatal("expected incompatibility error, got nil")
	}
	if slot.GaItems[0].AoWGaItemHandle != before {
		t.Fatalf("weapon AoW handle mutated on rejection: 0x%08X -> 0x%08X", before, slot.GaItems[0].AoWGaItemHandle)
	}
}

// ---- capacity failure reports without partial repair -----------------------

func TestCreateWeaponAoWCopy_GaItemsFullNoPartialRepair(t *testing.T) {
	// All 3 GaItems non-empty → zero free GaItems capacity.
	slot := makeStrictTestSlot(3)
	const wepHandle = uint32(0x80800001)
	addTestWeapon(slot, 0, wepHandle, testDaggerItemID, LegacyNoCustomAoWHandle)
	slot.GaItems[1] = GaItemFull{Handle: 0x80800002, ItemID: 0x00000001, Unk2: -1, Unk3: -1, AoWGaItemHandle: 0xFFFFFFFF}
	slot.GaItems[2] = GaItemFull{Handle: 0x80800003, ItemID: 0x00000002, Unk2: -1, Unk3: -1, AoWGaItemHandle: 0xFFFFFFFF}
	flushTestGaItems(slot)

	usedBefore := CountSlotUsage(slot).GaItemsUsed
	before := slot.GaItems[0].AoWGaItemHandle

	err := CreateWeaponAoWCopy(slot, wepHandle, testCompatAoWItemID) // compatible → passes compat, fails capacity
	if err == nil {
		t.Fatal("expected gaitem_full capacity error, got nil")
	}
	if got := CountSlotUsage(slot).GaItemsUsed; got != usedBefore {
		t.Fatalf("GaItems used changed on capacity failure: %d -> %d", usedBefore, got)
	}
	if slot.GaItems[0].AoWGaItemHandle != before {
		t.Fatalf("weapon AoW mutated on capacity failure: 0x%08X -> 0x%08X", before, slot.GaItems[0].AoWGaItemHandle)
	}
}

func TestCreateWeaponAoWCopy_GaItemDataFullNoPartialRepair(t *testing.T) {
	slot := makeStrictTestSlot(10) // plenty of free GaItems
	const wepHandle = uint32(0x80800001)
	addTestWeapon(slot, 0, wepHandle, testDaggerItemID, LegacyNoCustomAoWHandle)
	flushTestGaItems(slot)

	// Point GaItemData at a valid header and mark it full.
	slot.GaItemDataOffset = len(slot.Data) / 2
	binary.LittleEndian.PutUint32(slot.Data[slot.GaItemDataOffset:], uint32(GaItemDataMaxCount))

	before := slot.GaItems[0].AoWGaItemHandle
	err := CreateWeaponAoWCopy(slot, wepHandle, testCompatAoWItemID)
	if err == nil {
		t.Fatal("expected gaitemdata_full capacity error, got nil")
	}
	if slot.GaItems[0].AoWGaItemHandle != before {
		t.Fatalf("weapon AoW mutated on GaItemData capacity failure: 0x%08X -> 0x%08X", before, slot.GaItems[0].AoWGaItemHandle)
	}
}

// ---- current_aow_non_aow_category: clear works -----------------------------

func TestClearWeaponAoW_ClearsAttachedHandle(t *testing.T) {
	slot := makeStrictTestSlot(10)
	const wepHandle = uint32(0x80800001)
	const bogusAoW = uint32(0xC0800042) // stand-in for a non-AoW-category / stale attachment
	addTestWeapon(slot, 0, wepHandle, testDaggerItemID, bogusAoW)
	flushTestGaItems(slot)

	if err := ClearWeaponAoW(slot, wepHandle); err != nil {
		t.Fatalf("ClearWeaponAoW: %v", err)
	}
	if slot.GaItems[0].AoWGaItemHandle != NoCustomAoWHandle {
		t.Fatalf("in-memory AoW not cleared: 0x%08X", slot.GaItems[0].AoWGaItemHandle)
	}
	// Verify binary too — PatchWeaponAoWHandle writes 4 bytes at weaponOff+16.
	got := binary.LittleEndian.Uint32(slot.Data[GaItemsStart+16:])
	if got != NoCustomAoWHandle {
		t.Fatalf("binary AoW not cleared: 0x%08X", got)
	}
}

// ---- attach existing compatible free copy succeeds (happy path) ------------

func TestAttachExistingWeaponAoW_CompatibleAttaches(t *testing.T) {
	slot := makeStrictTestSlot(10)
	const wepHandle = uint32(0x80800001)
	const aowHandle = uint32(0xC0800001)
	addTestWeapon(slot, 0, wepHandle, testDaggerItemID, LegacyNoCustomAoWHandle)
	addTestAoW(slot, 1, aowHandle, testCompatAoWItemID) // Sword Dance — compatible, free
	flushTestGaItems(slot)

	if err := AttachExistingWeaponAoW(slot, wepHandle, aowHandle); err != nil {
		t.Fatalf("AttachExistingWeaponAoW: %v", err)
	}
	if slot.GaItems[0].AoWGaItemHandle != aowHandle {
		t.Fatalf("AoW not attached: 0x%08X", slot.GaItems[0].AoWGaItemHandle)
	}
}

// ---- shared AoW: after create-copy repair the two weapons no longer share --

func TestCreateWeaponAoWCopy_SharedAoWNoLongerShared(t *testing.T) {
	savePath := "../../tmp/save/ER0000.sl2"
	if _, err := os.Stat(savePath); os.IsNotExist(err) {
		t.Skipf("test save not found: %s", savePath)
	}
	save, err := LoadSave(savePath)
	if err != nil {
		t.Fatalf("LoadSave: %v", err)
	}

	// Find an active slot with a weapon that has a real custom AoW, and a
	// second weapon compatible with that AoW's itemID.
	var slot *SaveSlot
	var w1Handle, w1AoWHandle, w2Handle, aowItemID uint32
	for i := 0; i < 10 && slot == nil; i++ {
		s := &save.Slots[i]
		if s.Version == 0 {
			continue
		}
		// weapon1: first weapon carrying a real AoW handle present in GaMap.
		for _, g := range s.GaItems {
			if g.IsEmpty() || g.Handle&GaHandleTypeMask != ItemTypeWeapon {
				continue
			}
			if IsNoCustomAoWHandle(g.AoWGaItemHandle) {
				continue
			}
			id, ok := s.GaMap[g.AoWGaItemHandle]
			if !ok || id == 0 {
				continue
			}
			// weapon2: any other weapon compatible with this AoW itemID.
			for _, g2 := range s.GaItems {
				if g2.IsEmpty() || g2.Handle&GaHandleTypeMask != ItemTypeWeapon || g2.Handle == g.Handle {
					continue
				}
				compat, known := db.IsAshOfWarCompatibleWithWeapon(id, g2.ItemID)
				if known && compat {
					slot, w1Handle, w1AoWHandle, w2Handle, aowItemID = s, g.Handle, g.AoWGaItemHandle, g2.Handle, id
					break
				}
			}
			if slot != nil {
				break
			}
		}
	}
	if slot == nil {
		t.Skip("no suitable weapon pair with a shared-compatible AoW found in save")
	}

	// Force the shared state: point weapon2 at weapon1's AoW handle in the
	// GaItems slice (RebuildSlotFull serializes from the slice).
	for i := range slot.GaItems {
		if slot.GaItems[i].Handle == w2Handle {
			slot.GaItems[i].AoWGaItemHandle = w1AoWHandle
			break
		}
	}

	if err := CreateWeaponAoWCopy(slot, w2Handle, aowItemID); err != nil {
		t.Fatalf("CreateWeaponAoWCopy: %v", err)
	}

	var gotW1, gotW2 uint32
	for _, g := range slot.GaItems {
		if g.Handle == w1Handle {
			gotW1 = g.AoWGaItemHandle
		}
		if g.Handle == w2Handle {
			gotW2 = g.AoWGaItemHandle
		}
	}
	if gotW1 != w1AoWHandle {
		t.Fatalf("weapon1 AoW changed by repair: 0x%08X -> 0x%08X", w1AoWHandle, gotW1)
	}
	if gotW2 == w1AoWHandle {
		t.Fatalf("weapon2 still shares weapon1 AoW handle 0x%08X after repair", w1AoWHandle)
	}
	if gotW2 == gotW1 {
		t.Fatalf("weapons still share AoW handle 0x%08X after repair", gotW2)
	}
}
