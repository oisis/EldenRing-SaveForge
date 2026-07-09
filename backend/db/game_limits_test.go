package db

import "testing"

func TestGetItemData_PreservesSafeCapsAndAddsGameCaps(t *testing.T) {
	tests := []struct {
		id                         uint32
		safeInventory, safeStorage uint32
		gameInventory, gameStorage uint32
	}{
		{0x40000401, 1, 0, 20, 0},   // Flask of Crimson Tears +12
		{0x4000006F, 1, 99, 99, 0},  // Festering Bloody Finger
		{0x40000FA0, 1, 0, 99, 600}, // Glintstone Pebble
	}
	for _, tt := range tests {
		item := GetItemData(tt.id)
		if item.Name == "" {
			t.Fatalf("GetItemData(0x%08X) did not resolve", tt.id)
		}
		if item.MaxInventory != tt.safeInventory || item.MaxStorage != tt.safeStorage {
			t.Errorf("0x%08X safe caps = %d/%d, want %d/%d",
				tt.id, item.MaxInventory, item.MaxStorage, tt.safeInventory, tt.safeStorage)
		}
		if !item.GameMaxInventoryKnown || !item.GameMaxStorageKnown {
			t.Errorf("0x%08X game limit known flags = %v/%v, want true/true",
				tt.id, item.GameMaxInventoryKnown, item.GameMaxStorageKnown)
		}
		if item.GameMaxInventory != tt.gameInventory || item.GameMaxStorage != tt.gameStorage {
			t.Errorf("0x%08X game caps = %d/%d, want %d/%d",
				tt.id, item.GameMaxInventory, item.GameMaxStorage, tt.gameInventory, tt.gameStorage)
		}
	}
}
