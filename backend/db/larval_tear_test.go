package db

import "testing"

func TestLarvalTearVariantsResolveAndListSeparately(t *testing.T) {
	cases := []struct {
		id        uint32
		handle    uint32
		name      string
		safeMax   uint32
		gameMax   uint32
		gameStore uint32
	}{
		{0x40001FF9, 0xB0001FF9, "Larval Tear", 18, 99, 600},
		{0x401EA3E1, 0xB01EA3E1, "Larval Tear (DLC)", 6, 99, 600},
	}

	entries := GetItemsByCategory("key_items", "PC")
	for _, tt := range cases {
		exact := GetItemData(tt.id)
		if exact.Name != tt.name {
			t.Errorf("GetItemData(0x%08X).Name = %q, want %q", tt.id, exact.Name, tt.name)
		}
		if exact.MaxInventory != tt.safeMax {
			t.Errorf("GetItemData(0x%08X).MaxInventory = %d, want %d", tt.id, exact.MaxInventory, tt.safeMax)
		}
		fuzzy, baseID := GetItemDataFuzzy(tt.handle)
		if fuzzy.Name != tt.name || baseID != tt.id {
			t.Errorf("GetItemDataFuzzy(0x%08X) = %q / 0x%08X, want %q / 0x%08X", tt.handle, fuzzy.Name, baseID, tt.name, tt.id)
		}

		var found *ItemEntry
		for i := range entries {
			if entries[i].ID == tt.id {
				found = &entries[i]
				break
			}
		}
		if found == nil {
			t.Errorf("Key Items picker is missing 0x%08X (%s)", tt.id, tt.name)
			continue
		}
		if found.GameMaxInventory != tt.gameMax || found.GameMaxStorage != tt.gameStore {
			t.Errorf("picker game caps for %s = %d/%d, want %d/%d", tt.name, found.GameMaxInventory, found.GameMaxStorage, tt.gameMax, tt.gameStore)
		}
	}
}
