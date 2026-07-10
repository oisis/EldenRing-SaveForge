package db

import "testing"

// TestScorpionStew_Row8930Resolves proves the newly added second Scorpion Stew
// row (issue 5) resolves as a known DB item by raw ID and by goods handle.
func TestScorpionStew_Row8930Resolves(t *testing.T) {
	cases := []uint32{0x401E8930, 0xB01E8930} // raw itemID + goods handle
	for _, id := range cases {
		got, baseID := GetItemDataFuzzy(id)
		if got.Name != "Scorpion Stew" {
			t.Errorf("GetItemDataFuzzy(0x%08X).Name = %q, want %q", id, got.Name, "Scorpion Stew")
		}
		if baseID != 0x401E8930 {
			t.Errorf("GetItemDataFuzzy(0x%08X) baseID = 0x%08X, want 0x401E8930", id, baseID)
		}
	}
}

// TestScorpionStew_CategoryExposure locks the picker layout: two visible
// Scorpion Stew rows, exactly one Gourmet Scorpion Stew, and no 0x401E8931.
func TestScorpionStew_CategoryExposure(t *testing.T) {
	var stewIDs []uint32
	var gourmetIDs []uint32
	for _, e := range GetItemsByCategory("tools", "PC") {
		switch e.Name {
		case "Scorpion Stew":
			stewIDs = append(stewIDs, e.ID)
		case "Gourmet Scorpion Stew":
			gourmetIDs = append(gourmetIDs, e.ID)
		}
		if e.ID == 0x401E8931 {
			t.Errorf("0x401E8931 exposed in tools picker (%q); must stay absent", e.Name)
		}
	}

	wantStew := map[uint32]bool{0x401E8930: false, 0x401E8932: false}
	for _, id := range stewIDs {
		if _, ok := wantStew[id]; ok {
			wantStew[id] = true
		} else {
			t.Errorf("unexpected Scorpion Stew ID 0x%08X in picker", id)
		}
	}
	for id, seen := range wantStew {
		if !seen {
			t.Errorf("Scorpion Stew ID 0x%08X missing from picker", id)
		}
	}

	if len(gourmetIDs) != 1 || gourmetIDs[0] != 0x401E8933 {
		t.Errorf("Gourmet Scorpion Stew IDs = %#x, want exactly [0x401E8933]", gourmetIDs)
	}
}

// TestScorpionStew_8931Absent guards that the never-observed candidate resolves
// to nothing and is not a DB entry.
func TestScorpionStew_8931Absent(t *testing.T) {
	if got := GetItemData(0x401E8931); got.Name != "" {
		t.Errorf("GetItemData(0x401E8931) = %q, want empty", got.Name)
	}
}
