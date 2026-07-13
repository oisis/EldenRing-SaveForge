package data

// shields_subcat.go — sub-category assignment for the Shields tab.
//
// Sub-groups (in-game order):
//   1. Torches  (top of tab — left-hand light source)
//   2. Small Shields (bucklers, parry shields, light shields)
//   3. Medium Shields (kite, heater, crest)
//   4. Greatshields (towershields, full-block heavies)
//   5. Thrusting Shields (DLC — wepType 90)
//
// Classification is driven by canonical EquipParamWeapon.wepType
// (shieldWepTypeSubcat: 65 Small, 67 Medium, 69 Greatshield, 87 Torch,
// 90 Thrusting). The name-based classifyShield below is only a fallback for
// items missing from the wepType table.

import "strings"

var shieldInfusionPrefixes = []string{
	"Heavy ", "Keen ", "Quality ", "Fire ", "Flame Art ", "Lightning ",
	"Sacred ", "Magic ", "Cold ", "Poison ", "Blood ", "Occult ",
	"Bloody ", "Cracked ",
}

// shieldsSmall — base names that classify as Small Shields.
var shieldsSmall = map[string]struct{}{
	"Shield of Night":          {}, // DLC, parry shield
	"Buckler":                  {},
	"Perfumer's Shield":        {},
	"Man-Serpent's Shield":     {},
	"Rickety Shield":           {},
	"Pillory Shield":           {},
	"Beastman's Jar-Shield":    {},
	"Red Thorn Roundshield":    {},
	"Scripture Wooden Shield":  {},
	"Riveted Wooden Shield":    {},
	"Blue-White Wooden Shield": {},
	"Rift Shield":              {},
	"Iron Roundshield":         {},
	"Gilded Iron Shield":       {},
	"Ice Crest Shield":         {},
	"Smoldering Shield":        {},
	"Spiralhorn Shield":        {},
	"Coil Shield":              {},
	"Smithscript Shield":       {}, // DLC
}

// shieldsGreatshield — base names that classify as Greatshields.
var shieldsGreatshield = map[string]struct{}{
	"Dragon Towershield":          {},
	"Distinguished Greatshield":   {},
	"Crucible Hornshield":         {},
	"Dragonclaw Shield":           {},
	"Briar Greatshield":           {},
	"Erdtree Greatshield":         {},
	"Golden Beast Crest Shield":   {},
	"Jellyfish Shield":            {},
	"Fingerprint Stone Shield":    {},
	"Icon Shield":                 {},
	"One-Eyed Shield":             {},
	"Visage Shield":               {},
	"Spiked Palisade Shield":      {},
	"Manor Towershield":           {},
	"Crossed-Tree Towershield":    {},
	"Inverted Hawk Towershield":   {},
	"Redmane Greatshield":         {},
	"Eclipse Crest Greatshield":   {},
	"Cuckoo Greatshield":          {},
	"Golden Greatshield":          {},
	"Gilded Greatshield":          {},
	"Haligtree Crest Greatshield": {},
	"Wooden Greatshield":          {},
	"Lordsworn's Shield":          {},
	"Black Steel Greatshield":     {}, // DLC
	"Verdigris Greatshield":       {}, // DLC
	"Great Turtle Shell":          {},
	"Ant's Skull Plate":           {},
}

// stripShieldInfusionPrefix returns the base shield name with any leading
// infusion qualifier removed (e.g., "Heavy Wolf Crest Shield" → "Wolf Crest Shield").
func stripShieldInfusionPrefix(name string) string {
	for _, p := range shieldInfusionPrefixes {
		if strings.HasPrefix(name, p) {
			return name[len(p):]
		}
	}
	return name
}

func classifyShield(name string) string {
	if strings.Contains(name, "Torch") || name == "Torchpole" || name == "Lamenting Visage" {
		return SubcatShieldsTorches
	}
	base := stripShieldInfusionPrefix(name)
	if _, ok := shieldsSmall[base]; ok {
		return SubcatShieldsSmall
	}
	if _, ok := shieldsGreatshield[base]; ok {
		return SubcatShieldsGreatshields
	}
	return SubcatShieldsMedium
}

func init() {
	for id, item := range Shields {
		if item.SubCategory != "" {
			continue
		}
		// Canonical wepType wins; name heuristic is the fallback.
		if sc, ok := wepTypeSubcat(id, shieldWepTypeSubcat); ok {
			item.SubCategory = sc
		} else {
			item.SubCategory = classifyShield(item.Name)
		}
		Shields[id] = item
	}
}
