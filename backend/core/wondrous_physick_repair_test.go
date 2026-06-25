package core

import "testing"

func physickRows(slot *SaveSlot) []InventoryItem {
	var rows []InventoryItem
	for _, item := range slot.Inventory.CommonItems {
		if item.GaItemHandle == 0 || item.GaItemHandle == 0xFFFFFFFF {
			continue
		}
		if item.GaItemHandle == 0xB00000FA || item.GaItemHandle == 0xB00000FB {
			rows = append(rows, item)
		}
	}
	return rows
}

func TestRepairDuplicateWondrousPhysick_PrefersFilled(t *testing.T) {
	slot := buildRepairFixture(t, []InventoryItem{
		{GaItemHandle: 0xB00000FB, Quantity: 1, Index: 100},
		{GaItemHandle: 0xB00000FA, Quantity: 1, Index: 101},
	}, nil)

	issues := ScanDuplicateWondrousPhysick(slot)
	if len(issues) != 2 {
		t.Fatalf("ScanDuplicateWondrousPhysick = %d issues, want 2", len(issues))
	}
	removed, err := RepairDuplicateWondrousPhysick(slot)
	if err != nil {
		t.Fatalf("RepairDuplicateWondrousPhysick: %v", err)
	}
	if removed != 1 {
		t.Fatalf("removed = %d, want 1", removed)
	}
	rows := physickRows(slot)
	if len(rows) != 1 {
		t.Fatalf("remaining Physick rows = %d, want 1", len(rows))
	}
	if rows[0].GaItemHandle != 0xB00000FA {
		t.Fatalf("remaining handle = 0x%08X, want filled 0xB00000FA", rows[0].GaItemHandle)
	}
	if issues := ScanDuplicateWondrousPhysick(slot); len(issues) != 0 {
		t.Fatalf("duplicate Physick issues remain: %+v", issues)
	}
}

func TestRepairDuplicateWondrousPhysick_SameFilledHandleKeepsOne(t *testing.T) {
	slot := buildRepairFixture(t, []InventoryItem{
		{GaItemHandle: 0xB00000FA, Quantity: 1, Index: 100},
		{GaItemHandle: 0xB00000FA, Quantity: 1, Index: 101},
	}, nil)

	removed, err := RepairDuplicateWondrousPhysick(slot)
	if err != nil {
		t.Fatalf("RepairDuplicateWondrousPhysick: %v", err)
	}
	if removed != 1 {
		t.Fatalf("removed = %d, want 1", removed)
	}
	rows := physickRows(slot)
	if len(rows) != 1 || rows[0].GaItemHandle != 0xB00000FA {
		t.Fatalf("remaining Physick rows = %+v, want one filled row", rows)
	}
}

func TestRepairDuplicateWondrousPhysick_SameEmptyHandleKeepsOne(t *testing.T) {
	slot := buildRepairFixture(t, []InventoryItem{
		{GaItemHandle: 0xB00000FB, Quantity: 1, Index: 100},
		{GaItemHandle: 0xB00000FB, Quantity: 1, Index: 101},
	}, nil)

	removed, err := RepairDuplicateWondrousPhysick(slot)
	if err != nil {
		t.Fatalf("RepairDuplicateWondrousPhysick: %v", err)
	}
	if removed != 1 {
		t.Fatalf("removed = %d, want 1", removed)
	}
	rows := physickRows(slot)
	if len(rows) != 1 || rows[0].GaItemHandle != 0xB00000FB {
		t.Fatalf("remaining Physick rows = %+v, want one empty row", rows)
	}
}
