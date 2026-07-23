package main

import (
	"encoding/binary"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
)

// diagnosticGaItemApp builds a serializable native-layout GaItem table entirely
// in memory. It is shared by app-level diagnostic and workspace tests without
// depending on an on-disk save fixture.
func diagnosticGaItemApp(t *testing.T) *App {
	t.Helper()

	const (
		weaponHandle = uint32(core.ItemTypeWeapon | 0x00800002)
		armorHandle  = uint32(core.ItemTypeArmor | 0x00800004)
	)
	records := make([]core.GaItemFull, core.GaItemCountNew)
	records[2] = core.GaItemFull{
		Handle:          weaponHandle,
		ItemID:          0x000F4240,
		Unk2:            -7,
		Unk3:            9,
		AoWGaItemHandle: core.NoCustomAoWHandle,
		Unk5:            5,
	}
	records[4] = core.GaItemFull{
		Handle: armorHandle,
		ItemID: 0x10000001,
		Unk2:   11,
		Unk3:   12,
	}

	gaBytes := 0
	for i := range records {
		gaBytes += records[i].ByteSize()
	}
	data := make([]byte, core.SlotSize)
	binary.LittleEndian.PutUint32(data, core.GaItemVersionBreak+1)
	magicOffset := core.GaItemsStart + gaBytes + core.DynPlayerData - 1
	copy(data[magicOffset:], core.MagicPattern)
	pos := core.GaItemsStart
	for i := range records {
		pos += records[i].Serialize(data[pos:])
	}
	if pos != magicOffset-core.DynPlayerData+1 {
		t.Fatalf("GaItem fixture end=0x%X, want 0x%X", pos, magicOffset-core.DynPlayerData+1)
	}

	var slot core.SaveSlot
	if err := slot.Read(core.NewReader(data), string(core.PlatformPC)); err != nil {
		t.Fatalf("SaveSlot.Read: %v", err)
	}
	if free, err := core.NativeGaItemCapacity(&slot); err != nil || free == 0 {
		t.Fatalf("fixture native capacity=%d err=%v, want usable holes", free, err)
	}

	app := NewApp()
	app.save = &core.SaveFile{}
	app.save.Slots[0] = slot
	app.saveGeneration = 1
	return app
}
