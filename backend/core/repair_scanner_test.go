package core

import (
	"testing"
)

// Real item IDs from backend/db/data — used to avoid false unknown_item_id in tests.
// 0xB0002774 → HandleToItemID → 0x40002774 = "Smithing Stone [1]" (bolstering_materials)
const (
	testHandleSmithingStone = uint32(0xB0002774)
	testItemIDSmithingStone = uint32(0x40002774)
)

// ---- regression test --------------------------------------------------------

// TestScanRepairIssues_DuplicateHandle is the early regression anchor.
//
// Regression: duplicate_handle and duplicate_uid were reported by editor.Validate
// with CanRepair=false, making them appear as "unrepairable" in the UI. This test
// confirms the core scanner emits both codes with proposed actions.
func TestScanRepairIssues_DuplicateHandle(t *testing.T) {
	h := uint32(testHandleSmithingStone)
	slot := &SaveSlot{
		Inventory: EquipInventoryData{
			CommonItems: []InventoryItem{
				{GaItemHandle: h, Quantity: 5, Index: 500},
				{GaItemHandle: h, Quantity: 5, Index: 501}, // duplicate → regression
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
			if len(iss.Actions) == 0 {
				t.Errorf("duplicate_uid must have proposed actions, got none")
			}
		}
	}
	if !foundHandle {
		t.Error("expected duplicate_handle issue, scanner returned none")
	}
	if !foundUID {
		t.Error("expected duplicate_uid issue alongside duplicate_handle, scanner returned none")
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
			Level:        2, // mismatch → should trigger
			Vigor:        10, Mind: 10, Endurance: 10,
			Strength:     10, Dexterity: 10, Intelligence: 10,
			Faith:        10, Arcane: 10,
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

// ---- determinism ------------------------------------------------------------

// TestScanRepairIssues_IssueIDDeterministic confirms issueID is stable across runs.
func TestScanRepairIssues_IssueIDDeterministic(t *testing.T) {
	h := testHandleSmithingStone
	slot := &SaveSlot{
		Inventory: EquipInventoryData{
			CommonItems: []InventoryItem{
				{GaItemHandle: h, Quantity: 5, Index: 500},
				{GaItemHandle: h, Quantity: 5, Index: 501},
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
