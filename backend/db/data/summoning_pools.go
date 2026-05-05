package data

// SummoningPoolData holds the static definition of a summoning pool (Martyr Effigy).
type SummoningPoolData struct {
	Name   string
	Region string
}

// SummoningPools maps event flag IDs to summoning pool definitions.
// These flags track whether the player has activated a Martyr Effigy.
// Large flag IDs (10000000+) require lookup table entries in event_flags.go
// because the standard formula (byte = id/8) does not produce correct offsets.
// Source: ClayAmore/ER-Save-Editor summoning_pools.rs, Fextralife wiki
var SummoningPools = map[uint32]SummoningPoolData{
	// ============================================================
	// BASE GAME — Legacy Dungeons
	// ============================================================

	// Stormveil Castle
	10000040: {Name: "Stormveil Main Gate", Region: "Stormveil Castle"},
	10000041: {Name: "Stormveil Cliffside", Region: "Stormveil Castle"},
	10000042: {Name: "Rampart Tower", Region: "Stormveil Castle"},
	10000043: {Name: "Liftside Chamber", Region: "Stormveil Castle"},
	10000044: {Name: "Secluded Cell", Region: "Stormveil Castle"},
	10000045: {Name: "Gateside Chamber", Region: "Stormveil Castle"},

	// Leyndell, Royal Capital
	11000040: {Name: "East Capital Rampart", Region: "Leyndell, Royal Capital"},
	11000041: {Name: "Avenue Balcony", Region: "Leyndell, Royal Capital"},
	11000042: {Name: "Lower Capital Church", Region: "Leyndell, Royal Capital"},
	11000043: {Name: "West Capital Rampart", Region: "Leyndell, Royal Capital"},
	11000044: {Name: "Queen's Bedchamber", Region: "Leyndell, Royal Capital"},

	// Leyndell, Ashen Capital
	11050040: {Name: "Queen's Bedchamber", Region: "Leyndell, Ashen Capital"},
	11050041: {Name: "Erdtree Sanctuary", Region: "Leyndell, Ashen Capital"},

	// Ainsel River
	12010040: {Name: "Ainsel River Well Depths", Region: "Ainsel River"},
	12010041: {Name: "Ainsel River Sluice Gate", Region: "Ainsel River"},
	12010042: {Name: "Ainsel River Downstream", Region: "Ainsel River"},
	12010043: {Name: "Ainsel River Main", Region: "Ainsel River"},
	12010044: {Name: "Nokstella, Eternal City", Region: "Ainsel River"},
	12010045: {Name: "Lake of Rot Shoreside", Region: "Ainsel River"},
	12010046: {Name: "Grand Cloister", Region: "Ainsel River"},

	// Nokron / Siofra River
	12020040: {Name: "Nokron, Eternal City", Region: "Siofra River"},
	12020041: {Name: "Mimic Tear", Region: "Siofra River"},
	12020042: {Name: "Ancestral Woods", Region: "Siofra River"},
	12020043: {Name: "Aqueduct-Facing Cliffs", Region: "Siofra River"},
	12020044: {Name: "Night's Sacred Ground", Region: "Siofra River"},
	12020045: {Name: "Siofra River Well Depths", Region: "Siofra River"},
	12020046: {Name: "Siofra River Bank", Region: "Siofra River"},

	// Deeproot Depths
	12030040: {Name: "Root-Facing Cliffs", Region: "Deeproot Depths"},
	12030041: {Name: "Great Waterfall Crest", Region: "Deeproot Depths"},
	12030042: {Name: "Deeproot Depths", Region: "Deeproot Depths"},
	12030043: {Name: "Across the Roots", Region: "Deeproot Depths"},
	12030044: {Name: "The Nameless Eternal City", Region: "Deeproot Depths"},

	// Mohgwyn Palace
	12050040: {Name: "Palace Approach Ledge-Road", Region: "Mohgwyn Palace"},
	12050041: {Name: "Dynasty Mausoleum Entrance", Region: "Mohgwyn Palace"},
	12050042: {Name: "Dynasty Mausoleum Midpoint", Region: "Mohgwyn Palace"},

	// Siofra River (lower)
	12070040: {Name: "Below the Well", Region: "Siofra River"},
	12070041: {Name: "Siofra River Bank", Region: "Siofra River"},

	// Crumbling Farum Azula
	13000040: {Name: "Crumbling Beast Grave", Region: "Crumbling Farum Azula"},
	13000041: {Name: "Crumbling Beast Grave Depths", Region: "Crumbling Farum Azula"},
	13000042: {Name: "Tempest-Facing Balcony", Region: "Crumbling Farum Azula"},
	13000043: {Name: "Dragon Temple", Region: "Crumbling Farum Azula"},
	13000044: {Name: "Dragon Temple Altar", Region: "Crumbling Farum Azula"},
	13000045: {Name: "Dragon Temple Lift", Region: "Crumbling Farum Azula"},
	13000046: {Name: "Beside the Great Bridge", Region: "Crumbling Farum Azula"},
	13000047: {Name: "Dragon Temple Rooftop", Region: "Crumbling Farum Azula"},

	// Academy of Raya Lucaria
	14000040: {Name: "Main Academy Gate", Region: "Academy of Raya Lucaria"},
	14000041: {Name: "Schoolhouse Classroom", Region: "Academy of Raya Lucaria"},
	14000042: {Name: "Church of the Cuckoo", Region: "Academy of Raya Lucaria"},
	14000043: {Name: "Debate Parlor", Region: "Academy of Raya Lucaria"},

	// Miquella's Haligtree
	15000040: {Name: "Haligtree Canopy", Region: "Miquella's Haligtree"},
	15000041: {Name: "Haligtree Town", Region: "Miquella's Haligtree"},
	15000042: {Name: "Haligtree Town Plaza", Region: "Miquella's Haligtree"},
	15000044: {Name: "Haligtree Promenade", Region: "Miquella's Haligtree"},

	// Elphael, Brace of the Haligtree
	15000045: {Name: "Prayer Room", Region: "Elphael, Brace of the Haligtree"},
	15000046: {Name: "Elphael Inner Wall", Region: "Elphael, Brace of the Haligtree"},
	15000047: {Name: "Drainage Channel", Region: "Elphael, Brace of the Haligtree"},
	15000049: {Name: "Haligtree Roots", Region: "Elphael, Brace of the Haligtree"},

	// Volcano Manor
	16000040: {Name: "Prison Town Church", Region: "Volcano Manor"},
	16000041: {Name: "Guest Hall", Region: "Volcano Manor"},
	16000042: {Name: "Temple of Eiglay", Region: "Volcano Manor"},
	16000043: {Name: "Audience Pathway", Region: "Volcano Manor"},
	16000044: {Name: "Subterranean Inquisition Chamber", Region: "Volcano Manor"},

	// Elden Throne
	19000040: {Name: "Elden Throne", Region: "Elden Throne"},

	// ============================================================
	// BASE GAME — Catacombs
	// ============================================================
	30000040: {Name: "Tombsward Catacombs", Region: "Weeping Peninsula"},
	30010040: {Name: "Impaler's Catacombs", Region: "Weeping Peninsula"},
	30020040: {Name: "Stormfoot Catacombs", Region: "Limgrave"},
	30030040: {Name: "Road's End Catacombs", Region: "Liurnia"},
	30040040: {Name: "Murkwater Catacombs", Region: "Limgrave"},
	30050040: {Name: "Black Knife Catacombs", Region: "Liurnia"},
	30060040: {Name: "Cliffbottom Catacombs", Region: "Liurnia"},
	30070040: {Name: "Wyndham Catacombs", Region: "Mt. Gelmir"},
	30080040: {Name: "Sainted Hero's Grave", Region: "Altus Plateau"},
	30090040: {Name: "Gelmir Hero's Grave", Region: "Mt. Gelmir"},
	30100040: {Name: "Auriza Hero's Grave", Region: "Capital Outskirts"},
	30110040: {Name: "Deathtouched Catacombs", Region: "Stormhill"},
	30120040: {Name: "Unsightly Catacombs", Region: "Altus Plateau"},
	30140040: {Name: "Minor Erdtree Catacombs", Region: "Caelid"},
	30150040: {Name: "Caelid Catacombs", Region: "Caelid"},
	30160040: {Name: "War-Dead Catacombs", Region: "Caelid"},
	30170040: {Name: "Giant-Conquering Hero's Grave", Region: "Mountaintops of the Giants"},
	30180040: {Name: "Giants' Mountaintop Catacombs", Region: "Mountaintops of the Giants"},
	30190040: {Name: "Consecrated Snowfield Catacombs", Region: "Consecrated Snowfield"},
	30200040: {Name: "Hidden Path to the Haligtree", Region: "Forbidden Lands"},

	// ============================================================
	// BASE GAME — Caves
	// ============================================================
	31000040: {Name: "Murkwater Cave", Region: "Limgrave"},
	31010040: {Name: "Earthbore Cave", Region: "Weeping Peninsula"},
	31020040: {Name: "Tombsward Cave", Region: "Weeping Peninsula"},
	31030040: {Name: "Groveside Cave", Region: "Limgrave"},
	31040040: {Name: "Stillwater Cave", Region: "Liurnia"},
	31050040: {Name: "Lakeside Crystal Cave", Region: "Liurnia"},
	31060040: {Name: "Academy Crystal Cave", Region: "Liurnia"},
	31070040: {Name: "Seethewater Cave", Region: "Mt. Gelmir"},
	31090040: {Name: "Volcano Cave", Region: "Mt. Gelmir"},
	31100040: {Name: "Dragonbarrow Cave", Region: "Dragonbarrow"},
	31110040: {Name: "Sellia Hideaway", Region: "Dragonbarrow"},
	31120040: {Name: "Cave of the Forlorn", Region: "Consecrated Snowfield"},
	31150040: {Name: "Coastal Cave", Region: "Limgrave"},
	31170040: {Name: "Highroad Cave", Region: "Limgrave"},
	31180040: {Name: "Perfumer's Grotto", Region: "Altus Plateau"},
	31190040: {Name: "Sage's Cave", Region: "Altus Plateau"},
	31200040: {Name: "Abandoned Cave", Region: "Caelid"},
	31210040: {Name: "Gaol Cave", Region: "Caelid"},
	31220040: {Name: "Spiritcaller's Cave", Region: "Mountaintops of the Giants"},

	// ============================================================
	// BASE GAME — Tunnels
	// ============================================================
	32000040: {Name: "Morne Tunnel", Region: "Weeping Peninsula"},
	32010040: {Name: "Limgrave Tunnels", Region: "Limgrave"},
	32020040: {Name: "Raya Lucaria Crystal Tunnel", Region: "Liurnia"},
	32040040: {Name: "Old Altus Tunnel", Region: "Altus Plateau"},
	32050040: {Name: "Altus Tunnel", Region: "Altus Plateau"},
	32070040: {Name: "Gael Tunnel", Region: "Caelid"},
	32080040: {Name: "Sellia Crystal Tunnel", Region: "Caelid"},
	32110040: {Name: "Yelough Anix Tunnel", Region: "Consecrated Snowfield"},

	// ============================================================
	// BASE GAME — Divine Towers & Sealed Tunnel
	// ============================================================
	34100040: {Name: "Divine Tower of Limgrave", Region: "Limgrave"},
	34110040: {Name: "Divine Tower of Liurnia", Region: "Liurnia"},
	34120040: {Name: "Sealed Tunnel", Region: "Capital Outskirts"},
	34120041: {Name: "Divine Tower of West Altus", Region: "Capital Outskirts"},
	34130040: {Name: "Divine Tower of Caelid", Region: "Caelid"},

	// ============================================================
	// BASE GAME — Subterranean Shunning-Grounds
	// ============================================================
	35000040: {Name: "Underground Roadside", Region: "Subterranean Shunning-Grounds"},
	35000041: {Name: "Forsaken Depths", Region: "Subterranean Shunning-Grounds"},
	35000042: {Name: "Leyndell Catacombs", Region: "Subterranean Shunning-Grounds"},

	// ============================================================
	// BASE GAME — Ruin-Strewn Precipice
	// ============================================================
	39200040: {Name: "Ruin-Strewn Precipice", Region: "Ruin-Strewn Precipice"},
	39200041: {Name: "Ruin-Strewn Precipice Overlook", Region: "Ruin-Strewn Precipice"},

	// ============================================================
	// BASE GAME — Open World
	// ============================================================

	// Limgrave / Stormhill / Weeping Peninsula
	1035530040: {Name: "The First Step", Region: "Limgrave"},
	1036520040: {Name: "Witchbane Ruins", Region: "Weeping Peninsula"},
	1036540040: {Name: "Agheel Lake", Region: "Limgrave"},
	1036540041: {Name: "Dragon-Burnt Ruins", Region: "Limgrave"},
	1037530040: {Name: "Waypoint Ruins", Region: "Limgrave"},
	1038520040: {Name: "Peninsula Minor Erdtree", Region: "Weeping Peninsula"},
	1039540040: {Name: "Castleward Tunnel", Region: "Stormhill"},

	// Liurnia of the Lakes
	1040530040: {Name: "Lake Minor Erdtree", Region: "Liurnia"},
	1042540040: {Name: "Temple Quarter", Region: "Liurnia"},
	1044530040: {Name: "Village of the Albinaurics", Region: "Liurnia"},
	1045520040: {Name: "Kingsrealm Ruins", Region: "Liurnia"},

	// Altus Plateau
	1048370040: {Name: "Wyndham Ruins", Region: "Altus Plateau"},
	1049380040: {Name: "The Shaded Castle", Region: "Altus Plateau"},
	1049380041: {Name: "Writheblood Ruins", Region: "Altus Plateau"},

	// Caelid
	1046400040: {Name: "Caelid Minor Erdtree", Region: "Caelid"},
	1047400040: {Name: "Caelem Ruins", Region: "Caelid"},
	1050400040: {Name: "Caelid Highway South", Region: "Caelid"},
	1051400040: {Name: "Sellia, Town of Sorcery", Region: "Caelid"},
	1052410040: {Name: "Redmane Castle", Region: "Caelid"},

	// Mt. Gelmir
	1051360040: {Name: "Fort Laiedd", Region: "Mt. Gelmir"},
	1051370040: {Name: "Hermit Village", Region: "Mt. Gelmir"},

	// Mountaintops / Snowfield / Dragonbarrow
	1047510840: {Name: "Freezing Lake", Region: "Mountaintops of the Giants"},
	1049560840: {Name: "Freezing River", Region: "Consecrated Snowfield"},
	1049570840: {Name: "Snowfield Minor Erdtree", Region: "Consecrated Snowfield"},
	1051570840: {Name: "Foot of the Forge", Region: "Consecrated Snowfield"},
	1051570841: {Name: "Consecrated Snowfield", Region: "Consecrated Snowfield"},
	1052530840: {Name: "Greyoll's Dragonbarrow", Region: "Dragonbarrow"},
	1052570840: {Name: "Farum Greatbridge", Region: "Dragonbarrow"},
	1053570840: {Name: "Dragonbarrow Minor Erdtree", Region: "Dragonbarrow"},

	// ============================================================
	// SHADOW OF THE ERDTREE — DLC
	// ============================================================
	1060330040: {Name: "Rauh Base, Bear Woods", Region: "Rauh Base"},
	1060340040: {Name: "Ancient Ruins of Rauh", Region: "Rauh Ruins"},
	1060340041: {Name: "Ancient Ruins, Under-Stair", Region: "Rauh Ruins"},
	1060340043: {Name: "Church of the Bud", Region: "Rauh Ruins"},
	1060350040: {Name: "Cerulean Coast", Region: "Cerulean Coast"},
	1060380040: {Name: "Western Nameless Mausoleum", Region: "Gravesite Plain"},
	1060410040: {Name: "Castle Front", Region: "Gravesite Plain"},
	1060420040: {Name: "Lake North of the Greatbridge", Region: "Gravesite Plain"},
	1060430040: {Name: "Moorth Highway", Region: "Scadu Altus"},
	1060430041: {Name: "Moorth Highway South", Region: "Scadu Altus"},
	1060430042: {Name: "Eastern Nameless Mausoleum", Region: "Scadu Altus"},
	1060430043: {Name: "Fog Rift Fort", Region: "Scadu Altus"},
	1060440040: {Name: "Altus, Bear Woods", Region: "Scadu Altus"},
}

// Colosseums maps event flag IDs to colosseum definitions.
// Setting flag to 1 unlocks the corresponding colosseum.
// Source: The-Grand-Archives/Elden-Ring-CT-TGA
var Colosseums = map[uint32]SummoningPoolData{
	60350: {Name: "Caelid Colosseum", Region: "Caelid"},
	60360: {Name: "Limgrave Colosseum", Region: "Limgrave"},
	60370: {Name: "Royal Colosseum", Region: "Leyndell"},
}

// ColosseumFlagSet sets the matchmaking + map-marker flags for a colosseum.
// These make the arena appear on the world map and enable the fight menu.
// They DO NOT open the physical gate — that state is stored outside event
// flags (in WorldGeom binary blob) and cannot be edited from a save editor.
// Player must open the gate once in-game; thereafter the open state persists.
type ColosseumFlagSet struct {
	Activate uint32 // 60xxx — primary unlock, enables matchmaking
	MapPOI   uint32 // 62xxx — colosseum icon on world map
	NPC      uint32 // 69xxx — NPC/event-memory marker
	Gate     uint32 // 710xxx — matchmaking gate marker
}

// ColosseumFlagSets keyed by the Activate flag ID. Δ=10 stride verified
// against Tester slot 2 (legit Limgrave-only) at tmp/coloseum-debug/.
var ColosseumFlagSets = map[uint32]ColosseumFlagSet{
	60350: {Activate: 60350, MapPOI: 62720, NPC: 69450, Gate: 710850}, // Caelid
	60360: {Activate: 60360, MapPOI: 62730, NPC: 69460, Gate: 710860}, // Limgrave
	60370: {Activate: 60370, MapPOI: 62740, NPC: 69470, Gate: 710870}, // Royal
}

// ColosseumGlobalFlags fire when any colosseum is unlocked. Verified set
// in Tester slot 2 (after.sl2) after legit Limgrave-only unlock.
var ColosseumGlobalFlags = []uint32{
	6080,  // gameman — any colosseum unlocked
	60100, // event/map system global
	69480, // block 69 global
}

// AllFlags returns every per-colosseum flag in a stable order.
func (c ColosseumFlagSet) AllFlags() []uint32 {
	return []uint32{c.Activate, c.MapPOI, c.NPC, c.Gate}
}
