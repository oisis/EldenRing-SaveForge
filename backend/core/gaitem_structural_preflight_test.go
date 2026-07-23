package core

import (
	"encoding/binary"
	"reflect"
	"testing"
)

func gaItemStructuralIssueCodes(issues []GaItemStructuralIssue) []string {
	codes := make([]string, len(issues))
	for i, issue := range issues {
		codes[i] = issue.Code
	}
	return codes
}

func TestScanGaItemStructuralIssues_HealthySlotIsClean(t *testing.T) {
	slot := gaItemStructuralFixture()

	report := ScanGaItemStructuralIssues(slot)

	if len(report.Issues) != 0 {
		t.Fatalf("Issues=%+v, want none", report.Issues)
	}
}

// Raw acquisition Index duplicates are a separate container-level concern and
// do not change the GaItem reference graph checked here.
func TestScanGaItemStructuralIssues_AllowsGameWrittenDuplicateInventoryIndex(t *testing.T) {
	const secondWeapon = ItemTypeWeapon | 2

	slot := gaItemStructuralFixture()
	slot.GaItems[1] = GaItemFull{Handle: secondWeapon, ItemID: 2}
	slot.NextArmamentIndex = 2
	slot.GaMap[secondWeapon] = 2
	slot.Inventory.CommonItems = []InventoryItem{
		{GaItemHandle: slot.GaItems[0].Handle, Quantity: 1, Index: 1088},
		{GaItemHandle: secondWeapon, Quantity: 1, Index: 1088},
	}

	if issues := ScanDuplicateInventoryIndices(slot); len(issues) != 1 {
		t.Fatalf("duplicate scanner issues=%d, want 1", len(issues))
	}
	if issues := ScanGaItemStructuralIssues(slot).Issues; len(issues) != 0 {
		t.Fatalf("Issues=%+v, want none", issues)
	}
}

func TestScanGaItemStructuralIssues_StopsAfterStructuralFailure(t *testing.T) {
	slot := gaItemStructuralFixture()
	slot.Data = slot.Data[:16]
	slot.GaItems[0].Handle = 0x70000001

	report := ScanGaItemStructuralIssues(slot)

	if got, want := gaItemStructuralIssueCodes(report.Issues), []string{"slot_data_size"}; !reflect.DeepEqual(got, want) {
		t.Errorf("issue codes=%v, want %v", got, want)
	}
}

func TestScanGaItemStructuralIssues_OffsetFailureDoesNotNormalizeSlot(t *testing.T) {
	slot := gaItemStructuralFixture()
	slot.EventFlagsOffset = SlotSize
	before := slot.EventFlagsOffset

	report := ScanGaItemStructuralIssues(slot)

	if got, want := gaItemStructuralIssueCodes(report.Issues), []string{"offset_chain"}; !reflect.DeepEqual(got, want) {
		t.Errorf("issue codes=%v, want %v", got, want)
	}
	if slot.EventFlagsOffset != before {
		t.Errorf("EventFlagsOffset=%d, want unchanged %d", slot.EventFlagsOffset, before)
	}
}

func TestScanGaItemStructuralIssues_StopsAfterUnknownRecordType(t *testing.T) {
	slot := gaItemStructuralFixture()
	slot.GaItems[0].Handle = 0x70000001
	slot.GaMap = map[uint32]uint32{slot.GaItems[0].Handle: slot.GaItems[0].ItemID}

	report := ScanGaItemStructuralIssues(slot)

	if got, want := gaItemStructuralIssueCodes(report.Issues), []string{"unknown_handle_type"}; !reflect.DeepEqual(got, want) {
		t.Errorf("issue codes=%v, want %v", got, want)
	}
}

func TestScanGaItemStructuralIssues_RejectsInvalidSectionMap(t *testing.T) {
	slot := gaItemStructuralFixture()
	slot.SectionMap = nil

	report := ScanGaItemStructuralIssues(slot)

	if got, want := gaItemStructuralIssueCodes(report.Issues), []string{"section_map"}; !reflect.DeepEqual(got, want) {
		t.Errorf("issue codes=%v, want %v", got, want)
	}
}

func TestScanGaItemStructuralIssues_RejectsDuplicateHandlesBeforeReferenceChecks(t *testing.T) {
	slot := gaItemStructuralFixture()
	slot.GaItems = append(slot.GaItems, slot.GaItems[0])
	slot.NextArmamentIndex = 2

	report := ScanGaItemStructuralIssues(slot)

	if got, want := gaItemStructuralIssueCodes(report.Issues), []string{"duplicate_handle"}; !reflect.DeepEqual(got, want) {
		t.Errorf("issue codes=%v, want %v", got, want)
	}
	// The duplicate_handle issue must carry the offending handle structurally
	// so the shared duplicate-repair UI never parses it from the message.
	if got, want := report.Issues[0].Handle, slot.GaItems[0].Handle; got != want {
		t.Errorf("duplicate_handle issue Handle=0x%08X, want 0x%08X", got, want)
	}
}

func TestScanGaItemStructuralIssues_RejectsInvalidCursorIndices(t *testing.T) {
	slot := gaItemStructuralFixture()
	slot.NextAoWIndex = 2
	slot.NextArmamentIndex = 1

	report := ScanGaItemStructuralIssues(slot)

	if got, want := gaItemStructuralIssueCodes(report.Issues), []string{"gaitem_indices"}; !reflect.DeepEqual(got, want) {
		t.Errorf("issue codes=%v, want %v", got, want)
	}
}

func TestScanGaItemStructuralIssues_AggregatesReferenceIssuesDeterministically(t *testing.T) {
	const (
		weaponOne   = ItemTypeWeapon | 1
		weaponTwo   = ItemTypeWeapon | 2
		weaponThree = ItemTypeWeapon | 3
		aowHandle   = ItemTypeAow | 4
		danglingAoW = ItemTypeAow | 99
		accessory   = ItemTypeAccessory | 5
		zeroMap     = ItemTypeItem | 7
		orphanMap   = ItemTypeWeapon | 8
	)

	slot := gaItemStructuralFixture()
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
	}
	slot.Inventory.KeyItems = []InventoryItem{{GaItemHandle: accessory, Index: 42}}
	slot.Storage.CommonItems = []InventoryItem{{GaItemHandle: weaponOne, Index: 1}}
	binary.LittleEndian.PutUint32(slot.Data[slot.StorageBoxOffset:], 1)
	binary.LittleEndian.PutUint32(slot.Data[slot.GaItemDataOffset:], GaItemDataMaxCount+1)

	report := ScanGaItemStructuralIssues(slot)
	got := gaItemStructuralIssueCodes(report.Issues)
	want := []string{
		"dangling_aow_handle",
		"gaitemdata_count",
		"gamap_record_mismatch",
		"gamap_zero_id",
		"orphan_gamap_entry",
		"orphan_inventory_handle",
		"orphan_storage_handle",
		"shared_aow_handle",
		"storage_count",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("issue codes=%v, want %v", got, want)
	}
}

func TestScanGaItemStructuralIssues_AllowsHandleEncodedStackablesWithoutGaMap(t *testing.T) {
	const (
		inventoryGoods = ItemTypeItem | 0x2738
		keyTalisman    = ItemTypeAccessory | 0x73
		storageGoods   = ItemTypeItem | 0x12C
	)

	slot := gaItemStructuralFixture()
	slot.Inventory.CommonItems = []InventoryItem{{GaItemHandle: inventoryGoods, Quantity: 1, Index: 1}}
	slot.Inventory.KeyItems = []InventoryItem{{GaItemHandle: keyTalisman, Quantity: 1, Index: 2}}
	slot.Storage.CommonItems = []InventoryItem{{GaItemHandle: storageGoods, Quantity: 1, Index: 3}}
	writeFixtureStorage(slot, slot.Storage.CommonItems)

	report := ScanGaItemStructuralIssues(slot)
	if len(report.Issues) != 0 {
		t.Fatalf("Issues=%+v, want none for handle-encoded stackables", report.Issues)
	}
	for _, handle := range []uint32{inventoryGoods, keyTalisman, storageGoods} {
		if _, exists := slot.GaMap[handle]; exists {
			t.Fatalf("GaMap unexpectedly contains handle-encoded stackable 0x%08X", handle)
		}
	}
}

func TestScanGaItemStructuralIssues_EmptyGaItemTableIsValid(t *testing.T) {
	slot := gaItemStructuralFixture()
	slot.GaItems = nil
	slot.GaMap = map[uint32]uint32{ItemTypeAccessory | 5: 0x20000001}
	slot.NextAoWIndex = 0
	slot.NextArmamentIndex = 1

	report := ScanGaItemStructuralIssues(slot)

	if len(report.Issues) != 0 {
		t.Fatalf("Issues=%+v, want none", report.Issues)
	}
}
