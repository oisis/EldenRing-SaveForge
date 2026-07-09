package editor

import (
	"bytes"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
)

// ---- talisman identity red tests (test-first, see conversation report) -----
//
// Root cause: OriginalHandle / UID ("hnd:0x%08X") is treated as a per-record
// instance identifier everywhere in this package, which is only true for
// GaItem-backed types (weapon/armor/AoW). Talisman (0xA0) handles are
// item-derived (db.HandleToItemID) — every copy of the same talisman shares
// the identical handle, whether it sits in one container twice or split
// across inventory and storage. That is normal save state, not corruption.
//
// These tests assert the CORRECT behavior and are expected to FAIL against
// the current implementation, pinpointing exactly where the handle-only
// identity model breaks: BuildSnapshot's per-container dedup, Validate's
// cross-container handle/UID check, RemoveItem's UID lookup, and the
// session baseline map[handle]ContainerKind consumed by ApplyWorkspaceSave.

// talismanItemID / talismanHandle — "Sacrificial Twig" (backend/db/data/talismans.go).
// Handle = 0xA0 prefix | (itemID & 0x0FFFFFFF): 0xA0000000 | 0x000017B6.
const (
	talismanItemID = uint32(0x200017B6)
	talismanHandle = uint32(0xA00017B6)
)

// fixtureSlotForSave mirrors fixtureSlot but uses a positive MagicOffset.
// fixtureSlot's magicOff=0 is fine for BuildSnapshot-only tests (parsing
// only ever adds InvStartFromMagic to it), but ApplyWorkspaceSave's
// writeContainerLayout treats MagicOffset<=0 as "no inventory region" and
// refuses to write — so any test that exercises the real save path needs
// this variant instead.
func fixtureSlotForSave(t *testing.T) (*core.SaveSlot, int, int) {
	t.Helper()
	const magicOff = 16
	invStart := magicOff + core.InvStartFromMagic
	invEnd := invStart + core.CommonItemCount*core.InvRecordLen
	storageBoxOff := invEnd + 64
	storageStart := storageBoxOff + core.StorageHeaderSkip
	storageEnd := storageStart + core.StorageCommonCount*core.InvRecordLen
	bufSize := storageEnd + 64

	slot := &core.SaveSlot{
		Version:          1,
		MagicOffset:      magicOff,
		StorageBoxOffset: storageBoxOff,
		Data:             make([]byte, bufSize),
		GaMap:            make(map[uint32]uint32),
	}
	return slot, invStart, storageStart
}

// (1) Two identical talismans in the same container.
//
// RED today: classifyRecord's per-container `seen` map forces the second
// occurrence of any repeated handle to a RawInventoryRecord with
// ReasonDuplicateHandle — indistinguishable from real corruption (e.g. two
// weapon GaItems sharing a handle). Both Sacrificial Twig stacks should be
// independently editable.
func TestBuildSnapshot_DuplicateTalismanHandleSameContainer_BothEditable(t *testing.T) {
	slot, invStart, _ := fixtureSlot(t)
	fakeRecord(slot.Data, invStart+0*core.InvRecordLen, talismanHandle, 1, 1000)
	fakeRecord(slot.Data, invStart+1*core.InvRecordLen, talismanHandle, 1, 1002)

	snap, err := BuildSnapshot(slot, "ses-t1", 0)
	if err != nil {
		t.Fatalf("BuildSnapshot: %v", err)
	}
	if len(snap.InventoryItems) != 2 {
		t.Fatalf("expected both Sacrificial Twig copies editable (handle collision is normal for talismans), got %d editable + %d passthrough (passthrough reasons: %+v)",
			len(snap.InventoryItems), len(snap.UnsupportedInventoryRecords), snap.UnsupportedInventoryRecords)
	}
	for i, it := range snap.InventoryItems {
		if it.Category != "talismans" || !it.IsTalisman {
			t.Errorf("item[%d] = %+v, want category=talismans IsTalisman=true", i, it)
		}
	}
}

// (2) The same talisman split across inventory and storage.
//
// RED today: UID is "hnd:0x%08X" — handle-only — so both copies report the
// IDENTICAL UID to callers even though they are two distinct physical
// records in two different containers.
func TestBuildSnapshot_TalismanInventoryAndStorage_DistinctUID(t *testing.T) {
	slot, invStart, stoStart := fixtureSlot(t)
	fakeRecord(slot.Data, invStart+0*core.InvRecordLen, talismanHandle, 1, 1000)
	fakeRecord(slot.Data, stoStart+0*core.InvRecordLen, talismanHandle, 1, 2000)

	snap, err := BuildSnapshot(slot, "ses-t2", 0)
	if err != nil {
		t.Fatalf("BuildSnapshot: %v", err)
	}
	if len(snap.InventoryItems) != 1 || len(snap.StorageItems) != 1 {
		t.Fatalf("expected one editable Twig per container, got inv=%d sto=%d", len(snap.InventoryItems), len(snap.StorageItems))
	}
	if snap.InventoryItems[0].UID == snap.StorageItems[0].UID {
		t.Errorf("inventory and storage copies must have distinct record identity, both report UID %q — handle alone does not identify a talisman record",
			snap.InventoryItems[0].UID)
	}
}

// (2b) Validate must not treat the same legitimate cross-container split as
// a corruption error.
//
// RED today: Validate's check() shares one handleSeen/uidSeen map across
// both InventoryItems and StorageItems, so the second occurrence trips
// CodeDuplicateHandle and CodeDuplicateUID and flips WorkspaceValidationReport.OK
// to false for a perfectly ordinary save.
func TestValidate_TalismanCrossContainerDuplicate_NotAnError(t *testing.T) {
	slot, invStart, stoStart := fixtureSlot(t)
	fakeRecord(slot.Data, invStart+0*core.InvRecordLen, talismanHandle, 1, 1000)
	fakeRecord(slot.Data, stoStart+0*core.InvRecordLen, talismanHandle, 1, 2000)

	snap, err := BuildSnapshot(slot, "ses-t3", 0)
	if err != nil {
		t.Fatalf("BuildSnapshot: %v", err)
	}
	rep := Validate(snap)
	for _, e := range rep.Errors {
		if e.Code == CodeDuplicateUID || e.Code == CodeDuplicateHandle {
			t.Errorf("talisman held in both inventory and storage must not be a validation error: %s", e.Message)
		}
	}
	if !rep.OK {
		t.Errorf("Validation.OK = false, want true — errors: %+v", rep.Errors)
	}
}

// (3) Removing the storage copy must remove exactly the storage copy.
//
// RED today: findEditable (mutate.go) searches InventoryItems before
// StorageItems and matches by UID string equality. Since both copies share
// UID "hnd:0xA00017B6", asking to remove the STORAGE copy silently deletes
// the INVENTORY copy instead — the opposite of what was requested.
func TestRemoveItem_TalismanStorageCopy_RemovesCorrectRecord(t *testing.T) {
	slot, invStart, stoStart := fixtureSlot(t)
	fakeRecord(slot.Data, invStart+0*core.InvRecordLen, talismanHandle, 1, 1000)
	fakeRecord(slot.Data, stoStart+0*core.InvRecordLen, talismanHandle, 1, 2000)

	snap, err := BuildSnapshot(slot, "ses-t4", 0)
	if err != nil {
		t.Fatalf("BuildSnapshot: %v", err)
	}
	storageUID := snap.StorageItems[0].UID

	if err := RemoveItem(&snap, storageUID); err != nil {
		t.Fatalf("RemoveItem: %v", err)
	}

	if len(snap.StorageItems) != 0 {
		t.Errorf("expected the storage copy to be removed, storage still has %d item(s)", len(snap.StorageItems))
	}
	if len(snap.InventoryItems) != 1 {
		t.Errorf("removing the storage copy must not touch the inventory copy — inventory has %d item(s), want 1", len(snap.InventoryItems))
	}
}

// (4) A no-op save (nothing edited) must not rewrite bytes for a talisman
// that happens to be present in both containers.
//
// RED today: StartSession's BaselineEditableHandles is map[uint32]ContainerKind
// keyed by handle. The storage-loop write overwrites the inventory-loop
// write for the shared handle, so the baseline ends up claiming the
// talisman lives ONLY in storage. containerSameItemSet then sees the
// inventory container's editable count (1) disagree with what the baseline
// says inventory should hold (0) and falls through to the full
// wipe-and-replay path — reassigning the item a new physical slot and a
// new acquisition index even though the user changed nothing.
func TestApplyWorkspaceSave_NoOpWithTalismanInBothContainers_ByteIdentical(t *testing.T) {
	slot, invStart, stoStart := fixtureSlotForSave(t)
	fakeRecord(slot.Data, invStart+0*core.InvRecordLen, talismanHandle, 1, 1000)
	fakeRecord(slot.Data, stoStart+0*core.InvRecordLen, talismanHandle, 1, 2000)

	invEnd := invStart + core.CommonItemCount*core.InvRecordLen
	stoEnd := stoStart + core.StorageCommonCount*core.InvRecordLen
	invBefore := append([]byte(nil), slot.Data[invStart:invEnd]...)
	stoBefore := append([]byte(nil), slot.Data[stoStart:stoEnd]...)

	sess, err := StartSession(slot, 0)
	if err != nil {
		t.Fatalf("StartSession: %v", err)
	}
	if _, err := ApplyWorkspaceSave(slot, &sess.Workspace, sess.BaselineEditableHandles); err != nil {
		t.Fatalf("ApplyWorkspaceSave (no-op): %v", err)
	}

	if !bytes.Equal(invBefore, slot.Data[invStart:invEnd]) {
		t.Errorf("no-op save rewrote the inventory region — a talisman also held in storage corrupts the handle-keyed baseline")
	}
	if !bytes.Equal(stoBefore, slot.Data[stoStart:stoEnd]) {
		t.Errorf("no-op save rewrote the storage region even though nothing changed")
	}
}
