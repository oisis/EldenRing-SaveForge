package main

import (
	"encoding/binary"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
)

// storageHeaderItemID is a plain stackable crafting material (Aeonian
// Butterfly) that the remembrance fixture adds through the public Database Add
// path with no container/flag/tutorial side effects and a non-zero MaxStorage —
// so it can grow storage, and the only side-effect lifecycle in these tests is
// the storage-header count itself.
const storageHeaderItemID = uint32(0x40005141)

// seedStorageRows writes count non-empty physical storage rows into the
// fixture's storage region (both slot.Data — which core.ReconcileStorageHeader
// scans — and slot.Storage.CommonItems, kept parse-consistent) and stamps the
// 4-byte storage header at StorageBoxOffset with headerCount. Passing a
// headerCount different from count seeds a deliberately stale header.
func seedStorageRows(slot *core.SaveSlot, count int, headerCount uint32) {
	storageStart := slot.StorageBoxOffset + core.StorageHeaderSkip
	if slot.Storage.CommonItems == nil {
		slot.Storage.CommonItems = make([]core.InventoryItem, core.StorageCommonCount)
		for i := range slot.Storage.CommonItems {
			slot.Storage.CommonItems[i].GaItemHandle = core.GaHandleEmpty
		}
	}
	for i := 0; i < count; i++ {
		handle := uint32(0xB0000001) + uint32(i)
		off := storageStart + i*core.InvRecordLen
		binary.LittleEndian.PutUint32(slot.Data[off:], handle)
		binary.LittleEndian.PutUint32(slot.Data[off+4:], 1)
		binary.LittleEndian.PutUint32(slot.Data[off+8:], uint32(800+i))
		slot.Storage.CommonItems[i] = core.InventoryItem{GaItemHandle: handle, Quantity: 1, Index: uint32(800 + i)}
	}
	binary.LittleEndian.PutUint32(slot.Data[slot.StorageBoxOffset:], headerCount)
}

// rawStorageHeader reads the physical 4-byte storage-header scalar back out of
// the real slot after an operation, so tests can prove finished equals it.
func rawStorageHeader(slot *core.SaveSlot) string {
	return giDec(binary.LittleEndian.Uint32(slot.Data[slot.StorageBoxOffset:]))
}

// A. Stale header corrected by an inventory-only add. The executor always
// reconciles the storage header after container sync, so an inventory-only add
// still repairs a deliberately stale header down to the actual physical row
// count.
func TestGameItemsAddStorageHeaderStaleCorrected(t *testing.T) {
	app := gameItemAddApp(true)
	slot := &app.save.Slots[0]

	const physical = 3
	const stale = uint32(42)
	seedStorageRows(slot, physical, stale)

	res, err := app.AddItemsToCharacter(0, []uint32{storageHeaderItemID}, 0, 0, 0, 0, 2, 0)
	if err != nil {
		t.Fatalf("AddItemsToCharacter: %v", err)
	}
	if res.Added != 1 {
		t.Fatalf("res.Added = %d, want 1", res.Added)
	}

	lc := collectGameItemLifecycle(t, app.journal.Tail())
	assertGameItemPhaseGrouping(t, lc)
	if lc.outcome != string(characterChangeSuccess) {
		t.Errorf("outcome = %q, want success", lc.outcome)
	}
	if lc.stage != characterStageCompleted {
		t.Errorf("stage = %q, want completed", lc.stage)
	}

	// stale header -> actual physical count -> actual physical count.
	assertContainerLifecycle(t, lc, "storage_header_count", giDec(stale), giDec(physical), giDec(physical))

	// finished equals the real raw header scalar after the operation.
	if got := rawStorageHeader(slot); got != lc.finished["storage_header_count"] {
		t.Errorf("finished storage_header_count %q != real raw header %q", lc.finished["storage_header_count"], got)
	}
}

// B. Storage add. A storage-only Database Add grows the physical storage rows by
// one; the same reconcile that repairs stale headers also records normal storage
// growth: 0 -> 1 -> 1.
func TestGameItemsAddStorageHeaderStorageGrowth(t *testing.T) {
	app := gameItemAddApp(true)
	slot := &app.save.Slots[0]

	// Fresh fixture: empty storage region, header already 0.
	if got := rawStorageHeader(slot); got != "0" {
		t.Fatalf("precondition: raw storage header = %q, want 0", got)
	}

	res, err := app.AddItemsToCharacter(0, []uint32{storageHeaderItemID}, 0, 0, 0, 0, 0, 1)
	if err != nil {
		t.Fatalf("AddItemsToCharacter: %v", err)
	}
	if res.Added != 1 {
		t.Fatalf("res.Added = %d, want 1", res.Added)
	}

	lc := collectGameItemLifecycle(t, app.journal.Tail())
	assertGameItemAddSuccess(t, lc)

	assertContainerLifecycle(t, lc, "storage_header_count", "0", "1", "1")
	if got := rawStorageHeader(slot); got != lc.finished["storage_header_count"] {
		t.Errorf("finished storage_header_count %q != real raw header %q", lc.finished["storage_header_count"], got)
	}
}

// C. Canonical no-op. A header already equal to its physical row count is left
// untouched by an inventory-only add, so no storage_header_count lifecycle is
// emitted — but the normal direct add lifecycle is.
func TestGameItemsAddStorageHeaderCanonicalNoOp(t *testing.T) {
	app := gameItemAddApp(true)
	slot := &app.save.Slots[0]

	const physical = 3
	seedStorageRows(slot, physical, uint32(physical)) // header already canonical

	res, err := app.AddItemsToCharacter(0, []uint32{storageHeaderItemID}, 0, 0, 0, 0, 2, 0)
	if err != nil {
		t.Fatalf("AddItemsToCharacter: %v", err)
	}
	if res.Added != 1 {
		t.Fatalf("res.Added = %d, want 1", res.Added)
	}

	lc := collectGameItemLifecycle(t, app.journal.Tail())
	assertGameItemPhaseGrouping(t, lc)
	if lc.outcome != string(characterChangeSuccess) {
		t.Errorf("outcome = %q, want success", lc.outcome)
	}
	if lc.stage != characterStageCompleted {
		t.Errorf("stage = %q, want completed", lc.stage)
	}

	// Canonical header self-excludes: no lifecycle for the storage-header scalar.
	assertNoField(t, lc, "storage_header_count")

	// The normal direct add lifecycle remains.
	assertContainerLifecycle(t, lc, "inventory_common_row_0_item_id", giAbsent, giHex(storageHeaderItemID), giHex(storageHeaderItemID))
}

// D. Unavailable storage header. A fixture with no valid storage-header region
// (StorageBoxOffset zeroed) emits no storage_header_count field at all, while the
// normal direct add lifecycle still runs.
func TestGameItemsAddStorageHeaderUnavailable(t *testing.T) {
	app := gameItemAddApp(true)
	slot := &app.save.Slots[0]
	slot.StorageBoxOffset = 0 // no valid 4-byte storage-header region

	res, err := app.AddItemsToCharacter(0, []uint32{storageHeaderItemID}, 0, 0, 0, 0, 2, 0)
	if err != nil {
		t.Fatalf("AddItemsToCharacter: %v", err)
	}
	if res.Added != 1 {
		t.Fatalf("res.Added = %d, want 1", res.Added)
	}

	lc := collectGameItemLifecycle(t, app.journal.Tail())
	assertGameItemPhaseGrouping(t, lc)
	if lc.outcome != string(characterChangeSuccess) {
		t.Errorf("outcome = %q, want success", lc.outcome)
	}
	if lc.stage != characterStageCompleted {
		t.Errorf("stage = %q, want completed", lc.stage)
	}

	assertNoField(t, lc, "storage_header_count")
	assertContainerLifecycle(t, lc, "inventory_common_row_0_item_id", giAbsent, giHex(storageHeaderItemID), giHex(storageHeaderItemID))
}
