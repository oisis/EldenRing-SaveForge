package editor

import (
	"strings"
	"testing"
)

// Real DB IDs used in weapon tests.
const (
	wpDaggerBase      = uint32(0x000F4240) // melee_armaments, MaxUpgrade=25
	wpClaymoreBase    = uint32(0x003085E0) // melee_armaments, MaxUpgrade=25
	aowLionsClaw      = uint32(0x80002710) // ashes_of_war
	aowImpalingThrust = uint32(0x80002774) // ashes_of_war
)

// weaponSnap builds a clean single-weapon snapshot for tests. The weapon
// is a base-level un-infused Dagger placed in inventory.
func weaponSnap() *InventoryWorkspaceSnapshot {
	snap := &InventoryWorkspaceSnapshot{
		SessionID:      "ses-test",
		CharacterIndex: 0,
		InventoryItems: []EditableItem{
			{
				UID:              "hnd:0x80800001",
				Source:           ItemSourceOriginal,
				Container:        ContainerInventory,
				Position:         0,
				OriginalHandle:   0x80800001,
				ItemID:           wpDaggerBase,
				BaseItemID:       wpDaggerBase,
				Name:             "Dagger",
				Category:         "melee_armaments",
				Quantity:         1,
				AcquisitionIndex: 1000,
				CurrentUpgrade:   0,
				MaxUpgrade:       25,
				HasGaItem:        true,
				IsWeapon:         true,
			},
		},
		StorageItems:                []EditableItem{},
		UnsupportedInventoryRecords: []RawInventoryRecord{},
		UnsupportedStorageRecords:   []RawInventoryRecord{},
	}
	return snap
}

// armorSnap builds a snapshot containing a single armor item (Iron Kasa)
// for "non-weapon" rejection tests.
func armorSnap() *InventoryWorkspaceSnapshot {
	return &InventoryWorkspaceSnapshot{
		SessionID: "ses-test",
		InventoryItems: []EditableItem{
			{
				UID:            "hnd:0x90800001",
				Source:         ItemSourceOriginal,
				Container:      ContainerInventory,
				OriginalHandle: 0x90800001,
				ItemID:         0x100249F0,
				BaseItemID:     0x100249F0,
				Name:           "Iron Kasa",
				Category:       "head",
				Quantity:       1,
				HasGaItem:      true,
				IsArmor:        true,
			},
		},
	}
}

// talismanSnap builds a snapshot with a single talisman.
func talismanSnap() *InventoryWorkspaceSnapshot {
	return &InventoryWorkspaceSnapshot{
		SessionID: "ses-test",
		InventoryItems: []EditableItem{
			{
				UID:        "hnd:0xA0800001",
				Source:     ItemSourceOriginal,
				Container:  ContainerInventory,
				ItemID:     0x200003E8,
				BaseItemID: 0x200003E8,
				Name:       "Crimson Amber Medallion",
				Category:   "talismans",
				Quantity:   1,
				IsTalisman: true,
			},
		},
	}
}

func TestEncodeWeaponItemID_StandardZero(t *testing.T) {
	id, err := encodeWeaponItemID(wpDaggerBase, 0, "")
	if err != nil {
		t.Fatalf("encodeWeaponItemID: %v", err)
	}
	if id != wpDaggerBase {
		t.Errorf("id = 0x%08X, want 0x%08X", id, wpDaggerBase)
	}
}

func TestEncodeWeaponItemID_Heavy5(t *testing.T) {
	// Heavy offset = 100, level 5 → base + 105.
	id, err := encodeWeaponItemID(wpDaggerBase, 5, "Heavy")
	if err != nil {
		t.Fatalf("encodeWeaponItemID: %v", err)
	}
	want := wpDaggerBase + 105
	if id != want {
		t.Errorf("id = 0x%08X, want 0x%08X", id, want)
	}
	// Roundtrip via decode.
	level, inf := decodeWeaponUpgradeInfusion(id, wpDaggerBase)
	if level != 5 || inf != "Heavy" {
		t.Errorf("decode roundtrip: level=%d inf=%q, want 5 Heavy", level, inf)
	}
}

func TestEncodeWeaponItemID_UnknownInfusion(t *testing.T) {
	if _, err := encodeWeaponItemID(wpDaggerBase, 0, "Bogus"); err == nil {
		t.Fatal("expected error for unknown infusion")
	}
}

func TestUpdateWeapon_UpgradeZeroToThree(t *testing.T) {
	snap := weaponSnap()
	if err := UpdateWeapon(snap, "hnd:0x80800001", WeaponPatch{
		SetUpgrade: true, Upgrade: 3,
	}); err != nil {
		t.Fatalf("UpdateWeapon: %v", err)
	}
	it := snap.InventoryItems[0]
	if it.CurrentUpgrade != 3 {
		t.Errorf("CurrentUpgrade = %d, want 3", it.CurrentUpgrade)
	}
	if it.ItemID != wpDaggerBase+3 {
		t.Errorf("ItemID = 0x%08X, want 0x%08X", it.ItemID, wpDaggerBase+3)
	}
	if !it.HasPendingWeaponPatch {
		t.Error("HasPendingWeaponPatch should be true")
	}
	if !snap.Dirty {
		t.Error("Dirty should be true")
	}
}

func TestUpdateWeapon_UpgradeBeyondMaxRejected(t *testing.T) {
	snap := weaponSnap()
	err := UpdateWeapon(snap, "hnd:0x80800001", WeaponPatch{
		SetUpgrade: true, Upgrade: 26,
	})
	if err == nil {
		t.Fatal("expected error for upgrade beyond MaxUpgrade")
	}
	if !strings.Contains(err.Error(), "MaxUpgrade") {
		t.Errorf("error should mention MaxUpgrade, got %v", err)
	}
	// Item must be untouched on error.
	it := snap.InventoryItems[0]
	if it.CurrentUpgrade != 0 || it.ItemID != wpDaggerBase || it.HasPendingWeaponPatch {
		t.Errorf("item mutated on error: %+v", it)
	}
}

func TestUpdateWeapon_NegativeUpgradeRejected(t *testing.T) {
	snap := weaponSnap()
	err := UpdateWeapon(snap, "hnd:0x80800001", WeaponPatch{
		SetUpgrade: true, Upgrade: -1,
	})
	if err == nil {
		t.Fatal("expected error for negative upgrade")
	}
}

func TestUpdateWeapon_InfusionHeavyEncodesItemID(t *testing.T) {
	snap := weaponSnap()
	if err := UpdateWeapon(snap, "hnd:0x80800001", WeaponPatch{
		SetInfusionName: true, InfusionName: "Heavy",
	}); err != nil {
		t.Fatalf("UpdateWeapon: %v", err)
	}
	it := snap.InventoryItems[0]
	if it.InfusionName != "Heavy" {
		t.Errorf("InfusionName = %q, want Heavy", it.InfusionName)
	}
	if it.ItemID != wpDaggerBase+100 {
		t.Errorf("ItemID = 0x%08X, want 0x%08X", it.ItemID, wpDaggerBase+100)
	}
}

func TestUpdateWeapon_UpgradePlusInfusion(t *testing.T) {
	snap := weaponSnap()
	if err := UpdateWeapon(snap, "hnd:0x80800001", WeaponPatch{
		SetUpgrade:      true,
		Upgrade:         7,
		SetInfusionName: true,
		InfusionName:    "Fire",
	}); err != nil {
		t.Fatalf("UpdateWeapon: %v", err)
	}
	it := snap.InventoryItems[0]
	if it.CurrentUpgrade != 7 || it.InfusionName != "Fire" {
		t.Errorf("got upgrade=%d inf=%q, want 7 Fire", it.CurrentUpgrade, it.InfusionName)
	}
	// Fire offset = 400, level 7 → +407.
	if it.ItemID != wpDaggerBase+407 {
		t.Errorf("ItemID = 0x%08X, want 0x%08X", it.ItemID, wpDaggerBase+407)
	}
}

func TestUpdateWeapon_UnknownInfusionRejected(t *testing.T) {
	snap := weaponSnap()
	err := UpdateWeapon(snap, "hnd:0x80800001", WeaponPatch{
		SetInfusionName: true, InfusionName: "Bogus",
	})
	if err == nil {
		t.Fatal("expected error for unknown infusion")
	}
	if snap.InventoryItems[0].HasPendingWeaponPatch {
		t.Error("item mutated on error")
	}
}

func TestUpdateWeapon_ArmorReturnsNonWeaponError(t *testing.T) {
	snap := armorSnap()
	err := UpdateWeapon(snap, "hnd:0x90800001", WeaponPatch{
		SetUpgrade: true, Upgrade: 1,
	})
	if err == nil {
		t.Fatal("expected error for armor item")
	}
	if !strings.Contains(err.Error(), "not weapon-editable") {
		t.Errorf("error should mention non-weapon, got %v", err)
	}
}

func TestUpdateWeapon_TalismanReturnsNonWeaponError(t *testing.T) {
	snap := talismanSnap()
	err := UpdateWeapon(snap, "hnd:0xA0800001", WeaponPatch{
		SetUpgrade: true, Upgrade: 1,
	})
	if err == nil {
		t.Fatal("expected error for talisman item")
	}
}

func TestUpdateWeapon_UnknownUIDReturnsError(t *testing.T) {
	snap := weaponSnap()
	err := UpdateWeapon(snap, "hnd:0xDEADBEEF", WeaponPatch{SetUpgrade: true, Upgrade: 1})
	if err == nil {
		t.Fatal("expected error for unknown UID")
	}
}

func TestUpdateWeapon_NilSnapshot(t *testing.T) {
	if err := UpdateWeapon(nil, "x", WeaponPatch{}); err == nil {
		t.Fatal("expected error for nil snapshot")
	}
}

func TestUpdateWeapon_SetAoWStoresPendingFields(t *testing.T) {
	snap := weaponSnap()
	if err := UpdateWeapon(snap, "hnd:0x80800001", WeaponPatch{
		SetAoWItemID: true, AoWItemID: aowLionsClaw,
	}); err != nil {
		t.Fatalf("UpdateWeapon: %v", err)
	}
	it := snap.InventoryItems[0]
	if it.PendingAoWItemID != aowLionsClaw {
		t.Errorf("PendingAoWItemID = 0x%08X, want 0x%08X", it.PendingAoWItemID, aowLionsClaw)
	}
	if it.PendingAoWName != "Lion's Claw" {
		t.Errorf("PendingAoWName = %q, want Lion's Claw", it.PendingAoWName)
	}
	if !it.HasPendingWeaponPatch {
		t.Error("HasPendingWeaponPatch should be true")
	}
}

func TestUpdateWeapon_SetAoWZeroClearsPending(t *testing.T) {
	snap := weaponSnap()
	// Seed a pending AoW first.
	snap.InventoryItems[0].PendingAoWItemID = aowImpalingThrust
	snap.InventoryItems[0].PendingAoWName = "Impaling Thrust"

	if err := UpdateWeapon(snap, "hnd:0x80800001", WeaponPatch{
		SetAoWItemID: true, AoWItemID: 0,
	}); err != nil {
		t.Fatalf("UpdateWeapon: %v", err)
	}
	it := snap.InventoryItems[0]
	if it.PendingAoWItemID != 0 || it.PendingAoWName != "" {
		t.Errorf("pending AoW should be cleared: %+v", it)
	}
}

func TestUpdateWeapon_ClearAoWFlag(t *testing.T) {
	snap := weaponSnap()
	snap.InventoryItems[0].PendingAoWItemID = aowLionsClaw
	snap.InventoryItems[0].PendingAoWName = "Lion's Claw"

	if err := UpdateWeapon(snap, "hnd:0x80800001", WeaponPatch{
		ClearAoW: true,
	}); err != nil {
		t.Fatalf("UpdateWeapon: %v", err)
	}
	it := snap.InventoryItems[0]
	if it.PendingAoWItemID != 0 || it.PendingAoWName != "" {
		t.Errorf("ClearAoW failed to wipe pending fields: %+v", it)
	}
}

func TestUpdateWeapon_AoWUnknownIDRejected(t *testing.T) {
	snap := weaponSnap()
	err := UpdateWeapon(snap, "hnd:0x80800001", WeaponPatch{
		SetAoWItemID: true, AoWItemID: 0xDEADBEEF,
	})
	if err == nil {
		t.Fatal("expected error for unknown AoW ID")
	}
	if snap.InventoryItems[0].PendingAoWItemID != 0 {
		t.Error("pending AoW should remain cleared on error")
	}
}

func TestUpdateWeapon_AoWNonAoWCategoryRejected(t *testing.T) {
	snap := weaponSnap()
	// Pass a real weapon ID — wrong category for an AoW slot.
	err := UpdateWeapon(snap, "hnd:0x80800001", WeaponPatch{
		SetAoWItemID: true, AoWItemID: wpClaymoreBase,
	})
	if err == nil {
		t.Fatal("expected error for non-AoW category")
	}
	if !strings.Contains(err.Error(), "ashes_of_war") {
		t.Errorf("error should mention ashes_of_war category, got %v", err)
	}
}

func TestUpdateWeapon_RefreshesValidation(t *testing.T) {
	snap := weaponSnap()
	snap.Validation = WorkspaceValidationReport{OK: false}
	if err := UpdateWeapon(snap, "hnd:0x80800001", WeaponPatch{
		SetUpgrade: true, Upgrade: 5,
	}); err != nil {
		t.Fatalf("UpdateWeapon: %v", err)
	}
	if !snap.Validation.OK {
		t.Errorf("Validation.OK should be true, errors=%+v", snap.Validation.Errors)
	}
}

func TestUpdateWeapon_LeavesPassThroughUntouched(t *testing.T) {
	snap := weaponSnap()
	snap.UnsupportedInventoryRecords = []RawInventoryRecord{
		{Container: ContainerInventory, SlotIndex: 99, Handle: 0xB0FF0001, Reason: ReasonUnknownItem},
	}
	before := append([]RawInventoryRecord(nil), snap.UnsupportedInventoryRecords...)
	if err := UpdateWeapon(snap, "hnd:0x80800001", WeaponPatch{
		SetUpgrade: true, Upgrade: 1,
	}); err != nil {
		t.Fatalf("UpdateWeapon: %v", err)
	}
	if !equalRawRecords(snap.UnsupportedInventoryRecords, before) {
		t.Errorf("pass-through mutated: %+v", snap.UnsupportedInventoryRecords)
	}
}

func TestUpdateWeapon_AddedItemCanBeUpdated(t *testing.T) {
	snap := weaponSnap()
	// Remove the seeded weapon and Add a fresh one via AddItem.
	snap.InventoryItems = snap.InventoryItems[:0]
	if err := AddItem(snap, AddItemSpec{ItemID: wpDaggerBase}, ContainerInventory, 0); err != nil {
		t.Fatalf("AddItem: %v", err)
	}
	addedUID := snap.InventoryItems[0].UID
	if !strings.HasPrefix(addedUID, "new:") {
		t.Fatalf("expected new:* UID, got %q", addedUID)
	}
	if err := UpdateWeapon(snap, addedUID, WeaponPatch{
		SetUpgrade: true, Upgrade: 10,
	}); err != nil {
		t.Fatalf("UpdateWeapon on added item: %v", err)
	}
	it := snap.InventoryItems[0]
	if it.Source != ItemSourceAdded {
		t.Errorf("Source = %q, want added (must not change on UpdateWeapon)", it.Source)
	}
	if it.OriginalHandle != 0 {
		t.Errorf("OriginalHandle changed unexpectedly: 0x%X", it.OriginalHandle)
	}
	if it.CurrentUpgrade != 10 || it.ItemID != wpDaggerBase+10 {
		t.Errorf("upgrade not applied: %+v", it)
	}
	if !it.HasPendingWeaponPatch {
		t.Error("HasPendingWeaponPatch should be true")
	}
}

func TestUpdateWeapon_MoveAddUpdateSequence(t *testing.T) {
	snap := weaponSnap()
	// Snapshot already has Dagger at inv[0]; add Claymore at inv[1].
	if err := AddItem(snap, AddItemSpec{ItemID: wpClaymoreBase}, ContainerInventory, 1); err != nil {
		t.Fatalf("AddItem: %v", err)
	}
	// Move the original Dagger to storage.
	if err := MoveItem(snap, "hnd:0x80800001", ContainerStorage, 0); err != nil {
		t.Fatalf("MoveItem: %v", err)
	}
	// Update the Claymore upgrade.
	claymoreUID := ""
	for _, it := range snap.InventoryItems {
		if it.Name == "Claymore" {
			claymoreUID = it.UID
		}
	}
	if claymoreUID == "" {
		t.Fatal("Claymore lost from inventory")
	}
	if err := UpdateWeapon(snap, claymoreUID, WeaponPatch{
		SetUpgrade: true, Upgrade: 12, SetInfusionName: true, InfusionName: "Sacred",
	}); err != nil {
		t.Fatalf("UpdateWeapon: %v", err)
	}

	// Verify final state.
	if len(snap.StorageItems) != 1 || snap.StorageItems[0].UID != "hnd:0x80800001" {
		t.Errorf("storage = %+v", snap.StorageItems)
	}
	if len(snap.InventoryItems) != 1 {
		t.Fatalf("inventory should hold only Claymore, got %+v", snap.InventoryItems)
	}
	clay := snap.InventoryItems[0]
	if clay.CurrentUpgrade != 12 || clay.InfusionName != "Sacred" {
		t.Errorf("Claymore patch not applied: %+v", clay)
	}
	// Sacred offset = 700, level 12 → +712.
	if clay.ItemID != wpClaymoreBase+712 {
		t.Errorf("Claymore ItemID = 0x%08X, want 0x%08X", clay.ItemID, wpClaymoreBase+712)
	}
	if !snap.Dirty {
		t.Error("Dirty should be true after sequence")
	}
}

func TestValidate_PendingAoWBogusIsError(t *testing.T) {
	snap := weaponSnap()
	// Bypass UpdateWeapon — simulate a bad mutation reaching Validate.
	snap.InventoryItems[0].PendingAoWItemID = 0xDEADBEEF
	rep := Validate(*snap)
	if rep.OK {
		t.Fatal("expected OK=false for bogus pending AoW")
	}
	found := false
	for _, e := range rep.Errors {
		if e.Code == CodePendingAoWUnknown {
			found = true
		}
	}
	if !found {
		t.Errorf("expected %q error, got %+v", CodePendingAoWUnknown, rep.Errors)
	}
}

func TestValidate_PendingAoWNonAoWCategoryIsError(t *testing.T) {
	snap := weaponSnap()
	// Point at a real weapon ID — known in DB but wrong category.
	snap.InventoryItems[0].PendingAoWItemID = wpClaymoreBase
	rep := Validate(*snap)
	if rep.OK {
		t.Fatal("expected OK=false")
	}
}

func TestValidate_PendingAoWValidIsOK(t *testing.T) {
	snap := weaponSnap()
	snap.InventoryItems[0].PendingAoWItemID = aowLionsClaw
	rep := Validate(*snap)
	if !rep.OK {
		t.Fatalf("expected OK=true, errors=%+v", rep.Errors)
	}
}
