package core

import (
	"reflect"
	"testing"
)

func TestBuildGaItemRepackPlan_StableCompactionPreservesFullRecords(t *testing.T) {
	weapon := GaItemFull{
		Handle:          ItemTypeWeapon | 7,
		ItemID:          1,
		Unk2:            -1,
		Unk3:            -2,
		AoWGaItemHandle: ItemTypeAow | 9,
		Unk5:            3,
	}
	armor := GaItemFull{Handle: ItemTypeArmor | 2, ItemID: 0x10000001, Unk2: 4, Unk3: 5}
	aow := GaItemFull{Handle: ItemTypeAow | 7, ItemID: 0x80000001}
	slot := &SaveSlot{GaItems: []GaItemFull{{}, weapon, {}, armor, aow, {}}}
	before := append([]GaItemFull(nil), slot.GaItems...)

	plan := BuildGaItemRepackPlan(slot)

	if plan.NonEmptyRecords != 3 || !plan.Changes {
		t.Errorf("plan=%+v, want 3 non-empty records and changes", plan)
	}
	want := []GaItemFull{weapon, armor, aow, {}, {}, {}}
	if !reflect.DeepEqual(plan.GaItems, want) {
		t.Errorf("planned GaItems=%+v, want %+v", plan.GaItems, want)
	}
	if !reflect.DeepEqual(slot.GaItems, before) {
		t.Fatal("BuildGaItemRepackPlan mutated slot.GaItems")
	}
}

func TestApplyGaItemRepackPlan_ChangesOnlyGaItems(t *testing.T) {
	slot := &SaveSlot{
		Data:              []byte{1, 2, 3},
		GaItems:           []GaItemFull{{}, {Handle: ItemTypeWeapon | 1, ItemID: 1}, {}},
		GaMap:             map[uint32]uint32{ItemTypeWeapon | 1: 1},
		NextAoWIndex:      4,
		NextArmamentIndex: 5,
		NextGaItemHandle:  6,
		PartGaItemHandle:  0x81,
	}
	dataBefore := append([]byte(nil), slot.Data...)
	gaMapBefore := make(map[uint32]uint32, len(slot.GaMap))
	for handle, itemID := range slot.GaMap {
		gaMapBefore[handle] = itemID
	}
	plan := BuildGaItemRepackPlan(slot)

	if err := ApplyGaItemRepackPlan(slot, plan); err != nil {
		t.Fatalf("ApplyGaItemRepackPlan: %v", err)
	}
	if got, want := slot.GaItems, plan.GaItems; !reflect.DeepEqual(got, want) {
		t.Errorf("GaItems=%+v, want %+v", got, want)
	}
	if !reflect.DeepEqual(slot.Data, dataBefore) || !reflect.DeepEqual(slot.GaMap, gaMapBefore) {
		t.Fatal("ApplyGaItemRepackPlan changed data or GaMap")
	}
	if slot.NextAoWIndex != 4 || slot.NextArmamentIndex != 5 || slot.NextGaItemHandle != 6 || slot.PartGaItemHandle != 0x81 {
		t.Errorf("allocator state changed: %+v", slot)
	}
}

func TestApplyGaItemRepackPlan_NoOpKeepsOriginalSlice(t *testing.T) {
	slot := &SaveSlot{GaItems: []GaItemFull{{Handle: ItemTypeWeapon | 1, ItemID: 1}, {}, {}}}
	beforeSlice := &slot.GaItems[0]
	plan := BuildGaItemRepackPlan(slot)

	if plan.Changes {
		t.Fatal("plan.Changes=true for an already compact layout")
	}
	if err := ApplyGaItemRepackPlan(slot, plan); err != nil {
		t.Fatalf("ApplyGaItemRepackPlan: %v", err)
	}
	if &slot.GaItems[0] != beforeSlice {
		t.Fatal("no-op plan replaced the GaItems slice")
	}
}

func TestApplyGaItemRepackPlan_RejectsWrongLength(t *testing.T) {
	slot := &SaveSlot{GaItems: make([]GaItemFull, 2)}
	err := ApplyGaItemRepackPlan(slot, GaItemRepackPlan{GaItems: make([]GaItemFull, 1)})
	if err == nil {
		t.Fatal("ApplyGaItemRepackPlan accepted a plan with a different length")
	}
}
