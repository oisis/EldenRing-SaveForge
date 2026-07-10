package editor

import (
	"strings"
	"testing"
)

// Real DB IDs used across add tests.
const (
	idDagger       = uint32(0x000F4240) // melee_armaments, MaxUpgrade=25
	idCrimsonAmber = uint32(0x200003E8) // talismans
	idIronKasa     = uint32(0x100249F0) // head
)

func TestAddItem_WeaponToEmptyInventory(t *testing.T) {
	snap := mkSnap(0, 0)
	err := AddItem(snap, AddItemSpec{ItemID: idDagger}, ContainerInventory, 0)
	if err != nil {
		t.Fatalf("AddItem: %v", err)
	}
	if len(snap.InventoryItems) != 1 {
		t.Fatalf("inv count = %d, want 1", len(snap.InventoryItems))
	}
	it := snap.InventoryItems[0]
	if it.Source != ItemSourceAdded {
		t.Errorf("Source = %q, want added", it.Source)
	}
	if it.OriginalHandle != 0 {
		t.Errorf("OriginalHandle = 0x%X, want 0", it.OriginalHandle)
	}
	if it.HasGaItem {
		t.Error("HasGaItem should be false for new item")
	}
	if !strings.HasPrefix(it.UID, "new:") {
		t.Errorf("UID = %q, want prefix new:", it.UID)
	}
	if it.Name != "Dagger" {
		t.Errorf("Name = %q, want Dagger", it.Name)
	}
	if !it.IsWeapon {
		t.Error("IsWeapon should be true for Dagger")
	}
	if it.Quantity != 1 {
		t.Errorf("Quantity = %d, want 1 (normalized from 0)", it.Quantity)
	}
	if !snap.Dirty {
		t.Error("Dirty should be true after add")
	}
}

func TestAddItem_ArmorToStorage(t *testing.T) {
	snap := mkSnap(0, 0)
	if err := AddItem(snap, AddItemSpec{ItemID: idIronKasa, Quantity: 1}, ContainerStorage, 0); err != nil {
		t.Fatalf("AddItem: %v", err)
	}
	if len(snap.StorageItems) != 1 {
		t.Fatalf("sto count = %d, want 1", len(snap.StorageItems))
	}
	it := snap.StorageItems[0]
	if it.Category != "head" {
		t.Errorf("Category = %q, want head", it.Category)
	}
	if !it.IsArmor {
		t.Error("IsArmor should be true for Iron Kasa")
	}
	if it.Container != ContainerStorage {
		t.Errorf("Container = %q, want storage", it.Container)
	}
}

func TestAddItem_Talisman(t *testing.T) {
	snap := mkSnap(0, 0)
	if err := AddItem(snap, AddItemSpec{ItemID: idCrimsonAmber}, ContainerInventory, 0); err != nil {
		t.Fatalf("AddItem: %v", err)
	}
	it := snap.InventoryItems[0]
	if !it.IsTalisman {
		t.Error("IsTalisman should be true for Crimson Amber Medallion")
	}
	if it.Category != "talismans" {
		t.Errorf("Category = %q", it.Category)
	}
}

func TestAddItem_InsertAtZeroShiftsPositions(t *testing.T) {
	snap := mkSnap(2, 0)
	if err := AddItem(snap, AddItemSpec{ItemID: idDagger}, ContainerInventory, 0); err != nil {
		t.Fatalf("AddItem: %v", err)
	}
	if snap.InventoryItems[0].Source != ItemSourceAdded {
		t.Errorf("position 0 should be the added item, got %+v", snap.InventoryItems[0])
	}
	if snap.InventoryItems[1].UID != "inv:A" || snap.InventoryItems[2].UID != "inv:B" {
		t.Errorf("existing items not shifted: %v", uidsOf(snap.InventoryItems))
	}
	assertPositions(t, snap.InventoryItems, "inv")
}

func TestAddItem_PositionPastLengthAppends(t *testing.T) {
	snap := mkSnap(3, 0)
	if err := AddItem(snap, AddItemSpec{ItemID: idDagger}, ContainerInventory, 999); err != nil {
		t.Fatalf("AddItem: %v", err)
	}
	last := snap.InventoryItems[len(snap.InventoryItems)-1]
	if last.Source != ItemSourceAdded {
		t.Errorf("added item should be last, got %+v", snap.InventoryItems)
	}
}

func TestAddItem_NegativePositionClampedToZero(t *testing.T) {
	snap := mkSnap(2, 0)
	if err := AddItem(snap, AddItemSpec{ItemID: idDagger}, ContainerInventory, -5); err != nil {
		t.Fatalf("AddItem: %v", err)
	}
	if snap.InventoryItems[0].Source != ItemSourceAdded {
		t.Errorf("added item should be at index 0, got %+v", snap.InventoryItems[0])
	}
}

func TestAddItem_QuantityZeroNormalizedToOne(t *testing.T) {
	snap := mkSnap(0, 0)
	if err := AddItem(snap, AddItemSpec{ItemID: idDagger, Quantity: 0}, ContainerInventory, 0); err != nil {
		t.Fatalf("AddItem: %v", err)
	}
	if snap.InventoryItems[0].Quantity != 1 {
		t.Errorf("Quantity = %d, want 1", snap.InventoryItems[0].Quantity)
	}
}

func TestAddItem_RecordQuantityCreatesSeparateCopies(t *testing.T) {
	snap := mkSnap(0, 0)
	if err := AddItem(snap, AddItemSpec{ItemID: idDagger, Quantity: 3}, ContainerInventory, 0); err != nil {
		t.Fatalf("AddItem: %v", err)
	}
	if len(snap.InventoryItems) != 3 {
		t.Fatalf("inv count = %d, want 3", len(snap.InventoryItems))
	}
	seenUIDs := make(map[string]bool)
	for i, it := range snap.InventoryItems {
		if it.ItemID != idDagger {
			t.Errorf("item[%d].ItemID = 0x%08X, want Dagger", i, it.ItemID)
		}
		if it.Quantity != 1 {
			t.Errorf("item[%d].Quantity = %d, want 1 per record", i, it.Quantity)
		}
		if it.UID == "" || seenUIDs[it.UID] {
			t.Errorf("item[%d].UID = %q, want unique non-empty UID", i, it.UID)
		}
		seenUIDs[it.UID] = true
		if it.Position != i {
			t.Errorf("item[%d].Position = %d, want %d", i, it.Position, i)
		}
	}
}

func TestAddItem_RecordQuantityInsertsCopiesAtTargetPosition(t *testing.T) {
	snap := mkSnap(2, 0)
	if err := AddItem(snap, AddItemSpec{ItemID: idDagger, Quantity: 2}, ContainerInventory, 1); err != nil {
		t.Fatalf("AddItem: %v", err)
	}
	if len(snap.InventoryItems) != 4 {
		t.Fatalf("inv count = %d, want 4", len(snap.InventoryItems))
	}
	if snap.InventoryItems[0].UID != "inv:A" {
		t.Errorf("item[0].UID = %q, want inv:A", snap.InventoryItems[0].UID)
	}
	if snap.InventoryItems[3].UID != "inv:B" {
		t.Errorf("item[3].UID = %q, want inv:B", snap.InventoryItems[3].UID)
	}
	for i := 1; i <= 2; i++ {
		if snap.InventoryItems[i].Source != ItemSourceAdded {
			t.Errorf("item[%d].Source = %q, want added", i, snap.InventoryItems[i].Source)
		}
		if snap.InventoryItems[i].Quantity != 1 {
			t.Errorf("item[%d].Quantity = %d, want 1", i, snap.InventoryItems[i].Quantity)
		}
	}
	assertPositions(t, snap.InventoryItems, "inv")
}

func TestAddItem_NewUIDIncrements(t *testing.T) {
	snap := mkSnap(0, 0)
	if err := AddItem(snap, AddItemSpec{ItemID: idDagger}, ContainerInventory, 0); err != nil {
		t.Fatalf("first AddItem: %v", err)
	}
	if err := AddItem(snap, AddItemSpec{ItemID: idDagger}, ContainerStorage, 0); err != nil {
		t.Fatalf("second AddItem: %v", err)
	}
	if err := AddItem(snap, AddItemSpec{ItemID: idDagger}, ContainerInventory, 0); err != nil {
		t.Fatalf("third AddItem: %v", err)
	}
	uids := []string{}
	for _, it := range snap.InventoryItems {
		uids = append(uids, it.UID)
	}
	for _, it := range snap.StorageItems {
		uids = append(uids, it.UID)
	}
	// Expect new:1, new:2, new:3 across the three adds (order unspecified).
	want := map[string]bool{"new:1": true, "new:2": true, "new:3": true}
	got := map[string]bool{}
	for _, u := range uids {
		if strings.HasPrefix(u, "new:") {
			got[u] = true
		}
	}
	for u := range want {
		if !got[u] {
			t.Errorf("expected UID %q in workspace, got %v", u, uids)
		}
	}
}

func TestAddItem_AvoidsCollisionWithExistingNewUID(t *testing.T) {
	snap := mkSnap(0, 0)
	// Pre-seed a collision target.
	preset := mkItem("new:1", 0, ContainerInventory, 0)
	preset.Source = ItemSourceAdded
	preset.OriginalHandle = 0
	snap.InventoryItems = []EditableItem{preset}

	if err := AddItem(snap, AddItemSpec{ItemID: idDagger}, ContainerInventory, 1); err != nil {
		t.Fatalf("AddItem: %v", err)
	}
	if snap.InventoryItems[1].UID == "new:1" {
		t.Errorf("UID collision: %v", uidsOf(snap.InventoryItems))
	}
	if snap.InventoryItems[1].UID != "new:2" {
		t.Errorf("expected new:2, got %q", snap.InventoryItems[1].UID)
	}
}

func TestAddItem_RefreshesValidation(t *testing.T) {
	snap := mkSnap(0, 0)
	snap.Validation = WorkspaceValidationReport{OK: false}
	if err := AddItem(snap, AddItemSpec{ItemID: idDagger}, ContainerInventory, 0); err != nil {
		t.Fatalf("AddItem: %v", err)
	}
	if !snap.Validation.OK {
		t.Errorf("Validation.OK should be true, errors=%+v", snap.Validation.Errors)
	}
	if snap.Validation.InventoryItemCount != 1 {
		t.Errorf("Validation.InventoryItemCount = %d, want 1", snap.Validation.InventoryItemCount)
	}
}

func TestAddItem_UnknownItemIDReturnsError(t *testing.T) {
	snap := mkSnap(0, 0)
	err := AddItem(snap, AddItemSpec{ItemID: 0xDEADBEEF}, ContainerInventory, 0)
	if err == nil {
		t.Fatal("expected error for unknown itemID")
	}
	if len(snap.InventoryItems) != 0 {
		t.Errorf("no item should be added on error, got %+v", snap.InventoryItems)
	}
	if snap.Dirty {
		t.Error("Dirty should remain false on error")
	}
}

func TestAddItem_UnsupportedCategoryReturnsError(t *testing.T) {
	// Lion's Claw — real DB entry, category "ashes_of_war" which is not
	// editable in Phase 1.
	snap := mkSnap(0, 0)
	const aowLionsClaw = uint32(0x80002710)
	err := AddItem(snap, AddItemSpec{ItemID: aowLionsClaw}, ContainerInventory, 0)
	if err == nil {
		t.Fatal("expected error for unsupported category")
	}
	if !strings.Contains(err.Error(), "ashes_of_war") {
		t.Errorf("error should mention category, got %v", err)
	}
}

func TestAddItem_InvalidContainerReturnsError(t *testing.T) {
	snap := mkSnap(0, 0)
	if err := AddItem(snap, AddItemSpec{ItemID: idDagger}, ContainerKind("trash"), 0); err == nil {
		t.Fatal("expected error for invalid container")
	}
}

func TestAddItem_MissingIDsReturnsError(t *testing.T) {
	snap := mkSnap(0, 0)
	if err := AddItem(snap, AddItemSpec{}, ContainerInventory, 0); err == nil {
		t.Fatal("expected error when both ItemID and BaseItemID are zero")
	}
}

func TestAddItem_BaseItemIDFallback(t *testing.T) {
	snap := mkSnap(0, 0)
	// Pass BaseItemID only, ItemID zero.
	if err := AddItem(snap, AddItemSpec{BaseItemID: idDagger}, ContainerInventory, 0); err != nil {
		t.Fatalf("AddItem: %v", err)
	}
	if snap.InventoryItems[0].ItemID != idDagger {
		t.Errorf("ItemID = 0x%X, want fallback to BaseItemID 0x%X",
			snap.InventoryItems[0].ItemID, idDagger)
	}
}

func TestAddItem_LeavesPassThroughUntouched(t *testing.T) {
	snap := mkSnap(0, 0)
	before := append([]RawInventoryRecord(nil), snap.UnsupportedInventoryRecords...)
	if err := AddItem(snap, AddItemSpec{ItemID: idDagger}, ContainerInventory, 0); err != nil {
		t.Fatalf("AddItem: %v", err)
	}
	if !equalRawRecords(snap.UnsupportedInventoryRecords, before) {
		t.Errorf("pass-through mutated: %+v", snap.UnsupportedInventoryRecords)
	}
}
