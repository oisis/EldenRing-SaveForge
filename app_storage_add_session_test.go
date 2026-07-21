package main

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
)

// databaseAddTalismanID is Radagon's Scarseal — same representative the
// existing talisman capacity endpoint test (app_talisman_capacity_test.go)
// uses. Talismans need no serialized GaItem record and always append a new
// physical record even when the same ID repeats (dup=true), which is exactly
// what a T350 regression needs: six independent Database Add calls that each
// must land their own distinct Storage record.
const databaseAddTalismanID = endpointTalismanID

// emptyStorageDatabaseAddApp builds a real, byte-accurate single-slot save
// (parsed through the public core.SaveSlot.Read path, not a hand-built
// struct) whose Storage has the genuine T310 empty signature: no CommonItems
// records, NextAcquisitionSortId<=1, NextEquipIndex==0. Building it via
// Read() — the same real load path core.LoadSave uses — means
// slot.Storage's binary counter offsets are the real ones core computed, so
// tests can assert on slot.Data directly instead of only in-memory fields.
//
// storageNextAcqOff is returned alongside the app so tests can verify the
// binary NextAcquisitionSortId write-back; EquipInventoryData only exports
// NextEquipIndexOff(), not the acquisition-sort-id offset, so it's
// recomputed here from the same exported constants core itself uses
// (mirrors app_remembrance_game_limits_test.go / app_storage_order_test.go).
func emptyStorageDatabaseAddApp(t *testing.T) (app *App, storageNextAcqOff int) {
	t.Helper()

	data := make([]byte, core.SlotSize)
	binary.LittleEndian.PutUint32(data, core.GaItemVersionBreak+1)
	magicOffset := core.GaItemsStart + core.DynPlayerData - 1
	copy(data[magicOffset:], core.MagicPattern)

	var slot core.SaveSlot
	if err := slot.Read(core.NewReader(data), string(core.PlatformPC)); err != nil {
		t.Fatalf("SaveSlot.Read: %v", err)
	}
	if len(slot.Storage.CommonItems) != 0 || slot.Storage.NextAcquisitionSortId > 1 || slot.Storage.NextEquipIndex != 0 {
		t.Fatalf("fixture is not the T310 empty signature: CommonItems=%d NextAcquisitionSortId=%d NextEquipIndex=%d",
			len(slot.Storage.CommonItems), slot.Storage.NextAcquisitionSortId, slot.Storage.NextEquipIndex)
	}

	app = NewApp()
	app.save = &core.SaveFile{Platform: core.PlatformPC}
	app.save.UserData10.Data = make([]byte, 0x60000) // required by SaveFile.flushMetadata, exercised by writeSaveCore in the session-boundary tests
	app.save.Slots[0] = slot

	storageStart := slot.StorageBoxOffset + core.StorageHeaderSkip
	storageNextAcqOff = storageStart + core.StorageNextAcqSortRel
	return app, storageNextAcqOff
}

// TestAddItemsToCharacter_SixIndependentDatabaseAddCallsOnEmptyStorageMatchT350
// reproduces T350 through the real public app lifecycle: six separate
// App.AddItemsToCharacter calls (the exact endpoint the Database Add UI
// calls), each adding one item to Storage, all before any Save — against a
// character whose Storage was genuinely empty (T310 signature) when this test
// began. The result must match T330's single six-item native batch: indexes
// 2,4,6,8,10,12, NextEquipIndex=133, NextAcquisitionSortId=7 — both in the
// parsed struct and in slot.Data, not the pre-fix result where only the first
// of the six calls saw the empty-Storage signature.
func TestAddItemsToCharacter_SixIndependentDatabaseAddCallsOnEmptyStorageMatchT350(t *testing.T) {
	app, storageNextAcqOff := emptyStorageDatabaseAddApp(t)

	for i := 0; i < 6; i++ {
		result, err := app.AddItemsToCharacter(0, []uint32{databaseAddTalismanID}, 0, 0, 0, 0, 0, 1)
		if err != nil {
			t.Fatalf("independent AddItemsToCharacter call %d: %v", i+1, err)
		}
		if result.Added != 1 {
			t.Fatalf("call %d: Added=%d, want 1 (CapHit=%q)", i+1, result.Added, result.CapHit)
		}
	}

	slot := &app.save.Slots[0]
	if len(slot.Storage.CommonItems) != 6 {
		t.Fatalf("CommonItems count: got %d, want 6 (T350)", len(slot.Storage.CommonItems))
	}

	wantIndexes := map[uint32]bool{2: true, 4: true, 6: true, 8: true, 10: true, 12: true}
	gotIndexes := make(map[uint32]bool)
	for _, item := range slot.Storage.CommonItems {
		gotIndexes[item.Index] = true
	}
	for idx := range wantIndexes {
		if !gotIndexes[idx] {
			t.Errorf("missing expected record Index %d", idx)
		}
	}
	for idx := range gotIndexes {
		if !wantIndexes[idx] {
			t.Errorf("unexpected record Index %d", idx)
		}
	}

	if slot.Storage.NextEquipIndex != 133 {
		t.Errorf("Storage.NextEquipIndex: got %d, want 133 (T350)", slot.Storage.NextEquipIndex)
	}
	if slot.Storage.NextAcquisitionSortId != 7 {
		t.Errorf("Storage.NextAcquisitionSortId: got %d, want 7 (T350)", slot.Storage.NextAcquisitionSortId)
	}

	rawEquip := binary.LittleEndian.Uint32(slot.Data[slot.Storage.NextEquipIndexOff():])
	if rawEquip != 133 {
		t.Errorf("binary NextEquipIndex: got %d, want 133", rawEquip)
	}
	rawAcq := binary.LittleEndian.Uint32(slot.Data[storageNextAcqOff:])
	if rawAcq != 7 {
		t.Errorf("binary NextAcquisitionSortId: got %d, want 7", rawAcq)
	}
}

// TestAddItemsToCharacter_DatabaseAddSessionDoesNotSurviveSuccessfulSave is the
// session-boundary regression: a Database Add series must NOT carry its
// empty-Storage context across a successful ordinary Save. After one
// successful add (which both mutates Storage away from empty AND would
// establish the T350 session), a real save-to-disk via writeSaveCore (the
// same internal path WriteSave uses, exercised directly here to avoid the
// Wails file dialog) must end the series. A second independent add afterward
// must fall back to the pre-existing populated-Storage policy — NextEquipIndex
// left unchanged — never the T330 same-series +1 rule.
func TestAddItemsToCharacter_DatabaseAddSessionDoesNotSurviveSuccessfulSave(t *testing.T) {
	app, _ := emptyStorageDatabaseAddApp(t)

	if _, err := app.AddItemsToCharacter(0, []uint32{databaseAddTalismanID}, 0, 0, 0, 0, 0, 1); err != nil {
		t.Fatalf("first AddItemsToCharacter: %v", err)
	}
	slot := &app.save.Slots[0]
	if slot.Storage.NextEquipIndex != 128 || slot.Storage.NextAcquisitionSortId != 2 {
		t.Fatalf("after first add: NextEquipIndex=%d NextAcquisitionSortId=%d, want 128/2 (T310 init)",
			slot.Storage.NextEquipIndex, slot.Storage.NextAcquisitionSortId)
	}
	equipBeforeSave := slot.Storage.NextEquipIndex

	tmpPath := filepath.Join(t.TempDir(), "session-boundary.sl2")
	if _, err := app.writeSaveCore(tmpPath, app.save); err != nil {
		t.Fatalf("writeSaveCore: %v", err)
	}
	if _, err := os.Stat(tmpPath); err != nil {
		t.Fatalf("save file was not written: %v", err)
	}

	if _, err := app.AddItemsToCharacter(0, []uint32{databaseAddTalismanID}, 0, 0, 0, 0, 0, 1); err != nil {
		t.Fatalf("second AddItemsToCharacter (post-save): %v", err)
	}

	if slot.Storage.NextEquipIndex != equipBeforeSave {
		t.Errorf("Storage.NextEquipIndex: got %d, want unchanged %d — the Database Add series must not survive a successful Save",
			slot.Storage.NextEquipIndex, equipBeforeSave)
	}
}

// TestAddItemsToCharacter_DatabaseAddSessionDoesNotSurviveReload is the second
// session-boundary regression: reloading a save (installLoadedSave — the same
// commit path SelectAndOpenSave/LoadSaveFromPath use) must end any open
// Database Add series for every character, even one whose Storage the OLD
// save had genuinely empty. The reloaded save's slot 0 Storage is
// pre-populated (non-empty at the start of the new session), so if the old
// session context wrongly survived, this add would incorrectly apply the
// T310/T330 rule (NextEquipIndex advancing by 1) instead of the
// populated-Storage policy (NextEquipIndex left untouched).
func TestAddItemsToCharacter_DatabaseAddSessionDoesNotSurviveReload(t *testing.T) {
	app, _ := emptyStorageDatabaseAddApp(t)

	if _, err := app.AddItemsToCharacter(0, []uint32{databaseAddTalismanID}, 0, 0, 0, 0, 0, 1); err != nil {
		t.Fatalf("AddItemsToCharacter on save A: %v", err)
	}
	if !app.storageAddSessions[0] {
		t.Fatal("precondition: save A's Database Add series was not established")
	}

	reloaded, _ := emptyStorageDatabaseAddApp(t)
	reloadedSlot := &reloaded.save.Slots[0]
	// Pre-populate the reloaded save's Storage so it is non-empty at the start
	// of the new session — the exact case that must never be treated as a
	// T310 empty-init.
	if _, err := reloaded.AddItemsToCharacter(0, []uint32{databaseAddTalismanID}, 0, 0, 0, 0, 0, 1); err != nil {
		t.Fatalf("seeding save B's Storage: %v", err)
	}
	reloaded.storageAddSessions[0] = false // seeding call above must not itself count as an open session
	equipBeforeReload := reloadedSlot.Storage.NextEquipIndex

	app.saveMu.Lock()
	app.installLoadedSave(reloaded.save, "")
	app.saveMu.Unlock()

	if app.storageAddSessions[0] {
		t.Fatal("installLoadedSave did not reset storageAddSessions[0]")
	}

	if _, err := app.AddItemsToCharacter(0, []uint32{databaseAddTalismanID}, 0, 0, 0, 0, 0, 1); err != nil {
		t.Fatalf("AddItemsToCharacter after reload: %v", err)
	}

	slot := &app.save.Slots[0]
	if slot.Storage.NextEquipIndex != equipBeforeReload {
		t.Errorf("Storage.NextEquipIndex: got %d, want unchanged %d — a reload must not let the old save's Database Add series apply to the new save",
			slot.Storage.NextEquipIndex, equipBeforeReload)
	}
}

// TestCloseSave_ResetsStorageAddSessions is the CloseSave lifecycle
// regression: the doc comment on App.storageAddSessions and on
// clearAllStorageAddSessions both claim CloseSave resets the series, but
// CloseSave never called clearAllStorageAddSessions. After a successful
// direct Database Add to an empty Storage establishes storageAddSessions[0],
// CloseSave must zero the whole array.
func TestCloseSave_ResetsStorageAddSessions(t *testing.T) {
	app, _ := emptyStorageDatabaseAddApp(t)

	if _, err := app.AddItemsToCharacter(0, []uint32{databaseAddTalismanID}, 0, 0, 0, 0, 0, 1); err != nil {
		t.Fatalf("AddItemsToCharacter: %v", err)
	}
	if !app.storageAddSessions[0] {
		t.Fatal("precondition: Database Add series was not established")
	}

	if err := app.CloseSave(); err != nil {
		t.Fatalf("CloseSave: %v", err)
	}

	if app.storageAddSessions != ([maxCharacters]bool{}) {
		t.Errorf("storageAddSessions after CloseSave: got %v, want all-false", app.storageAddSessions)
	}
}

// TestAddItemsToCharacter_InventoryOnlyAddDoesNotOpenStorageSession is the
// T350 scope regression: a direct Database Add that only deposits into
// Inventory (StorageQty==0 for every effective entry) must not open a T350
// series, even though it succeeds and even though Storage is still genuinely
// empty. Only a call that actually lands at least one item in Storage may
// set storageAddSessions[charIdx]. The first real Storage add afterward must
// still see the untouched T310 empty signature and get the T310 init
// contract (Index=2, NextEquipIndex=128, NextAcquisitionSortId=2) — not a
// wrongly-continued series and not a wrongly-reset one either.
func TestAddItemsToCharacter_InventoryOnlyAddDoesNotOpenStorageSession(t *testing.T) {
	app, storageNextAcqOff := emptyStorageDatabaseAddApp(t)

	invResult, err := app.AddItemsToCharacter(0, []uint32{databaseAddTalismanID}, 0, 0, 0, 0, 1, 0)
	if err != nil {
		t.Fatalf("Inventory-only AddItemsToCharacter: %v", err)
	}
	if invResult.Added != 1 {
		t.Fatalf("Inventory-only add: Added=%d, want 1 (CapHit=%q)", invResult.Added, invResult.CapHit)
	}
	if app.storageAddSessions[0] {
		t.Fatal("Inventory-only add must not set storageAddSessions[0]")
	}

	if _, err := app.AddItemsToCharacter(0, []uint32{databaseAddTalismanID}, 0, 0, 0, 0, 0, 1); err != nil {
		t.Fatalf("first Storage AddItemsToCharacter: %v", err)
	}

	slot := &app.save.Slots[0]
	if len(slot.Storage.CommonItems) != 1 || slot.Storage.CommonItems[0].Index != 2 {
		t.Fatalf("Storage.CommonItems after first Storage add: got %+v, want one record with Index=2", slot.Storage.CommonItems)
	}
	if slot.Storage.NextEquipIndex != 128 {
		t.Errorf("Storage.NextEquipIndex: got %d, want 128 (T310 init)", slot.Storage.NextEquipIndex)
	}
	if slot.Storage.NextAcquisitionSortId != 2 {
		t.Errorf("Storage.NextAcquisitionSortId: got %d, want 2 (T310 init)", slot.Storage.NextAcquisitionSortId)
	}
	rawAcq := binary.LittleEndian.Uint32(slot.Data[storageNextAcqOff:])
	if rawAcq != 2 {
		t.Errorf("binary NextAcquisitionSortId: got %d, want 2", rawAcq)
	}
}
