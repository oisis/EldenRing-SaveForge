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
		AoW:       ItemTypeAow | 0x101,
		Weapon:    ItemTypeWeapon | 0x102,
		Armor:     ItemTypeArmor | 0x103,
		NakedHead: ItemTypeArmor | 0x104,
		Unarmed:   ItemTypeWeapon | 0x105,
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
