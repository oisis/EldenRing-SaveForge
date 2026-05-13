package tests

import (
	"encoding/binary"
	"fmt"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
	"github.com/oisis/EldenRing-SaveForge/backend/db"
	"github.com/oisis/EldenRing-SaveForge/backend/vm"
)

// ── fixture helpers ─────────────────────────────────────────────────────────

// capsForHandles mirrors the app-level cap resolver: includes a cap only for
// quantity-merge handles (goods 0xB0). Instance-move handles return empty
// entries — the core transfer never reads caps for those.
func capsForHandles(slot *core.SaveSlot, handles []uint32, direction core.TransferDirection) *core.TransferOptions {
	caps := make(map[uint32]uint32, len(handles))
	for _, h := range handles {
		if h == core.GaHandleEmpty || h == core.GaHandleInvalid {
			continue
		}
		if h&core.GaHandleTypeMask != core.ItemTypeItem {
			continue
		}
		itemID, ok := slot.GaMap[h]
		if !ok {
			itemID = db.HandleToItemID(h)
		}
		itemData, _ := db.GetItemDataFuzzy(itemID)
		if direction == core.TransferToStorage {
			caps[h] = itemData.MaxStorage
		} else {
			caps[h] = itemData.MaxInventory
		}
	}
	return &core.TransferOptions{DestCaps: caps}
}

// diagnoseTransfer logs detailed handle context on test failure to make
// transfer regressions cheap to debug.
func diagnoseTransfer(t *testing.T, slot *core.SaveSlot, handle uint32, dir string) {
	t.Helper()
	prefix := handle & core.GaHandleTypeMask
	gaMapID, gaMapOk := slot.GaMap[handle]
	htoiID := db.HandleToItemID(handle)
	itemData, _ := db.GetItemDataFuzzy(htoiID)
	itemName := itemData.Name
	if itemName == "" && gaMapOk {
		itemData2, _ := db.GetItemDataFuzzy(gaMapID)
		itemName = itemData2.Name
		itemData = itemData2
	}
	var srcStart, srcSlots, dstStart, dstSlots int
	var srcLabel, dstLabel string
	if dir == "to-storage" {
		srcStart = slot.MagicOffset + core.InvStartFromMagic
		srcSlots = core.CommonItemCount
		srcLabel = "inventory"
		dstStart = slot.StorageBoxOffset + core.StorageHeaderSkip
		dstSlots = core.StorageCommonCount
		dstLabel = "storage"
	} else {
		srcStart = slot.StorageBoxOffset + core.StorageHeaderSkip
		srcSlots = core.StorageCommonCount
		srcLabel = "storage"
		dstStart = slot.MagicOffset + core.InvStartFromMagic
		dstSlots = core.CommonItemCount
		dstLabel = "inventory"
	}
	var srcPresent, dstPresent bool
	var srcQty, dstQty uint32
	for i := 0; i < srcSlots; i++ {
		off := srcStart + i*core.InvRecordLen
		if binary.LittleEndian.Uint32(slot.Data[off:]) == handle {
			srcPresent = true
			srcQty = binary.LittleEndian.Uint32(slot.Data[off+4:])
			break
		}
	}
	for i := 0; i < dstSlots; i++ {
		off := dstStart + i*core.InvRecordLen
		if binary.LittleEndian.Uint32(slot.Data[off:]) == handle {
			dstPresent = true
			dstQty = binary.LittleEndian.Uint32(slot.Data[off+4:])
			break
		}
	}
	instanceMove := prefix == core.ItemTypeWeapon || prefix == core.ItemTypeArmor ||
		prefix == core.ItemTypeAccessory || prefix == core.ItemTypeAow
	t.Logf("─── TRANSFER DIAG ───")
	t.Logf("handle=0x%08X prefix=0x%08X instance-move=%v", handle, prefix, instanceMove)
	t.Logf("GaMap[handle]=0x%08X (ok=%v) | HandleToItemID=0x%08X", gaMapID, gaMapOk, htoiID)
	t.Logf("name=%q MaxInventory=%d MaxStorage=%d", itemName, itemData.MaxInventory, itemData.MaxStorage)
	t.Logf("%s: present=%v qty=%d  |  %s: present=%v qty=%d",
		srcLabel, srcPresent, srcQty, dstLabel, dstPresent, dstQty)
}

// loadTransferTestSave loads the standard PC test save and returns the first
// non-empty slot. Mirrors loadBulkTestSave to keep the fixtures consistent.
func loadTransferTestSave(t *testing.T) (*core.SaveFile, int) {
	t.Helper()
	save, err := core.LoadSave("../tmp/save/ER0000.sl2")
	if err != nil {
		t.Skipf("test save not found: %v", err)
	}
	slotIdx := -1
	for i, s := range save.Slots {
		if s.Version > 0 {
			slotIdx = i
			break
		}
	}
	if slotIdx < 0 {
		t.Skip("no non-empty slots in test save")
	}
	return save, slotIdx
}

func countInventoryRecords(slot *core.SaveSlot) int {
	n := 0
	start := slot.MagicOffset + core.InvStartFromMagic
	for i := 0; i < core.CommonItemCount; i++ {
		off := start + i*core.InvRecordLen
		if off+core.InvRecordLen > len(slot.Data) {
			break
		}
		h := binary.LittleEndian.Uint32(slot.Data[off:])
		if h != core.GaHandleEmpty && h != core.GaHandleInvalid {
			n++
		}
	}
	return n
}

func countStorageRecords(slot *core.SaveSlot) int {
	n := 0
	start := slot.StorageBoxOffset + core.StorageHeaderSkip
	for i := 0; i < core.StorageCommonCount; i++ {
		off := start + i*core.InvRecordLen
		if off+core.InvRecordLen > len(slot.Data) {
			break
		}
		h := binary.LittleEndian.Uint32(slot.Data[off:])
		if h != core.GaHandleEmpty && h != core.GaHandleInvalid {
			n++
		}
	}
	return n
}

func inventoryHeaderCount(slot *core.SaveSlot) uint32 {
	off := slot.MagicOffset + core.InvStartFromMagic - 4
	return binary.LittleEndian.Uint32(slot.Data[off:])
}

func storageHeaderCount(slot *core.SaveSlot) uint32 {
	return binary.LittleEndian.Uint32(slot.Data[slot.StorageBoxOffset:])
}

// findInventoryHandle returns (handle, qty) for the first inventory record
// with a handle matching the given prefix mask. Returns (0, 0) if none found.
func findInventoryHandle(slot *core.SaveSlot, prefixMask uint32) (uint32, uint32) {
	start := slot.MagicOffset + core.InvStartFromMagic
	for i := 0; i < core.CommonItemCount; i++ {
		off := start + i*core.InvRecordLen
		if off+core.InvRecordLen > len(slot.Data) {
			break
		}
		h := binary.LittleEndian.Uint32(slot.Data[off:])
		if h == core.GaHandleEmpty || h == core.GaHandleInvalid {
			continue
		}
		if h&core.GaHandleTypeMask == prefixMask {
			return h, binary.LittleEndian.Uint32(slot.Data[off+4:])
		}
	}
	return 0, 0
}

// findStorageHandle mirrors findInventoryHandle for Storage.
func findStorageHandle(slot *core.SaveSlot, prefixMask uint32) (uint32, uint32) {
	start := slot.StorageBoxOffset + core.StorageHeaderSkip
	for i := 0; i < core.StorageCommonCount; i++ {
		off := start + i*core.InvRecordLen
		if off+core.InvRecordLen > len(slot.Data) {
			break
		}
		h := binary.LittleEndian.Uint32(slot.Data[off:])
		if h == core.GaHandleEmpty || h == core.GaHandleInvalid {
			continue
		}
		if h&core.GaHandleTypeMask == prefixMask {
			return h, binary.LittleEndian.Uint32(slot.Data[off+4:])
		}
	}
	return 0, 0
}

// ensureStorageHasStackable inserts a stackable item with the given handle and
// quantity directly into the first empty storage slot. Bypasses the regular
// AddItemsToSlot machinery to keep test setup deterministic. Returns false if
// no empty slot is available.
func ensureStorageHasStackable(t *testing.T, slot *core.SaveSlot, handle, qty uint32) bool {
	t.Helper()
	start := slot.StorageBoxOffset + core.StorageHeaderSkip
	for i := 0; i < core.StorageCommonCount; i++ {
		off := start + i*core.InvRecordLen
		if off+core.InvRecordLen > len(slot.Data) {
			break
		}
		h := binary.LittleEndian.Uint32(slot.Data[off:])
		if h == core.GaHandleEmpty || h == core.GaHandleInvalid {
			binary.LittleEndian.PutUint32(slot.Data[off:], handle)
			binary.LittleEndian.PutUint32(slot.Data[off+4:], qty)
			// Use a unique Index above InvEquipReservedMax.
			binary.LittleEndian.PutUint32(slot.Data[off+8:], 5000+uint32(i))
			// Update header
			currentCount := binary.LittleEndian.Uint32(slot.Data[slot.StorageBoxOffset:])
			binary.LittleEndian.PutUint32(slot.Data[slot.StorageBoxOffset:], currentCount+1)
			// Update in-memory list
			slot.Storage.CommonItems = append(slot.Storage.CommonItems, core.InventoryItem{
				GaItemHandle: handle, Quantity: qty, Index: 5000 + uint32(i),
			})
			// Make sure GaMap has an entry so the equipped check / id-lookup paths work.
			if _, ok := slot.GaMap[handle]; !ok {
				lower := handle & 0x0FFFFFFF
				slot.GaMap[handle] = lower | 0x40000000 // assume goods
			}
			return true
		}
	}
	return false
}

// ── tests ───────────────────────────────────────────────────────────────────

// TestMoveNonStackableInvToStorage moves a single weapon from Inventory to
// Storage and verifies: handle preserved, GaMap intact, headers correct,
// quantity preserved, GaItem record untouched.
func TestMoveNonStackableInvToStorage(t *testing.T) {
	save, slotIdx := loadTransferTestSave(t)
	slot := &save.Slots[slotIdx]

	// Pick a weapon handle that is NOT equipped.
	var pick uint32
	var pickQty uint32
	start := slot.MagicOffset + core.InvStartFromMagic
	for i := 0; i < core.CommonItemCount; i++ {
		off := start + i*core.InvRecordLen
		h := binary.LittleEndian.Uint32(slot.Data[off:])
		if h == core.GaHandleEmpty || h == core.GaHandleInvalid {
			continue
		}
		if h&core.GaHandleTypeMask != core.ItemTypeWeapon {
			continue
		}
		if core.IsHandleEquipped(slot, h) {
			continue
		}
		pick = h
		pickQty = binary.LittleEndian.Uint32(slot.Data[off+4:])
		break
	}
	if pick == 0 {
		t.Skip("no non-equipped weapon handle found in inventory")
	}

	gaMapItemID := slot.GaMap[pick]
	if gaMapItemID == 0 {
		t.Fatalf("handle 0x%08X not in GaMap before transfer", pick)
	}
	var gaItemBefore core.GaItemFull
	for _, g := range slot.GaItems {
		if g.Handle == pick {
			gaItemBefore = g
			break
		}
	}

	invBefore := countInventoryRecords(slot)
	stoBefore := countStorageRecords(slot)

	res, err := core.MoveItemsBetweenContainers(slot, []uint32{pick}, core.TransferToStorage, capsForHandles(slot, []uint32{pick}, core.TransferToStorage))
	if err != nil {
		t.Fatalf("transfer failed: %v", err)
	}
	if res.Moved != 1 || len(res.Skipped) != 0 {
		t.Fatalf("expected moved=1 skipped=0, got moved=%d skipped=%v", res.Moved, res.Skipped)
	}

	// Source cleared.
	if h, _ := findInventoryHandle(slot, core.ItemTypeWeapon); h == pick {
		// scan whole inv to make sure handle really gone
		for i := 0; i < core.CommonItemCount; i++ {
			off := start + i*core.InvRecordLen
			if binary.LittleEndian.Uint32(slot.Data[off:]) == pick {
				t.Fatalf("handle 0x%08X still present in inventory after transfer", pick)
			}
		}
	}

	// Dest has the handle with same qty.
	stoStart := slot.StorageBoxOffset + core.StorageHeaderSkip
	foundInStorage := false
	for i := 0; i < core.StorageCommonCount; i++ {
		off := stoStart + i*core.InvRecordLen
		if binary.LittleEndian.Uint32(slot.Data[off:]) == pick {
			q := binary.LittleEndian.Uint32(slot.Data[off+4:])
			if q != pickQty {
				t.Errorf("storage qty=%d, expected %d", q, pickQty)
			}
			foundInStorage = true
			break
		}
	}
	if !foundInStorage {
		t.Fatalf("handle 0x%08X not found in storage after transfer", pick)
	}

	// GaMap entry preserved.
	if slot.GaMap[pick] != gaMapItemID {
		t.Errorf("GaMap entry for 0x%08X changed: before=0x%08X, after=0x%08X",
			pick, gaMapItemID, slot.GaMap[pick])
	}
	// GaItem record preserved.
	for _, g := range slot.GaItems {
		if g.Handle == pick {
			if g.ItemID != gaItemBefore.ItemID || g.AoWGaItemHandle != gaItemBefore.AoWGaItemHandle {
				t.Errorf("GaItem changed for 0x%08X: before=%+v, after=%+v", pick, gaItemBefore, g)
			}
			break
		}
	}

	// Headers reconciled.
	invHdr := inventoryHeaderCount(slot)
	stoHdr := storageHeaderCount(slot)
	if int(invHdr) != countInventoryRecords(slot) {
		t.Errorf("inventory header drift: %d vs scan %d", invHdr, countInventoryRecords(slot))
	}
	if int(stoHdr) != countStorageRecords(slot) {
		t.Errorf("storage header drift: %d vs scan %d", stoHdr, countStorageRecords(slot))
	}
	if countInventoryRecords(slot) != invBefore-1 {
		t.Errorf("inventory record count: expected %d, got %d", invBefore-1, countInventoryRecords(slot))
	}
	if countStorageRecords(slot) != stoBefore+1 {
		t.Errorf("storage record count: expected %d, got %d", stoBefore+1, countStorageRecords(slot))
	}
}

// TestMoveNonStackableStorageToInv is the mirror direction. Uses an item we
// inject into Storage to keep the test deterministic (test save may have empty
// storage of the needed type).
func TestMoveNonStackableStorageToInv(t *testing.T) {
	save, slotIdx := loadTransferTestSave(t)
	slot := &save.Slots[slotIdx]

	// Find any non-equipped weapon, move it to storage first so we can move it back.
	var pick, pickQty uint32
	start := slot.MagicOffset + core.InvStartFromMagic
	for i := 0; i < core.CommonItemCount; i++ {
		off := start + i*core.InvRecordLen
		h := binary.LittleEndian.Uint32(slot.Data[off:])
		if h == core.GaHandleEmpty || h == core.GaHandleInvalid {
			continue
		}
		if h&core.GaHandleTypeMask != core.ItemTypeWeapon {
			continue
		}
		if core.IsHandleEquipped(slot, h) {
			continue
		}
		pick = h
		pickQty = binary.LittleEndian.Uint32(slot.Data[off+4:])
		break
	}
	if pick == 0 {
		t.Skip("no non-equipped weapon for round-trip test")
	}

	// Seed: move to storage.
	res1, err := core.MoveItemsBetweenContainers(slot, []uint32{pick}, core.TransferToStorage, capsForHandles(slot, []uint32{pick}, core.TransferToStorage))
	if err != nil || res1.Moved != 1 {
		t.Fatalf("seed move to storage failed: err=%v, moved=%d", err, res1.Moved)
	}

	invBefore := countInventoryRecords(slot)
	stoBefore := countStorageRecords(slot)

	// Move back.
	res2, err := core.MoveItemsBetweenContainers(slot, []uint32{pick}, core.TransferToInventory, capsForHandles(slot, []uint32{pick}, core.TransferToInventory))
	if err != nil {
		t.Fatalf("transfer back failed: %v", err)
	}
	if res2.Moved != 1 {
		t.Fatalf("expected moved=1, got %+v", res2)
	}

	// Storage now lacks the handle, Inventory has it back with same qty.
	stoStart := slot.StorageBoxOffset + core.StorageHeaderSkip
	for i := 0; i < core.StorageCommonCount; i++ {
		off := stoStart + i*core.InvRecordLen
		if binary.LittleEndian.Uint32(slot.Data[off:]) == pick {
			t.Errorf("handle 0x%08X still in storage after move-back", pick)
		}
	}
	foundInInv := false
	for i := 0; i < core.CommonItemCount; i++ {
		off := start + i*core.InvRecordLen
		if binary.LittleEndian.Uint32(slot.Data[off:]) == pick {
			q := binary.LittleEndian.Uint32(slot.Data[off+4:])
			if q != pickQty {
				t.Errorf("inv qty=%d after move-back, expected %d", q, pickQty)
			}
			foundInInv = true
			break
		}
	}
	if !foundInInv {
		t.Fatalf("handle 0x%08X not found in inventory after move-back", pick)
	}

	if countInventoryRecords(slot) != invBefore+1 {
		t.Errorf("inv count after move-back: expected %d, got %d", invBefore+1, countInventoryRecords(slot))
	}
	if countStorageRecords(slot) != stoBefore-1 {
		t.Errorf("storage count after move-back: expected %d, got %d", stoBefore-1, countStorageRecords(slot))
	}
}

// TestMoveStackableInvToStorage_NoDest creates a stackable item in inventory,
// then moves it to empty Storage. Result: storage gets the handle, qty preserved.
func TestMoveStackableInvToStorage_NoDest(t *testing.T) {
	save, slotIdx := loadTransferTestSave(t)
	slot := &save.Slots[slotIdx]
	platform := string(save.Platform)

	// Pick a stackable tool ID and add only to inventory.
	ids := collectIDs("tools", platform, 1)
	if len(ids) == 0 {
		t.Skip("no tool IDs available")
	}
	id := ids[0]
	if err := core.AddItemsToSlot(slot, []uint32{id}, 7, 0, false); err != nil {
		t.Fatalf("seed add failed: %v", err)
	}

	// Find the resulting handle in inventory.
	var handle uint32
	start := slot.MagicOffset + core.InvStartFromMagic
	for i := 0; i < core.CommonItemCount; i++ {
		off := start + i*core.InvRecordLen
		h := binary.LittleEndian.Uint32(slot.Data[off:])
		if h == core.GaHandleEmpty || h == core.GaHandleInvalid {
			continue
		}
		if mapID, ok := slot.GaMap[h]; ok && mapID == id {
			handle = h
			break
		}
	}
	if handle == 0 {
		t.Fatalf("seeded handle for id 0x%08X not found", id)
	}

	// Ensure storage doesn't already have it; if it does, clear that slot.
	stoStart := slot.StorageBoxOffset + core.StorageHeaderSkip
	for i := 0; i < core.StorageCommonCount; i++ {
		off := stoStart + i*core.InvRecordLen
		if binary.LittleEndian.Uint32(slot.Data[off:]) == handle {
			binary.LittleEndian.PutUint32(slot.Data[off:], 0)
			binary.LittleEndian.PutUint32(slot.Data[off+4:], 0)
			binary.LittleEndian.PutUint32(slot.Data[off+8:], 0)
		}
	}
	core.ReconcileStorageHeader(slot)

	res, err := core.MoveItemsBetweenContainers(slot, []uint32{handle}, core.TransferToStorage, capsForHandles(slot, []uint32{handle}, core.TransferToStorage))
	if err != nil || res.Moved != 1 {
		t.Fatalf("transfer failed: err=%v, %+v", err, res)
	}

	// Inventory empty for this handle; storage has qty=7.
	for i := 0; i < core.CommonItemCount; i++ {
		off := start + i*core.InvRecordLen
		if binary.LittleEndian.Uint32(slot.Data[off:]) == handle {
			t.Errorf("inv still has handle 0x%08X", handle)
		}
	}
	gotQty := uint32(0)
	for i := 0; i < core.StorageCommonCount; i++ {
		off := stoStart + i*core.InvRecordLen
		if binary.LittleEndian.Uint32(slot.Data[off:]) == handle {
			gotQty = binary.LittleEndian.Uint32(slot.Data[off+4:])
		}
	}
	if gotQty != 7 {
		t.Errorf("storage qty for transferred stackable: got %d, expected 7", gotQty)
	}
}

// TestMoveStackableStorageToInv_Merge: storage has qty=5, inv has qty=2 of same
// stackable handle. Move storage→inv merges into qty=7. Inv side keeps a single
// record.
func TestMoveStackableStorageToInv_Merge(t *testing.T) {
	save, slotIdx := loadTransferTestSave(t)
	slot := &save.Slots[slotIdx]
	platform := string(save.Platform)

	ids := collectIDs("tools", platform, 1)
	if len(ids) == 0 {
		t.Skip("no tool IDs available")
	}
	id := ids[0]
	// Seed both inv (qty=2) and storage (qty=5). Shared handle is expected for
	// stackable items.
	if err := core.AddItemsToSlot(slot, []uint32{id}, 2, 5, false); err != nil {
		t.Fatalf("seed add failed: %v", err)
	}

	var handle uint32
	for h, mapID := range slot.GaMap {
		if mapID == id {
			handle = h
			break
		}
	}
	if handle == 0 {
		t.Fatalf("seeded handle not in GaMap")
	}

	res, err := core.MoveItemsBetweenContainers(slot, []uint32{handle}, core.TransferToInventory, capsForHandles(slot, []uint32{handle}, core.TransferToInventory))
	if err != nil || res.Moved != 1 {
		t.Fatalf("transfer failed: err=%v, %+v", err, res)
	}

	// Storage cleared for handle.
	stoStart := slot.StorageBoxOffset + core.StorageHeaderSkip
	for i := 0; i < core.StorageCommonCount; i++ {
		off := stoStart + i*core.InvRecordLen
		if binary.LittleEndian.Uint32(slot.Data[off:]) == handle {
			t.Errorf("storage still has handle 0x%08X after merge", handle)
		}
	}
	// Inventory has merged qty=7.
	invStart := slot.MagicOffset + core.InvStartFromMagic
	merged := uint32(0)
	matches := 0
	for i := 0; i < core.CommonItemCount; i++ {
		off := invStart + i*core.InvRecordLen
		if binary.LittleEndian.Uint32(slot.Data[off:]) == handle {
			merged = binary.LittleEndian.Uint32(slot.Data[off+4:])
			matches++
		}
	}
	if matches != 1 {
		t.Errorf("inv match count for handle 0x%08X: got %d, expected 1", handle, matches)
	}
	if merged != 7 {
		t.Errorf("merged qty: got %d, expected 7 (5+2)", merged)
	}

	// GaMap still has the handle.
	if _, ok := slot.GaMap[handle]; !ok {
		t.Errorf("GaMap lost handle 0x%08X after merge transfer", handle)
	}
}

// TestMoveInvalidHandle: handle == 0 or 0xFFFFFFFF → skipped, no mutation.
func TestMoveInvalidHandle(t *testing.T) {
	save, slotIdx := loadTransferTestSave(t)
	slot := &save.Slots[slotIdx]

	invBefore := countInventoryRecords(slot)
	stoBefore := countStorageRecords(slot)

	res, err := core.MoveItemsBetweenContainers(slot, []uint32{0, 0xFFFFFFFF}, core.TransferToStorage, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Moved != 0 {
		t.Errorf("moved=%d, expected 0", res.Moved)
	}
	if len(res.Skipped) != 2 {
		t.Fatalf("expected 2 skipped, got %d", len(res.Skipped))
	}
	for _, s := range res.Skipped {
		if s.Reason != "invalid_handle" {
			t.Errorf("skip reason: got %q, expected invalid_handle", s.Reason)
		}
	}
	if countInventoryRecords(slot) != invBefore || countStorageRecords(slot) != stoBefore {
		t.Errorf("invalid-handle batch mutated state")
	}
}

// TestMoveMixedValidInvalid: batch contains both valid and unknown handles.
// Valid is moved; unknown reported as not_found.
func TestMoveMixedValidInvalid(t *testing.T) {
	save, slotIdx := loadTransferTestSave(t)
	slot := &save.Slots[slotIdx]

	// Pick a non-equipped weapon handle to act as the "valid" one.
	var pick uint32
	start := slot.MagicOffset + core.InvStartFromMagic
	for i := 0; i < core.CommonItemCount; i++ {
		off := start + i*core.InvRecordLen
		h := binary.LittleEndian.Uint32(slot.Data[off:])
		if h == core.GaHandleEmpty || h == core.GaHandleInvalid {
			continue
		}
		if h&core.GaHandleTypeMask != core.ItemTypeWeapon {
			continue
		}
		if core.IsHandleEquipped(slot, h) {
			continue
		}
		pick = h
		break
	}
	if pick == 0 {
		t.Skip("no non-equipped weapon available")
	}

	// Unknown handle: pick something that definitely doesn't exist.
	unknown := uint32(0x8000FFFE)

	res, err := core.MoveItemsBetweenContainers(slot, []uint32{pick, unknown}, core.TransferToStorage, capsForHandles(slot, []uint32{pick, unknown}, core.TransferToStorage))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if res.Moved != 1 {
		t.Errorf("moved=%d, expected 1", res.Moved)
	}
	gotNotFound := false
	for _, s := range res.Skipped {
		if s.Handle == unknown && s.Reason == "not_found" {
			gotNotFound = true
		}
	}
	if !gotNotFound {
		t.Errorf("expected skip not_found for unknown handle, got %+v", res.Skipped)
	}
}

// TestMoveEquippedInvToStorageSkipped: an equipped item is rejected with
// reason "equipped" and the source state is unchanged. The fixture save
// (ER0000.sl2) carries ChrAsmEquipment values that do not reference any of
// its inventory handles (test-only quirk), so this test plants the inventory
// handle's encoded-equipped form into ChrAsmEquipment[0] before invoking the
// transfer, verifying the IsHandleEquipped → skip path end-to-end.
func TestMoveEquippedInvToStorageSkipped(t *testing.T) {
	save, slotIdx := loadTransferTestSave(t)
	slot := &save.Slots[slotIdx]

	// Pick the first non-Unarmed weapon handle to act as our "to be equipped"
	// target.
	var pick uint32
	start := slot.MagicOffset + core.InvStartFromMagic
	for i := 0; i < core.CommonItemCount; i++ {
		off := start + i*core.InvRecordLen
		h := binary.LittleEndian.Uint32(slot.Data[off:])
		if h == core.GaHandleEmpty || h == core.GaHandleInvalid {
			continue
		}
		if h&core.GaHandleTypeMask != core.ItemTypeWeapon {
			continue
		}
		id := slot.GaMap[h]
		// Skip Unarmed placeholder (itemID 0x0001ADB0).
		if id == 0x0001ADB0 {
			continue
		}
		pick = h
		break
	}
	if pick == 0 {
		t.Skip("no real weapon in inventory")
	}

	// Plant the encoded-equipped form (itemID | 0x80000000) into ChrAsm[0].
	id := slot.GaMap[pick]
	binary.LittleEndian.PutUint32(slot.Data[slot.EquipItemsIDOffset:], id|0x80000000)

	// Sanity: IsHandleEquipped now reports true.
	if !core.IsHandleEquipped(slot, pick) {
		t.Fatalf("IsHandleEquipped returned false for planted handle 0x%08X (id 0x%08X)", pick, id)
	}

	equipped := pick

	invBefore := countInventoryRecords(slot)
	stoBefore := countStorageRecords(slot)

	res, err := core.MoveItemsBetweenContainers(slot, []uint32{equipped}, core.TransferToStorage, capsForHandles(slot, []uint32{equipped}, core.TransferToStorage))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if res.Moved != 0 {
		t.Errorf("moved=%d, expected 0 (equipped should be skipped)", res.Moved)
	}
	if len(res.Skipped) != 1 || res.Skipped[0].Reason != "equipped" {
		t.Errorf("expected skip equipped, got %+v", res.Skipped)
	}
	if countInventoryRecords(slot) != invBefore || countStorageRecords(slot) != stoBefore {
		t.Errorf("equipped-skip mutated state")
	}
}

// TestMoveHeadersReconciled: after a batch transfer the binary count headers
// match the actual record scans.
func TestMoveHeadersReconciled(t *testing.T) {
	save, slotIdx := loadTransferTestSave(t)
	slot := &save.Slots[slotIdx]
	platform := string(save.Platform)

	// Add 3 distinct talismans to both inv and storage to give us moveable
	// fixtures.
	talismanIDs := collectIDs("talismans", platform, 3)
	if len(talismanIDs) == 0 {
		t.Skip("no talisman IDs available")
	}
	if err := core.AddItemsToSlot(slot, talismanIDs, 1, 0, false); err != nil {
		t.Fatalf("seed inv add failed: %v", err)
	}

	var handles []uint32
	for _, id := range talismanIDs {
		for h, mapID := range slot.GaMap {
			if mapID == id || db.HandleToItemID(h) == id {
				handles = append(handles, h)
				break
			}
		}
	}
	if len(handles) == 0 {
		t.Skip("no seeded handles resolvable")
	}

	res, err := core.MoveItemsBetweenContainers(slot, handles, core.TransferToStorage, capsForHandles(slot, handles, core.TransferToStorage))
	if err != nil {
		t.Fatalf("transfer err: %v", err)
	}
	if res.Moved == 0 {
		t.Fatalf("nothing moved, %+v", res)
	}

	// Header == scan.
	if int(inventoryHeaderCount(slot)) != countInventoryRecords(slot) {
		t.Errorf("inventory header drift: hdr=%d, scan=%d", inventoryHeaderCount(slot), countInventoryRecords(slot))
	}
	if int(storageHeaderCount(slot)) != countStorageRecords(slot) {
		t.Errorf("storage header drift: hdr=%d, scan=%d", storageHeaderCount(slot), countStorageRecords(slot))
	}
}

// TestMoveNoOrphanedGaItem: after a batch transfer, no GaMap entry was dropped
// for a moved handle, and RepairOrphanedGaItems reports 0 orphans on the
// transferred handles.
func TestMoveNoOrphanedGaItem(t *testing.T) {
	save, slotIdx := loadTransferTestSave(t)
	slot := &save.Slots[slotIdx]

	var pick uint32
	start := slot.MagicOffset + core.InvStartFromMagic
	for i := 0; i < core.CommonItemCount; i++ {
		off := start + i*core.InvRecordLen
		h := binary.LittleEndian.Uint32(slot.Data[off:])
		if h == core.GaHandleEmpty || h == core.GaHandleInvalid {
			continue
		}
		if h&core.GaHandleTypeMask != core.ItemTypeArmor {
			continue
		}
		if core.IsHandleEquipped(slot, h) {
			continue
		}
		pick = h
		break
	}
	if pick == 0 {
		t.Skip("no non-equipped armor available")
	}

	beforeID, beforeOk := slot.GaMap[pick]
	if !beforeOk {
		t.Fatalf("test setup: handle 0x%08X not in GaMap before transfer", pick)
	}

	res, err := core.MoveItemsBetweenContainers(slot, []uint32{pick}, core.TransferToStorage, capsForHandles(slot, []uint32{pick}, core.TransferToStorage))
	if err != nil || res.Moved != 1 {
		t.Fatalf("transfer failed: err=%v, %+v", err, res)
	}

	if id, ok := slot.GaMap[pick]; !ok || id != beforeID {
		t.Errorf("GaMap entry for handle 0x%08X lost or changed: ok=%v, id=0x%08X, was=0x%08X",
			pick, ok, id, beforeID)
	}
}

// TestMoveRoundTripSave: after transfer + write + reload, transferred handle
// is in the expected new location and the GaItem record is intact.
func TestMoveRoundTripSave(t *testing.T) {
	save, slotIdx := loadTransferTestSave(t)
	slot := &save.Slots[slotIdx]
	platform := string(save.Platform)

	var pick uint32
	start := slot.MagicOffset + core.InvStartFromMagic
	for i := 0; i < core.CommonItemCount; i++ {
		off := start + i*core.InvRecordLen
		h := binary.LittleEndian.Uint32(slot.Data[off:])
		if h == core.GaHandleEmpty || h == core.GaHandleInvalid {
			continue
		}
		if h&core.GaHandleTypeMask != core.ItemTypeArmor {
			continue
		}
		if core.IsHandleEquipped(slot, h) {
			continue
		}
		pick = h
		break
	}
	if pick == 0 {
		t.Skip("no non-equipped armor available")
	}

	res, err := core.MoveItemsBetweenContainers(slot, []uint32{pick}, core.TransferToStorage, capsForHandles(slot, []uint32{pick}, core.TransferToStorage))
	if err != nil || res.Moved != 1 {
		t.Fatalf("transfer failed: err=%v, %+v", err, res)
	}

	// Serialize, write to memory, parse back.
	data := slot.Write(platform)
	if len(data) == 0 {
		t.Fatalf("Write produced empty data")
	}

	r2 := core.NewReader(data)
	var slot2 core.SaveSlot
	if err := slot2.Read(r2, platform); err != nil {
		t.Fatalf("re-read after Write failed: %v", err)
	}

	stoStart := slot2.StorageBoxOffset + core.StorageHeaderSkip
	found := false
	for i := 0; i < core.StorageCommonCount; i++ {
		off := stoStart + i*core.InvRecordLen
		if binary.LittleEndian.Uint32(slot2.Data[off:]) == pick {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("handle 0x%08X lost from storage across round-trip", pick)
	}

	if _, ok := slot2.GaMap[pick]; !ok {
		t.Errorf("GaMap lost handle 0x%08X across round-trip", pick)
	}
}

// clearStorageHandle wipes any storage record matching the given handle and
// reconciles the storage header. Used by tests that need a clean dest side
// regardless of fixture state.
func clearStorageHandle(slot *core.SaveSlot, handle uint32) {
	start := slot.StorageBoxOffset + core.StorageHeaderSkip
	for i := 0; i < core.StorageCommonCount; i++ {
		off := start + i*core.InvRecordLen
		if off+core.InvRecordLen > len(slot.Data) {
			break
		}
		if binary.LittleEndian.Uint32(slot.Data[off:]) == handle {
			binary.LittleEndian.PutUint32(slot.Data[off:], 0)
			binary.LittleEndian.PutUint32(slot.Data[off+4:], 0)
			binary.LittleEndian.PutUint32(slot.Data[off+8:], 0)
		}
	}
	core.ReconcileStorageHeader(slot)
	// Rebuild sparse list.
	slot.Storage.CommonItems = slot.Storage.CommonItems[:0]
	for i := 0; i < core.StorageCommonCount; i++ {
		off := start + i*core.InvRecordLen
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
}

// clearInventoryHandle wipes any inventory record matching the given handle.
// Used by tests that need a clean inv side regardless of fixture state.
func clearInventoryHandle(slot *core.SaveSlot, handle uint32) {
	start := slot.MagicOffset + core.InvStartFromMagic
	for i := 0; i < core.CommonItemCount; i++ {
		off := start + i*core.InvRecordLen
		if off+core.InvRecordLen > len(slot.Data) {
			break
		}
		if binary.LittleEndian.Uint32(slot.Data[off:]) == handle {
			binary.LittleEndian.PutUint32(slot.Data[off:], 0)
			binary.LittleEndian.PutUint32(slot.Data[off+4:], 0)
			binary.LittleEndian.PutUint32(slot.Data[off+8:], uint32(i))
			if i < len(slot.Inventory.CommonItems) {
				slot.Inventory.CommonItems[i] = core.InventoryItem{
					GaItemHandle: 0, Quantity: 0, Index: uint32(i),
				}
			}
		}
	}
	core.ReconcileInventoryHeader(slot)
}

// TestMoveTalismanInvToEmptyStorage_PhysicalMove: regression test for the bug
// where talismans (handle prefix 0xA0) were misclassified as quantity-merge
// stackable and rejected with SkipReasonDestAtCap when the destination held
// the same handle. With instance-move semantics a talisman in Inventory must
// move into an empty Storage slot without consulting any cap.
func TestMoveTalismanInvToEmptyStorage_PhysicalMove(t *testing.T) {
	save, slotIdx := loadTransferTestSave(t)
	slot := &save.Slots[slotIdx]
	platform := string(save.Platform)

	talismanIDs := collectIDs("talismans", platform, 1)
	if len(talismanIDs) == 0 {
		t.Skip("no talisman IDs available")
	}
	id := talismanIDs[0]
	if err := core.AddItemsToSlot(slot, []uint32{id}, 1, 0, false); err != nil {
		t.Fatalf("seed: %v", err)
	}
	var handle uint32
	for h, mapID := range slot.GaMap {
		if mapID == id || db.HandleToItemID(h) == id {
			handle = h
			break
		}
	}
	if handle == 0 {
		t.Fatalf("seeded talisman handle for id 0x%08X not found", id)
	}
	// Defensively clear any storage entry for this handle so the test
	// exercises the empty-dest path regardless of fixture state.
	clearStorageHandle(slot, handle)

	opts := capsForHandles(slot, []uint32{handle}, core.TransferToStorage)
	res, err := core.MoveItemsBetweenContainers(slot, []uint32{handle}, core.TransferToStorage, opts)
	if err != nil {
		t.Fatalf("transfer err: %v", err)
	}
	if res.Moved != 1 || len(res.Skipped) != 0 {
		diagnoseTransfer(t, slot, handle, "to-storage")
		t.Fatalf("expected moved=1 skipped=0, got moved=%d skipped=%+v", res.Moved, res.Skipped)
	}
	// Inv cleared, storage has handle with qty=1.
	invStart := slot.MagicOffset + core.InvStartFromMagic
	for i := 0; i < core.CommonItemCount; i++ {
		off := invStart + i*core.InvRecordLen
		if binary.LittleEndian.Uint32(slot.Data[off:]) == handle {
			t.Errorf("handle 0x%08X still in inventory after physical move", handle)
		}
	}
	stoStart := slot.StorageBoxOffset + core.StorageHeaderSkip
	found := false
	for i := 0; i < core.StorageCommonCount; i++ {
		off := stoStart + i*core.InvRecordLen
		if binary.LittleEndian.Uint32(slot.Data[off:]) == handle {
			found = true
			break
		}
	}
	if !found {
		diagnoseTransfer(t, slot, handle, "to-storage")
		t.Errorf("handle 0x%08X not present in storage after move", handle)
	}
	if id, ok := slot.GaMap[handle]; !ok {
		diagnoseTransfer(t, slot, handle, "to-storage")
		t.Errorf("GaMap entry for talisman 0x%08X lost (was 0x%08X)", handle, id)
	}
}

// TestMoveTalismanStorageToEmptyInventory_PhysicalMove mirrors the inv→storage
// case for the reverse direction.
func TestMoveTalismanStorageToEmptyInventory_PhysicalMove(t *testing.T) {
	save, slotIdx := loadTransferTestSave(t)
	slot := &save.Slots[slotIdx]
	platform := string(save.Platform)

	talismanIDs := collectIDs("talismans", platform, 1)
	if len(talismanIDs) == 0 {
		t.Skip("no talisman IDs available")
	}
	id := talismanIDs[0]
	// Seed into storage only.
	if err := core.AddItemsToSlot(slot, []uint32{id}, 0, 1, false); err != nil {
		t.Fatalf("seed: %v", err)
	}
	var handle uint32
	for h, mapID := range slot.GaMap {
		if mapID == id || db.HandleToItemID(h) == id {
			handle = h
			break
		}
	}
	if handle == 0 {
		t.Fatalf("seeded handle for id 0x%08X not found", id)
	}
	clearInventoryHandle(slot, handle)

	opts := capsForHandles(slot, []uint32{handle}, core.TransferToInventory)
	res, err := core.MoveItemsBetweenContainers(slot, []uint32{handle}, core.TransferToInventory, opts)
	if err != nil {
		t.Fatalf("transfer err: %v", err)
	}
	if res.Moved != 1 || len(res.Skipped) != 0 {
		diagnoseTransfer(t, slot, handle, "to-inventory")
		t.Fatalf("expected moved=1 skipped=0, got moved=%d skipped=%+v", res.Moved, res.Skipped)
	}
}

// TestMoveTalismanDestHasSameHandle_RehandlesAndMoves covers the exact
// scenario from the manual report: AddItemsToCharacter populates both
// Inventory and Storage with the same talisman handle. The transfer must
// allocate a fresh unique handle for the moved instance (so the duplicate
// stays in storage alongside the new record), report moved=1, and never emit
// SkipReasonDestDuplicate or SkipReasonDestAtCap.
func TestMoveTalismanDestHasSameHandle_RehandlesAndMoves(t *testing.T) {
	save, slotIdx := loadTransferTestSave(t)
	slot := &save.Slots[slotIdx]
	platform := string(save.Platform)

	talismanIDs := collectIDs("talismans", platform, 1)
	if len(talismanIDs) == 0 {
		t.Skip("no talisman IDs available")
	}
	id := talismanIDs[0]
	// Reproduce UI default: AddItemsToSlot with both qtys puts the same
	// shared talisman handle into inv and storage.
	if err := core.AddItemsToSlot(slot, []uint32{id}, 1, 1, false); err != nil {
		t.Fatalf("seed: %v", err)
	}
	var oldHandle uint32
	for h, mapID := range slot.GaMap {
		if mapID == id || db.HandleToItemID(h) == id {
			oldHandle = h
			break
		}
	}
	if oldHandle == 0 {
		t.Fatalf("seeded handle for id 0x%08X not found", id)
	}

	opts := capsForHandles(slot, []uint32{oldHandle}, core.TransferToStorage)
	res, err := core.MoveItemsBetweenContainers(slot, []uint32{oldHandle}, core.TransferToStorage, opts)
	if err != nil {
		t.Fatalf("transfer err: %v", err)
	}
	if res.Moved != 1 {
		diagnoseTransfer(t, slot, oldHandle, "to-storage")
		t.Fatalf("moved=%d, expected 1 (rehandle path)", res.Moved)
	}
	for _, s := range res.Skipped {
		if s.Reason == core.SkipReasonDestDuplicate || s.Reason == core.SkipReasonDestAtCap {
			diagnoseTransfer(t, slot, oldHandle, "to-storage")
			t.Errorf("unexpected skip reason %q (regression)", s.Reason)
		}
	}

	// Source inventory cleared.
	invStart := slot.MagicOffset + core.InvStartFromMagic
	for i := 0; i < core.CommonItemCount; i++ {
		off := invStart + i*core.InvRecordLen
		if binary.LittleEndian.Uint32(slot.Data[off:]) == oldHandle {
			t.Errorf("inventory still holds oldHandle 0x%08X after rehandled move", oldHandle)
		}
	}

	// Storage must hold both oldHandle (original) and at least one fresh
	// handle whose itemID matches.
	stoStart := slot.StorageBoxOffset + core.StorageHeaderSkip
	originalStillPresent := false
	var newHandle uint32
	for i := 0; i < core.StorageCommonCount; i++ {
		off := stoStart + i*core.InvRecordLen
		h := binary.LittleEndian.Uint32(slot.Data[off:])
		if h == core.GaHandleEmpty || h == core.GaHandleInvalid {
			continue
		}
		if h == oldHandle {
			originalStillPresent = true
			continue
		}
		// Heuristic: look for the talisman item ID via GaMap (rehandled
		// instances always populate GaMap[newHandle]=itemID).
		if mapID, ok := slot.GaMap[h]; ok && mapID == id {
			newHandle = h
		}
	}
	if !originalStillPresent {
		diagnoseTransfer(t, slot, oldHandle, "to-storage")
		t.Errorf("oldHandle 0x%08X missing from storage after rehandled move", oldHandle)
	}
	if newHandle == 0 {
		diagnoseTransfer(t, slot, oldHandle, "to-storage")
		t.Fatalf("no fresh storage record with itemID 0x%08X found after rehandle", id)
	}
	if newHandle == oldHandle {
		t.Fatalf("rehandle returned same handle as original (0x%08X)", newHandle)
	}
	if slot.GaMap[newHandle] != id {
		t.Errorf("GaMap[newHandle 0x%08X] = 0x%08X, expected 0x%08X",
			newHandle, slot.GaMap[newHandle], id)
	}

	// Write+reparse round-trip: both records must survive.
	data := slot.Write(platform)
	if len(data) == 0 {
		t.Fatalf("Write returned empty")
	}
	r := core.NewReader(data)
	var slot2 core.SaveSlot
	if err := slot2.Read(r, platform); err != nil {
		t.Fatalf("re-read failed: %v", err)
	}
	stoStart2 := slot2.StorageBoxOffset + core.StorageHeaderSkip
	rrOld, rrNew := false, false
	for i := 0; i < core.StorageCommonCount; i++ {
		off := stoStart2 + i*core.InvRecordLen
		h := binary.LittleEndian.Uint32(slot2.Data[off:])
		if h == oldHandle {
			rrOld = true
		}
		if h == newHandle {
			rrNew = true
		}
	}
	if !rrOld || !rrNew {
		t.Errorf("round-trip lost a handle: rrOld=%v rrNew=%v", rrOld, rrNew)
	}
}

// TestMoveTalismanStorageToInventory_DuplicateHandle_RehandlesAndMoves mirrors
// the same duplicate-handle rehandle for the reverse direction.
func TestMoveTalismanStorageToInventory_DuplicateHandle_RehandlesAndMoves(t *testing.T) {
	save, slotIdx := loadTransferTestSave(t)
	slot := &save.Slots[slotIdx]
	platform := string(save.Platform)

	talismanIDs := collectIDs("talismans", platform, 1)
	if len(talismanIDs) == 0 {
		t.Skip("no talisman IDs available")
	}
	id := talismanIDs[0]
	if err := core.AddItemsToSlot(slot, []uint32{id}, 1, 1, false); err != nil {
		t.Fatalf("seed: %v", err)
	}
	var oldHandle uint32
	for h, mapID := range slot.GaMap {
		if mapID == id || db.HandleToItemID(h) == id {
			oldHandle = h
			break
		}
	}
	if oldHandle == 0 {
		t.Fatalf("seeded handle not found")
	}
	opts := capsForHandles(slot, []uint32{oldHandle}, core.TransferToInventory)
	res, err := core.MoveItemsBetweenContainers(slot, []uint32{oldHandle}, core.TransferToInventory, opts)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if res.Moved != 1 {
		diagnoseTransfer(t, slot, oldHandle, "to-inventory")
		t.Fatalf("moved=%d, expected 1", res.Moved)
	}
	for _, s := range res.Skipped {
		if s.Reason == core.SkipReasonDestDuplicate || s.Reason == core.SkipReasonDestAtCap {
			t.Errorf("unexpected skip %q", s.Reason)
		}
	}
	// Storage now lacks the source record; inventory has oldHandle and a new handle.
	stoStart := slot.StorageBoxOffset + core.StorageHeaderSkip
	for i := 0; i < core.StorageCommonCount; i++ {
		off := stoStart + i*core.InvRecordLen
		if binary.LittleEndian.Uint32(slot.Data[off:]) == oldHandle {
			t.Errorf("source storage record not cleared")
		}
	}
	invStart := slot.MagicOffset + core.InvStartFromMagic
	hadOld, hadNew := false, false
	for i := 0; i < core.CommonItemCount; i++ {
		off := invStart + i*core.InvRecordLen
		h := binary.LittleEndian.Uint32(slot.Data[off:])
		if h == oldHandle {
			hadOld = true
		} else if h != core.GaHandleEmpty && h != core.GaHandleInvalid {
			if mapID, ok := slot.GaMap[h]; ok && mapID == id {
				hadNew = true
			}
		}
	}
	if !hadOld || !hadNew {
		diagnoseTransfer(t, slot, oldHandle, "to-inventory")
		t.Errorf("inventory should have both original + rehandled record: hadOld=%v hadNew=%v", hadOld, hadNew)
	}
}

// TestMoveGoodsDuplicateHandle_StillQuantityMerge verifies the rehandle path
// is NOT triggered for stackable goods (0xB0) — the original cap-aware
// quantity-merge behaviour must remain.
func TestMoveGoodsDuplicateHandle_StillQuantityMerge(t *testing.T) {
	save, slotIdx := loadTransferTestSave(t)
	slot := &save.Slots[slotIdx]
	platform := string(save.Platform)

	ids := collectIDs("tools", platform, 1)
	if len(ids) == 0 {
		t.Skip("no tool IDs")
	}
	id := ids[0]
	// Seed: tool qty=3 in inv, qty=4 in storage, share handle (stackable).
	if err := core.AddItemsToSlot(slot, []uint32{id}, 3, 4, false); err != nil {
		t.Fatalf("seed: %v", err)
	}
	var handle uint32
	for h, mapID := range slot.GaMap {
		if mapID == id {
			handle = h
			break
		}
	}
	if handle == 0 {
		t.Fatalf("seeded handle not found")
	}
	if handle&core.GaHandleTypeMask != core.ItemTypeItem {
		t.Fatalf("expected goods prefix 0xB0, got 0x%08X", handle)
	}
	opts := capsForHandles(slot, []uint32{handle}, core.TransferToStorage)
	res, err := core.MoveItemsBetweenContainers(slot, []uint32{handle}, core.TransferToStorage, opts)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if res.Moved != 1 {
		t.Fatalf("moved=%d, expected 1", res.Moved)
	}
	// After merge: storage qty == 4+3 = 7 (cap permitting), source cleared.
	// Storage must NOT have multiple records for this handle (no rehandle).
	stoStart := slot.StorageBoxOffset + core.StorageHeaderSkip
	matches := 0
	var stoQty uint32
	for i := 0; i < core.StorageCommonCount; i++ {
		off := stoStart + i*core.InvRecordLen
		if binary.LittleEndian.Uint32(slot.Data[off:]) == handle {
			matches++
			stoQty = binary.LittleEndian.Uint32(slot.Data[off+4:])
		}
	}
	if matches != 1 {
		t.Errorf("goods merge: expected 1 storage record for handle, got %d (rehandle leaked into quantity-merge path)", matches)
	}
	if stoQty != 7 {
		t.Errorf("goods merge qty: got %d, expected 7", stoQty)
	}
	// Source inv cleared.
	invStart := slot.MagicOffset + core.InvStartFromMagic
	for i := 0; i < core.CommonItemCount; i++ {
		off := invStart + i*core.InvRecordLen
		if binary.LittleEndian.Uint32(slot.Data[off:]) == handle {
			t.Errorf("source inventory not cleared after merge")
		}
	}
}

// TestMoveWeaponAllowsDuplicateSameItemIDOnDestination: two weapon instances
// of the same itemID get unique handles via AddItemsToSlot allocation; the
// transfer must move the inventory instance to storage as a separate record,
// not merge with the existing storage record.
func TestMoveWeaponAllowsDuplicateSameItemIDOnDestination(t *testing.T) {
	save, slotIdx := loadTransferTestSave(t)
	slot := &save.Slots[slotIdx]
	platform := string(save.Platform)

	weaponIDs := collectIDs("melee_armaments", platform, 1)
	if len(weaponIDs) == 0 {
		t.Skip("no weapon IDs")
	}
	id := weaponIDs[0]
	gaMapBefore := make(map[uint32]uint32, len(slot.GaMap))
	for k, v := range slot.GaMap {
		gaMapBefore[k] = v
	}
	if err := core.AddItemsToSlot(slot, []uint32{id}, 1, 0, false); err != nil {
		t.Fatalf("seed inv: %v", err)
	}
	if err := core.AddItemsToSlot(slot, []uint32{id}, 0, 1, false); err != nil {
		t.Fatalf("seed sto: %v", err)
	}
	// Collect handles allocated by the seed for this itemID.
	var invHandle, stoHandle uint32
	invStart := slot.MagicOffset + core.InvStartFromMagic
	for i := 0; i < core.CommonItemCount; i++ {
		off := invStart + i*core.InvRecordLen
		h := binary.LittleEndian.Uint32(slot.Data[off:])
		if h == 0 || h == 0xFFFFFFFF {
			continue
		}
		if _, was := gaMapBefore[h]; was {
			continue
		}
		if mapID, ok := slot.GaMap[h]; ok && mapID == id {
			invHandle = h
			break
		}
	}
	stoStart := slot.StorageBoxOffset + core.StorageHeaderSkip
	for i := 0; i < core.StorageCommonCount; i++ {
		off := stoStart + i*core.InvRecordLen
		h := binary.LittleEndian.Uint32(slot.Data[off:])
		if h == 0 || h == 0xFFFFFFFF {
			continue
		}
		if _, was := gaMapBefore[h]; was {
			continue
		}
		if mapID, ok := slot.GaMap[h]; ok && mapID == id {
			stoHandle = h
			break
		}
	}
	if invHandle == 0 || stoHandle == 0 {
		t.Skipf("could not isolate seeded handles (inv=0x%08X sto=0x%08X)", invHandle, stoHandle)
	}
	if invHandle == stoHandle {
		t.Fatalf("weapons should receive distinct handles: invHandle == stoHandle == 0x%08X", invHandle)
	}

	opts := capsForHandles(slot, []uint32{invHandle}, core.TransferToStorage)
	res, err := core.MoveItemsBetweenContainers(slot, []uint32{invHandle}, core.TransferToStorage, opts)
	if err != nil {
		t.Fatalf("transfer err: %v", err)
	}
	if res.Moved != 1 || len(res.Skipped) != 0 {
		diagnoseTransfer(t, slot, invHandle, "to-storage")
		t.Fatalf("expected moved=1 skipped=0 (different handles, same itemID), got moved=%d skipped=%+v",
			res.Moved, res.Skipped)
	}
	// Storage now has BOTH the original stoHandle and the moved invHandle.
	hasInv, hasSto := false, false
	for i := 0; i < core.StorageCommonCount; i++ {
		off := stoStart + i*core.InvRecordLen
		h := binary.LittleEndian.Uint32(slot.Data[off:])
		if h == invHandle {
			hasInv = true
		}
		if h == stoHandle {
			hasSto = true
		}
	}
	if !hasInv || !hasSto {
		t.Errorf("storage must contain both instances after transfer: hasInv=%v hasSto=%v", hasInv, hasSto)
	}
}

// TestMoveArmorAllowsDuplicateSameItemIDOnDestination mirrors the weapon
// duplicate-itemID test for armor (handle prefix 0x90).
func TestMoveArmorAllowsDuplicateSameItemIDOnDestination(t *testing.T) {
	save, slotIdx := loadTransferTestSave(t)
	slot := &save.Slots[slotIdx]
	platform := string(save.Platform)

	armorIDs := collectIDs("chest", platform, 1)
	if len(armorIDs) == 0 {
		t.Skip("no armor IDs")
	}
	id := armorIDs[0]
	gaMapBefore := make(map[uint32]uint32, len(slot.GaMap))
	for k, v := range slot.GaMap {
		gaMapBefore[k] = v
	}
	if err := core.AddItemsToSlot(slot, []uint32{id}, 1, 0, false); err != nil {
		t.Fatalf("seed inv: %v", err)
	}
	if err := core.AddItemsToSlot(slot, []uint32{id}, 0, 1, false); err != nil {
		t.Fatalf("seed sto: %v", err)
	}
	var invHandle, stoHandle uint32
	invStart := slot.MagicOffset + core.InvStartFromMagic
	for i := 0; i < core.CommonItemCount; i++ {
		off := invStart + i*core.InvRecordLen
		h := binary.LittleEndian.Uint32(slot.Data[off:])
		if h == 0 || h == 0xFFFFFFFF {
			continue
		}
		if _, was := gaMapBefore[h]; was {
			continue
		}
		if mapID, ok := slot.GaMap[h]; ok && mapID == id {
			invHandle = h
			break
		}
	}
	stoStart := slot.StorageBoxOffset + core.StorageHeaderSkip
	for i := 0; i < core.StorageCommonCount; i++ {
		off := stoStart + i*core.InvRecordLen
		h := binary.LittleEndian.Uint32(slot.Data[off:])
		if h == 0 || h == 0xFFFFFFFF {
			continue
		}
		if _, was := gaMapBefore[h]; was {
			continue
		}
		if mapID, ok := slot.GaMap[h]; ok && mapID == id {
			stoHandle = h
			break
		}
	}
	if invHandle == 0 || stoHandle == 0 || invHandle == stoHandle {
		t.Skipf("could not isolate distinct seeded handles (inv=0x%08X sto=0x%08X)", invHandle, stoHandle)
	}

	opts := capsForHandles(slot, []uint32{invHandle}, core.TransferToStorage)
	res, err := core.MoveItemsBetweenContainers(slot, []uint32{invHandle}, core.TransferToStorage, opts)
	if err != nil {
		t.Fatalf("transfer err: %v", err)
	}
	if res.Moved != 1 || len(res.Skipped) != 0 {
		diagnoseTransfer(t, slot, invHandle, "to-storage")
		t.Fatalf("expected moved=1 skipped=0, got moved=%d skipped=%+v", res.Moved, res.Skipped)
	}
	hasInv, hasSto := false, false
	for i := 0; i < core.StorageCommonCount; i++ {
		off := stoStart + i*core.InvRecordLen
		h := binary.LittleEndian.Uint32(slot.Data[off:])
		if h == invHandle {
			hasInv = true
		}
		if h == stoHandle {
			hasSto = true
		}
	}
	if !hasInv || !hasSto {
		t.Errorf("storage must contain both armor instances: hasInv=%v hasSto=%v", hasInv, hasSto)
	}
}

// seedStackable seeds a fresh stackable inventory record (and optionally a
// storage one) with explicit quantities. Bypasses AddItemsToSlot so we can
// tune qty in either container independently of the standard add path.
// Returns the handle that was used. Skips the test if no tool ID is available.
func seedStackable(t *testing.T, slot *core.SaveSlot, platform string, invQty, storageQty uint32) uint32 {
	t.Helper()
	ids := collectIDs("tools", platform, 1)
	if len(ids) == 0 {
		t.Skip("no tool IDs available")
	}
	id := ids[0]
	// Use AddItemsToSlot to wire up GaItem/GaMap correctly with qty=1 on both
	// sides, then patch qty in binary to the desired values.
	if err := core.AddItemsToSlot(slot, []uint32{id}, 1, 1, false); err != nil {
		t.Fatalf("seed add: %v", err)
	}
	var handle uint32
	for h, mapID := range slot.GaMap {
		if mapID == id {
			handle = h
			break
		}
	}
	if handle == 0 {
		t.Fatalf("seeded handle for id 0x%08X not found", id)
	}
	// Patch qty in inv binary.
	invStart := slot.MagicOffset + core.InvStartFromMagic
	for i := 0; i < core.CommonItemCount; i++ {
		off := invStart + i*core.InvRecordLen
		if binary.LittleEndian.Uint32(slot.Data[off:]) == handle {
			binary.LittleEndian.PutUint32(slot.Data[off+4:], invQty)
			slot.Inventory.CommonItems[i].Quantity = invQty
			break
		}
	}
	// Patch qty in storage binary.
	stoStart := slot.StorageBoxOffset + core.StorageHeaderSkip
	for i := 0; i < core.StorageCommonCount; i++ {
		off := stoStart + i*core.InvRecordLen
		if binary.LittleEndian.Uint32(slot.Data[off:]) == handle {
			binary.LittleEndian.PutUint32(slot.Data[off+4:], storageQty)
			break
		}
	}
	// Refresh in-memory storage list.
	slot.Storage.CommonItems = nil
	for i := 0; i < core.StorageCommonCount; i++ {
		off := stoStart + i*core.InvRecordLen
		h := binary.LittleEndian.Uint32(slot.Data[off:])
		if h == core.GaHandleEmpty || h == core.GaHandleInvalid {
			continue
		}
		q := binary.LittleEndian.Uint32(slot.Data[off+4:])
		idx := binary.LittleEndian.Uint32(slot.Data[off+8:])
		slot.Storage.CommonItems = append(slot.Storage.CommonItems, core.InventoryItem{
			GaItemHandle: h, Quantity: q, Index: idx,
		})
	}
	return handle
}

// TestMoveStackableInvToStorage_MergePartialAtCap: source qty exceeds the
// available room in the destination → destination filled to cap, source keeps
// the remainder, result reports moved=1 plus a SkipReasonDestAtCap entry with
// MovedQty/RemainingQty.
func TestMoveStackableInvToStorage_MergePartialAtCap(t *testing.T) {
	save, slotIdx := loadTransferTestSave(t)
	slot := &save.Slots[slotIdx]

	handle := seedStackable(t, slot, string(save.Platform), 80, 80)
	cap := uint32(99) // explicit cap regardless of DB value

	opts := &core.TransferOptions{DestCaps: map[uint32]uint32{handle: cap}}
	res, err := core.MoveItemsBetweenContainers(slot, []uint32{handle}, core.TransferToStorage, opts)
	if err != nil {
		t.Fatalf("transfer err: %v", err)
	}
	if res.Moved != 1 {
		t.Errorf("moved=%d, expected 1 (partial counted as moved)", res.Moved)
	}
	if len(res.Skipped) != 1 || res.Skipped[0].Reason != core.SkipReasonDestAtCap {
		t.Fatalf("expected one skip dest_at_cap, got %+v", res.Skipped)
	}
	if res.Skipped[0].MovedQty != 19 {
		t.Errorf("MovedQty=%d, expected 19 (cap 99 - dst 80)", res.Skipped[0].MovedQty)
	}
	if res.Skipped[0].RemainingQty != 61 {
		t.Errorf("RemainingQty=%d, expected 61 (src 80 - moved 19)", res.Skipped[0].RemainingQty)
	}

	// Storage qty == 99.
	stoStart := slot.StorageBoxOffset + core.StorageHeaderSkip
	gotDst := uint32(0)
	for i := 0; i < core.StorageCommonCount; i++ {
		off := stoStart + i*core.InvRecordLen
		if binary.LittleEndian.Uint32(slot.Data[off:]) == handle {
			gotDst = binary.LittleEndian.Uint32(slot.Data[off+4:])
			break
		}
	}
	if gotDst != 99 {
		t.Errorf("storage qty after merge: got %d, expected 99", gotDst)
	}

	// Source qty == 61.
	invStart := slot.MagicOffset + core.InvStartFromMagic
	gotSrc := uint32(0)
	for i := 0; i < core.CommonItemCount; i++ {
		off := invStart + i*core.InvRecordLen
		if binary.LittleEndian.Uint32(slot.Data[off:]) == handle {
			gotSrc = binary.LittleEndian.Uint32(slot.Data[off+4:])
			break
		}
	}
	if gotSrc != 61 {
		t.Errorf("inv qty after partial: got %d, expected 61", gotSrc)
	}
}

// TestMoveStackableInvToStorage_DestAlreadyAtCap: destination at exactly cap
// → no movement, source unchanged, skip dest_at_cap with MovedQty=0.
func TestMoveStackableInvToStorage_DestAlreadyAtCap(t *testing.T) {
	save, slotIdx := loadTransferTestSave(t)
	slot := &save.Slots[slotIdx]

	handle := seedStackable(t, slot, string(save.Platform), 10, 99)
	cap := uint32(99)

	opts := &core.TransferOptions{DestCaps: map[uint32]uint32{handle: cap}}
	res, err := core.MoveItemsBetweenContainers(slot, []uint32{handle}, core.TransferToStorage, opts)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if res.Moved != 0 {
		t.Errorf("moved=%d, expected 0", res.Moved)
	}
	if len(res.Skipped) != 1 || res.Skipped[0].Reason != core.SkipReasonDestAtCap {
		t.Fatalf("expected skip dest_at_cap, got %+v", res.Skipped)
	}
	if res.Skipped[0].MovedQty != 0 {
		t.Errorf("MovedQty=%d, expected 0 (no movement)", res.Skipped[0].MovedQty)
	}

	// Source unchanged.
	invStart := slot.MagicOffset + core.InvStartFromMagic
	gotSrc := uint32(0)
	for i := 0; i < core.CommonItemCount; i++ {
		off := invStart + i*core.InvRecordLen
		if binary.LittleEndian.Uint32(slot.Data[off:]) == handle {
			gotSrc = binary.LittleEndian.Uint32(slot.Data[off+4:])
			break
		}
	}
	if gotSrc != 10 {
		t.Errorf("inv qty after no-move: got %d, expected 10", gotSrc)
	}
}

// TestMoveStackableStorageToInv_PartialAtCap mirrors the partial path for the
// to-inventory direction (verifies MaxInventory cap is honored, not just
// MaxStorage).
func TestMoveStackableStorageToInv_PartialAtCap(t *testing.T) {
	save, slotIdx := loadTransferTestSave(t)
	slot := &save.Slots[slotIdx]

	handle := seedStackable(t, slot, string(save.Platform), 50, 100)
	cap := uint32(60) // inventory cap smaller than total

	opts := &core.TransferOptions{DestCaps: map[uint32]uint32{handle: cap}}
	res, err := core.MoveItemsBetweenContainers(slot, []uint32{handle}, core.TransferToInventory, opts)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if res.Moved != 1 {
		t.Errorf("moved=%d, expected 1 (partial counted)", res.Moved)
	}
	if len(res.Skipped) != 1 || res.Skipped[0].Reason != core.SkipReasonDestAtCap {
		t.Fatalf("expected skip dest_at_cap, got %+v", res.Skipped)
	}
	if res.Skipped[0].MovedQty != 10 {
		t.Errorf("MovedQty=%d, expected 10 (cap 60 - inv 50)", res.Skipped[0].MovedQty)
	}
	if res.Skipped[0].RemainingQty != 90 {
		t.Errorf("RemainingQty=%d, expected 90 (storage 100 - moved 10)", res.Skipped[0].RemainingQty)
	}

	// Inv qty == 60.
	invStart := slot.MagicOffset + core.InvStartFromMagic
	gotInv := uint32(0)
	for i := 0; i < core.CommonItemCount; i++ {
		off := invStart + i*core.InvRecordLen
		if binary.LittleEndian.Uint32(slot.Data[off:]) == handle {
			gotInv = binary.LittleEndian.Uint32(slot.Data[off+4:])
			break
		}
	}
	if gotInv != 60 {
		t.Errorf("inv qty: got %d, expected 60", gotInv)
	}

	// Storage qty == 90 (remainder).
	stoStart := slot.StorageBoxOffset + core.StorageHeaderSkip
	gotSto := uint32(0)
	for i := 0; i < core.StorageCommonCount; i++ {
		off := stoStart + i*core.InvRecordLen
		if binary.LittleEndian.Uint32(slot.Data[off:]) == handle {
			gotSto = binary.LittleEndian.Uint32(slot.Data[off+4:])
			break
		}
	}
	if gotSto != 90 {
		t.Errorf("storage qty: got %d, expected 90", gotSto)
	}
}

// TestMoveStackableMissingCap: stackable handle without cap in opts → skip
// missing_cap, no state change.
func TestMoveStackableMissingCap(t *testing.T) {
	save, slotIdx := loadTransferTestSave(t)
	slot := &save.Slots[slotIdx]

	handle := seedStackable(t, slot, string(save.Platform), 10, 5)

	// Empty caps map.
	opts := &core.TransferOptions{DestCaps: map[uint32]uint32{}}
	res, err := core.MoveItemsBetweenContainers(slot, []uint32{handle}, core.TransferToStorage, opts)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if res.Moved != 0 {
		t.Errorf("moved=%d, expected 0", res.Moved)
	}
	if len(res.Skipped) != 1 || res.Skipped[0].Reason != core.SkipReasonMissingCap {
		t.Fatalf("expected skip missing_cap, got %+v", res.Skipped)
	}

	// Source unchanged.
	invStart := slot.MagicOffset + core.InvStartFromMagic
	gotSrc := uint32(0)
	for i := 0; i < core.CommonItemCount; i++ {
		off := invStart + i*core.InvRecordLen
		if binary.LittleEndian.Uint32(slot.Data[off:]) == handle {
			gotSrc = binary.LittleEndian.Uint32(slot.Data[off+4:])
			break
		}
	}
	if gotSrc != 10 {
		t.Errorf("inv qty after missing_cap skip: got %d, expected 10", gotSrc)
	}
}

// TestMoveDestFull: fill every storage slot with placeholder records, then
// try to transfer a non-stackable inventory weapon. Result: skip dest_full,
// source unchanged.
func TestMoveDestFull(t *testing.T) {
	save, slotIdx := loadTransferTestSave(t)
	slot := &save.Slots[slotIdx]

	// Pick a non-equipped weapon to attempt transferring.
	var pick uint32
	invStart := slot.MagicOffset + core.InvStartFromMagic
	for i := 0; i < core.CommonItemCount; i++ {
		off := invStart + i*core.InvRecordLen
		h := binary.LittleEndian.Uint32(slot.Data[off:])
		if h == core.GaHandleEmpty || h == core.GaHandleInvalid {
			continue
		}
		if h&core.GaHandleTypeMask != core.ItemTypeWeapon {
			continue
		}
		if core.IsHandleEquipped(slot, h) {
			continue
		}
		pick = h
		break
	}
	if pick == 0 {
		t.Skip("no non-equipped weapon to transfer")
	}

	// Saturate storage with placeholder handles. Use a sentinel handle that
	// has the weapon prefix but a counter unlikely to collide with anything.
	// We do not need real GaItems — the transfer rejects dest_full by
	// scanning binary for an empty slot.
	stoStart := slot.StorageBoxOffset + core.StorageHeaderSkip
	for i := 0; i < core.StorageCommonCount; i++ {
		off := stoStart + i*core.InvRecordLen
		if off+core.InvRecordLen > len(slot.Data) {
			break
		}
		h := binary.LittleEndian.Uint32(slot.Data[off:])
		if h == core.GaHandleEmpty || h == core.GaHandleInvalid {
			// Fill the empty slot with a unique placeholder handle. Use goods
			// prefix 0xB0 with the slot index in the lower 28 bits — guaranteed
			// distinct from GaHandleEmpty (0) and GaHandleInvalid (0xFFFFFFFF).
			placeholder := uint32(0xB0000000) | uint32(i)
			binary.LittleEndian.PutUint32(slot.Data[off:], placeholder)
			binary.LittleEndian.PutUint32(slot.Data[off+4:], 1)
			binary.LittleEndian.PutUint32(slot.Data[off+8:], uint32(10000+i))
		}
	}
	core.ReconcileStorageHeader(slot)

	invBefore := countInventoryRecords(slot)

	res, err := core.MoveItemsBetweenContainers(slot, []uint32{pick}, core.TransferToStorage, capsForHandles(slot, []uint32{pick}, core.TransferToStorage))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if res.Moved != 0 {
		t.Errorf("moved=%d, expected 0", res.Moved)
	}
	if len(res.Skipped) != 1 || res.Skipped[0].Reason != core.SkipReasonDestFull {
		t.Fatalf("expected skip dest_full, got %+v", res.Skipped)
	}

	// Source still has the handle.
	if countInventoryRecords(slot) != invBefore {
		t.Errorf("inventory record count changed: before=%d, after=%d", invBefore, countInventoryRecords(slot))
	}
	stillThere := false
	for i := 0; i < core.CommonItemCount; i++ {
		off := invStart + i*core.InvRecordLen
		if binary.LittleEndian.Uint32(slot.Data[off:]) == pick {
			stillThere = true
			break
		}
	}
	if !stillThere {
		t.Errorf("source handle 0x%08X lost after dest_full skip", pick)
	}
}

// countVMTalismans counts talismans in a CharacterViewModel container by
// subCategory (matches the SortOrderTab filter STORAGE_TAB_CATEGORIES).
func countVMTalismans(items []vm.ItemViewModel) int {
	n := 0
	for _, it := range items {
		if it.SubCategory == "talismans" {
			n++
		}
	}
	return n
}

// storageRecordSummary is a minimal projection of a storage CommonItems
// record used by ordering tests.
type storageRecordSummary struct {
	Handle uint32
	Index  uint32
	ID     uint32
}

// storageOrderFor mirrors the read path of App.GetStorageOrder (read storage
// CommonItems, filter by Sort Order tab category, sort by Index ascending).
// Used to verify ordering invariants from tests/ which cannot import the
// main package directly.
func storageOrderFor(slot *core.SaveSlot, tab string) []storageRecordSummary {
	cats := map[string][]string{
		"weapons":   {"melee_armaments", "ranged_and_catalysts", "shields"},
		"talismans": {"talismans"},
		"head":      {"head"},
		"chest":     {"chest"},
		"arms":      {"arms"},
		"legs":      {"legs"},
	}[tab]
	allowed := make(map[string]bool, len(cats))
	for _, c := range cats {
		allowed[c] = true
	}
	start := slot.StorageBoxOffset + core.StorageHeaderSkip
	var entries []storageRecordSummary
	for i := 0; i < core.StorageCommonCount; i++ {
		off := start + i*core.InvRecordLen
		h := binary.LittleEndian.Uint32(slot.Data[off:])
		if h == core.GaHandleEmpty || h == core.GaHandleInvalid {
			continue
		}
		itemID, ok := slot.GaMap[h]
		if !ok {
			itemID = db.HandleToItemID(h)
		}
		data, _ := db.GetItemDataFuzzy(itemID)
		if data.Name == "" || !allowed[data.Category] {
			continue
		}
		idx := binary.LittleEndian.Uint32(slot.Data[off+8:])
		entries = append(entries, storageRecordSummary{Handle: h, Index: idx, ID: itemID})
	}
	for i := 1; i < len(entries); i++ {
		for j := i; j > 0 && entries[j-1].Index > entries[j].Index; j-- {
			entries[j-1], entries[j] = entries[j], entries[j-1]
		}
	}
	return entries
}

// TestTransferTalismanVisibleInVM_InvToStorage reproduces the manual report:
// with a talisman duplicated in Inventory and Storage (shared handle), the
// transfer must (1) clear the source record, (2) materialize a new instance
// on the destination side, and (3) the new instance MUST be visible in
// MapParsedSlotToVM output. Before the VM fix the rehandled instance was
// filtered out because mapItems used HandleToItemID directly — that returns
// garbage for counter-encoded rehandled handles. Inventory count dropped but
// Storage count stayed flat in the UI.
func TestTransferTalismanVisibleInVM_InvToStorage(t *testing.T) {
	save, slotIdx := loadTransferTestSave(t)
	slot := &save.Slots[slotIdx]
	platform := string(save.Platform)

	talismanIDs := collectIDs("talismans", platform, 1)
	if len(talismanIDs) == 0 {
		t.Skip("no talisman IDs")
	}
	id := talismanIDs[0]
	if err := core.AddItemsToSlot(slot, []uint32{id}, 1, 1, false); err != nil {
		t.Fatalf("seed: %v", err)
	}
	var oldHandle uint32
	for h, mapID := range slot.GaMap {
		if mapID == id || db.HandleToItemID(h) == id {
			oldHandle = h
			break
		}
	}
	if oldHandle == 0 {
		t.Fatalf("seeded handle not found")
	}

	vmBefore, err := vm.MapParsedSlotToVM(slot)
	if err != nil {
		t.Fatalf("VM map before: %v", err)
	}
	invTalismanBefore := countVMTalismans(vmBefore.Inventory)
	stoTalismanBefore := countVMTalismans(vmBefore.Storage)
	if invTalismanBefore == 0 || stoTalismanBefore == 0 {
		t.Fatalf("seed precondition failed: invTalismans=%d stoTalismans=%d",
			invTalismanBefore, stoTalismanBefore)
	}

	opts := capsForHandles(slot, []uint32{oldHandle}, core.TransferToStorage)
	res, err := core.MoveItemsBetweenContainers(slot, []uint32{oldHandle}, core.TransferToStorage, opts)
	if err != nil {
		t.Fatalf("transfer: %v", err)
	}
	if res.Moved != 1 {
		diagnoseTransfer(t, slot, oldHandle, "to-storage")
		t.Fatalf("moved=%d skipped=%+v", res.Moved, res.Skipped)
	}

	vmAfter, err := vm.MapParsedSlotToVM(slot)
	if err != nil {
		t.Fatalf("VM map after: %v", err)
	}
	invTalismanAfter := countVMTalismans(vmAfter.Inventory)
	stoTalismanAfter := countVMTalismans(vmAfter.Storage)

	if invTalismanAfter != invTalismanBefore-1 {
		diagnoseTransfer(t, slot, oldHandle, "to-storage")
		t.Errorf("inventory talisman count: before=%d after=%d (expected -1)",
			invTalismanBefore, invTalismanAfter)
	}
	if stoTalismanAfter != stoTalismanBefore+1 {
		diagnoseTransfer(t, slot, oldHandle, "to-storage")
		t.Errorf("storage talisman count: before=%d after=%d (expected +1) — VM dropped the rehandled instance",
			stoTalismanBefore, stoTalismanAfter)
	}

	foundNew := false
	for _, it := range vmAfter.Storage {
		if it.Handle != oldHandle && it.ID == id && it.SubCategory == "talismans" {
			if it.Name == "" {
				t.Errorf("rehandled VM entry has empty name (DB lookup failed?)")
			}
			foundNew = true
			break
		}
	}
	if !foundNew {
		diagnoseTransfer(t, slot, oldHandle, "to-storage")
		t.Errorf("no rehandled VM entry with itemID 0x%08X in storage after transfer", id)
	}
}

// TestTransferTalismanVisibleInVM_StorageToInv mirrors the VM-level visibility
// check for the reverse direction.
func TestTransferTalismanVisibleInVM_StorageToInv(t *testing.T) {
	save, slotIdx := loadTransferTestSave(t)
	slot := &save.Slots[slotIdx]
	platform := string(save.Platform)

	talismanIDs := collectIDs("talismans", platform, 1)
	if len(talismanIDs) == 0 {
		t.Skip("no talisman IDs")
	}
	id := talismanIDs[0]
	if err := core.AddItemsToSlot(slot, []uint32{id}, 1, 1, false); err != nil {
		t.Fatalf("seed: %v", err)
	}
	var oldHandle uint32
	for h, mapID := range slot.GaMap {
		if mapID == id || db.HandleToItemID(h) == id {
			oldHandle = h
			break
		}
	}
	if oldHandle == 0 {
		t.Fatalf("seeded handle not found")
	}

	vmBefore, err := vm.MapParsedSlotToVM(slot)
	if err != nil {
		t.Fatalf("VM map before: %v", err)
	}
	invTalismanBefore := countVMTalismans(vmBefore.Inventory)
	stoTalismanBefore := countVMTalismans(vmBefore.Storage)

	opts := capsForHandles(slot, []uint32{oldHandle}, core.TransferToInventory)
	res, err := core.MoveItemsBetweenContainers(slot, []uint32{oldHandle}, core.TransferToInventory, opts)
	if err != nil {
		t.Fatalf("transfer: %v", err)
	}
	if res.Moved != 1 {
		diagnoseTransfer(t, slot, oldHandle, "to-inventory")
		t.Fatalf("moved=%d", res.Moved)
	}

	vmAfter, err := vm.MapParsedSlotToVM(slot)
	if err != nil {
		t.Fatalf("VM map after: %v", err)
	}
	invTalismanAfter := countVMTalismans(vmAfter.Inventory)
	stoTalismanAfter := countVMTalismans(vmAfter.Storage)

	if stoTalismanAfter != stoTalismanBefore-1 {
		diagnoseTransfer(t, slot, oldHandle, "to-inventory")
		t.Errorf("storage talisman count: before=%d after=%d (expected -1)",
			stoTalismanBefore, stoTalismanAfter)
	}
	if invTalismanAfter != invTalismanBefore+1 {
		diagnoseTransfer(t, slot, oldHandle, "to-inventory")
		t.Errorf("inventory talisman count: before=%d after=%d (expected +1) — VM dropped the rehandled instance",
			invTalismanBefore, invTalismanAfter)
	}
	foundNew := false
	for _, it := range vmAfter.Inventory {
		if it.Handle != oldHandle && it.ID == id && it.SubCategory == "talismans" {
			if it.Name == "" {
				t.Errorf("rehandled VM entry has empty name")
			}
			foundNew = true
			break
		}
	}
	if !foundNew {
		diagnoseTransfer(t, slot, oldHandle, "to-inventory")
		t.Errorf("no rehandled VM entry with itemID 0x%08X in inventory after transfer", id)
	}
}

// TestStorageOrderUsesRecordIndex seeds three talisman records in storage with
// hand-picked Index values and verifies storageOrderFor (mirror of
// App.GetStorageOrder) sorts them by record Index ascending — not by binary
// position, GaMap iteration order, or any other proxy.
func TestStorageOrderUsesRecordIndex(t *testing.T) {
	save, slotIdx := loadTransferTestSave(t)
	slot := &save.Slots[slotIdx]
	platform := string(save.Platform)

	// Seed: three different talismans into storage so each gets its own
	// handle/itemID. Clear any pre-existing copies on the inv side so the
	// fixture is deterministic.
	talismanIDs := collectIDs("talismans", platform, 3)
	if len(talismanIDs) < 3 {
		t.Skip("need at least 3 talisman IDs")
	}
	for _, id := range talismanIDs {
		if err := core.AddItemsToSlot(slot, []uint32{id}, 0, 1, false); err != nil {
			t.Fatalf("seed id=0x%08X: %v", id, err)
		}
	}
	// Collect the three seeded handles + their current binary positions.
	type seeded struct {
		handle uint32
		off    int
	}
	var seeds []seeded
	stoStart := slot.StorageBoxOffset + core.StorageHeaderSkip
	for _, id := range talismanIDs {
		for i := 0; i < core.StorageCommonCount; i++ {
			off := stoStart + i*core.InvRecordLen
			h := binary.LittleEndian.Uint32(slot.Data[off:])
			if h == core.GaHandleEmpty || h == core.GaHandleInvalid {
				continue
			}
			mapID, ok := slot.GaMap[h]
			if !ok {
				mapID = db.HandleToItemID(h)
			}
			if mapID == id {
				seeds = append(seeds, seeded{handle: h, off: off})
				break
			}
		}
	}
	if len(seeds) != 3 {
		t.Skipf("could not isolate three seeded handles (found %d)", len(seeds))
	}
	// Patch each record's Index to a hand-picked value (non-monotonic w.r.t.
	// binary position so trivially-buggy implementations that sort by slot
	// index instead of record Index fail).
	indices := []uint32{6000, 4000, 8000}
	for i, s := range seeds {
		binary.LittleEndian.PutUint32(slot.Data[s.off+8:], indices[i])
	}

	got := storageOrderFor(slot, "talismans")
	pos := map[uint32]int{}
	for i, r := range got {
		pos[r.Handle] = i
	}
	for _, s := range seeds {
		if _, ok := pos[s.handle]; !ok {
			t.Fatalf("handle 0x%08X missing from storage order", s.handle)
		}
	}
	// Acquisition ascending: handle with Index=4000 (seeds[1]) < 6000
	// (seeds[0]) < 8000 (seeds[2]).
	if !(pos[seeds[1].handle] < pos[seeds[0].handle] && pos[seeds[0].handle] < pos[seeds[2].handle]) {
		var dump []string
		for _, r := range got {
			dump = append(dump, fmt.Sprintf("0x%08X(idx=%d)", r.Handle, r.Index))
		}
		t.Errorf("storage order mismatch — expected Index ascending [4000,6000,8000], got %v", dump)
	}
}

// TestTransferInvToStorageAppearsAtEndByAcquisition is the user-reported
// scenario: after Inv→Storage transfer, the new storage record must have the
// highest Index in the destination category so Acquisition ↑ places it at
// the end of the storage grid.
func TestTransferInvToStorageAppearsAtEndByAcquisition(t *testing.T) {
	save, slotIdx := loadTransferTestSave(t)
	slot := &save.Slots[slotIdx]
	platform := string(save.Platform)

	// Seed: three talismans into storage (background population) + one
	// duplicate in inventory that we'll transfer.
	talismanIDs := collectIDs("talismans", platform, 3)
	if len(talismanIDs) < 3 {
		t.Skip("need 3+ talisman IDs")
	}
	for _, id := range talismanIDs[:2] {
		if err := core.AddItemsToSlot(slot, []uint32{id}, 0, 1, false); err != nil {
			t.Fatalf("seed background: %v", err)
		}
	}
	// Third talisman in both inventory and storage so the transfer triggers
	// the rehandle path (the user's actual scenario).
	dupID := talismanIDs[2]
	if err := core.AddItemsToSlot(slot, []uint32{dupID}, 1, 1, false); err != nil {
		t.Fatalf("seed duplicate: %v", err)
	}
	var oldHandle uint32
	for h, mapID := range slot.GaMap {
		if mapID == dupID || db.HandleToItemID(h) == dupID {
			oldHandle = h
			break
		}
	}
	if oldHandle == 0 {
		t.Fatalf("seeded duplicate handle not found")
	}

	beforeMaxIndex := uint32(0)
	for _, r := range storageOrderFor(slot, "talismans") {
		if r.Index > beforeMaxIndex {
			beforeMaxIndex = r.Index
		}
	}

	opts := capsForHandles(slot, []uint32{oldHandle}, core.TransferToStorage)
	res, err := core.MoveItemsBetweenContainers(slot, []uint32{oldHandle}, core.TransferToStorage, opts)
	if err != nil {
		t.Fatalf("transfer: %v", err)
	}
	if res.Moved != 1 {
		t.Fatalf("moved=%d skipped=%+v", res.Moved, res.Skipped)
	}

	after := storageOrderFor(slot, "talismans")
	if len(after) == 0 {
		t.Fatal("storage talisman list empty after transfer")
	}
	// Acquisition ↑ → last entry is the newest.
	last := after[len(after)-1]
	if last.Index <= beforeMaxIndex {
		t.Errorf("new storage record Index=%d not above previous max=%d", last.Index, beforeMaxIndex)
	}
	// The last entry should be the rehandled instance: same itemID as the
	// duplicate, but NOT the oldHandle (which also still exists in storage).
	if last.ID != dupID {
		t.Errorf("last entry itemID=0x%08X, expected dupID=0x%08X", last.ID, dupID)
	}
	if last.Handle == oldHandle {
		t.Errorf("last entry handle is oldHandle 0x%08X — expected rehandled new handle", oldHandle)
	}
}

// TestTransferStorageToInvAppearsAtEndOfInventory is the mirror: after
// Storage→Inv the new Inventory record must be last when sorted by
// AcquisitionIndex ascending. Verified via App.GetInventoryOrder semantics
// (Index ascending) replicated through a direct binary scan.
func TestTransferStorageToInvAppearsAtEndOfInventory(t *testing.T) {
	save, slotIdx := loadTransferTestSave(t)
	slot := &save.Slots[slotIdx]
	platform := string(save.Platform)

	talismanIDs := collectIDs("talismans", platform, 3)
	if len(talismanIDs) < 3 {
		t.Skip("need 3+ talisman IDs")
	}
	for _, id := range talismanIDs[:2] {
		if err := core.AddItemsToSlot(slot, []uint32{id}, 1, 0, false); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	dupID := talismanIDs[2]
	if err := core.AddItemsToSlot(slot, []uint32{dupID}, 1, 1, false); err != nil {
		t.Fatalf("seed duplicate: %v", err)
	}
	var oldHandle uint32
	for h, mapID := range slot.GaMap {
		if mapID == dupID || db.HandleToItemID(h) == dupID {
			oldHandle = h
			break
		}
	}
	if oldHandle == 0 {
		t.Fatalf("seeded duplicate handle not found")
	}

	// Determine the inventory talisman max-Index before the transfer.
	invStart := slot.MagicOffset + core.InvStartFromMagic
	beforeMaxIndex := uint32(0)
	for i := 0; i < core.CommonItemCount; i++ {
		off := invStart + i*core.InvRecordLen
		h := binary.LittleEndian.Uint32(slot.Data[off:])
		if h == core.GaHandleEmpty || h == core.GaHandleInvalid {
			continue
		}
		if h&core.GaHandleTypeMask != core.ItemTypeAccessory {
			continue
		}
		idx := binary.LittleEndian.Uint32(slot.Data[off+8:])
		if idx > beforeMaxIndex {
			beforeMaxIndex = idx
		}
	}

	opts := capsForHandles(slot, []uint32{oldHandle}, core.TransferToInventory)
	res, err := core.MoveItemsBetweenContainers(slot, []uint32{oldHandle}, core.TransferToInventory, opts)
	if err != nil {
		t.Fatalf("transfer: %v", err)
	}
	if res.Moved != 1 {
		t.Fatalf("moved=%d skipped=%+v", res.Moved, res.Skipped)
	}

	// Scan inventory for the highest-Index talisman record; verify it has
	// dupID itemID and a non-oldHandle handle (the rehandled instance).
	var maxIdx uint32
	var lastHandle, lastID uint32
	for i := 0; i < core.CommonItemCount; i++ {
		off := invStart + i*core.InvRecordLen
		h := binary.LittleEndian.Uint32(slot.Data[off:])
		if h == core.GaHandleEmpty || h == core.GaHandleInvalid {
			continue
		}
		if h&core.GaHandleTypeMask != core.ItemTypeAccessory {
			continue
		}
		idx := binary.LittleEndian.Uint32(slot.Data[off+8:])
		if idx >= maxIdx {
			maxIdx = idx
			lastHandle = h
			id, ok := slot.GaMap[h]
			if !ok {
				id = db.HandleToItemID(h)
			}
			lastID = id
		}
	}
	if maxIdx <= beforeMaxIndex {
		t.Errorf("post-transfer max Index=%d not above previous max=%d", maxIdx, beforeMaxIndex)
	}
	if lastID != dupID {
		t.Errorf("last inv talisman itemID=0x%08X, expected dupID=0x%08X", lastID, dupID)
	}
	if lastHandle == oldHandle {
		t.Errorf("last inv talisman handle is oldHandle 0x%08X — expected rehandled new handle", oldHandle)
	}
}

