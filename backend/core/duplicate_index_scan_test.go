package core

import "testing"

func newSlotWithItems(common, key []InventoryItem) *SaveSlot {
	s := &SaveSlot{}
	s.Inventory.CommonItems = append(s.Inventory.CommonItems, common...)
	s.Inventory.KeyItems = append(s.Inventory.KeyItems, key...)
	return s
}

func TestScanDuplicateInventoryIndices_Clean(t *testing.T) {
	slot := newSlotWithItems(
		[]InventoryItem{
			{GaItemHandle: 0xB0000001, Quantity: 1, Index: 100},
			{GaItemHandle: 0xB0000002, Quantity: 1, Index: 101},
			{GaItemHandle: 0xB0000003, Quantity: 1, Index: 102},
		},
		[]InventoryItem{
			{GaItemHandle: 0xC0000001, Quantity: 1, Index: 200},
		},
	)
	if issues := ScanDuplicateInventoryIndices(slot); len(issues) != 0 {
		t.Fatalf("expected 0 issues on clean slot, got %d: %+v", len(issues), issues)
	}
}

func TestScanDuplicateInventoryIndices_IgnoresEmptyHandles(t *testing.T) {
	slot := newSlotWithItems(
		[]InventoryItem{
			{GaItemHandle: GaHandleEmpty, Quantity: 0, Index: 552},
			{GaItemHandle: GaHandleInvalid, Quantity: 0, Index: 552},
			{GaItemHandle: 0xB0000001, Quantity: 1, Index: 552},
		},
		nil,
	)
	if issues := ScanDuplicateInventoryIndices(slot); len(issues) != 0 {
		t.Fatalf("empty/invalid handles must be ignored even when their Index collides; got %d", len(issues))
	}
}

func TestScanDuplicateInventoryIndices_DuplicateInCommon(t *testing.T) {
	slot := newSlotWithItems(
		[]InventoryItem{
			{GaItemHandle: 0xB0000334, Quantity: 99, Index: 552}, // row 0 — Boiled Crab analogue
			{GaItemHandle: 0xB00003C0, Quantity: 99, Index: 552}, // row 1 — Clarifying Boluses analogue
		},
		nil,
	)
	issues := ScanDuplicateInventoryIndices(slot)
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue, got %d: %+v", len(issues), issues)
	}
	got := issues[0]
	if got.Index != 552 || got.Scope != "inventory_common" {
		t.Errorf("unexpected issue scope/index: %+v", got)
	}
	if got.FirstRow != 0 || got.FirstHandle != 0xB0000334 {
		t.Errorf("first occurrence mismatch: %+v", got)
	}
	if got.DuplicateRow != 1 || got.DuplicateHandle != 0xB00003C0 {
		t.Errorf("duplicate occurrence mismatch: %+v", got)
	}
}

func TestScanDuplicateInventoryIndices_DuplicateAcrossCommonAndKey(t *testing.T) {
	slot := newSlotWithItems(
		[]InventoryItem{
			{GaItemHandle: 0xB0000001, Quantity: 1, Index: 700},
		},
		[]InventoryItem{
			{GaItemHandle: 0xC0000099, Quantity: 1, Index: 700},
		},
	)
	issues := ScanDuplicateInventoryIndices(slot)
	if len(issues) != 1 {
		t.Fatalf("expected 1 cross-list collision, got %d", len(issues))
	}
	if issues[0].Scope != "inventory_key" || issues[0].DuplicateHandle != 0xC0000099 {
		t.Errorf("expected key-side duplicate, got %+v", issues[0])
	}
	if issues[0].FirstHandle != 0xB0000001 {
		t.Errorf("first handle should point to the common-side entry: %+v", issues[0])
	}
}

func TestScanDuplicateInventoryIndices_ManyDuplicates(t *testing.T) {
	// Reproduces the shape observed in the Steam Deck post-cycle save: many pairs
	// of adjacent rows sharing one Index. The scanner must surface every issue
	// (not stop at the first), so the caller can report the total count.
	var common []InventoryItem
	handle := uint32(0xB0001000)
	for idx := uint32(500); idx < 530; idx++ {
		common = append(common,
			InventoryItem{GaItemHandle: handle, Quantity: 1, Index: idx},
			InventoryItem{GaItemHandle: handle + 1, Quantity: 1, Index: idx},
		)
		handle += 2
	}
	slot := newSlotWithItems(common, nil)
	issues := ScanDuplicateInventoryIndices(slot)
	if len(issues) != 30 {
		t.Fatalf("expected 30 issues (one per paired Index), got %d", len(issues))
	}
}

func TestScanDuplicateInventoryIndices_NilSlot(t *testing.T) {
	if issues := ScanDuplicateInventoryIndices(nil); issues != nil {
		t.Errorf("expected nil for nil slot, got %+v", issues)
	}
}
