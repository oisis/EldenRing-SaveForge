package core

import "testing"

// Weapon (prefix 0x00) → addKindGaItem, placed by physical handle index.
const cursorWeaponID = uint32(0x001E8480)

func TestCapacity_PhysicalHolesRemainUsableWhenLegacyCursorIsExhausted(t *testing.T) {
	slot := capSlot(0, make([]GaItemFull, 100)) // FreeGaItems = 100 (all empty)
	slot.NextArmamentIndex = 100                // cursor at end → zero cursor room

	r := CheckAddCapacity(slot, []ItemToAdd{{ItemID: cursorWeaponID, InvQty: 1}})
	if !r.CanFitAll {
		t.Fatalf("CanFitAll=false CapHit=%q: physical holes must remain usable", r.CapHit)
	}
	if r.FreeGaItems != 100 {
		t.Errorf("FreeGaItems=%d, want 100", r.FreeGaItems)
	}
}

func TestCapacity_LegacyCursorDoesNotLimitBatch(t *testing.T) {
	slot := capSlot(0, make([]GaItemFull, 100))
	slot.NextArmamentIndex = 99 // exactly one cursor slot left

	r := CheckAddCapacity(slot, []ItemToAdd{
		{ItemID: cursorWeaponID, InvQty: 1},
		{ItemID: cursorWeaponID, InvQty: 1},
	})
	if !r.CanFitAll {
		t.Fatalf("CanFitAll=false CapHit=%q: 2 armaments fit 100 physical holes", r.CapHit)
	}
	if r.NeededGaItems != 2 || r.FreeGaItems != 100 {
		t.Errorf("NeededGaItems=%d FreeGaItems=%d, want 2 / 100", r.NeededGaItems, r.FreeGaItems)
	}
}

// A batch that fits the physical empty-record count passes regardless of the
// legacy in-memory cursor value.
func TestCapacity_FitsPhysicalHoleConstraint(t *testing.T) {
	slot := capSlot(0, make([]GaItemFull, 100))
	slot.NextArmamentIndex = 95 // five cursor slots left

	r := CheckAddCapacity(slot, []ItemToAdd{
		{ItemID: cursorWeaponID, InvQty: 1},
		{ItemID: cursorWeaponID, InvQty: 1},
		{ItemID: cursorWeaponID, InvQty: 1},
	})
	if !r.CanFitAll {
		t.Fatalf("CanFitAll=false CapHit=%q: 3 armaments fit 100 empty records", r.CapHit)
	}
	if r.NeededGaItems != 3 {
		t.Errorf("NeededGaItems=%d, want 3", r.NeededGaItems)
	}
}
