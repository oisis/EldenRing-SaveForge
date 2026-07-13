package data

// key_items_subcat.go — sub-category assignment for the Key Items tab.
//
// Sub-groups (in-game order, 9 total per spec/36):
//   1. Active Great Runes               (empty in DB — same item IDs as inactive)
//   2. Crystal Tears
//   3. Containers + Slot Upgrades
//   4. Inactive Great Runes + Keys + Medallions  (catch-all for misc keys & quest items)
//   5. DLC Keys
//   6. Larval Tears + Deathroot + Lost Ashes of War
//   7. Cookbooks                         (incl. Crafting Kit, Spirit Calling Bell, Whetblades, Sewing Needles)
//   8. World Maps                        (already assigned in key_items.go)
//   9. Sorcery Scrolls + Incantation Scrolls
//
// Classification is best-effort: many key items don't fit cleanly into the 9
// game-UI groups (e.g., quest tokens, NPC unlocks, story items). These fall
// through to "Inactive Great Runes + Keys + Medallions" as the catch-all.

import "strings"

// crystalTearIDs — every Flask of Wondrous Physick tear (base + DLC). Membership
// is the canonical set EquipParamGoods.goodsType == 10 (Wondrous Physick tear),
// restricted to the IDs defined in this editor's KeyItems map. Source of truth:
// tmp/regulation-bin-dump/csv/EquipParamGoods.csv. Larval Tear and
// "Asimi, Silver Tear" have goodsType != 10 and are intentionally excluded.
var crystalTearIDs = map[uint32]struct{}{
	// Base game (0x40002AF8–0x40002B17)
	0x40002AF8: {}, 0x40002AF9: {}, 0x40002AFA: {}, 0x40002AFB: {},
	0x40002AFC: {}, 0x40002AFD: {}, 0x40002AFE: {}, 0x40002AFF: {},
	0x40002B00: {}, 0x40002B01: {}, 0x40002B02: {}, 0x40002B03: {},
	0x40002B04: {}, 0x40002B05: {}, 0x40002B06: {}, 0x40002B07: {},
	0x40002B08: {}, 0x40002B09: {}, 0x40002B0A: {}, 0x40002B0B: {},
	0x40002B0C: {}, 0x40002B0D: {}, 0x40002B0E: {}, 0x40002B0F: {},
	0x40002B10: {}, 0x40002B11: {}, 0x40002B12: {}, 0x40002B13: {},
	0x40002B14: {}, 0x40002B15: {}, 0x40002B16: {}, 0x40002B17: {},
	// DLC (Shadow of the Erdtree)
	0x401EAF78: {}, 0x401EAF82: {}, 0x401EAF8C: {}, 0x401EAF96: {},
	0x401EAFA0: {}, 0x401EAFAA: {}, 0x401EAFB4: {}, 0x401EAFBE: {},
}

// containerNames — empty pots / bottles / slot upgrades.
var containerNames = map[string]struct{}{
	"Cracked Pot":       {},
	"Ritual Pot":        {},
	"Perfume Bottle":    {},
	"Hefty Cracked Pot": {},
	"Memory Stone":      {},
	"Talisman Pouch":    {},
}

// larvalDeathrootIDs identifies the separate base-game and DLC Larval Tear
// records. Both belong to the same in-game section despite their distinct UI
// labels and per-playthrough caps.
var larvalDeathrootIDs = map[uint32]struct{}{
	0x40001FF9: {}, // Larval Tear
	0x401EA3E1: {}, // Larval Tear (DLC)
}

// larvalDeathrootNames — Deathroot + Lost Ashes of War.
var larvalDeathrootNames = map[string]struct{}{
	"Deathroot":         {},
	"Lost Ashes of War": {},
}

// cookbooksKeywords — substrings that identify a Cookbooks-group item.
// "Cookbook" / "Whetblade" handle most, plus the curated set below.
var cookbooksByName = map[string]struct{}{
	"Crafting Kit":           {},
	"Spirit Calling Bell":    {},
	"Sewing Needle":          {},
	"Gold Sewing Needle":     {},
	"Tailoring Tools":        {},
	"Golden Tailoring Tools": {},
}

// dlcKeyNames — items that classify as DLC Keys (manual override; rule-based
// "dlc flag + name ends with Key" already catches most).
var dlcKeyNames = map[string]struct{}{
	"Cross-Marked Map": {}, // physical map key for DLC area access
}

func itemHasFlag(flags []string, want string) bool {
	for _, f := range flags {
		if f == want {
			return true
		}
	}
	return false
}

func classifyKeyItem(id uint32, item ItemData) string {
	name := item.Name

	// 1. Crystal Tears (curated ID list)
	if _, ok := crystalTearIDs[id]; ok {
		return SubcatKeyCrystalTears
	}
	// 2. Cookbooks (name-pattern + curated names)
	//    Prayerbooks / Codex / Principles / Principia are spell-unlock books
	//    (sub-group 9, not Cookbooks) — handled below.
	if strings.Contains(name, "Cookbook") || strings.Contains(name, "Whetblade") {
		return SubcatKeyCookbooks
	}
	if _, ok := cookbooksByName[name]; ok {
		return SubcatKeyCookbooks
	}
	// 9. Sorcery Scrolls + Incantation Scrolls (Prayerbooks, Codices, Principia,
	//    Principles, and items named "Scroll").
	if strings.Contains(name, "Prayerbook") || strings.Contains(name, "Codex") ||
		strings.Contains(name, "Principia") || strings.Contains(name, "Principles") ||
		strings.HasSuffix(name, " Scroll") {
		return SubcatKeySpellScrolls
	}
	// 3. Containers + Slot Upgrades
	if _, ok := containerNames[name]; ok {
		return SubcatKeyContainers
	}
	// 4. Larval / Deathroot / Lost AoW
	if _, ok := larvalDeathrootIDs[id]; ok {
		return SubcatKeyLarvalDeathroot
	}
	if _, ok := larvalDeathrootNames[name]; ok {
		return SubcatKeyLarvalDeathroot
	}
	// 5. (Spell scroll detection moved above — runs before DLC Key fallback)
	// 6. DLC Keys (curated + rule)
	if _, ok := dlcKeyNames[name]; ok {
		return SubcatKeyDLCKeys
	}
	if itemHasFlag(item.Flags, "dlc") && strings.HasSuffix(name, "Key") {
		return SubcatKeyDLCKeys
	}
	// 7. Catch-all: Inactive Great Runes + Keys + Medallions
	return SubcatKeyInactiveRunesKeys
}

func init() {
	for id, item := range KeyItems {
		if item.SubCategory != "" {
			continue
		}
		item.SubCategory = classifyKeyItem(id, item)
		KeyItems[id] = item
	}
}
