package data

// ArrowsAndBolts — projectile ammunition for ranged weapons. 1:1 with in-game
// "Arrows / Bolts" tab.
//
// Sub-groups (in-game order):
//   1. Arrows — short-bow / long-bow / light-bow ammunition
//   2. Greatarrows — greatbow ammunition (incl. Radahn's Spear)
//   3. Bolts — crossbow ammunition
//   4. Greatbolts — ballista ammunition (incl. Igon's Harpoon)
var ArrowsAndBolts = map[uint32]ItemData{
	// ─── Arrows ─────────────────────────────────────────────────────────
	0x02FAF080: {Name: "Arrow", Category: "arrows_and_bolts", SubCategory: SubcatArrowsArrows, MaxInventory: 99, MaxStorage: 600, MaxUpgrade: 0, IconPath: "items/arrows_and_bolts/arrow.png", Flags: []string{"stackable"}},
	0x02FB1790: {Name: "Fire Arrow", Category: "arrows_and_bolts", SubCategory: SubcatArrowsArrows, MaxInventory: 99, MaxStorage: 600, MaxUpgrade: 0, IconPath: "items/arrows_and_bolts/fire_arrow.png", Flags: []string{"stackable"}},
	0x02FB3EA0: {Name: "Serpent Arrow", Category: "arrows_and_bolts", SubCategory: SubcatArrowsArrows, MaxInventory: 99, MaxStorage: 600, MaxUpgrade: 0, IconPath: "items/arrows_and_bolts/serpent_arrow.png", Flags: []string{"stackable"}},
	0x02FB65B0: {Name: "Bone Arrow (Fletched)", Category: "arrows_and_bolts", SubCategory: SubcatArrowsArrows, MaxInventory: 99, MaxStorage: 600, MaxUpgrade: 0, IconPath: "items/arrows_and_bolts/bone_arrow_fletched.png", Flags: []string{"stackable"}},
	0x02FB8CC0: {Name: "St. Trina's Arrow", Category: "arrows_and_bolts", SubCategory: SubcatArrowsArrows, MaxInventory: 99, MaxStorage: 600, MaxUpgrade: 0, IconPath: "items/arrows_and_bolts/st_trinas_arrow.png", Flags: []string{"stackable"}},
	0x02FBDAE0: {Name: "Shattershard Arrow (Fletched)", Category: "arrows_and_bolts", SubCategory: SubcatArrowsArrows, MaxInventory: 99, MaxStorage: 600, MaxUpgrade: 0, IconPath: "items/arrows_and_bolts/shattershard_arrow_fletched.png", Flags: []string{"stackable"}},
	0x02FC2900: {Name: "Rainbow Stone Arrow (Fletched)", Category: "arrows_and_bolts", SubCategory: SubcatArrowsArrows, MaxInventory: 99, MaxStorage: 600, MaxUpgrade: 0, IconPath: "items/arrows_and_bolts/rainbow_stone_arrow_fletched.png", Flags: []string{"stackable"}},
	0x02FC5010: {Name: "Golden Arrow", Category: "arrows_and_bolts", SubCategory: SubcatArrowsArrows, MaxInventory: 99, MaxStorage: 600, MaxUpgrade: 0, IconPath: "items/arrows_and_bolts/golden_arrow.png", Flags: []string{"stackable"}},
	0x02FC7720: {Name: "Dwelling Arrow", Category: "arrows_and_bolts", SubCategory: SubcatArrowsArrows, MaxInventory: 99, MaxStorage: 600, MaxUpgrade: 0, IconPath: "items/arrows_and_bolts/dwelling_arrow.png", Flags: []string{"stackable"}},
	0x02FC9E30: {Name: "Bone Arrow", Category: "arrows_and_bolts", SubCategory: SubcatArrowsArrows, MaxInventory: 99, MaxStorage: 600, MaxUpgrade: 0, IconPath: "items/arrows_and_bolts/bone_arrow.png", Flags: []string{"stackable"}},
	0x02FCEC50: {Name: "Firebone Arrow (Fletched)", Category: "arrows_and_bolts", SubCategory: SubcatArrowsArrows, MaxInventory: 99, MaxStorage: 600, MaxUpgrade: 0, IconPath: "items/arrows_and_bolts/firebone_arrow_fletched.png", Flags: []string{"stackable"}},
	0x02FD1360: {Name: "Firebone Arrow", Category: "arrows_and_bolts", SubCategory: SubcatArrowsArrows, MaxInventory: 99, MaxStorage: 600, MaxUpgrade: 0, IconPath: "items/arrows_and_bolts/firebone_arrow.png", Flags: []string{"stackable"}},
	0x02FD3A70: {Name: "Poisonbone Arrow (Fletched)", Category: "arrows_and_bolts", SubCategory: SubcatArrowsArrows, MaxInventory: 99, MaxStorage: 600, MaxUpgrade: 0, IconPath: "items/arrows_and_bolts/poisonbone_arrow_fletched.png", Flags: []string{"stackable"}},
	0x02FD6180: {Name: "Poisonbone Arrow", Category: "arrows_and_bolts", SubCategory: SubcatArrowsArrows, MaxInventory: 99, MaxStorage: 600, MaxUpgrade: 0, IconPath: "items/arrows_and_bolts/poisonbone_arrow.png", Flags: []string{"stackable"}},
	0x02FD8890: {Name: "Sleepbone Arrow (Fletched)", Category: "arrows_and_bolts", SubCategory: SubcatArrowsArrows, MaxInventory: 99, MaxStorage: 600, MaxUpgrade: 0, IconPath: "items/arrows_and_bolts/sleepbone_arrow_fletched.png", Flags: []string{"stackable"}},
	0x02FDAFA0: {Name: "Sleepbone Arrow", Category: "arrows_and_bolts", SubCategory: SubcatArrowsArrows, MaxInventory: 99, MaxStorage: 600, MaxUpgrade: 0, IconPath: "items/arrows_and_bolts/sleepbone_arrow.png", Flags: []string{"stackable"}},
	0x02FDD6B0: {Name: "Stormwing Bone Arrow", Category: "arrows_and_bolts", SubCategory: SubcatArrowsArrows, MaxInventory: 99, MaxStorage: 600, MaxUpgrade: 0, IconPath: "items/arrows_and_bolts/stormwing_bone_arrow.png", Flags: []string{"stackable"}},
	0x02FDFDC0: {Name: "Lightningbone Arrow (Fletched)", Category: "arrows_and_bolts", SubCategory: SubcatArrowsArrows, MaxInventory: 99, MaxStorage: 600, MaxUpgrade: 0, IconPath: "items/arrows_and_bolts/lightningbone_arrow_fletched.png", Flags: []string{"stackable"}},
	0x02FE24D0: {Name: "Lightningbone Arrow", Category: "arrows_and_bolts", SubCategory: SubcatArrowsArrows, MaxInventory: 99, MaxStorage: 600, MaxUpgrade: 0, IconPath: "items/arrows_and_bolts/lightningbone_arrow.png", Flags: []string{"stackable"}},
	0x02FE4BE0: {Name: "Rainbow Stone Arrow", Category: "arrows_and_bolts", SubCategory: SubcatArrowsArrows, MaxInventory: 99, MaxStorage: 600, MaxUpgrade: 0, IconPath: "items/arrows_and_bolts/rainbow_stone_arrow.png", Flags: []string{"stackable"}},
	0x02FE72F0: {Name: "Shattershard Arrow", Category: "arrows_and_bolts", SubCategory: SubcatArrowsArrows, MaxInventory: 99, MaxStorage: 600, MaxUpgrade: 0, IconPath: "items/arrows_and_bolts/shattershard_arrow.png", Flags: []string{"stackable"}},
	0x02FE9A00: {Name: "Spiritflame Arrow", Category: "arrows_and_bolts", SubCategory: SubcatArrowsArrows, MaxInventory: 99, MaxStorage: 600, MaxUpgrade: 0, IconPath: "items/arrows_and_bolts/spiritflame_arrow.png", Flags: []string{"stackable"}},
	0x02FEE820: {Name: "Magicbone Arrow (Fletched)", Category: "arrows_and_bolts", SubCategory: SubcatArrowsArrows, MaxInventory: 99, MaxStorage: 600, MaxUpgrade: 0, IconPath: "items/arrows_and_bolts/magicbone_arrow_fletched.png", Flags: []string{"stackable"}},
	0x02FF0F30: {Name: "Magicbone Arrow", Category: "arrows_and_bolts", SubCategory: SubcatArrowsArrows, MaxInventory: 99, MaxStorage: 600, MaxUpgrade: 0, IconPath: "items/arrows_and_bolts/magicbone_arrow.png", Flags: []string{"stackable"}},
	0x02FF3640: {Name: "Haligbone Arrow (Fletched)", Category: "arrows_and_bolts", SubCategory: SubcatArrowsArrows, MaxInventory: 99, MaxStorage: 600, MaxUpgrade: 0, IconPath: "items/arrows_and_bolts/haligbone_arrow_fletched.png", Flags: []string{"stackable"}},
	0x02FF5D50: {Name: "Haligbone Arrow", Category: "arrows_and_bolts", SubCategory: SubcatArrowsArrows, MaxInventory: 99, MaxStorage: 600, MaxUpgrade: 0, IconPath: "items/arrows_and_bolts/haligbone_arrow.png", Flags: []string{"stackable"}},
	0x02FF8460: {Name: "Bloodbone Arrow (Fletched)", Category: "arrows_and_bolts", SubCategory: SubcatArrowsArrows, MaxInventory: 99, MaxStorage: 600, MaxUpgrade: 0, IconPath: "items/arrows_and_bolts/bloodbone_arrow_fletched.png", Flags: []string{"stackable"}},
	0x02FFAB70: {Name: "Bloodbone Arrow", Category: "arrows_and_bolts", SubCategory: SubcatArrowsArrows, MaxInventory: 99, MaxStorage: 600, MaxUpgrade: 0, IconPath: "items/arrows_and_bolts/bloodbone_arrow.png", Flags: []string{"stackable"}},
	0x02FFD280: {Name: "Coldbone Arrow (Fletched)", Category: "arrows_and_bolts", SubCategory: SubcatArrowsArrows, MaxInventory: 99, MaxStorage: 600, MaxUpgrade: 0, IconPath: "items/arrows_and_bolts/coldbone_arrow_fletched.png", Flags: []string{"stackable"}},
	0x02FFF990: {Name: "Coldbone Arrow", Category: "arrows_and_bolts", SubCategory: SubcatArrowsArrows, MaxInventory: 99, MaxStorage: 600, MaxUpgrade: 0, IconPath: "items/arrows_and_bolts/coldbone_arrow.png", Flags: []string{"stackable"}},
	0x030020A0: {Name: "Rotbone Arrow (Fletched)", Category: "arrows_and_bolts", SubCategory: SubcatArrowsArrows, MaxInventory: 99, MaxStorage: 600, MaxUpgrade: 0, IconPath: "items/arrows_and_bolts/rotbone_arrow_fletched.png", Flags: []string{"stackable"}},
	0x030047B0: {Name: "Rotbone Arrow", Category: "arrows_and_bolts", SubCategory: SubcatArrowsArrows, MaxInventory: 99, MaxStorage: 600, MaxUpgrade: 0, IconPath: "items/arrows_and_bolts/rotbone_arrow.png", Flags: []string{"stackable"}},
	0x03032DE0: {Name: "Piquebone Arrow (Fletched)", Category: "arrows_and_bolts", SubCategory: SubcatArrowsArrows, MaxInventory: 99, MaxStorage: 600, MaxUpgrade: 0, IconPath: "items/arrows_and_bolts/piquebone_arrow_fletched.png", Flags: []string{"dlc", "stackable"}},
	0x030354F0: {Name: "Piquebone Arrow", Category: "arrows_and_bolts", SubCategory: SubcatArrowsArrows, MaxInventory: 99, MaxStorage: 600, MaxUpgrade: 0, IconPath: "items/arrows_and_bolts/piquebone_arrow.png", Flags: []string{"dlc", "stackable"}},

	// ─── Greatarrows ────────────────────────────────────────────────────
	0x030A32C0: {Name: "Great Arrow", Category: "arrows_and_bolts", SubCategory: SubcatArrowsGreatarrows, MaxInventory: 30, MaxStorage: 600, MaxUpgrade: 0, IconPath: "items/arrows_and_bolts/great_arrow.png", Flags: []string{"stackable"}},
	0x030A59D0: {Name: "Golem's Great Arrow", Category: "arrows_and_bolts", SubCategory: SubcatArrowsGreatarrows, MaxInventory: 30, MaxStorage: 600, MaxUpgrade: 0, IconPath: "items/arrows_and_bolts/golems_great_arrow.png", Flags: []string{"stackable"}},
	0x030A80E0: {Name: "Golden Great Arrow", Category: "arrows_and_bolts", SubCategory: SubcatArrowsGreatarrows, MaxInventory: 30, MaxStorage: 600, MaxUpgrade: 0, IconPath: "items/arrows_and_bolts/golden_great_arrow.png", Flags: []string{"stackable"}},
	0x030AA7F0: {Name: "Golem's Magic Arrow", Category: "arrows_and_bolts", SubCategory: SubcatArrowsGreatarrows, MaxInventory: 30, MaxStorage: 600, MaxUpgrade: 0, IconPath: "items/arrows_and_bolts/golems_magic_arrow.png", Flags: []string{"stackable"}},
	0x030ACF00: {Name: "Radahn's Spear", Category: "arrows_and_bolts", SubCategory: SubcatArrowsGreatarrows, MaxInventory: 30, MaxStorage: 600, MaxUpgrade: 0, IconPath: "items/arrows_and_bolts/radahns_spear.png", Flags: []string{"stackable"}},
	0x030AF610: {Name: "Bone Great Arrow (Fletched)", Category: "arrows_and_bolts", SubCategory: SubcatArrowsGreatarrows, MaxInventory: 30, MaxStorage: 600, MaxUpgrade: 0, IconPath: "items/arrows_and_bolts/bone_great_arrow_fletched.png", Flags: []string{"stackable"}},
	0x030B1D20: {Name: "Bone Great Arrow", Category: "arrows_and_bolts", SubCategory: SubcatArrowsGreatarrows, MaxInventory: 30, MaxStorage: 600, MaxUpgrade: 0, IconPath: "items/arrows_and_bolts/bone_great_arrow.png", Flags: []string{"stackable"}},

	// ─── Bolts ──────────────────────────────────────────────────────────
	0x0311D3E0: {Name: "Igon's Harpoon", Category: "arrows_and_bolts", SubCategory: SubcatArrowsGreatbolts, MaxInventory: 30, MaxStorage: 600, MaxUpgrade: 0, IconPath: "items/arrows_and_bolts/igons_harpoon.png", Flags: []string{"dlc", "stackable"}},
	0x03197500: {Name: "Bolt", Category: "arrows_and_bolts", SubCategory: SubcatArrowsBolts, MaxInventory: 99, MaxStorage: 600, MaxUpgrade: 0, IconPath: "items/arrows_and_bolts/bolt.png", Flags: []string{"stackable"}},
	0x03199C10: {Name: "Lightning Bolt", Category: "arrows_and_bolts", SubCategory: SubcatArrowsBolts, MaxInventory: 99, MaxStorage: 600, MaxUpgrade: 0, IconPath: "items/arrows_and_bolts/lightning_bolt.png", Flags: []string{"stackable"}},
	0x0319C320: {Name: "Perfumer's Bolt", Category: "arrows_and_bolts", SubCategory: SubcatArrowsBolts, MaxInventory: 99, MaxStorage: 600, MaxUpgrade: 0, IconPath: "items/arrows_and_bolts/perfumers_bolt.png", Flags: []string{"stackable"}},
	0x0319EA30: {Name: "Black-Key Bolt", Category: "arrows_and_bolts", SubCategory: SubcatArrowsBolts, MaxInventory: 99, MaxStorage: 600, MaxUpgrade: 0, IconPath: "items/arrows_and_bolts/black_key_bolt.png", Flags: []string{"stackable"}},
	0x031A1140: {Name: "Burred Bolt", Category: "arrows_and_bolts", SubCategory: SubcatArrowsBolts, MaxInventory: 99, MaxStorage: 600, MaxUpgrade: 0, IconPath: "items/arrows_and_bolts/burred_bolt.png", Flags: []string{"stackable"}},
	0x031A3850: {Name: "Meteor Bolt", Category: "arrows_and_bolts", SubCategory: SubcatArrowsBolts, MaxInventory: 99, MaxStorage: 600, MaxUpgrade: 0, IconPath: "items/arrows_and_bolts/meteor_bolt.png", Flags: []string{"stackable"}},
	0x031A5F60: {Name: "Explosive Bolt", Category: "arrows_and_bolts", SubCategory: SubcatArrowsBolts, MaxInventory: 99, MaxStorage: 600, MaxUpgrade: 0, IconPath: "items/arrows_and_bolts/explosive_bolt.png", Flags: []string{"stackable"}},
	0x031A8670: {Name: "Golden Bolt", Category: "arrows_and_bolts", SubCategory: SubcatArrowsBolts, MaxInventory: 99, MaxStorage: 600, MaxUpgrade: 0, IconPath: "items/arrows_and_bolts/golden_bolt.png", Flags: []string{"stackable"}},
	0x031AAD80: {Name: "Lordsworn's Bolt", Category: "arrows_and_bolts", SubCategory: SubcatArrowsBolts, MaxInventory: 99, MaxStorage: 600, MaxUpgrade: 0, IconPath: "items/arrows_and_bolts/lordsworns_bolt.png", Flags: []string{"stackable"}},
	0x031AD490: {Name: "Bone Bolt", Category: "arrows_and_bolts", SubCategory: SubcatArrowsBolts, MaxInventory: 99, MaxStorage: 600, MaxUpgrade: 0, IconPath: "items/arrows_and_bolts/bone_bolt.png", Flags: []string{"stackable"}},
	0x031AFBA0: {Name: "Firebone Bolt", Category: "arrows_and_bolts", SubCategory: SubcatArrowsBolts, MaxInventory: 99, MaxStorage: 600, MaxUpgrade: 0, IconPath: "items/arrows_and_bolts/firebone_bolt.png", Flags: []string{"stackable"}},
	0x031B22B0: {Name: "Lightningbone Bolt", Category: "arrows_and_bolts", SubCategory: SubcatArrowsBolts, MaxInventory: 99, MaxStorage: 600, MaxUpgrade: 0, IconPath: "items/arrows_and_bolts/lightningbone_bolt.png", Flags: []string{"stackable"}},
	0x031B49C0: {Name: "Magicbone Bolt", Category: "arrows_and_bolts", SubCategory: SubcatArrowsBolts, MaxInventory: 99, MaxStorage: 600, MaxUpgrade: 0, IconPath: "items/arrows_and_bolts/magicbone_bolt.png", Flags: []string{"stackable"}},
	0x031B70D0: {Name: "Haligbone Bolt", Category: "arrows_and_bolts", SubCategory: SubcatArrowsBolts, MaxInventory: 99, MaxStorage: 600, MaxUpgrade: 0, IconPath: "items/arrows_and_bolts/haligbone_bolt.png", Flags: []string{"stackable"}},
	0x031B97E0: {Name: "Poisonbone Bolt", Category: "arrows_and_bolts", SubCategory: SubcatArrowsBolts, MaxInventory: 99, MaxStorage: 600, MaxUpgrade: 0, IconPath: "items/arrows_and_bolts/poisonbone_bolt.png", Flags: []string{"stackable"}},
	0x031BBEF0: {Name: "Bloodbone Bolt", Category: "arrows_and_bolts", SubCategory: SubcatArrowsBolts, MaxInventory: 99, MaxStorage: 600, MaxUpgrade: 0, IconPath: "items/arrows_and_bolts/bloodbone_bolt.png", Flags: []string{"stackable"}},
	0x031BE600: {Name: "Coldbone Bolt", Category: "arrows_and_bolts", SubCategory: SubcatArrowsBolts, MaxInventory: 99, MaxStorage: 600, MaxUpgrade: 0, IconPath: "items/arrows_and_bolts/coldbone_bolt.png", Flags: []string{"stackable"}},
	0x031C0D10: {Name: "Rotbone Bolt", Category: "arrows_and_bolts", SubCategory: SubcatArrowsBolts, MaxInventory: 99, MaxStorage: 600, MaxUpgrade: 0, IconPath: "items/arrows_and_bolts/rotbone_bolt.png", Flags: []string{"stackable"}},
	0x031C3420: {Name: "Sleepbone Bolt", Category: "arrows_and_bolts", SubCategory: SubcatArrowsBolts, MaxInventory: 99, MaxStorage: 600, MaxUpgrade: 0, IconPath: "items/arrows_and_bolts/sleepbone_bolt.png", Flags: []string{"stackable"}},
	0x031C5B30: {Name: "Flaming Bolt", Category: "arrows_and_bolts", SubCategory: SubcatArrowsBolts, MaxInventory: 99, MaxStorage: 600, MaxUpgrade: 0, IconPath: "items/arrows_and_bolts/flaming_bolt.png", Flags: []string{"stackable"}},
	0x03216440: {Name: "Piquebone Bolt", Category: "arrows_and_bolts", SubCategory: SubcatArrowsBolts, MaxInventory: 99, MaxStorage: 600, MaxUpgrade: 0, IconPath: "items/arrows_and_bolts/piquebone_bolt.png", Flags: []string{"dlc", "stackable"}},

	// ─── Greatbolts ─────────────────────────────────────────────────────
	0x0328B740: {Name: "Ballista Bolt", Category: "arrows_and_bolts", SubCategory: SubcatArrowsGreatbolts, MaxInventory: 20, MaxStorage: 600, MaxUpgrade: 0, IconPath: "items/arrows_and_bolts/ballista_bolt.png", Flags: []string{"stackable"}},
	0x0328DE50: {Name: "Lightning Greatbolt", Category: "arrows_and_bolts", SubCategory: SubcatArrowsGreatbolts, MaxInventory: 20, MaxStorage: 600, MaxUpgrade: 0, IconPath: "items/arrows_and_bolts/lightning_greatbolt.png", Flags: []string{"stackable"}},
	0x03290560: {Name: "Explosive Greatbolt", Category: "arrows_and_bolts", SubCategory: SubcatArrowsGreatbolts, MaxInventory: 20, MaxStorage: 600, MaxUpgrade: 0, IconPath: "items/arrows_and_bolts/explosive_greatbolt.png", Flags: []string{"stackable"}},
	0x03292C70: {Name: "Bone Ballista Bolt", Category: "arrows_and_bolts", SubCategory: SubcatArrowsGreatbolts, MaxInventory: 20, MaxStorage: 600, MaxUpgrade: 0, IconPath: "items/arrows_and_bolts/bone_ballista_bolt.png", Flags: []string{"stackable"}},
	0x03305860: {Name: "Rabbath's Greatbolt", Category: "arrows_and_bolts", SubCategory: SubcatArrowsGreatbolts, MaxInventory: 20, MaxStorage: 600, MaxUpgrade: 0, IconPath: "items/arrows_and_bolts/rabbaths_greatbolt.png", Flags: []string{"dlc", "stackable"}},
}
