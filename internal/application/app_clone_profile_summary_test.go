package application

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
)

// These tests cover the confirmed CloneSlot defect reported against v1.8.0: the
// clone appeared greyed out or incorrectly rendered on the character-select
// screen because CloneSlot copied only the parsed ProfileSummary (name + level)
// and left the rest of the 0x24C record — the opaque face/equipment snapshot the
// menu renders — as whatever bytes the destination already carried.
//
// Whether copying the full record also resolves the reported freeze after
// selecting the ORIGINAL character is deliberately NOT asserted here: that needs
// a controlled in-game test.

// profileSummaryRecord returns the full 0x24C UserData10 record for slot idx.
func profileSummaryRecord(t *testing.T, save *core.SaveFile, idx int) []byte {
	t.Helper()
	rec := core.ProfileSummaryRegion(save.UserData10.Data, idx)
	if rec == nil {
		t.Fatalf("ProfileSummaryRegion(slot %d) = nil, want a %d-byte record", idx, core.ProfileSummaryStride)
	}
	return rec
}

// fillProfileSummaryOpaque writes a recognizable pattern over the opaque part of
// slot idx (everything after name+level), mimicking a real face/equipment
// snapshot.
func fillProfileSummaryOpaque(save *core.SaveFile, idx int, b byte) {
	off := core.ProfileSummaryOffset + idx*core.ProfileSummaryStride
	for i := 0x26; i < core.ProfileSummaryStride; i++ {
		save.UserData10.Data[off+i] = b
	}
}

// TestCloneSlotCopiesFullProfileSummaryRecord is the core regression: the
// destination must receive the SOURCE opaque snapshot, not keep its own.
func TestCloneSlotCopiesFullProfileSummaryRecord(t *testing.T) {
	app := newSlotStateTestApp()
	app.save.ActiveSlots[2] = true
	app.save.Slots[2].Player.CharacterName = testCharacterName("ali")
	app.save.ProfileSummaries[2] = core.ProfileSummary{CharacterName: testCharacterName("ali"), Level: 142}
	app.save.ProfileSummaries[2].Serialize(app.save.UserData10.Data, core.ProfileSummaryOffset+2*core.ProfileSummaryStride)
	fillProfileSummaryOpaque(app.save, 2, 0xA7)

	// The destination is an unused slot that nevertheless carries non-zero
	// opaque bytes — exactly the class of state in the reported save, where the
	// cloned slot 0 record stayed byte-identical to the untouched slots 3 and 4.
	fillProfileSummaryOpaque(app.save, 0, 0x5C)
	before0 := profileSummaryRecord(t, app.save, 0)
	srcBefore := profileSummaryRecord(t, app.save, 2)

	if err := app.CloneSlot(2, 0); err != nil {
		t.Fatalf("CloneSlot: %v", err)
	}

	got := profileSummaryRecord(t, app.save, 0)
	if bytes.Equal(got[0x26:], before0[0x26:]) {
		t.Fatalf("destination opaque ProfileSummary unchanged after clone — greyed-out-clone defect")
	}
	if !bytes.Equal(got[0x26:], srcBefore[0x26:]) {
		t.Fatalf("destination opaque ProfileSummary does not match the source record")
	}

	// Name is the unique clone name in the parsed slot, the parsed summary AND
	// the raw record; level is inherited from the source.
	const wantName = "ali 2"
	if name := core.UTF16ToString(app.save.Slots[0].Player.CharacterName[:]); name != wantName {
		t.Fatalf("cloned slot name = %q, want %q", name, wantName)
	}
	if name := core.UTF16ToString(app.save.ProfileSummaries[0].CharacterName[:]); name != wantName {
		t.Fatalf("cloned summary name = %q, want %q", name, wantName)
	}
	var rawName [16]uint16
	for i := range rawName {
		rawName[i] = uint16(got[i*2]) | uint16(got[i*2+1])<<8
	}
	if name := core.UTF16ToString(rawName[:]); name != wantName {
		t.Fatalf("cloned raw record name = %q, want %q", name, wantName)
	}
	if app.save.ProfileSummaries[0].Level != 142 {
		t.Fatalf("cloned summary level = %d, want 142", app.save.ProfileSummaries[0].Level)
	}
	if lvl := uint32(got[0x22]) | uint32(got[0x23])<<8 | uint32(got[0x24])<<16 | uint32(got[0x25])<<24; lvl != 142 {
		t.Fatalf("cloned raw record level = %d, want 142", lvl)
	}

	// The source must be untouched, record and parsed summary alike.
	if srcAfter := profileSummaryRecord(t, app.save, 2); !bytes.Equal(srcAfter, srcBefore) {
		t.Fatalf("source ProfileSummary record mutated by CloneSlot")
	}
	if name := core.UTF16ToString(app.save.ProfileSummaries[2].CharacterName[:]); name != "ali" {
		t.Fatalf("source summary name = %q, want ali", name)
	}
	if name := core.UTF16ToString(app.save.Slots[2].Player.CharacterName[:]); name != "ali" {
		t.Fatalf("source slot name = %q, want ali", name)
	}
}

// TestCloneSlotDoesNotAliasSourceMutableState proves the destination owns every
// mutable field, not just Data and GaMap (the only two the old manual copy
// handled).
func TestCloneSlotDoesNotAliasSourceMutableState(t *testing.T) {
	app := newSlotStateTestApp()
	src := &app.save.Slots[3]
	app.save.ActiveSlots[3] = true
	src.Player.CharacterName = testCharacterName("Source")
	src.GaMap[0x1000] = 42
	src.GaItems = []core.GaItemFull{{Handle: 0x80000001}}
	src.Inventory.CommonItems = []core.InventoryItem{{GaItemHandle: 0xB0000001, Quantity: 1}}
	src.Storage.CommonItems = []core.InventoryItem{{GaItemHandle: 0xB0000002, Quantity: 2}}
	src.UnlockedRegions = []uint32{7}
	src.SectionMap = []core.SectionRange{{Name: "src", Start: 0, End: 1}}
	src.Warnings = []string{"src warning"}
	src.Data[0x10] = 0x11

	if err := app.CloneSlot(3, 4); err != nil {
		t.Fatalf("CloneSlot: %v", err)
	}

	dst := &app.save.Slots[4]
	dst.Data[0x10] = 0x99
	dst.GaMap[0x1000] = 99
	dst.GaItems[0].Handle = 0x8000FFFF
	dst.Inventory.CommonItems[0].Quantity = 99
	dst.Storage.CommonItems[0].Quantity = 99
	dst.UnlockedRegions[0] = 99
	dst.SectionMap[0].Name = "dst"
	dst.Warnings[0] = "dst warning"

	if src.Data[0x10] != 0x11 {
		t.Fatalf("source Data aliased by clone")
	}
	if src.GaMap[0x1000] != 42 {
		t.Fatalf("source GaMap aliased by clone")
	}
	if src.GaItems[0].Handle != 0x80000001 {
		t.Fatalf("source GaItems aliased by clone")
	}
	if src.Inventory.CommonItems[0].Quantity != 1 {
		t.Fatalf("source Inventory aliased by clone")
	}
	if src.Storage.CommonItems[0].Quantity != 2 {
		t.Fatalf("source Storage aliased by clone")
	}
	if src.UnlockedRegions[0] != 7 {
		t.Fatalf("source UnlockedRegions aliased by clone")
	}
	if src.SectionMap[0].Name != "src" {
		t.Fatalf("source SectionMap aliased by clone")
	}
	if src.Warnings[0] != "src warning" {
		t.Fatalf("source Warnings aliased by clone")
	}
}

// TestRevertSlotRestoresFullProfileSummaryRecord covers undo for every operation
// that replaces or clears the 0x24C record: the previous record must come back
// byte-for-byte, together with the slot data and the active flag.
func TestRevertSlotRestoresFullProfileSummaryRecord(t *testing.T) {
	setup := func() *App {
		app := newSlotStateTestApp()
		app.save.ActiveSlots[2] = true
		app.save.Slots[2].Player.CharacterName = testCharacterName("Source")
		app.save.ProfileSummaries[2] = core.ProfileSummary{CharacterName: testCharacterName("Source"), Level: 60}
		app.save.ProfileSummaries[2].Serialize(app.save.UserData10.Data, core.ProfileSummaryOffset+2*core.ProfileSummaryStride)
		fillProfileSummaryOpaque(app.save, 2, 0xA7)
		return app
	}

	t.Run("clone", func(t *testing.T) {
		app := setup()
		fillProfileSummaryOpaque(app.save, 0, 0x5C)
		before := profileSummaryRecord(t, app.save, 0)
		beforeData := append([]byte(nil), app.save.Slots[0].Data...)

		if err := app.CloneSlot(2, 0); err != nil {
			t.Fatalf("CloneSlot: %v", err)
		}
		if err := app.RevertSlot(0); err != nil {
			t.Fatalf("RevertSlot: %v", err)
		}
		if got := profileSummaryRecord(t, app.save, 0); !bytes.Equal(got, before) {
			t.Fatalf("ProfileSummary record not restored byte-for-byte after undo of clone")
		}
		if !bytes.Equal(app.save.Slots[0].Data, beforeData) {
			t.Fatalf("slot data not restored after undo of clone")
		}
		if app.save.ActiveSlots[0] {
			t.Fatalf("destination still active after undo of clone")
		}
	})

	t.Run("delete", func(t *testing.T) {
		app := setup()
		before := profileSummaryRecord(t, app.save, 2)

		if err := app.DeleteSlot(2); err != nil {
			t.Fatalf("DeleteSlot: %v", err)
		}
		if got := profileSummaryRecord(t, app.save, 2); !bytes.Equal(got, make([]byte, core.ProfileSummaryStride)) {
			t.Fatalf("DeleteSlot left a non-zero ProfileSummary record")
		}
		if err := app.RevertSlot(2); err != nil {
			t.Fatalf("RevertSlot: %v", err)
		}
		if got := profileSummaryRecord(t, app.save, 2); !bytes.Equal(got, before) {
			t.Fatalf("ProfileSummary record not restored byte-for-byte after undo of delete")
		}
		if !app.save.ActiveSlots[2] {
			t.Fatalf("slot not re-activated after undo of delete")
		}
	})

	t.Run("clean residual", func(t *testing.T) {
		app := setup()
		app.save.ActiveSlots[2] = false // in-game deletion leaves a phantom
		before := profileSummaryRecord(t, app.save, 2)

		if err := app.CleanResidualSlot(2); err != nil {
			t.Fatalf("CleanResidualSlot: %v", err)
		}
		if err := app.RevertSlot(2); err != nil {
			t.Fatalf("RevertSlot: %v", err)
		}
		if got := profileSummaryRecord(t, app.save, 2); !bytes.Equal(got, before) {
			t.Fatalf("ProfileSummary record not restored byte-for-byte after undo of residual cleanup")
		}
		if app.save.ActiveSlots[2] {
			t.Fatalf("residual slot became active after undo")
		}
	})
}

// TestCloneSlotRejectsActiveDestinationWithoutMutation is the adjacent negative
// case: a rejected clone must leave both slots and both records untouched.
func TestCloneSlotRejectsActiveDestinationWithoutMutation(t *testing.T) {
	app := newSlotStateTestApp()
	app.save.ActiveSlots[2] = true
	app.save.Slots[2].Player.CharacterName = testCharacterName("Source")
	fillProfileSummaryOpaque(app.save, 2, 0xA7)
	app.save.ActiveSlots[0] = true
	app.save.Slots[0].Player.CharacterName = testCharacterName("Occupied")
	app.save.ProfileSummaries[0] = core.ProfileSummary{CharacterName: testCharacterName("Occupied"), Level: 12}
	fillProfileSummaryOpaque(app.save, 0, 0x5C)

	destBefore := profileSummaryRecord(t, app.save, 0)
	srcBefore := profileSummaryRecord(t, app.save, 2)
	undoBefore := app.GetUndoDepth(0)

	if err := app.CloneSlot(2, 0); err == nil {
		t.Fatalf("CloneSlot into active destination succeeded, want error")
	}
	if got := profileSummaryRecord(t, app.save, 0); !bytes.Equal(got, destBefore) {
		t.Fatalf("rejected clone mutated the destination ProfileSummary record")
	}
	if got := profileSummaryRecord(t, app.save, 2); !bytes.Equal(got, srcBefore) {
		t.Fatalf("rejected clone mutated the source ProfileSummary record")
	}
	if name := core.UTF16ToString(app.save.Slots[0].Player.CharacterName[:]); name != "Occupied" {
		t.Fatalf("destination slot name = %q, want Occupied", name)
	}
	if app.GetUndoDepth(0) != undoBefore {
		t.Fatalf("rejected clone pushed an undo entry")
	}
}

// TestCloneSlotFullProfileSummarySurvivesSaveReload proves the copied record
// reaches the file and comes back after a real serialize → LoadSave round trip,
// on a throwaway copy only. The user save under tmp/ is never written.
//
// SUPPLEMENTAL: this test depends on a gitignored fixture under tmp/save/ and is
// skipped on a clean checkout. The contract itself is protected by the
// deterministic, fixture-free tests above; this one only adds local end-to-end
// confidence on real save data.
func TestCloneSlotFullProfileSummarySurvivesSaveReload(t *testing.T) {
	var fixture string
	for _, p := range []string{
		"tmp/save/ER0000-out.sl2",
		"tmp/save/ER0000-oisis-117.sl2",
		"tmp/save/ER0000-oisisk_pl-vanilla.sl2",
	} {
		if _, err := os.Stat(p); err == nil {
			fixture = p
			break
		}
	}
	if fixture == "" {
		t.Skip("SKIP(real fixture): no PC save present under tmp/save/ (gitignored)")
	}

	save, err := core.LoadSave(fixture)
	if err != nil {
		t.Fatalf("LoadSave: %v", err)
	}
	app := NewApp()
	app.save = save

	srcIdx, destIdx := -1, -1
	for i := 0; i < 10; i++ {
		if srcIdx < 0 && save.ActiveSlots[i] && core.UTF16ToString(save.Slots[i].Player.CharacterName[:]) != "" {
			srcIdx = i
			continue
		}
		if destIdx < 0 && !save.ActiveSlots[i] && !save.SlotHasResidualData(i) {
			destIdx = i
		}
	}
	if srcIdx < 0 || destIdx < 0 {
		t.Skipf("SKIP(real fixture): %s has no usable source/destination slot pair", fixture)
	}

	srcRecord := profileSummaryRecord(t, save, srcIdx)
	if err := app.CloneSlot(srcIdx, destIdx); err != nil {
		t.Fatalf("CloneSlot: %v", err)
	}
	cloneName := core.UTF16ToString(save.Slots[destIdx].Player.CharacterName[:])

	out := filepath.Join(t.TempDir(), "ER0000.sl2")
	if err := save.SaveFile(out); err != nil {
		t.Fatalf("SaveFile: %v", err)
	}
	reloaded, err := core.LoadSave(out)
	if err != nil {
		t.Fatalf("LoadSave(round trip): %v", err)
	}

	if !reloaded.ActiveSlots[destIdx] {
		t.Fatalf("cloned slot %d inactive after reload", destIdx)
	}
	if got := core.UTF16ToString(reloaded.ProfileSummaries[destIdx].CharacterName[:]); got != cloneName {
		t.Fatalf("reloaded summary name = %q, want %q", got, cloneName)
	}
	if got := reloaded.ProfileSummaries[destIdx].Level; got != save.ProfileSummaries[srcIdx].Level {
		t.Fatalf("reloaded summary level = %d, want %d", got, save.ProfileSummaries[srcIdx].Level)
	}
	gotRecord := profileSummaryRecord(t, reloaded, destIdx)
	if !bytes.Equal(gotRecord[0x26:], srcRecord[0x26:]) {
		t.Fatalf("reloaded opaque ProfileSummary does not match the source record")
	}
	if got := profileSummaryRecord(t, reloaded, srcIdx); !bytes.Equal(got, srcRecord) {
		t.Fatalf("source ProfileSummary record changed across the round trip")
	}
}

// --- fail-closed contract -------------------------------------------------
//
// A clone that cannot carry the FULL 0x24C ProfileSummary record is the very
// defect being fixed, so CloneSlot must reject it instead of publishing a
// half-cloned slot, and RevertSlot must keep its snapshot rather than leave a
// partially restored one.

// truncateUserData10 shrinks UserData10 so that slot cutoffIdx no longer has room
// for a complete ProfileSummary record.
func truncateUserData10(save *core.SaveFile, cutoffIdx int) {
	save.UserData10.Data = save.UserData10.Data[:core.ProfileSummaryOffset+cutoffIdx*core.ProfileSummaryStride+1]
}

func TestCloneSlotFailsClosedWhenProfileSummaryRecordIsUnavailable(t *testing.T) {
	cases := []struct {
		name            string
		srcIdx, destIdx int
		breakUserData10 func(save *core.SaveFile)
	}{
		{
			name:   "destination record does not fit",
			srcIdx: 2, destIdx: 5,
			breakUserData10: func(save *core.SaveFile) { truncateUserData10(save, 5) },
		},
		{
			name:   "source record does not fit",
			srcIdx: 8, destIdx: 1,
			breakUserData10: func(save *core.SaveFile) { truncateUserData10(save, 8) },
		},
		{
			name:   "no UserData10 at all",
			srcIdx: 2, destIdx: 5,
			breakUserData10: func(save *core.SaveFile) { save.UserData10.Data = nil },
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app := newSlotStateTestApp()
			app.save.ActiveSlots[tc.srcIdx] = true
			app.save.Slots[tc.srcIdx].Player.CharacterName = testCharacterName("Source")
			app.save.ProfileSummaries[tc.srcIdx] = core.ProfileSummary{CharacterName: testCharacterName("Source"), Level: 77}
			app.save.Slots[tc.srcIdx].Data[0x10] = 0x11
			tc.breakUserData10(app.save)

			// A live dedup token for the destination proves the rejected call
			// does not invalidate slot-scoped caches either.
			app.gaItemDedupTokens["token"] = gaItemDedupToken{CharacterIndex: tc.destIdx}

			udBefore := append([]byte(nil), app.save.UserData10.Data...)
			activeBefore := app.save.ActiveSlots
			summariesBefore := app.save.ProfileSummaries
			destDataBefore := append([]byte(nil), app.save.Slots[tc.destIdx].Data...)
			srcDataBefore := append([]byte(nil), app.save.Slots[tc.srcIdx].Data...)
			undoBefore := app.GetUndoDepth(tc.destIdx)
			revisionsBefore := app.slotRevisions

			if err := app.CloneSlot(tc.srcIdx, tc.destIdx); err == nil {
				t.Fatalf("CloneSlot succeeded without a complete ProfileSummary record, want error")
			}

			if !bytes.Equal(app.save.UserData10.Data, udBefore) {
				t.Errorf("rejected clone mutated UserData10")
			}
			if app.save.ActiveSlots != activeBefore {
				t.Errorf("rejected clone mutated ActiveSlots")
			}
			if app.save.ProfileSummaries != summariesBefore {
				t.Errorf("rejected clone mutated ProfileSummaries")
			}
			if !bytes.Equal(app.save.Slots[tc.destIdx].Data, destDataBefore) {
				t.Errorf("rejected clone mutated the destination slot")
			}
			if !bytes.Equal(app.save.Slots[tc.srcIdx].Data, srcDataBefore) {
				t.Errorf("rejected clone mutated the source slot")
			}
			if got := app.GetUndoDepth(tc.destIdx); got != undoBefore {
				t.Errorf("rejected clone changed undo depth: %d, want %d", got, undoBefore)
			}
			if app.slotRevisions != revisionsBefore {
				t.Errorf("rejected clone changed slotRevisions: %v, want %v", app.slotRevisions, revisionsBefore)
			}
			if _, ok := app.gaItemDedupTokens["token"]; !ok {
				t.Errorf("rejected clone invalidated the slot dedup tokens")
			}
		})
	}
}

func TestRevertSlotKeepsSnapshotWhenProfileSummaryCannotBeRestored(t *testing.T) {
	app := newSlotStateTestApp()
	app.save.ActiveSlots[2] = true
	app.save.Slots[2].Player.CharacterName = testCharacterName("Source")
	app.save.ProfileSummaries[2] = core.ProfileSummary{CharacterName: testCharacterName("Source"), Level: 60}
	app.save.ProfileSummaries[2].Serialize(app.save.UserData10.Data, core.ProfileSummaryOffset+2*core.ProfileSummaryStride)
	fillProfileSummaryOpaque(app.save, 2, 0xA7)

	if err := app.CloneSlot(2, 5); err != nil {
		t.Fatalf("CloneSlot: %v", err)
	}
	undoBefore := app.GetUndoDepth(5)
	if undoBefore != 1 {
		t.Fatalf("undo depth after clone = %d, want 1", undoBefore)
	}

	// UserData10 can no longer hold slot 5's record — undo must fail closed.
	truncateUserData10(app.save, 5)
	udBefore := append([]byte(nil), app.save.UserData10.Data...)
	summaryBefore := app.save.ProfileSummaries[5]
	dataBefore := append([]byte(nil), app.save.Slots[5].Data...)

	if err := app.RevertSlot(5); err == nil {
		t.Fatalf("RevertSlot succeeded without restoring the full record, want error")
	}
	if got := app.GetUndoDepth(5); got != undoBefore {
		t.Fatalf("failed RevertSlot consumed the snapshot: depth %d, want %d", got, undoBefore)
	}
	if !bytes.Equal(app.save.UserData10.Data, udBefore) {
		t.Errorf("failed RevertSlot mutated UserData10")
	}
	if app.save.ProfileSummaries[5] != summaryBefore {
		t.Errorf("failed RevertSlot partially restored the parsed summary")
	}
	if !app.save.ActiveSlots[5] {
		t.Errorf("failed RevertSlot partially restored the active flag")
	}
	if !bytes.Equal(app.save.Slots[5].Data, dataBefore) {
		t.Errorf("failed RevertSlot partially restored the slot data")
	}
}
