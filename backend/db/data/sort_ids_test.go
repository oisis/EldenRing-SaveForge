package data

import "testing"

// Known values verified directly against regulation.bin CSV dumps.
//
//	EquipParamWeapon.csv  rowID 1000000 = Dagger         → saveID 0x000F4240, sortId 1000000, sortGroupId 10
//	EquipParamWeapon.csv  rowID 3180000 = Claymore        → saveID 0x003085E0, sortId 1201000, sortGroupId 30
//	EquipParamAccessory.csv rowID 1000 = Crimson Amber Medallion → saveID 0x200003E8, sortId 400000, sortGroupId 10
//	EquipParamProtector.csv rowID 150000 = Iron Kasa       → saveID 0x100249F0, sortId 506110, sortGroupId 70
var sortKeyKnown = []struct {
	id          uint32
	name        string
	wantSortId  uint32
	wantGroupId uint8
}{
	{0x000F4240, "Dagger", 1000000, 10},
	{0x003085E0, "Claymore", 1201000, 30},
	{0x200003E8, "Crimson Amber Medallion", 400000, 10},
	{0x100249F0, "Iron Kasa", 506110, 70},
}

func TestItemSortKeys_KnownItems(t *testing.T) {
	for _, tc := range sortKeyKnown {
		sk, ok := ItemSortKeys[tc.id]
		if !ok {
			t.Errorf("ItemSortKeys[0x%08X] (%s): missing entry", tc.id, tc.name)
			continue
		}
		if sk.SortId != tc.wantSortId {
			t.Errorf("ItemSortKeys[0x%08X] (%s): SortId want %d got %d", tc.id, tc.name, tc.wantSortId, sk.SortId)
		}
		if sk.SortGroupId != tc.wantGroupId {
			t.Errorf("ItemSortKeys[0x%08X] (%s): SortGroupId want %d got %d", tc.id, tc.name, tc.wantGroupId, sk.SortGroupId)
		}
	}
}

func TestItemSortKeys_WeaponsPresent(t *testing.T) {
	var weaponCount int
	for id := range ItemSortKeys {
		if id < 0x10000000 {
			weaponCount++
		}
	}
	if weaponCount < 100 {
		t.Errorf("expected ≥100 weapon entries (id < 0x10000000), got %d", weaponCount)
	}
}

func TestItemSortKeys_ArmorPresent(t *testing.T) {
	var armorCount int
	for id := range ItemSortKeys {
		if id >= 0x10000000 && id < 0x20000000 {
			armorCount++
		}
	}
	if armorCount < 100 {
		t.Errorf("expected ≥100 armor entries (0x10000000 ≤ id < 0x20000000), got %d", armorCount)
	}
}

func TestItemSortKeys_TalismansPresent(t *testing.T) {
	var talismanCount int
	for id := range ItemSortKeys {
		if id >= 0x20000000 && id < 0x30000000 {
			talismanCount++
		}
	}
	if talismanCount < 50 {
		t.Errorf("expected ≥50 talisman entries (0x20000000 ≤ id < 0x30000000), got %d", talismanCount)
	}
}

func TestItemSortKeys_NoZeroSortId(t *testing.T) {
	for id, sk := range ItemSortKeys {
		if sk.SortId == 0 {
			t.Errorf("ItemSortKeys[0x%08X]: SortId must not be 0 (skipped during generation)", id)
		}
	}
}
