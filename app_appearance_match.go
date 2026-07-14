package main

import (
	"bytes"
	"encoding/binary"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
	"github.com/oisis/EldenRing-SaveForge/backend/db/data"
)

// appearanceBlobMatches reports whether the eight raw model u32s and the
// FaceShape/Body/Skin blocks at the given offsets in b equal the resolved
// payload. Shared by the character and Mirror matchers because both layouts
// store contiguous u32 models followed by the same 64/7/91-byte slider blocks.
func appearanceBlobMatches(b []byte, modelOff, faceShapeOff, bodyOff, skinOff int, r resolvedAppearance) bool {
	for i, id := range r.Models {
		if binary.LittleEndian.Uint32(b[modelOff+i*4:]) != uint32(id) {
			return false
		}
	}
	return bytes.Equal(b[faceShapeOff:faceShapeOff+64], r.FaceShape[:]) &&
		bytes.Equal(b[bodyOff:bodyOff+7], r.Body[:]) &&
		bytes.Equal(b[skinOff:skinOff+91], r.Skin[:])
}

// matchCharacterAppearance returns the single preset whose resolved appearance
// exactly equals slot's FaceData, or nil. Candidates are built only via
// resolveAppearance (unmapped Type B presets are skipped, never guessed).
//
// Compared: all eight raw model u32s, FaceShape (64B), Body (7B), Skin (91B),
// Player.Gender and Player.VoiceType. Ignored: unk0x6c and the two trailing
// game-reset sex flags at 0x125/0x126. Zero matches or more than one → nil.
// Read-only: it never mutates the slot.
func matchCharacterAppearance(slot *core.SaveSlot) *data.AppearancePreset {
	fd := slot.FaceDataStart()
	if fd < 0 || fd+core.FaceDataBlobSize > len(slot.Data) {
		return nil
	}
	var found *data.AppearancePreset
	for i := range data.Presets {
		p := &data.Presets[i]
		r, err := resolveAppearance(p)
		if err != nil {
			continue
		}
		if slot.Player.Gender != r.BodyType || slot.Player.VoiceType != r.VoiceType {
			continue
		}
		if !appearanceBlobMatches(slot.Data, fd+core.FDOffFaceModel, fd+core.FDOffFaceShape, fd+core.FDOffHead, fd+core.FDOffSkinR, r) {
			continue
		}
		if found != nil {
			return nil // ambiguous — never choose by order
		}
		found = p
	}
	return found
}

// matchMirrorAppearance returns the single preset whose resolved appearance
// exactly equals an active Mirror Favorite entry, or nil. entry is the raw
// FavSlotSize slot bytes. Candidates are built only via resolveAppearance.
//
// Compared: all eight raw model u32s, FaceShape (64B), Body (7B), Skin (91B),
// and the Mirror body-type byte under the existing inverted convention
// (Type A→0, Type B→1). NOT compared: unk0x6c and VoiceType (Mirror stores no
// voice). Inactive entry, zero matches, or more than one → nil. Read-only.
func matchMirrorAppearance(entry []byte) *data.AppearancePreset {
	if len(entry) < core.FavOffSkin+91 {
		return nil
	}
	if string(entry[core.FavOffMagic:core.FavOffMagic+4]) != "FACE" {
		return nil
	}
	var found *data.AppearancePreset
	for i := range data.Presets {
		p := &data.Presets[i]
		r, err := resolveAppearance(p)
		if err != nil {
			continue
		}
		// Mirror body-type is inverted vs gender: Type A(1)→0, Type B(0)→1.
		wantBodyType := byte(1)
		if r.BodyType == 1 {
			wantBodyType = 0
		}
		if entry[core.FavOffBodyType] != wantBodyType {
			continue
		}
		if !appearanceBlobMatches(entry, core.FavOffModelIDs, core.FavOffFaceShape, core.FavOffBody, core.FavOffSkin, r) {
			continue
		}
		if found != nil {
			return nil // ambiguous — never choose by order
		}
		found = p
	}
	return found
}
