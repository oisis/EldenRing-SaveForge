package editor

import "testing"

// cleanItem builds a valid EditableItem suitable for "should validate OK" tests.
func cleanItem(uid string, h uint32) EditableItem {
	return EditableItem{
		UID:              uid,
		Source:           ItemSourceOriginal,
		Container:        ContainerInventory,
		OriginalHandle:   h,
		ItemID:           0x000F4240,
		BaseItemID:       0x000F4240,
		Name:             "Dagger",
		Category:         "melee_armaments",
		Quantity:         1,
		AcquisitionIndex: 1000,
		CurrentUpgrade:   0,
		MaxUpgrade:       25,
		HasGaItem:        true,
		IsWeapon:         true,
	}
}

func TestValidate_CleanWorkspaceIsOK(t *testing.T) {
	snap := InventoryWorkspaceSnapshot{
		InventoryItems: []EditableItem{cleanItem("hnd:0x80800001", 0x80800001)},
	}
	rep := Validate(snap)
	if !rep.OK {
		t.Fatalf("expected OK=true, got errors=%+v", rep.Errors)
	}
	if len(rep.Errors) != 0 {
		t.Errorf("unexpected errors: %+v", rep.Errors)
	}
	// Phase 4B removed the global CodeSharedAoWNotChecked deferral
	// warning — per-item AoW warnings (current_aow_*) replaced it.
	for _, w := range rep.Warnings {
		if w.Code == CodeSharedAoWNotChecked {
			t.Errorf("global CodeSharedAoWNotChecked should no longer be emitted; got %+v", w)
		}
	}
}

func TestValidate_DuplicateUID(t *testing.T) {
	snap := InventoryWorkspaceSnapshot{
		InventoryItems: []EditableItem{
			cleanItem("hnd:0xAAAA", 0x80800001),
			cleanItem("hnd:0xAAAA", 0x80800002),
		},
	}
	rep := Validate(snap)
	if rep.OK {
		t.Fatal("expected OK=false")
	}
	if len(rep.DuplicateUIDs) != 1 || rep.DuplicateUIDs[0] != "hnd:0xAAAA" {
		t.Errorf("DuplicateUIDs = %+v", rep.DuplicateUIDs)
	}
	foundCode := false
	for _, e := range rep.Errors {
		if e.Code == CodeDuplicateUID {
			foundCode = true
		}
	}
	if !foundCode {
		t.Errorf("expected duplicate_uid error in %+v", rep.Errors)
	}
}

func TestValidate_DuplicateHandle(t *testing.T) {
	snap := InventoryWorkspaceSnapshot{
		InventoryItems: []EditableItem{cleanItem("uid:1", 0x80800099)},
		StorageItems:   []EditableItem{cleanItem("uid:2", 0x80800099)},
	}
	rep := Validate(snap)
	if rep.OK {
		t.Fatal("expected OK=false")
	}
	if len(rep.DuplicateHandles) != 1 || rep.DuplicateHandles[0] != 0x80800099 {
		t.Errorf("DuplicateHandles = %+v", rep.DuplicateHandles)
	}
}

func TestValidate_QuantityZero(t *testing.T) {
	it := cleanItem("uid:q", 0x80800001)
	it.Quantity = 0
	snap := InventoryWorkspaceSnapshot{InventoryItems: []EditableItem{it}}
	rep := Validate(snap)
	if rep.OK {
		t.Fatal("expected OK=false for zero quantity")
	}
}

func TestValidate_UpgradeOutOfRange(t *testing.T) {
	it := cleanItem("uid:u", 0x80800001)
	it.CurrentUpgrade = 26
	it.MaxUpgrade = 25
	snap := InventoryWorkspaceSnapshot{InventoryItems: []EditableItem{it}}
	rep := Validate(snap)
	if rep.OK {
		t.Fatal("expected OK=false for upgrade above MaxUpgrade")
	}
}

func TestValidate_UnknownItemID(t *testing.T) {
	it := cleanItem("uid:?", 0x80800001)
	it.Name = ""
	it.ItemID = 0
	it.BaseItemID = 0
	snap := InventoryWorkspaceSnapshot{InventoryItems: []EditableItem{it}}
	rep := Validate(snap)
	if rep.OK {
		t.Fatal("expected OK=false")
	}
}

// TestValidate_NoPassThroughAggregate confirms Prompt 12 removed the aggregate
// "N records will round-trip unchanged" warning. Pass-through is a write
// strategy, not an integrity defect — record resolution is now reported through
// the measurable coverage model (core.BuildCoverageReport), not a fake issue.
// The raw unsupported counts remain on the report for callers that need them.
func TestValidate_NoPassThroughAggregate(t *testing.T) {
	snap := InventoryWorkspaceSnapshot{
		InventoryItems: []EditableItem{cleanItem("uid:p", 0x80800001)},
		UnsupportedInventoryRecords: []RawInventoryRecord{
			{Container: ContainerInventory, SlotIndex: 1, Handle: 0xB0001111, Reason: ReasonUnknownItem},
		},
	}
	rep := Validate(snap)
	if !rep.OK {
		t.Fatalf("expected OK=true (warnings only), got errors=%+v", rep.Errors)
	}
	for _, w := range rep.Warnings {
		if w.Code == CodePassThroughRecords {
			t.Errorf("pass-through aggregate must no longer be emitted, got %+v", w)
		}
	}
	if rep.UnsupportedInventoryCount != 1 {
		t.Errorf("UnsupportedInventoryCount = %d, want 1", rep.UnsupportedInventoryCount)
	}
}

func TestValidate_UnsupportedCategoryIsError(t *testing.T) {
	it := cleanItem("uid:c", 0x80800001)
	it.Category = "goods"
	snap := InventoryWorkspaceSnapshot{InventoryItems: []EditableItem{it}}
	rep := Validate(snap)
	if rep.OK {
		t.Fatal("expected OK=false for unsupported category on editable item")
	}
}
