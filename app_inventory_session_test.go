package main

import (
	"strings"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/editor"
)

func TestStartInventoryEditSession_NoSave(t *testing.T) {
	app := NewApp()
	_, err := app.StartInventoryEditSession(0)
	if err == nil || !strings.Contains(err.Error(), "no save loaded") {
		t.Fatalf("want 'no save loaded', got %v", err)
	}
}

func TestStartInventoryEditSession_InvalidIdx(t *testing.T) {
	app := inventoryOrderFixture(testWeapons)
	if _, err := app.StartInventoryEditSession(-1); err == nil {
		t.Fatal("expected error for negative index")
	}
	if _, err := app.StartInventoryEditSession(10); err == nil {
		t.Fatal("expected error for index 10")
	}
}

func TestStartInventoryEditSession_EmptySlot(t *testing.T) {
	app := inventoryOrderFixture(testWeapons)
	app.save.Slots[0].Version = 0
	if _, err := app.StartInventoryEditSession(0); err == nil {
		t.Fatal("expected error for empty slot")
	}
}

func TestStartInventoryEditSession_ReturnsEditableWeapons(t *testing.T) {
	app := inventoryOrderFixture(testWeapons)
	snap, err := app.StartInventoryEditSession(0)
	if err != nil {
		t.Fatalf("StartInventoryEditSession: %v", err)
	}
	if snap.SessionID == "" {
		t.Fatal("session ID should be non-empty")
	}
	if snap.CharacterIndex != 0 {
		t.Errorf("CharacterIndex = %d, want 0", snap.CharacterIndex)
	}
	if len(snap.InventoryItems) != len(testWeapons) {
		t.Fatalf("got %d editable inventory items, want %d (snapshot: %+v)",
			len(snap.InventoryItems), len(testWeapons), snap.InventoryItems)
	}
	// Items should be sorted by AcquisitionIndex (testWeapons happens to
	// be in ascending order already).
	for i := range snap.InventoryItems {
		if int(snap.InventoryItems[i].Position) != i {
			t.Errorf("Position[%d] = %d, want %d", i, snap.InventoryItems[i].Position, i)
		}
	}
	// Validation should populate the report inside the snapshot.
	if snap.Validation.InventoryItemCount != len(testWeapons) {
		t.Errorf("Validation.InventoryItemCount = %d, want %d",
			snap.Validation.InventoryItemCount, len(testWeapons))
	}
	// Session should be retrievable.
	got, err := app.GetInventoryEditSession(snap.SessionID)
	if err != nil {
		t.Fatalf("GetInventoryEditSession: %v", err)
	}
	if got.SessionID != snap.SessionID {
		t.Errorf("SessionID mismatch: %q vs %q", got.SessionID, snap.SessionID)
	}
}

func TestStartInventoryEditSession_ReplacesPreviousForSameChar(t *testing.T) {
	app := inventoryOrderFixture(testWeapons)
	first, err := app.StartInventoryEditSession(0)
	if err != nil {
		t.Fatalf("first Start: %v", err)
	}
	second, err := app.StartInventoryEditSession(0)
	if err != nil {
		t.Fatalf("second Start: %v", err)
	}
	if first.SessionID == second.SessionID {
		t.Errorf("expected new session ID on re-Start (got %q twice)", first.SessionID)
	}
	if _, err := app.GetInventoryEditSession(first.SessionID); err == nil {
		t.Errorf("expected first session to be discarded after second Start")
	}
	if _, err := app.GetInventoryEditSession(second.SessionID); err != nil {
		t.Errorf("second session should still be active: %v", err)
	}
}

func TestValidateInventoryWorkspace_OK(t *testing.T) {
	app := inventoryOrderFixture(testWeapons)
	snap, err := app.StartInventoryEditSession(0)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	rep, err := app.ValidateInventoryWorkspace(snap.SessionID)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if !rep.OK {
		t.Errorf("expected clean baseline to validate OK, errors=%+v", rep.Errors)
	}
}

func TestValidateInventoryWorkspace_UnknownSession(t *testing.T) {
	app := NewApp()
	_, err := app.ValidateInventoryWorkspace("nope")
	if err == nil {
		t.Fatal("expected error for unknown session ID")
	}
}

func TestDiscardInventoryEditSession_Removes(t *testing.T) {
	app := inventoryOrderFixture(testWeapons)
	snap, err := app.StartInventoryEditSession(0)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := app.DiscardInventoryEditSession(snap.SessionID); err != nil {
		t.Fatalf("Discard: %v", err)
	}
	if _, err := app.GetInventoryEditSession(snap.SessionID); err == nil {
		t.Error("session should be gone after Discard")
	}
	// Discard is idempotent.
	if err := app.DiscardInventoryEditSession(snap.SessionID); err != nil {
		t.Errorf("second Discard should be no-op, got %v", err)
	}
}

func TestStartInventoryEditSession_DoesNotMutateSlot(t *testing.T) {
	app := inventoryOrderFixture(testWeapons)
	slot := &app.save.Slots[0]
	dataBefore := make([]byte, len(slot.Data))
	copy(dataBefore, slot.Data)
	gaMapBefore := make(map[uint32]uint32, len(slot.GaMap))
	for k, v := range slot.GaMap {
		gaMapBefore[k] = v
	}

	if _, err := app.StartInventoryEditSession(0); err != nil {
		t.Fatalf("Start: %v", err)
	}

	for i := range slot.Data {
		if slot.Data[i] != dataBefore[i] {
			t.Fatalf("slot.Data[%d] mutated: %02X → %02X", i, dataBefore[i], slot.Data[i])
		}
	}
	if len(slot.GaMap) != len(gaMapBefore) {
		t.Errorf("GaMap size changed: %d → %d", len(gaMapBefore), len(slot.GaMap))
	}
}

// _ = editor.* references silence unused-import warnings if the workspace
// types are touched directly later in this file.
var _ = editor.SupportedCategories

// ─── Phase 1.5: RAM-only mutations ──────────────────────────────────────────

func uidOfFirstEditable(t *testing.T, snap editor.InventoryWorkspaceSnapshot) string {
	t.Helper()
	if len(snap.InventoryItems) == 0 {
		t.Fatal("expected at least one editable inventory item")
	}
	return snap.InventoryItems[0].UID
}

func TestMoveInventoryWorkspaceItem_InventoryToStorage(t *testing.T) {
	app := inventoryOrderFixture(testWeapons)
	snap, err := app.StartInventoryEditSession(0)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	uid := uidOfFirstEditable(t, snap)
	invCountBefore := len(snap.InventoryItems)

	updated, err := app.MoveInventoryWorkspaceItem(snap.SessionID, uid, "storage", 0)
	if err != nil {
		t.Fatalf("Move: %v", err)
	}
	if !updated.Dirty {
		t.Error("Dirty should be true after move")
	}
	if len(updated.InventoryItems) != invCountBefore-1 {
		t.Errorf("inv count = %d, want %d", len(updated.InventoryItems), invCountBefore-1)
	}
	if len(updated.StorageItems) != 1 || updated.StorageItems[0].UID != uid {
		t.Errorf("storage = %+v, want UID %q at index 0", updated.StorageItems, uid)
	}
	if updated.StorageItems[0].Container != editor.ContainerStorage {
		t.Errorf("moved item Container = %q, want storage", updated.StorageItems[0].Container)
	}
}

func TestMoveInventoryWorkspaceItem_UnknownSession(t *testing.T) {
	app := NewApp()
	_, err := app.MoveInventoryWorkspaceItem("nope", "hnd:0x80800001", "inventory", 0)
	if err == nil {
		t.Fatal("expected error for unknown session")
	}
}

func TestMoveInventoryWorkspaceItem_UnknownUID(t *testing.T) {
	app := inventoryOrderFixture(testWeapons)
	snap, err := app.StartInventoryEditSession(0)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if _, err := app.MoveInventoryWorkspaceItem(snap.SessionID, "hnd:0xDEADBEEF", "inventory", 0); err == nil {
		t.Fatal("expected error for unknown UID")
	}
}

func TestMoveInventoryWorkspaceItem_InvalidContainer(t *testing.T) {
	app := inventoryOrderFixture(testWeapons)
	snap, err := app.StartInventoryEditSession(0)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	uid := uidOfFirstEditable(t, snap)
	if _, err := app.MoveInventoryWorkspaceItem(snap.SessionID, uid, "trash", 0); err == nil {
		t.Fatal("expected error for invalid container")
	}
}

func TestTransferInventoryWorkspaceItem_AppendsToTarget(t *testing.T) {
	app := inventoryOrderFixture(testWeapons)
	snap, err := app.StartInventoryEditSession(0)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	uid := uidOfFirstEditable(t, snap)

	updated, err := app.TransferInventoryWorkspaceItem(snap.SessionID, uid, "storage")
	if err != nil {
		t.Fatalf("Transfer: %v", err)
	}
	if len(updated.StorageItems) == 0 || updated.StorageItems[len(updated.StorageItems)-1].UID != uid {
		t.Errorf("transfer should append to storage, got %+v", updated.StorageItems)
	}
}

func TestRemoveInventoryWorkspaceItem_DropsItem(t *testing.T) {
	app := inventoryOrderFixture(testWeapons)
	snap, err := app.StartInventoryEditSession(0)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	uid := uidOfFirstEditable(t, snap)
	invCountBefore := len(snap.InventoryItems)

	updated, err := app.RemoveInventoryWorkspaceItem(snap.SessionID, uid)
	if err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if len(updated.InventoryItems) != invCountBefore-1 {
		t.Errorf("inv count after remove = %d, want %d", len(updated.InventoryItems), invCountBefore-1)
	}
	for _, it := range updated.InventoryItems {
		if it.UID == uid {
			t.Errorf("removed UID %q still present", uid)
		}
	}
	if !updated.Dirty {
		t.Error("Dirty should be true after remove")
	}
}

func TestRemoveInventoryWorkspaceItem_UnknownUID(t *testing.T) {
	app := inventoryOrderFixture(testWeapons)
	snap, err := app.StartInventoryEditSession(0)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if _, err := app.RemoveInventoryWorkspaceItem(snap.SessionID, "hnd:0xDEADBEEF"); err == nil {
		t.Fatal("expected error for unknown UID")
	}
}

func TestValidateAfterMutation_StillRuns(t *testing.T) {
	app := inventoryOrderFixture(testWeapons)
	snap, err := app.StartInventoryEditSession(0)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	uid := uidOfFirstEditable(t, snap)
	if _, err := app.MoveInventoryWorkspaceItem(snap.SessionID, uid, "storage", 0); err != nil {
		t.Fatalf("Move: %v", err)
	}
	rep, err := app.ValidateInventoryWorkspace(snap.SessionID)
	if err != nil {
		t.Fatalf("Validate after move: %v", err)
	}
	if !rep.OK {
		t.Errorf("expected OK=true after clean move, errors=%+v", rep.Errors)
	}
}

// ─── Phase 1.6: Add item ────────────────────────────────────────────────────

func TestAddInventoryWorkspaceItem_WeaponToInventory(t *testing.T) {
	app := inventoryOrderFixture(testWeapons)
	snap, err := app.StartInventoryEditSession(0)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	invBefore := len(snap.InventoryItems)

	updated, err := app.AddInventoryWorkspaceItem(snap.SessionID,
		editor.AddItemSpec{ItemID: 0x000F4240}, "inventory", 0) // Dagger
	if err != nil {
		t.Fatalf("AddInventoryWorkspaceItem: %v", err)
	}
	if len(updated.InventoryItems) != invBefore+1 {
		t.Errorf("inv count = %d, want %d", len(updated.InventoryItems), invBefore+1)
	}
	added := updated.InventoryItems[0]
	if added.Source != editor.ItemSourceAdded {
		t.Errorf("Source = %q, want added", added.Source)
	}
	if added.OriginalHandle != 0 {
		t.Errorf("OriginalHandle = 0x%X, want 0", added.OriginalHandle)
	}
	if added.HasGaItem {
		t.Error("HasGaItem should be false")
	}
	if !updated.Dirty {
		t.Error("Dirty should be true")
	}
}

func TestAddInventoryWorkspaceItem_TalismanToStorage(t *testing.T) {
	app := inventoryOrderFixture(testWeapons)
	snap, err := app.StartInventoryEditSession(0)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	updated, err := app.AddInventoryWorkspaceItem(snap.SessionID,
		editor.AddItemSpec{ItemID: 0x200003E8}, "storage", 0) // Crimson Amber Medallion
	if err != nil {
		t.Fatalf("AddInventoryWorkspaceItem: %v", err)
	}
	if len(updated.StorageItems) == 0 || !updated.StorageItems[0].IsTalisman {
		t.Errorf("expected talisman at storage[0], got %+v", updated.StorageItems)
	}
}

func TestAddInventoryWorkspaceItem_UnknownSession(t *testing.T) {
	app := NewApp()
	_, err := app.AddInventoryWorkspaceItem("nope",
		editor.AddItemSpec{ItemID: 0x000F4240}, "inventory", 0)
	if err == nil {
		t.Fatal("expected error for unknown session")
	}
}

func TestAddInventoryWorkspaceItem_InvalidContainer(t *testing.T) {
	app := inventoryOrderFixture(testWeapons)
	snap, err := app.StartInventoryEditSession(0)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if _, err := app.AddInventoryWorkspaceItem(snap.SessionID,
		editor.AddItemSpec{ItemID: 0x000F4240}, "trash", 0); err == nil {
		t.Fatal("expected error for invalid container")
	}
}

func TestAddInventoryWorkspaceItem_UnknownItem(t *testing.T) {
	app := inventoryOrderFixture(testWeapons)
	snap, err := app.StartInventoryEditSession(0)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if _, err := app.AddInventoryWorkspaceItem(snap.SessionID,
		editor.AddItemSpec{ItemID: 0xDEADBEEF}, "inventory", 0); err == nil {
		t.Fatal("expected error for unknown itemID")
	}
}

func TestAddInventoryWorkspaceItem_AfterMoveAndTransferPreservesOrdering(t *testing.T) {
	app := inventoryOrderFixture(testWeapons)
	snap, err := app.StartInventoryEditSession(0)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	firstUID := snap.InventoryItems[0].UID
	secondUID := snap.InventoryItems[1].UID

	// Reorder inside inventory: move first item to position 2.
	if _, err := app.MoveInventoryWorkspaceItem(snap.SessionID, firstUID, "inventory", 2); err != nil {
		t.Fatalf("Move: %v", err)
	}
	// Transfer secondUID to storage.
	if _, err := app.TransferInventoryWorkspaceItem(snap.SessionID, secondUID, "storage"); err != nil {
		t.Fatalf("Transfer: %v", err)
	}
	// Add a new weapon at inventory[0].
	updated, err := app.AddInventoryWorkspaceItem(snap.SessionID,
		editor.AddItemSpec{ItemID: 0x000F4240}, "inventory", 0)
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	// firstUID should now be at index 2 (since we moved it; +1 not applied
	// because we already removed secondUID from inventory before Add).
	// Verify firstUID is somewhere in inventory and secondUID is in storage.
	foundFirst := false
	for _, it := range updated.InventoryItems {
		if it.UID == firstUID {
			foundFirst = true
		}
	}
	if !foundFirst {
		t.Errorf("firstUID lost from inventory: %v", updated.InventoryItems)
	}
	foundSecond := false
	for _, it := range updated.StorageItems {
		if it.UID == secondUID {
			foundSecond = true
		}
	}
	if !foundSecond {
		t.Errorf("secondUID not in storage: %v", updated.StorageItems)
	}
	// Added item should be Source=added.
	if updated.InventoryItems[0].Source != editor.ItemSourceAdded {
		t.Errorf("inventory[0] should be Source=added, got %+v", updated.InventoryItems[0])
	}
}

func TestMutations_DoNotMutateSlotData(t *testing.T) {
	app := inventoryOrderFixture(testWeapons)
	slot := &app.save.Slots[0]
	dataBefore := make([]byte, len(slot.Data))
	copy(dataBefore, slot.Data)
	gaMapBefore := make(map[uint32]uint32, len(slot.GaMap))
	for k, v := range slot.GaMap {
		gaMapBefore[k] = v
	}

	snap, err := app.StartInventoryEditSession(0)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	uid := uidOfFirstEditable(t, snap)
	if _, err := app.MoveInventoryWorkspaceItem(snap.SessionID, uid, "storage", 0); err != nil {
		t.Fatalf("Move: %v", err)
	}
	if _, err := app.TransferInventoryWorkspaceItem(snap.SessionID, snap.InventoryItems[1].UID, "storage"); err != nil {
		t.Fatalf("Transfer: %v", err)
	}
	if _, err := app.AddInventoryWorkspaceItem(snap.SessionID,
		editor.AddItemSpec{ItemID: 0x000F4240}, "inventory", 0); err != nil {
		t.Fatalf("Add: %v", err)
	}
	// Patch the newly added weapon (Phase 1.7).
	if _, err := app.UpdateInventoryWorkspaceWeapon(snap.SessionID,
		"new:1",
		editor.WeaponPatch{
			SetUpgrade:      true,
			Upgrade:         8,
			SetInfusionName: true,
			InfusionName:    "Heavy",
			SetAoWItemID:    true,
			AoWItemID:       0x80002710, // Lion's Claw
		}); err != nil {
		t.Fatalf("UpdateWeapon: %v", err)
	}
	if _, err := app.RemoveInventoryWorkspaceItem(snap.SessionID, snap.InventoryItems[2].UID); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	for i := range slot.Data {
		if slot.Data[i] != dataBefore[i] {
			t.Fatalf("slot.Data[%d] mutated: %02X → %02X", i, dataBefore[i], slot.Data[i])
		}
	}
	if len(slot.GaMap) != len(gaMapBefore) {
		t.Errorf("GaMap size changed: %d → %d", len(gaMapBefore), len(slot.GaMap))
	}
	for k, v := range gaMapBefore {
		if got, ok := slot.GaMap[k]; !ok || got != v {
			t.Errorf("GaMap[0x%08X] mutated: %v → %v (ok=%v)", k, v, got, ok)
		}
	}
}

// ─── Phase 1.7: RAM-only weapon updates ─────────────────────────────────────

func TestUpdateInventoryWorkspaceWeapon_UpgradesExistingWeapon(t *testing.T) {
	app := inventoryOrderFixture(testWeapons)
	snap, err := app.StartInventoryEditSession(0)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	uid := snap.InventoryItems[0].UID // Dagger (0x000F4240)
	baseID := snap.InventoryItems[0].BaseItemID

	updated, err := app.UpdateInventoryWorkspaceWeapon(snap.SessionID, uid, editor.WeaponPatch{
		SetUpgrade: true, Upgrade: 5,
	})
	if err != nil {
		t.Fatalf("UpdateWeapon: %v", err)
	}
	patched := updated.InventoryItems[0]
	if patched.CurrentUpgrade != 5 || patched.ItemID != baseID+5 {
		t.Errorf("expected upgrade=5 and ItemID=baseID+5, got %+v", patched)
	}
	if !patched.HasPendingWeaponPatch {
		t.Error("HasPendingWeaponPatch should be true")
	}
	if !updated.Dirty {
		t.Error("Dirty should be true")
	}
}

func TestUpdateInventoryWorkspaceWeapon_OnAddedItemPreservesSourceAndHandle(t *testing.T) {
	app := inventoryOrderFixture(testWeapons)
	snap, err := app.StartInventoryEditSession(0)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	addSnap, err := app.AddInventoryWorkspaceItem(snap.SessionID,
		editor.AddItemSpec{ItemID: 0x003085E0}, "inventory", 0) // Claymore
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	addedUID := addSnap.InventoryItems[0].UID

	updated, err := app.UpdateInventoryWorkspaceWeapon(snap.SessionID, addedUID, editor.WeaponPatch{
		SetUpgrade:      true,
		Upgrade:         15,
		SetInfusionName: true,
		InfusionName:    "Cold",
	})
	if err != nil {
		t.Fatalf("UpdateWeapon: %v", err)
	}
	patched := updated.InventoryItems[0]
	if patched.UID != addedUID {
		t.Errorf("UID changed: %q → %q", addedUID, patched.UID)
	}
	if patched.Source != editor.ItemSourceAdded {
		t.Errorf("Source = %q, want added (must not change)", patched.Source)
	}
	if patched.OriginalHandle != 0 {
		t.Errorf("OriginalHandle = 0x%X, want 0", patched.OriginalHandle)
	}
	if patched.CurrentUpgrade != 15 || patched.InfusionName != "Cold" {
		t.Errorf("patch not applied: %+v", patched)
	}
}

func TestUpdateInventoryWorkspaceWeapon_SetAoWStoresPending(t *testing.T) {
	app := inventoryOrderFixture(testWeapons)
	snap, err := app.StartInventoryEditSession(0)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	uid := snap.InventoryItems[0].UID

	updated, err := app.UpdateInventoryWorkspaceWeapon(snap.SessionID, uid, editor.WeaponPatch{
		SetAoWItemID: true, AoWItemID: 0x80002710, // Lion's Claw
	})
	if err != nil {
		t.Fatalf("UpdateWeapon: %v", err)
	}
	it := updated.InventoryItems[0]
	if it.PendingAoWItemID != 0x80002710 {
		t.Errorf("PendingAoWItemID = 0x%08X, want 0x80002710", it.PendingAoWItemID)
	}
	if it.PendingAoWName != "Lion's Claw" {
		t.Errorf("PendingAoWName = %q, want Lion's Claw", it.PendingAoWName)
	}
}

func TestUpdateInventoryWorkspaceWeapon_NonWeaponReturnsError(t *testing.T) {
	app := inventoryOrderFixture(testWeapons)
	snap, err := app.StartInventoryEditSession(0)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	// Inject a talisman via Add then try to update — should fail.
	addSnap, err := app.AddInventoryWorkspaceItem(snap.SessionID,
		editor.AddItemSpec{ItemID: 0x200003E8}, "inventory", 0) // Crimson Amber Medallion
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	talismanUID := addSnap.InventoryItems[0].UID

	_, err = app.UpdateInventoryWorkspaceWeapon(snap.SessionID, talismanUID, editor.WeaponPatch{
		SetUpgrade: true, Upgrade: 1,
	})
	if err == nil {
		t.Fatal("expected error for talisman item")
	}
	if !strings.Contains(err.Error(), "not weapon-editable") {
		t.Errorf("error should mention non-weapon, got %v", err)
	}
}

func TestUpdateInventoryWorkspaceWeapon_UnknownSession(t *testing.T) {
	app := NewApp()
	_, err := app.UpdateInventoryWorkspaceWeapon("nope", "hnd:0x80800001", editor.WeaponPatch{
		SetUpgrade: true, Upgrade: 1,
	})
	if err == nil {
		t.Fatal("expected error for unknown session")
	}
}

func TestUpdateInventoryWorkspaceWeapon_UnknownUID(t *testing.T) {
	app := inventoryOrderFixture(testWeapons)
	snap, err := app.StartInventoryEditSession(0)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	_, err = app.UpdateInventoryWorkspaceWeapon(snap.SessionID, "hnd:0xDEADBEEF", editor.WeaponPatch{
		SetUpgrade: true, Upgrade: 1,
	})
	if err == nil {
		t.Fatal("expected error for unknown UID")
	}
}

func TestUpdateInventoryWorkspaceWeapon_InvalidUpgradeRejected(t *testing.T) {
	app := inventoryOrderFixture(testWeapons)
	snap, err := app.StartInventoryEditSession(0)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	uid := snap.InventoryItems[0].UID
	_, err = app.UpdateInventoryWorkspaceWeapon(snap.SessionID, uid, editor.WeaponPatch{
		SetUpgrade: true, Upgrade: 999,
	})
	if err == nil {
		t.Fatal("expected error for upgrade beyond MaxUpgrade")
	}
}

func TestUpdateInventoryWorkspaceWeapon_InvalidAoWRejected(t *testing.T) {
	app := inventoryOrderFixture(testWeapons)
	snap, err := app.StartInventoryEditSession(0)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	uid := snap.InventoryItems[0].UID
	_, err = app.UpdateInventoryWorkspaceWeapon(snap.SessionID, uid, editor.WeaponPatch{
		SetAoWItemID: true, AoWItemID: 0xDEADBEEF,
	})
	if err == nil {
		t.Fatal("expected error for unknown AoW ID")
	}
}
