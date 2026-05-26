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

func TestValidatePreset_SummoningPools_ValidIDs(t *testing.T) {
	preset := &CharacterPreset{
		FormatVersion: PresetFormatVersion,
		Character:     CharacterPresetCore{Class: 0, Vigor: 15, Mind: 10, Endurance: 11, Strength: 14, Dexterity: 13, Intelligence: 9, Faith: 9, Arcane: 7},
		World: &WorldPresetData{
			SummoningPools: []uint32{670101, 670130, 670800},
		},
	}
	warnings := ValidatePreset(preset)
	for _, w := range warnings {
		if containsString(w, "summoningPools") {
			t.Errorf("unexpected summoning pool warning for valid IDs: %q", w)
		}
	}
}

func TestValidatePreset_SummoningPools_OldPreDLCID(t *testing.T) {
	preset := &CharacterPreset{
		FormatVersion: PresetFormatVersion,
		Character:     CharacterPresetCore{Class: 0, Vigor: 15, Mind: 10, Endurance: 11, Strength: 14, Dexterity: 13, Intelligence: 9, Faith: 9, Arcane: 7},
		World: &WorldPresetData{
			SummoningPools: []uint32{10000040},
		},
	}
	warnings := ValidatePreset(preset)
	found := false
	for _, w := range warnings {
		if containsString(w, "pre-v1.12") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected pre-v1.12 warning for ID 10000040, got: %v", warnings)
	}
}

func TestValidatePreset_SummoningPools_OldPreDLCID_Large(t *testing.T) {
	preset := &CharacterPreset{
		FormatVersion: PresetFormatVersion,
		Character:     CharacterPresetCore{Class: 0, Vigor: 15, Mind: 10, Endurance: 11, Strength: 14, Dexterity: 13, Intelligence: 9, Faith: 9, Arcane: 7},
		World: &WorldPresetData{
			SummoningPools: []uint32{1035530040},
		},
	}
	warnings := ValidatePreset(preset)
	found := false
	for _, w := range warnings {
		if containsString(w, "pre-v1.12") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected pre-v1.12 warning for ID 1035530040, got: %v", warnings)
	}
}

func TestValidatePreset_SummoningPools_UnknownID(t *testing.T) {
	preset := &CharacterPreset{
		FormatVersion: PresetFormatVersion,
		Character:     CharacterPresetCore{Class: 0, Vigor: 15, Mind: 10, Endurance: 11, Strength: 14, Dexterity: 13, Intelligence: 9, Faith: 9, Arcane: 7},
		World: &WorldPresetData{
			SummoningPools: []uint32{999999},
		},
	}
	warnings := ValidatePreset(preset)
	found := false
	for _, w := range warnings {
		if containsString(w, "not found in current database") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected unknown-ID warning for ID 999999, got: %v", warnings)
	}
}

func TestValidatePreset_SummoningPools_NilWorld(t *testing.T) {
	preset := &CharacterPreset{
		FormatVersion: PresetFormatVersion,
		Character:     CharacterPresetCore{Class: 0, Vigor: 15, Mind: 10, Endurance: 11, Strength: 14, Dexterity: 13, Intelligence: 9, Faith: 9, Arcane: 7},
		World:         nil,
	}
	warnings := ValidatePreset(preset)
	for _, w := range warnings {
		if containsString(w, "summoningPools") {
			t.Errorf("unexpected summoning pool warning for nil World: %q", w)
		}
	}
}

func TestValidatePreset_Colosseums_FullSetLimgrave(t *testing.T) {
	// Full valid set: Limgrave activate (60360) + global flags (6080, 60100, 69480) in
	// Colosseums, MapPOI (62730) in MapFlags. Expect no colosseum-related warnings.
	preset := &CharacterPreset{
		FormatVersion: PresetFormatVersion,
		Character:     CharacterPresetCore{Class: 0, Vigor: 15, Mind: 10, Endurance: 11, Strength: 14, Dexterity: 13, Intelligence: 9, Faith: 9, Arcane: 7},
		World: &WorldPresetData{
			Colosseums: []uint32{60360, 6080, 60100, 69480},
			MapFlags:   []uint32{62730},
		},
	}
	warnings := ValidatePreset(preset)
	for _, w := range warnings {
		if containsString(w, "world.colosseums") {
			t.Errorf("unexpected colosseum warning for full Limgrave set: %q", w)
		}
	}
}

func TestValidatePreset_Colosseums_UnknownID(t *testing.T) {
	preset := &CharacterPreset{
		FormatVersion: PresetFormatVersion,
		Character:     CharacterPresetCore{Class: 0, Vigor: 15, Mind: 10, Endurance: 11, Strength: 14, Dexterity: 13, Intelligence: 9, Faith: 9, Arcane: 7},
		World:         &WorldPresetData{Colosseums: []uint32{99999}},
	}
	warnings := ValidatePreset(preset)
	found := false
	for _, w := range warnings {
		if containsString(w, "world.colosseums") && containsString(w, "not found in colosseum database") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected unknown-colosseum warning for ID 99999, got: %v", warnings)
	}
}

func TestValidatePreset_Colosseums_Duplicate(t *testing.T) {
	preset := &CharacterPreset{
		FormatVersion: PresetFormatVersion,
		Character:     CharacterPresetCore{Class: 0, Vigor: 15, Mind: 10, Endurance: 11, Strength: 14, Dexterity: 13, Intelligence: 9, Faith: 9, Arcane: 7},
		World: &WorldPresetData{
			Colosseums: []uint32{60360, 6080, 60100, 69480, 60360},
			MapFlags:   []uint32{62730},
		},
	}
	warnings := ValidatePreset(preset)
	found := false
	for _, w := range warnings {
		if containsString(w, "world.colosseums") && containsString(w, "duplicate") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected duplicate-colosseum warning for repeated ID 60360, got: %v", warnings)
	}
}

func TestValidatePreset_Colosseums_ActivateOnlyMissingCompanions(t *testing.T) {
	// Only activate flag — no MapPOI in MapFlags, no global flags.
	// Expect: MapPOI warning + 3 global flag warnings.
	preset := &CharacterPreset{
		FormatVersion: PresetFormatVersion,
		Character:     CharacterPresetCore{Class: 0, Vigor: 15, Mind: 10, Endurance: 11, Strength: 14, Dexterity: 13, Intelligence: 9, Faith: 9, Arcane: 7},
		World:         &WorldPresetData{Colosseums: []uint32{60360}},
	}
	warnings := ValidatePreset(preset)
	gotMapPOI := false
	globalCount := 0
	for _, w := range warnings {
		if containsString(w, "world.colosseums") && containsString(w, "missing companion map flag") {
			gotMapPOI = true
		}
		if containsString(w, "world.colosseums") && containsString(w, "global colosseum flag") {
			globalCount++
		}
	}
	if !gotMapPOI {
		t.Errorf("expected MapPOI missing warning for activate-only colosseum, got: %v", warnings)
	}
	if globalCount != 3 {
		t.Errorf("expected 3 global flag warnings, got %d: %v", globalCount, warnings)
	}
}

func TestValidatePreset_Colosseums_MissingGlobalFlags(t *testing.T) {
	// Activate + MapPOI present, but no global flags in Colosseums.
	preset := &CharacterPreset{
		FormatVersion: PresetFormatVersion,
		Character:     CharacterPresetCore{Class: 0, Vigor: 15, Mind: 10, Endurance: 11, Strength: 14, Dexterity: 13, Intelligence: 9, Faith: 9, Arcane: 7},
		World: &WorldPresetData{
			Colosseums: []uint32{60360},
			MapFlags:   []uint32{62730},
		},
	}
	warnings := ValidatePreset(preset)
	globalCount := 0
	for _, w := range warnings {
		if containsString(w, "world.colosseums") && containsString(w, "global colosseum flag") {
			globalCount++
		}
	}
	if globalCount != 3 {
		t.Errorf("expected 3 global flag warnings, got %d: %v", globalCount, warnings)
	}
}

func TestValidatePreset_Colosseums_NilWorld(t *testing.T) {
	preset := &CharacterPreset{
		FormatVersion: PresetFormatVersion,
		Character:     CharacterPresetCore{Class: 0, Vigor: 15, Mind: 10, Endurance: 11, Strength: 14, Dexterity: 13, Intelligence: 9, Faith: 9, Arcane: 7},
		World:         nil,
	}
	warnings := ValidatePreset(preset)
	for _, w := range warnings {
		if containsString(w, "world.colosseums") {
			t.Errorf("unexpected colosseum warning for nil World: %q", w)
		}
	}
}

func TestValidatePreset_Colosseums_EmptyList(t *testing.T) {
	preset := &CharacterPreset{
		FormatVersion: PresetFormatVersion,
		Character:     CharacterPresetCore{Class: 0, Vigor: 15, Mind: 10, Endurance: 11, Strength: 14, Dexterity: 13, Intelligence: 9, Faith: 9, Arcane: 7},
		World:         &WorldPresetData{Colosseums: []uint32{}},
	}
	warnings := ValidatePreset(preset)
	for _, w := range warnings {
		if containsString(w, "world.colosseums") {
			t.Errorf("unexpected colosseum warning for empty list: %q", w)
		}
	}
}

func TestValidatePreset_Colosseums_AllThree(t *testing.T) {
	// All three colosseums with full activate + global flags + all MapPOIs.
	// Caelid=60350/MapPOI=62720, Limgrave=60360/MapPOI=62730, Royal=60370/MapPOI=62740.
	preset := &CharacterPreset{
		FormatVersion: PresetFormatVersion,
		Character:     CharacterPresetCore{Class: 0, Vigor: 15, Mind: 10, Endurance: 11, Strength: 14, Dexterity: 13, Intelligence: 9, Faith: 9, Arcane: 7},
		World: &WorldPresetData{
			Colosseums: []uint32{60350, 60360, 60370, 6080, 60100, 69480},
			MapFlags:   []uint32{62720, 62730, 62740},
		},
	}
	warnings := ValidatePreset(preset)
	for _, w := range warnings {
		if containsString(w, "world.colosseums") {
			t.Errorf("unexpected colosseum warning for full 3-colosseum set: %q", w)
		}
	}
}

func TestValidatePreset_MapFlags_KnownVisible(t *testing.T) {
	// 62010 = "Limgrave, West" in MapVisible; 62000 = "Allow Map Display" in MapSystem
	preset := &CharacterPreset{
		FormatVersion: PresetFormatVersion,
		Character:     CharacterPresetCore{Class: 0, Vigor: 15, Mind: 10, Endurance: 11, Strength: 14, Dexterity: 13, Intelligence: 9, Faith: 9, Arcane: 7},
		World:         &WorldPresetData{MapFlags: []uint32{62000, 62010}},
	}
	warnings := ValidatePreset(preset)
	for _, w := range warnings {
		if containsString(w, "world.mapFlags") {
			t.Errorf("unexpected mapFlags warning for known 62xxx IDs: %q", w)
		}
	}
}

func TestValidatePreset_MapFlags_KnownSystem82(t *testing.T) {
	// 82001 = "Show Underground", 82002 = "Show Shadow Realm Map" in MapSystem
	preset := &CharacterPreset{
		FormatVersion: PresetFormatVersion,
		Character:     CharacterPresetCore{Class: 0, Vigor: 15, Mind: 10, Endurance: 11, Strength: 14, Dexterity: 13, Intelligence: 9, Faith: 9, Arcane: 7},
		World:         &WorldPresetData{MapFlags: []uint32{82001, 82002}},
	}
	warnings := ValidatePreset(preset)
	for _, w := range warnings {
		if containsString(w, "world.mapFlags") {
			t.Errorf("unexpected mapFlags warning for known 82xxx IDs: %q", w)
		}
	}
}

func TestValidatePreset_MapFlags_UnknownID(t *testing.T) {
	preset := &CharacterPreset{
		FormatVersion: PresetFormatVersion,
		Character:     CharacterPresetCore{Class: 0, Vigor: 15, Mind: 10, Endurance: 11, Strength: 14, Dexterity: 13, Intelligence: 9, Faith: 9, Arcane: 7},
		World:         &WorldPresetData{MapFlags: []uint32{99999}},
	}
	warnings := ValidatePreset(preset)
	found := false
	for _, w := range warnings {
		if containsString(w, "world.mapFlags") && containsString(w, "not found in map flag database") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected unknown-mapFlag warning for ID 99999, got: %v", warnings)
	}
}

func TestValidatePreset_MapFlags_Duplicate(t *testing.T) {
	preset := &CharacterPreset{
		FormatVersion: PresetFormatVersion,
		Character:     CharacterPresetCore{Class: 0, Vigor: 15, Mind: 10, Endurance: 11, Strength: 14, Dexterity: 13, Intelligence: 9, Faith: 9, Arcane: 7},
		World:         &WorldPresetData{MapFlags: []uint32{62010, 62000, 62010}},
	}
	warnings := ValidatePreset(preset)
	found := false
	for _, w := range warnings {
		if containsString(w, "world.mapFlags") && containsString(w, "duplicate") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected duplicate-mapFlag warning for repeated ID 62010, got: %v", warnings)
	}
}

func TestValidatePreset_MapFlags_NilWorld(t *testing.T) {
	preset := &CharacterPreset{
		FormatVersion: PresetFormatVersion,
		Character:     CharacterPresetCore{Class: 0, Vigor: 15, Mind: 10, Endurance: 11, Strength: 14, Dexterity: 13, Intelligence: 9, Faith: 9, Arcane: 7},
		World:         nil,
	}
	warnings := ValidatePreset(preset)
	for _, w := range warnings {
		if containsString(w, "world.mapFlags") {
			t.Errorf("unexpected mapFlags warning for nil World: %q", w)
		}
	}
}

func TestValidatePreset_MapFlags_EmptyList(t *testing.T) {
	preset := &CharacterPreset{
		FormatVersion: PresetFormatVersion,
		Character:     CharacterPresetCore{Class: 0, Vigor: 15, Mind: 10, Endurance: 11, Strength: 14, Dexterity: 13, Intelligence: 9, Faith: 9, Arcane: 7},
		World:         &WorldPresetData{MapFlags: []uint32{}},
	}
	warnings := ValidatePreset(preset)
	for _, w := range warnings {
		if containsString(w, "world.mapFlags") {
			t.Errorf("unexpected mapFlags warning for empty list: %q", w)
		}
	}
}

func TestValidatePreset_MapFlags_GraceIDMisplaced(t *testing.T) {
	// 76100 = Church of Elleh grace — wrong field, should be in World.Graces
	preset := &CharacterPreset{
		FormatVersion: PresetFormatVersion,
		Character:     CharacterPresetCore{Class: 0, Vigor: 15, Mind: 10, Endurance: 11, Strength: 14, Dexterity: 13, Intelligence: 9, Faith: 9, Arcane: 7},
		World:         &WorldPresetData{MapFlags: []uint32{76100}},
	}
	warnings := ValidatePreset(preset)
	found := false
	for _, w := range warnings {
		if containsString(w, "world.mapFlags") && containsString(w, "not found in map flag database") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected mapFlags warning for misplaced grace ID 76100, got: %v", warnings)
	}
}

func TestValidatePreset_MapFlags_SummoningPoolIDMisplaced(t *testing.T) {
	// 670100 = summoning pool ID — wrong field, should be in World.SummoningPools
	preset := &CharacterPreset{
		FormatVersion: PresetFormatVersion,
		Character:     CharacterPresetCore{Class: 0, Vigor: 15, Mind: 10, Endurance: 11, Strength: 14, Dexterity: 13, Intelligence: 9, Faith: 9, Arcane: 7},
		World:         &WorldPresetData{MapFlags: []uint32{670100}},
	}
	warnings := ValidatePreset(preset)
	found := false
	for _, w := range warnings {
		if containsString(w, "world.mapFlags") && containsString(w, "not found in map flag database") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected mapFlags warning for misplaced summoningPool ID 670100, got: %v", warnings)
	}
}

func TestValidatePreset_Graces_KnownBase(t *testing.T) {
	// 71000 = first grace in base game range
	preset := &CharacterPreset{
		FormatVersion: PresetFormatVersion,
		Character:     CharacterPresetCore{Class: 0, Vigor: 15, Mind: 10, Endurance: 11, Strength: 14, Dexterity: 13, Intelligence: 9, Faith: 9, Arcane: 7},
		World:         &WorldPresetData{Graces: []uint32{71000, 76100}},
	}
	warnings := ValidatePreset(preset)
	for _, w := range warnings {
		if containsString(w, "world.graces") {
			t.Errorf("unexpected grace warning for known base IDs: %q", w)
		}
	}
}

func TestValidatePreset_Graces_KnownDLC(t *testing.T) {
	// 72000 = first DLC grace (Belurat)
	preset := &CharacterPreset{
		FormatVersion: PresetFormatVersion,
		Character:     CharacterPresetCore{Class: 0, Vigor: 15, Mind: 10, Endurance: 11, Strength: 14, Dexterity: 13, Intelligence: 9, Faith: 9, Arcane: 7},
		World:         &WorldPresetData{Graces: []uint32{72000, 72001}},
	}
	warnings := ValidatePreset(preset)
	for _, w := range warnings {
		if containsString(w, "world.graces") {
			t.Errorf("unexpected grace warning for known DLC IDs: %q", w)
		}
	}
}

func TestValidatePreset_Graces_KnownCatacombWithDoorFlag(t *testing.T) {
	// 73000 = Tombsward Catacombs (Cat grace, has DoorFlag) — DoorFlag is NOT required in preset
	preset := &CharacterPreset{
		FormatVersion: PresetFormatVersion,
		Character:     CharacterPresetCore{Class: 0, Vigor: 15, Mind: 10, Endurance: 11, Strength: 14, Dexterity: 13, Intelligence: 9, Faith: 9, Arcane: 7},
		World:         &WorldPresetData{Graces: []uint32{73000}},
	}
	warnings := ValidatePreset(preset)
	for _, w := range warnings {
		if containsString(w, "world.graces") {
			t.Errorf("unexpected grace warning for catacomb grace with DoorFlag: %q", w)
		}
	}
}

func TestValidatePreset_Graces_UnknownID(t *testing.T) {
	preset := &CharacterPreset{
		FormatVersion: PresetFormatVersion,
		Character:     CharacterPresetCore{Class: 0, Vigor: 15, Mind: 10, Endurance: 11, Strength: 14, Dexterity: 13, Intelligence: 9, Faith: 9, Arcane: 7},
		World:         &WorldPresetData{Graces: []uint32{99999}},
	}
	warnings := ValidatePreset(preset)
	found := false
	for _, w := range warnings {
		if containsString(w, "world.graces") && containsString(w, "not found in grace database") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected unknown-grace warning for ID 99999, got: %v", warnings)
	}
}

func TestValidatePreset_Graces_Duplicate(t *testing.T) {
	preset := &CharacterPreset{
		FormatVersion: PresetFormatVersion,
		Character:     CharacterPresetCore{Class: 0, Vigor: 15, Mind: 10, Endurance: 11, Strength: 14, Dexterity: 13, Intelligence: 9, Faith: 9, Arcane: 7},
		World:         &WorldPresetData{Graces: []uint32{76100, 71000, 76100}},
	}
	warnings := ValidatePreset(preset)
	found := false
	for _, w := range warnings {
		if containsString(w, "world.graces") && containsString(w, "duplicate") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected duplicate-grace warning for repeated ID 76100, got: %v", warnings)
	}
}

func TestValidatePreset_Graces_NilWorld(t *testing.T) {
	preset := &CharacterPreset{
		FormatVersion: PresetFormatVersion,
		Character:     CharacterPresetCore{Class: 0, Vigor: 15, Mind: 10, Endurance: 11, Strength: 14, Dexterity: 13, Intelligence: 9, Faith: 9, Arcane: 7},
		World:         nil,
	}
	warnings := ValidatePreset(preset)
	for _, w := range warnings {
		if containsString(w, "world.graces") {
			t.Errorf("unexpected grace warning for nil World: %q", w)
		}
	}
}

func TestValidatePreset_Graces_EmptyList(t *testing.T) {
	preset := &CharacterPreset{
		FormatVersion: PresetFormatVersion,
		Character:     CharacterPresetCore{Class: 0, Vigor: 15, Mind: 10, Endurance: 11, Strength: 14, Dexterity: 13, Intelligence: 9, Faith: 9, Arcane: 7},
		World:         &WorldPresetData{Graces: []uint32{}},
	}
	warnings := ValidatePreset(preset)
	for _, w := range warnings {
		if containsString(w, "world.graces") {
			t.Errorf("unexpected grace warning for empty grace list: %q", w)
		}
	}
}

func TestValidatePreset_Regions_KnownOverworld(t *testing.T) {
	preset := &CharacterPreset{
		FormatVersion: PresetFormatVersion,
		Character:     CharacterPresetCore{Class: 0, Vigor: 15, Mind: 10, Endurance: 11, Strength: 14, Dexterity: 13, Intelligence: 9, Faith: 9, Arcane: 7},
		World: &WorldPresetData{
			Regions: []uint32{6100000, 6100001, 6200000},
		},
	}
	warnings := ValidatePreset(preset)
	for _, w := range warnings {
		if containsString(w, "world.regions") {
			t.Errorf("unexpected region warning for known overworld IDs: %q", w)
		}
	}
}

func TestValidatePreset_Regions_LegacyDungeonValid(t *testing.T) {
	preset := &CharacterPreset{
		FormatVersion: PresetFormatVersion,
		Character:     CharacterPresetCore{Class: 0, Vigor: 15, Mind: 10, Endurance: 11, Strength: 14, Dexterity: 13, Intelligence: 9, Faith: 9, Arcane: 7},
		World: &WorldPresetData{
			Regions: []uint32{1000000, 1000001, 1100000},
		},
	}
	warnings := ValidatePreset(preset)
	for _, w := range warnings {
		if containsString(w, "world.regions") {
			t.Errorf("unexpected region warning for known legacy dungeon IDs: %q", w)
		}
	}
}

func TestValidatePreset_Regions_DLCValid(t *testing.T) {
	preset := &CharacterPreset{
		FormatVersion: PresetFormatVersion,
		Character:     CharacterPresetCore{Class: 0, Vigor: 15, Mind: 10, Endurance: 11, Strength: 14, Dexterity: 13, Intelligence: 9, Faith: 9, Arcane: 7},
		World: &WorldPresetData{
			// Real Shadow of the Erdtree PlayRegion IDs: Scadu Altus + Gravesite
			// Plain (overworld) and Belurat (DLC legacy dungeon).
			Regions: []uint32{6900000, 6800000, 2000000},
		},
	}
	warnings := ValidatePreset(preset)
	for _, w := range warnings {
		if containsString(w, "world.regions") {
			t.Errorf("unexpected region warning for known DLC region IDs: %q", w)
		}
	}
}

func TestValidatePreset_Regions_UnknownID(t *testing.T) {
	preset := &CharacterPreset{
		FormatVersion: PresetFormatVersion,
		Character:     CharacterPresetCore{Class: 0, Vigor: 15, Mind: 10, Endurance: 11, Strength: 14, Dexterity: 13, Intelligence: 9, Faith: 9, Arcane: 7},
		World: &WorldPresetData{
			Regions: []uint32{9999999},
		},
	}
	warnings := ValidatePreset(preset)
	found := false
	for _, w := range warnings {
		if containsString(w, "world.regions") && containsString(w, "not found in region database") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected unknown-region warning for ID 9999999, got: %v", warnings)
	}
}

func TestValidatePreset_Regions_Duplicate(t *testing.T) {
	preset := &CharacterPreset{
		FormatVersion: PresetFormatVersion,
		Character:     CharacterPresetCore{Class: 0, Vigor: 15, Mind: 10, Endurance: 11, Strength: 14, Dexterity: 13, Intelligence: 9, Faith: 9, Arcane: 7},
		World: &WorldPresetData{
			Regions: []uint32{6100000, 6100001, 6100000},
		},
	}
	warnings := ValidatePreset(preset)
	found := false
	for _, w := range warnings {
		if containsString(w, "world.regions") && containsString(w, "duplicate") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected duplicate-region warning for repeated ID 6100000, got: %v", warnings)
	}
}

func TestValidatePreset_Regions_NilWorld(t *testing.T) {
	preset := &CharacterPreset{
		FormatVersion: PresetFormatVersion,
		Character:     CharacterPresetCore{Class: 0, Vigor: 15, Mind: 10, Endurance: 11, Strength: 14, Dexterity: 13, Intelligence: 9, Faith: 9, Arcane: 7},
		World:         nil,
	}
	warnings := ValidatePreset(preset)
	for _, w := range warnings {
		if containsString(w, "world.regions") {
			t.Errorf("unexpected region warning for nil World: %q", w)
		}
	}
}

func TestValidatePreset_Regions_EmptyList(t *testing.T) {
	preset := &CharacterPreset{
		FormatVersion: PresetFormatVersion,
		Character:     CharacterPresetCore{Class: 0, Vigor: 15, Mind: 10, Endurance: 11, Strength: 14, Dexterity: 13, Intelligence: 9, Faith: 9, Arcane: 7},
		World:         &WorldPresetData{Regions: []uint32{}},
	}
	warnings := ValidatePreset(preset)
	for _, w := range warnings {
		if containsString(w, "world.regions") {
			t.Errorf("unexpected region warning for empty region list: %q", w)
		}
	}
}

func TestValidatePreset_Bosses_Duplicate(t *testing.T) {
	preset := &CharacterPreset{
		FormatVersion: PresetFormatVersion,
		Character:     CharacterPresetCore{Class: 0, Vigor: 15, Mind: 10, Endurance: 11, Strength: 14, Dexterity: 13, Intelligence: 9, Faith: 9, Arcane: 7},
		World:         &WorldPresetData{Bosses: []uint32{111, 111}},
	}
	warnings := ValidatePreset(preset)
	found := false
	for _, w := range warnings {
		if containsString(w, "world.bosses") && containsString(w, "appears more than once") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected duplicate warning for world.bosses, got: %v", warnings)
	}
}

func TestValidatePreset_Bosses_VeryHighID(t *testing.T) {
	preset := &CharacterPreset{
		FormatVersion: PresetFormatVersion,
		Character:     CharacterPresetCore{Class: 0, Vigor: 15, Mind: 10, Endurance: 11, Strength: 14, Dexterity: 13, Intelligence: 9, Faith: 9, Arcane: 7},
		World:         &WorldPresetData{Bosses: []uint32{1_000_000_001}},
	}
	warnings := ValidatePreset(preset)
	found := false
	for _, w := range warnings {
		if containsString(w, "world.bosses") && containsString(w, "outside all known EventFlag ranges") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected out-of-range warning for world.bosses ID >= 1B, got: %v", warnings)
	}
}

func TestValidatePreset_Bosses_NoWarningNormalID(t *testing.T) {
	// Generic validator only checks duplicates and >= 1B IDs.
	// An unknown but non-extreme boss ID must NOT produce a warning
	// (no dedicated boss DB lookup in the generic validator).
	preset := &CharacterPreset{
		FormatVersion: PresetFormatVersion,
		Character:     CharacterPresetCore{Class: 0, Vigor: 15, Mind: 10, Endurance: 11, Strength: 14, Dexterity: 13, Intelligence: 9, Faith: 9, Arcane: 7},
		World:         &WorldPresetData{Bosses: []uint32{12345}},
	}
	warnings := ValidatePreset(preset)
	for _, w := range warnings {
		if containsString(w, "world.bosses") {
			t.Errorf("unexpected boss warning for non-extreme unknown ID 12345: %q", w)
		}
	}
}

func TestValidatePreset_Cookbooks_Duplicate(t *testing.T) {
	preset := &CharacterPreset{
		FormatVersion: PresetFormatVersion,
		Character:     CharacterPresetCore{Class: 0, Vigor: 15, Mind: 10, Endurance: 11, Strength: 14, Dexterity: 13, Intelligence: 9, Faith: 9, Arcane: 7},
		World:         &WorldPresetData{Cookbooks: []uint32{222, 333, 222}},
	}
	warnings := ValidatePreset(preset)
	found := false
	for _, w := range warnings {
		if containsString(w, "world.cookbooks") && containsString(w, "appears more than once") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected duplicate warning for world.cookbooks, got: %v", warnings)
	}
}

func TestValidatePreset_WorldPickups_VeryHighID(t *testing.T) {
	preset := &CharacterPreset{
		FormatVersion: PresetFormatVersion,
		Character:     CharacterPresetCore{Class: 0, Vigor: 15, Mind: 10, Endurance: 11, Strength: 14, Dexterity: 13, Intelligence: 9, Faith: 9, Arcane: 7},
		World:         &WorldPresetData{WorldPickups: []uint32{2_000_000_000}},
	}
	warnings := ValidatePreset(preset)
	found := false
	for _, w := range warnings {
		if containsString(w, "world.worldPickups") && containsString(w, "outside all known EventFlag ranges") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected out-of-range warning for world.worldPickups ID >= 1B, got: %v", warnings)
	}
}

func TestValidatePresetModules_EmptyWorld(t *testing.T) {
	preset := &CharacterPreset{
		FormatVersion: PresetFormatVersion,
		Character:     CharacterPresetCore{Class: 0, Vigor: 15, Mind: 10, Endurance: 11, Strength: 14, Dexterity: 13, Intelligence: 9, Faith: 9, Arcane: 7},
		World:         &WorldPresetData{},
	}
	warnings := ValidatePreset(preset)
	for _, w := range warnings {
		if containsString(w, "world.") {
			t.Errorf("unexpected world warning for empty WorldPresetData: %q", w)
		}
	}
}

func TestValidatePresetModules_AggregatesWarnings(t *testing.T) {
	// Each sub-validator must fire at least once. Uses one invalid input per module.
	preset := &CharacterPreset{
		FormatVersion: PresetFormatVersion,
		Character:     CharacterPresetCore{Class: 0, Vigor: 15, Mind: 10, Endurance: 11, Strength: 14, Dexterity: 13, Intelligence: 9, Faith: 9, Arcane: 7},
		World: &WorldPresetData{
			SummoningPools: []uint32{10000040},  // pre-v1.12 → validateWorldSummoningPools
			Regions:        []uint32{9999999},   // unknown → validateWorldRegions
			Graces:         []uint32{99999},     // unknown → validateWorldGraces
			MapFlags:       []uint32{99999},     // unknown → validateWorldMapFlags
			Colosseums:     []uint32{99999},     // unknown → validateWorldColosseums
			Bosses:         []uint32{111, 111},  // duplicate → validateKnownEventFlags
		},
	}
	warnings := ValidatePreset(preset)
	required := []string{
		"pre-v1.12",
		"not found in region database",
		"not found in grace database",
		"not found in map flag database",
		"not found in colosseum database",
		"world.bosses",
	}
	for _, substr := range required {
		found := false
		for _, w := range warnings {
			if containsString(w, substr) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected warning containing %q, not found in: %v", substr, warnings)
		}
	}
}

func TestValidatePreset_SummoningPools_NoDoubleWarning(t *testing.T) {
	// ID 1035530040 is a pre-v1.12 summoning pool ID and also >= 1_000_000_000.
	// validateKnownEventFlags must NOT be applied to World.SummoningPools —
	// only the dedicated validateWorldSummoningPools should warn ("pre-v1.12"),
	// not the generic "outside all known EventFlag ranges" warning.
	preset := &CharacterPreset{
		FormatVersion: PresetFormatVersion,
		Character:     CharacterPresetCore{Class: 0, Vigor: 15, Mind: 10, Endurance: 11, Strength: 14, Dexterity: 13, Intelligence: 9, Faith: 9, Arcane: 7},
		World:         &WorldPresetData{SummoningPools: []uint32{1035530040}},
	}
	warnings := ValidatePreset(preset)
	foundPreV112 := false
	foundGeneric := false
	for _, w := range warnings {
		if containsString(w, "pre-v1.12") {
			foundPreV112 = true
		}
		if containsString(w, "outside all known EventFlag ranges") {
			foundGeneric = true
		}
	}
	if !foundPreV112 {
		t.Errorf("expected pre-v1.12 warning for ID 1035530040, got: %v", warnings)
	}
	if foundGeneric {
		t.Errorf("unexpected generic EventFlag-range warning for summoningPools ID: %v", warnings)
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
