package db

import "testing"

func TestGetItemsByCategory_WhetstoneKnifeUsesWhetbladeUnlock(t *testing.T) {
	items := GetItemsByCategory("key_items", "PC")
	for _, item := range items {
		if item.ID != 0x4000218E {
			continue
		}
		if item.Name != "Whetstone Knife" {
			t.Fatalf("name = %q, want Whetstone Knife", item.Name)
		}
		if item.UnlockCategory != "whetblade" {
			t.Fatalf("UnlockCategory = %q, want whetblade", item.UnlockCategory)
		}
		if item.FlagID != 60130 {
			t.Fatalf("FlagID = %d, want 60130", item.FlagID)
		}
		if item.MaxInventory != 1 || item.MaxStorage != 0 {
			t.Fatalf("caps = inv %d storage %d, want 1/0", item.MaxInventory, item.MaxStorage)
		}
		return
	}
	t.Fatal("Whetstone Knife not found in key_items")
}

// TestGetItemsByCategory_AllFiveWhetbladesExposed locks issues 3/4: the five
// obtainable Whetblades must appear in the Key Items picker and route through
// the World-tab whetblade unlock path (not the ordinary add-item path), while
// Memory Stone and Talisman Pouch stay hidden.
func TestGetItemsByCategory_AllFiveWhetbladesExposed(t *testing.T) {
	want := map[uint32]struct {
		name   string
		flagID uint32
	}{
		0x4000230A: {"Iron Whetblade", 65610},
		0x4000230B: {"Red-Hot Whetblade", 65640},
		0x4000230C: {"Sanctified Whetblade", 65660},
		0x4000230D: {"Glintstone Whetblade", 65680},
		0x4000230E: {"Black Whetblade", 65700},
	}
	hidden := map[uint32]string{
		0x4000272E: "Memory Stone",
		0x40002738: "Talisman Pouch",
	}

	items := GetItemsByCategory("key_items", "PC")
	seen := map[uint32]bool{}
	for _, item := range items {
		if h, ok := hidden[item.ID]; ok {
			t.Errorf("%s (0x%08X) must stay hidden but appears in picker", h, item.ID)
		}
		w, ok := want[item.ID]
		if !ok {
			continue
		}
		seen[item.ID] = true
		if item.Name != w.name {
			t.Errorf("0x%08X name = %q, want %q", item.ID, item.Name, w.name)
		}
		if item.UnlockCategory != "whetblade" {
			t.Errorf("%s UnlockCategory = %q, want whetblade (unlock path, not add-item)", w.name, item.UnlockCategory)
		}
		if item.FlagID != w.flagID {
			t.Errorf("%s FlagID = %d, want %d", w.name, item.FlagID, w.flagID)
		}
		if item.MaxInventory != 1 || item.MaxStorage != 0 {
			t.Errorf("%s caps = inv %d storage %d, want 1/0", w.name, item.MaxInventory, item.MaxStorage)
		}
	}

	if len(seen) != 5 {
		t.Errorf("exposed %d Whetblades, want 5 (issue 4): seen %v", len(seen), seen)
	}
}
