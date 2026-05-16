package main

import (
	"strings"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
)

// dupIndexFixture builds a minimal App whose slot 0 already contains two
// non-empty CommonItems sharing the same acquisition Index — the exact shape
// the scanner is meant to refuse before AddItemsToCharacter mutates anything.
func dupIndexFixture() *App {
	app := NewApp()
	app.save = &core.SaveFile{}
	slot := &app.save.Slots[0]
	slot.Version = 1
	slot.Inventory.CommonItems = []core.InventoryItem{
		{GaItemHandle: 0xB0000334, Quantity: 99, Index: 552},
		{GaItemHandle: 0xB00003C0, Quantity: 99, Index: 552},
	}
	slot.Inventory.NextAcquisitionSortId = 600
	slot.Inventory.NextEquipIndex = 600
	return app
}

func TestAddItemsToCharacter_PreflightAbortsOnDuplicateIndex(t *testing.T) {
	app := dupIndexFixture()
	slot := &app.save.Slots[0]
	originalCommonLen := len(slot.Inventory.CommonItems)
	originalIndex0 := slot.Inventory.CommonItems[0].Index
	originalIndex1 := slot.Inventory.CommonItems[1].Index
	originalNextAcq := slot.Inventory.NextAcquisitionSortId

	res, err := app.AddItemsToCharacter(0, []uint32{0x000F4240}, 0, 0, 0, 0, 1, 0)
	if err == nil {
		t.Fatalf("expected pre-flight error, got nil (result=%+v)", res)
	}

	msg := err.Error()
	for _, want := range []string{
		"duplicate acquisition index",
		"Index 552",
		"inventory_common",
		"0xB00003C0",
		"repair",
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("error message missing %q: %s", want, msg)
		}
	}

	if res.Added != 0 {
		t.Errorf("Added should be 0 on pre-flight abort, got %d", res.Added)
	}

	// Slot must be untouched: no snapshot was taken, no items appended,
	// counters not advanced.
	if got := len(slot.Inventory.CommonItems); got != originalCommonLen {
		t.Errorf("CommonItems length changed: %d → %d", originalCommonLen, got)
	}
	if slot.Inventory.CommonItems[0].Index != originalIndex0 ||
		slot.Inventory.CommonItems[1].Index != originalIndex1 {
		t.Errorf("inventory indices were mutated by aborted call")
	}
	if slot.Inventory.NextAcquisitionSortId != originalNextAcq {
		t.Errorf("NextAcquisitionSortId advanced despite aborted call: %d → %d",
			originalNextAcq, slot.Inventory.NextAcquisitionSortId)
	}
}
