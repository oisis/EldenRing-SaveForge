package main

import (
	"bytes"
	"encoding/binary"
	"strconv"
	"strings"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
)

// Task 7A — SaveInventoryWorkspaceChanges Debug Mode lifecycle at the RAM → save
// boundary. Unlike the RAM-only workspace mutation suites (Tasks 6A/6B) these
// records describe the PHYSICAL slot rows the commit changes: inventory/storage
// records, GaItem rows, the acquisition/equip counters and the container
// headers. The whole file is fixture-free — it builds a parseable slot in the
// same shape editor's own save tests use (fixtureSlotForSave: MagicOffset>0 so
// writeContainerLayout will write) and drives everything through the public App
// API. No ER0000.sl2, no t.Skip.
//
// Expected values are never hand-computed: the test clones the real slot just
// before the commit, then re-runs the production projector
// planGameItemsMutation(preSaveClone, realPostSaveSlot, nil) to obtain the exact
// field set the commit changed, and asserts the emitted lifecycle equals it.
// planned == finished == the real post-save slot follows by construction, so the
// test proves the debug clone projection matches what actually landed.

const (
	swDaggerHandle = uint32(0x80800001)
	swDaggerID     = uint32(0x000F4240) // Dagger (melee_armaments) — a weapon
	swDaggerAcq    = uint32(1000)
)

// saveWorkspaceApp builds an App whose slot charIdx holds one live Dagger in
// held-inventory row 0. MagicOffset is positive so ApplyWorkspaceSave writes;
// the in-memory Inventory struct and the 4-byte common-item header are seeded to
// match the single record so the pre-save physical state is self-consistent. The
// journal is in-memory with Debug Mode OFF — tests flip it on immediately before
// the commit they measure so only workspace_save records reach the tail.
func saveWorkspaceApp(charIdx int) (*App, *core.SaveSlot) {
	const magicOff = 16
	invStart := magicOff + core.InvStartFromMagic
	invEnd := invStart + core.CommonItemCount*core.InvRecordLen
	storageBoxOff := invEnd + 64
	storageStart := storageBoxOff + core.StorageHeaderSkip
	storageEnd := storageStart + core.StorageCommonCount*core.InvRecordLen
	bufSize := storageEnd + 64

	app := NewApp()
	app.journal = newInMemoryDiagnosticJournal()
	app.save = &core.SaveFile{}
	slot := &app.save.Slots[charIdx]
	slot.Version = 1
	slot.MagicOffset = magicOff
	slot.StorageBoxOffset = storageBoxOff
	slot.Data = make([]byte, bufSize)
	slot.GaMap = map[uint32]uint32{swDaggerHandle: swDaggerID}

	binary.LittleEndian.PutUint32(slot.Data[invStart:], swDaggerHandle)
	binary.LittleEndian.PutUint32(slot.Data[invStart+4:], 1)
	binary.LittleEndian.PutUint32(slot.Data[invStart+8:], swDaggerAcq)
	slot.Inventory.CommonItems = []core.InventoryItem{{GaItemHandle: swDaggerHandle, Quantity: 1, Index: swDaggerAcq}}
	slot.Inventory.NextAcquisitionSortId = swDaggerAcq + 1
	// common_item_count header at MagicOffset + InvStartFromMagic - 4 = one live row.
	binary.LittleEndian.PutUint32(slot.Data[invStart-4:], 1)

	return app, slot
}

func swIsDirectRowField(f string) bool { return strings.Contains(f, "_row_") }

func swIsHeaderOrCounterField(f string) bool {
	switch f {
	case "inventory_next_equip_index", "inventory_next_acquisition_sort_id",
		"storage_next_equip_index", "storage_next_acquisition_sort_id",
		"inventory_common_header_count", "storage_header_count":
		return true
	}
	return false
}

// A. Successful save: transferring the live item inventory → storage wipes and
// replays both containers, so the commit changes physical rows, both
// acquisition counters and both container headers — without allocating a new
// GaItem. The emitted before→planned→finished lifecycle must equal the
// production diff between the pre-save clone and the real post-save slot, with
// planned == finished == the real slot.
func TestGameItemsWorkspaceSaveLifecycleSuccess(t *testing.T) {
	const charIdx = 2
	app, slot := saveWorkspaceApp(charIdx)

	snap, err := app.StartInventoryEditSession(charIdx)
	if err != nil {
		t.Fatalf("StartInventoryEditSession: %v", err)
	}
	if len(snap.InventoryItems) == 0 {
		t.Fatal("seeded inventory item missing from snapshot")
	}
	// Real workspace edit (Debug Mode still off — no records for the transfer).
	if _, err := app.TransferInventoryWorkspaceItem(snap.SessionID, snap.InventoryItems[0].UID, "storage"); err != nil {
		t.Fatalf("TransferInventoryWorkspaceItem: %v", err)
	}

	// Measure only the commit.
	app.journal.SetDebugEnabled(true)
	preSave := core.CloneSlot(slot)

	updated, err := app.SaveInventoryWorkspaceChanges(snap.SessionID)
	if err != nil {
		t.Fatalf("SaveInventoryWorkspaceChanges: %v", err)
	}
	if updated.Dirty {
		t.Error("Dirty should be false after a successful save")
	}

	// Truth: the production projector run against the real pre/post slots.
	truth := planGameItemsMutation(preSave, slot, nil)
	planned := truth.records()
	finished := truth.finished(slot)
	if len(planned) == 0 {
		t.Fatal("prepared scenario changed no physical field")
	}

	lc := collectUnlockLifecycle(t, app.journal.Tail(), actionGameItemsWorkspaceSave, strconv.Itoa(charIdx))
	assertUnlockSuccess(t, lc)

	if lc.count != 3*len(planned) {
		t.Errorf("emitted %d game_items_change_* records, want %d (before+planned+finished per field)",
			lc.count, 3*len(planned))
	}

	sawRow, sawHeaderCounter := false, false
	for i, rec := range planned {
		f := rec.Field
		if swIsDirectRowField(f) {
			sawRow = true
		}
		if swIsHeaderOrCounterField(f) {
			sawHeaderCounter = true
		}
		if rec.Before == rec.After {
			t.Errorf("field %q self-exclude broken: before == planned == %q", f, rec.Before)
		}
		if finished[i].Field != f {
			t.Fatalf("planned/finished field order drift at %d: %q vs %q", i, f, finished[i].Field)
		}
		// planned == finished == real post-save slot.
		if rec.After != finished[i].After {
			t.Errorf("field %q planned %q != finished %q", f, rec.After, finished[i].After)
		}
		if got, ok := lc.before[f]; !ok || got != rec.Before {
			t.Errorf("before %q = %q (ok=%v), want %q", f, got, ok, rec.Before)
		}
		if got, ok := lc.planned[f]; !ok || got != rec.After {
			t.Errorf("planned %q = %q (ok=%v), want %q", f, got, ok, rec.After)
		}
		if got, ok := lc.finished[f]; !ok || got != finished[i].After {
			t.Errorf("finished %q = %q (ok=%v), want %q", f, got, ok, finished[i].After)
		}
	}
	if !sawRow {
		t.Error("no direct physical row field in the lifecycle")
	}
	if !sawHeaderCounter {
		t.Error("prepared scenario logged no changed header/counter family")
	}

	// before records must carry neither after, outcome nor stage.
	for _, rec := range app.journal.Tail() {
		if rec.Event != eventGameItemsChangeBefore {
			continue
		}
		if operationField(rec, "after") != "" || operationField(rec, "outcome") != "" || operationField(rec, "stage") != "" {
			t.Errorf("before record leaked a terminal field: %+v", rec.Fields)
		}
	}
}

// B. No-op save: re-committing an already-canonical slot with no new edits
// changes no physical field, so zero game_items_change_* records are emitted.
// The first (Debug-off) save canonicalizes headers/counters so the measured
// second save is a genuine byte-level no-op regardless of the seeded header.
func TestGameItemsWorkspaceSaveLifecycleNoOp(t *testing.T) {
	const charIdx = 5
	app, slot := saveWorkspaceApp(charIdx)

	snap, err := app.StartInventoryEditSession(charIdx)
	if err != nil {
		t.Fatalf("StartInventoryEditSession: %v", err)
	}
	// Canonicalize the slot with a real (Debug-off) save; no edits queued.
	if _, err := app.SaveInventoryWorkspaceChanges(snap.SessionID); err != nil {
		t.Fatalf("canonicalizing save: %v", err)
	}

	app.journal.SetDebugEnabled(true)
	before := append([]byte(nil), slot.Data...)

	if _, err := app.SaveInventoryWorkspaceChanges(snap.SessionID); err != nil {
		t.Fatalf("no-op save: %v", err)
	}
	if !bytes.Equal(before, slot.Data) {
		t.Fatal("no-op save mutated slot.Data — scenario is not a genuine no-op")
	}

	lc := collectUnlockLifecycle(t, app.journal.Tail(), actionGameItemsWorkspaceSave, strconv.Itoa(charIdx))
	if lc.count != 0 {
		t.Errorf("no-op save emitted %d game_items_change_* records, want 0", lc.count)
	}
}

// C. Rejected save: ApplyWorkspaceSave rejects a non-AoW item planted as a
// pending Ash of War BEFORE any mutation. The error reaches the caller, the real
// slot is byte-identical after rollback, and — because the clone rejects the
// same way before mutating — no lifecycle record is fabricated.
func TestGameItemsWorkspaceSaveLifecycleRejected(t *testing.T) {
	const charIdx = 7
	app, slot := saveWorkspaceApp(charIdx)

	snap, err := app.StartInventoryEditSession(charIdx)
	if err != nil {
		t.Fatalf("StartInventoryEditSession: %v", err)
	}
	weaponUID := ""
	for _, it := range snap.InventoryItems {
		if it.IsWeapon {
			weaponUID = it.UID
			break
		}
	}
	if weaponUID == "" {
		t.Fatal("seeded Dagger not classified as a weapon")
	}
	// Plant a non-AoW item ID (the Dagger itself) directly into PendingAoWItemID,
	// bypassing UpdateWeapon's validation — ApplyWorkspaceSave's pending-AoW
	// pre-flight rejects it before touching slot.Data.
	sess := app.editSessions[snap.SessionID]
	for i := range sess.Workspace.InventoryItems {
		if sess.Workspace.InventoryItems[i].UID == weaponUID {
			sess.Workspace.InventoryItems[i].PendingAoWItemID = swDaggerID
			break
		}
	}

	app.journal.SetDebugEnabled(true)
	before := append([]byte(nil), slot.Data...)

	if _, err := app.SaveInventoryWorkspaceChanges(snap.SessionID); err == nil {
		t.Fatal("expected rejection for a non-AoW pending item")
	}
	if !bytes.Equal(before, slot.Data) {
		t.Error("slot.Data mutated after a pre-mutation rejection")
	}

	lc := collectUnlockLifecycle(t, app.journal.Tail(), actionGameItemsWorkspaceSave, strconv.Itoa(charIdx))
	if lc.count != 0 {
		t.Errorf("rejected save emitted %d game_items_change_* records, want 0", lc.count)
	}
}

// D. Debug off: the real commit still runs and physically changes the slot, but
// no game_items_change_* record is emitted and no clone is taken.
func TestGameItemsWorkspaceSaveLifecycleDebugOff(t *testing.T) {
	const charIdx = 4
	app, slot := saveWorkspaceApp(charIdx)

	snap, err := app.StartInventoryEditSession(charIdx)
	if err != nil {
		t.Fatalf("StartInventoryEditSession: %v", err)
	}
	if len(snap.InventoryItems) == 0 {
		t.Fatal("seeded inventory item missing from snapshot")
	}
	transferHandle := snap.InventoryItems[0].OriginalHandle
	if _, err := app.TransferInventoryWorkspaceItem(snap.SessionID, snap.InventoryItems[0].UID, "storage"); err != nil {
		t.Fatalf("TransferInventoryWorkspaceItem: %v", err)
	}

	// Debug Mode stays off through the commit.
	before := append([]byte(nil), slot.Data...)
	updated, err := app.SaveInventoryWorkspaceChanges(snap.SessionID)
	if err != nil {
		t.Fatalf("SaveInventoryWorkspaceChanges: %v", err)
	}
	if bytes.Equal(before, slot.Data) {
		t.Error("Debug-off save did not physically change slot.Data")
	}
	// The transferred item now lives in storage, not inventory.
	for _, it := range updated.InventoryItems {
		if it.OriginalHandle == transferHandle {
			t.Errorf("handle 0x%08X still in inventory after transfer", transferHandle)
		}
	}
	found := false
	for _, it := range updated.StorageItems {
		if it.OriginalHandle == transferHandle {
			found = true
			break
		}
	}
	if !found {
		t.Error("transferred item missing from storage after Debug-off save")
	}

	lc := collectUnlockLifecycle(t, app.journal.Tail(), actionGameItemsWorkspaceSave, strconv.Itoa(charIdx))
	if lc.count != 0 {
		t.Errorf("Debug-off save emitted %d game_items_change_* records, want 0", lc.count)
	}
}
