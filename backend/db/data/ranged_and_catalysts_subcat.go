package data

// ranged_and_catalysts_subcat.go — sub-category assignment for the
// "Ranged Weapons / Catalysts" tab.
//
// Sub-groups (in-game order):
//   1. Bows
//   2. Light Bows
//   3. Greatbows
//   4. Crossbows
//   5. Ballistas
//   6. Glintstone Staffs
//   7. Sacred Seals
//
// Classification rules (highest priority first):
//   1. Name in ballistasByName set                → Ballistas
//   2. Name has "Greatbow" suffix                  → Greatbows
//   3. Name has "Shortbow" suffix                  → Light Bows
//   4. Name has "Crossbow" or "Arbalest" in name   → Crossbows
//   5. Name has "Bow" suffix                       → Bows
//   6. Name has "Seal" suffix                      → Sacred Seals
//   7. Otherwise (Staff, Scepter, Staff-Spear)     → Glintstone Staffs
//
// Note: a few items in ranged_and_catalysts.go are mislabeled at source
// (Devourer's Scepter — actually a Hammer; Barbed Staff-Spear — actually a
// Heavy Thrusting Sword). Best-effort sub-cat assignment by the rule above;
// fix the underlying mislabeling in a separate pass.

import "strings"

var ballistasByName = map[string]struct{}{
	"Hand Ballista":    {},
	"Jar Cannon":       {},
	"Rabbath's Cannon": {},
}

func classifyRangedCatalyst(name string) string {
	if _, ok := ballistasByName[name]; ok {
		return SubcatRangedBallistas
	}
	if strings.HasSuffix(name, "Greatbow") {
		return SubcatRangedGreatbows
	}
	if strings.HasSuffix(name, "Shortbow") {
		return SubcatRangedLightBows
	}
	if strings.Contains(name, "Crossbow") || name == "Arbalest" {
		return SubcatRangedCrossbows
	}
	if strings.HasSuffix(name, "Bow") {
		return SubcatRangedBows
	}
	if strings.HasSuffix(name, "Seal") {
		return SubcatRangedSacredSeals
	}
	return SubcatRangedGlintstoneStaffs
}

func init() {
	for id, item := range RangedAndCatalysts {
		if item.SubCategory != "" {
			continue
		}
		// Canonical wepType wins; name heuristic is the fallback. Cross-category
		// source mislabels (e.g. Rotten Staff, wepType 41) are not in
		// rangedWepTypeSubcat and stay on the name fallback by design.
		if sc, ok := wepTypeSubcat(id, rangedWepTypeSubcat); ok {
			item.SubCategory = sc
		} else {
			item.SubCategory = classifyRangedCatalyst(item.Name)
		}
		RangedAndCatalysts[id] = item
	}
}
