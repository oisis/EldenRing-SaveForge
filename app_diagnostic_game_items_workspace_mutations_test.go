package main

import (
	"bytes"
	"strconv"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
	"github.com/oisis/EldenRing-SaveForge/backend/editor"
)

// Inventory Workspace Add / UpdateWeapon / Remove lifecycle coverage. Like the
// Move/Transfer/Reorder suite these are RAM-only session operations driven
// exclusively through the public App API. The session is injected with
// registerWorkspaceSession and every record is read back through the shared
// unlockLifecycle collector. Expected semantic values are derived from the real
// returned snapshot via the production workspaceItemValue/workspaceMutableKinds
// so the tests can never drift from what the driver actually recorded. No
// save-file fixture, no crafted binary, no t.Skip.

// Real DB values used below. The weapon base is a supported melee_armament with
// MaxUpgrade 25; the AoW is a real ashes_of_war entry; "Heavy" is a real
// infusion. UpdateWeapon needs these to resolve without a save fixture.
const (
	wsWeaponBaseID = uint32(0x002EFF40) // Banished Knight's Greatsword
	wsAoWItemID    = uint32(0x80002710) // Lion's Claw
	wsInfusion     = "Heavy"            // InfuseTypes offset 100
	wsUpgrade      = 5
)

// wsWeaponItem builds a weapon-editable original item. UpdateWeapon only needs
// IsWeapon + BaseItemID + MaxUpgrade to validate and re-encode, so no DB lookup
// is required to place it in the workspace.
func wsWeaponItem(uid string, pos int) editor.EditableItem {
	return editor.EditableItem{
		UID:        uid,
		Source:     editor.ItemSourceOriginal,
		Container:  editor.ContainerInventory,
		Position:   pos,
		ItemID:     wsWeaponBaseID,
		BaseItemID: wsWeaponBaseID,
		Quantity:   1,
		IsWeapon:   true,
		MaxUpgrade: 25,
	}
}

// snapItem returns a copy of the item with uid from the snapshot, or ok=false.
func snapItem(snap editor.InventoryWorkspaceSnapshot, uid string) (editor.EditableItem, bool) {
	for _, it := range append(append([]editor.EditableItem(nil), snap.InventoryItems...), snap.StorageItems...) {
		if it.UID == uid {
			return it, true
		}
	}
	return editor.EditableItem{}, false
}

// A. Add: inserting a new supported item at the front mints a "new:N" UID whose
// every mutable kind goes absent -> value, and renumbers the items behind it.
func TestInventoryWorkspaceAddLifecycle(t *testing.T) {
	const charIdx = 2
	app := NewApp()
	inv := []editor.EditableItem{wsItem("A", editor.ContainerInventory, 0), wsItem("B", editor.ContainerInventory, 1)}
	id := registerWorkspaceSession(app, charIdx, inv, nil, true)
	journal := app.journal

	spec := editor.AddItemSpec{BaseItemID: wsWeaponBaseID, Quantity: 1}
	snap, err := app.AddInventoryWorkspaceItem(id, spec, "inventory", 0)
	if err != nil {
		t.Fatalf("AddInventoryWorkspaceItem: %v", err)
	}

	// The new item is the one UID that was not present before.
	var newUID string
	for _, it := range snap.InventoryItems {
		if it.UID != "A" && it.UID != "B" {
			newUID = it.UID
		}
	}
	if newUID == "" {
		t.Fatalf("no new item found in returned snapshot")
	}
	newItem, ok := snapItem(snap, newUID)
	if !ok {
		t.Fatalf("new item %q missing from snapshot", newUID)
	}

	lc := collectUnlockLifecycle(t, journal.Tail(), actionGameItemsWorkspaceAdd, strconv.Itoa(charIdx))
	assertUnlockSuccess(t, lc)

	// Every mutable kind of the new item: absent -> real value, and finished
	// equals the real returned snapshot value (assertUnlockLifecycle checks the
	// finished column against the value we pull straight from snap).
	for _, kind := range workspaceMutableKinds {
		v := workspaceItemValue(newItem, true, kind)
		assertUnlockLifecycle(t, lc, wsField(newUID, kind), workspaceAbsent, v, v)
	}

	// Insertion at position 0 pushes the pre-existing items back one slot.
	assertUnlockLifecycle(t, lc, wsField("A", "position"), "0", "1", "1")
	assertUnlockLifecycle(t, lc, wsField("B", "position"), "1", "2", "2")

	// finished == real snapshot for the renumbered items.
	for _, uid := range []string{"A", "B"} {
		if got := snapPosition(snap, uid); got != lc.finished[wsField(uid, "position")] {
			t.Errorf("finished %s = %q, real snapshot = %q", wsField(uid, "position"), lc.finished[wsField(uid, "position")], got)
		}
	}
}

// B. Weapon patch: a patch that really changes upgrade, infusion and pending
// AoW logs exactly those fields (plus the re-encoded item_id and the pending
// flag) and nothing else; pending_aow_clear stays false and self-excludes.
func TestInventoryWorkspaceWeaponLifecycle(t *testing.T) {
	const charIdx = 1
	app := NewApp()
	inv := []editor.EditableItem{wsWeaponItem("W", 0)}
	id := registerWorkspaceSession(app, charIdx, inv, nil, true)
	journal := app.journal

	patch := editor.WeaponPatch{
		SetUpgrade:      true,
		Upgrade:         wsUpgrade,
		SetInfusionName: true,
		InfusionName:    wsInfusion,
		SetAoWItemID:    true,
		AoWItemID:       wsAoWItemID,
	}
	snap, err := app.UpdateInventoryWorkspaceWeapon(id, "W", patch)
	if err != nil {
		t.Fatalf("UpdateInventoryWorkspaceWeapon: %v", err)
	}
	weapon, ok := snapItem(snap, "W")
	if !ok {
		t.Fatalf("weapon missing from snapshot")
	}

	lc := collectUnlockLifecycle(t, journal.Tail(), actionGameItemsWorkspaceWeapon, strconv.Itoa(charIdx))
	assertUnlockSuccess(t, lc)

	// item_id re-encoded away from the base ID; before is the registered base.
	encodedID := workspaceItemValue(weapon, true, "item_id")
	if encodedID == giHex(wsWeaponBaseID) {
		t.Fatalf("item_id was not re-encoded, still %s", encodedID)
	}
	assertUnlockLifecycle(t, lc, wsField("W", "item_id"), giHex(wsWeaponBaseID), encodedID, encodedID)
	assertUnlockLifecycle(t, lc, wsField("W", "current_upgrade"), "0", strconv.Itoa(wsUpgrade), strconv.Itoa(wsUpgrade))
	assertUnlockLifecycle(t, lc, wsField("W", "infusion_name"), "", wsInfusion, wsInfusion)
	assertUnlockLifecycle(t, lc, wsField("W", "pending_aow_item_id"), giHex(0), giHex(wsAoWItemID), giHex(wsAoWItemID))

	pendingName := workspaceItemValue(weapon, true, "pending_aow_name")
	if pendingName == "" {
		t.Fatalf("pending_aow_name not resolved from DB")
	}
	assertUnlockLifecycle(t, lc, wsField("W", "pending_aow_name"), "", pendingName, pendingName)
	assertUnlockLifecycle(t, lc, wsField("W", "has_pending_weapon_patch"), "false", "true", "true")

	// No clear intent — pending_aow_clear stayed false and self-excludes.
	assertFieldAbsent(t, lc, wsField("W", "pending_aow_clear"))
}

// C. Remove: deleting the middle item of a three-item inventory drives it to
// absent across every kind and renumbers the tail; the untouched head
// self-excludes.
func TestInventoryWorkspaceRemoveLifecycle(t *testing.T) {
	const charIdx = 3
	app := NewApp()
	inv := []editor.EditableItem{
		wsItem("A", editor.ContainerInventory, 0),
		wsItem("B", editor.ContainerInventory, 1),
		wsItem("C", editor.ContainerInventory, 2),
	}
	id := registerWorkspaceSession(app, charIdx, inv, nil, true)
	journal := app.journal

	snap, err := app.RemoveInventoryWorkspaceItem(id, "B")
	if err != nil {
		t.Fatalf("RemoveInventoryWorkspaceItem: %v", err)
	}

	lc := collectUnlockLifecycle(t, journal.Tail(), actionGameItemsWorkspaceRemove, strconv.Itoa(charIdx))
	assertUnlockSuccess(t, lc)

	// The removed item: every mutable kind real value -> absent. Before values
	// are read from the same item that was registered.
	removed := wsItem("B", editor.ContainerInventory, 1)
	for _, kind := range workspaceMutableKinds {
		before := workspaceItemValue(removed, true, kind)
		assertUnlockLifecycle(t, lc, wsField("B", kind), before, workspaceAbsent, workspaceAbsent)
	}
	// B is truly gone from the returned snapshot.
	if _, ok := snapItem(snap, "B"); ok {
		t.Errorf("removed item B still present in snapshot")
	}

	// C shifts up to fill the gap; A keeps its slot and self-excludes.
	assertUnlockLifecycle(t, lc, wsField("C", "position"), "2", "1", "1")
	assertFieldAbsent(t, lc, wsField("A", "position"))
	assertFieldAbsent(t, lc, wsField("A", "container"))

	// finished == real snapshot for the shifted item.
	if got := snapPosition(snap, "C"); got != lc.finished[wsField("C", "position")] {
		t.Errorf("finished C position = %q, real snapshot = %q", lc.finished[wsField("C", "position")], got)
	}
}

// D1. No-op: an empty weapon patch changes no semantic field, so the operation
// succeeds and emits zero game_items_change_* records.
func TestInventoryWorkspaceWeaponEmptyPatchEmitsNothing(t *testing.T) {
	const charIdx = 0
	app := NewApp()
	inv := []editor.EditableItem{wsWeaponItem("W", 0)}
	id := registerWorkspaceSession(app, charIdx, inv, nil, true)
	journal := app.journal

	if _, err := app.UpdateInventoryWorkspaceWeapon(id, "W", editor.WeaponPatch{}); err != nil {
		t.Fatalf("UpdateInventoryWorkspaceWeapon empty patch: %v", err)
	}

	lc := collectUnlockLifecycle(t, journal.Tail(), actionGameItemsWorkspaceWeapon, strconv.Itoa(charIdx))
	if lc.count != 0 {
		t.Errorf("empty weapon patch emitted %d game_items_change_* records, want 0", lc.count)
	}
}

// D2. Rejected: a patch rejected before mutation (upgrade over MaxUpgrade)
// returns the editor error verbatim and emits zero records — the driver never
// fabricates a lifecycle for a mutation that did not run.
func TestInventoryWorkspaceWeaponRejectedEmitsNothing(t *testing.T) {
	const charIdx = 0
	app := NewApp()
	inv := []editor.EditableItem{wsWeaponItem("W", 0)}
	id := registerWorkspaceSession(app, charIdx, inv, nil, true)
	journal := app.journal

	_, err := app.UpdateInventoryWorkspaceWeapon(id, "W", editor.WeaponPatch{SetUpgrade: true, Upgrade: 999})
	if err == nil {
		t.Fatalf("expected an error for an out-of-range upgrade")
	}
	// The editor API error must reach the caller unchanged.
	var direct editor.InventoryWorkspaceSnapshot
	direct.InventoryItems = []editor.EditableItem{wsWeaponItem("W", 0)}
	wantErr := editor.UpdateWeapon(&direct, "W", editor.WeaponPatch{SetUpgrade: true, Upgrade: 999})
	if wantErr == nil || err.Error() != wantErr.Error() {
		t.Errorf("error = %v, want editor error %v", err, wantErr)
	}

	lc := collectUnlockLifecycle(t, journal.Tail(), actionGameItemsWorkspaceWeapon, strconv.Itoa(charIdx))
	if lc.count != 0 {
		t.Errorf("rejected weapon patch emitted %d game_items_change_* records, want 0", lc.count)
	}
}

// E. Debug off: a Remove still mutates RAM but keeps no records, and the backing
// slot.Data stays byte-identical — the operation never reaches into app.save. A
// synthetic non-empty slot stands in for a loaded save so the byte-identical
// claim is real without any save-file fixture.
func TestInventoryWorkspaceRemoveDebugOff(t *testing.T) {
	const charIdx = 4
	app := NewApp()
	app.save = &core.SaveFile{}
	slot := &app.save.Slots[charIdx]
	slot.Data = []byte{0xDE, 0xAD, 0xBE, 0xEF, 0x01, 0x02, 0x03, 0x04}
	before := append([]byte(nil), slot.Data...)

	inv := []editor.EditableItem{wsItem("A", editor.ContainerInventory, 0), wsItem("B", editor.ContainerInventory, 1)}
	id := registerWorkspaceSession(app, charIdx, inv, nil, false) // Debug Mode off
	journal := app.journal

	out, err := app.RemoveInventoryWorkspaceItem(id, "A")
	if err != nil {
		t.Fatalf("RemoveInventoryWorkspaceItem: %v", err)
	}

	// RAM mutation occurred: A is gone.
	if _, ok := snapItem(out, "A"); ok {
		t.Errorf("item A not removed from RAM workspace")
	}
	// slot.Data untouched — RAM-only operation.
	if !bytes.Equal(before, slot.Data) {
		t.Errorf("slot.Data mutated by a RAM-only workspace remove")
	}
	// No lifecycle records with Debug Mode off.
	lc := collectUnlockLifecycle(t, journal.Tail(), actionGameItemsWorkspaceRemove, strconv.Itoa(charIdx))
	if lc.count != 0 {
		t.Errorf("Debug-off remove emitted %d game_items_change_* records, want 0", lc.count)
	}
}
