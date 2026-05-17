package main

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
	"github.com/oisis/EldenRing-SaveForge/backend/editor"
)

// realSaveAppForSave loads tmp/save/ER0000.sl2 into a fresh App and
// returns the App plus the index of the first non-empty slot. Tests
// using this helper skip cleanly when the fixture isn't checked in
// (tmp/ is gitignored).
func realSaveAppForSave(t *testing.T) (*App, int) {
	t.Helper()
	const savePath = "tmp/save/ER0000.sl2"
	if _, err := os.Stat(savePath); os.IsNotExist(err) {
		t.Skipf("test save not found: %s", savePath)
	}
	save, err := core.LoadSave(savePath)
	if err != nil {
		t.Fatalf("LoadSave: %v", err)
	}
	app := NewApp()
	app.save = save
	for i := 0; i < 10; i++ {
		if save.Slots[i].Version != 0 {
			return app, i
		}
	}
	t.Fatal("no active slot found")
	return nil, -1
}

// snapshotSlotBytes returns a copy of slot.Data for byte-for-byte
// equality assertions.
func snapshotSlotBytes(slot *core.SaveSlot) []byte {
	cp := make([]byte, len(slot.Data))
	copy(cp, slot.Data)
	return cp
}

func assertNoDuplicateHandles(t *testing.T, snap editor.InventoryWorkspaceSnapshot) {
	t.Helper()
	seen := map[uint32]string{}
	for _, it := range snap.InventoryItems {
		if it.OriginalHandle == 0 {
			continue
		}
		if other, dup := seen[it.OriginalHandle]; dup {
			t.Errorf("duplicate handle 0x%08X in inventory: %s and %s", it.OriginalHandle, other, it.UID)
		}
		seen[it.OriginalHandle] = it.UID
	}
	for _, it := range snap.StorageItems {
		if it.OriginalHandle == 0 {
			continue
		}
		if other, dup := seen[it.OriginalHandle]; dup {
			t.Errorf("duplicate handle 0x%08X across containers: %s and %s", it.OriginalHandle, other, it.UID)
		}
		seen[it.OriginalHandle] = it.UID
	}
}

func assertNoDuplicateAcqIndices(t *testing.T, snap editor.InventoryWorkspaceSnapshot) {
	t.Helper()
	check := func(items []editor.EditableItem, label string) {
		seen := map[uint32]string{}
		for _, it := range items {
			if other, dup := seen[it.AcquisitionIndex]; dup {
				t.Errorf("%s: duplicate AcquisitionIndex %d: %s and %s", label, it.AcquisitionIndex, other, it.UID)
			}
			seen[it.AcquisitionIndex] = it.UID
		}
	}
	check(snap.InventoryItems, "inventory")
	check(snap.StorageItems, "storage")
}

// ─── Rejection tests (no real save needed) ────────────────────────

func TestSaveInventoryWorkspaceChanges_UnknownSession(t *testing.T) {
	app := NewApp()
	_, err := app.SaveInventoryWorkspaceChanges("nope")
	if err == nil {
		t.Fatal("expected error for unknown session")
	}
}

func TestSaveInventoryWorkspaceChanges_RejectsPendingAoW(t *testing.T) {
	app, idx := realSaveAppForSave(t)
	snap, err := app.StartInventoryEditSession(idx)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	// Set pending AoW on an existing weapon. Use the first inventory
	// item that's a weapon.
	weaponUID := ""
	for _, it := range snap.InventoryItems {
		if it.IsWeapon {
			weaponUID = it.UID
			break
		}
	}
	if weaponUID == "" {
		t.Skip("no weapon in inventory")
	}
	if _, err := app.UpdateInventoryWorkspaceWeapon(snap.SessionID, weaponUID, editor.WeaponPatch{
		SetAoWItemID: true, AoWItemID: 0x80002710, // Lion's Claw
	}); err != nil {
		t.Fatalf("UpdateWeapon: %v", err)
	}

	slot := &app.save.Slots[idx]
	before := snapshotSlotBytes(slot)
	_, err = app.SaveInventoryWorkspaceChanges(snap.SessionID)
	if err == nil {
		t.Fatal("expected error for pending AoW")
	}
	if !strings.Contains(err.Error(), "pending AoW") {
		t.Errorf("error should mention pending AoW, got %v", err)
	}
	if !bytes.Equal(slot.Data, before) {
		t.Error("slot.Data mutated after rejection")
	}
}

func TestSaveInventoryWorkspaceChanges_RejectsTransfer(t *testing.T) {
	app, idx := realSaveAppForSave(t)
	snap, err := app.StartInventoryEditSession(idx)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	weaponUID := ""
	for _, it := range snap.InventoryItems {
		if it.IsWeapon {
			weaponUID = it.UID
			break
		}
	}
	if weaponUID == "" {
		t.Skip("no weapon in inventory")
	}
	if _, err := app.MoveInventoryWorkspaceItem(snap.SessionID, weaponUID, "storage", 0); err != nil {
		t.Fatalf("Move: %v", err)
	}

	slot := &app.save.Slots[idx]
	before := snapshotSlotBytes(slot)
	_, err = app.SaveInventoryWorkspaceChanges(snap.SessionID)
	if err == nil {
		t.Fatal("expected error for transfer")
	}
	if !strings.Contains(err.Error(), "transfer not implemented") {
		t.Errorf("error should mention transfer, got %v", err)
	}
	if !bytes.Equal(slot.Data, before) {
		t.Error("slot.Data mutated after transfer rejection")
	}
}

func TestSaveInventoryWorkspaceChanges_RejectsRemove(t *testing.T) {
	app, idx := realSaveAppForSave(t)
	snap, err := app.StartInventoryEditSession(idx)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	weaponUID := ""
	for _, it := range snap.InventoryItems {
		if it.IsWeapon {
			weaponUID = it.UID
			break
		}
	}
	if weaponUID == "" {
		t.Skip("no weapon in inventory")
	}
	if _, err := app.RemoveInventoryWorkspaceItem(snap.SessionID, weaponUID); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	slot := &app.save.Slots[idx]
	before := snapshotSlotBytes(slot)
	_, err = app.SaveInventoryWorkspaceChanges(snap.SessionID)
	if err == nil {
		t.Fatal("expected error for remove")
	}
	if !strings.Contains(err.Error(), "remove not implemented") {
		t.Errorf("error should mention remove, got %v", err)
	}
	if !bytes.Equal(slot.Data, before) {
		t.Error("slot.Data mutated after remove rejection")
	}
}

// ─── Reorder tests ────────────────────────────────────────────────

func TestSaveInventoryWorkspaceChanges_ReorderInventory(t *testing.T) {
	app, idx := realSaveAppForSave(t)
	snap, err := app.StartInventoryEditSession(idx)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if len(snap.InventoryItems) < 3 {
		t.Skip("need at least 3 inventory items to reorder")
	}
	firstUID := snap.InventoryItems[0].UID
	firstHandle := snap.InventoryItems[0].OriginalHandle
	// Move(0 → 2) on [A, B, C, D] yields [B, C, A, D]: remove A, then
	// insert at index 2 of the shortened list.
	wantOrder := []string{
		snap.InventoryItems[1].UID,
		snap.InventoryItems[2].UID,
		snap.InventoryItems[0].UID,
	}

	// Move position 0 → position 2.
	if _, err := app.MoveInventoryWorkspaceItem(snap.SessionID, firstUID, "inventory", 2); err != nil {
		t.Fatalf("Move: %v", err)
	}

	updated, err := app.SaveInventoryWorkspaceChanges(snap.SessionID)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if updated.Dirty {
		t.Error("Dirty should be false after Save")
	}
	if len(updated.InventoryItems) < 3 {
		t.Fatalf("inventory shrank: got %d", len(updated.InventoryItems))
	}
	for i, want := range wantOrder {
		if updated.InventoryItems[i].UID != want {
			t.Errorf("position %d: UID = %q, want %q", i, updated.InventoryItems[i].UID, want)
		}
	}
	// Handle of the moved item must be stable.
	for _, it := range updated.InventoryItems {
		if it.UID == firstUID && it.OriginalHandle != firstHandle {
			t.Errorf("moved item handle changed: 0x%08X → 0x%08X", firstHandle, it.OriginalHandle)
		}
	}
	assertNoDuplicateHandles(t, updated)
	assertNoDuplicateAcqIndices(t, updated)
}

func TestSaveInventoryWorkspaceChanges_ReorderStorage(t *testing.T) {
	app, idx := realSaveAppForSave(t)
	snap, err := app.StartInventoryEditSession(idx)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if len(snap.StorageItems) < 2 {
		t.Skipf("need at least 2 storage items to reorder; got %d", len(snap.StorageItems))
	}
	firstUID := snap.StorageItems[0].UID
	secondUID := snap.StorageItems[1].UID

	if _, err := app.MoveInventoryWorkspaceItem(snap.SessionID, firstUID, "storage", 1); err != nil {
		t.Fatalf("Move: %v", err)
	}

	updated, err := app.SaveInventoryWorkspaceChanges(snap.SessionID)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if updated.Dirty {
		t.Error("Dirty should be false after Save")
	}
	// secondUID should now be at position 0, firstUID at position 1.
	if updated.StorageItems[0].UID != secondUID {
		t.Errorf("storage[0].UID = %q, want %q", updated.StorageItems[0].UID, secondUID)
	}
	if updated.StorageItems[1].UID != firstUID {
		t.Errorf("storage[1].UID = %q, want %q", updated.StorageItems[1].UID, firstUID)
	}
	assertNoDuplicateHandles(t, updated)
	assertNoDuplicateAcqIndices(t, updated)
}

// ─── Add tests ────────────────────────────────────────────────────

func TestSaveInventoryWorkspaceChanges_AddWeaponToInventory(t *testing.T) {
	app, idx := realSaveAppForSave(t)
	snap, err := app.StartInventoryEditSession(idx)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	invBefore := len(snap.InventoryItems)

	// Add a base Dagger at workspace position 0.
	if _, err := app.AddInventoryWorkspaceItem(snap.SessionID,
		editor.AddItemSpec{ItemID: 0x000F4240}, "inventory", 0); err != nil {
		t.Fatalf("Add: %v", err)
	}

	updated, err := app.SaveInventoryWorkspaceChanges(snap.SessionID)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if updated.Dirty {
		t.Error("Dirty should be false after Save")
	}
	if len(updated.InventoryItems) != invBefore+1 {
		t.Errorf("inv count = %d, want %d", len(updated.InventoryItems), invBefore+1)
	}
	// The newly added item should be at position 0 with a real
	// (non-zero) handle and ItemSourceOriginal (since it now comes
	// from the freshly reparsed slot, not a pending workspace add).
	added := updated.InventoryItems[0]
	if added.Name != "Dagger" {
		t.Errorf("expected Dagger at inv[0], got %q", added.Name)
	}
	if added.OriginalHandle == 0 {
		t.Error("added item must have a real handle after Save")
	}
	if added.Source != editor.ItemSourceOriginal {
		t.Errorf("post-Save Source = %q, want original (reparsed)", added.Source)
	}
	assertNoDuplicateHandles(t, updated)
	assertNoDuplicateAcqIndices(t, updated)
}

func TestSaveInventoryWorkspaceChanges_AddTalismanToStorage(t *testing.T) {
	app, idx := realSaveAppForSave(t)
	snap, err := app.StartInventoryEditSession(idx)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	stoBefore := len(snap.StorageItems)

	// Add Crimson Amber Medallion to storage.
	if _, err := app.AddInventoryWorkspaceItem(snap.SessionID,
		editor.AddItemSpec{ItemID: 0x200003E8}, "storage", 0); err != nil {
		t.Fatalf("Add: %v", err)
	}

	updated, err := app.SaveInventoryWorkspaceChanges(snap.SessionID)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	// Storage may have grown by 1 OR may have reused an existing
	// stackable handle (if the save already had this talisman). Either
	// way, post-Save snapshot should contain the talisman somewhere.
	found := false
	for _, it := range updated.StorageItems {
		if it.ItemID == 0x200003E8 && it.IsTalisman {
			if it.OriginalHandle == 0 {
				t.Errorf("talisman has zero handle: %+v", it)
			}
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Crimson Amber Medallion not in storage after save (had %d items before, %d after)",
			stoBefore, len(updated.StorageItems))
	}
	assertNoDuplicateHandles(t, updated)
	assertNoDuplicateAcqIndices(t, updated)
}

// ─── Weapon update test ───────────────────────────────────────────

func TestSaveInventoryWorkspaceChanges_WeaponUpgrade(t *testing.T) {
	app, idx := realSaveAppForSave(t)
	snap, err := app.StartInventoryEditSession(idx)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	// Find a weapon at +0 with MaxUpgrade >= 3.
	target := ""
	var targetBase, targetHandle uint32
	for _, it := range snap.InventoryItems {
		if it.IsWeapon && it.CurrentUpgrade == 0 && it.MaxUpgrade >= 3 {
			target = it.UID
			targetBase = it.BaseItemID
			targetHandle = it.OriginalHandle
			break
		}
	}
	if target == "" {
		t.Skip("no eligible weapon for upgrade test")
	}

	if _, err := app.UpdateInventoryWorkspaceWeapon(snap.SessionID, target, editor.WeaponPatch{
		SetUpgrade: true, Upgrade: 3,
	}); err != nil {
		t.Fatalf("UpdateWeapon: %v", err)
	}
	updated, err := app.SaveInventoryWorkspaceChanges(snap.SessionID)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if updated.Dirty {
		t.Error("Dirty should be false after Save")
	}
	// The patched item should be findable in the reparsed snapshot
	// with the new ItemID = base+3 and CurrentUpgrade=3.
	found := false
	for _, it := range updated.InventoryItems {
		if it.OriginalHandle == targetHandle {
			if it.ItemID != targetBase+3 {
				t.Errorf("ItemID = 0x%08X, want 0x%08X", it.ItemID, targetBase+3)
			}
			if it.CurrentUpgrade != 3 {
				t.Errorf("CurrentUpgrade = %d, want 3", it.CurrentUpgrade)
			}
			found = true
			break
		}
	}
	if !found {
		t.Error("patched weapon not found in reparsed snapshot")
	}
	assertNoDuplicateHandles(t, updated)
}

// ─── Combined-edit test ───────────────────────────────────────────

func TestSaveInventoryWorkspaceChanges_ReorderAddUpdateCombined(t *testing.T) {
	app, idx := realSaveAppForSave(t)
	snap, err := app.StartInventoryEditSession(idx)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if len(snap.InventoryItems) < 2 {
		t.Skip("need at least 2 inventory items for combined test")
	}
	// Find an upgradable weapon for the patch step.
	upgradeUID := ""
	var upgradeHandle, upgradeBase uint32
	for _, it := range snap.InventoryItems {
		if it.IsWeapon && it.CurrentUpgrade == 0 && it.MaxUpgrade >= 5 {
			upgradeUID = it.UID
			upgradeHandle = it.OriginalHandle
			upgradeBase = it.BaseItemID
			break
		}
	}
	if upgradeUID == "" {
		t.Skip("no eligible weapon for upgrade in combined test")
	}

	// 1. Reorder: move first item to position 1.
	firstUID := snap.InventoryItems[0].UID
	if _, err := app.MoveInventoryWorkspaceItem(snap.SessionID, firstUID, "inventory", 1); err != nil {
		t.Fatalf("Move: %v", err)
	}
	// 2. Add a Dagger at inventory[0].
	if _, err := app.AddInventoryWorkspaceItem(snap.SessionID,
		editor.AddItemSpec{ItemID: 0x000F4240}, "inventory", 0); err != nil {
		t.Fatalf("Add: %v", err)
	}
	// 3. Update existing weapon: upgrade +5.
	if _, err := app.UpdateInventoryWorkspaceWeapon(snap.SessionID, upgradeUID, editor.WeaponPatch{
		SetUpgrade: true, Upgrade: 5,
	}); err != nil {
		t.Fatalf("UpdateWeapon: %v", err)
	}

	updated, err := app.SaveInventoryWorkspaceChanges(snap.SessionID)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if updated.Dirty {
		t.Error("Dirty should be false after Save")
	}
	// Added Dagger present at inv[0] with a real handle.
	if updated.InventoryItems[0].Name != "Dagger" {
		t.Errorf("inv[0].Name = %q, want Dagger", updated.InventoryItems[0].Name)
	}
	if updated.InventoryItems[0].OriginalHandle == 0 {
		t.Error("added Dagger has zero handle after Save")
	}
	// firstUID handle stable.
	foundFirst := false
	for _, it := range updated.InventoryItems {
		if it.UID == firstUID {
			foundFirst = true
			break
		}
	}
	if !foundFirst {
		t.Errorf("firstUID %q lost from inventory after Save", firstUID)
	}
	// Upgraded weapon has new ItemID.
	foundUpgrade := false
	for _, it := range updated.InventoryItems {
		if it.OriginalHandle == upgradeHandle {
			if it.ItemID != upgradeBase+5 || it.CurrentUpgrade != 5 {
				t.Errorf("upgraded weapon: ItemID=0x%08X upgrade=%d, want 0x%08X +5",
					it.ItemID, it.CurrentUpgrade, upgradeBase+5)
			}
			foundUpgrade = true
			break
		}
	}
	if !foundUpgrade {
		t.Error("upgraded weapon not found post-Save")
	}
	assertNoDuplicateHandles(t, updated)
	assertNoDuplicateAcqIndices(t, updated)
}

// ─── Pass-through preservation ────────────────────────────────────

func TestSaveInventoryWorkspaceChanges_PreservesPassThrough(t *testing.T) {
	app, idx := realSaveAppForSave(t)
	snap, err := app.StartInventoryEditSession(idx)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if len(snap.UnsupportedInventoryRecords) == 0 && len(snap.UnsupportedStorageRecords) == 0 {
		t.Skip("no pass-through records in fixture to preserve")
	}
	invPassBefore := append([]editor.RawInventoryRecord(nil), snap.UnsupportedInventoryRecords...)
	stoPassBefore := append([]editor.RawInventoryRecord(nil), snap.UnsupportedStorageRecords...)

	// Trigger a no-op-ish edit: reorder by moving first item to its
	// own position. Even a same-position move flips Dirty, which is
	// enough to make Save actually run the rebuild path.
	if len(snap.InventoryItems) > 0 {
		uid := snap.InventoryItems[0].UID
		if _, err := app.MoveInventoryWorkspaceItem(snap.SessionID, uid, "inventory", 0); err != nil {
			t.Fatalf("Move: %v", err)
		}
	} else if len(snap.StorageItems) > 0 {
		uid := snap.StorageItems[0].UID
		if _, err := app.MoveInventoryWorkspaceItem(snap.SessionID, uid, "storage", 0); err != nil {
			t.Fatalf("Move: %v", err)
		}
	} else {
		t.Skip("no editable items to trigger a save")
	}

	updated, err := app.SaveInventoryWorkspaceChanges(snap.SessionID)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	// Every pre-Save pass-through handle must reappear post-Save.
	wantHandles := map[uint32]bool{}
	for _, r := range invPassBefore {
		wantHandles[r.Handle] = true
	}
	for _, r := range stoPassBefore {
		wantHandles[r.Handle] = true
	}
	gotHandles := map[uint32]bool{}
	for _, r := range updated.UnsupportedInventoryRecords {
		gotHandles[r.Handle] = true
	}
	for _, r := range updated.UnsupportedStorageRecords {
		gotHandles[r.Handle] = true
	}
	for h := range wantHandles {
		if !gotHandles[h] {
			t.Errorf("pass-through handle 0x%08X missing post-Save", h)
		}
	}
}
