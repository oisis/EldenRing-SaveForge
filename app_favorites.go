package main

import (
	"encoding/binary"
	"fmt"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
	"github.com/oisis/EldenRing-SaveForge/backend/db/data"
)

// FavoriteSlotInfo describes a single Favorites slot in the Mirror.
type FavoriteSlotInfo struct {
	Index  int    `json:"index"`  // absolute slot index (0-14)
	Active bool   `json:"active"` // true if slot has FACE magic
	Safe   bool   `json:"safe"`   // true if not colliding with ProfileSummary
	Name   string `json:"name"`   // preset name if we wrote it, empty otherwise
}

// GetFavoritesStatus returns the state of all 15 Favorites slots.
func (a *App) GetFavoritesStatus() []FavoriteSlotInfo {
	result := make([]FavoriteSlotInfo, core.FavSlotCount)
	if a.save == nil {
		return result
	}

	ud := a.save.UserData10.Data

	for i := 0; i < core.FavSlotCount; i++ {
		off := core.FavBaseOffset + i*core.FavSlotSize
		active := off+core.FavOffAlignment <= len(ud) && string(ud[off+core.FavOffMagic:off+core.FavOffMagic+4]) == "FACE"
		result[i] = FavoriteSlotInfo{
			Index:  i,
			Active: active,
			Safe:   true,
			Name:   a.favSlotNames[i],
		}
	}
	return result
}

// RemoveFavoritePreset clears a Favorites slot in UserData10.
func (a *App) RemoveFavoritePreset(slotIndex int) error {
	if a.save == nil {
		return fmt.Errorf("no save loaded")
	}
	if slotIndex < 0 || slotIndex >= core.FavSlotCount {
		return fmt.Errorf("invalid favorites slot index")
	}

	ud := a.save.UserData10.Data
	off := core.FavBaseOffset + slotIndex*core.FavSlotSize
	if off+core.FavSlotSize > len(ud) {
		return fmt.Errorf("slot offset out of bounds")
	}

	// Zero out the entire slot
	for i := 0; i < core.FavSlotSize; i++ {
		ud[off+i] = 0
	}
	delete(a.favSlotNames, slotIndex)
	return nil
}

// ApplyMirrorFavoriteToCharacter copies the appearance from a Mirror Favorites slot
// onto a character's FaceData blob, replicating the in-game "Apply" action.
//
// Algorithm (per spec/31):
//   - Model IDs (32 B), face shape (64 B), body proportions (7 B), and skin & cosmetics
//     (91 B) are copied verbatim from the preset slot to the character's FaceData blob.
//   - The unk0x6c block (64 B at FaceData offset 0x70) is preserved; the game does NOT
//     overwrite it on apply.
//   - slot.Player.Gender is set from preset body_type (Mirror stores body_type inverted
//     from the slot's Gender field: body_type=0 → Gender=1 male, body_type=1 → Gender=0 female).
//   - Trailing flags at FaceData offset 0x12D..0x12E are zeroed (game does this on apply).
//
// Equipment handles are NOT cleared. The game zeroes gender-specific equipment to avoid
// model mismatches; we leave gear intact and let the user decide.
func (a *App) ApplyMirrorFavoriteToCharacter(charIndex, mirrorSlotIndex int) error {
	if a.save == nil {
		return fmt.Errorf("no save loaded")
	}
	if charIndex < 0 || charIndex >= 10 {
		return fmt.Errorf("invalid character index")
	}
	if mirrorSlotIndex < 0 || mirrorSlotIndex >= core.FavSlotCount {
		return fmt.Errorf("invalid mirror slot index")
	}

	ud := a.save.UserData10.Data
	mirrorOff := core.FavBaseOffset + mirrorSlotIndex*core.FavSlotSize
	if mirrorOff+core.FavSlotSize > len(ud) {
		return fmt.Errorf("mirror slot offset out of bounds")
	}
	if string(ud[mirrorOff+core.FavOffMagic:mirrorOff+core.FavOffMagic+4]) != "FACE" {
		return fmt.Errorf("mirror slot %d is empty", mirrorSlotIndex)
	}

	slot := &a.save.Slots[charIndex]
	fd := slot.FaceDataStart()
	if fd < 0 || fd+core.FaceDataBlobSize > len(slot.Data) {
		return fmt.Errorf("FaceData blob out of bounds: start=0x%X", fd)
	}

	a.pushUndo(charIndex)

	// Model IDs (32 B): preset[0x24..0x44] → slot[fd+0x10..0x30]
	copy(slot.Data[fd+core.FDOffFaceModel:fd+core.FDOffFaceModel+32],
		ud[mirrorOff+core.FavOffModelIDs:mirrorOff+core.FavOffModelIDs+32])

	// FaceShape (64 B): preset[0x44..0x84] → slot[fd+0x30..0x70]
	copy(slot.Data[fd+core.FDOffFaceShape:fd+core.FDOffFaceShape+64],
		ud[mirrorOff+core.FavOffFaceShape:mirrorOff+core.FavOffFaceShape+64])

	// PRESERVE unk0x6c at slot[fd+0x70..0xB0] — game does NOT touch this.

	// Body proportions (7 B): preset[0xC4..0xCB] → slot[fd+0xB0..0xB7]
	copy(slot.Data[fd+core.FDOffHead:fd+core.FDOffHead+7],
		ud[mirrorOff+core.FavOffBody:mirrorOff+core.FavOffBody+7])

	// Skin & cosmetics (91 B): preset[0xCB..0x126] → slot[fd+0xB7..0x112]
	copy(slot.Data[fd+core.FDOffSkinR:fd+core.FDOffSkinR+91],
		ud[mirrorOff+core.FavOffSkin:mirrorOff+core.FavOffSkin+91])

	// Trailing flags: observed game behavior is to zero bytes 0x125 and 0x126 on apply
	// (byte 0x124 stays 0x01). Semantics unknown — see tmp/re-character/facedata_dump.txt.
	slot.Data[fd+0x125] = 0
	slot.Data[fd+0x126] = 0

	// Gender flip — Mirror body_type is INVERTED relative to slot.Gender.
	if ud[mirrorOff+core.FavOffBodyType] == 0 {
		slot.Player.Gender = 1 // male
	} else {
		slot.Player.Gender = 0 // female
	}

	return nil
}

// WriteSelectedToFavorites writes selected presets to the next available safe Favorites slots.
// Returns the number of presets written.
func (a *App) WriteSelectedToFavorites(charIndex int, presetNames []string) (int, error) {
	if a.save == nil {
		return 0, fmt.Errorf("no save loaded")
	}
	if charIndex < 0 || charIndex >= 10 {
		return 0, fmt.Errorf("invalid character index")
	}
	if len(presetNames) == 0 {
		return 0, nil
	}

	ud := a.save.UserData10.Data
	slot := &a.save.Slots[charIndex]

	// Find available slots
	var freeSlots []int
	for s := 0; s < core.FavSlotCount; s++ {
		off := core.FavBaseOffset + s*core.FavSlotSize
		if off+core.FavOffAlignment > len(ud) {
			continue
		}
		if string(ud[off+core.FavOffMagic:off+core.FavOffMagic+4]) != "FACE" {
			freeSlots = append(freeSlots, s)
		}
	}

	if len(presetNames) > len(freeSlots) {
		return 0, fmt.Errorf("not enough free slots: need %d, have %d", len(presetNames), len(freeSlots))
	}

	// Read unknown block from character slot
	var unkBlock [64]byte
	fd := slot.FaceDataStart()
	if fd >= 0 && fd+core.FaceDataBlobSize <= len(slot.Data) {
		copy(unkBlock[:], slot.Data[fd+core.FDOffUnknownBlock:fd+core.FDOffUnknownBlock+64])
	}

	written := 0
	for i, name := range presetNames {
		preset := findPresetByName(name)
		if preset == nil {
			continue
		}

		safeIdx := freeSlots[i]
		slotOff := core.FavBaseOffset + safeIdx*core.FavSlotSize

		buf := make([]byte, core.FavSlotSize)

		// Slot header
		binary.LittleEndian.PutUint16(buf[0x00:], core.FavHeaderMagicU16)
		binary.LittleEndian.PutUint32(buf[0x04:], core.FavHeaderUnk)
		buf[core.FavOffBodyFlag] = 1
		if preset.BodyType == 1 {
			buf[core.FavOffBodyType] = 0 // male in Favorites
		} else {
			buf[core.FavOffBodyType] = 1 // female in Favorites
		}

		// FACE block header
		copy(buf[core.FavOffMagic:], []byte("FACE"))
		binary.LittleEndian.PutUint32(buf[core.FavOffAlignment:], 4)
		binary.LittleEndian.PutUint32(buf[core.FavOffInnerSize:], 0x120)

		// Model IDs — male only, female skipped (no mapping)
		if preset.BodyType == 1 {
			writeModel := func(off int, uiVal uint8) {
				val := uiVal
				if val > 0 {
					val--
				}
				binary.LittleEndian.PutUint32(buf[core.FavOffModelIDs+(off*4):], uint32(val))
			}
			// Hair: lookup table first, fallback to UI-1 for unmapped styles
			if partsId, ok := data.LookupMaleHairPartsID(preset.HairModel); ok {
				binary.LittleEndian.PutUint32(buf[core.FavOffModelIDs+1*4:], uint32(partsId))
			} else if preset.HairModel > 0 {
				binary.LittleEndian.PutUint32(buf[core.FavOffModelIDs+1*4:], uint32(preset.HairModel-1))
			}
			writeModel(2, preset.EyeModel)
			writeModel(3, preset.EyebrowModel)
			writeModel(4, preset.BeardModel)
			writeModel(5, preset.EyepatchModel)
			writeModel(6, preset.DecalModel)
			writeModel(7, preset.EyelashModel)
		}

		copy(buf[core.FavOffFaceShape:], preset.FaceShape[:])
		copy(buf[core.FavOffUnkBlock:], unkBlock[:])
		copy(buf[core.FavOffBody:], preset.Body[:])
		copy(buf[core.FavOffSkin:], preset.Skin[:])

		copy(ud[slotOff:], buf)
		a.favSlotNames[safeIdx] = name
		written++
	}

	return written, nil
}
