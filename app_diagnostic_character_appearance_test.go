package main

import (
	"encoding/binary"
	"encoding/hex"
	"strconv"
	"strings"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
)

// appearancePresetFixture builds an App with a single slot that exposes a valid
// FaceData blob at fd=0, pre-seeded with 0xFF so every Appearance field the
// preset writes differs from the starting state. Gender/VoiceType are seeded to
// values distinct from the Geralt preset so those scalar fields change too.
func appearancePresetFixture() *App {
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
	slot.Player.Gender = 0    // Geralt is BodyType 1
	slot.Player.VoiceType = 0 // Geralt is VoiceType 2
	return app
}

// appearancePhases returns the before/planned/finished records for a single
// Appearance field, in lifecycle order.
func appearancePhases(records []diagnosticRecord, field string) []diagnosticRecord {
	return saveCharacterPhases(records, field)
}

// TestApplyPresetAppearanceDiagnosticSuccess drives a real ApplyPresetToCharacter
// applying the Geralt preset onto a slot whose Appearance differs in every field,
// and asserts the full before -> planned -> finished lifecycle for the eight
// models, the three hex blocks, the apply flags, gender and voice — with exact,
// full-length values and the real post-write slot matching the finished phase.
func TestApplyPresetAppearanceDiagnosticSuccess(t *testing.T) {
	app := appearancePresetFixture()
	enableDebugJournal(t, app)

	const presetName = "Geralt of Rivia, the Witcher"
	preset := findPresetByName(presetName)
	if preset == nil {
		t.Fatalf("findPresetByName(%q) = nil", presetName)
	}
	resolved, err := resolveAppearance(preset)
	if err != nil {
		t.Fatalf("resolveAppearance: %v", err)
	}

	dec := func(v uint32) string { return strconv.FormatUint(uint64(v), 10) }
	faceShapeHex := hex.EncodeToString(resolved.FaceShape[:])
	bodyHex := hex.EncodeToString(resolved.Body[:])
	skinHex := hex.EncodeToString(resolved.Skin[:])

	// Expected planned/finished value per field. The exact before values live in
	// the separate wantBefore map below and are asserted there.
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

	// Exact before values from the 0xFF-seeded fixture: every model uint32 LE reads
	// 0xFFFFFFFF, every hex block is all-ff, and gender/voice were seeded to 0.
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

		"gender":     "0",
		"voice_type": "0",
	}

	if err := app.ApplyPresetToCharacter(0, presetName); err != nil {
		t.Fatalf("ApplyPresetToCharacter: %v", err)
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
			if got := operationField(rec, "action"); got != actionApplyAppearancePreset {
				t.Errorf("field %q phase %d: action = %q, want %q", field, i, got, actionApplyAppearancePreset)
			}
			if got := operationField(rec, "before"); got != wantBefore[field] {
				t.Errorf("field %q phase %d: before = %q, want %q", field, i, got, wantBefore[field])
			}
		}
		// before phase omits after; planned and finished carry the planned value.
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

// TestApplyPresetAppearanceDiagnosticUnchangedEmitsNothing applies the same
// preset twice: the second application finds the slot already matching the
// resolved payload, so no Appearance field records are emitted.
func TestApplyPresetAppearanceDiagnosticUnchangedEmitsNothing(t *testing.T) {
	app := appearancePresetFixture()

	const presetName = "Geralt of Rivia, the Witcher"
	if err := app.ApplyPresetToCharacter(0, presetName); err != nil {
		t.Fatalf("first ApplyPresetToCharacter: %v", err)
	}

	// Enable the journal only for the second (no-op) application.
	enableDebugJournal(t, app)
	if err := app.ApplyPresetToCharacter(0, presetName); err != nil {
		t.Fatalf("second ApplyPresetToCharacter: %v", err)
	}

	if got := characterRecords(app.journal.Tail()); len(got) != 0 {
		t.Fatalf("re-applying identical preset emitted %d records, want 0", len(got))
	}
}
