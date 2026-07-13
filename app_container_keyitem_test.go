package main

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
	"github.com/oisis/EldenRing-SaveForge/backend/db"
	"github.com/oisis/EldenRing-SaveForge/backend/db/data"
)

// Confirmed against the real save tmp/save/ER0000.sl2: the game's containers
// (Cracked Pot, Ritual Pot, …) live in Inventory.KeyItems with a 0xB0… goods
// handle — never in Inventory.CommonItems. These tests drive the public
// App.AddItemsToCharacter path so they cover the whole add + rollback flow, not
// just the upsert helper in isolation.
const (
	firePotID        = uint32(0x4000012C)
	firePotHandle    = uint32(0xB000012C)
	crackedPotHandle = uint32(0xB000251C)

	sparkAromaticID     = uint32(0x40000DB6)
	upliftingAromaticID = uint32(0x40000DAC)
	upliftingHandle     = uint32(0xB0000DAC)
	perfumeBottleHandle = uint32(0xB0002526)
)

// findKeyItem returns the row and quantity of the first KeyItems record with the
// given handle, or (-1, 0) if absent.
func findKeyItem(slot *core.SaveSlot, handle uint32) (int, uint32) {
	for i := range slot.Inventory.KeyItems {
		if slot.Inventory.KeyItems[i].GaItemHandle == handle {
			return i, slot.Inventory.KeyItems[i].Quantity
		}
	}
	return -1, 0
}

// eventFlagsRegionSize covers every Cracked Pot flag the container loop touches:
// the pickup block (66000 → byte 0x7D0, group ends ~0x7E7) and Kale's vendor
// purchase flag 710580 → byte 0x367B. Sized past the highest so no SetEventFlag
// runs out of bounds (an out-of-bounds warning would hit runtime logging).
const eventFlagsRegionSize = 0x4000

// withContainerEventFlags carves a zeroed event-flags region at the end of the
// slot buffer and wires EventFlagsOffset to it, so the container loop can set
// pickup flags and db.GetEventFlag can read them back.
func withContainerEventFlags(app *App) {
	slot := &app.save.Slots[0]
	slot.EventFlagsOffset = len(slot.Data)
	slot.Data = append(slot.Data, make([]byte, eventFlagsRegionSize)...)
}

// TestContainer covers the container-writer patch through the public add path.
func TestContainer(t *testing.T) {
	// 1. Existing container: a Cracked Pot already in KeyItems has its quantity
	// bumped to match the pots added — in memory and in the binary KeyItems
	// record — never spawning a CommonItems container record.
	t.Run("ExistingCrackedPotBumped", func(t *testing.T) {
		app := remembranceGameLimitsFixture()
		addKeyItemFixtureRow(t, app, 0, crackedPotHandle, 2000, 1)
		withContainerEventFlags(app)
		slot := &app.save.Slots[0]

		res, err := app.AddItemsToCharacter(0, []uint32{firePotID}, 0, 0, 0, 0, 2, 0)
		if err != nil {
			t.Fatalf("AddItemsToCharacter: %v", err)
		}
		if res.Added != 1 {
			t.Fatalf("Added = %d, want 1", res.Added)
		}

		if got := slot.Inventory.KeyItems[0].Quantity; got != 2 {
			t.Errorf("in-memory Cracked Pot qty = %d, want 2", got)
		}
		if got := keyItemBinQty(slot, 0); got != 2 {
			t.Errorf("binary Cracked Pot qty = %d, want 2", got)
		}
		invStart := slot.MagicOffset + core.InvStartFromMagic
		if _, ok := quantityInRecords(slot.Data, invStart, core.CommonItemCount, crackedPotHandle); ok {
			t.Error("Cracked Pot found in CommonItems, want none")
		}

		flags := slot.Data[slot.EventFlagsOffset:]
		for i := 0; i < 2; i++ {
			if set, err := db.GetEventFlag(flags, data.ContainerPickupFlags[data.CrackedPotKeyItemID][i]); err != nil || !set {
				t.Errorf("pickup flag[%d] set=%v err=%v, want set", i, set, err)
			}
		}
	})

	// 2. Missing container: adding one Fire Pot creates the Cracked Pot from
	// scratch, only in KeyItems, with the canonical handle, qty 1 and a fresh
	// acquisition index.
	t.Run("MissingCrackedPotCreated", func(t *testing.T) {
		app := remembranceGameLimitsFixture()
		slot := &app.save.Slots[0]
		slot.Inventory.KeyItems = make([]core.InventoryItem, core.KeyItemCount)
		withContainerEventFlags(app)

		res, err := app.AddItemsToCharacter(0, []uint32{firePotID}, 0, 0, 0, 0, 1, 0)
		if err != nil {
			t.Fatalf("AddItemsToCharacter: %v", err)
		}
		if res.Added != 1 {
			t.Fatalf("Added = %d, want 1", res.Added)
		}

		row := -1
		for i := range slot.Inventory.KeyItems {
			if slot.Inventory.KeyItems[i].GaItemHandle == crackedPotHandle {
				row = i
				break
			}
		}
		if row < 0 {
			t.Fatal("Cracked Pot not found in KeyItems")
		}
		it := slot.Inventory.KeyItems[row]
		if it.Quantity != 1 {
			t.Errorf("Cracked Pot qty = %d, want 1", it.Quantity)
		}
		// Fresh, safe acquisition index: at least the fixture's starting
		// NextAcquisitionSortId (1000), and the counter must have advanced past it.
		if it.Index < 1000 {
			t.Errorf("Cracked Pot acquisition index = %d, want >= 1000", it.Index)
		}
		if slot.Inventory.NextAcquisitionSortId <= it.Index {
			t.Errorf("NextAcquisitionSortId = %d, want > record index %d", slot.Inventory.NextAcquisitionSortId, it.Index)
		}
		if got := keyItemBinQty(slot, row); got != 1 {
			t.Errorf("binary Cracked Pot qty = %d, want 1", got)
		}
		keyStart := slot.MagicOffset + core.InvStartFromMagic +
			core.CommonItemCount*core.InvRecordLen + core.InvKeyCountHeader
		recOff := keyStart + row*core.InvRecordLen
		if got := binary.LittleEndian.Uint32(slot.Data[recOff:]); got != crackedPotHandle {
			t.Errorf("binary Cracked Pot handle = 0x%08X, want 0x%08X", got, crackedPotHandle)
		}
		if got := binary.LittleEndian.Uint32(slot.Data[recOff+8:]); got != it.Index {
			t.Errorf("binary Cracked Pot index = %d, want %d (matches in-memory)", got, it.Index)
		}
		invStart := slot.MagicOffset + core.InvStartFromMagic
		if _, ok := quantityInRecords(slot.Data, invStart, core.CommonItemCount, crackedPotHandle); ok {
			t.Error("Cracked Pot found in CommonItems, want none")
		}

		flags := slot.Data[slot.EventFlagsOffset:]
		if set, err := db.GetEventFlag(flags, data.ContainerPickupFlags[data.CrackedPotKeyItemID][0]); err != nil || !set {
			t.Errorf("first pickup flag set=%v err=%v, want set", set, err)
		}
	})

	// 3. Full KeyItems + rollback: with every KeyItems row occupied the container
	// upsert fails; the public method returns that error, the whole slot rolls
	// back byte-identical, no pickup flag is set, and neither Fire Pot nor
	// Cracked Pot leaks into CommonItems.
	t.Run("FullKeyItemsRollback", func(t *testing.T) {
		app := remembranceGameLimitsFixture()
		// Filler handles sit in an empty goods range (0xB0001xxx) so none resolves
		// to Fire Pot (0xB000012C) or Cracked Pot (0xB000251C); each index is unique.
		for i := 0; i < core.KeyItemCount; i++ {
			addKeyItemFixtureRow(t, app, i, 0xB0001000+uint32(i), 2000+uint32(i), 1)
		}
		withContainerEventFlags(app)
		slot := &app.save.Slots[0]

		before := append([]byte(nil), slot.Data...)

		_, err := app.AddItemsToCharacter(0, []uint32{firePotID}, 0, 0, 0, 0, 1, 0)
		if err == nil {
			t.Fatal("AddItemsToCharacter into full KeyItems: want error, got nil")
		}

		if !bytes.Equal(slot.Data, before) {
			t.Error("slot.Data not byte-identical after rollback")
		}
		flags := slot.Data[slot.EventFlagsOffset:]
		if set, err := db.GetEventFlag(flags, data.ContainerPickupFlags[data.CrackedPotKeyItemID][0]); err != nil || set {
			t.Errorf("first pickup flag set=%v err=%v, want unset", set, err)
		}
		invStart := slot.MagicOffset + core.InvStartFromMagic
		if _, ok := quantityInRecords(slot.Data, invStart, core.CommonItemCount, firePotHandle); ok {
			t.Error("Fire Pot leaked into CommonItems after rollback")
		}
		if _, ok := quantityInRecords(slot.Data, invStart, core.CommonItemCount, crackedPotHandle); ok {
			t.Error("Cracked Pot leaked into CommonItems after rollback")
		}
	})
}

// TestContainerStorageAdd: adding a pot/perfume to Storage must still guarantee a
// container in Key Items, but only the minimal one — Storage consumes no per-unit
// containers, so raising the Storage stack never raises the container past 1.
func TestContainerStorageAdd(t *testing.T) {
	app := remembranceGameLimitsFixture()
	slot := &app.save.Slots[0]
	slot.Inventory.KeyItems = make([]core.InventoryItem, core.KeyItemCount)
	withContainerEventFlags(app)

	// Spark Aromatic added to Storage only (invQty 0, storageQty 2).
	if _, err := app.AddItemsToCharacter(0, []uint32{sparkAromaticID}, 0, 0, 0, 0, 0, 2); err != nil {
		t.Fatalf("storage add: %v", err)
	}
	row, qty := findKeyItem(slot, perfumeBottleHandle)
	if row < 0 {
		t.Fatal("Perfume Bottle not created in KeyItems from a Storage-only add")
	}
	if qty != 1 {
		t.Errorf("Perfume Bottle qty = %d, want 1 (Storage consumes no per-unit container)", qty)
	}
	invStart := slot.MagicOffset + core.InvStartFromMagic
	if _, ok := quantityInRecords(slot.Data, invStart, core.CommonItemCount, perfumeBottleHandle); ok {
		t.Error("Perfume Bottle found in Inventory CommonItems, want KeyItems only")
	}

	// A larger Storage stack must not raise the bottle above the minimal 1.
	if _, err := app.AddItemsToCharacter(0, []uint32{sparkAromaticID}, 0, 0, 0, 0, 0, 5); err != nil {
		t.Fatalf("second storage add: %v", err)
	}
	if _, qty := findKeyItem(slot, perfumeBottleHandle); qty != 1 {
		t.Errorf("Perfume Bottle qty after larger Storage add = %d, want unchanged 1", qty)
	}

	// The minimal container sets exactly its first pickup flag; the second stays
	// unset because Storage never raises the bottle past 1.
	pickup := data.ContainerPickupFlags[data.PerfumeBottleKeyItemID]
	flags := slot.Data[slot.EventFlagsOffset:]
	if set, err := db.GetEventFlag(flags, pickup[0]); err != nil || !set {
		t.Errorf("first pickup flag set=%v err=%v, want set", set, err)
	}
	if set, err := db.GetEventFlag(flags, pickup[1]); err != nil || set {
		t.Errorf("second pickup flag set=%v err=%v, want unset", set, err)
	}
}

// containerEditFixture builds a slot holding one Inventory common item (a
// pot/perfume) and its Key Items container — both in memory and binary — plus an
// event-flags region. It mirrors the state the Inventory tab edits: GetCharacter →
// change char.inventory → SaveCharacter.
func containerEditFixture(t *testing.T, itemHandle, itemID, itemQty, containerHandle, containerQty uint32) *App {
	t.Helper()
	app := remembranceGameLimitsFixture()
	slot := &app.save.Slots[0]
	invStart := slot.MagicOffset + core.InvStartFromMagic
	keyStart := invStart + core.CommonItemCount*core.InvRecordLen + core.InvKeyCountHeader

	slot.GaMap[itemHandle] = itemID
	binary.LittleEndian.PutUint32(slot.Data[invStart:], itemHandle)
	binary.LittleEndian.PutUint32(slot.Data[invStart+4:], itemQty)
	binary.LittleEndian.PutUint32(slot.Data[invStart+8:], 900)
	slot.Inventory.CommonItems[0] = core.InventoryItem{GaItemHandle: itemHandle, Quantity: itemQty, Index: 900}

	slot.Inventory.KeyItems = make([]core.InventoryItem, core.KeyItemCount)
	binary.LittleEndian.PutUint32(slot.Data[keyStart:], containerHandle)
	binary.LittleEndian.PutUint32(slot.Data[keyStart+4:], containerQty)
	binary.LittleEndian.PutUint32(slot.Data[keyStart+8:], 901)
	slot.Inventory.KeyItems[0] = core.InventoryItem{GaItemHandle: containerHandle, Quantity: containerQty, Index: 901}

	withContainerEventFlags(app)
	return app
}

// assertContainerSaved checks the container reached wantQty in memory and binary,
// never leaked into CommonItems, and set its first wantQty pickup flags.
func assertContainerSaved(t *testing.T, slot *core.SaveSlot, containerHandle, containerID uint32, wantQty int) {
	t.Helper()
	row, qty := findKeyItem(slot, containerHandle)
	if row < 0 {
		t.Fatalf("container 0x%08X missing from KeyItems after save", containerHandle)
	}
	if int(qty) != wantQty {
		t.Errorf("in-memory container qty = %d, want %d", qty, wantQty)
	}
	if got := keyItemBinQty(slot, row); int(got) != wantQty {
		t.Errorf("binary container qty = %d, want %d", got, wantQty)
	}
	invStart := slot.MagicOffset + core.InvStartFromMagic
	if _, ok := quantityInRecords(slot.Data, invStart, core.CommonItemCount, containerHandle); ok {
		t.Error("container leaked into Inventory CommonItems, want KeyItems only")
	}
	flags := slot.Data[slot.EventFlagsOffset:]
	for i := 0; i < wantQty && i < len(data.ContainerPickupFlags[containerID]); i++ {
		if set, err := db.GetEventFlag(flags, data.ContainerPickupFlags[containerID][i]); err != nil || !set {
			t.Errorf("pickup flag[%d] set=%v err=%v, want set", i, set, err)
		}
	}
}

// TestContainerSaveCharacterHeftyPot: editing Hefty Rock Pot 1→2 in the
// CharacterViewModel and committing via SaveCharacter (exactly what the Inventory
// tab does) must raise Hefty Cracked Pot 1→2, in memory and binary, plus flags.
func TestContainerSaveCharacterHeftyPot(t *testing.T) {
	const (
		heftyRockPotHandle    = uint32(0xB01E85B6)
		heftyRockPotID        = uint32(0x401E85B6)
		heftyCrackedPotHandle = uint32(0xB01EA99C)
	)
	app := containerEditFixture(t, heftyRockPotHandle, heftyRockPotID, 1, heftyCrackedPotHandle, 1)
	slot := &app.save.Slots[0]

	// Real UI path: GetCharacter → bump the pot in the returned VM → SaveCharacter.
	charVM, err := app.GetCharacter(0)
	if err != nil {
		t.Fatalf("GetCharacter: %v", err)
	}
	bumped := false
	for i := range charVM.Inventory {
		if charVM.Inventory[i].Handle == heftyRockPotHandle {
			charVM.Inventory[i].Quantity = 2
			bumped = true
			break
		}
	}
	if !bumped {
		t.Fatalf("Hefty Rock Pot (0x%08X) absent from GetCharacter VM", heftyRockPotHandle)
	}
	if err := app.SaveCharacter(0, *charVM); err != nil {
		t.Fatalf("SaveCharacter: %v", err)
	}
	assertContainerSaved(t, slot, heftyCrackedPotHandle, data.HeftyCrackedPotKeyItemID, 2)
}

// TestContainerSaveCharacterPerfume: Uplifting Aromatic 1→2 via SaveCharacter must
// raise Perfume Bottle 1→2, in memory and binary, plus flags.
func TestContainerSaveCharacterPerfume(t *testing.T) {
	app := containerEditFixture(t, upliftingHandle, upliftingAromaticID, 1, perfumeBottleHandle, 1)
	slot := &app.save.Slots[0]

	charVM, err := app.GetCharacter(0)
	if err != nil {
		t.Fatalf("GetCharacter: %v", err)
	}
	bumped := false
	for i := range charVM.Inventory {
		if charVM.Inventory[i].Handle == upliftingHandle {
			charVM.Inventory[i].Quantity = 2
			bumped = true
			break
		}
	}
	if !bumped {
		t.Fatalf("Uplifting Aromatic (0x%08X) absent from GetCharacter VM", upliftingHandle)
	}
	if err := app.SaveCharacter(0, *charVM); err != nil {
		t.Fatalf("SaveCharacter: %v", err)
	}
	assertContainerSaved(t, slot, perfumeBottleHandle, data.PerfumeBottleKeyItemID, 2)
}

// TestContainerSaveCharacterFullKeyItemsRollback: raising a pot via SaveCharacter
// when Key Items is full and the required container is missing must fail, leave
// slot.Data byte-identical, create no container and set no pickup flag.
func TestContainerSaveCharacterFullKeyItemsRollback(t *testing.T) {
	app := remembranceGameLimitsFixture()
	slot := &app.save.Slots[0]
	invStart := slot.MagicOffset + core.InvStartFromMagic

	// One Fire Pot in Inventory (memory + binary), qty 1.
	slot.GaMap[firePotHandle] = firePotID
	binary.LittleEndian.PutUint32(slot.Data[invStart:], firePotHandle)
	binary.LittleEndian.PutUint32(slot.Data[invStart+4:], 1)
	binary.LittleEndian.PutUint32(slot.Data[invStart+8:], 900)
	slot.Inventory.CommonItems[0] = core.InventoryItem{GaItemHandle: firePotHandle, Quantity: 1, Index: 900}

	// Fill every KeyItems row with filler handles — no room for the Cracked Pot.
	for i := 0; i < core.KeyItemCount; i++ {
		addKeyItemFixtureRow(t, app, i, 0xB0001000+uint32(i), 2000+uint32(i), 1)
	}
	withContainerEventFlags(app)

	before := append([]byte(nil), slot.Data...)

	charVM, err := app.GetCharacter(0)
	if err != nil {
		t.Fatalf("GetCharacter: %v", err)
	}
	bumped := false
	for i := range charVM.Inventory {
		if charVM.Inventory[i].Handle == firePotHandle {
			charVM.Inventory[i].Quantity = 2
			bumped = true
			break
		}
	}
	if !bumped {
		t.Fatalf("Fire Pot (0x%08X) absent from GetCharacter VM", firePotHandle)
	}

	if err := app.SaveCharacter(0, *charVM); err == nil {
		t.Fatal("SaveCharacter into full KeyItems: want error, got nil")
	}

	if !bytes.Equal(slot.Data, before) {
		t.Error("slot.Data not byte-identical after rollback")
	}
	if row, _ := findKeyItem(slot, crackedPotHandle); row >= 0 {
		t.Error("Cracked Pot created despite rollback")
	}
	flags := slot.Data[slot.EventFlagsOffset:]
	if set, err := db.GetEventFlag(flags, data.ContainerPickupFlags[data.CrackedPotKeyItemID][0]); err != nil || set {
		t.Errorf("first pickup flag set=%v err=%v, want unset", set, err)
	}
}
