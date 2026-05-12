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
