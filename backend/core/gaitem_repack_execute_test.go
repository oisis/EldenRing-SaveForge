package core

import (
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestSnapshotSlot_RestoresCompleteMutableState(t *testing.T) {
	slot := repackPreflightFixture()
	slot.SteamID = 42
	slot.Warnings = []string{"existing warning"}
	slot.ClearCountOffset = 100
	slot.EquipItemsIDOffset = 200
	slot.EquippedSpellsOffset = 300
	slot.UnlockedRegionsOffset = 400
	slot.UnlockedRegions = []uint32{1, 2}
	slot.SectionMap = []SectionRange{{Name: "first", Start: 0, End: 100}, {Name: "second", Start: 100, End: SlotSize}}
	slot.Inventory.CommonItems = []InventoryItem{{GaItemHandle: ItemTypeWeapon | 1, Quantity: 2, Index: 3}}
	slot.Storage.KeyItems = []InventoryItem{{GaItemHandle: ItemTypeItem | 4, Quantity: 5, Index: 6}}

	expected := CloneSlot(slot)
	snapshot := SnapshotSlot(slot)

	slot.Data[0] = 0xFF
	slot.Version++
	slot.Player.Level = 99
	slot.GaMap[ItemTypeArmor|2] = 2
	slot.GaItems[0] = GaItemFull{}
	slot.Inventory.CommonItems[0].Quantity = 99
	slot.Storage.KeyItems[0].Quantity = 99
	slot.SteamID++
	slot.Warnings = nil
	slot.MagicOffset++
	slot.InventoryEnd++
	slot.EventFlagsOffset++
	slot.PlayerDataOffset++
	slot.FaceDataOffset++
	slot.StorageBoxOffset++
	slot.IngameTimerOffset++
	slot.GaItemDataOffset++
	slot.TutorialDataOffset++
	slot.ClearCountOffset++
	slot.EquipItemsIDOffset++
	slot.EquippedSpellsOffset++
	slot.UnlockedRegionsOffset++
	slot.UnlockedRegions[0] = 99
	slot.SectionMap[0].End = 99
	slot.NextAoWIndex++
	slot.NextArmamentIndex++
	slot.NextGaItemHandle++
	slot.PartGaItemHandle++

	RestoreSlot(slot, snapshot)
	if !reflect.DeepEqual(slot, expected) {
		t.Fatalf("RestoreSlot did not restore complete state\n got: %#v\nwant: %#v", slot, expected)
	}
}

func TestValidateGaItemRepackPostMutation_IgnoresRawDuplicateInventoryIndex(t *testing.T) {
	const secondWeapon = ItemTypeWeapon | 2

	slot := repackPreflightFixture()
	slot.GaItems[1] = GaItemFull{Handle: secondWeapon, ItemID: 2}
	slot.NextArmamentIndex = 2
	slot.GaMap[secondWeapon] = 2
	slot.Inventory.CommonItems = []InventoryItem{
		{GaItemHandle: slot.GaItems[0].Handle, Quantity: 1, Index: 1088},
		{GaItemHandle: secondWeapon, Quantity: 1, Index: 1088},
	}

	if violations := ValidatePostMutation(slot); len(violations) != 1 || violations[0].Check != "duplicate_index" {
		t.Fatalf("ValidatePostMutation=%+v, want one duplicate_index", violations)
	}
	if violations := validateGaItemRepackPostMutation(slot); len(violations) != 0 {
		t.Fatalf("repack post-mutation violations=%+v, want none", violations)
	}
}

func TestRepackGaItems_RefusalDoesNotMutateSlot(t *testing.T) {
	slot := repackPreflightFixture()
	slot.GaItems[0].Handle = 0x70000001
	slot.GaMap = map[uint32]uint32{slot.GaItems[0].Handle: slot.GaItems[0].ItemID}
	before := CloneSlot(slot)

	_, err := RepackGaItems(slot)

	if err == nil {
		t.Fatal("RepackGaItems accepted an unsafe slot")
	}
	if !reflect.DeepEqual(slot, before) {
		t.Fatal("RepackGaItems changed a refused slot")
	}
}

func TestRepackGaItems_RefusalGatesDoNotMutateSlot(t *testing.T) {
	tests := []struct {
		name     string
		prepare  func(*SaveSlot)
		wantCode string
	}{
		{
			name: "invalid slot data size",
			prepare: func(slot *SaveSlot) {
				slot.Data = slot.Data[:16]
			},
			wantCode: "slot_data_size",
		},
		{
			name: "invalid section map",
			prepare: func(slot *SaveSlot) {
				slot.SectionMap = nil
			},
			wantCode: "section_map",
		},
		{
			name: "duplicate GaItem handle",
			prepare: func(slot *SaveSlot) {
				slot.GaItems = append(slot.GaItems, slot.GaItems[0])
				slot.NextArmamentIndex = len(slot.GaItems)
			},
			wantCode: "duplicate_handle",
		},
		{
			name: "dangling inventory handle",
			prepare: func(slot *SaveSlot) {
				slot.Inventory.CommonItems = []InventoryItem{{GaItemHandle: ItemTypeWeapon | 99, Quantity: 1, Index: 1}}
			},
			wantCode: "orphan_inventory_handle",
		},
		{
			name: "dangling Ash of War link",
			prepare: func(slot *SaveSlot) {
				slot.GaItems[0].AoWGaItemHandle = ItemTypeAow | 99
			},
			wantCode: "dangling_aow_handle",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			slot := repackPreflightFixture()
			tc.prepare(slot)
			before := CloneSlot(slot)

			_, err := RepackGaItems(slot)

			if err == nil || !strings.Contains(err.Error(), tc.wantCode) {
				t.Fatalf("RepackGaItems error=%v, want refusal %q", err, tc.wantCode)
			}
			if !reflect.DeepEqual(slot, before) {
				t.Fatal("RepackGaItems changed a refused slot")
			}
		})
	}
}

func TestRepackGaItems_RebuildFailureRollsBack(t *testing.T) {
	slot := repackPreflightFixture()
	weapon := slot.GaItems[0]
	slot.GaItems = []GaItemFull{{}, {}, {}, weapon}
	slot.NextArmamentIndex = len(slot.GaItems)
	before := CloneSlot(slot)

	_, err := RepackGaItems(slot)

	if err == nil {
		t.Fatal("RepackGaItems unexpectedly rebuilt a fixture without unlocked_regions")
	}
	if !reflect.DeepEqual(slot, before) {
		t.Fatal("RepackGaItems left partial state after rebuild failure")
	}
}

func TestRepackGaItems_PostconditionFailureRollsBackCompleteSlot(t *testing.T) {
	fixture := fragmentedRepackRoundTripFixture(t)
	slot := fixture.Slot
	// scanGaItems derives 0x80 from this fixture's handle bits. A different
	// snapshot value passes preflight but must fail the post-reparse contract.
	slot.PartGaItemHandle = 0x81
	slot.SteamID = 42
	slot.Warnings = []string{"pre-existing warning"}
	slot.Player.Level = 99
	before := CloneSlot(slot)

	_, err := RepackGaItems(slot)

	if err == nil || !strings.Contains(err.Error(), "postcondition") {
		t.Fatalf("RepackGaItems error=%v, want postcondition failure", err)
	}
	if !reflect.DeepEqual(slot, before) {
		t.Fatal("RepackGaItems did not restore the complete slot after postcondition failure")
	}
}

func TestRepackGaItems_StableCompactionPC(t *testing.T) {
	const savePath = "../../tmp/save/ER0000.sl2"
	if _, err := os.Stat(savePath); os.IsNotExist(err) {
		t.Skipf("test save not found: %s", savePath)
	}

	save, err := LoadSave(savePath)
	if err != nil {
		t.Fatalf("LoadSave(%q): %v", savePath, err)
	}
	candidate := fragmentedRepackCandidate(save.Slots[:])
	if candidate == nil {
		t.Skip("no healthy active slot with a suitable GaItem gap")
	}
	before := AnalyzeGaItemRepack(candidate)

	result, err := RepackGaItems(candidate)
	if err != nil {
		t.Fatalf("RepackGaItems: %v", err)
	}
	if !result.Changed {
		t.Fatal("RepackGaItems reported no change despite recoverable capacity")
	}
	if result.Before != before.Before || result.Recovered != before.Recovered {
		t.Errorf("result=%+v, want before=%+v recovered=%d", result, before.Before, before.Recovered)
	}
	if result.After.Usable != result.Before.Usable+result.Recovered {
		t.Errorf("usable capacity %d -> %d does not match recovered %d", result.Before.Usable, result.After.Usable, result.Recovered)
	}
	post := PreflightGaItemRepack(candidate)
	if len(post.Blockers) != 0 || post.Analysis.Recovered != 0 {
		t.Fatalf("post-repack preflight=%+v, want no blockers and no recoverable capacity", post)
	}
}

func fragmentedRepackCandidate(slots []SaveSlot) *SaveSlot {
	for i := range slots {
		original := &slots[i]
		if original.Version == 0 || len(PreflightGaItemRepack(original).Blockers) != 0 {
			continue
		}
		candidate := CloneSlot(original)
		firstNonEmpty := -1
		for index, record := range candidate.GaItems {
			if !record.IsEmpty() {
				firstNonEmpty = index
				break
			}
		}
		if firstNonEmpty < 0 {
			continue
		}

		emptyIndex, recordIndex := -1, -1
		for index := firstNonEmpty + 1; index < len(candidate.GaItems); index++ {
			if emptyIndex < 0 && candidate.GaItems[index].IsEmpty() {
				emptyIndex = index
				continue
			}
			if emptyIndex >= 0 && !candidate.GaItems[index].IsEmpty() {
				recordIndex = index
				break
			}
		}
		if emptyIndex < 0 || recordIndex < 0 {
			continue
		}

		candidate.GaItems[emptyIndex] = candidate.GaItems[recordIndex]
		candidate.GaItems[recordIndex] = GaItemFull{}
		candidate.NextArmamentIndex = len(candidate.GaItems)
		if preflight := PreflightGaItemRepack(candidate); len(preflight.Blockers) == 0 && preflight.Analysis.Recovered > 0 {
			return candidate
		}
	}
	return nil
}
