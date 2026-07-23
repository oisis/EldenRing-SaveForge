package core

import (
	"encoding/binary"
	"reflect"
	"testing"
)

// rawKeyItemFixture builds a serializable round-trip slot whose held KeyItems
// section carries two rows:
//
//   - row 0: a valid-but-unknown/raw handle 0x00000001 (type nibble 0x0, absent
//     from the DB and from GaMap) with quantity 1 and acquisition index 4029 —
//     the exact record whose silent loss this suite regresses;
//   - row 1: an ordinary game-placed key item, so the assertions prove the
//     preservation is row-level and not an "entire section absent" pass.
//
// It never reads or writes an ignored real save file.
func rawKeyItemFixture(t *testing.T) *SaveSlot {
	t.Helper()
	slot := fragmentedGaItemRoundTripFixtureForVersion(t, GaItemVersionBreak+1).Slot
	slot.Inventory.KeyItems = make([]InventoryItem, KeyItemCount)
	slot.Inventory.KeyItems[0] = InventoryItem{GaItemHandle: 0x00000001, Quantity: 1, Index: 4029}
	slot.Inventory.KeyItems[1] = InventoryItem{GaItemHandle: 0x40009C41, Quantity: 1, Index: 4030}
	writeFixtureInventory(slot, slot.Inventory.CommonItems)
	if err := slot.parseFromData(); err != nil {
		t.Fatalf("parseFromData: %v", err)
	}
	return slot
}

// keyItemRowBytes reads one physical held-KeyItems record straight from
// slot.Data at the current layout, independent of any fixed absolute offset so
// GaItem growth that shifts later sections does not break the check.
func keyItemRowBytes(slot *SaveSlot, row int) (handle, qty, idx uint32) {
	keyStart := slot.MagicOffset + InvStartFromMagic +
		CommonItemCount*InvRecordLen + InvKeyCountHeader
	off := keyStart + row*InvRecordLen
	return binary.LittleEndian.Uint32(slot.Data[off:]),
		binary.LittleEndian.Uint32(slot.Data[off+4:]),
		binary.LittleEndian.Uint32(slot.Data[off+8:])
}

func assertKeyRowsPreserved(t *testing.T, slot *SaveSlot, where string) {
	t.Helper()
	want := []InventoryItem{
		{GaItemHandle: 0x00000001, Quantity: 1, Index: 4029},
		{GaItemHandle: 0x40009C41, Quantity: 1, Index: 4030},
	}
	for row, w := range want {
		got := slot.Inventory.KeyItems[row]
		if got != w {
			t.Fatalf("%s: parsed KeyItems[%d] = %+v, want %+v", where, row, got, w)
		}
		h, q, i := keyItemRowBytes(slot, row)
		if h != w.GaItemHandle || q != w.Quantity || i != w.Index {
			t.Fatalf("%s: raw KeyItems[%d] bytes = {0x%08X,%d,%d}, want {0x%08X,%d,%d}",
				where, row, h, q, i, w.GaItemHandle, w.Quantity, w.Index)
		}
	}
}

// TestRebuildSlotFull_PreservesKeyItemsWithShiftedLayout is the core regression.
// It reproduces the general stale-layout state a prior writer leaves behind — the
// state the previous two-offset heuristic could NOT detect — and proves the
// fail-closed refresh handles it.
//
// The setup performs a REAL layout shift: every byte from GaItemsStart onward is
// physically moved right by `shift` in slot.Data (the exact effect of a GaItems
// section growing), so the live MagicPattern, held inventory, and gestures now sit
// `shift` bytes later. No reparse follows, so the cached anchors are stale:
//
//   - MagicOffset stays at the OLD position (stale by `shift`) — and every offset
//     the rebuild derives from it (oldGaLimit, the heuristic's inventory end) is
//     stale with it;
//   - UnlockedRegionsOffset and SectionMap retain their old positions because
//     nothing reparses the shifted bytes.
//
// Crucially this is NOT an isolated UnlockedRegionsOffset poke: MagicOffset is
// stale too. The previous guard compared UnlockedRegionsOffset against a
// MagicOffset-derived inventory end; because that end is computed from the same
// stale MagicOffset, the comparison stays false — the guard never fires. The shift
// is chosen so the old regions boundary lands exactly between the moved key_count
// header and KeyItems[0]. The pre-fix path therefore preserves the header but drops
// the physical KeyItems rows into fresh zero padding — the exact field signature.
// The fail-closed refresh rediscovers MagicOffset from the current bytes,
// recomputes the whole offset chain and section map, slices the correct window, and
// preserves every row.
func TestRebuildSlotFull_PreservesKeyItemsWithShiftedLayout(t *testing.T) {
	slot := rawKeyItemFixture(t)

	oldMagic := slot.MagicOffset
	oldURO := slot.UnlockedRegionsOffset
	oldSectionMap := append([]SectionRange(nil), slot.SectionMap...)
	keyStart := oldMagic + InvStartFromMagic +
		CommonItemCount*InvRecordLen + InvKeyCountHeader

	// Choose the physical shift so the old regions boundary falls exactly after
	// the moved key_count header and immediately before KeyItems[0]. This is a
	// real stale-cache state: the cached boundary itself is not changed.
	shift := oldURO - keyStart
	if shift <= 0 || GaItemsStart+shift >= oldMagic {
		t.Fatalf("shift 0x%X does not fit before magic 0x%X (keyStart=0x%X, URO=0x%X)",
			shift, oldMagic, keyStart, oldURO)
	}

	// Physically move [GaItemsStart, SlotSize-shift) right by `shift`; the inserted
	// gap is zero-filled (empty GaItem padding). MagicOffset / SectionMap are left
	// untouched, so they now describe the OLD positions.
	shifted := make([]byte, SlotSize)
	copy(shifted, slot.Data[:GaItemsStart])
	copy(shifted[GaItemsStart+shift:], slot.Data[GaItemsStart:SlotSize-shift])
	slot.Data = shifted

	if slot.MagicOffset != oldMagic || slot.UnlockedRegionsOffset != oldURO ||
		!reflect.DeepEqual(slot.SectionMap, oldSectionMap) {
		t.Fatal("physical shift unexpectedly changed cached layout")
	}
	if got := NewReader(slot.Data).FindPattern(MagicPattern); got != oldMagic+shift {
		t.Fatalf("live MagicPattern = 0x%X, want 0x%X", got, oldMagic+shift)
	}
	if got := oldMagic + InvStartFromMagic + CommonItemCount*InvRecordLen + InvKeyCountHeader + shift; got != oldURO {
		t.Fatalf("moved KeyItems[0] start = 0x%X, want stale URO 0x%X", got, oldURO)
	}

	rebuilt, err := RebuildSlotFull(slot)
	if err != nil {
		t.Fatalf("RebuildSlotFull: %v", err)
	}
	slot.Data = rebuilt
	if err := slot.parseFromData(); err != nil {
		t.Fatalf("parseFromData: %v", err)
	}
	assertKeyRowsPreserved(t, slot, "shifted-layout rebuild")
}

// TestAddItemsToSlotBatch_PreservesRawKeyItemsRowAcrossGrowth proves a GaItem-
// growing batch addition that invokes RebuildSlotFull + reparse preserves the
// raw KeyItems row exactly. The added weapon allocates a fresh GaItem, so the
// GaItems section grows and every later section (inventory included) shifts.
func TestAddItemsToSlotBatch_PreservesRawKeyItemsRowAcrossGrowth(t *testing.T) {
	slot := rawKeyItemFixture(t)
	before := slot.MagicOffset
	if err := AddItemsToSlotBatch(slot, []ItemToAdd{{ItemID: 0x000F4241, InvQty: 1}}); err != nil {
		t.Fatalf("AddItemsToSlotBatch: %v", err)
	}
	if slot.MagicOffset <= before {
		t.Fatalf("batch add did not grow GaItems (magic 0x%X -> 0x%X)", before, slot.MagicOffset)
	}
	assertKeyRowsPreserved(t, slot, "after batch add")
}
