package core

import "testing"

// ---- talisman identity red tests (test-first, see conversation report) -----
//
// Talisman handles (0xA0 prefix) are derived directly from the item ID
// (db.HandleToItemID), not allocated per-instance like weapon/armor GaItems.
// Every copy of the same talisman — in the same container, or split across
// inventory and storage — carries the IDENTICAL handle by design. That is
// normal save state, not corruption, and must not be reported the same way
// TestScanRepairIssues_DuplicateHandle treats a repeated goods handle.

// talismanHandle / talismanItemID — "Sacrificial Twig" (backend/db/data/talismans.go).
// Handle = 0xA0 prefix | (itemID & 0x0FFFFFFF): 0xA0000000 | 0x000017B6.
const (
	talismanHandle = uint32(0xA00017B6)
)

// TestScanRepairIssues_TalismanDuplicateHandle_NotAnError is RED against the
// current scanner: scanInventoryRepairIssues (repair_scanner.go) applies the
// same seenHandles dedup to ItemTypeAccessory as it does to weapons/armor/
// goods, so two legitimate Sacrificial Twig stacks in the same container are
// currently reported as duplicate_handle + duplicate_uid.
func TestScanRepairIssues_TalismanDuplicateHandle_NotAnError(t *testing.T) {
	slot := &SaveSlot{
		GaMap: map[uint32]uint32{},
		Inventory: EquipInventoryData{
			CommonItems: []InventoryItem{
				{GaItemHandle: talismanHandle, Quantity: 1, Index: 500},
				{GaItemHandle: talismanHandle, Quantity: 1, Index: 502}, // legitimate second copy
			},
		},
	}

	issues := ScanRepairIssues(0, slot)

	for _, iss := range issues {
		if iss.Key.Code == RepairCodeDuplicateHandle || iss.Key.Code == RepairCodeDuplicateUID {
			t.Errorf("talisman handle 0x%08X repeated in the same container is normal (handle is item-derived, not instance-unique) — must not be reported as %s: %q",
				talismanHandle, iss.Key.Code, iss.Description)
		}
	}
}
