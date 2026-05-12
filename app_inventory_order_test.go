package main

import (
	"encoding/binary"
	"strings"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
)

// ─── Fixture ─────────────────────────────────────────────────────────────────

type testInvWeapon struct {
	handle uint32
	itemID uint32 // DB key (e.g. 0x000F4240 = Dagger)
	acqIdx uint32
}

// inventoryOrderFixture builds an App with the given weapons placed in slot 0's
// CommonItems section. MagicOffset is 0; slot.Data is sized to cover the full
// inventory layout (CommonItems + KeyItems + counters). GaMap and in-memory
// CommonItems are populated to match.
func inventoryOrderFixture(weapons []testInvWeapon) *App {
	const magicOff = 0
	startOff := magicOff + core.InvStartFromMagic
	keyStartOff := startOff + core.CommonItemCount*core.InvRecordLen + core.InvKeyCountHeader
	nextEquipIdxOff := keyStartOff + core.KeyItemCount*core.InvRecordLen
	nextAcqSortIdOff := nextEquipIdxOff + 4
	bufSize := nextAcqSortIdOff + 4 + 64

	app := NewApp()
	app.save = &core.SaveFile{}
	slot := &app.save.Slots[0]
	slot.Version = 1
	slot.MagicOffset = magicOff
	slot.Data = make([]byte, bufSize)
	slot.GaMap = make(map[uint32]uint32)

	var maxAcq uint32
	for i, w := range weapons {
		off := startOff + i*core.InvRecordLen
		binary.LittleEndian.PutUint32(slot.Data[off:], w.handle)
		binary.LittleEndian.PutUint32(slot.Data[off+4:], 1) // qty
		binary.LittleEndian.PutUint32(slot.Data[off+8:], w.acqIdx)
		slot.GaMap[w.handle] = w.itemID
		slot.Inventory.CommonItems = append(slot.Inventory.CommonItems, core.InventoryItem{
			GaItemHandle: w.handle,
			Quantity:     1,
			Index:        w.acqIdx,
		})
		if w.acqIdx > maxAcq {
			maxAcq = w.acqIdx
		}
	}

	nextAcq := maxAcq + 1
	nextEquip := nextAcq + 1
	binary.LittleEndian.PutUint32(slot.Data[nextEquipIdxOff:], nextEquip)
	binary.LittleEndian.PutUint32(slot.Data[nextAcqSortIdOff:], nextAcq)
	slot.Inventory.NextEquipIndex = nextEquip
	slot.Inventory.NextAcquisitionSortId = nextAcq

	return app
}

// testWeapons is the default set used by most tests.
// Uses real DB weapon IDs so db.GetItemDataFuzzy resolves correctly.
//   0x000F4240 = Dagger         (melee_armaments)
//   0x003085E0 = Claymore       (melee_armaments)
//   0x000F9060 = Parrying Dagger (melee_armaments)
//   0x00116520 = Bloodstained Dagger (melee_armaments)
var testWeapons = []testInvWeapon{
	{0x80800001, 0x000F4240, 2000},
	{0x80800002, 0x003085E0, 2002},
	{0x80800003, 0x000F9060, 2004},
	{0x80800004, 0x00116520, 2006},
}

// unarmedEntry is the Unarmed technical placeholder.
var unarmedEntry = testInvWeapon{0x8080007A, 0x0001ADB0, 509}

// ─── GetWeaponInventoryOrder ──────────────────────────────────────────────────

func TestGetWeaponInventoryOrder_NoSave(t *testing.T) {
	app := NewApp()
	_, err := app.GetWeaponInventoryOrder(0)
	if err == nil || !strings.Contains(err.Error(), "no save loaded") {
		t.Fatalf("want 'no save loaded', got %v", err)
	}
}

func TestGetWeaponInventoryOrder_InvalidIdx(t *testing.T) {
	app := inventoryOrderFixture(testWeapons)
	for _, idx := range []int{-1, 10, 99} {
		_, err := app.GetWeaponInventoryOrder(idx)
		if err == nil || !strings.Contains(err.Error(), "invalid character index") {
			t.Fatalf("idx=%d: want 'invalid character index', got %v", idx, err)
		}
	}
}

func TestGetWeaponInventoryOrder_EmptySlot(t *testing.T) {
	app := inventoryOrderFixture(testWeapons)
	app.save.Slots[0].Version = 0
	_, err := app.GetWeaponInventoryOrder(0)
	if err == nil || !strings.Contains(err.Error(), "empty") {
		t.Fatalf("want empty slot error, got %v", err)
	}
}

func TestGetWeaponInventoryOrder_ReturnsWeaponsAscending(t *testing.T) {
	// Intentionally store weapons out of order to verify sort.
	weapons := []testInvWeapon{
		{0x80800001, 0x000F4240, 3000}, // Dagger
		{0x80800002, 0x003085E0, 1000}, // Claymore
		{0x80800003, 0x000F9060, 2000}, // Parrying Dagger
	}
	app := inventoryOrderFixture(weapons)
	items, err := app.GetWeaponInventoryOrder(0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 3 {
		t.Fatalf("want 3 items, got %d", len(items))
	}
	// Must be sorted ascending by AcquisitionIndex.
	if items[0].AcquisitionIndex != 1000 || items[1].AcquisitionIndex != 2000 || items[2].AcquisitionIndex != 3000 {
		t.Fatalf("wrong order: %v", items)
	}
	// Verify fields are populated.
	if items[0].Handle == 0 || items[0].ItemID == 0 || items[0].Name == "" || items[0].Category == "" {
		t.Fatalf("item fields not populated: %+v", items[0])
	}
}

func TestGetWeaponInventoryOrder_HidesUnarmed(t *testing.T) {
	weapons := append([]testInvWeapon{unarmedEntry}, testWeapons...)
	app := inventoryOrderFixture(weapons)
	items, err := app.GetWeaponInventoryOrder(0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != len(testWeapons) {
		t.Fatalf("want %d weapons (Unarmed hidden), got %d", len(testWeapons), len(items))
	}
	for _, item := range items {
		if item.Name == "Unarmed" || item.IsTechnical {
			t.Fatalf("Unarmed must not appear in results: %+v", item)
		}
	}
}

func TestGetWeaponInventoryOrder_UpgradeInfusionDecoded(t *testing.T) {
	// Dagger +5 = 0x000F4240 + 5 = 0x000F4245
	// Dagger Keen+3 = 0x000F4240 + 200 + 3 = 0x000F430B
	weapons := []testInvWeapon{
		{0x80800001, 0x000F4245, 2000}, // Dagger +5
		{0x80800002, 0x000F430B, 2002}, // Dagger Keen+3
	}
	app := inventoryOrderFixture(weapons)
	items, err := app.GetWeaponInventoryOrder(0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("want 2 items, got %d", len(items))
	}
	if items[0].CurrentUpgrade != 5 || items[0].InfusionName != "" {
		t.Errorf("Dagger+5: want upgrade=5 infusion='', got upgrade=%d infusion=%q", items[0].CurrentUpgrade, items[0].InfusionName)
	}
	if items[1].CurrentUpgrade != 3 || items[1].InfusionName != "Keen" {
		t.Errorf("Dagger Keen+3: want upgrade=3 infusion='Keen', got upgrade=%d infusion=%q", items[1].CurrentUpgrade, items[1].InfusionName)
	}
}

// ─── ReorderWeaponInventory ───────────────────────────────────────────────────

func TestReorderWeaponInventory_NoSave(t *testing.T) {
	app := NewApp()
	err := app.ReorderWeaponInventory(0, []uint32{0x80800001})
	if err == nil || !strings.Contains(err.Error(), "no save loaded") {
		t.Fatalf("want 'no save loaded', got %v", err)
	}
}

func TestReorderWeaponInventory_InvalidCharIdx(t *testing.T) {
	app := inventoryOrderFixture(testWeapons)
	for _, idx := range []int{-1, 10, 99} {
		err := app.ReorderWeaponInventory(idx, []uint32{0x80800001})
		if err == nil || !strings.Contains(err.Error(), "invalid character index") {
			t.Fatalf("idx=%d: want 'invalid character index', got %v", idx, err)
		}
	}
}

func TestReorderWeaponInventory_DuplicateHandle(t *testing.T) {
	app := inventoryOrderFixture(testWeapons)
	handles := []uint32{0x80800001, 0x80800002, 0x80800001, 0x80800004} // 0x80800001 twice
	err := app.ReorderWeaponInventory(0, handles)
	if err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("want 'duplicate' error, got %v", err)
	}
}

func TestReorderWeaponInventory_MissingHandle(t *testing.T) {
	app := inventoryOrderFixture(testWeapons)
	// Replace one handle with a handle that exists in GaMap but not in CommonItems.
	app.save.Slots[0].GaMap[0x80800099] = 0x000F4240
	handles := []uint32{0x80800001, 0x80800002, 0x80800003, 0x80800099}
	err := app.ReorderWeaponInventory(0, handles)
	if err == nil || !strings.Contains(err.Error(), "not found in weapon inventory") {
		t.Fatalf("want 'not found' error, got %v", err)
	}
}

func TestReorderWeaponInventory_UnarmedHandle(t *testing.T) {
	weapons := append([]testInvWeapon{unarmedEntry}, testWeapons...)
	app := inventoryOrderFixture(weapons)
	// Include Unarmed handle in the list (the rest are valid but count is also wrong — Unarmed excluded from total).
	handles := []uint32{unarmedEntry.handle, 0x80800001, 0x80800002, 0x80800003, 0x80800004}
	err := app.ReorderWeaponInventory(0, handles)
	if err == nil || !strings.Contains(err.Error(), "technical placeholder") {
		t.Fatalf("want 'technical placeholder' error, got %v", err)
	}
}

func TestReorderWeaponInventory_IncompleteList(t *testing.T) {
	app := inventoryOrderFixture(testWeapons) // 4 weapons
	handles := []uint32{0x80800001, 0x80800002, 0x80800003} // only 3
	err := app.ReorderWeaponInventory(0, handles)
	if err == nil || !strings.Contains(err.Error(), "4") {
		t.Fatalf("want error mentioning 4 weapons, got %v", err)
	}
}

func TestReorderWeaponInventory_HappyPath(t *testing.T) {
	app := inventoryOrderFixture(testWeapons)
	slot := &app.save.Slots[0]

	// Reverse the order: [4, 3, 2, 1].
	reversed := []uint32{0x80800004, 0x80800003, 0x80800002, 0x80800001}
	if err := app.ReorderWeaponInventory(0, reversed); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Expected base: NextAcquisitionSortId was 2007 → base = 2007 (raw next index, > 432).
	base := uint32(2007)
	startOff := slot.MagicOffset + core.InvStartFromMagic

	// Build expected: handle → expectedNewIndex (position in reversed slice + base)
	expectedIdx := map[uint32]uint32{
		0x80800004: base,
		0x80800003: base + 1,
		0x80800002: base + 2,
		0x80800001: base + 3,
	}

	// Verify slot.Data at each weapon's original position.
	for i, w := range testWeapons {
		off := startOff + i*core.InvRecordLen
		gotIdx := binary.LittleEndian.Uint32(slot.Data[off+8:])
		wantIdx := expectedIdx[w.handle]
		if gotIdx != wantIdx {
			t.Errorf("weapon 0x%08X: slot.Data index want %d got %d", w.handle, wantIdx, gotIdx)
		}
	}

	// Verify in-memory CommonItems are updated.
	for _, item := range slot.Inventory.CommonItems {
		want, ok := expectedIdx[item.GaItemHandle]
		if !ok {
			continue
		}
		if item.Index != want {
			t.Errorf("CommonItems[handle=0x%08X]: index want %d got %d", item.GaItemHandle, want, item.Index)
		}
	}

	// Verify NextAcquisitionSortId = topIdx + 1 = (base + 3) + 1 = 2011.
	wantNextAcq := base + 3 + 1
	if slot.Inventory.NextAcquisitionSortId != wantNextAcq {
		t.Errorf("NextAcquisitionSortId want %d got %d", wantNextAcq, slot.Inventory.NextAcquisitionSortId)
	}

	// Verify NextEquipIndex >= NextAcquisitionSortId.
	if slot.Inventory.NextEquipIndex < slot.Inventory.NextAcquisitionSortId {
		t.Errorf("NextEquipIndex %d < NextAcquisitionSortId %d", slot.Inventory.NextEquipIndex, slot.Inventory.NextAcquisitionSortId)
	}
}

func TestReorderWeaponInventory_DoesNotTouchGaItems(t *testing.T) {
	app := inventoryOrderFixture(testWeapons)
	slot := &app.save.Slots[0]

	// Pre-populate a fake GaItems slice to verify it is untouched.
	slot.GaItems = []core.GaItemFull{
		{Handle: 0x80800001, ItemID: 0x000F4240, Unk2: -1, Unk3: -1, AoWGaItemHandle: 0xFFFFFFFF},
		{Handle: 0x80800002, ItemID: 0x003085E0, Unk2: -1, Unk3: -1, AoWGaItemHandle: 0xFFFFFFFF},
	}
	origGaItems := make([]core.GaItemFull, len(slot.GaItems))
	copy(origGaItems, slot.GaItems)

	reversed := []uint32{0x80800004, 0x80800003, 0x80800002, 0x80800001}
	if err := app.ReorderWeaponInventory(0, reversed); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// GaItems must be identical.
	if len(slot.GaItems) != len(origGaItems) {
		t.Fatalf("GaItems length changed: %d → %d", len(origGaItems), len(slot.GaItems))
	}
	for i, orig := range origGaItems {
		got := slot.GaItems[i]
		if got.Handle != orig.Handle || got.ItemID != orig.ItemID || got.AoWGaItemHandle != orig.AoWGaItemHandle {
			t.Errorf("GaItems[%d] modified: was %+v, got %+v", i, orig, got)
		}
	}

	// Handles and quantities in slot.Data must be unchanged.
	startOff := slot.MagicOffset + core.InvStartFromMagic
	for i, w := range testWeapons {
		off := startOff + i*core.InvRecordLen
		gotHandle := binary.LittleEndian.Uint32(slot.Data[off:])
		gotQty := binary.LittleEndian.Uint32(slot.Data[off+4:])
		if gotHandle != w.handle {
			t.Errorf("slot.Data handle changed at pos %d: want 0x%08X got 0x%08X", i, w.handle, gotHandle)
		}
		if gotQty != 1 {
			t.Errorf("slot.Data qty changed at pos %d: want 1 got %d", i, gotQty)
		}
	}
}

func TestReorderWeaponInventory_StorageHandle(t *testing.T) {
	app := inventoryOrderFixture(testWeapons)
	// Add a handle to GaMap that is NOT in slot.Data's CommonItems section.
	storageHandle := uint32(0x80800099)
	app.save.Slots[0].GaMap[storageHandle] = 0x000F4240

	// Replace one valid handle with the storage handle.
	handles := []uint32{storageHandle, 0x80800002, 0x80800003, 0x80800004}
	err := app.ReorderWeaponInventory(0, handles)
	if err == nil || !strings.Contains(err.Error(), "not found in weapon inventory") {
		t.Fatalf("want 'not found in weapon inventory', got %v", err)
	}
}

// TestReorderWeaponInventory_NextItemSortsAfter verifies that after reordering,
// NextAcquisitionSortId (the raw index writer.go assigns to the next new item) is
// strictly greater than every index written by the reorder. This guarantees a newly
// added item will sort after all reordered weapons in "Acquisition Order" view.
func TestReorderWeaponInventory_NextItemSortsAfter(t *testing.T) {
	app := inventoryOrderFixture(testWeapons)
	slot := &app.save.Slots[0]

	reversed := []uint32{0x80800004, 0x80800003, 0x80800002, 0x80800001}
	if err := app.ReorderWeaponInventory(0, reversed); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Find the highest index written to slot.Data for any reordered weapon.
	startOff := slot.MagicOffset + core.InvStartFromMagic
	var maxReorderedIdx uint32
	for i := range testWeapons {
		off := startOff + i*core.InvRecordLen
		idx := binary.LittleEndian.Uint32(slot.Data[off+8:])
		if idx > maxReorderedIdx {
			maxReorderedIdx = idx
		}
	}

	// writer.go uses NextAcquisitionSortId directly as the next item's Index.
	nextIdx := slot.Inventory.NextAcquisitionSortId
	if nextIdx <= maxReorderedIdx {
		t.Errorf("NextAcquisitionSortId=%d is not > maxReorderedIdx=%d; "+
			"a new item added via AddItemsToSlot would sort before a reordered weapon",
			nextIdx, maxReorderedIdx)
	}
}
