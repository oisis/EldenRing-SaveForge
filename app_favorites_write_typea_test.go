package main

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
	"github.com/oisis/EldenRing-SaveForge/backend/db/data"
)

// TestWriteSelectedToFavorites_TypeA_InMemory is the persistent, fixture-free
// regression for the normal WriteSelectedToFavorites path (replaces the old
// save-file-dependent FaceModel test). It builds an entirely in-memory App and
// verifies a Type A preset lands in the Mirror slot with the exact expected
// bytes, including the FaceModel=6 → raw 5 (UI-1) A1 fix and the hair lookup.
func TestWriteSelectedToFavorites_TypeA_InMemory(t *testing.T) {
	const charIdx = 0
	const testName = "A3b In-Memory Type A Preset"

	// Distinct, non-zero UI values for all eight models. HairModel=37 exercises
	// the male hair lookup table (UI 37 → PartsId 124); the rest use UI-1.
	preset := data.AppearancePreset{
		Name:          testName,
		BodyType:      1, // Type A (male)
		FaceModel:     6,
		HairModel:     37,
		EyeModel:      2,
		EyebrowModel:  3,
		BeardModel:    4,
		EyepatchModel: 5,
		DecalModel:    8,
		EyelashModel:  9,
	}
	for i := range preset.FaceShape {
		preset.FaceShape[i] = byte(0x40 + i)
	}
	for i := range preset.Body {
		preset.Body[i] = byte(0x81 + i)
	}
	for i := range preset.Skin {
		preset.Skin[i] = byte(0xC0 + i)
	}

	// Reference value: hair UI 37 must map to raw PartsId 124. Assert this with
	// a literal so the test pins the reference, not the production table — and
	// separately confirm the lookup agrees, guarding against silent drift.
	const wantHairPartsID uint32 = 124
	if got, ok := data.LookupMaleHairPartsID(preset.HairModel); !ok || uint32(got) != wantHairPartsID {
		t.Fatalf("LookupMaleHairPartsID(%d) = (%d, %v), want (%d, true)", preset.HairModel, got, ok, wantHairPartsID)
	}

	// Expected raw Model IDs in the written slot: UI-1 for all except hair,
	// which is the literal reference PartsId 124.
	wantModels := [8]uint32{
		uint32(preset.FaceModel - 1),     // 5
		wantHairPartsID,                  // 124 (literal reference)
		uint32(preset.EyeModel - 1),      // 1
		uint32(preset.EyebrowModel - 1),  // 2
		uint32(preset.BeardModel - 1),    // 3
		uint32(preset.EyepatchModel - 1), // 4
		uint32(preset.DecalModel - 1),    // 7
		uint32(preset.EyelashModel - 1),  // 8
	}

	orig := data.Presets
	data.Presets = append(append([]data.AppearancePreset{}, orig...), preset)
	t.Cleanup(func() { data.Presets = orig })

	// In-memory App: empty UserData10 (all Mirror slots free) and a target slot
	// with a valid FaceData blob so the unkBlock read path is exercised.
	app := &App{save: &core.SaveFile{}, favSlotNames: make(map[int]string)}
	app.save.UserData10.Data = make([]byte, 0x60000)
	app.save.Slots[charIdx] = core.SaveSlot{
		Data:           make([]byte, core.FaceDataBlobSize),
		FaceDataOffset: core.FaceDataBlobSize, // → FaceDataStart() == 0
	}

	written, err := app.WriteSelectedToFavorites(charIdx, []string{testName})
	if err != nil {
		t.Fatalf("WriteSelectedToFavorites: %v", err)
	}
	if written != 1 {
		t.Fatalf("WriteSelectedToFavorites wrote %d, want 1", written)
	}

	// The first free slot is slot 0.
	slotOff := core.FavBaseOffset
	ud := app.save.UserData10.Data

	if magic := string(ud[slotOff+core.FavOffMagic : slotOff+core.FavOffMagic+4]); magic != "FACE" {
		t.Fatalf("slot magic = %q, want FACE", magic)
	}
	// Type A → body type byte 0 (inverted vs gender).
	if bt := ud[slotOff+core.FavOffBodyType]; bt != 0 {
		t.Errorf("body type byte = %d, want 0 (Type A)", bt)
	}

	for i, want := range wantModels {
		got := binary.LittleEndian.Uint32(ud[slotOff+core.FavOffModelIDs+i*4:])
		if got != want {
			t.Errorf("Model ID[%d] = %d, want %d", i, got, want)
		}
	}

	if got := ud[slotOff+core.FavOffFaceShape : slotOff+core.FavOffFaceShape+64]; !bytes.Equal(got, preset.FaceShape[:]) {
		t.Error("FaceShape not copied")
	}
	if got := ud[slotOff+core.FavOffBody : slotOff+core.FavOffBody+7]; !bytes.Equal(got, preset.Body[:]) {
		t.Error("Body not copied")
	}
	if got := ud[slotOff+core.FavOffSkin : slotOff+core.FavOffSkin+91]; !bytes.Equal(got, preset.Skin[:]) {
		t.Error("Skin not copied")
	}

	if app.favSlotNames[0] != testName {
		t.Errorf("favSlotNames[0] = %q, want %q", app.favSlotNames[0], testName)
	}
}
