package core

import "testing"

func nativeTestRecord(handle, itemID uint32) GaItemFull {
	return GaItemFull{
		Handle:          handle,
		ItemID:          itemID,
		Unk2:            0,
		Unk3:            0,
		AoWGaItemHandle: NoCustomAoWHandle,
	}
}

func TestNativeGaItemAllocator_ReusesHolePastLegacyCursor(t *testing.T) {
	slot := makeTestSlot(8)
	slot.GaItems[7] = nativeTestRecord(ItemTypeWeapon|0x00800007, 0x00100007)
	slot.GaMap[ItemTypeWeapon|0x00800007] = 0x00100007
	slot.NextArmamentIndex = len(slot.GaItems)

	handle, err := generateUniqueHandle(slot, ItemTypeArmor)
	if err != nil {
		t.Fatalf("generateUniqueHandle: %v", err)
	}
	if got := handle & 0xFFFF; got != 0 {
		t.Fatalf("physical index = %d, want first free index 0", got)
	}
	if err := allocateGaItem(slot, handle, 0x10100000); err != nil {
		t.Fatalf("allocateGaItem: %v", err)
	}
	if got := slot.GaItems[0].Handle; got != handle {
		t.Fatalf("GaItems[0].Handle = 0x%08X, want 0x%08X", got, handle)
	}
}

func TestNativeGaItemAllocator_AoWUsesIndexRankAndRemovesPassTwoMarker(t *testing.T) {
	slot := makeTestSlot(8)
	oldAoW := uint32(ItemTypeAow | 0x00800003)
	weapon := uint32(ItemTypeWeapon | 0x00800005)
	slot.GaItems[0] = nativeTestRecord(oldAoW, 0x80000003)
	slot.GaItems[5] = nativeTestRecord(weapon, 0x00100005)
	slot.GaMap[oldAoW] = 0x80000003
	slot.GaMap[weapon] = 0x00100005

	handle, err := generateUniqueHandle(slot, ItemTypeAow)
	if err != nil {
		t.Fatalf("generateUniqueHandle: %v", err)
	}
	if got := handle & 0xFFFF; got != 0 {
		t.Fatalf("physical index = %d, want 0", got)
	}
	if err := allocateGaItem(slot, handle, 0x80000010); err != nil {
		t.Fatalf("allocateGaItem: %v", err)
	}
	if slot.GaItems[0].Handle != handle || slot.GaItems[1].Handle != oldAoW {
		t.Fatalf("AoW prefix = [0x%08X, 0x%08X], want [0x%08X, 0x%08X]",
			slot.GaItems[0].Handle, slot.GaItems[1].Handle, handle, oldAoW)
	}
	if slot.GaItems[5].Handle != weapon {
		t.Fatalf("weapon moved from projected position 5 to handle 0x%08X", slot.GaItems[5].Handle)
	}
	if _, err := analyzeNativeGaItemLayout(slot); err != nil {
		t.Fatalf("post-allocation layout: %v", err)
	}
}

func TestNativeGaItemAllocator_RejectsProjectionMismatchWithoutMutation(t *testing.T) {
	slot := makeTestSlot(8)
	slot.GaItems[0] = nativeTestRecord(ItemTypeWeapon|0x00800003, 0x00100003)
	before := append([]GaItemFull(nil), slot.GaItems...)

	err := allocateGaItem(slot, ItemTypeArmor|0x00800000, 0x10100000)
	if err == nil || !contains(err.Error(), "native GaItem projection") {
		t.Fatalf("allocateGaItem error = %v, want native projection refusal", err)
	}
	for i := range before {
		if slot.GaItems[i] != before[i] {
			t.Fatalf("GaItems[%d] mutated on refusal", i)
		}
	}
}

func TestNativeGaItemAllocator_IgnoresHandleOnlyLowIndexCollision(t *testing.T) {
	slot := makeTestSlot(8)
	slot.GaMap[ItemTypeAccessory|0x00000000] = 0x20000000

	handle, err := generateUniqueHandle(slot, ItemTypeWeapon)
	if err != nil {
		t.Fatalf("generateUniqueHandle: %v", err)
	}
	if got := handle & 0xFFFF; got != 0 {
		t.Fatalf("physical index = %d, want 0 despite handle-only collision", got)
	}
}

func TestNativeGaItemAllocator_RejectsHandleItemTypeMismatch(t *testing.T) {
	slot := makeTestSlot(8)
	before := append([]GaItemFull(nil), slot.GaItems...)
	err := allocateGaItem(slot, ItemTypeWeapon|gaItemHandleValidBit, 0x10000001)
	if err == nil || !contains(err.Error(), "conflicts with item ID") {
		t.Fatalf("allocateGaItem error = %v, want type mismatch", err)
	}
	for i := range before {
		if slot.GaItems[i] != before[i] {
			t.Fatalf("GaItems[%d] mutated on type mismatch", i)
		}
	}
}

func TestNativeGaItemAllocator_AcceptsMixedGenerationBytes(t *testing.T) {
	slot := makeTestSlot(8)
	first := uint32(ItemTypeWeapon | 0x00800002)
	second := uint32(ItemTypeArmor | 0x00810004)
	slot.GaItems[2] = nativeTestRecord(first, 0x00100002)
	slot.GaItems[4] = nativeTestRecord(second, 0x10100004)
	slot.GaMap[first] = 0x00100002
	slot.GaMap[second] = 0x10100004

	handle, err := generateUniqueHandle(slot, ItemTypeWeapon)
	if err != nil {
		t.Fatalf("generateUniqueHandle: %v", err)
	}
	if handle&0xFFFF != 0 {
		t.Fatalf("physical index = %d, want 0", handle&0xFFFF)
	}
	if err := allocateGaItem(slot, handle, 0x00100000); err != nil {
		t.Fatalf("allocateGaItem: %v", err)
	}
}
