package core

import (
	"bytes"
	"encoding/binary"
	"math"
	"testing"
)

// torrentFixtureRegionsOffset is an arbitrary in-slot offset for the synthetic
// unlocked_regions block; the RideGameData block follows it.
const torrentFixtureRegionsOffset = 0x1000

// torrentFixture builds a synthetic SlotSize slot whose RideGameData block sits
// right after a three-entry unlocked_regions block, carrying the given HP and
// ride state plus recognisable coordinates / map ID / angle so a repair that
// touches more than State is detectable. No real save is read.
func torrentFixture(hp int32, state uint32) *SaveSlot {
	slot := &SaveSlot{
		Version:               1,
		Data:                  make([]byte, SlotSize),
		GaMap:                 make(map[uint32]uint32),
		MagicOffset:           1000,
		UnlockedRegionsOffset: torrentFixtureRegionsOffset,
		UnlockedRegions:       []uint32{10, 20, 30},
	}
	start := torrentRideOffset(slot)

	// coordinates(12): three distinct finite floats.
	binary.LittleEndian.PutUint32(slot.Data[start+0:], math.Float32bits(101.5))
	binary.LittleEndian.PutUint32(slot.Data[start+4:], math.Float32bits(-202.25))
	binary.LittleEndian.PutUint32(slot.Data[start+8:], math.Float32bits(303.75))
	// map_id(4)
	copy(slot.Data[start+12:], []byte{0x3C, 0x00, 0x00, 0x3C})
	// angle(16): four distinct finite floats.
	for i := 0; i < 4; i++ {
		binary.LittleEndian.PutUint32(slot.Data[start+16+4*i:], math.Float32bits(float32(i)+0.5))
	}
	binary.LittleEndian.PutUint32(slot.Data[start+32:], uint32(hp))
	binary.LittleEndian.PutUint32(slot.Data[start+36:], state)
	return slot
}

// torrentRideOffset mirrors the documented WorldHead start contract for tests.
func torrentRideOffset(slot *SaveSlot) int {
	return slot.UnlockedRegionsOffset + 4 + 4*len(slot.UnlockedRegions)
}

func torrentDiagnosticCount(diag SlotDiagnostics) int {
	n := 0
	for _, iss := range diag.Issues {
		if iss.Category == "torrent" {
			n++
			if iss.Severity != SeverityCritical {
				return -1
			}
		}
	}
	return n
}

func torrentIssues(issues []RepairIssue) []RepairIssue {
	var out []RepairIssue
	for _, iss := range issues {
		if iss.Key.Code == RepairCodeTorrentDeadActive {
			out = append(out, iss)
		}
	}
	return out
}

// The confirmed freeze condition: no HP, ride state still ACTIVE.
func TestDiagnoseSaveCorruption_TorrentDeadActiveIsCritical(t *testing.T) {
	slot := torrentFixture(0, HorseStateActive)

	diag := DiagnoseSaveCorruption(slot, 2)

	if got := torrentDiagnosticCount(diag); got != 1 {
		t.Fatalf("torrent critical issues = %d, want 1: %+v", got, diag.Issues)
	}
}

// Every other combination is legal native data, and an unlocatable or
// unparseable RideGameData must stay silent rather than guess.
func TestDiagnoseSaveCorruption_TorrentNegativeCases(t *testing.T) {
	unlocatable := torrentFixture(0, HorseStateActive)
	unlocatable.UnlockedRegionsOffset = 0
	unlocatable.UnlockedRegions = nil

	outOfBounds := torrentFixture(0, HorseStateActive)
	outOfBounds.UnlockedRegionsOffset = SlotSize - 8

	emptySlot := torrentFixture(0, HorseStateActive)
	emptySlot.Version = 0

	shortData := torrentFixture(0, HorseStateActive)
	shortData.Data = shortData.Data[:SlotSize-1]

	cases := []struct {
		name string
		slot *SaveSlot
	}{
		{"hp0_dead", torrentFixture(0, HorseStateDead)},
		{"hp_positive_active", torrentFixture(1000, HorseStateActive)},
		{"hp_positive_inactive", torrentFixture(1000, 0)},
		{"empty_slot", emptySlot},
		{"unlocatable_ridegamedata", unlocatable},
		{"ridegamedata_out_of_bounds", outOfBounds},
		{"truncated_slot_data", shortData},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if TorrentDeadButActive(tc.slot) {
				t.Fatal("TorrentDeadButActive reported the freeze condition")
			}
			if got := torrentDiagnosticCount(DiagnoseSaveCorruption(tc.slot, 2)); got != 0 {
				t.Fatalf("torrent diagnostics = %d, want 0", got)
			}
			if got := torrentIssues(ScanRepairIssues(2, tc.slot)); len(got) != 0 {
				t.Fatalf("repair issues = %d, want 0: %+v", len(got), got)
			}
		})
	}
}

// The regions boundary recorded in the SectionMap is authoritative — a slot
// whose UnlockedRegions slice was mutated in memory must still resolve the
// on-disk RideGameData block.
func TestTorrentDeadButActive_UsesSectionMapBoundary(t *testing.T) {
	slot := torrentFixture(0, HorseStateActive)
	regionsEnd := torrentRideOffset(slot)
	slot.SectionMap = []SectionRange{
		{Name: SectionPreUnlockedRegs, Start: 0, End: slot.UnlockedRegionsOffset},
		{Name: SectionUnlockedRegs, Start: slot.UnlockedRegionsOffset, End: regionsEnd},
	}
	slot.UnlockedRegions = append(slot.UnlockedRegions, 40, 50) // in-memory mutation

	if !TorrentDeadButActive(slot) {
		t.Fatal("freeze condition not detected via the SectionMap boundary")
	}
}

func TestScanRepairIssues_TorrentDeadActive(t *testing.T) {
	slot := torrentFixture(0, HorseStateActive)

	found := torrentIssues(ScanRepairIssues(4, slot))
	if len(found) != 1 {
		t.Fatalf("torrent issues = %d, want 1", len(found))
	}
	iss := found[0]
	if iss.Severity != repairSeverityError {
		t.Errorf("severity = %q, want %q", iss.Severity, repairSeverityError)
	}
	if iss.Key.Slot != 4 || iss.Key.Domain != repairDomainWorld || iss.Key.Scope != repairScopeWorld {
		t.Errorf("key = %+v, want slot 4 / domain %q / scope %q", iss.Key, repairDomainWorld, repairScopeWorld)
	}
	if iss.DefaultAction != RepairActionFixTorrentState {
		t.Errorf("default action = %q, want %q", iss.DefaultAction, RepairActionFixTorrentState)
	}
	if len(iss.Actions) != 1 || iss.Actions[0] != RepairActionFixTorrentState {
		t.Errorf("actions = %v, want [%s]", iss.Actions, RepairActionFixTorrentState)
	}
}

// The repair moves State to DEAD and leaves every other byte of the slot —
// HP, coordinates, map ID, angle included — untouched.
func TestRepairTorrentState_ChangesOnlyState(t *testing.T) {
	slot := torrentFixture(0, HorseStateActive)
	before := append([]byte(nil), slot.Data...)
	start := torrentRideOffset(slot)
	stateOff := start + torrentStateFieldOffset

	if err := RepairTorrentState(slot); err != nil {
		t.Fatalf("RepairTorrentState: %v", err)
	}

	horse, _, ok := ReadTorrentGameData(slot)
	if !ok {
		t.Fatal("RideGameData unreadable after repair")
	}
	if horse.State != HorseStateDead {
		t.Fatalf("state = %d, want %d", horse.State, HorseStateDead)
	}
	if horse.HP != 0 {
		t.Fatalf("HP = %d, want 0 (repair must not touch HP)", horse.HP)
	}
	if !bytes.Equal(before[:stateOff], slot.Data[:stateOff]) {
		t.Error("bytes before RideGameData.State changed")
	}
	if !bytes.Equal(before[stateOff+4:], slot.Data[stateOff+4:]) {
		t.Error("bytes after RideGameData.State changed")
	}
	if TorrentDeadButActive(slot) {
		t.Error("freeze condition still reported after repair")
	}
	if got := torrentIssues(ScanRepairIssues(0, slot)); len(got) != 0 {
		t.Errorf("re-scan still reports %d torrent issue(s)", len(got))
	}
	if got := torrentDiagnosticCount(DiagnoseSaveCorruption(slot, 0)); got != 0 {
		t.Errorf("re-diagnose still reports %d torrent issue(s)", got)
	}
}

// A repair asked for on a slot that is not in the freeze condition — or whose
// RideGameData cannot be located — fails without writing a single byte.
func TestRepairTorrentState_RefusesWithoutMutating(t *testing.T) {
	unlocatable := torrentFixture(0, HorseStateActive)
	unlocatable.UnlockedRegionsOffset = 0
	unlocatable.UnlockedRegions = nil

	cases := []struct {
		name string
		slot *SaveSlot
	}{
		{"hp0_dead", torrentFixture(0, HorseStateDead)},
		{"hp_positive_active", torrentFixture(1000, HorseStateActive)},
		{"unlocatable_ridegamedata", unlocatable},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			before := append([]byte(nil), tc.slot.Data...)
			if err := RepairTorrentState(tc.slot); err == nil {
				t.Fatal("RepairTorrentState succeeded, want error")
			}
			if !bytes.Equal(before, tc.slot.Data) {
				t.Fatal("slot bytes changed despite the refused repair")
			}
		})
	}
}
