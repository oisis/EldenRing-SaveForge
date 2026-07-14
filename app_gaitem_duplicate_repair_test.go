package main

import (
	"encoding/binary"
	"reflect"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
)

// dedupInMemoryApp builds an App whose slot 0 holds a synthetic physical GaItem
// duplicate pair (one weapon handle, two records with different ItemIDs, one
// inventory reference). It needs no serialized save because the App analysis path
// and every could-not-start guard read only in-memory GaItems/containers. It is
// therefore a lightweight, always-executable fixture for the analysis and guard
// tests. The execute-success and undo path, which rebuilds bytes, uses
// readyDedupApp's synthetic serializable slot instead.
func dedupInMemoryApp(t *testing.T) (*App, int, uint32) {
	t.Helper()
	const handle = uint32(core.ItemTypeWeapon | 0x0102)

	app := NewApp()
	app.save = &core.SaveFile{}
	slot := &app.save.Slots[0]
	slot.Data = make([]byte, core.SlotSize)
	slot.Version = 1
	slot.MagicOffset = 1000
	slot.InventoryEnd = core.GaItemsStart
	slot.PlayerDataOffset = 1000
	slot.FaceDataOffset = 2000
	slot.StorageBoxOffset = 2000
	slot.GaItemDataOffset = 0x8000
	slot.EquipItemsIDOffset = 0x4000 // readable equipment section (all zero == nothing equipped)
	slot.SectionMap = []core.SectionRange{{Name: "all", Start: 0, End: core.SlotSize}}
	slot.NextAoWIndex = 0
	slot.NextArmamentIndex = 4
	slot.NextGaItemHandle = 0x0200
	slot.PartGaItemHandle = 0x80
	slot.GaItems = []core.GaItemFull{
		{},
		{Handle: handle, ItemID: 0x000F4240, AoWGaItemHandle: core.NoCustomAoWHandle},
		{Handle: handle, ItemID: 0x000F4241, AoWGaItemHandle: core.NoCustomAoWHandle},
		{},
	}
	slot.GaMap = map[uint32]uint32{handle: 0x000F4241}
	slot.Inventory.CommonItems = []core.InventoryItem{{GaItemHandle: handle, Quantity: 1, Index: 100}}
	app.saveGeneration = 1

	if a := core.AnalyzeGaItemDuplicate(slot, handle); !a.Repairable {
		t.Fatalf("synthetic dedup slot not repairable: %s: %s", a.RefusalCode, a.RefusalMsg)
	}
	return app, 0, handle
}

func TestAnalyzeGaItemDuplicate_App_ReadyIssuesTokenAndCandidates(t *testing.T) {
	app, charIdx, handle := dedupInMemoryApp(t)
	before := core.CloneSlot(&app.save.Slots[charIdx])

	analysis, err := app.AnalyzeGaItemDuplicate(charIdx, handle)
	if err != nil {
		t.Fatalf("AnalyzeGaItemDuplicate: %v", err)
	}
	if analysis.Outcome != "ready" || analysis.AnalysisToken == "" {
		t.Fatalf("analysis=%+v, want ready with token", analysis)
	}
	if len(analysis.Candidates) != 2 || analysis.Candidates[0].ItemID == analysis.Candidates[1].ItemID {
		t.Fatalf("candidates=%+v, want two with distinct ItemIDs", analysis.Candidates)
	}
	if !reflect.DeepEqual(&app.save.Slots[charIdx], before) {
		t.Fatal("analysis mutated the active slot")
	}
}

func TestAnalyzeGaItemDuplicate_App_ActiveWorkspaceUnavailable(t *testing.T) {
	app, charIdx, handle := dedupInMemoryApp(t)
	app.editSessionByChar[charIdx] = "active-workspace"

	analysis, err := app.AnalyzeGaItemDuplicate(charIdx, handle)
	if err != nil {
		t.Fatalf("AnalyzeGaItemDuplicate: %v", err)
	}
	if analysis.Outcome != "unavailable" || analysis.Failure == nil || analysis.Failure.Code != "inventory_edit_session_active" {
		t.Fatalf("analysis=%+v, want workspace unavailable", analysis)
	}
	if analysis.AnalysisToken != "" {
		t.Error("unavailable analysis must not issue a token")
	}
}

func TestExecuteGaItemDuplicateRepair_App_StaleTokenDoesNotMutate(t *testing.T) {
	app, charIdx, handle := dedupInMemoryApp(t)
	analysis, err := app.AnalyzeGaItemDuplicate(charIdx, handle)
	if err != nil {
		t.Fatalf("AnalyzeGaItemDuplicate: %v", err)
	}

	app.save.Slots[charIdx].Data[0] ^= 0x01 // change the slot after analysis
	changed := core.CloneSlot(&app.save.Slots[charIdx])

	result, err := app.ExecuteGaItemDuplicateRepair(GaItemDuplicateExecuteRequest{
		CharacterIndex: charIdx, Handle: handle, KeepIndex: analysis.Candidates[0].Index, AnalysisToken: analysis.AnalysisToken,
	})
	if err != nil {
		t.Fatalf("ExecuteGaItemDuplicateRepair: %v", err)
	}
	if result.Outcome != "could_not_start" || result.Failure == nil || result.Failure.Code != "analysis_stale" {
		t.Fatalf("result=%+v, want stale no-start", result)
	}
	if !reflect.DeepEqual(&app.save.Slots[charIdx], changed) {
		t.Fatal("stale execution changed the active slot")
	}
	if depth := app.GetUndoDepth(charIdx); depth != 0 {
		t.Fatalf("undo depth=%d, want 0", depth)
	}
}

func TestExecuteGaItemDuplicateRepair_App_InvalidKeepIndexRefuses(t *testing.T) {
	app, charIdx, handle := dedupInMemoryApp(t)
	analysis, err := app.AnalyzeGaItemDuplicate(charIdx, handle)
	if err != nil {
		t.Fatalf("AnalyzeGaItemDuplicate: %v", err)
	}
	before := core.CloneSlot(&app.save.Slots[charIdx])

	result, err := app.ExecuteGaItemDuplicateRepair(GaItemDuplicateExecuteRequest{
		CharacterIndex: charIdx, Handle: handle, KeepIndex: 4242, AnalysisToken: analysis.AnalysisToken,
	})
	if err != nil {
		t.Fatalf("ExecuteGaItemDuplicateRepair: %v", err)
	}
	if result.Outcome != "could_not_start" || result.Failure == nil || result.Failure.Code != "invalid_keep_index" {
		t.Fatalf("result=%+v, want invalid keep index no-start", result)
	}
	if !reflect.DeepEqual(&app.save.Slots[charIdx], before) {
		t.Fatal("invalid keep index changed the active slot")
	}
	if depth := app.GetUndoDepth(charIdx); depth != 0 {
		t.Fatalf("undo depth=%d, want 0", depth)
	}
}

func TestExecuteGaItemDuplicateRepair_App_MissingTokenRefuses(t *testing.T) {
	app, charIdx, handle := dedupInMemoryApp(t)
	before := core.CloneSlot(&app.save.Slots[charIdx])

	result, err := app.ExecuteGaItemDuplicateRepair(GaItemDuplicateExecuteRequest{
		CharacterIndex: charIdx, Handle: handle, KeepIndex: 1, AnalysisToken: "",
	})
	if err != nil {
		t.Fatalf("ExecuteGaItemDuplicateRepair: %v", err)
	}
	if result.Outcome != "could_not_start" || result.Failure == nil || result.Failure.Code != "analysis_stale" {
		t.Fatalf("result=%+v, want missing-token no-start", result)
	}
	if !reflect.DeepEqual(&app.save.Slots[charIdx], before) {
		t.Fatal("missing token changed the active slot")
	}
}

// TestGaItemDuplicateRepair_App_AnalyzeExecuteAndUndo exercises the full success
// path on an always-executable synthetic save: it deduplicates one physical
// duplicate through the App with an explicit keep choice, verifies exactly one
// record remains and GaMap resolves it, confirms the container reference is
// unchanged and the undo depth becomes 1, then reverts to the complete original.
func TestGaItemDuplicateRepair_App_AnalyzeExecuteAndUndo(t *testing.T) {
	app, charIdx, handle, keepIndex := readyDedupApp(t)
	before := core.CloneSlot(&app.save.Slots[charIdx])
	keptItemID := before.GaItems[keepIndex].ItemID
	invBefore := append([]core.InventoryItem(nil), before.Inventory.CommonItems...)

	analysis, err := app.AnalyzeGaItemDuplicate(charIdx, handle)
	if err != nil {
		t.Fatalf("AnalyzeGaItemDuplicate: %v", err)
	}
	if analysis.Outcome != "ready" || analysis.AnalysisToken == "" {
		t.Fatalf("analysis=%+v, want ready with token", analysis)
	}
	if !reflect.DeepEqual(&app.save.Slots[charIdx], before) {
		t.Fatal("analysis mutated the active slot")
	}

	result, err := app.ExecuteGaItemDuplicateRepair(GaItemDuplicateExecuteRequest{
		CharacterIndex: charIdx, Handle: handle, KeepIndex: keepIndex, AnalysisToken: analysis.AnalysisToken,
	})
	if err != nil {
		t.Fatalf("ExecuteGaItemDuplicateRepair: %v", err)
	}
	if result.Outcome != "success" {
		t.Fatalf("result=%+v, want success", result)
	}
	if result.KeptIndex != keepIndex || result.RemovedIndex == keepIndex {
		t.Fatalf("kept=%d removed=%d, want kept=%d and a distinct removed index", result.KeptIndex, result.RemovedIndex, keepIndex)
	}

	// exactly one physical record remains for the handle, and preflight is clean.
	slot := &app.save.Slots[charIdx]
	remaining := 0
	for i := range slot.GaItems {
		if !slot.GaItems[i].IsEmpty() && slot.GaItems[i].Handle == handle {
			remaining++
		}
	}
	if remaining != 1 {
		t.Fatalf("remaining records for handle = %d, want 1", remaining)
	}
	if slot.GaItems[keepIndex].ItemID != keptItemID {
		t.Fatalf("kept record ItemID = 0x%08X, want 0x%08X", slot.GaItems[keepIndex].ItemID, keptItemID)
	}
	if got := slot.GaMap[handle]; got != keptItemID {
		t.Fatalf("GaMap[handle] = 0x%08X, want kept ItemID 0x%08X", got, keptItemID)
	}
	if len(slot.Inventory.CommonItems) != len(invBefore) {
		t.Fatalf("container references changed: %+v -> %+v", invBefore, slot.Inventory.CommonItems)
	}
	for i := range invBefore {
		if slot.Inventory.CommonItems[i] != invBefore[i] {
			t.Fatalf("container reference %d changed: %+v -> %+v", i, invBefore[i], slot.Inventory.CommonItems[i])
		}
	}
	if pf := core.PreflightGaItemRepack(slot); len(pf.Blockers) != 0 {
		t.Fatalf("preflight still blocked after dedup: %+v", pf.Blockers)
	}
	if depth := app.GetUndoDepth(charIdx); depth != 1 {
		t.Fatalf("undo depth=%d, want 1", depth)
	}

	if err := app.RevertSlot(charIdx); err != nil {
		t.Fatalf("RevertSlot: %v", err)
	}
	if !reflect.DeepEqual(&app.save.Slots[charIdx], before) {
		t.Fatal("undo did not restore the complete pre-repair slot")
	}
}

// readyDedupApp builds an always-executable App whose slot 0 holds a fully
// serializable physical GaItem duplicate pair (one weapon handle, two records
// with distinct ItemIDs), a single inventory reference to that handle, one
// storage armor record, and a readable equipment section. Unlike the analysis-
// only fixture, this slot round-trips through RebuildSlotFull, so it drives the
// real App execute-success path without loading any user save.
//
// It builds the byte layout with the same formulas as the core repack fixtures,
// using only exported core API: a first Read() computes the dynamic offsets, the
// container/equipment bytes are written at those offsets, and a second Read()
// reparses the coherent slot. It returns the lower-index candidate as the keep
// choice.
func readyDedupApp(t *testing.T) (*App, int, uint32, int) {
	t.Helper()
	const (
		handle     = uint32(core.ItemTypeWeapon | 0x0102)
		armorH     = uint32(core.ItemTypeArmor | 0x0200)
		lowItemID  = uint32(0x000F4240)
		highItemID = uint32(0x000F4241)
		armorItem  = uint32(0x10000001)
	)

	gaItems := make([]core.GaItemFull, core.GaItemCountNew)
	copy(gaItems, []core.GaItemFull{
		{Handle: armorH, ItemID: armorItem, Unk2: 20, Unk3: 21},
		{Handle: handle, ItemID: lowItemID, Unk2: 10, Unk3: 11, AoWGaItemHandle: core.NoCustomAoWHandle, Unk5: 1},
		{Handle: handle, ItemID: highItemID, Unk2: 12, Unk3: 13, AoWGaItemHandle: core.NoCustomAoWHandle, Unk5: 2},
	})

	gaBytes := 0
	for i := range gaItems {
		gaBytes += gaItems[i].ByteSize()
	}
	magicOffset := core.GaItemsStart + gaBytes + core.DynPlayerData - 1

	data := make([]byte, core.SlotSize)
	binary.LittleEndian.PutUint32(data, uint32(core.GaItemVersionBreak+1))
	copy(data[magicOffset:], core.MagicPattern)
	pos := core.GaItemsStart
	for i := range gaItems {
		pos += gaItems[i].Serialize(data[pos:])
	}
	// One inventory reference to the duplicated handle (common_count header at -4).
	invStart := magicOffset + core.InvStartFromMagic
	binary.LittleEndian.PutUint32(data[invStart-core.InvKeyCountHeader:], 1)
	binary.LittleEndian.PutUint32(data[invStart:], handle)
	binary.LittleEndian.PutUint32(data[invStart+4:], 7)
	binary.LittleEndian.PutUint32(data[invStart+8:], 100)

	app := NewApp()
	app.save = &core.SaveFile{}
	app.saveGeneration = 1
	slot := &app.save.Slots[0]

	// First parse computes the dynamic offsets (storage, equipment) from the layout.
	if err := slot.Read(core.NewReader(data), ""); err != nil {
		t.Fatalf("initial parse: %v", err)
	}
	storageStart := slot.StorageBoxOffset + core.StorageHeaderSkip
	binary.LittleEndian.PutUint32(slot.Data[slot.StorageBoxOffset:], 1)
	binary.LittleEndian.PutUint32(slot.Data[storageStart:], armorH)
	binary.LittleEndian.PutUint32(slot.Data[storageStart+4:], 1)
	binary.LittleEndian.PutUint32(slot.Data[storageStart+8:], 200)
	// Non-zero, readable equipment section whose small byte values cannot collide
	// with the candidate ItemID signatures.
	for i := 0; i < core.ChrAsmEquipmentSize; i++ {
		slot.Data[slot.EquipItemsIDOffset+i] = byte(i + 1)
	}
	// One unlocked region so the parsed slice is non-nil and round-trips through
	// SnapshotSlot/CloneSlot (an empty region list would clone back as nil and
	// break the reflect.DeepEqual undo/revert contract).
	binary.LittleEndian.PutUint32(slot.Data[slot.UnlockedRegionsOffset:], 1)
	binary.LittleEndian.PutUint32(slot.Data[slot.UnlockedRegionsOffset+4:], 0x2A)
	if err := slot.Read(core.NewReader(slot.Data), ""); err != nil {
		t.Fatalf("reparse: %v", err)
	}

	analysis := core.AnalyzeGaItemDuplicate(slot, handle)
	if !analysis.Repairable {
		t.Fatalf("synthetic dedup slot not repairable: %s: %s", analysis.RefusalCode, analysis.RefusalMsg)
	}
	return app, 0, handle, analysis.Candidates[0].Index
}
