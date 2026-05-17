package editor

import (
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
)

// Common AoW item IDs used across Phase 4A tests. Same as weapon_test.go
// — confirmed members of db.GetItemDataFuzzy with category="ashes_of_war".
const (
	aowLionsClawItemID      = uint32(0x80002710)
	aowImpalingThrustItemID = uint32(0x80002774)
)

// addGaItem inserts a GaItem entry at the slot's GaItems[idx]. The
// helper mirrors core's addTestWeapon / addTestAoW utilities but lives
// in editor's test package so we don't expose them outside core.
func addGaItem(slot *core.SaveSlot, idx int, handle, itemID, aowGaItemHandle uint32) {
	slot.GaItems[idx] = core.GaItemFull{
		Handle:          handle,
		ItemID:          itemID,
		Unk2:            -1,
		Unk3:            -1,
		AoWGaItemHandle: aowGaItemHandle,
	}
	if slot.GaMap == nil {
		slot.GaMap = map[uint32]uint32{}
	}
	slot.GaMap[handle] = itemID
}

// aowFixtureSlot extends fixtureSlot with a sized GaItems slice so AoW
// scans have something to walk. Enough room for ~16 GaItems is plenty
// for any individual test.
func aowFixtureSlot(t *testing.T) (*core.SaveSlot, int, int) {
	t.Helper()
	slot, invStart, stoStart := fixtureSlot(t)
	slot.GaItems = make([]core.GaItemFull, 16)
	for i := range slot.GaItems {
		slot.GaItems[i] = core.GaItemFull{Unk2: -1, Unk3: -1, AoWGaItemHandle: core.LegacyNoCustomAoWHandle}
	}
	return slot, invStart, stoStart
}

// ─── BuildSnapshot AoW classification ─────────────────────────────

func TestBuildSnapshot_WeaponWithCustomAoW(t *testing.T) {
	slot, invStart, _ := aowFixtureSlot(t)

	const (
		wepHandle = uint32(0x80800001)
		wepItemID = uint32(0x000F4240) // Dagger
		aowHandle = uint32(0xC0800001)
	)
	addGaItem(slot, 0, aowHandle, aowLionsClawItemID, core.LegacyNoCustomAoWHandle)
	addGaItem(slot, 1, wepHandle, wepItemID, aowHandle)
	fakeRecord(slot.Data, invStart, wepHandle, 1, 1000)

	snap, err := BuildSnapshot(slot, "ses-aow", 0)
	if err != nil {
		t.Fatalf("BuildSnapshot: %v", err)
	}
	if len(snap.InventoryItems) != 1 {
		t.Fatalf("expected 1 editable, got %d", len(snap.InventoryItems))
	}
	it := snap.InventoryItems[0]
	if it.CurrentAoWStatus != AoWStatusCustom {
		t.Errorf("status = %q, want %q", it.CurrentAoWStatus, AoWStatusCustom)
	}
	if !it.HasCurrentAoW {
		t.Error("HasCurrentAoW should be true")
	}
	if it.CurrentAoWHandle != aowHandle {
		t.Errorf("CurrentAoWHandle = 0x%08X, want 0x%08X", it.CurrentAoWHandle, aowHandle)
	}
	if it.CurrentAoWItemID != aowLionsClawItemID {
		t.Errorf("CurrentAoWItemID = 0x%08X, want 0x%08X", it.CurrentAoWItemID, aowLionsClawItemID)
	}
	if it.CurrentAoWName != "Lion's Claw" {
		t.Errorf("CurrentAoWName = %q, want Lion's Claw", it.CurrentAoWName)
	}
	if it.CurrentAoWShared {
		t.Error("CurrentAoWShared should be false for a single-weapon reference")
	}
}

func TestBuildSnapshot_WeaponNoCustomAoW_CanonicalSentinel(t *testing.T) {
	slot, invStart, _ := aowFixtureSlot(t)

	const (
		wepHandle = uint32(0x80800001)
		wepItemID = uint32(0x000F4240)
	)
	addGaItem(slot, 0, wepHandle, wepItemID, core.NoCustomAoWHandle)
	fakeRecord(slot.Data, invStart, wepHandle, 1, 1000)

	snap, err := BuildSnapshot(slot, "ses-aow-none", 0)
	if err != nil {
		t.Fatalf("BuildSnapshot: %v", err)
	}
	if len(snap.InventoryItems) != 1 {
		t.Fatalf("expected 1 editable")
	}
	it := snap.InventoryItems[0]
	if it.CurrentAoWStatus != AoWStatusNone {
		t.Errorf("status = %q, want %q", it.CurrentAoWStatus, AoWStatusNone)
	}
	if it.HasCurrentAoW {
		t.Error("HasCurrentAoW should be false for no-custom-AoW weapon")
	}
	if it.CurrentAoWHandle != 0 || it.CurrentAoWItemID != 0 || it.CurrentAoWName != "" {
		t.Errorf("no-custom AoW should leave fields empty: %+v", it)
	}
}

func TestBuildSnapshot_WeaponNoCustomAoW_LegacySentinel(t *testing.T) {
	slot, invStart, _ := aowFixtureSlot(t)

	const (
		wepHandle = uint32(0x80800001)
		wepItemID = uint32(0x000F4240)
	)
	addGaItem(slot, 0, wepHandle, wepItemID, core.LegacyNoCustomAoWHandle)
	fakeRecord(slot.Data, invStart, wepHandle, 1, 1000)

	snap, err := BuildSnapshot(slot, "ses-aow-legacy", 0)
	if err != nil {
		t.Fatalf("BuildSnapshot: %v", err)
	}
	if snap.InventoryItems[0].CurrentAoWStatus != AoWStatusNone {
		t.Errorf("legacy 0xFFFFFFFF sentinel should also yield AoWStatusNone, got %q",
			snap.InventoryItems[0].CurrentAoWStatus)
	}
}

func TestBuildSnapshot_WeaponMissingAoW(t *testing.T) {
	slot, invStart, _ := aowFixtureSlot(t)

	const (
		wepHandle = uint32(0x80800001)
		wepItemID = uint32(0x000F4240)
		// Non-sentinel handle that points to nothing in GaMap.
		danglingAoW = uint32(0xC0800099)
	)
	addGaItem(slot, 0, wepHandle, wepItemID, danglingAoW)
	fakeRecord(slot.Data, invStart, wepHandle, 1, 1000)

	snap, err := BuildSnapshot(slot, "ses-aow-missing", 0)
	if err != nil {
		t.Fatalf("BuildSnapshot: %v", err)
	}
	it := snap.InventoryItems[0]
	if it.CurrentAoWStatus != AoWStatusMissing {
		t.Errorf("status = %q, want %q", it.CurrentAoWStatus, AoWStatusMissing)
	}
	if !it.HasCurrentAoW {
		t.Error("HasCurrentAoW should be true even when AoW is missing — handle is set")
	}
	if it.CurrentAoWHandle != danglingAoW {
		t.Errorf("handle should be preserved on missing: got 0x%08X want 0x%08X",
			it.CurrentAoWHandle, danglingAoW)
	}
	if it.CurrentAoWItemID != 0 || it.CurrentAoWName != "" {
		t.Errorf("missing AoW should leave ItemID/Name zero, got %+v", it)
	}
}

func TestBuildSnapshot_WeaponSharedAoW(t *testing.T) {
	slot, invStart, _ := aowFixtureSlot(t)

	const (
		wep1Handle = uint32(0x80800001)
		wep2Handle = uint32(0x80800002)
		wepItemID  = uint32(0x000F4240)
		aowHandle  = uint32(0xC0800001)
	)
	addGaItem(slot, 0, aowHandle, aowLionsClawItemID, core.LegacyNoCustomAoWHandle)
	addGaItem(slot, 1, wep1Handle, wepItemID, aowHandle)
	addGaItem(slot, 2, wep2Handle, wepItemID, aowHandle)
	fakeRecord(slot.Data, invStart+0*core.InvRecordLen, wep1Handle, 1, 1000)
	fakeRecord(slot.Data, invStart+1*core.InvRecordLen, wep2Handle, 1, 1002)

	snap, err := BuildSnapshot(slot, "ses-aow-shared", 0)
	if err != nil {
		t.Fatalf("BuildSnapshot: %v", err)
	}
	if len(snap.InventoryItems) != 2 {
		t.Fatalf("expected 2 editables, got %d", len(snap.InventoryItems))
	}
	for _, it := range snap.InventoryItems {
		if it.CurrentAoWStatus != AoWStatusShared {
			t.Errorf("weapon 0x%08X status = %q, want %q",
				it.OriginalHandle, it.CurrentAoWStatus, AoWStatusShared)
		}
		if !it.CurrentAoWShared {
			t.Errorf("weapon 0x%08X CurrentAoWShared should be true", it.OriginalHandle)
		}
		if it.CurrentAoWHandle != aowHandle {
			t.Errorf("weapon 0x%08X CurrentAoWHandle = 0x%08X, want 0x%08X",
				it.OriginalHandle, it.CurrentAoWHandle, aowHandle)
		}
	}
}

func TestBuildSnapshot_NonWeaponHasNoAoWFields(t *testing.T) {
	slot, invStart, _ := aowFixtureSlot(t)

	// Talisman (handle prefix 0xA0) — should never get CurrentAoW fields.
	const talHandle = uint32(0xA00003E8)
	slot.GaMap[talHandle] = 0x200003E8 // Crimson Amber Medallion
	fakeRecord(slot.Data, invStart, talHandle, 1, 1000)

	snap, err := BuildSnapshot(slot, "ses-tal", 0)
	if err != nil {
		t.Fatalf("BuildSnapshot: %v", err)
	}
	if len(snap.InventoryItems) != 1 {
		t.Fatalf("expected 1 editable, got %d", len(snap.InventoryItems))
	}
	it := snap.InventoryItems[0]
	if !it.IsTalisman {
		t.Fatalf("expected talisman classification, got %+v", it)
	}
	if it.CurrentAoWStatus != "" || it.HasCurrentAoW || it.CurrentAoWHandle != 0 {
		t.Errorf("non-weapon should have empty AoW fields, got %+v", it)
	}
}

// ─── AddedItems do not pick up CurrentAoW fields ──────────────────

func TestAddedWeapon_HasNoCurrentAoWFields(t *testing.T) {
	// Build an empty snapshot then AddItem a Dagger. Source=added items
	// are constructed by AddItem, not BuildSnapshot, so they never run
	// through populateCurrentAoW.
	snap := &InventoryWorkspaceSnapshot{
		InventoryItems: []EditableItem{},
		StorageItems:   []EditableItem{},
	}
	if err := AddItem(snap, AddItemSpec{ItemID: 0x000F4240}, ContainerInventory, 0); err != nil {
		t.Fatalf("AddItem: %v", err)
	}
	it := snap.InventoryItems[0]
	if !it.IsWeapon {
		t.Fatalf("expected weapon, got %+v", it)
	}
	if it.CurrentAoWStatus != "" || it.HasCurrentAoW || it.CurrentAoWHandle != 0 || it.CurrentAoWItemID != 0 {
		t.Errorf("Source=added item should not carry CurrentAoW fields, got %+v", it)
	}
}

// ─── UpdateWeapon does not overwrite CurrentAoW fields ────────────

func TestUpdateWeapon_PendingAoWPreservesCurrentAoW(t *testing.T) {
	// Seed an existing weapon with custom AoW state, then issue an
	// UpdateWeapon SetAoWItemID pending request. CurrentAoW* must
	// remain populated from the original snapshot — they describe
	// what's *in the save now*, not what the user wants.
	snap := makeSeededWeaponWithCurrentAoW()
	uid := snap.InventoryItems[0].UID

	if err := UpdateWeapon(snap, uid, WeaponPatch{
		SetAoWItemID: true, AoWItemID: aowImpalingThrustItemID,
	}); err != nil {
		t.Fatalf("UpdateWeapon: %v", err)
	}
	it := snap.InventoryItems[0]
	if it.PendingAoWItemID != aowImpalingThrustItemID {
		t.Errorf("PendingAoWItemID = 0x%08X, want 0x%08X",
			it.PendingAoWItemID, aowImpalingThrustItemID)
	}
	if it.CurrentAoWItemID != aowLionsClawItemID {
		t.Errorf("CurrentAoWItemID changed: got 0x%08X, want 0x%08X (preserved)",
			it.CurrentAoWItemID, aowLionsClawItemID)
	}
	if it.CurrentAoWName != "Lion's Claw" {
		t.Errorf("CurrentAoWName changed: got %q, want Lion's Claw", it.CurrentAoWName)
	}
	if it.CurrentAoWStatus != AoWStatusCustom {
		t.Errorf("CurrentAoWStatus changed: got %q, want %q", it.CurrentAoWStatus, AoWStatusCustom)
	}
}

// makeSeededWeaponWithCurrentAoW returns a snapshot containing a single
// weapon with realistic CurrentAoW* state already populated. Used by
// the UpdateWeapon-preserves test where we don't want to round-trip
// through BuildSnapshot for setup.
func makeSeededWeaponWithCurrentAoW() *InventoryWorkspaceSnapshot {
	return &InventoryWorkspaceSnapshot{
		InventoryItems: []EditableItem{
			{
				UID:              "hnd:0x80800001",
				Source:           ItemSourceOriginal,
				Container:        ContainerInventory,
				OriginalHandle:   0x80800001,
				ItemID:           0x000F4240,
				BaseItemID:       0x000F4240,
				Name:             "Dagger",
				Category:         "melee_armaments",
				Quantity:         1,
				AcquisitionIndex: 1000,
				MaxUpgrade:       25,
				HasGaItem:        true,
				IsWeapon:         true,
				CurrentAoWHandle: 0xC0800001,
				CurrentAoWItemID: aowLionsClawItemID,
				CurrentAoWName:   "Lion's Claw",
				HasCurrentAoW:    true,
				CurrentAoWStatus: AoWStatusCustom,
			},
		},
		StorageItems: []EditableItem{},
	}
}

// ─── Validation: read-side AoW anomalies surface as warnings ──────

func TestValidate_CurrentAoWMissingIsWarning(t *testing.T) {
	snap := makeSeededWeaponWithCurrentAoW()
	snap.InventoryItems[0].CurrentAoWStatus = AoWStatusMissing
	snap.InventoryItems[0].CurrentAoWItemID = 0
	snap.InventoryItems[0].CurrentAoWName = ""

	rep := Validate(*snap)
	if !rep.OK {
		t.Errorf("missing AoW should not flip OK to false; errors: %+v", rep.Errors)
	}
	found := false
	for _, w := range rep.Warnings {
		if w.Code == CodeCurrentAoWMissing {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("missing CodeCurrentAoWMissing warning; warnings: %+v", rep.Warnings)
	}
}

func TestValidate_CurrentAoWSharedIsWarning(t *testing.T) {
	snap := makeSeededWeaponWithCurrentAoW()
	snap.InventoryItems[0].CurrentAoWStatus = AoWStatusShared
	snap.InventoryItems[0].CurrentAoWShared = true

	rep := Validate(*snap)
	if !rep.OK {
		t.Errorf("shared AoW should not flip OK to false; errors: %+v", rep.Errors)
	}
	found := false
	for _, w := range rep.Warnings {
		if w.Code == CodeCurrentAoWShared {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("missing CodeCurrentAoWShared warning; warnings: %+v", rep.Warnings)
	}
}

// Phase 4B: a single item with both pending-set AND pending-clear is
// invalid workspace state. Validate must surface CodePendingAoWConflict
// as an error so save refuses byte-for-byte.
func TestValidate_PendingAoWConflictIsError(t *testing.T) {
	snap := makeSeededWeaponWithCurrentAoW()
	snap.InventoryItems[0].PendingAoWItemID = aowLionsClawItemID
	snap.InventoryItems[0].PendingAoWClear = true

	rep := Validate(*snap)
	if rep.OK {
		t.Fatal("expected OK=false for conflicting pending AoW state")
	}
	found := false
	for _, e := range rep.Errors {
		if e.Code == CodePendingAoWConflict {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("missing CodePendingAoWConflict error; errors: %+v", rep.Errors)
	}
}

func TestValidate_CurrentAoWNonAoWCategoryIsWarning(t *testing.T) {
	// Build a snapshot where CurrentAoWItemID resolves to a non-AoW
	// category. We use a known weapon itemID (Claymore) here — its
	// db.GetItemDataFuzzy result has category "melee_armaments".
	snap := makeSeededWeaponWithCurrentAoW()
	snap.InventoryItems[0].CurrentAoWItemID = 0x003085E0 // Claymore
	snap.InventoryItems[0].CurrentAoWName = "Claymore"
	snap.InventoryItems[0].CurrentAoWStatus = AoWStatusCustom

	rep := Validate(*snap)
	if !rep.OK {
		t.Errorf("non-AoW-category current AoW must remain a warning, not error; errors: %+v", rep.Errors)
	}
	found := false
	for _, w := range rep.Warnings {
		if w.Code == CodeCurrentAoWNonAoWCategory {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("missing CodeCurrentAoWNonAoWCategory warning; warnings: %+v", rep.Warnings)
	}
}
