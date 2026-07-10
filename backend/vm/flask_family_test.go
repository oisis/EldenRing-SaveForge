package vm

import (
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
)

// TestMapParsedSlotToVM_FlaskFamilyID proves an upgraded flask keeps its exact
// inventory identity (ID / Name / BaseID / CurrentUpgrade) while exposing the
// +0 family base via FamilyID, so the DB tab counts it under the picker row.
func TestMapParsedSlotToVM_FlaskFamilyID(t *testing.T) {
	slot := &core.SaveSlot{
		Version: 1,
		GaMap:   make(map[uint32]uint32),
	}
	slot.Inventory.CommonItems = []core.InventoryItem{
		{GaItemHandle: 0xB0000401, Quantity: 1, Index: 100}, // Flask of Crimson Tears +12
	}

	got, err := MapParsedSlotToVM(slot)
	if err != nil {
		t.Fatalf("MapParsedSlotToVM: %v", err)
	}
	if len(got.Inventory) != 1 {
		t.Fatalf("Inventory rows = %d, want 1", len(got.Inventory))
	}
	item := got.Inventory[0]

	if item.ID != 0x40000401 {
		t.Errorf("ID = 0x%08X, want exact upgraded ID 0x40000401", item.ID)
	}
	if item.Name != "Flask of Crimson Tears +12" {
		t.Errorf("Name = %q, want %q", item.Name, "Flask of Crimson Tears +12")
	}
	if item.BaseID != 0x40000401 {
		t.Errorf("BaseID = 0x%08X, want unchanged 0x40000401", item.BaseID)
	}
	if item.CurrentUpgrade != 0 {
		t.Errorf("CurrentUpgrade = %d, want 0 (flask level lives in the name)", item.CurrentUpgrade)
	}
	if item.FamilyID != 0x400003E9 {
		t.Errorf("FamilyID = 0x%08X, want +0 family base 0x400003E9", item.FamilyID)
	}
}
