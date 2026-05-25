package core

import "testing"

// nameU16 packs an ASCII string into a [16]uint16 CharacterName field.
func nameU16(s string) [16]uint16 {
	var out [16]uint16
	for i, r := range s {
		if i >= 16 {
			break
		}
		out[i] = uint16(r)
	}
	return out
}

// buildSlotsFixture returns a SaveFile with a UserData10 buffer large enough for
// all 10 ProfileSummary regions, and every slot given a real-sized data block.
func buildSlotsFixture(t *testing.T) *SaveFile {
	t.Helper()
	s := &SaveFile{}
	s.UserData10.Data = make([]byte, ProfileSummaryOffset+10*ProfileSummaryStride+0x100)
	for i := range s.Slots {
		s.Slots[i].Data = make([]byte, SlotSize)
		s.Slots[i].GaMap = make(map[uint32]uint32)
	}
	return s
}

func TestClearSlot_ZeroesEverythingInPlace(t *testing.T) {
	s := buildSlotsFixture(t)

	// Slot 1: the target. Slot 2: an active neighbour that must stay untouched.
	s.ActiveSlots[1] = true
	s.Slots[1].Player.CharacterName = nameU16("Bydlaczka")
	s.ProfileSummaries[1] = ProfileSummary{CharacterName: nameU16("Bydlaczka"), Level: 9}
	region1 := ProfileSummaryOffset + 1*ProfileSummaryStride
	s.UserData10.Data[region1] = 0xAB // opaque byte that Serialize never rewrites
	s.UserData10.Data[region1+0x200] = 0xCD

	s.ActiveSlots[2] = true
	s.Slots[2].Player.CharacterName = nameU16("Niziol")
	s.ProfileSummaries[2] = ProfileSummary{CharacterName: nameU16("Niziol"), Level: 150}
	region2 := ProfileSummaryOffset + 2*ProfileSummaryStride
	s.UserData10.Data[region2+0x200] = 0xEE

	s.ClearSlot(1)

	// Target fully cleared.
	if s.ActiveSlots[1] {
		t.Error("slot 1 active flag not cleared")
	}
	if UTF16ToString(s.Slots[1].Player.CharacterName[:]) != "" {
		t.Error("slot 1 data name not cleared")
	}
	if UTF16ToString(s.ProfileSummaries[1].CharacterName[:]) != "" || s.ProfileSummaries[1].Level != 0 {
		t.Error("slot 1 ProfileSummary struct not cleared")
	}
	if len(s.Slots[1].Data) != SlotSize {
		t.Errorf("slot 1 data size: want %d, got %d", SlotSize, len(s.Slots[1].Data))
	}
	for off := region1; off < region1+ProfileSummaryStride; off++ {
		if s.UserData10.Data[off] != 0 {
			t.Errorf("slot 1 summary region byte at +%d not zeroed: 0x%02X",
				off-region1, s.UserData10.Data[off])
			break
		}
	}

	// Neighbour untouched (no shift, no spillover into adjacent region).
	if !s.ActiveSlots[2] || UTF16ToString(s.Slots[2].Player.CharacterName[:]) != "Niziol" {
		t.Error("active neighbour slot 2 was disturbed by clearing slot 1")
	}
	if s.UserData10.Data[region2+0x200] != 0xEE {
		t.Error("neighbour slot 2 summary region was corrupted")
	}
}

func TestSlotHasResidualData(t *testing.T) {
	s := buildSlotsFixture(t)

	// Phantom: inactive flag, but data + summary name present.
	s.ActiveSlots[1] = false
	s.Slots[1].Player.CharacterName = nameU16("Bydlaczka")
	s.ProfileSummaries[1] = ProfileSummary{CharacterName: nameU16("Bydlaczka")}
	if !s.SlotHasResidualData(1) {
		t.Error("expected slot 1 to be flagged as residual")
	}

	// Active slot is never residual.
	s.ActiveSlots[2] = true
	s.Slots[2].Player.CharacterName = nameU16("Niziol")
	if s.SlotHasResidualData(2) {
		t.Error("active slot 2 must not be residual")
	}

	// Truly empty inactive slot.
	if s.SlotHasResidualData(3) {
		t.Error("empty slot 3 must not be residual")
	}
}

func TestCleanResidualSlots(t *testing.T) {
	s := buildSlotsFixture(t)

	s.ActiveSlots[0] = true
	s.Slots[0].Player.CharacterName = nameU16("Niziol")

	// Slot 1 phantom (the OiSiSiSBack-vanilla.dat shape).
	s.ActiveSlots[1] = false
	s.Slots[1].Player.CharacterName = nameU16("Bydlaczka")
	s.ProfileSummaries[1] = ProfileSummary{CharacterName: nameU16("Bydlaczka"), Level: 9}

	s.ActiveSlots[2] = true
	s.Slots[2].Player.CharacterName = nameU16("Bydlaczka")
	s.ProfileSummaries[2] = ProfileSummary{CharacterName: nameU16("Bydlaczka"), Level: 150}

	cleaned := s.CleanResidualSlots()
	if cleaned != 1 {
		t.Fatalf("expected 1 residual slot cleaned, got %d", cleaned)
	}
	if s.SlotHasResidualData(1) || UTF16ToString(s.Slots[1].Player.CharacterName[:]) != "" {
		t.Error("slot 1 phantom not cleaned")
	}
	// Active slots preserved.
	if UTF16ToString(s.Slots[0].Player.CharacterName[:]) != "Niziol" {
		t.Error("active slot 0 disturbed")
	}
	if UTF16ToString(s.Slots[2].Player.CharacterName[:]) != "Bydlaczka" || s.ProfileSummaries[2].Level != 150 {
		t.Error("active slot 2 (real Bydlaczka) disturbed")
	}
	// Idempotent.
	if again := s.CleanResidualSlots(); again != 0 {
		t.Errorf("second clean should be a no-op, cleaned %d", again)
	}
}
