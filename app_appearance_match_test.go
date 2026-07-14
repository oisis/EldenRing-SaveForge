package main

import (
	"bytes"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
	"github.com/oisis/EldenRing-SaveForge/backend/db/data"
)

// typeAPreset and typeBPreset are real generated presets used across the
// matcher tests. The Type B pick is a D1-converted preset (Casca), so the
// matcher is exercised against the verified female UI→PartsId mapping.
const (
	typeAPreset = "Geralt of Rivia, the Witcher"
	typeBPreset = "Casca, Berserk's Band of the Falcon Commander"
)

// applyToFreshChar builds an in-memory character slot and applies presetName to
// it via the production writer, so the slot is an exact resolved match.
func applyToFreshChar(t *testing.T, presetName string) *App {
	t.Helper()
	app := &App{save: &core.SaveFile{}}
	app.save.Slots[0] = core.SaveSlot{
		Data:           make([]byte, core.FaceDataBlobSize),
		FaceDataOffset: core.FaceDataBlobSize, // → FaceDataStart() == 0
	}
	if err := app.ApplyPresetToCharacter(0, presetName); err != nil {
		t.Fatalf("ApplyPresetToCharacter(%q): %v", presetName, err)
	}
	return app
}

// writeToFreshFav writes presetName into Mirror slot 0 via the production
// writer and returns the app plus the raw slot-0 entry bytes.
func writeToFreshFav(t *testing.T, presetName string) (*App, int) {
	t.Helper()
	app := &App{save: &core.SaveFile{}, favSlotNames: make(map[int]string)}
	app.save.UserData10.Data = make([]byte, 0x60000)
	app.save.Slots[0] = core.SaveSlot{
		Data:           make([]byte, core.FaceDataBlobSize),
		FaceDataOffset: core.FaceDataBlobSize,
	}
	if n, err := app.WriteSelectedToFavorites(0, []string{presetName}); err != nil || n != 1 {
		t.Fatalf("WriteSelectedToFavorites(%q) = (%d, %v), want (1, nil)", presetName, n, err)
	}
	return app, core.FavBaseOffset
}

func TestCharacterAppearanceMatch_ExactRoundTrip(t *testing.T) {
	for _, name := range []string{typeAPreset, typeBPreset} {
		t.Run(name, func(t *testing.T) {
			app := applyToFreshChar(t, name)
			got := matchCharacterAppearance(&app.save.Slots[0])
			if got == nil || got.Name != name {
				t.Fatalf("matchCharacterAppearance = %v, want %q", got, name)
			}
		})
	}
}

func TestMirrorAppearanceMatch_ExactRoundTrip(t *testing.T) {
	for _, name := range []string{typeAPreset, typeBPreset} {
		t.Run(name, func(t *testing.T) {
			app, slotOff := writeToFreshFav(t, name)
			entry := app.save.UserData10.Data[slotOff : slotOff+core.FavSlotSize]
			got := matchMirrorAppearance(entry)
			if got == nil || got.Name != name {
				t.Fatalf("matchMirrorAppearance = %v, want %q", got, name)
			}
		})
	}
}

func TestCharacterAppearanceMatch_Mismatches(t *testing.T) {
	fd := 0 // FaceDataStart() for these fixtures

	t.Run("one model byte", func(t *testing.T) {
		app := applyToFreshChar(t, typeAPreset)
		app.save.Slots[0].Data[fd+core.FDOffFaceModel]++
		if got := matchCharacterAppearance(&app.save.Slots[0]); got != nil {
			t.Fatalf("model mismatch matched %q, want nil", got.Name)
		}
	})

	t.Run("one slider byte", func(t *testing.T) {
		app := applyToFreshChar(t, typeAPreset)
		app.save.Slots[0].Data[fd+core.FDOffFaceShape] ^= 0xFF
		if got := matchCharacterAppearance(&app.save.Slots[0]); got != nil {
			t.Fatalf("slider mismatch matched %q, want nil", got.Name)
		}
	})

	t.Run("voice type", func(t *testing.T) {
		app := applyToFreshChar(t, typeAPreset)
		app.save.Slots[0].Player.VoiceType ^= 0xFF // guaranteed different from preset
		if got := matchCharacterAppearance(&app.save.Slots[0]); got != nil {
			t.Fatalf("voice mismatch matched %q, want nil", got.Name)
		}
	})
}

func TestMirrorAppearanceMatch_Mismatches(t *testing.T) {
	t.Run("one model byte", func(t *testing.T) {
		app, slotOff := writeToFreshFav(t, typeAPreset)
		app.save.UserData10.Data[slotOff+core.FavOffModelIDs]++
		entry := app.save.UserData10.Data[slotOff : slotOff+core.FavSlotSize]
		if got := matchMirrorAppearance(entry); got != nil {
			t.Fatalf("model mismatch matched %q, want nil", got.Name)
		}
	})

	t.Run("one slider byte", func(t *testing.T) {
		app, slotOff := writeToFreshFav(t, typeAPreset)
		app.save.UserData10.Data[slotOff+core.FavOffFaceShape] ^= 0xFF
		entry := app.save.UserData10.Data[slotOff : slotOff+core.FavSlotSize]
		if got := matchMirrorAppearance(entry); got != nil {
			t.Fatalf("slider mismatch matched %q, want nil", got.Name)
		}
	})
}

// TestAppearanceMatch_ReadOnly proves neither matcher mutates its input.
func TestAppearanceMatch_ReadOnly(t *testing.T) {
	app := applyToFreshChar(t, typeBPreset)
	before := append([]byte(nil), app.save.Slots[0].Data...)
	beforeGender, beforeVoice := app.save.Slots[0].Player.Gender, app.save.Slots[0].Player.VoiceType
	matchCharacterAppearance(&app.save.Slots[0])
	if !bytes.Equal(before, app.save.Slots[0].Data) {
		t.Error("matchCharacterAppearance mutated slot.Data")
	}
	if app.save.Slots[0].Player.Gender != beforeGender || app.save.Slots[0].Player.VoiceType != beforeVoice {
		t.Error("matchCharacterAppearance mutated Player fields")
	}

	favApp, slotOff := writeToFreshFav(t, typeBPreset)
	udBefore := append([]byte(nil), favApp.save.UserData10.Data...)
	matchMirrorAppearance(favApp.save.UserData10.Data[slotOff : slotOff+core.FavSlotSize])
	if !bytes.Equal(udBefore, favApp.save.UserData10.Data) {
		t.Error("matchMirrorAppearance mutated UserData10")
	}
}

// TestAppearanceMatch_Ambiguous proves both matchers refuse to guess: when two
// presets resolve to the identical appearance, they return nil rather than
// picking the first by order. A copy of the Type A preset (distinct Name only)
// is appended to data.Presets and restored on cleanup.
func TestAppearanceMatch_Ambiguous(t *testing.T) {
	orig := data.Presets
	t.Cleanup(func() { data.Presets = orig })

	dup := *findPresetByName(typeAPreset)
	dup.Name = typeAPreset + " (duplicate)"
	data.Presets = append(append([]data.AppearancePreset(nil), orig...), dup)

	app := applyToFreshChar(t, typeAPreset)
	if got := matchCharacterAppearance(&app.save.Slots[0]); got != nil {
		t.Fatalf("ambiguous character match = %q, want nil", got.Name)
	}

	favApp, slotOff := writeToFreshFav(t, typeAPreset)
	entry := favApp.save.UserData10.Data[slotOff : slotOff+core.FavSlotSize]
	if got := matchMirrorAppearance(entry); got != nil {
		t.Fatalf("ambiguous mirror match = %q, want nil", got.Name)
	}
}
