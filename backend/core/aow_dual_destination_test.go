package core

import (
	"os"
	"testing"
)

// TestNonStackableDualDestinationUniqueHandles regresses the AoW shared-handle
// bug. When a non-stackable item (weapon/armor/AoW) is added with both
// invQty>0 AND storageQty>0, each destination must receive its OWN GaItem
// record with a distinct handle. Prior behavior allocated a single GaItem and
// wrote the same handle to both lists, causing the game to treat both list
// entries as the same physical item — equipping an AoW on one weapon would
// propagate to the duplicate inventory entry, and the post-game-load save
// cycle pushed duplicates to indices >= next_equip_index, making them
// invisible in-game.
func TestNonStackableDualDestinationUniqueHandles(t *testing.T) {
	savePath := "../../tmp/save/ER0000.sl2"
	if _, err := os.Stat(savePath); os.IsNotExist(err) {
		t.Skipf("Test save not found: %s", savePath)
	}
	save, err := LoadSave(savePath)
	if err != nil {
		t.Fatalf("LoadSave: %v", err)
	}

	var slot *SaveSlot
	for i := 0; i < 10; i++ {
		s := &save.Slots[i]
		if s.Version == 0 {
			continue
		}
		emptyCount := 0
		for _, ga := range s.GaItems {
			if ga.IsEmpty() {
				emptyCount++
			}
		}
		if emptyCount > 100 {
			slot = s
			break
		}
	}
	if slot == nil {
		t.Skip("no active slot with sufficient empty GaItem capacity")
	}

	// AoW item ID — Lion's Claw 0x80002710.
	aowID := uint32(0x80002710)

	cases := []struct {
		name      string
		runBatch  bool
		invQty    int
		storeQty  int
		wantInv   bool
		wantStore bool
	}{
		{"single-batch-inv-and-storage", true, 1, 1, true, true},
		{"single-call-inv-and-storage", false, 1, 1, true, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Snapshot pre-state so we can isolate the new entries this test created.
			invBefore := snapshotHandles(slot.Inventory.CommonItems)
			stoBefore := snapshotHandles(slot.Storage.CommonItems)

			if tc.runBatch {
				items := []ItemToAdd{{
					ItemID:     aowID,
					InvQty:     tc.invQty,
					StorageQty: tc.storeQty,
				}}
				if err := AddItemsToSlotBatch(slot, items); err != nil {
					t.Fatalf("AddItemsToSlotBatch: %v", err)
				}
			} else {
				if err := AddItemsToSlot(slot, []uint32{aowID}, tc.invQty, tc.storeQty, false); err != nil {
					t.Fatalf("AddItemsToSlot: %v", err)
				}
			}

			newInv := newHandlesOf(slot.Inventory.CommonItems, invBefore, aowID, slot)
			newStore := newHandlesOf(slot.Storage.CommonItems, stoBefore, aowID, slot)

			if tc.wantInv && len(newInv) != 1 {
				t.Errorf("expected 1 new inventory entry for AoW 0x%X, got %d (handles=%v)",
					aowID, len(newInv), newInv)
			}
			if tc.wantStore && len(newStore) != 1 {
				t.Errorf("expected 1 new storage entry for AoW 0x%X, got %d (handles=%v)",
					aowID, len(newStore), newStore)
			}
			if len(newInv) == 1 && len(newStore) == 1 {
				if newInv[0] == newStore[0] {
					t.Errorf("non-stackable AoW shares handle 0x%08X between inventory and storage — must be distinct GaItems",
						newInv[0])
				}
			}

			// Verify both new handles appear in GaItems with the same ItemID.
			invFound, stoFound := false, false
			for _, g := range slot.GaItems {
				if g.IsEmpty() {
					continue
				}
				if len(newInv) == 1 && g.Handle == newInv[0] && g.ItemID == aowID {
					invFound = true
				}
				if len(newStore) == 1 && g.Handle == newStore[0] && g.ItemID == aowID {
					stoFound = true
				}
			}
			if tc.wantInv && !invFound {
				t.Errorf("inventory GaItem for handle 0x%08X (itemID 0x%X) not found in GaItems array", newInv, aowID)
			}
			if tc.wantStore && !stoFound {
				t.Errorf("storage GaItem for handle 0x%08X (itemID 0x%X) not found in GaItems array", newStore, aowID)
			}
		})
	}
}

func snapshotHandles(items []InventoryItem) map[uint32]struct{} {
	m := make(map[uint32]struct{}, len(items))
	for _, it := range items {
		if it.GaItemHandle != 0 && it.GaItemHandle != 0xFFFFFFFF {
			m[it.GaItemHandle] = struct{}{}
		}
	}
	return m
}

func newHandlesOf(items []InventoryItem, before map[uint32]struct{}, itemID uint32, slot *SaveSlot) []uint32 {
	var out []uint32
	for _, it := range items {
		if it.GaItemHandle == 0 || it.GaItemHandle == 0xFFFFFFFF {
			continue
		}
		if _, was := before[it.GaItemHandle]; was {
			continue
		}
		// Only consider entries whose backing GaItem has the requested itemID.
		// (The same handle may appear elsewhere; we want the one we just added.)
		if slot.GaMap[it.GaItemHandle] != itemID {
			continue
		}
		out = append(out, it.GaItemHandle)
	}
	return out
}
