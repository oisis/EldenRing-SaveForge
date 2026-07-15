package core

import (
	"bytes"
	"encoding/binary"
	"reflect"
	"testing"
)

func TestRepackGaItems_PreservesContainersAndGaItemDataAcrossVersions(t *testing.T) {
	tests := []struct {
		name    string
		version uint32
		count   int
	}{
		{name: "5118 records", version: GaItemVersionBreak, count: GaItemCountOld},
		{name: "5120 records", version: GaItemVersionBreak + 1, count: GaItemCountNew},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fixture := fragmentedRepackRoundTripFixtureForVersion(t, tc.version)
			slot := fixture.Slot
			before := CloneSlot(slot)
			beforeGaItemData := append([]byte(nil), before.Data[before.GaItemDataOffset:before.GaItemDataOffset+GaitemGameDataSize]...)

			result, err := RepackGaItems(slot)

			if err != nil {
				t.Fatalf("RepackGaItems: %v", err)
			}
			if !result.Changed || result.Recovered <= 0 {
				t.Fatalf("result=%+v, want compacting repack", result)
			}
			if len(slot.GaItems) != tc.count {
				t.Fatalf("GaItems length=%d, want %d for version %d", len(slot.GaItems), tc.count, tc.version)
			}
			if slot.Version != tc.version {
				t.Fatalf("Version=%d, want %d", slot.Version, tc.version)
			}

			nonEmpty := len(nonEmptyGaItemRecords(slot.GaItems))
			wantCapacity := tc.count - nonEmpty
			if got := AnalyzeGaItemRepack(slot).Before; got != (GaItemCapacity{PhysicalEmpty: wantCapacity, CursorRoom: wantCapacity, Usable: wantCapacity}) {
				t.Fatalf("capacity=%+v, want full usable capacity %d", got, wantCapacity)
			}
			if !reflect.DeepEqual(slot.Inventory, before.Inventory) {
				t.Fatal("Inventory changed")
			}
			if !reflect.DeepEqual(slot.Storage, before.Storage) {
				t.Fatal("Storage changed")
			}
			if got := slot.Data[slot.GaItemDataOffset : slot.GaItemDataOffset+GaitemGameDataSize]; !bytes.Equal(got, beforeGaItemData) {
				t.Fatal("GaItemData bytes changed")
			}
			if count := binary.LittleEndian.Uint64(slot.Data[slot.GaItemDataOffset:]); count != 1 {
				t.Fatalf("GaItemData count=%d, want 1", count)
			}
		})
	}
}
