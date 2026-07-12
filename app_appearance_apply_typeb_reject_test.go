package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
	"github.com/oisis/EldenRing-SaveForge/backend/db/data"
)

// TestApplyPresetToCharacter_TypeB_Rejected verifies that the public Apply path
// refuses a Type B (BodyType == 0) preset before creating any Undo snapshot or
// touching the slot — the raw female model mapping is unverified, so applying
// would risk a scrambled look and a lost tattoo (A4a).
func TestApplyPresetToCharacter_TypeB_Rejected(t *testing.T) {
	const charIdx = 0
	const testName = "A4a Type B Reject Preset"

	preset := data.AppearancePreset{
		Name:      testName,
		BodyType:  0, // Type B (female)
		FaceModel: 3,
		HairModel: 4,
	}
	orig := data.Presets
	data.Presets = append(append([]data.AppearancePreset{}, orig...), preset)
	t.Cleanup(func() { data.Presets = orig })

	// In-memory App with a zeroed target slot whose gender is set to a distinct
	// value so an accidental write is visible.
	app := &App{save: &core.SaveFile{}}
	slotData := make([]byte, core.FaceDataBlobSize)
	app.save.Slots[charIdx] = core.SaveSlot{
		Data:           slotData,
		FaceDataOffset: core.FaceDataBlobSize, // → FaceDataStart() == 0
	}
	app.save.Slots[charIdx].Player.Gender = 1 // Type A; must stay untouched

	err := app.ApplyPresetToCharacter(charIdx, testName)
	if err == nil {
		t.Fatal("ApplyPresetToCharacter(Type B) = nil, want rejection error")
	}
	if !strings.Contains(err.Error(), "Type B") {
		t.Errorf("error = %q, want mention of Type B", err)
	}

	// Slot bytes untouched.
	if !bytes.Equal(app.save.Slots[charIdx].Data, make([]byte, core.FaceDataBlobSize)) {
		t.Error("slot Data was modified by a rejected Type B apply")
	}
	// Gender untouched.
	if g := app.save.Slots[charIdx].Player.Gender; g != 1 {
		t.Errorf("gender = %d, want 1 (unchanged)", g)
	}
	// No Undo snapshot pushed.
	if d := len(app.undoStacks[charIdx]); d != 0 {
		t.Errorf("undo depth = %d, want 0 (no snapshot on rejection)", d)
	}
}
