package main

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
	"github.com/oisis/EldenRing-SaveForge/backend/db/data"
)

// TestWritePresetAppearance_TypeB_MappedModels verifies that a Type B preset with
// distinct verified UI values writes all eight raw PartsIds in the correct order,
// including a non-zero tattoo (Decal 29 → 29 is never zeroed).
func TestWritePresetAppearance_TypeB_MappedModels(t *testing.T) {
	preset := data.AppearancePreset{
		Name: "A4 Type B Mapped", BodyType: 0, VoiceType: 3,
		FaceModel: 5, HairModel: 24, EyeModel: 0, EyebrowModel: 15,
		BeardModel: 1, EyepatchModel: 1, DecalModel: 29, EyelashModel: 4,
	}

	slot := &core.SaveSlot{Data: make([]byte, core.FaceDataBlobSize)}
	slot.Player.Gender = 1 // Type A before; must flip to female
	if err := writePresetAppearance(slot, 0, &preset); err != nil {
		t.Fatalf("writePresetAppearance(Type B mapped) = %v, want nil", err)
	}

	// Order: Face, Hair, Eye, Eyebrow, Beard, Eyepatch, Decal, Eyelash.
	want := map[int]uint32{
		core.FDOffFaceModel:     40,
		core.FDOffHairModel:     109,
		core.FDOffEyeModel:      0,
		core.FDOffEyebrowModel:  14,
		core.FDOffBeardModel:    0,
		core.FDOffEyepatchModel: 0,
		core.FDOffDecalModel:    29, // non-zero tattoo carried through
		core.FDOffEyelashModel:  3,
	}
	for off, exp := range want {
		if got := binary.LittleEndian.Uint32(slot.Data[off:]); got != exp {
			t.Errorf("model at FaceData+0x%X = %d, want %d", off, got, exp)
		}
	}
	if slot.Player.Gender != 0 {
		t.Errorf("gender = %d, want 0 (female)", slot.Player.Gender)
	}
	if slot.Player.VoiceType != 3 {
		t.Errorf("voice = %d, want 3", slot.Player.VoiceType)
	}
}

// TestWritePresetAppearance_TypeB_UnmappedRejected proves an un-mapped Type B UI
// value is rejected with the target FaceData, Gender and VoiceType untouched —
// no partial or scrambled appearance reaches the slot, and no fallback is used.
func TestWritePresetAppearance_TypeB_UnmappedRejected(t *testing.T) {
	preset := data.AppearancePreset{
		Name: "A4 Type B Unmapped", BodyType: 0, VoiceType: 5,
		FaceModel: 99, // outside the verified table
		HairModel: 24, EyeModel: 0, EyebrowModel: 15,
		BeardModel: 1, EyepatchModel: 1, DecalModel: 29, EyelashModel: 4,
	}

	slot := &core.SaveSlot{Data: make([]byte, core.FaceDataBlobSize)}
	slot.Player.Gender = 1
	slot.Player.VoiceType = 2

	if err := writePresetAppearance(slot, 0, &preset); err == nil {
		t.Fatal("writePresetAppearance(unmapped Type B) = nil, want error")
	}
	if !bytes.Equal(slot.Data, make([]byte, core.FaceDataBlobSize)) {
		t.Error("FaceData mutated by a rejected unmapped Type B write")
	}
	if slot.Player.Gender != 1 {
		t.Errorf("gender = %d, want 1 (unchanged)", slot.Player.Gender)
	}
	if slot.Player.VoiceType != 2 {
		t.Errorf("voice = %d, want 2 (unchanged)", slot.Player.VoiceType)
	}
}
