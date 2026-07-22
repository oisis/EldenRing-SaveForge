package main

import (
	"encoding/binary"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
	"github.com/oisis/EldenRing-SaveForge/backend/db"
	"github.com/oisis/EldenRing-SaveForge/backend/db/data"
)

// Confirmed against the real save tmp/save/ER0000-kro55.sl2: the game's
// containers (Cracked Pot, Ritual Pot, …) live in Inventory.CommonItems with a
// 0xB0… goods handle. These tests drive the public App.AddItemsToCharacter path
// so they cover the whole add + rollback flow, not just the upsert helper in
// isolation.
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

func findCommonItem(slot *core.SaveSlot, handle uint32) (int, uint32) {
	for i := range slot.Inventory.CommonItems {
		if slot.Inventory.CommonItems[i].GaItemHandle == handle {
			return i, slot.Inventory.CommonItems[i].Quantity
		}
	}
	return -1, 0
}

func addCommonItemFixtureRow(t *testing.T, app *App, row int, handle, index, qty uint32) {
	t.Helper()
	slot := &app.save.Slots[0]
	if row < 0 || row >= len(slot.Inventory.CommonItems) {
		t.Fatalf("common row %d out of range", row)
	}
	off := slot.MagicOffset + core.InvStartFromMagic + row*core.InvRecordLen
	binary.LittleEndian.PutUint32(slot.Data[off:], handle)
	binary.LittleEndian.PutUint32(slot.Data[off+4:], qty)
	binary.LittleEndian.PutUint32(slot.Data[off+8:], index)
	slot.Inventory.CommonItems[row] = core.InventoryItem{GaItemHandle: handle, Quantity: qty, Index: index}
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
	// 1. Regression case: a vanilla Cracked Pot in CommonItems is updated in
	// place when Fire Pots are added. No duplicate KeyItems record may appear.
	t.Run("ExistingCrackedPotBumped", func(t *testing.T) {
		app := remembranceGameLimitsFixture()
		addCommonItemFixtureRow(t, app, 0, crackedPotHandle, 2000, 1)
		withContainerEventFlags(app)
		slot := &app.save.Slots[0]

		res, err := app.AddItemsToCharacter(0, []uint32{firePotID}, 0, 0, 0, 0, 2, 0)
		if err != nil {
			t.Fatalf("AddItemsToCharacter: %v", err)
		}
		if res.Added != 1 {
			t.Fatalf("Added = %d, want 1", res.Added)
		}

		if got := slot.Inventory.CommonItems[0].Quantity; got != 2 {
			t.Errorf("in-memory Cracked Pot qty = %d, want 2", got)
		}
		invStart := slot.MagicOffset + core.InvStartFromMagic
		if got, ok := quantityInRecords(slot.Data, invStart, core.CommonItemCount, crackedPotHandle); !ok || got != 2 {
			t.Errorf("binary Cracked Pot qty = %d, want 2", got)
		}
		if row, _ := findKeyItem(slot, crackedPotHandle); row >= 0 {
			t.Error("Cracked Pot duplicated into KeyItems")
		}

		flags := slot.Data[slot.EventFlagsOffset:]
		for i := 0; i < 2; i++ {
			if set, err := db.GetEventFlag(flags, data.ContainerPickupFlags[data.CrackedPotKeyItemID][i]); err != nil || !set {
				t.Errorf("pickup flag[%d] set=%v err=%v, want set", i, set, err)
			}
		}
	})

	// 2. Saves created by versions 1.3.3–1.5.1 may have the container only in
	// KeyItems. Preserve that physical layout when raising the quantity instead
	// of creating yet another CommonItems record.
	t.Run("LegacyKeyItemsCrackedPotBumped", func(t *testing.T) {
		app := remembranceGameLimitsFixture()
		addKeyItemFixtureRow(t, app, 0, crackedPotHandle, 2000, 1)
		withContainerEventFlags(app)
		slot := &app.save.Slots[0]

		if _, err := app.AddItemsToCharacter(0, []uint32{firePotID}, 0, 0, 0, 0, 2, 0); err != nil {
			t.Fatalf("AddItemsToCharacter: %v", err)
		}
		if got := slot.Inventory.KeyItems[0].Quantity; got != 2 {
			t.Errorf("legacy KeyItems Cracked Pot qty = %d, want 2", got)
		}
		if row, _ := findCommonItem(slot, crackedPotHandle); row >= 0 {
			t.Error("legacy Cracked Pot duplicated into CommonItems")
		}
	})

	// 3. Missing container: adding one Fire Pot creates the Cracked Pot from
	// scratch in canonical CommonItems, with qty 1 and a fresh acquisition index.
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

		row, qty := findCommonItem(slot, crackedPotHandle)
		if row < 0 {
			t.Fatal("Cracked Pot not found in CommonItems")
		}
		if qty != 1 {
			t.Errorf("Cracked Pot qty = %d, want 1", qty)
		}
		// Fresh, safe acquisition index: at least the fixture's starting
		// NextAcquisitionSortId (1000), and the counter must have advanced past it.
		it := slot.Inventory.CommonItems[row]
		if it.Index < 1000 {
			t.Errorf("Cracked Pot acquisition index = %d, want >= 1000", it.Index)
		}
		if slot.Inventory.NextAcquisitionSortId <= it.Index {
			t.Errorf("NextAcquisitionSortId = %d, want > record index %d", slot.Inventory.NextAcquisitionSortId, it.Index)
		}
		invStart := slot.MagicOffset + core.InvStartFromMagic
		if got, ok := quantityInRecords(slot.Data, invStart, core.CommonItemCount, crackedPotHandle); !ok || got != 1 {
			t.Errorf("binary Cracked Pot qty = %d, want 1", got)
		}
		recOff := invStart + row*core.InvRecordLen
		if got := binary.LittleEndian.Uint32(slot.Data[recOff:]); got != crackedPotHandle {
			t.Errorf("binary Cracked Pot handle = 0x%08X, want 0x%08X", got, crackedPotHandle)
		}
		if got := binary.LittleEndian.Uint32(slot.Data[recOff+8:]); got != it.Index {
			t.Errorf("binary Cracked Pot index = %d, want %d (matches in-memory)", got, it.Index)
		}
		if keyRow, _ := findKeyItem(slot, crackedPotHandle); keyRow >= 0 {
			t.Error("Cracked Pot unexpectedly created in KeyItems")
		}

		flags := slot.Data[slot.EventFlagsOffset:]
		if set, err := db.GetEventFlag(flags, data.ContainerPickupFlags[data.CrackedPotKeyItemID][0]); err != nil || !set {
			t.Errorf("first pickup flag set=%v err=%v, want set", set, err)
		}
	})

	// 4. KeyItems capacity is irrelevant to a new canonical CommonItems
	// container. This guards against reintroducing the old routing regression.
	t.Run("FullKeyItemsDoesNotBlockCommonContainer", func(t *testing.T) {
		app := remembranceGameLimitsFixture()
		// Filler handles sit in an empty goods range (0xB0001xxx) so none resolves
		// to Fire Pot (0xB000012C) or Cracked Pot (0xB000251C); each index is unique.
		for i := 0; i < core.KeyItemCount; i++ {
			// Stride-2 indices → distinct Index>>1 buckets (native shape, not the
			// old stride-1 pollution the preflight now rejects).
			addKeyItemFixtureRow(t, app, i, 0xB0001000+uint32(i), 2000+uint32(2*i), 1)
		}
		withContainerEventFlags(app)
		slot := &app.save.Slots[0]

		if _, err := app.AddItemsToCharacter(0, []uint32{firePotID}, 0, 0, 0, 0, 1, 0); err != nil {
			t.Fatalf("AddItemsToCharacter with full KeyItems: %v", err)
		}
		flags := slot.Data[slot.EventFlagsOffset:]
		if set, err := db.GetEventFlag(flags, data.ContainerPickupFlags[data.CrackedPotKeyItemID][0]); err != nil || !set {
			t.Errorf("first pickup flag set=%v err=%v, want set", set, err)
		}
		invStart := slot.MagicOffset + core.InvStartFromMagic
		if _, ok := quantityInRecords(slot.Data, invStart, core.CommonItemCount, firePotHandle); !ok {
			t.Error("Fire Pot missing from CommonItems")
		}
		if got, ok := quantityInRecords(slot.Data, invStart, core.CommonItemCount, crackedPotHandle); !ok || got != 1 {
			t.Errorf("Cracked Pot in CommonItems = %d, want 1", got)
		}
	})
}

// TestContainerStorageAdd: adding a pot/perfume to Storage must still guarantee a
// container in CommonItems, but only the minimal one — Storage consumes no per-unit
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
	row, qty := findCommonItem(slot, perfumeBottleHandle)
	if row < 0 {
		t.Fatal("Perfume Bottle not created in CommonItems from a Storage-only add")
	}
	if qty != 1 {
		t.Errorf("Perfume Bottle qty = %d, want 1 (Storage consumes no per-unit container)", qty)
	}
	if keyRow, _ := findKeyItem(slot, perfumeBottleHandle); keyRow >= 0 {
		t.Error("Perfume Bottle unexpectedly created in KeyItems")
	}

	// A larger Storage stack must not raise the bottle above the minimal 1.
	if _, err := app.AddItemsToCharacter(0, []uint32{sparkAromaticID}, 0, 0, 0, 0, 0, 5); err != nil {
		t.Fatalf("second storage add: %v", err)
	}
	if _, qty := findCommonItem(slot, perfumeBottleHandle); qty != 1 {
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
// pot/perfume) and its canonical CommonItems container — both in memory and
// binary — plus an event-flags region. It mirrors the state the Inventory tab
// edits: GetCharacter → change char.inventory → SaveCharacter.
func containerEditFixture(t *testing.T, itemHandle, itemID, itemQty, containerHandle, containerQty uint32) *App {
	t.Helper()
	app := remembranceGameLimitsFixture()
	slot := &app.save.Slots[0]
	invStart := slot.MagicOffset + core.InvStartFromMagic
	slot.GaMap[itemHandle] = itemID
	binary.LittleEndian.PutUint32(slot.Data[invStart:], itemHandle)
	binary.LittleEndian.PutUint32(slot.Data[invStart+4:], itemQty)
	binary.LittleEndian.PutUint32(slot.Data[invStart+8:], 900)
	slot.Inventory.CommonItems[0] = core.InventoryItem{GaItemHandle: itemHandle, Quantity: itemQty, Index: 900}

	binary.LittleEndian.PutUint32(slot.Data[invStart+core.InvRecordLen:], containerHandle)
	binary.LittleEndian.PutUint32(slot.Data[invStart+core.InvRecordLen+4:], containerQty)
	binary.LittleEndian.PutUint32(slot.Data[invStart+core.InvRecordLen+8:], 901)
	slot.Inventory.CommonItems[1] = core.InventoryItem{GaItemHandle: containerHandle, Quantity: containerQty, Index: 901}

	withContainerEventFlags(app)
	return app
}

// assertContainerSaved checks the container reached wantQty in canonical
// CommonItems in memory and binary, never leaked into KeyItems, and set its first
// wantQty pickup flags.
func assertContainerSaved(t *testing.T, slot *core.SaveSlot, containerHandle, containerID uint32, wantQty int) {
	t.Helper()
	row, qty := findCommonItem(slot, containerHandle)
	if row < 0 {
		t.Fatalf("container 0x%08X missing from CommonItems after save", containerHandle)
	}
	if int(qty) != wantQty {
		t.Errorf("in-memory container qty = %d, want %d", qty, wantQty)
	}
	invStart := slot.MagicOffset + core.InvStartFromMagic
	if got, ok := quantityInRecords(slot.Data, invStart, core.CommonItemCount, containerHandle); !ok || int(got) != wantQty {
		t.Errorf("binary container qty = %d, want %d", got, wantQty)
	}
	if keyRow, _ := findKeyItem(slot, containerHandle); keyRow >= 0 {
		t.Error("container leaked into Inventory KeyItems")
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

// TestContainerSaveCharacterFullKeyItems: a full KeyItems section does not block
// SaveCharacter from creating the missing canonical CommonItems container.
func TestContainerSaveCharacterFullKeyItems(t *testing.T) {
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

	if err := app.SaveCharacter(0, *charVM); err != nil {
		t.Fatalf("SaveCharacter into full KeyItems: %v", err)
	}
	if row, _ := findKeyItem(slot, crackedPotHandle); row >= 0 {
		t.Error("Cracked Pot created in KeyItems")
	}
	if _, qty := findCommonItem(slot, crackedPotHandle); qty != 2 {
		t.Errorf("Cracked Pot CommonItems qty = %d, want 2", qty)
	}
	flags := slot.Data[slot.EventFlagsOffset:]
	if set, err := db.GetEventFlag(flags, data.ContainerPickupFlags[data.CrackedPotKeyItemID][0]); err != nil || !set {
		t.Errorf("first pickup flag set=%v err=%v, want set", set, err)
	}
}
