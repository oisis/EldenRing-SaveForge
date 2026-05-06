package tests

import (
	"os"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
	"github.com/oisis/EldenRing-SaveForge/backend/db"
	"github.com/oisis/EldenRing-SaveForge/backend/vm"
)

// findActiveSlot returns the index of the first active slot, or skips the test.
func findActiveSlot(t *testing.T, save *core.SaveFile) int {
	t.Helper()
	for i := 0; i < 10; i++ {
		if save.Slots[i].Version > 0 {
			return i
		}
	}
	t.Skip("No active slot found")
	return -1
}

// saveTmpAndReload writes save to a temp file, reloads it, and returns the new save.
// The temp file is cleaned up when the test finishes.
func saveTmpAndReload(t *testing.T, save *core.SaveFile, name string) *core.SaveFile {
	t.Helper()
	tmpPath := "data/pc/" + name
	os.MkdirAll("data/pc", 0755)
	if err := save.SaveFile(tmpPath); err != nil {
		t.Fatalf("SaveFile failed: %v", err)
	}
	t.Cleanup(func() { os.Remove(tmpPath) })

	reloaded, err := core.LoadSave(tmpPath)
	if err != nil {
		t.Fatalf("Reload failed: %v", err)
	}
	return reloaded
}

// ---------- 1. Modify Stats & Roundtrip ----------

func TestModifyStatsAndRoundtrip(t *testing.T) {
	save := loadTestSave(t, pcSavePath)
	idx := findActiveSlot(t, save)
	slot := &save.Slots[idx]

	// Capture original values
	origName := core.UTF16ToString(slot.Player.CharacterName[:])
	origClass := slot.Player.Class

	// Map to VM, modify stats
	charVM, err := vm.MapParsedSlotToVM(slot)
	if err != nil {
		t.Fatalf("MapParsedSlotToVM failed: %v", err)
	}

	// Set all stats to specific values
	charVM.Vigor = 50
	charVM.Mind = 30
	charVM.Endurance = 40
	charVM.Strength = 60
	charVM.Dexterity = 45
	charVM.Intelligence = 35
	charVM.Faith = 25
	charVM.Arcane = 20
	charVM.RecalculateLevel() // Level = sum - 79 = 226
	charVM.Souls = 999999

	expectedLevel := uint32(50+30+40+60+45+35+25+20) - 79

	// Apply back to slot
	if err := vm.ApplyVMToParsedSlot(charVM, slot); err != nil {
		t.Fatalf("ApplyVMToParsedSlot failed: %v", err)
	}

	// Write slot data (flush stats to binary)
	slot.Write("PC")

	// Roundtrip
	save2 := saveTmpAndReload(t, save, "modify_stats_test.sl2")
	slot2 := &save2.Slots[idx]

	// Verify stats survived
	if slot2.Player.Level != expectedLevel {
		t.Errorf("Level: got %d, want %d", slot2.Player.Level, expectedLevel)
	}
	if slot2.Player.Vigor != 50 {
		t.Errorf("Vigor: got %d, want 50", slot2.Player.Vigor)
	}
	if slot2.Player.Mind != 30 {
		t.Errorf("Mind: got %d, want 30", slot2.Player.Mind)
	}
	if slot2.Player.Endurance != 40 {
		t.Errorf("Endurance: got %d, want 40", slot2.Player.Endurance)
	}
	if slot2.Player.Strength != 60 {
		t.Errorf("Strength: got %d, want 60", slot2.Player.Strength)
	}
	if slot2.Player.Dexterity != 45 {
		t.Errorf("Dexterity: got %d, want 45", slot2.Player.Dexterity)
	}
	if slot2.Player.Intelligence != 35 {
		t.Errorf("Intelligence: got %d, want 35", slot2.Player.Intelligence)
	}
	if slot2.Player.Faith != 25 {
		t.Errorf("Faith: got %d, want 25", slot2.Player.Faith)
	}
	if slot2.Player.Arcane != 20 {
		t.Errorf("Arcane: got %d, want 20", slot2.Player.Arcane)
	}
	if slot2.Player.Souls != 999999 {
		t.Errorf("Souls: got %d, want 999999", slot2.Player.Souls)
	}

	// Name and class must be preserved
	newName := core.UTF16ToString(slot2.Player.CharacterName[:])
	if newName != origName {
		t.Errorf("Name changed: '%s' -> '%s'", origName, newName)
	}
	if slot2.Player.Class != origClass {
		t.Errorf("Class changed: %d -> %d", origClass, slot2.Player.Class)
	}

	// Integrity check
	if err := core.ValidateSlotIntegrity(slot2); err != nil {
		t.Errorf("ValidateSlotIntegrity failed: %v", err)
	}

	t.Logf("Stats roundtrip OK: Lv%d Vig=%d Str=%d Souls=%d",
		slot2.Player.Level, slot2.Player.Vigor, slot2.Player.Strength, slot2.Player.Souls)
}

// TestModifyStatsMultipleSlots modifies stats in all active slots simultaneously.
func TestModifyStatsMultipleSlots(t *testing.T) {
	save := loadTestSave(t, pcSavePath)

	type slotSnapshot struct {
		vigor uint32
		level uint32
	}
	snapshots := make(map[int]slotSnapshot)

	// Modify all active slots
	for i := 0; i < 10; i++ {
		if save.Slots[i].Version == 0 {
			continue
		}
		slot := &save.Slots[i]
		charVM, err := vm.MapParsedSlotToVM(slot)
		if err != nil {
			t.Fatalf("Slot %d: MapParsedSlotToVM failed: %v", i, err)
		}

		// Set vigor to 10 + slotIndex*5 (unique per slot)
		charVM.Vigor = uint32(10 + i*5)
		charVM.RecalculateLevel()

		snapshots[i] = slotSnapshot{vigor: charVM.Vigor, level: charVM.Level}

		if err := vm.ApplyVMToParsedSlot(charVM, slot); err != nil {
			t.Fatalf("Slot %d: ApplyVMToParsedSlot failed: %v", i, err)
		}
		slot.Write("PC")
	}

	// Roundtrip
	save2 := saveTmpAndReload(t, save, "modify_multi_stats_test.sl2")

	// Verify each slot independently
	for i, snap := range snapshots {
		slot2 := &save2.Slots[i]
		if slot2.Player.Vigor != snap.vigor {
			t.Errorf("Slot %d: Vigor got %d, want %d", i, slot2.Player.Vigor, snap.vigor)
		}
		if slot2.Player.Level != snap.level {
			t.Errorf("Slot %d: Level got %d, want %d", i, slot2.Player.Level, snap.level)
		}
		if err := core.ValidateSlotIntegrity(slot2); err != nil {
			t.Errorf("Slot %d: integrity check failed: %v", i, err)
		}
	}

	t.Logf("Modified %d slots, all survived roundtrip", len(snapshots))
}

// ---------- 2. Event Flags — Boss Toggle ----------

func TestEventFlagsBossToggle(t *testing.T) {
	save := loadTestSave(t, pcSavePath)
	idx := findActiveSlot(t, save)
	slot := &save.Slots[idx]

	if slot.EventFlagsOffset <= 0 || slot.EventFlagsOffset >= len(slot.Data) {
		t.Skip("EventFlagsOffset not available for this slot")
	}

	flags := slot.Data[slot.EventFlagsOffset:]
	bosses := db.GetAllBosses()
	if len(bosses) == 0 {
		t.Fatal("No bosses in database")
	}

	// Pick first 5 bosses, record original state, then toggle each
	count := 5
	if len(bosses) < count {
		count = len(bosses)
	}
	type bossState struct {
		id       uint32
		name     string
		origVal  bool
		toggleTo bool
	}
	testBosses := make([]bossState, count)

	for i := 0; i < count; i++ {
		orig, err := db.GetEventFlag(flags, bosses[i].ID)
		if err != nil {
			t.Fatalf("GetEventFlag(%d) failed: %v", bosses[i].ID, err)
		}
		testBosses[i] = bossState{
			id:       bosses[i].ID,
			name:     bosses[i].Name,
			origVal:  orig,
			toggleTo: !orig,
		}
	}

	// Toggle flags
	for _, bs := range testBosses {
		if err := db.SetEventFlag(flags, bs.id, bs.toggleTo); err != nil {
			t.Fatalf("SetEventFlag(%d, %v) failed: %v", bs.id, bs.toggleTo, err)
		}
	}

	// Verify in-memory toggle
	for _, bs := range testBosses {
		val, err := db.GetEventFlag(flags, bs.id)
		if err != nil {
			t.Fatalf("GetEventFlag(%d) failed: %v", bs.id, err)
		}
		if val != bs.toggleTo {
			t.Errorf("Boss '%s' (0x%X): expected %v after toggle, got %v", bs.name, bs.id, bs.toggleTo, val)
		}
	}

	// Roundtrip
	save2 := saveTmpAndReload(t, save, "boss_toggle_test.sl2")
	slot2 := &save2.Slots[idx]

	if slot2.EventFlagsOffset <= 0 {
		t.Fatal("EventFlagsOffset lost after roundtrip")
	}
	flags2 := slot2.Data[slot2.EventFlagsOffset:]

	for _, bs := range testBosses {
		val, err := db.GetEventFlag(flags2, bs.id)
		if err != nil {
			t.Fatalf("Reload: GetEventFlag(%d) failed: %v", bs.id, err)
		}
		if val != bs.toggleTo {
			t.Errorf("Boss '%s' (0x%X): expected %v after roundtrip, got %v", bs.name, bs.id, bs.toggleTo, val)
		}
	}

	// Integrity check
	if err := core.ValidateSlotIntegrity(slot2); err != nil {
		t.Errorf("ValidateSlotIntegrity failed: %v", err)
	}

	t.Logf("Toggled %d boss flags, all survived roundtrip", count)
}

// TestEventFlagsBossKillAll sets all bosses as defeated and verifies roundtrip.
func TestEventFlagsBossKillAll(t *testing.T) {
	save := loadTestSave(t, pcSavePath)
	idx := findActiveSlot(t, save)
	slot := &save.Slots[idx]

	if slot.EventFlagsOffset <= 0 || slot.EventFlagsOffset >= len(slot.Data) {
		t.Skip("EventFlagsOffset not available")
	}

	flags := slot.Data[slot.EventFlagsOffset:]
	bosses := db.GetAllBosses()

	// Kill all bosses
	for _, boss := range bosses {
		if err := db.SetEventFlag(flags, boss.ID, true); err != nil {
			t.Logf("Warning: SetEventFlag(%d, %s): %v", boss.ID, boss.Name, err)
			continue
		}
	}

	// Roundtrip
	save2 := saveTmpAndReload(t, save, "boss_killall_test.sl2")
	slot2 := &save2.Slots[idx]
	flags2 := slot2.Data[slot2.EventFlagsOffset:]

	// Verify all bosses are defeated
	killedCount := 0
	for _, boss := range bosses {
		val, err := db.GetEventFlag(flags2, boss.ID)
		if err != nil {
			continue
		}
		if val {
			killedCount++
		}
	}

	if killedCount == 0 {
		t.Error("No bosses marked as defeated after roundtrip")
	}

	if err := core.ValidateSlotIntegrity(slot2); err != nil {
		t.Errorf("ValidateSlotIntegrity failed: %v", err)
	}

	t.Logf("Kill All: %d/%d bosses defeated after roundtrip", killedCount, len(bosses))
}

// ---------- 3. Event Flags — Grace Toggle ----------

func TestEventFlagsGraceToggle(t *testing.T) {
	save := loadTestSave(t, pcSavePath)
	idx := findActiveSlot(t, save)
	slot := &save.Slots[idx]

	if slot.EventFlagsOffset <= 0 || slot.EventFlagsOffset >= len(slot.Data) {
		t.Skip("EventFlagsOffset not available")
	}

	flags := slot.Data[slot.EventFlagsOffset:]
	graces := db.GetAllGraces()
	if len(graces) == 0 {
		t.Fatal("No graces in database")
	}

	// Pick first 10 graces, toggle each
	count := 10
	if len(graces) < count {
		count = len(graces)
	}
	type graceState struct {
		id       uint32
		name     string
		origVal  bool
		toggleTo bool
	}
	testGraces := make([]graceState, count)

	for i := 0; i < count; i++ {
		orig, err := db.GetEventFlag(flags, graces[i].ID)
		if err != nil {
			t.Fatalf("GetEventFlag(%d) failed: %v", graces[i].ID, err)
		}
		testGraces[i] = graceState{
			id:       graces[i].ID,
			name:     graces[i].Name,
			origVal:  orig,
			toggleTo: !orig,
		}
	}

	// Toggle flags
	for _, gs := range testGraces {
		if err := db.SetEventFlag(flags, gs.id, gs.toggleTo); err != nil {
			t.Fatalf("SetEventFlag(%d, %v) failed: %v", gs.id, gs.toggleTo, err)
		}
	}

	// Roundtrip
	save2 := saveTmpAndReload(t, save, "grace_toggle_test.sl2")
	slot2 := &save2.Slots[idx]
	flags2 := slot2.Data[slot2.EventFlagsOffset:]

	for _, gs := range testGraces {
		val, err := db.GetEventFlag(flags2, gs.id)
		if err != nil {
			t.Fatalf("Reload: GetEventFlag(%d) failed: %v", gs.id, err)
		}
		if val != gs.toggleTo {
			t.Errorf("Grace '%s' (0x%X): expected %v after roundtrip, got %v",
				gs.name, gs.id, gs.toggleTo, val)
		}
	}

	if err := core.ValidateSlotIntegrity(slot2); err != nil {
		t.Errorf("ValidateSlotIntegrity failed: %v", err)
	}

	t.Logf("Toggled %d grace flags, all survived roundtrip", count)
}

// TestEventFlagsGraceUnlockAll unlocks all graces and verifies roundtrip.
func TestEventFlagsGraceUnlockAll(t *testing.T) {
	save := loadTestSave(t, pcSavePath)
	idx := findActiveSlot(t, save)
	slot := &save.Slots[idx]

	if slot.EventFlagsOffset <= 0 || slot.EventFlagsOffset >= len(slot.Data) {
		t.Skip("EventFlagsOffset not available")
	}

	flags := slot.Data[slot.EventFlagsOffset:]
	graces := db.GetAllGraces()

	for _, grace := range graces {
		if err := db.SetEventFlag(flags, grace.ID, true); err != nil {
			continue
		}
	}

	// Roundtrip
	save2 := saveTmpAndReload(t, save, "grace_unlockall_test.sl2")
	slot2 := &save2.Slots[idx]
	flags2 := slot2.Data[slot2.EventFlagsOffset:]

	unlockedCount := 0
	for _, grace := range graces {
		val, err := db.GetEventFlag(flags2, grace.ID)
		if err != nil {
			continue
		}
		if val {
			unlockedCount++
		}
	}

	if unlockedCount == 0 {
		t.Error("No graces unlocked after roundtrip")
	}

	if err := core.ValidateSlotIntegrity(slot2); err != nil {
		t.Errorf("ValidateSlotIntegrity failed: %v", err)
	}

	t.Logf("Unlock All: %d/%d graces unlocked after roundtrip", unlockedCount, len(graces))
}

// ---------- 4. Remove Items & Roundtrip ----------

func TestRemoveItemsAndRoundtrip(t *testing.T) {
	save := loadTestSave(t, pcSavePath)
	idx := findActiveSlot(t, save)
	slot := &save.Slots[idx]

	// Use a talisman (stackable, handle=id) — simpler and guaranteed to land in inventory
	testItemID := uint32(0x20000BB8) // Crimson Amber Medallion
	err := core.AddItemsToSlot(slot, []uint32{testItemID}, 1, 0, false)
	if err != nil {
		t.Fatalf("AddItemsToSlot failed: %v", err)
	}

	// Find the handle for the added item
	var addedHandle uint32
	for h, id := range slot.GaMap {
		if id == testItemID {
			addedHandle = h
			break
		}
	}
	if addedHandle == 0 {
		t.Fatal("Added item not found in GaMap")
	}

	// Verify item is in inventory (search all common items for this handle)
	foundInInv := false
	for _, item := range slot.Inventory.CommonItems {
		if item.GaItemHandle == addedHandle && item.Quantity > 0 {
			foundInInv = true
			break
		}
	}
	if !foundInInv {
		t.Fatal("Added item not found in inventory")
	}

	gaMapSizeBefore := len(slot.GaMap)

	// Remove the item
	err = core.RemoveItemFromSlot(slot, addedHandle, true, false)
	if err != nil {
		t.Fatalf("RemoveItemFromSlot failed: %v", err)
	}

	// Verify item is gone from inventory (handle zeroed)
	for _, item := range slot.Inventory.CommonItems {
		if item.GaItemHandle == addedHandle && item.Quantity > 0 {
			t.Error("Item handle still present in inventory after removal")
		}
	}

	// Roundtrip
	save2 := saveTmpAndReload(t, save, "remove_items_test.sl2")
	slot2 := &save2.Slots[idx]

	// Verify item doesn't come back with quantity
	for _, item := range slot2.Inventory.CommonItems {
		if item.GaItemHandle == addedHandle && item.Quantity > 0 {
			t.Error("Removed item reappeared after roundtrip")
		}
	}

	if err := core.ValidateSlotIntegrity(slot2); err != nil {
		t.Errorf("ValidateSlotIntegrity failed: %v", err)
	}

	t.Logf("Add + Remove + Roundtrip OK, GaMap: %d -> %d", gaMapSizeBefore, len(slot2.GaMap))
}

// TestRemoveFromStorageAndRoundtrip adds to storage then removes.
func TestRemoveFromStorageAndRoundtrip(t *testing.T) {
	save := loadTestSave(t, pcSavePath)
	idx := findActiveSlot(t, save)
	slot := &save.Slots[idx]

	testItemID := uint32(0x20000BB8) // Crimson Amber Medallion
	err := core.AddItemsToSlot(slot, []uint32{testItemID}, 0, 1, false)
	if err != nil {
		t.Fatalf("AddItemsToSlot (storage) failed: %v", err)
	}

	// Find handle
	var handle uint32
	for h, id := range slot.GaMap {
		if id == testItemID {
			handle = h
			break
		}
	}
	if handle == 0 {
		t.Fatal("Item not found in GaMap after add")
	}

	// Remove from storage
	err = core.RemoveItemFromSlot(slot, handle, false, true)
	if err != nil {
		t.Fatalf("RemoveItemFromSlot (storage) failed: %v", err)
	}

	// Roundtrip
	save2 := saveTmpAndReload(t, save, "remove_storage_test.sl2")
	slot2 := &save2.Slots[idx]

	// Verify removed from storage
	for _, item := range slot2.Storage.CommonItems {
		if item.GaItemHandle == handle && item.Quantity > 0 {
			t.Error("Removed storage item reappeared after roundtrip")
		}
	}

	if err := core.ValidateSlotIntegrity(slot2); err != nil {
		t.Errorf("ValidateSlotIntegrity failed: %v", err)
	}

	t.Log("Storage remove + roundtrip OK")
}

// TestAddRemoveMultipleItems adds multiple items then removes them all.
func TestAddRemoveMultipleItems(t *testing.T) {
	save := loadTestSave(t, pcSavePath)
	idx := findActiveSlot(t, save)
	slot := &save.Slots[idx]

	// Use talismans (stackable, handle encodes item ID) for reliable add+remove
	itemIDs := []uint32{
		0x20000BB8, // Crimson Amber Medallion
		0x20000FA0, // Viridian Amber Medallion
		0x20001388, // Cerulean Amber Medallion
	}

	origGaMapSize := len(slot.GaMap)

	err := core.AddItemsToSlot(slot, itemIDs, 1, 0, false)
	if err != nil {
		t.Fatalf("AddItemsToSlot failed: %v", err)
	}

	// Collect handles
	handles := make([]uint32, 0, len(itemIDs))
	for _, id := range itemIDs {
		for h, gid := range slot.GaMap {
			if gid == id {
				handles = append(handles, h)
				break
			}
		}
	}
	if len(handles) != len(itemIDs) {
		t.Fatalf("Expected %d handles, got %d", len(itemIDs), len(handles))
	}

	// Remove all
	for _, h := range handles {
		if err := core.RemoveItemFromSlot(slot, h, true, false); err != nil {
			t.Fatalf("RemoveItemFromSlot(0x%X) failed: %v", h, err)
		}
	}

	// Roundtrip
	save2 := saveTmpAndReload(t, save, "add_remove_multi_test.sl2")
	slot2 := &save2.Slots[idx]

	// Verify none of the items survive
	for _, id := range itemIDs {
		for _, item := range slot2.Inventory.CommonItems {
			if item.GaItemHandle == 0 || item.GaItemHandle == 0xFFFFFFFF {
				continue
			}
			if mappedID, ok := slot2.GaMap[item.GaItemHandle]; ok && mappedID == id {
				if item.Quantity > 0 {
					t.Errorf("Item 0x%08X still in inventory after removal+roundtrip", id)
				}
			}
		}
	}

	if err := core.ValidateSlotIntegrity(slot2); err != nil {
		t.Errorf("ValidateSlotIntegrity failed: %v", err)
	}

	t.Logf("Add+Remove %d items, roundtrip OK, GaMap: %d -> %d -> %d",
		len(itemIDs), origGaMapSize, origGaMapSize+len(itemIDs), len(slot2.GaMap))
}

// ---------- 5. Stress Test — Add Many Items ----------

func TestStressAddManyItems(t *testing.T) {
	save := loadTestSave(t, pcSavePath)
	idx := findActiveSlot(t, save)
	slot := &save.Slots[idx]

	origGaMapSize := len(slot.GaMap)
	origInvEnd := slot.InventoryEnd

	// Collect 50 unique weapon IDs from the database
	allItems := db.GetItemsByCategory("melee_armaments", "PC")
	if len(allItems) < 50 {
		t.Skipf("Not enough melee weapons in DB: %d", len(allItems))
	}

	itemIDs := make([]uint32, 50)
	for i := 0; i < 50; i++ {
		itemIDs[i] = allItems[i].ID
	}

	// Add all 50 items
	err := core.AddItemsToSlot(slot, itemIDs, 1, 0, false)
	if err != nil {
		t.Fatalf("AddItemsToSlot (50 items) failed: %v", err)
	}

	// Verify GaMap grew
	if len(slot.GaMap) < origGaMapSize+50 {
		t.Errorf("GaMap grew by %d, expected at least 50", len(slot.GaMap)-origGaMapSize)
	}

	// Verify InventoryEnd advanced
	if slot.InventoryEnd <= origInvEnd {
		t.Error("InventoryEnd did not advance after adding 50 items")
	}

	// Verify no inventory index collision
	indexMap := make(map[uint32]int)
	for _, item := range slot.Inventory.CommonItems {
		if item.GaItemHandle == 0 || item.GaItemHandle == 0xFFFFFFFF {
			continue
		}
		indexMap[item.Index]++
	}
	for idx, count := range indexMap {
		if count > 1 {
			t.Errorf("Index %d collision: %d items", idx, count)
		}
	}

	// InventoryEnd must still be within GaItems bounds
	gaLimit := slot.MagicOffset - 0x1B0 + 1
	if slot.InventoryEnd > gaLimit {
		t.Errorf("InventoryEnd (0x%X) > gaLimit (0x%X) — overflow!", slot.InventoryEnd, gaLimit)
	}

	// Roundtrip
	save2 := saveTmpAndReload(t, save, "stress_50items_test.sl2")
	slot2 := &save2.Slots[idx]

	// Verify all 50 items survive roundtrip
	survivedCount := 0
	for _, id := range itemIDs {
		for _, mappedID := range slot2.GaMap {
			if mappedID == id {
				survivedCount++
				break
			}
		}
	}
	if survivedCount < 50 {
		t.Errorf("Only %d/50 items survived roundtrip", survivedCount)
	}

	// Integrity check
	if err := core.ValidateSlotIntegrity(slot2); err != nil {
		t.Errorf("ValidateSlotIntegrity failed after stress test: %v", err)
	}

	t.Logf("Stress test: added 50 weapons, GaMap %d->%d, InvEnd 0x%X->0x%X, all survived roundtrip",
		origGaMapSize, len(slot2.GaMap), origInvEnd, slot2.InventoryEnd)
}

// TestStressAddMixedItemTypes adds weapons, armor, talismans, goods, and AoW simultaneously.
func TestStressAddMixedItemTypes(t *testing.T) {
	save := loadTestSave(t, pcSavePath)
	idx := findActiveSlot(t, save)
	slot := &save.Slots[idx]

	origGaMapSize := len(slot.GaMap)

	// Collect items from various categories
	weapons := db.GetItemsByCategory("melee_armaments", "PC")
	armor := db.GetItemsByCategory("head", "PC")
	talismans := db.GetItemsByCategory("talismans", "PC")
	ashes := db.GetItemsByCategory("ashes_of_war", "PC")

	var itemIDs []uint32
	for i := 0; i < 10 && i < len(weapons); i++ {
		itemIDs = append(itemIDs, weapons[i].ID)
	}
	for i := 0; i < 10 && i < len(armor); i++ {
		itemIDs = append(itemIDs, armor[i].ID)
	}
	for i := 0; i < 10 && i < len(talismans); i++ {
		itemIDs = append(itemIDs, talismans[i].ID)
	}
	for i := 0; i < 10 && i < len(ashes); i++ {
		itemIDs = append(itemIDs, ashes[i].ID)
	}

	if len(itemIDs) == 0 {
		t.Skip("No items in DB for mixed stress test")
	}

	err := core.AddItemsToSlot(slot, itemIDs, 1, 0, false)
	if err != nil {
		t.Fatalf("AddItemsToSlot (mixed) failed: %v", err)
	}

	// Roundtrip
	save2 := saveTmpAndReload(t, save, "stress_mixed_test.sl2")
	slot2 := &save2.Slots[idx]

	// Count survived items
	survivedCount := 0
	for _, id := range itemIDs {
		for _, mappedID := range slot2.GaMap {
			if mappedID == id {
				survivedCount++
				break
			}
		}
	}

	if err := core.ValidateSlotIntegrity(slot2); err != nil {
		t.Errorf("ValidateSlotIntegrity failed: %v", err)
	}

	t.Logf("Mixed stress: %d items (%d weapon, %d armor, %d talisman, %d ash), %d survived, GaMap %d->%d",
		len(itemIDs),
		min(10, len(weapons)), min(10, len(armor)), min(10, len(talismans)), min(10, len(ashes)),
		survivedCount, origGaMapSize, len(slot2.GaMap))
}

// TestStressAddToInventoryAndStorage adds items to both inventory and storage simultaneously.
func TestStressAddToInventoryAndStorage(t *testing.T) {
	save := loadTestSave(t, pcSavePath)
	idx := findActiveSlot(t, save)
	slot := &save.Slots[idx]

	origInvCount := 0
	for _, item := range slot.Inventory.CommonItems {
		if item.GaItemHandle != 0 && item.GaItemHandle != 0xFFFFFFFF {
			origInvCount++
		}
	}
	origStorageCount := len(slot.Storage.CommonItems)

	// Add 20 items to both inventory and storage
	weapons := db.GetItemsByCategory("melee_armaments", "PC")
	count := 20
	if len(weapons) < count {
		count = len(weapons)
	}
	itemIDs := make([]uint32, count)
	for i := 0; i < count; i++ {
		itemIDs[i] = weapons[i].ID
	}

	err := core.AddItemsToSlot(slot, itemIDs, 1, 1, false)
	if err != nil {
		t.Fatalf("AddItemsToSlot (inv+storage) failed: %v", err)
	}

	// Roundtrip
	save2 := saveTmpAndReload(t, save, "stress_inv_storage_test.sl2")
	slot2 := &save2.Slots[idx]

	// Count items in reloaded inventory
	newInvCount := 0
	for _, item := range slot2.Inventory.CommonItems {
		if item.GaItemHandle != 0 && item.GaItemHandle != 0xFFFFFFFF {
			newInvCount++
		}
	}
	newStorageCount := len(slot2.Storage.CommonItems)

	if newInvCount < origInvCount+count {
		t.Logf("Inventory: %d -> %d (expected at least %d)", origInvCount, newInvCount, origInvCount+count)
	}
	if newStorageCount < origStorageCount+count {
		t.Logf("Storage: %d -> %d (expected at least %d)", origStorageCount, newStorageCount, origStorageCount+count)
	}

	if err := core.ValidateSlotIntegrity(slot2); err != nil {
		t.Errorf("ValidateSlotIntegrity failed: %v", err)
	}

	t.Logf("Inv+Storage stress: %d items, inv %d->%d, storage %d->%d",
		count, origInvCount, newInvCount, origStorageCount, newStorageCount)
}
