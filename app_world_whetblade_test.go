package main

import (
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
	"github.com/oisis/EldenRing-SaveForge/backend/db/data"
)

func TestWhetbladeAlreadyOwned_KeyItems(t *testing.T) {
	slot := &core.SaveSlot{
		Inventory: core.EquipInventoryData{
			KeyItems: []core.InventoryItem{{
				GaItemHandle: 0xB000218E, // Whetstone Knife computed handle
				Quantity:     1,
			}},
		},
	}
	if !whetbladeAlreadyOwned(slot, data.WhetstoneKnifeFlag, []byte{}) {
		t.Fatal("expected already owned via KeyItems")
	}
}

func TestWhetbladeAlreadyOwned_CommonItems(t *testing.T) {
	slot := &core.SaveSlot{
		Inventory: core.EquipInventoryData{
			CommonItems: []core.InventoryItem{{
				GaItemHandle: 0xB000218E,
				Quantity:     1,
			}},
		},
	}
	if !whetbladeAlreadyOwned(slot, data.WhetstoneKnifeFlag, []byte{}) {
		t.Fatal("expected already owned via CommonItems")
	}
}

func TestWhetbladeAlreadyOwned_NotOwned(t *testing.T) {
	slot := &core.SaveSlot{}
	if whetbladeAlreadyOwned(slot, data.WhetstoneKnifeFlag, []byte{}) {
		t.Fatal("expected not owned in empty slot")
	}
}
