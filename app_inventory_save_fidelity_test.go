package main

import (
	"bytes"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
	"github.com/oisis/EldenRing-SaveForge/backend/editor"
)

// These tests pin the save-fidelity contract that fixes the game-crash
// regression: the workspace save path must only rewrite what actually
// changed. The pre-fix code wiped and reindexed the entire inventory +
// storage region on every save, producing a layout the game rejected.

// TestSaveWorkspace_NoOpIsByteIdentical — committing an unedited session must
// leave the slot bytes exactly as loaded.
func TestSaveWorkspace_NoOpIsByteIdentical(t *testing.T) {
	app, idx := realSaveAppForSave(t)
	before := append([]byte(nil), app.save.Slots[idx].Data...)

	snap, err := app.StartInventoryEditSession(idx)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if _, err := app.SaveInventoryWorkspaceChanges(snap.SessionID); err != nil {
		t.Fatalf("Save (no-op): %v", err)
	}

	if !bytes.Equal(before, app.save.Slots[idx].Data) {
		n, first := diffStats(before, app.save.Slots[idx].Data)
		t.Fatalf("no-op save changed %d bytes (firstDiff=0x%X) — must be byte-identical", n, first)
	}
}

// TestSaveWorkspace_WeaponLevelLeavesContainersUntouched — editing a weapon
// upgrade rewrites the weapon's GaItem ItemID but must NOT touch the
// inventory or storage CommonItems regions.
func TestSaveWorkspace_WeaponLevelLeavesContainersUntouched(t *testing.T) {
	app, idx := realSaveAppForSave(t)
	slot := &app.save.Slots[idx]

	invS := slot.MagicOffset + core.InvStartFromMagic
	invE := invS + core.CommonItemCount*core.InvRecordLen
	stoS := slot.StorageBoxOffset + core.StorageHeaderSkip
	stoE := stoS + core.StorageCommonCount*core.InvRecordLen
	invBefore := append([]byte(nil), slot.Data[invS:invE]...)
	stoBefore := append([]byte(nil), slot.Data[stoS:stoE]...)

	snap, err := app.StartInventoryEditSession(idx)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	uid, target := firstUpgradeableWeapon(snap.InventoryItems)
	if uid == "" {
		t.Skip("no upgradeable weapon in inventory")
	}
	if _, err := app.UpdateInventoryWorkspaceWeapon(snap.SessionID, uid,
		editor.WeaponPatch{SetUpgrade: true, Upgrade: target}); err != nil {
		t.Fatalf("UpdateWeapon: %v", err)
	}
	if _, err := app.SaveInventoryWorkspaceChanges(snap.SessionID); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if !bytes.Equal(invBefore, slot.Data[invS:invE]) {
		t.Error("weapon level edit altered the inventory region (should be untouched)")
	}
	if !bytes.Equal(stoBefore, slot.Data[stoS:stoE]) {
		t.Error("weapon level edit altered the storage region (should be untouched)")
	}
}

// TestSaveWorkspace_ReorderPreservesSlotsAndReloads — a pure reorder must
// keep items at their physical slots, only swap acquisition indices among the
// existing pool, and reload as a structurally valid save.
func TestSaveWorkspace_ReorderPreservesSlotsAndReloads(t *testing.T) {
	app, idx := realSaveAppForSave(t)
	slot := &app.save.Slots[idx]

	snap, err := app.StartInventoryEditSession(idx)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if len(snap.InventoryItems) < 3 {
		t.Skip("need >=3 inventory items to reorder")
	}

	invS := slot.MagicOffset + core.InvStartFromMagic
	invE := invS + core.CommonItemCount*core.InvRecordLen
	// Acquisition-index multiset before the reorder.
	poolBefore := acqMultiset(slot.Data[invS:invE])

	if _, err := app.MoveInventoryWorkspaceItem(snap.SessionID, snap.InventoryItems[0].UID, "inventory", 2); err != nil {
		t.Fatalf("Move: %v", err)
	}
	if _, err := app.SaveInventoryWorkspaceChanges(snap.SessionID); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// The acquisition-index pool must be reused (no new indices invented).
	poolAfter := acqMultiset(slot.Data[invS:invE])
	if !equalMultiset(poolBefore, poolAfter) {
		t.Error("reorder allocated new acquisition indices instead of reusing the existing pool")
	}

	// Result must reload and pass integrity.
	if err := core.ValidateSlotIntegrity(slot); err != nil {
		t.Fatalf("post-reorder slot fails integrity: %v", err)
	}
}

// ── helpers ──────────────────────────────────────────────────────────────

func diffStats(a, b []byte) (count, first int) {
	first = -1
	for i := 0; i < len(a) && i < len(b); i++ {
		if a[i] != b[i] {
			count++
			if first < 0 {
				first = i
			}
		}
	}
	return
}

func firstUpgradeableWeapon(items []editor.EditableItem) (uid string, target int) {
	for _, it := range items {
		if it.IsWeapon && it.MaxUpgrade > 0 && it.CurrentUpgrade >= 0 && it.CurrentUpgrade <= it.MaxUpgrade {
			if it.CurrentUpgrade > 0 {
				return it.UID, it.CurrentUpgrade - 1
			}
			return it.UID, 1
		}
	}
	return "", 0
}

func acqMultiset(region []byte) map[uint32]int {
	m := map[uint32]int{}
	for off := 0; off+core.InvRecordLen <= len(region); off += core.InvRecordLen {
		handle := uint32(region[off]) | uint32(region[off+1])<<8 | uint32(region[off+2])<<16 | uint32(region[off+3])<<24
		if handle == 0 {
			continue // empty slot
		}
		acq := uint32(region[off+8]) | uint32(region[off+9])<<8 | uint32(region[off+10])<<16 | uint32(region[off+11])<<24
		m[acq]++
	}
	return m
}

func equalMultiset(a, b map[uint32]int) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}
