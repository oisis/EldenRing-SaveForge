package tests

import (
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/db"
	"github.com/oisis/EldenRing-SaveForge/backend/db/data"
)

// TestCompanionFlagsSetOnRealSave verifies that:
//  1. CompanionEventFlagsForItem returns the expected flags for the Spectral Steed Whistle.
//  2. Each flag can be written to and read back from a real save slot's event flag region.
//  3. Flags known to be transient/forbidden are absent from the companion set.
//
// This test operates on a copy of slot data — no file is modified.
func TestCompanionFlagsSetOnRealSave(t *testing.T) {
	save, slotIdx := loadBulkTestSave(t)
	slot := &save.Slots[slotIdx]

	if slot.EventFlagsOffset <= 0 || slot.EventFlagsOffset >= len(slot.Data) {
		t.Skip("EventFlagsOffset not set in test save slot")
	}

	companions := data.CompanionEventFlagsForItem(data.ItemSpectralSteedWhistle)
	if len(companions) == 0 {
		t.Fatal("CompanionEventFlagsForItem returned empty for Spectral Steed Whistle")
	}

	// Work on a copy so the test is non-destructive.
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

// TestCompanionFlagsForbiddenAbsent ensures none of the transient flags
// appear in any companion set.
func TestCompanionFlagsForbiddenAbsent(t *testing.T) {
	forbidden := []uint32{
		4698, // Melina cutscene trigger
		4651, 4652, 4653, // Melina dialogue states
		4656, // Level up
	}
	for _, itemID := range []uint32{data.ItemSpectralSteedWhistle} {
		for _, cf := range data.CompanionEventFlagsForItem(itemID) {
			for _, bad := range forbidden {
				if cf == bad {
					t.Errorf("item 0x%08X companion set contains forbidden flag %d", itemID, bad)
				}
			}
		}
	}
}

// TestCompanionFlagsMechanicFlagPresent verifies that the Torrent mechanic
// unlock flag (60100) is always included for the Spectral Steed Whistle.
func TestCompanionFlagsMechanicFlagPresent(t *testing.T) {
	companions := data.CompanionEventFlagsForItem(data.ItemSpectralSteedWhistle)
	for _, f := range companions {
		if f == data.EventFlagObtainedSpectralSteedWhistle {
			return
		}
	}
	t.Errorf("mechanic unlock flag %d (EventFlagObtainedSpectralSteedWhistle) missing from whistle companion set",
		data.EventFlagObtainedSpectralSteedWhistle)
}
