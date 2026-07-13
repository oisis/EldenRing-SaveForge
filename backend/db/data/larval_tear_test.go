package data

import "testing"

func TestLarvalTearsAreSeparateRecordsWithCorrectLimits(t *testing.T) {
	tests := []struct {
		id           uint32
		name         string
		maxInventory uint32
		iconPath     string
		isDLC        bool
	}{
		{0x40001FF9, "Larval Tear", 18, "items/key_items/larval_tear.png", false},
		{0x401EA3E1, "Larval Tear (DLC)", 6, "items/key_items/larval_tear_dlc.png", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			item, ok := KeyItems[tt.id]
			if !ok {
				t.Fatalf("KeyItems[0x%08X] missing", tt.id)
			}
			if item.Name != tt.name {
				t.Errorf("Name = %q, want %q", item.Name, tt.name)
			}
			if item.MaxInventory != tt.maxInventory || item.MaxStorage != 0 {
				t.Errorf("safe caps = %d/%d, want %d/0", item.MaxInventory, item.MaxStorage, tt.maxInventory)
			}
			if item.IconPath != tt.iconPath {
				t.Errorf("IconPath = %q, want %q", item.IconPath, tt.iconPath)
			}
			if item.SubCategory != SubcatKeyLarvalDeathroot {
				t.Errorf("SubCategory = %q, want %q", item.SubCategory, SubcatKeyLarvalDeathroot)
			}
			if itemHasFlag(item.Flags, "dlc") != tt.isDLC {
				t.Errorf("dlc flag = %t, want %t", itemHasFlag(item.Flags, "dlc"), tt.isDLC)
			}
			if !itemHasFlag(item.Flags, "stackable") || !itemHasFlag(item.Flags, "scales_with_ng") {
				t.Errorf("Flags = %v, want stackable and scales_with_ng", item.Flags)
			}

			limits, ok := GameLimitsByItemID[tt.id]
			if !ok {
				t.Fatalf("GameLimitsByItemID[0x%08X] missing", tt.id)
			}
			if !limits.InventoryKnown || !limits.StorageKnown || limits.MaxInventory != 99 || limits.MaxStorage != 600 {
				t.Errorf("game caps = %+v, want known 99/600", limits)
			}
		})
	}

	if _, aliased := TechnicalItemAliases[0x401EA3E1]; aliased {
		t.Error("DLC Larval Tear must be an independent record, not a technical alias")
	}
}

func TestLarvalTearVariantTextsStayDistinct(t *testing.T) {
	base, baseOK := ItemTexts[0x40001FF9]
	dlc, dlcOK := ItemTexts[0x401EA3E1]
	if !baseOK || !dlcOK {
		t.Fatalf("ItemTexts presence: base=%t DLC=%t, want both", baseOK, dlcOK)
	}
	if base.DisplayName != "Larval Tear" || dlc.DisplayName != "Larval Tear (DLC)" {
		t.Errorf("display names = %q / %q, want base and DLC variants", base.DisplayName, dlc.DisplayName)
	}
	if base.CanonicalName != "Larval Tear" || dlc.CanonicalName != "Larval Tear" {
		t.Errorf("canonical names = %q / %q, want the game's Larval Tear label", base.CanonicalName, dlc.CanonicalName)
	}
	if base.Caption == dlc.Caption {
		t.Error("base and DLC captions must preserve their distinct in-game text")
	}
	if dlc.DLCSource != "dlc01" || dlc.DisplayNameSource != TextSourceMixed {
		t.Errorf("DLC text sources = DLC %q, display %q; want dlc01/Mixed", dlc.DLCSource, dlc.DisplayNameSource)
	}
}
