package data

// BolsteringMaterials — items used to upgrade Sacred Flasks, Spirit Ashes,
// weapons, and Shadow Realm progression. 1:1 with in-game "Bolstering Materials" tab.
//
// Sub-groups (in-game order):
//   1. Flask Enhancers (Golden Seed + Sacred Tear)
//   2. Shadow Realm Blessings (DLC: Scadutree Fragment + Revered Spirit Ash)
//   3. Smithing Stones [1-8] + Ancient Dragon Smithing Stone
//   4. Somberstones [1-9] + Somber Ancient Dragon Smithing Stone
//   5. Grave Glovewort [1-9] + Great Grave Glovewort
//   6. Ghost Glovewort [1-9] + Great Ghost Glovewort
var BolsteringMaterials = map[uint32]ItemData{
	// ─── Flask Enhancers ────────────────────────────────────────────────
	// Caps reflect "max useful" amounts (full flask charges / max potency),
	// not raw world counts. NG+ does not raise the effective cap because surplus
	// past these limits has zero functional value:
	//   Golden Seed cap 30 = full flask charges (14)
	//   Sacred Tear cap 12 = full flask potency (+12)
	0x4000271A: {Name: "Golden Seed", Category: "bolstering_materials", SubCategory: SubcatBolsteringFlaskEnhancers, MaxInventory: 30, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/bolstering_materials/golden_seed.png", Flags: []string{"stackable"}},
	0x40002724: {Name: "Sacred Tear", Category: "bolstering_materials", SubCategory: SubcatBolsteringFlaskEnhancers, MaxInventory: 12, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/bolstering_materials/sacred_tear.png", Flags: []string{"stackable"}},

	// ─── Shadow Realm Blessings (DLC) ───────────────────────────────────
	//   Scadutree Fragment cap 50 = max Scadutree Blessing (+20 cumulative cost)
	//   Revered Spirit Ash cap 25 = max Revered Spirit Ash Blessing (+10 cumulative cost)
	0x401EAB90: {Name: "Scadutree Fragment", Category: "bolstering_materials", SubCategory: SubcatBolsteringShadowRealmBlessings, MaxInventory: 50, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/bolstering_materials/scadutree_fragment.png", Flags: []string{"dlc", "stackable"}},
	0x401EABF4: {Name: "Revered Spirit Ash", Category: "bolstering_materials", SubCategory: SubcatBolsteringShadowRealmBlessings, MaxInventory: 25, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/bolstering_materials/revered_spirit_ash.png", Flags: []string{"dlc", "stackable"}},

	// ─── Smithing Stones [1-8] + Ancient Dragon Smithing Stone ──────────
	0x40002774: {Name: "Smithing Stone [1]", Category: "bolstering_materials", SubCategory: SubcatBolsteringSmithingStones, MaxInventory: 999, MaxStorage: 999, MaxUpgrade: 0, IconPath: "items/bolstering_materials/smithing_stone_1.png", Flags: []string{"stackable"}},
	0x40002775: {Name: "Smithing Stone [2]", Category: "bolstering_materials", SubCategory: SubcatBolsteringSmithingStones, MaxInventory: 999, MaxStorage: 999, MaxUpgrade: 0, IconPath: "items/bolstering_materials/smithing_stone_2.png", Flags: []string{"stackable"}},
	0x40002776: {Name: "Smithing Stone [3]", Category: "bolstering_materials", SubCategory: SubcatBolsteringSmithingStones, MaxInventory: 999, MaxStorage: 999, MaxUpgrade: 0, IconPath: "items/bolstering_materials/smithing_stone_3.png", Flags: []string{"stackable"}},
	0x40002777: {Name: "Smithing Stone [4]", Category: "bolstering_materials", SubCategory: SubcatBolsteringSmithingStones, MaxInventory: 999, MaxStorage: 999, MaxUpgrade: 0, IconPath: "items/bolstering_materials/smithing_stone_4.png", Flags: []string{"stackable"}},
	0x40002778: {Name: "Smithing Stone [5]", Category: "bolstering_materials", SubCategory: SubcatBolsteringSmithingStones, MaxInventory: 999, MaxStorage: 999, MaxUpgrade: 0, IconPath: "items/bolstering_materials/smithing_stone_5.png", Flags: []string{"stackable"}},
	0x40002779: {Name: "Smithing Stone [6]", Category: "bolstering_materials", SubCategory: SubcatBolsteringSmithingStones, MaxInventory: 999, MaxStorage: 999, MaxUpgrade: 0, IconPath: "items/bolstering_materials/smithing_stone_6.png", Flags: []string{"stackable"}},
	0x4000277A: {Name: "Smithing Stone [7]", Category: "bolstering_materials", SubCategory: SubcatBolsteringSmithingStones, MaxInventory: 999, MaxStorage: 999, MaxUpgrade: 0, IconPath: "items/bolstering_materials/smithing_stone_7.png", Flags: []string{"stackable"}},
	0x4000277B: {Name: "Smithing Stone [8]", Category: "bolstering_materials", SubCategory: SubcatBolsteringSmithingStones, MaxInventory: 999, MaxStorage: 999, MaxUpgrade: 0, IconPath: "items/bolstering_materials/smithing_stone_8.png", Flags: []string{"stackable"}},
	0x4000279C: {Name: "Ancient Dragon Smithing Stone", Category: "bolstering_materials", SubCategory: SubcatBolsteringSmithingStones, MaxInventory: 18, MaxStorage: 18, MaxUpgrade: 0, IconPath: "items/bolstering_materials/ancient_dragon_smithing_stone.png", Flags: []string{"stackable"}},

	// ─── Somberstones [1-9] + Somber Ancient Dragon Smithing Stone ──────
	0x400027B0: {Name: "Somber Smithing Stone [1]", Category: "bolstering_materials", SubCategory: SubcatBolsteringSomberstones, MaxInventory: 999, MaxStorage: 999, MaxUpgrade: 0, IconPath: "items/bolstering_materials/somber_smithing_stone_1.png", Flags: []string{"stackable"}},
	0x400027B1: {Name: "Somber Smithing Stone [2]", Category: "bolstering_materials", SubCategory: SubcatBolsteringSomberstones, MaxInventory: 999, MaxStorage: 999, MaxUpgrade: 0, IconPath: "items/bolstering_materials/somber_smithing_stone_2.png", Flags: []string{"stackable"}},
	0x400027B2: {Name: "Somber Smithing Stone [3]", Category: "bolstering_materials", SubCategory: SubcatBolsteringSomberstones, MaxInventory: 999, MaxStorage: 999, MaxUpgrade: 0, IconPath: "items/bolstering_materials/somber_smithing_stone_3.png", Flags: []string{"stackable"}},
	0x400027B3: {Name: "Somber Smithing Stone [4]", Category: "bolstering_materials", SubCategory: SubcatBolsteringSomberstones, MaxInventory: 999, MaxStorage: 999, MaxUpgrade: 0, IconPath: "items/bolstering_materials/somber_smithing_stone_4.png", Flags: []string{"stackable"}},
	0x400027B4: {Name: "Somber Smithing Stone [5]", Category: "bolstering_materials", SubCategory: SubcatBolsteringSomberstones, MaxInventory: 999, MaxStorage: 999, MaxUpgrade: 0, IconPath: "items/bolstering_materials/somber_smithing_stone_5.png", Flags: []string{"stackable"}},
	0x400027B5: {Name: "Somber Smithing Stone [6]", Category: "bolstering_materials", SubCategory: SubcatBolsteringSomberstones, MaxInventory: 999, MaxStorage: 999, MaxUpgrade: 0, IconPath: "items/bolstering_materials/somber_smithing_stone_6.png", Flags: []string{"stackable"}},
	0x400027B6: {Name: "Somber Smithing Stone [7]", Category: "bolstering_materials", SubCategory: SubcatBolsteringSomberstones, MaxInventory: 999, MaxStorage: 999, MaxUpgrade: 0, IconPath: "items/bolstering_materials/somber_smithing_stone_7.png", Flags: []string{"stackable"}},
	0x400027B7: {Name: "Somber Smithing Stone [8]", Category: "bolstering_materials", SubCategory: SubcatBolsteringSomberstones, MaxInventory: 999, MaxStorage: 999, MaxUpgrade: 0, IconPath: "items/bolstering_materials/somber_smithing_stone_8.png", Flags: []string{"stackable"}},
	0x400027D8: {Name: "Somber Smithing Stone [9]", Category: "bolstering_materials", SubCategory: SubcatBolsteringSomberstones, MaxInventory: 999, MaxStorage: 999, MaxUpgrade: 0, IconPath: "items/bolstering_materials/somber_smithing_stone_9.png", Flags: []string{"stackable"}},
	0x400027B8: {Name: "Somber Ancient Dragon Smithing Stone", Category: "bolstering_materials", SubCategory: SubcatBolsteringSomberstones, MaxInventory: 15, MaxStorage: 15, MaxUpgrade: 0, IconPath: "items/bolstering_materials/somber_ancient_dragon_smithing_stone.png", Flags: []string{"stackable"}},

	// ─── Grave Glovewort [1-9] + Great Grave Glovewort ──────────────────
	0x40002A94: {Name: "Grave Glovewort [1]", Category: "bolstering_materials", SubCategory: SubcatBolsteringGraveGlovewort, MaxInventory: 999, MaxStorage: 999, MaxUpgrade: 0, IconPath: "items/bolstering_materials/grave_glovewort_1.png", Flags: []string{"stackable"}},
	0x40002A95: {Name: "Grave Glovewort [2]", Category: "bolstering_materials", SubCategory: SubcatBolsteringGraveGlovewort, MaxInventory: 999, MaxStorage: 999, MaxUpgrade: 0, IconPath: "items/bolstering_materials/grave_glovewort_2.png", Flags: []string{"stackable"}},
	0x40002A96: {Name: "Grave Glovewort [3]", Category: "bolstering_materials", SubCategory: SubcatBolsteringGraveGlovewort, MaxInventory: 999, MaxStorage: 999, MaxUpgrade: 0, IconPath: "items/bolstering_materials/grave_glovewort_3.png", Flags: []string{"stackable"}},
	0x40002A97: {Name: "Grave Glovewort [4]", Category: "bolstering_materials", SubCategory: SubcatBolsteringGraveGlovewort, MaxInventory: 999, MaxStorage: 999, MaxUpgrade: 0, IconPath: "items/bolstering_materials/grave_glovewort_4.png", Flags: []string{"stackable"}},
	0x40002A98: {Name: "Grave Glovewort [5]", Category: "bolstering_materials", SubCategory: SubcatBolsteringGraveGlovewort, MaxInventory: 999, MaxStorage: 999, MaxUpgrade: 0, IconPath: "items/bolstering_materials/grave_glovewort_5.png", Flags: []string{"stackable"}},
	0x40002A99: {Name: "Grave Glovewort [6]", Category: "bolstering_materials", SubCategory: SubcatBolsteringGraveGlovewort, MaxInventory: 999, MaxStorage: 999, MaxUpgrade: 0, IconPath: "items/bolstering_materials/grave_glovewort_6.png", Flags: []string{"stackable"}},
	0x40002A9A: {Name: "Grave Glovewort [7]", Category: "bolstering_materials", SubCategory: SubcatBolsteringGraveGlovewort, MaxInventory: 999, MaxStorage: 999, MaxUpgrade: 0, IconPath: "items/bolstering_materials/grave_glovewort_7.png", Flags: []string{"stackable"}},
	0x40002A9B: {Name: "Grave Glovewort [8]", Category: "bolstering_materials", SubCategory: SubcatBolsteringGraveGlovewort, MaxInventory: 999, MaxStorage: 999, MaxUpgrade: 0, IconPath: "items/bolstering_materials/grave_glovewort_8.png", Flags: []string{"stackable"}},
	0x40002A9C: {Name: "Grave Glovewort [9]", Category: "bolstering_materials", SubCategory: SubcatBolsteringGraveGlovewort, MaxInventory: 999, MaxStorage: 999, MaxUpgrade: 0, IconPath: "items/bolstering_materials/grave_glovewort_9.png", Flags: []string{"stackable"}},
	0x40002A9D: {Name: "Great Grave Glovewort", Category: "bolstering_materials", SubCategory: SubcatBolsteringGraveGlovewort, MaxInventory: 12, MaxStorage: 12, MaxUpgrade: 0, IconPath: "items/bolstering_materials/great_grave_glovewort.png", Flags: []string{"stackable"}},

	// ─── Ghost Glovewort [1-9] + Great Ghost Glovewort ──────────────────
	0x40002A9E: {Name: "Ghost Glovewort [1]", Category: "bolstering_materials", SubCategory: SubcatBolsteringGhostGlovewort, MaxInventory: 999, MaxStorage: 999, MaxUpgrade: 0, IconPath: "items/bolstering_materials/ghost_glovewort_1.png", Flags: []string{"stackable"}},
	0x40002A9F: {Name: "Ghost Glovewort [2]", Category: "bolstering_materials", SubCategory: SubcatBolsteringGhostGlovewort, MaxInventory: 999, MaxStorage: 999, MaxUpgrade: 0, IconPath: "items/bolstering_materials/ghost_glovewort_2.png", Flags: []string{"stackable"}},
	0x40002AA0: {Name: "Ghost Glovewort [3]", Category: "bolstering_materials", SubCategory: SubcatBolsteringGhostGlovewort, MaxInventory: 999, MaxStorage: 999, MaxUpgrade: 0, IconPath: "items/bolstering_materials/ghost_glovewort_3.png", Flags: []string{"stackable"}},
	0x40002AA1: {Name: "Ghost Glovewort [4]", Category: "bolstering_materials", SubCategory: SubcatBolsteringGhostGlovewort, MaxInventory: 999, MaxStorage: 999, MaxUpgrade: 0, IconPath: "items/bolstering_materials/ghost_glovewort_4.png", Flags: []string{"stackable"}},
	0x40002AA2: {Name: "Ghost Glovewort [5]", Category: "bolstering_materials", SubCategory: SubcatBolsteringGhostGlovewort, MaxInventory: 999, MaxStorage: 999, MaxUpgrade: 0, IconPath: "items/bolstering_materials/ghost_glovewort_5.png", Flags: []string{"stackable"}},
	0x40002AA3: {Name: "Ghost Glovewort [6]", Category: "bolstering_materials", SubCategory: SubcatBolsteringGhostGlovewort, MaxInventory: 999, MaxStorage: 999, MaxUpgrade: 0, IconPath: "items/bolstering_materials/ghost_glovewort_6.png", Flags: []string{"stackable"}},
	0x40002AA4: {Name: "Ghost Glovewort [7]", Category: "bolstering_materials", SubCategory: SubcatBolsteringGhostGlovewort, MaxInventory: 999, MaxStorage: 999, MaxUpgrade: 0, IconPath: "items/bolstering_materials/ghost_glovewort_7.png", Flags: []string{"stackable"}},
	0x40002AA5: {Name: "Ghost Glovewort [8]", Category: "bolstering_materials", SubCategory: SubcatBolsteringGhostGlovewort, MaxInventory: 999, MaxStorage: 999, MaxUpgrade: 0, IconPath: "items/bolstering_materials/ghost_glovewort_8.png", Flags: []string{"stackable"}},
	0x40002AA6: {Name: "Ghost Glovewort [9]", Category: "bolstering_materials", SubCategory: SubcatBolsteringGhostGlovewort, MaxInventory: 999, MaxStorage: 999, MaxUpgrade: 0, IconPath: "items/bolstering_materials/ghost_glovewort_9.png", Flags: []string{"stackable"}},
	0x40002AA7: {Name: "Great Ghost Glovewort", Category: "bolstering_materials", SubCategory: SubcatBolsteringGhostGlovewort, MaxInventory: 9, MaxStorage: 9, MaxUpgrade: 0, IconPath: "items/bolstering_materials/great_ghost_glovewort.png", Flags: []string{"stackable"}},
}
