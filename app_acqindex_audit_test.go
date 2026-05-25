package main

import (
	"strconv"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
	"github.com/oisis/EldenRing-SaveForge/backend/db"
	"github.com/oisis/EldenRing-SaveForge/backend/editor"
)

// TestAcqIndexAudit drives the operations the user suspects of creating
// duplicate acquisition indices — bulk add, reorder (workspace wipe-and-replay),
// apply AoW to weapons, change weapon level — on a clean save, scanning for
// duplicate inventory indices after every step. Driven through the editor/core
// layer directly (App wrappers need a live Wails context). A clean current
// backend must keep the duplicate count at 0 throughout.
func TestAcqIndexAudit(t *testing.T) {
	app, idx := realSaveAppForSave(t)
	slot := &app.save.Slots[idx]

	scan := func(label string) {
		t.Helper()
		dups := core.ScanDuplicateInventoryIndices(slot)
		if len(dups) > 0 {
			d := dups[0]
			t.Fatalf("%s: %d dup acq-index issue(s); e.g. Index %d in %s (0x%08X also row %d 0x%08X)",
				label, len(dups), d.Index, d.Scope, d.DuplicateHandle, d.FirstRow, d.FirstHandle)
		}
		t.Logf("%s: 0 dups (NextAcq=%d NextEquip=%d)", label,
			slot.Inventory.NextAcquisitionSortId, slot.Inventory.NextEquipIndex)
	}
	scan("baseline")

	var aowIDs []uint32      // ash goods items (0x4-prefix) for inventory add
	var mountAoWIDs []uint32 // mountable AoW gems (0x8-prefix) for weapon attach
	for _, it := range db.GetAllItems("PC") {
		if it.Category == "ashes" && it.ID>>28 == 4 && len(aowIDs) < 60 {
			aowIDs = append(aowIDs, it.ID)
		}
		if it.ID>>28 == 8 && len(mountAoWIDs) < 60 {
			mountAoWIDs = append(mountAoWIDs, it.ID)
		}
	}
	if len(aowIDs) == 0 || len(mountAoWIDs) == 0 {
		t.Skip("no AoW in DB")
	}

	// STEP 1 — bulk add AoW (qty 10 each; AoW caps to 1 per item).
	if err := core.AddItemsToSlot(slot, aowIDs, 10, 0, false); err != nil {
		t.Fatalf("AddItemsToSlot(AoW): %v", err)
	}
	scan("after bulk-add AoW x60")

	// STEP 2 — reorder via workspace (forces wipe-and-replay) then save.
	sess, err := editor.StartSession(slot, idx)
	if err != nil {
		t.Fatalf("StartSession: %v", err)
	}
	if n := len(sess.Workspace.InventoryItems); n >= 3 {
		if err := editor.MoveItem(&sess.Workspace, sess.Workspace.InventoryItems[0].UID, editor.ContainerInventory, n-1); err != nil {
			t.Fatalf("MoveItem: %v", err)
		}
		if _, err := editor.ApplyWorkspaceSave(slot, &sess.Workspace, sess.BaselineEditableHandles); err != nil {
			t.Fatalf("ApplyWorkspaceSave(reorder): %v", err)
		}
	}
	scan("after reorder+save")

	// Add known AoW-mountable weapons so STEP 3 actually exercises the path
	// (ER0000's inventory weapons are all somber/unique = gemMountType 0).
	if err := core.AddItemsToSlot(slot, []uint32{1030000, 2010000, 4000000, 1000000}, 1, 0, false); err != nil {
		t.Fatalf("AddItemsToSlot(weapons): %v", err)
	}
	scan("after add mountable weapons")

	// STEP 3 — apply AoW to several mountable weapons (mints AoW GaItems).
	sess2, err := editor.StartSession(slot, idx)
	if err != nil {
		t.Fatalf("StartSession2: %v", err)
	}
	wepCount, mountCount := 0, 0
	for _, it := range sess2.Workspace.InventoryItems {
		if it.IsWeapon {
			wepCount++
		}
		if it.IsWeapon && it.CanMountAoW {
			mountCount++
		}
	}
	t.Logf("sess2: invItems=%d weapons=%d mountable=%d", len(sess2.Workspace.InventoryItems), wepCount, mountCount)

	applied := 0
	for _, it := range sess2.Workspace.InventoryItems {
		if applied >= 8 {
			break
		}
		if !it.IsWeapon || !it.CanMountAoW || it.OriginalHandle == 0 {
			continue
		}
		// Pick a compatible AoW; fall back to first AoW (PatchWeaponAoW is
		// core-level and does not gate on compatibility).
		chosen := mountAoWIDs[0]
		for _, a := range mountAoWIDs {
			if ok, known := db.IsAshOfWarCompatibleWithWeapon(a, it.ItemID); known && ok {
				chosen = a
				break
			}
		}
		if err := core.PatchWeaponAoW(slot, it.OriginalHandle, chosen); err != nil {
			t.Logf("PatchWeaponAoW(0x%08X, 0x%08X): %v", it.OriginalHandle, chosen, err)
			continue
		}
		applied++
		scan("after PatchWeaponAoW #" + strconv.Itoa(applied))
	}
	t.Logf("applied AoW to %d weapons", applied)

	// STEP 4 — change a weapon level via workspace.
	sess3, err := editor.StartSession(slot, idx)
	if err != nil {
		t.Fatalf("StartSession3: %v", err)
	}
	for _, it := range sess3.Workspace.InventoryItems {
		if it.IsWeapon && it.MaxUpgrade > 0 {
			target := 1
			if it.CurrentUpgrade == 1 {
				target = 2
			}
			if err := editor.UpdateWeapon(&sess3.Workspace, it.UID, editor.WeaponPatch{SetUpgrade: true, Upgrade: target}); err != nil {
				t.Fatalf("UpdateWeapon: %v", err)
			}
			break
		}
	}
	if _, err := editor.ApplyWorkspaceSave(slot, &sess3.Workspace, sess3.BaselineEditableHandles); err != nil {
		t.Fatalf("ApplyWorkspaceSave(level): %v", err)
	}
	scan("after weapon-level+save")

	// STEP 5 — the user's exact suspect order: AoW-on-weapons already applied,
	// now reorder+save repeatedly (accumulation). Each cycle reparses, so a
	// counter/index leak would compound and surface as duplicates.
	for cycle := 1; cycle <= 5; cycle++ {
		s, err := editor.StartSession(slot, idx)
		if err != nil {
			t.Fatalf("StartSession cycle %d: %v", cycle, err)
		}
		if n := len(s.Workspace.InventoryItems); n >= 4 {
			if err := editor.MoveItem(&s.Workspace, s.Workspace.InventoryItems[cycle%n].UID, editor.ContainerInventory, (cycle*3)%n); err != nil {
				t.Fatalf("MoveItem cycle %d: %v", cycle, err)
			}
		}
		if _, err := editor.ApplyWorkspaceSave(slot, &s.Workspace, s.BaselineEditableHandles); err != nil {
			t.Fatalf("ApplyWorkspaceSave cycle %d: %v", cycle, err)
		}
		scan("after reorder+save cycle #" + strconv.Itoa(cycle))
	}

	// Final structural integrity check — the result must reload cleanly.
	if err := core.ValidateSlotIntegrity(slot); err != nil {
		t.Fatalf("final slot integrity: %v", err)
	}
}
