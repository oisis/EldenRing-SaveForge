package main

import (
	"encoding/binary"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
	"github.com/oisis/EldenRing-SaveForge/backend/db"
)

// keyItemFixture builds an App whose slot 0 holds a single stackable Key Item at
// physical KeyItems row 0, backed by a real binary slot.Data laid out over the
// full inventory section (CommonItems + KeyItems + counters). MagicOffset is 0;
// the in-memory KeyItems slice index matches the binary row so the issue-7 stack
// bump can target the record by row.
func keyItemFixture(handle, index, qty uint32) *App {
	const magicOff = 0
	startOff := magicOff + core.InvStartFromMagic
	keyStartOff := startOff + core.CommonItemCount*core.InvRecordLen + core.InvKeyCountHeader
	nextEquipIdxOff := keyStartOff + core.KeyItemCount*core.InvRecordLen
	nextAcqSortIdOff := nextEquipIdxOff + 4
	bufSize := nextAcqSortIdOff + 4 + 64

	app := NewApp()
	app.save = &core.SaveFile{}
	slot := &app.save.Slots[0]
	slot.Version = 1
	slot.MagicOffset = magicOff
	slot.Data = make([]byte, bufSize)
	slot.GaMap = make(map[uint32]uint32)

	off := keyStartOff // physical row 0
	binary.LittleEndian.PutUint32(slot.Data[off:], handle)
	binary.LittleEndian.PutUint32(slot.Data[off+4:], qty)
	binary.LittleEndian.PutUint32(slot.Data[off+8:], index)
	slot.Inventory.KeyItems = []core.InventoryItem{{GaItemHandle: handle, Quantity: qty, Index: index}}

	nextAcq := index + 100
	binary.LittleEndian.PutUint32(slot.Data[nextEquipIdxOff:], nextAcq+1)
	binary.LittleEndian.PutUint32(slot.Data[nextAcqSortIdOff:], nextAcq)
	slot.Inventory.NextEquipIndex = nextAcq + 1
	slot.Inventory.NextAcquisitionSortId = nextAcq
	return app
}

func keyItemBinQty(slot *core.SaveSlot, row int) uint32 {
	keyStartOff := slot.MagicOffset + core.InvStartFromMagic +
		core.CommonItemCount*core.InvRecordLen + core.InvKeyCountHeader
	return binary.LittleEndian.Uint32(slot.Data[keyStartOff+row*core.InvRecordLen+4:])
}

// gameMaxInv returns the Game Max inventory cap the add path would resolve for id.
func gameMaxInv(id uint32) int {
	inv, _ := addItemCaps(db.GetItemData(id), true)
	return int(inv)
}

// TestChaosKeyItem_LostAshesOfWarBumped: an already-owned Lost Ashes of War stack
// (KeyItems, qty 1) must be raised to the Game Max in place, not skipped, and no
// duplicate CommonItems record may be created (issue 7).
func TestChaosKeyItem_LostAshesOfWarBumped(t *testing.T) {
	const id, handle = uint32(0x40002756), uint32(0xB0002756)
	want := gameMaxInv(id)
	if want <= 1 {
		t.Fatalf("Lost Ashes of War Game Max inv = %d, expected > 1", want)
	}

	app := keyItemFixture(handle, 4000, 1)
	slot := &app.save.Slots[0]

	res, err := app.AddItemsToCharacterWithGameLimits(0, []uint32{id}, 0, 0, 0, 0, -1, 0)
	if err != nil {
		t.Fatalf("AddItemsToCharacterWithGameLimits: %v", err)
	}
	if res.Added != 1 {
		t.Fatalf("Added = %d, want 1", res.Added)
	}
	if got := slot.Inventory.KeyItems[0].Quantity; got != uint32(want) {
		t.Errorf("in-memory KeyItems qty = %d, want %d", got, want)
	}
	if got := keyItemBinQty(slot, 0); got != uint32(want) {
		t.Errorf("binary KeyItems qty = %d, want %d", got, want)
	}
	if n := len(slot.Inventory.CommonItems); n != 0 {
		t.Errorf("CommonItems len = %d, want 0 (no duplicate record)", n)
	}
}

// TestChaosKeyItem_LarvalTearBumped mirrors the Lost Ashes case for Larval Tear:
// existing stack below target, Game Max updates the row in place.
func TestChaosKeyItem_LarvalTearBumped(t *testing.T) {
	const id, handle = uint32(0x40001FF9), uint32(0xB0001FF9)
	want := gameMaxInv(id)
	if want <= 1 {
		t.Fatalf("Larval Tear Game Max inv = %d, expected > 1", want)
	}

	app := keyItemFixture(handle, 4100, 3)
	slot := &app.save.Slots[0]

	res, err := app.AddItemsToCharacterWithGameLimits(0, []uint32{id}, 0, 0, 0, 0, -1, 0)
	if err != nil {
		t.Fatalf("AddItemsToCharacterWithGameLimits: %v", err)
	}
	if res.Added != 1 {
		t.Fatalf("Added = %d, want 1", res.Added)
	}
	if got := slot.Inventory.KeyItems[0].Quantity; got != uint32(want) {
		t.Errorf("in-memory KeyItems qty = %d, want %d", got, want)
	}
	if got := keyItemBinQty(slot, 0); got != uint32(want) {
		t.Errorf("binary KeyItems qty = %d, want %d", got, want)
	}
	if n := len(slot.Inventory.CommonItems); n != 0 {
		t.Errorf("CommonItems len = %d, want 0", n)
	}
}

// TestChaosKeyItem_AlreadyAtTargetNoOp: a stackable Key Item already at (or above)
// the requested target must not mutate and must not gain a duplicate record.
func TestChaosKeyItem_AlreadyAtTargetNoOp(t *testing.T) {
	const id, handle = uint32(0x40001FF9), uint32(0xB0001FF9)
	target := gameMaxInv(id)
	if target <= 1 {
		t.Fatalf("Larval Tear Game Max inv = %d, expected > 1", target)
	}

	app := keyItemFixture(handle, 4200, uint32(target))
	slot := &app.save.Slots[0]

	res, err := app.AddItemsToCharacterWithGameLimits(0, []uint32{id}, 0, 0, 0, 0, -1, 0)
	if err != nil {
		t.Fatalf("AddItemsToCharacterWithGameLimits: %v", err)
	}
	if res.Added != 0 {
		t.Errorf("Added = %d, want 0 (already at target)", res.Added)
	}
	if got := slot.Inventory.KeyItems[0].Quantity; got != uint32(target) {
		t.Errorf("in-memory KeyItems qty = %d, want unchanged %d", got, target)
	}
	if got := keyItemBinQty(slot, 0); got != uint32(target) {
		t.Errorf("binary KeyItems qty = %d, want unchanged %d", got, target)
	}
	if n := len(slot.Inventory.CommonItems); n != 0 {
		t.Errorf("CommonItems len = %d, want 0", n)
	}
	if len(res.SkippedExisting) != 1 || res.SkippedExisting[0].ItemID != id {
		t.Errorf("SkippedExisting = %+v, want [0x%08X]", res.SkippedExisting, id)
	}
}

// TestChaosKeyItem_NonStackableStillSkipped: a MaxInventory-1 Key Item (Godskin
// Prayerbook) already owned stays skipped as owned under Game Max: no bump,
// no duplicate record.
func TestChaosKeyItem_NonStackableStillSkipped(t *testing.T) {
	const id, handle = uint32(0x40002299), uint32(0xB0002299) // Godskin Prayerbook
	if want := gameMaxInv(id); want != 1 {
		t.Fatalf("Godskin Prayerbook Game Max inv = %d, expected 1", want)
	}

	app := keyItemFixture(handle, 4300, 1)
	slot := &app.save.Slots[0]

	res, err := app.AddItemsToCharacterWithGameLimits(0, []uint32{id}, 0, 0, 0, 0, -1, 0)
	if err != nil {
		t.Fatalf("AddItemsToCharacterWithGameLimits: %v", err)
	}
	if res.Added != 0 {
		t.Errorf("Added = %d, want 0 (already owned)", res.Added)
	}
	if got := slot.Inventory.KeyItems[0].Quantity; got != 1 {
		t.Errorf("KeyItems qty = %d, want unchanged 1", got)
	}
	if got := keyItemBinQty(slot, 0); got != 1 {
		t.Errorf("binary KeyItems qty = %d, want unchanged 1", got)
	}
	if n := len(slot.Inventory.CommonItems); n != 0 {
		t.Errorf("CommonItems len = %d, want 0", n)
	}
	if len(res.SkippedExisting) != 1 || res.SkippedExisting[0].ItemID != id {
		t.Errorf("SkippedExisting = %+v, want [0x%08X]", res.SkippedExisting, id)
	}
}
