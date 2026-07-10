package main

import (
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
	"github.com/oisis/EldenRing-SaveForge/backend/db"
)

// countItemID reports how many CommonItems rows resolve to the given item ID.
func countItemID(slot *core.SaveSlot, itemID uint32) int {
	n := 0
	for _, item := range slot.Inventory.CommonItems {
		if db.HandleToItemID(item.GaItemHandle) == itemID {
			n++
		}
	}
	return n
}

// TestAddItemsToCharacter_SkipsFlaskFamilyWhenUpgradedExists proves adding the
// +0 Crimson/Cerulean picker row is skipped (reported via SkippedExisting, not
// duplicated) when the save already holds an upgraded level of the same family.
// A non-flask stackable is never skipped by this guard: FlaskFamilyBaseID
// returns false for it (asserted in backend/db flask_family_test.go).
func TestAddItemsToCharacter_SkipsFlaskFamilyWhenUpgradedExists(t *testing.T) {
	cases := []struct {
		name       string
		existing   uint32 // goods handle already in inventory
		existingID uint32 // resolved item ID of the existing upgraded flask
		addID      uint32 // +0 DB picker row requested
	}{
		{"Crimson +12 blocks +0 add", 0xB0000401, 0x40000401, db.ItemFlaskCrimsonBase},
		{"Cerulean +12 blocks +0 add", 0xB0000433, 0x40000433, db.ItemFlaskCeruleanBase},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			app := repairFixture([]core.InventoryItem{
				{GaItemHandle: c.existing, Quantity: 1, Index: 100},
			}, nil)

			result, err := app.AddItemsToCharacter(0, []uint32{c.addID}, 0, 0, 0, 0, 1, 0)
			if err != nil {
				t.Fatalf("AddItemsToCharacter: %v", err)
			}
			if result.Added != 0 {
				t.Fatalf("Added = %d, want 0 (family already present)", result.Added)
			}
			if len(result.SkippedExisting) != 1 || result.SkippedExisting[0].ItemID != c.addID {
				t.Fatalf("SkippedExisting = %+v, want [{ItemID: 0x%08X}]", result.SkippedExisting, c.addID)
			}
			// Original upgraded flask untouched; no +0 row created.
			if got := countItemID(&app.save.Slots[0], c.existingID); got != 1 {
				t.Fatalf("upgraded flask rows = %d, want 1 (unchanged)", got)
			}
			if got := countItemID(&app.save.Slots[0], c.addID); got != 0 {
				t.Fatalf("+0 flask rows = %d, want 0 (skipped)", got)
			}
		})
	}
}
