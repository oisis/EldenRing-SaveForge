package main

import (
	"encoding/binary"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
)

// TestApplyAndFavorites_D1TypeBPresets is the D1 regression: Casca and Fire
// Keeper — newly converted to Type B — must both directly Apply and write to
// Mirror Favorites with their exact verified raw model IDs and the female
// Gender / Mirror body-type byte. Uses the real generated presets, no injected
// fixtures and no guessed values (unmapped Type B would be rejected upstream).
func TestApplyAndFavorites_D1TypeBPresets(t *testing.T) {
	cases := []struct {
		name   string
		models [8]uint32
	}{
		{"Casca, Berserk's Band of the Falcon Commander", [8]uint32{20, 100, 0, 2, 0, 0, 0, 3}},
		{"Fire Keeper, the Dark Souls 3 NPC", [8]uint32{50, 106, 0, 9, 0, 0, 17, 3}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if p := findPresetByName(c.name); p == nil || p.BodyType != 0 {
				t.Fatalf("fixture assumption broken: %q must be a known Type B preset", c.name)
			}

			// --- Direct Apply onto a Type A character ---
			app := &App{save: &core.SaveFile{}}
			app.save.Slots[0] = core.SaveSlot{
				Data:           make([]byte, core.FaceDataBlobSize),
				FaceDataOffset: core.FaceDataBlobSize, // → FaceDataStart() == 0
			}
			app.save.Slots[0].Player.Gender = 1 // Type A before
			if err := app.ApplyPresetToCharacter(0, c.name); err != nil {
				t.Fatalf("ApplyPresetToCharacter: %v", err)
			}
			fd := app.save.Slots[0].FaceDataStart()
			for i, exp := range c.models {
				got := binary.LittleEndian.Uint32(app.save.Slots[0].Data[fd+core.FDOffFaceModel+i*4:])
				if got != exp {
					t.Errorf("Apply model[%d] = %d, want %d", i, got, exp)
				}
			}
			if g := app.save.Slots[0].Player.Gender; g != 0 {
				t.Errorf("Apply gender = %d, want 0 (female/Type B)", g)
			}

			// --- Add to Mirror Favorites ---
			fav := &App{save: &core.SaveFile{}, favSlotNames: make(map[int]string)}
			fav.save.UserData10.Data = make([]byte, 0x60000)
			fav.save.Slots[0] = core.SaveSlot{
				Data:           make([]byte, core.FaceDataBlobSize),
				FaceDataOffset: core.FaceDataBlobSize,
			}
			written, err := fav.WriteSelectedToFavorites(0, []string{c.name})
			if err != nil || written != 1 {
				t.Fatalf("WriteSelectedToFavorites = (%d, %v), want (1, nil)", written, err)
			}
			slotOff := core.FavBaseOffset
			ud := fav.save.UserData10.Data
			// Type B → Mirror body-type byte 1 (female, inverted vs gender).
			if bt := ud[slotOff+core.FavOffBodyType]; bt != 1 {
				t.Errorf("Mirror body-type byte = %d, want 1 (Type B female)", bt)
			}
			for i, exp := range c.models {
				got := binary.LittleEndian.Uint32(ud[slotOff+core.FavOffModelIDs+i*4:])
				if got != exp {
					t.Errorf("Mirror model[%d] = %d, want %d", i, got, exp)
				}
			}
		})
	}
}
