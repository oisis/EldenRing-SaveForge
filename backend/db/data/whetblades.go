package data

// WhetbladeData holds metadata for a whetblade unlock.
type WhetbladeData struct {
	Name string
}

// Whetblades maps event flag ID → whetblade metadata.
// Whetblades unlock weapon affinities at the smithing table.
// Source: er-save-manager/event_flags_db.py
var Whetblades = map[uint32]WhetbladeData{
	60130: {Name: "Whetstone Knife"},
	65610: {Name: "Iron Whetblade"},
	65640: {Name: "Red-Hot Whetblade"},
	65660: {Name: "Sanctified Whetblade"},
	65680: {Name: "Glintstone Whetblade"},
	65700: {Name: "Black Whetblade"},
}

// AoWMenuUnlockedFlag is the event flag that enables the "Ashes of War"
// menu at Sites of Grace. Set when the first whetblade is obtained.
const AoWMenuUnlockedFlag uint32 = 65800

// WhetbladeFlagToItemID maps whetblade event flag → inventory item ID.
var WhetbladeFlagToItemID = map[uint32]uint32{
	60130: 0x4000218E, // Whetstone Knife
	65610: 0x4000230A, // Iron Whetblade
	65640: 0x4000230B, // Red-Hot Whetblade
	65660: 0x4000230C, // Sanctified Whetblade
	65680: 0x4000230D, // Glintstone Whetblade
	65700: 0x4000230E, // Black Whetblade
}

// WhetbladeRelatedFlags maps whetblade event flag → affinity unlock flags.
// These flags are set/cleared together with the whetblade flag.
var WhetbladeRelatedFlags = map[uint32][]uint32{
	60130: {65600, 1042378601},    // Standard affinity + system flag (enables AoW menu at Grace)
	65610: {65620, 65630},         // Keen, Quality
	65640: {65650},                // Flame Art
	65660: {65670},                // Sacred
	65680: {65690},                // Frost
	65700: {65710, 65720},         // Poison, Blood
}

// WhetstoneKnifeFlag is the event flag for the base Whetstone Knife.
const WhetstoneKnifeFlag uint32 = 60130

// StormStompItemID is the inventory item ID for Ash of War: Storm Stomp,
// found together with the Whetstone Knife at Gatefront Ruins.
const StormStompItemID uint32 = 0x8000C418

// StormStompDupFlag is the AoW duplication flag for Storm Stomp.
const StormStompDupFlag uint32 = 65841

// whetbladeItemIDs is a set of all whetblade inventory item IDs for filtering.
var whetbladeItemIDs map[uint32]bool

// WhetbladeItemToFlagID is the reverse of WhetbladeFlagToItemID.
var WhetbladeItemToFlagID map[uint32]uint32

func init() {
	whetbladeItemIDs = make(map[uint32]bool, len(WhetbladeFlagToItemID))
	WhetbladeItemToFlagID = make(map[uint32]uint32, len(WhetbladeFlagToItemID))
	for flagID, itemID := range WhetbladeFlagToItemID {
		whetbladeItemIDs[itemID] = true
		WhetbladeItemToFlagID[itemID] = flagID
	}
}

// IsWhetbladeItemID returns true if the item ID is a whetblade Key Item.
func IsWhetbladeItemID(id uint32) bool {
	return whetbladeItemIDs[id]
}
