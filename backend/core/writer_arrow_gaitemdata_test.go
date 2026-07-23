package core

import "testing"

// testArrowID is the real "Arrow" item ID from the item-save-lab evidence
// (T063/T064/T211): weapon-prefixed (0x00...), decimal 50000000, handle
// prefix ItemTypeWeapon once allocated. Same constant used by
// backend/editor/workspace_dedup_test.go.
const testArrowID = uint32(0x02FAF080)

func countNonEmptyGaItems(slot *SaveSlot) int {
	n := 0
	for i := range slot.GaItems {
		if !slot.GaItems[i].IsEmpty() {
			n++
		}
	}
	return n
}

// TestAllocateGaItem_WeaponAndArrowDefaultUnk2Unk3Zero locks in the native
// GaItem contract confirmed by T020 (armor Chain Coif/Chain Armor) and
// T063/T211 (Arrow): weapon- and armor-shaped records default Unk2=Unk3=0,
// not the app's old unconditional -1/-1.
func TestAllocateGaItem_WeaponAndArrowDefaultUnk2Unk3Zero(t *testing.T) {
	slot := makeTestSlot(10)

	weaponHandle := uint32(ItemTypeWeapon | gaItemHandleValidBit | 0x000001)
	if err := allocateGaItem(slot, weaponHandle, testArrowID); err != nil {
		t.Fatalf("allocateGaItem (arrow/weapon): %v", err)
	}
	got := slot.GaItems[1]
	if got.Unk2 != 0 || got.Unk3 != 0 {
		t.Errorf("weapon-type GaItem Unk2/Unk3 = %d/%d, want 0/0 (T020, T063/T211 native evidence)", got.Unk2, got.Unk3)
	}

	armorHandle := uint32(ItemTypeArmor | gaItemHandleValidBit | 0x000002)
	if err := allocateGaItem(slot, armorHandle, 0x10000001); err != nil {
		t.Fatalf("allocateGaItem (armor): %v", err)
	}
	got = slot.GaItems[2]
	if got.Unk2 != 0 || got.Unk3 != 0 {
		t.Errorf("armor GaItem Unk2/Unk3 = %d/%d, want 0/0 (T020 native evidence)", got.Unk2, got.Unk3)
	}
}

// TestAllocateGaItem_AoWKeepsUnk2Unk3NegativeOne documents that AoW entries
// are left at the pre-fix -1/-1 default. It is harmless either way: an AoW
// item ID serializes as an 8-byte GaRecordItem (Handle+ItemID only), so
// Unk2/Unk3 are never written to disk for this record type.
func TestAllocateGaItem_AoWKeepsUnk2Unk3NegativeOne(t *testing.T) {
	slot := makeTestSlot(10)
	aowHandle := uint32(ItemTypeAow | gaItemHandleValidBit | 0x000001)
	if err := allocateGaItem(slot, aowHandle, 0x80000001); err != nil {
		t.Fatalf("allocateGaItem (AoW): %v", err)
	}
	got := slot.GaItems[0]
	if got.Unk2 != -1 || got.Unk3 != -1 {
		t.Errorf("AoW GaItem Unk2/Unk3 = %d/%d, want -1/-1 (unchanged; not serialized for this record type)", got.Unk2, got.Unk3)
	}
}

// TestAddItemsToSlotBatch_NewArrowGetsGaItemAndActiveGaItemData is the T211
// regression: a brand new Arrow stack must get the full native trio —
// CommonItems record, serialized GaItem (weapon-prefixed, Unk2=Unk3=0), and
// an active GaItemData record with flag 1 — matching T063's confirmed
// contract. Before the fix, allocNewGaItem special-cased db.IsArrowID(id) to
// skip upsertWeaponGaItemData, so the GaItemData record never existed and the
// arrow was invisible in the game's Arrows/Bolts category.
func TestAddItemsToSlotBatch_NewArrowGetsGaItemAndActiveGaItemData(t *testing.T) {
	fixture := fragmentedGaItemRoundTripFixtureForVersion(t, GaItemVersionBreak+1)
	slot := fixture.Slot
	before := activeGaItemDataRecords(t, slot)

	if err := AddItemsToSlotBatch(slot, []ItemToAdd{{ItemID: testArrowID, InvQty: 5, ForceStackable: true}}); err != nil {
		t.Fatalf("AddItemsToSlotBatch: %v", err)
	}

	var arrowGaItem *GaItemFull
	for i := range slot.GaItems {
		if !slot.GaItems[i].IsEmpty() && slot.GaItems[i].ItemID == testArrowID {
			arrowGaItem = &slot.GaItems[i]
			break
		}
	}
	if arrowGaItem == nil {
		t.Fatal("no GaItem record created for the new Arrow stack")
	}
	if arrowGaItem.Handle&GaHandleTypeMask != ItemTypeWeapon {
		t.Errorf("Arrow GaItem handle 0x%08X is not weapon-prefixed", arrowGaItem.Handle)
	}
	if arrowGaItem.Unk2 != 0 || arrowGaItem.Unk3 != 0 {
		t.Errorf("Arrow GaItem Unk2/Unk3 = %d/%d, want 0/0 (T063/T211)", arrowGaItem.Unk2, arrowGaItem.Unk3)
	}

	after := activeGaItemDataRecords(t, slot)
	if len(after) != len(before)+1 {
		t.Fatalf("active GaItemData count = %d, want %d (+1 for the new Arrow — T211 regression)", len(after), len(before)+1)
	}
	found := false
	for _, r := range after {
		if r.id == testArrowID {
			found = true
			if r.flag != 1 {
				t.Errorf("Arrow GaItemData flag = %d, want 1", r.flag)
			}
		}
	}
	if !found {
		t.Fatal("no active GaItemData record for the Arrow — T211 regression")
	}

	qtyFound := false
	for _, inv := range slot.Inventory.CommonItems {
		if inv.GaItemHandle == arrowGaItem.Handle {
			qtyFound = true
			if inv.Quantity != 5 {
				t.Errorf("Arrow inventory quantity = %d, want 5", inv.Quantity)
			}
		}
	}
	if !qtyFound {
		t.Fatal("no CommonItems record for the Arrow handle")
	}
}

// TestAddItemsToSlotBatch_ExistingArrowStackDoesNotDuplicateGaItemOrGaItemData
// covers T063's existing-stack contract: raising an already-owned Arrow stack
// must update quantity in place, without allocating a second GaItem or a
// second GaItemData record.
func TestAddItemsToSlotBatch_ExistingArrowStackDoesNotDuplicateGaItemOrGaItemData(t *testing.T) {
	fixture := fragmentedGaItemRoundTripFixtureForVersion(t, GaItemVersionBreak+1)
	slot := fixture.Slot

	if err := AddItemsToSlotBatch(slot, []ItemToAdd{{ItemID: testArrowID, InvQty: 3, ForceStackable: true}}); err != nil {
		t.Fatalf("first add: %v", err)
	}
	gaCountAfterFirst := countNonEmptyGaItems(slot)
	dataCountAfterFirst := len(activeGaItemDataRecords(t, slot))

	if err := AddItemsToSlotBatch(slot, []ItemToAdd{{ItemID: testArrowID, InvQty: 8, ForceStackable: true}}); err != nil {
		t.Fatalf("second add (existing stack): %v", err)
	}

	if got := countNonEmptyGaItems(slot); got != gaCountAfterFirst {
		t.Errorf("GaItems count after existing-stack bump = %d, want unchanged %d", got, gaCountAfterFirst)
	}
	if got := len(activeGaItemDataRecords(t, slot)); got != dataCountAfterFirst {
		t.Errorf("active GaItemData count after existing-stack bump = %d, want unchanged %d", got, dataCountAfterFirst)
	}

	found := false
	for _, inv := range slot.Inventory.CommonItems {
		if inv.GaItemHandle == GaHandleEmpty || inv.GaItemHandle == GaHandleInvalid {
			continue
		}
		if id, ok := slot.GaMap[inv.GaItemHandle]; ok && id == testArrowID {
			found = true
			if inv.Quantity != 8 {
				t.Errorf("Arrow quantity after existing-stack bump = %d, want 8 (SET, not additive)", inv.Quantity)
			}
		}
	}
	if !found {
		t.Fatal("Arrow record missing after existing-stack bump")
	}
}

// TestCheckAddCapacity_NewArrowNeedsGaItemAndGaItemData is the capacity-side
// half of the T211 fix: preflight must count exactly what the writer now
// allocates for a brand new Arrow stack (one GaItem, one GaItemData entry),
// matching TestAddItemsToSlotBatch_NewArrowGetsGaItemAndActiveGaItemData.
func TestCheckAddCapacity_NewArrowNeedsGaItemAndGaItemData(t *testing.T) {
	slot := capSlot(0, make([]GaItemFull, 10))

	report := CheckAddCapacity(slot, []ItemToAdd{{ItemID: testArrowID, InvQty: 1, ForceStackable: true}})
	if !report.CanFitAll {
		t.Fatalf("CanFitAll=false, CapHit=%q; want fit with 10 free GaItems/GaItemData", report.CapHit)
	}
	if report.NeededGaItems != 1 {
		t.Errorf("NeededGaItems=%d, want 1 (new Arrow allocates a GaItem — T211)", report.NeededGaItems)
	}
	if report.NeededGaItemData != 1 {
		t.Errorf("NeededGaItemData=%d, want 1 (new Arrow allocates GaItemData — T211)", report.NeededGaItemData)
	}
	if report.NeededInv != 1 {
		t.Errorf("NeededInv=%d, want 1", report.NeededInv)
	}
}

// TestCheckAddCapacity_ExistingArrowStackNeedsNoNewGaItemOrInv guards the
// existingItemIDs lookup: an arrow's handle is counter-allocated (not
// id-derived like goods/talismans), so a naive (id & mask) | prefix guess
// can never find it, and would wrongly re-count a new GaItem/GaItemData/inv
// slot for an already-owned stack.
func TestCheckAddCapacity_ExistingArrowStackNeedsNoNewGaItemOrInv(t *testing.T) {
	const arrowHandle = uint32(ItemTypeWeapon | 0x00800042) // counter-allocated, unrelated to testArrowID's bits

	slot := capSlot(0, make([]GaItemFull, 10))
	slot.GaMap[arrowHandle] = testArrowID
	slot.Inventory.CommonItems = append(slot.Inventory.CommonItems, InventoryItem{GaItemHandle: arrowHandle, Quantity: 1, Index: 969})

	report := CheckAddCapacity(slot, []ItemToAdd{{ItemID: testArrowID, InvQty: 2, ForceStackable: true}})
	if !report.CanFitAll {
		t.Fatalf("CanFitAll=false, CapHit=%q; existing Arrow stack bump must always fit", report.CapHit)
	}
	if report.NeededGaItems != 0 {
		t.Errorf("NeededGaItems=%d, want 0 (existing Arrow stack, no new GaItem)", report.NeededGaItems)
	}
	if report.NeededGaItemData != 0 {
		t.Errorf("NeededGaItemData=%d, want 0 (existing Arrow stack, no new GaItemData)", report.NeededGaItemData)
	}
	if report.NeededInv != 0 {
		t.Errorf("NeededInv=%d, want 0 (existing stack updates its record in place)", report.NeededInv)
	}
}

// TestCheckAddCapacity_GoodsStackStillNeedsNoGaItem is the regression guard
// for T050/T060: ordinary goods/crafting materials (handle-encoded, 0x40 item
// ID prefix) must never count a GaItem need — only arrows/bolts and
// weapon/armor/AoW (addKindArrow / addKindGaItem) allocate a serialized
// GaItem record. The active-GaItemData contract for goods/talismans/Key
// Items is covered separately in writer_native_gaitemdata_families_test.go.
func TestCheckAddCapacity_GoodsStackStillNeedsNoGaItem(t *testing.T) {
	const goodsID = uint32(0x40000064) // stackable good, prefix 0x40 (Throwing Dagger family)

	slot := capSlot(0, nil) // zero free GaItems — must not matter for goods
	report := CheckAddCapacity(slot, []ItemToAdd{{ItemID: goodsID, InvQty: 1}})
	if !report.CanFitAll {
		t.Fatalf("CanFitAll=false, CapHit=%q; goods must fit with zero GaItems free", report.CapHit)
	}
	if report.NeededGaItems != 0 {
		t.Errorf("NeededGaItems=%d, want 0 (goods never allocate a GaItem — T050/T060)", report.NeededGaItems)
	}
}

// testChainCoifID is the real "Chain Coif" armor ID from item-save-lab T020
// (Merchant Kalé, decimal 269535456 = 0x1010C8E0).
const testChainCoifID = uint32(0x1010C8E0)

// TestAddItemsToSlotBatch_NewArmorGetsGaItemAndActiveGaItemData is
// requirement 2's core contract (T020): a brand-new armor piece gets a
// serialized GaItem (armor-prefixed, Unk2=Unk3=0) AND an active GaItemData
// entry with flag 1 — the writer previously only did this for
// ItemTypeWeapon/ItemTypeAow, so armor never got a GaItemData entry at all.
func TestAddItemsToSlotBatch_NewArmorGetsGaItemAndActiveGaItemData(t *testing.T) {
	fixture := fragmentedGaItemRoundTripFixtureForVersion(t, GaItemVersionBreak+1)
	slot := fixture.Slot
	before := activeGaItemDataRecords(t, slot)

	if err := AddItemsToSlotBatch(slot, []ItemToAdd{{ItemID: testChainCoifID, InvQty: 1}}); err != nil {
		t.Fatalf("AddItemsToSlotBatch: %v", err)
	}

	var armorGaItem *GaItemFull
	for i := range slot.GaItems {
		if !slot.GaItems[i].IsEmpty() && slot.GaItems[i].ItemID == testChainCoifID {
			armorGaItem = &slot.GaItems[i]
			break
		}
	}
	if armorGaItem == nil {
		t.Fatal("no GaItem record created for the new Chain Coif")
	}
	if armorGaItem.Handle&GaHandleTypeMask != ItemTypeArmor {
		t.Errorf("Chain Coif GaItem handle 0x%08X is not armor-prefixed", armorGaItem.Handle)
	}
	if armorGaItem.Unk2 != 0 || armorGaItem.Unk3 != 0 {
		t.Errorf("Chain Coif GaItem Unk2/Unk3 = %d/%d, want 0/0 (T020)", armorGaItem.Unk2, armorGaItem.Unk3)
	}

	after := activeGaItemDataRecords(t, slot)
	if len(after) != len(before)+1 {
		t.Fatalf("active GaItemData count = %d, want %d (+1 for the new Chain Coif — requirement 2)", len(after), len(before)+1)
	}
	found := false
	for _, r := range after {
		if r.id == testChainCoifID {
			found = true
			if r.flag != 1 {
				t.Errorf("Chain Coif GaItemData flag = %d, want 1", r.flag)
			}
		}
	}
	if !found {
		t.Fatal("no active GaItemData record for the new Chain Coif")
	}
}

// TestAddItemsToSlotBatch_ExistingArmorSecondDestinationDoesNotDuplicateGaItemData
// covers the "existing ID" half of requirement 2: adding the same armor ID to
// a second destination (Storage, after Inventory) allocates its own GaItem
// record per destination (armor is not stackable) but must not create a
// second active GaItemData entry for the ID — upsertActiveGaItemData's
// existing itemID scan dedups this exactly like it already does for weapons.
func TestAddItemsToSlotBatch_ExistingArmorSecondDestinationDoesNotDuplicateGaItemData(t *testing.T) {
	fixture := fragmentedGaItemRoundTripFixtureForVersion(t, GaItemVersionBreak+1)
	slot := fixture.Slot
	before := activeGaItemDataRecords(t, slot)

	if err := AddItemsToSlotBatch(slot, []ItemToAdd{{ItemID: testChainCoifID, InvQty: 1}}); err != nil {
		t.Fatalf("first add (Inventory): %v", err)
	}
	afterFirst := activeGaItemDataRecords(t, slot)
	if len(afterFirst) != len(before)+1 {
		t.Fatalf("active GaItemData count after first add = %d, want %d (+1)", len(afterFirst), len(before)+1)
	}

	if err := AddItemsToSlotBatch(slot, []ItemToAdd{{ItemID: testChainCoifID, StorageQty: 1}}); err != nil {
		t.Fatalf("second add (Storage, existing ID): %v", err)
	}
	afterSecond := activeGaItemDataRecords(t, slot)
	if len(afterSecond) != len(afterFirst) {
		t.Errorf("active GaItemData count after second destination = %d, want unchanged %d (no duplicate)", len(afterSecond), len(afterFirst))
	}

	gaCount := 0
	for _, ga := range slot.GaItems {
		if !ga.IsEmpty() && ga.ItemID == testChainCoifID {
			gaCount++
		}
	}
	if gaCount != 2 {
		t.Errorf("Chain Coif GaItem record count = %d, want 2 (one per destination)", gaCount)
	}
}

// --- Direct AddItemsToSlot parity (requirement 1: both public entry points
// must behave identically) ---
//
// AddItemsToSlot is now a thin per-id wrapper around AddItemsToSlotBatch (see
// writer.go), so these tests exercise the legacy API directly rather than
// just trusting the delegation — a bump of an existing stack through
// AddItemsToSlot in particular must not create a second GaItemData entry.

// TestAddItemsToSlot_NewArrowGetsGaItemAndActiveGaItemData is the T211/T063
// contract driven through AddItemsToSlot's forceStackable path.
func TestAddItemsToSlot_NewArrowGetsGaItemAndActiveGaItemData(t *testing.T) {
	fixture := fragmentedGaItemRoundTripFixtureForVersion(t, GaItemVersionBreak+1)
	slot := fixture.Slot
	before := activeGaItemDataRecords(t, slot)

	if err := AddItemsToSlot(slot, []uint32{testArrowID}, 5, 0, true); err != nil {
		t.Fatalf("AddItemsToSlot: %v", err)
	}

	var arrowGaItem *GaItemFull
	for i := range slot.GaItems {
		if !slot.GaItems[i].IsEmpty() && slot.GaItems[i].ItemID == testArrowID {
			arrowGaItem = &slot.GaItems[i]
			break
		}
	}
	if arrowGaItem == nil {
		t.Fatal("no GaItem record created for the new Arrow")
	}
	if arrowGaItem.Unk2 != 0 || arrowGaItem.Unk3 != 0 {
		t.Errorf("Arrow GaItem Unk2/Unk3 = %d/%d, want 0/0", arrowGaItem.Unk2, arrowGaItem.Unk3)
	}

	after := activeGaItemDataRecords(t, slot)
	if len(after) != len(before)+1 {
		t.Fatalf("active GaItemData count = %d, want %d (+1)", len(after), len(before)+1)
	}

	qtyFound := false
	for _, inv := range slot.Inventory.CommonItems {
		if inv.GaItemHandle == arrowGaItem.Handle {
			qtyFound = true
			if inv.Quantity != 5 {
				t.Errorf("Arrow inventory quantity = %d, want 5", inv.Quantity)
			}
		}
	}
	if !qtyFound {
		t.Fatal("no CommonItems record for the Arrow handle")
	}
}

// TestAddItemsToSlot_BoltIsDistinctNewGaItemDataEntry proves Arrow and Bolt
// (T064's two distinct ammo IDs) each get their own GaItem/GaItemData
// through AddItemsToSlot, mirroring TestCheckAddCapacity_ArrowAndBoltAsTwoNewIDsInBatch.
func TestAddItemsToSlot_BoltIsDistinctNewGaItemDataEntry(t *testing.T) {
	fixture := fragmentedGaItemRoundTripFixtureForVersion(t, GaItemVersionBreak+1)
	slot := fixture.Slot

	if err := AddItemsToSlot(slot, []uint32{testArrowID}, 1, 0, true); err != nil {
		t.Fatalf("AddItemsToSlot(Arrow): %v", err)
	}
	afterArrow := activeGaItemDataRecords(t, slot)

	if err := AddItemsToSlot(slot, []uint32{testBoltID}, 1, 0, true); err != nil {
		t.Fatalf("AddItemsToSlot(Bolt): %v", err)
	}
	afterBolt := activeGaItemDataRecords(t, slot)
	if len(afterBolt) != len(afterArrow)+1 {
		t.Fatalf("active GaItemData count after Bolt = %d, want %d (+1, distinct from Arrow)", len(afterBolt), len(afterArrow)+1)
	}

	handles := map[uint32]bool{}
	for h, id := range slot.GaMap {
		if id == testArrowID || id == testBoltID {
			handles[h] = true
		}
	}
	if len(handles) != 2 {
		t.Errorf("distinct Arrow/Bolt handles = %d, want 2", len(handles))
	}
}
