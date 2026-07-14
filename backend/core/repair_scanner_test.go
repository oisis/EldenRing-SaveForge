package core

import (
	"testing"
)

// Real item IDs from backend/db/data — used to avoid false unknown_item_id in tests.
// 0xB0002774 → HandleToItemID → 0x40002774 = "Smithing Stone [1]" (bolstering_materials)
const (
	testHandleSmithingStone = uint32(0xB0002774)
	testItemIDSmithingStone = uint32(0x40002774)
	testHandleDagger        = uint32(0x80000002)
	testItemIDDagger        = uint32(0x00400110)
	testHandleArrow         = uint32(0x80800AA1)
	testItemIDArrow         = uint32(0x02FAF080)
)

// ---- regression test --------------------------------------------------------

// TestScanRepairIssues_DuplicateHandle is the early regression anchor.
//
// Regression: duplicate_handle and duplicate_uid were reported by editor.Validate
// with CanRepair=false, making them appear as "unrepairable" in the UI. The core
// scanner now emits only duplicate_handle for GaItem-backed duplicate records;
// duplicate_uid is redundant UI/workspace identity fallout, not a separate repair.
func TestScanRepairIssues_DuplicateHandle(t *testing.T) {
	slot := &SaveSlot{
		GaMap: map[uint32]uint32{testHandleDagger: testItemIDDagger},
		Inventory: EquipInventoryData{
			CommonItems: []InventoryItem{
				{GaItemHandle: testHandleDagger, Quantity: 1, Index: 500},
				{GaItemHandle: testHandleDagger, Quantity: 1, Index: 501}, // duplicate → regression
			},
		},
	}

	issues := ScanRepairIssues(0, slot)

	foundHandle, foundUID := false, false
	for _, iss := range issues {
		switch iss.Key.Code {
		case RepairCodeDuplicateHandle:
			foundHandle = true
			if len(iss.Actions) == 0 {
				t.Errorf("duplicate_handle must have proposed actions, got none")
			}
			if iss.DefaultAction == "" {
				t.Errorf("duplicate_handle must have a default action")
			}
			if iss.IssueID == "" {
				t.Errorf("IssueID must be non-empty")
			}
		case RepairCodeDuplicateUID:
			foundUID = true
		}
	}
	if !foundHandle {
		t.Error("expected duplicate_handle issue, scanner returned none")
	}
	if foundUID {
		t.Error("duplicate_uid is redundant with duplicate_handle and must not be emitted by core scanner")
	}
}

// TestScanRepairIssues_GoodsDuplicateHandle_NotAnError confirms repeated goods
// handles are not treated as duplicate records. Goods handles are item-derived
// like talismans; rehandling them into synthetic GaMap-backed handles is unsafe
// for game decoding.
func TestScanRepairIssues_GoodsDuplicateHandle_NotAnError(t *testing.T) {
	slot := &SaveSlot{
		GaMap: map[uint32]uint32{},
		Inventory: EquipInventoryData{
			CommonItems: []InventoryItem{
				{GaItemHandle: testHandleSmithingStone, Quantity: 10, Index: 500},
			},
		},
		Storage: EquipInventoryData{
			CommonItems: []InventoryItem{
				{GaItemHandle: testHandleSmithingStone, Quantity: 10, Index: 501},
			},
		},
	}

	for _, iss := range ScanRepairIssues(0, slot) {
		if iss.Key.Code == RepairCodeDuplicateHandle || iss.Key.Code == RepairCodeDuplicateUID {
			t.Errorf("goods handle 0x%08X repeated across inventory/storage is normal — must not be reported as %s: %q",
				testHandleSmithingStone, iss.Key.Code, iss.Description)
		}
	}
}

// TestScanRepairIssues_AmmoDuplicateHandle_NotAnError confirms arrows/bolts are
// not treated like unique weapon instances. They use the weapon handle prefix,
// but are stackable ammo and can legitimately appear in inventory and storage.
func TestScanRepairIssues_AmmoDuplicateHandle_NotAnError(t *testing.T) {
	slot := &SaveSlot{
		GaMap: map[uint32]uint32{testHandleArrow: testItemIDArrow},
		Inventory: EquipInventoryData{
			CommonItems: []InventoryItem{
				{GaItemHandle: testHandleArrow, Quantity: 600, Index: 500},
			},
		},
		Storage: EquipInventoryData{
			CommonItems: []InventoryItem{
				{GaItemHandle: testHandleArrow, Quantity: 600, Index: 501},
			},
		},
	}

	for _, iss := range ScanRepairIssues(0, slot) {
		if iss.Key.Code == RepairCodeDuplicateHandle || iss.Key.Code == RepairCodeDuplicateUID {
			t.Errorf("ammo handle 0x%08X repeated across inventory/storage is normal — must not be reported as %s: %q",
				testHandleArrow, iss.Key.Code, iss.Description)
		}
	}
}

// ---- unknown_item_id false positive -----------------------------------------

// TestScanRepairIssues_NoFalseAlarmHandleEncoded confirms that a handle-encoded
// item (e.g. goods/talismans) without a GaMap entry does NOT trigger unknown_item_id
// when the item resolves correctly via db.HandleToItemID + db.GetItemDataFuzzy.
func TestScanRepairIssues_NoFalseAlarmHandleEncoded(t *testing.T) {
	slot := &SaveSlot{
		// No GaMap entry for testHandleSmithingStone — goods don't need GaItems.
		GaMap: map[uint32]uint32{},
		Inventory: EquipInventoryData{
			CommonItems: []InventoryItem{
				{GaItemHandle: testHandleSmithingStone, Quantity: 10, Index: 500},
			},
		},
	}

	issues := ScanRepairIssues(0, slot)

	for _, iss := range issues {
		if iss.Key.Code == RepairCodeUnknownItemID {
			t.Errorf("false alarm: handle-encoded item without GaMap should not be unknown_item_id; got %q", iss.Description)
		}
	}
}

// TestScanRepairIssues_NoFalseAlarmFilledWondrousPhysick confirms the filled
// save-state Flask of Wondrous Physick ID is normalized to the DB display ID
// before unknown_item_id checks.
func TestScanRepairIssues_NoFalseAlarmFilledWondrousPhysick(t *testing.T) {
	slot := &SaveSlot{
		GaMap: map[uint32]uint32{},
		Inventory: EquipInventoryData{
			CommonItems: []InventoryItem{
				{GaItemHandle: 0xB00000FA, Quantity: 1, Index: 500},
			},
		},
	}

	for _, iss := range ScanRepairIssues(0, slot) {
		if iss.Key.Code == RepairCodeUnknownItemID {
			t.Errorf("filled Wondrous Physick handle should resolve through display ID normalization; got %q", iss.Description)
		}
	}
}

// ---- duplicate_acquisition_index scope --------------------------------------

// TestScanRepairIssues_IndexDedup_InventoryStorageSeparate confirms that the same
// acquisition index appearing in both inventory and storage does NOT generate
// duplicate_acquisition_index (storage is excluded from the dedup map).
func TestScanRepairIssues_IndexDedup_InventoryStorageSeparate(t *testing.T) {
	slot := &SaveSlot{
		GaMap: map[uint32]uint32{},
		Inventory: EquipInventoryData{
			CommonItems: []InventoryItem{
				// testHandleSmithingStone in inventory at index 500
				{GaItemHandle: testHandleSmithingStone, Quantity: 5, Index: 500},
			},
		},
		Storage: EquipInventoryData{
			CommonItems: []InventoryItem{
				// Different handle, same index 500 — should NOT trigger duplicate
				{GaItemHandle: 0xB0002775, Quantity: 5, Index: 500},
			},
		},
	}

	issues := ScanRepairIssues(0, slot)

	for _, iss := range issues {
		if iss.Key.Code == RepairCodeDuplicateAcquisitionIndex {
			t.Errorf("same index in inventory and storage must not generate duplicate_acquisition_index; got %q", iss.Description)
		}
	}
}

// ---- AoW tests --------------------------------------------------------------

// TestScanRepairIssues_AoWMissing confirms current_aow_missing is detected
// when the AoW handle is not in GaMap.
func TestScanRepairIssues_AoWMissing(t *testing.T) {
	aowHandle := uint32(0xC0000001)
	weapHandle := uint32(0x80000002)
	slot := &SaveSlot{
		GaMap: map[uint32]uint32{
			weapHandle: 0x00400110, // weapon itemID (in GaMap)
			// aowHandle intentionally absent → missing
		},
		Inventory: EquipInventoryData{
			CommonItems: []InventoryItem{
				{GaItemHandle: weapHandle, Quantity: 1, Index: 500},
			},
		},
		GaItems: []GaItemFull{
			{Handle: weapHandle, ItemID: 0x00400110, AoWGaItemHandle: aowHandle, Unk2: -1, Unk3: -1},
			// no GaItem for aowHandle
		},
	}

	issues := ScanRepairIssues(0, slot)

	found := false
	for _, iss := range issues {
		if iss.Key.Code == RepairCodeCurrentAoWMissing {
			found = true
			if iss.DefaultAction != RepairActionClearAoW {
				t.Errorf("expected default action %q, got %q", RepairActionClearAoW, iss.DefaultAction)
			}
			hasPickAoW := false
			for _, a := range iss.Actions {
				if a == RepairActionPickAoW {
					hasPickAoW = true
				}
			}
			if !hasPickAoW {
				t.Errorf("current_aow_missing should offer pick_aow action")
			}
		}
	}
	if !found {
		t.Error("expected current_aow_missing issue, got none")
	}
}

// TestScanRepairIssues_AoWNonAoWCategory confirms current_aow_non_aow_category is
// detected when the AoW handle resolves (via GaMap) to an item whose DB category
// is not "ashes_of_war".
func TestScanRepairIssues_AoWNonAoWCategory(t *testing.T) {
	// Use Smithing Stone [1] as the "AoW" — category "bolstering_materials".
	aowHandle := uint32(0xC0002774)
	weapHandle := uint32(0x80000002)
	slot := &SaveSlot{
		GaMap: map[uint32]uint32{
			aowHandle: testItemIDSmithingStone, // resolves to Smithing Stone, wrong category
		},
		Inventory: EquipInventoryData{
			CommonItems: []InventoryItem{
				{GaItemHandle: weapHandle, Quantity: 1, Index: 500},
			},
		},
		GaItems: []GaItemFull{
			{Handle: weapHandle, ItemID: 0x00400110, AoWGaItemHandle: aowHandle, Unk2: -1, Unk3: -1},
			{Handle: aowHandle, ItemID: testItemIDSmithingStone, Unk2: -1, Unk3: -1, AoWGaItemHandle: NoCustomAoWHandle},
		},
	}

	issues := ScanRepairIssues(0, slot)

	found := false
	for _, iss := range issues {
		if iss.Key.Code == RepairCodeCurrentAoWNonAoWCategory {
			found = true
			if iss.DefaultAction != RepairActionClearAoW {
				t.Errorf("expected default action %q, got %q", RepairActionClearAoW, iss.DefaultAction)
			}
		}
	}
	if !found {
		t.Error("expected current_aow_non_aow_category issue, got none")
	}
}

// ---- stats ------------------------------------------------------------------

// TestScanRepairIssues_StatsFormula confirms stats_formula is detected when
// Level does not match sum(attrs) - 79.
func TestScanRepairIssues_StatsFormula(t *testing.T) {
	slot := &SaveSlot{
		Player: PlayerGameData{
			// sum = 80, expected level = 1
			Level: 2, // mismatch → should trigger
			Vigor: 10, Mind: 10, Endurance: 10,
			Strength: 10, Dexterity: 10, Intelligence: 10,
			Faith: 10, Arcane: 10,
		},
	}

	issues := ScanRepairIssues(0, slot)

	found := false
	for _, iss := range issues {
		if iss.Key.Code == RepairCodeStatsFormula {
			found = true
			if len(iss.Actions) == 0 {
				t.Errorf("stats_formula issue has no actions")
			}
			if iss.Fingerprint == "" {
				t.Errorf("stats_formula issue has empty fingerprint")
			}
		}
	}
	if !found {
		t.Error("expected stats_formula issue, got none")
	}
}

// ---- clean slot -------------------------------------------------------------

// TestScanRepairIssues_CleanSlot confirms that an empty slot produces zero issues.
func TestScanRepairIssues_CleanSlot(t *testing.T) {
	// Empty slot: no inventory, no GaItems, Level=0 (skips stats check).
	issues := ScanRepairIssues(0, &SaveSlot{})
	if len(issues) != 0 {
		t.Errorf("empty slot must produce 0 issues, got %d:", len(issues))
		for _, iss := range issues {
			t.Logf("  code=%q desc=%q", iss.Key.Code, iss.Description)
		}
	}
}

// ---- low acquisition index is legal -----------------------------------------

// TestScanRepairIssues_LowAcquisitionIndexNotReserved proves a low acquisition
// index is not itself a defect. Genuine game-created records (e.g. Memory of
// Grace at 432, Lordsworn weapons at very low indices) legitimately sit at or
// below InvEquipReservedMax; the scanner must not flag them. InvEquipReservedMax
// is only a conservative floor for freshly generated editor indices.
func TestScanRepairIssues_LowAcquisitionIndexNotReserved(t *testing.T) {
	for _, idx := range []uint32{2, 100, InvEquipReservedMax} {
		slot := &SaveSlot{
			GaMap: map[uint32]uint32{},
			Inventory: EquipInventoryData{
				CommonItems: []InventoryItem{
					{GaItemHandle: testHandleSmithingStone, Quantity: 1, Index: idx},
				},
			},
		}
		issues := ScanRepairIssues(0, slot)
		if len(issues) != 0 {
			t.Errorf("Index=%d: expected 0 issues from a legal low index, got %d:", idx, len(issues))
			for _, iss := range issues {
				t.Logf("  code=%q desc=%q", iss.Key.Code, iss.Description)
			}
		}
	}
}

// TestScanRepairIssues_LowDuplicateIndexStillDetected confirms that accepting
// low indices did not weaken duplicate detection: two records sharing a low
// acquisition index are still suspicious and must report
// duplicate_acquisition_index.
func TestScanRepairIssues_LowDuplicateIndexStillDetected(t *testing.T) {
	slot := &SaveSlot{
		GaMap: map[uint32]uint32{},
		Inventory: EquipInventoryData{
			CommonItems: []InventoryItem{
				{GaItemHandle: testHandleSmithingStone, Quantity: 1, Index: 2},
				{GaItemHandle: 0xB0002775, Quantity: 1, Index: 2}, // different item, same low index
			},
		},
	}

	found := false
	for _, iss := range ScanRepairIssues(0, slot) {
		if iss.Key.Code == RepairCodeDuplicateAcquisitionIndex {
			found = true
		}
	}
	if !found {
		t.Error("duplicate low acquisition index must still report duplicate_acquisition_index")
	}
}

// ---- determinism ------------------------------------------------------------

// TestScanRepairIssues_IssueIDDeterministic confirms issueID is stable across runs.
func TestScanRepairIssues_IssueIDDeterministic(t *testing.T) {
	slot := &SaveSlot{
		GaMap: map[uint32]uint32{testHandleDagger: testItemIDDagger},
		Inventory: EquipInventoryData{
			CommonItems: []InventoryItem{
				{GaItemHandle: testHandleDagger, Quantity: 1, Index: 500},
				{GaItemHandle: testHandleDagger, Quantity: 1, Index: 501},
			},
		},
	}

	a := ScanRepairIssues(0, slot)
	b := ScanRepairIssues(0, slot)

	if len(a) != len(b) {
		t.Fatalf("scan not deterministic: %d vs %d issues", len(a), len(b))
	}
	for i := range a {
		if a[i].IssueID != b[i].IssueID {
			t.Errorf("issue[%d] IssueID not stable: %q vs %q", i, a[i].IssueID, b[i].IssueID)
		}
	}
}

// ---- physical GaItem duplicate handle ---------------------------------------

// TestScanRepairIssues_DuplicatePhysicalGaItemHandle covers the real corruption
// (Slot 4 "Średniak": GaItem[5103] reuses handle 0x808113EE from GaItem[5102]):
// two non-empty physical GaItems with the same handle but different ItemIDs yield
// exactly one report-only physical-duplicate issue with deterministic indexes.
func TestScanRepairIssues_DuplicatePhysicalGaItemHandle(t *testing.T) {
	const dupHandle = uint32(0x808113EE)
	slot := &SaveSlot{
		GaItems: []GaItemFull{
			{}, // empty leading record must be skipped
			{Handle: dupHandle, ItemID: 0x00000001},
			{Handle: dupHandle, ItemID: 0x00000002}, // repeat → physical duplicate
		},
	}

	var got []RepairIssue
	for _, iss := range ScanRepairIssues(4, slot) {
		if iss.Key.Code == RepairCodeDuplicatePhysicalHandle {
			got = append(got, iss)
		}
	}
	if len(got) != 1 {
		t.Fatalf("expected exactly 1 %s issue, got %d", RepairCodeDuplicatePhysicalHandle, len(got))
	}
	iss := got[0]
	if iss.Key.Handle != dupHandle {
		t.Errorf("issue handle = 0x%08X, want 0x%08X", iss.Key.Handle, dupHandle)
	}
	if iss.Key.Row != 2 || iss.Key.Value != "1" {
		t.Errorf("indexes = repeated %d first %q, want repeated 2 first \"1\"", iss.Key.Row, iss.Key.Value)
	}
	if want := "GaItem[2] reuses handle 0x808113EE from GaItem[1]"; iss.Description != want {
		t.Errorf("description = %q, want %q", iss.Description, want)
	}
	if iss.Severity != repairSeverityError {
		t.Errorf("severity = %q, want %q (real integrity problem)", iss.Severity, repairSeverityError)
	}
	if len(iss.Actions) != 1 || iss.Actions[0] != RepairActionNoAction || iss.DefaultAction != RepairActionNoAction {
		t.Errorf("issue must be report-only: actions=%v default=%q", iss.Actions, iss.DefaultAction)
	}
}

// TestScanRepairIssues_DuplicatePhysicalGaItemHandle_NoMutation confirms the scan
// leaves slot.GaItems untouched.
func TestScanRepairIssues_DuplicatePhysicalGaItemHandle_NoMutation(t *testing.T) {
	const dupHandle = uint32(0x808113EE)
	slot := &SaveSlot{
		GaItems: []GaItemFull{
			{Handle: dupHandle, ItemID: 0x00000001},
			{Handle: dupHandle, ItemID: 0x00000002},
		},
	}
	before := append([]GaItemFull(nil), slot.GaItems...)
	_ = ScanRepairIssues(4, slot)
	for i := range before {
		if slot.GaItems[i] != before[i] {
			t.Errorf("GaItems[%d] mutated by scan: %+v -> %+v", i, before[i], slot.GaItems[i])
		}
	}
}

// TestScanRepairIssues_PhysicalAndContainerDuplicatesAreDistinct confirms the two
// duplicate-handle defects use separate codes and coexist: a duplicate physical
// GaItem record (report-only) and a duplicate inventory container record (the
// existing repairable duplicate_handle).
func TestScanRepairIssues_PhysicalAndContainerDuplicatesAreDistinct(t *testing.T) {
	const physHandle = uint32(0x808113EE)
	slot := &SaveSlot{
		GaMap: map[uint32]uint32{testHandleDagger: testItemIDDagger},
		GaItems: []GaItemFull{
			{Handle: physHandle, ItemID: 0x00000001},
			{Handle: physHandle, ItemID: 0x00000002}, // physical duplicate
		},
		Inventory: EquipInventoryData{
			CommonItems: []InventoryItem{
				{GaItemHandle: testHandleDagger, Quantity: 1, Index: 500},
				{GaItemHandle: testHandleDagger, Quantity: 1, Index: 501}, // container duplicate
			},
		},
	}

	foundPhysical, foundContainer := false, false
	for _, iss := range ScanRepairIssues(0, slot) {
		switch iss.Key.Code {
		case RepairCodeDuplicatePhysicalHandle:
			foundPhysical = true
			if iss.DefaultAction != RepairActionNoAction {
				t.Errorf("physical duplicate must be report-only, default=%q", iss.DefaultAction)
			}
		case RepairCodeDuplicateHandle:
			foundContainer = true
			if iss.DefaultAction != RepairActionCreateCopy {
				t.Errorf("container duplicate_handle default changed: %q, want %q", iss.DefaultAction, RepairActionCreateCopy)
			}
		}
	}
	if !foundPhysical {
		t.Error("expected physical GaItem duplicate issue, none found")
	}
	if !foundContainer {
		t.Error("expected container duplicate_handle issue, none found")
	}
}

// TestScanRepairIssues_CleanGaItemsNoPhysicalDuplicate confirms a slot with
// distinct non-empty GaItem handles produces no physical-duplicate issue.
func TestScanRepairIssues_CleanGaItemsNoPhysicalDuplicate(t *testing.T) {
	slot := &SaveSlot{
		GaItems: []GaItemFull{
			{}, {},
			{Handle: 0x80000001, ItemID: 0x00000001},
			{Handle: 0x80000002, ItemID: 0x00000002},
		},
	}
	for _, iss := range ScanRepairIssues(0, slot) {
		if iss.Key.Code == RepairCodeDuplicatePhysicalHandle {
			t.Fatalf("clean GaItems produced a physical-duplicate issue: %+v", iss)
		}
	}
}
