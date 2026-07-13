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

func addKeyItemFixtureRow(t *testing.T, app *App, row int, handle, index, qty uint32) {
	t.Helper()
	slot := &app.save.Slots[0]
	if row != len(slot.Inventory.KeyItems) {
		t.Fatalf("fixture row = %d, want next row %d", row, len(slot.Inventory.KeyItems))
	}
	keyStartOff := slot.MagicOffset + core.InvStartFromMagic +
		core.CommonItemCount*core.InvRecordLen + core.InvKeyCountHeader
	off := keyStartOff + row*core.InvRecordLen
	binary.LittleEndian.PutUint32(slot.Data[off:], handle)
	binary.LittleEndian.PutUint32(slot.Data[off+4:], qty)
	binary.LittleEndian.PutUint32(slot.Data[off+8:], index)
	slot.Inventory.KeyItems = append(slot.Inventory.KeyItems, core.InventoryItem{
		GaItemHandle: handle,
		Quantity:     qty,
		Index:        index,
	})
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

// TestChaosKeyItem_Issue8Items locks the full reported issue-8 list: every one is
// a goods/key item with a Game Max cap > 1, so an already-owned stack below target
// must be bumped in place (issue 7 mechanism), not skipped, no duplicate
// CommonItems record. Seedbed Curse is included deliberately despite carrying no
// "stackable" DB flag: the add path keys stackability off the goods handle prefix,
// not the flag, so it must still bump.
func TestChaosKeyItem_Issue8Items(t *testing.T) {
	cases := []struct {
		name string
		id   uint32
	}{
		{"Stonesword Key", 0x40001F40},
		{"Lost Ashes of War", 0x40002756},
		{"Larval Tear", 0x40001FF9},
		{"Celestial Dew", 0x40000852},
		{"Dragon Heart", 0x4000274C},
		{"Seedbed Curse", 0x40002001}, // no "stackable" flag, must still bump
		{"Deathroot", 0x4000082A},
	}
	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			want := gameMaxInv(tc.id)
			if want <= 1 {
				t.Fatalf("%s Game Max inv = %d, expected > 1", tc.name, want)
			}
			handle := (tc.id & 0x0FFFFFFF) | 0xB0000000
			app := keyItemFixture(handle, uint32(5000+i), 1)
			slot := &app.save.Slots[0]

			res, err := app.AddItemsToCharacterWithGameLimits(0, []uint32{tc.id}, 0, 0, 0, 0, -1, 0)
			if err != nil {
				t.Fatalf("AddItemsToCharacterWithGameLimits: %v", err)
			}
			if res.Added != 1 {
				t.Errorf("Added = %d, want 1", res.Added)
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
		})
	}
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

// TestChaosKeyItem_LarvalTearVariantsRemainIndependent proves that the base-game
// and DLC Larval Tear records can coexist. Raising one to Game Max must neither
// skip nor overwrite the other physical KeyItems row.
func TestChaosKeyItem_LarvalTearVariantsRemainIndependent(t *testing.T) {
	const (
		baseID     = uint32(0x40001FF9)
		dlcID      = uint32(0x401EA3E1)
		baseHandle = uint32(0xB0001FF9)
		dlcHandle  = uint32(0xB01EA3E1)
	)
	baseMax, dlcMax := gameMaxInv(baseID), gameMaxInv(dlcID)
	if baseMax != 99 || dlcMax != 99 {
		t.Fatalf("Game Max inventory caps = base %d, DLC %d; want 99/99", baseMax, dlcMax)
	}

	app := keyItemFixture(baseHandle, 4100, 3)
	addKeyItemFixtureRow(t, app, 1, dlcHandle, 4101, 4)
	slot := &app.save.Slots[0]

	res, err := app.AddItemsToCharacterWithGameLimits(0, []uint32{dlcID}, 0, 0, 0, 0, -1, 0)
	if err != nil {
		t.Fatalf("add DLC Larval Tear: %v", err)
	}
	if res.Added != 1 {
		t.Fatalf("DLC Added = %d, want 1", res.Added)
	}
	if got := slot.Inventory.KeyItems[0].Quantity; got != 3 {
		t.Errorf("base quantity after DLC add = %d, want 3", got)
	}
	if got := keyItemBinQty(slot, 0); got != 3 {
		t.Errorf("base binary quantity after DLC add = %d, want 3", got)
	}
	if got := slot.Inventory.KeyItems[1].Quantity; got != uint32(dlcMax) {
		t.Errorf("DLC quantity = %d, want %d", got, dlcMax)
	}
	if got := keyItemBinQty(slot, 1); got != uint32(dlcMax) {
		t.Errorf("DLC binary quantity = %d, want %d", got, dlcMax)
	}

	res, err = app.AddItemsToCharacterWithGameLimits(0, []uint32{baseID}, 0, 0, 0, 0, -1, 0)
	if err != nil {
		t.Fatalf("add base Larval Tear: %v", err)
	}
	if res.Added != 1 {
		t.Fatalf("base Added = %d, want 1", res.Added)
	}
	if got := slot.Inventory.KeyItems[0].Quantity; got != uint32(baseMax) {
		t.Errorf("base quantity = %d, want %d", got, baseMax)
	}
	if got := keyItemBinQty(slot, 0); got != uint32(baseMax) {
		t.Errorf("base binary quantity = %d, want %d", got, baseMax)
	}
	if got := slot.Inventory.KeyItems[1].Quantity; got != uint32(dlcMax) {
		t.Errorf("DLC quantity after base add = %d, want %d", got, dlcMax)
	}
	if got := keyItemBinQty(slot, 1); got != uint32(dlcMax) {
		t.Errorf("DLC binary quantity after base add = %d, want %d", got, dlcMax)
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
