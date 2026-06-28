package data

import "strings"

// CookbookData holds metadata for a cookbook item.
type CookbookData struct {
	Name     string
	Category string // series name for grouping
}

// Cookbooks maps event flag ID → cookbook metadata.
// Event flags sourced from ER-Save-Editor (Rust) cookbooks.rs.
var Cookbooks = map[uint32]CookbookData{
	// Nomadic Warrior's Cookbook (24)
	67000: {Name: "Nomadic Warrior's Cookbook [1]", Category: "Nomadic Warrior's Cookbook"},
	67110: {Name: "Nomadic Warrior's Cookbook [2]", Category: "Nomadic Warrior's Cookbook"},
	67010: {Name: "Nomadic Warrior's Cookbook [3]", Category: "Nomadic Warrior's Cookbook"},
	67800: {Name: "Nomadic Warrior's Cookbook [4]", Category: "Nomadic Warrior's Cookbook"},
	67830: {Name: "Nomadic Warrior's Cookbook [5]", Category: "Nomadic Warrior's Cookbook"},
	67020: {Name: "Nomadic Warrior's Cookbook [6]", Category: "Nomadic Warrior's Cookbook"},
	67050: {Name: "Nomadic Warrior's Cookbook [7]", Category: "Nomadic Warrior's Cookbook"},
	67880: {Name: "Nomadic Warrior's Cookbook [8]", Category: "Nomadic Warrior's Cookbook"},
	67430: {Name: "Nomadic Warrior's Cookbook [9]", Category: "Nomadic Warrior's Cookbook"},
	67030: {Name: "Nomadic Warrior's Cookbook [10]", Category: "Nomadic Warrior's Cookbook"},
	67220: {Name: "Nomadic Warrior's Cookbook [11]", Category: "Nomadic Warrior's Cookbook"},
	67060: {Name: "Nomadic Warrior's Cookbook [12]", Category: "Nomadic Warrior's Cookbook"},
	67080: {Name: "Nomadic Warrior's Cookbook [13]", Category: "Nomadic Warrior's Cookbook"},
	67870: {Name: "Nomadic Warrior's Cookbook [14]", Category: "Nomadic Warrior's Cookbook"},
	67900: {Name: "Nomadic Warrior's Cookbook [15]", Category: "Nomadic Warrior's Cookbook"},
	67290: {Name: "Nomadic Warrior's Cookbook [16]", Category: "Nomadic Warrior's Cookbook"},
	67100: {Name: "Nomadic Warrior's Cookbook [17]", Category: "Nomadic Warrior's Cookbook"},
	67270: {Name: "Nomadic Warrior's Cookbook [18]", Category: "Nomadic Warrior's Cookbook"},
	67070: {Name: "Nomadic Warrior's Cookbook [19]", Category: "Nomadic Warrior's Cookbook"},
	67230: {Name: "Nomadic Warrior's Cookbook [20]", Category: "Nomadic Warrior's Cookbook"},
	67120: {Name: "Nomadic Warrior's Cookbook [21]", Category: "Nomadic Warrior's Cookbook"},
	67890: {Name: "Nomadic Warrior's Cookbook [22]", Category: "Nomadic Warrior's Cookbook"},
	67090: {Name: "Nomadic Warrior's Cookbook [23]", Category: "Nomadic Warrior's Cookbook"},
	67910: {Name: "Nomadic Warrior's Cookbook [24]", Category: "Nomadic Warrior's Cookbook"},

	// Missionary's Cookbook (7)
	67610: {Name: "Missionary's Cookbook [1]", Category: "Missionary's Cookbook"},
	67600: {Name: "Missionary's Cookbook [2]", Category: "Missionary's Cookbook"},
	67650: {Name: "Missionary's Cookbook [3]", Category: "Missionary's Cookbook"},
	67640: {Name: "Missionary's Cookbook [4]", Category: "Missionary's Cookbook"},
	67630: {Name: "Missionary's Cookbook [5]", Category: "Missionary's Cookbook"},
	67130: {Name: "Missionary's Cookbook [6]", Category: "Missionary's Cookbook"},
	68230: {Name: "Missionary's Cookbook [7]", Category: "Missionary's Cookbook"},

	// Armorer's Cookbook (7)
	67200: {Name: "Armorer's Cookbook [1]", Category: "Armorer's Cookbook"},
	67210: {Name: "Armorer's Cookbook [2]", Category: "Armorer's Cookbook"},
	67280: {Name: "Armorer's Cookbook [3]", Category: "Armorer's Cookbook"},
	67260: {Name: "Armorer's Cookbook [4]", Category: "Armorer's Cookbook"},
	67310: {Name: "Armorer's Cookbook [5]", Category: "Armorer's Cookbook"},
	67300: {Name: "Armorer's Cookbook [6]", Category: "Armorer's Cookbook"},
	67250: {Name: "Armorer's Cookbook [7]", Category: "Armorer's Cookbook"},

	// Ancient Dragon Apostle's Cookbook (4)
	68000: {Name: "Ancient Dragon Apostle's Cookbook [1]", Category: "Ancient Dragon Apostle's Cookbook"},
	68010: {Name: "Ancient Dragon Apostle's Cookbook [2]", Category: "Ancient Dragon Apostle's Cookbook"},
	68030: {Name: "Ancient Dragon Apostle's Cookbook [3]", Category: "Ancient Dragon Apostle's Cookbook"},
	68020: {Name: "Ancient Dragon Apostle's Cookbook [4]", Category: "Ancient Dragon Apostle's Cookbook"},

	// Fevor's Cookbook (3)
	68200: {Name: "Fevor's Cookbook [1]", Category: "Fevor's Cookbook"},
	68220: {Name: "Fevor's Cookbook [2]", Category: "Fevor's Cookbook"},
	68210: {Name: "Fevor's Cookbook [3]", Category: "Fevor's Cookbook"},

	// Perfumer's Cookbook (4)
	67840: {Name: "Perfumer's Cookbook [1]", Category: "Perfumer's Cookbook"},
	67850: {Name: "Perfumer's Cookbook [2]", Category: "Perfumer's Cookbook"},
	67860: {Name: "Perfumer's Cookbook [3]", Category: "Perfumer's Cookbook"},
	67920: {Name: "Perfumer's Cookbook [4]", Category: "Perfumer's Cookbook"},

	// Glintstone Craftsman's Cookbook (8)
	67410: {Name: "Glintstone Craftsman's Cookbook [1]", Category: "Glintstone Craftsman's Cookbook"},
	67450: {Name: "Glintstone Craftsman's Cookbook [2]", Category: "Glintstone Craftsman's Cookbook"},
	67480: {Name: "Glintstone Craftsman's Cookbook [3]", Category: "Glintstone Craftsman's Cookbook"},
	67400: {Name: "Glintstone Craftsman's Cookbook [4]", Category: "Glintstone Craftsman's Cookbook"},
	67420: {Name: "Glintstone Craftsman's Cookbook [5]", Category: "Glintstone Craftsman's Cookbook"},
	67460: {Name: "Glintstone Craftsman's Cookbook [6]", Category: "Glintstone Craftsman's Cookbook"},
	67470: {Name: "Glintstone Craftsman's Cookbook [7]", Category: "Glintstone Craftsman's Cookbook"},
	67440: {Name: "Glintstone Craftsman's Cookbook [8]", Category: "Glintstone Craftsman's Cookbook"},

	// Frenzied's Cookbook (2)
	68400: {Name: "Frenzied's Cookbook [1]", Category: "Frenzied's Cookbook"},
	68410: {Name: "Frenzied's Cookbook [2]", Category: "Frenzied's Cookbook"},

	// === SHADOW OF THE ERDTREE DLC ===

	// Forager Brood Cookbook (7)
	68520: {Name: "Forager Brood Cookbook [1]", Category: "Forager Brood Cookbook"},
	68530: {Name: "Forager Brood Cookbook [2]", Category: "Forager Brood Cookbook"},
	68540: {Name: "Forager Brood Cookbook [3]", Category: "Forager Brood Cookbook"},
	68550: {Name: "Forager Brood Cookbook [4]", Category: "Forager Brood Cookbook"},
	68560: {Name: "Forager Brood Cookbook [5]", Category: "Forager Brood Cookbook"},
	68510: {Name: "Forager Brood Cookbook [6]", Category: "Forager Brood Cookbook"},
	68830: {Name: "Forager Brood Cookbook [7]", Category: "Forager Brood Cookbook"},

	// Greater Potentate's Cookbook (14)
	68590: {Name: "Greater Potentate's Cookbook [1]", Category: "Greater Potentate's Cookbook"},
	68730: {Name: "Greater Potentate's Cookbook [2]", Category: "Greater Potentate's Cookbook"},
	68690: {Name: "Greater Potentate's Cookbook [3]", Category: "Greater Potentate's Cookbook"},
	68600: {Name: "Greater Potentate's Cookbook [4]", Category: "Greater Potentate's Cookbook"},
	68610: {Name: "Greater Potentate's Cookbook [5]", Category: "Greater Potentate's Cookbook"},
	68720: {Name: "Greater Potentate's Cookbook [6]", Category: "Greater Potentate's Cookbook"},
	68630: {Name: "Greater Potentate's Cookbook [7]", Category: "Greater Potentate's Cookbook"},
	68680: {Name: "Greater Potentate's Cookbook [8]", Category: "Greater Potentate's Cookbook"},
	68640: {Name: "Greater Potentate's Cookbook [9]", Category: "Greater Potentate's Cookbook"},
	68650: {Name: "Greater Potentate's Cookbook [10]", Category: "Greater Potentate's Cookbook"},
	68660: {Name: "Greater Potentate's Cookbook [11]", Category: "Greater Potentate's Cookbook"},
	68620: {Name: "Greater Potentate's Cookbook [12]", Category: "Greater Potentate's Cookbook"},
	68700: {Name: "Greater Potentate's Cookbook [13]", Category: "Greater Potentate's Cookbook"},
	68710: {Name: "Greater Potentate's Cookbook [14]", Category: "Greater Potentate's Cookbook"},

	// Mad Craftsman's Cookbook (3)
	68750: {Name: "Mad Craftsman's Cookbook [1]", Category: "Mad Craftsman's Cookbook"},
	68670: {Name: "Mad Craftsman's Cookbook [2]", Category: "Mad Craftsman's Cookbook"},
	68880: {Name: "Mad Craftsman's Cookbook [3]", Category: "Mad Craftsman's Cookbook"},

	// Ancient Dragon Knight's Cookbook (2)
	68740: {Name: "Ancient Dragon Knight's Cookbook [1]", Category: "Ancient Dragon Knight's Cookbook"},
	68780: {Name: "Ancient Dragon Knight's Cookbook [2]", Category: "Ancient Dragon Knight's Cookbook"},

	// St. Trina Disciple's Cookbook (3)
	68760: {Name: "St. Trina Disciple's Cookbook [1]", Category: "St. Trina Disciple's Cookbook"},
	68950: {Name: "St. Trina Disciple's Cookbook [2]", Category: "St. Trina Disciple's Cookbook"},
	68840: {Name: "St. Trina Disciple's Cookbook [3]", Category: "St. Trina Disciple's Cookbook"},

	// Fire Knight's Cookbook (2)
	68770: {Name: "Fire Knight's Cookbook [1]", Category: "Fire Knight's Cookbook"},
	68900: {Name: "Fire Knight's Cookbook [2]", Category: "Fire Knight's Cookbook"},

	// Igon's Cookbook (2)
	68810: {Name: "Igon's Cookbook [1]", Category: "Igon's Cookbook"},
	68570: {Name: "Igon's Cookbook [2]", Category: "Igon's Cookbook"},

	// Finger-Weaver's Cookbook (2)
	68920: {Name: "Finger-Weaver's Cookbook [1]", Category: "Finger-Weaver's Cookbook"},
	68580: {Name: "Finger-Weaver's Cookbook [2]", Category: "Finger-Weaver's Cookbook"},

	// Battlefield Priest's Cookbook (4)
	68800: {Name: "Battlefield Priest's Cookbook [1]", Category: "Battlefield Priest's Cookbook"},
	68820: {Name: "Battlefield Priest's Cookbook [2]", Category: "Battlefield Priest's Cookbook"},
	68890: {Name: "Battlefield Priest's Cookbook [3]", Category: "Battlefield Priest's Cookbook"},
	68930: {Name: "Battlefield Priest's Cookbook [4]", Category: "Battlefield Priest's Cookbook"},

	// Grave Keeper's Cookbook (2)
	68940: {Name: "Grave Keeper's Cookbook [1]", Category: "Grave Keeper's Cookbook"},
	68850: {Name: "Grave Keeper's Cookbook [2]", Category: "Grave Keeper's Cookbook"},

	// Antiquity Scholar's Cookbook (2)
	68910: {Name: "Antiquity Scholar's Cookbook [1]", Category: "Antiquity Scholar's Cookbook"},
	68860: {Name: "Antiquity Scholar's Cookbook [2]", Category: "Antiquity Scholar's Cookbook"},

	// Loyal Knight's Cookbook (1)
	68790: {Name: "Loyal Knight's Cookbook", Category: "Loyal Knight's Cookbook"},

	// Tibia's Cookbook (1)
	68870: {Name: "Tibia's Cookbook", Category: "Tibia's Cookbook"},
}

// CookbookFlagToItemID maps cookbook event flag ID → inventory item ID (Key Items).
// Built by matching cookbook names between Cookbooks and KeyItems.
var CookbookFlagToItemID = map[uint32]uint32{
	// Nomadic Warrior's Cookbook [1-24]
	67000: 0x40002454, // [1]
	67110: 0x4000245F, // [2]
	67010: 0x40002455, // [3]
	67800: 0x400024A4, // [4]
	67830: 0x400024A7, // [5]
	67020: 0x40002456, // [6]
	67050: 0x40002459, // [7]
	67880: 0x400024AC, // [8]
	67430: 0x4000247F, // [9]
	67030: 0x40002457, // [10]
	67220: 0x4000246A, // [11]
	67060: 0x4000245A, // [12]
	67080: 0x4000245C, // [13]
	67870: 0x400024AB, // [14]
	67900: 0x400024AE, // [15]
	67290: 0x40002471, // [16]
	67100: 0x4000245E, // [17]
	67270: 0x4000246F, // [18]
	67070: 0x4000245B, // [19]
	67230: 0x4000246B, // [20]
	67120: 0x40002460, // [21]
	67890: 0x400024AD, // [22]
	67090: 0x4000245D, // [23]
	67910: 0x400024AF, // [24]

	// Missionary's Cookbook [1-7]
	67610: 0x40002491, // [1]
	67600: 0x40002490, // [2]
	67650: 0x40002495, // [3]
	67640: 0x40002494, // [4]
	67630: 0x40002493, // [5]
	67130: 0x40002461, // [6]
	68230: 0x400024CF, // [7]

	// Armorer's Cookbook [1-7]
	67200: 0x40002468, // [1]
	67210: 0x40002469, // [2]
	67280: 0x40002470, // [3]
	67260: 0x4000246E, // [4]
	67310: 0x40002473, // [5]
	67300: 0x40002472, // [6]
	67250: 0x4000246D, // [7]

	// Ancient Dragon Apostle's Cookbook [1-4]
	68000: 0x400024B8, // [1]
	68010: 0x400024B9, // [2]
	68030: 0x400024BB, // [3]
	68020: 0x400024BA, // [4]

	// Fevor's Cookbook [1-3]
	68200: 0x400024CC, // [1]
	68220: 0x400024CE, // [2]
	68210: 0x400024CD, // [3]

	// Perfumer's Cookbook [1-4]
	67840: 0x400024A8, // [1]
	67850: 0x400024A9, // [2]
	67860: 0x400024AA, // [3]
	67920: 0x400024B0, // [4]

	// Glintstone Craftsman's Cookbook [1-8]
	67410: 0x4000247D, // [1]
	67450: 0x40002481, // [2]
	67480: 0x40002484, // [3]
	67400: 0x4000247C, // [4]
	67420: 0x4000247E, // [5]
	67460: 0x40002482, // [6]
	67470: 0x40002483, // [7]
	67440: 0x40002480, // [8]

	// Frenzied's Cookbook [1-2]
	68400: 0x400024E0, // [1]
	68410: 0x400024E1, // [2]

	// === DLC — Shadow of the Erdtree ===

	// Forager Brood Cookbook [1-7]
	68520: 0x401EA8D6, // [1]
	68530: 0x401EA8D7, // [2]
	68540: 0x401EA8D8, // [3]
	68550: 0x401EA8D9, // [4]
	68560: 0x401EA8DA, // [5]
	68510: 0x401EA8D5, // [6]
	68830: 0x401EA8F5, // [7]

	// Greater Potentate's Cookbook [1-14]
	68590: 0x401EA8DD, // [1]
	68730: 0x401EA8EB, // [2]
	68690: 0x401EA8E7, // [3]
	68600: 0x401EA8DE, // [4]
	68610: 0x401EA8DF, // [5]
	68720: 0x401EA8EA, // [6]
	68630: 0x401EA8E1, // [7]
	68680: 0x401EA8E6, // [8]
	68640: 0x401EA8E2, // [9]
	68650: 0x401EA8E3, // [10]
	68660: 0x401EA8E4, // [11]
	68620: 0x401EA8E0, // [12]
	68700: 0x401EA8E8, // [13]
	68710: 0x401EA8E9, // [14]

	// Mad Craftsman's Cookbook [1-3]
	68750: 0x401EA8ED, // [1]
	68670: 0x401EA8E5, // [2]
	68880: 0x401EA8FA, // [3]

	// Ancient Dragon Knight's Cookbook [1-2]
	68740: 0x401EA8EC, // [1]
	68780: 0x401EA8F0, // [2]

	// St. Trina Disciple's Cookbook [1-3]
	68760: 0x401EA8EE, // [1]
	68950: 0x401EA901, // [2]
	68840: 0x401EA8F6, // [3]

	// Fire Knight's Cookbook [1-2]
	68770: 0x401EA8EF, // [1]
	68900: 0x401EA8FC, // [2]

	// Igon's Cookbook [1-2]
	68810: 0x401EA8F3, // [1]
	68570: 0x401EA8DB, // [2]

	// Finger-Weaver's Cookbook [1-2]
	68920: 0x401EA8FE, // [1]
	68580: 0x401EA8DC, // [2]

	// Battlefield Priest's Cookbook [1-4]
	68800: 0x401EA8F2, // [1]
	68820: 0x401EA8F4, // [2]
	68890: 0x401EA8FB, // [3]
	68930: 0x401EA8FF, // [4]

	// Grave Keeper's Cookbook [1-2]
	68940: 0x401EA900, // [1]
	68850: 0x401EA8F7, // [2]

	// Antiquity Scholar's Cookbook [1-2]
	68910: 0x401EA8FD, // [1]
	68860: 0x401EA8F8, // [2]

	// Loyal Knight's Cookbook
	68790: 0x401EA8F1,

	// Tibia's Cookbook
	68870: 0x401EA8F9,
}

// cookbookItemIDs is a set of all cookbook inventory item IDs for filtering.
var cookbookItemIDs map[uint32]bool

// CookbookItemToFlagID is the reverse of CookbookFlagToItemID.
var CookbookItemToFlagID map[uint32]uint32

func init() {
	cookbookItemIDs = make(map[uint32]bool, len(CookbookFlagToItemID))
	CookbookItemToFlagID = make(map[uint32]uint32, len(CookbookFlagToItemID))
	for flagID, itemID := range CookbookFlagToItemID {
		cookbookItemIDs[itemID] = true
		CookbookItemToFlagID[itemID] = flagID
	}
}

// IsCookbookItemID returns true if the item ID is a cookbook Key Item.
// Checks both the mapped IDs and any Key Item with "Cookbook" in the name.
func IsCookbookItemID(id uint32) bool {
	if cookbookItemIDs[id] {
		return true
	}
	if item, ok := KeyItems[id]; ok {
		return strings.Contains(item.Name, "Cookbook")
	}
	return false
}
