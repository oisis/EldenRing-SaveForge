package main

import (
	"encoding/binary"
	"fmt"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
	"github.com/oisis/EldenRing-SaveForge/backend/db/data"
)

// resolvedAppearance is a preset's appearance fully resolved to raw save-file
// values, ready to write. It is the single source of truth shared by direct
// Apply (writePresetAppearance) and the Mirror writer (WriteSelectedToFavorites)
// so the two destinations can never diverge on model conversion again.
//
// Models order is the FaceData/Mirror layout:
// Face, Hair, Eye, Eyebrow, Beard, Eyepatch, Decal, Eyelash.
type resolvedAppearance struct {
	Models    [8]uint8 // raw PartsIds, already converted from UI values
	FaceShape [64]byte
	Body      [7]byte
	Skin      [91]byte
	BodyType  uint8 // 1=Type A (male), 0=Type B (female)
	VoiceType uint8
}

// resolveAppearance converts a preset's UI model values into raw save-file
// PartsIds, validating BEFORE any mutation:
//   - Type A (BodyType 1): UI-1 for every model, hair via LookupMaleHairPartsID
//     (UI-1 fallback for unmapped styles) — byte-identical to the prior behavior.
//   - Type B (BodyType 0): data.LookupFemaleModelIDs, which rejects any value
//     outside the verified UI→PartsId table with NO fallback.
//
// An unmapped Type B preset returns an error and a zero payload, so callers can
// fail closed before touching Undo, a snapshot, or the save.
func resolveAppearance(preset *data.AppearancePreset) (resolvedAppearance, error) {
	r := resolvedAppearance{
		FaceShape: preset.FaceShape,
		Body:      preset.Body,
		Skin:      preset.Skin,
		BodyType:  preset.BodyType,
		VoiceType: preset.VoiceType,
	}

	if preset.BodyType == 1 {
		ui1 := func(v uint8) uint8 {
			if v > 0 {
				return v - 1
			}
			return 0
		}
		r.Models[0] = ui1(preset.FaceModel)
		if partsID, ok := data.LookupMaleHairPartsID(preset.HairModel); ok {
			r.Models[1] = partsID
		} else {
			r.Models[1] = ui1(preset.HairModel)
		}
		r.Models[2] = ui1(preset.EyeModel)
		r.Models[3] = ui1(preset.EyebrowModel)
		r.Models[4] = ui1(preset.BeardModel)
		r.Models[5] = ui1(preset.EyepatchModel)
		r.Models[6] = ui1(preset.DecalModel)
		r.Models[7] = ui1(preset.EyelashModel)
		return r, nil
	}

	models, ok := data.LookupFemaleModelIDs(*preset)
	if !ok {
		return resolvedAppearance{}, fmt.Errorf("preset %q has Type B model values outside the verified UI→PartsId mapping", preset.Name)
	}
	r.Models = models
	return r, nil
}

// applyResolvedAppearance writes a resolved payload into a character's FaceData
// blob and sets gender/voice. It cannot fail (all validation happened in
// resolveAppearance) and preserves the target slot's unk0x6c block. fd is the
// FaceData blob start (slot.FaceDataStart()).
func applyResolvedAppearance(slot *core.SaveSlot, fd int, r resolvedAppearance) {
	copy(slot.Data[fd+core.FDOffFaceShape:fd+core.FDOffFaceShape+64], r.FaceShape[:])
	copy(slot.Data[fd+core.FDOffHead:fd+core.FDOffHead+7], r.Body[:])
	copy(slot.Data[fd+core.FDOffSkinR:fd+core.FDOffSkinR+91], r.Skin[:])

	// Eight model IDs at contiguous FDOffFaceModel..FDOffEyelashModel (0x10..0x2C).
	for i, id := range r.Models {
		binary.LittleEndian.PutUint32(slot.Data[fd+core.FDOffFaceModel+i*4:], uint32(id))
	}

	// Zero trailing sex-flag bytes — game resets these on apply; leaving them
	// at non-zero causes Type A body even when Gender is set to female.
	slot.Data[fd+0x125] = 0
	slot.Data[fd+0x126] = 0

	slot.Player.Gender = r.BodyType
	slot.Player.VoiceType = r.VoiceType
}
