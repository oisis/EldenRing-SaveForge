package main

import (
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
)

const (
	removeTestKick           = uint32(0x8000C47C)
	removeTestNoSkill        = uint32(0x800078B4)
	removeTestImpalingThrust = uint32(0x80002774)
	removeTestFlail          = uint32(0x00C68450)
	removeTestLordsworn      = uint32(0x002E3BF0)
	removeTestWarCry         = uint32(0x8000FE4C)
)

func removeTestHandle(t *testing.T, slot *core.SaveSlot, itemID uint32) uint32 {
	t.Helper()
	for handle, mappedID := range slot.GaMap {
		if mappedID == itemID {
			return handle
		}
	}
	t.Fatalf("item 0x%08X has no handle", itemID)
	return 0
}

func removeTestHasGaItem(slot *core.SaveSlot, itemID uint32) bool {
	for _, item := range slot.GaItems {
		if !item.IsEmpty() && item.ItemID == itemID {
			return true
		}
	}
	return false
}

func TestRemoveItemsFromCharacter_ReprojectsAoWBeforeNextAdd(t *testing.T) {
	tests := []struct {
		name   string
		remove func(t *testing.T, app *App, flail, kick uint32)
	}{
		{
			name: "individual removals",
			remove: func(t *testing.T, app *App, flail, kick uint32) {
				if err := app.RemoveItemsFromCharacter(0, []uint32{flail}, true, true); err != nil {
					t.Fatalf("remove Flail: %v", err)
				}
				if err := app.RemoveItemsFromCharacter(0, []uint32{kick}, true, true); err != nil {
					t.Fatalf("remove Kick: %v", err)
				}
			},
		},
		{
			name: "batch removal",
			remove: func(t *testing.T, app *App, flail, kick uint32) {
				if err := app.RemoveItemsFromCharacter(0, []uint32{flail, kick}, true, true); err != nil {
					t.Fatalf("remove batch: %v", err)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			app := gaItemAddApp(t, true)
			initial := []uint32{
				removeTestKick,
				removeTestNoSkill,
				removeTestImpalingThrust,
				removeTestFlail,
			}
			if _, err := app.AddItemsToCharacter(0, initial, 0, 0, 0, 0, 1, 0); err != nil {
				t.Fatalf("seed items: %v", err)
			}

			slot := &app.save.Slots[0]
			kickHandle := removeTestHandle(t, slot, removeTestKick)
			flailHandle := removeTestHandle(t, slot, removeTestFlail)
			tc.remove(t, app, flailHandle, kickHandle)

			if removeTestHasGaItem(slot, removeTestKick) {
				t.Fatal("Kick GaItem survived removal")
			}
			if got := slot.GaItems[0].ItemID; got != removeTestNoSkill {
				t.Fatalf("GaItems[0] itemID = 0x%08X, want No Skill", got)
			}
			if got := slot.GaItems[1].ItemID; got != removeTestImpalingThrust {
				t.Fatalf("GaItems[1] itemID = 0x%08X, want Impaling Thrust", got)
			}
			if _, err := core.NativeGaItemCapacity(slot); err != nil {
				t.Fatalf("native layout after removal: %v", err)
			}

			if _, err := app.AddItemsToCharacter(0, []uint32{removeTestLordsworn}, 0, 0, 0, 0, 1, 0); err != nil {
				t.Fatalf("add Lordsworn after removal: %v", err)
			}
			if _, err := app.AddItemsToCharacter(0, []uint32{removeTestWarCry}, 0, 0, 0, 0, 1, 0); err != nil {
				t.Fatalf("add War Cry after removal: %v", err)
			}
			if !removeTestHasGaItem(slot, removeTestLordsworn) || !removeTestHasGaItem(slot, removeTestWarCry) {
				t.Fatal("new physical items are missing after add")
			}
			if _, err := core.NativeGaItemCapacity(slot); err != nil {
				t.Fatalf("native layout after re-add: %v", err)
			}

			var reparsed core.SaveSlot
			if err := reparsed.Read(core.NewReader(slot.Data), ""); err != nil {
				t.Fatalf("reparse serialized slot: %v", err)
			}
			if removeTestHasGaItem(&reparsed, removeTestKick) {
				t.Fatal("serialized slot still contains the removed Kick GaItem")
			}
			if _, err := core.NativeGaItemCapacity(&reparsed); err != nil {
				t.Fatalf("serialized native layout: %v", err)
			}
		})
	}
}
