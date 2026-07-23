package core

import (
	"testing"
)

// makeTestSlot creates a minimal SaveSlot with empty GaItems array for placement tests.
func makeTestSlot(numEntries int) *SaveSlot {
	slot := &SaveSlot{
		Data:             make([]byte, SlotSize),
		Version:          100,
		GaMap:            make(map[uint32]uint32),
		GaItems:          make([]GaItemFull, numEntries),
		MagicOffset:      0x15420 + 432,
		PartGaItemHandle: 0x80,
		NextGaItemHandle: 1,
	}
	// Initialize all entries as empty
	for i := range slot.GaItems {
		slot.GaItems[i] = GaItemFull{Unk2: -1, Unk3: -1, AoWGaItemHandle: 0xFFFFFFFF}
	}
	return slot
}

func TestAllocateGaItem_AoWAtLowIndex(t *testing.T) {
	slot := makeTestSlot(100)
	slot.NextAoWIndex = 0
	slot.NextArmamentIndex = 0

	// Add an AoW
	handle := uint32(ItemTypeAow | 0x00800001)
	itemID := uint32(0x80000000) // AoW item ID
	if err := allocateGaItem(slot, handle, itemID); err != nil {
		t.Fatalf("allocateGaItem AoW: %v", err)
	}

	if slot.GaItems[0].Handle != handle {
		t.Errorf("AoW should be at index 0, got handle 0x%X at index 0", slot.GaItems[0].Handle)
	}
	if slot.NextAoWIndex != 1 {
		t.Errorf("NextAoWIndex should be 1, got %d", slot.NextAoWIndex)
	}
	if slot.NextArmamentIndex != 1 {
		t.Errorf("NextArmamentIndex should be 1 (shifted by AoW insert), got %d", slot.NextArmamentIndex)
	}
}

func TestAllocateGaItem_WeaponAfterAoW(t *testing.T) {
	slot := makeTestSlot(100)
	slot.NextAoWIndex = 0
	slot.NextArmamentIndex = 0

	// Add AoW first
	aowHandle := uint32(ItemTypeAow | 0x00800001)
	if err := allocateGaItem(slot, aowHandle, 0x80000000); err != nil {
		t.Fatalf("allocateGaItem AoW: %v", err)
	}

	// Add weapon — should go after AoW
	weaponHandle := uint32(ItemTypeWeapon | 0x00800002)
	weaponID := uint32(0x00100000) // weapon item ID
	if err := allocateGaItem(slot, weaponHandle, weaponID); err != nil {
		t.Fatalf("allocateGaItem Weapon: %v", err)
	}

	if slot.GaItems[0].Handle != aowHandle {
		t.Errorf("Index 0 should be AoW, got 0x%X", slot.GaItems[0].Handle)
	}
	if slot.GaItems[2].Handle != weaponHandle {
		t.Errorf("Projected index 2 should be Weapon, got 0x%X", slot.GaItems[2].Handle)
	}
	if slot.NextAoWIndex != 1 {
		t.Errorf("NextAoWIndex should be 1, got %d", slot.NextAoWIndex)
	}
	if slot.NextArmamentIndex != 3 {
		t.Errorf("NextArmamentIndex should be 3, got %d", slot.NextArmamentIndex)
	}
}

func TestAllocateGaItem_TypeSegregation(t *testing.T) {
	slot := makeTestSlot(100)
	slot.NextAoWIndex = 0
	slot.NextArmamentIndex = 0

	// Add: AoW, Weapon, Armor, AoW, Weapon
	items := []struct {
		handle uint32
		itemID uint32
	}{
		{ItemTypeAow | 0x00800001, 0x80000001},    // AoW 1
		{ItemTypeWeapon | 0x00800002, 0x00100002}, // Weapon 1
		{ItemTypeArmor | 0x00800003, 0x10100003},  // Armor 1
		{ItemTypeAow | 0x00800004, 0x80000004},    // AoW 2
		{ItemTypeWeapon | 0x00800005, 0x00100005}, // Weapon 2
	}

	for _, it := range items {
		if err := allocateGaItem(slot, it.handle, it.itemID); err != nil {
			t.Fatalf("allocateGaItem handle=0x%X: %v", it.handle, err)
		}
	}

	// Native layout keeps an empty second-pass marker for physical index 0:
	// [AoW1, AoW2, empty, Weapon1, Armor1, Weapon2].
	aowCount := 0
	nonAowStart := -1
	for i := 0; i < 6; i++ {
		h := slot.GaItems[i].Handle
		if slot.GaItems[i].IsEmpty() {
			continue
		}
		isAoW := (h & GaHandleTypeMask) == ItemTypeAow
		if isAoW {
			if nonAowStart >= 0 {
				t.Errorf("AoW at index %d found AFTER non-AoW at index %d — type segregation broken", i, nonAowStart)
			}
			aowCount++
		} else if nonAowStart < 0 {
			nonAowStart = i
		}
	}

	if aowCount != 2 {
		t.Errorf("Expected 2 AoW entries, got %d", aowCount)
	}
	if slot.NextAoWIndex != 2 {
		t.Errorf("NextAoWIndex should be 2, got %d", slot.NextAoWIndex)
	}
	if slot.NextArmamentIndex != 6 {
		t.Errorf("NextArmamentIndex should be 6, got %d", slot.NextArmamentIndex)
	}
}

func TestAllocateGaItem_ShiftPreservesExisting(t *testing.T) {
	slot := makeTestSlot(100)

	// Physical index 1 projects to record 1 before an AoW exists.
	slot.GaItems[1] = GaItemFull{
		Handle: ItemTypeWeapon | 0x00800001,
		ItemID: 0x00100001,
		Unk2:   -1, Unk3: -1, AoWGaItemHandle: 0xFFFFFFFF,
	}
	slot.NextAoWIndex = 0
	slot.NextArmamentIndex = 1

	// Insert AoW at physical index 2. The prefix insertion removes the marker
	// for index 2 and shifts the existing weapon to its new projected record 2.
	aowHandle := uint32(ItemTypeAow | 0x00800002)
	if err := allocateGaItem(slot, aowHandle, 0x80000002); err != nil {
		t.Fatalf("allocateGaItem AoW: %v", err)
	}

	if slot.GaItems[0].Handle != aowHandle {
		t.Errorf("Index 0 should be AoW after shift, got 0x%X", slot.GaItems[0].Handle)
	}
	if slot.GaItems[2].Handle != (ItemTypeWeapon | 0x00800001) {
		t.Errorf("Index 2 should be shifted weapon, got 0x%X", slot.GaItems[2].Handle)
	}
}

func TestGenerateUniqueHandle_PhysicalUsesFirstFreeIndex(t *testing.T) {
	slot := makeTestSlot(100)
	slot.NextGaItemHandle = 42
	slot.PartGaItemHandle = 0x80

	h1, err := generateUniqueHandle(slot, ItemTypeWeapon)
	if err != nil {
		t.Fatalf("generateUniqueHandle weapon: %v", err)
	}
	if h1&0xFFFF != 0 {
		t.Errorf("Expected first free physical index 0, got %d (handle=0x%X)", h1&0xFFFF, h1)
	}
	if (h1>>16)&0xFF != 0x80 {
		t.Errorf("Expected partID 0x80, got 0x%X", (h1>>16)&0xFF)
	}
	if err := allocateGaItem(slot, h1, 0x00100000); err != nil {
		t.Fatalf("allocateGaItem weapon: %v", err)
	}
	slot.GaMap[h1] = 0x00100000

	h2, err := generateUniqueHandle(slot, ItemTypeArmor)
	if err != nil {
		t.Fatalf("generateUniqueHandle armor: %v", err)
	}
	if h2&0xFFFF != 1 {
		t.Errorf("Expected next free physical index 1, got %d (handle=0x%X)", h2&0xFFFF, h2)
	}
	if h2&GaHandleTypeMask != ItemTypeArmor {
		t.Errorf("Expected armor type prefix, got 0x%X", h2&GaHandleTypeMask)
	}
}

func TestScanGaItems_TrackedIndices(t *testing.T) {
	// Build a minimal slot with known GaItems in binary data.
	slot := &SaveSlot{
		Data:        make([]byte, SlotSize),
		Version:     100,
		MagicOffset: 0x15420 + 432,
	}

	// Write 3 entries at GaItemsStart (0x20):
	// [0] AoW: handle=0xC0800001, itemID=0x40000001 (8 bytes)
	// [1] AoW: handle=0xC0800005, itemID=0x40000002 (8 bytes)
	// [2] Weapon: handle=0x80800003, itemID=0x00100000 (21 bytes)
	off := GaItemsStart
	writeEntry := func(handle, itemID uint32) {
		le := func(buf []byte, v uint32) {
			buf[0] = byte(v)
			buf[1] = byte(v >> 8)
			buf[2] = byte(v >> 16)
			buf[3] = byte(v >> 24)
		}
		le(slot.Data[off:], handle)
		le(slot.Data[off+4:], itemID)
		recSize := GaItemRecordSize(itemID)
		if recSize >= GaRecordArmor {
			le(slot.Data[off+8:], 0xFFFFFFFF)  // unk2
			le(slot.Data[off+12:], 0xFFFFFFFF) // unk3
		}
		if recSize >= GaRecordWeapon {
			le(slot.Data[off+16:], 0xFFFFFFFF) // AoWGaItemHandle
			slot.Data[off+20] = 0              // unk5
		}
		off += recSize
	}

	writeEntry(0xC0800001, 0x40000001) // AoW
	writeEntry(0xC0800005, 0x40000002) // AoW
	writeEntry(0x80800003, 0x00100000) // Weapon

	slot.scanGaItems(GaItemsStart)

	if slot.NextAoWIndex != 2 {
		t.Errorf("NextAoWIndex: expected 2, got %d", slot.NextAoWIndex)
	}
	// Entry with highest counter (0x0005) is at index 1 → NextArmamentIndex = 2
	if slot.NextArmamentIndex != 2 {
		t.Errorf("NextArmamentIndex: expected 2, got %d", slot.NextArmamentIndex)
	}
	if slot.NextGaItemHandle != 6 {
		t.Errorf("NextGaItemHandle: expected 6 (max counter 5 + 1), got %d", slot.NextGaItemHandle)
	}
	if slot.PartGaItemHandle != 0x80 {
		t.Errorf("PartGaItemHandle: expected 0x80, got 0x%X", slot.PartGaItemHandle)
	}
}

// A table is full only when every physical index is occupied. Legacy cursor
// exhaustion is not a capacity condition.
func TestAllocateGaItem_ReturnsFullErrorAtCapacity(t *testing.T) {
	slot := makeTestSlot(8)
	for i := range slot.GaItems {
		handle := uint32(ItemTypeWeapon | gaItemHandleValidBit | uint32(i))
		slot.GaItems[i] = nativeTestRecord(handle, uint32(0x00100000+i))
		slot.GaMap[handle] = uint32(0x00100000 + i)
	}
	if _, err := generateUniqueHandle(slot, ItemTypeWeapon); err == nil || !contains(err.Error(), "no free index") {
		t.Fatalf("generateUniqueHandle error = %v, want no free index", err)
	}
	if err := allocateGaItem(slot, ItemTypeArmor|gaItemHandleValidBit, 0x10100000); err == nil || !contains(err.Error(), "already occupied") {
		t.Fatalf("allocateGaItem error = %v, want occupied index", err)
	}
}

func TestAllocateGaItem_AoWUsesHoleWhenLegacyCursorAtCapacity(t *testing.T) {
	slot := makeTestSlot(8)
	slot.GaItems[7] = nativeTestRecord(ItemTypeWeapon|gaItemHandleValidBit|7, 0x00100007)
	slot.NextArmamentIndex = len(slot.GaItems)
	if err := allocateGaItem(slot, ItemTypeAow|gaItemHandleValidBit, 0x80000099); err != nil {
		t.Fatalf("allocateGaItem should reuse physical index 0: %v", err)
	}
	if slot.GaItems[0].Handle != ItemTypeAow|gaItemHandleValidBit {
		t.Fatalf("new AoW handle at record 0 = 0x%08X", slot.GaItems[0].Handle)
	}
}

// Legacy cursor fields are derived metadata and cannot veto an otherwise valid
// native projection.
func TestAllocateGaItem_IgnoresLegacyCursorInversion(t *testing.T) {
	newInvertedSlot := func() *SaveSlot {
		slot := makeTestSlot(8)
		for i := 0; i < 2; i++ {
			slot.GaItems[i] = GaItemFull{
				Handle:          uint32(ItemTypeAow | 0x00800000 | uint32(i)),
				ItemID:          uint32(0x80000000 + i),
				Unk2:            -1,
				Unk3:            -1,
				AoWGaItemHandle: NoCustomAoWHandle,
			}
		}
		slot.NextAoWIndex = 2
		slot.NextArmamentIndex = 1 // inverted: < NextAoWIndex
		slot.NextGaItemHandle = 3
		return slot
	}

	for _, tc := range []struct {
		name   string
		handle uint32
		itemID uint32
	}{
		{"Weapon", ItemTypeWeapon | 0x00800003, 0x00100000},
		{"Armor", ItemTypeArmor | 0x00800003, 0x10100000},
	} {
		t.Run(tc.name, func(t *testing.T) {
			slot := newInvertedSlot()
			err := allocateGaItem(slot, tc.handle, tc.itemID)
			if err != nil {
				t.Fatalf("allocateGaItem %s: %v", tc.name, err)
			}
			if slot.GaItems[3].Handle != tc.handle {
				t.Fatalf("projected record 3 handle = 0x%08X, want 0x%08X", slot.GaItems[3].Handle, tc.handle)
			}
		})
	}
}

// TestAllocateGaItem_CanonicalLayoutStillAllocates guards against the fail-closed
// check over-firing: a canonical layout (NextArmamentIndex >= NextAoWIndex) must
// still place Weapon/Armor normally.
func TestAllocateGaItem_CanonicalLayoutStillAllocates(t *testing.T) {
	slot := makeTestSlot(8)
	for i := 0; i < 2; i++ {
		slot.GaItems[i] = GaItemFull{
			Handle:          uint32(ItemTypeAow | 0x00800000 | uint32(i)),
			ItemID:          uint32(0x80000000 + i),
			Unk2:            -1,
			Unk3:            -1,
			AoWGaItemHandle: NoCustomAoWHandle,
		}
	}
	slot.NextAoWIndex = 2
	slot.NextArmamentIndex = 2 // canonical: == NextAoWIndex

	weaponHandle := uint32(ItemTypeWeapon | 0x00800003)
	if err := allocateGaItem(slot, weaponHandle, 0x00100000); err != nil {
		t.Fatalf("allocateGaItem Weapon on canonical layout: %v", err)
	}
	if slot.GaItems[3].Handle != weaponHandle {
		t.Errorf("Weapon should land at projected record 3, got handle 0x%X", slot.GaItems[3].Handle)
	}
	if slot.NextArmamentIndex != 4 {
		t.Errorf("NextArmamentIndex should derive to 4, got %d", slot.NextArmamentIndex)
	}

	armorHandle := uint32(ItemTypeArmor | 0x00800004)
	if err := allocateGaItem(slot, armorHandle, 0x10100000); err != nil {
		t.Fatalf("allocateGaItem Armor on canonical layout: %v", err)
	}
	if slot.GaItems[4].Handle != armorHandle {
		t.Errorf("Armor should land at projected record 4, got handle 0x%X", slot.GaItems[4].Handle)
	}
	if slot.NextArmamentIndex != 5 {
		t.Errorf("NextArmamentIndex should derive to 5, got %d", slot.NextArmamentIndex)
	}
}

func contains(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
