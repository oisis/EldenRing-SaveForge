package main

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
)

// TestApplyMirrorFavorite_TypeB_InMemory exercises the binary Apply path with a
// fully in-memory fixture — no realSaveAppForSave, no tmp/save/. It builds one
// active Type B Mirror slot with raw reference Model IDs (incl. Decal=29) and a
// target character FaceData blob carrying a distinct unk0x6c, then asserts
// ApplyMirrorFavoriteToCharacter copies every appearance field verbatim, keeps
// unk0x6c, and sets the gender to Type B (female).
func TestApplyMirrorFavorite_TypeB_InMemory(t *testing.T) {
	const charIdx = 0
	const mirrorIdx = 0

	// Reference Type B Model IDs (raw, as stored in the save).
	wantModels := [8]uint32{
		21,  // Face
		124, // Hair
		0,   // Eye
		14,  // Eyebrow
		0,   // Beard
		0,   // Eyepatch
		29,  // Decal (tattoo) — must not be zeroed
		3,   // Eyelash
	}

	// --- Build the Mirror slot in UserData10 ---
	ud := make([]byte, 0x60000)
	mirrorOff := core.FavBaseOffset + mirrorIdx*core.FavSlotSize

	// Header: FACE magic (active), Type B body type. FavOffBodyType is stored
	// inverted vs. gender — 1 here yields a female (Type B) target on apply.
	copy(ud[mirrorOff+core.FavOffMagic:], []byte("FACE"))
	binary.LittleEndian.PutUint32(ud[mirrorOff+core.FavOffAlignment:], 4)
	binary.LittleEndian.PutUint32(ud[mirrorOff+core.FavOffInnerSize:], 0x120)
	ud[mirrorOff+core.FavOffBodyFlag] = 1
	ud[mirrorOff+core.FavOffBodyType] = 1 // Type B (female)

	for i, m := range wantModels {
		binary.LittleEndian.PutUint32(ud[mirrorOff+core.FavOffModelIDs+i*4:], m)
	}

	// Distinguishable FaceShape (64B), Body (7B), Skin (91B) — deterministic,
	// non-zero patterns so a missed copy is visible against the zeroed target.
	wantFaceShape := make([]byte, 64)
	for i := range wantFaceShape {
		wantFaceShape[i] = byte(0x40 + i)
	}
	wantBody := []byte{0x81, 0x82, 0x83, 0x84, 0x85, 0x86, 0x87}
	wantSkin := make([]byte, 91)
	for i := range wantSkin {
		wantSkin[i] = byte(0xC0 + i)
	}
	copy(ud[mirrorOff+core.FavOffFaceShape:], wantFaceShape)
	copy(ud[mirrorOff+core.FavOffBody:], wantBody)
	copy(ud[mirrorOff+core.FavOffSkin:], wantSkin)

	// --- Build the target character slot ---
	// FaceDataStart = FaceDataOffset - FaceDataBlobSize; pin the blob at 0.
	slotData := make([]byte, core.FaceDataBlobSize)
	// Distinct unk0x6c (64B) that Apply must preserve untouched.
	wantUnk := make([]byte, 64)
	for i := range wantUnk {
		wantUnk[i] = 0x77
	}
	copy(slotData[core.FDOffUnknownBlock:], wantUnk)

	app := &App{save: &core.SaveFile{}}
	app.save.ActiveSlots[charIdx] = true
	app.save.Slots[charIdx] = core.SaveSlot{
		Data:           slotData,
		FaceDataOffset: core.FaceDataBlobSize, // → FaceDataStart() == 0
	}
	// Distinctive target voice — Mirror Favorites carry no VoiceType field, so
	// Apply must leave it untouched (direct preset Apply is what sets voice).
	const wantVoice = 4
	app.save.Slots[charIdx].Player.VoiceType = wantVoice
	app.save.UserData10.Data = ud

	// --- Act ---
	if err := app.ApplyMirrorFavoriteToCharacter(charIdx, mirrorIdx); err != nil {
		t.Fatalf("ApplyMirrorFavoriteToCharacter: %v", err)
	}

	// --- Assert ---
	fd := app.save.Slots[charIdx].FaceDataStart()
	data := app.save.Slots[charIdx].Data

	for i, want := range wantModels {
		got := binary.LittleEndian.Uint32(data[fd+core.FDOffFaceModel+i*4:])
		if got != want {
			t.Errorf("Model ID[%d] = %d, want %d", i, got, want)
		}
	}

	if got := data[fd+core.FDOffFaceShape : fd+core.FDOffFaceShape+64]; !bytes.Equal(got, wantFaceShape) {
		t.Error("FaceShape not copied verbatim")
	}
	if got := data[fd+core.FDOffHead : fd+core.FDOffHead+7]; !bytes.Equal(got, wantBody) {
		t.Error("Body not copied verbatim")
	}
	if got := data[fd+core.FDOffSkinR : fd+core.FDOffSkinR+91]; !bytes.Equal(got, wantSkin) {
		t.Error("Skin not copied verbatim")
	}
	if got := data[fd+core.FDOffUnknownBlock : fd+core.FDOffUnknownBlock+64]; !bytes.Equal(got, wantUnk) {
		t.Error("unk0x6c was not preserved")
	}
	if gender := app.save.Slots[charIdx].Player.Gender; gender != 0 {
		t.Errorf("target gender = %d, want 0 (female/Type B)", gender)
	}
	// Confirmed format rule: Mirror Apply never writes VoiceType — the target's
	// own voice is preserved (only direct preset Apply sets the preset voice).
	if voice := app.save.Slots[charIdx].Player.VoiceType; voice != wantVoice {
		t.Errorf("target VoiceType = %d, want %d (preserved)", voice, wantVoice)
	}
}
