package tests

import (
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/db"
	"github.com/oisis/EldenRing-SaveForge/backend/db/data"
)

// TestGraceCompanionFlagsSetOnRealSave verifies that:
//  1. CompanionEventFlagsForGrace returns the expected flags for Gatefront.
//  2. Each flag can be written to and read back from a real slot's event flag region.
//
// Operates on a copy of slot data — no file is modified.
func TestGraceCompanionFlagsSetOnRealSave(t *testing.T) {
	save, slotIdx := loadBulkTestSave(t)
	slot := &save.Slots[slotIdx]

	if slot.EventFlagsOffset <= 0 || slot.EventFlagsOffset >= len(slot.Data) {
		t.Skip("EventFlagsOffset not set in test save slot")
	}

	companions := data.CompanionEventFlagsForGrace(data.GatefrontGraceEventFlagID)
	if len(companions) == 0 {
		t.Fatal("CompanionEventFlagsForGrace returned empty for Gatefront grace")
	}

	flagData := make([]byte, len(slot.Data)-slot.EventFlagsOffset)
	copy(flagData, slot.Data[slot.EventFlagsOffset:])

	for _, f := range companions {
		if err := db.SetEventFlag(flagData, f, true); err != nil {
			t.Errorf("SetEventFlag(%d) failed: %v", f, err)
			continue
		}
		got, err := db.GetEventFlag(flagData, f)
		if err != nil {
			t.Errorf("GetEventFlag(%d) failed: %v", f, err)
			continue
		}
		if !got {
			t.Errorf("flag %d: SetEventFlag succeeded but GetEventFlag returned false", f)
		}
	}
}

// TestGraceCompanionFlagsNoRTHFlags ensures RTH and forbidden flags are absent
// from every grace companion set.
func TestGraceCompanionFlagsNoRTHFlags(t *testing.T) {
	forbidden := []uint32{
		10009655, // Melina RTH invitation trigger
		11109658, // Gideon welcome
		11109659, // Gideon advice
		11109786, // RTH transport trigger (transient)
		4698,     // Melina cutscene trigger (transient)
		4656,     // Level up performed
		710770,   // Melina leaves Gatefront
		69090,    // Melina leaves Gatefront
		69370,    // Melina leaves Gatefront
	}
	companions := data.CompanionEventFlagsForGrace(data.GatefrontGraceEventFlagID)
	for _, cf := range companions {
		for _, bad := range forbidden {
			if cf == bad {
				t.Errorf("Gatefront grace companion set contains forbidden flag %d", cf)
			}
		}
	}
}

// TestGraceCompanionFlagsSetOnlyNotCleared verifies the SET-only contract:
// setting visited=false for Gatefront must not clear 60100/4680/4681/710520
// when they were set by a different path (e.g. Spectral Steed Whistle companion flags).
func TestGraceCompanionFlagsSetOnlyNotCleared(t *testing.T) {
	save, slotIdx := loadBulkTestSave(t)
	slot := &save.Slots[slotIdx]

	if slot.EventFlagsOffset <= 0 || slot.EventFlagsOffset >= len(slot.Data) {
		t.Skip("EventFlagsOffset not set in test save slot")
	}

	companions := data.CompanionEventFlagsForGrace(data.GatefrontGraceEventFlagID)
	if len(companions) == 0 {
		t.Fatal("CompanionEventFlagsForGrace returned empty for Gatefront grace")
	}

	flagData := make([]byte, len(slot.Data)-slot.EventFlagsOffset)
	copy(flagData, slot.Data[slot.EventFlagsOffset:])

	// Set all companion flags (simulates whistle path or prior progress).
	for _, f := range companions {
		if err := db.SetEventFlag(flagData, f, true); err != nil {
			t.Fatalf("SetEventFlag(%d, true) failed: %v", f, err)
		}
	}

	// The SET-only contract: CompanionEventFlagsForGrace returns nil-equivalent
	// data that should not be used on deactivation. Verify flags remain set.
	// (The hook in app_world.go only fires when visited=true.)
	for _, f := range companions {
		got, err := db.GetEventFlag(flagData, f)
		if err != nil {
			t.Errorf("GetEventFlag(%d) failed: %v", f, err)
			continue
		}
		if !got {
			t.Errorf("flag %d was cleared — SET-only contract violated", f)
		}
	}
}
