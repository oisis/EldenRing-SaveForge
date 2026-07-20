package core

import (
	"encoding/binary"
	"testing"
)

// TestAddItemsToSlotBatch_NewInventoryRecordAdvancesCounters is the T210
// regression test (backend/core/writer.go, addToInventory's !isStorage branch):
// a brand-new Inventory.CommonItems record (Throwing Dagger, 0x400006A4)
// created from native T210's exact reproduction state — NextAcquisitionSortId
// =968, NextEquipIndex=433 — must get Index=969 and advance
// NextAcquisitionSortId to 970 and NextEquipIndex to 434, matching confirmed
// native T050 evidence from the same base save (hub-elleh.sl2). The prior
// writer forced an even Index (968, colliding with the existing native
// record's own index) and never touched NextEquipIndex, which the game then
// silently refused to render. Runs entirely against an in-memory fixture — no
// dependency on any private save under tmp/.
func TestAddItemsToSlotBatch_NewInventoryRecordAdvancesCounters(t *testing.T) {
	fixture := fragmentedRepackRoundTripFixtureForVersion(t, GaItemVersionBreak+1)
	slot := fixture.Slot

	const wantIndex = uint32(969)
	const wantNextAcq = uint32(970)
	const wantNextEquip = uint32(434)

	slot.Inventory.NextAcquisitionSortId = 968
	slot.Inventory.NextEquipIndex = 433
	binary.LittleEndian.PutUint32(slot.Data[slot.Inventory.nextAcqSortIdOff:], 968)
	binary.LittleEndian.PutUint32(slot.Data[slot.Inventory.nextEquipIndexOff:], 433)

	if err := AddItemsToSlotBatch(slot, []ItemToAdd{{ItemID: testThrowingDaggerID, InvQty: 1}}); err != nil {
		t.Fatalf("AddItemsToSlotBatch: %v", err)
	}

	handle, ok := findCommonItemHandle(slot, testThrowingDaggerID)
	if !ok {
		t.Fatal("no CommonItems record created for Throwing Dagger")
	}
	var rec *InventoryItem
	for i := range slot.Inventory.CommonItems {
		if slot.Inventory.CommonItems[i].GaItemHandle == handle {
			rec = &slot.Inventory.CommonItems[i]
			break
		}
	}
	if rec == nil {
		t.Fatal("CommonItems record disappeared after lookup")
	}
	if rec.Index != wantIndex {
		t.Errorf("record Index = %d, want %d", rec.Index, wantIndex)
	}

	if slot.Inventory.NextAcquisitionSortId != wantNextAcq {
		t.Errorf("struct NextAcquisitionSortId = %d, want %d", slot.Inventory.NextAcquisitionSortId, wantNextAcq)
	}
	if slot.Inventory.NextEquipIndex != wantNextEquip {
		t.Errorf("struct NextEquipIndex = %d, want %d", slot.Inventory.NextEquipIndex, wantNextEquip)
	}

	// Physical bytes for both counters must match the in-memory struct state.
	rawNextAcq := binary.LittleEndian.Uint32(slot.Data[slot.Inventory.nextAcqSortIdOff:])
	if rawNextAcq != wantNextAcq {
		t.Errorf("binary NextAcquisitionSortId = %d, want %d", rawNextAcq, wantNextAcq)
	}
	rawNextEquip := binary.LittleEndian.Uint32(slot.Data[slot.Inventory.nextEquipIndexOff:])
	if rawNextEquip != wantNextEquip {
		t.Errorf("binary NextEquipIndex = %d, want %d", rawNextEquip, wantNextEquip)
	}

	// T050 contract: exactly one active GaItemData entry, no serialized GaItem.
	found := false
	for _, r := range activeGaItemDataRecords(t, slot) {
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
	for _, ga := range slot.GaItems {
		if !ga.IsEmpty() && ga.ItemID == testThrowingDaggerID {
			t.Errorf("Throwing Dagger 0x%08X must not have a separate GaItem record", testThrowingDaggerID)
		}
	}
}

// TestAddToInventory_ExistingStackTopupDoesNotAdvanceCounters guards the
// scoped-out half of the T210 fix: raising the quantity of an already-owned
// Inventory.CommonItems stack must leave its Index, NextAcquisitionSortId, and
// NextEquipIndex untouched — only a genuinely NEW record advances them.
func TestAddToInventory_ExistingStackTopupDoesNotAdvanceCounters(t *testing.T) {
	const handle = uint32(0xB0000A11)
	items := []InventoryItem{{GaItemHandle: handle, Quantity: 1, Index: 969}}
	slot := buildNextEquipFixture(t, items, 434, 970)

	if err := addToInventory(slot, handle, 5, false, false); err != nil {
		t.Fatalf("addToInventory (topup): %v", err)
	}

	if got := slot.Inventory.CommonItems[0].Quantity; got != 5 {
		t.Fatalf("quantity after topup = %d, want 5", got)
	}
	if got := slot.Inventory.CommonItems[0].Index; got != 969 {
		t.Errorf("Index changed by topup: got %d, want unchanged 969", got)
	}
	if slot.Inventory.NextAcquisitionSortId != 970 {
		t.Errorf("NextAcquisitionSortId changed by topup: got %d, want unchanged 970", slot.Inventory.NextAcquisitionSortId)
	}
	if slot.Inventory.NextEquipIndex != 434 {
		t.Errorf("NextEquipIndex changed by topup: got %d, want unchanged 434", slot.Inventory.NextEquipIndex)
	}
}
