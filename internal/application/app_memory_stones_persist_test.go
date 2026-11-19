package application

import (
	"encoding/binary"
	"strconv"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
	"github.com/oisis/EldenRing-SaveForge/backend/vm"
)

// seedCommonMemoryStones writes a Memory Stones record into Common Items row 0,
// both in the parsed slice and in the binary slot.Data, as a parsed save has it.
func seedCommonMemoryStones(slot *core.SaveSlot, qty uint32) int {
	off := slot.MagicOffset + core.InvStartFromMagic
	binary.LittleEndian.PutUint32(slot.Data[off:], memoryStonesHandle)
	binary.LittleEndian.PutUint32(slot.Data[off+4:], qty)
	slot.Inventory.CommonItems[0] = core.InventoryItem{GaItemHandle: memoryStonesHandle, Quantity: qty}
	return off
}

// finishedMemoryStones returns the "after" value of the finished memory_stones
// journal record.
func finishedMemoryStones(t *testing.T, app *App) string {
	t.Helper()
	phases := memoryStonesPhases(characterRecords(app.journal.Tail()), "memory_stones")
	if len(phases) != 3 {
		t.Fatalf("memory_stones: got %d records, want 3", len(phases))
	}
	return operationField(phases[2], "after")
}

func TestSaveCharacterMemoryStonesPersistsQuantityInSlotData(t *testing.T) {
	tests := []struct {
		name      string
		requested uint32
		want      uint32
	}{
		{"raise", 8, 8},
		{"clamp above max", 12, 8},
		{"zero keeps record with qty 0", 0, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			app := remembranceGameLimitsFixture()
			slot := &app.save.Slots[0]
			off := seedCommonMemoryStones(slot, 3)
			enableDebugJournal(t, app)

			charVM, err := app.GetCharacter(0)
			if err != nil {
				t.Fatalf("GetCharacter: %v", err)
			}
			charVM.MemoryStones = tc.requested
			if err := app.SaveCharacter(0, *charVM); err != nil {
				t.Fatalf("SaveCharacter: %v", err)
			}

			if got := binary.LittleEndian.Uint32(slot.Data[off+4:]); got != tc.want {
				t.Errorf("slot.Data quantity = %d, want %d", got, tc.want)
			}
			if got := slot.Inventory.CommonItems[0].Quantity; got != tc.want {
				t.Errorf("slice quantity = %d, want %d", got, tc.want)
			}
			if got := finishedMemoryStones(t, app); got != strconv.Itoa(int(tc.want)) {
				t.Errorf("journal finished memory_stones = %q, want %d (value in Data)", got, tc.want)
			}
		})
	}
}

func TestSaveCharacterMemoryStonesSurvivesWriteReadRoundTrip(t *testing.T) {
	tests := []struct {
		name      string
		requested uint32
		want      uint32
	}{
		{"raise", 8, 8},
		{"clamp above max", 12, 8},
		{"zero", 0, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			app := diagnosticGaItemApp(t)
			slot := &app.save.Slots[0]
			slot.Inventory.CommonItems = make([]core.InventoryItem, core.CommonItemCount)
			seedCommonMemoryStones(slot, 3)

			charVM, err := app.GetCharacter(0)
			if err != nil {
				t.Fatalf("GetCharacter: %v", err)
			}
			charVM.MemoryStones = tc.requested
			if err := app.SaveCharacter(0, *charVM); err != nil {
				t.Fatalf("SaveCharacter: %v", err)
			}

			written := append([]byte(nil), slot.Write(string(core.PlatformPC))...)
			var reread core.SaveSlot
			if err := reread.Read(core.NewReader(written), string(core.PlatformPC)); err != nil {
				t.Fatalf("SaveSlot.Read: %v", err)
			}
			got, err := vm.MapParsedSlotToVM(&reread)
			if err != nil {
				t.Fatalf("MapParsedSlotToVM: %v", err)
			}
			if got.MemoryStones != tc.want {
				t.Errorf("reloaded VM MemoryStones = %d, want %d", got.MemoryStones, tc.want)
			}
		})
	}
}

// TestSaveCharacterMemoryStonesKeyItemsRecordUpdatedInPlace uses the real save
// shape: the stone record lives only in Key Items and goods have no GaMap entry.
func TestSaveCharacterMemoryStonesKeyItemsRecordUpdatedInPlace(t *testing.T) {
	tests := []struct {
		name      string
		requested uint32
		want      uint32
	}{
		{"raise", 8, 8},
		{"lower", 2, 2},
		{"clamp above max", 12, 8},
		{"zero keeps record with qty 0", 0, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			app := remembranceGameLimitsFixture()
			slot := &app.save.Slots[0]
			slot.Inventory.KeyItems = []core.InventoryItem{{GaItemHandle: memoryStonesHandle, Quantity: 5}}
			keyOff := slot.MagicOffset + core.InvStartFromMagic + core.CommonItemCount*core.InvRecordLen + core.InvKeyCountHeader
			binary.LittleEndian.PutUint32(slot.Data[keyOff:], memoryStonesHandle)
			binary.LittleEndian.PutUint32(slot.Data[keyOff+4:], 5)
			enableDebugJournal(t, app)

			charVM, err := app.GetCharacter(0)
			if err != nil {
				t.Fatalf("GetCharacter: %v", err)
			}
			if charVM.MemoryStones != 5 {
				t.Fatalf("seed: MemoryStones = %d, want 5", charVM.MemoryStones)
			}
			charVM.MemoryStones = tc.requested
			if err := app.SaveCharacter(0, *charVM); err != nil {
				t.Fatalf("SaveCharacter: %v", err)
			}

			if got := slot.Inventory.KeyItems[0].Quantity; got != tc.want {
				t.Errorf("Key Items slice quantity = %d, want %d", got, tc.want)
			}
			if got := binary.LittleEndian.Uint32(slot.Data[keyOff+4:]); got != tc.want {
				t.Errorf("Key Items slot.Data quantity = %d, want %d", got, tc.want)
			}
			if got := readMemoryStonesCommonQuantity(slot); got != memoryStonesAbsent {
				t.Errorf("Common Items quantity = %q, want %q (no duplicate record)", got, memoryStonesAbsent)
			}
			if got := finishedMemoryStones(t, app); got != strconv.Itoa(int(tc.want)) {
				t.Errorf("journal finished memory_stones = %q, want %d", got, tc.want)
			}
			if phases := memoryStonesPhases(characterRecords(app.journal.Tail()), "memory_stones_common_quantity"); len(phases) != 0 {
				t.Errorf("memory_stones_common_quantity: got %d records, want 0", len(phases))
			}
		})
	}
}

// TestApplyCharacterPresetMemoryStonesPersistsQuantity covers the preset path
// (ReplaceStats without ReplaceInventory), which applies MemoryStones through
// the same writer as SaveCharacter.
func TestApplyCharacterPresetMemoryStonesPersistsQuantity(t *testing.T) {
	const want = uint32(8)
	tests := []struct {
		name string
		seed func(slot *core.SaveSlot) (qtyOff int)
	}{
		{"common items record", func(slot *core.SaveSlot) int {
			slot.Inventory.CommonItems = make([]core.InventoryItem, core.CommonItemCount)
			return seedCommonMemoryStones(slot, 3) + 4
		}},
		{"key items record only", func(slot *core.SaveSlot) int {
			slot.Inventory.CommonItems = make([]core.InventoryItem, core.CommonItemCount)
			slot.Inventory.KeyItems = []core.InventoryItem{{GaItemHandle: memoryStonesHandle, Quantity: 5}}
			keyOff := slot.MagicOffset + core.InvStartFromMagic + core.CommonItemCount*core.InvRecordLen + core.InvKeyCountHeader
			binary.LittleEndian.PutUint32(slot.Data[keyOff:], memoryStonesHandle)
			binary.LittleEndian.PutUint32(slot.Data[keyOff+4:], 5)
			return keyOff + 4
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			app := diagnosticGaItemApp(t)
			slot := &app.save.Slots[0]
			slot.Player.CharacterName[0] = 'A'
			qtyOff := tc.seed(slot)

			preset := vm.CharacterPreset{FormatVersion: vm.PresetFormatVersion}
			preset.Character.MemoryStones = want
			if _, err := app.applyCharacterPresetLocked(0, preset, vm.ApplyOptions{ReplaceStats: true}); err != nil {
				t.Fatalf("applyCharacterPresetLocked: %v", err)
			}

			if got := binary.LittleEndian.Uint32(slot.Data[qtyOff:]); got != want {
				t.Errorf("slot.Data quantity = %d, want %d", got, want)
			}
			written := append([]byte(nil), slot.Write(string(core.PlatformPC))...)
			var reread core.SaveSlot
			if err := reread.Read(core.NewReader(written), string(core.PlatformPC)); err != nil {
				t.Fatalf("SaveSlot.Read: %v", err)
			}
			got, err := vm.MapParsedSlotToVM(&reread)
			if err != nil {
				t.Fatalf("MapParsedSlotToVM: %v", err)
			}
			if got.MemoryStones != want {
				t.Errorf("reloaded VM MemoryStones = %d, want %d", got.MemoryStones, want)
			}
		})
	}
}

// TestSaveCharacterMemoryStonesZeroesLegacyKeyItemsDuplicate covers a Common
// Items stone record plus a legacy Key Items duplicate (no GaMap entry): Common
// takes the desired count and the duplicate is zeroed, never removed.
func TestSaveCharacterMemoryStonesZeroesLegacyKeyItemsDuplicate(t *testing.T) {
	tests := []struct {
		name      string
		requested uint32
		want      uint32
	}{
		{"raise", 8, 8},
		{"lower", 2, 2},
		{"zero", 0, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			app := diagnosticGaItemApp(t)
			slot := &app.save.Slots[0]
			slot.Inventory.CommonItems = make([]core.InventoryItem, core.CommonItemCount)
			commonQtyOff := seedCommonMemoryStones(slot, 4) + 4
			slot.Inventory.KeyItems = []core.InventoryItem{{GaItemHandle: memoryStonesHandle, Quantity: 5}}
			keyOff := slot.MagicOffset + core.InvStartFromMagic + core.CommonItemCount*core.InvRecordLen + core.InvKeyCountHeader
			binary.LittleEndian.PutUint32(slot.Data[keyOff:], memoryStonesHandle)
			binary.LittleEndian.PutUint32(slot.Data[keyOff+4:], 5)

			charVM, err := app.GetCharacter(0)
			if err != nil {
				t.Fatalf("GetCharacter: %v", err)
			}
			charVM.MemoryStones = tc.requested
			if err := app.SaveCharacter(0, *charVM); err != nil {
				t.Fatalf("SaveCharacter: %v", err)
			}

			if got := binary.LittleEndian.Uint32(slot.Data[commonQtyOff:]); got != tc.want {
				t.Errorf("Common slot.Data quantity = %d, want %d", got, tc.want)
			}
			if got := slot.Inventory.CommonItems[0].Quantity; got != tc.want {
				t.Errorf("Common slice quantity = %d, want %d", got, tc.want)
			}
			if len(slot.Inventory.KeyItems) != 1 || slot.Inventory.KeyItems[0].Quantity != 0 {
				t.Errorf("Key Items = %+v, want the record kept with quantity 0", slot.Inventory.KeyItems)
			}
			if got := binary.LittleEndian.Uint32(slot.Data[keyOff+4:]); got != 0 {
				t.Errorf("Key Items slot.Data quantity = %d, want 0", got)
			}
			if got := memoryStonesEffective(slot); got != tc.want {
				t.Errorf("effective memory stones = %d, want %d", got, tc.want)
			}

			written := append([]byte(nil), slot.Write(string(core.PlatformPC))...)
			var reread core.SaveSlot
			if err := reread.Read(core.NewReader(written), string(core.PlatformPC)); err != nil {
				t.Fatalf("SaveSlot.Read: %v", err)
			}
			got, err := vm.MapParsedSlotToVM(&reread)
			if err != nil {
				t.Fatalf("MapParsedSlotToVM: %v", err)
			}
			if got.MemoryStones != tc.want {
				t.Errorf("reloaded VM MemoryStones = %d, want %d", got.MemoryStones, tc.want)
			}
		})
	}
}
