package db

import "testing"

// The five technical variant GoodsParam rows observed in a real save
// (tmp/save/ER0000.sl2 slot 0 "[PL] Jagna"). Each is a legal in-game item ID
// that shares its base item's params; they were previously absent from the DB
// and therefore raised false-positive unknown_item_id in the repair scanner.
var technicalVariantIDs = map[uint32]string{
	0x40002AFA: "Crimson Crystal Tear (Variant)",
	0x40002AFC: "Cerulean Crystal Tear (Variant)",
	0x40002B08: "Ruptured Crystal Tear (Variant)",
	0x40001FAD: "Academy Glintstone Key (Variant)",
	0x40001FD2: "Miniature Ranni (Variant)",
}

// TestTechnicalVariantIDs_KnownInDB guards that each variant resolves to a
// non-empty DB entry via both the exact and fuzzy lookups, carries the
// key_items category, and inherits the regulation game caps (MaxStorage stays 0
// because these goods have isDeposit=0).
func TestTechnicalVariantIDs_KnownInDB(t *testing.T) {
	for id, wantName := range technicalVariantIDs {
		exact := GetItemData(id)
		if exact.Name != wantName {
			t.Errorf("GetItemData(0x%08X).Name = %q, want %q", id, exact.Name, wantName)
		}
		if exact.Category != "key_items" {
			t.Errorf("GetItemData(0x%08X).Category = %q, want key_items", id, exact.Category)
		}
		if !exact.GameMaxInventoryKnown || exact.GameMaxInventory != 1 {
			t.Errorf("GetItemData(0x%08X) game inventory cap = %d (known=%v), want 1 (known)", id, exact.GameMaxInventory, exact.GameMaxInventoryKnown)
		}
		if !exact.GameMaxStorageKnown || exact.GameMaxStorage != 0 {
			t.Errorf("GetItemData(0x%08X) game storage cap = %d (known=%v), want 0 (known)", id, exact.GameMaxStorage, exact.GameMaxStorageKnown)
		}

		fuzzy, baseID := GetItemDataFuzzy(id)
		if fuzzy.Name != wantName || baseID != id {
			t.Errorf("GetItemDataFuzzy(0x%08X) = (%q, 0x%08X), want (%q, 0x%08X)", id, fuzzy.Name, baseID, wantName, id)
		}
	}
}

// TestTechnicalVariantIDs_UnrelatedIDStillUnknown ensures the additive DB
// entries did not turn a genuinely absent ID into a resolvable one.
func TestTechnicalVariantIDs_UnrelatedIDStillUnknown(t *testing.T) {
	const bogus = uint32(0x40009999)
	if got := GetItemData(bogus); got.Name != "" {
		t.Errorf("GetItemData(0x%08X).Name = %q, want empty (unknown)", bogus, got.Name)
	}
}
