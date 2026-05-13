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

// ─── Generic inventory fixture (non-weapon items) ────────────────────────────

type testInvItem struct {
	handle uint32
	itemID uint32
	acqIdx uint32
}

// inventoryItemFixture is structurally identical to inventoryOrderFixture but
// accepts testInvItem so that non-weapon tabs (talismans, armor) can be tested.
func inventoryItemFixture(items []testInvItem) *App {
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
	for i, item := range items {
		off := startOff + i*core.InvRecordLen
		binary.LittleEndian.PutUint32(slot.Data[off:], item.handle)
		binary.LittleEndian.PutUint32(slot.Data[off+4:], 1)
		binary.LittleEndian.PutUint32(slot.Data[off+8:], item.acqIdx)
		slot.GaMap[item.handle] = item.itemID
		slot.Inventory.CommonItems = append(slot.Inventory.CommonItems, core.InventoryItem{
			GaItemHandle: item.handle,
			Quantity:     1,
			Index:        item.acqIdx,
		})
		if item.acqIdx > maxAcq {
			maxAcq = item.acqIdx
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

// testTalismans: 0xA0 GaItem handles, DB key = 0x20XXXXXX (converted by GetItemDataFuzzy).
//
//	0xA00003E8 → 0x200003E8 = Crimson Amber Medallion (talismans)
//	0xA00003F2 → 0x200003F2 = Cerulean Amber Medallion (talismans)
var testTalismans = []testInvItem{
	{0xA00003E8, 0xA00003E8, 500},
	{0xA00003F2, 0xA00003F2, 502},
}

// testHelms: 0x90 GaItem handles, GaMap value = 0x10 DB item ID.
//
//	0x100249F0 = Iron Kasa (head)
//	0x1003D090 = Nomadic Merchant's Chapeau (head)
var testHelms = []testInvItem{
	{0x900249F0, 0x100249F0, 600},
	{0x9003D090, 0x1003D090, 602},
}

// ─── GetInventoryOrder — generic tab tests ────────────────────────────────────

func TestGetInventoryOrder_UnknownTab(t *testing.T) {
	app := inventoryItemFixture(testTalismans)
	_, err := app.GetInventoryOrder(0, "unknown_tab")
	if err == nil || !strings.Contains(err.Error(), "unknown sort order tab") {
		t.Fatalf("GetInventoryOrder: want 'unknown sort order tab', got %v", err)
	}
	err = app.ReorderInventory(0, "unknown_tab", []uint32{testTalismans[0].handle})
	if err == nil || !strings.Contains(err.Error(), "unknown sort order tab") {
		t.Fatalf("ReorderInventory: want 'unknown sort order tab', got %v", err)
	}
}

func TestGetInventoryOrder_Talismans_Items(t *testing.T) {
	app := inventoryItemFixture(testTalismans)
	items, err := app.GetInventoryOrder(0, "talismans")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != len(testTalismans) {
		t.Fatalf("want %d talismans, got %d", len(testTalismans), len(items))
	}
	for _, item := range items {
		if item.Category != "talismans" {
			t.Errorf("unexpected category %q, want 'talismans'", item.Category)
		}
		if item.Name == "" {
			t.Errorf("talisman name empty: %+v", item)
		}
		if item.CurrentUpgrade != 0 || item.InfusionName != "" {
			t.Errorf("talismans must have no upgrade/infusion: %+v", item)
		}
	}
	if items[0].AcquisitionIndex > items[1].AcquisitionIndex {
		t.Errorf("items not sorted ascending: %d > %d", items[0].AcquisitionIndex, items[1].AcquisitionIndex)
	}
}

func TestGetInventoryOrder_Head_Items(t *testing.T) {
	app := inventoryItemFixture(testHelms)
	items, err := app.GetInventoryOrder(0, "head")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != len(testHelms) {
		t.Fatalf("want %d helm items, got %d", len(testHelms), len(items))
	}
	for _, item := range items {
		if item.Category != "head" {
			t.Errorf("unexpected category %q, want 'head'", item.Category)
		}
		if item.Name == "" {
			t.Errorf("head item name empty: %+v", item)
		}
	}
	if items[0].AcquisitionIndex > items[1].AcquisitionIndex {
		t.Errorf("items not sorted ascending: %d > %d", items[0].AcquisitionIndex, items[1].AcquisitionIndex)
	}
}

// ─── ReorderInventory — generic tab tests ────────────────────────────────────

func TestReorderInventory_Talismans_HappyPath(t *testing.T) {
	app := inventoryItemFixture(testTalismans)
	slot := &app.save.Slots[0]

	beforeBase := slot.Inventory.NextAcquisitionSortId
	reversed := []uint32{testTalismans[1].handle, testTalismans[0].handle}
	if err := app.ReorderInventory(0, "talismans", reversed); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	startOff := slot.MagicOffset + core.InvStartFromMagic
	expectedIdx := map[uint32]uint32{
		testTalismans[1].handle: beforeBase,
		testTalismans[0].handle: beforeBase + 1,
	}
	for i, item := range testTalismans {
		off := startOff + i*core.InvRecordLen
		got := binary.LittleEndian.Uint32(slot.Data[off+8:])
		want := expectedIdx[item.handle]
		if got != want {
			t.Errorf("talisman[%d] 0x%08X: want idx %d got %d", i, item.handle, want, got)
		}
	}

	wantNextAcq := beforeBase + uint32(len(testTalismans))
	if slot.Inventory.NextAcquisitionSortId != wantNextAcq {
		t.Errorf("NextAcquisitionSortId want %d got %d", wantNextAcq, slot.Inventory.NextAcquisitionSortId)
	}
}

func TestReorderInventory_HeadOrChest_HappyPath(t *testing.T) {
	app := inventoryItemFixture(testHelms)
	slot := &app.save.Slots[0]

	beforeBase := slot.Inventory.NextAcquisitionSortId
	reversed := []uint32{testHelms[1].handle, testHelms[0].handle}
	if err := app.ReorderInventory(0, "head", reversed); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	startOff := slot.MagicOffset + core.InvStartFromMagic
	expectedIdx := map[uint32]uint32{
		testHelms[1].handle: beforeBase,
		testHelms[0].handle: beforeBase + 1,
	}
	for i, item := range testHelms {
		off := startOff + i*core.InvRecordLen
		got := binary.LittleEndian.Uint32(slot.Data[off+8:])
		want := expectedIdx[item.handle]
		if got != want {
			t.Errorf("helm[%d] 0x%08X: want idx %d got %d", i, item.handle, want, got)
		}
	}

	wantNextAcq := beforeBase + uint32(len(testHelms))
	if slot.Inventory.NextAcquisitionSortId != wantNextAcq {
		t.Errorf("NextAcquisitionSortId want %d got %d", wantNextAcq, slot.Inventory.NextAcquisitionSortId)
	}
}

func TestReorderInventory_WrongTabHandle_Blocks(t *testing.T) {
	// Head armor handles passed to "talismans" tab must be rejected.
	app := inventoryItemFixture(testHelms)
	handles := []uint32{testHelms[0].handle, testHelms[1].handle}
	err := app.ReorderInventory(0, "talismans", handles)
	if err == nil || !strings.Contains(err.Error(), "does not belong to sort order tab") {
		t.Fatalf("want 'does not belong to sort order tab', got %v", err)
	}
}

func TestReorderInventory_IncompleteList_Talismans(t *testing.T) {
	app := inventoryItemFixture(testTalismans) // 2 talismans
	handles := []uint32{testTalismans[0].handle} // only 1
	err := app.ReorderInventory(0, "talismans", handles)
	if err == nil || !strings.Contains(err.Error(), "2") {
		t.Fatalf("want error mentioning 2, got %v", err)
	}
}

func TestReorderInventory_Duplicate_Talismans(t *testing.T) {
	app := inventoryItemFixture(testTalismans)
	handles := []uint32{testTalismans[0].handle, testTalismans[0].handle}
	err := app.ReorderInventory(0, "talismans", handles)
	if err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("want 'duplicate' error, got %v", err)
	}
}

// ─── Wrapper delegation tests ─────────────────────────────────────────────────

func TestGetWeaponInventoryOrder_WrapperWorks(t *testing.T) {
	app := inventoryOrderFixture(testWeapons)
	viaGeneric, err := app.GetInventoryOrder(0, "weapons")
	if err != nil {
		t.Fatalf("GetInventoryOrder: %v", err)
	}
	viaWrapper, err := app.GetWeaponInventoryOrder(0)
	if err != nil {
		t.Fatalf("GetWeaponInventoryOrder: %v", err)
	}
	if len(viaGeneric) != len(viaWrapper) {
		t.Fatalf("length mismatch: generic=%d wrapper=%d", len(viaGeneric), len(viaWrapper))
	}
	for i := range viaGeneric {
		if viaGeneric[i].Handle != viaWrapper[i].Handle || viaGeneric[i].AcquisitionIndex != viaWrapper[i].AcquisitionIndex {
			t.Errorf("item[%d] mismatch: generic=%+v wrapper=%+v", i, viaGeneric[i], viaWrapper[i])
		}
	}
}

func TestReorderWeaponInventory_WrapperWorks(t *testing.T) {
	app := inventoryOrderFixture(testWeapons)
	reversed := []uint32{0x80800004, 0x80800003, 0x80800002, 0x80800001}
	if err := app.ReorderWeaponInventory(0, reversed); err != nil {
		t.Fatalf("ReorderWeaponInventory: %v", err)
	}
	// After reversing, handle 0x80800004 must sort first (lowest new index).
	items, err := app.GetInventoryOrder(0, "weapons")
	if err != nil {
		t.Fatalf("GetInventoryOrder: %v", err)
	}
	if items[0].Handle != 0x80800004 {
		t.Errorf("want first handle 0x80800004, got 0x%08X", items[0].Handle)
	}
}

// ─── Weight field ─────────────────────────────────────────────────────────────

func TestGetInventoryOrder_WeaponWeight(t *testing.T) {
	// Dagger 0x000F4240 → data.ItemWeights[0x000F4240] ≈ 1.5
	weapons := []testInvWeapon{{0x80800001, 0x000F4240, 2000}}
	app := inventoryOrderFixture(weapons)
	items, err := app.GetInventoryOrder(0, "weapons")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("want 1 item, got %d", len(items))
	}
	if items[0].Weight <= 0 {
		t.Errorf("Dagger: want Weight > 0, got %g", items[0].Weight)
	}
}

func TestGetInventoryOrder_TalismanWeight(t *testing.T) {
	// Crimson Amber Medallion: GaItem 0xA00003E8, DB key 0x200003E8, weight ≈ 0.3
	app := inventoryItemFixture([]testInvItem{{0xA00003E8, 0xA00003E8, 500}})
	items, err := app.GetInventoryOrder(0, "talismans")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("want 1 item, got %d", len(items))
	}
	if items[0].Weight <= 0 {
		t.Errorf("Crimson Amber Medallion: want Weight > 0, got %g", items[0].Weight)
	}
}

func TestGetInventoryOrder_HeadArmorWeight(t *testing.T) {
	// Iron Kasa: DB key 0x100249F0
	app := inventoryItemFixture([]testInvItem{{0x900249F0, 0x100249F0, 600}})
	items, err := app.GetInventoryOrder(0, "head")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("want 1 item, got %d", len(items))
	}
	if items[0].Weight <= 0 {
		t.Errorf("Iron Kasa: want Weight > 0, got %g", items[0].Weight)
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
