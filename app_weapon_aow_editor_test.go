package main

import (
	"encoding/binary"
	"strings"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
)

// Editor Mode semantics (PatchWeaponAoW, called by ApplyWeaponAoW):
//
//   newAoWItemID == 0 → removes AoW in-place (no rebuild). Old AoW GaItem stays
//     orphaned in the slot — the game tolerates unused entries. GaItems count unchanged.
//
//   newAoWItemID != 0 → ALWAYS allocates a fresh AoW GaItem via generateUniqueHandle +
//     allocateGaItem. Never reuses an existing handle — sharing causes EXCEPTION_ACCESS_VIOLATION.
//     Calls RebuildSlotFull + parseFromData, then re-locates the weapon and patches its
//     AoWGaItemHandle field. GaItems count grows by 1 in a real save; with a Version=0
//     fixture the rebuild is verbatim and parseFromData re-scans slot.Data, so the
//     in-memory GaItem count reflects slot.Data content, not the pre-rebuild allocation.

// ─── Remove fixture (512B — no rebuild needed) ──────────────────────────────────

// weaponAoWEditorRemoveFixture returns an App with a Dagger (0x000F4240) that already
// has Sword Dance (0x80003070) attached, ready for a remove-AoW test.
//
// Layout at GaItemsStart (0x20 = 32):
//
//	[32:40]  AoW GaItem (8B) — handle 0xC0800001, itemID 0x80003070
//	[40:61]  Weapon (21B)    — handle 0x80800001, AoWGaItemHandle 0xC0800001
func weaponAoWEditorRemoveFixture() *App {
	const (
		wepHandle = uint32(0x80800001)
		wepItemID = uint32(0x000F4240)  // Standard Dagger +0
		aowHandle = uint32(0xC0800001)  // Sword Dance handle
		aowItemID = uint32(0x80003070)  // Sword Dance
		gaStart   = core.GaItemsStart   // 32
		gaAoWSize = core.GaRecordAoW    // 8
		gaWepSize = core.GaRecordWeapon // 21
		bufSize   = 512
	)

	app := NewApp()
	app.save = &core.SaveFile{}
	slot := &app.save.Slots[0]

	slot.GaItems = []core.GaItemFull{
		{Handle: aowHandle, ItemID: aowItemID, Unk2: -1, Unk3: -1, AoWGaItemHandle: 0xFFFFFFFF},
		{Handle: wepHandle, ItemID: wepItemID, Unk2: -1, Unk3: -1, AoWGaItemHandle: aowHandle},
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
	binary.LittleEndian.PutUint32(slot.Data[off+16:], aowHandle)  // AoWGaItemHandle — attached
	slot.Data[off+20] = 0                                         // Unk5

	slot.InventoryEnd = gaStart + gaAoWSize + gaWepSize // 61
	slot.GaItemDataOffset = 0
	return app
}

// ─── Set fixture (SlotSize — RebuildSlotFull required) ─────────────────────────

// buildEditorSetSlot assembles an App whose slot[0].Data is exactly core.SlotSize bytes
// (mandatory for RebuildSlotFull). Three fields are required for RebuildSlotFull to
// take the real rebuild path (not verbatim copy):
//
//  1. slot.Version != 0  (checked before the verbatim guard)
//  2. slot.UnlockedRegionsOffset != 0  (same guard)
//  3. slot.SectionMap contains a SectionUnlockedRegs entry so RebuildSlotFull
//     knows origRegsEnd (= URO + 4 with zero UnlockedRegions).
//
// With Version=100, GaItemsStart URO and a zero-filled slot.Data the subsequent
// parseFromData() call in PatchWeaponAoW succeeds: MagicPattern is not found so
// MagicOffset falls back to FallbackMagicBase (87504), dynamic offsets are computed
// from zero data (projCount=0, regCount=0), validateOffsetChain passes (it checks
// ranges and ordering — stat sanity is only in ValidateSlotIntegrity which is NOT
// called by PatchWeaponAoW), mapInventory reads zeros without error.
//
// gaData must contain the serialized GaItems starting at offset 0.
func buildEditorSetSlot(
	gaData []byte,
	gaItems []core.GaItemFull,
	gaMap map[uint32]uint32,
	inventoryEnd int,
	nextAoW, nextArmament int,
	partHandle uint8,
	nextHandle uint32,
) *App {
	app := NewApp()
	app.save = &core.SaveFile{}
	slot := &app.save.Slots[0]

	slot.Data = make([]byte, core.SlotSize)
	binary.LittleEndian.PutUint32(slot.Data[0:4], 100) // version → real rebuild path
	copy(slot.Data[core.GaItemsStart:], gaData)

	slot.Version = 100 // must match slot.Data[0:4]; also selects GaItemCountNew (5120)
	slot.GaItems = gaItems
	slot.GaMap = gaMap
	slot.InventoryEnd = inventoryEnd
	slot.NextAoWIndex = nextAoW
	slot.NextArmamentIndex = nextArmament
	slot.PartGaItemHandle = partHandle
	slot.NextGaItemHandle = nextHandle
	slot.GaItemDataOffset = 0

	// UnlockedRegionsOffset must be non-zero for real rebuild path.
	// GaItemsStart (32) is the minimum valid value; preRegsBlob = slot.Data[32:32] = empty.
	slot.UnlockedRegionsOffset = core.GaItemsStart
	slot.UnlockedRegions = []uint32{}

	// SectionMap must contain SectionUnlockedRegs so RebuildSlotFull finds origRegsEnd.
	// End = URO + 4 (zero UnlockedRegions → 4-byte count field only).
	slot.SectionMap = []core.SectionRange{
		{Name: core.SectionUnlockedRegs, Start: core.GaItemsStart, End: core.GaItemsStart + 4},
	}

	return app
}

// serializeGaItemsEditor serializes entries into a byte buffer and returns it.
func serializeGaItemsEditor(entries []core.GaItemFull) []byte {
	buf := make([]byte, len(entries)*core.GaRecordWeapon)
	pos := 0
	for i := range entries {
		pos += entries[i].Serialize(buf[pos:])
	}
	return buf[:pos]
}

// findGaItemByHandle returns the index of the entry with the given handle, or -1.
func findGaItemByHandle(items []core.GaItemFull, handle uint32) int {
	for i, g := range items {
		if g.Handle == handle {
			return i
		}
	}
	return -1
}

// gaItemByteOffset returns the byte offset of the GaItem with the given handle
// in slot.Data, computed by walking slot.GaItems from GaItemsStart. Returns -1 if not found.
// After RebuildSlotFull the byte layout shifts (new entries may be inserted), so callers
// must use this walker instead of hardcoded pre-rebuild offsets.
func gaItemByteOffset(slot *core.SaveSlot, handle uint32) int {
	curr := core.GaItemsStart
	for i := range slot.GaItems {
		if curr >= slot.InventoryEnd {
			break
		}
		g := &slot.GaItems[i]
		if !g.IsEmpty() && g.Handle == handle {
			return curr
		}
		curr += core.GaItemRecordSize(g.ItemID)
	}
	return -1
}

// ─── Guard tests ────────────────────────────────────────────────────────────────

func TestApplyWeaponAoW_NoSave(t *testing.T) {
	app := NewApp()
	err := app.ApplyWeaponAoW(0, 0x80800001, 0x80003070)
	if err == nil || !strings.Contains(err.Error(), "no save loaded") {
		t.Fatalf("want 'no save loaded', got %v", err)
	}
}

func TestApplyWeaponAoW_InvalidCharIdx(t *testing.T) {
	app := weaponAoWEditorRemoveFixture()
	for _, idx := range []int{-1, 10, 99} {
		err := app.ApplyWeaponAoW(idx, 0x80800001, 0)
		if err == nil || !strings.Contains(err.Error(), "invalid character index") {
			t.Errorf("idx %d: want 'invalid character index', got %v", idx, err)
		}
	}
}

func TestApplyWeaponAoW_HandleNotFound(t *testing.T) {
	app := weaponAoWEditorRemoveFixture()
	err := app.ApplyWeaponAoW(0, 0xDEADBEEF, 0)
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("want 'not found', got %v", err)
	}
}

// ─── Scenario A: Remove AoW ─────────────────────────────────────────────────────
//
// Editor Mode remove is an in-place patch: no rebuild, no GaItem deallocation.
// Old AoW GaItem stays orphaned; the game tolerates unused entries.

func TestApplyWeaponAoW_RemoveAoW(t *testing.T) {
	const (
		wepHandle = uint32(0x80800001)
		aowHandle = uint32(0xC0800001)
		aowItemID = uint32(0x80003070)
		gaStart   = core.GaItemsStart
		gaAoWSize = core.GaRecordAoW
	)

	app := weaponAoWEditorRemoveFixture()
	slot := &app.save.Slots[0]

	if err := app.ApplyWeaponAoW(0, wepHandle, 0); err != nil {
		t.Fatalf("ApplyWeaponAoW (remove): unexpected error: %v", err)
	}

	// Weapon (GaItems[1]) AoWGaItemHandle reset to canonical NoCustomAoWHandle (vanilla-aligned).
	wepIdx := findGaItemByHandle(slot.GaItems, wepHandle)
	if wepIdx == -1 {
		t.Fatal("weapon not found in GaItems after remove")
	}
	if slot.GaItems[wepIdx].AoWGaItemHandle != core.NoCustomAoWHandle {
		t.Errorf("GaItems[%d].AoWGaItemHandle = 0x%08X, want 0x%08X", wepIdx, slot.GaItems[wepIdx].AoWGaItemHandle, core.NoCustomAoWHandle)
	}

	// slot.Data at weapon's AoWGaItemHandle offset reset to canonical NoCustomAoWHandle.
	weaponByteOff := gaStart + gaAoWSize // 40
	if got := binary.LittleEndian.Uint32(slot.Data[weaponByteOff+16:]); got != core.NoCustomAoWHandle {
		t.Errorf("slot.Data AoWHandle = 0x%08X, want 0x%08X", got, core.NoCustomAoWHandle)
	}

	// AoW GaItem still present (orphaned — game tolerates unused entries).
	aowIdx := findGaItemByHandle(slot.GaItems, aowHandle)
	if aowIdx == -1 {
		t.Error("AoW GaItem was removed from GaItems; should stay as orphan")
	} else if slot.GaItems[aowIdx].ItemID != aowItemID {
		t.Errorf("AoW ItemID changed: 0x%08X", slot.GaItems[aowIdx].ItemID)
	}

	// GaItems count unchanged (no alloc/dealloc on remove).
	if len(slot.GaItems) != 2 {
		t.Errorf("GaItems count changed: want 2, got %d", len(slot.GaItems))
	}
}

// ─── Scenario B: Existing free AoW → Editor Mode creates a NEW copy anyway ──────
//
// Unlike Strict Mode, Editor Mode NEVER reuses an existing AoW handle — sharing a
// handle between two weapons causes EXCEPTION_ACCESS_VIOLATION on game load.
// Even with a free copy available, ApplyWeaponAoW allocates a fresh GaItem.
//
// Fixture: AoW (free) + weapon (no AoW). Version=0 in slot.Data → verbatim rebuild.
// Post-rebuild parseFromData re-scans slot.Data; the new AoW GaItem (created in the
// pre-rebuild GaItems) is not persisted in slot.Data but the weapon's AoWGaItemHandle
// field in slot.Data and in the final GaItems[weapon] is correctly updated.
func TestApplyWeaponAoW_ExistingFreeAoWCreatesNewCopy(t *testing.T) {
	const (
		wepHandle = uint32(0x80800001)
		wepItemID = uint32(0x000F4240) // Standard Dagger +0
		aowHandle = uint32(0xC0800001) // existing free Sword Dance
		aowItemID = uint32(0x80003070)
		// Expected new handle: PartGaItemHandle=0x80, NextGaItemHandle=2
		// → 0xC0000000 | (0x80<<16) | 2 = 0xC0800002
		expectedNewHandle = uint32(0xC0800002)
	)

	entries := []core.GaItemFull{
		{Handle: aowHandle, ItemID: aowItemID, Unk2: -1, Unk3: -1, AoWGaItemHandle: 0xFFFFFFFF},
		{Handle: wepHandle, ItemID: wepItemID, Unk2: -1, Unk3: -1, AoWGaItemHandle: 0xFFFFFFFF},
	}
	gaData := serializeGaItemsEditor(entries)
	inventoryEnd := core.GaItemsStart + len(gaData) // 32 + 8 + 21 = 61

	gaItems := make([]core.GaItemFull, 5120)
	copy(gaItems[:len(entries)], entries)

	gaMap := map[uint32]uint32{
		aowHandle: aowItemID,
		wepHandle: wepItemID,
	}

	app := buildEditorSetSlot(
		gaData, gaItems, gaMap, inventoryEnd,
		1,    // NextAoWIndex — after existing AoW at [0]
		2,    // NextArmamentIndex — after weapon at [1]
		0x80, // PartGaItemHandle
		2,    // NextGaItemHandle — both AoW and weapon counters are 1, next = 2
	)
	slot := &app.save.Slots[0]

	if err := app.ApplyWeaponAoW(0, wepHandle, aowItemID); err != nil {
		t.Fatalf("ApplyWeaponAoW (existing free AoW): unexpected error: %v", err)
	}

	// After parseFromData, slot.GaItems is rebuilt from slot.Data.
	// Weapon is at slot.Data[40:61] → slot.GaItems[1] post-rebuild.
	wepIdx := findGaItemByHandle(slot.GaItems, wepHandle)
	if wepIdx == -1 {
		t.Fatal("weapon not found in GaItems after set")
	}
	got := slot.GaItems[wepIdx].AoWGaItemHandle

	// Editor Mode allocated a new handle — must be different from existing free AoW.
	if got == aowHandle {
		t.Errorf("AoWGaItemHandle = 0x%08X: Editor Mode reused existing handle (want new copy)", got)
	}
	if got != expectedNewHandle {
		t.Errorf("AoWGaItemHandle = 0x%08X, want 0x%08X (new AoW handle)", got, expectedNewHandle)
	}
	if got&core.GaHandleTypeMask != core.ItemTypeAow {
		t.Errorf("AoWGaItemHandle 0x%08X does not have AoW prefix 0xC0000000", got)
	}

	// slot.Data at weapon's AoWGaItemHandle offset — use post-rebuild walker; new AoW is
	// inserted before the weapon (type-segregated placement) shifting its byte offset.
	wepByteOff := gaItemByteOffset(slot, wepHandle)
	if wepByteOff < 0 {
		t.Fatal("weapon not found in slot.Data byte walk after rebuild")
	}
	if dataGot := binary.LittleEndian.Uint32(slot.Data[wepByteOff+16:]); dataGot != got {
		t.Errorf("slot.Data AoWHandle = 0x%08X, want 0x%08X", dataGot, got)
	}

	// Original free AoW GaItem still present in slot.GaItems (rebuild re-scans slot.Data).
	if findGaItemByHandle(slot.GaItems, aowHandle) == -1 {
		t.Error("existing free AoW GaItem lost from GaItems after rebuild")
	}

	// Integrity: new AoW GaItem must exist in post-rebuild GaItems and GaMap.
	// FAILS with Version=0 fixture: verbatim RebuildSlotFull does not write the new
	// GaItem into slot.Data; parseFromData re-scans slot.Data and finds nothing,
	// leaving weapon.AoWGaItemHandle pointing to a dangling handle.
	if findGaItemByHandle(slot.GaItems, got) == -1 {
		t.Errorf("new AoW GaItem (handle 0x%08X) not found in slot.GaItems after rebuild: "+
			"weapon has dangling AoWGaItemHandle", got)
	}
	if itemID := slot.GaMap[got]; itemID != aowItemID {
		t.Errorf("slot.GaMap[0x%08X] = 0x%08X, want 0x%08X — AoW not in GaMap after reparse",
			got, itemID, aowItemID)
	}
}

// ─── Scenario C: No AoW in save → creates new GaItem ────────────────────────────
//
// When no copy of the requested AoW exists, Editor Mode allocates a fresh GaItem.
// With a Version=0 fixture the rebuild is verbatim; the new AoW entry is not written
// to slot.Data by RebuildSlotFull, but the weapon's AoWGaItemHandle field in both
// slot.Data and the final GaItems is correctly patched by the post-rebuild step.
func TestApplyWeaponAoW_MissingAoWCreatesNew(t *testing.T) {
	const (
		wepHandle = uint32(0x80800001)
		wepItemID = uint32(0x000F4240) // Standard Dagger +0
		aowItemID = uint32(0x80003070) // Sword Dance
		// Expected handle: PartGaItemHandle=0x80, NextGaItemHandle=2
		// → 0xC0000000 | (0x80<<16) | 2 = 0xC0800002
		expectedNewHandle = uint32(0xC0800002)
	)

	entries := []core.GaItemFull{
		{Handle: wepHandle, ItemID: wepItemID, Unk2: -1, Unk3: -1, AoWGaItemHandle: 0xFFFFFFFF},
	}
	gaData := serializeGaItemsEditor(entries)
	inventoryEnd := core.GaItemsStart + len(gaData) // 32 + 21 = 53

	gaItems := make([]core.GaItemFull, 5120)
	copy(gaItems[:len(entries)], entries)

	gaMap := map[uint32]uint32{wepHandle: wepItemID}

	app := buildEditorSetSlot(
		gaData, gaItems, gaMap, inventoryEnd,
		0,    // NextAoWIndex — no existing AoW, new one at index 0
		1,    // NextArmamentIndex — weapon is at [0], next armament at [1]
		0x80, // PartGaItemHandle
		2,    // NextGaItemHandle — weapon counter = 1, next = 2
	)
	slot := &app.save.Slots[0]

	if err := app.ApplyWeaponAoW(0, wepHandle, aowItemID); err != nil {
		t.Fatalf("ApplyWeaponAoW (missing AoW): unexpected error: %v", err)
	}

	// Post-rebuild: weapon is at slot.Data[32:53] → GaItems[0].
	wepIdx := findGaItemByHandle(slot.GaItems, wepHandle)
	if wepIdx == -1 {
		t.Fatal("weapon not found in GaItems after set")
	}
	got := slot.GaItems[wepIdx].AoWGaItemHandle
	if got != expectedNewHandle {
		t.Errorf("AoWGaItemHandle = 0x%08X, want 0x%08X", got, expectedNewHandle)
	}
	if got&core.GaHandleTypeMask != core.ItemTypeAow {
		t.Errorf("AoWGaItemHandle 0x%08X does not have AoW prefix 0xC0000000", got)
	}

	// slot.Data at weapon's AoWGaItemHandle offset — use post-rebuild walker; new AoW is
	// inserted before the weapon (type-segregated placement) shifting its byte offset.
	wepByteOff := gaItemByteOffset(slot, wepHandle)
	if wepByteOff < 0 {
		t.Fatal("weapon not found in slot.Data byte walk after rebuild")
	}
	if dataGot := binary.LittleEndian.Uint32(slot.Data[wepByteOff+16:]); dataGot != got {
		t.Errorf("slot.Data AoWHandle = 0x%08X, want 0x%08X", dataGot, got)
	}

	// Integrity: new AoW GaItem must exist in post-rebuild GaItems and GaMap.
	// FAILS with Version=0 fixture: verbatim RebuildSlotFull does not write the new
	// GaItem into slot.Data; parseFromData re-scans slot.Data and finds nothing,
	// leaving weapon.AoWGaItemHandle pointing to a dangling handle.
	if findGaItemByHandle(slot.GaItems, got) == -1 {
		t.Errorf("new AoW GaItem (handle 0x%08X) not found in slot.GaItems after rebuild: "+
			"weapon has dangling AoWGaItemHandle", got)
	}
	if itemID := slot.GaMap[got]; itemID != aowItemID {
		t.Errorf("slot.GaMap[0x%08X] = 0x%08X, want 0x%08X — AoW not in GaMap after reparse",
			got, itemID, aowItemID)
	}
}

// ─── Scenario D: AoW already used by another weapon → Editor Mode creates new copy
//
// Unlike Strict Mode (which returns an error), Editor Mode allocates a fresh GaItem.
// weapon1 keeps its existing AoW; weapon2 gets a brand-new handle.
func TestApplyWeaponAoW_UsedAoWCreatesNewCopy(t *testing.T) {
	const (
		aowHandle  = uint32(0xC0800001) // used by weapon1
		aowItemID  = uint32(0x80003070) // Sword Dance
		wep1Handle = uint32(0x80800001) // has AoW
		wep2Handle = uint32(0x80800002) // wants same AoW itemID
		wepItemID  = uint32(0x000F4240) // Standard Dagger +0
		// NextGaItemHandle=3 (wep1 counter=1, wep2 counter=2, aow counter=1, max=2, next=3)
		// Expected new handle: 0xC0000000|(0x80<<16)|3 = 0xC0800003
		expectedNewHandle = uint32(0xC0800003)
	)

	entries := []core.GaItemFull{
		{Handle: aowHandle, ItemID: aowItemID, Unk2: -1, Unk3: -1, AoWGaItemHandle: 0xFFFFFFFF},
		{Handle: wep1Handle, ItemID: wepItemID, Unk2: -1, Unk3: -1, AoWGaItemHandle: aowHandle},
		{Handle: wep2Handle, ItemID: wepItemID, Unk2: -1, Unk3: -1, AoWGaItemHandle: 0xFFFFFFFF},
	}
	gaData := serializeGaItemsEditor(entries)
	inventoryEnd := core.GaItemsStart + len(gaData) // 32 + 8 + 21 + 21 = 82

	gaItems := make([]core.GaItemFull, 5120)
	copy(gaItems[:len(entries)], entries)

	gaMap := map[uint32]uint32{
		aowHandle:  aowItemID,
		wep1Handle: wepItemID,
		wep2Handle: wepItemID,
	}

	app := buildEditorSetSlot(
		gaData, gaItems, gaMap, inventoryEnd,
		1,    // NextAoWIndex — after existing AoW at [0]
		3,    // NextArmamentIndex — after wep1 at [1] and wep2 at [2]
		0x80, // PartGaItemHandle
		3,    // NextGaItemHandle — max counter = 2 (wep2), next = 3
	)
	slot := &app.save.Slots[0]

	// weapon2 tries to use Sword Dance — Editor Mode creates a new copy, does NOT error.
	if err := app.ApplyWeaponAoW(0, wep2Handle, aowItemID); err != nil {
		t.Fatalf("ApplyWeaponAoW (used AoW): unexpected error: %v", err)
	}

	// weapon2 gets the new handle.
	wep2Idx := findGaItemByHandle(slot.GaItems, wep2Handle)
	if wep2Idx == -1 {
		t.Fatal("weapon2 not found in GaItems after set")
	}
	got := slot.GaItems[wep2Idx].AoWGaItemHandle
	if got != expectedNewHandle {
		t.Errorf("weapon2 AoWGaItemHandle = 0x%08X, want 0x%08X (new copy)", got, expectedNewHandle)
	}
	if got == aowHandle {
		t.Errorf("weapon2 got weapon1's AoW handle 0x%08X — handle was shared (dangerous)", got)
	}
	if got&core.GaHandleTypeMask != core.ItemTypeAow {
		t.Errorf("weapon2 AoWGaItemHandle 0x%08X does not have AoW prefix 0xC0000000", got)
	}

	// slot.Data at weapon2's AoWGaItemHandle offset — use post-rebuild walker.
	wep2ByteOff := gaItemByteOffset(slot, wep2Handle)
	if wep2ByteOff < 0 {
		t.Fatal("weapon2 not found in slot.Data byte walk after rebuild")
	}
	if dataGot := binary.LittleEndian.Uint32(slot.Data[wep2ByteOff+16:]); dataGot != got {
		t.Errorf("slot.Data weapon2 AoWHandle = 0x%08X, want 0x%08X", dataGot, got)
	}

	// weapon1 AoWGaItemHandle unchanged (still points to original AoW).
	wep1Idx := findGaItemByHandle(slot.GaItems, wep1Handle)
	if wep1Idx == -1 {
		t.Fatal("weapon1 not found in GaItems after set")
	}
	if slot.GaItems[wep1Idx].AoWGaItemHandle != aowHandle {
		t.Errorf("weapon1 AoWGaItemHandle = 0x%08X, want 0x%08X (unchanged)", slot.GaItems[wep1Idx].AoWGaItemHandle, aowHandle)
	}

	// Integrity: new AoW GaItem must exist in post-rebuild GaItems and GaMap.
	// FAILS with Version=0 fixture: verbatim RebuildSlotFull does not write the new
	// GaItem into slot.Data; parseFromData re-scans slot.Data and finds nothing,
	// leaving weapon2.AoWGaItemHandle pointing to a dangling handle.
	if findGaItemByHandle(slot.GaItems, got) == -1 {
		t.Errorf("new AoW GaItem (handle 0x%08X) not found in slot.GaItems after rebuild: "+
			"weapon2 has dangling AoWGaItemHandle", got)
	}
	if itemID := slot.GaMap[got]; itemID != aowItemID {
		t.Errorf("slot.GaMap[0x%08X] = 0x%08X, want 0x%08X — AoW not in GaMap after reparse",
			got, itemID, aowItemID)
	}
}
