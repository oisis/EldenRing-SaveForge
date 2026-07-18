package main

import (
	"encoding/binary"
	"strconv"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
	"github.com/oisis/EldenRing-SaveForge/backend/editor"
)

// repairWorkspaceFixture starts an ordinary editable workspace on the compact
// in-memory slot used by the workspace-save suite, then introduces precisely
// the repairable validation issue the public endpoint is expected to clamp.
func repairWorkspaceFixture(t *testing.T) (*App, string, string) {
	t.Helper()
	app := diagnosticRepackApp(t)
	app.journal = newInMemoryDiagnosticJournal()
	slot := &app.save.Slots[0]
	weapon := slot.GaItems[2]
	if weapon.IsEmpty() {
		t.Fatal("diagnostic repack fixture lost its weapon record")
	}
	invStart := slot.MagicOffset + core.InvStartFromMagic
	binary.LittleEndian.PutUint32(slot.Data[invStart-4:], 1)
	binary.LittleEndian.PutUint32(slot.Data[invStart:], weapon.Handle)
	binary.LittleEndian.PutUint32(slot.Data[invStart+4:], 1)
	binary.LittleEndian.PutUint32(slot.Data[invStart+8:], 1000)
	slot.Inventory.CommonItems = []core.InventoryItem{{GaItemHandle: weapon.Handle, Quantity: 1, Index: 1000}}
	slot.Inventory.NextAcquisitionSortId = 1001

	snap, err := app.StartInventoryEditSession(0)
	if err != nil {
		t.Fatalf("StartInventoryEditSession: %v", err)
	}
	if len(snap.InventoryItems) != 1 {
		t.Fatalf("inventory items=%d, want one", len(snap.InventoryItems))
	}

	sess, err := app.acquireSession(snap.SessionID)
	if err != nil {
		t.Fatalf("acquireSession: %v", err)
	}
	item := &sess.Workspace.InventoryItems[0]
	item.IsWeapon = true
	item.MaxUpgrade = 25
	item.CurrentUpgrade = 99
	uid := item.UID
	sess.Unlock()
	return app, snap.SessionID, uid
}

// repairActionRecords excludes the separately logged workspace_save commit
// that follows a successful repair. This task asserts the RAM-side auto-repair
// lifecycle itself; SaveInventoryWorkspaceChanges keeps its established
// physical-slot lifecycle under actionGameItemsWorkspaceSave.
func repairActionRecords(records []diagnosticRecord) []diagnosticRecord {
	var out []diagnosticRecord
	for _, rec := range records {
		if operationField(rec, "action") == actionGameItemsWorkspaceRepair {
			out = append(out, rec)
		}
	}
	return out
}

func TestInventoryWorkspaceAutoRepairLifecycle(t *testing.T) {
	app, sessionID, uid := repairWorkspaceFixture(t)
	app.journal.SetDebugEnabled(true)

	snap, err := app.RepairInventoryWorkspaceItem(sessionID, uid, editor.CodeUpgradeOutOfRange)
	if err != nil {
		t.Fatalf("RepairInventoryWorkspaceItem: %v", err)
	}
	item, ok := snapItem(snap, uid)
	if !ok {
		t.Fatalf("repaired item %q missing", uid)
	}
	if item.CurrentUpgrade != item.MaxUpgrade {
		t.Fatalf("CurrentUpgrade=%d, want clamped MaxUpgrade=%d", item.CurrentUpgrade, item.MaxUpgrade)
	}

	lc := collectUnlockLifecycle(t, repairActionRecords(app.journal.Tail()), actionGameItemsWorkspaceRepair, "0")
	assertUnlockSuccess(t, lc)
	assertUnlockLifecycle(t, lc, wsField(uid, "current_upgrade"), "99", strconv.Itoa(item.MaxUpgrade), strconv.Itoa(item.MaxUpgrade))
}

func TestInventoryWorkspaceAutoRepairBatchLifecycle(t *testing.T) {
	app, sessionID, uid := repairWorkspaceFixture(t)
	app.journal.SetDebugEnabled(true)

	// The endpoint remains best-effort: an invalid spec is ignored while the
	// repairable one still changes the workspace and emits its actual lifecycle.
	snap, err := app.RepairInventoryWorkspaceItems(sessionID, []WorkspaceRepairSpec{
		{UID: "missing", Code: editor.CodeUpgradeOutOfRange},
		{UID: uid, Code: editor.CodeUpgradeOutOfRange},
	})
	if err != nil {
		t.Fatalf("RepairInventoryWorkspaceItems: %v", err)
	}
	item, ok := snapItem(snap, uid)
	if !ok {
		t.Fatalf("repaired item %q missing", uid)
	}
	if item.CurrentUpgrade != item.MaxUpgrade {
		t.Fatalf("CurrentUpgrade=%d, want clamped MaxUpgrade=%d", item.CurrentUpgrade, item.MaxUpgrade)
	}

	lc := collectUnlockLifecycle(t, repairActionRecords(app.journal.Tail()), actionGameItemsWorkspaceRepair, "0")
	assertUnlockSuccess(t, lc)
	assertUnlockLifecycle(t, lc, wsField(uid, "current_upgrade"), "99", strconv.Itoa(item.MaxUpgrade), strconv.Itoa(item.MaxUpgrade))
}

func TestInventoryWorkspaceAutoRepairRejectedEmitsNothing(t *testing.T) {
	app, sessionID, uid := repairWorkspaceFixture(t)
	app.journal.SetDebugEnabled(true)

	if _, err := app.RepairInventoryWorkspaceItem(sessionID, uid, "not_repairable"); err == nil {
		t.Fatal("RepairInventoryWorkspaceItem: want error for non-repairable code")
	}
	lc := collectUnlockLifecycle(t, repairActionRecords(app.journal.Tail()), actionGameItemsWorkspaceRepair, "0")
	if lc.count != 0 {
		t.Fatalf("rejected repair emitted %d records, want 0", lc.count)
	}
}

func TestInventoryWorkspaceAutoRepairDebugOffEmitsNothing(t *testing.T) {
	app, sessionID, uid := repairWorkspaceFixture(t)
	app.journal.SetDebugEnabled(false)

	snap, err := app.RepairInventoryWorkspaceItem(sessionID, uid, editor.CodeUpgradeOutOfRange)
	if err != nil {
		t.Fatalf("RepairInventoryWorkspaceItem: %v", err)
	}
	item, ok := snapItem(snap, uid)
	if !ok || item.CurrentUpgrade != item.MaxUpgrade {
		t.Fatalf("debug-off repair did not clamp item: %+v", item)
	}
	lc := collectUnlockLifecycle(t, repairActionRecords(app.journal.Tail()), actionGameItemsWorkspaceRepair, "0")
	if lc.count != 0 {
		t.Fatalf("debug-off repair emitted %d records, want 0", lc.count)
	}
}
