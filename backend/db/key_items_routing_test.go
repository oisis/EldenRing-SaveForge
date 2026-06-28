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
