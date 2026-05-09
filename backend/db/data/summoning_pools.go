package data

// SummoningPoolData holds the static definition of a summoning pool (Martyr Effigy).
type SummoningPoolData struct {
	Name   string
	Region string
}

// SummoningPools maps event flag IDs to summoning pool definitions.
// These flags track whether the player has activated a Martyr Effigy.
// All IDs use BST block 670 (position 107 in eventflag_bst.txt) — standard lookup applies.
// Source: Elden-Ring-CT-TGA "Unlock all Summoning Pools.cea" — active block (game >= v1.12 / v2.02.0)
// Pre-v1.12 IDs (10000040, 1035530040, etc.) are ignored by the current game engine.
var SummoningPools = map[uint32]SummoningPoolData{
	// ============================================================
	// BASE GAME — Legacy Dungeons
	// ============================================================

	// Stormveil Castle
	670130: {Name: "Gateside Chamber", Region: "Stormveil Castle"},
	670131: {Name: "Liftside Chamber", Region: "Stormveil Castle"},
	670132: {Name: "Stormveil Cliffside", Region: "Stormveil Castle"},
	670133: {Name: "Rampart Tower", Region: "Stormveil Castle"},
	670134: {Name: "Secluded Cell", Region: "Stormveil Castle"},
	670135: {Name: "Stormveil Main Gate", Region: "Stormveil Castle"},

	// Leyndell, Royal Capital
	670330: {Name: "East Capital Rampart", Region: "Leyndell, Royal Capital"},
	670331: {Name: "West Capital Rampart", Region: "Leyndell, Royal Capital"},
	670332: {Name: "Avenue Balcony", Region: "Leyndell, Royal Capital"},
	670333: {Name: "Lower Capital Church", Region: "Leyndell, Royal Capital"},
	670334: {Name: "Queen's Bedchamber", Region: "Leyndell, Royal Capital"},

	// Leyndell, Ashen Capital
	670730: {Name: "Erdtree Sanctuary", Region: "Leyndell, Ashen Capital"},
	670731: {Name: "Queen's Bedchamber", Region: "Leyndell, Ashen Capital"},

	// Ainsel River / Lake of Rot
	670610: {Name: "Ainsel River Well Depths", Region: "Ainsel River"},
	670611: {Name: "Ainsel River Sluice Gate", Region: "Ainsel River"},
	670612: {Name: "Ainsel River Downstream", Region: "Ainsel River"},
	670613: {Name: "Ainsel River Main", Region: "Ainsel River"},
	670614: {Name: "Nokstella, Eternal City", Region: "Ainsel River"},
	670615: {Name: "Lake of Rot Shoreside", Region: "Ainsel River"},
	670616: {Name: "Grand Cloister", Region: "Ainsel River"},

	// Siofra River / Nokron
	670620: {Name: "Siofra River Bank", Region: "Siofra River"},
	670621: {Name: "Siofra River Well Depths", Region: "Siofra River"},
	670622: {Name: "Ancestral Woods", Region: "Siofra River"},
	670623: {Name: "Aqueduct-Facing Cliffs", Region: "Siofra River"},
	670624: {Name: "Night's Sacred Ground", Region: "Siofra River"},
	670625: {Name: "Nokron, Eternal City", Region: "Siofra River"},
	670626: {Name: "Mimic Tear", Region: "Siofra River"},

	// Deeproot Depths
	670630: {Name: "Deeproot Depths", Region: "Deeproot Depths"},
	670631: {Name: "Root-Facing Cliffs", Region: "Deeproot Depths"},
	670632: {Name: "Great Waterfall Crest", Region: "Deeproot Depths"},
	670633: {Name: "Across the Roots", Region: "Deeproot Depths"},
	670634: {Name: "The Nameless Eternal City", Region: "Deeproot Depths"},

	// Mohgwyn Palace
	670650: {Name: "Palace Approach Ledge-Road", Region: "Mohgwyn Palace"},
	670651: {Name: "Dynasty Mausoleum Entrance", Region: "Mohgwyn Palace"},
	670652: {Name: "Dynasty Mausoleum Midpoint", Region: "Mohgwyn Palace"},

	// Nokron — Start
	670670: {Name: "Siofra River Bank", Region: "Siofra River"},
	670671: {Name: "Below the Well", Region: "Siofra River"},

	// Crumbling Farum Azula
	670740: {Name: "Crumbling Beast Grave", Region: "Crumbling Farum Azula"},
	670741: {Name: "Crumbling Beast Grave Depths", Region: "Crumbling Farum Azula"},
	670742: {Name: "Tempest-Facing Balcony", Region: "Crumbling Farum Azula"},
	670743: {Name: "Dragon Temple", Region: "Crumbling Farum Azula"},
	670744: {Name: "Dragon Temple Altar", Region: "Crumbling Farum Azula"},
	670745: {Name: "Dragon Temple Lift", Region: "Crumbling Farum Azula"},
	670746: {Name: "Beside the Great Bridge", Region: "Crumbling Farum Azula"},
	670747: {Name: "Dragon Temple Rooftop", Region: "Crumbling Farum Azula"},

	// Academy of Raya Lucaria
	670231: {Name: "Main Academy Gate", Region: "Academy of Raya Lucaria"},
	670232: {Name: "Schoolhouse Classroom", Region: "Academy of Raya Lucaria"},
	670233: {Name: "Debate Parlor", Region: "Academy of Raya Lucaria"},

	// Miquella's Haligtree
	670530: {Name: "Haligtree Canopy", Region: "Miquella's Haligtree"},
	670531: {Name: "Haligtree Town", Region: "Miquella's Haligtree"},
	670532: {Name: "Haligtree Town Plaza", Region: "Miquella's Haligtree"},
	670534: {Name: "Haligtree Promenade", Region: "Miquella's Haligtree"},
	670535: {Name: "Prayer Room", Region: "Elphael, Brace of the Haligtree"},
	670536: {Name: "Elphael Inner Wall", Region: "Elphael, Brace of the Haligtree"},
	670537: {Name: "Drainage Channel", Region: "Elphael, Brace of the Haligtree"},
	670539: {Name: "Haligtree Roots", Region: "Elphael, Brace of the Haligtree"},

	// Volcano Manor
	670351: {Name: "Guest Hall", Region: "Volcano Manor"},
	670352: {Name: "Prison Town Church", Region: "Volcano Manor"},
	670353: {Name: "Temple of Eiglay", Region: "Volcano Manor"},
	670354: {Name: "Audience Pathway", Region: "Volcano Manor"},

	// Stone Platform (Elden Throne)
	670750: {Name: "Elden Throne", Region: "Elden Throne"},

	// ============================================================
	// BASE GAME — Catacombs
	// ============================================================
	670160: {Name: "Tombsward Catacombs", Region: "Weeping Peninsula"},
	670161: {Name: "Impaler's Catacombs", Region: "Weeping Peninsula"},
	670162: {Name: "Stormfoot Catacombs", Region: "Limgrave"},
	670163: {Name: "Deathtouched Catacombs", Region: "Stormhill"},
	670164: {Name: "Murkwater Catacombs", Region: "Limgrave"},
	670260: {Name: "Black Knife Catacombs", Region: "Liurnia"},
	670261: {Name: "Road's End Catacombs", Region: "Liurnia"},
	670262: {Name: "Cliffbottom Catacombs", Region: "Liurnia"},
	670360: {Name: "Sainted Hero's Grave", Region: "Altus Plateau"},
	670361: {Name: "Gelmir Hero's Grave", Region: "Mt. Gelmir"},
	670362: {Name: "Auriza Hero's Grave", Region: "Capital Outskirts"},
	670363: {Name: "Unsightly Catacombs", Region: "Altus Plateau"},
	670364: {Name: "Wyndham Catacombs", Region: "Mt. Gelmir"},
	670460: {Name: "Minor Erdtree Catacombs", Region: "Caelid"},
	670461: {Name: "Caelid Catacombs", Region: "Caelid"},
	670462: {Name: "War-Dead Catacombs", Region: "Caelid"},
	670560: {Name: "Giant-Conquering Hero's Grave", Region: "Mountaintops of the Giants"},
	670561: {Name: "Giants' Mountaintop Catacombs", Region: "Mountaintops of the Giants"},
	670562: {Name: "Consecrated Snowfield Catacombs", Region: "Consecrated Snowfield"},
	670563: {Name: "Hidden Path to the Haligtree", Region: "Forbidden Lands"},

	// ============================================================
	// BASE GAME — Caves
	// ============================================================
	670170: {Name: "Tombsward Cave", Region: "Weeping Peninsula"},
	670171: {Name: "Earthbore Cave", Region: "Weeping Peninsula"},
	670172: {Name: "Murkwater Cave", Region: "Limgrave"},
	670173: {Name: "Groveside Cave", Region: "Limgrave"},
	670174: {Name: "Coastal Cave", Region: "Limgrave"},
	670175: {Name: "Highroad Cave", Region: "Limgrave"},
	670270: {Name: "Stillwater Cave", Region: "Liurnia"},
	670271: {Name: "Lakeside Crystal Cave", Region: "Liurnia"},
	670272: {Name: "Academy Crystal Cave", Region: "Liurnia"},
	670370: {Name: "Seethewater Cave", Region: "Mt. Gelmir"},
	670371: {Name: "Volcano Cave", Region: "Mt. Gelmir"},
	670372: {Name: "Perfumer's Grotto", Region: "Altus Plateau"},
	670373: {Name: "Sage's Cave", Region: "Altus Plateau"},
	670470: {Name: "Gaol Cave", Region: "Caelid"},
	670471: {Name: "Dragonbarrow Cave", Region: "Dragonbarrow"},
	670472: {Name: "Abandoned Cave", Region: "Caelid"},
	670473: {Name: "Sellia Hideaway", Region: "Dragonbarrow"},
	670570: {Name: "Cave of the Forlorn", Region: "Consecrated Snowfield"},
	670571: {Name: "Spiritcaller's Cave", Region: "Mountaintops of the Giants"},

	// ============================================================
	// BASE GAME — Tunnels
	// ============================================================
	670180: {Name: "Morne Tunnel", Region: "Weeping Peninsula"},
	670181: {Name: "Limgrave Tunnels", Region: "Limgrave"},
	670280: {Name: "Raya Lucaria Crystal Tunnel", Region: "Liurnia"},
	670380: {Name: "Old Altus Tunnel", Region: "Altus Plateau"},
	670381: {Name: "Altus Tunnel", Region: "Altus Plateau"},
	670480: {Name: "Gael Tunnel", Region: "Caelid"},
	670481: {Name: "Sellia Crystal Tunnel", Region: "Caelid"},
	670580: {Name: "Yelough Anix Tunnel", Region: "Consecrated Snowfield"},

	// ============================================================
	// BASE GAME — Divine Towers
	// ============================================================
	670390: {Name: "Divine Tower of West Altus", Region: "Capital Outskirts"},
	670490: {Name: "Divine Tower of Caelid", Region: "Caelid"},

	// ============================================================
	// BASE GAME — Subterranean Shunning-Grounds
	// ============================================================
	670340: {Name: "Underground Roadside", Region: "Subterranean Shunning-Grounds"},
	670341: {Name: "Forsaken Depths", Region: "Subterranean Shunning-Grounds"},
	670342: {Name: "Leyndell Catacombs", Region: "Subterranean Shunning-Grounds"},

	// ============================================================
	// BASE GAME — Ruin-Strewn Precipice
	// ============================================================
	670240: {Name: "Ruin-Strewn Precipice", Region: "Ruin-Strewn Precipice"},
	670241: {Name: "Ruin-Strewn Precipice Overlook", Region: "Ruin-Strewn Precipice"},

	// ============================================================
	// BASE GAME — Open World (Liurnia of the Lakes)
	// ============================================================
	670200: {Name: "Lake Minor Erdtree", Region: "Liurnia"},
	670201: {Name: "Temple Quarter", Region: "Liurnia"},
	670202: {Name: "Village of the Albinaurics", Region: "Liurnia"},
	670203: {Name: "Kingsrealm Ruins", Region: "Liurnia"},
	670204: {Name: "Main Caria Manor Gate", Region: "Liurnia"},
	670205: {Name: "Frenzied Flame Village Outskirts", Region: "Liurnia"},
	670230: {Name: "Schoolhouse Classroom (Overworld)", Region: "Liurnia"},

	// ============================================================
	// BASE GAME — Open World (Mt. Gelmir)
	// ============================================================
	670300: {Name: "Seethewater Terminus", Region: "Mt. Gelmir"},
	670301: {Name: "Hermit Village", Region: "Mt. Gelmir"},
	670303: {Name: "Road of Iniquity", Region: "Mt. Gelmir"},
	670304: {Name: "Craftsman's Shack", Region: "Mt. Gelmir"},

	// ============================================================
	// BASE GAME — Open World (Altus Plateau)
	// ============================================================
	670305: {Name: "Wyndham Ruins", Region: "Altus Plateau"},
	670306: {Name: "The Shaded Castle", Region: "Altus Plateau"},
	670307: {Name: "Writheblood Ruins", Region: "Altus Plateau"},
	670308: {Name: "Windmill Heights", Region: "Altus Plateau"},
	670309: {Name: "Capital Outskirts Minor Erdtree", Region: "Capital Outskirts"},
	670310: {Name: "Capital Rampart", Region: "Capital Outskirts"},

	// ============================================================
	// BASE GAME — Open World (Limgrave / Weeping Peninsula)
	// ============================================================
	670100: {Name: "Witchbane Ruins", Region: "Weeping Peninsula"},
	670101: {Name: "Church of Elleh", Region: "Limgrave"},
	670102: {Name: "Agheel Lake North", Region: "Limgrave"},
	670103: {Name: "Dragon-Burnt Ruins", Region: "Limgrave"},
	670104: {Name: "Peninsula Minor Erdtree", Region: "Weeping Peninsula"},
	670105: {Name: "Castle Morne", Region: "Weeping Peninsula"},
	670106: {Name: "Waypoint Ruins", Region: "Limgrave"},

	// ============================================================
	// BASE GAME — Open World (Caelid / Dragonbarrow)
	// ============================================================
	670400: {Name: "Rotview Balcony", Region: "Caelid"},
	670401: {Name: "Caelem Ruins", Region: "Caelid"},
	670402: {Name: "Caelid Highway South", Region: "Caelid"},
	670403: {Name: "Swamp of Aeonia (West)", Region: "Caelid"},
	670404: {Name: "Swamp of Aeonia (East)", Region: "Caelid"},
	670405: {Name: "Dragonbarrow Fork", Region: "Dragonbarrow"},
	670406: {Name: "Redmane Castle", Region: "Caelid"},
	670407: {Name: "Radahn Festival Arena", Region: "Caelid"},
	670408: {Name: "Dragonbarrow Minor Erdtree", Region: "Dragonbarrow"},
	670409: {Name: "Farum Greatbridge", Region: "Dragonbarrow"},

	// ============================================================
	// BASE GAME — Open World (Mountaintops / Snowfield)
	// ============================================================
	670500: {Name: "Forbidden Lands", Region: "Mountaintops of the Giants"},
	670501: {Name: "Freezing Lake", Region: "Mountaintops of the Giants"},
	670502: {Name: "Foot of the Forge", Region: "Mountaintops of the Giants"},
	670503: {Name: "Heretical Rise", Region: "Mountaintops of the Giants"},
	670504: {Name: "Snow Valley Ruins Overlook", Region: "Mountaintops of the Giants"},
	670505: {Name: "Castle Sol", Region: "Mountaintops of the Giants"},
	670506: {Name: "Ordina, Liturgical Town (South)", Region: "Consecrated Snowfield"},
	670507: {Name: "Ordina, Liturgical Town (East)", Region: "Consecrated Snowfield"},

	// ============================================================
	// SHADOW OF THE ERDTREE — Belurat / Enir-Ilim
	// ============================================================
	670841: {Name: "Belurat, Tower Settlement", Region: "Land of Shadow"},
	670842: {Name: "Belurat Gaolside", Region: "Land of Shadow"},
	670843: {Name: "Small Private Altar", Region: "Land of Shadow"},
	670850: {Name: "Enir-Ilim: Outer Wall", Region: "Land of Shadow"},
	670851: {Name: "Enir-Ilim: Second Floor", Region: "Land of Shadow"},
	670852: {Name: "Spiral Rise", Region: "Land of Shadow"},
	670853: {Name: "Gate of Divinity", Region: "Land of Shadow"},
	670854: {Name: "Cleansing Chamber Anteroom", Region: "Land of Shadow"},

	// SHADOW OF THE ERDTREE — Shadow Keep / Specimen Storehouse
	670909: {Name: "Shadow Keep: Church District", Region: "Land of Shadow"},
	670940: {Name: "Shadow Keep: Main Gate Plaza", Region: "Land of Shadow"},
	670941: {Name: "Shadow Keep: Storehouse, Back Section", Region: "Land of Shadow"},
	670942: {Name: "Shadow Keep: Storehouse, First Floor", Region: "Land of Shadow"},
	670943: {Name: "Shadow Keep: Storehouse, Fifth Floor", Region: "Land of Shadow"},
	670945: {Name: "West Rampart", Region: "Land of Shadow"},
	670950: {Name: "Specimen Storehouse: Main Hall", Region: "Land of Shadow"},
	670951: {Name: "Specimen Storehouse: First Floor", Region: "Land of Shadow"},
	670952: {Name: "Specimen Storehouse: Third Floor", Region: "Land of Shadow"},
	670953: {Name: "Specimen Storehouse: Fourth Floor", Region: "Land of Shadow"},
	670955: {Name: "Specimen Storehouse: Sixth Floor", Region: "Land of Shadow"},
	670956: {Name: "Specimen Storehouse: Seventh Floor", Region: "Land of Shadow"},

	// SHADOW OF THE ERDTREE — Gaols / Caves / Catacombs
	670813: {Name: "Darklight Catacombs Entry", Region: "Land of Shadow"},
	670830: {Name: "Stone Coffin Fissure", Region: "Land of Shadow"},
	670831: {Name: "Stone Coffin Fissure Deep", Region: "Land of Shadow"},
	670860: {Name: "Fog Rift Catacombs", Region: "Land of Shadow"},
	670870: {Name: "Belurat Gaol", Region: "Land of Shadow"},
	670871: {Name: "Lamenter's Gaol", Region: "Land of Shadow"},
	670880: {Name: "Dragon's Pit", Region: "Land of Shadow"},
	670960: {Name: "Scorpion River Catacombs", Region: "Land of Shadow"},
	670961: {Name: "Darklight Catacombs", Region: "Land of Shadow"},
	670970: {Name: "Bonny Gaol", Region: "Land of Shadow"},
	670980: {Name: "Rivermouth Cave", Region: "Land of Shadow"},

	// SHADOW OF THE ERDTREE — Midra's Manse
	670814: {Name: "Midra's Manse: Reading Room", Region: "Land of Shadow"},
	670815: {Name: "Midra's Manse: Deathbed Chamber", Region: "Land of Shadow"},

	// SHADOW OF THE ERDTREE — Open World (Gravesite Plain)
	670800: {Name: "Gravesite Plain (West)", Region: "Land of Shadow"},
	670801: {Name: "Gravesite Plain (North)", Region: "Land of Shadow"},
	670802: {Name: "Castle Ensis Checkpoint", Region: "Land of Shadow"},
	670804: {Name: "Cerulean Coast", Region: "Land of Shadow"},
	670805: {Name: "Church of Benediction", Region: "Land of Shadow"},
	670806: {Name: "Cerulean Coast South", Region: "Land of Shadow"},
	670807: {Name: "Abyssal Woods", Region: "Land of Shadow"},
	670808: {Name: "Cerulean Coast (Dragon Area)", Region: "Land of Shadow"},
	670809: {Name: "Charo's Hidden Grave", Region: "Land of Shadow"},
	670812: {Name: "Jagged Peak Summit", Region: "Land of Shadow"},

	// SHADOW OF THE ERDTREE — Open World (Scadu Altus / Rauh Base)
	670900: {Name: "Moorth Ruins", Region: "Land of Shadow"},
	670901: {Name: "Rauh Base (Northwest)", Region: "Land of Shadow"},
	670902: {Name: "Temple Town Ruins", Region: "Land of Shadow"},
	670903: {Name: "Bridge Leading to the Village", Region: "Land of Shadow"},
	670904: {Name: "Commander Gaius Arena", Region: "Land of Shadow"},
	670906: {Name: "Ancient Ruins of Rauh", Region: "Land of Shadow"},
	670907: {Name: "Rauh Base (Rot Area)", Region: "Land of Shadow"},
	670908: {Name: "Rauh Base (Spirit Spring)", Region: "Land of Shadow"},
	670910: {Name: "Hinterland", Region: "Land of Shadow"},
	670911: {Name: "Moorth Highway Camp", Region: "Land of Shadow"},
	670912: {Name: "Scadu Altus West", Region: "Land of Shadow"},
	670913: {Name: "Fort of Reprimand", Region: "Land of Shadow"},
	670930: {Name: "Road to Manus Metyr", Region: "Land of Shadow"},
}

// IsDLCSummoningPool reports whether a summoning pool ID belongs to Shadow of the Erdtree DLC.
func IsDLCSummoningPool(id uint32) bool {
	return id >= 670800 && id <= 670999
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
