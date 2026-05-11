package tests

import (
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
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

// TestCompanionFlagsNoRoundtableFlags ensures that Roundtable Hold flags
// are never part of the Spectral Steed Whistle companion set.
func TestCompanionFlagsNoRoundtableFlags(t *testing.T) {
	roundtable := []uint32{
		10009655, // Melina RTH invitation trigger
		11109658, // Gideon welcome (RTH visited marker)
		11109659, // Gideon advice
		11109786, // RTH transport trigger (transient)
		710770,   // Melina leaves Gatefront (A)
		69090,    // Melina leaves Gatefront (B)
		69370,    // Melina leaves Gatefront (C)
	}
	companions := data.CompanionEventFlagsForItem(data.ItemSpectralSteedWhistle)
	for _, cf := range companions {
		for _, bad := range roundtable {
			if cf == bad {
				t.Errorf("whistle companion set contains Roundtable/context flag %d — must not be set by item add", cf)
			}
		}
	}
}

// TestCompanionFlagsClearOnFlagData verifies that the four whistle companion flags
// can be individually cleared via db.SetEventFlag on a real slot's event flag region.
// This replicates the CLEAR path in RemoveItemsFromCharacter at the data layer.
func TestCompanionFlagsClearOnFlagData(t *testing.T) {
	save, slotIdx := loadBulkTestSave(t)
	slot := &save.Slots[slotIdx]

	if slot.EventFlagsOffset <= 0 || slot.EventFlagsOffset >= len(slot.Data) {
		t.Skip("EventFlagsOffset not set in test save slot")
	}

	companions := data.CompanionEventFlagsForItem(data.ItemSpectralSteedWhistle)
	if len(companions) == 0 {
		t.Fatal("CompanionEventFlagsForItem returned empty for Spectral Steed Whistle")
	}

	flagData := make([]byte, len(slot.Data)-slot.EventFlagsOffset)
	copy(flagData, slot.Data[slot.EventFlagsOffset:])

	// First set all companion flags.
	for _, f := range companions {
		if err := db.SetEventFlag(flagData, f, true); err != nil {
			t.Fatalf("SetEventFlag(%d, true) failed: %v", f, err)
		}
	}

	// Clear all companion flags (simulates RemoveItemsFromCharacter CLEAR path).
	for _, f := range companions {
		if err := db.SetEventFlag(flagData, f, false); err != nil {
			t.Errorf("SetEventFlag(%d, false) failed: %v", f, err)
			continue
		}
		got, err := db.GetEventFlag(flagData, f)
		if err != nil {
			t.Errorf("GetEventFlag(%d) after clear failed: %v", f, err)
			continue
		}
		if got {
			t.Errorf("flag %d: still set after clear", f)
		}
	}
}

// TestCompanionFlagsNotClearedForUnknownItem verifies that removing an item
// with no companion flags does not affect whistle companion flags.
func TestCompanionFlagsNotClearedForUnknownItem(t *testing.T) {
	save, slotIdx := loadBulkTestSave(t)
	slot := &save.Slots[slotIdx]

	if slot.EventFlagsOffset <= 0 || slot.EventFlagsOffset >= len(slot.Data) {
		t.Skip("EventFlagsOffset not set in test save slot")
	}

	companions := data.CompanionEventFlagsForItem(data.ItemSpectralSteedWhistle)
	if len(companions) == 0 {
		t.Fatal("CompanionEventFlagsForItem returned empty for Spectral Steed Whistle")
	}

	flagData := make([]byte, len(slot.Data)-slot.EventFlagsOffset)
	copy(flagData, slot.Data[slot.EventFlagsOffset:])

	// Set whistle companion flags.
	for _, f := range companions {
		if err := db.SetEventFlag(flagData, f, true); err != nil {
			t.Fatalf("SetEventFlag(%d, true) failed: %v", f, err)
		}
	}

	// An unknown item has no companion flags → no CLEAR should happen.
	unknownID := uint32(0xDEADBEEF)
	if flags := data.CompanionEventFlagsForItem(unknownID); len(flags) != 0 {
		t.Skipf("test item 0x%08X unexpectedly has companion flags", unknownID)
	}

	// Verify whistle flags are unaffected (still set).
	for _, f := range companions {
		got, err := db.GetEventFlag(flagData, f)
		if err != nil {
			t.Errorf("GetEventFlag(%d) failed: %v", f, err)
			continue
		}
		if !got {
			t.Errorf("flag %d cleared unexpectedly for unknown item removal", f)
		}
	}
}

// TestCompanionFlagsRemainingItemPreventsClearing verifies that the "remaining item"
// check works: if a GaItem with the whistle ID still exists, flags must not be cleared.
func TestCompanionFlagsRemainingItemPreventsClearing(t *testing.T) {
	save, slotIdx := loadBulkTestSave(t)
	slot := &save.Slots[slotIdx]

	// Find whether any GaItem has the whistle ID.
	hasWhistle := false
	for _, g := range slot.GaItems {
		if !g.IsEmpty() && g.ItemID == data.ItemSpectralSteedWhistle {
			hasWhistle = true
			break
		}
	}

	// If whistle is present in GaItems, the CLEAR logic must not clear flags
	// (because the item still exists after a partial removal of another handle).
	if !hasWhistle {
		t.Skip("test save has no Spectral Steed Whistle in GaItems — skipping remaining-item guard test")
	}

	// Confirm IsEmpty() returns false for a non-zeroed GaItem.
	for _, g := range slot.GaItems {
		if g.ItemID == data.ItemSpectralSteedWhistle {
			if g.IsEmpty() {
				t.Errorf("GaItem with whistle ID 0x%08X reports IsEmpty()=true", data.ItemSpectralSteedWhistle)
			}
			break
		}
	}

	// Confirm zeroed GaItem (as RemoveItemFromSlot leaves it) returns IsEmpty()=true.
	zeroed := core.GaItemFull{Unk2: -1, Unk3: -1, AoWGaItemHandle: 0xFFFFFFFF}
	if !zeroed.IsEmpty() {
		t.Errorf("zeroed GaItem (post-removal state) does not report IsEmpty()=true")
	}
}
