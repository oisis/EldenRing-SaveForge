package main

import (
	"reflect"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
)

// nativeHoleSlotFixture returns a small native-layout slot with a free physical
// index before an existing weapon. It verifies that additions use holes without
// requiring compaction.
func nativeHoleSlotFixture(t *testing.T) *core.SaveSlot {
	t.Helper()
	slot := &core.SaveSlot{
		Data:              make([]byte, core.SlotSize),
		Version:           1,
		GaMap:             make(map[uint32]uint32),
		MagicOffset:       1000,
		InventoryEnd:      core.GaItemsStart,
		PlayerDataOffset:  1000,
		FaceDataOffset:    2000,
		StorageBoxOffset:  2000,
		GaItemDataOffset:  0x8000,
		SectionMap:        []core.SectionRange{{Name: "all", Start: 0, End: core.SlotSize}},
		NextAoWIndex:      0,
		NextArmamentIndex: 4,
		NextGaItemHandle:  2,
		PartGaItemHandle:  0x80,
	}
	weapon := core.GaItemFull{Handle: core.ItemTypeWeapon | 0x00800001, ItemID: 1}
	slot.GaItems = []core.GaItemFull{{}, weapon, {}, {}}
	slot.GaMap[weapon.Handle] = weapon.ItemID
	if free, err := core.NativeGaItemCapacity(slot); err != nil || free != 3 {
		t.Fatalf("native capacity=%d err=%v, want three usable holes", free, err)
	}
	return slot
}

const endpointGaItemWeaponID = uint32(0x02810590) // Golem Greatbow

func TestAddItems_CursorExhaustedUsesPhysicalHole(t *testing.T) {
	app := NewApp()
	app.save = &core.SaveFile{}
	app.save.Slots[0] = *nativeHoleSlotFixture(t)
	before := core.CloneSlot(&app.save.Slots[0])

	res, err := app.AddItemsToCharacter(0, []uint32{endpointGaItemWeaponID}, 0, 0, 0, 0, 1, 0)
	if err != nil {
		t.Fatalf("AddItemsToCharacter: %v", err)
	}
	if res.CapHit != "" || res.Added != 1 {
		t.Fatalf("result=%+v, want one item added through a physical hole", res)
	}
	if reflect.DeepEqual(&app.save.Slots[0], before) {
		t.Fatal("successful hole allocation did not mutate the slot")
	}
}
