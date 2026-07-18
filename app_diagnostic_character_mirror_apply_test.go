package main

import (
	"encoding/binary"
	"encoding/hex"
	"strconv"
	"strings"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
)

// mirrorApplyFixture builds an App with a single character slot exposing a valid
// FaceData blob at fd=0 (0xFF-seeded so every Appearance field the Mirror writer
// touches differs from the starting state) and an active Mirror Favorites slot in
// UserData10 carrying distinctive raw model/hex data. The Mirror models are
// deliberately > 255 to prove they are logged as raw uint32 LE, not clamped to
// uint8. FavOffBodyType=0 yields planned Gender 1, and the slot's own gender is
// seeded to 0 so the gender field changes. Returns the App and the expected raw
// Mirror payload the writer will copy.
func mirrorApplyFixture(t *testing.T) (*App, [8]uint32, []byte, []byte, []byte) {
	t.Helper()
	const charIdx = 0
	const mirrorIdx = 0

	app := NewApp()
	app.save = &core.SaveFile{}

	slot := &app.save.Slots[charIdx]
	slot.Data = make([]byte, 1024)
	for i := range slot.Data {
		slot.Data[i] = 0xFF
	}
	slot.FaceDataOffset = core.FaceDataBlobSize // FaceDataStart() == 0
	slot.Player.Gender = 0                      // Mirror FavOffBodyType=0 -> Gender 1
	slot.Player.VoiceType = 4                   // must stay untouched (never logged)

	// Raw model IDs, some > 255 so a uint8 clamp would be visible.
	models := [8]uint32{300, 400, 0, 14, 999, 65536, 29, 3}

	faceShape := make([]byte, 64)
	for i := range faceShape {
		faceShape[i] = byte(0x40 + i)
	}
	body := []byte{0x81, 0x82, 0x83, 0x84, 0x85, 0x86, 0x87}
	skin := make([]byte, 91)
	for i := range skin {
		skin[i] = byte(0xC0 + i)
	}

	ud := make([]byte, 0x60000)
	off := core.FavBaseOffset + mirrorIdx*core.FavSlotSize
	copy(ud[off+core.FavOffMagic:], []byte("FACE"))
	binary.LittleEndian.PutUint32(ud[off+core.FavOffAlignment:], 4)
	binary.LittleEndian.PutUint32(ud[off+core.FavOffInnerSize:], 0x120)
	ud[off+core.FavOffBodyFlag] = 1
	ud[off+core.FavOffBodyType] = 0 // -> Gender 1 (male) on apply
	for i, m := range models {
		binary.LittleEndian.PutUint32(ud[off+core.FavOffModelIDs+i*4:], m)
	}
	copy(ud[off+core.FavOffFaceShape:], faceShape)
	copy(ud[off+core.FavOffBody:], body)
	copy(ud[off+core.FavOffSkin:], skin)
	app.save.UserData10.Data = ud

	return app, models, faceShape, body, skin
}

// TestApplyMirrorFavoriteAppearanceDiagnosticSuccess drives a real
// ApplyMirrorFavoriteToCharacter applying an active Mirror slot onto a character
// whose Appearance differs in every field, and asserts the full before -> planned
// -> finished lifecycle for the eight raw models, the three hex blocks, the apply
// flags and gender (13 fields) — with exact, full-length values, the real
// post-write slot matching the finished phase, and voice_type never logged.
func TestApplyMirrorFavoriteAppearanceDiagnosticSuccess(t *testing.T) {
	app, models, faceShape, body, skin := mirrorApplyFixture(t)
	enableDebugJournal(t, app)

	dec := func(v uint32) string { return strconv.FormatUint(uint64(v), 10) }
	faceShapeHex := hex.EncodeToString(faceShape)
	bodyHex := hex.EncodeToString(body)
	skinHex := hex.EncodeToString(skin)

	want := map[string]string{
		"appearance_face_model":     dec(models[0]),
		"appearance_hair_model":     dec(models[1]),
		"appearance_eye_model":      dec(models[2]),
		"appearance_eyebrow_model":  dec(models[3]),
		"appearance_beard_model":    dec(models[4]),
		"appearance_eyepatch_model": dec(models[5]),
		"appearance_decal_model":    dec(models[6]),
		"appearance_eyelash_model":  dec(models[7]),

		"appearance_face_shape_hex":       faceShapeHex,
		"appearance_body_proportions_hex": bodyHex,
		"appearance_skin_cosmetics_hex":   skinHex,
		"appearance_apply_flags_hex":      "0000",

		"gender": "1", // FavOffBodyType=0 -> Gender 1
	}

	// Exact before values from the 0xFF-seeded FaceData blob.
	const modelBefore = "4294967295" // 0xFFFFFFFF
	wantBefore := map[string]string{
		"appearance_face_model":     modelBefore,
		"appearance_hair_model":     modelBefore,
		"appearance_eye_model":      modelBefore,
		"appearance_eyebrow_model":  modelBefore,
		"appearance_beard_model":    modelBefore,
		"appearance_eyepatch_model": modelBefore,
		"appearance_decal_model":    modelBefore,
		"appearance_eyelash_model":  modelBefore,

		"appearance_face_shape_hex":       strings.Repeat("ff", 64),
		"appearance_body_proportions_hex": strings.Repeat("ff", 7),
		"appearance_skin_cosmetics_hex":   strings.Repeat("ff", 91),
		"appearance_apply_flags_hex":      "ffff",

		"gender": "0",
	}

	if err := app.ApplyMirrorFavoriteToCharacter(0, 0); err != nil {
		t.Fatalf("ApplyMirrorFavoriteToCharacter: %v", err)
	}

	records := characterRecords(app.journal.Tail())

	for field, planned := range want {
		phases := saveCharacterPhases(records, field)
		if len(phases) != 3 {
			t.Fatalf("field %q: got %d records, want 3", field, len(phases))
		}
		wantEvents := []string{eventCharacterChangeBefore, eventCharacterChangePlanned, eventCharacterChangeFinished}
		for i, rec := range phases {
			if rec.Event != wantEvents[i] {
				t.Errorf("field %q phase %d: event = %q, want %q", field, i, rec.Event, wantEvents[i])
			}
			if got := operationField(rec, "action"); got != actionApplyMirrorFavorite {
				t.Errorf("field %q phase %d: action = %q, want %q", field, i, got, actionApplyMirrorFavorite)
			}
			if got := operationField(rec, "before"); got != wantBefore[field] {
				t.Errorf("field %q phase %d: before = %q, want %q", field, i, got, wantBefore[field])
			}
		}
		if got := operationField(phases[0], "after"); got != "" {
			t.Errorf("field %q before phase leaked after = %q", field, got)
		}
		if got := operationField(phases[1], "after"); got != planned {
			t.Errorf("field %q planned after = %q, want %q", field, got, planned)
		}
		finished := phases[2]
		if got := operationField(finished, "after"); got != planned {
			t.Errorf("field %q finished after = %q, want %q (real post-write)", field, got, planned)
		}
		if got := operationField(finished, "outcome"); got != string(characterChangeSuccess) {
			t.Errorf("field %q finished outcome = %q, want success", field, got)
		}
		if got := operationField(finished, "stage"); got != characterStageCompleted {
			t.Errorf("field %q finished stage = %q, want %q", field, got, characterStageCompleted)
		}
	}

	// Exactly 13 fields logged — voice_type is never recorded by this writer.
	if got := len(saveCharacterPhases(records, "voice_type")); got != 0 {
		t.Errorf("voice_type emitted %d records, want 0 (writer leaves voice untouched)", got)
	}

	// Nothing beyond those 13 fields is logged: exactly 13 fields × 3 phases
	// (before/planned/finished) and no stray extra records.
	if got, want := len(records), len(want)*3; got != want {
		t.Errorf("got %d character records, want %d (13 fields × 3 phases)", got, want)
	}

	// Full hex content and exact lengths (64/7/91 bytes -> 128/14/182 chars).
	if len(faceShapeHex) != 128 {
		t.Fatalf("face_shape_hex len = %d, want 128", len(faceShapeHex))
	}
	if len(bodyHex) != 14 {
		t.Fatalf("body_proportions_hex len = %d, want 14", len(bodyHex))
	}
	if len(skinHex) != 182 {
		t.Fatalf("skin_cosmetics_hex len = %d, want 182", len(skinHex))
	}

	// All before precede all planned precede all finished.
	assertPhaseGrouping(t, records)

	// The real slot after the write must match every finished value.
	slot := &app.save.Slots[0]
	fd := slot.FaceDataStart()
	if got := hex.EncodeToString(slot.Data[fd+core.FDOffFaceShape : fd+core.FDOffFaceShape+64]); got != faceShapeHex {
		t.Errorf("slot face_shape = %q, want %q", got, faceShapeHex)
	}
	if got := hex.EncodeToString(slot.Data[fd+core.FDOffHead : fd+core.FDOffHead+7]); got != bodyHex {
		t.Errorf("slot body = %q, want %q", got, bodyHex)
	}
	if got := hex.EncodeToString(slot.Data[fd+core.FDOffSkinR : fd+core.FDOffSkinR+91]); got != skinHex {
		t.Errorf("slot skin = %q, want %q", got, skinHex)
	}
	if got := hex.EncodeToString(slot.Data[fd+0x125 : fd+0x127]); got != "0000" {
		t.Errorf("slot apply_flags = %q, want 0000", got)
	}
	modelOffsets := []struct {
		field  string
		offset int
	}{
		{"appearance_face_model", core.FDOffFaceModel},
		{"appearance_hair_model", core.FDOffHairModel},
		{"appearance_eye_model", core.FDOffEyeModel},
		{"appearance_eyebrow_model", core.FDOffEyebrowModel},
		{"appearance_beard_model", core.FDOffBeardModel},
		{"appearance_eyepatch_model", core.FDOffEyepatchModel},
		{"appearance_decal_model", core.FDOffDecalModel},
		{"appearance_eyelash_model", core.FDOffEyelashModel},
	}
	for _, m := range modelOffsets {
		if got := binary.LittleEndian.Uint32(slot.Data[fd+m.offset:]); dec(got) != want[m.field] {
			t.Errorf("slot %s = %d, want %s", m.field, got, want[m.field])
		}
	}
	if dec(uint32(slot.Player.Gender)) != want["gender"] {
		t.Errorf("slot gender = %d, want %s", slot.Player.Gender, want["gender"])
	}
	if slot.Player.VoiceType != 4 {
		t.Errorf("slot voice = %d, want 4 (preserved)", slot.Player.VoiceType)
	}
}

// TestApplyMirrorFavoriteAppearanceDiagnosticUnchangedEmitsNothing applies the same
// Mirror slot twice: the second application finds the slot already matching the raw
// Mirror payload, so no Appearance field records are emitted.
func TestApplyMirrorFavoriteAppearanceDiagnosticUnchangedEmitsNothing(t *testing.T) {
	app, _, _, _, _ := mirrorApplyFixture(t)

	if err := app.ApplyMirrorFavoriteToCharacter(0, 0); err != nil {
		t.Fatalf("first ApplyMirrorFavoriteToCharacter: %v", err)
	}

	// Enable the journal only for the second (no-op) application.
	enableDebugJournal(t, app)
	if err := app.ApplyMirrorFavoriteToCharacter(0, 0); err != nil {
		t.Fatalf("second ApplyMirrorFavoriteToCharacter: %v", err)
	}

	if got := characterRecords(app.journal.Tail()); len(got) != 0 {
		t.Fatalf("re-applying identical Mirror slot emitted %d records, want 0", len(got))
	}
}
