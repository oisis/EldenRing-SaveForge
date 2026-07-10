package db

import "github.com/oisis/EldenRing-SaveForge/backend/db/data"

const (
	ItemFlaskWondrousPhysickFilled uint32 = 0x400000FA
	ItemFlaskWondrousPhysickEmpty  uint32 = 0x400000FB

	// +0 family base rows for the two upgradeable Tears flasks. Each family
	// spans 13 odd level rows (base + level*2, level 0..12); the DB picker only
	// exposes the +0 row, so every upgraded variant must count under it.
	ItemFlaskCrimsonBase  uint32 = 0x400003E9
	ItemFlaskCeruleanBase uint32 = 0x4000041B

	flaskMaxLevel uint32 = 12
)

// IsWondrousPhysick reports whether id is either raw save-state variant of the
// single logical Flask of Wondrous Physick item.
func IsWondrousPhysick(id uint32) bool {
	id = HandleToItemID(id)
	return id == ItemFlaskWondrousPhysickFilled || id == ItemFlaskWondrousPhysickEmpty
}

// WondrousPhysickDisplayID returns the database item ID used for metadata.
// It does not imply the raw save item should be rewritten.
func WondrousPhysickDisplayID(id uint32) uint32 {
	if IsWondrousPhysick(id) {
		return ItemFlaskWondrousPhysickEmpty
	}
	return id
}

// FlaskFamilyBaseID maps any Crimson/Cerulean Flask of Tears variant to its +0
// family base (0x400003E9 / 0x4000041B). It accepts a raw item ID, a goods
// handle (0xB0…), or a technical even-ID "empty flask" variant already listed
// in data.TechnicalItemAliases — all of them resolve to the family the DB
// picker exposes, so upgraded flasks a save carries group under the +0 row for
// ownership/counting. This is a metadata-only grouping key; it never rewrites
// the raw inventory ID. Wondrous Physick is intentionally excluded — it is a
// distinct single-item flask handled by IsWondrousPhysick.
func FlaskFamilyBaseID(id uint32) (uint32, bool) {
	itemID := HandleToItemID(id)
	// Even technical variant rows resolve to their odd canonical level first
	// (e.g. 0x40000400 -> 0x40000401) so they land in the right family.
	if canonical, ok := data.TechnicalItemAliases[itemID]; ok {
		itemID = canonical
	}
	for _, base := range [...]uint32{ItemFlaskCrimsonBase, ItemFlaskCeruleanBase} {
		if itemID >= base && itemID <= base+flaskMaxLevel*2 && (itemID-base)%2 == 0 {
			return base, true
		}
	}
	return 0, false
}
