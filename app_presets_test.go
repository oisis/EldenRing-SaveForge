package main

import (
	"strings"
	"testing"
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
