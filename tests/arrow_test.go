package tests

import (
	"testing"

	"github.com/oisis/EldenRing-SaveEditor/backend/core"
	"github.com/oisis/EldenRing-SaveEditor/backend/db"
	"github.com/oisis/EldenRing-SaveEditor/backend/vm"
)

// TestArrowsAddToInventoryOnly reproduces the user's issue:
// Adding arrows to inventory only → arrows should appear in VM inventory, not storage.
func TestArrowsAddToInventoryOnly(t *testing.T) {
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
		t.Skip("no non-empty slots")
	}

	slot := &save.Slots[slotIdx]
	platform := string(save.Platform)

	// Collect all arrow IDs.
	arrowIDs := collectIDs("arrows_and_bolts", platform, 100)
	if len(arrowIDs) == 0 {
		t.Skip("no arrow IDs in DB")
	}
	t.Logf("Testing with %d arrow IDs", len(arrowIDs))

	// Snapshot inventory before.
	invCountBefore := 0
	for _, item := range slot.Inventory.CommonItems {
		if item.GaItemHandle != 0 && item.GaItemHandle != 0xFFFFFFFF {
			invCountBefore++
		}
	}
	storageCountBefore := len(slot.Storage.CommonItems)

	// Add arrows: invQty=1, storageQty=0, forceStackable=true (as app.go does).
	for _, id := range arrowIDs {
		forceStackable := db.IsArrowID(id)
		if !forceStackable {
			t.Errorf("Arrow ID 0x%X not detected by IsArrowID", id)
			continue
		}
		if err := core.AddItemsToSlot(slot, []uint32{id}, 1, 0, forceStackable); err != nil {
			t.Fatalf("AddItemsToSlot failed for arrow 0x%X: %v", id, err)
		}
	}

	// Count inventory items after.
	invCountAfter := 0
	arrowsInInv := 0
	for _, item := range slot.Inventory.CommonItems {
		if item.GaItemHandle == 0 || item.GaItemHandle == 0xFFFFFFFF {
			continue
		}
		invCountAfter++
		// Check if this handle maps to an arrow.
		if itemID, ok := slot.GaMap[item.GaItemHandle]; ok {
			if db.IsArrowID(itemID) {
				arrowsInInv++
			}
		}
	}

	storageCountAfter := len(slot.Storage.CommonItems)

	t.Logf("Inventory items: %d → %d (arrows found: %d)", invCountBefore, invCountAfter, arrowsInInv)
	t.Logf("Storage items: %d → %d (should be unchanged)", storageCountBefore, storageCountAfter)

	if arrowsInInv == 0 {
		t.Error("FAIL: No arrows found in inventory after adding with invQty=1")
	}
	if arrowsInInv < len(arrowIDs) {
		t.Errorf("Only %d/%d arrows found in inventory", arrowsInInv, len(arrowIDs))
	}
	if storageCountAfter != storageCountBefore {
		t.Errorf("Storage changed unexpectedly: %d → %d", storageCountBefore, storageCountAfter)
	}

	// Now check via VM — this is what the UI sees.
	vmData, err := vm.MapParsedSlotToVM(slot)
	if err != nil {
		t.Fatalf("MapParsedSlotToVM failed: %v", err)
	}

	vmArrowsInInv := 0
	vmArrowsInStorage := 0
	for _, item := range vmData.Inventory {
		if item.SubCategory == "arrows_and_bolts" || item.Category == "Weapon" {
			if db.IsArrowID(item.ID) {
				vmArrowsInInv++
			}
		}
	}
	for _, item := range vmData.Storage {
		if item.SubCategory == "arrows_and_bolts" || item.Category == "Weapon" {
			if db.IsArrowID(item.ID) {
				vmArrowsInStorage++
			}
		}
	}

	t.Logf("VM Inventory arrows: %d", vmArrowsInInv)
	t.Logf("VM Storage arrows: %d", vmArrowsInStorage)

	if vmArrowsInInv == 0 {
		t.Error("FAIL: No arrows visible in VM inventory — this is the user's reported bug")
	}
	if vmArrowsInInv < len(arrowIDs) {
		t.Errorf("Only %d/%d arrows visible in VM inventory", vmArrowsInInv, len(arrowIDs))
	}

	// Verify GaMap has all arrow handles.
	for _, id := range arrowIDs {
		found := false
		for _, mapID := range slot.GaMap {
			if mapID == id {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Arrow ID 0x%X not in GaMap", id)
		}
	}

	// Verify re-parse consistency.
	reparsed := &core.SaveSlot{}
	r := core.NewReader(slot.Data)
	if err := reparsed.Read(r, platform); err != nil {
		t.Fatalf("Re-parse failed: %v", err)
	}

	reParsedArrowsInInv := 0
	for _, item := range reparsed.Inventory.CommonItems {
		if item.GaItemHandle == 0 || item.GaItemHandle == 0xFFFFFFFF {
			continue
		}
		if itemID, ok := reparsed.GaMap[item.GaItemHandle]; ok {
			if db.IsArrowID(itemID) {
				reParsedArrowsInInv++
			}
		}
	}
	t.Logf("Re-parsed inventory arrows: %d", reParsedArrowsInInv)

	if reParsedArrowsInInv != arrowsInInv {
		t.Errorf("Re-parse mismatch: in-memory=%d, re-parsed=%d", arrowsInInv, reParsedArrowsInInv)
	}
}

// TestArrowsRustCompatibility verifies arrows are handled like the Rust ER-Save-Editor:
// 1. Arrows get a GaItem entry (weapon type, 21 bytes)
// 2. Arrows are NOT added to GaItemData (they use projectile list)
// 3. Arrow handles are 0x80xxxxxx (weapon prefix)
// 4. Arrows are stackable in inventory (qty > 1)
func TestArrowsRustCompatibility(t *testing.T) {
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
		t.Skip("no non-empty slots")
	}

	slot := &save.Slots[slotIdx]

	// Pick a known arrow ID.
	arrowID := uint32(0x02FAF080) // Arrow (basic)
	if !db.IsArrowID(arrowID) {
		t.Fatal("IsArrowID returned false for known arrow 0x02FAF080")
	}

	// Snapshot GaItemData count before.
	gaItemDataCountBefore := uint32(0)
	if slot.GaItemDataOffset > 0 && slot.GaItemDataOffset+4 <= len(slot.Data) {
		sa := core.NewSlotAccessor(slot.Data)
		gaItemDataCountBefore, _ = sa.ReadU32(slot.GaItemDataOffset)
	}

	// Check if arrow already exists in GaMap (forceStackable reuses existing handle).
	preExisting := false
	var arrowHandle uint32
	for h, id := range slot.GaMap {
		if id == arrowID {
			preExisting = true
			arrowHandle = h
			break
		}
	}
	t.Logf("Arrow 0x%X pre-existing in GaMap: %v (handle=0x%X)", arrowID, preExisting, arrowHandle)

	// Add arrow with qty=99 to inventory.
	err = core.AddItemsToSlot(slot, []uint32{arrowID}, 99, 0, true)
	if err != nil {
		t.Fatalf("AddItemsToSlot failed: %v", err)
	}

	// 1. Verify GaItem exists for the arrow (either pre-existing or newly created).
	arrowHandle = 0
	for h, id := range slot.GaMap {
		if id == arrowID {
			arrowHandle = h
			break
		}
	}
	if arrowHandle == 0 {
		t.Fatal("Arrow not found in GaMap after addition")
	}
	t.Logf("Arrow GaItem handle: 0x%X", arrowHandle)

	// 2. Verify handle prefix is 0x80 (weapon).
	if arrowHandle&0xF0000000 != 0x80000000 {
		t.Errorf("Arrow handle prefix should be 0x80, got 0x%X", arrowHandle&0xF0000000)
	}

	// 3. Verify GaItem is 21 bytes (weapon record).
	arrowRecordSize := core.GaItemRecordSize(arrowID)
	if arrowRecordSize != core.GaRecordWeapon {
		t.Errorf("Arrow record size should be %d (weapon), got %d", core.GaRecordWeapon, arrowRecordSize)
	}

	// 4. Verify GaItemData was NOT updated (arrows go to projectile list, not GaItemData).
	gaItemDataCountAfter := uint32(0)
	if slot.GaItemDataOffset > 0 && slot.GaItemDataOffset+4 <= len(slot.Data) {
		sa := core.NewSlotAccessor(slot.Data)
		gaItemDataCountAfter, _ = sa.ReadU32(slot.GaItemDataOffset)
	}
	if gaItemDataCountAfter != gaItemDataCountBefore {
		t.Errorf("GaItemData count changed from %d to %d — arrows should NOT be in GaItemData",
			gaItemDataCountBefore, gaItemDataCountAfter)
	}

	// 5. Verify arrow (any handle for arrowID) is in inventory with qty=99.
	// GaMap may have multiple handles for the same item ID (pre-existing duplicates
	// from the test save), so we search inventory for any handle that maps to arrowID
	// rather than relying on a single randomly-selected handle from map iteration.
	foundInInv := false
	for _, item := range slot.Inventory.CommonItems {
		if mapID, ok := slot.GaMap[item.GaItemHandle]; ok && mapID == arrowID {
			foundInInv = true
			if item.Quantity != 99 {
				t.Errorf("Arrow quantity in inventory: expected 99, got %d", item.Quantity)
			}
			break
		}
	}
	if !foundInInv {
		t.Error("No arrow handle for arrowID found in inventory CommonItems")
	}

	// 6. Verify arrowHandle is in GaMap (may be one of several handles for arrowID).
	if mapID, ok := slot.GaMap[arrowHandle]; !ok || mapID != arrowID {
		t.Errorf("Arrow handle 0x%X not correctly in GaMap (id=0x%X, ok=%v)", arrowHandle, mapID, ok)
	}

	t.Logf("Arrow Rust compatibility: GaItem ✓, Handle 0x80 ✓, 21B record ✓, no GaItemData ✓, inventory ✓")
}
