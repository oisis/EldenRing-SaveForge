package main

import (
	"encoding/binary"
	"strings"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
)

// ─── Fixture ─────────────────────────────────────────────────────────────────

// storageOrderFixture builds an App with the given items placed in slot 0's
// Storage CommonItems section. Layout mirrors the live save:
//
//	[StorageBoxOffset]           : storage count header (u32)
//	[StorageBoxOffset + 4]       : start of CommonItems × StorageCommonCount
//	... + key_count(u32) + KeyItems × StorageKeyCount + next_equip + next_acq
//
// Inventory section is also allocated and seeded with `invItems` so tests can
// verify that ReorderStorage does not mutate Inventory.
func storageOrderFixture(stoItems, invItems []testInvItem) *App {
	const magicOff = 0
	invStart := magicOff + core.InvStartFromMagic
	invKeyStart := invStart + core.CommonItemCount*core.InvRecordLen + core.InvKeyCountHeader
	invNextEquipOff := invKeyStart + core.KeyItemCount*core.InvRecordLen
	invNextAcqOff := invNextEquipOff + 4
	invEnd := invNextAcqOff + 4

	// Place storage right after inventory with a small gap.
	storageBoxOff := invEnd + 16
	stoStart := storageBoxOff + core.StorageHeaderSkip
	stoNextEquipOff := stoStart + core.StorageNextEquipIdxRel
	stoNextAcqOff := stoStart + core.StorageNextAcqSortRel
	bufSize := stoNextAcqOff + 4 + 64

	app := NewApp()
	app.save = &core.SaveFile{}
	slot := &app.save.Slots[0]
	slot.Version = 1
	slot.MagicOffset = magicOff
	slot.StorageBoxOffset = storageBoxOff
	slot.Data = make([]byte, bufSize)
	slot.GaMap = make(map[uint32]uint32)

	// ─ Inventory ─
	var invMaxAcq uint32
	for i, item := range invItems {
		off := invStart + i*core.InvRecordLen
		binary.LittleEndian.PutUint32(slot.Data[off:], item.handle)
		binary.LittleEndian.PutUint32(slot.Data[off+4:], 1)
		binary.LittleEndian.PutUint32(slot.Data[off+8:], item.acqIdx)
		slot.GaMap[item.handle] = item.itemID
		slot.Inventory.CommonItems = append(slot.Inventory.CommonItems, core.InventoryItem{
			GaItemHandle: item.handle,
			Quantity:     1,
			Index:        item.acqIdx,
		})
		if item.acqIdx > invMaxAcq {
			invMaxAcq = item.acqIdx
		}
	}
	if invMaxAcq > 0 {
		invNextAcq := invMaxAcq + 1
		invNextEquip := invNextAcq + 1
		binary.LittleEndian.PutUint32(slot.Data[invNextEquipOff:], invNextEquip)
		binary.LittleEndian.PutUint32(slot.Data[invNextAcqOff:], invNextAcq)
		slot.Inventory.NextEquipIndex = invNextEquip
		slot.Inventory.NextAcquisitionSortId = invNextAcq
	}

	// ─ Storage ─
	var stoMaxAcq uint32
	for i, item := range stoItems {
		off := stoStart + i*core.InvRecordLen
		binary.LittleEndian.PutUint32(slot.Data[off:], item.handle)
		binary.LittleEndian.PutUint32(slot.Data[off+4:], 1)
		binary.LittleEndian.PutUint32(slot.Data[off+8:], item.acqIdx)
		slot.GaMap[item.handle] = item.itemID
		slot.Storage.CommonItems = append(slot.Storage.CommonItems, core.InventoryItem{
			GaItemHandle: item.handle,
			Quantity:     1,
			Index:        item.acqIdx,
		})
		if item.acqIdx > stoMaxAcq {
			stoMaxAcq = item.acqIdx
		}
	}
	binary.LittleEndian.PutUint32(slot.Data[storageBoxOff:], uint32(len(stoItems)))

	stoNextAcq := stoMaxAcq + 1
	stoNextEquip := stoNextAcq + 1
	binary.LittleEndian.PutUint32(slot.Data[stoNextEquipOff:], stoNextEquip)
	binary.LittleEndian.PutUint32(slot.Data[stoNextAcqOff:], stoNextAcq)
	slot.Storage.NextEquipIndex = stoNextEquip
	slot.Storage.NextAcquisitionSortId = stoNextAcq

	// Counter back-write offsets stay at zero: ReorderStorage / ReorderInventory
	// gracefully skip slot.Data writes for the trailing counters when the
	// offset is unset. In-memory fields and per-record Index writes still
	// happen and that's what these tests assert on.
	_ = invNextEquipOff
	_ = invNextAcqOff
	_ = stoNextEquipOff
	_ = stoNextAcqOff

	return app
}

// testStoTalismans is a deterministic two-talisman set placed in storage.
//
//	0xA00003FC → 0x200003FC = Viridian Amber Medallion
//	0xA0000406 → 0x20000406 = Arsenal Charm
//
// Distinct from testTalismans (0xA00003E8, 0xA00003F2) so inventory and
// storage fixtures can coexist with non-overlapping handles.
var testStoTalismans = []testInvItem{
	{0xA00003FC, 0xA00003FC, 700},
	{0xA0000406, 0xA0000406, 702},
}

// ─── Tests ───────────────────────────────────────────────────────────────────

func TestReorderStorage_RejectsMissingHandle(t *testing.T) {
	app := storageOrderFixture(testStoTalismans, nil)
	slot := &app.save.Slots[0]

	beforeIdx := []uint32{}
	for _, it := range slot.Storage.CommonItems {
		beforeIdx = append(beforeIdx, it.Index)
	}

	bogus := uint32(0xA00099AA)
	handles := []uint32{testStoTalismans[0].handle, bogus}
	err := app.ReorderStorage(0, "talismans", handles)
	if err == nil || !strings.Contains(err.Error(), "not found in talisman storage") {
		t.Fatalf("want 'not found in talisman storage', got %v", err)
	}

	for i, it := range slot.Storage.CommonItems {
		if it.Index != beforeIdx[i] {
			t.Errorf("storage item %d index changed after rejected reorder: was %d got %d", i, beforeIdx[i], it.Index)
		}
	}
}

func TestReorderStorage_RejectsDuplicateHandle(t *testing.T) {
	app := storageOrderFixture(testStoTalismans, nil)
	slot := &app.save.Slots[0]

	beforeIdx := []uint32{}
	for _, it := range slot.Storage.CommonItems {
		beforeIdx = append(beforeIdx, it.Index)
	}

	handles := []uint32{testStoTalismans[0].handle, testStoTalismans[0].handle}
	err := app.ReorderStorage(0, "talismans", handles)
	if err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("want 'duplicate' error, got %v", err)
	}

	for i, it := range slot.Storage.CommonItems {
		if it.Index != beforeIdx[i] {
			t.Errorf("storage item %d index changed after rejected reorder: was %d got %d", i, beforeIdx[i], it.Index)
		}
	}
}

func TestReorderStorage_RejectsIncompleteList(t *testing.T) {
	app := storageOrderFixture(testStoTalismans, nil)
	slot := &app.save.Slots[0]

	beforeIdx := []uint32{}
	for _, it := range slot.Storage.CommonItems {
		beforeIdx = append(beforeIdx, it.Index)
	}

	// Only one of the two storage talismans.
	handles := []uint32{testStoTalismans[0].handle}
	err := app.ReorderStorage(0, "talismans", handles)
	if err == nil || !strings.Contains(err.Error(), "2") {
		t.Fatalf("want error mentioning '2' (total), got %v", err)
	}

	for i, it := range slot.Storage.CommonItems {
		if it.Index != beforeIdx[i] {
			t.Errorf("storage item %d index changed after rejected reorder: was %d got %d", i, beforeIdx[i], it.Index)
		}
	}
}

func TestReorderStorage_DoesNotTouchInventory(t *testing.T) {
	// Storage talismans + inventory talismans (different handles).
	invTalismans := []testInvItem{
		{0xA00003E8, 0xA00003E8, 500},
		{0xA00003F2, 0xA00003F2, 502},
	}
	app := storageOrderFixture(testStoTalismans, invTalismans)
	slot := &app.save.Slots[0]

	// Snapshot inventory state.
	type invSnap struct {
		handle uint32
		qty    uint32
		idx    uint32
	}
	var invBefore []invSnap
	invStart := slot.MagicOffset + core.InvStartFromMagic
	for i := 0; i < core.CommonItemCount; i++ {
		off := invStart + i*core.InvRecordLen
		if off+core.InvRecordLen > len(slot.Data) {
			break
		}
		h := binary.LittleEndian.Uint32(slot.Data[off:])
		if h == core.GaHandleEmpty || h == core.GaHandleInvalid {
			continue
		}
		invBefore = append(invBefore, invSnap{
			handle: h,
			qty:    binary.LittleEndian.Uint32(slot.Data[off+4:]),
			idx:    binary.LittleEndian.Uint32(slot.Data[off+8:]),
		})
	}
	invNextAcqBefore := slot.Inventory.NextAcquisitionSortId
	invNextEquipBefore := slot.Inventory.NextEquipIndex

	reversed := []uint32{testStoTalismans[1].handle, testStoTalismans[0].handle}
	if err := app.ReorderStorage(0, "talismans", reversed); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Inventory binary records must be byte-identical.
	var invAfter []invSnap
	for i := 0; i < core.CommonItemCount; i++ {
		off := invStart + i*core.InvRecordLen
		if off+core.InvRecordLen > len(slot.Data) {
			break
		}
		h := binary.LittleEndian.Uint32(slot.Data[off:])
		if h == core.GaHandleEmpty || h == core.GaHandleInvalid {
			continue
		}
		invAfter = append(invAfter, invSnap{
			handle: h,
			qty:    binary.LittleEndian.Uint32(slot.Data[off+4:]),
			idx:    binary.LittleEndian.Uint32(slot.Data[off+8:]),
		})
	}
	if len(invBefore) != len(invAfter) {
		t.Fatalf("inventory record count changed: %d → %d", len(invBefore), len(invAfter))
	}
	for i := range invBefore {
		if invBefore[i] != invAfter[i] {
			t.Errorf("inventory record %d mutated: was %+v got %+v", i, invBefore[i], invAfter[i])
		}
	}

	// Inventory counters must be untouched.
	if slot.Inventory.NextAcquisitionSortId != invNextAcqBefore {
		t.Errorf("Inventory.NextAcquisitionSortId mutated: %d → %d", invNextAcqBefore, slot.Inventory.NextAcquisitionSortId)
	}
	if slot.Inventory.NextEquipIndex != invNextEquipBefore {
		t.Errorf("Inventory.NextEquipIndex mutated: %d → %d", invNextEquipBefore, slot.Inventory.NextEquipIndex)
	}
}

func TestReorderStorage_PersistsAcquisitionOrder(t *testing.T) {
	app := storageOrderFixture(testStoTalismans, nil)
	slot := &app.save.Slots[0]

	reversed := []uint32{testStoTalismans[1].handle, testStoTalismans[0].handle}
	if err := app.ReorderStorage(0, "talismans", reversed); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// nextAcq was 703 (702+1, odd) → stride-2 base rounds up to 704.
	base := uint32(704)
	stoStart := slot.StorageBoxOffset + core.StorageHeaderSkip

	expected := map[uint32]uint32{
		testStoTalismans[1].handle: base,
		testStoTalismans[0].handle: base + 2,
	}
	for i, item := range testStoTalismans {
		off := stoStart + i*core.InvRecordLen
		got := binary.LittleEndian.Uint32(slot.Data[off+8:])
		want := expected[item.handle]
		if got != want {
			t.Errorf("storage[%d] 0x%08X: want idx %d got %d", i, item.handle, want, got)
		}
	}

	// GetStorageOrder must reflect the new order (acq asc).
	items, err := app.GetStorageOrder(0, "talismans")
	if err != nil {
		t.Fatalf("GetStorageOrder: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("want 2 storage talismans, got %d", len(items))
	}
	if items[0].Handle != testStoTalismans[1].handle || items[1].Handle != testStoTalismans[0].handle {
		t.Errorf("storage order not reversed: got [0x%08X, 0x%08X]", items[0].Handle, items[1].Handle)
	}

	// Counters must have advanced past the new max index.
	wantNextAcq := base + 2 + 1
	if slot.Storage.NextAcquisitionSortId != wantNextAcq {
		t.Errorf("Storage.NextAcquisitionSortId want %d got %d", wantNextAcq, slot.Storage.NextAcquisitionSortId)
	}
	if slot.Storage.NextEquipIndex < slot.Storage.NextAcquisitionSortId {
		t.Errorf("Storage.NextEquipIndex %d < NextAcquisitionSortId %d",
			slot.Storage.NextEquipIndex, slot.Storage.NextAcquisitionSortId)
	}
}

func TestReorderStorage_DoesNotTouchHandlesOrQty(t *testing.T) {
	app := storageOrderFixture(testStoTalismans, nil)
	slot := &app.save.Slots[0]

	stoStart := slot.StorageBoxOffset + core.StorageHeaderSkip

	reversed := []uint32{testStoTalismans[1].handle, testStoTalismans[0].handle}
	if err := app.ReorderStorage(0, "talismans", reversed); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Handles/qty at the original binary positions are unchanged (reorder
	// touches only the in-record Index, not handle/qty).
	for i, item := range testStoTalismans {
		off := stoStart + i*core.InvRecordLen
		gotHandle := binary.LittleEndian.Uint32(slot.Data[off:])
		gotQty := binary.LittleEndian.Uint32(slot.Data[off+4:])
		if gotHandle != item.handle {
			t.Errorf("slot.Data handle changed at pos %d: want 0x%08X got 0x%08X", i, item.handle, gotHandle)
		}
		if gotQty != 1 {
			t.Errorf("slot.Data qty changed at pos %d: want 1 got %d", i, gotQty)
		}
	}
}

func TestReorderStorage_RejectsHandleFromInventory(t *testing.T) {
	// Place same-category handle (talisman) in inventory; pass it to ReorderStorage.
	invTalismans := []testInvItem{
		{0xA00003E8, 0xA00003E8, 500},
	}
	app := storageOrderFixture(testStoTalismans, invTalismans)

	// Use the inventory talisman handle as one of the storage handles.
	handles := []uint32{testStoTalismans[0].handle, invTalismans[0].handle}
	err := app.ReorderStorage(0, "talismans", handles)
	if err == nil || !strings.Contains(err.Error(), "not found in talisman storage") {
		t.Fatalf("want 'not found in talisman storage', got %v", err)
	}
}

func TestReorderStorage_RoundTripReread(t *testing.T) {
	// Mirrors TestMoveRoundTripSave's pattern but for in-memory rebuild:
	// after ReorderStorage, the slot.Data bytes contain the new indices, so
	// re-scanning slot.Data via GetStorageOrder reflects the persisted order.
	app := storageOrderFixture(testStoTalismans, nil)
	slot := &app.save.Slots[0]

	reversed := []uint32{testStoTalismans[1].handle, testStoTalismans[0].handle}
	if err := app.ReorderStorage(0, "talismans", reversed); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Snapshot slot.Data and rebuild Storage.CommonItems from raw bytes —
	// proves the persisted on-disk layout (slot.Data) carries the new order
	// independently of the in-memory cache.
	stoStart := slot.StorageBoxOffset + core.StorageHeaderSkip
	slot.Storage.CommonItems = slot.Storage.CommonItems[:0]
	for i := 0; i < core.StorageCommonCount; i++ {
		off := stoStart + i*core.InvRecordLen
		if off+core.InvRecordLen > len(slot.Data) {
			break
		}
		h := binary.LittleEndian.Uint32(slot.Data[off:])
		if h == core.GaHandleEmpty || h == core.GaHandleInvalid {
			continue
		}
		slot.Storage.CommonItems = append(slot.Storage.CommonItems, core.InventoryItem{
			GaItemHandle: h,
			Quantity:     binary.LittleEndian.Uint32(slot.Data[off+4:]),
			Index:        binary.LittleEndian.Uint32(slot.Data[off+8:]),
		})
	}

	items, err := app.GetStorageOrder(0, "talismans")
	if err != nil {
		t.Fatalf("GetStorageOrder after re-scan: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("want 2 talismans after re-scan, got %d", len(items))
	}
	if items[0].Handle != testStoTalismans[1].handle || items[1].Handle != testStoTalismans[0].handle {
		t.Errorf("reverse order lost across re-scan: got [0x%08X, 0x%08X]",
			items[0].Handle, items[1].Handle)
	}
}

func TestInventoryReorder_DoesNotTouchStorage(t *testing.T) {
	// Inventory weapons + storage talismans. Reorder inventory weapons,
	// assert storage indices and counters are byte-identical.
	app := storageOrderFixture(testStoTalismans, nil)
	slot := &app.save.Slots[0]

	// Seed inventory with 4 weapons. Use the same handles/IDs as testWeapons.
	invStart := slot.MagicOffset + core.InvStartFromMagic
	weapons := []testInvWeapon{
		{0x80800001, 0x000F4240, 2000},
		{0x80800002, 0x003085E0, 2002},
		{0x80800003, 0x000F9060, 2004},
		{0x80800004, 0x00116520, 2006},
	}
	for i, w := range weapons {
		off := invStart + i*core.InvRecordLen
		binary.LittleEndian.PutUint32(slot.Data[off:], w.handle)
		binary.LittleEndian.PutUint32(slot.Data[off+4:], 1)
		binary.LittleEndian.PutUint32(slot.Data[off+8:], w.acqIdx)
		slot.GaMap[w.handle] = w.itemID
		slot.Inventory.CommonItems = append(slot.Inventory.CommonItems, core.InventoryItem{
			GaItemHandle: w.handle, Quantity: 1, Index: w.acqIdx,
		})
	}
	slot.Inventory.NextAcquisitionSortId = 2007
	slot.Inventory.NextEquipIndex = 2008

	// Snapshot storage bytes.
	stoStart := slot.StorageBoxOffset + core.StorageHeaderSkip
	stoBytesBefore := make([]byte, core.StorageCommonCount*core.InvRecordLen)
	copy(stoBytesBefore, slot.Data[stoStart:stoStart+len(stoBytesBefore)])
	stoNextAcqBefore := slot.Storage.NextAcquisitionSortId
	stoNextEquipBefore := slot.Storage.NextEquipIndex

	reversed := []uint32{0x80800004, 0x80800003, 0x80800002, 0x80800001}
	if err := app.ReorderInventory(0, "weapons", reversed); err != nil {
		t.Fatalf("ReorderInventory: %v", err)
	}

	stoBytesAfter := slot.Data[stoStart : stoStart+len(stoBytesBefore)]
	for i := range stoBytesBefore {
		if stoBytesBefore[i] != stoBytesAfter[i] {
			t.Fatalf("storage byte %d mutated by ReorderInventory: 0x%02X → 0x%02X",
				i, stoBytesBefore[i], stoBytesAfter[i])
		}
	}
	if slot.Storage.NextAcquisitionSortId != stoNextAcqBefore {
		t.Errorf("Storage.NextAcquisitionSortId mutated: %d → %d", stoNextAcqBefore, slot.Storage.NextAcquisitionSortId)
	}
	if slot.Storage.NextEquipIndex != stoNextEquipBefore {
		t.Errorf("Storage.NextEquipIndex mutated: %d → %d", stoNextEquipBefore, slot.Storage.NextEquipIndex)
	}
}
