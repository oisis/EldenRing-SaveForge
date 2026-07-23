package core

import (
	"reflect"
	"testing"
)

func assertSlotRestored(t *testing.T, slot *SaveSlot, before SlotSnapshot) {
	t.Helper()
	if after := SnapshotSlot(slot); !reflect.DeepEqual(after, before) {
		t.Fatal("slot mutated despite native projection refusal")
	}
}

func TestRepackGaItems_ReparseIsCanonicalAndIdempotent(t *testing.T) {
	fixture := fragmentedRepackRoundTripFixture(t)
	slot := fixture.Slot

	result, err := RepackGaItems(slot)
	if err != nil {
		t.Fatalf("RepackGaItems: %v", err)
	}
	if !result.Changed || result.Recovered <= 0 {
		t.Fatalf("result=%+v, want compacting repack", result)
	}

	records := nonEmptyGaItemRecords(slot.GaItems)
	assertCompactedGaItemPrefix(t, slot, len(records))
	if slot.NextArmamentIndex != len(records) || slot.NextAoWIndex != 1 {
		t.Fatalf("cursors AoW=%d Armament=%d, want 1/%d", slot.NextAoWIndex, slot.NextArmamentIndex, len(records))
	}
	if got, want := AnalyzeGaItemRepack(slot).Before, (GaItemCapacity{PhysicalEmpty: len(slot.GaItems) - len(records), CursorRoom: len(slot.GaItems) - len(records), Usable: len(slot.GaItems) - len(records)}); got != want {
		t.Fatalf("repacked capacity=%+v, want %+v", got, want)
	}

	reloaded := reparseGaItemFixture(t, slot)
	if reloaded.NextAoWIndex != slot.NextAoWIndex || reloaded.NextArmamentIndex != slot.NextArmamentIndex || reloaded.NextGaItemHandle != slot.NextGaItemHandle || reloaded.PartGaItemHandle != slot.PartGaItemHandle {
		t.Fatalf("fresh reparse allocator state differs: got AoW=%d Armament=%d Next=%d Part=0x%02X; want AoW=%d Armament=%d Next=%d Part=0x%02X", reloaded.NextAoWIndex, reloaded.NextArmamentIndex, reloaded.NextGaItemHandle, reloaded.PartGaItemHandle, slot.NextAoWIndex, slot.NextArmamentIndex, slot.NextGaItemHandle, slot.PartGaItemHandle)
	}
	if second, err := RepackGaItems(reloaded); err != nil {
		t.Fatalf("second RepackGaItems: %v", err)
	} else if second.Changed || second.Recovered != 0 {
		t.Fatalf("second repack=%+v, want no-op", second)
	}
}

func TestRepackGaItems_FollowUpArmamentPreservesCanonicalLayout(t *testing.T) {
	fixture := fragmentedRepackRoundTripFixture(t)
	slot := fixture.Slot
	if _, err := RepackGaItems(slot); err != nil {
		t.Fatalf("RepackGaItems: %v", err)
	}
	before := SnapshotSlot(slot)
	const weaponID = 0x000F4250
	err := AddItemsToSlot(slot, []uint32{weaponID}, 1, 0, false)
	if err == nil || !contains(err.Error(), "native GaItem projection") {
		t.Fatalf("AddItemsToSlot error = %v, want native projection refusal", err)
	}
	assertSlotRestored(t, slot, before)
}

func TestRepackGaItems_FollowUpAoWPreservesCanonicalLayout(t *testing.T) {
	fixture := fragmentedRepackRoundTripFixture(t)
	slot := fixture.Slot
	if _, err := RepackGaItems(slot); err != nil {
		t.Fatalf("RepackGaItems: %v", err)
	}
	before := SnapshotSlot(slot)
	const aowID = 0x80000002
	err := AddItemsToSlot(slot, []uint32{aowID}, 1, 0, false)
	if err == nil || !contains(err.Error(), "native GaItem projection") {
		t.Fatalf("AddItemsToSlot error = %v, want native projection refusal", err)
	}
	assertSlotRestored(t, slot, before)
}

func fragmentedHighestAoWFixture(t *testing.T) repackReferenceFixture {
	t.Helper()
	fixture := fragmentedRepackRoundTripFixture(t)
	replaceFixtureAoWHandle(t, &fixture, ItemTypeAow|0x106)
	return fixture
}

func fragmentedAoWLastFixture(t *testing.T) repackReferenceFixture {
	t.Helper()
	fixture := fragmentedHighestAoWFixture(t)
	slot := fixture.Slot
	records := make(map[uint32]GaItemFull)
	for _, record := range slot.GaItems {
		if !record.IsEmpty() {
			records[record.Handle] = record
		}
	}
	slot.GaItems = make([]GaItemFull, GaItemCountNew)
	slot.GaItems[0] = records[fixture.Handles.Weapon]
	slot.GaItems[2] = records[fixture.Handles.Armor]
	slot.GaItems[3] = records[fixture.Handles.NakedHead]
	slot.GaItems[5] = records[fixture.Handles.Unarmed]
	slot.GaItems[6] = records[fixture.Handles.AoW]
	slot.NextAoWIndex = 1
	slot.NextArmamentIndex = len(slot.GaItems)
	return fixture
}

func replaceFixtureAoWHandle(t *testing.T, fixture *repackReferenceFixture, handle uint32) {
	t.Helper()
	slot := fixture.Slot
	old := fixture.Handles.AoW
	handle |= uint32(slot.PartGaItemHandle) << 16
	itemID, ok := slot.GaMap[old]
	if !ok {
		t.Fatalf("fixture AoW handle 0x%08X missing from GaMap", old)
	}
	for i := range slot.GaItems {
		if slot.GaItems[i].Handle == old {
			slot.GaItems[i].Handle = handle
		}
		if slot.GaItems[i].AoWGaItemHandle == old {
			slot.GaItems[i].AoWGaItemHandle = handle
		}
	}
	delete(slot.GaMap, old)
	slot.GaMap[handle] = itemID
	slot.NextGaItemHandle = (handle & 0xFFFF) + 1
	fixture.Handles.AoW = handle
}

func reparseGaItemFixture(t *testing.T, slot *SaveSlot) *SaveSlot {
	t.Helper()
	reloaded := &SaveSlot{Data: append([]byte(nil), slot.Data...)}
	if err := reloaded.parseFromData(); err != nil {
		t.Fatalf("fresh parseFromData: %v", err)
	}
	return reloaded
}

func assertCompactedGaItemPrefix(t *testing.T, slot *SaveSlot, nonEmpty int) {
	t.Helper()
	for i := 0; i < nonEmpty; i++ {
		if slot.GaItems[i].IsEmpty() {
			t.Fatalf("GaItem[%d] is empty in compacted prefix", i)
		}
	}
	for i := nonEmpty; i < len(slot.GaItems); i++ {
		if !slot.GaItems[i].IsEmpty() {
			t.Fatalf("GaItem[%d] is non-empty in compacted suffix", i)
		}
	}
}

func assertExistingGaItemsRemain(t *testing.T, before, after []GaItemFull) {
	t.Helper()
	remaining := append([]GaItemFull(nil), nonEmptyGaItemRecords(after)...)
	for _, record := range before {
		index := -1
		for i, candidate := range remaining {
			if candidate == record {
				index = i
				break
			}
		}
		if index < 0 {
			t.Fatalf("existing GaItem record lost: %#v", record)
		}
		remaining = append(remaining[:index], remaining[index+1:]...)
	}
}

func findGaItemIndex(records []GaItemFull, itemID uint32) int {
	for i, record := range records {
		if record.ItemID == itemID {
			return i
		}
	}
	return -1
}
