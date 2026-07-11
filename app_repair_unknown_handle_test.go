package main

import (
	"encoding/binary"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
)

// unknownKeyHandle is the reported corrupted handle. Its zero type prefix is
// not a known GaItem type, and its remaining value cannot identify an item, so
// removal is manual and irrecoverable.
const unknownKeyHandle = uint32(0x00000001)

// keyItemStart is the binary offset of the KeyItems block in buildApplyInvFixture.
func keyItemStart() int {
	return 16 + core.InvStartFromMagic + core.CommonItemCount*core.InvRecordLen + core.InvKeyCountHeader
}

// slotWithUnknownKeyRecord returns a fixture whose KeyItems row 0 holds an
// unknown-prefix record (both binary and in-memory list in sync).
func slotWithUnknownKeyRecord(t *testing.T) *core.SaveSlot {
	slot := buildApplyInvFixture(t, nil)
	key := core.InventoryItem{GaItemHandle: unknownKeyHandle, Quantity: 1, Index: 0}
	off := keyItemStart()
	binary.LittleEndian.PutUint32(slot.Data[off:], key.GaItemHandle)
	binary.LittleEndian.PutUint32(slot.Data[off+4:], key.Quantity)
	binary.LittleEndian.PutUint32(slot.Data[off+8:], key.Index)
	slot.Inventory.KeyItems = []core.InventoryItem{key}
	return slot
}

func unknownHandleIssue(t *testing.T, slot *core.SaveSlot) core.RepairIssue {
	t.Helper()
	for _, iss := range core.ScanRepairIssues(0, slot) {
		if iss.Key.Code == core.RepairCodeUnknownHandleType {
			return iss
		}
	}
	t.Fatal("scan did not report an unknown_handle_type issue")
	return core.RepairIssue{}
}

// The DTO must advertise leave_unchanged + remove_record, defaulting to the
// non-destructive leave_unchanged.
func TestRepairActionsForCode_UnknownHandleType(t *testing.T) {
	actions, def := repairActionsForCode(core.RepairCodeUnknownHandleType)
	if def != RepairActionLeaveUnchanged {
		t.Errorf("default = %q, want %q", def, RepairActionLeaveUnchanged)
	}
	if len(actions) != 2 || actions[0].ID != RepairActionLeaveUnchanged || actions[1].ID != core.RepairActionRemoveRecord {
		t.Fatalf("actions = %+v, want [leave_unchanged, remove_record]", actions)
	}
}

// The DTO surfaced by the scan endpoint for the unknown key record carries the
// same action set + default.
func TestScanDTO_UnknownHandleKeyItem_AdvertisesRemove(t *testing.T) {
	slot := slotWithUnknownKeyRecord(t)
	rep := buildRepairIssueReport(0, "", slot, nil, nil)

	var dto *RepairIssueDTO
	for i := range rep.Issues {
		if rep.Issues[i].Key.Code == core.RepairCodeUnknownHandleType {
			dto = &rep.Issues[i]
			break
		}
	}
	if dto == nil {
		t.Fatal("report has no unknown_handle_type issue")
	}
	if dto.Key.Scope != "inventory_key" {
		t.Errorf("scope = %q, want inventory_key", dto.Key.Scope)
	}
	if dto.DefaultAction != RepairActionLeaveUnchanged {
		t.Errorf("default = %q, want leave_unchanged", dto.DefaultAction)
	}
	if len(dto.Actions) != 2 || dto.Actions[1].ID != core.RepairActionRemoveRecord {
		t.Errorf("actions = %+v, want remove_record available", dto.Actions)
	}
}

// Applying remove_record clears exactly that KeyItems row, keeps the KeyItems
// layout intact, and the diagnostic is gone on rescan.
func TestApplyRemoveRecord_UnknownHandleKeyItem(t *testing.T) {
	slot := slotWithUnknownKeyRecord(t)
	iss := unknownHandleIssue(t, slot)
	fp, ok := core.FingerprintRecordAt(slot, "inventory_key", 0)
	if !ok {
		t.Fatal("cannot fingerprint the key record")
	}
	target := RepairApplyTarget{
		IssueID:        iss.IssueID,
		Key:            iss.Key,
		Fingerprint:    fp,
		SelectedAction: core.RepairActionRemoveRecord,
	}

	rep := applyRepairBatchToSlot(slot, 0, []RepairApplyTarget{target}, false)
	if rep.Applied != 1 || rep.Failed != 0 {
		t.Fatalf("applied=%d failed=%d, want 1/0 (results=%+v)", rep.Applied, rep.Failed, rep.Results)
	}
	// KeyItems slice length preserved (record zeroed in place, not dropped).
	if len(slot.Inventory.KeyItems) != 1 {
		t.Errorf("KeyItems length = %d, want 1 (header/layout preserved)", len(slot.Inventory.KeyItems))
	}
	if h := binary.LittleEndian.Uint32(slot.Data[keyItemStart():]); h != 0 {
		t.Errorf("binary key handle = 0x%08X, want 0 (cleared)", h)
	}
	for _, r := range core.ScanRepairIssues(0, slot) {
		if r.Key.Code == core.RepairCodeUnknownHandleType {
			t.Fatal("unknown_handle_type still present after removal")
		}
	}
}

// "Repair all" uses the default action (leave_unchanged), which is a skip: the
// record must remain untouched.
func TestRepairDefault_UnknownHandleLeftUnchanged(t *testing.T) {
	slot := slotWithUnknownKeyRecord(t)
	iss := unknownHandleIssue(t, slot)
	_, def := repairActionsForCode(iss.Key.Code)
	target := RepairApplyTarget{
		IssueID:        iss.IssueID,
		Key:            iss.Key,
		Fingerprint:    iss.Fingerprint,
		SelectedAction: def, // repair-all picks the default
	}

	rep := applyRepairBatchToSlot(slot, 0, []RepairApplyTarget{target}, false)
	if rep.Skipped != 1 || rep.Applied != 0 {
		t.Fatalf("skipped=%d applied=%d, want 1/0", rep.Skipped, rep.Applied)
	}
	if h := binary.LittleEndian.Uint32(slot.Data[keyItemStart():]); h != unknownKeyHandle {
		t.Errorf("key handle = 0x%08X, want it left unchanged 0x%08X", h, unknownKeyHandle)
	}
}
