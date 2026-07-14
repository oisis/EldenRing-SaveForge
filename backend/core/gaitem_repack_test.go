package core

import (
	"reflect"
	"testing"
)

func TestAnalyzeGaItemRepack_ForecastsRecoveredCapacityWithoutMutation(t *testing.T) {
	slot := &SaveSlot{
		GaItems: []GaItemFull{
			{Handle: ItemTypeWeapon | 1, ItemID: 1},
			{},
			{},
			{Handle: ItemTypeArmor | 2, ItemID: 0x10000001},
			{},
			{},
		},
		NextArmamentIndex: 6,
	}
	before := *slot
	before.GaItems = append([]GaItemFull(nil), slot.GaItems...)

	analysis := AnalyzeGaItemRepack(slot)

	if analysis.NonEmptyRecords != 2 {
		t.Errorf("NonEmptyRecords=%d, want 2", analysis.NonEmptyRecords)
	}
	if analysis.Before != (GaItemCapacity{PhysicalEmpty: 4, CursorRoom: 0, Usable: 0}) {
		t.Errorf("Before=%+v, want {PhysicalEmpty:4 CursorRoom:0 Usable:0}", analysis.Before)
	}
	if analysis.ProjectedAfter != (GaItemCapacity{PhysicalEmpty: 4, CursorRoom: 4, Usable: 4}) {
		t.Errorf("ProjectedAfter=%+v, want {PhysicalEmpty:4 CursorRoom:4 Usable:4}", analysis.ProjectedAfter)
	}
	if analysis.Recovered != 4 {
		t.Errorf("Recovered=%d, want 4", analysis.Recovered)
	}
	if !reflect.DeepEqual(*slot, before) {
		t.Fatal("AnalyzeGaItemRepack mutated the slot")
	}
}

func TestAnalyzeGaItemRepack_AlreadyFullUsableCapacityIsNoOp(t *testing.T) {
	slot := &SaveSlot{
		GaItems: []GaItemFull{
			{Handle: ItemTypeWeapon | 7, ItemID: 1},
			{Handle: ItemTypeArmor | 1, ItemID: 0x10000001},
			{}, {}, {}, {},
		},
		NextArmamentIndex: 1,
	}

	analysis := AnalyzeGaItemRepack(slot)

	if analysis.Before != (GaItemCapacity{PhysicalEmpty: 4, CursorRoom: 5, Usable: 4}) {
		t.Errorf("Before=%+v, want {PhysicalEmpty:4 CursorRoom:5 Usable:4}", analysis.Before)
	}
	if analysis.ProjectedAfter != analysis.Before {
		t.Errorf("ProjectedAfter=%+v, want unchanged %+v", analysis.ProjectedAfter, analysis.Before)
	}
	if analysis.Recovered != 0 {
		t.Errorf("Recovered=%d, want 0", analysis.Recovered)
	}
}

func TestAnalyzeGaItemRepack_UsesLastMaxCounterTieAfterCompaction(t *testing.T) {
	slot := &SaveSlot{
		GaItems: []GaItemFull{
			{},
			{Handle: ItemTypeWeapon | 7, ItemID: 1},
			{},
			{Handle: ItemTypeArmor | 2, ItemID: 0x10000001},
			{Handle: ItemTypeAow | 7, ItemID: 0x80000001},
			{},
		},
		NextArmamentIndex: 5,
	}

	analysis := AnalyzeGaItemRepack(slot)

	if analysis.ProjectedAfter != (GaItemCapacity{PhysicalEmpty: 3, CursorRoom: 3, Usable: 3}) {
		t.Errorf("ProjectedAfter=%+v, want {PhysicalEmpty:3 CursorRoom:3 Usable:3}", analysis.ProjectedAfter)
	}
	if analysis.Recovered != 2 {
		t.Errorf("Recovered=%d, want 2", analysis.Recovered)
	}
}

func TestAnalyzeGaItemRepack_EmptyTableRemainsNoOp(t *testing.T) {
	slot := &SaveSlot{
		GaItems:           make([]GaItemFull, 4),
		NextArmamentIndex: 1,
	}

	analysis := AnalyzeGaItemRepack(slot)

	if analysis.NonEmptyRecords != 0 {
		t.Errorf("NonEmptyRecords=%d, want 0", analysis.NonEmptyRecords)
	}
	if analysis.Before != (GaItemCapacity{PhysicalEmpty: 4, CursorRoom: 3, Usable: 3}) {
		t.Errorf("Before=%+v, want {PhysicalEmpty:4 CursorRoom:3 Usable:3}", analysis.Before)
	}
	if analysis.ProjectedAfter != analysis.Before || analysis.Recovered != 0 {
		t.Errorf("ProjectedAfter=%+v Recovered=%d, want unchanged / 0", analysis.ProjectedAfter, analysis.Recovered)
	}
}
