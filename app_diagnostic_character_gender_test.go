package main

import (
	"encoding/binary"
	"encoding/hex"
	"strconv"
	"strings"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
	"github.com/oisis/EldenRing-SaveForge/backend/db/data"
)

// genderAppearanceFixture builds an App with a single slot exposing a valid
// FaceData blob at fd=0, pre-seeded with 0xFF so every Appearance field the
// default preset writes differs. Gender/VoiceType are seeded to 1 — distinct
// from the Ciri female default (BodyType 0, VoiceType 0) — so those scalar
// fields change too and all 14 fields emit records.
func genderAppearanceFixture() *App {
	app := NewApp()
	app.save = &core.SaveFile{}
	slot := &app.save.Slots[0]
	slot.Version = 1
	slot.Data = make([]byte, 1024)
	for i := range slot.Data {
		slot.Data[i] = 0xFF
	}
	slot.FaceDataOffset = core.FaceDataBlobSize // FaceDataStart() == 0
	slot.GaMap = map[uint32]uint32{}
	slot.Player.Gender = 1    // Ciri is BodyType 0
	slot.Player.VoiceType = 1 // Ciri is VoiceType 0
	return app
}

// TestSetCharacterGenderAppearanceDiagnostic drives a real SetCharacterGender(0, 0)
// applying the Ciri female default onto a slot differing in every field, and
// asserts the full before -> planned -> finished lifecycle for the eight models,
// three hex blocks, apply flags, gender and voice — with exact, full-length
// values and the real post-write slot matching the finished phase.
func TestSetCharacterGenderAppearanceDiagnostic(t *testing.T) {
	app := genderAppearanceFixture()
	enableDebugJournal(t, app)

	preset := findPresetByName(data.DefaultFemalePresetName)
	if preset == nil {
		t.Fatalf("findPresetByName(female default) = nil")
	}
	resolved, err := resolveAppearance(preset)
	if err != nil {
		t.Fatalf("resolveAppearance: %v", err)
	}

	dec := func(v uint32) string { return strconv.FormatUint(uint64(v), 10) }
	faceShapeHex := hex.EncodeToString(resolved.FaceShape[:])
	bodyHex := hex.EncodeToString(resolved.Body[:])
	skinHex := hex.EncodeToString(resolved.Skin[:])

	want := map[string]string{
		"appearance_face_model":     dec(uint32(resolved.Models[0])),
		"appearance_hair_model":     dec(uint32(resolved.Models[1])),
		"appearance_eye_model":      dec(uint32(resolved.Models[2])),
		"appearance_eyebrow_model":  dec(uint32(resolved.Models[3])),
		"appearance_beard_model":    dec(uint32(resolved.Models[4])),
		"appearance_eyepatch_model": dec(uint32(resolved.Models[5])),
		"appearance_decal_model":    dec(uint32(resolved.Models[6])),
		"appearance_eyelash_model":  dec(uint32(resolved.Models[7])),

		"appearance_face_shape_hex":       faceShapeHex,
		"appearance_body_proportions_hex": bodyHex,
		"appearance_skin_cosmetics_hex":   skinHex,
		"appearance_apply_flags_hex":      "0000",

		"gender":     dec(uint32(resolved.BodyType)),
		"voice_type": dec(uint32(resolved.VoiceType)),
	}

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

		"gender":     "1",
		"voice_type": "1",
	}

	if err := app.SetCharacterGender(0, 0); err != nil {
		t.Fatalf("SetCharacterGender: %v", err)
	}

	records := characterRecords(app.journal.Tail())

	for field, planned := range want {
		phases := appearancePhases(records, field)
		if len(phases) != 3 {
			t.Fatalf("field %q: got %d records, want 3", field, len(phases))
		}
		wantEvents := []string{eventCharacterChangeBefore, eventCharacterChangePlanned, eventCharacterChangeFinished}
		for i, rec := range phases {
			if rec.Event != wantEvents[i] {
				t.Errorf("field %q phase %d: event = %q, want %q", field, i, rec.Event, wantEvents[i])
			}
			if got := operationField(rec, "action"); got != actionSetCharacterGender {
				t.Errorf("field %q phase %d: action = %q, want %q", field, i, got, actionSetCharacterGender)
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
	if dec(uint32(slot.Player.VoiceType)) != want["voice_type"] {
		t.Errorf("slot voice = %d, want %s", slot.Player.VoiceType, want["voice_type"])
	}
}

// TestSetCharacterGenderAppearanceDiagnosticUnchangedEmitsNothing switches to the
// same gender twice: the second call finds the slot already matching the default
// preset, so no Appearance field records are emitted.
func TestSetCharacterGenderAppearanceDiagnosticUnchangedEmitsNothing(t *testing.T) {
	app := genderAppearanceFixture()

	if err := app.SetCharacterGender(0, 0); err != nil {
		t.Fatalf("first SetCharacterGender: %v", err)
	}

	enableDebugJournal(t, app)
	if err := app.SetCharacterGender(0, 0); err != nil {
		t.Fatalf("second SetCharacterGender: %v", err)
	}

	if got := characterRecords(app.journal.Tail()); len(got) != 0 {
		t.Fatalf("re-applying identical gender emitted %d records, want 0", len(got))
	}
}
