package main

import (
	"encoding/binary"
	"encoding/hex"
	"strconv"
	"strings"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
)

// removeFavoritesFixture builds an App whose Favorites slot slotIndex is active
// (FACE magic) and fully stale/non-zero: the whole FavSlotSize region is 0xFF-seeded
// so every one of the 14 planned fields differs from the all-zero planned buffer,
// and a favSlotNames entry is present so the delete can be asserted. Returns the App
// and a pristine snapshot of the slot bytes for deriving before expectations.
func removeFavoritesFixture(t *testing.T, slotIndex int, active bool) (*App, []byte) {
	t.Helper()

	app := NewApp()
	app.save = &core.SaveFile{}

	ud := make([]byte, 0x60000)
	for i := range ud {
		ud[i] = 0xFF
	}
	off := core.FavBaseOffset + slotIndex*core.FavSlotSize
	if active {
		copy(ud[off+core.FavOffMagic:], []byte("FACE"))
	} else {
		// Ensure the magic is NOT "FACE" so the slot reads as empty/inactive.
		copy(ud[off+core.FavOffMagic:], []byte("\x00\x00\x00\x00"))
	}
	app.save.UserData10.Data = ud
	app.favSlotNames[slotIndex] = "Some Preset Name"

	snapshot := make([]byte, core.FavSlotSize)
	copy(snapshot, ud[off:off+core.FavSlotSize])
	return app, snapshot
}

// removeFavoriteFieldSpec is one of the 14 fields planWriteFavoriteSlot partitions
// the 304-byte slot into. isModel selects the uint32-LE decimal reader; otherwise a
// fixed-length hex block from off.
type removeFavoriteFieldSpec struct {
	name    string
	isModel bool
	off     int
	length  int
}

func removeFavoriteFieldSpecs() []removeFavoriteFieldSpec {
	return []removeFavoriteFieldSpec{
		{"header_hex", false, 0, core.FavOffModelIDs},
		{"face_model", true, core.FavOffModelIDs + 0*4, 0},
		{"hair_model", true, core.FavOffModelIDs + 1*4, 0},
		{"eye_model", true, core.FavOffModelIDs + 2*4, 0},
		{"eyebrow_model", true, core.FavOffModelIDs + 3*4, 0},
		{"beard_model", true, core.FavOffModelIDs + 4*4, 0},
		{"eyepatch_model", true, core.FavOffModelIDs + 5*4, 0},
		{"decal_model", true, core.FavOffModelIDs + 6*4, 0},
		{"eyelash_model", true, core.FavOffModelIDs + 7*4, 0},
		{"face_shape_hex", false, core.FavOffFaceShape, 64},
		{"unknown_block_hex", false, core.FavOffUnkBlock, 64},
		{"body_proportions_hex", false, core.FavOffBody, 7},
		{"skin_cosmetics_hex", false, core.FavOffSkin, 91},
		{"trailing_hex", false, core.FavOffSkin + 91, core.FavSlotSize - (core.FavOffSkin + 91)},
	}
}

// TestRemoveFavoritePresetDiagnosticSuccess drives a real RemoveFavoritePreset on an
// active, fully non-zero Favorites slot and asserts the full before -> planned ->
// finished lifecycle for all 14 fields: action remove_favorite, character_index=-1,
// planned/finished values that represent a complete zeroing of the whole 304-byte
// partition, the real slot zeroed, and the favSlotNames entry gone.
func TestRemoveFavoritePresetDiagnosticSuccess(t *testing.T) {
	const slotIndex = 3
	app, before := removeFavoritesFixture(t, slotIndex, true)
	enableDebugJournal(t, app)

	prefix := "favorite_slot_" + strconv.Itoa(slotIndex) + "_"
	dec := func(b []byte, off int) string {
		return strconv.FormatUint(uint64(binary.LittleEndian.Uint32(b[off:])), 10)
	}

	// before from the pristine snapshot; planned/finished are a full zeroing.
	wantBefore := map[string]string{}
	wantPlanned := map[string]string{}
	for _, s := range removeFavoriteFieldSpecs() {
		field := prefix + s.name
		if s.isModel {
			wantBefore[field] = dec(before, s.off)
			wantPlanned[field] = "0"
		} else {
			wantBefore[field] = hex.EncodeToString(before[s.off : s.off+s.length])
			wantPlanned[field] = strings.Repeat("00", s.length)
		}
	}

	if err := app.RemoveFavoritePreset(slotIndex); err != nil {
		t.Fatalf("RemoveFavoritePreset: %v", err)
	}

	records := characterRecords(app.journal.Tail())

	for field, planned := range wantPlanned {
		phases := saveCharacterPhases(records, field)
		if len(phases) != 3 {
			t.Fatalf("field %q: got %d records, want 3", field, len(phases))
		}
		wantEvents := []string{eventCharacterChangeBefore, eventCharacterChangePlanned, eventCharacterChangeFinished}
		for i, rec := range phases {
			if rec.Event != wantEvents[i] {
				t.Errorf("field %q phase %d: event = %q, want %q", field, i, rec.Event, wantEvents[i])
			}
			if got := operationField(rec, "action"); got != actionRemoveFavorite {
				t.Errorf("field %q phase %d: action = %q, want %q", field, i, got, actionRemoveFavorite)
			}
			if got := operationField(rec, "character_index"); got != "-1" {
				t.Errorf("field %q phase %d: character_index = %q, want -1", field, i, got)
			}
			if got := operationField(rec, "before"); got != wantBefore[field] {
				t.Errorf("field %q phase %d: before = %q, want %q", field, i, got, wantBefore[field])
			}
		}
		if got := operationField(phases[0], "after"); got != "" {
			t.Errorf("field %q before phase leaked after = %q", field, got)
		}
		if got := operationField(phases[1], "after"); got != planned {
			t.Errorf("field %q planned after = %q, want %q (zeroing)", field, got, planned)
		}
		finished := phases[2]
		if got := operationField(finished, "after"); got != planned {
			t.Errorf("field %q finished after = %q, want %q (real post-zero)", field, got, planned)
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

	// The fields exactly partition the whole 304-byte slot.
	total := core.FavOffModelIDs /*header*/ + 8*4 /*models*/ + 64 + 64 + 7 + 91 + (core.FavSlotSize - (core.FavOffSkin + 91))
	if total != core.FavSlotSize {
		t.Fatalf("field coverage = %d bytes, want FavSlotSize %d", total, core.FavSlotSize)
	}

	// The real slot is fully zeroed and its favSlotNames entry is gone.
	off := core.FavBaseOffset + slotIndex*core.FavSlotSize
	if got := hex.EncodeToString(app.save.UserData10.Data[off : off+core.FavSlotSize]); got != strings.Repeat("00", core.FavSlotSize) {
		t.Errorf("real slot not fully zeroed: %q", got)
	}
	if _, ok := app.favSlotNames[slotIndex]; ok {
		t.Errorf("favSlotNames[%d] still present after remove", slotIndex)
	}

	// All before precede all planned precede all finished.
	assertPhaseGrouping(t, records)
}

// TestRemoveFavoritePresetDiagnosticEmptySlotEmitsNothing verifies that removing an
// inactive (no FACE magic) slot keeps the existing no-op and emits zero field records.
func TestRemoveFavoritePresetDiagnosticEmptySlotEmitsNothing(t *testing.T) {
	const slotIndex = 3
	app, _ := removeFavoritesFixture(t, slotIndex, false)
	enableDebugJournal(t, app)

	if err := app.RemoveFavoritePreset(slotIndex); err != nil {
		t.Fatalf("RemoveFavoritePreset: %v", err)
	}

	if got := characterRecords(app.journal.Tail()); len(got) != 0 {
		t.Fatalf("empty-slot remove emitted %d records, want 0", len(got))
	}
}
