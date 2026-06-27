package main

import (
	"os"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
	"github.com/oisis/EldenRing-SaveForge/backend/db"
)

func countPhysick(slot *core.SaveSlot) (filled, empty int) {
	for _, item := range slot.Inventory.CommonItems {
		switch item.GaItemHandle {
		case 0xB00000FA:
			filled++
		case 0xB00000FB:
			empty++
		}
	}
	return filled, empty
}

func TestAddItemsToCharacter_AddsOnlyEmptyPhysickWhenMissing(t *testing.T) {
	const savePath = "tmp/save/ER0000.sl2"
	if _, err := os.Stat(savePath); os.IsNotExist(err) {
		t.Skipf("test save not found: %s", savePath)
	}
	save, err := core.LoadSave(savePath)
	if err != nil {
		t.Fatalf("LoadSave: %v", err)
	}
	app := NewApp()
	app.save = save
	slotIdx := -1
	for i := range save.Slots {
		slot := &save.Slots[i]
		if slot.Version == 0 || len(core.ScanDuplicateInventoryIndices(slot)) > 0 || len(core.ScanDuplicateWondrousPhysick(slot)) > 0 {
			continue
		}
		filled, empty := countPhysick(slot)
		if filled == 0 && empty == 0 {
			slotIdx = i
			break
		}
	}
	if slotIdx < 0 {
		t.Skip("no loaded slot without existing Physick")
	}

	result, err := app.AddItemsToCharacter(slotIdx, []uint32{db.ItemFlaskWondrousPhysickFilled}, 0, 0, 0, 0, 1, 0)
	if err != nil {
		t.Fatalf("AddItemsToCharacter: %v", err)
	}
	if result.Added != 1 {
		t.Fatalf("Added = %d, want 1", result.Added)
	}
	filled, empty := countPhysick(&app.save.Slots[slotIdx])
	if filled != 0 || empty != 1 {
		t.Fatalf("Physick rows filled=%d empty=%d, want 0/1", filled, empty)
	}
}

func TestAddItemsToCharacter_BlocksPhysickWhenFilledExists(t *testing.T) {
	app := repairFixture([]core.InventoryItem{
		{GaItemHandle: 0xB00000FA, Quantity: 1, Index: 100},
	}, nil)

	result, err := app.AddItemsToCharacter(0, []uint32{db.ItemFlaskWondrousPhysickEmpty}, 0, 0, 0, 0, 1, 0)
	if err != nil {
		t.Fatalf("AddItemsToCharacter: %v", err)
	}
	if result.Added != 0 {
		t.Fatalf("Added = %d, want 0", result.Added)
	}
	filled, empty := countPhysick(&app.save.Slots[0])
	if filled != 1 || empty != 0 {
		t.Fatalf("Physick rows filled=%d empty=%d, want 1/0", filled, empty)
	}
}

func TestAddItemsToCharacter_BlocksPhysickWhenEmptyExists(t *testing.T) {
	app := repairFixture([]core.InventoryItem{
		{GaItemHandle: 0xB00000FB, Quantity: 1, Index: 100},
	}, nil)

	result, err := app.AddItemsToCharacter(0, []uint32{db.ItemFlaskWondrousPhysickEmpty}, 0, 0, 0, 0, 1, 0)
	if err != nil {
		t.Fatalf("AddItemsToCharacter: %v", err)
	}
	if result.Added != 0 {
		t.Fatalf("Added = %d, want 0", result.Added)
	}
	filled, empty := countPhysick(&app.save.Slots[0])
	if filled != 0 || empty != 1 {
		t.Fatalf("Physick rows filled=%d empty=%d, want 0/1", filled, empty)
	}
}
