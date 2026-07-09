package data

import "testing"

func TestGeneratedGameLimits_KnownRegulationCases(t *testing.T) {
	tests := []struct {
		name      string
		id        uint32
		inventory uint32
		storage   uint32
	}{
		{"Crimson flask +12 full", 0x40000401, 20, 0},
		{"Cerulean flask +12 full", 0x40000433, 20, 0},
		{"Festering Bloody Finger", 0x4000006F, 99, 0},
		{"Glintstone Pebble", 0x40000FA0, 99, 600},
		{`Prattling Pate "Hello"`, 0x40000898, 1, 600},
		{"Remembrance of the Starscourge", 0x40000B87, 99, 600},
		{"Arrow", 0x02FAF080, 99, 600},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := GameLimitsByItemID[tt.id]
			if !ok {
				t.Fatalf("GameLimitsByItemID[0x%08X] missing", tt.id)
			}
			if !got.InventoryKnown || !got.StorageKnown {
				t.Fatalf("known flags = inventory:%v storage:%v, want both true", got.InventoryKnown, got.StorageKnown)
			}
			if got.MaxInventory != tt.inventory || got.MaxStorage != tt.storage {
				t.Fatalf("limits = %d/%d, want %d/%d", got.MaxInventory, got.MaxStorage, tt.inventory, tt.storage)
			}
		})
	}
}
