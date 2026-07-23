package core

import (
	"encoding/binary"
	"testing"
)

// repackPreflightFixture is the smallest healthy slot accepted by the repack
// preflight. Keep it deliberately narrow for refusal tests; richer regression
// cases use fragmentedRepackReferenceFixture below.
func repackPreflightFixture() *SaveSlot {
	slot := &SaveSlot{
		Data:              make([]byte, SlotSize),
		Version:           1,
		GaMap:             make(map[uint32]uint32),
		MagicOffset:       1000,
		InventoryEnd:      GaItemsStart,
		PlayerDataOffset:  1000,
		FaceDataOffset:    2000,
		StorageBoxOffset:  2000,
		GaItemDataOffset:  0x8000,
		SectionMap:        []SectionRange{{Name: "all", Start: 0, End: SlotSize}},
		NextAoWIndex:      0,
		NextArmamentIndex: 1,
		NextGaItemHandle:  2,
		PartGaItemHandle:  0x80,
	}
	weapon := GaItemFull{Handle: ItemTypeWeapon | 1, ItemID: 1}
	slot.GaItems = []GaItemFull{weapon, {}, {}, {}}
	slot.GaMap[weapon.Handle] = weapon.ItemID
	return slot
}

type repackFixtureHandles struct {
	AoW       uint32
	Weapon    uint32
	Armor     uint32
	NakedHead uint32
	Unarmed   uint32
}

type repackReferenceFixture struct {
	Slot    *SaveSlot
	Handles repackFixtureHandles
}

// fragmentedRepackReferenceFixture is the reusable, in-memory source for the
// repack regression suite. It deliberately contains holes before the exhausted
// armament cursor and exercises the reference classes that repack must preserve:
// a weapon -> AoW link, ordinary armor, and both game technical placeholders
// (naked armor and Unarmed). It never reads or writes tmp/save fixtures.
func fragmentedRepackReferenceFixture(t *testing.T) repackReferenceFixture {
	t.Helper()

	handles := repackFixtureHandles{
		AoW:       ItemTypeAow | gaItemHandleValidBit | 0x101,
		Weapon:    ItemTypeWeapon | gaItemHandleValidBit | 0x102,
		Armor:     ItemTypeArmor | gaItemHandleValidBit | 0x103,
		NakedHead: ItemTypeArmor | gaItemHandleValidBit | 0x104,
		Unarmed:   ItemTypeWeapon | gaItemHandleValidBit | 0x105,
	}
	slot := repackPreflightFixture()
	slot.GaItems = []GaItemFull{
		{Handle: handles.AoW, ItemID: 0x80000001, Unk2: -1, Unk3: -2, AoWGaItemHandle: LegacyNoCustomAoWHandle, Unk5: 1},
		{},
		{Handle: handles.Weapon, ItemID: 0x000F4240, Unk2: 10, Unk3: 11, AoWGaItemHandle: handles.AoW, Unk5: 12},
		{Handle: handles.Armor, ItemID: 0x10000001, Unk2: 20, Unk3: 21},
		{},
		{Handle: handles.NakedHead, ItemID: nakedHeadItemID, Unk2: 30, Unk3: 31},
		{Handle: handles.Unarmed, ItemID: InvUnarmedBaseID, Unk2: 40, Unk3: 41, AoWGaItemHandle: NoCustomAoWHandle, Unk5: 42},
	}
	slot.NextAoWIndex = 1
	slot.NextArmamentIndex = len(slot.GaItems)
	slot.NextGaItemHandle = 0x106
	slot.GaMap = map[uint32]uint32{
		handles.AoW:       0x80000001,
		handles.Weapon:    0x000F4240,
		handles.Armor:     0x10000001,
		handles.NakedHead: nakedHeadItemID,
		handles.Unarmed:   InvUnarmedBaseID,
	}

	slot.Inventory.CommonItems = []InventoryItem{
		{GaItemHandle: handles.Weapon, Quantity: 1, Index: 100},
		{GaItemHandle: handles.NakedHead, Quantity: 1, Index: 101},
		{GaItemHandle: handles.Unarmed, Quantity: 1, Index: 102},
	}
	slot.Storage.CommonItems = []InventoryItem{{GaItemHandle: handles.Armor, Quantity: 1, Index: 200}}
	storageRecord := slot.StorageBoxOffset + StorageHeaderSkip
	binary.LittleEndian.PutUint32(slot.Data[slot.StorageBoxOffset:], 1)
	binary.LittleEndian.PutUint32(slot.Data[storageRecord:], handles.Armor)
	binary.LittleEndian.PutUint32(slot.Data[storageRecord+4:], 1)
	binary.LittleEndian.PutUint32(slot.Data[storageRecord+8:], 200)

	return repackReferenceFixture{Slot: slot, Handles: handles}
}

// fragmentedRepackRoundTripFixture is the fully serializable form of the
// reference fixture. It has the production-size GaItem table and a coherent
// binary layout, so RepackGaItems can rebuild and reparse it in a test without
// using a user save file.
func fragmentedRepackRoundTripFixture(t *testing.T) repackReferenceFixture {
	return fragmentedRepackRoundTripFixtureForVersion(t, GaItemVersionBreak+1)
}

func fragmentedRepackRoundTripFixtureForVersion(t *testing.T, version uint32) repackReferenceFixture {
	t.Helper()

	fixture := fragmentedRepackReferenceFixture(t)
	slot := fixture.Slot
	initial := append([]GaItemFull(nil), slot.GaItems...)
	gaItemCount := GaItemCountNew
	if version <= GaItemVersionBreak {
		gaItemCount = GaItemCountOld
	}
	slot.GaItems = make([]GaItemFull, gaItemCount)
	// The in-memory reference fixture is deliberately fragmented for Optimize
	// tests. Its serializable form must instead obey the game's two-pass
	// projection so it is a valid source for writer/add integration tests.
	for _, record := range initial {
		if record.IsEmpty() {
			continue
		}
		index := int(record.Handle & 0xFFFF)
		position := index
		if record.Handle&GaHandleTypeMask == ItemTypeAow {
			position = 0
		}
		slot.GaItems[position] = record
	}

	gaBytes := 0
	for i := range slot.GaItems {
		gaBytes += slot.GaItems[i].ByteSize()
	}
	slot.MagicOffset = GaItemsStart + gaBytes + DynPlayerData - 1
	slot.Data = make([]byte, SlotSize)
	slot.Version = version
	binary.LittleEndian.PutUint32(slot.Data, slot.Version)
	copy(slot.Data[slot.MagicOffset:], MagicPattern)

	pos := GaItemsStart
	for i := range slot.GaItems {
		pos += slot.GaItems[i].Serialize(slot.Data[pos:])
	}
	if pos != slot.MagicOffset-DynPlayerData+1 {
		t.Fatalf("GaItem fixture end=0x%X, want 0x%X", pos, slot.MagicOffset-DynPlayerData+1)
	}

	if err := slot.calculateDynamicOffsets(); err != nil {
		t.Fatalf("calculateDynamicOffsets: %v", err)
	}
	writeFixtureInventory(slot, slot.Inventory.CommonItems)
	writeFixtureStorage(slot, slot.Storage.CommonItems)
	if err := slot.buildSectionMap(); err != nil {
		t.Fatalf("buildSectionMap: %v", err)
	}

	// Make the non-GaItem regions observably non-zero, including the equipped
	// items and GaItemData areas whose preservation is asserted by Task 7.2.
	for i := 0; i < ChrAsmEquipmentSize; i++ {
		slot.Data[slot.EquipItemsIDOffset+i] = byte(i + 1)
	}
	binary.LittleEndian.PutUint64(slot.Data[slot.GaItemDataOffset:], 1)
	binary.LittleEndian.PutUint32(slot.Data[slot.GaItemDataOffset+8:], 0x000F4240)
	slot.Data[slot.GaItemDataOffset+12] = 0xA4
	binary.LittleEndian.PutUint32(slot.Data[slot.GaItemDataOffset+16:], 0x10000001)
	slot.Data[slot.GaItemDataOffset+20] = 0xA8

	if err := slot.parseFromData(); err != nil {
		t.Fatalf("parseFromData: %v", err)
	}
	return fixture
}

func writeFixtureInventory(slot *SaveSlot, items []InventoryItem) {
	start := slot.MagicOffset + InvStartFromMagic
	nonEmpty := 0
	for _, item := range items {
		if item.GaItemHandle != GaHandleEmpty && item.GaItemHandle != GaHandleInvalid {
			nonEmpty++
		}
	}
	binary.LittleEndian.PutUint32(slot.Data[start-InvKeyCountHeader:], uint32(nonEmpty))
	for i, item := range items {
		off := start + i*InvRecordLen
		binary.LittleEndian.PutUint32(slot.Data[off:], item.GaItemHandle)
		binary.LittleEndian.PutUint32(slot.Data[off+4:], item.Quantity)
		binary.LittleEndian.PutUint32(slot.Data[off+8:], item.Index)
	}
	keyStart := start + CommonItemCount*InvRecordLen + InvKeyCountHeader
	for i, item := range slot.Inventory.KeyItems {
		off := keyStart + i*InvRecordLen
		binary.LittleEndian.PutUint32(slot.Data[off:], item.GaItemHandle)
		binary.LittleEndian.PutUint32(slot.Data[off+4:], item.Quantity)
		binary.LittleEndian.PutUint32(slot.Data[off+8:], item.Index)
	}
}

func writeFixtureStorage(slot *SaveSlot, items []InventoryItem) {
	binary.LittleEndian.PutUint32(slot.Data[slot.StorageBoxOffset:], uint32(len(items)))
	start := slot.StorageBoxOffset + StorageHeaderSkip
	for i, item := range items {
		off := start + i*InvRecordLen
		binary.LittleEndian.PutUint32(slot.Data[off:], item.GaItemHandle)
		binary.LittleEndian.PutUint32(slot.Data[off+4:], item.Quantity)
		binary.LittleEndian.PutUint32(slot.Data[off+8:], item.Index)
	}
}

func TestFragmentedRepackReferenceFixture_IsHealthyAndRepresentative(t *testing.T) {
	fixture := fragmentedRepackReferenceFixture(t)
	preflight := PreflightGaItemRepack(fixture.Slot)
	if len(preflight.Blockers) != 0 {
		t.Fatalf("fixture blockers=%+v, want none", preflight.Blockers)
	}
	if preflight.Analysis.Recovered <= 0 {
		t.Fatalf("fixture recovery=%d, want positive", preflight.Analysis.Recovered)
	}

	availability := ScanAoWAvailability(fixture.Slot)
	if len(availability) != 1 || availability[0].Handle != fixture.Handles.AoW || availability[0].UsedByWeaponHandle != fixture.Handles.Weapon {
		t.Fatalf("AoW availability=%+v, want one copy used by weapon 0x%08X", availability, fixture.Handles.Weapon)
	}

	naked := ResolveRecord(fixture.Slot, repairScopeInventoryCommon, 1, fixture.Handles.NakedHead, 1, 101)
	unarmed := ResolveRecord(fixture.Slot, repairScopeInventoryCommon, 2, fixture.Handles.Unarmed, 1, 102)
	if naked.Resolution != ResolutionTechnicalPlaceholder || unarmed.Resolution != ResolutionTechnicalPlaceholder {
		t.Fatalf("placeholder resolutions = %q / %q, want both %q", naked.Resolution, unarmed.Resolution, ResolutionTechnicalPlaceholder)
	}
}
