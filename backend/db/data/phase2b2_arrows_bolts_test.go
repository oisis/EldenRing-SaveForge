package data

import "testing"

// phase2b2Additions enumerates the 4 base-game arrows/bolts added in
// Phase 2B.2. Data verified against:
//   - tmp/regulation-bin-dump/csv/EquipParamWeapon.csv      (wepType)
//   - tmp/regulation-bin-dump/msg/name_mapping.csv          (FMG names)
//   - tmp/item-audit/phase2_classification_missing.csv      (audit classification)
//
// All four are base-game (non-DLC) elemental ammunition with MaxUpgrade=0.
// Stack caps follow the existing subcategory convention:
//   - Arrows/Bolts: MaxInventory=99, MaxStorage=600
//   - Greatarrows:  MaxInventory=30, MaxStorage=600
//   - Greatbolts (Ballista Bolt class, wepType=86): MaxInventory=20, MaxStorage=600
var phase2b2Additions = []struct {
	ID           uint32
	Name         string
	SubCategory  string
	MaxInventory uint32
	MaxStorage   uint32
}{
	{0x02FB1790, "Fire Arrow", SubcatArrowsArrows, 99, 600},
	{0x030AA7F0, "Golem's Magic Arrow", SubcatArrowsGreatarrows, 30, 600},
	{0x03199C10, "Lightning Bolt", SubcatArrowsBolts, 99, 600},
	{0x0328DE50, "Lightning Greatbolt", SubcatArrowsGreatbolts, 20, 600},
}

func TestPhase2B2ArrowsBoltsPresent(t *testing.T) {
	for _, w := range phase2b2Additions {
		item, ok := ArrowsAndBolts[w.ID]
		if !ok {
			t.Errorf("ArrowsAndBolts[0x%08X] (%s): missing entry", w.ID, w.Name)
			continue
		}
		if item.Name != w.Name {
			t.Errorf("ArrowsAndBolts[0x%08X] name = %q, want %q", w.ID, item.Name, w.Name)
		}
		if item.Category != "arrows_and_bolts" {
			t.Errorf("ArrowsAndBolts[0x%08X] (%s) category = %q, want %q",
				w.ID, w.Name, item.Category, "arrows_and_bolts")
		}
		if item.SubCategory != w.SubCategory {
			t.Errorf("ArrowsAndBolts[0x%08X] (%s) sub-category = %q, want %q",
				w.ID, w.Name, item.SubCategory, w.SubCategory)
		}
		if item.MaxInventory != w.MaxInventory {
			t.Errorf("ArrowsAndBolts[0x%08X] (%s) MaxInventory = %d, want %d",
				w.ID, w.Name, item.MaxInventory, w.MaxInventory)
		}
		if item.MaxStorage != w.MaxStorage {
			t.Errorf("ArrowsAndBolts[0x%08X] (%s) MaxStorage = %d, want %d",
				w.ID, w.Name, item.MaxStorage, w.MaxStorage)
		}
		if item.MaxUpgrade != 0 {
			t.Errorf("ArrowsAndBolts[0x%08X] (%s) MaxUpgrade = %d, want 0",
				w.ID, w.Name, item.MaxUpgrade)
		}
		hasStackable := false
		hasDLC := false
		for _, f := range item.Flags {
			switch f {
			case "stackable":
				hasStackable = true
			case "dlc":
				hasDLC = true
			}
		}
		if !hasStackable {
			t.Errorf("ArrowsAndBolts[0x%08X] (%s) missing 'stackable' flag",
				w.ID, w.Name)
		}
		if hasDLC {
			t.Errorf("ArrowsAndBolts[0x%08X] (%s) carries 'dlc' flag but is base-game",
				w.ID, w.Name)
		}
	}
}

// TestPhase2B2StackSizesMatchNeighbours guards against drift: each new entry's
// stack caps must match the cap convention of an already-present sibling in
// the same sub-category.
func TestPhase2B2StackSizesMatchNeighbours(t *testing.T) {
	siblings := map[uint32]uint32{
		0x02FB1790: 0x02FAF080, // Fire Arrow            ↔ Arrow
		0x030AA7F0: 0x030A32C0, // Golem's Magic Arrow   ↔ Great Arrow
		0x03199C10: 0x03197500, // Lightning Bolt        ↔ Bolt
		0x0328DE50: 0x0328B740, // Lightning Greatbolt   ↔ Ballista Bolt
	}
	for newID, refID := range siblings {
		newItem := ArrowsAndBolts[newID]
		ref := ArrowsAndBolts[refID]
		if newItem.MaxInventory != ref.MaxInventory {
			t.Errorf("0x%08X (%s) MaxInventory=%d does not match sibling 0x%08X (%s) MaxInventory=%d",
				newID, newItem.Name, newItem.MaxInventory, refID, ref.Name, ref.MaxInventory)
		}
		if newItem.MaxStorage != ref.MaxStorage {
			t.Errorf("0x%08X (%s) MaxStorage=%d does not match sibling 0x%08X (%s) MaxStorage=%d",
				newID, newItem.Name, newItem.MaxStorage, refID, ref.Name, ref.MaxStorage)
		}
	}
}
