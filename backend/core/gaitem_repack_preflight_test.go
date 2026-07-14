package core

import (
	"encoding/binary"
	"reflect"
	"testing"
)

func repackPreflightFixture() *SaveSlot {
	slot := &SaveSlot{
		Data:              make([]byte, SlotSize),
		Version:           1,
		GaMap:             make(map[uint32]uint32),
		MagicOffset:       1000,
		InventoryEnd:      GaItemsStart,
		PlayerDataOffset:  1000,
		FaceDataOffset:    2000,
		StorageBoxOffset:  2000,
		GaItemDataOffset:  0x8000,
		SectionMap:        []SectionRange{{Name: "all", Start: 0, End: SlotSize}},
		NextAoWIndex:      0,
		NextArmamentIndex: 1,
		NextGaItemHandle:  2,
		PartGaItemHandle:  0x80,
	}
	weapon := GaItemFull{Handle: ItemTypeWeapon | 1, ItemID: 1}
	slot.GaItems = []GaItemFull{weapon, {}, {}, {}}
	slot.GaMap[weapon.Handle] = weapon.ItemID
	return slot
}

func repackBlockerCodes(blockers []GaItemRepackBlocker) []string {
	codes := make([]string, len(blockers))
	for i, blocker := range blockers {
		codes[i] = blocker.Code
	}
	return codes
}

func TestPreflightGaItemRepack_HealthySlotAllowsAnalysis(t *testing.T) {
	slot := repackPreflightFixture()

	preflight := PreflightGaItemRepack(slot)

	if len(preflight.Blockers) != 0 {
		t.Fatalf("Blockers=%+v, want none", preflight.Blockers)
	}
	if preflight.Analysis.NonEmptyRecords != 1 {
		t.Errorf("NonEmptyRecords=%d, want 1", preflight.Analysis.NonEmptyRecords)
	}
	if preflight.Analysis.Recovered != 0 {
		t.Errorf("Recovered=%d, want 0", preflight.Analysis.Recovered)
	}
}

func TestPreflightGaItemRepack_StopsAfterStructuralFailure(t *testing.T) {
	slot := repackPreflightFixture()
	slot.Data = slot.Data[:16]
	slot.GaItems[0].Handle = 0x70000001

	preflight := PreflightGaItemRepack(slot)

	if got, want := repackBlockerCodes(preflight.Blockers), []string{"slot_data_size"}; !reflect.DeepEqual(got, want) {
		t.Errorf("blocker codes=%v, want %v", got, want)
	}
	if preflight.Analysis != (GaItemRepackAnalysis{}) {
		t.Errorf("Analysis=%+v, want zero after structural failure", preflight.Analysis)
	}
}

func TestPreflightGaItemRepack_OffsetFailureDoesNotNormalizeSlot(t *testing.T) {
	slot := repackPreflightFixture()
	slot.EventFlagsOffset = SlotSize
	before := slot.EventFlagsOffset

	preflight := PreflightGaItemRepack(slot)

	if got, want := repackBlockerCodes(preflight.Blockers), []string{"offset_chain"}; !reflect.DeepEqual(got, want) {
		t.Errorf("blocker codes=%v, want %v", got, want)
	}
	if slot.EventFlagsOffset != before {
		t.Errorf("EventFlagsOffset=%d, want unchanged %d", slot.EventFlagsOffset, before)
	}
}

func TestPreflightGaItemRepack_StopsAfterUnknownRecordType(t *testing.T) {
	slot := repackPreflightFixture()
	slot.GaItems[0].Handle = 0x70000001
	slot.GaMap = map[uint32]uint32{slot.GaItems[0].Handle: slot.GaItems[0].ItemID}

	preflight := PreflightGaItemRepack(slot)

	if got, want := repackBlockerCodes(preflight.Blockers), []string{"unknown_handle_type"}; !reflect.DeepEqual(got, want) {
		t.Errorf("blocker codes=%v, want %v", got, want)
	}
}

func TestPreflightGaItemRepack_RejectsInvalidSectionMap(t *testing.T) {
	slot := repackPreflightFixture()
	slot.SectionMap = nil

	preflight := PreflightGaItemRepack(slot)

	if got, want := repackBlockerCodes(preflight.Blockers), []string{"section_map"}; !reflect.DeepEqual(got, want) {
		t.Errorf("blocker codes=%v, want %v", got, want)
	}
}

func TestPreflightGaItemRepack_RejectsDuplicateHandlesBeforeReferenceChecks(t *testing.T) {
	slot := repackPreflightFixture()
	slot.GaItems = append(slot.GaItems, slot.GaItems[0])
	slot.NextArmamentIndex = 2

	preflight := PreflightGaItemRepack(slot)

	if got, want := repackBlockerCodes(preflight.Blockers), []string{"duplicate_handle"}; !reflect.DeepEqual(got, want) {
		t.Errorf("blocker codes=%v, want %v", got, want)
	}
}

func TestPreflightGaItemRepack_RejectsInvalidCursorIndices(t *testing.T) {
	slot := repackPreflightFixture()
	slot.NextAoWIndex = 2
	slot.NextArmamentIndex = 1

	preflight := PreflightGaItemRepack(slot)

	if got, want := repackBlockerCodes(preflight.Blockers), []string{"gaitem_indices"}; !reflect.DeepEqual(got, want) {
		t.Errorf("blocker codes=%v, want %v", got, want)
	}
}

func TestPreflightGaItemRepack_AggregatesReferenceBlockersDeterministically(t *testing.T) {
	const (
		weaponOne   = ItemTypeWeapon | 1
		weaponTwo   = ItemTypeWeapon | 2
		weaponThree = ItemTypeWeapon | 3
		aowHandle   = ItemTypeAow | 4
		danglingAoW = ItemTypeAow | 99
		accessory   = ItemTypeAccessory | 5
		unresolved  = ItemTypeItem | 6
		zeroMap     = ItemTypeItem | 7
		orphanMap   = ItemTypeWeapon | 8
	)

	slot := repackPreflightFixture()
	slot.GaItems = []GaItemFull{
		{Handle: weaponOne, ItemID: 1, AoWGaItemHandle: aowHandle},
		{Handle: weaponTwo, ItemID: 2, AoWGaItemHandle: aowHandle},
		{Handle: weaponThree, ItemID: 3, AoWGaItemHandle: danglingAoW},
		{Handle: aowHandle, ItemID: 0x80000001},
	}
	slot.NextAoWIndex = 4
	slot.NextArmamentIndex = 4
	slot.GaMap = map[uint32]uint32{
		weaponOne:   99, // physical record mismatch
		weaponTwo:   2,
		weaponThree: 3,
		aowHandle:   0x80000001,
		accessory:   0x20000001, // valid non-backed stackable
		zeroMap:     0,
		orphanMap:   8,
	}
	slot.Inventory.CommonItems = []InventoryItem{
		{GaItemHandle: weaponOne, Index: 42},
		{GaItemHandle: unresolved, Index: 43},
	}
	slot.Inventory.KeyItems = []InventoryItem{{GaItemHandle: accessory, Index: 42}}
	slot.Storage.CommonItems = []InventoryItem{{GaItemHandle: weaponOne, Index: 1}}
	binary.LittleEndian.PutUint32(slot.Data[slot.StorageBoxOffset:], 1)
	binary.LittleEndian.PutUint32(slot.Data[slot.GaItemDataOffset:], GaItemDataMaxCount+1)

	preflight := PreflightGaItemRepack(slot)
	got := repackBlockerCodes(preflight.Blockers)
	want := []string{
		"dangling_aow_handle",
		"duplicate_index",
		"gaitemdata_count",
		"gamap_record_mismatch",
		"gamap_zero_id",
		"orphan_gamap_entry",
		"orphan_inventory_handle",
		"orphan_storage_handle",
		"shared_aow_handle",
		"storage_count",
		"unresolved_stackable_handle",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("blocker codes=%v, want %v", got, want)
	}
	if preflight.Analysis != (GaItemRepackAnalysis{}) {
		t.Errorf("Analysis=%+v, want zero when blockers exist", preflight.Analysis)
	}
}

func TestPreflightGaItemRepack_EmptyGaItemTableIsValidNoOpInput(t *testing.T) {
	slot := repackPreflightFixture()
	slot.GaItems = nil
	slot.GaMap = map[uint32]uint32{ItemTypeAccessory | 5: 0x20000001}
	slot.NextAoWIndex = 0
	slot.NextArmamentIndex = 1

	preflight := PreflightGaItemRepack(slot)

	if len(preflight.Blockers) != 0 {
		t.Fatalf("Blockers=%+v, want none", preflight.Blockers)
	}
	if preflight.Analysis.NonEmptyRecords != 0 || preflight.Analysis.Recovered != 0 {
		t.Errorf("Analysis=%+v, want empty no-op forecast", preflight.Analysis)
	}
}
