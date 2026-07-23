package core

import (
	"bytes"
	"encoding/binary"
	"reflect"
	"testing"
)

// duplicateGaItemFixture builds a fully serializable slot whose GaItem table
// contains exactly the Task 7.9b reproduction shape: two non-empty physical
// records sharing one weapon handle but carrying different ItemIDs, with a single
// inventory reference to that handle. The slot is otherwise healthy (the
// structural scan reports only duplicate_handle). Nothing here reads
// or writes a user save file.
//
// Layout of the leading GaItems:
//
//	[0] armor          (highest counter, keeps NextGaItemHandle stable)
//	[1] weapon dup A   handle H, itemID lowItemID
//	[2] weapon dup B   handle H, itemID highItemID
const (
	dupHandle     = uint32(ItemTypeWeapon | 0x0102) // 0x80000102
	dupArmorH     = uint32(ItemTypeArmor | 0x0200)  // higher counter than the dup pair
	dupLowItemID  = uint32(0x000F4240)
	dupHighItemID = uint32(0x000F4241)
	dupArmorItem  = uint32(0x10000001)
)

func duplicateGaItemFixture(t *testing.T) *SaveSlot {
	t.Helper()

	slot := gaItemStructuralFixture()
	leading := []GaItemFull{
		{Handle: dupArmorH, ItemID: dupArmorItem, Unk2: 20, Unk3: 21},
		{Handle: dupHandle, ItemID: dupLowItemID, Unk2: 10, Unk3: 11, AoWGaItemHandle: NoCustomAoWHandle, Unk5: 1},
		{Handle: dupHandle, ItemID: dupHighItemID, Unk2: 12, Unk3: 13, AoWGaItemHandle: NoCustomAoWHandle, Unk5: 2},
	}

	gaItemCount := GaItemCountNew
	version := uint32(GaItemVersionBreak + 1)
	slot.GaItems = make([]GaItemFull, gaItemCount)
	copy(slot.GaItems, leading)

	gaBytes := 0
	for i := range slot.GaItems {
		gaBytes += slot.GaItems[i].ByteSize()
	}
	slot.MagicOffset = GaItemsStart + gaBytes + DynPlayerData - 1
	slot.Data = make([]byte, SlotSize)
	slot.Version = version
	binary.LittleEndian.PutUint32(slot.Data, slot.Version)
	copy(slot.Data[slot.MagicOffset:], MagicPattern)

	pos := GaItemsStart
	for i := range slot.GaItems {
		pos += slot.GaItems[i].Serialize(slot.Data[pos:])
	}
	if pos != slot.MagicOffset-DynPlayerData+1 {
		t.Fatalf("GaItem fixture end=0x%X, want 0x%X", pos, slot.MagicOffset-DynPlayerData+1)
	}

	// One inventory reference to the duplicated handle; armor sits in storage.
	slot.Inventory.CommonItems = []InventoryItem{{GaItemHandle: dupHandle, Quantity: 7, Index: 100}}
	slot.Inventory.KeyItems = nil
	slot.Storage.CommonItems = []InventoryItem{{GaItemHandle: dupArmorH, Quantity: 1, Index: 200}}

	if err := slot.calculateDynamicOffsets(); err != nil {
		t.Fatalf("calculateDynamicOffsets: %v", err)
	}
	writeFixtureInventory(slot, slot.Inventory.CommonItems)
	writeFixtureStorage(slot, slot.Storage.CommonItems)
	if err := slot.buildSectionMap(); err != nil {
		t.Fatalf("buildSectionMap: %v", err)
	}
	// Non-zero equipment + GaItemData regions; equip values are small bytes that
	// cannot collide with the candidate ItemID signatures.
	for i := 0; i < ChrAsmEquipmentSize; i++ {
		slot.Data[slot.EquipItemsIDOffset+i] = byte(i + 1)
	}
	binary.LittleEndian.PutUint64(slot.Data[slot.GaItemDataOffset:], 0)

	if err := slot.parseFromData(); err != nil {
		t.Fatalf("parseFromData: %v", err)
	}
	return slot
}

// candidateIndexes returns the two physical indexes of the duplicate pair as the
// analysis reports them (ascending order).
func candidateIndexes(t *testing.T, slot *SaveSlot) (int, int) {
	t.Helper()
	analysis := AnalyzeGaItemDuplicate(slot, dupHandle)
	if !analysis.Repairable {
		t.Fatalf("fixture not repairable: %s: %s", analysis.RefusalCode, analysis.RefusalMsg)
	}
	return analysis.Candidates[0].Index, analysis.Candidates[1].Index
}

// TestAnalyzeGaItemDuplicate_ReproductionRequiresChoice covers required test 1:
// the reproduction shape is recognized, both candidates are returned with their
// distinct ItemIDs, and the analysis makes no choice on its own.
func TestAnalyzeGaItemDuplicate_ReproductionRequiresChoice(t *testing.T) {
	slot := duplicateGaItemFixture(t)
	analysis := AnalyzeGaItemDuplicate(slot, dupHandle)

	if !analysis.Repairable {
		t.Fatalf("want repairable reproduction, got refusal %s: %s", analysis.RefusalCode, analysis.RefusalMsg)
	}
	if analysis.Handle != dupHandle {
		t.Errorf("handle=0x%08X, want 0x%08X", analysis.Handle, dupHandle)
	}
	c0, c1 := analysis.Candidates[0], analysis.Candidates[1]
	if c0.Index >= c1.Index {
		t.Errorf("candidates not in ascending index order: %d, %d", c0.Index, c1.Index)
	}
	if c0.ItemID == c1.ItemID {
		t.Errorf("candidate ItemIDs must differ, both 0x%08X", c0.ItemID)
	}
	got := map[uint32]bool{c0.ItemID: true, c1.ItemID: true}
	if !got[dupLowItemID] || !got[dupHighItemID] {
		t.Errorf("candidate ItemIDs = %v, want %#x and %#x", got, dupLowItemID, dupHighItemID)
	}
}

// TestRepairGaItemDuplicate_KeepEachIndex covers required test 2: executing with
// each possible keep index, verifying every postcondition.
func TestRepairGaItemDuplicate_KeepEachIndex(t *testing.T) {
	base := duplicateGaItemFixture(t)
	lo, hi := candidateIndexes(t, base)

	cases := []struct {
		name       string
		keepIndex  int
		wantItemID uint32
	}{
		{"keep_lower_index", lo, dupLowItemID},
		{"keep_higher_index", hi, dupHighItemID},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			slot := duplicateGaItemFixture(t)
			keptRecord := slot.GaItems[tc.keepIndex]
			removeIndex := lo
			if tc.keepIndex == lo {
				removeIndex = hi
			}
			invBefore := append([]InventoryItem(nil), slot.Inventory.CommonItems...)

			if err := RepairGaItemDuplicate(slot, dupHandle, tc.keepIndex); err != nil {
				t.Fatalf("RepairGaItemDuplicate: %v", err)
			}

			// exactly one physical record remains for the handle, and it is kept.
			remaining := 0
			for i := range slot.GaItems {
				if !slot.GaItems[i].IsEmpty() && slot.GaItems[i].Handle == dupHandle {
					remaining++
					if i != tc.keepIndex {
						t.Errorf("handle survived at index %d, want %d", i, tc.keepIndex)
					}
				}
			}
			if remaining != 1 {
				t.Fatalf("remaining records for handle = %d, want 1", remaining)
			}
			if slot.GaItems[tc.keepIndex] != keptRecord {
				t.Errorf("kept record changed: %+v -> %+v", keptRecord, slot.GaItems[tc.keepIndex])
			}
			if !slot.GaItems[removeIndex].IsEmpty() {
				t.Errorf("removed record at %d is not empty", removeIndex)
			}
			// GaMap maps the handle to the kept ItemID.
			if slot.GaMap[dupHandle] != tc.wantItemID {
				t.Errorf("GaMap[handle]=0x%08X, want 0x%08X", slot.GaMap[dupHandle], tc.wantItemID)
			}
			// the sole reference keeps handle, quantity, and acquisition index.
			if !sameInventoryItems(slot.Inventory.CommonItems, invBefore) {
				t.Errorf("inventory reference changed: %+v -> %+v", invBefore, slot.Inventory.CommonItems)
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

// TestRepairGaItemDuplicate_NoAutomaticChoice covers required test 3: a missing
// or invalid keep index refuses without mutating the slot.
func TestRepairGaItemDuplicate_NoAutomaticChoice(t *testing.T) {
	for _, keep := range []int{-1, 999} {
		slot := duplicateGaItemFixture(t)
		before := SnapshotSlot(slot)
		if err := RepairGaItemDuplicate(slot, dupHandle, keep); err == nil {
			t.Fatalf("keepIndex %d: want refusal, got success", keep)
		}
		assertSlotUnchanged(t, slot, before)
	}
}

// TestAnalyzeGaItemDuplicate_RefusesWithoutMutation covers required test 4: every
// missing safety condition refuses, and analysis never mutates.
func TestAnalyzeGaItemDuplicate_RefusesWithoutMutation(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(t *testing.T, slot *SaveSlot)
		want   string
	}{
		{"single_record", func(t *testing.T, slot *SaveSlot) {
			// collapse the pair: give the second record a distinct handle so only
			// one physical record uses dupHandle.
			_, hi := candidateIndexes(t, slot)
			slot.GaItems[hi].Handle = ItemTypeWeapon | 0x0300
		}, "not_exactly_two_records"},
		{"identical_item_id", func(t *testing.T, slot *SaveSlot) {
			_, hi := candidateIndexes(t, slot)
			slot.GaItems[hi].ItemID = dupLowItemID
		}, "identical_item_id"},
		{"zero_references", func(t *testing.T, slot *SaveSlot) {
			slot.Inventory.CommonItems = nil
		}, "reference_count"},
		{"multiple_references", func(t *testing.T, slot *SaveSlot) {
			slot.Inventory.CommonItems = append(slot.Inventory.CommonItems,
				InventoryItem{GaItemHandle: dupHandle, Quantity: 1, Index: 101})
		}, "reference_count"},
		{"equipped_candidate", func(t *testing.T, slot *SaveSlot) {
			// weapon signature is itemID | ItemTypeWeapon
			binary.LittleEndian.PutUint32(slot.Data[slot.EquipItemsIDOffset:], dupLowItemID|ItemTypeWeapon)
		}, "candidate_equipped"},
		{"aow_reference", func(t *testing.T, slot *SaveSlot) {
			// make the armor slot a weapon that references the handle as its AoW
			slot.GaItems[0] = GaItemFull{Handle: ItemTypeWeapon | 0x0300, ItemID: 0x000F5000, AoWGaItemHandle: dupHandle}
		}, "aow_reference"},
		{"unrelated_blocker", func(t *testing.T, slot *SaveSlot) {
			// orphan GaMap entry for a weapon handle with no physical record
			slot.GaMap[ItemTypeWeapon|0x0900] = 0x000F6000
		}, "other_preflight_blocker"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			slot := duplicateGaItemFixture(t)
			tc.mutate(t, slot)
			before := SnapshotSlot(slot)

			analysis := AnalyzeGaItemDuplicate(slot, dupHandle)
			if analysis.Repairable {
				t.Fatalf("want refusal, got repairable")
			}
			if analysis.RefusalCode != tc.want {
				t.Errorf("refusal code = %q, want %q (msg: %s)", analysis.RefusalCode, tc.want, analysis.RefusalMsg)
			}
			assertSlotUnchanged(t, slot, before)
		})
	}
}

// TestAnalyzeGaItemDuplicate_FailsClosedWhenEquipmentUnavailable proves that an
// unreadable equipment section refuses repair (never assumes "unequipped") and
// leaves the slot untouched. Both an absent offset and an out-of-bounds offset
// must yield the equipment_state_unavailable refusal.
func TestAnalyzeGaItemDuplicate_FailsClosedWhenEquipmentUnavailable(t *testing.T) {
	cases := []struct {
		name   string
		offset int
	}{
		{"absent_offset", 0},
		{"out_of_bounds_offset", SlotSize - ChrAsmEquipmentSize + 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			slot := duplicateGaItemFixture(t)
			slot.EquipItemsIDOffset = tc.offset
			before := SnapshotSlot(slot)

			analysis := AnalyzeGaItemDuplicate(slot, dupHandle)
			if analysis.Repairable {
				t.Fatal("want refusal for unreadable equipment, got repairable")
			}
			if analysis.RefusalCode != "equipment_state_unavailable" {
				t.Errorf("refusal code = %q, want equipment_state_unavailable (msg: %s)", analysis.RefusalCode, analysis.RefusalMsg)
			}
			assertSlotUnchanged(t, slot, before)
		})
	}
}

// TestRepairGaItemDuplicate_RollbackOnPostconditionFailure covers required test
// 5: a repairable analysis whose rebuild fails to drop the record must roll the
// slot back completely. Zeroing UnlockedRegionsOffset forces RebuildSlotFull onto
// its verbatim-copy path, so the removed record survives the byte round-trip; the
// reparse then still sees the physical duplicate and the postcondition triggers a
// rollback. Analysis reads only GaItems/GaMap/containers/equipment, so it still
// reports repairable before repair begins.
func TestRepairGaItemDuplicate_RollbackOnPostconditionFailure(t *testing.T) {
	slot := duplicateGaItemFixture(t)
	lo, _ := candidateIndexes(t, slot)

	slot.UnlockedRegionsOffset = 0
	if a := AnalyzeGaItemDuplicate(slot, dupHandle); !a.Repairable {
		t.Fatalf("want repairable before repair, got refusal %s: %s", a.RefusalCode, a.RefusalMsg)
	}
	before := CloneSlot(slot)

	if err := RepairGaItemDuplicate(slot, dupHandle, lo); err == nil {
		t.Fatal("want postcondition failure with rollback, got success")
	}
	if !reflect.DeepEqual(slot, before) {
		t.Fatal("rollback did not restore the complete pre-repair slot")
	}
}

// TestAnalyzeGaItemDuplicate_HealthySlotNotOffered covers required test 6: a
// slot with no duplicate is never offered this repair and is left untouched.
func TestAnalyzeGaItemDuplicate_HealthySlotNotOffered(t *testing.T) {
	fixture := fragmentedGaItemRoundTripFixture(t)
	slot := fixture.Slot
	before := SnapshotSlot(slot)

	analysis := AnalyzeGaItemDuplicate(slot, fixture.Handles.Weapon)
	if analysis.Repairable {
		t.Fatalf("healthy slot offered dedup repair: %+v", analysis)
	}
	if analysis.RefusalCode != "not_exactly_two_records" {
		t.Errorf("refusal code = %q, want not_exactly_two_records", analysis.RefusalCode)
	}
	assertSlotUnchanged(t, slot, before)
}

func assertSlotUnchanged(t *testing.T, slot *SaveSlot, before SlotSnapshot) {
	t.Helper()
	if !bytes.Equal(slot.Data, before.Data) {
		t.Error("slot.Data changed")
	}
	if len(slot.GaItems) != len(before.GaItems) {
		t.Fatalf("GaItems length changed: %d -> %d", len(before.GaItems), len(slot.GaItems))
	}
	for i := range slot.GaItems {
		if slot.GaItems[i] != before.GaItems[i] {
			t.Errorf("GaItems[%d] changed", i)
		}
	}
	if !sameInventoryItems(slot.Inventory.CommonItems, before.Inventory.CommonItems) ||
		!sameInventoryItems(slot.Storage.CommonItems, before.Storage.CommonItems) {
		t.Error("inventory/storage changed")
	}
}
