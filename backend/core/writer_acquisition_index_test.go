package core

import "testing"

const (
	massAddBatchSize  = 328
	massAddBatchCount = 2
)

// TestAddToInventory_MassBatchesUseDistinctAcquisitionBuckets reproduces the
// four app batches from the Steam Deck regression: two 328-weapon batches to
// Inventory and two to Storage. AddItemsToSlotBatch inserts its pending items
// through addToInventory, so this isolates the exact record-writing path while
// keeping the fixture independent from GaItem allocation.
//
// Elden Ring sorts acquisition order by Index >> 1. Therefore a normal batch
// must allocate a distinct shifted bucket for every added record, not merely a
// distinct raw Index value.
func TestAddToInventory_MassBatchesUseDistinctAcquisitionBuckets(t *testing.T) {
	containers := []struct {
		name      string
		slot      *SaveSlot
		isStorage bool
		items     func(*SaveSlot) []InventoryItem
	}{
		{
			name:  "inventory",
			slot:  buildNextEquipFixture(t, nil, 420, 312),
			items: func(slot *SaveSlot) []InventoryItem { return slot.Inventory.CommonItems },
		},
		{
			name:      "storage",
			slot:      buildStorageFixture(t, 0, 1),
			isStorage: true,
			items:     func(slot *SaveSlot) []InventoryItem { return slot.Storage.CommonItems },
		},
	}

	for _, tc := range containers {
		t.Run(tc.name, func(t *testing.T) {
			preNextEquip := func() uint32 {
				if tc.isStorage {
					return tc.slot.Storage.NextEquipIndex
				}
				return tc.slot.Inventory.NextEquipIndex
			}()
			for batch := 0; batch < massAddBatchCount; batch++ {
				for item := 0; item < massAddBatchSize; item++ {
					handle := ItemTypeWeapon | uint32(batch*massAddBatchSize+item+1)
					if err := addToInventory(tc.slot, handle, 1, tc.isStorage, false); err != nil {
						t.Fatalf("batch %d item %d: addToInventory: %v", batch, item, err)
					}
				}
			}

			assertDistinctAcquisitionBuckets(t, tc.items(tc.slot))
			if tc.isStorage {
				if tc.slot.Storage.NextEquipIndex != preNextEquip {
					t.Fatalf("NextEquipIndex changed: got %d, want preserved %d", tc.slot.Storage.NextEquipIndex, preNextEquip)
				}
			} else if tc.slot.Inventory.NextEquipIndex != preNextEquip {
				t.Fatalf("NextEquipIndex changed: got %d, want preserved %d", tc.slot.Inventory.NextEquipIndex, preNextEquip)
			}
		})
	}
}

func assertDistinctAcquisitionBuckets(t *testing.T, items []InventoryItem) {
	t.Helper()

	seen := make(map[uint32]int)
	for row, item := range items {
		if item.GaItemHandle == GaHandleEmpty || item.GaItemHandle == GaHandleInvalid {
			continue
		}
		if item.Index <= InvEquipReservedMax {
			t.Fatalf("row %d: Index %d is inside reserved equipment range", row, item.Index)
		}

		bucket := item.Index >> 1
		if previousRow, exists := seen[bucket]; exists {
			t.Fatalf("Index >> 1 collision in bucket %d: rows %d and %d have raw indexes %d and %d",
				bucket, previousRow, row, items[previousRow].Index, item.Index)
		}
		seen[bucket] = row
	}
}
