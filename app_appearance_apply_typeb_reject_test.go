package main

import (
	"encoding/binary"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
	"github.com/oisis/EldenRing-SaveForge/backend/db/data"
)

// TestApplyPresetToCharacter_TypeB_MappedSucceeds proves the public Apply path
// now accepts a Type B preset whose UI values are fully mapped: it writes the
// verified raw model IDs (incl. a non-zero tattoo), flips gender to female, and
// creates exactly one Undo snapshot (A5 lifts the A4a guard).
func TestApplyPresetToCharacter_TypeB_MappedSucceeds(t *testing.T) {
	const charIdx = 0
	const testName = "A5 Type B Mapped Apply"

	// Distinct mapped values → models [40, 109, 0, 14, 0, 0, 29, 3].
	preset := data.AppearancePreset{
		Name: testName, BodyType: 0, VoiceType: 3,
		FaceModel: 5, HairModel: 24, EyeModel: 0, EyebrowModel: 15,
		BeardModel: 1, EyepatchModel: 1, DecalModel: 29, EyelashModel: 4,
	}
	orig := data.Presets
	data.Presets = append(append([]data.AppearancePreset{}, orig...), preset)
	t.Cleanup(func() { data.Presets = orig })

	app := &App{save: &core.SaveFile{}}
	app.save.Slots[charIdx] = core.SaveSlot{
		Data:           make([]byte, core.FaceDataBlobSize),
		FaceDataOffset: core.FaceDataBlobSize, // → FaceDataStart() == 0
	}
	app.save.Slots[charIdx].Player.Gender = 1 // Type A before

	if err := app.ApplyPresetToCharacter(charIdx, testName); err != nil {
		t.Fatalf("ApplyPresetToCharacter(mapped Type B) = %v, want nil", err)
	}

	want := [8]uint32{40, 109, 0, 14, 0, 0, 29, 3}
	fd := app.save.Slots[charIdx].FaceDataStart()
	for i, exp := range want {
		got := binary.LittleEndian.Uint32(app.save.Slots[charIdx].Data[fd+core.FDOffFaceModel+i*4:])
		if got != exp {
			t.Errorf("model[%d] = %d, want %d", i, got, exp)
		}
	}
	if g := app.save.Slots[charIdx].Player.Gender; g != 0 {
		t.Errorf("gender = %d, want 0 (female/Type B)", g)
	}
	if d := len(app.undoStacks[charIdx]); d != 1 {
		t.Errorf("undo depth = %d, want 1", d)
	}
}

// TestSetCharacterGender_TypeB_MappedSucceeds proves switching to Type B now
// applies the mapped Ciri default: gender flips to female, the Ciri face model
// (UI 5 → raw 40) lands, and exactly one Undo snapshot is created (A5 lifts A4c).
func TestSetCharacterGender_TypeB_MappedSucceeds(t *testing.T) {
	const charIdx = 0

	// Guard the fixture assumption: Ciri (the female default) must be mapped.
	ciri := findPresetByName(data.DefaultFemalePresetName)
	if ciri == nil {
		t.Fatalf("default female preset %q not found", data.DefaultFemalePresetName)
	}
	if _, ok := data.LookupFemaleModelIDs(*ciri); !ok {
		t.Fatalf("default female preset %q is unexpectedly unmapped", data.DefaultFemalePresetName)
	}

	app := &App{save: &core.SaveFile{}}
	app.save.Slots[charIdx] = core.SaveSlot{
		Data:           make([]byte, core.FaceDataBlobSize),
		FaceDataOffset: core.FaceDataBlobSize,
	}
	app.save.Slots[charIdx].Player.Gender = 1 // Type A before

	if err := app.SetCharacterGender(charIdx, 0); err != nil {
		t.Fatalf("SetCharacterGender(female) = %v, want nil", err)
	}

	fd := app.save.Slots[charIdx].FaceDataStart()
	if g := app.save.Slots[charIdx].Player.Gender; g != 0 {
		t.Errorf("gender = %d, want 0 (female)", g)
	}
	// Ciri FaceModel UI 5 → raw 40.
	if got := binary.LittleEndian.Uint32(app.save.Slots[charIdx].Data[fd+core.FDOffFaceModel:]); got != 40 {
		t.Errorf("face model = %d, want 40 (Ciri UI 5 → raw 40)", got)
	}
	if d := len(app.undoStacks[charIdx]); d != 1 {
		t.Errorf("undo depth = %d, want 1", d)
	}
}
