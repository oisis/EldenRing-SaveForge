package core

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// rawQtyAt reads the raw quantity field of the record at a physical byte offset.
func rawInvQtyAt(slot *SaveSlot, base, row int) uint32 {
	return binary.LittleEndian.Uint32(slot.Data[base+row*InvRecordLen+4:])
}

func fpAt(t *testing.T, slot *SaveSlot, scope string, row int) string {
	t.Helper()
	fp, ok := FingerprintRecordAt(slot, scope, row)
	if !ok {
		t.Fatalf("FingerprintRecordAt(%s,%d) not addressable", scope, row)
	}
	return fp
}

func TestClampQuantity_InventoryCommon(t *testing.T) {
	slot := buildInvFixtureNZ(t, []InventoryItem{{GaItemHandle: smithingStoneHandle, Quantity: 1500, Index: 500}})
	base := slot.MagicOffset + InvStartFromMagic

	ch, err := ClampInventoryQuantityAt(slot, repairScopeInventoryCommon, 0, fpAt(t, slot, repairScopeInventoryCommon, 0))
	if err != nil {
		t.Fatalf("clamp: %v", err)
	}
	if ch.OldQuantity != 1500 || ch.NewQuantity != 999 || ch.Cap != 999 {
		t.Fatalf("change = %+v, want old 1500 / new 999 / cap 999", ch)
	}
	if slot.Inventory.CommonItems[0].Quantity != 999 {
		t.Errorf("in-memory quantity = %d, want 999", slot.Inventory.CommonItems[0].Quantity)
	}
	if got := rawInvQtyAt(slot, base, 0); got != 999 {
		t.Errorf("raw quantity = %d, want 999", got)
	}
}

func TestClampQuantity_InventoryKey(t *testing.T) {
	// Stonesword Key keeps the conservative Normal Mode cap 55, but regulation
	// allows 99. Repair clamps only to the technical game cap.
	keyStart := 16 + InvStartFromMagic + CommonItemCount*InvRecordLen + InvKeyCountHeader
	slot := buildInvFixtureNZ(t, nil)
	// Extend Data to cover the key block and place a record at key row 0.
	if keyStart+InvRecordLen > len(slot.Data) {
		t.Fatalf("fixture Data too short for key block")
	}
	binary.LittleEndian.PutUint32(slot.Data[keyStart:], stoneswordKeyHandle)
	binary.LittleEndian.PutUint32(slot.Data[keyStart+4:], 200)
	binary.LittleEndian.PutUint32(slot.Data[keyStart+8:], 700)
	slot.Inventory.KeyItems = []InventoryItem{{GaItemHandle: stoneswordKeyHandle, Quantity: 200, Index: 700}}

	ch, err := ClampInventoryQuantityAt(slot, repairScopeInventoryKey, 0, fpAt(t, slot, repairScopeInventoryKey, 0))
	if err != nil {
		t.Fatalf("clamp: %v", err)
	}
	if ch.NewQuantity != 99 || slot.Inventory.KeyItems[0].Quantity != 99 {
		t.Fatalf("key clamp: change=%+v inmem=%d, want 99", ch, slot.Inventory.KeyItems[0].Quantity)
	}
	if got := binary.LittleEndian.Uint32(slot.Data[keyStart+4:]); got != 99 {
		t.Errorf("raw key quantity = %d, want 99", got)
	}
}

func TestClampQuantity_GappedStorage_MapsCompactedToPhysical(t *testing.T) {
	// Physical rows 0 and 2 populated (gap at 1) → compacted rows 0,1.
	slot := buildStorageFixtureRecords(t, map[int]InventoryItem{
		0: {GaItemHandle: smithingStoneHandle, Quantity: 100, Index: 700},
		2: {GaItemHandle: smithingStoneHandle, Quantity: 1500, Index: 710},
	})
	storageStart := slot.StorageBoxOffset + StorageHeaderSkip

	// Clamp compacted row 1 → physical slot 2.
	ch, err := ClampInventoryQuantityAt(slot, repairScopeStorageCommon, 1, fpAt(t, slot, repairScopeStorageCommon, 1))
	if err != nil {
		t.Fatalf("clamp: %v", err)
	}
	if ch.NewQuantity != 999 {
		t.Fatalf("new quantity = %d, want 999", ch.NewQuantity)
	}
	if got := binary.LittleEndian.Uint32(slot.Data[storageStart+2*InvRecordLen+4:]); got != 999 {
		t.Errorf("physical row 2 raw quantity = %d, want 999", got)
	}
	// Physical row 0 (compacted row 0) untouched.
	if got := binary.LittleEndian.Uint32(slot.Data[storageStart+0*InvRecordLen+4:]); got != 100 {
		t.Errorf("physical row 0 raw quantity changed to %d, want 100", got)
	}
	if slot.Storage.CommonItems[0].Quantity != 100 {
		t.Errorf("compacted row 0 in-memory quantity changed to %d, want 100", slot.Storage.CommonItems[0].Quantity)
	}
}

func TestClampQuantity_PreservesHighBit(t *testing.T) {
	const flag = uint32(0x80000000)
	slot := buildInvFixtureNZ(t, []InventoryItem{{GaItemHandle: smithingStoneHandle, Quantity: flag | 1500, Index: 500}})

	ch, err := ClampInventoryQuantityAt(slot, repairScopeInventoryCommon, 0, fpAt(t, slot, repairScopeInventoryCommon, 0))
	if err != nil {
		t.Fatalf("clamp: %v", err)
	}
	if ch.NewQuantity != flag|999 {
		t.Fatalf("new raw = 0x%08X, want 0x%08X (flag preserved)", ch.NewQuantity, flag|999)
	}
	if slot.Inventory.CommonItems[0].Quantity != flag|999 {
		t.Errorf("in-memory raw = 0x%08X, want flag|999", slot.Inventory.CommonItems[0].Quantity)
	}
}

func TestClampQuantity_GameCapDoesNotScaleWithNG(t *testing.T) {
	slot := buildInvFixtureNZ(t, []InventoryItem{{GaItemHandle: stoneswordKeyHandle, Quantity: 300, Index: 500}})
	slot.Player.ClearCount = 3

	ch, err := ClampInventoryQuantityAt(slot, repairScopeInventoryCommon, 0, fpAt(t, slot, repairScopeInventoryCommon, 0))
	if err != nil {
		t.Fatalf("clamp: %v", err)
	}
	if ch.Cap != 99 || ch.NewQuantity != 99 {
		t.Fatalf("NG+3 clamp: change=%+v, want technical cap/new 99", ch)
	}
}

func TestClampQuantity_UnflaggedDoesNotScale(t *testing.T) {
	slot := buildInvFixtureNZ(t, []InventoryItem{{GaItemHandle: smithingStoneHandle, Quantity: 1500, Index: 500}})
	slot.Player.ClearCount = 3 // Smithing Stone does NOT scale → cap stays 999

	ch, err := ClampInventoryQuantityAt(slot, repairScopeInventoryCommon, 0, fpAt(t, slot, repairScopeInventoryCommon, 0))
	if err != nil {
		t.Fatalf("clamp: %v", err)
	}
	if ch.Cap != 999 || ch.NewQuantity != 999 {
		t.Fatalf("unflagged clamp at NG+3: change=%+v, want cap/new 999", ch)
	}
}

func TestClampQuantity_ExactCap_NoMutation(t *testing.T) {
	slot := buildInvFixtureNZ(t, []InventoryItem{{GaItemHandle: smithingStoneHandle, Quantity: 999, Index: 500}})
	before := append([]byte(nil), slot.Data...)

	if _, err := ClampInventoryQuantityAt(slot, repairScopeInventoryCommon, 0, fpAt(t, slot, repairScopeInventoryCommon, 0)); err == nil {
		t.Fatal("clamp at exact cap must error")
	}
	if !bytes.Equal(before, slot.Data) || slot.Inventory.CommonItems[0].Quantity != 999 {
		t.Fatal("clamp at exact cap mutated the slot")
	}
}

func TestClampQuantity_BelowCap_NoMutation(t *testing.T) {
	slot := buildInvFixtureNZ(t, []InventoryItem{{GaItemHandle: smithingStoneHandle, Quantity: 500, Index: 500}})
	before := append([]byte(nil), slot.Data...)

	if _, err := ClampInventoryQuantityAt(slot, repairScopeInventoryCommon, 0, fpAt(t, slot, repairScopeInventoryCommon, 0)); err == nil {
		t.Fatal("clamp below cap must error")
	}
	if !bytes.Equal(before, slot.Data) {
		t.Fatal("clamp below cap mutated the slot")
	}
}

func TestClampQuantity_StaleFingerprint_NoMutation(t *testing.T) {
	slot := buildInvFixtureNZ(t, []InventoryItem{{GaItemHandle: smithingStoneHandle, Quantity: 1500, Index: 500}})
	before := append([]byte(nil), slot.Data...)

	if _, err := ClampInventoryQuantityAt(slot, repairScopeInventoryCommon, 0, "deadbeefdeadbeef"); err == nil {
		t.Fatal("stale fingerprint must error")
	}
	if !bytes.Equal(before, slot.Data) || slot.Inventory.CommonItems[0].Quantity != 1500 {
		t.Fatal("stale fingerprint mutated the slot")
	}
}

func TestClampQuantity_InventoryDrift_NoMutation(t *testing.T) {
	slot := buildInvFixtureNZ(t, []InventoryItem{{GaItemHandle: smithingStoneHandle, Quantity: 1500, Index: 500}})
	fp := fpAt(t, slot, repairScopeInventoryCommon, 0)
	// Corrupt the raw quantity so raw != in-memory; fingerprint still matches
	// the in-memory item, so only the drift guard can catch it.
	base := slot.MagicOffset + InvStartFromMagic
	binary.LittleEndian.PutUint32(slot.Data[base+4:], 1234)

	if _, err := ClampInventoryQuantityAt(slot, repairScopeInventoryCommon, 0, fp); err == nil {
		t.Fatal("raw/list drift must error")
	}
}

func TestClampQuantity_StorageDrift_NoMutation(t *testing.T) {
	slot := buildStorageFixtureRecords(t, map[int]InventoryItem{
		0: {GaItemHandle: smithingStoneHandle, Quantity: 1500, Index: 700},
	})
	fp := fpAt(t, slot, repairScopeStorageCommon, 0)
	storageStart := slot.StorageBoxOffset + StorageHeaderSkip
	binary.LittleEndian.PutUint32(slot.Data[storageStart+4:], 42) // raw drifts from in-memory

	if _, err := ClampInventoryQuantityAt(slot, repairScopeStorageCommon, 0, fp); err == nil {
		t.Fatal("storage raw/list drift must error")
	}
}

func TestClampQuantity_InvalidScope(t *testing.T) {
	slot := buildInvFixtureNZ(t, []InventoryItem{{GaItemHandle: smithingStoneHandle, Quantity: 1500, Index: 500}})
	if _, err := ClampInventoryQuantityAt(slot, "bogus_scope", 0, ""); err == nil {
		t.Fatal("unsupported scope must error")
	}
}

func TestClampQuantity_RowOutOfRange(t *testing.T) {
	slot := buildInvFixtureNZ(t, []InventoryItem{{GaItemHandle: smithingStoneHandle, Quantity: 1500, Index: 500}})
	for _, row := range []int{-1, 5, 99} {
		if _, err := ClampInventoryQuantityAt(slot, repairScopeInventoryCommon, row, ""); err == nil {
			t.Errorf("row %d must error", row)
		}
	}
}

func TestClampQuantity_NilSlot(t *testing.T) {
	if _, err := ClampInventoryQuantityAt(nil, repairScopeInventoryCommon, 0, ""); err == nil {
		t.Fatal("nil slot must error")
	}
}

func TestClampQuantity_UnknownRecord_Rejected(t *testing.T) {
	// Illegal handle prefix → resolution unknown → no applicable cap.
	slot := buildInvFixtureNZ(t, []InventoryItem{{GaItemHandle: 0x10000005, Quantity: 9999, Index: 500}})
	before := append([]byte(nil), slot.Data...)
	if _, err := ClampInventoryQuantityAt(slot, repairScopeInventoryCommon, 0, fpAt(t, slot, repairScopeInventoryCommon, 0)); err == nil {
		t.Fatal("unknown record must be rejected")
	}
	if !bytes.Equal(before, slot.Data) {
		t.Fatal("unknown record clamp mutated the slot")
	}
}

func TestClampQuantity_TechnicalPlaceholder_Rejected(t *testing.T) {
	slot := buildInvFixtureNZ(t, []InventoryItem{{GaItemHandle: nakedHeadHandle, Quantity: 9, Index: 500}})
	slot.GaMap[nakedHeadHandle] = nakedHeadItemID // resolves as technical placeholder
	if _, err := ClampInventoryQuantityAt(slot, repairScopeInventoryCommon, 0, fpAt(t, slot, repairScopeInventoryCommon, 0)); err == nil {
		t.Fatal("technical placeholder must be rejected")
	}
}

func TestClampQuantity_ZeroCap_Rejected(t *testing.T) {
	// Wondrous Physick GameMaxStorage 0 → not permitted in storage → clamp must refuse
	// (removal is the correct repair; clamp must never drive quantity to zero).
	slot := buildStorageFixtureRecords(t, map[int]InventoryItem{
		0: {GaItemHandle: physickHandleQty, Quantity: 5, Index: 700},
	})
	before := append([]byte(nil), slot.Data...)
	if _, err := ClampInventoryQuantityAt(slot, repairScopeStorageCommon, 0, fpAt(t, slot, repairScopeStorageCommon, 0)); err == nil {
		t.Fatal("zero-cap record must be rejected by the clamp primitive")
	}
	if !bytes.Equal(before, slot.Data) {
		t.Fatal("zero-cap clamp mutated the slot")
	}
}

func TestClampQuantity_OnlySelectedRowChanges(t *testing.T) {
	slot := buildInvFixtureNZ(t, []InventoryItem{
		{GaItemHandle: smithingStoneHandle, Quantity: 1500, Index: 500},
		{GaItemHandle: smithingStoneHandle, Quantity: 1500, Index: 600},
	})
	if _, err := ClampInventoryQuantityAt(slot, repairScopeInventoryCommon, 0, fpAt(t, slot, repairScopeInventoryCommon, 0)); err != nil {
		t.Fatalf("clamp row 0: %v", err)
	}
	if slot.Inventory.CommonItems[0].Quantity != 999 {
		t.Errorf("row 0 quantity = %d, want 999", slot.Inventory.CommonItems[0].Quantity)
	}
	if slot.Inventory.CommonItems[1].Quantity != 1500 {
		t.Errorf("row 1 (same handle) changed to %d, want untouched 1500", slot.Inventory.CommonItems[1].Quantity)
	}
}

// ---- EffectiveQuantityCap direct tests --------------------------------------

func TestEffectiveQuantityCap(t *testing.T) {
	cases := []struct {
		name       string
		rec        ResolvedRecord
		clearCount uint32
		wantLimit  uint64
		wantApply  bool
	}{
		{"known inventory", resolveRec(repairScopeInventoryCommon, 0, smithingStoneHandle, 1, nil), 0, 999, true},
		{"known key inventory", resolveRec(repairScopeInventoryKey, 0, stoneswordKeyHandle, 1, nil), 0, 99, true},
		{"known storage", resolveRec(repairScopeStorageCommon, 0, smithingStoneHandle, 1, nil), 0, 999, true},
		{"NG+ does not scale game cap", resolveRec(repairScopeInventoryCommon, 0, stoneswordKeyHandle, 1, nil), 3, 99, true},
		{"storage game cap", resolveRec(repairScopeStorageCommon, 0, stoneswordKeyHandle, 1, nil), 3, 600, true},
		{"unknown record", resolveRec(repairScopeInventoryCommon, 0, 0x10000005, 1, nil), 0, 0, false},
		{"technical placeholder", resolveRec(repairScopeStorageCommon, 0, nakedHeadHandle, 1, map[uint32]uint32{nakedHeadHandle: nakedHeadItemID}), 0, 0, false},
		{"invalid scope", ResolvedRecord{Resolution: ResolutionKnownDB, Scope: "bogus"}, 0, 0, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			limit, applies := EffectiveQuantityCap(c.rec, c.clearCount)
			if limit != c.wantLimit || applies != c.wantApply {
				t.Errorf("EffectiveQuantityCap = (%d,%v), want (%d,%v)", limit, applies, c.wantLimit, c.wantApply)
			}
		})
	}
}
