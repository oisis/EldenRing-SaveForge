package main

import (
	"encoding/binary"
	"encoding/hex"
	"strconv"
	"strings"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
)

// writeFavoritesFixture builds an App with a single character slot exposing a
// valid FaceData blob at fd=0 (its unknown block seeded to a distinct non-0xFF
// pattern so the planned unknown_block_hex differs from the stale slot) and a
// UserData10 whose Favorites region is 0xFF-seeded — every slot is free (no FACE
// magic) yet stale/non-zero, so every field the writer plans differs from before.
// The first free slot is therefore slot 0.
func writeFavoritesFixture(t *testing.T) *App {
	t.Helper()
	const charIdx = 0

	app := NewApp()
	app.save = &core.SaveFile{}

	slot := &app.save.Slots[charIdx]
	slot.Version = 1
	slot.Data = make([]byte, 1024)
	for i := range slot.Data {
		slot.Data[i] = 0xFF
	}
	slot.FaceDataOffset = core.FaceDataBlobSize // FaceDataStart() == 0
	slot.GaMap = map[uint32]uint32{}
	slot.Player.Gender = 0    // Geralt is BodyType 1
	slot.Player.VoiceType = 0 // Geralt is VoiceType 2

	// Distinct unknown block so the copied planned value differs from the 0xFF
	// stale Favorites slot bytes.
	for i := 0; i < 64; i++ {
		slot.Data[core.FDOffUnknownBlock+i] = byte(i)
	}

	// 0xFF-seeded UserData10: all Favorites slots are free but non-zero.
	ud := make([]byte, 0x60000)
	for i := range ud {
		ud[i] = 0xFF
	}
	app.save.UserData10.Data = ud

	return app
}

// TestWriteSelectedToFavoritesDiagnosticSuccess drives a real WriteSelectedToFavorites
// writing the Geralt preset into the first free Favorites slot (slot 0) whose
// before bytes are stale/non-zero, and asserts the full before -> planned ->
// finished lifecycle for all 14 fields (header hex, eight raw models, five hex
// blocks) — with exact, full-length values and the real post-write UserData10
// matching every finished value.
func TestWriteSelectedToFavoritesDiagnosticSuccess(t *testing.T) {
	app := writeFavoritesFixture(t)
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

	// Reconstruct the exact target buffer the shared helper builds, so the test
	// derives expectations from the writer's own bytes rather than duplicating the
	// layout. unkBlock mirrors the fixture's seeded FaceData unknown block.
	var unkBlock [64]byte
	for i := range unkBlock {
		unkBlock[i] = byte(i)
	}
	buf := buildFavoriteSlotBuffer(&resolved, unkBlock)

	dec := func(b []byte, off int) string {
		return strconv.FormatUint(uint64(binary.LittleEndian.Uint32(b[off:])), 10)
	}

	// Planned/finished value per field, all derived from the writer's buffer.
	want := map[string]string{
		"favorite_slot_0_header_hex":           hex.EncodeToString(buf[0:core.FavOffModelIDs]),
		"favorite_slot_0_face_model":           dec(buf, core.FavOffModelIDs+0*4),
		"favorite_slot_0_hair_model":           dec(buf, core.FavOffModelIDs+1*4),
		"favorite_slot_0_eye_model":            dec(buf, core.FavOffModelIDs+2*4),
		"favorite_slot_0_eyebrow_model":        dec(buf, core.FavOffModelIDs+3*4),
		"favorite_slot_0_beard_model":          dec(buf, core.FavOffModelIDs+4*4),
		"favorite_slot_0_eyepatch_model":       dec(buf, core.FavOffModelIDs+5*4),
		"favorite_slot_0_decal_model":          dec(buf, core.FavOffModelIDs+6*4),
		"favorite_slot_0_eyelash_model":        dec(buf, core.FavOffModelIDs+7*4),
		"favorite_slot_0_face_shape_hex":       hex.EncodeToString(buf[core.FavOffFaceShape : core.FavOffFaceShape+64]),
		"favorite_slot_0_unknown_block_hex":    hex.EncodeToString(buf[core.FavOffUnkBlock : core.FavOffUnkBlock+64]),
		"favorite_slot_0_body_proportions_hex": hex.EncodeToString(buf[core.FavOffBody : core.FavOffBody+7]),
		"favorite_slot_0_skin_cosmetics_hex":   hex.EncodeToString(buf[core.FavOffSkin : core.FavOffSkin+91]),
		"favorite_slot_0_trailing_hex":         hex.EncodeToString(buf[core.FavOffSkin+91 : core.FavSlotSize]),
	}

	// Every field's before is read from the pristine 0xFF-seeded slot.
	const modelBefore = "4294967295" // 0xFFFFFFFF
	wantBefore := map[string]string{
		"favorite_slot_0_header_hex":           strings.Repeat("ff", core.FavOffModelIDs),
		"favorite_slot_0_face_model":           modelBefore,
		"favorite_slot_0_hair_model":           modelBefore,
		"favorite_slot_0_eye_model":            modelBefore,
		"favorite_slot_0_eyebrow_model":        modelBefore,
		"favorite_slot_0_beard_model":          modelBefore,
		"favorite_slot_0_eyepatch_model":       modelBefore,
		"favorite_slot_0_decal_model":          modelBefore,
		"favorite_slot_0_eyelash_model":        modelBefore,
		"favorite_slot_0_face_shape_hex":       strings.Repeat("ff", 64),
		"favorite_slot_0_unknown_block_hex":    strings.Repeat("ff", 64),
		"favorite_slot_0_body_proportions_hex": strings.Repeat("ff", 7),
		"favorite_slot_0_skin_cosmetics_hex":   strings.Repeat("ff", 91),
		"favorite_slot_0_trailing_hex":         strings.Repeat("ff", core.FavSlotSize-(core.FavOffSkin+91)),
	}

	written, err := app.WriteSelectedToFavorites(0, []string{presetName})
	if err != nil {
		t.Fatalf("WriteSelectedToFavorites: %v", err)
	}
	if written != 1 {
		t.Fatalf("written = %d, want 1", written)
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
			if got := operationField(rec, "action"); got != actionWriteFavorites {
				t.Errorf("field %q phase %d: action = %q, want %q", field, i, got, actionWriteFavorites)
			}
			if got := operationField(rec, "character_index"); got != "0" {
				t.Errorf("field %q phase %d: character_index = %q, want 0", field, i, got)
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

	// Exactly 14 fields × 3 phases and no stray records.
	if got, wantN := len(records), 14*3; got != wantN {
		t.Errorf("got %d character records, want %d (14 fields × 3 phases)", got, wantN)
	}

	// Exact hex block lengths (36/64/64/7/91/10 bytes -> 72/128/128/14/182/20 chars).
	lenChecks := []struct {
		field  string
		length int
	}{
		{"favorite_slot_0_header_hex", 2 * core.FavOffModelIDs},
		{"favorite_slot_0_face_shape_hex", 128},
		{"favorite_slot_0_unknown_block_hex", 128},
		{"favorite_slot_0_body_proportions_hex", 14},
		{"favorite_slot_0_skin_cosmetics_hex", 182},
		{"favorite_slot_0_trailing_hex", 2 * (core.FavSlotSize - (core.FavOffSkin + 91))},
	}
	for _, c := range lenChecks {
		if got := len(want[c.field]); got != c.length {
			t.Errorf("%s len = %d, want %d", c.field, got, c.length)
		}
	}

	// The fields exactly partition the whole 304-byte slot.
	total := core.FavOffModelIDs /*header*/ + 8*4 /*models*/ + 64 + 64 + 7 + 91 + (core.FavSlotSize - (core.FavOffSkin + 91))
	if total != core.FavSlotSize {
		t.Fatalf("field coverage = %d bytes, want FavSlotSize %d", total, core.FavSlotSize)
	}

	// The real UserData10 after the write must match every finished value.
	ud := app.save.UserData10.Data
	slotOff := core.FavBaseOffset + 0*core.FavSlotSize
	if got := hex.EncodeToString(ud[slotOff : slotOff+core.FavOffModelIDs]); got != want["favorite_slot_0_header_hex"] {
		t.Errorf("real header_hex = %q, want %q", got, want["favorite_slot_0_header_hex"])
	}
	modelNames := []string{"face_model", "hair_model", "eye_model", "eyebrow_model", "beard_model", "eyepatch_model", "decal_model", "eyelash_model"}
	for m, name := range modelNames {
		if got := dec(ud[slotOff:], core.FavOffModelIDs+m*4); got != want["favorite_slot_0_"+name] {
			t.Errorf("real %s = %q, want %q", name, got, want["favorite_slot_0_"+name])
		}
	}
	realHexBlocks := []struct {
		field  string
		start  int
		length int
	}{
		{"favorite_slot_0_face_shape_hex", core.FavOffFaceShape, 64},
		{"favorite_slot_0_unknown_block_hex", core.FavOffUnkBlock, 64},
		{"favorite_slot_0_body_proportions_hex", core.FavOffBody, 7},
		{"favorite_slot_0_skin_cosmetics_hex", core.FavOffSkin, 91},
		{"favorite_slot_0_trailing_hex", core.FavOffSkin + 91, core.FavSlotSize - (core.FavOffSkin + 91)},
	}
	for _, b := range realHexBlocks {
		if got := hex.EncodeToString(ud[slotOff+b.start : slotOff+b.start+b.length]); got != want[b.field] {
			t.Errorf("real %s = %q, want %q", b.field, got, want[b.field])
		}
	}

	// All before precede all planned precede all finished.
	assertPhaseGrouping(t, records)
}

// TestWriteSelectedToFavoritesDiagnosticTruncatedSlotNoPanic guards the case where
// the existing free-slot guard (FavOffAlignment) selects slot 0 but the truncated
// UserData10 does not hold its full FavSlotSize bytes. The planner must not slice
// the full range (which would panic); it plans nothing for the truncated slot while
// the writer's partial copy and its result (written, favSlotNames) stay exactly as
// on the old path.
func TestWriteSelectedToFavoritesDiagnosticTruncatedSlotNoPanic(t *testing.T) {
	app := writeFavoritesFixture(t)
	enableDebugJournal(t, app)

	const presetName = "Geralt of Rivia, the Witcher"
	if findPresetByName(presetName) == nil {
		t.Fatalf("findPresetByName(%q) = nil", presetName)
	}

	// slot 0 offset 0x154: passes the FavOffAlignment guard at len 0x170, but its
	// full 0x130-byte range (ending at 0x284) is out of bounds.
	truncated := core.FavBaseOffset + core.FavOffAlignment // 0x170
	if truncated >= core.FavBaseOffset+core.FavSlotSize {
		t.Fatalf("test setup: len %d not truncated below full slot", truncated)
	}
	ud := make([]byte, truncated)
	for i := range ud {
		ud[i] = 0xFF
	}
	app.save.UserData10.Data = ud

	written, err := app.WriteSelectedToFavorites(0, []string{presetName})
	if err != nil {
		t.Fatalf("WriteSelectedToFavorites: %v", err)
	}
	if written != 1 {
		t.Fatalf("written = %d, want 1 (writer partial copy unchanged)", written)
	}
	if got := app.favSlotNames[0]; got != presetName {
		t.Fatalf("favSlotNames[0] = %q, want %q", got, presetName)
	}
	if got := characterRecords(app.journal.Tail()); len(got) != 0 {
		t.Fatalf("truncated slot emitted %d records, want 0", len(got))
	}
}

// TestWriteSelectedToFavoritesDiagnosticNoPresetsEmitsNothing verifies that a call
// which writes no actual preset (only unknown names) emits zero field records — no
// slot is planned, so no before/planned/finished records are produced.
func TestWriteSelectedToFavoritesDiagnosticNoPresetsEmitsNothing(t *testing.T) {
	app := writeFavoritesFixture(t)
	enableDebugJournal(t, app)

	written, err := app.WriteSelectedToFavorites(0, []string{"not a real preset name"})
	if err != nil {
		t.Fatalf("WriteSelectedToFavorites: %v", err)
	}
	if written != 0 {
		t.Fatalf("written = %d, want 0", written)
	}

	if got := characterRecords(app.journal.Tail()); len(got) != 0 {
		t.Fatalf("no-preset call emitted %d records, want 0", len(got))
	}
}
