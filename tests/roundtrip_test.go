package tests

import (
	"github.com/oisis/EldenRing-SaveForge/backend/core"
	"os"
	"testing"
)

// recalculatedRegions returns file-level byte ranges that are recalculated on save:
// - CSPlayerGameDataHash (0x80 bytes in each slot)
// - MD5 checksum prefix (16 bytes before each slot, PC only)
// - NextEquipIndex field for slots where mapInventory corrected a gap (external
//   editors may leave NextEquipIndex < NextAcquisitionSortId; we fix that on load)
// These must be excluded from byte-for-byte round-trip comparison.
func recalculatedRegions(save *core.SaveFile) [][2]int {
	var regions [][2]int
	headerSize := 0x70 // PS4
	if save.Platform == core.PlatformPC {
		headerSize = 0x300
	}
	slotRecordSize := core.SlotSize
	if save.Platform == core.PlatformPC {
		slotRecordSize += 16 // MD5 prefix
	}
	for i := 0; i < 10; i++ {
		recordStart := headerSize + i*slotRecordSize
		if save.Platform == core.PlatformPC {
			// Exclude MD5 prefix (recalculated because hash inside slot changed)
			regions = append(regions, [2]int{recordStart, recordStart + 16})
		}
		slotDataStart := recordStart
		if save.Platform == core.PlatformPC {
			slotDataStart += 16 // skip MD5 prefix
		}
		hashStart := slotDataStart + core.HashOffset
		hashEnd := hashStart + core.HashSize
		regions = append(regions, [2]int{hashStart, hashEnd})

		// Exclude NextEquipIndex for slots where mapInventory corrected a gap.
		// The 4-byte field is intentionally overwritten with the corrected value.
		if save.Slots[i].Version > 0 {
			off := save.Slots[i].Inventory.NextEquipIndexOff()
			if off > 0 {
				fileOff := slotDataStart + off
				regions = append(regions, [2]int{fileOff, fileOff + 4})
			}
		}
	}
	return regions
}

// bytesEqualExcluding compares two byte slices, ignoring specified regions.
// Returns true if equal (excluding the regions), and the first mismatch offset if not.
func bytesEqualExcluding(a, b []byte, exclude [][2]int) (bool, int) {
	if len(a) != len(b) {
		return false, min(len(a), len(b))
	}
	isExcluded := func(offset int) bool {
		for _, r := range exclude {
			if offset >= r[0] && offset < r[1] {
				return true
			}
		}
		return false
	}
	for i := range a {
		if isExcluded(i) {
			continue
		}
		if a[i] != b[i] {
			return false, i
		}
	}
	return true, -1
}

func TestRoundTripPS4(t *testing.T) {
	// Use the provided test save file
	path := "../tmp/save/oisis_pl-org.txt"
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Skip("Test save file not found in tmp/save/")
	}

	// 1. Load original
	save, err := core.LoadSave(path)
	if err != nil {
		t.Fatalf("Failed to load save: %v", err)
	}

	if save.Platform != core.PlatformPS {
		t.Fatalf("Expected PS4 platform, got %s", save.Platform)
	}

	// Verify known-good save produces no warnings on active slots
	for i := 0; i < 10; i++ {
		if save.ActiveSlots[i] && len(save.Slots[i].Warnings) > 0 {
			t.Errorf("Slot %d has unexpected warnings: %v", i, save.Slots[i].Warnings)
		}
	}

	// 2. Write to a temporary file
	tmpPath := "data/ps4/roundtrip_test.dat"
	os.MkdirAll("data/ps4", 0755)
	if err := save.SaveFile(tmpPath); err != nil {
		t.Fatalf("Failed to write save: %v", err)
	}
	defer os.Remove(tmpPath)

	// 3. Compare bytes excluding hash regions (recalculated on save)
	originalData, _ := os.ReadFile(path)
	newData, _ := os.ReadFile(tmpPath)
	hashRegions := recalculatedRegions(save)

	if equal, offset := bytesEqualExcluding(originalData, newData, hashRegions); !equal {
		t.Errorf("Byte mismatch! Round-trip failed to preserve data integrity.")
		if offset >= 0 && offset < len(originalData) && offset < len(newData) {
			t.Logf("First mismatch at offset 0x%x: expected %02x, got %02x",
				offset, originalData[offset], newData[offset])
		} else {
			t.Logf("Size mismatch: original=%d, new=%d", len(originalData), len(newData))
		}
	} else {
		t.Logf("Round-trip successful! Data integrity preserved (hash regions recalculated).")
	}
}

func TestRoundTripPC(t *testing.T) {
	path := "../tmp/save/ER0000.sl2"
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Skip("Test save file not found in tmp/save/")
	}

	// 1. Load original
	save, err := core.LoadSave(path)
	if err != nil {
		t.Fatalf("Failed to load save: %v", err)
	}

	if save.Platform != core.PlatformPC {
		t.Fatalf("Expected PC platform, got %s", save.Platform)
	}

	// Verify known-good save produces no warnings on active slots
	for i := 0; i < 10; i++ {
		if save.ActiveSlots[i] && len(save.Slots[i].Warnings) > 0 {
			t.Errorf("Slot %d has unexpected warnings: %v", i, save.Slots[i].Warnings)
		}
	}

	// 2. Write to a temporary file
	tmpPath := "data/pc/roundtrip_test.sl2"
	os.MkdirAll("data/pc", 0755)
	if err := save.SaveFile(tmpPath); err != nil {
		t.Fatalf("Failed to write save: %v", err)
	}
	defer os.Remove(tmpPath)

	// 3. Compare bytes excluding hash regions (recalculated on save)
	originalData, _ := os.ReadFile(path)
	newData, _ := os.ReadFile(tmpPath)
	hashRegions := recalculatedRegions(save)

	if equal, offset := bytesEqualExcluding(originalData, newData, hashRegions); !equal {
		t.Errorf("Byte mismatch! Round-trip failed to preserve data integrity.")
		if offset >= 0 && offset < len(originalData) && offset < len(newData) {
			t.Logf("First mismatch at offset 0x%x: expected %02x, got %02x",
				offset, originalData[offset], newData[offset])
		} else {
			t.Logf("Size mismatch: original=%d, new=%d", len(originalData), len(newData))
		}
	} else {
		t.Logf("Round-trip successful! Data integrity preserved (hash regions recalculated).")
	}
}

func TestConversionPS4ToPC(t *testing.T) {
	path := "../tmp/save/oisis_pl-org.txt"
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Skip("Test save file not found in tmp/save/")
	}

	// 1. Load original PS4
	ps4Save, err := core.LoadSave(path)
	if err != nil {
		t.Fatalf("Failed to load PS4 save: %v", err)
	}

	// 2. Convert to PC
	ps4Save.Platform = core.PlatformPC
	tmpPath := "data/pc/conversion_test.sl2"
	os.MkdirAll("data/pc", 0755)
	if err := ps4Save.SaveFile(tmpPath); err != nil {
		t.Fatalf("Failed to write as PC: %v", err)
	}
	defer os.Remove(tmpPath)

	// 3. Load the new PC save
	pcSave, err := core.LoadSave(tmpPath)
	if err != nil {
		t.Fatalf("Failed to load converted PC save: %v", err)
	}

	if pcSave.Platform != core.PlatformPC {
		t.Errorf("Expected PC platform after conversion, got %s", pcSave.Platform)
	}

	// 4. Verify data preservation (e.g., Name of first active slot)
	for i := 0; i < 10; i++ {
		if ps4Save.ActiveSlots[i] {
			ps4Name := core.UTF16ToString(ps4Save.Slots[i].Player.CharacterName[:])
			pcName := core.UTF16ToString(pcSave.Slots[i].Player.CharacterName[:])
			if ps4Name != pcName {
				t.Errorf("Name mismatch after conversion at slot %d: expected %s, got %s", i, ps4Name, pcName)
			}
			if ps4Save.Slots[i].Player.Level != pcSave.Slots[i].Player.Level {
				t.Errorf("Level mismatch after conversion at slot %d", i)
			}
		}
	}
}

func TestConversionPCToPS4(t *testing.T) {
	path := "../tmp/save/ER0000.sl2"
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Skip("Test save file not found in tmp/save/")
	}

	// 1. Load original PC
	pcSave, err := core.LoadSave(path)
	if err != nil {
		t.Fatalf("Failed to load PC save: %v", err)
	}

	// 2. Convert to PS4
	pcSave.Platform = core.PlatformPS
	tmpPath := "data/ps4/conversion_test.dat"
	os.MkdirAll("data/ps4", 0755)
	if err := pcSave.SaveFile(tmpPath); err != nil {
		t.Fatalf("Failed to write as PS4: %v", err)
	}
	defer os.Remove(tmpPath)

	// 3. Load the new PS4 save
	ps4Save, err := core.LoadSave(tmpPath)
	if err != nil {
		t.Fatalf("Failed to load converted PS4 save: %v", err)
	}

	if ps4Save.Platform != core.PlatformPS {
		t.Errorf("Expected PS4 platform after conversion, got %s", ps4Save.Platform)
	}

	// 4. Verify data preservation
	for i := 0; i < 10; i++ {
		if pcSave.ActiveSlots[i] {
			pcName := core.UTF16ToString(pcSave.Slots[i].Player.CharacterName[:])
			ps4Name := core.UTF16ToString(ps4Save.Slots[i].Player.CharacterName[:])
			if pcName != ps4Name {
				t.Errorf("Name mismatch after conversion at slot %d: expected %s, got %s", i, pcName, ps4Name)
			}
		}
	}
}
