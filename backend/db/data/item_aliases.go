package data

// TechnicalItemAliases maps a technical duplicate GoodsParam item ID (the
// "alias") to the canonical item ID that already carries the item's metadata in
// the app DB. These are NOT separate items: every alias shares its canonical
// twin's name, caption and caps. They exist only because the regulation exposes
// two (or three) rows for one logical item, and a real save can legitimately
// carry either row.
//
// Purpose: resolver-safety. When GetItemDataFuzzy sees an alias ID it returns
// the canonical metadata so the scanner stops firing false-positive
// unknown_item_id. It is NOT a save rewrite - the raw ID/handle is preserved by
// the caller; only the metadata lookup follows the alias.
//
// Deliberately NOT a DB entry (unlike the "(Variant)" rows in key_items.go):
// because aliases never enter any category map or globalItemIndex, they can
// never appear in the item picker as separate addable items. That property is
// structural, not a filter.
//
// Scope is intentionally narrow — only exact-name Goods/key-item duplicates
// whose canonical twin is already in the DB. This is NOT for: armor
// normal/altered (distinct items), empty container vs crafted consumable
// (distinct mechanics), or weapon reinforcement/affinity variants (out of
// scope). Derived from a local regulation audit of exact-name Goods duplicate
// rows.
var TechnicalItemAliases = map[uint32]uint32{
	// Great Runes: low-row "held/inactive" copy -> canonical rune.
	0x400000BF: 0x40001FD4, // Godrick's Great Rune
	0x400000C0: 0x40001FD5, // Radahn's Great Rune
	0x400000C1: 0x40001FD6, // Morgott's Great Rune
	0x400000C2: 0x40001FD7, // Rykard's Great Rune
	0x400000C3: 0x40001FD8, // Mohg's Great Rune
	0x400000C4: 0x40001FD9, // Malenia's Great Rune

	// Key items: technical duplicate rows -> canonical row.
	0x40001FDA: 0x40001FDB, // Lord of Blood's Favor
	0x40002004: 0x40002310, // Unalloyed Gold Needle (quest-state row)
	0x4000230F: 0x40002310, // Unalloyed Gold Needle (quest-state row)

	// Scorpion Stew: technical twin rows (no FMG text) -> canonical FMG row.
	0x401E8932: 0x401E8930, // Scorpion Stew (technical variant)
	0x401E8931: 0x401E8933, // Gourmet Scorpion Stew (technical variant, reported handle 0xB01E8931)
}

// Flask of Crimson/Cerulean Tears expose two GoodsParam rows per upgrade level:
// an even ID (technical variant, absent from the DB) and the following odd ID
// (canonical, in tools.go). alias = canonical-1 for every level +0..+12.
func init() {
	const flaskLevels = 13 // +0 through +12
	for _, canonicalBase := range []uint32{
		0x400003E9, // Flask of Crimson Tears +0
		0x4000041B, // Flask of Cerulean Tears +0
	} {
		for lvl := uint32(0); lvl < flaskLevels; lvl++ {
			canonical := canonicalBase + lvl*2
			TechnicalItemAliases[canonical-1] = canonical
		}
	}
}
