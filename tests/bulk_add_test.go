package tests

import (
	"fmt"
	"strings"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
	"github.com/oisis/EldenRing-SaveForge/backend/db"
)

// loadBulkTestSave loads the PC test save and returns the save file and first non-empty slot index.
func loadBulkTestSave(t *testing.T) (*core.SaveFile, int) {
	t.Helper()
	save, err := core.LoadSave("../tmp/save/ER0000.sl2")
	if err != nil {
		t.Skipf("test save not found: %v", err)
	}
	slotIdx := -1
	for i, s := range save.Slots {
		if s.Version > 0 {
			slotIdx = i
			break
		}
	}
	if slotIdx < 0 {
		t.Skip("no non-empty slots in test save")
	}
	return save, slotIdx
}

// countEmpty returns the number of empty GaItem entries in the slot.
func countEmpty(slot *core.SaveSlot) int {
	n := 0
	for _, g := range slot.GaItems {
		if g.IsEmpty() {
			n++
		}
	}
	return n
}

// armamentCapacity returns the number of non-stackable GaItem records that can
// still be allocated in the slot's armament zone. The save format places new
// weapon/armor/AoW entries at slot.NextArmamentIndex and advances it monotonically;
// empty slots BELOW NextArmamentIndex are historical "holes" left by in-game
// deletions and are NOT reusable (handle counters must keep increasing and
// stay segregated by type). Callers must size bulk inputs against this value,
// not against countEmpty(slot).
func armamentCapacity(slot *core.SaveSlot) int {
	return len(slot.GaItems) - slot.NextArmamentIndex
}

// collectIDs gathers up to `limit` item IDs from a given category.
func collectIDs(category, platform string, limit int) []uint32 {
	var ids []uint32
	for _, item := range db.GetItemsByCategory(category, platform) {
		if len(ids) >= limit {
			break
		}
		ids = append(ids, item.ID)
	}
	return ids
}

// verifyPostAdd validates slot integrity, data size, GaMap presence, and re-parse consistency.
func verifyPostAdd(t *testing.T, slot *core.SaveSlot, platform string, addedIDs []uint32, label string) {
	t.Helper()

	// 1. Slot integrity.
	if err := core.ValidateSlotIntegrity(slot); err != nil {
		t.Fatalf("[%s] ValidateSlotIntegrity failed: %v", label, err)
	}

	// 2. Data length unchanged.
	if len(slot.Data) != core.SlotSize {
		t.Fatalf("[%s] slot.Data length changed: %d (expected %d)", label, len(slot.Data), core.SlotSize)
	}

	// 3. GaMap has entries for non-stackable items.
	for _, id := range addedIDs {
		prefix := db.ItemIDToHandlePrefix(id)
		if prefix == core.ItemTypeItem || prefix == core.ItemTypeAccessory {
			continue // stackable — only in inventory, not GaItems
		}
		found := false
		for _, mapID := range slot.GaMap {
			if mapID == id {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("[%s] Item ID 0x%X not found in GaMap", label, id)
		}
	}

	// 4. Re-parse modified slot data.
	reparsed := &core.SaveSlot{}
	r := core.NewReader(slot.Data)
	if err := reparsed.Read(r, platform); err != nil {
		t.Fatalf("[%s] Re-parse failed: %v", label, err)
	}

	// 5. Re-parsed GaMap contains all non-stackable items.
	for _, id := range addedIDs {
		prefix := db.ItemIDToHandlePrefix(id)
		if prefix == core.ItemTypeItem || prefix == core.ItemTypeAccessory {
			continue
		}
		found := false
		for _, mapID := range reparsed.GaMap {
			if mapID == id {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("[%s] Item ID 0x%X not found in re-parsed GaMap", label, id)
		}
	}
}

// TestBulkAddItems verifies that adding 50 items (weapons, armors, AoW) works.
// This is the exact scenario that previously failed with:
//   "writeGaItem: no space in GaItems section (InventoryEnd=0xD5A8 + size=8 > gaLimit=0xD5A8)"
func TestBulkAddItems(t *testing.T) {
	save, slotIdx := loadBulkTestSave(t)
	slot := &save.Slots[slotIdx]
	platform := string(save.Platform)

	emptyBefore := countEmpty(slot)
	t.Logf("Slot %d: GaItems=%d, GaMap=%d, empty=%d, MagicOffset=0x%X",
		slotIdx, len(slot.GaItems), len(slot.GaMap), emptyBefore, slot.MagicOffset)

	weaponIDs := collectIDs("melee_armaments", platform, 20)
	armorIDs := collectIDs("head", platform, 20)
	aowIDs := collectIDs("ashes_of_war", platform, 10)
	allIDs := append(append(weaponIDs, armorIDs...), aowIDs...)

	t.Logf("Adding %d items: %d weapons, %d armors, %d AoW", len(allIDs), len(weaponIDs), len(armorIDs), len(aowIDs))

	magicBefore := slot.MagicOffset
	if err := core.AddItemsToSlot(slot, allIDs, 1, 0, false); err != nil {
		t.Fatalf("AddItemsToSlot failed: %v", err)
	}

	emptyAfter := countEmpty(slot)
	expectedUsed := len(weaponIDs) + len(armorIDs) + len(aowIDs)
	actualUsed := emptyBefore - emptyAfter

	t.Logf("Empty slots: %d → %d (used=%d, expected=%d)", emptyBefore, emptyAfter, actualUsed, expectedUsed)
	t.Logf("MagicOffset: 0x%X → 0x%X (delta=%d)", magicBefore, slot.MagicOffset, slot.MagicOffset-magicBefore)

	if actualUsed != expectedUsed {
		t.Errorf("Expected %d GaItem entries used, got %d", expectedUsed, actualUsed)
	}

	verifyPostAdd(t, slot, platform, allIDs, "BulkAdd50")
	t.Logf("SUCCESS: %d items added and verified", len(allIDs))
}

// TestMassiveAddAllCategories adds items from EVERY non-stackable category up
// to the armament-zone capacity (NOT the total empty-slot count) to stress-test
// GaItems re-serialization. The shared fixture `tmp/save/ER0000.sl2` ships
// with NextArmamentIndex deep in the array — capacity is `len(GaItems) -
// NextArmamentIndex`, not countEmpty(slot). See armamentCapacity() docstring.
func TestMassiveAddAllCategories(t *testing.T) {
	save, slotIdx := loadBulkTestSave(t)
	slot := &save.Slots[slotIdx]
	platform := string(save.Platform)

	emptyBefore := countEmpty(slot)
	gaMapBefore := len(slot.GaMap)
	magicBefore := slot.MagicOffset
	capacity := armamentCapacity(slot)
	t.Logf("Slot %d: GaItems=%d, GaMap=%d, empty=%d, armamentCapacity=%d, MagicOffset=0x%X",
		slotIdx, len(slot.GaItems), gaMapBefore, emptyBefore, capacity, magicBefore)

	if capacity < 30 {
		t.Skipf("test save has insufficient armament-zone capacity (free=%d)", capacity)
	}

	// Non-stackable categories: weapons (0x80 handle), armor (0x90), AoW (0xC0).
	// Stackable categories (talismans 0xA0, goods 0xB0) don't use GaItems slots.
	// Per-category limits are RAW DB caps; we trim them below to fit capacity.
	type catBatch struct {
		category string
		limit    int
	}
	nonStackable := []catBatch{
		{"melee_armaments", 200},
		{"ranged_and_catalysts", 69},
		{"shields", 165},
		{"head", 200},
		{"chest", 200},
		{"arms", 121},
		{"legs", 138},
		{"ashes_of_war", 116},
	}

	// Distribute capacity evenly across categories. Each non-stackable insert
	// (incl. AoW) consumes one armament-zone slot — AoW pushes NextArmamentIndex
	// right via `slot.NextArmamentIndex++` in allocateGaItem.
	perCat := capacity / len(nonStackable)
	if perCat < 1 {
		perCat = 1
	}

	var allIDs []uint32
	var totalWeapons, totalArmors, totalAoW int

	for _, cb := range nonStackable {
		take := cb.limit
		if take > perCat {
			take = perCat
		}
		ids := collectIDs(cb.category, platform, take)
		for _, id := range ids {
			prefix := db.ItemIDToHandlePrefix(id)
			switch prefix {
			case core.ItemTypeWeapon:
				totalWeapons++
			case core.ItemTypeArmor:
				totalArmors++
			case core.ItemTypeAow:
				totalAoW++
			}
		}
		allIDs = append(allIDs, ids...)
	}

	// Final safety trim: never exceed capacity even if rounding overshoots.
	if len(allIDs) > capacity {
		allIDs = allIDs[:capacity]
		totalWeapons, totalArmors, totalAoW = 0, 0, 0
		for _, id := range allIDs {
			switch db.ItemIDToHandlePrefix(id) {
			case core.ItemTypeWeapon:
				totalWeapons++
			case core.ItemTypeArmor:
				totalArmors++
			case core.ItemTypeAow:
				totalAoW++
			}
		}
	}

	// Also add stackable categories to verify they don't consume GaItem slots.
	stackable := []catBatch{
		{"talismans", 155},            // all available
		{"tools", 100},
		{"crafting_materials", 71},    // all available
		{"bolstering_materials", 77},  // all available
		{"sorceries", 79},             // all available
		{"incantations", 123},         // all available
	}
	var stackableIDs []uint32
	for _, cb := range stackable {
		ids := collectIDs(cb.category, platform, cb.limit)
		stackableIDs = append(stackableIDs, ids...)
	}

	t.Logf("Non-stackable: %d items (%d weapons, %d armors, %d AoW)",
		len(allIDs), totalWeapons, totalArmors, totalAoW)
	t.Logf("Stackable: %d items (talismans, tools, crafting, spells)", len(stackableIDs))

	if len(allIDs) == 0 {
		t.Skip("no items found in DB")
	}

	nonStackableCount := totalWeapons + totalArmors + totalAoW
	if nonStackableCount > capacity {
		t.Fatalf("internal: nonStackableCount %d exceeds capacity %d after trim", nonStackableCount, capacity)
	}

	// Add non-stackable items first.
	if err := core.AddItemsToSlot(slot, allIDs, 1, 0, false); err != nil {
		t.Fatalf("AddItemsToSlot (non-stackable) failed: %v", err)
	}

	emptyAfterNonStackable := countEmpty(slot)
	usedSlots := emptyBefore - emptyAfterNonStackable
	t.Logf("After non-stackable: empty=%d (used=%d), MagicOffset=0x%X (delta=%d)",
		emptyAfterNonStackable, usedSlots, slot.MagicOffset, slot.MagicOffset-magicBefore)

	if usedSlots != nonStackableCount {
		t.Errorf("Expected %d GaItem slots used, got %d", nonStackableCount, usedSlots)
	}

	// Add stackable items — should NOT consume any GaItem slots.
	emptyBeforeStackable := countEmpty(slot)
	if err := core.AddItemsToSlot(slot, stackableIDs, 1, 0, false); err != nil {
		t.Fatalf("AddItemsToSlot (stackable) failed: %v", err)
	}

	emptyAfterStackable := countEmpty(slot)
	if emptyBeforeStackable != emptyAfterStackable {
		t.Errorf("Stackable items consumed GaItem slots: %d → %d (should be unchanged)",
			emptyBeforeStackable, emptyAfterStackable)
	}

	// Compute expected delta for non-stackable items.
	expectedDelta := totalWeapons*(core.GaRecordWeapon-core.GaRecordItem) +
		totalArmors*(core.GaRecordArmor-core.GaRecordItem)
	actualDelta := slot.MagicOffset - magicBefore
	t.Logf("MagicOffset delta: expected=%d, actual=%d", expectedDelta, actualDelta)
	if actualDelta != expectedDelta {
		t.Errorf("MagicOffset delta mismatch: expected %d, got %d", expectedDelta, actualDelta)
	}

	// Full verification.
	allAddedIDs := append(allIDs, stackableIDs...)
	verifyPostAdd(t, slot, platform, allAddedIDs, "MassiveAdd")

	totalAdded := len(allIDs) + len(stackableIDs)
	t.Logf("SUCCESS: %d total items added (%d non-stackable + %d stackable), GaMap=%d",
		totalAdded, len(allIDs), len(stackableIDs), len(slot.GaMap))
}

// TestMaxCapacityFill fills the armament-zone (slots at and above
// NextArmamentIndex) to verify allocator behavior near the capacity limit.
// Pre-NextArmamentIndex empty slots are intentionally NOT touched — the save
// format expects them to remain unused (see armamentCapacity() docstring).
func TestMaxCapacityFill(t *testing.T) {
	save, slotIdx := loadBulkTestSave(t)
	slot := &save.Slots[slotIdx]
	platform := string(save.Platform)

	emptyBefore := countEmpty(slot)
	capacity := armamentCapacity(slot)
	t.Logf("Slot %d: empty GaItem slots = %d / %d, armamentCapacity = %d",
		slotIdx, emptyBefore, len(slot.GaItems), capacity)

	if capacity < 20 {
		t.Skipf("test save has insufficient armament-zone capacity (free=%d)", capacity)
	}

	// Generate unique weapon IDs to fill as many capacity slots as we can.
	weaponBases := collectIDs("melee_armaments", platform, 200)
	if len(weaponBases) == 0 {
		t.Skip("no weapon IDs in DB")
	}

	// We'll add AoW items (8 bytes each — no shift needed) to fill a portion
	// of the armament zone, then weapons to fill the rest. Both consume from
	// the SAME capacity pool because allocateGaItem advances NextArmamentIndex
	// on every insert (AoW included).
	aowBases := collectIDs("ashes_of_war", platform, 116)

	// Reserve ~half capacity for weapons below to exercise the 21B shift path.
	reserveForWeapons := capacity / 2
	maxAoW := capacity - reserveForWeapons
	if maxAoW > len(aowBases) {
		maxAoW = len(aowBases)
	}
	if maxAoW < 0 {
		maxAoW = 0
	}
	aowIDs := append([]uint32(nil), aowBases[:maxAoW]...)

	t.Logf("Adding %d AoW items (8B each, no shift expected)", len(aowIDs))

	magicBefore := slot.MagicOffset
	if err := core.AddItemsToSlot(slot, aowIDs, 1, 0, false); err != nil {
		t.Fatalf("AddItemsToSlot (AoW fill) failed: %v", err)
	}

	emptyAfterAoW := countEmpty(slot)
	t.Logf("After AoW: empty=%d (used=%d), MagicOffset delta=%d (expected 0 for 8B records)",
		emptyAfterAoW, emptyBefore-emptyAfterAoW, slot.MagicOffset-magicBefore)

	// AoW is 8 bytes — same size as empty entries, no shift expected.
	if slot.MagicOffset != magicBefore {
		t.Errorf("MagicOffset should not change for 8B records: was 0x%X, now 0x%X",
			magicBefore, slot.MagicOffset)
	}

	// Now add weapons (21B each — max shift stress). Remaining capacity must
	// be computed from armamentCapacity post-AoW, not countEmpty (the pre-Next
	// holes remain unused on purpose).
	capacityAfterAoW := armamentCapacity(slot)
	maxWeapons := capacityAfterAoW
	if maxWeapons > len(weaponBases) {
		maxWeapons = len(weaponBases)
	}
	weaponIDs := weaponBases[:maxWeapons]

	t.Logf("Adding %d weapons (21B each, shift expected)", len(weaponIDs))

	magicBeforeWeapons := slot.MagicOffset
	if err := core.AddItemsToSlot(slot, weaponIDs, 1, 0, false); err != nil {
		t.Fatalf("AddItemsToSlot (weapon fill) failed: %v", err)
	}

	emptyAfterAll := countEmpty(slot)
	weaponDelta := slot.MagicOffset - magicBeforeWeapons
	expectedWeaponDelta := len(weaponIDs) * (core.GaRecordWeapon - core.GaRecordItem)

	t.Logf("After weapons: empty=%d, MagicOffset delta=%d (expected=%d)",
		emptyAfterAll, weaponDelta, expectedWeaponDelta)

	if weaponDelta != expectedWeaponDelta {
		t.Errorf("Weapon shift delta mismatch: expected %d, got %d", expectedWeaponDelta, weaponDelta)
	}

	// Full verification.
	allIDs := append(aowIDs, weaponIDs...)
	verifyPostAdd(t, slot, platform, allIDs, "MaxCapacity")

	t.Logf("SUCCESS: filled %d/%d GaItem slots (%d AoW + %d weapons), empty remaining=%d",
		len(allIDs), len(slot.GaItems), len(aowIDs), len(weaponIDs), emptyAfterAll)
}

// TestAddThenOverflow verifies graceful error when GaItems array is completely full.
func TestAddThenOverflow(t *testing.T) {
	save, slotIdx := loadBulkTestSave(t)
	slot := &save.Slots[slotIdx]
	platform := string(save.Platform)

	// Count how many empty slots are available.
	emptyCount := countEmpty(slot)
	t.Logf("Slot %d: %d empty GaItem slots", slotIdx, emptyCount)

	// Fill ALL empty slots with AoW (8B, no shift — fastest way to fill).
	aowBases := collectIDs("ashes_of_war", platform, 116)
	if len(aowBases) == 0 {
		t.Skip("no AoW IDs in DB")
	}

	// Fill in batches using all available AoW IDs, reusing with upgrade offsets.
	filled := 0
	batch := 0
	for filled < emptyCount {
		remaining := emptyCount - filled
		batchSize := remaining
		if batchSize > len(aowBases) {
			batchSize = len(aowBases)
		}
		ids := aowBases[:batchSize]
		if err := core.AddItemsToSlot(slot, ids, 1, 0, false); err != nil {
			t.Logf("Fill batch %d failed at item %d/%d: %v", batch, filled, emptyCount, err)
			break
		}
		filled += batchSize
		batch++

		// After first batch, the same IDs would get "already exists" handles.
		// Break to avoid infinite loop — one batch of unique IDs is enough.
		break
	}

	emptyAfter := countEmpty(slot)
	t.Logf("After fill: empty=%d (filled %d)", emptyAfter, filled)

	// Verify integrity is still valid even after heavy fill.
	if err := core.ValidateSlotIntegrity(slot); err != nil {
		t.Fatalf("ValidateSlotIntegrity failed after fill: %v", err)
	}

	// If there are no empty slots left, verify that adding one more item returns error.
	if emptyAfter == 0 {
		extraID := aowBases[0] + 999999 // non-existing variant to force new handle
		err := core.AddItemsToSlot(slot, []uint32{extraID}, 1, 0, false)
		if err == nil {
			t.Error("Expected error when adding to full GaItems array, got nil")
		} else {
			t.Logf("Correctly got error on overflow: %v", err)
		}
	} else {
		t.Logf("Still %d empty slots — overflow test skipped (would need more unique IDs)", emptyAfter)
	}
}

// TestAddWithInventoryAndStorage adds items to both inventory and storage simultaneously.
//
// Non-stackable dual-destination semantics: each (item_id, destination) pair
// receives its OWN GaItem record with a distinct handle. Sharing a handle
// between inventory and storage corrupts the save (game treats both list
// entries as the same physical item; observed: AoW applied to one weapon
// propagates to the duplicate inventory entry).
func TestAddWithInventoryAndStorage(t *testing.T) {
	save, slotIdx := loadBulkTestSave(t)
	slot := &save.Slots[slotIdx]
	platform := string(save.Platform)

	// Reserve capacity for dual-destination non-stackable adds: each item
	// consumes 2 armament-zone GaItem slots (one for inv handle, one for
	// storage). The shared test save (`tmp/save/ER0000.sl2`) carries a
	// near-full armament zone; cap items so we don't exhaust the array.
	freeArmament := len(slot.GaItems) - slot.NextArmamentIndex
	perCategoryLimit := freeArmament / 4 // 2 categories × 2 destinations
	if perCategoryLimit > 30 {
		perCategoryLimit = 30
	}
	if perCategoryLimit < 5 {
		t.Skipf("test save has insufficient armament-zone capacity (free=%d)", freeArmament)
	}

	weaponIDs := collectIDs("melee_armaments", platform, perCategoryLimit)
	armorIDs := collectIDs("chest", platform, perCategoryLimit)
	talismanIDs := collectIDs("talismans", platform, 50)
	toolIDs := collectIDs("tools", platform, 50)

	t.Logf("Adding to inv+storage: %d weapons, %d armors, %d talismans, %d tools (free armament=%d)",
		len(weaponIDs), len(armorIDs), len(talismanIDs), len(toolIDs), freeArmament)

	// Add weapons and armors with invQty=1, storageQty=1.
	nonStackable := append(weaponIDs, armorIDs...)
	if err := core.AddItemsToSlot(slot, nonStackable, 1, 1, false); err != nil {
		t.Fatalf("AddItemsToSlot (non-stackable inv+storage) failed: %v", err)
	}

	// Add stackable items with invQty=99, storageQty=99.
	stackable := append(talismanIDs, toolIDs...)
	if err := core.AddItemsToSlot(slot, stackable, 99, 99, false); err != nil {
		t.Fatalf("AddItemsToSlot (stackable inv+storage) failed: %v", err)
	}

	allIDs := append(nonStackable, stackable...)
	verifyPostAdd(t, slot, platform, allIDs, "InvAndStorage")

	invHandles := make(map[uint32]bool)
	for _, item := range slot.Inventory.CommonItems {
		if item.GaItemHandle != 0 && item.GaItemHandle != 0xFFFFFFFF {
			invHandles[item.GaItemHandle] = true
		}
	}

	storageHandles := make(map[uint32]bool)
	for _, item := range slot.Storage.CommonItems {
		if item.GaItemHandle != 0 && item.GaItemHandle != 0xFFFFFFFF {
			storageHandles[item.GaItemHandle] = true
		}
	}

	t.Logf("Inventory distinct handles: %d, Storage distinct handles: %d",
		len(invHandles), len(storageHandles))

	// Per non-stackable id, collect every handle that maps to it. Each id
	// must have ≥1 handle in inventory and ≥1 handle in storage, and the
	// inv-side and storage-side handles must be DISJOINT (the regression).
	invMissing, storageMissing, sharedHandle := 0, 0, 0
	for _, id := range nonStackable {
		var handlesForID []uint32
		for h, mapID := range slot.GaMap {
			if mapID == id {
				handlesForID = append(handlesForID, h)
			}
		}
		if len(handlesForID) == 0 {
			continue
		}
		invHas, stoHas := false, false
		for _, h := range handlesForID {
			inInv := invHandles[h]
			inSto := storageHandles[h]
			if inInv && inSto {
				sharedHandle++
				t.Errorf("non-stackable id 0x%08X: handle 0x%08X shared between inventory and storage (regression)", id, h)
			}
			if inInv {
				invHas = true
			}
			if inSto {
				stoHas = true
			}
		}
		if !invHas {
			invMissing++
		}
		if !stoHas {
			storageMissing++
		}
	}

	t.Logf("Inventory: %d/%d items present (missing=%d)", len(nonStackable)-invMissing, len(nonStackable), invMissing)
	t.Logf("Storage: %d/%d items present (missing=%d)", len(nonStackable)-storageMissing, len(nonStackable), storageMissing)
	t.Logf("Shared handles between inv and storage (must be 0 for non-stackable): %d", sharedHandle)

	minPresent := len(nonStackable) * 80 / 100
	if len(nonStackable)-invMissing < minPresent {
		t.Errorf("Too many items missing from inventory: %d/%d", invMissing, len(nonStackable))
	}
	if len(nonStackable)-storageMissing < minPresent {
		t.Errorf("Too many items missing from storage: %d/%d", storageMissing, len(nonStackable))
	}

	t.Logf("SUCCESS: %d items added to both inventory and storage", len(allIDs))
}

// TestBulkAddPerCategory adds items per non-stackable category in turn, sized
// to the armament-zone capacity remaining after each batch. The cap comes from
// armamentCapacity(slot), NOT from countEmpty — see armamentCapacity() docstring
// for why pre-NextArmamentIndex holes are not reusable.
func TestBulkAddPerCategory(t *testing.T) {
	save, slotIdx := loadBulkTestSave(t)
	slot := &save.Slots[slotIdx]
	platform := string(save.Platform)

	categories := []struct {
		name  string
		limit int
	}{
		{"melee_armaments", 200},
		{"ranged_and_catalysts", 69},
		{"shields", 165},
		{"head", 200},
		{"chest", 200},
		{"arms", 121},
		{"legs", 138},
		{"ashes_of_war", 116},
	}

	emptyBefore := countEmpty(slot)
	capacityBefore := armamentCapacity(slot)
	totalAdded := 0
	var allIDs []uint32

	// Allocate at most ~per-category share of remaining capacity so every
	// category gets to add at least a handful of items even on a near-full save.
	perCategoryShare := capacityBefore / len(categories)
	if perCategoryShare < 1 {
		t.Skipf("armament-zone capacity %d too small to split across %d categories", capacityBefore, len(categories))
	}

	for _, cat := range categories {
		ids := collectIDs(cat.name, platform, cat.limit)
		if len(ids) == 0 {
			t.Logf("  [%s] no items found, skipping", cat.name)
			continue
		}

		// Cap to both the per-category share and the LIVE remaining capacity.
		take := perCategoryShare
		if remaining := armamentCapacity(slot); take > remaining {
			take = remaining
		}
		if take > len(ids) {
			take = len(ids)
		}
		if take <= 0 {
			t.Logf("  [%s] armament zone full, stopping at %d total", cat.name, totalAdded)
			break
		}
		ids = ids[:take]

		err := core.AddItemsToSlot(slot, ids, 1, 0, false)
		if err != nil {
			t.Fatalf("[%s] AddItemsToSlot failed after %d total items: %v", cat.name, totalAdded, err)
		}

		totalAdded += len(ids)
		allIDs = append(allIDs, ids...)
		t.Logf("  [%s] added %d items (total=%d, empty=%d, remainingCapacity=%d)",
			cat.name, len(ids), totalAdded, countEmpty(slot), armamentCapacity(slot))
	}

	emptyAfter := countEmpty(slot)
	t.Logf("Total: added %d items, empty slots %d → %d, capacity %d → %d",
		totalAdded, emptyBefore, emptyAfter, capacityBefore, armamentCapacity(slot))

	// Full verification.
	verifyPostAdd(t, slot, platform, allIDs, fmt.Sprintf("PerCategory_%d", totalAdded))

	t.Logf("SUCCESS: %d items across %d categories added and verified", totalAdded, len(categories))
}

// TestAddItems_RespectsArmamentCapacity locks the allocator's over-capacity
// contract: requesting more non-stackable items than `armamentCapacity(slot)`
// must surface the "armament/armor array full" error from allocateGaItem.
// This guards against a future change that silently swallows over-capacity
// adds (e.g. by reusing pre-NextArmamentIndex holes — which would corrupt the
// save's monotonic counter ordering).
func TestAddItems_RespectsArmamentCapacity(t *testing.T) {
	save, slotIdx := loadBulkTestSave(t)
	slot := &save.Slots[slotIdx]
	platform := string(save.Platform)

	capacity := armamentCapacity(slot)
	weaponIDs := collectIDs("melee_armaments", platform, capacity+50)
	if len(weaponIDs) < capacity+1 {
		t.Skipf("not enough weapons in DB to overflow capacity (have %d, need %d)", len(weaponIDs), capacity+1)
	}

	// Take capacity+5 items so allocateGaItem must error after exactly
	// `capacity` placements.
	overflow := capacity + 5
	if overflow > len(weaponIDs) {
		overflow = len(weaponIDs)
	}
	ids := weaponIDs[:overflow]

	err := core.AddItemsToSlot(slot, ids, 1, 0, false)
	if err == nil {
		t.Fatalf("AddItemsToSlot accepted %d items with capacity=%d — allocator must error",
			overflow, capacity)
	}
	if !strings.Contains(err.Error(), "armament/armor array full") {
		t.Errorf("expected 'armament/armor array full' error, got: %v", err)
	}
	t.Logf("Correctly rejected over-capacity add (%d items > capacity %d): %v",
		overflow, capacity, err)
}
