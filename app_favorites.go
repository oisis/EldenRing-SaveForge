package main

import (
	"encoding/binary"
	"fmt"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
)

// FavoriteSlotInfo describes a single Favorites slot in the Mirror.
type FavoriteSlotInfo struct {
	Index  int    `json:"index"`  // absolute slot index (0-14)
	Active bool   `json:"active"` // true if slot has FACE magic
	Safe   bool   `json:"safe"`   // true if not colliding with ProfileSummary
	Name   string `json:"name"`   // canonical preset name if matched, else session name, else empty
	Image  string `json:"image"`  // preset image filename if the entry exactly matches a known preset
}

// GetFavoritesStatus returns the state of all 15 Favorites slots.
func (a *App) GetFavoritesStatus() []FavoriteSlotInfo {
	result := make([]FavoriteSlotInfo, core.FavSlotCount)
	a.saveMu.RLock()
	defer a.saveMu.RUnlock()
	if a.save == nil {
		return result
	}
	// favMu.RLock pairs with the Lock taken by RemoveFavoritePreset /
	// WriteSelectedToFavorites so a reader cannot iterate favSlotNames or
	// UserData10.Data while a peer mutates them (the map iteration would
	// otherwise crash with concurrent-map-writes panic).
	a.favMu.RLock()
	defer a.favMu.RUnlock()

	ud := a.save.UserData10.Data

	for i := 0; i < core.FavSlotCount; i++ {
		off := core.FavBaseOffset + i*core.FavSlotSize
		active := off+core.FavOffAlignment <= len(ud) && string(ud[off+core.FavOffMagic:off+core.FavOffMagic+4]) == "FACE"
		info := FavoriteSlotInfo{
			Index:  i,
			Active: active,
			Safe:   true,
			Name:   a.favSlotNames[i],
		}
		// Exact match wins over the session name and survives a reload (empty
		// favSlotNames): read-only, so it cannot mutate UserData10. An active
		// but unmatched slot keeps its session name (empty after reload → the
		// frontend shows "In-game favorite").
		if active && off+core.FavSlotSize <= len(ud) {
			if p := matchMirrorAppearance(ud[off : off+core.FavSlotSize]); p != nil {
				info.Name = p.Name
				info.Image = p.Image
			}
		}
		result[i] = info
	}
	return result
}

// RemoveFavoritePreset clears a Favorites slot in UserData10.
func (a *App) RemoveFavoritePreset(slotIndex int) error {
	a.saveMu.RLock()
	defer a.saveMu.RUnlock()
	if a.save == nil {
		return fmt.Errorf("no save loaded")
	}
	if slotIndex < 0 || slotIndex >= core.FavSlotCount {
		return fmt.Errorf("invalid favorites slot index")
	}
	a.favMu.Lock()
	defer a.favMu.Unlock()

	ud := a.save.UserData10.Data
	off := core.FavBaseOffset + slotIndex*core.FavSlotSize
	if off+core.FavSlotSize > len(ud) {
		return fmt.Errorf("slot offset out of bounds")
	}

	// Empty slot: removing it is a no-op, so it must NOT create an undo step
	// (an empty snapshot would let a later Undo look like it did something).
	if string(ud[off+core.FavOffMagic:off+core.FavOffMagic+4]) != "FACE" {
		return nil
	}

	// Snapshot the pre-removal state (after all validations, right before the
	// mutation) and push it so RemoveFavoritePreset is undoable too. The shared
	// favUndoStack keeps Add and Remove chronological: Add→Remove→Undo restores
	// the removed entry first, a second Undo reverts the earlier Add.
	a.pushFavUndoLocked(a.buildFavSnapshotLocked())

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
	a.saveMu.RLock()
	defer a.saveMu.RUnlock()
	if a.save == nil {
		return fmt.Errorf("no save loaded")
	}
	if charIndex < 0 || charIndex >= 10 {
		return fmt.Errorf("invalid character index")
	}
	if mirrorSlotIndex < 0 || mirrorSlotIndex >= core.FavSlotCount {
		return fmt.Errorf("invalid mirror slot index")
	}
	// Lock order saveMu → favMu → slotMu (per the App-level hierarchy).
	// favMu.RLock so a concurrent favorites mutator cannot tear the bytes
	// we are about to copy out of UserData10; slotMu.Lock because we
	// mutate slot.Data + slot.Player.Gender below.
	a.favMu.RLock()
	defer a.favMu.RUnlock()
	a.slotMu[charIndex].Lock()
	defer a.slotMu[charIndex].Unlock()

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

	a.pushUndoLocked(charIndex)

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
	a.saveMu.RLock()
	defer a.saveMu.RUnlock()
	if a.save == nil {
		return 0, fmt.Errorf("no save loaded")
	}
	if charIndex < 0 || charIndex >= 10 {
		return 0, fmt.Errorf("invalid character index")
	}
	if len(presetNames) == 0 {
		return 0, nil
	}
	// Lock order saveMu → favMu → slotMu. favMu.Lock because we mutate
	// UserData10 bytes AND favSlotNames; slotMu.Lock because we read
	// FaceData from the character slot.
	a.favMu.Lock()
	defer a.favMu.Unlock()
	a.slotMu[charIndex].Lock()
	defer a.slotMu[charIndex].Unlock()

	ud := a.save.UserData10.Data
	slot := &a.save.Slots[charIndex]

	// Pre-resolve every known preset through the shared appearance builder BEFORE
	// snapshotting or mutating. An unmapped Type B preset (or a mixed batch that
	// contains one) fails atomically here — nothing is written. Unknown names
	// resolve to nil and are skipped by the write loop, preserving prior behavior.
	resolved := make([]*resolvedAppearance, len(presetNames))
	for i, name := range presetNames {
		preset := findPresetByName(name)
		if preset == nil {
			continue
		}
		r, err := resolveAppearance(preset)
		if err != nil {
			return 0, err
		}
		resolved[i] = &r
	}

	// Snapshot the favorites state BEFORE any mutation so the whole Add
	// (even a multi-preset one) reverts as a single logical undo step.
	// Pushed onto the stack only if we actually write ≥1 preset.
	undoSnap := a.buildFavSnapshotLocked()

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
		r := resolved[i]
		if r == nil {
			continue
		}

		safeIdx := freeSlots[i]
		slotOff := core.FavBaseOffset + safeIdx*core.FavSlotSize

		buf := make([]byte, core.FavSlotSize)

		// Slot header
		binary.LittleEndian.PutUint16(buf[0x00:], core.FavHeaderMagicU16)
		binary.LittleEndian.PutUint32(buf[0x04:], core.FavHeaderUnk)
		buf[core.FavOffBodyFlag] = 1
		if r.BodyType == 1 {
			buf[core.FavOffBodyType] = 0 // male in Favorites
		} else {
			buf[core.FavOffBodyType] = 1 // female in Favorites
		}

		// FACE block header
		copy(buf[core.FavOffMagic:], []byte("FACE"))
		binary.LittleEndian.PutUint32(buf[core.FavOffAlignment:], 4)
		binary.LittleEndian.PutUint32(buf[core.FavOffInnerSize:], 0x120)

		// Eight resolved model IDs — shared with direct Apply so Add and Apply
		// can never diverge. Type A = UI-1 + hair lookup, Type B = verified
		// female mapping (incl. DecalModel).
		for m, id := range r.Models {
			binary.LittleEndian.PutUint32(buf[core.FavOffModelIDs+m*4:], uint32(id))
		}

		copy(buf[core.FavOffFaceShape:], r.FaceShape[:])
		copy(buf[core.FavOffUnkBlock:], unkBlock[:])
		copy(buf[core.FavOffBody:], r.Body[:])
		copy(buf[core.FavOffSkin:], r.Skin[:])

		copy(ud[slotOff:], buf)
		a.favSlotNames[safeIdx] = name
		written++
	}

	if written > 0 {
		a.pushFavUndoLocked(undoSnap)
	}

	return written, nil
}

// buildFavSnapshotLocked deep-copies the current Mirror Favorites state.
// Caller must hold favMu (and saveMu.RLock so a.save is stable).
func (a *App) buildFavSnapshotLocked() favSnapshot {
	dataCopy := make([]byte, len(a.save.UserData10.Data))
	copy(dataCopy, a.save.UserData10.Data)
	names := make(map[int]string, len(a.favSlotNames))
	for k, v := range a.favSlotNames {
		names[k] = v
	}
	return favSnapshot{Data: dataCopy, SlotNames: names}
}

// pushFavUndoLocked appends a snapshot, enforcing maxUndoDepth. Holds favMu.
func (a *App) pushFavUndoLocked(snap favSnapshot) {
	if len(a.favUndoStack) >= maxUndoDepth {
		a.favUndoStack = a.favUndoStack[1:] // drop oldest
	}
	a.favUndoStack = append(a.favUndoStack, snap)
}

// RevertFavorites pops the last favorites snapshot and restores both the
// UserData10 bytes and favSlotNames — undoing one WriteSelectedToFavorites Add.
func (a *App) RevertFavorites() error {
	a.saveMu.RLock()
	defer a.saveMu.RUnlock()
	if a.save == nil {
		return fmt.Errorf("no save loaded")
	}
	a.favMu.Lock()
	defer a.favMu.Unlock()
	if len(a.favUndoStack) == 0 {
		return fmt.Errorf("nothing to undo for favorites")
	}
	snap := a.favUndoStack[len(a.favUndoStack)-1]
	a.favUndoStack = a.favUndoStack[:len(a.favUndoStack)-1]
	a.save.UserData10.Data = snap.Data
	a.favSlotNames = snap.SlotNames
	return nil
}

// GetFavoritesUndoDepth returns the number of favorites undo snapshots
// available. Takes favMu.RLock only — safe within the saveMu → favMu → slotMu
// order since it acquires no lower lock.
func (a *App) GetFavoritesUndoDepth() int {
	a.saveMu.RLock()
	defer a.saveMu.RUnlock()
	if a.save == nil {
		return 0
	}
	a.favMu.RLock()
	defer a.favMu.RUnlock()
	return len(a.favUndoStack)
}
