package data

// Information holds items that appear on the in-game "Information" tab
// (Polish: "Informacje"). The split was created after the user verified in-game
// that several letters and maps that er-save-manager classifies in
// `KeyItems.txt` / `Tools.txt` actually live in the Information tab.
//
// Source of truth: Fextralife "Info Items" master list cross-checked against
// per-item Fextralife pages and in-game verification by the user.
// See spec/33-info-tab-category.md for the audit trail.
var Information = map[uint32]ItemData{
	// ─── About * tutorial messages (base) ───────────────────────────────
	0x4000238C: {Name: "About Sites of Grace", Category: "info", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/info/about_sites_of_grace.png"},
	0x4000238D: {Name: "About Sorceries and Incantations", Category: "info", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/info/about_sorceries_and_incantations.png"},
	0x4000238E: {Name: "About Bows", Category: "info", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/info/about_bows.png"},
	0x4000238F: {Name: "About Crouching", Category: "info", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/info/about_crouching.png"},
	0x40002390: {Name: "About Stance-Breaking", Category: "info", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/info/about_stance_breaking.png"},
	0x40002391: {Name: "About Stakes of Marika", Category: "info", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/info/about_stakes_of_marika.png"},
	0x40002392: {Name: "About Guard Counters", Category: "info", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/info/about_guard_counters.png"},
	0x40002393: {Name: "About the Map", Category: "info", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/info/about_the_map.png"},
	0x40002394: {Name: "About Guidance of Grace", Category: "info", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/info/about_guidance_of_grace.png"},
	0x40002395: {Name: "About Horseback Riding", Category: "info", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/info/about_horseback_riding.png"},
	0x40002396: {Name: "About Death", Category: "info", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/info/about_death.png"},
	0x40002397: {Name: "About Summoning Spirits", Category: "info", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/info/about_summoning_spirits.png"},
	0x40002398: {Name: "About Guarding", Category: "info", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/info/about_guarding.png"},
	0x40002399: {Name: "About Item Crafting", Category: "info", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/info/about_item_crafting.png"},
	0x4000239B: {Name: "About Flask of Wondrous Physick", Category: "info", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/info/about_flask_of_wondrous_physick.png"},
	0x4000239C: {Name: "About Adding Skills", Category: "info", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/info/about_adding_skills.png"},
	0x4000239D: {Name: "About Birdseye Telescopes", Category: "info", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/info/about_birdseye_telescopes.png"},
	0x4000239E: {Name: "About Spiritspring Jumping", Category: "info", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/info/about_spiritspring_jumping.png"},
	0x4000239F: {Name: "About Vanquishing Enemy Groups", Category: "info", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/info/about_vanquishing_enemy_groups.png"},
	0x400023A0: {Name: "About Teardrop Scarabs", Category: "info", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/info/about_teardrop_scarabs.png"},
	0x400023A1: {Name: "About Summoning Other Players", Category: "info", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/info/about_summoning_other_players.png"},
	0x400023A2: {Name: "About Cooperative Multiplayer", Category: "info", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/info/about_cooperative_multiplayer.png"},
	0x400023A3: {Name: "About Competitive Multiplayer", Category: "info", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/info/about_competitive_multiplayer.png"},
	0x400023A4: {Name: "About Invasion Multiplayer", Category: "info", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/info/about_invasion_multiplayer.png"},
	0x400023A5: {Name: "About Hunter Multiplayer", Category: "info", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/info/about_hunter_multiplayer.png"},
	0x400023A6: {Name: "About Summoning Pools", Category: "info", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/info/about_summoning_pools.png"},
	// 0x400023A7: removed in patch 1.06 — was reachable on disc v1.0.
	// Spawning it now triggers EAC soft-ban (ban_risk). Not "cut content"
	// (it shipped legitimately), so cut_content flag intentionally dropped.
	0x400023A7: {Name: "About Monument Icon", Category: "info", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/info/about_monument_icon.png", Flags: []string{"ban_risk"}},
	0x400023A8: {Name: "About Requesting Help from Hunters", Category: "info", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/info/about_requesting_help_from_hunters.png"},
	0x400023A9: {Name: "About Skills", Category: "info", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/info/about_skills.png"},
	0x400023AA: {Name: "About Fast Travel to Sites of Grace", Category: "info", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/info/about_fast_travel_to_sites_of_grace.png"},
	0x400023AB: {Name: "About Strengthening Armaments", Category: "info", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/info/about_strengthening_armaments.png"},
	0x400023AC: {Name: "About Roundtable Hold", Category: "info", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/info/about_roundtable_hold.png"},
	0x400023AE: {Name: "About Materials", Category: "info", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/info/about_materials.png"},
	0x400023AF: {Name: "About Containers", Category: "info", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/info/about_containers.png"},
	0x400023B0: {Name: "About Adding Affinities", Category: "info", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/info/about_adding_affinities.png"},
	0x400023B1: {Name: "About Pouches", Category: "info", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/info/about_pouches.png"},
	0x400023B2: {Name: "About Dodging", Category: "info", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/info/about_dodging.png"},
	0x400023B4: {Name: "About Wielding Armaments", Category: "info", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/info/about_wielding_armaments.png"},
	0x400023B5: {Name: "About Great Runes", Category: "info", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/info/about_great_runes.png"},
	0x400023B6: {Name: "About the Cave of Knowledge", Category: "info", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/info/about_the_cave_of_knowledge.png"},
	0x400023BE: {Name: "About Duels", Category: "info", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/info/about_duels.png"},
	0x400023BF: {Name: "About United Combat and Combat Ordeals", Category: "info", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/info/about_united_combat_and_combat_ordeals.png"},
	0x400023C0: {Name: "About Combat with Spirit Ashes", Category: "info", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/info/about_combat_with_spirit_ashes.png"},
	0x400023C1: {Name: "About Marika's Effigy at the Roundtable", Category: "info", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/info/about_marikas_effigy_at_the_roundtable.png"},
	// 0x400023EB: cut content (never shipped). Spawned copies carry [ERROR]
	// prefix at runtime — Fextralife "About Multiplayer" page + Unobtainable Items list.
	0x400023EB: {Name: "About Multiplayer", Category: "info", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/info/about_multiplayer.png", Flags: []string{"cut_content", "ban_risk"}},

	// ─── About * tutorial messages (DLC) ────────────────────────────────
	0x401EA848: {Name: "About the Scadutree Blessing", Category: "info", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/info/about_the_scadutree_blessing.png", Flags: []string{"dlc"}},
	0x401EA849: {Name: "About the Revered Spirit Ash Blessing", Category: "info", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/info/about_the_revered_spirit_ash_blessing.png", Flags: []string{"dlc"}},
	0x401EA84A: {Name: "About New Inventory Features", Category: "info", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/info/about_new_inventory_features.png", Flags: []string{"dlc"}},

	// ─── Paintings ──────────────────────────────────────────────────────
	0x40002008: {Name: "Homing Instinct Painting", Category: "info", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/info/homing_instinct_painting.png"},
	0x40002009: {Name: "Resurrection Painting", Category: "info", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/info/resurrection_painting.png"},
	0x4000200A: {Name: "Champion's Song Painting", Category: "info", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/info/champions_song_painting.png"},
	0x4000200B: {Name: "Sorcerer Painting", Category: "info", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/info/sorcerer_painting.png"},
	0x4000200C: {Name: "Prophecy Painting", Category: "info", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/info/prophecy_painting.png"},
	0x4000200D: {Name: "Flightless Bird Painting", Category: "info", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/info/flightless_bird_painting.png"},
	0x4000200E: {Name: "Redmane Painting", Category: "info", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/info/redmane_painting.png"},
	0x401EA488: {Name: "Incursion Painting", Category: "info", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/info/incursion_painting.png", Flags: []string{"dlc"}},
	0x401EA489: {Name: "The Sacred Tower Painting", Category: "info", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/info/the_sacred_tower_painting.png", Flags: []string{"dlc"}},
	0x401EA48A: {Name: "Domain of Dragons Painting", Category: "info", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/info/domain_of_dragons_painting.png", Flags: []string{"dlc"}},

	// ─── Letters (base) ─────────────────────────────────────────────────
	// User verified all of these appear in the Information tab in-game,
	// even though er-save-manager classifies them under KeyItems.txt.
	// FMG uses the same "Letter from Volcano Manor" name for the Istvan
	// (0x40001FBF) and Rileigh (0x40001FC4) hunt letters. App display
	// names carry a target-NPC suffix so Add Items can disambiguate them;
	// the third hunt letter (Juno Hoslow) is already discriminated in FMG
	// itself as "Red Letter" (0x40001FC5).
	0x40001FBF: {Name: "Letter from Volcano Manor (Istvan)", Category: "info", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/info/letter_from_volcano_manor.png"},
	0x40001FC3: {Name: "Irina's Letter", Category: "info", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/info/irinas_letter.png"},
	0x40001FC4: {Name: "Letter from Volcano Manor (Rileigh)", Category: "info", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/info/letter_from_volcano_manor.png"},
	0x40001FC5: {Name: "Red Letter", Category: "info", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/info/red_letter.png"},
	0x40001FE7: {Name: "Letter to Patches", Category: "info", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/info/letter_to_patches.png"},
	0x40001FED: {Name: "Letter to Bernahl", Category: "info", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/info/letter_to_bernahl.png"},
	// 0x40001FF5: Burial Crow's Letter — Fextralife per-item page calls it
	// cut content, but the user verified it does appear in the Information
	// tab in-game on a save that received it. Keep cut+ban flags.
	0x40001FF5: {Name: "Burial Crow's Letter", Category: "info", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/info/burial_crows_letter.png", Flags: []string{"cut_content", "ban_risk"}},
	0x4000201D: {Name: "Zorayas's Letter", Category: "info", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/info/zorayass_letter.png"},
	0x4000201F: {Name: "Rogier's Letter", Category: "info", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/info/rogiers_letter.png"},

	// ─── Letters / messages (DLC) ──────────────────────────────────────
	// 0x401EA3CF: Letter for Freyja — Fextralife master list says Information,
	// per-item page says Key Item. User has not verified in-game yet.
	// Tagged Information per master list; revisit if user finds it elsewhere.
	0x401EA3C7: {Name: "Cross Map", Category: "info", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/info/cross_map.png", Flags: []string{"dlc"}},
	0x401EA3CF: {Name: "Letter for Freyja", Category: "info", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/info/letter_for_freyja.png", Flags: []string{"dlc"}},
	0x401EA3D0: {Name: "Ruins Map", Category: "info", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/info/ruins_map.png", Flags: []string{"dlc"}},
	0x401EA3D1: {Name: "Ruins Map (2nd)", Category: "info", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/info/ruins_map_2nd.png", Flags: []string{"dlc"}},
	0x401EA3D2: {Name: "Ruins Map (3rd)", Category: "info", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/info/ruins_map_3rd.png", Flags: []string{"dlc"}},
	0x401EA3DB: {Name: "Castle Cross Message", Category: "info", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/info/castle_cross_message.png", Flags: []string{"dlc"}},
	0x401EA3DC: {Name: "Ancient Ruins Cross Message", Category: "info", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/info/ancient_ruins_cross_message.png", Flags: []string{"dlc"}},
	0x401EA3DD: {Name: "Monk's Missive", Category: "info", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/info/monks_missive.png", Flags: []string{"dlc"}},
	0x401EA3DE: {Name: "Storehouse Cross Message", Category: "info", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/info/storehouse_cross_message.png", Flags: []string{"dlc"}},
	0x401EA3E0: {Name: "Torn Diary Page", Category: "info", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/info/torn_diary_page.png", Flags: []string{"dlc"}},
	0x401EA3E2: {Name: "Message from Leda", Category: "info", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/info/message_from_leda.png", Flags: []string{"dlc"}},
	0x401EA3E5: {Name: "Tower of Shadow Message", Category: "info", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/info/tower_of_shadow_message.png", Flags: []string{"dlc"}},

	// ─── Notes (base) ───────────────────────────────────────────────────
	// Hex IDs are Set A (8700–8717 / 0x400021FC–0x4000220D) — the real shipped Notes.
	// Set B (8750+/0x4000222E+) duplicates were cut content and are intentionally not used.
	0x400021FC: {Name: "Note: Hidden Cave", Category: "info", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/info/note_hidden_cave.png"},
	0x400021FD: {Name: "Note: Imp Shades", Category: "info", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/info/note_imp_shades.png"},
	0x400021FE: {Name: "Note: Flask of Wondrous Physick", Category: "info", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/info/note_flask_of_wondrous_physick.png"},
	0x400021FF: {Name: "Note: Stonedigger Trolls", Category: "info", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/info/note_stonedigger_trolls.png"},
	0x40002200: {Name: "Note: Walking Mausoleum", Category: "info", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/info/note_walking_mausoleum.png"},
	0x40002201: {Name: "Note: Unseen Assassins", Category: "info", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/info/note_unseen_assassins.png"},
	// 0x40002202: Note: Great Coffins — VERIFIED cut/inaccessible content via
	// regulation.bin param dump (post-DLC build):
	//   • ZERO references in ItemLotParam_map, ItemLotParam_enemy,
	//     ShopLineupParam, EnvObjLotParam — never given to player in shipped game
	//   • Only reference: CharaInitParam row 8515 (dev/debug NPC template
	//     hardcoded with all 10 Notes 8700–8709 as starting items)
	//   • Polish FMG localization broken: in-game name renders as
	//     "[ERROR]List: Wielkie sarkofagi" — FromSoftware removed the
	//     Polish text entry, confirming the item was cut from shipped product
	// Spawning this is a likely EAC trigger. Flagged ban_risk + cut_content.
	0x40002202: {Name: "Note: Great Coffins", Category: "info", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/info/note_great_coffins.png", Flags: []string{"cut_content", "ban_risk"}},
	0x40002203: {Name: "Note: Flame Chariots", Category: "info", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/info/note_flame_chariots.png"},
	0x40002204: {Name: "Note: Demi-human Mobs", Category: "info", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/info/note_demi_human_mobs.png"},
	0x40002205: {Name: "Note: Land Squirts", Category: "info", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/info/note_land_squirts.png"},
	0x40002206: {Name: "Note: Gravity's Advantage", Category: "info", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/info/note_gravitys_advantage.png"},
	0x40002207: {Name: "Note: Revenants", Category: "info", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/info/note_revenants.png"},
	0x40002208: {Name: "Note: Waypoint Ruins", Category: "info", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/info/note_waypoint_ruins.png"},
	0x40002209: {Name: "Note: Gateway", Category: "info", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/info/note_gateway.png"},
	0x4000220A: {Name: "Note: Miquella's Needle", Category: "info", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/info/note_miquellas_needle.png"},
	0x4000220B: {Name: "Note: Frenzied Flame Village", Category: "info", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/info/note_frenzied_flame_village.png"},
	0x4000220C: {Name: "Note: The Lord of Frenzied Flame", Category: "info", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/info/note_the_lord_of_frenzied_flame.png"},
	0x4000220D: {Name: "Note: Below the Capital", Category: "info", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/info/note_below_the_capital.png"},

	// ─── Sortable misc info items (low-range / standalone) ─────────────
	// Items outside the Set A Notes range (0x400021FC+) and outside the Letters
	// range that nevertheless live on the Information tab in-game per Fextralife.
	// All three have goodsType=12, real iconId, finite sortId in EquipParamGoods.
	0x40002020: {Name: "Note: The Preceptor's Secret", Category: "info", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/info/note_the_preceptors_secret.png"},
	0x40002021: {Name: "Weathered Map", Category: "info", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/info/weathered_map.png"},
	0x40002312: {Name: "Sellia's Secret", Category: "info", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/info/sellias_secret.png"},

	// ─── Notes (DLC) ────────────────────────────────────────────────────
	0x401EA3D9: {Name: "Furnace Keeper's Note", Category: "info", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/info/furnace_keepers_note.png", Flags: []string{"dlc"}},
	// Set A / Set B for SOTE Sealed Spiritsprings: 0x401EA3DF is the shipped
	// canonical entry (goodsType=12, iconId=3861, sortId=453100); the duplicate
	// 0x401EA443 (goodsType=0, iconId=0, sortId=999999) is cut/broken and is
	// intentionally NOT exposed — same precedent as the base-game Set B Notes.
	0x401EA3DF: {Name: "Note: Sealed Spiritsprings", Category: "info", MaxInventory: 1, MaxStorage: 0, MaxUpgrade: 0, IconPath: "items/info/note_sealed_spiritsprings.png", Flags: []string{"dlc"}},
}

// Note: Region/World Maps (Map: Limgrave West, Map: Caelid, etc.) — relocated
// to key_items.go per spec/36 (correct in-game tab is Key Items > World Maps).
// Single source of truth — do NOT duplicate here.
