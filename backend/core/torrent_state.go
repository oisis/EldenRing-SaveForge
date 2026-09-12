package core

import (
	"encoding/binary"
	"fmt"
)

// HorseState values observed in native saves (RideGameData.State).
const (
	HorseStateDead   uint32 = 3
	HorseStateActive uint32 = 13
)

// torrentStateFieldOffset is RideGameData.State's position inside the struct:
// coordinates(12) + map_id(4) + angle(16) + hp(4). Only those four bytes are
// ever written by RepairTorrentState.
const torrentStateFieldOffset = 12 + 4 + 16 + 4

// regionsEndOffset returns the byte offset directly after `unlocked_regions`,
// which is where WorldHead — and therefore RideGameData — begins. The
// SectionMap boundary recorded at read time is authoritative, because
// slot.UnlockedRegions may have been mutated in memory since; the documented
// formula is the fallback for slots whose map does not carry the section.
func regionsEndOffset(slot *SaveSlot) (int, bool) {
	for _, sec := range slot.SectionMap {
		if sec.Name == SectionUnlockedRegs {
			return sec.End, true
		}
	}
	if slot.UnlockedRegionsOffset <= 0 {
		return 0, false
	}
	return slot.UnlockedRegionsOffset + 4 + 4*len(slot.UnlockedRegions), true
}

// ReadTorrentGameData parses the slot's RideGameData block and returns it
// together with its absolute offset in slot.Data. ok is false whenever the
// block cannot be safely located or parsed (empty slot, unknown regions
// boundary, out-of-bounds range) — callers must then report nothing and mutate
// nothing rather than guess an offset.
func ReadTorrentGameData(slot *SaveSlot) (horse RideGameData, offset int, ok bool) {
	if slot == nil || slot.Version == 0 || len(slot.Data) != SlotSize {
		return horse, 0, false
	}
	start, located := regionsEndOffset(slot)
	if !located || start <= 0 || start+RideGameDataSize > len(slot.Data) {
		return horse, 0, false
	}
	r := NewReader(slot.Data)
	if _, err := r.Seek(int64(start), 0); err != nil {
		return horse, 0, false
	}
	if err := horse.Read(r); err != nil {
		return horse, 0, false
	}
	return horse, start, true
}

// TorrentDeadButActive is the single source of truth for the confirmed freeze
// condition: Torrent has no HP but the game still holds it in the ACTIVE ride
// state, which makes the character hang forever on load. Both the diagnostics
// scan and the repair scanner ask this function — neither re-derives the rule.
//
// Every other HP/state combination (including HP == 0 with State == DEAD) is
// legal native data and is deliberately not reported.
func TorrentDeadButActive(slot *SaveSlot) bool {
	horse, _, ok := ReadTorrentGameData(slot)
	return ok && horse.HP == 0 && horse.State == HorseStateActive
}

// RepairTorrentState moves Torrent from ACTIVE to DEAD for a slot that is in
// the confirmed freeze condition. It re-verifies bounds and the condition and
// fails without touching a single byte when either no longer holds. Only
// RideGameData.State is written: HP, coordinates, map ID and angle — and every
// other byte of the slot — are preserved.
func RepairTorrentState(slot *SaveSlot) error {
	horse, offset, ok := ReadTorrentGameData(slot)
	if !ok {
		return fmt.Errorf("RepairTorrentState: RideGameData cannot be located or parsed")
	}
	if horse.HP != 0 || horse.State != HorseStateActive {
		return fmt.Errorf("RepairTorrentState: slot is not in the HP=0 + ACTIVE state (hp=%d state=%d)",
			horse.HP, horse.State)
	}
	binary.LittleEndian.PutUint32(slot.Data[offset+torrentStateFieldOffset:], HorseStateDead)
	return nil
}
