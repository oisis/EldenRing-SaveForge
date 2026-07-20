package core

// UserData10 per-slot metadata offsets (see spec/23-user-data-10.md).
const (
	// SteamIDOffset is the offset of the PC global Steam ID within UserData10.Data.
	// Bytes [0x00:0x04) are metadata/version, not part of the ID.
	SteamIDOffset        = 0x04
	ActiveSlotsOffset    = 0x1954 // 10 × u8 active-slot flags
	ProfileSummaryOffset = 0x195E // ProfileSummary[i] at base + i*ProfileSummaryStride
	ProfileSummaryStride = 0x24C  // 588 bytes per summary
)

// ClearProfileSummaryRegion zeroes the FULL ProfileSummary region for slot idx in
// UserData10.Data — name, level, AND the opaque face/equipment snapshot
// (ProfileSummaryStride bytes). ProfileSummary.Serialize only rewrites name+level,
// so without this a deleted slot would keep the previous occupant's face/equipment
// snapshot, which the character-select menu could still render as a phantom.
func ClearProfileSummaryRegion(data []byte, idx int) {
	if idx < 0 || idx >= 10 {
		return
	}
	off := ProfileSummaryOffset + idx*ProfileSummaryStride
	if off < 0 || off+ProfileSummaryStride > len(data) {
		return
	}
	for i := 0; i < ProfileSummaryStride; i++ {
		data[off+i] = 0
	}
}

// SlotHasResidualData reports whether slot idx carries leftover character data
// (a slot-data name or a ProfileSummary name) while its active flag is cleared.
// This is the "phantom" state produced when a character is deleted in-game (the
// game clears the active flag but does not zero the data block / summary): the
// game ignores the slot (flag=0), but a name-based UI would show it as a
// duplicate. Active slots are never residual.
func (s *SaveFile) SlotHasResidualData(idx int) bool {
	if idx < 0 || idx >= 10 || s.ActiveSlots[idx] {
		return false
	}
	if UTF16ToString(s.Slots[idx].Player.CharacterName[:]) != "" {
		return true
	}
	return UTF16ToString(s.ProfileSummaries[idx].CharacterName[:]) != ""
}

// ClearSlot zeroes slot idx entirely IN PLACE — data block, active flag, and the
// full ProfileSummary region. This mirrors the game's positional deletion model:
// slots 0-9 keep their positions and only the target slot is cleared; subsequent
// slots are NOT shifted down. (Confirmed by the independent per-slot active-flag
// array at 0x1954 — gaps between active slots are valid — and by the reference
// ER-Save-Editor, which is purely positional.)
func (s *SaveFile) ClearSlot(idx int) {
	if idx < 0 || idx >= 10 {
		return
	}
	s.Slots[idx] = SaveSlot{
		Data:        make([]byte, SlotSize),
		GaMap:       make(map[uint32]uint32),
		MagicOffset: FallbackMagicBase, // valid offset so Write() does not panic
	}
	s.ActiveSlots[idx] = false
	s.ProfileSummaries[idx] = ProfileSummary{}
	if s.UserData10.Data != nil {
		ClearProfileSummaryRegion(s.UserData10.Data, idx)
	}
}

// CleanResidualSlots zeroes every slot flagged inactive that still carries
// residual character data (see SlotHasResidualData). Returns the number of slots
// cleaned. Active slots are never touched. Idempotent: a second call returns 0.
func (s *SaveFile) CleanResidualSlots() int {
	cleaned := 0
	for i := 0; i < 10; i++ {
		if s.SlotHasResidualData(i) {
			s.ClearSlot(i)
			cleaned++
		}
	}
	return cleaned
}
