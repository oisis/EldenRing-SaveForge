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
	itemID := uint32(0x40000000) // AoW item ID
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
	if err := allocateGaItem(slot, aowHandle, 0x40000000); err != nil {
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
	if slot.GaItems[1].Handle != weaponHandle {
		t.Errorf("Index 1 should be Weapon, got 0x%X", slot.GaItems[1].Handle)
	}
	if slot.NextAoWIndex != 1 {
		t.Errorf("NextAoWIndex should be 1, got %d", slot.NextAoWIndex)
	}
	if slot.NextArmamentIndex != 2 {
		t.Errorf("NextArmamentIndex should be 2, got %d", slot.NextArmamentIndex)
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
		{ItemTypeAow | 0x00800001, 0x40000001},    // AoW 1
		{ItemTypeWeapon | 0x00800002, 0x00100002},  // Weapon 1
		{ItemTypeArmor | 0x00800003, 0x10100003},   // Armor 1
		{ItemTypeAow | 0x00800004, 0x40000004},     // AoW 2
		{ItemTypeWeapon | 0x00800005, 0x00100005},   // Weapon 2
	}

	for _, it := range items {
		if err := allocateGaItem(slot, it.handle, it.itemID); err != nil {
			t.Fatalf("allocateGaItem handle=0x%X: %v", it.handle, err)
		}
	}

	// Expected layout: [AoW1, AoW2, Weapon1, Armor1, Weapon2]
	// AoW entries should be at indices 0-1 (both AoWs)
	// Non-AoW entries should be at indices 2+ (weapons, armor)
	aowCount := 0
	nonAowStart := -1
	for i := 0; i < 5; i++ {
		h := slot.GaItems[i].Handle
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
	if slot.NextArmamentIndex != 5 {
		t.Errorf("NextArmamentIndex should be 5, got %d", slot.NextArmamentIndex)
	}
}

func TestAllocateGaItem_ShiftPreservesExisting(t *testing.T) {
	slot := makeTestSlot(100)

	// Pre-populate: weapon at index 0
	slot.GaItems[0] = GaItemFull{
		Handle: ItemTypeWeapon | 0x00800001,
		ItemID: 0x00100001,
		Unk2:   -1, Unk3: -1, AoWGaItemHandle: 0xFFFFFFFF,
	}
	slot.NextAoWIndex = 0
	slot.NextArmamentIndex = 1

	// Insert AoW — should shift weapon right
	aowHandle := uint32(ItemTypeAow | 0x00800002)
	if err := allocateGaItem(slot, aowHandle, 0x40000002); err != nil {
		t.Fatalf("allocateGaItem AoW: %v", err)
	}

	if slot.GaItems[0].Handle != aowHandle {
		t.Errorf("Index 0 should be AoW after shift, got 0x%X", slot.GaItems[0].Handle)
	}
	if slot.GaItems[1].Handle != (ItemTypeWeapon | 0x00800001) {
		t.Errorf("Index 1 should be shifted weapon, got 0x%X", slot.GaItems[1].Handle)
	}
}

func TestGenerateUniqueHandle_GlobalCounter(t *testing.T) {
	slot := makeTestSlot(100)
	slot.NextGaItemHandle = 42
	slot.PartGaItemHandle = 0x80

	h1, err := generateUniqueHandle(slot, ItemTypeWeapon)
	if err != nil {
		t.Fatalf("generateUniqueHandle weapon: %v", err)
	}
	if h1&0xFFFF != 42 {
		t.Errorf("Expected counter 42, got %d (handle=0x%X)", h1&0xFFFF, h1)
	}
	if (h1>>16)&0xFF != 0x80 {
		t.Errorf("Expected partID 0x80, got 0x%X", (h1>>16)&0xFF)
	}
	slot.GaMap[h1] = 0x00100000

	h2, err := generateUniqueHandle(slot, ItemTypeArmor)
	if err != nil {
		t.Fatalf("generateUniqueHandle armor: %v", err)
	}
	if h2&0xFFFF != 43 {
		t.Errorf("Expected counter 43 (global increment), got %d (handle=0x%X)", h2&0xFFFF, h2)
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

// TestAllocateGaItem_ReturnsFullErrorAtCapacity locks the allocator's
// boundary contract: when NextArmamentIndex (or NextAoWIndex for AoW) has
// reached len(GaItems), allocateGaItem must surface "armament/armor array
// full" (or "AoW array full") instead of silently reusing pre-Next empty
// slots. Reusing pre-Next holes would break the save format's monotonic
// handle-counter ordering inside the armament zone.
//
// Note: the GaItems array can contain empty slots at indices < NextArmamentIndex
// (left over from in-game deletions); the test seeds such a layout so the
// boundary check cannot accidentally be satisfied by "scan back for an empty
// hole" logic.
func TestAllocateGaItem_ReturnsFullErrorAtCapacity(t *testing.T) {
	t.Run("Weapon at capacity", func(t *testing.T) {
		slot := makeTestSlot(8)
		// Layout: indices 0..7 occupied except 2 and 4 (pre-Next holes).
		// NextArmamentIndex = 8 → no room above.
		for i := 0; i < 8; i++ {
			if i == 2 || i == 4 {
				continue
			}
			slot.GaItems[i] = GaItemFull{
				Handle:          uint32(ItemTypeWeapon | 0x00800000 | uint32(i)),
				ItemID:          uint32(0x00100000 + i),
				Unk2:            -1,
				Unk3:            -1,
				AoWGaItemHandle: NoCustomAoWHandle,
			}
		}
		slot.NextAoWIndex = 0
		slot.NextArmamentIndex = 8 // == len(GaItems) — capacity exhausted

		err := allocateGaItem(slot, uint32(ItemTypeWeapon|0x00800009), 0x00200000)
		if err == nil {
			t.Fatal("allocateGaItem must error when NextArmamentIndex == len(GaItems); got nil")
		}
		if !contains(err.Error(), "armament/armor array full") {
			t.Errorf("expected 'armament/armor array full' error, got: %v", err)
		}

		// Pre-Next holes (indices 2, 4) must remain empty — allocator must
		// not silently fill them.
		if !slot.GaItems[2].IsEmpty() {
			t.Error("index 2 (pre-Next hole) was overwritten — allocator broke monotonic ordering")
		}
		if !slot.GaItems[4].IsEmpty() {
			t.Error("index 4 (pre-Next hole) was overwritten — allocator broke monotonic ordering")
		}
	})

	t.Run("AoW at capacity", func(t *testing.T) {
		slot := makeTestSlot(4)
		for i := 0; i < 4; i++ {
			slot.GaItems[i] = GaItemFull{
				Handle:          uint32(ItemTypeAow | 0x00800000 | uint32(i)),
				ItemID:          uint32(0x40000000 + i),
				Unk2:            -1,
				Unk3:            -1,
				AoWGaItemHandle: NoCustomAoWHandle,
			}
		}
		slot.NextAoWIndex = 4 // == len(GaItems) — full
		slot.NextArmamentIndex = 4

		err := allocateGaItem(slot, uint32(ItemTypeAow|0x00800009), 0x40000099)
		if err == nil {
			t.Fatal("allocateGaItem (AoW) must error when NextAoWIndex == len(GaItems); got nil")
		}
		if !contains(err.Error(), "AoW array full") {
			t.Errorf("expected 'AoW array full' error, got: %v", err)
		}
	})
}

// TestAllocateGaItem_AoWRejectsWhenArmamentZoneAtCapacity locks the fix for the
// "NextArmamentIndex N > len(GaItems) N" rollback that fires when an AoW is
// added to a slot whose armament zone is already pinned to the last array
// index (e.g. an in-game entry placed at maxEntries-1). The AoW branch must
// reject upfront instead of incrementing NextArmamentIndex past maxEntries —
// the post-mutation validator would otherwise surface a confusing numeric
// violation. Observed on a real PS4 save's slot 1 ("Bydlaczka") where vanilla
// state has NextAoWIndex=3 with room, but NextArmamentIndex==len(GaItems)
// because the highest-counter entry sits at array index maxEntries-1.
func TestAllocateGaItem_AoWRejectsWhenArmamentZoneAtCapacity(t *testing.T) {
	slot := makeTestSlot(8)
	// AoW zone has room (NextAoW=3 < 8), but armament zone is "logically
	// full": some non-empty entry sits at the last array index, forcing
	// NextArmamentIndex to maxEntries on load.
	slot.GaItems[7] = GaItemFull{
		Handle:          uint32(ItemTypeWeapon | 0x00800007),
		ItemID:          uint32(0x00100007),
		Unk2:            -1,
		Unk3:            -1,
		AoWGaItemHandle: NoCustomAoWHandle,
	}
	slot.NextAoWIndex = 3
	slot.NextArmamentIndex = 8 // == len(GaItems)
	slot.NextGaItemHandle = 9

	err := allocateGaItem(slot, uint32(ItemTypeAow|0x00800009), 0x40000099)
	if err == nil {
		t.Fatal("allocateGaItem (AoW) must reject when NextArmamentIndex == len(GaItems); got nil")
	}
	if !contains(err.Error(), "armament zone at capacity") {
		t.Errorf("expected 'armament zone at capacity' error, got: %v", err)
	}
	// Critical: state must not be mutated.
	if slot.NextAoWIndex != 3 {
		t.Errorf("NextAoWIndex must stay 3 on rejection, got %d", slot.NextAoWIndex)
	}
	if slot.NextArmamentIndex != 8 {
		t.Errorf("NextArmamentIndex must stay 8 on rejection (no overflow past maxEntries=8), got %d", slot.NextArmamentIndex)
	}
	if !slot.GaItems[3].IsEmpty() {
		t.Error("index 3 (NextAoW slot) must remain empty on rejection")
	}
	// Validator must be happy with this (pre-mutation) state.
	if v := ValidatePostMutation(slot); len(v) > 0 {
		t.Errorf("ValidatePostMutation expected OK after rejection; got %d violations: %v", len(v), v)
	}
}

// TestAllocateGaItem_RejectsNonCanonicalInversion locks the fail-closed guard
// against inherited/non-canonical GaItem layouts where NextArmamentIndex sits
// below NextAoWIndex. The native writer emits all AoW records first, so on a
// canonical save NextArmamentIndex >= NextAoWIndex always holds. scanGaItems
// can nonetheless produce an inverted state (NextArmamentIndex derived from the
// highest-counter record, which may live below the last AoW). Allocating a
// Weapon/Armor at NextArmamentIndex would drop it inside the AoW block and make
// the linear RebuildSlotFull diverge from the two-pass native writer, so the
// allocator must reject before mutating anything.
func TestAllocateGaItem_RejectsNonCanonicalInversion(t *testing.T) {
	// Synthetic inverted layout: AoW at indices 0..1, NextAoWIndex=2,
	// NextArmamentIndex=1 (< NextAoWIndex).
	newInvertedSlot := func() *SaveSlot {
		slot := makeTestSlot(8)
		for i := 0; i < 2; i++ {
			slot.GaItems[i] = GaItemFull{
				Handle:          uint32(ItemTypeAow | 0x00800000 | uint32(i)),
				ItemID:          uint32(0x40000000 + i),
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
			before := append([]GaItemFull(nil), slot.GaItems...)

			err := allocateGaItem(slot, tc.handle, tc.itemID)
			if err == nil {
				t.Fatalf("allocateGaItem %s must reject inverted layout; got nil", tc.name)
			}
			if !contains(err.Error(), "non-canonical GaItem layout") {
				t.Errorf("expected 'non-canonical GaItem layout' error, got: %v", err)
			}

			// Proof of no mutation: array, both cursors, handle counter unchanged.
			for i := range before {
				if slot.GaItems[i] != before[i] {
					t.Errorf("GaItems[%d] mutated on rejection: %+v -> %+v", i, before[i], slot.GaItems[i])
				}
			}
			if slot.NextAoWIndex != 2 {
				t.Errorf("NextAoWIndex must stay 2 on rejection, got %d", slot.NextAoWIndex)
			}
			if slot.NextArmamentIndex != 1 {
				t.Errorf("NextArmamentIndex must stay 1 on rejection, got %d", slot.NextArmamentIndex)
			}
			if slot.NextGaItemHandle != 3 {
				t.Errorf("NextGaItemHandle must stay 3 on rejection, got %d", slot.NextGaItemHandle)
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
			ItemID:          uint32(0x40000000 + i),
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
	if slot.GaItems[2].Handle != weaponHandle {
		t.Errorf("Weapon should land at index 2, got handle 0x%X", slot.GaItems[2].Handle)
	}
	if slot.NextArmamentIndex != 3 {
		t.Errorf("NextArmamentIndex should advance to 3, got %d", slot.NextArmamentIndex)
	}

	armorHandle := uint32(ItemTypeArmor | 0x00800004)
	if err := allocateGaItem(slot, armorHandle, 0x10100000); err != nil {
		t.Fatalf("allocateGaItem Armor on canonical layout: %v", err)
	}
	if slot.GaItems[3].Handle != armorHandle {
		t.Errorf("Armor should land at index 3, got handle 0x%X", slot.GaItems[3].Handle)
	}
	if slot.NextArmamentIndex != 4 {
		t.Errorf("NextArmamentIndex should advance to 4, got %d", slot.NextArmamentIndex)
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
