package core

import "testing"

// Weapon (prefix 0x00) → addKindGaItem, placed at the allocator cursor.
const cursorWeaponID = uint32(0x001E8480)

// A slot can be physically full of empty GaItem records yet have no room to
// place a new armament because the allocator only writes at/after
// NextArmamentIndex and never reuses holes below it. Preflight must reject.
func TestCapacity_CursorExhaustedRejectsArmament(t *testing.T) {
	slot := capSlot(0, make([]GaItemFull, 100)) // FreeGaItems = 100 (all empty)
	slot.NextArmamentIndex = 100                // cursor at end → zero cursor room

	r := CheckAddCapacity(slot, []ItemToAdd{{ItemID: cursorWeaponID, InvQty: 1}})
	if r.CanFitAll {
		t.Fatalf("CanFitAll=true: 100 empty GaItems but cursor at end must reject the armament")
	}
	if r.CapHit != "gaitem_full" {
		t.Errorf("CapHit=%q, want gaitem_full", r.CapHit)
	}
	if r.FreeGaItems != 100 || r.FreeGaItemCursor != 0 {
		t.Errorf("FreeGaItems=%d FreeGaItemCursor=%d, want 100 / 0", r.FreeGaItems, r.FreeGaItemCursor)
	}
}

// A batch larger than the remaining cursor room is rejected even though the
// total empty-record budget is ample.
func TestCapacity_CursorRoomSmallerThanBatch(t *testing.T) {
	slot := capSlot(0, make([]GaItemFull, 100))
	slot.NextArmamentIndex = 99 // exactly one cursor slot left

	r := CheckAddCapacity(slot, []ItemToAdd{
		{ItemID: cursorWeaponID, InvQty: 1},
		{ItemID: cursorWeaponID, InvQty: 1},
	})
	if r.CanFitAll || r.CapHit != "gaitem_full" {
		t.Fatalf("CanFitAll=%v CapHit=%q: 2 armaments into 1 cursor slot must be gaitem_full", r.CanFitAll, r.CapHit)
	}
	if r.NeededGaItems != 2 || r.FreeGaItemCursor != 1 {
		t.Errorf("NeededGaItems=%d FreeGaItemCursor=%d, want 2 / 1", r.NeededGaItems, r.FreeGaItemCursor)
	}
}

// A batch that fits BOTH the empty-record count and the cursor room passes.
func TestCapacity_FitsBothConstraints(t *testing.T) {
	slot := capSlot(0, make([]GaItemFull, 100))
	slot.NextArmamentIndex = 95 // five cursor slots left

	r := CheckAddCapacity(slot, []ItemToAdd{
		{ItemID: cursorWeaponID, InvQty: 1},
		{ItemID: cursorWeaponID, InvQty: 1},
		{ItemID: cursorWeaponID, InvQty: 1},
	})
	if !r.CanFitAll {
		t.Fatalf("CanFitAll=false CapHit=%q: 3 armaments fit 5 cursor slots and 100 empty records", r.CapHit)
	}
	if r.NeededGaItems != 3 {
		t.Errorf("NeededGaItems=%d, want 3", r.NeededGaItems)
	}
}
