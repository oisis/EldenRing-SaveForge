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

// TestAddItemsToCharacter_ToleratesPreExistingDuplicateIndex pins the post-fix
// behaviour: the game tolerates pre-existing duplicate acquisition indices, so
// the pre-flight guard must NO LONGER abort (and must not demand the destructive
// repair). The minimal fixture has no backing slot.Data, so the add fails later
// for an unrelated reason — that is fine; we only assert the guard's behaviour
// changed and the pre-existing duplicate records are left untouched.
func TestAddItemsToCharacter_ToleratesPreExistingDuplicateIndex(t *testing.T) {
	app := dupIndexFixture()
	slot := &app.save.Slots[0]

	_, err := app.AddItemsToCharacter(0, []uint32{0x000F4240}, 0, 0, 0, 0, 1, 0)

	// The duplicate guard must not fire anymore.
	if err != nil {
		msg := err.Error()
		for _, forbidden := range []string{"duplicate acquisition index", "Run inventory index repair"} {
			if strings.Contains(msg, forbidden) {
				t.Fatalf("guard still aborts on pre-existing duplicates: %s", msg)
			}
		}
	}

	// The pre-existing duplicate records must be left exactly as they were —
	// tolerated, never renumbered.
	if slot.Inventory.CommonItems[0].Index != 552 || slot.Inventory.CommonItems[1].Index != 552 {
		t.Errorf("pre-existing duplicate indices were altered: %d, %d",
			slot.Inventory.CommonItems[0].Index, slot.Inventory.CommonItems[1].Index)
	}
}
