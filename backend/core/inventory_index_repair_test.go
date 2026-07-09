package core

import (
	"encoding/binary"
	"testing"
)

// buildRepairFixture builds a SaveSlot with backing slot.Data sized for the full
// inventory layout, places the given Common/Key items at consecutive rows
// starting at row 0, and seeds NextEquipIndex / NextAcquisitionSortId. Both
// counter offsets are wired up so RepairDuplicateInventoryIndices can write the
// updated values back to raw bytes — same shape as a real loaded slot.
func buildRepairFixture(t *testing.T, common, key []InventoryItem) *SaveSlot {
	t.Helper()
	const magicOff = 0
	commonStart := magicOff + InvStartFromMagic
	keyStart := commonStart + CommonItemCount*InvRecordLen + InvKeyCountHeader
	nextEquipIdxOff := keyStart + KeyItemCount*InvRecordLen
	nextAcqSortIdOff := nextEquipIdxOff + 4
	bufSize := nextAcqSortIdOff + 4 + 64

	slot := &SaveSlot{
		Version:     1,
		MagicOffset: magicOff,
		Data:        make([]byte, bufSize),
		GaMap:       make(map[uint32]uint32),
	}
	for i, it := range common {
		off := commonStart + i*InvRecordLen
		binary.LittleEndian.PutUint32(slot.Data[off:], it.GaItemHandle)
		binary.LittleEndian.PutUint32(slot.Data[off+4:], it.Quantity)
		binary.LittleEndian.PutUint32(slot.Data[off+8:], it.Index)
	}
	for i, it := range key {
		off := keyStart + i*InvRecordLen
		binary.LittleEndian.PutUint32(slot.Data[off:], it.GaItemHandle)
		binary.LittleEndian.PutUint32(slot.Data[off+4:], it.Quantity)
		binary.LittleEndian.PutUint32(slot.Data[off+8:], it.Index)
	}
	slot.Inventory.CommonItems = append([]InventoryItem(nil), common...)
	slot.Inventory.KeyItems = append([]InventoryItem(nil), key...)

	var maxAcq uint32
	for _, it := range common {
		if it.Index > maxAcq {
			maxAcq = it.Index
		}
	}
	for _, it := range key {
		if it.Index > maxAcq {
			maxAcq = it.Index
		}
	}
	nextAcq := maxAcq + 1
	nextEquip := nextAcq
	binary.LittleEndian.PutUint32(slot.Data[nextEquipIdxOff:], nextEquip)
	binary.LittleEndian.PutUint32(slot.Data[nextAcqSortIdOff:], nextAcq)
	slot.Inventory.NextEquipIndex = nextEquip
	slot.Inventory.NextAcquisitionSortId = nextAcq
	slot.Inventory.nextEquipIndexOff = nextEquipIdxOff
	slot.Inventory.nextAcqSortIdOff = nextAcqSortIdOff
	return slot
}

func readRawIndex(t *testing.T, slot *SaveSlot, scope string, row int) uint32 {
	t.Helper()
	commonStart := slot.MagicOffset + InvStartFromMagic
	var off int
	switch scope {
	case "inventory_common":
		off = commonStart + row*InvRecordLen + 8
	case "inventory_key":
		off = commonStart + CommonItemCount*InvRecordLen + InvKeyCountHeader + row*InvRecordLen + 8
	default:
		t.Fatalf("unknown scope %q", scope)
	}
	return binary.LittleEndian.Uint32(slot.Data[off:])
}

func TestRepairDuplicateInventoryIndices_NilSlot(t *testing.T) {
	if _, err := RepairDuplicateInventoryIndices(nil); err == nil {
		t.Fatal("expected error for nil slot")
	}
}

func TestRepairDuplicateInventoryIndices_Clean_NoOp(t *testing.T) {
	slot := buildRepairFixture(t, []InventoryItem{
		{GaItemHandle: 0xB0000001, Quantity: 1, Index: 100},
		{GaItemHandle: 0xB0000002, Quantity: 1, Index: 101},
		{GaItemHandle: 0xB0000003, Quantity: 1, Index: 102},
	}, []InventoryItem{
		{GaItemHandle: 0xC0000001, Quantity: 1, Index: 200},
	})
	preAcq := slot.Inventory.NextAcquisitionSortId

	report, err := RepairDuplicateInventoryIndices(slot)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.Changed != 0 || len(report.Changes) != 0 {
		t.Errorf("expected no-op, got Changed=%d Changes=%+v", report.Changed, report.Changes)
	}
	if slot.Inventory.NextAcquisitionSortId != preAcq {
		t.Errorf("counter advanced on no-op: %d → %d", preAcq, slot.Inventory.NextAcquisitionSortId)
	}
	if issues := ScanDuplicateInventoryIndices(slot); len(issues) != 0 {
		t.Errorf("scanner still reports issues after no-op: %+v", issues)
	}
}

func TestRepairDuplicateInventoryIndices_DuplicateInCommon_RepairsSecondOnly(t *testing.T) {
	slot := buildRepairFixture(t, []InventoryItem{
		{GaItemHandle: 0xB0000334, Quantity: 99, Index: 552}, // first
		{GaItemHandle: 0xB00003C0, Quantity: 99, Index: 552}, // duplicate
	}, nil)

	report, err := RepairDuplicateInventoryIndices(slot)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.Changed != 1 || len(report.Changes) != 1 {
		t.Fatalf("expected 1 change, got %d: %+v", report.Changed, report.Changes)
	}
	c := report.Changes[0]
	if c.Scope != "inventory_common" || c.Row != 1 || c.Handle != 0xB00003C0 {
		t.Errorf("wrong change target: %+v", c)
	}
	if c.OldIndex != 552 {
		t.Errorf("OldIndex should be 552, got %d", c.OldIndex)
	}
	if c.NewIndex <= 552 {
		t.Errorf("NewIndex should be > 552, got %d", c.NewIndex)
	}
	if slot.Inventory.CommonItems[0].Index != 552 {
		t.Errorf("first occurrence must keep Index 552, got %d", slot.Inventory.CommonItems[0].Index)
	}
	if got := slot.Inventory.CommonItems[1].Index; got != c.NewIndex {
		t.Errorf("struct Index not updated: want %d got %d", c.NewIndex, got)
	}
	if got := readRawIndex(t, slot, "inventory_common", 1); got != c.NewIndex {
		t.Errorf("raw slot.Data not updated: want %d got %d", c.NewIndex, got)
	}
	if issues := ScanDuplicateInventoryIndices(slot); len(issues) != 0 {
		t.Errorf("scanner still reports issues: %+v", issues)
	}
}

func TestRepairDuplicateInventoryIndices_DuplicateAcrossCommonAndKey(t *testing.T) {
	slot := buildRepairFixture(t,
		[]InventoryItem{{GaItemHandle: 0xB0000001, Quantity: 1, Index: 700}},
		[]InventoryItem{{GaItemHandle: 0xC0000099, Quantity: 1, Index: 700}},
	)
	report, err := RepairDuplicateInventoryIndices(slot)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.Changed != 1 {
		t.Fatalf("expected 1 change, got %d: %+v", report.Changed, report.Changes)
	}
	c := report.Changes[0]
	if c.Scope != "inventory_key" || c.Row != 0 || c.Handle != 0xC0000099 {
		t.Errorf("expected key-side reassignment, got %+v", c)
	}
	if slot.Inventory.CommonItems[0].Index != 700 {
		t.Errorf("common-side first occurrence must be preserved")
	}
	if got := readRawIndex(t, slot, "inventory_key", 0); got != c.NewIndex {
		t.Errorf("raw key data not updated: want %d got %d", c.NewIndex, got)
	}
	if issues := ScanDuplicateInventoryIndices(slot); len(issues) != 0 {
		t.Errorf("scanner still reports issues: %+v", issues)
	}
}

func TestRepairDuplicateInventoryIndices_ManyAdjacentPairs(t *testing.T) {
	// Reproduces the shape observed in the Steam Deck cycle save:
	// 30 pairs of adjacent rows sharing one Index each.
	var common []InventoryItem
	handle := uint32(0xB0001000)
	for idx := uint32(500); idx < 530; idx++ {
		common = append(common,
			InventoryItem{GaItemHandle: handle, Quantity: 1, Index: idx},
			InventoryItem{GaItemHandle: handle + 1, Quantity: 1, Index: idx},
		)
		handle += 2
	}
	slot := buildRepairFixture(t, common, nil)

	report, err := RepairDuplicateInventoryIndices(slot)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.Changed != 30 {
		t.Fatalf("expected 30 reassignments, got %d", report.Changed)
	}
	if issues := ScanDuplicateInventoryIndices(slot); len(issues) != 0 {
		t.Fatalf("scanner still reports %d issues after repair", len(issues))
	}

	// All reassigned Indices must be unique and strictly greater than every
	// preserved (odd-row) Index. First occurrences live at even rows.
	maxOriginal := uint32(529)
	seen := make(map[uint32]bool)
	for _, c := range report.Changes {
		if c.NewIndex <= maxOriginal {
			t.Errorf("NewIndex %d not > maxOriginal %d", c.NewIndex, maxOriginal)
		}
		if seen[c.NewIndex] {
			t.Errorf("duplicate NewIndex %d across reassignments", c.NewIndex)
		}
		seen[c.NewIndex] = true
	}
	// NextAcquisitionSortId must stay strictly greater than every assigned Index
	// (it is the source counter for fresh indices). NextEquipIndex must be
	// PRESERVED — repair renumbers duplicates but must not touch the equip-list
	// counter (forcing it up corrupts the slot, CE-108255-1). The fixture seeds
	// NextEquipIndex = maxOriginal+1 = 530, so it stays below the reassigned
	// (>530) indices, which is correct.
	const wantPreservedEquip = uint32(530)
	if slot.Inventory.NextEquipIndex != wantPreservedEquip {
		t.Errorf("NextEquipIndex must be preserved: want %d, got %d",
			wantPreservedEquip, slot.Inventory.NextEquipIndex)
	}
	for _, it := range slot.Inventory.CommonItems {
		if it.GaItemHandle == GaHandleEmpty {
			continue
		}
		if it.Index >= slot.Inventory.NextAcquisitionSortId {
			t.Errorf("NextAcquisitionSortId=%d not > item Index %d",
				slot.Inventory.NextAcquisitionSortId, it.Index)
		}
	}
}

func TestRepairDuplicateInventoryIndices_Idempotent(t *testing.T) {
	slot := buildRepairFixture(t, []InventoryItem{
		{GaItemHandle: 0xB0000010, Quantity: 1, Index: 500},
		{GaItemHandle: 0xB0000020, Quantity: 1, Index: 500},
		{GaItemHandle: 0xB0000030, Quantity: 1, Index: 600},
		{GaItemHandle: 0xB0000040, Quantity: 1, Index: 600},
	}, nil)

	r1, err := RepairDuplicateInventoryIndices(slot)
	if err != nil {
		t.Fatalf("first repair: %v", err)
	}
	if r1.Changed != 2 {
		t.Fatalf("first repair: expected 2 changes, got %d", r1.Changed)
	}

	r2, err := RepairDuplicateInventoryIndices(slot)
	if err != nil {
		t.Fatalf("second repair: %v", err)
	}
	if r2.Changed != 0 || len(r2.Changes) != 0 {
		t.Errorf("second repair must be no-op, got %+v", r2)
	}
}

func TestRepairDuplicateInventoryIndices_UpdatesRawSlotData(t *testing.T) {
	slot := buildRepairFixture(t, []InventoryItem{
		{GaItemHandle: 0xB0000001, Quantity: 1, Index: 800},
		{GaItemHandle: 0xB0000002, Quantity: 1, Index: 800},
	}, []InventoryItem{
		{GaItemHandle: 0xC0000001, Quantity: 1, Index: 900},
		{GaItemHandle: 0xC0000002, Quantity: 1, Index: 900},
	})
	report, err := RepairDuplicateInventoryIndices(slot)
	if err != nil {
		t.Fatalf("repair: %v", err)
	}
	if report.Changed != 2 {
		t.Fatalf("expected 2 changes, got %d", report.Changed)
	}
	// Verify raw bytes for each reassigned row match the in-memory struct.
	for _, c := range report.Changes {
		raw := readRawIndex(t, slot, c.Scope, c.Row)
		if raw != c.NewIndex {
			t.Errorf("%s row %d: raw=%d struct=%d", c.Scope, c.Row, raw, c.NewIndex)
		}
		var structIdx uint32
		switch c.Scope {
		case "inventory_common":
			structIdx = slot.Inventory.CommonItems[c.Row].Index
		case "inventory_key":
			structIdx = slot.Inventory.KeyItems[c.Row].Index
		}
		if structIdx != c.NewIndex {
			t.Errorf("%s row %d: in-memory struct=%d but change reported %d",
				c.Scope, c.Row, structIdx, c.NewIndex)
		}
	}
	// Also verify counter raw-byte write-back.
	rawAcq := binary.LittleEndian.Uint32(slot.Data[slot.Inventory.nextAcqSortIdOff:])
	if rawAcq != slot.Inventory.NextAcquisitionSortId {
		t.Errorf("raw NextAcquisitionSortId=%d != struct %d",
			rawAcq, slot.Inventory.NextAcquisitionSortId)
	}
}

func TestRepairDuplicateInventoryIndices_IgnoresEmptyHandles(t *testing.T) {
	// Empty / invalid handles share Index 552 but must NOT be touched —
	// they are placeholders, not real items.
	slot := buildRepairFixture(t, []InventoryItem{
		{GaItemHandle: GaHandleEmpty, Quantity: 0, Index: 552},
		{GaItemHandle: 0xB0000001, Quantity: 1, Index: 552},
		{GaItemHandle: GaHandleInvalid, Quantity: 0, Index: 552},
		{GaItemHandle: 0xB0000002, Quantity: 1, Index: 552},
	}, nil)
	report, err := RepairDuplicateInventoryIndices(slot)
	if err != nil {
		t.Fatalf("repair: %v", err)
	}
	// Only row 3 should be reassigned: rows 0, 2 are ignored (empty); row 1
	// is the first real occurrence and stays at 552.
	if report.Changed != 1 {
		t.Fatalf("expected 1 change, got %d: %+v", report.Changed, report.Changes)
	}
	if report.Changes[0].Row != 3 || report.Changes[0].Handle != 0xB0000002 {
		t.Errorf("wrong row reassigned: %+v", report.Changes[0])
	}
	if slot.Inventory.CommonItems[0].Index != 552 || slot.Inventory.CommonItems[2].Index != 552 {
		t.Errorf("empty rows must keep their original Index")
	}
}

// ---- AssignFreshInventoryIndex ---------------------------------------------

func TestAssignFreshInventoryIndex_NilSlot(t *testing.T) {
	if _, err := AssignFreshInventoryIndex(nil, "inventory_common", 0); err == nil {
		t.Fatal("expected error for nil slot")
	}
}

func TestAssignFreshInventoryIndex_UnknownScope(t *testing.T) {
	slot := buildRepairFixture(t, []InventoryItem{
		{GaItemHandle: 0xB0000001, Quantity: 1, Index: 500},
	}, nil)
	if _, err := AssignFreshInventoryIndex(slot, "storage_common", 0); err == nil {
		t.Fatal("expected error for unknown scope")
	}
}

func TestAssignFreshInventoryIndex_RowOutOfBounds(t *testing.T) {
	slot := buildRepairFixture(t, []InventoryItem{
		{GaItemHandle: 0xB0000001, Quantity: 1, Index: 500},
	}, nil)
	if _, err := AssignFreshInventoryIndex(slot, "inventory_common", 5); err == nil {
		t.Fatal("expected error for out-of-bounds row")
	}
}

func TestAssignFreshInventoryIndex_EmptyHandle(t *testing.T) {
	slot := buildRepairFixture(t, []InventoryItem{
		{GaItemHandle: GaHandleEmpty, Quantity: 0, Index: 500},
	}, nil)
	if _, err := AssignFreshInventoryIndex(slot, "inventory_common", 0); err == nil {
		t.Fatal("expected error for empty handle")
	}
}

// TestAssignFreshInventoryIndex_LowIndexGetsFreshEditorIndex verifies that when
// the editor explicitly reassigns one record, the fresh index stays above both
// InvEquipReservedMax and all existing indices. This does not imply that
// pre-existing low indices are corrupt.
func TestAssignFreshInventoryIndex_LowIndexGetsFreshEditorIndex(t *testing.T) {
	const lowIdx = uint32(100)  // legal existing low index, but not reused for a fresh editor index
	const highIdx = uint32(600) // existing high-watermark item

	slot := buildRepairFixture(t, []InventoryItem{
		{GaItemHandle: 0xB0000001, Quantity: 1, Index: lowIdx},  // target: assign a fresh editor index
		{GaItemHandle: 0xB0000002, Quantity: 1, Index: highIdx}, // unrelated high item
	}, nil)
	preEquip := slot.Inventory.NextEquipIndex

	change, err := AssignFreshInventoryIndex(slot, "inventory_common", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if change.OldIndex != lowIdx {
		t.Errorf("OldIndex = %d, want %d", change.OldIndex, lowIdx)
	}
	if change.NewIndex <= InvEquipReservedMax {
		t.Errorf("NewIndex %d must be > InvEquipReservedMax (%d)", change.NewIndex, InvEquipReservedMax)
	}
	if change.NewIndex <= highIdx {
		t.Errorf("NewIndex %d must be > max existing index %d", change.NewIndex, highIdx)
	}
	// Raw slot.Data must match.
	if got := readRawIndex(t, slot, "inventory_common", 0); got != change.NewIndex {
		t.Errorf("raw slot.Data not updated: got %d, want %d", got, change.NewIndex)
	}
	// In-memory struct must match.
	if slot.Inventory.CommonItems[0].Index != change.NewIndex {
		t.Errorf("in-memory Index not updated: got %d, want %d",
			slot.Inventory.CommonItems[0].Index, change.NewIndex)
	}
	// NextEquipIndex must be preserved (CE-108255-1).
	if slot.Inventory.NextEquipIndex != preEquip {
		t.Errorf("NextEquipIndex changed: %d → %d", preEquip, slot.Inventory.NextEquipIndex)
	}
	// NextAcquisitionSortId must be > new index.
	if slot.Inventory.NextAcquisitionSortId <= change.NewIndex {
		t.Errorf("NextAcquisitionSortId %d not > new index %d",
			slot.Inventory.NextAcquisitionSortId, change.NewIndex)
	}
}

func TestAssignFreshInventoryIndex_DuplicateIndex_AssignsUnique(t *testing.T) {
	slot := buildRepairFixture(t, []InventoryItem{
		{GaItemHandle: 0xB0000001, Quantity: 1, Index: 552}, // first — keep
		{GaItemHandle: 0xB0000002, Quantity: 1, Index: 552}, // second — target
	}, nil)

	change, err := AssignFreshInventoryIndex(slot, "inventory_common", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if change.OldIndex != 552 {
		t.Errorf("OldIndex = %d, want 552", change.OldIndex)
	}
	if change.NewIndex <= 552 {
		t.Errorf("NewIndex %d must be > 552", change.NewIndex)
	}
	// First row untouched.
	if slot.Inventory.CommonItems[0].Index != 552 {
		t.Errorf("first row must keep Index 552, got %d", slot.Inventory.CommonItems[0].Index)
	}
	// No duplicates remain.
	if issues := ScanDuplicateInventoryIndices(slot); len(issues) != 0 {
		t.Errorf("scanner still reports %d issues after single-record repair", len(issues))
	}
}

func TestAssignFreshInventoryIndex_KeyScope(t *testing.T) {
	slot := buildRepairFixture(t,
		[]InventoryItem{{GaItemHandle: 0xB0000001, Quantity: 1, Index: 700}},
		[]InventoryItem{{GaItemHandle: 0xC0000099, Quantity: 1, Index: 700}},
	)
	change, err := AssignFreshInventoryIndex(slot, "inventory_key", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if change.Scope != "inventory_key" {
		t.Errorf("Scope = %q, want inventory_key", change.Scope)
	}
	if change.NewIndex <= 700 {
		t.Errorf("NewIndex %d must be > 700", change.NewIndex)
	}
	if got := readRawIndex(t, slot, "inventory_key", 0); got != change.NewIndex {
		t.Errorf("raw key data not updated: got %d want %d", got, change.NewIndex)
	}
	if issues := ScanDuplicateInventoryIndices(slot); len(issues) != 0 {
		t.Errorf("scanner still reports issues: %+v", issues)
	}
}

// TestMapInventoryReconcilesKeyItemIndex reproduces Bug #2: NextAcquisitionSortId was
// reconciled against CommonItems only, so a KeyItem with a higher Index was invisible
// to the reconcile. The next addToInventory would then assign an Index that collides
// with the KeyItem, producing a duplicate acquisition entry.
func TestMapInventoryReconcilesKeyItemIndex(t *testing.T) {
	common := []InventoryItem{{GaItemHandle: 0xB0000001, Quantity: 1, Index: 500}}
	key := []InventoryItem{{GaItemHandle: 0xC0000001, Quantity: 1, Index: 700}}
	slot := buildRepairFixture(t, common, key)

	// Simulate the stale value: only CommonItems max (500) was counted → 501.
	stale := uint32(501)
	slot.Inventory.NextAcquisitionSortId = stale
	binary.LittleEndian.PutUint32(slot.Data[slot.Inventory.nextAcqSortIdOff:], stale)

	if err := slot.mapInventory(); err != nil {
		t.Fatalf("mapInventory: %v", err)
	}
	if slot.Inventory.NextAcquisitionSortId <= 700 {
		t.Errorf("NextAcquisitionSortId not reconciled past KeyItem Index 700: got %d", slot.Inventory.NextAcquisitionSortId)
	}
}
