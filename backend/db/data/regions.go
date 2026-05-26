package data

// RegionData describes an "unlocked region" entry stored in the per-slot Regions
// struct (count: u32 followed by count * u32 region IDs). Each ID corresponds to a
// discrete map area. The list controls:
//   - Invasion eligibility (PvP / NPC invaders) for that area
//   - Blue summons (Recusant Henricus, Bloody Finger questlines)
//   - The "You have entered <X>" map label after teleport
//
// IMPORTANT — this map is a CURATED invasion/blue-summon ALLOWLIST, not the full
// PlayRegionParam table and not the full set of raw IDs a save may carry:
//   - A real region ID is merely a row in regulation.bin PlayRegionParam (594 rows).
//   - A curated region is one confirmed to be a standard invasion / blue-summon
//     target. This set is the Elden-Ring-CT-TGA "Invasion Regions" table — a
//     dedicated invasion-targeting list curated by invaders (Dasaav; DLC by
//     Joel/SeriouslyCasual). Its open-world / dungeon / boss-fog entries are all
//     valid invasion contexts; it deliberately omits multiplayer hubs (Roundtable
//     Hold has no PlayRegion row at all) and colosseums (separate matchmaking).
//   - Internal sub-area / network-only PlayRegionParam rows are deliberately
//     excluded. The World tab's "Unlock All" therefore unlocks all VERIFIED legal
//     PvP regions, never "PvP everywhere", and preserves any non-curated raw IDs.
//
// Every ID below is a real PlayRegionParam Row ID and present 1:1 in the TGA list.
// The 6800000/6900000 labels are corrected per the game's PlaceName_dlc01 FMG
// (680000 = "Gravesite Plain", 690000 = "Scadu Altus") + BonfireWarpParam.
// Totals: 274 curated regions (208 base, 66 DLC).
type RegionData struct {
	Name string
	Area string
	DLC  bool
}

// Regions maps region IDs to their human-readable name and grouping area.
var Regions = map[uint32]RegionData{
	// ========================================================
	// Limgrave
	// ========================================================
	6100000: {Name: "The First Step", Area: "Limgrave"},
	6100001: {Name: "Seaside Ruins", Area: "Limgrave"},
	6100002: {Name: "Agheel Lake North", Area: "Limgrave"},
	6100003: {Name: "Summonwater Village Outskirts", Area: "Limgrave"},
	6100004: {Name: "Mistwood Outskirts", Area: "Limgrave"},
	6100010: {Name: "Waypoint Ruins Cellar", Area: "Limgrave"},
	6100090: {Name: "Church of Dragon Communion", Area: "Limgrave"},
	6101000: {Name: "Stormhill Shack", Area: "Limgrave"},
	6101010: {Name: "Margit, the Fell Omen", Area: "Limgrave"},
	6102000: {Name: "Weeping Peninsula (West)", Area: "Limgrave"},
	6102001: {Name: "Weeping Peninsula (East)", Area: "Limgrave"},
	6102002: {Name: "Castle Morne", Area: "Limgrave"},
	6102020: {Name: "Morne Moangrave", Area: "Limgrave"},

	// ========================================================
	// Liurnia
	// ========================================================
	6200000: {Name: "Lake-Facing Cliffs", Area: "Liurnia"},
	6200001: {Name: "Liurnia Highway South", Area: "Liurnia"},
	6200002: {Name: "Liurnia Lake Shore", Area: "Liurnia"},
	6200003: {Name: "Kingsrealm Ruins", Area: "Liurnia"},
	6200004: {Name: "Eastern Tableland", Area: "Liurnia"},
	6200005: {Name: "Crystalline Woods", Area: "Liurnia"},
	6200006: {Name: "The Ravine", Area: "Liurnia"},
	6200007: {Name: "Main Caria Manor Gate", Area: "Liurnia"},
	6200008: {Name: "Behind Caria Manor", Area: "Liurnia"},
	6200010: {Name: "Royal Moongazing Grounds", Area: "Liurnia"},
	6200090: {Name: "Grand Lift of Dectus", Area: "Liurnia"},
	6201000: {Name: "Bellum Church", Area: "Liurnia"},
	6202000: {Name: "Moonlight Altar", Area: "Liurnia"},

	// ========================================================
	// Altus Plateau
	// ========================================================
	6300000: {Name: "Stormcaller Church", Area: "Altus Plateau"},
	6300001: {Name: "The Shaded Castle", Area: "Altus Plateau"},
	6300002: {Name: "Altus Highway Junction", Area: "Altus Plateau"},
	6300003: {Name: "Forest-Spanning Greatbridge", Area: "Altus Plateau"},
	6300004: {Name: "Dominula, Windmill Village", Area: "Altus Plateau"},
	6300005: {Name: "Rampartside Path", Area: "Altus Plateau"},
	6300030: {Name: "Castellan's Hall", Area: "Altus Plateau"},
	6301000: {Name: "Capital Outskirts", Area: "Altus Plateau"},
	6301090: {Name: "Capital Rampart", Area: "Altus Plateau"},

	// ========================================================
	// Mt. Gelmir
	// ========================================================
	6302000: {Name: "Ninth Mt. Gelmir Campsite", Area: "Mt. Gelmir"},
	6302001: {Name: "Road of Iniquity", Area: "Mt. Gelmir"},
	6302002: {Name: "Seethewater Terminus", Area: "Mt. Gelmir"},

	// ========================================================
	// Caelid
	// ========================================================
	6400000: {Name: "Caelid Highway South", Area: "Caelid"},
	6400001: {Name: "Caelem Ruins", Area: "Caelid"},
	6400002: {Name: "Chamber Outside the Plaza", Area: "Caelid"},
	6400010: {Name: "Redmane Castle Plaza", Area: "Caelid"},
	6400020: {Name: "Chair-Crypt of Sellia", Area: "Caelid"},
	6400040: {Name: "Starscourge Radahn", Area: "Caelid"},
	6401000: {Name: "Swamp of Aeonia", Area: "Caelid"},
	6402000: {Name: "Dragonbarrow West", Area: "Caelid"},
	6402001: {Name: "Bestial Sanctum", Area: "Caelid"},

	// ========================================================
	// Mountaintops
	// ========================================================
	6500000: {Name: "Forbidden Lands", Area: "Mountaintops"},
	6500090: {Name: "Grand Lift of Rold", Area: "Mountaintops"},
	6501000: {Name: "Zamor Ruins", Area: "Mountaintops"},
	6501001: {Name: "Central Mountaintops", Area: "Mountaintops"},
	6501002: {Name: "Castle Sol", Area: "Mountaintops"},
	6501003: {Name: "Castle Sol Main Gate, Church of the Eclipse", Area: "Mountaintops"},
	6501010: {Name: "Castle Sol Rooftop", Area: "Mountaintops"},
	6502000: {Name: "Consecrated Snowfield", Area: "Mountaintops"},
	6502010: {Name: "Fire Giant", Area: "Mountaintops"},
	6503000: {Name: "Consecrated Snowfield", Area: "Mountaintops"},

	// ========================================================
	// Underground
	// ========================================================
	1201000: {Name: "Dragonkin Soldier of Nokstella", Area: "Underground"},
	1201001: {Name: "Ainsel River Well Depths", Area: "Underground"},
	1201002: {Name: "Ainsel River Downstream", Area: "Underground"},
	1201003: {Name: "Ainsel River Downstream Part II", Area: "Underground"},
	1201011: {Name: "Ainsel River Main", Area: "Underground"},
	1201013: {Name: "Nokstella, Eternal City", Area: "Underground"},
	1201014: {Name: "Nokstella Waterfall Basin", Area: "Underground"},
	1201015: {Name: "Lake of Rot Shoreside", Area: "Underground"},
	1201016: {Name: "Grand Cloister", Area: "Underground"},
	1201017: {Name: "Grand Cloister Part II", Area: "Underground"},
	1202000: {Name: "Great Waterfall Basin", Area: "Underground"},
	1202002: {Name: "Ancestral Woods", Area: "Underground"},
	1202003: {Name: "Aqueduct-Facing Cliffs", Area: "Underground"},
	1202004: {Name: "Aqueduct-Facing Cliffs Part II", Area: "Underground"},
	1202007: {Name: "Night's Sacred Ground", Area: "Underground"},
	1202020: {Name: "Mimic Tear", Area: "Underground"},
	1202033: {Name: "Siofra River Bank", Area: "Underground"},
	1202034: {Name: "Worshippers' Woods", Area: "Underground"},
	1203000: {Name: "Prince of Death's Throne", Area: "Underground"},
	1203001: {Name: "Root-Facing Cliffs", Area: "Underground"},
	1203002: {Name: "Great Waterfall Crest Part II", Area: "Underground"},
	1203003: {Name: "Deeproot Depths", Area: "Underground"},
	1203004: {Name: "The Nameless Eternal City", Area: "Underground"},
	1203005: {Name: "Across the Roots", Area: "Underground"},
	1204000: {Name: "Astel, Naturalborn of the Void", Area: "Underground"},
	1205000: {Name: "Cocoon of the Empyrean", Area: "Underground"},
	1205001: {Name: "Palace Approach Ledge-Road", Area: "Underground"},
	1205004: {Name: "Dynasty Mausoleum Entrance", Area: "Underground"},
	1205006: {Name: "Dynasty Mausoleum Midpoint", Area: "Underground"},
	1207026: {Name: "Nokron, Eternal City", Area: "Underground"},
	1207031: {Name: "Siofra River Well Depths", Area: "Underground"},

	// ========================================================
	// Farum Azula
	// ========================================================
	1300000: {Name: "Maliketh, the Black Blade", Area: "Farum Azula"},
	1300003: {Name: "Dragon Temple Rooftop", Area: "Farum Azula"},
	1300006: {Name: "Beside the Great Bridge", Area: "Farum Azula"},
	1300010: {Name: "Dragon Temple Altar", Area: "Farum Azula"},
	1300012: {Name: "Crumbling Beast Grave", Area: "Farum Azula"},
	1300013: {Name: "Crumbling Beast Grave Depths, Tempest-Facing Balcony", Area: "Farum Azula"},
	1300017: {Name: "Dragon Temple", Area: "Farum Azula"},
	1300018: {Name: "Dragon Temple Transept", Area: "Farum Azula"},
	1300019: {Name: "Dragon Temple Lift", Area: "Farum Azula"},
	1300020: {Name: "Dragonlord Placidusax", Area: "Farum Azula"},

	// ========================================================
	// Haligtree
	// ========================================================
	1500000: {Name: "Malenia, Goddess of Rot", Area: "Haligtree"},
	1500001: {Name: "Prayer Room", Area: "Haligtree"},
	1500002: {Name: "Elphael Inner Wall, Drainage Channel", Area: "Haligtree"},
	1500003: {Name: "Haligtree Roots", Area: "Haligtree"},
	1500010: {Name: "Haligtree Promenade", Area: "Haligtree"},
	1500011: {Name: "Haligtree Canopy", Area: "Haligtree"},
	1500012: {Name: "Haligtree Town Plaza", Area: "Haligtree"},

	// ========================================================
	// Legacy Dungeons
	// ========================================================
	1000000: {Name: "Stormveil Castle", Area: "Legacy Dungeons"},
	1000001: {Name: "Stormveil Main Gate", Area: "Legacy Dungeons"},
	1000003: {Name: "Rampart Tower", Area: "Legacy Dungeons"},
	1000005: {Name: "Liftside Chamber", Area: "Legacy Dungeons"},
	1000006: {Name: "Gateside Chamber", Area: "Legacy Dungeons"},
	1100000: {Name: "Leyndell, Royal Capital", Area: "Legacy Dungeons"},
	1100001: {Name: "Queen's Bedchamber", Area: "Legacy Dungeons"},
	1100010: {Name: "Leyndell - Erdtree Sanctuary", Area: "Legacy Dungeons"},
	1100012: {Name: "East Capital Rampart", Area: "Legacy Dungeons"},
	1100013: {Name: "Avenue Balcony", Area: "Legacy Dungeons"},
	1100015: {Name: "Lower Capital Church", Area: "Legacy Dungeons"},
	1100016: {Name: "West Capital Rampart", Area: "Legacy Dungeons"},
	1100017: {Name: "Divine Bridge", Area: "Legacy Dungeons"},
	1105000: {Name: "Ashen Elden Throne", Area: "Legacy Dungeons"},
	1105001: {Name: "Ashen Queen's Bedchamber", Area: "Legacy Dungeons"},
	1105011: {Name: "Leyndell, Capital of Ash", Area: "Legacy Dungeons"},
	1105092: {Name: "Ashen Divine Bridge", Area: "Legacy Dungeons"},
	1400000: {Name: "Academy of Raya Lucaria", Area: "Legacy Dungeons"},
	1400010: {Name: "Debate Parlor", Area: "Legacy Dungeons"},
	1400011: {Name: "Main Academy Gate", Area: "Legacy Dungeons"},
	1400013: {Name: "Church of the Cuckoo", Area: "Legacy Dungeons"},
	1400015: {Name: "School House Classroom", Area: "Legacy Dungeons"},
	1600000: {Name: "Volcano Manor", Area: "Legacy Dungeons"},
	1600006: {Name: "Audience Pathway", Area: "Legacy Dungeons"},
	1600010: {Name: "Temple of Eiglay", Area: "Legacy Dungeons"},
	1600012: {Name: "Volcano Manor (interior)", Area: "Legacy Dungeons"},
	1600014: {Name: "Prison Town Church", Area: "Legacy Dungeons"},
	1600016: {Name: "Guest Hall", Area: "Legacy Dungeons"},
	1600020: {Name: "Abductor Virgin", Area: "Legacy Dungeons"},
	1600022: {Name: "Subterranean Inquisition Chamber", Area: "Legacy Dungeons"},
	1800001: {Name: "Stranded Graveyard", Area: "Legacy Dungeons"},
	1800090: {Name: "Cave of Knowledge", Area: "Legacy Dungeons"},
	1900000: {Name: "Stone Platform", Area: "Legacy Dungeons"},
	1900001: {Name: "Elden Beast", Area: "Legacy Dungeons"},
	3500000: {Name: "Cathedral of the Forsaken", Area: "Legacy Dungeons"},
	3500002: {Name: "Underground Roadside", Area: "Legacy Dungeons"},
	3500008: {Name: "Forsaken Depths", Area: "Legacy Dungeons"},
	3500010: {Name: "Leyndell Catacombs", Area: "Legacy Dungeons"},
	3500011: {Name: "Leyndell Catacombs Part II", Area: "Legacy Dungeons"},
	3500092: {Name: "Frenzied Flame Proscription", Area: "Legacy Dungeons"},

	// ========================================================
	// Catacombs, Caves & Tunnels
	// ========================================================
	3000001: {Name: "Tombsward Catacombs", Area: "Catacombs, Caves & Tunnels"},
	3001001: {Name: "Impaler's Catacombs", Area: "Catacombs, Caves & Tunnels"},
	3002001: {Name: "Stormfoot Catacombs", Area: "Catacombs, Caves & Tunnels"},
	3003001: {Name: "Road's End Catacombs", Area: "Catacombs, Caves & Tunnels"},
	3004001: {Name: "Murkwater Catacombs", Area: "Catacombs, Caves & Tunnels"},
	3005001: {Name: "Black Knife Catacombs", Area: "Catacombs, Caves & Tunnels"},
	3006001: {Name: "Cliffbottom Catacombs", Area: "Catacombs, Caves & Tunnels"},
	3007001: {Name: "Wyndham Catacombs", Area: "Catacombs, Caves & Tunnels"},
	3008001: {Name: "Sainted Hero's Grave", Area: "Catacombs, Caves & Tunnels"},
	3009001: {Name: "Gelmir Hero's Grave", Area: "Catacombs, Caves & Tunnels"},
	3010001: {Name: "Auriza Hero's Grave", Area: "Catacombs, Caves & Tunnels"},
	3011001: {Name: "Deathtouched Catacombs", Area: "Catacombs, Caves & Tunnels"},
	3012001: {Name: "Unsightly Catacombs", Area: "Catacombs, Caves & Tunnels"},
	3013091: {Name: "Auriza Side Tomb", Area: "Catacombs, Caves & Tunnels"},
	3014001: {Name: "Minor Erdtree Catacombs", Area: "Catacombs, Caves & Tunnels"},
	3015001: {Name: "Caelid Catacombs", Area: "Catacombs, Caves & Tunnels"},
	3016001: {Name: "War-Dead Catacombs", Area: "Catacombs, Caves & Tunnels"},
	3017002: {Name: "Giant-Conquering Hero's Grave", Area: "Catacombs, Caves & Tunnels"},
	3018001: {Name: "Giants' Mountaintop Catacombs", Area: "Catacombs, Caves & Tunnels"},
	3019001: {Name: "Consecrated Snowfield Catacombs", Area: "Catacombs, Caves & Tunnels"},
	3020001: {Name: "Hidden Path to the Haligtree", Area: "Catacombs, Caves & Tunnels"},
	3020002: {Name: "Hidden Path to the Haligtree Part II", Area: "Catacombs, Caves & Tunnels"},
	3100001: {Name: "Murkwater Cave", Area: "Catacombs, Caves & Tunnels"},
	3101001: {Name: "Earthbore Cave", Area: "Catacombs, Caves & Tunnels"},
	3102001: {Name: "Tombsward Cave", Area: "Catacombs, Caves & Tunnels"},
	3103001: {Name: "Groveside Cave", Area: "Catacombs, Caves & Tunnels"},
	3104001: {Name: "Stillwater Cave", Area: "Catacombs, Caves & Tunnels"},
	3105001: {Name: "Lakeside Crystal Cave", Area: "Catacombs, Caves & Tunnels"},
	3105090: {Name: "Slumbering Wolf's Shack", Area: "Catacombs, Caves & Tunnels"},
	3106001: {Name: "Academy Crystal Cave", Area: "Catacombs, Caves & Tunnels"},
	3107001: {Name: "Seethewater Cave", Area: "Catacombs, Caves & Tunnels"},
	3109001: {Name: "Volcano Cave", Area: "Catacombs, Caves & Tunnels"},
	3110001: {Name: "Dragonbarrow Cave", Area: "Catacombs, Caves & Tunnels"},
	3111001: {Name: "Sellia Hideaway", Area: "Catacombs, Caves & Tunnels"},
	3112001: {Name: "Cave of the Forlorn", Area: "Catacombs, Caves & Tunnels"},
	3115001: {Name: "Coastal Cave", Area: "Catacombs, Caves & Tunnels"},
	3117001: {Name: "Highroad Cave", Area: "Catacombs, Caves & Tunnels"},
	3118001: {Name: "Perfumer's Grotto", Area: "Catacombs, Caves & Tunnels"},
	3119001: {Name: "Sage's Cave", Area: "Catacombs, Caves & Tunnels"},
	3120001: {Name: "Abandoned Cave", Area: "Catacombs, Caves & Tunnels"},
	3121001: {Name: "Gaol Cave", Area: "Catacombs, Caves & Tunnels"},
	3122001: {Name: "Spiritcaller's Cave", Area: "Catacombs, Caves & Tunnels"},
	3200001: {Name: "Morne Tunnel", Area: "Catacombs, Caves & Tunnels"},
	3201001: {Name: "Limgrave Tunnels", Area: "Catacombs, Caves & Tunnels"},
	3202001: {Name: "Raya Lucaria Crystal Tunnel", Area: "Catacombs, Caves & Tunnels"},
	3204001: {Name: "Old Altus Tunnel", Area: "Catacombs, Caves & Tunnels"},
	3205001: {Name: "Altus Tunnel", Area: "Catacombs, Caves & Tunnels"},
	3207001: {Name: "Gael Tunnel", Area: "Catacombs, Caves & Tunnels"},
	3207002: {Name: "Gael Tunnel Part II", Area: "Catacombs, Caves & Tunnels"},
	3207090: {Name: "Rear Gael Tunnel Entrance", Area: "Catacombs, Caves & Tunnels"},
	3208001: {Name: "Sellia Crystal Tunnel", Area: "Catacombs, Caves & Tunnels"},
	3211001: {Name: "Yelough Anix Tunnel", Area: "Catacombs, Caves & Tunnels"},
	3410090: {Name: "Divine Tower of Limgrave", Area: "Catacombs, Caves & Tunnels"},
	3411090: {Name: "Divine Tower of Liurnia", Area: "Catacombs, Caves & Tunnels"},
	3412011: {Name: "Sealed Tunnel", Area: "Catacombs, Caves & Tunnels"},
	3412090: {Name: "Divine Tower of West Altus", Area: "Catacombs, Caves & Tunnels"},
	3413003: {Name: "Divine Tower of Caelid: Center", Area: "Catacombs, Caves & Tunnels"},
	3413013: {Name: "Divine Tower of Caelid: Basement", Area: "Catacombs, Caves & Tunnels"},
	3414011: {Name: "Divine Tower of East Altus", Area: "Catacombs, Caves & Tunnels"},
	3415090: {Name: "Isolated Divine Tower", Area: "Catacombs, Caves & Tunnels"},
	3920000: {Name: "Magma Wyrm Makar", Area: "Catacombs, Caves & Tunnels"},
	3920002: {Name: "Ruin-Strewn Precipice", Area: "Catacombs, Caves & Tunnels"},
	3920003: {Name: "Ruin-Strewn Precipice Overlook", Area: "Catacombs, Caves & Tunnels"},

	// ========================================================
	// Land of Shadow (DLC)
	// ========================================================
	6800000: {Name: "Gravesite Plain", Area: "Land of Shadow", DLC: true},
	6810000: {Name: "Gravesite Plain: Pillar Path", Area: "Land of Shadow", DLC: true},
	6810001: {Name: "Gravesite Plain: Ellac River Cave", Area: "Land of Shadow", DLC: true},
	6810090: {Name: "Gravesite Plain: Ellac River Downstream", Area: "Land of Shadow", DLC: true},
	6820000: {Name: "Castle Ensis: Castle Ensis", Area: "Land of Shadow", DLC: true},
	6820010: {Name: "Castle Ensis: Ensis Moongazing Grounds", Area: "Land of Shadow", DLC: true},
	6830000: {Name: "Cerulean Coast: Cerulean Coast", Area: "Land of Shadow", DLC: true},
	6830002: {Name: "Cerulean Coast: The Fissure", Area: "Land of Shadow", DLC: true},
	6840000: {Name: "Charo's Hidden Grave: Charo's Hidden Grave", Area: "Land of Shadow", DLC: true},
	6841000: {Name: "Foot of the Jagged Peak", Area: "Land of Shadow", DLC: true},
	6850000: {Name: "Jagged Peak: Mountainside", Area: "Land of Shadow", DLC: true},
	6850001: {Name: "Jagged Peak: Summit", Area: "Land of Shadow", DLC: true},
	6850010: {Name: "Jagged Peak: Rest of the Dread Dragon", Area: "Land of Shadow", DLC: true},
	6860000: {Name: "Abyssal Woods: Abyssal Woods", Area: "Land of Shadow", DLC: true},
	6860001: {Name: "Midra's Manse: Manse Hall", Area: "Land of Shadow", DLC: true},
	6860004: {Name: "Midra's Manse: Midra's Library", Area: "Land of Shadow", DLC: true},
	6860010: {Name: "Midra's Manse: Discussion Chamber", Area: "Land of Shadow", DLC: true},
	6900000: {Name: "Scadu Altus", Area: "Land of Shadow", DLC: true},
	6900010: {Name: "Shadow Keep: Main Gate Plaza", Area: "Land of Shadow", DLC: true},
	6901000: {Name: "Rauh Base: Ancient Ruins", Area: "Land of Shadow", DLC: true},
	6902000: {Name: "Scadu Altus: Bonny Village", Area: "Land of Shadow", DLC: true},
	6903000: {Name: "Scadu Altus: Recluses' River Downstream", Area: "Land of Shadow", DLC: true},
	6903090: {Name: "Scadu Altus: Castle Watering Hole", Area: "Land of Shadow", DLC: true},
	6930000: {Name: "Scaduview: Hinterland", Area: "Land of Shadow", DLC: true},
	6940000: {Name: "Ancient Ruins of Rauh: East", Area: "Land of Shadow", DLC: true},
	6941000: {Name: "Ancient Ruins of Rauh: West", Area: "Land of Shadow", DLC: true},
	6941010: {Name: "Ancient Ruins of Rauh: Church of the Bud", Area: "Land of Shadow", DLC: true},

	// ========================================================
	// Land of Shadow — Dungeons (DLC)
	// ========================================================
	2000000: {Name: "Belurat: Theatre of the Divine Beast", Area: "Land of Shadow — Dungeons", DLC: true},
	2000001: {Name: "Belurat: Belurat, Tower Settlement", Area: "Land of Shadow — Dungeons", DLC: true},
	2000002: {Name: "Belurat: Stagefront", Area: "Land of Shadow — Dungeons", DLC: true},
	2001000: {Name: "Enir-Ilim: Gate of Divinity", Area: "Land of Shadow — Dungeons", DLC: true},
	2001001: {Name: "Enir-Ilim: Outer Wall", Area: "Land of Shadow — Dungeons", DLC: true},
	2001004: {Name: "Enir-Ilim: Spiral Rise", Area: "Land of Shadow — Dungeons", DLC: true},
	2001005: {Name: "Enir-Ilim: Cleansing Chamber Anteroom", Area: "Land of Shadow — Dungeons", DLC: true},
	2001007: {Name: "Enir-Ilim: Divine Gate Front Staircase", Area: "Land of Shadow — Dungeons", DLC: true},
	2100010: {Name: "Scaduview: Scadutree Base", Area: "Land of Shadow — Dungeons", DLC: true},
	2100011: {Name: "Shadow Keep: Church District Entrance", Area: "Land of Shadow — Dungeons", DLC: true},
	2100014: {Name: "Shadow Keep: Sunken Chapel", Area: "Land of Shadow — Dungeons", DLC: true},
	2100015: {Name: "Shadow Keep: Tree-Worship Sanctum", Area: "Land of Shadow — Dungeons", DLC: true},
	2101000: {Name: "Storehouse: Messmer's Dark Chamber", Area: "Land of Shadow — Dungeons", DLC: true},
	2101001: {Name: "Storehouse: First Floor", Area: "Land of Shadow — Dungeons", DLC: true},
	2101003: {Name: "Storehouse: Fourth Floor", Area: "Land of Shadow — Dungeons", DLC: true},
	2101004: {Name: "Storehouse: Seventh Floor", Area: "Land of Shadow — Dungeons", DLC: true},
	2101006: {Name: "Storehouse: Dark Chamber Entrance", Area: "Land of Shadow — Dungeons", DLC: true},
	2101010: {Name: "Scaduview: Scaduview", Area: "Land of Shadow — Dungeons", DLC: true},
	2101011: {Name: "Storehouse: Back Section", Area: "Land of Shadow — Dungeons", DLC: true},
	2101012: {Name: "Storehouse: Loft", Area: "Land of Shadow — Dungeons", DLC: true},
	2101013: {Name: "Scaduview: Shadow Keep, Back Gate", Area: "Land of Shadow — Dungeons", DLC: true},
	2102001: {Name: "Storehouse: West Rampart", Area: "Land of Shadow — Dungeons", DLC: true},
	2200000: {Name: "Stone Coffin Fissure: Garden of Deep Purple", Area: "Land of Shadow — Dungeons", DLC: true},
	2200001: {Name: "Stone Coffin Fissure: Stone Coffin Fissure", Area: "Land of Shadow — Dungeons", DLC: true},
	2200002: {Name: "Stone Coffin Fissure: Fissure Cross", Area: "Land of Shadow — Dungeons", DLC: true},
	2200004: {Name: "Stone Coffin Fissure: Fissure Waypoint & Depths", Area: "Land of Shadow — Dungeons", DLC: true},
	2500000: {Name: "Scadu Altus: Finger Birthing Grounds", Area: "Land of Shadow — Dungeons", DLC: true},
	4000001: {Name: "Gravesite Plain: Fog Rift Catacombs", Area: "Land of Shadow — Dungeons", DLC: true},
	4001001: {Name: "Rauh Base: Scorpion River Catacombs", Area: "Land of Shadow — Dungeons", DLC: true},
	4002000: {Name: "Abyssal Woods: Forsaken Graveyard", Area: "Land of Shadow — Dungeons", DLC: true},
	4002001: {Name: "Scadu Altus: Darklight Catacombs", Area: "Land of Shadow — Dungeons", DLC: true},
	4100001: {Name: "Gravesite Plain: Belurat Gaol", Area: "Land of Shadow — Dungeons", DLC: true},
	4101001: {Name: "Scadu Altus: Bonny Gaol", Area: "Land of Shadow — Dungeons", DLC: true},
	4102001: {Name: "Charo's Hidden Grave: Lamenter's Gaol", Area: "Land of Shadow — Dungeons", DLC: true},
	4200090: {Name: "Gravesite Plain: Ruined Forge Lava Intake", Area: "Land of Shadow — Dungeons", DLC: true},
	4202090: {Name: "Scadu Altus: Ruined Forge of Starfall Past", Area: "Land of Shadow — Dungeons", DLC: true},
	4203090: {Name: "Rauh Base: Taylew's Ruined Forge", Area: "Land of Shadow — Dungeons", DLC: true},
	4300001: {Name: "Gravesite Plain: Rivermouth Cave", Area: "Land of Shadow — Dungeons", DLC: true},
	4301090: {Name: "Gravesite Plain: Dragon's Pit Terminus", Area: "Land of Shadow — Dungeons", DLC: true},
}

// dlcRegionIDs is the authoritative set of Shadow of the Erdtree region IDs,
// derived from the DLC column of the data table above. The DLC ID space is
// non-contiguous (2xxxxxx legacy, 4xxxxxx minor dungeons, 68xxxxx/69xxxxx
// overworld), so a simple numeric range cannot identify DLC regions.
func IsDLCRegion(id uint32) bool {
	r, ok := Regions[id]
	return ok && r.DLC
}
