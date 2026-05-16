package data

import "testing"

// TestPhase2B4BlackSyrupPresent verifies Black Syrup (0x401EA3D3) is the only
// Phase 2B.4 addition. Regulation row 2008019: goodsType=1, iconId=801,
// sortId=204363, maxNum=1 — a shipped SOTE key item missing from app maps
// prior to this change.
//
// Out of scope for Phase 2B.4 (deferred to dedicated investigation):
//   - Scorpion Stew / Gourmet Scorpion Stew Set A 0x401E8930/31 (Set B
//     already present in tools.go, ambiguous A/B canonical choice).
//   - 5 Miquella questline phrases 0x401EA7A8–AC (already exposed via
//     gestures.go under same canonical IDs).
func TestPhase2B4BlackSyrupPresent(t *testing.T) {
	item, ok := KeyItems[0x401EA3D3]
	if !ok {
		t.Fatalf("KeyItems[0x401EA3D3] missing — shipped SOTE Black Syrup must be present")
	}
	if item.Name != "Black Syrup" {
		t.Errorf("KeyItems[0x401EA3D3] name = %q, want %q", item.Name, "Black Syrup")
	}
	if item.Category != "key_items" {
		t.Errorf("KeyItems[0x401EA3D3] category = %q, want %q", item.Category, "key_items")
	}
	if item.MaxInventory != 1 {
		t.Errorf("KeyItems[0x401EA3D3] MaxInventory = %d, want 1", item.MaxInventory)
	}
	if item.MaxStorage != 0 {
		t.Errorf("KeyItems[0x401EA3D3] MaxStorage = %d, want 0", item.MaxStorage)
	}
	if item.MaxUpgrade != 0 {
		t.Errorf("KeyItems[0x401EA3D3] MaxUpgrade = %d, want 0", item.MaxUpgrade)
	}
	if item.IconPath != "items/tools/black_syrup.png" {
		t.Errorf("KeyItems[0x401EA3D3] IconPath = %q, want %q",
			item.IconPath, "items/tools/black_syrup.png")
	}
	hasDLC := false
	for _, f := range item.Flags {
		if f == "dlc" {
			hasDLC = true
		}
		if f == "cut_content" {
			t.Errorf("KeyItems[0x401EA3D3] must NOT carry cut_content flag — shipped item")
		}
		if f == "ban_risk" {
			t.Errorf("KeyItems[0x401EA3D3] must NOT carry ban_risk flag — shipped item")
		}
	}
	if !hasDLC {
		t.Errorf("KeyItems[0x401EA3D3] missing dlc flag, got %v", item.Flags)
	}
}

// TestPhase2B4BlackSyrupNoDuplicateAcrossMaps guards against accidentally adding
// Black Syrup to tools.go or any other map under the same canonical ID, and
// against any other ID using the same display name.
func TestPhase2B4BlackSyrupNoDuplicateAcrossMaps(t *testing.T) {
	maps := []struct {
		name  string
		items map[uint32]ItemData
	}{
		{"Tools", Tools},
		{"Information", Information},
		{"Gestures", Gestures},
		{"StandardAshes", StandardAshes},
		{"ArrowsAndBolts", ArrowsAndBolts},
		{"BolsteringMaterials", BolsteringMaterials},
		{"CraftingMaterials", CraftingMaterials},
		{"Incantations", Incantations},
		{"Sorceries", Sorceries},
	}
	for _, m := range maps {
		if _, ok := m.items[0x401EA3D3]; ok {
			t.Errorf("%s[0x401EA3D3] present — Black Syrup must live only in KeyItems", m.name)
		}
		for id, it := range m.items {
			if it.Name == "Black Syrup" {
				t.Errorf("%s[0x%08X] named %q — duplicate of KeyItems[0x401EA3D3]",
					m.name, id, it.Name)
			}
		}
	}
}

// TestPhase2B4ScorpionStewUntouched confirms Phase 2B.4 left both Scorpion Stew
// variants (Set B in tools.go, Set A NOT in app) unchanged. The Set A/B
// canonical choice is deferred to a dedicated investigation.
func TestPhase2B4ScorpionStewUntouched(t *testing.T) {
	if _, ok := KeyItems[0x401E8930]; ok {
		t.Errorf("KeyItems[0x401E8930] present — Scorpion Stew Set A must remain out " +
			"of app pending Set A/B canonical investigation")
	}
	if _, ok := KeyItems[0x401E8931]; ok {
		t.Errorf("KeyItems[0x401E8931] present — Gourmet Scorpion Stew Set A must " +
			"remain out of app pending Set A/B canonical investigation")
	}
	setB := Tools[0x401E8932]
	if setB.Name != "Scorpion Stew" {
		t.Errorf("Tools[0x401E8932] name = %q, want %q (Set B canonical kept intact)",
			setB.Name, "Scorpion Stew")
	}
	gourmetB := Tools[0x401E8933]
	if gourmetB.Name != "Gourmet Scorpion Stew" {
		t.Errorf("Tools[0x401E8933] name = %q, want %q (Set B canonical kept intact)",
			gourmetB.Name, "Gourmet Scorpion Stew")
	}
}

// TestPhase2B4MiquellaPhrasesStayInGestures confirms the 5 Miquella questline
// phrase IDs remain exposed via Gestures map only — they must NOT be duplicated
// into KeyItems or Tools. The audit gap (Gesture↔Goods cross-source matching)
// is tooling-side and deferred to a dedicated cleanup.
func TestPhase2B4MiquellaPhrasesStayInGestures(t *testing.T) {
	miquellaPhrases := []uint32{
		0x401EA7A8, // Ring of Miquella
		0x401EA7A9, // May the Best Win
		0x401EA7AA, // The Two Fingers
		0x401EA7AB, // Let Us Go Together
		0x401EA7AC, // O Mother
	}
	for _, id := range miquellaPhrases {
		if _, ok := Gestures[id]; !ok {
			t.Errorf("Gestures[0x%08X] missing — Miquella phrase must remain in gestures", id)
		}
		if _, ok := KeyItems[id]; ok {
			t.Errorf("KeyItems[0x%08X] present — Miquella phrase must not be duplicated "+
				"into key_items", id)
		}
		if _, ok := Tools[id]; ok {
			t.Errorf("Tools[0x%08X] present — Miquella phrase must not be duplicated "+
				"into tools", id)
		}
	}
}
