package main

import (
	"bytes"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
)

// TestGetCharacterAppearancePreset covers the D3 read path: an exact match
// returns the canonical PresetInfo (incl. a Type B D1 preset), a one-byte
// mismatch returns nil, and the read never mutates the slot.
func TestGetCharacterAppearancePreset(t *testing.T) {
	t.Run("exact Type B match", func(t *testing.T) {
		app := applyToFreshChar(t, typeBPreset)
		info, err := app.GetCharacterAppearancePreset(0)
		if err != nil {
			t.Fatalf("GetCharacterAppearancePreset: %v", err)
		}
		if info == nil || info.Name != typeBPreset || info.BodyType != "Type B" {
			t.Fatalf("info = %+v, want name %q BodyType Type B", info, typeBPreset)
		}
	})

	t.Run("one-byte mismatch returns nil", func(t *testing.T) {
		app := applyToFreshChar(t, typeAPreset)
		app.save.Slots[0].Data[core.FDOffFaceModel]++ // FaceDataStart()==0
		info, err := app.GetCharacterAppearancePreset(0)
		if err != nil {
			t.Fatalf("GetCharacterAppearancePreset: %v", err)
		}
		if info != nil {
			t.Fatalf("info = %+v, want nil", info)
		}
	})

	t.Run("read is non-mutating", func(t *testing.T) {
		app := applyToFreshChar(t, typeBPreset)
		before := append([]byte(nil), app.save.Slots[0].Data...)
		if _, err := app.GetCharacterAppearancePreset(0); err != nil {
			t.Fatalf("GetCharacterAppearancePreset: %v", err)
		}
		if !bytes.Equal(before, app.save.Slots[0].Data) {
			t.Error("GetCharacterAppearancePreset mutated slot.Data")
		}
	})
}

// TestGetFavoritesStatusAppearance covers the D3 Mirror path: an exact entry is
// recognised after favSlotNames is cleared (reload), an unmatched active entry
// stays unnamed with empty Image, and the read never mutates UserData10.
func TestGetFavoritesStatusAppearance(t *testing.T) {
	t.Run("recognised after favSlotNames cleared", func(t *testing.T) {
		app, _ := writeToFreshFav(t, typeBPreset)
		app.favSlotNames = make(map[int]string) // simulate a reload

		status := app.GetFavoritesStatus()
		if !status[0].Active {
			t.Fatal("slot 0 should be active")
		}
		if status[0].Name != typeBPreset {
			t.Errorf("Name = %q, want %q", status[0].Name, typeBPreset)
		}
		if status[0].Image == "" {
			t.Error("matched slot has empty Image")
		}
	})

	t.Run("unmatched active slot stays unnamed with empty image", func(t *testing.T) {
		app, slotOff := writeToFreshFav(t, typeBPreset)
		app.save.UserData10.Data[slotOff+core.FavOffModelIDs]++ // break the match
		app.favSlotNames = make(map[int]string)                 // reload

		status := app.GetFavoritesStatus()
		if !status[0].Active {
			t.Fatal("slot 0 should still be active")
		}
		if status[0].Name != "" || status[0].Image != "" {
			t.Errorf("unmatched slot = {Name:%q Image:%q}, want both empty", status[0].Name, status[0].Image)
		}
	})

	t.Run("read is non-mutating", func(t *testing.T) {
		app, _ := writeToFreshFav(t, typeBPreset)
		before := append([]byte(nil), app.save.UserData10.Data...)
		app.GetFavoritesStatus()
		if !bytes.Equal(before, app.save.UserData10.Data) {
			t.Error("GetFavoritesStatus mutated UserData10")
		}
	})
}
