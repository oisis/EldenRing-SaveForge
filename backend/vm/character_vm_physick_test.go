package vm

import (
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
)

func TestMapParsedSlotToVM_DisplaysFilledPhysick(t *testing.T) {
	slot := &core.SaveSlot{
		Version: 1,
		GaMap:   make(map[uint32]uint32),
	}
	slot.Inventory.CommonItems = []core.InventoryItem{
		{GaItemHandle: 0xB00000FA, Quantity: 1, Index: 100},
	}

	got, err := MapParsedSlotToVM(slot)
	if err != nil {
		t.Fatalf("MapParsedSlotToVM: %v", err)
	}
	if len(got.Inventory) != 1 {
		t.Fatalf("Inventory rows = %d, want 1", len(got.Inventory))
	}
	item := got.Inventory[0]
	if item.ID != 0x400000FB {
		t.Fatalf("ID = 0x%08X, want display ID 0x400000FB", item.ID)
	}
	if item.Handle != 0xB00000FA {
		t.Fatalf("Handle = 0x%08X, want raw handle 0xB00000FA", item.Handle)
	}
	if item.Name != "Flask of Wondrous Physick" {
		t.Fatalf("Name = %q, want Flask of Wondrous Physick", item.Name)
	}
}

func TestMapParsedSlotToVM_DedupesPhysickVariants(t *testing.T) {
	slot := &core.SaveSlot{
		Version: 1,
		GaMap:   make(map[uint32]uint32),
	}
	slot.Inventory.CommonItems = []core.InventoryItem{
		{GaItemHandle: 0xB00000FA, Quantity: 1, Index: 100},
		{GaItemHandle: 0xB00000FB, Quantity: 1, Index: 101},
	}

	got, err := MapParsedSlotToVM(slot)
	if err != nil {
		t.Fatalf("MapParsedSlotToVM: %v", err)
	}
	if len(got.Inventory) != 1 {
		t.Fatalf("Inventory rows = %d, want one logical Physick row", len(got.Inventory))
	}
}
