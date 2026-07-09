package core

import (
	"encoding/binary"
	"testing"
)

// buildStorageOnlyFixture builds a minimal slot with one storage record and
// an optional in-memory inventory entry. MagicOffset points past Data so
// inventory binary parsing is skipped.
func buildStorageOnlyFixture(t *testing.T, storHandle uint32, gaMap map[uint32]uint32, invEntry *InventoryItem) *SaveSlot {
	t.Helper()
	const storageBoxOff = 0
	storageStart := storageBoxOff + StorageHeaderSkip
	bufSize := storageStart + InvRecordLen + 4

	slot := &SaveSlot{
		Version:          1,
		StorageBoxOffset: storageBoxOff,
		Data:             make([]byte, bufSize),
		GaMap:            gaMap,
	}
	binary.LittleEndian.PutUint32(slot.Data[storageStart:], storHandle)
	binary.LittleEndian.PutUint32(slot.Data[storageStart+4:], 1)
	binary.LittleEndian.PutUint32(slot.Data[storageStart+8:], 600)
	slot.Storage.CommonItems = []InventoryItem{{GaItemHandle: storHandle, Quantity: 1, Index: 600}}

	// Point MagicOffset past Data so inventory is not parsed on re-scan.
	slot.MagicOffset = bufSize
	if invEntry != nil {
		slot.Inventory.CommonItems = []InventoryItem{*invEntry}
	}
	return slot
}

// ---- nil / bad args ---------------------------------------------------------

func TestRehandleInventoryRecord_NilSlot(t *testing.T) {
	_, err := RehandleInventoryRecord(nil, "inventory_common", 0)
	if err == nil {
		t.Fatal("expected error for nil slot")
	}
}

func TestRehandleInventoryRecord_UnknownScope(t *testing.T) {
	slot := buildRepairFixture(t,
		[]InventoryItem{{GaItemHandle: 0xB0001234, Quantity: 1, Index: 500}},
		nil,
	)
	_, err := RehandleInventoryRecord(slot, "inventory_bogus", 0)
	if err == nil {
		t.Fatal("expected error for unknown scope")
	}
}

func TestRehandleInventoryRecord_RowOutOfBounds(t *testing.T) {
	slot := buildRepairFixture(t,
		[]InventoryItem{{GaItemHandle: 0xB0001234, Quantity: 1, Index: 500}},
		nil,
	)
	_, err := RehandleInventoryRecord(slot, "inventory_common", 5)
	if err == nil {
		t.Fatal("expected error for out-of-bounds row")
	}
}

// ---- duplicate handle between containers ------------------------------------

// TestRehandleInventoryRecord_DuplicateHandle_BetweenContainers exercises the
// storage_common scope: the same handle appears in both inventory (in-memory
// only) and storage (in-memory + binary). After rehandling the storage record,
// the inventory entry is unchanged and the storage record has a new unique handle.
func TestRehandleInventoryRecord_DuplicateHandle_BetweenContainers(t *testing.T) {
	const h = uint32(0xB0001234)
	const itemID = uint32(0x40001234) // goods: lower 28 bits | 0x40000000

	invEntry := &InventoryItem{GaItemHandle: h, Quantity: 1, Index: 500}
	slot := buildStorageOnlyFixture(t, h, map[uint32]uint32{h: itemID}, invEntry)

	change, err := RehandleInventoryRecord(slot, "storage_common", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if change.OldHandle != h {
		t.Errorf("OldHandle = 0x%08X, want 0x%08X", change.OldHandle, h)
	}
	if change.NewHandle == h {
		t.Error("NewHandle must differ from OldHandle")
	}
	if change.NewHandle&GaHandleTypeMask != ItemTypeItem {
		t.Errorf("NewHandle prefix 0x%08X, want 0x%08X", change.NewHandle&GaHandleTypeMask, ItemTypeItem)
	}

	// Inventory in-memory must be unchanged.
	if slot.Inventory.CommonItems[0].GaItemHandle != h {
		t.Error("inventory record was modified")
	}

	// Storage in-memory updated.
	if slot.Storage.CommonItems[0].GaItemHandle != change.NewHandle {
		t.Errorf("storage in-memory = 0x%08X, want 0x%08X", slot.Storage.CommonItems[0].GaItemHandle, change.NewHandle)
	}

	// Binary updated.
	storageStart := slot.StorageBoxOffset + StorageHeaderSkip
	raw := binary.LittleEndian.Uint32(slot.Data[storageStart:])
	if raw != change.NewHandle {
		t.Errorf("binary storage handle = 0x%08X, want 0x%08X", raw, change.NewHandle)
	}

	// GaMap has new entry.
	if got, ok := slot.GaMap[change.NewHandle]; !ok || got != itemID {
		t.Errorf("GaMap[new] = 0x%08X ok=%v, want 0x%08X", got, ok, itemID)
	}
	// Old entry preserved.
	if _, ok := slot.GaMap[h]; !ok {
		t.Error("old handle removed from GaMap prematurely")
	}
}

// ---- duplicate handle within same container ---------------------------------

// TestRehandleInventoryRecord_DuplicateHandle_SameContainer rehandles the
// second of two inventory rows that share a handle. Row 0 must be unchanged.
func TestRehandleInventoryRecord_DuplicateHandle_SameContainer(t *testing.T) {
	const h = uint32(0xB0005678)
	const itemID = uint32(0x40005678)

	slot := buildRepairFixture(t,
		[]InventoryItem{
			{GaItemHandle: h, Quantity: 5, Index: 500},
			{GaItemHandle: h, Quantity: 5, Index: 501},
		},
		nil,
	)
	slot.GaMap[h] = itemID

	change, err := RehandleInventoryRecord(slot, "inventory_common", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if change.NewHandle == h {
		t.Error("NewHandle must differ from OldHandle")
	}

	// Row 0 unchanged.
	if slot.Inventory.CommonItems[0].GaItemHandle != h {
		t.Error("row 0 was modified")
	}
	// Row 1 updated in-memory.
	if slot.Inventory.CommonItems[1].GaItemHandle != change.NewHandle {
		t.Errorf("row 1 in-memory = 0x%08X, want 0x%08X", slot.Inventory.CommonItems[1].GaItemHandle, change.NewHandle)
	}

	// Binary row 0 unchanged, row 1 updated.
	commonStart := slot.MagicOffset + InvStartFromMagic
	raw0 := binary.LittleEndian.Uint32(slot.Data[commonStart:])
	raw1 := binary.LittleEndian.Uint32(slot.Data[commonStart+InvRecordLen:])
	if raw0 != h {
		t.Errorf("binary row 0 = 0x%08X, want 0x%08X", raw0, h)
	}
	if raw1 != change.NewHandle {
		t.Errorf("binary row 1 = 0x%08X, want 0x%08X", raw1, change.NewHandle)
	}
}

// ---- talisman (0xA0 prefix) — no GaItem allocated --------------------------

// TestRehandleInventoryRecord_Talisman verifies that a talisman rehandle
// generates a new 0xA0-prefix handle without touching GaItems.
func TestRehandleInventoryRecord_Talisman(t *testing.T) {
	const h = uint32(0xA0123456)

	slot := buildRepairFixture(t,
		[]InventoryItem{{GaItemHandle: h, Quantity: 1, Index: 500}},
		nil,
	)
	// Empty GaItems: if allocateGaItem were called it would fail immediately.
	slot.GaItems = make([]GaItemFull, 0)
	armBefore := slot.NextArmamentIndex

	change, err := RehandleInventoryRecord(slot, "inventory_common", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if change.NewHandle&GaHandleTypeMask != ItemTypeAccessory {
		t.Errorf("NewHandle prefix 0x%08X, want 0x%08X", change.NewHandle&GaHandleTypeMask, ItemTypeAccessory)
	}
	if change.NewHandle == h {
		t.Error("NewHandle must differ from OldHandle")
	}
	// No GaItem allocated.
	if len(slot.GaItems) != 0 {
		t.Errorf("GaItems len = %d, want 0 (no GaItem for talisman)", len(slot.GaItems))
	}
	if slot.NextArmamentIndex != armBefore {
		t.Errorf("NextArmamentIndex changed: %d → %d", armBefore, slot.NextArmamentIndex)
	}
}

// ---- GaItems at capacity — error, no partial mutation ----------------------

// TestRehandleInventoryRecord_GaItemsFull_NoMutation verifies that when the
// GaItems array is full, the function returns an error and leaves slot.Data,
// the in-memory item list, GaMap, and NextGaItemHandle completely unchanged.
func TestRehandleInventoryRecord_GaItemsFull_NoMutation(t *testing.T) {
	const weapHandle = uint32(0x80001234)
	const itemID = uint32(0x00001234)

	slot := buildRepairFixture(t,
		[]InventoryItem{{GaItemHandle: weapHandle, Quantity: 1, Index: 500}},
		nil,
	)
	slot.GaMap[weapHandle] = itemID
	// Empty GaItems array → NextArmamentIndex(0) >= len(GaItems)(0) → full.
	slot.GaItems = make([]GaItemFull, 0)
	gaMapSizeBefore := len(slot.GaMap)
	handleCounterBefore := slot.NextGaItemHandle

	_, err := RehandleInventoryRecord(slot, "inventory_common", 0)
	if err == nil {
		t.Fatal("expected error for full GaItems")
	}

	// In-memory handle unchanged.
	if got := slot.Inventory.CommonItems[0].GaItemHandle; got != weapHandle {
		t.Errorf("in-memory handle = 0x%08X after error, want 0x%08X", got, weapHandle)
	}
	// Binary handle unchanged.
	commonStart := slot.MagicOffset + InvStartFromMagic
	rawH := binary.LittleEndian.Uint32(slot.Data[commonStart:])
	if rawH != weapHandle {
		t.Errorf("binary handle = 0x%08X after error, want 0x%08X", rawH, weapHandle)
	}
	// GaMap unchanged (no new entry).
	if len(slot.GaMap) != gaMapSizeBefore {
		t.Errorf("GaMap grew on error: was %d, now %d", gaMapSizeBefore, len(slot.GaMap))
	}
	// GaItems still empty.
	if len(slot.GaItems) != 0 {
		t.Errorf("GaItems grew on error: len = %d", len(slot.GaItems))
	}
	// NextGaItemHandle restored (snapshot rollback covers generateUniqueHandle advance).
	if slot.NextGaItemHandle != handleCounterBefore {
		t.Errorf("NextGaItemHandle = %d after error, want %d", slot.NextGaItemHandle, handleCounterBefore)
	}
}

// TestRehandleInventoryRecord_RollbackAfterGenerateHandle verifies full snapshot
// rollback on a failure that occurs after generateUniqueHandle has already
// advanced NextGaItemHandle and slot.GaMap has been mutated.
//
// Setup: storage_common scope, talisman handle (no GaItem path). The in-memory
// item says handle h, but the binary storage slot holds a different handle —
// storageBinaryOff returns an error after GaMap was already mutated. The
// snapshot must restore NextGaItemHandle, GaMap, in-memory list, and slot.Data.
func TestRehandleInventoryRecord_RollbackAfterGenerateHandle(t *testing.T) {
	const h = uint32(0xA0999999)             // talisman (no GaItem path)
	const wrongBinary = uint32(0xA0AAAAAA)   // binary holds a different handle

	const storageBoxOff = 0
	storageStart := storageBoxOff + StorageHeaderSkip
	bufSize := storageStart + InvRecordLen + 4

	slot := &SaveSlot{
		Version:          1,
		StorageBoxOffset: storageBoxOff,
		Data:             make([]byte, bufSize),
		GaMap:            map[uint32]uint32{},
		MagicOffset:      bufSize, // no inventory binary
	}
	// Binary storage has a different handle → storageBinaryOff will fail.
	binary.LittleEndian.PutUint32(slot.Data[storageStart:], wrongBinary)
	binary.LittleEndian.PutUint32(slot.Data[storageStart+4:], 1)
	// In-memory says h.
	slot.Storage.CommonItems = []InventoryItem{{GaItemHandle: h, Quantity: 1, Index: 600}}

	handleCounterBefore := slot.NextGaItemHandle
	gaMapSizeBefore := len(slot.GaMap)

	_, err := RehandleInventoryRecord(slot, "storage_common", 0)
	if err == nil {
		t.Fatal("expected error from storageBinaryOff mismatch")
	}

	// NextGaItemHandle must be restored.
	if slot.NextGaItemHandle != handleCounterBefore {
		t.Errorf("NextGaItemHandle = %d after error, want %d", slot.NextGaItemHandle, handleCounterBefore)
	}
	// GaMap must be restored (no new entry).
	if len(slot.GaMap) != gaMapSizeBefore {
		t.Errorf("GaMap grew on error: was %d, now %d", gaMapSizeBefore, len(slot.GaMap))
	}
	// In-memory storage unchanged.
	if slot.Storage.CommonItems[0].GaItemHandle != h {
		t.Errorf("in-memory storage handle = 0x%08X after error, want 0x%08X",
			slot.Storage.CommonItems[0].GaItemHandle, h)
	}
	// Binary unchanged.
	rawH := binary.LittleEndian.Uint32(slot.Data[storageStart:])
	if rawH != wrongBinary {
		t.Errorf("binary handle = 0x%08X after error, want 0x%08X", rawH, wrongBinary)
	}
}
