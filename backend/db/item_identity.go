package db

const (
	ItemFlaskWondrousPhysickFilled uint32 = 0x400000FA
	ItemFlaskWondrousPhysickEmpty  uint32 = 0x400000FB
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
