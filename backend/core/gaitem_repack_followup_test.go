package core

import "testing"

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
	tests := []struct {
		name          string
		fixture       func(*testing.T) repackReferenceFixture
		wantCursorPos func(int) int
	}{
		{
			name:    "cursor at compacted prefix end",
			fixture: fragmentedRepackRoundTripFixture,
			wantCursorPos: func(records int) int {
				return records
			},
		},
		{
			name:    "cursor inside compacted prefix",
			fixture: fragmentedHighestAoWFixture,
			wantCursorPos: func(int) int {
				return 1
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fixture := tc.fixture(t)
			slot := fixture.Slot
			if _, err := RepackGaItems(slot); err != nil {
				t.Fatalf("RepackGaItems: %v", err)
			}
			beforeRecords := nonEmptyGaItemRecords(slot.GaItems)
			insertAt := tc.wantCursorPos(len(beforeRecords))
			if slot.NextArmamentIndex != insertAt {
				t.Fatalf("NextArmamentIndex=%d, want %d before add", slot.NextArmamentIndex, insertAt)
			}

			const weaponID = 0x000F4250
			if err := AddItemsToSlot(slot, []uint32{weaponID}, 1, 0, false); err != nil {
				t.Fatalf("AddItemsToSlot: %v", err)
			}
			assertCompactedGaItemPrefix(t, slot, len(beforeRecords)+1)
			if got := findGaItemIndex(slot.GaItems, weaponID); got != insertAt {
				t.Fatalf("new weapon index=%d, want %d", got, insertAt)
			}
			if slot.NextArmamentIndex != insertAt+1 {
				t.Fatalf("NextArmamentIndex=%d, want %d after add", slot.NextArmamentIndex, insertAt+1)
			}
			assertExistingGaItemsRemain(t, beforeRecords, slot.GaItems)

			reloaded := reparseGaItemFixture(t, slot)
			if reloaded.NextArmamentIndex != slot.NextArmamentIndex {
				t.Fatalf("fresh reparse NextArmamentIndex=%d, want in-memory %d", reloaded.NextArmamentIndex, slot.NextArmamentIndex)
			}
		})
	}
}

func TestRepackGaItems_FollowUpAoWPreservesCanonicalLayout(t *testing.T) {
	tests := []struct {
		name                  string
		fixture               func(*testing.T) repackReferenceFixture
		wantInsertAt          func(int) int
		wantReloadArmamentPos func(int) int
	}{
		{
			name:    "cursor before compacted prefix end",
			fixture: fragmentedRepackRoundTripFixture,
			wantInsertAt: func(int) int {
				return 1
			},
			wantReloadArmamentPos: func(int) int {
				return 2
			},
		},
		{
			name:    "cursor at compacted prefix end",
			fixture: fragmentedAoWLastFixture,
			wantInsertAt: func(records int) int {
				return records
			},
			wantReloadArmamentPos: func(records int) int {
				return records + 1
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fixture := tc.fixture(t)
			slot := fixture.Slot
			if _, err := RepackGaItems(slot); err != nil {
				t.Fatalf("RepackGaItems: %v", err)
			}
			beforeRecords := nonEmptyGaItemRecords(slot.GaItems)
			insertAt := tc.wantInsertAt(len(beforeRecords))
			beforeArmament := slot.NextArmamentIndex

			const aowID = 0x80000002
			if err := AddItemsToSlot(slot, []uint32{aowID}, 1, 0, false); err != nil {
				t.Fatalf("AddItemsToSlot: %v", err)
			}
			assertCompactedGaItemPrefix(t, slot, len(beforeRecords)+1)
			if got := findGaItemIndex(slot.GaItems, aowID); got != insertAt {
				t.Fatalf("new AoW index=%d, want %d", got, insertAt)
			}
			if slot.NextAoWIndex != insertAt+1 || slot.NextArmamentIndex != beforeArmament+1 {
				t.Fatalf("in-memory cursors AoW=%d Armament=%d, want %d/%d", slot.NextAoWIndex, slot.NextArmamentIndex, insertAt+1, beforeArmament+1)
			}
			assertExistingGaItemsRemain(t, beforeRecords, slot.GaItems)

			reloaded := reparseGaItemFixture(t, slot)
			wantReloadArmament := tc.wantReloadArmamentPos(len(beforeRecords))
			if reloaded.NextArmamentIndex != wantReloadArmament {
				t.Fatalf("fresh reparse NextArmamentIndex=%d, want %d", reloaded.NextArmamentIndex, wantReloadArmament)
			}
			if insertAt < beforeArmament && reloaded.NextArmamentIndex == slot.NextArmamentIndex {
				t.Fatal("expected documented in-memory/reload armament cursor divergence")
			}
		})
	}
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
