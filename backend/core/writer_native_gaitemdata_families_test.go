package core

import (
	"encoding/binary"
	"testing"
)

// Real item IDs confirmed by item-save-lab (tmp/item-save-lab/WYNIKI_I_IMPLEMENTACJA.md).
const (
	testThrowingDaggerID = uint32(0x400006A4) // T050/T051 — goods, new + existing stack
	testTrickMirrorID    = uint32(0x200017CA) // T040 — Host's Trick-Mirror, talisman
	testCookbook1ID      = uint32(0x40002454) // T071 — Nomadic Warrior's Cookbook [1]
	testCookbook2ID      = uint32(0x4000245F) // T071 — Nomadic Warrior's Cookbook [2]
	testCrackedPotID     = uint32(0x4000251C) // T074 — Cracked Pot, container key item
	testBoltID           = uint32(0x03197500) // T064 — Bolt, second ammo family
)

// --- Part 1: CheckAddCapacity per-destination planning for addKindArrow ---
//
// Regression coverage for the bug described in the task: an arrow/bolt ID
// that already has a GaItem+handle somewhere (Inventory OR Storage) was
// treated as "exists" everywhere, so a request to add it to the ONE
// destination it doesn't yet occupy reported NeededInv/NeededStorage=0 even
// though AddItemsToSlotBatch goes on to create a real new physical record
// there. CheckAddCapacity must plan per destination, while still counting at
// most one GaItem/GaItemData per genuinely new ID.

// TestCheckAddCapacity_ArrowExistsInInventory_AddToStorageOnly is case 1:
// Arrow already owned in Inventory, not in Storage — adding to Storage only
// must report NeededStorage=1 and NeededGaItems=0 (the GaItem already
// exists; only a new physical Storage record is needed).
func TestCheckAddCapacity_ArrowExistsInInventory_AddToStorageOnly(t *testing.T) {
	slot := buildInvStorageFixture(t)
	const arrowHandle = uint32(ItemTypeWeapon | 0x000042)
	slot.GaMap[arrowHandle] = testArrowID
	slot.Inventory.CommonItems[0] = InventoryItem{GaItemHandle: arrowHandle, Quantity: 3, Index: 969}

	report := CheckAddCapacity(slot, []ItemToAdd{{ItemID: testArrowID, StorageQty: 2, ForceStackable: true}})
	if !report.CanFitAll {
		t.Fatalf("CanFitAll=false, CapHit=%q; want fit", report.CapHit)
	}
	if report.NeededStorage != 1 {
		t.Errorf("NeededStorage=%d, want 1 (arrow not yet in Storage — this is the T211-style bug: it used to report 0)", report.NeededStorage)
	}
	if report.NeededInv != 0 {
		t.Errorf("NeededInv=%d, want 0 (Inventory not requested)", report.NeededInv)
	}
	if report.NeededGaItems != 0 {
		t.Errorf("NeededGaItems=%d, want 0 (arrow ID already has a GaItem, shared across destinations)", report.NeededGaItems)
	}
	if report.NeededGaItemData != 0 {
		t.Errorf("NeededGaItemData=%d, want 0 (arrow ID already has an active GaItemData entry)", report.NeededGaItemData)
	}
}

// TestCheckAddCapacity_ArrowExistsInStorage_AddToInventoryOnly is case 2, the
// mirror of case 1: Arrow already owned in Storage, not in Inventory.
func TestCheckAddCapacity_ArrowExistsInStorage_AddToInventoryOnly(t *testing.T) {
	slot := buildInvStorageFixture(t)
	const arrowHandle = uint32(ItemTypeWeapon | 0x000043)
	slot.GaMap[arrowHandle] = testArrowID
	storageStart := slot.StorageBoxOffset + StorageHeaderSkip
	binary.LittleEndian.PutUint32(slot.Data[storageStart:], arrowHandle)
	binary.LittleEndian.PutUint32(slot.Data[storageStart+4:], 5)
	binary.LittleEndian.PutUint32(slot.Data[storageStart+8:], 969)

	report := CheckAddCapacity(slot, []ItemToAdd{{ItemID: testArrowID, InvQty: 4, ForceStackable: true}})
	if !report.CanFitAll {
		t.Fatalf("CanFitAll=false, CapHit=%q; want fit", report.CapHit)
	}
	if report.NeededInv != 1 {
		t.Errorf("NeededInv=%d, want 1 (arrow not yet in Inventory)", report.NeededInv)
	}
	if report.NeededStorage != 0 {
		t.Errorf("NeededStorage=%d, want 0 (Storage not requested)", report.NeededStorage)
	}
	if report.NeededGaItems != 0 {
		t.Errorf("NeededGaItems=%d, want 0 (arrow ID already has a GaItem)", report.NeededGaItems)
	}
}

// TestCheckAddCapacity_TwoEntriesSameNewArrowInBatch is case 3: a batch with
// two separate ItemToAdd entries for the SAME brand-new arrow ID (one per
// destination) must count exactly one GaItem/GaItemData for the ID, plus one
// Inventory slot and one Storage slot — never double-counted.
func TestCheckAddCapacity_TwoEntriesSameNewArrowInBatch(t *testing.T) {
	slot := buildInvStorageFixture(t)
	slot.GaItems = make([]GaItemFull, 10) // room for the one new arrow GaItem record

	report := CheckAddCapacity(slot, []ItemToAdd{
		{ItemID: testArrowID, InvQty: 2, ForceStackable: true},
		{ItemID: testArrowID, StorageQty: 3, ForceStackable: true},
	})
	if !report.CanFitAll {
		t.Fatalf("CanFitAll=false, CapHit=%q; want fit", report.CapHit)
	}
	if report.NeededGaItems != 1 {
		t.Errorf("NeededGaItems=%d, want 1 (one new ID, not two)", report.NeededGaItems)
	}
	if report.NeededGaItemData != 1 {
		t.Errorf("NeededGaItemData=%d, want 1", report.NeededGaItemData)
	}
	if report.NeededInv != 1 {
		t.Errorf("NeededInv=%d, want 1", report.NeededInv)
	}
	if report.NeededStorage != 1 {
		t.Errorf("NeededStorage=%d, want 1", report.NeededStorage)
	}
}

// TestCheckAddCapacity_ArrowAndBoltAsTwoNewIDsInBatch is case 4: Arrow and
// Bolt are two different new ammo IDs (T064) — each needs its own GaItem,
// GaItemData, and physical record; neither may be folded into the other's
// count.
func TestCheckAddCapacity_ArrowAndBoltAsTwoNewIDsInBatch(t *testing.T) {
	slot := buildInvStorageFixture(t)
	slot.GaItems = make([]GaItemFull, 10) // room for two new arrow-family GaItem records

	report := CheckAddCapacity(slot, []ItemToAdd{
		{ItemID: testArrowID, InvQty: 1, ForceStackable: true},
		{ItemID: testBoltID, InvQty: 1, ForceStackable: true},
	})
	if !report.CanFitAll {
		t.Fatalf("CanFitAll=false, CapHit=%q; want fit", report.CapHit)
	}
	if report.NeededGaItems != 2 {
		t.Errorf("NeededGaItems=%d, want 2 (Arrow and Bolt are distinct IDs)", report.NeededGaItems)
	}
	if report.NeededGaItemData != 2 {
		t.Errorf("NeededGaItemData=%d, want 2", report.NeededGaItemData)
	}
	if report.NeededInv != 2 {
		t.Errorf("NeededInv=%d, want 2", report.NeededInv)
	}
}

// --- Part 2: broad GaItemData contract for confirmed native families ---
//
// T040 (talismans), T050/T060/T062 (goods/crafting/bolstering materials),
// T070/T071/T074/T090 (Key Items, cookbooks, containers, Physick pieces,
// Sacred Tear) all show the same pattern: the FIRST physical record of a new
// item ID also gets exactly one active GaItemData entry with flag 1, no
// serialized GaItem, and a quantity bump to an already-existing record never
// creates a second GaItemData entry. All of these item IDs sit in the same
// ascending "ordinary" GaItemData segment as weapons (itemID top nibble !=
// 8), which is why they reuse upsertOrdinaryGaItemData (== the existing,
// already-native-order-confirmed upsertWeaponGaItemData) instead of a new
// insertion algorithm.
//
// NOTE: goods/crafting/bolstering materials and talismans land in
// Inventory.CommonItems, matching T050/T060/T062/T040. The narrower,
// individually confirmed sub-family — Crafting Kit, the cookbook family,
// Cracked Pot, and the Physick package's Crimson Crystal Tear variant — now
// routes to Inventory.KeyItems instead (see item_add_plan.go's
// nativeKeyItemFamily): T070/T071/T074/T090 directly show the native game
// placing exactly these IDs there. AddItemsToSlot's legacy single-item
// callers (container auto-sync, flag-unlock helpers) opt out of this via
// ItemToAdd.ForceCommonItems — see writer_native_gaitemdata_families_test.go
// and AddItemsToSlot's doc comment.

func newRoundTripFixture(t *testing.T) *SaveSlot {
	t.Helper()
	return fragmentedGaItemRoundTripFixtureForVersion(t, GaItemVersionBreak+1).Slot
}

func findCommonItemHandle(slot *SaveSlot, itemID uint32) (uint32, bool) {
	handle := (itemID & 0x0FFFFFFF) | ItemTypeItem
	for _, inv := range slot.Inventory.CommonItems {
		if inv.GaItemHandle == handle {
			return handle, true
		}
	}
	return 0, false
}

func findKeyItemHandle(slot *SaveSlot, itemID uint32) (uint32, bool) {
	handle := (itemID & 0x0FFFFFFF) | ItemTypeItem
	for _, ki := range slot.Inventory.KeyItems {
		if ki.GaItemHandle == handle {
			return handle, true
		}
	}
	return 0, false
}

// TestAddItemsToSlotBatch_NewThrowingDaggerGetsGaItemDataNoGaItem is the
// T050 contract: a brand-new goods stack gets exactly one active GaItemData
// entry (flag 1) and no serialized GaItem record.
func TestAddItemsToSlotBatch_NewThrowingDaggerGetsGaItemDataNoGaItem(t *testing.T) {
	slot := newRoundTripFixture(t)
	before := activeGaItemDataRecords(t, slot)

	if err := AddItemsToSlotBatch(slot, []ItemToAdd{{ItemID: testThrowingDaggerID, InvQty: 1}}); err != nil {
		t.Fatalf("AddItemsToSlotBatch: %v", err)
	}

	if _, ok := findCommonItemHandle(slot, testThrowingDaggerID); !ok {
		t.Fatal("no CommonItems record created for Throwing Dagger")
	}
	for _, ga := range slot.GaItems {
		if !ga.IsEmpty() && ga.ItemID == testThrowingDaggerID {
			t.Errorf("Throwing Dagger 0x%08X must not have a GaItem record", testThrowingDaggerID)
		}
	}

	after := activeGaItemDataRecords(t, slot)
	if len(after) != len(before)+1 {
		t.Fatalf("active GaItemData count = %d, want %d (+1 for the new stack)", len(after), len(before)+1)
	}
	found := false
	for _, r := range after {
		if r.id == testThrowingDaggerID {
			found = true
			if r.flag != 1 {
				t.Errorf("GaItemData flag = %d, want 1", r.flag)
			}
		}
	}
	if !found {
		t.Fatal("no active GaItemData record for Throwing Dagger")
	}
}

// TestAddItemsToSlotBatch_ExistingThrowingDaggerStackDoesNotDuplicateGaItemData
// is the T051 contract: raising an already-owned goods stack updates quantity
// in place without allocating a second GaItemData record.
func TestAddItemsToSlotBatch_ExistingThrowingDaggerStackDoesNotDuplicateGaItemData(t *testing.T) {
	slot := newRoundTripFixture(t)

	if err := AddItemsToSlotBatch(slot, []ItemToAdd{{ItemID: testThrowingDaggerID, InvQty: 1}}); err != nil {
		t.Fatalf("first add: %v", err)
	}
	countAfterFirst := len(activeGaItemDataRecords(t, slot))

	if err := AddItemsToSlotBatch(slot, []ItemToAdd{{ItemID: testThrowingDaggerID, InvQty: 2}}); err != nil {
		t.Fatalf("second add (existing stack): %v", err)
	}

	if got := len(activeGaItemDataRecords(t, slot)); got != countAfterFirst {
		t.Errorf("active GaItemData count after existing-stack bump = %d, want unchanged %d", got, countAfterFirst)
	}
	handle, ok := findCommonItemHandle(slot, testThrowingDaggerID)
	if !ok {
		t.Fatal("Throwing Dagger record missing after bump")
	}
	for _, inv := range slot.Inventory.CommonItems {
		if inv.GaItemHandle == handle && inv.Quantity != 2 {
			t.Errorf("quantity after bump = %d, want 2 (SET, not additive)", inv.Quantity)
		}
	}
}

// TestAddItemsToSlotBatch_NewTrickMirrorTalismanGetsGaItemDataNoGaItem is the
// T040 contract: a brand-new talisman copy gets one active GaItemData entry
// and no serialized GaItem, same as goods.
func TestAddItemsToSlotBatch_NewTrickMirrorTalismanGetsGaItemDataNoGaItem(t *testing.T) {
	slot := newRoundTripFixture(t)
	before := activeGaItemDataRecords(t, slot)

	if err := AddItemsToSlotBatch(slot, []ItemToAdd{{ItemID: testTrickMirrorID, InvQty: 1}}); err != nil {
		t.Fatalf("AddItemsToSlotBatch: %v", err)
	}

	handle := (testTrickMirrorID & 0x0FFFFFFF) | ItemTypeAccessory
	found := false
	for _, inv := range slot.Inventory.CommonItems {
		if inv.GaItemHandle == handle {
			found = true
		}
	}
	if !found {
		t.Fatal("no CommonItems record created for Host's Trick-Mirror")
	}
	for _, ga := range slot.GaItems {
		if !ga.IsEmpty() && ga.ItemID == testTrickMirrorID {
			t.Errorf("talisman 0x%08X must not have a GaItem record", testTrickMirrorID)
		}
	}

	after := activeGaItemDataRecords(t, slot)
	if len(after) != len(before)+1 {
		t.Fatalf("active GaItemData count = %d, want %d (+1 for the new talisman)", len(after), len(before)+1)
	}
	dataFound := false
	for _, r := range after {
		if r.id == testTrickMirrorID {
			dataFound = true
			if r.flag != 1 {
				t.Errorf("GaItemData flag = %d, want 1", r.flag)
			}
		}
	}
	if !dataFound {
		t.Fatal("no active GaItemData record for Host's Trick-Mirror")
	}
}

// TestAddItemsToSlotBatch_NewCookbookLandsInKeyItemsWithGaItemData is the
// T071 contract in full: a brand-new cookbook creates exactly one
// Inventory.KeyItems record (never CommonItems), no serialized GaItem, and
// one active GaItemData entry with flag 1.
func TestAddItemsToSlotBatch_NewCookbookLandsInKeyItemsWithGaItemData(t *testing.T) {
	slot := newRoundTripFixture(t)
	before := activeGaItemDataRecords(t, slot)

	if err := AddItemsToSlotBatch(slot, []ItemToAdd{{ItemID: testCookbook1ID, InvQty: 1}}); err != nil {
		t.Fatalf("AddItemsToSlotBatch: %v", err)
	}

	if _, ok := findKeyItemHandle(slot, testCookbook1ID); !ok {
		t.Fatal("no KeyItems record created for Cookbook [1]")
	}
	if _, ok := findCommonItemHandle(slot, testCookbook1ID); ok {
		t.Error("Cookbook [1] unexpectedly duplicated into CommonItems")
	}

	after := activeGaItemDataRecords(t, slot)
	if len(after) != len(before)+1 {
		t.Fatalf("active GaItemData count = %d, want %d (+1 for the new cookbook)", len(after), len(before)+1)
	}
	found := false
	for _, r := range after {
		if r.id == testCookbook1ID {
			found = true
			if r.flag != 1 {
				t.Errorf("GaItemData flag = %d, want 1", r.flag)
			}
		}
	}
	if !found {
		t.Fatal("no active GaItemData record for Cookbook [1]")
	}
	for _, ga := range slot.GaItems {
		if !ga.IsEmpty() && ga.ItemID == testCookbook1ID {
			t.Errorf("cookbook 0x%08X must not have a GaItem record", testCookbook1ID)
		}
	}
}

// TestAddItemsToSlotBatch_T071MerchantKaleCookbooks preserves the complete
// two-Cookbook acquisition contract observed in T071. The single-Cookbook test
// above proves the generic family rule; this test additionally locks in the
// second Merchant Kalé acquisition, the 0->2 key_count update, and active
// GaItemData for both distinct IDs in one real batch.
func TestAddItemsToSlotBatch_T071MerchantKaleCookbooks(t *testing.T) {
	slot := newRoundTripFixture(t)
	beforeData := activeGaItemDataRecords(t, slot)
	keyCountOff := slot.MagicOffset + InvStartFromMagic + CommonItemCount*InvRecordLen
	beforeKeyCount := binary.LittleEndian.Uint32(slot.Data[keyCountOff:])
	cookbooks := []uint32{testCookbook1ID, testCookbook2ID}

	items := make([]ItemToAdd, 0, len(cookbooks))
	for _, id := range cookbooks {
		items = append(items, ItemToAdd{ItemID: id, InvQty: 1})
	}
	if err := AddItemsToSlotBatch(slot, items); err != nil {
		t.Fatalf("AddItemsToSlotBatch: %v", err)
	}

	if got := binary.LittleEndian.Uint32(slot.Data[keyCountOff:]); got != beforeKeyCount+uint32(len(cookbooks)) {
		t.Errorf("key_count = %d, want %d", got, beforeKeyCount+uint32(len(cookbooks)))
	}
	afterData := activeGaItemDataRecords(t, slot)
	if got, want := len(afterData), len(beforeData)+len(cookbooks); got != want {
		t.Fatalf("active GaItemData count = %d, want %d", got, want)
	}
	active := make(map[uint32]uint32, len(afterData))
	for _, record := range afterData {
		active[record.id] = record.flag
	}
	for _, id := range cookbooks {
		if _, ok := findKeyItemHandle(slot, id); !ok {
			t.Errorf("Cookbook 0x%08X missing from KeyItems", id)
		}
		if _, ok := findCommonItemHandle(slot, id); ok {
			t.Errorf("Cookbook 0x%08X unexpectedly present in CommonItems", id)
		}
		if got := active[id]; got != 1 {
			t.Errorf("Cookbook 0x%08X GaItemData flag = %d, want 1", id, got)
		}
		for _, ga := range slot.GaItems {
			if !ga.IsEmpty() && ga.ItemID == id {
				t.Errorf("Cookbook 0x%08X unexpectedly has a serialized GaItem", id)
			}
		}
	}
}

// TestAddItemsToSlotBatch_CrackedPotLandsInKeyItemsNoGaItemNoDuplicate is the
// T074 contract in full: first physical creation lands in Inventory.KeyItems
// (never CommonItems) with one active GaItemData entry and no GaItem;
// raising the stack updates the same KeyItems record in place — no second
// GaItemData entry, no duplicate record in either container.
func TestAddItemsToSlotBatch_CrackedPotLandsInKeyItemsNoGaItemNoDuplicate(t *testing.T) {
	slot := newRoundTripFixture(t)
	before := activeGaItemDataRecords(t, slot)

	if err := AddItemsToSlotBatch(slot, []ItemToAdd{{ItemID: testCrackedPotID, InvQty: 1}}); err != nil {
		t.Fatalf("first add: %v", err)
	}
	afterFirst := activeGaItemDataRecords(t, slot)
	if len(afterFirst) != len(before)+1 {
		t.Fatalf("active GaItemData count after first add = %d, want %d (+1)", len(afterFirst), len(before)+1)
	}
	for _, ga := range slot.GaItems {
		if !ga.IsEmpty() && ga.ItemID == testCrackedPotID {
			t.Errorf("Cracked Pot 0x%08X must not have a GaItem record", testCrackedPotID)
		}
	}
	if _, ok := findCommonItemHandle(slot, testCrackedPotID); ok {
		t.Fatal("Cracked Pot created in CommonItems, want KeyItems")
	}

	if err := AddItemsToSlotBatch(slot, []ItemToAdd{{ItemID: testCrackedPotID, InvQty: 2}}); err != nil {
		t.Fatalf("second add (existing stack): %v", err)
	}
	afterSecond := activeGaItemDataRecords(t, slot)
	if len(afterSecond) != len(afterFirst) {
		t.Errorf("active GaItemData count after stack bump = %d, want unchanged %d", len(afterSecond), len(afterFirst))
	}
	if _, ok := findCommonItemHandle(slot, testCrackedPotID); ok {
		t.Fatal("Cracked Pot duplicated into CommonItems after stack bump")
	}

	handle, ok := findKeyItemHandle(slot, testCrackedPotID)
	if !ok {
		t.Fatal("Cracked Pot record missing from KeyItems after bump")
	}
	for _, inv := range slot.Inventory.KeyItems {
		if inv.GaItemHandle == handle && inv.Quantity != 2 {
			t.Errorf("quantity after bump = %d, want 2 (SET, not additive)", inv.Quantity)
		}
	}
}

// --- Direct AddItemsToSlot parity (requirement 1) ---
//
// AddItemsToSlot is now a thin per-id wrapper around AddItemsToSlotBatch, so
// these exercise the legacy API directly for the remaining confirmed
// families (goods, talismans, and — with ForceCommonItems implied — Key
// Items) rather than only trusting the delegation.

// TestAddItemsToSlot_NewThrowingDaggerGetsGaItemDataNoGaItem is the T050
// contract driven through AddItemsToSlot.
func TestAddItemsToSlot_NewThrowingDaggerGetsGaItemDataNoGaItem(t *testing.T) {
	slot := newRoundTripFixture(t)
	before := activeGaItemDataRecords(t, slot)

	if err := AddItemsToSlot(slot, []uint32{testThrowingDaggerID}, 1, 0, false); err != nil {
		t.Fatalf("AddItemsToSlot: %v", err)
	}

	if _, ok := findCommonItemHandle(slot, testThrowingDaggerID); !ok {
		t.Fatal("no CommonItems record created for Throwing Dagger")
	}
	after := activeGaItemDataRecords(t, slot)
	if len(after) != len(before)+1 {
		t.Fatalf("active GaItemData count = %d, want %d (+1)", len(after), len(before)+1)
	}
}

// TestAddItemsToSlot_ExistingThrowingDaggerStackDoesNotDuplicateGaItemData is
// the T051 contract driven through AddItemsToSlot: bumping an already-owned
// goods stack via the legacy API must not create a second GaItemData entry.
func TestAddItemsToSlot_ExistingThrowingDaggerStackDoesNotDuplicateGaItemData(t *testing.T) {
	slot := newRoundTripFixture(t)

	if err := AddItemsToSlot(slot, []uint32{testThrowingDaggerID}, 1, 0, false); err != nil {
		t.Fatalf("first add: %v", err)
	}
	countAfterFirst := len(activeGaItemDataRecords(t, slot))

	if err := AddItemsToSlot(slot, []uint32{testThrowingDaggerID}, 2, 0, false); err != nil {
		t.Fatalf("second add (existing stack): %v", err)
	}
	if got := len(activeGaItemDataRecords(t, slot)); got != countAfterFirst {
		t.Errorf("active GaItemData count after existing-stack bump = %d, want unchanged %d", got, countAfterFirst)
	}
}

// TestAddItemsToSlot_NewTrickMirrorTalismanGetsGaItemDataNoGaItem is the T040
// contract driven through AddItemsToSlot.
func TestAddItemsToSlot_NewTrickMirrorTalismanGetsGaItemDataNoGaItem(t *testing.T) {
	slot := newRoundTripFixture(t)
	before := activeGaItemDataRecords(t, slot)

	if err := AddItemsToSlot(slot, []uint32{testTrickMirrorID}, 1, 0, false); err != nil {
		t.Fatalf("AddItemsToSlot: %v", err)
	}

	handle := (testTrickMirrorID & 0x0FFFFFFF) | ItemTypeAccessory
	found := false
	for _, inv := range slot.Inventory.CommonItems {
		if inv.GaItemHandle == handle {
			found = true
		}
	}
	if !found {
		t.Fatal("no CommonItems record created for Host's Trick-Mirror")
	}
	after := activeGaItemDataRecords(t, slot)
	if len(after) != len(before)+1 {
		t.Fatalf("active GaItemData count = %d, want %d (+1)", len(after), len(before)+1)
	}
}

// TestAddItemsToSlot_CookbookForcesCommonItemsButStillGetsGaItemData is
// requirement 1's actual point for AddItemsToSlot: the legacy API keeps its
// established CommonItems placement for every existing caller
// (ForceCommonItems is implied — see AddItemsToSlot's doc comment) instead of
// the native KeyItems routing AddItemsToSlotBatch applies for a direct pick,
// but it must still create the GaItemData entry T071 shows the native game
// creates — identically to AddItemsToSlotBatch.
func TestAddItemsToSlot_CookbookForcesCommonItemsButStillGetsGaItemData(t *testing.T) {
	slot := newRoundTripFixture(t)
	before := activeGaItemDataRecords(t, slot)

	if err := AddItemsToSlot(slot, []uint32{testCookbook1ID}, 1, 0, false); err != nil {
		t.Fatalf("AddItemsToSlot: %v", err)
	}

	if _, ok := findCommonItemHandle(slot, testCookbook1ID); !ok {
		t.Fatal("AddItemsToSlot must keep placing the cookbook in CommonItems (ForceCommonItems)")
	}
	if _, ok := findKeyItemHandle(slot, testCookbook1ID); ok {
		t.Error("cookbook unexpectedly routed to KeyItems through AddItemsToSlot")
	}
	after := activeGaItemDataRecords(t, slot)
	if len(after) != len(before)+1 {
		t.Fatalf("active GaItemData count = %d, want %d (+1, same contract as AddItemsToSlotBatch)", len(after), len(before)+1)
	}
}

// TestAddItemsToSlot_CrackedPotBumpDoesNotDuplicateGaItemData is requirement
// 1's "bump must not create a second GaItemData" guarantee, driven through
// AddItemsToSlot for a confirmed Key Items family member.
func TestAddItemsToSlot_CrackedPotBumpDoesNotDuplicateGaItemData(t *testing.T) {
	slot := newRoundTripFixture(t)

	if err := AddItemsToSlot(slot, []uint32{testCrackedPotID}, 1, 0, false); err != nil {
		t.Fatalf("first add: %v", err)
	}
	countAfterFirst := len(activeGaItemDataRecords(t, slot))

	if err := AddItemsToSlot(slot, []uint32{testCrackedPotID}, 2, 0, false); err != nil {
		t.Fatalf("second add (existing stack): %v", err)
	}
	if got := len(activeGaItemDataRecords(t, slot)); got != countAfterFirst {
		t.Errorf("active GaItemData count after existing-stack bump = %d, want unchanged %d", got, countAfterFirst)
	}
}
