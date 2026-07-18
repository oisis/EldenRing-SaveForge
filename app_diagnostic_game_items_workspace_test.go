package main

import (
	"bytes"
	"strconv"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
	"github.com/oisis/EldenRing-SaveForge/backend/editor"
)

// Inventory Workspace Move / Transfer / Reorder lifecycle coverage. These are
// RAM-only session operations, so the tests build a session directly from
// hand-crafted EditableItems (package main can set the session's exported
// fields) and drive it exclusively through the public App API — no save-file
// fixture, no crafted binary. Every record is read back from the in-memory
// journal via the shared unlockLifecycle collector. The Debug-off /
// slot.Data-untouched contract is asserted against a synthetic in-memory slot
// buffer, so the whole file is fixture-free and never skips.

// wsItem builds an original editable item at a fixed container position. Source
// is constant across every test snapshot so only the position/container fields
// under test ever diff.
func wsItem(uid string, container editor.ContainerKind, pos int) editor.EditableItem {
	return editor.EditableItem{
		UID:       uid,
		Source:    editor.ItemSourceOriginal,
		Container: container,
		Position:  pos,
	}
}

// registerWorkspaceSession injects a session with the given workspace directly
// into the registry and returns its ID. debug toggles the journal's Debug Mode.
func registerWorkspaceSession(app *App, charIdx int, inv, sto []editor.EditableItem, debug bool) string {
	journal := newInMemoryDiagnosticJournal()
	journal.SetDebugEnabled(debug)
	app.journal = journal

	const id = "ws-test-session"
	app.editSessions[id] = &editor.InventoryEditSession{
		ID:             id,
		CharacterIndex: charIdx,
		Workspace: editor.InventoryWorkspaceSnapshot{
			SessionID:      id,
			CharacterIndex: charIdx,
			InventoryItems: inv,
			StorageItems:   sto,
		},
	}
	return id
}

// snapPosition returns the recorded position of uid in the snapshot as the
// journal formats it (decimal string), or "" when absent.
func snapPosition(snap editor.InventoryWorkspaceSnapshot, uid string) string {
	for _, it := range append(append([]editor.EditableItem(nil), snap.InventoryItems...), snap.StorageItems...) {
		if it.UID == uid {
			return strconv.Itoa(it.Position)
		}
	}
	return ""
}

// snapContainer returns the container of uid as the journal formats it, or ""
// when absent.
func snapContainer(snap editor.InventoryWorkspaceSnapshot, uid string) string {
	for _, it := range append(append([]editor.EditableItem(nil), snap.InventoryItems...), snap.StorageItems...) {
		if it.UID == uid {
			return string(it.Container)
		}
	}
	return ""
}

func wsField(uid, kind string) string { return "workspace_item_" + uid + "_" + kind }

// A. Move within Inventory: relocating C to the front shifts every item's
// position; container is untouched, so no container field is emitted.
func TestInventoryWorkspaceMoveLifecycle(t *testing.T) {
	const charIdx = 3
	app := NewApp()
	inv := []editor.EditableItem{wsItem("A", editor.ContainerInventory, 0), wsItem("B", editor.ContainerInventory, 1), wsItem("C", editor.ContainerInventory, 2)}
	id := registerWorkspaceSession(app, charIdx, inv, nil, true)
	journal := app.journal

	snap, err := app.MoveInventoryWorkspaceItem(id, "C", "inventory", 0)
	if err != nil {
		t.Fatalf("MoveInventoryWorkspaceItem: %v", err)
	}

	lc := collectUnlockLifecycle(t, journal.Tail(), actionGameItemsWorkspaceMove, strconv.Itoa(charIdx))
	assertUnlockSuccess(t, lc)

	// Every position changed: [A,B,C] -> [C,A,B].
	assertUnlockLifecycle(t, lc, wsField("A", "position"), "0", "1", "1")
	assertUnlockLifecycle(t, lc, wsField("B", "position"), "1", "2", "2")
	assertUnlockLifecycle(t, lc, wsField("C", "position"), "2", "0", "0")

	// Container never changed within the same container.
	for _, uid := range []string{"A", "B", "C"} {
		assertFieldAbsent(t, lc, wsField(uid, "container"))
	}

	// finished equals the real returned workspace.
	for _, uid := range []string{"A", "B", "C"} {
		if got := snapPosition(snap, uid); got != lc.finished[wsField(uid, "position")] {
			t.Errorf("finished %s = %q, real snapshot = %q", wsField(uid, "position"), lc.finished[wsField(uid, "position")], got)
		}
	}
}

// B. Transfer Inventory -> Storage: the moved item flips container and lands at
// the end of storage; the vacated inventory rows renumber.
func TestInventoryWorkspaceTransferLifecycle(t *testing.T) {
	const charIdx = 1
	app := NewApp()
	inv := []editor.EditableItem{wsItem("A", editor.ContainerInventory, 0), wsItem("B", editor.ContainerInventory, 1), wsItem("C", editor.ContainerInventory, 2)}
	sto := []editor.EditableItem{wsItem("S", editor.ContainerStorage, 0)}
	id := registerWorkspaceSession(app, charIdx, inv, sto, true)
	journal := app.journal

	snap, err := app.TransferInventoryWorkspaceItem(id, "A", "storage")
	if err != nil {
		t.Fatalf("TransferInventoryWorkspaceItem: %v", err)
	}

	lc := collectUnlockLifecycle(t, journal.Tail(), actionGameItemsWorkspaceTransfer, strconv.Itoa(charIdx))
	assertUnlockSuccess(t, lc)

	// A: inventory -> storage, appended after S (position 1).
	assertUnlockLifecycle(t, lc, wsField("A", "container"), string(editor.ContainerInventory), string(editor.ContainerStorage), string(editor.ContainerStorage))
	assertUnlockLifecycle(t, lc, wsField("A", "position"), "0", "1", "1")
	// Inventory renumbers behind it.
	assertUnlockLifecycle(t, lc, wsField("B", "position"), "1", "0", "0")
	assertUnlockLifecycle(t, lc, wsField("C", "position"), "2", "1", "1")
	// The pre-existing storage item never moved.
	assertFieldAbsent(t, lc, wsField("S", "position"))
	assertFieldAbsent(t, lc, wsField("S", "container"))

	// finished equals the real returned workspace.
	if got := snapContainer(snap, "A"); got != lc.finished[wsField("A", "container")] {
		t.Errorf("finished A container = %q, real snapshot = %q", lc.finished[wsField("A", "container")], got)
	}
	for _, uid := range []string{"A", "B", "C"} {
		if got := snapPosition(snap, uid); got != lc.finished[wsField(uid, "position")] {
			t.Errorf("finished %s = %q, real snapshot = %q", wsField(uid, "position"), lc.finished[wsField(uid, "position")], got)
		}
	}
}

// C. Reorder: swapping the first two inventory items logs only the two items
// whose position actually changed; the untouched third item self-excludes.
func TestInventoryWorkspaceReorderLifecycle(t *testing.T) {
	const charIdx = 2
	app := NewApp()
	inv := []editor.EditableItem{wsItem("A", editor.ContainerInventory, 0), wsItem("B", editor.ContainerInventory, 1), wsItem("C", editor.ContainerInventory, 2)}
	sto := []editor.EditableItem{wsItem("S", editor.ContainerStorage, 0)}
	id := registerWorkspaceSession(app, charIdx, inv, sto, true)
	journal := app.journal

	snap, err := app.ReorderInventoryWorkspaceItems(id, []string{"B", "A", "C"}, []string{"S"})
	if err != nil {
		t.Fatalf("ReorderInventoryWorkspaceItems: %v", err)
	}

	lc := collectUnlockLifecycle(t, journal.Tail(), actionGameItemsWorkspaceReorder, strconv.Itoa(charIdx))
	assertUnlockSuccess(t, lc)

	// Only A and B swapped positions.
	assertUnlockLifecycle(t, lc, wsField("A", "position"), "0", "1", "1")
	assertUnlockLifecycle(t, lc, wsField("B", "position"), "1", "0", "0")
	// C and S kept their positions and emit nothing.
	assertFieldAbsent(t, lc, wsField("C", "position"))
	assertFieldAbsent(t, lc, wsField("S", "position"))

	for _, uid := range []string{"A", "B"} {
		if got := snapPosition(snap, uid); got != lc.finished[wsField(uid, "position")] {
			t.Errorf("finished %s = %q, real snapshot = %q", wsField(uid, "position"), lc.finished[wsField(uid, "position")], got)
		}
	}
}

// D. No-op: a valid Move to an item's current position changes no semantic
// state, so zero game_items_change_* records are emitted.
func TestInventoryWorkspaceMoveNoopEmitsNothing(t *testing.T) {
	const charIdx = 0
	app := NewApp()
	inv := []editor.EditableItem{wsItem("A", editor.ContainerInventory, 0), wsItem("B", editor.ContainerInventory, 1), wsItem("C", editor.ContainerInventory, 2)}
	id := registerWorkspaceSession(app, charIdx, inv, nil, true)
	journal := app.journal

	if _, err := app.MoveInventoryWorkspaceItem(id, "A", "inventory", 0); err != nil {
		t.Fatalf("MoveInventoryWorkspaceItem: %v", err)
	}

	lc := collectUnlockLifecycle(t, journal.Tail(), actionGameItemsWorkspaceMove, strconv.Itoa(charIdx))
	if lc.count != 0 {
		t.Errorf("no-op move emitted %d game_items_change_* records, want 0", lc.count)
	}
}

// E. Debug off: the RAM mutation still happens, no lifecycle records are kept,
// and the backing slot.Data is byte-identical — the workspace transfer is
// RAM-only and never reaches into app.save. Fully in-memory: a synthetic slot
// buffer stands in for a loaded save so the byte-identical claim is asserted on
// a real slot without any save-file fixture.
func TestInventoryWorkspaceTransferDebugOff(t *testing.T) {
	const charIdx = 4
	app := NewApp()
	app.save = &core.SaveFile{}
	// Deterministic non-empty slot buffer; the transfer must leave it untouched.
	slot := &app.save.Slots[charIdx]
	slot.Data = []byte{0xDE, 0xAD, 0xBE, 0xEF, 0x01, 0x02, 0x03, 0x04}
	before := append([]byte(nil), slot.Data...)

	inv := []editor.EditableItem{wsItem("A", editor.ContainerInventory, 0), wsItem("B", editor.ContainerInventory, 1)}
	sto := []editor.EditableItem{wsItem("S", editor.ContainerStorage, 0)}
	id := registerWorkspaceSession(app, charIdx, inv, sto, false) // Debug Mode off
	journal := app.journal

	out, err := app.TransferInventoryWorkspaceItem(id, "A", "storage")
	if err != nil {
		t.Fatalf("TransferInventoryWorkspaceItem: %v", err)
	}

	// RAM mutation occurred: the item is now in storage.
	if snapContainer(out, "A") != string(editor.ContainerStorage) {
		t.Errorf("item %q not transferred to storage", "A")
	}
	// slot.Data untouched — RAM-only operation.
	if !bytes.Equal(before, slot.Data) {
		t.Errorf("slot.Data mutated by a RAM-only workspace transfer")
	}
	// No lifecycle records with Debug Mode off.
	lc := collectUnlockLifecycle(t, journal.Tail(), actionGameItemsWorkspaceTransfer, strconv.Itoa(charIdx))
	if lc.count != 0 {
		t.Errorf("Debug-off transfer emitted %d game_items_change_* records, want 0", lc.count)
	}
}
