package core

import (
	"os"
	"reflect"
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
