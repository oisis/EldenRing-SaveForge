package core

import (
	"strings"
	"testing"
)

// TestCheckGaItems_NoFalseAlarmOnValidItemID verifies that a GaItem with a
// valid Handle prefix (0x8...) but an ItemID with prefix 0x0 or 0x1 does NOT
// trigger the "unknown type prefix" warning.
func TestCheckGaItems_NoFalseAlarmOnValidItemID(t *testing.T) {
	slot := &SaveSlot{
		MagicOffset: DynPlayerData + 1000,
		GaItems: []GaItemFull{
			// weapon GaItem: Handle has valid 0x8 prefix; ItemID has 0x0 prefix (weapon record size)
			{Handle: ItemTypeWeapon | 0x00800001, ItemID: 0x00400110, Unk2: -1, Unk3: -1, AoWGaItemHandle: NoCustomAoWHandle},
			// armor GaItem: Handle has valid 0x9 prefix; ItemID has 0x1 prefix (armor record size)
			{Handle: ItemTypeArmor | 0x00800002, ItemID: 0x10000100, Unk2: -1, Unk3: -1, AoWGaItemHandle: NoCustomAoWHandle},
		},
	}

	diag := SlotDiagnostics{}
	diag.checkGaItems(slot)

	for _, e := range diag.Issues {
		if strings.Contains(e.Description, "unknown type prefix") {
			t.Errorf("false alarm: got unexpected warning %q", e.Description)
		}
	}
}

// TestCheckGaItems_ReportsInvalidHandle verifies that a GaItem whose Handle
// has a genuinely unknown prefix is still reported.
func TestCheckGaItems_ReportsInvalidHandle(t *testing.T) {
	slot := &SaveSlot{
		MagicOffset: DynPlayerData + 1000,
		GaItems: []GaItemFull{
			// Handle prefix 0x7 is not a recognized type.
			{Handle: 0x70000001, ItemID: 0x00400110, Unk2: -1, Unk3: -1, AoWGaItemHandle: NoCustomAoWHandle},
		},
	}

	diag := SlotDiagnostics{}
	diag.checkGaItems(slot)

	found := false
	for _, e := range diag.Issues {
		if strings.Contains(e.Description, "unknown type prefix") {
			found = true
		}
	}
	if !found {
		t.Error("expected 'unknown type prefix' warning for handle with prefix 0x7, got none")
	}
}
