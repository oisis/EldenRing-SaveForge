package main

import (
	"strings"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/vm"
)

func TestValidateBuiltinCharacterPreset_NoSave(t *testing.T) {
	app := NewApp()
	_, err := app.ValidateBuiltinCharacterPreset(0, "wretch-rl1")
	if err == nil || !strings.Contains(err.Error(), "no save loaded") {
		t.Fatalf("want 'no save loaded' error, got %v", err)
	}
}

func TestValidateBuiltinCharacterPreset_InvalidSlotIndex(t *testing.T) {
	app := minimalSave(0)
	for _, idx := range []int{-1, 10, 99} {
		_, err := app.ValidateBuiltinCharacterPreset(idx, "wretch-rl1")
		if err == nil || !strings.Contains(err.Error(), "invalid slot index") {
			t.Errorf("slot %d: want 'invalid slot index', got %v", idx, err)
		}
	}
}

func TestValidateBuiltinCharacterPreset_UnknownID(t *testing.T) {
	app := minimalSave(0)
	_, err := app.ValidateBuiltinCharacterPreset(0, "does-not-exist")
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("want 'not found' error, got %v", err)
	}
}

func TestValidateBuiltinCharacterPreset_ValidPresets(t *testing.T) {
	app := minimalSave(0)
	ids := []string{"wretch-rl1", "dex-pvp-60", "quality-pvp-125"}
	for _, id := range ids {
		warnings, err := app.ValidateBuiltinCharacterPreset(0, id)
		if err != nil {
			t.Errorf("preset %s: unexpected error: %v", id, err)
			continue
		}
		if len(warnings) > 0 {
			t.Errorf("preset %s: expected no warnings, got: %v", id, warnings)
		}
	}
}

// ─── ApplyBuiltinCharacterPresetStats ─────────────────────────────────────────

func TestIsStatOnlyPreset(t *testing.T) {
	empty := vm.CharacterPreset{
		Inventory: []vm.PresetItem{},
		Storage:   []vm.PresetItem{},
		World:     nil,
	}
	if !isStatOnlyPreset(empty) {
		t.Error("empty preset should be stat-only")
	}

	withInventory := empty
	withInventory.Inventory = []vm.PresetItem{{BaseID: 1}}
	if isStatOnlyPreset(withInventory) {
		t.Error("preset with inventory should NOT be stat-only")
	}

	withStorage := empty
	withStorage.Storage = []vm.PresetItem{{BaseID: 1}}
	if isStatOnlyPreset(withStorage) {
		t.Error("preset with storage should NOT be stat-only")
	}

	withWorld := empty
	withWorld.World = &vm.WorldPresetData{}
	if isStatOnlyPreset(withWorld) {
		t.Error("preset with world should NOT be stat-only")
	}
}

func TestApplyBuiltinCharacterPresetStats_NoSave(t *testing.T) {
	app := NewApp()
	_, err := app.ApplyBuiltinCharacterPresetStats(0, "wretch-rl1")
	if err == nil || !strings.Contains(err.Error(), "no save loaded") {
		t.Fatalf("want 'no save loaded', got %v", err)
	}
}

func TestApplyBuiltinCharacterPresetStats_InvalidSlotIndex(t *testing.T) {
	app := minimalSave(0)
	for _, idx := range []int{-1, 10, 99} {
		_, err := app.ApplyBuiltinCharacterPresetStats(idx, "wretch-rl1")
		if err == nil || !strings.Contains(err.Error(), "invalid slot index") {
			t.Errorf("slot %d: want 'invalid slot index', got %v", idx, err)
		}
	}
}

func TestApplyBuiltinCharacterPresetStats_UnknownID(t *testing.T) {
	app := minimalSave(0)
	_, err := app.ApplyBuiltinCharacterPresetStats(0, "does-not-exist")
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("want 'not found', got %v", err)
	}
}

func TestApplyBuiltinCharacterPresetStats_RejectsNonStatOnly(t *testing.T) {
	// All registered built-in presets must be stat-only; this tests the helper guard directly.
	nonStatOnly := vm.CharacterPreset{
		Inventory: []vm.PresetItem{{BaseID: 1}},
		Storage:   []vm.PresetItem{},
		World:     nil,
	}
	if isStatOnlyPreset(nonStatOnly) {
		t.Error("preset with inventory items should be rejected by isStatOnlyPreset")
	}
}

func TestApplyBuiltinCharacterPresetStats_Success_WretchRL1(t *testing.T) {
	app := minimalSave(0)
	// Set a non-empty character name so ApplyCharacterPreset doesn't treat slot as empty.
	app.save.Slots[0].Player.CharacterName[0] = uint16('T')

	res, err := app.ApplyBuiltinCharacterPresetStats(0, "wretch-rl1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Result flags
	if !res.StatsApplied {
		t.Error("StatsApplied should be true")
	}
	if res.WorldApplied {
		t.Error("WorldApplied should be false")
	}
	if res.ItemsAdded != 0 {
		t.Errorf("ItemsAdded = %d, want 0", res.ItemsAdded)
	}
	if res.ItemsRemoved != 0 {
		t.Errorf("ItemsRemoved = %d, want 0", res.ItemsRemoved)
	}

	// Slot Player fields match Wretch RL1 preset values
	p := app.save.Slots[0].Player
	if p.Level != 1 {
		t.Errorf("Level = %d, want 1", p.Level)
	}
	if p.Class != 9 { // Wretch
		t.Errorf("Class = %d, want 9 (Wretch)", p.Class)
	}
	for _, tc := range []struct {
		name string
		got  uint32
		want uint32
	}{
		{"Vigor", p.Vigor, 10},
		{"Mind", p.Mind, 10},
		{"Endurance", p.Endurance, 10},
		{"Strength", p.Strength, 10},
		{"Dexterity", p.Dexterity, 10},
		{"Intelligence", p.Intelligence, 10},
		{"Faith", p.Faith, 10},
		{"Arcane", p.Arcane, 10},
	} {
		if tc.got != tc.want {
			t.Errorf("%s = %d, want %d", tc.name, tc.got, tc.want)
		}
	}

	// Character name preserved (KeepName=true)
	if app.save.Slots[0].Player.CharacterName[0] != uint16('T') {
		t.Error("CharacterName was overwritten; KeepName=true should preserve it")
	}
}
