package vm

import (
	"encoding/json"
	"testing"
)

func TestVMToPresetRoundTrip(t *testing.T) {
	vm := &CharacterViewModel{
		Name:                "TestChar",
		Level:               150,
		Souls:               999999,
		Class:               0,
		ClassName:           "Vagabond",
		Vigor:               60,
		Mind:                25,
		Endurance:           40,
		Strength:            50,
		Dexterity:           30,
		Intelligence:        9,
		Faith:               9,
		Arcane:              7,
		TalismanSlots:       3,
		ClearCount:          2,
		ScadutreeBlessing:   20,
		ShadowRealmBlessing: 13,
		Inventory: []ItemViewModel{
			{Handle: 0x80000001, ID: 0x003D0C39, BaseID: 0x003D0900, Name: "Uchigatana", Quantity: 1, CurrentUpgrade: 25, MaxUpgrade: 25},
			{Handle: 0xB0002345, ID: 0x40002345, BaseID: 0x40002345, Name: "Golden Rune [1]", Quantity: 5, MaxUpgrade: 0},
		},
		Storage: []ItemViewModel{
			{Handle: 0xA0000010, ID: 0x20000010, BaseID: 0x20000010, Name: "Crimson Amber Medallion", Quantity: 1, MaxUpgrade: 0},
		},
	}

	preset := VMToPreset(vm, "0.8.0")

	if preset.FormatVersion != PresetFormatVersion {
		t.Errorf("FormatVersion = %d, want %d", preset.FormatVersion, PresetFormatVersion)
	}
	if preset.AppVersion != "0.8.0" {
		t.Errorf("AppVersion = %q, want %q", preset.AppVersion, "0.8.0")
	}

	c := preset.Character
	if c.Name != "TestChar" {
		t.Errorf("Name = %q, want %q", c.Name, "TestChar")
	}
	if c.Level != 150 {
		t.Errorf("Level = %d, want 150", c.Level)
	}
	if c.Vigor != 60 {
		t.Errorf("Vigor = %d, want 60", c.Vigor)
	}

	if len(preset.Inventory) != 2 {
		t.Fatalf("Inventory len = %d, want 2", len(preset.Inventory))
	}
	uchigatana := preset.Inventory[0]
	if uchigatana.BaseID != 0x003D0900 {
		t.Errorf("Uchigatana BaseID = 0x%08X, want 0x003D0900", uchigatana.BaseID)
	}
	if uchigatana.CurrentUpgrade != 25 {
		t.Errorf("Uchigatana upgrade = %d, want 25", uchigatana.CurrentUpgrade)
	}
	if uchigatana.InfuseOffset != 800 {
		t.Errorf("Uchigatana infuse = %d, want 800 (Magic)", uchigatana.InfuseOffset)
	}

	goldenRune := preset.Inventory[1]
	if goldenRune.InfuseOffset != 0 {
		t.Errorf("Golden Rune infuse = %d, want 0", goldenRune.InfuseOffset)
	}
	if goldenRune.Quantity != 5 {
		t.Errorf("Golden Rune qty = %d, want 5", goldenRune.Quantity)
	}

	if len(preset.Storage) != 1 {
		t.Fatalf("Storage len = %d, want 1", len(preset.Storage))
	}

	// Round-trip: Preset → VM
	backVM := PresetToVM(preset)
	if backVM.Name != vm.Name {
		t.Errorf("Roundtrip Name = %q, want %q", backVM.Name, vm.Name)
	}
	if backVM.Level != vm.Level {
		t.Errorf("Roundtrip Level = %d, want %d", backVM.Level, vm.Level)
	}
	if backVM.Vigor != vm.Vigor {
		t.Errorf("Roundtrip Vigor = %d, want %d", backVM.Vigor, vm.Vigor)
	}
}

func TestPresetItemToFinalID(t *testing.T) {
	tests := []struct {
		name     string
		item     PresetItem
		expected uint32
	}{
		{
			name:     "consumable no upgrade",
			item:     PresetItem{BaseID: 0x4000012C, CurrentUpgrade: 0, InfuseOffset: 0},
			expected: 0x4000012C,
		},
		{
			name:     "talisman no upgrade",
			item:     PresetItem{BaseID: 0x20000010, CurrentUpgrade: 0, InfuseOffset: 0},
			expected: 0x20000010,
		},
		{
			name:     "non-infusable item ignores infuse",
			item:     PresetItem{BaseID: 0x40002345, CurrentUpgrade: 0, InfuseOffset: 800},
			expected: 0x40002345,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PresetItemToFinalID(tt.item)
			if got != tt.expected {
				t.Errorf("PresetItemToFinalID() = 0x%08X, want 0x%08X", got, tt.expected)
			}
		})
	}
}

func TestValidatePreset_Valid(t *testing.T) {
	preset := &CharacterPreset{
		FormatVersion: PresetFormatVersion,
		Character: CharacterPresetCore{
			Class: 0, // Vagabond
			Vigor: 60, Mind: 25, Endurance: 40, Strength: 50,
			Dexterity: 30, Intelligence: 9, Faith: 9, Arcane: 7,
		},
	}
	warnings := ValidatePreset(preset)
	if len(warnings) > 0 {
		t.Errorf("Expected no warnings for valid preset, got: %v", warnings)
	}
}

func TestValidatePreset_UnknownBaseID(t *testing.T) {
	preset := &CharacterPreset{
		FormatVersion: PresetFormatVersion,
		Character:     CharacterPresetCore{Class: 0, Vigor: 15, Mind: 10, Endurance: 11, Strength: 14, Dexterity: 13, Intelligence: 9, Faith: 9, Arcane: 7},
		Inventory: []PresetItem{
			{BaseID: 0xDEADBEEF, Name: "Bogus", Quantity: 1},
		},
	}
	warnings := ValidatePreset(preset)
	found := false
	for _, w := range warnings {
		if containsString(w, "unknown BaseID") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected warning about unknown BaseID, got: %v", warnings)
	}
}

func TestValidatePreset_StatBelowClassBase(t *testing.T) {
	preset := &CharacterPreset{
		FormatVersion: PresetFormatVersion,
		Character: CharacterPresetCore{
			Class: 0, // Vagabond: Vigor base = 15
			Vigor: 5, Mind: 10, Endurance: 11, Strength: 14,
			Dexterity: 13, Intelligence: 9, Faith: 9, Arcane: 7,
		},
	}
	warnings := ValidatePreset(preset)
	found := false
	for _, w := range warnings {
		if containsString(w, "Vigor") && containsString(w, "below") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected warning about Vigor below class base, got: %v", warnings)
	}
}

func TestValidatePreset_InvalidFormatVersion(t *testing.T) {
	preset := &CharacterPreset{
		FormatVersion: 99,
		Character:     CharacterPresetCore{Class: 9, Vigor: 10, Mind: 10, Endurance: 10, Strength: 10, Dexterity: 10, Intelligence: 10, Faith: 10, Arcane: 10},
	}
	warnings := ValidatePreset(preset)
	found := false
	for _, w := range warnings {
		if containsString(w, "format version") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected warning about format version, got: %v", warnings)
	}
}

func TestJSONSerialization(t *testing.T) {
	preset := &CharacterPreset{
		FormatVersion: PresetFormatVersion,
		ExportedAt:    "2026-05-04T12:00:00Z",
		AppVersion:    "0.8.0",
		Character: CharacterPresetCore{
			Name: "Test", Class: 0, ClassName: "Vagabond",
			Level: 10, Souls: 100,
			Vigor: 15, Mind: 10, Endurance: 11, Strength: 14,
			Dexterity: 13, Intelligence: 9, Faith: 9, Arcane: 7,
		},
		Inventory: []PresetItem{
			{BaseID: 0x4000012C, Name: "Fire Pot", Quantity: 5},
		},
		Storage: []PresetItem{},
	}

	data, err := json.MarshalIndent(preset, "", "  ")
	if err != nil {
		t.Fatalf("JSON marshal failed: %v", err)
	}

	var roundtrip CharacterPreset
	if err := json.Unmarshal(data, &roundtrip); err != nil {
		t.Fatalf("JSON unmarshal failed: %v", err)
	}

	if roundtrip.FormatVersion != preset.FormatVersion {
		t.Errorf("FormatVersion mismatch after roundtrip")
	}
	if roundtrip.Character.Name != preset.Character.Name {
		t.Errorf("Character.Name mismatch after roundtrip")
	}
	if len(roundtrip.Inventory) != 1 {
		t.Errorf("Inventory len = %d after roundtrip, want 1", len(roundtrip.Inventory))
	}
	if roundtrip.Inventory[0].BaseID != 0x4000012C {
		t.Errorf("Inventory[0].BaseID = 0x%08X, want 0x4000012C", roundtrip.Inventory[0].BaseID)
	}

	// Verify omitempty: InfuseOffset=0 should not appear
	if containsString(string(data), `"infuse"`) {
		t.Error("InfuseOffset=0 should be omitted from JSON (omitempty)")
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
