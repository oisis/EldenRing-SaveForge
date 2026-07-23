package main

import (
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
	"github.com/oisis/EldenRing-SaveForge/backend/db"
)

const crimsonCrystalTearID = uint32(0x40002AFB)

func hasItemHandle(items []core.InventoryItem, handle uint32) bool {
	for _, item := range items {
		if item.GaItemHandle == handle {
			return true
		}
	}
	return false
}

func TestAddItemsToCharacter_AddsCrimsonCrystalTearWithoutPhysickBundle(t *testing.T) {
	app := gaItemAddApp(t, false)
	slot := &app.save.Slots[0]

	result, err := app.AddItemsToCharacter(0, []uint32{crimsonCrystalTearID}, 0, 0, 0, 0, 1, 0)
	if err != nil {
		t.Fatalf("AddItemsToCharacter: %v", err)
	}
	if result.Added != 1 {
		t.Fatalf("Added = %d, want 1", result.Added)
	}

	if !hasItemHandle(slot.Inventory.CommonItems, 0xB0002AFB) {
		t.Error("standalone Crimson Crystal Tear not written")
	}
	for _, handle := range []uint32{0xB0002AFA, 0xB00000FA, 0xB00000FB, 0xB000239B} {
		if hasItemHandle(slot.Inventory.CommonItems, handle) ||
			hasItemHandle(slot.Inventory.KeyItems, handle) {
			t.Errorf("unexpected T090 bundle handle 0x%08X written", handle)
		}
	}
}

func TestAddItemsToCharacter_AddsCrimsonCrystalTearAndPhysickIndependently(t *testing.T) {
	for _, tc := range []struct {
		name       string
		storageQty int
	}{
		{name: "inventory"},
		{name: "inventory_and_storage", storageQty: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := gaItemAddApp(t, false)
			slot := &app.save.Slots[0]

			result, err := app.AddItemsToCharacter(
				0,
				[]uint32{crimsonCrystalTearID, db.ItemFlaskWondrousPhysickEmpty},
				0, 0, 0, 0, 1, tc.storageQty,
			)
			if err != nil {
				t.Fatalf("AddItemsToCharacter: %v", err)
			}
			if result.Added != 2 {
				t.Fatalf("Added = %d, want 2", result.Added)
			}

			if !hasItemHandle(slot.Inventory.CommonItems, 0xB0002AFB) {
				t.Error("standalone Crimson Crystal Tear not written")
			}
			if !hasItemHandle(slot.Inventory.CommonItems, 0xB00000FB) {
				t.Error("standalone Flask of Wondrous Physick not written")
			}
			for _, handle := range []uint32{0xB0002AFA, 0xB00000FA, 0xB000239B} {
				if hasItemHandle(slot.Inventory.CommonItems, handle) ||
					hasItemHandle(slot.Inventory.KeyItems, handle) {
					t.Errorf("unexpected T090 bundle handle 0x%08X written", handle)
				}
			}
		})
	}
}

func TestAddItemsToCharacter_CrimsonCrystalTearRespectsMaxOne(t *testing.T) {
	app := gaItemAddApp(t, false)

	if _, err := app.AddItemsToCharacter(0, []uint32{crimsonCrystalTearID}, 0, 0, 0, 0, 2, 0); err != nil {
		t.Fatalf("first AddItemsToCharacter: %v", err)
	}
	result, err := app.AddItemsToCharacter(0, []uint32{crimsonCrystalTearID}, 0, 0, 0, 0, 1, 0)
	if err != nil {
		t.Fatalf("second AddItemsToCharacter: %v", err)
	}
	if result.Added != 0 {
		t.Fatalf("Added = %d, want 0", result.Added)
	}

	count := 0
	for _, item := range app.save.Slots[0].Inventory.CommonItems {
		if item.GaItemHandle == 0xB0002AFB {
			count++
			if quantity := item.Quantity & 0x7FFFFFFF; quantity != 1 {
				t.Errorf("Crimson Crystal Tear quantity = %d, want 1", quantity)
			}
		}
	}
	if count != 1 {
		t.Fatalf("Crimson Crystal Tear records = %d, want 1", count)
	}
}
