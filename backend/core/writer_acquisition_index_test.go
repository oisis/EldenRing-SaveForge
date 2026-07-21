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
		floor     uint32
		items     func(*SaveSlot) []InventoryItem
	}{
		{
			name:  "inventory",
			slot:  buildNextEquipFixture(t, nil, 420, 312),
			floor: InvEquipReservedMax,
			items: func(slot *SaveSlot) []InventoryItem { return slot.Inventory.CommonItems },
		},
		{
			// buildStorageFixture(t, 0, 1) matches the T310 empty-Storage signature
			// (NextEquipIndex=0, NextAcquisitionSortId=1), so the first insert here
			// exercises the confirmed empty-init contract (Index=2), while the rest
			// of the batch exercises the T320-pending stride-2 hypothesis. Storage
			// has no reserved equipment range, so its floor is 2, not
			// InvEquipReservedMax (that constant is Inventory-specific).
			name:      "storage",
			slot:      buildStorageFixture(t, 0, 1),
			isStorage: true,
			floor:     1,
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
			// Mirrors AddItemsToSlotBatch: decided once, from pre-loop state, not
			// re-derived per insert (see writer.go's storageBatchStartedEmpty doc).
			storageBatchStartedEmpty := tc.isStorage && len(tc.slot.Storage.CommonItems) == 0 &&
				tc.slot.Storage.NextAcquisitionSortId <= 1 && tc.slot.Storage.NextEquipIndex == 0
			for batch := 0; batch < massAddBatchCount; batch++ {
				for item := 0; item < massAddBatchSize; item++ {
					handle := ItemTypeWeapon | uint32(batch*massAddBatchSize+item+1)
					if err := addToInventory(tc.slot, handle, 1, tc.isStorage, false, storageBatchStartedEmpty); err != nil {
						t.Fatalf("batch %d item %d: addToInventory: %v", batch, item, err)
					}
				}
			}

			assertDistinctAcquisitionBuckets(t, tc.items(tc.slot), tc.floor)
			if tc.isStorage {
				// T310: an empty Storage's first direct-add record jumps NextEquipIndex
				// to 128 as a one-time initialization. T330: every insert after that,
				// including later inserts in the same batch, advances it by exactly 1.
				wantEquip := uint32(128) + uint32(massAddBatchSize*massAddBatchCount-1)
				if tc.slot.Storage.NextEquipIndex != wantEquip {
					t.Fatalf("NextEquipIndex: got %d, want %d (T310 init 128 + %d later inserts, preserved from %d)",
						tc.slot.Storage.NextEquipIndex, wantEquip, massAddBatchSize*massAddBatchCount-1, preNextEquip)
				}
			} else {
				// T050/T210: each new Inventory.CommonItems record advances NextEquipIndex
				// by exactly one, so a batch of N new records advances it by exactly N.
				wantEquip := preNextEquip + uint32(massAddBatchSize*massAddBatchCount)
				if tc.slot.Inventory.NextEquipIndex != wantEquip {
					t.Fatalf("NextEquipIndex: got %d, want %d (preserved %d + %d new records)",
						tc.slot.Inventory.NextEquipIndex, wantEquip, preNextEquip, massAddBatchSize*massAddBatchCount)
				}
			}
		})
	}
}

func assertDistinctAcquisitionBuckets(t *testing.T, items []InventoryItem, floor uint32) {
	t.Helper()

	seen := make(map[uint32]int)
	for row, item := range items {
		if item.GaItemHandle == GaHandleEmpty || item.GaItemHandle == GaHandleInvalid {
			continue
		}
		if item.Index <= floor {
			t.Fatalf("row %d: Index %d is inside reserved range (floor %d)", row, item.Index, floor)
		}

		bucket := item.Index >> 1
		if previousRow, exists := seen[bucket]; exists {
			t.Fatalf("Index >> 1 collision in bucket %d: rows %d and %d have raw indexes %d and %d",
				bucket, previousRow, row, items[previousRow].Index, item.Index)
		}
		seen[bucket] = row
	}
}
