package db

import "testing"

// TestGetItemsByCategory_TalismanWeight verifies that the "talismans" category
// is included in weightedCategory and that enrichItemEntry populates Weight
// for talismans from data.ItemWeights.
func TestGetItemsByCategory_TalismanWeight(t *testing.T) {
	items := GetItemsByCategory("talismans", "PC")
	if len(items) == 0 {
		t.Fatal("GetItemsByCategory(talismans) returned no items")
	}

	var withWeight int
	for _, item := range items {
		if item.Weight > 0 {
			withWeight++
		}
	}
	if withWeight == 0 {
		t.Error("no talisman has Weight > 0; weightedCategory or ItemWeights lookup is broken")
	}

	// At least 90% of talismans should have weight data (a few obscure entries may be 0).
	pct := float64(withWeight) / float64(len(items))
	if pct < 0.9 {
		t.Errorf("only %.0f%% of talismans have Weight > 0 (%d/%d)", pct*100, withWeight, len(items))
	}
}

func TestGetItemsByCategory_CrimsonAmberMedallionWeight(t *testing.T) {
	items := GetItemsByCategory("talismans", "PC")
	for _, item := range items {
		if item.Name == "Crimson Amber Medallion" {
			if item.Weight <= 0 {
				t.Errorf("Crimson Amber Medallion: want Weight > 0, got %g", item.Weight)
			}
			return
		}
	}
	t.Error("Crimson Amber Medallion not found in talismans category")
}
