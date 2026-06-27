package main

import (
	"strings"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
	"github.com/oisis/EldenRing-SaveForge/backend/db"
)

// integrityFixture builds an App whose slot 0 is in-memory only (no slot.Data
// buffer). The integrity scan reads slot.Inventory.{CommonItems,KeyItems}
// directly and does not need a backing binary, so this is sufficient for
// every test except those that exercise RepairDuplicateInventoryIndices
// (which writes back to slot.Data and uses repairFixture).
func integrityFixture() *App {
	app := NewApp()
	app.save = &core.SaveFile{}
	app.save.Slots[0].Version = 1
	app.save.Slots[0].Inventory.NextAcquisitionSortId = 1000
	app.save.Slots[0].Inventory.NextEquipIndex = 1000
	return app
}

// knownGoodsHandles picks N distinct goods-category items the DB recognises
// and returns their inventory handles (handle = 0xB0 | low24 of itemID). Used
// to exercise the "known item" enrichment path without hard-coding IDs.
func knownGoodsHandles(t *testing.T, n int) []uint32 {
	t.Helper()
	items := db.GetAllItems("PC")
	var handles []uint32
	seen := make(map[uint32]bool)
	for _, it := range items {
		if it.Name == "" {
			continue
		}
		if it.Category != "tools" && it.Category != "crafting_materials" {
			continue
		}
		if it.ID&0xF0000000 != 0x40000000 {
			continue
		}
		if seen[it.ID] {
			continue
		}
		seen[it.ID] = true
		handle := (it.ID & 0x0FFFFFFF) | 0xB0000000
		handles = append(handles, handle)
		if len(handles) == n {
			return handles
		}
	}
	t.Fatalf("knownGoodsHandles: only resolved %d known goods, need %d", len(handles), n)
	return nil
}

// unknownHandle returns a handle whose ItemID is guaranteed to miss the DB so
// the scan can verify the Unknown=true / ItemID-handle-fallback path.
func unknownHandle() uint32 {
	// 0x0FFFFFFE is outside every known itemID range; combined with the goods
	// prefix it stays a non-empty handle that ScanDuplicateInventoryIndices
	// will not skip as GaHandleEmpty/GaHandleInvalid.
	return 0xB0FFFFFE
}

func TestGetSaveInventoryIntegrityReport_NoSave(t *testing.T) {
	app := NewApp()
	_, err := app.GetSaveInventoryIntegrityReport()
	if err == nil {
		t.Fatalf("expected error when no save is loaded")
	}
	if !strings.Contains(err.Error(), "no save loaded") {
		t.Errorf("expected 'no save loaded', got %q", err.Error())
	}
}

func TestGetSaveInventoryIntegrityReport_AllSlotsClean_ReturnsClean(t *testing.T) {
	app := integrityFixture()
	handles := knownGoodsHandles(t, 3)
	slot := &app.save.Slots[0]
	for i, h := range handles {
		slot.Inventory.CommonItems = append(slot.Inventory.CommonItems, core.InventoryItem{
			GaItemHandle: h, Quantity: 1, Index: uint32(500 + i),
		})
	}

	report, err := app.GetSaveInventoryIntegrityReport()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !report.Clean {
		t.Errorf("Clean should be true for a slot without duplicates, got Slots=%+v", report.Slots)
	}
	if len(report.Slots) != 0 {
		t.Errorf("Slots should be empty when Clean, got %d entries", len(report.Slots))
	}
}

func TestGetSaveInventoryIntegrityReport_SingleConflict_KnownItems(t *testing.T) {
	app := integrityFixture()
	handles := knownGoodsHandles(t, 2)
	slot := &app.save.Slots[0]
	slot.Inventory.CommonItems = []core.InventoryItem{
		{GaItemHandle: handles[0], Quantity: 5, Index: 552},
		{GaItemHandle: handles[1], Quantity: 7, Index: 552},
	}

	report, err := app.GetSaveInventoryIntegrityReport()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.Clean {
		t.Fatalf("Clean should be false when a slot has duplicates")
	}
	if len(report.Slots) != 1 {
		t.Fatalf("expected 1 affected slot, got %d", len(report.Slots))
	}
	s := report.Slots[0]
	if s.SlotIndex != 0 {
		t.Errorf("SlotIndex: want 0, got %d", s.SlotIndex)
	}
	if s.DuplicateEntryCount != 1 {
		t.Errorf("DuplicateEntryCount: want 1 (additional occurrences), got %d", s.DuplicateEntryCount)
	}
	if s.ConflictingIndexCount != 1 {
		t.Errorf("ConflictingIndexCount: want 1, got %d", s.ConflictingIndexCount)
	}
	if len(s.Conflicts) != 1 {
		t.Fatalf("Conflicts: want 1 group, got %d", len(s.Conflicts))
	}
	c := s.Conflicts[0]
	if c.Index != 552 {
		t.Errorf("Conflict.Index: want 552, got %d", c.Index)
	}
	if len(c.Items) != 2 {
		t.Fatalf("Conflict.Items should contain first + duplicate occurrence (2 total), got %d", len(c.Items))
	}
	rows := map[int]bool{}
	for _, it := range c.Items {
		if it.Scope != "inventory_common" {
			t.Errorf("Item scope: want inventory_common, got %q", it.Scope)
		}
		if it.Name == "" || it.Category == "" {
			t.Errorf("Known item should carry Name and Category, got %+v", it)
		}
		if it.Unknown {
			t.Errorf("Known item must not be flagged Unknown: %+v", it)
		}
		if it.Quantity == 0 {
			t.Errorf("Quantity should reflect the inventory record, got 0 for %+v", it)
		}
		rows[it.Row] = true
	}
	if !rows[0] || !rows[1] {
		t.Errorf("Conflict.Items should include both rows 0 and 1, got rows=%v", rows)
	}
}

func TestGetSaveInventoryIntegrityReport_TripleOccurrence_FirstNotDoubled(t *testing.T) {
	app := integrityFixture()
	handles := knownGoodsHandles(t, 3)
	slot := &app.save.Slots[0]
	for _, h := range handles {
		slot.Inventory.CommonItems = append(slot.Inventory.CommonItems, core.InventoryItem{
			GaItemHandle: h, Quantity: 1, Index: 700,
		})
	}

	report, err := app.GetSaveInventoryIntegrityReport()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(report.Slots) != 1 {
		t.Fatalf("expected 1 affected slot, got %d", len(report.Slots))
	}
	s := report.Slots[0]
	if s.DuplicateEntryCount != 2 {
		t.Errorf("DuplicateEntryCount for triple occurrence: want 2 (additional), got %d", s.DuplicateEntryCount)
	}
	if s.ConflictingIndexCount != 1 {
		t.Errorf("ConflictingIndexCount: want 1, got %d", s.ConflictingIndexCount)
	}
	if len(s.Conflicts) != 1 {
		t.Fatalf("Conflicts: want 1 group, got %d", len(s.Conflicts))
	}
	items := s.Conflicts[0].Items
	if len(items) != 3 {
		t.Fatalf("Items: want 3 unique occurrences, got %d", len(items))
	}
	seenRows := map[int]int{}
	for _, it := range items {
		seenRows[it.Row]++
	}
	for row, count := range seenRows {
		if count != 1 {
			t.Errorf("row %d appears %d times in Items; first occurrence must not be duplicated", row, count)
		}
	}
	if len(seenRows) != 3 {
		t.Errorf("expected 3 distinct rows (0,1,2), got %v", seenRows)
	}
}

func TestGetSaveInventoryIntegrityReport_TwoDistinctConflicts(t *testing.T) {
	app := integrityFixture()
	handles := knownGoodsHandles(t, 4)
	slot := &app.save.Slots[0]
	slot.Inventory.CommonItems = []core.InventoryItem{
		{GaItemHandle: handles[0], Quantity: 1, Index: 552},
		{GaItemHandle: handles[1], Quantity: 1, Index: 552},
		{GaItemHandle: handles[2], Quantity: 1, Index: 600},
		{GaItemHandle: handles[3], Quantity: 1, Index: 600},
	}

	report, err := app.GetSaveInventoryIntegrityReport()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(report.Slots) != 1 {
		t.Fatalf("expected 1 affected slot, got %d", len(report.Slots))
	}
	s := report.Slots[0]
	if s.DuplicateEntryCount != 2 {
		t.Errorf("DuplicateEntryCount: want 2 (1 per pair), got %d", s.DuplicateEntryCount)
	}
	if s.ConflictingIndexCount != 2 {
		t.Errorf("ConflictingIndexCount: want 2 distinct conflicting indices, got %d", s.ConflictingIndexCount)
	}
	if len(s.Conflicts) != 2 {
		t.Fatalf("Conflicts: want 2 groups, got %d", len(s.Conflicts))
	}
	indices := map[uint32]int{}
	for _, c := range s.Conflicts {
		indices[c.Index] = len(c.Items)
	}
	if indices[552] != 2 || indices[600] != 2 {
		t.Errorf("each conflict group should carry exactly 2 items, got %+v", indices)
	}
}

func TestGetSaveInventoryIntegrityReport_KeyItemsScope(t *testing.T) {
	app := integrityFixture()
	handles := knownGoodsHandles(t, 2)
	slot := &app.save.Slots[0]
	slot.Inventory.KeyItems = []core.InventoryItem{
		{GaItemHandle: handles[0], Quantity: 1, Index: 800},
		{GaItemHandle: handles[1], Quantity: 1, Index: 800},
	}

	report, err := app.GetSaveInventoryIntegrityReport()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(report.Slots) != 1 {
		t.Fatalf("expected 1 affected slot, got %d", len(report.Slots))
	}
	if len(report.Slots[0].Conflicts) != 1 {
		t.Fatalf("expected 1 conflict group, got %d", len(report.Slots[0].Conflicts))
	}
	for _, it := range report.Slots[0].Conflicts[0].Items {
		if it.Scope != "inventory_key" {
			t.Errorf("Scope: want inventory_key, got %q", it.Scope)
		}
	}
}

func TestGetSaveInventoryIntegrityReport_UnknownItem_Reported(t *testing.T) {
	app := integrityFixture()
	known := knownGoodsHandles(t, 1)
	uh := unknownHandle()
	slot := &app.save.Slots[0]
	slot.Inventory.CommonItems = []core.InventoryItem{
		{GaItemHandle: known[0], Quantity: 1, Index: 900},
		{GaItemHandle: uh, Quantity: 3, Index: 900},
	}

	report, err := app.GetSaveInventoryIntegrityReport()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(report.Slots) != 1 || len(report.Slots[0].Conflicts) != 1 {
		t.Fatalf("expected 1 slot with 1 conflict, got %+v", report)
	}
	items := report.Slots[0].Conflicts[0].Items
	if len(items) != 2 {
		t.Fatalf("expected both rows in conflict (known + unknown), got %d", len(items))
	}
	var unknownItem *InventoryIntegrityConflictItem
	for i := range items {
		if items[i].Handle == uh {
			unknownItem = &items[i]
			break
		}
	}
	if unknownItem == nil {
		t.Fatalf("unknown item missing from Conflict.Items; got %+v", items)
	}
	if !unknownItem.Unknown {
		t.Errorf("unknown item should have Unknown=true, got %+v", *unknownItem)
	}
	if unknownItem.Handle == 0 || unknownItem.ItemID == 0 {
		t.Errorf("unknown item must keep Handle and ItemID for fallback, got %+v", *unknownItem)
	}
	if unknownItem.Quantity != 3 {
		t.Errorf("unknown item Quantity should reflect the record, got %d", unknownItem.Quantity)
	}
}

func TestGetSaveInventoryIntegrityReport_MultiSlot(t *testing.T) {
	app := integrityFixture()
	handles := knownGoodsHandles(t, 4)

	// Slot 0 stays populated by the fixture; add a duplicate pair.
	slot0 := &app.save.Slots[0]
	slot0.Inventory.CommonItems = []core.InventoryItem{
		{GaItemHandle: handles[0], Quantity: 1, Index: 552},
		{GaItemHandle: handles[1], Quantity: 1, Index: 552},
	}

	// Slot 2 is brought online with its own conflict.
	slot2 := &app.save.Slots[2]
	slot2.Version = 1
	slot2.Inventory.NextAcquisitionSortId = 1500
	slot2.Inventory.NextEquipIndex = 1500
	slot2.Inventory.CommonItems = []core.InventoryItem{
		{GaItemHandle: handles[2], Quantity: 1, Index: 700},
		{GaItemHandle: handles[3], Quantity: 1, Index: 700},
	}

	report, err := app.GetSaveInventoryIntegrityReport()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(report.Slots) != 2 {
		t.Fatalf("expected 2 affected slots, got %d (%+v)", len(report.Slots), report.Slots)
	}
	seen := map[int]bool{}
	for _, s := range report.Slots {
		seen[s.SlotIndex] = true
	}
	if !seen[0] || !seen[2] {
		t.Errorf("expected slots 0 and 2 in report, got %v", seen)
	}
}

func TestGetSaveInventoryIntegrityReport_RepairThenReScan_Clean(t *testing.T) {
	app := repairFixture(
		[]core.InventoryItem{
			{GaItemHandle: 0xB0000010, Quantity: 1, Index: 500},
			{GaItemHandle: 0xB0000020, Quantity: 1, Index: 500},
		},
		nil,
	)
	pre, err := app.GetSaveInventoryIntegrityReport()
	if err != nil {
		t.Fatalf("pre-repair: %v", err)
	}
	if pre.Clean {
		t.Fatalf("pre-repair report should be dirty")
	}

	if _, err := app.RepairDuplicateInventoryIndices(0); err != nil {
		t.Fatalf("repair: %v", err)
	}

	post, err := app.GetSaveInventoryIntegrityReport()
	if err != nil {
		t.Fatalf("post-repair: %v", err)
	}
	if !post.Clean {
		t.Errorf("post-repair report should be Clean, got Slots=%+v", post.Slots)
	}
	if len(post.Slots) != 0 {
		t.Errorf("post-repair Slots should be empty, got %d", len(post.Slots))
	}

	// Defense-in-depth for the NextEquipIndex regression (CE-108255-1): the
	// repair must NEVER advance NextEquipIndex, even though it advances
	// NextAcquisitionSortId so future adds skip the freshly-assigned indices.
	slot := &app.save.Slots[0]
	if slot.Inventory.NextEquipIndex != 501 {
		// repairFixture seeds NextEquipIndex = maxIndex + 1 = 501 for two rows at Index 500.
		t.Errorf("NextEquipIndex must remain untouched by repair; want 501, got %d",
			slot.Inventory.NextEquipIndex)
	}
}

// TestGetSaveInventoryIntegrityReport_CrossScopeConflict pins a duplicate
// acquisition Index shared between Inventory.CommonItems and Inventory.KeyItems.
// core.ScanDuplicateInventoryIndices walks the two slices as a single Index
// space, so the report must surface ONE conflict group with BOTH scopes
// represented in Items.
func TestGetSaveInventoryIntegrityReport_CrossScopeConflict(t *testing.T) {
	app := integrityFixture()
	handles := knownGoodsHandles(t, 2)
	slot := &app.save.Slots[0]
	slot.Inventory.CommonItems = []core.InventoryItem{
		{GaItemHandle: handles[0], Quantity: 1, Index: 777},
	}
	slot.Inventory.KeyItems = []core.InventoryItem{
		{GaItemHandle: handles[1], Quantity: 1, Index: 777},
	}

	report, err := app.GetSaveInventoryIntegrityReport()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(report.Slots) != 1 {
		t.Fatalf("expected 1 affected slot, got %d", len(report.Slots))
	}
	s := report.Slots[0]
	if s.DuplicateEntryCount != 1 || s.ConflictingIndexCount != 1 {
		t.Errorf("counters: DEC=%d, CIC=%d (want 1/1)", s.DuplicateEntryCount, s.ConflictingIndexCount)
	}
	if len(s.Conflicts) != 1 {
		t.Fatalf("expected 1 conflict group for cross-scope duplicate, got %d", len(s.Conflicts))
	}
	c := s.Conflicts[0]
	if c.Index != 777 {
		t.Errorf("Conflict.Index: want 777, got %d", c.Index)
	}
	if len(c.Items) != 2 {
		t.Fatalf("Conflict.Items: want 2 (one per scope), got %d", len(c.Items))
	}
	scopes := map[string]int{}
	for _, it := range c.Items {
		scopes[it.Scope]++
	}
	if scopes["inventory_common"] != 1 || scopes["inventory_key"] != 1 {
		t.Errorf("cross-scope conflict must contain exactly one item per scope, got %+v", scopes)
	}
}

// TestGetSaveInventoryIntegrityReport_EmptyHandlesIgnored verifies that empty
// and invalid (sentinel) inventory rows neither inflate the conflict count
// nor leak into the affected-items listing. Only real, populated duplicates
// must reach the user.
func TestGetSaveInventoryIntegrityReport_EmptyHandlesIgnored(t *testing.T) {
	app := integrityFixture()
	handles := knownGoodsHandles(t, 2)
	slot := &app.save.Slots[0]
	slot.Inventory.CommonItems = []core.InventoryItem{
		{GaItemHandle: core.GaHandleEmpty, Quantity: 0, Index: 552},
		{GaItemHandle: handles[0], Quantity: 1, Index: 552},
		{GaItemHandle: core.GaHandleInvalid, Quantity: 0, Index: 552},
		{GaItemHandle: handles[1], Quantity: 1, Index: 552},
	}

	report, err := app.GetSaveInventoryIntegrityReport()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(report.Slots) != 1 || len(report.Slots[0].Conflicts) != 1 {
		t.Fatalf("expected 1 slot with 1 conflict group, got %+v", report)
	}
	items := report.Slots[0].Conflicts[0].Items
	if len(items) != 2 {
		t.Fatalf("Items must include only the 2 populated rows, got %d", len(items))
	}
	for _, it := range items {
		if it.Handle == core.GaHandleEmpty || it.Handle == core.GaHandleInvalid {
			t.Errorf("empty/invalid handle leaked into report: %+v", it)
		}
	}
	// The scanner emits one issue per duplicate (additional) occurrence: rows
	// 1 and 3 are duplicates of each other → 1 issue.
	if dec := report.Slots[0].DuplicateEntryCount; dec != 1 {
		t.Errorf("DuplicateEntryCount must count only real duplicates, got %d", dec)
	}
}

func TestGetSaveInventoryIntegrityReport_DuplicatePhysick(t *testing.T) {
	app := integrityFixture()
	slot := &app.save.Slots[0]
	slot.Inventory.CommonItems = []core.InventoryItem{
		{GaItemHandle: 0xB00000FA, Quantity: 1, Index: 100},
		{GaItemHandle: 0xB00000FB, Quantity: 1, Index: 101},
	}

	report, err := app.GetSaveInventoryIntegrityReport()
	if err != nil {
		t.Fatalf("GetSaveInventoryIntegrityReport: %v", err)
	}
	if report.Clean {
		t.Fatalf("duplicate Physick should mark report dirty")
	}
	if len(report.Slots) != 1 {
		t.Fatalf("affected slots = %d, want 1", len(report.Slots))
	}
	slotReport := report.Slots[0]
	if slotReport.DuplicateEntryCount != 1 {
		t.Fatalf("DuplicateEntryCount = %d, want 1", slotReport.DuplicateEntryCount)
	}
	if len(slotReport.Conflicts) != 1 {
		t.Fatalf("Conflicts = %d, want 1", len(slotReport.Conflicts))
	}
	conflict := slotReport.Conflicts[0]
	if conflict.Kind != "duplicate_physick" {
		t.Fatalf("conflict.Kind = %q, want duplicate_physick", conflict.Kind)
	}
	if len(conflict.Items) != 2 {
		t.Fatalf("conflict.Items = %d, want 2", len(conflict.Items))
	}
	for _, item := range conflict.Items {
		if item.Name != "Flask of Wondrous Physick" {
			t.Fatalf("item.Name = %q, want Flask of Wondrous Physick", item.Name)
		}
	}
}

func TestGetSaveInventoryIntegrityReport_RepairDuplicatePhysickThenClean(t *testing.T) {
	app := repairFixture([]core.InventoryItem{
		{GaItemHandle: 0xB00000FA, Quantity: 1, Index: 100},
		{GaItemHandle: 0xB00000FA, Quantity: 1, Index: 101},
	}, nil)

	pre, err := app.GetSaveInventoryIntegrityReport()
	if err != nil {
		t.Fatalf("pre scan: %v", err)
	}
	if pre.Clean {
		t.Fatalf("pre scan should be dirty")
	}
	if _, err := app.RepairDuplicateInventoryIndices(0); err != nil {
		t.Fatalf("repair: %v", err)
	}
	post, err := app.GetSaveInventoryIntegrityReport()
	if err != nil {
		t.Fatalf("post scan: %v", err)
	}
	if !post.Clean {
		t.Fatalf("post scan should be clean, got %+v", post)
	}
}

// TestGetSaveInventoryIntegrityReport_InactiveResidualSlot_IsReportedAndMarkedInactive
// confirms that a phantom slot (Version != 0 but ActiveSlots[i] == false) is
// included in the integrity report (its bytes round-trip through WriteSave)
// and that the DTO marks it Active=false so the UI can label it as a
// residual slot rather than a regular character.
func TestGetSaveInventoryIntegrityReport_InactiveResidualSlot_IsReportedAndMarkedInactive(t *testing.T) {
	app := integrityFixture()
	handles := knownGoodsHandles(t, 4)

	// Slot 0 is active in the fixture; load it with a clean inventory so it
	// does not pollute the report.
	app.save.ActiveSlots[0] = true
	slot0 := &app.save.Slots[0]
	slot0.Inventory.CommonItems = []core.InventoryItem{
		{GaItemHandle: handles[0], Quantity: 1, Index: 600},
		{GaItemHandle: handles[1], Quantity: 1, Index: 601},
	}

	// Slot 3 has residual data (Version != 0) and a duplicate, but the
	// active flag is cleared — the in-game roster would not show it.
	slot3 := &app.save.Slots[3]
	slot3.Version = 1
	slot3.Inventory.NextAcquisitionSortId = 800
	slot3.Inventory.NextEquipIndex = 800
	slot3.Inventory.CommonItems = []core.InventoryItem{
		{GaItemHandle: handles[2], Quantity: 1, Index: 700},
		{GaItemHandle: handles[3], Quantity: 1, Index: 700},
	}
	app.save.ActiveSlots[3] = false

	report, err := app.GetSaveInventoryIntegrityReport()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(report.Slots) != 1 {
		t.Fatalf("expected 1 affected slot (residual phantom), got %d (%+v)", len(report.Slots), report.Slots)
	}
	s := report.Slots[0]
	if s.SlotIndex != 3 {
		t.Errorf("SlotIndex: want 3, got %d", s.SlotIndex)
	}
	if s.Active {
		t.Errorf("Active should be false for a residual phantom slot, got true")
	}
}

// TestGetSaveInventoryIntegrityReport_ActiveFlagMirrored guards the happy
// path: an active slot must round-trip ActiveSlots[i] = true through the
// DTO so the modal can label it as a regular character.
func TestGetSaveInventoryIntegrityReport_ActiveFlagMirrored(t *testing.T) {
	app := integrityFixture()
	handles := knownGoodsHandles(t, 2)
	app.save.ActiveSlots[0] = true
	slot := &app.save.Slots[0]
	slot.Inventory.CommonItems = []core.InventoryItem{
		{GaItemHandle: handles[0], Quantity: 1, Index: 999},
		{GaItemHandle: handles[1], Quantity: 1, Index: 999},
	}

	report, err := app.GetSaveInventoryIntegrityReport()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(report.Slots) != 1 {
		t.Fatalf("expected 1 affected slot, got %d", len(report.Slots))
	}
	if !report.Slots[0].Active {
		t.Errorf("Active should be true for ActiveSlots[0] = true, got false")
	}
}

// firstUpgradeable25MeleeHandle returns a (handle, baseID, encodedItemID,
// upgradeLevel, infusionName) tuple drawn from the live DB so the next test
// can exercise the GaMap-aware enrichment path without hard-coding IDs that
// might change.
func firstUpgradeable25MeleeHandle(t *testing.T, level int, infuseOffset uint32, infuseName string) (handle, encoded, baseID uint32) {
	t.Helper()
	for _, it := range db.GetAllItems("PC") {
		if it.Category != "melee_armaments" || it.Name == "" || it.MaxUpgrade != 25 {
			continue
		}
		if it.ID&0xF0000000 != 0 {
			continue
		}
		// Weapon handle prefix: 0x80; preserve the base low 28 bits.
		handle = (it.ID & 0x0FFFFFFF) | 0x80000000
		baseID = it.ID
		encoded = baseID + infuseOffset + uint32(level)
		_ = infuseName // emitted by caller via decodeWeaponUpgradeInfusion
		return
	}
	t.Fatal("no MaxUpgrade=25 melee armament available in DB")
	return
}

// TestGetSaveInventoryIntegrityReport_WeaponUpgradeInfusionFromGaMap proves
// that a weapon record gets its upgrade level and infusion populated from
// slot.GaMap (where the encoded ItemID lives), not from the bare handle
// prefix. The fixture plants GaMap[handle] = baseID + Heavy + 5; the DTO
// must report CurrentUpgrade=5 and InfusionName="Heavy".
func TestGetSaveInventoryIntegrityReport_WeaponUpgradeInfusionFromGaMap(t *testing.T) {
	const upgradeLevel = 5
	const heavyOffset uint32 = 100
	const heavyName = "Heavy"

	app := integrityFixture()
	slot := &app.save.Slots[0]
	slot.GaMap = make(map[uint32]uint32)

	weaponHandle, encoded, _ := firstUpgradeable25MeleeHandle(t, upgradeLevel, heavyOffset, heavyName)
	slot.GaMap[weaponHandle] = encoded

	knownGoods := knownGoodsHandles(t, 1)
	slot.Inventory.CommonItems = []core.InventoryItem{
		{GaItemHandle: weaponHandle, Quantity: 1, Index: 1234},
		{GaItemHandle: knownGoods[0], Quantity: 1, Index: 1234},
	}

	report, err := app.GetSaveInventoryIntegrityReport()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(report.Slots) != 1 || len(report.Slots[0].Conflicts) != 1 {
		t.Fatalf("expected 1 slot with 1 conflict, got %+v", report)
	}
	var weaponItem *InventoryIntegrityConflictItem
	for i := range report.Slots[0].Conflicts[0].Items {
		if report.Slots[0].Conflicts[0].Items[i].Handle == weaponHandle {
			weaponItem = &report.Slots[0].Conflicts[0].Items[i]
			break
		}
	}
	if weaponItem == nil {
		t.Fatalf("weapon row missing from conflict; items=%+v", report.Slots[0].Conflicts[0].Items)
	}
	if weaponItem.Unknown {
		t.Errorf("weapon should resolve through DB once GaMap delivers the encoded ItemID, got Unknown=true %+v", *weaponItem)
	}
	if weaponItem.Category != "melee_armaments" {
		t.Errorf("Category: want melee_armaments, got %q", weaponItem.Category)
	}
	if weaponItem.CurrentUpgrade != upgradeLevel {
		t.Errorf("CurrentUpgrade: want %d, got %d", upgradeLevel, weaponItem.CurrentUpgrade)
	}
	if weaponItem.InfusionName != heavyName {
		t.Errorf("InfusionName: want %q, got %q", heavyName, weaponItem.InfusionName)
	}
}
