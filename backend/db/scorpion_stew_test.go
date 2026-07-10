package db

import "testing"

// TestScorpionStew_CanonicalRowsResolve proves both canonical FMG rows resolve
// by raw itemID and by goods handle: 0x401E8930 = Scorpion Stew, 0x401E8933 =
// Gourmet Scorpion Stew. These are the rows that carry item text.
func TestScorpionStew_CanonicalRowsResolve(t *testing.T) {
	cases := []struct {
		ids  []uint32 // raw itemID + goods handle
		name string
		base uint32
	}{
		{[]uint32{0x401E8930, 0xB01E8930}, "Scorpion Stew", 0x401E8930},
		{[]uint32{0x401E8933, 0xB01E8933}, "Gourmet Scorpion Stew", 0x401E8933},
	}
	for _, c := range cases {
		for _, id := range c.ids {
			got, baseID := GetItemDataFuzzy(id)
			if got.Name != c.name {
				t.Errorf("GetItemDataFuzzy(0x%08X).Name = %q, want %q", id, got.Name, c.name)
			}
			if baseID != c.base {
				t.Errorf("GetItemDataFuzzy(0x%08X) baseID = 0x%08X, want 0x%08X", id, baseID, c.base)
			}
		}
	}
}

// TestScorpionStew_TechnicalVariantsAlias proves the twin rows (no FMG text)
// resolve to their canonical row via the alias table (resolver-safety) without
// being DB entries of their own. 0x401E8931 is the reported failure: a genuine
// save carried handle 0xB01E8931 and it scanned as unknown_item_id.
func TestScorpionStew_TechnicalVariantsAlias(t *testing.T) {
	cases := []struct {
		ids  []uint32 // raw itemID + goods handle
		name string
		base uint32
	}{
		{[]uint32{0x401E8932, 0xB01E8932}, "Scorpion Stew", 0x401E8930},
		{[]uint32{0x401E8931, 0xB01E8931}, "Gourmet Scorpion Stew", 0x401E8933},
	}
	for _, c := range cases {
		if got := GetItemData(c.ids[0]); got.Name != "" {
			t.Errorf("GetItemData(0x%08X) = %q, want empty (variant must not be a DB entry)", c.ids[0], got.Name)
		}
		for _, id := range c.ids {
			got, baseID := GetItemDataFuzzy(id)
			if got.Name != c.name || baseID != c.base {
				t.Errorf("GetItemDataFuzzy(0x%08X) = (%q, 0x%08X), want (%q, 0x%08X)", id, got.Name, baseID, c.name, c.base)
			}
		}
	}
}

// TestScorpionStew_CategoryExposure locks the picker layout: exactly one
// visible Scorpion Stew (0x401E8930) and one Gourmet Scorpion Stew
// (0x401E8933); neither technical variant leaks in as a separate row.
func TestScorpionStew_CategoryExposure(t *testing.T) {
	var stewIDs, gourmetIDs []uint32
	for _, e := range GetItemsByCategory("tools", "PC") {
		switch e.Name {
		case "Scorpion Stew":
			stewIDs = append(stewIDs, e.ID)
		case "Gourmet Scorpion Stew":
			gourmetIDs = append(gourmetIDs, e.ID)
		}
		if e.ID == 0x401E8931 || e.ID == 0x401E8932 {
			t.Errorf("technical variant 0x%08X exposed in tools picker (%q); must stay absent", e.ID, e.Name)
		}
	}

	if len(stewIDs) != 1 || stewIDs[0] != 0x401E8930 {
		t.Errorf("Scorpion Stew IDs = %#x, want exactly [0x401E8930]", stewIDs)
	}
	if len(gourmetIDs) != 1 || gourmetIDs[0] != 0x401E8933 {
		t.Errorf("Gourmet Scorpion Stew IDs = %#x, want exactly [0x401E8933]", gourmetIDs)
	}
}

// TestScorpionStew_VisibleRowsCarryText guards the concrete regression this
// mapping avoids: the enriched detail card (GetItemEntryByID) for each visible
// row must carry FMG item text. Both canonical IDs have an ItemTexts entry;
// the technical twins (8931/8932) do not, which is why they must stay hidden.
func TestScorpionStew_VisibleRowsCarryText(t *testing.T) {
	for _, id := range []uint32{0x401E8930, 0x401E8933} {
		e := GetItemEntryByID(id)
		if e == nil {
			t.Fatalf("GetItemEntryByID(0x%08X) = nil, want enriched entry", id)
		}
		if e.Text == nil || e.Text.Caption == "" {
			t.Errorf("visible row 0x%08X (%q) has no item text/caption; canonical row must carry FMG text", id, e.Name)
		}
	}
}
