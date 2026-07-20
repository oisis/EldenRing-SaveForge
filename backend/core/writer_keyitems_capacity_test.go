package core

import (
	"encoding/binary"
	"errors"
	"io"
	"testing"
)

// writeKeyItemsPhysical writes slot.Inventory.KeyItems into the KeyItems
// physical byte region, mirroring gaitem_repack_fixture_test.go's
// writeFixtureInventory. Needed because addToKeyItems scans slot.Data
// directly (see writer.go), not the in-memory slice — a test that only
// mutates slot.Inventory.KeyItems in Go memory would not reproduce the
// writer's real "array full" detection.
func writeKeyItemsPhysical(slot *SaveSlot) {
	keyStart := slot.MagicOffset + InvStartFromMagic + CommonItemCount*InvRecordLen + InvKeyCountHeader
	for i, item := range slot.Inventory.KeyItems {
		off := keyStart + i*InvRecordLen
		binary.LittleEndian.PutUint32(slot.Data[off:], item.GaItemHandle)
		binary.LittleEndian.PutUint32(slot.Data[off+4:], item.Quantity)
		binary.LittleEndian.PutUint32(slot.Data[off+8:], item.Index)
	}
}

// fillKeyItems returns a fully-occupied KeyItemCount-slot array of distinct
// filler handles, none of which collide with any real confirmed KeyItem
// family handle used elsewhere in these tests.
func fillKeyItems() []InventoryItem {
	items := make([]InventoryItem, KeyItemCount)
	for i := range items {
		items[i] = InventoryItem{GaItemHandle: ItemTypeItem | uint32(0x100000+i), Quantity: 1, Index: uint32(2000 + i)}
	}
	return items
}

// TestCheckAddCapacity_FullKeyItemsRejectsNewNativeKeyItem is the capacity
// half of the KeyItems/CommonItems budget-separation fix: Inventory.KeyItems
// is an independently-sized physical array (KeyItemCount slots — see
// addToKeyItems), not a subset of CommonItems capacity. A completely full
// KeyItems array, with plenty of free CommonItems room, must still reject a
// brand-new confirmed native KeyItem (addKindKeyItemStack). Before this fix
// the preflight only checked FreeInv (CommonItems), so it silently reported
// "fits" while AddItemsToSlotBatch's addToKeyItems went on to fail.
func TestCheckAddCapacity_FullKeyItemsRejectsNewNativeKeyItem(t *testing.T) {
	slot := capSlot(0, nil) // 2688 free CommonItems, GaItems irrelevant here
	slot.Inventory.KeyItems = fillKeyItems()

	report := CheckAddCapacity(slot, []ItemToAdd{{ItemID: testCrackedPotID, InvQty: 1}})
	if report.CanFitAll {
		t.Fatal("CanFitAll=true, want false: KeyItems array is completely full")
	}
	if report.CapHit != "inventory_full" {
		t.Errorf("CapHit=%q, want inventory_full (no dedicated KeyItems label — safely mapped)", report.CapHit)
	}
	if report.NeededKeyItems != 1 {
		t.Errorf("NeededKeyItems=%d, want 1", report.NeededKeyItems)
	}
	if report.FreeKeyItems != 0 {
		t.Errorf("FreeKeyItems=%d, want 0", report.FreeKeyItems)
	}
	if report.FreeInv != CommonItemCount {
		t.Errorf("FreeInv=%d, want %d (CommonItems budget untouched by the KeyItems shortfall)", report.FreeInv, CommonItemCount)
	}
}

// TestAddItemsToSlotBatch_FullKeyItemsFailsWriter demonstrates the writer
// failure the preflight fix above now lets callers avoid: attempting to
// write a genuinely new native KeyItem into an already-full 384-slot array
// fails with io.ErrShortBuffer. Preflight and writer must never disagree —
// this test and the one above check the identical fixture shape.
func TestAddItemsToSlotBatch_FullKeyItemsFailsWriter(t *testing.T) {
	fixture := fragmentedRepackRoundTripFixtureForVersion(t, GaItemVersionBreak+1)
	slot := fixture.Slot
	slot.Inventory.KeyItems = fillKeyItems()
	writeKeyItemsPhysical(slot)

	err := AddItemsToSlotBatch(slot, []ItemToAdd{{ItemID: testCrackedPotID, InvQty: 1}})
	if !errors.Is(err, io.ErrShortBuffer) {
		t.Fatalf("AddItemsToSlotBatch error = %v, want io.ErrShortBuffer", err)
	}
}

// TestCheckAddCapacity_ExistingKeyItemBumpFitsDespiteFullKeyItems and
// TestAddItemsToSlotBatch_ExistingKeyItemBumpSucceedsDespiteFullKeyItems
// prove the flip side: bumping an already-owned native KeyItem's quantity
// consumes no new KeyItems slot, so it must succeed even when the array is
// otherwise completely full.
func TestCheckAddCapacity_ExistingKeyItemBumpFitsDespiteFullKeyItems(t *testing.T) {
	slot := capSlot(0, nil)
	handle := (testCrackedPotID & 0x0FFFFFFF) | ItemTypeItem
	slot.Inventory.KeyItems = fillKeyItems()
	slot.Inventory.KeyItems[0] = InventoryItem{GaItemHandle: handle, Quantity: 1, Index: 969}

	report := CheckAddCapacity(slot, []ItemToAdd{{ItemID: testCrackedPotID, InvQty: 2}})
	if !report.CanFitAll {
		t.Fatalf("CanFitAll=false, CapHit=%q; existing KeyItem bump must fit despite a full array", report.CapHit)
	}
	if report.NeededKeyItems != 0 {
		t.Errorf("NeededKeyItems=%d, want 0 (bump in place, no new slot)", report.NeededKeyItems)
	}
}

func TestAddItemsToSlotBatch_ExistingKeyItemBumpSucceedsDespiteFullKeyItems(t *testing.T) {
	fixture := fragmentedRepackRoundTripFixtureForVersion(t, GaItemVersionBreak+1)
	slot := fixture.Slot
	handle := (testCrackedPotID & 0x0FFFFFFF) | ItemTypeItem
	slot.Inventory.KeyItems = fillKeyItems()
	slot.Inventory.KeyItems[0] = InventoryItem{GaItemHandle: handle, Quantity: 1, Index: 969}
	writeKeyItemsPhysical(slot)

	if err := AddItemsToSlotBatch(slot, []ItemToAdd{{ItemID: testCrackedPotID, InvQty: 2}}); err != nil {
		t.Fatalf("AddItemsToSlotBatch: %v", err)
	}
	if got := slot.Inventory.KeyItems[0].Quantity; got != 2 {
		t.Errorf("Cracked Pot quantity = %d, want 2", got)
	}
}

// TestAddItemsToSlotBatch_KeyCountHeaderTracksNewEntriesOnly locks in the
// key_count header semantics confirmed by direct byte-level inspection of the
// T071/T074 read-only lab artifacts (see
// tmp/item-save-lab/WYNIKI_I_IMPLEMENTACJA.md, "Analiza key_count" section,
// 2026-07-20): task-071-key-items-cookbooks 01-before-a -> 02-pickup-a shows
// key_count going 0->1 for the first-ever native KeyItems insert, matching
// the physical non-empty-entry count exactly at every step (0,1,1,1,2 across
// 01..05); task-074-containers 04-before-b -> 05-pickup-a-b shows key_count
// staying at 1 across a quantity-only bump (Cracked Pot 1->2, same physical
// record). Not a symmetry guess — an independently verified header.
func TestAddItemsToSlotBatch_KeyCountHeaderTracksNewEntriesOnly(t *testing.T) {
	fixture := fragmentedRepackRoundTripFixtureForVersion(t, GaItemVersionBreak+1)
	slot := fixture.Slot
	keyCountOff := slot.MagicOffset + InvStartFromMagic + CommonItemCount*InvRecordLen
	readKeyCount := func() uint32 { return binary.LittleEndian.Uint32(slot.Data[keyCountOff:]) }

	if got := readKeyCount(); got != 0 {
		t.Fatalf("initial key_count = %d, want 0", got)
	}

	if err := AddItemsToSlotBatch(slot, []ItemToAdd{{ItemID: testCrackedPotID, InvQty: 1}}); err != nil {
		t.Fatalf("first native KeyItem: %v", err)
	}
	if got := readKeyCount(); got != 1 {
		t.Errorf("key_count after first native KeyItem = %d, want 1 (T071/T074: 0->1)", got)
	}

	if err := AddItemsToSlotBatch(slot, []ItemToAdd{{ItemID: testCrackedPotID, InvQty: 2}}); err != nil {
		t.Fatalf("quantity-only bump: %v", err)
	}
	if got := readKeyCount(); got != 1 {
		t.Errorf("key_count after quantity-only bump = %d, want unchanged 1 (T074: 04-before-b->05-pickup-a-b stays 1)", got)
	}

	if err := AddItemsToSlotBatch(slot, []ItemToAdd{{ItemID: testCookbook1ID, InvQty: 1}}); err != nil {
		t.Fatalf("second distinct native KeyItem: %v", err)
	}
	if got := readKeyCount(); got != 2 {
		t.Errorf("key_count after second distinct native KeyItem = %d, want 2 (T071: 01-before-a->02-pickup-a then 04-before-b->05-pickup-a-b: 0->1->2)", got)
	}
}
