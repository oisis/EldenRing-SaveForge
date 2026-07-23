package core

import (
	"encoding/binary"
	"testing"
)

// E8b — canonicalization of the AoW partition after a duplicate GaItem repair.
//
// When the removed duplicate is an Ash of War that sat before a later AoW, the
// freed slot lands inside the AoW block. RebuildSlotFull serializes slot.GaItems
// linearly, so the table must be reordered into the native CSGaitem::WriteArray
// fixed point (all non-empty AoW first, then everything else — the freed empty
// included — in relative order) before the rebuild. Unrelated compaction is not
// used because it would remove every empty and fail to reproduce the native
// second-pass ordering of the freed slot relative to the trailing Weapon.

const (
	aowDupHandle   = uint32(ItemTypeAow | 0x0110)    // 0xC0000110 — shared AoW handle
	aowOtherHandle = uint32(ItemTypeAow | 0x0120)    // 0xC0000120 — a later, distinct AoW
	aowWeaponH     = uint32(ItemTypeWeapon | 0x0300) // 0x80000300 — highest counter (armament cursor anchor)
	aowDupLowID    = uint32(0x80000010)              // AoW itemID: high nibble 0x8
	aowDupHighID   = uint32(0x80000011)              // AoW itemID: high nibble 0x8
	aowOtherID     = uint32(0x80000020)              // AoW itemID: high nibble 0x8
	aowWeaponID    = uint32(0x0000F300)              // weapon itemID: high nibble 0x0
)

// duplicateAoWGaItemFixture builds the E8b reproduction: two non-empty AoW
// records share one handle (distinct ItemIDs) and sit before a later AoW and a
// Weapon. The dup pair is at indexes [0]/[1], so whichever the user removes, the
// freed slot is inside the AoW block. The slot is otherwise healthy (the
// structural scan reports only duplicate_handle). Nothing here reads
// or writes a user save file.
//
//	[0] AoW dup A   handle aowDupHandle, itemID aowDupLowID
//	[1] AoW dup B   handle aowDupHandle, itemID aowDupHighID
//	[2] AoW other   handle aowOtherHandle
//	[3] Weapon      handle aowWeaponH (highest counter)
func duplicateAoWGaItemFixture(t *testing.T) *SaveSlot {
	t.Helper()

	slot := gaItemStructuralFixture()
	leading := []GaItemFull{
		{Handle: aowDupHandle, ItemID: aowDupLowID},
		{Handle: aowDupHandle, ItemID: aowDupHighID},
		{Handle: aowOtherHandle, ItemID: aowOtherID},
		{Handle: aowWeaponH, ItemID: aowWeaponID, AoWGaItemHandle: NoCustomAoWHandle},
	}

	slot.GaItems = make([]GaItemFull, GaItemCountNew)
	copy(slot.GaItems, leading)

	gaBytes := 0
	for i := range slot.GaItems {
		gaBytes += slot.GaItems[i].ByteSize()
	}
	slot.MagicOffset = GaItemsStart + gaBytes + DynPlayerData - 1
	slot.Data = make([]byte, SlotSize)
	slot.Version = uint32(GaItemVersionBreak + 1)
	binary.LittleEndian.PutUint32(slot.Data, slot.Version)
	copy(slot.Data[slot.MagicOffset:], MagicPattern)

	pos := GaItemsStart
	for i := range slot.GaItems {
		pos += slot.GaItems[i].Serialize(slot.Data[pos:])
	}
	if pos != slot.MagicOffset-DynPlayerData+1 {
		t.Fatalf("GaItem fixture end=0x%X, want 0x%X", pos, slot.MagicOffset-DynPlayerData+1)
	}

	// One inventory reference to the duplicated AoW handle; the weapon sits in storage.
	slot.Inventory.CommonItems = []InventoryItem{{GaItemHandle: aowDupHandle, Quantity: 1, Index: 100}}
	slot.Inventory.KeyItems = nil
	slot.Storage.CommonItems = []InventoryItem{{GaItemHandle: aowWeaponH, Quantity: 1, Index: 200}}

	if err := slot.calculateDynamicOffsets(); err != nil {
		t.Fatalf("calculateDynamicOffsets: %v", err)
	}
	writeFixtureInventory(slot, slot.Inventory.CommonItems)
	writeFixtureStorage(slot, slot.Storage.CommonItems)
	if err := slot.buildSectionMap(); err != nil {
		t.Fatalf("buildSectionMap: %v", err)
	}
	for i := 0; i < ChrAsmEquipmentSize; i++ {
		slot.Data[slot.EquipItemsIDOffset+i] = byte(i + 1)
	}
	binary.LittleEndian.PutUint64(slot.Data[slot.GaItemDataOffset:], 0)

	if err := slot.parseFromData(); err != nil {
		t.Fatalf("parseFromData: %v", err)
	}
	return slot
}

// nonEmptyHandles returns the handles of the leading non-empty records, in order.
func nonEmptyHandles(slot *SaveSlot) []uint32 {
	var out []uint32
	for i := range slot.GaItems {
		if !slot.GaItems[i].IsEmpty() {
			out = append(out, slot.GaItems[i].Handle)
		}
	}
	return out
}

// TestRepairGaItemDuplicate_AoWPartitionCanonicalized covers E8b required tests
// 1-4: removing the AoW duplicate before another AoW and a Weapon leaves all AoWs
// compacted at the front in preserved order, with the freed empty and the Weapon
// in the order the native second pass emits, and a slot that reparses with a
// correct GaMap, cursors, and container references.
func TestRepairGaItemDuplicate_AoWPartitionCanonicalized(t *testing.T) {
	base := duplicateAoWGaItemFixture(t)
	lo, hi := candidateIndexesFor(t, base, aowDupHandle)

	cases := []struct {
		name       string
		keepIndex  int
		keepHandle uint32
		wantItemID uint32
		// After repair the leading AoW block is [kept dup AoW, other AoW]; the
		// freed slot is empty (dropped from the handle list) and the Weapon trails.
	}{
		{"keep_lower_index", lo, aowDupHandle, aowDupLowID},
		{"keep_higher_index", hi, aowDupHandle, aowDupHighID},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			slot := duplicateAoWGaItemFixture(t)
			keptRecord := slot.GaItems[tc.keepIndex]

			if err := RepairGaItemDuplicate(slot, aowDupHandle, tc.keepIndex); err != nil {
				t.Fatalf("RepairGaItemDuplicate: %v", err)
			}

			// All non-empty AoWs are compacted to the front in preserved order,
			// followed by the Weapon. The freed empty is gone from the handle list.
			gotHandles := nonEmptyHandles(slot)
			wantHandles := []uint32{aowDupHandle, aowOtherHandle, aowWeaponH}
			if len(gotHandles) != len(wantHandles) {
				t.Fatalf("non-empty handles = %#x, want %#x", gotHandles, wantHandles)
			}
			for i := range wantHandles {
				if gotHandles[i] != wantHandles[i] {
					t.Fatalf("non-empty handles = %#x, want %#x", gotHandles, wantHandles)
				}
			}

			// Native second-pass ordering: the freed slot (originally at index
			// lo/hi, before the Weapon) stays before the Weapon. So the compacted
			// table is [AoW, AoW, empty, Weapon, ...] — an empty sits at index 2.
			if slot.GaItems[0].Handle != aowDupHandle || slot.GaItems[1].Handle != aowOtherHandle {
				t.Fatalf("leading AoW block = [0x%08X, 0x%08X], want [0x%08X, 0x%08X]",
					slot.GaItems[0].Handle, slot.GaItems[1].Handle, aowDupHandle, aowOtherHandle)
			}
			if !slot.GaItems[2].IsEmpty() {
				t.Errorf("index 2 should hold the freed empty from the native second pass, got %+v", slot.GaItems[2])
			}
			if slot.GaItems[3].Handle != aowWeaponH {
				t.Errorf("Weapon should trail the freed empty at index 3, got 0x%08X", slot.GaItems[3].Handle)
			}

			// exactly one physical record remains for the handle, and it is kept.
			remaining := 0
			for i := range slot.GaItems {
				if !slot.GaItems[i].IsEmpty() && slot.GaItems[i].Handle == aowDupHandle {
					remaining++
					if slot.GaItems[i] != keptRecord {
						t.Errorf("surviving record != kept record: %+v vs %+v", slot.GaItems[i], keptRecord)
					}
				}
			}
			if remaining != 1 {
				t.Fatalf("remaining records for handle = %d, want 1", remaining)
			}

			// GaMap, cursors, container references, and structural checks are all sound.
			if slot.GaMap[aowDupHandle] != tc.wantItemID {
				t.Errorf("GaMap[dup]=0x%08X, want 0x%08X", slot.GaMap[aowDupHandle], tc.wantItemID)
			}
			if slot.GaMap[aowOtherHandle] != aowOtherID || slot.GaMap[aowWeaponH] != aowWeaponID {
				t.Errorf("GaMap other/weapon = 0x%08X/0x%08X, want 0x%08X/0x%08X",
					slot.GaMap[aowOtherHandle], slot.GaMap[aowWeaponH], aowOtherID, aowWeaponID)
			}
			if slot.NextAoWIndex != 2 {
				t.Errorf("NextAoWIndex=%d, want 2 (two compacted AoWs)", slot.NextAoWIndex)
			}
			if slot.NextArmamentIndex != 4 {
				t.Errorf("NextArmamentIndex=%d, want 4 (weapon carries the highest counter at index 3)", slot.NextArmamentIndex)
			}
			if slot.Inventory.CommonItems[0].GaItemHandle != aowDupHandle {
				t.Errorf("inventory ref changed: %+v", slot.Inventory.CommonItems)
			}
			if slot.Storage.CommonItems[0].GaItemHandle != aowWeaponH {
				t.Errorf("storage ref changed: %+v", slot.Storage.CommonItems)
			}
			if report := ScanGaItemStructuralIssues(slot); len(report.Issues) != 0 {
				t.Errorf("structural issues remain after repair: %+v", report.Issues)
			}
			if v := ValidatePostMutation(slot); len(v) != 0 {
				t.Errorf("post-mutation validation failed: %v", v)
			}
		})
	}
}

// TestRepairGaItemDuplicate_WeaponCaseNoReorder covers E8b required test 5: a
// Weapon/Armor duplicate with no AoW in the table repairs stably and performs no
// reorder — the non-removed records keep their exact indexes.
func TestRepairGaItemDuplicate_WeaponCaseNoReorder(t *testing.T) {
	slot := duplicateGaItemFixture(t)
	lo, hi := candidateIndexes(t, slot)
	before := SnapshotSlot(slot)

	if err := RepairGaItemDuplicate(slot, dupHandle, lo); err != nil {
		t.Fatalf("RepairGaItemDuplicate: %v", err)
	}

	// No AoW records, so canonicalizeAoWPartition is the identity: every record
	// except the removed one keeps its original index.
	for i := range slot.GaItems {
		if i == hi {
			if !slot.GaItems[i].IsEmpty() {
				t.Errorf("removed index %d is not empty", i)
			}
			continue
		}
		if slot.GaItems[i] != before.GaItems[i] {
			t.Errorf("GaItems[%d] moved or changed: %+v -> %+v (unexpected reorder)", i, before.GaItems[i], slot.GaItems[i])
		}
	}
}

// candidateIndexesFor is candidateIndexes for an arbitrary duplicate handle.
func candidateIndexesFor(t *testing.T, slot *SaveSlot, handle uint32) (int, int) {
	t.Helper()
	analysis := AnalyzeGaItemDuplicate(slot, handle)
	if !analysis.Repairable {
		t.Fatalf("fixture not repairable: %s: %s", analysis.RefusalCode, analysis.RefusalMsg)
	}
	return analysis.Candidates[0].Index, analysis.Candidates[1].Index
}
