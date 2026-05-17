package main

import (
	"bytes"
	"os"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
	"github.com/oisis/EldenRing-SaveForge/backend/db"
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

// Phase 4B: Save now SETS pending AoW. The legacy "rejects pending AoW"
// test was inverted to "rejects non-AoW item as AoW" — Save still
// refuses an obviously bogus pending state (e.g. AoWItemID set to a
// weapon ID by direct field mutation that bypassed UpdateWeapon).
func TestSaveInventoryWorkspaceChanges_RejectsNonAoWItemAsAoW(t *testing.T) {
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
	// Direct workspace mutation (bypassing UpdateWeapon's validation)
	// to plant a non-AoW item ID into PendingAoWItemID.
	sess := app.editSessions[snap.SessionID]
	for i := range sess.Workspace.InventoryItems {
		if sess.Workspace.InventoryItems[i].UID == weaponUID {
			sess.Workspace.InventoryItems[i].PendingAoWItemID = 0x003085E0 // Claymore — weapon, not AoW
			break
		}
	}

	slot := &app.save.Slots[idx]
	before := snapshotSlotBytes(slot)
	_, err = app.SaveInventoryWorkspaceChanges(snap.SessionID)
	if err == nil {
		t.Fatal("expected rejection for non-AoW pending item")
	}
	if !bytes.Equal(slot.Data, before) {
		t.Error("slot.Data mutated after non-AoW rejection")
	}
}

// Save with a known-incompatible AoW (different weapon family) is
// rejected fail-closed by validatePendingAoWChanges before any
// mutation reaches slot.Data.
func TestSaveInventoryWorkspaceChanges_RejectsIncompatibleAoW(t *testing.T) {
	app, idx := realSaveAppForSave(t)
	snap, err := app.StartInventoryEditSession(idx)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	// Find any weapon the DB says is incompatible with Lion's Claw
	// (known=true, compatible=false). Daggers, bows, fists etc. all
	// qualify — search broadly.
	const lionsClaw = uint32(0x80002710)
	weaponUID := ""
	for _, it := range snap.InventoryItems {
		if !it.IsWeapon {
			continue
		}
		compat, known := db.IsAshOfWarCompatibleWithWeapon(lionsClaw, it.ItemID)
		if known && !compat {
			weaponUID = it.UID
			break
		}
	}
	if weaponUID == "" {
		t.Skip("no known-incompatible weapon in fixture to test against Lion's Claw")
	}
	// Plant directly to bypass UpdateWeapon's accept check (which is
	// identity-based, not compat-based).
	sess := app.editSessions[snap.SessionID]
	for i := range sess.Workspace.InventoryItems {
		if sess.Workspace.InventoryItems[i].UID == weaponUID {
			sess.Workspace.InventoryItems[i].PendingAoWItemID = 0x80002710
			break
		}
	}
	slot := &app.save.Slots[idx]
	before := snapshotSlotBytes(slot)
	_, err = app.SaveInventoryWorkspaceChanges(snap.SessionID)
	if err == nil {
		t.Fatal("expected incompatible-AoW rejection")
	}
	if !bytes.Equal(slot.Data, before) {
		t.Error("slot.Data mutated after incompatibility rejection")
	}
}

// ─── Phase 3B: transfer success paths ─────────────────────────────

func TestSaveInventoryWorkspaceChanges_TransferInventoryToStorage(t *testing.T) {
	app, idx := realSaveAppForSave(t)
	snap, err := app.StartInventoryEditSession(idx)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	target := ""
	var targetHandle uint32
	for _, it := range snap.InventoryItems {
		if it.IsWeapon {
			target = it.UID
			targetHandle = it.OriginalHandle
			break
		}
	}
	if target == "" {
		t.Skip("no weapon in inventory to transfer")
	}
	if _, err := app.MoveInventoryWorkspaceItem(snap.SessionID, target, "storage", 0); err != nil {
		t.Fatalf("Move: %v", err)
	}

	updated, err := app.SaveInventoryWorkspaceChanges(snap.SessionID)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if updated.Dirty {
		t.Error("Dirty should be false after Save")
	}
	// Handle absent from inventory.
	for _, it := range updated.InventoryItems {
		if it.OriginalHandle == targetHandle {
			t.Errorf("handle 0x%08X still in inventory after transfer", targetHandle)
		}
	}
	// Handle present in storage with same value.
	foundInSto := false
	for _, it := range updated.StorageItems {
		if it.OriginalHandle == targetHandle {
			foundInSto = true
			break
		}
	}
	if !foundInSto {
		t.Errorf("handle 0x%08X missing from storage after transfer", targetHandle)
	}
	assertNoDuplicateHandles(t, updated)
	assertNoDuplicateAcqIndices(t, updated)
}

func TestSaveInventoryWorkspaceChanges_TransferStorageToInventory(t *testing.T) {
	app, idx := realSaveAppForSave(t)
	snap, err := app.StartInventoryEditSession(idx)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if len(snap.StorageItems) == 0 {
		t.Skip("no storage items to transfer")
	}
	target := snap.StorageItems[0].UID
	targetHandle := snap.StorageItems[0].OriginalHandle
	if _, err := app.MoveInventoryWorkspaceItem(snap.SessionID, target, "inventory", 0); err != nil {
		t.Fatalf("Move: %v", err)
	}

	updated, err := app.SaveInventoryWorkspaceChanges(snap.SessionID)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if updated.Dirty {
		t.Error("Dirty should be false after Save")
	}
	for _, it := range updated.StorageItems {
		if it.OriginalHandle == targetHandle {
			t.Errorf("handle 0x%08X still in storage after transfer", targetHandle)
		}
	}
	foundInInv := false
	for _, it := range updated.InventoryItems {
		if it.OriginalHandle == targetHandle {
			foundInInv = true
			break
		}
	}
	if !foundInInv {
		t.Errorf("handle 0x%08X missing from inventory after transfer", targetHandle)
	}
	assertNoDuplicateHandles(t, updated)
	assertNoDuplicateAcqIndices(t, updated)
}

// ─── Phase 3B: remove success paths ───────────────────────────────

func TestSaveInventoryWorkspaceChanges_RemoveFromInventory(t *testing.T) {
	app, idx := realSaveAppForSave(t)
	snap, err := app.StartInventoryEditSession(idx)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	target := ""
	var targetHandle uint32
	for _, it := range snap.InventoryItems {
		if it.IsWeapon {
			target = it.UID
			targetHandle = it.OriginalHandle
			break
		}
	}
	if target == "" {
		t.Skip("no weapon in inventory to remove")
	}
	if _, err := app.RemoveInventoryWorkspaceItem(snap.SessionID, target); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	updated, err := app.SaveInventoryWorkspaceChanges(snap.SessionID)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if updated.Dirty {
		t.Error("Dirty should be false after Save")
	}
	for _, it := range updated.InventoryItems {
		if it.OriginalHandle == targetHandle {
			t.Errorf("handle 0x%08X still in inventory after remove", targetHandle)
		}
	}
	for _, it := range updated.StorageItems {
		if it.OriginalHandle == targetHandle {
			t.Errorf("handle 0x%08X leaked into storage after remove", targetHandle)
		}
	}
	assertNoDuplicateHandles(t, updated)
	assertNoDuplicateAcqIndices(t, updated)
}

func TestSaveInventoryWorkspaceChanges_RemoveFromStorage(t *testing.T) {
	app, idx := realSaveAppForSave(t)
	snap, err := app.StartInventoryEditSession(idx)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if len(snap.StorageItems) == 0 {
		t.Skip("no storage items to remove")
	}
	target := snap.StorageItems[0].UID
	targetHandle := snap.StorageItems[0].OriginalHandle

	if _, err := app.RemoveInventoryWorkspaceItem(snap.SessionID, target); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	updated, err := app.SaveInventoryWorkspaceChanges(snap.SessionID)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	for _, it := range updated.StorageItems {
		if it.OriginalHandle == targetHandle {
			t.Errorf("handle 0x%08X still in storage after remove", targetHandle)
		}
	}
	for _, it := range updated.InventoryItems {
		if it.OriginalHandle == targetHandle {
			t.Errorf("handle 0x%08X leaked into inventory after remove", targetHandle)
		}
	}
	assertNoDuplicateHandles(t, updated)
	assertNoDuplicateAcqIndices(t, updated)
}

// ─── Phase 3B: transfer + sibling edits ──────────────────────────

func TestSaveInventoryWorkspaceChanges_TransferAndReorderTarget(t *testing.T) {
	app, idx := realSaveAppForSave(t)
	snap, err := app.StartInventoryEditSession(idx)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if len(snap.StorageItems) < 1 {
		t.Skip("need at least 1 storage item for reorder")
	}
	// Pick a weapon to transfer from inventory.
	target := ""
	var targetHandle uint32
	for _, it := range snap.InventoryItems {
		if it.IsWeapon {
			target = it.UID
			targetHandle = it.OriginalHandle
			break
		}
	}
	if target == "" {
		t.Skip("no weapon in inventory")
	}
	// Transfer to storage at the END.
	if _, err := app.TransferInventoryWorkspaceItem(snap.SessionID, target, "storage"); err != nil {
		t.Fatalf("Transfer: %v", err)
	}
	// Reorder: pick the (now first) existing storage item and move to position 0.
	// After append-transfer the original first-storage item is still
	// at position 0, so move it to position 1 to swap with the transferred one.
	mid, err := app.GetInventoryEditSession(snap.SessionID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if len(mid.StorageItems) < 2 {
		t.Skip("need at least 2 storage items post-transfer to reorder")
	}
	firstStorageUID := mid.StorageItems[0].UID
	if _, err := app.MoveInventoryWorkspaceItem(snap.SessionID, firstStorageUID, "storage", 1); err != nil {
		t.Fatalf("Move: %v", err)
	}

	updated, err := app.SaveInventoryWorkspaceChanges(snap.SessionID)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	// Transferred handle must be present in storage.
	foundInSto := false
	for _, it := range updated.StorageItems {
		if it.OriginalHandle == targetHandle {
			foundInSto = true
			break
		}
	}
	if !foundInSto {
		t.Errorf("transferred handle 0x%08X missing from storage after transfer+reorder", targetHandle)
	}
	assertNoDuplicateHandles(t, updated)
	assertNoDuplicateAcqIndices(t, updated)
}

func TestSaveInventoryWorkspaceChanges_TransferAndAdd(t *testing.T) {
	app, idx := realSaveAppForSave(t)
	snap, err := app.StartInventoryEditSession(idx)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	target := ""
	var targetHandle uint32
	for _, it := range snap.InventoryItems {
		if it.IsWeapon {
			target = it.UID
			targetHandle = it.OriginalHandle
			break
		}
	}
	if target == "" {
		t.Skip("no weapon to transfer")
	}
	// Transfer to storage.
	if _, err := app.TransferInventoryWorkspaceItem(snap.SessionID, target, "storage"); err != nil {
		t.Fatalf("Transfer: %v", err)
	}
	// Add a new Dagger to inventory.
	if _, err := app.AddInventoryWorkspaceItem(snap.SessionID,
		editor.AddItemSpec{ItemID: 0x000F4240}, "inventory", 0); err != nil {
		t.Fatalf("Add: %v", err)
	}

	updated, err := app.SaveInventoryWorkspaceChanges(snap.SessionID)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	// Transferred handle in storage.
	foundInSto := false
	for _, it := range updated.StorageItems {
		if it.OriginalHandle == targetHandle {
			foundInSto = true
			break
		}
	}
	if !foundInSto {
		t.Errorf("transferred handle 0x%08X missing from storage", targetHandle)
	}
	// Added Dagger has a real handle and is in inventory.
	foundDagger := false
	for _, it := range updated.InventoryItems {
		if it.ItemID == 0x000F4240 && it.OriginalHandle != 0 {
			foundDagger = true
			break
		}
	}
	if !foundDagger {
		t.Error("added Dagger missing from inventory after transfer+add")
	}
	assertNoDuplicateHandles(t, updated)
	assertNoDuplicateAcqIndices(t, updated)
}

func TestSaveInventoryWorkspaceChanges_TransferAndWeaponUpgrade(t *testing.T) {
	app, idx := realSaveAppForSave(t)
	snap, err := app.StartInventoryEditSession(idx)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	target := ""
	var targetHandle, targetBase uint32
	for _, it := range snap.InventoryItems {
		if it.IsWeapon && it.CurrentUpgrade == 0 && it.MaxUpgrade >= 3 {
			target = it.UID
			targetHandle = it.OriginalHandle
			targetBase = it.BaseItemID
			break
		}
	}
	if target == "" {
		t.Skip("no upgradable weapon to transfer")
	}
	// Transfer to storage AND upgrade +3 in same session.
	if _, err := app.TransferInventoryWorkspaceItem(snap.SessionID, target, "storage"); err != nil {
		t.Fatalf("Transfer: %v", err)
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
	// Find the transferred+patched weapon in storage.
	var found *editor.EditableItem
	for i, it := range updated.StorageItems {
		if it.OriginalHandle == targetHandle {
			found = &updated.StorageItems[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("transferred handle 0x%08X missing from storage after transfer+upgrade", targetHandle)
	}
	if found.ItemID != targetBase+3 {
		t.Errorf("ItemID = 0x%08X, want 0x%08X (+3)", found.ItemID, targetBase+3)
	}
	if found.CurrentUpgrade != 3 {
		t.Errorf("CurrentUpgrade = %d, want 3", found.CurrentUpgrade)
	}
	assertNoDuplicateHandles(t, updated)
}

// ─── Phase 3B: remove + add ──────────────────────────────────────

func TestSaveInventoryWorkspaceChanges_RemoveAndAdd(t *testing.T) {
	app, idx := realSaveAppForSave(t)
	snap, err := app.StartInventoryEditSession(idx)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	target := ""
	var targetHandle uint32
	for _, it := range snap.InventoryItems {
		if it.IsWeapon {
			target = it.UID
			targetHandle = it.OriginalHandle
			break
		}
	}
	if target == "" {
		t.Skip("no weapon to remove")
	}
	if _, err := app.RemoveInventoryWorkspaceItem(snap.SessionID, target); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if _, err := app.AddInventoryWorkspaceItem(snap.SessionID,
		editor.AddItemSpec{ItemID: 0x000F4240}, "inventory", 0); err != nil {
		t.Fatalf("Add: %v", err)
	}

	updated, err := app.SaveInventoryWorkspaceChanges(snap.SessionID)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	// Removed handle absent.
	for _, it := range updated.InventoryItems {
		if it.OriginalHandle == targetHandle {
			t.Errorf("removed handle 0x%08X still in inventory", targetHandle)
		}
	}
	// Added Dagger present.
	foundDagger := false
	for _, it := range updated.InventoryItems {
		if it.ItemID == 0x000F4240 && it.OriginalHandle != 0 {
			foundDagger = true
			break
		}
	}
	if !foundDagger {
		t.Error("added Dagger missing from inventory after remove+add")
	}
	assertNoDuplicateHandles(t, updated)
	assertNoDuplicateAcqIndices(t, updated)
}

// ─── Phase 3B: full combined workflow ────────────────────────────

func TestSaveInventoryWorkspaceChanges_FullCombinedWorkflow(t *testing.T) {
	app, idx := realSaveAppForSave(t)
	snap, err := app.StartInventoryEditSession(idx)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if len(snap.InventoryItems) < 3 || len(snap.StorageItems) < 2 {
		t.Skip("need >=3 inv + >=2 storage items for combined workflow")
	}
	// Find an upgradable weapon that is NOT the one we'll transfer or remove.
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
		t.Skip("no eligible upgrade weapon in combined workflow")
	}
	// Pick a different item to transfer (any non-upgrade weapon, or
	// any item if needed). And another for remove.
	transferUID, removeUID := "", ""
	var transferHandle, removeHandle uint32
	for _, it := range snap.InventoryItems {
		if it.UID == upgradeUID {
			continue
		}
		if transferUID == "" {
			transferUID = it.UID
			transferHandle = it.OriginalHandle
			continue
		}
		if removeUID == "" {
			removeUID = it.UID
			removeHandle = it.OriginalHandle
			break
		}
	}
	if transferUID == "" || removeUID == "" {
		t.Skip("not enough distinct inventory items for combined workflow")
	}

	// 1. Reorder inventory: swap first two.
	firstInvUID := snap.InventoryItems[0].UID
	if firstInvUID != upgradeUID && firstInvUID != transferUID && firstInvUID != removeUID {
		if _, err := app.MoveInventoryWorkspaceItem(snap.SessionID, firstInvUID, "inventory", 1); err != nil {
			t.Fatalf("Move inv: %v", err)
		}
	}
	// 2. Reorder storage: swap first two.
	firstStoUID := snap.StorageItems[0].UID
	if _, err := app.MoveInventoryWorkspaceItem(snap.SessionID, firstStoUID, "storage", 1); err != nil {
		t.Fatalf("Move sto: %v", err)
	}
	// 3. Transfer one item inventory → storage.
	if _, err := app.TransferInventoryWorkspaceItem(snap.SessionID, transferUID, "storage"); err != nil {
		t.Fatalf("Transfer: %v", err)
	}
	// 4. Add a Dagger to inventory.
	if _, err := app.AddInventoryWorkspaceItem(snap.SessionID,
		editor.AddItemSpec{ItemID: 0x000F4240}, "inventory", 0); err != nil {
		t.Fatalf("Add: %v", err)
	}
	// 5. Upgrade existing weapon +5.
	if _, err := app.UpdateInventoryWorkspaceWeapon(snap.SessionID, upgradeUID, editor.WeaponPatch{
		SetUpgrade: true, Upgrade: 5,
	}); err != nil {
		t.Fatalf("UpdateWeapon: %v", err)
	}
	// 6. Remove an item.
	if _, err := app.RemoveInventoryWorkspaceItem(snap.SessionID, removeUID); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	updated, err := app.SaveInventoryWorkspaceChanges(snap.SessionID)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if updated.Dirty {
		t.Error("Dirty should be false after Save")
	}
	// Verify final state.
	// Transferred handle in storage, not inventory.
	for _, it := range updated.InventoryItems {
		if it.OriginalHandle == transferHandle {
			t.Errorf("transferred handle 0x%08X still in inventory", transferHandle)
		}
	}
	foundTransfer := false
	for _, it := range updated.StorageItems {
		if it.OriginalHandle == transferHandle {
			foundTransfer = true
			break
		}
	}
	if !foundTransfer {
		t.Errorf("transferred handle 0x%08X missing from storage", transferHandle)
	}
	// Added Dagger present in inventory with real handle.
	foundDagger := false
	for _, it := range updated.InventoryItems {
		if it.ItemID == 0x000F4240 && it.OriginalHandle != 0 {
			foundDagger = true
			break
		}
	}
	if !foundDagger {
		t.Error("added Dagger missing from inventory after combined save")
	}
	// Upgraded weapon updated.
	foundUpgrade := false
	for _, it := range updated.InventoryItems {
		if it.OriginalHandle == upgradeHandle {
			if it.ItemID != upgradeBase+5 || it.CurrentUpgrade != 5 {
				t.Errorf("upgraded weapon ItemID=0x%08X upgrade=%d, want 0x%08X +5",
					it.ItemID, it.CurrentUpgrade, upgradeBase+5)
			}
			foundUpgrade = true
			break
		}
	}
	if !foundUpgrade {
		t.Error("upgraded weapon missing from inventory after combined save")
	}
	// Removed handle absent everywhere.
	for _, it := range updated.InventoryItems {
		if it.OriginalHandle == removeHandle {
			t.Errorf("removed handle 0x%08X still in inventory", removeHandle)
		}
	}
	for _, it := range updated.StorageItems {
		if it.OriginalHandle == removeHandle {
			t.Errorf("removed handle 0x%08X leaked into storage", removeHandle)
		}
	}
	assertNoDuplicateHandles(t, updated)
	assertNoDuplicateAcqIndices(t, updated)
}

// ─── Phase 4A: CurrentAoW snapshot field readback ─────────────────

// After a no-AoW save (reorder only), the reparsed snapshot must still
// report the same CurrentAoW* state for any weapon that had a custom
// AoW pre-save. Asserts the read-side AoW pipeline survives the full
// rebuild path.
func TestSaveInventoryWorkspaceChanges_PreservesCurrentAoWAfterReparse(t *testing.T) {
	app, idx := realSaveAppForSave(t)
	snap, err := app.StartInventoryEditSession(idx)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	// Find any weapon with CurrentAoWStatus set — if the fixture has
	// no custom AoW, fall back to checking AoWStatusNone preservation.
	preWeapons := map[uint32]string{}
	for _, it := range snap.InventoryItems {
		if it.IsWeapon && it.CurrentAoWStatus != "" {
			preWeapons[it.OriginalHandle] = it.CurrentAoWStatus
		}
	}
	for _, it := range snap.StorageItems {
		if it.IsWeapon && it.CurrentAoWStatus != "" {
			preWeapons[it.OriginalHandle] = it.CurrentAoWStatus
		}
	}
	if len(preWeapons) == 0 {
		t.Skip("fixture has no weapons with AoW status to verify")
	}
	// Trigger a reorder so Save actually runs (no AoW change).
	if len(snap.InventoryItems) > 0 {
		first := snap.InventoryItems[0].UID
		if _, err := app.MoveInventoryWorkspaceItem(snap.SessionID, first, "inventory", 0); err != nil {
			t.Fatalf("Move: %v", err)
		}
	}
	updated, err := app.SaveInventoryWorkspaceChanges(snap.SessionID)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	// Every pre-save (handle → status) must reappear post-save.
	check := func(items []editor.EditableItem) {
		for _, it := range items {
			pre, tracked := preWeapons[it.OriginalHandle]
			if !tracked {
				continue
			}
			if it.CurrentAoWStatus != pre {
				t.Errorf("weapon 0x%08X status: pre=%q post=%q (handle stable across save)",
					it.OriginalHandle, pre, it.CurrentAoWStatus)
			}
		}
	}
	check(updated.InventoryItems)
	check(updated.StorageItems)
}

// ─── Phase 4B: pending AoW save success paths ────────────────────

// Save pending AoW on an existing weapon. Picks the weapon whose
// CurrentAoWStatus already permits a known-compatible AoW so the
// IsAshOfWarCompatibleWithWeapon gate accepts.
func TestSaveInventoryWorkspaceChanges_SavesPendingAoWOnExistingWeapon(t *testing.T) {
	app, idx := realSaveAppForSave(t)
	snap, err := app.StartInventoryEditSession(idx)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	// Use Lion's Claw (greatsword-class). Pick the first Claymore (or
	// any GemMountType=2 weapon known to take Lion's Claw). Easier: any
	// weapon already showing CurrentAoWStatus="custom" is by definition
	// AoW-compatible at the weapon level — we can swap in Lion's Claw
	// only if the IsAshOfWarCompatibleWithWeapon helper accepts. Iterate
	// candidates until one passes the compat gate.
	const aowID = uint32(0x80002710) // Lion's Claw
	weaponUID, weaponHandle := "", uint32(0)
	for _, it := range snap.InventoryItems {
		if !it.IsWeapon || it.MaxUpgrade < 25 {
			continue
		}
		compat, known := dbCompatHelper(it.ItemID, aowID)
		if known && compat {
			weaponUID = it.UID
			weaponHandle = it.OriginalHandle
			break
		}
	}
	if weaponUID == "" {
		t.Skip("no Lion's Claw-compatible weapon in inventory")
	}
	if _, err := app.UpdateInventoryWorkspaceWeapon(snap.SessionID, weaponUID, editor.WeaponPatch{
		SetAoWItemID: true, AoWItemID: aowID,
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
	// Find weapon post-save: its handle is stable across reorder/save.
	var post *editor.EditableItem
	for i := range updated.InventoryItems {
		if updated.InventoryItems[i].OriginalHandle == weaponHandle {
			post = &updated.InventoryItems[i]
			break
		}
	}
	if post == nil {
		t.Fatalf("weapon 0x%08X missing after save", weaponHandle)
	}
	if post.CurrentAoWItemID != aowID {
		t.Errorf("CurrentAoWItemID = 0x%08X, want 0x%08X", post.CurrentAoWItemID, aowID)
	}
	if post.CurrentAoWStatus != editor.AoWStatusCustom {
		t.Errorf("CurrentAoWStatus = %q, want %q", post.CurrentAoWStatus, editor.AoWStatusCustom)
	}
	if post.CurrentAoWShared {
		t.Error("CurrentAoWShared should be false for a freshly minted AoW handle")
	}
	if post.PendingAoWItemID != 0 || post.PendingAoWName != "" || post.PendingAoWClear {
		t.Errorf("Pending* fields should clear after save, got %+v", post)
	}
}

// Save pending AoW on a newly added weapon.
func TestSaveInventoryWorkspaceChanges_SavesPendingAoWOnAddedWeapon(t *testing.T) {
	app, idx := realSaveAppForSave(t)
	snap, err := app.StartInventoryEditSession(idx)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	// Add a Claymore (greatsword, Lion's Claw-compatible) and queue
	// Lion's Claw as its pending AoW.
	const (
		claymoreID = uint32(0x003085E0)
		aowID      = uint32(0x80002710)
	)
	if _, err := app.AddInventoryWorkspaceItem(snap.SessionID,
		editor.AddItemSpec{ItemID: claymoreID}, "inventory", 0); err != nil {
		t.Fatalf("Add: %v", err)
	}
	post1, err := app.GetInventoryEditSession(snap.SessionID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if len(post1.InventoryItems) == 0 || post1.InventoryItems[0].ItemID != claymoreID {
		t.Fatalf("added Claymore not at inv[0]: %+v", post1.InventoryItems)
	}
	addedUID := post1.InventoryItems[0].UID
	if _, err := app.UpdateInventoryWorkspaceWeapon(snap.SessionID, addedUID, editor.WeaponPatch{
		SetAoWItemID: true, AoWItemID: aowID,
	}); err != nil {
		t.Fatalf("UpdateWeapon: %v", err)
	}
	updated, err := app.SaveInventoryWorkspaceChanges(snap.SessionID)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	// The added Claymore should now be original (reparsed from slot).
	var added *editor.EditableItem
	for i := range updated.InventoryItems {
		if updated.InventoryItems[i].ItemID == claymoreID &&
			updated.InventoryItems[i].CurrentAoWItemID == aowID {
			added = &updated.InventoryItems[i]
			break
		}
	}
	if added == nil {
		t.Fatalf("added Claymore with Lion's Claw not found post-save")
	}
	if added.OriginalHandle == 0 {
		t.Error("added weapon must have a real handle after save")
	}
	if added.CurrentAoWStatus != editor.AoWStatusCustom {
		t.Errorf("CurrentAoWStatus = %q, want custom", added.CurrentAoWStatus)
	}
}

// Save pending AoW + transfer: weapon moves to other container and
// receives Lion's Claw in same session.
func TestSaveInventoryWorkspaceChanges_SavesPendingAoWPlusTransfer(t *testing.T) {
	app, idx := realSaveAppForSave(t)
	snap, err := app.StartInventoryEditSession(idx)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	const aowID = uint32(0x80002710)
	weaponUID, weaponHandle := "", uint32(0)
	for _, it := range snap.InventoryItems {
		if !it.IsWeapon || it.MaxUpgrade < 25 {
			continue
		}
		if compat, known := dbCompatHelper(it.ItemID, aowID); known && compat {
			weaponUID = it.UID
			weaponHandle = it.OriginalHandle
			break
		}
	}
	if weaponUID == "" {
		t.Skip("no Lion's Claw-compatible weapon for transfer+AoW")
	}
	if _, err := app.TransferInventoryWorkspaceItem(snap.SessionID, weaponUID, "storage"); err != nil {
		t.Fatalf("Transfer: %v", err)
	}
	if _, err := app.UpdateInventoryWorkspaceWeapon(snap.SessionID, weaponUID, editor.WeaponPatch{
		SetAoWItemID: true, AoWItemID: aowID,
	}); err != nil {
		t.Fatalf("UpdateWeapon: %v", err)
	}
	updated, err := app.SaveInventoryWorkspaceChanges(snap.SessionID)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	var post *editor.EditableItem
	for i := range updated.StorageItems {
		if updated.StorageItems[i].OriginalHandle == weaponHandle {
			post = &updated.StorageItems[i]
			break
		}
	}
	if post == nil {
		t.Fatalf("weapon 0x%08X missing from storage post-save", weaponHandle)
	}
	if post.CurrentAoWItemID != aowID {
		t.Errorf("CurrentAoWItemID = 0x%08X, want 0x%08X", post.CurrentAoWItemID, aowID)
	}
}

// Save pending AoW + weapon upgrade: both ItemID and AoW persist.
func TestSaveInventoryWorkspaceChanges_SavesPendingAoWPlusUpgrade(t *testing.T) {
	app, idx := realSaveAppForSave(t)
	snap, err := app.StartInventoryEditSession(idx)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	const aowID = uint32(0x80002710)
	// Find any Lion's Claw-compatible weapon with at least 1 upgrade
	// step of headroom. We'll patch to (CurrentUpgrade+1) to keep the
	// math simple regardless of starting point.
	weaponUID, weaponHandle, weaponBase, startUp := "", uint32(0), uint32(0), 0
	for _, it := range snap.InventoryItems {
		if !it.IsWeapon || it.MaxUpgrade < it.CurrentUpgrade+1 {
			continue
		}
		if compat, known := dbCompatHelper(it.ItemID, aowID); known && compat {
			weaponUID = it.UID
			weaponHandle = it.OriginalHandle
			weaponBase = it.BaseItemID
			startUp = it.CurrentUpgrade
			break
		}
	}
	if weaponUID == "" {
		t.Skip("no eligible upgrade+AoW weapon")
	}
	targetUp := startUp + 1
	if _, err := app.UpdateInventoryWorkspaceWeapon(snap.SessionID, weaponUID, editor.WeaponPatch{
		SetUpgrade: true, Upgrade: targetUp,
	}); err != nil {
		t.Fatalf("UpdateWeapon upgrade: %v", err)
	}
	if _, err := app.UpdateInventoryWorkspaceWeapon(snap.SessionID, weaponUID, editor.WeaponPatch{
		SetAoWItemID: true, AoWItemID: aowID,
	}); err != nil {
		t.Fatalf("UpdateWeapon AoW: %v", err)
	}
	updated, err := app.SaveInventoryWorkspaceChanges(snap.SessionID)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	var post *editor.EditableItem
	for i := range updated.InventoryItems {
		if updated.InventoryItems[i].OriginalHandle == weaponHandle {
			post = &updated.InventoryItems[i]
			break
		}
	}
	if post == nil {
		t.Fatalf("weapon missing post-save")
	}
	// Note: ItemID re-encoding uses BaseItemID + infusionOffset + level.
	// We only check that CurrentUpgrade reached the target; the exact
	// ItemID may carry a non-zero infusion offset if the source weapon
	// is already infused, which is independent of this test's goal.
	if post.CurrentUpgrade != targetUp {
		t.Errorf("CurrentUpgrade = %d, want %d (base 0x%08X)", post.CurrentUpgrade, targetUp, weaponBase)
	}
	if post.CurrentAoWItemID != aowID {
		t.Errorf("CurrentAoWItemID = 0x%08X, want 0x%08X", post.CurrentAoWItemID, aowID)
	}
}

// Save the same AoW item ID on two different weapons in one session.
// PatchWeaponAoW must mint distinct handles so CurrentAoWShared stays
// false for both.
func TestSaveInventoryWorkspaceChanges_TwoWeaponsSameAoWItem_DistinctHandles(t *testing.T) {
	app, idx := realSaveAppForSave(t)
	snap, err := app.StartInventoryEditSession(idx)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	const aowID = uint32(0x80002710)
	var picks []editor.EditableItem
	for _, it := range snap.InventoryItems {
		if !it.IsWeapon || it.MaxUpgrade < 25 {
			continue
		}
		if compat, known := dbCompatHelper(it.ItemID, aowID); known && compat {
			picks = append(picks, it)
			if len(picks) == 2 {
				break
			}
		}
	}
	if len(picks) < 2 {
		t.Skip("need at least 2 Lion's Claw-compatible weapons for distinct-handle test")
	}
	for _, w := range picks {
		if _, err := app.UpdateInventoryWorkspaceWeapon(snap.SessionID, w.UID, editor.WeaponPatch{
			SetAoWItemID: true, AoWItemID: aowID,
		}); err != nil {
			t.Fatalf("UpdateWeapon %s: %v", w.UID, err)
		}
	}
	updated, err := app.SaveInventoryWorkspaceChanges(snap.SessionID)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	handles := map[uint32]bool{}
	for _, w := range picks {
		for _, it := range updated.InventoryItems {
			if it.OriginalHandle != w.OriginalHandle {
				continue
			}
			if it.CurrentAoWItemID != aowID {
				t.Errorf("weapon 0x%08X CurrentAoWItemID = 0x%08X, want 0x%08X",
					w.OriginalHandle, it.CurrentAoWItemID, aowID)
			}
			if it.CurrentAoWShared {
				t.Errorf("weapon 0x%08X CurrentAoWShared should be false (distinct handles)",
					w.OriginalHandle)
			}
			if it.CurrentAoWHandle == 0 {
				t.Errorf("weapon 0x%08X has zero CurrentAoWHandle post-save", w.OriginalHandle)
			}
			if handles[it.CurrentAoWHandle] {
				t.Errorf("duplicate CurrentAoWHandle 0x%08X across saved weapons",
					it.CurrentAoWHandle)
			}
			handles[it.CurrentAoWHandle] = true
			break
		}
	}
}

// Save pending AoW clear: weapon's CurrentAoWStatus drops to "none".
func TestSaveInventoryWorkspaceChanges_SavesPendingAoWClear(t *testing.T) {
	app, idx := realSaveAppForSave(t)
	snap, err := app.StartInventoryEditSession(idx)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	// Find any weapon with a custom AoW currently attached.
	weaponUID, weaponHandle := "", uint32(0)
	for _, it := range snap.InventoryItems {
		if it.IsWeapon && it.CurrentAoWStatus == editor.AoWStatusCustom {
			weaponUID = it.UID
			weaponHandle = it.OriginalHandle
			break
		}
	}
	for _, it := range snap.StorageItems {
		if weaponUID != "" {
			break
		}
		if it.IsWeapon && it.CurrentAoWStatus == editor.AoWStatusCustom {
			weaponUID = it.UID
			weaponHandle = it.OriginalHandle
			break
		}
	}
	if weaponUID == "" {
		t.Skip("no weapon with custom AoW in fixture to clear")
	}
	if _, err := app.UpdateInventoryWorkspaceWeapon(snap.SessionID, weaponUID, editor.WeaponPatch{
		ClearAoW: true,
	}); err != nil {
		t.Fatalf("UpdateWeapon ClearAoW: %v", err)
	}
	updated, err := app.SaveInventoryWorkspaceChanges(snap.SessionID)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	check := func(items []editor.EditableItem) bool {
		for _, it := range items {
			if it.OriginalHandle == weaponHandle {
				if it.CurrentAoWStatus != editor.AoWStatusNone {
					t.Errorf("after clear, CurrentAoWStatus = %q, want %q",
						it.CurrentAoWStatus, editor.AoWStatusNone)
				}
				if it.HasCurrentAoW {
					t.Error("HasCurrentAoW should be false after clear")
				}
				return true
			}
		}
		return false
	}
	if !check(updated.InventoryItems) && !check(updated.StorageItems) {
		t.Errorf("weapon 0x%08X missing post-save", weaponHandle)
	}
}

// dbCompatHelper wraps db.IsAshOfWarCompatibleWithWeapon so tests can
// call it without importing db at the test file's package level. Keeps
// the import surface narrow.
func dbCompatHelper(weaponItemID, aowItemID uint32) (bool, bool) {
	return db.IsAshOfWarCompatibleWithWeapon(aowItemID, weaponItemID)
}

// ─── Phase 3B: baseline regeneration ──────────────────────────────

// After a successful save, the session's baseline must be refreshed so
// a subsequent save against the same session doesn't see the previous
// transfer / remove as if it were a fresh out-of-scope edit.
func TestSaveInventoryWorkspaceChanges_BaselineRegeneratedAfterSave(t *testing.T) {
	app, idx := realSaveAppForSave(t)
	snap, err := app.StartInventoryEditSession(idx)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	target := ""
	var targetHandle uint32
	for _, it := range snap.InventoryItems {
		if it.IsWeapon {
			target = it.UID
			targetHandle = it.OriginalHandle
			break
		}
	}
	if target == "" {
		t.Skip("no weapon to transfer")
	}
	// Save 1: transfer inventory → storage.
	if _, err := app.TransferInventoryWorkspaceItem(snap.SessionID, target, "storage"); err != nil {
		t.Fatalf("Transfer: %v", err)
	}
	if _, err := app.SaveInventoryWorkspaceChanges(snap.SessionID); err != nil {
		t.Fatalf("Save #1: %v", err)
	}
	// Save 2: reorder only (no further transfer / remove). Should
	// succeed without baseline-detection confusion. We pick the now-
	// in-storage transferred item and move it to the end of storage.
	mid, err := app.GetInventoryEditSession(snap.SessionID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	transferredUID := ""
	for _, it := range mid.StorageItems {
		if it.OriginalHandle == targetHandle {
			transferredUID = it.UID
			break
		}
	}
	if transferredUID == "" {
		t.Fatalf("transferred item lost between saves")
	}
	if len(mid.StorageItems) < 2 {
		t.Skip("not enough storage items for second-reorder check")
	}
	if _, err := app.MoveInventoryWorkspaceItem(snap.SessionID, transferredUID, "storage", len(mid.StorageItems)-1); err != nil {
		t.Fatalf("Move: %v", err)
	}
	updated, err := app.SaveInventoryWorkspaceChanges(snap.SessionID)
	if err != nil {
		t.Fatalf("Save #2: %v — baseline likely stale", err)
	}
	if updated.Dirty {
		t.Error("Dirty should be false after Save #2")
	}
	// Still in storage with same handle.
	stillInSto := false
	for _, it := range updated.StorageItems {
		if it.OriginalHandle == targetHandle {
			stillInSto = true
			break
		}
	}
	if !stillInSto {
		t.Errorf("handle 0x%08X missing after second save", targetHandle)
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
