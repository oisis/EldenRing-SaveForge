package data

// MapRegionData holds the static definition of a map region.
type MapRegionData struct {
	Name string
	Area string // "Limgrave", "Liurnia", "Altus", "Caelid", "Mountaintops", "Underground", "DLC", "System"
}

// MapSystem contains system-level map display flags.
var MapSystem = map[uint32]MapRegionData{
	62000: {Name: "Allow Map Display", Area: "System"},
	62001: {Name: "Allow Underground Map Display", Area: "System"},
	82001: {Name: "Show Underground", Area: "System"},
	82002: {Name: "Show Shadow Realm Map", Area: "System"},
}

// MapVisible contains safe map region visibility flags (62xxx).
// Setting these reveals the map texture for each region.
// Only includes flags verified as safe — see MapUnsafe for risky sub-region flags.
var MapVisible = map[uint32]MapRegionData{
	// ── Overworld — Limgrave ──
	62010: {Name: "Limgrave, West", Area: "Limgrave"},
	62011: {Name: "Weeping Peninsula", Area: "Limgrave"},
	62012: {Name: "Limgrave, East", Area: "Limgrave"},
	// ── Overworld — Liurnia ──
	62020: {Name: "Liurnia, East", Area: "Liurnia"},
	62021: {Name: "Liurnia, North", Area: "Liurnia"},
	62022: {Name: "Liurnia, West", Area: "Liurnia"},
	// ── Overworld — Altus Plateau ──
	62030: {Name: "Altus Plateau", Area: "Altus"},
	62031: {Name: "Leyndell, Royal Capital", Area: "Altus"},
	62032: {Name: "Mt. Gelmir", Area: "Altus"},
	// ── Overworld — Caelid ──
	62040: {Name: "Caelid", Area: "Caelid"},
	62041: {Name: "Dragonbarrow", Area: "Caelid"},
	// ── Overworld — Mountaintops ──
	62050: {Name: "Mountaintops of the Giants, West", Area: "Mountaintops"},
	62051: {Name: "Mountaintops of the Giants, East", Area: "Mountaintops"},
	62052: {Name: "Consecrated Snowfield", Area: "Mountaintops"},
	// ── Overworld — Underground ──
	62060: {Name: "Ainsel River", Area: "Underground"},
	62061: {Name: "Lake of Rot", Area: "Underground"},
	62062: {Name: "Mohgwyn Palace", Area: "Underground"},
	62063: {Name: "Siofra River", Area: "Underground"},
	62064: {Name: "Deeproot Depths", Area: "Underground"},
	// ── Overworld — DLC (Shadow of the Erdtree) ──
	62080: {Name: "Gravesite Plain", Area: "DLC"},
	62081: {Name: "Scadu Altus", Area: "DLC"},
	62082: {Name: "Southern Shore", Area: "DLC"},
	62083: {Name: "Rauh Ruins", Area: "DLC"},
	62084: {Name: "Abyss", Area: "DLC"},
	// ── Dungeon maps — Limgrave / Weeping Peninsula ──
	62100: {Name: "Murkwater Catacombs", Area: "Limgrave"},
	62101: {Name: "Murkwater Cave", Area: "Limgrave"},
	62102: {Name: "Fringefolk Hero's Cave", Area: "Limgrave"},
	62103: {Name: "Stormfoot Catacombs", Area: "Limgrave"},
	62104: {Name: "Deathtouched Catacombs", Area: "Limgrave"},
	62105: {Name: "Limgrave Tunnels", Area: "Limgrave"},
	62106: {Name: "Groveside Cave", Area: "Limgrave"},
	62107: {Name: "Coastal Cave", Area: "Limgrave"},
	62108: {Name: "Highroad Cave", Area: "Limgrave"},
	62109: {Name: "Tombsward Catacombs", Area: "Limgrave"},
	62110: {Name: "Tombsward Cave", Area: "Limgrave"},
	62111: {Name: "Impaler's Catacombs", Area: "Limgrave"},
	62120: {Name: "Stormveil Castle", Area: "Limgrave"},
	62121: {Name: "Stormveil Cliffside", Area: "Limgrave"},
	62122: {Name: "Stormveil Main Gate", Area: "Limgrave"},
	62123: {Name: "Secluded Cell", Area: "Limgrave"},
	62124: {Name: "Godrick the Grafted", Area: "Limgrave"},
	62125: {Name: "Liftside Chamber", Area: "Limgrave"},
	62126: {Name: "Rampart Tower", Area: "Limgrave"},
	62127: {Name: "Stormveil Legacy", Area: "Limgrave"},
	62128: {Name: "Stormveil Legacy (2)", Area: "Limgrave"},
	62129: {Name: "Stormveil Legacy (3)", Area: "Limgrave"},
	62130: {Name: "Morne Tunnel", Area: "Limgrave"},
	62131: {Name: "Castle Morne", Area: "Limgrave"},
	62132: {Name: "Castle Morne (2)", Area: "Limgrave"},
	62133: {Name: "Castle Morne (3)", Area: "Limgrave"},
	62134: {Name: "Castle Morne Lift", Area: "Limgrave"},
	62135: {Name: "Behind the Castle", Area: "Limgrave"},
	62137: {Name: "Earthbore Cave", Area: "Limgrave"},
	62138: {Name: "Waypoint Ruins", Area: "Limgrave"},
	62150: {Name: "Roundtable Hold", Area: "Limgrave"},
	62151: {Name: "Roundtable Hold (2)", Area: "Limgrave"},
	62152: {Name: "Roundtable Hold (3)", Area: "Limgrave"},
	62153: {Name: "Roundtable Hold (4)", Area: "Limgrave"},
	62154: {Name: "Roundtable Hold (5)", Area: "Limgrave"},
	62170: {Name: "Stranded Graveyard", Area: "Limgrave"},
	62171: {Name: "Cave of Knowledge", Area: "Limgrave"},
	62172: {Name: "Cave of Knowledge (2)", Area: "Limgrave"},
	62173: {Name: "Cave of Knowledge (3)", Area: "Limgrave"},
	62174: {Name: "Chapel of Anticipation", Area: "Limgrave"},
	62175: {Name: "Chapel of Anticipation (2)", Area: "Limgrave"},
	62176: {Name: "Chapel of Anticipation (3)", Area: "Limgrave"},
	62177: {Name: "Chapel of Anticipation (4)", Area: "Limgrave"},
	62178: {Name: "Chapel of Anticipation (5)", Area: "Limgrave"},
	62180: {Name: "Divine Tower of Limgrave", Area: "Limgrave"},
	62181: {Name: "Divine Tower of Limgrave (2)", Area: "Limgrave"},
	62182: {Name: "Divine Tower of Limgrave (3)", Area: "Limgrave"},
	62183: {Name: "Divine Tower of Limgrave (4)", Area: "Limgrave"},
	62184: {Name: "Divine Tower of Limgrave (5)", Area: "Limgrave"},
	// ── Dungeon maps — Liurnia ──
	62200: {Name: "Raya Lucaria Crystal Tunnel", Area: "Liurnia"},
	62201: {Name: "Academy of Raya Lucaria", Area: "Liurnia"},
	62202: {Name: "Academy (2)", Area: "Liurnia"},
	62203: {Name: "Academy (3)", Area: "Liurnia"},
	62204: {Name: "Academy (4)", Area: "Liurnia"},
	62205: {Name: "Academy (5)", Area: "Liurnia"},
	62206: {Name: "Academy (6)", Area: "Liurnia"},
	62207: {Name: "Academy (7)", Area: "Liurnia"},
	62208: {Name: "Academy (8)", Area: "Liurnia"},
	62209: {Name: "Academy (9)", Area: "Liurnia"},
	62220: {Name: "Caria Manor", Area: "Liurnia"},
	62221: {Name: "Caria Manor (2)", Area: "Liurnia"},
	62222: {Name: "Caria Manor (3)", Area: "Liurnia"},
	62223: {Name: "Caria Manor (4)", Area: "Liurnia"},
	62224: {Name: "Caria Manor (5)", Area: "Liurnia"},
	62225: {Name: "Ruin-Strewn Precipice", Area: "Liurnia"},
	62226: {Name: "Ruin-Strewn Precipice (2)", Area: "Liurnia"},
	62227: {Name: "Ruin-Strewn Precipice (3)", Area: "Liurnia"},
	62228: {Name: "Road's End Catacombs", Area: "Liurnia"},
	62229: {Name: "Cliffbottom Catacombs", Area: "Liurnia"},
	62230: {Name: "Black Knife Catacombs", Area: "Liurnia"},
	62231: {Name: "Stillwater Cave", Area: "Liurnia"},
	62232: {Name: "Lakeside Crystal Cave", Area: "Liurnia"},
	62233: {Name: "Academy Crystal Cave", Area: "Liurnia"},
	62234: {Name: "Converted Tower", Area: "Liurnia"},
	62235: {Name: "Cuckoo's Evergaol", Area: "Liurnia"},
	62236: {Name: "Liurnia Dungeon", Area: "Liurnia"},
	62237: {Name: "Liurnia Dungeon (2)", Area: "Liurnia"},
	62238: {Name: "Liurnia Dungeon (3)", Area: "Liurnia"},
	62239: {Name: "Liurnia Dungeon (4)", Area: "Liurnia"},
	62240: {Name: "Liurnia Dungeon (5)", Area: "Liurnia"},
	62241: {Name: "Liurnia Dungeon (6)", Area: "Liurnia"},
	62242: {Name: "Liurnia Dungeon (7)", Area: "Liurnia"},
	62243: {Name: "Liurnia Dungeon (8)", Area: "Liurnia"},
	62244: {Name: "Liurnia Dungeon (9)", Area: "Liurnia"},
	62245: {Name: "Liurnia Dungeon (10)", Area: "Liurnia"},
	62246: {Name: "Liurnia Dungeon (11)", Area: "Liurnia"},
	62247: {Name: "Liurnia Dungeon (12)", Area: "Liurnia"},
	62248: {Name: "Liurnia Dungeon (13)", Area: "Liurnia"},
	62249: {Name: "Liurnia Dungeon (14)", Area: "Liurnia"},
	62250: {Name: "Divine Tower of Liurnia", Area: "Liurnia"},
	62251: {Name: "Divine Tower of Liurnia (2)", Area: "Liurnia"},
	62252: {Name: "Divine Tower of Liurnia (3)", Area: "Liurnia"},
	62253: {Name: "Divine Tower of Liurnia (4)", Area: "Liurnia"},
	62254: {Name: "Divine Tower of Liurnia (5)", Area: "Liurnia"},
	62280: {Name: "Miquella's Haligtree", Area: "Liurnia"},
	62281: {Name: "Haligtree (2)", Area: "Liurnia"},
	62282: {Name: "Haligtree (3)", Area: "Liurnia"},
	62283: {Name: "Haligtree (4)", Area: "Liurnia"},
	62284: {Name: "Elphael, Brace of the Haligtree", Area: "Liurnia"},
	62285: {Name: "Elphael (2)", Area: "Liurnia"},
	// ── Dungeon maps — Altus / Mt. Gelmir ──
	62300: {Name: "Altus Tunnel", Area: "Altus"},
	62310: {Name: "Leyndell, Royal Capital (dungeon)", Area: "Altus"},
	62311: {Name: "Leyndell (2)", Area: "Altus"},
	62312: {Name: "Leyndell (3)", Area: "Altus"},
	62313: {Name: "Leyndell (4)", Area: "Altus"},
	62314: {Name: "Leyndell (5)", Area: "Altus"},
	62315: {Name: "Leyndell (6)", Area: "Altus"},
	62316: {Name: "Leyndell (7)", Area: "Altus"},
	62317: {Name: "Leyndell (8)", Area: "Altus"},
	62318: {Name: "Leyndell (9)", Area: "Altus"},
	62319: {Name: "Leyndell (10)", Area: "Altus"},
	62320: {Name: "Leyndell (11)", Area: "Altus"},
	62321: {Name: "Leyndell (12)", Area: "Altus"},
	62322: {Name: "Leyndell (13)", Area: "Altus"},
	62323: {Name: "Leyndell (14)", Area: "Altus"},
	62324: {Name: "Leyndell (15)", Area: "Altus"},
	62325: {Name: "Leyndell (16)", Area: "Altus"},
	62330: {Name: "Volcano Manor", Area: "Altus"},
	62331: {Name: "Volcano Manor (2)", Area: "Altus"},
	62332: {Name: "Volcano Manor (3)", Area: "Altus"},
	62333: {Name: "Volcano Manor (4)", Area: "Altus"},
	62334: {Name: "Volcano Manor (5)", Area: "Altus"},
	62335: {Name: "Volcano Manor (6)", Area: "Altus"},
	62337: {Name: "Volcano Manor (7)", Area: "Altus"},
	62338: {Name: "Volcano Manor (8)", Area: "Altus"},
	62339: {Name: "Volcano Manor (9)", Area: "Altus"},
	62340: {Name: "Sainted Hero's Grave", Area: "Altus"},
	62341: {Name: "Auriza Hero's Grave", Area: "Altus"},
	62342: {Name: "Auriza Side Tomb", Area: "Altus"},
	62343: {Name: "Unsightly Catacombs", Area: "Altus"},
	62344: {Name: "Perfumer's Grotto", Area: "Altus"},
	62345: {Name: "Sage's Cave", Area: "Altus"},
	62346: {Name: "Old Altus Tunnel", Area: "Altus"},
	62347: {Name: "Sealed Tunnel", Area: "Altus"},
	62348: {Name: "Wyndham Catacombs", Area: "Altus"},
	62360: {Name: "Divine Tower of West Altus", Area: "Altus"},
	62380: {Name: "Crumbling Farum Azula", Area: "Altus"},
	62381: {Name: "Farum Azula (2)", Area: "Altus"},
	62382: {Name: "Farum Azula (3)", Area: "Altus"},
	62383: {Name: "Farum Azula (4)", Area: "Altus"},
	62384: {Name: "Farum Azula (5)", Area: "Altus"},
	62385: {Name: "Farum Azula (6)", Area: "Altus"},
	62386: {Name: "Farum Azula (7)", Area: "Altus"},
	62389: {Name: "Farum Azula (8)", Area: "Altus"},
	// ── Dungeon maps — Caelid / Dragonbarrow ──
	62410: {Name: "Redmane Castle", Area: "Caelid"},
	62411: {Name: "Redmane Castle (2)", Area: "Caelid"},
	62412: {Name: "Redmane Castle (3)", Area: "Caelid"},
	62413: {Name: "Redmane Castle (4)", Area: "Caelid"},
	62415: {Name: "Minor Erdtree Catacombs", Area: "Caelid"},
	62416: {Name: "Caelid Catacombs", Area: "Caelid"},
	62417: {Name: "War-Dead Catacombs", Area: "Caelid"},
	62420: {Name: "Gael Tunnel", Area: "Caelid"},
	62421: {Name: "Sellia Crystal Tunnel", Area: "Caelid"},
	62422: {Name: "Sellia Hideaway", Area: "Caelid"},
	62424: {Name: "Dragonbarrow Cave", Area: "Caelid"},
	62425: {Name: "Abandoned Cave", Area: "Caelid"},
	62426: {Name: "Gaol Cave", Area: "Caelid"},
	62427: {Name: "Caelid Dungeon", Area: "Caelid"},
	62428: {Name: "Caelid Dungeon (2)", Area: "Caelid"},
	62429: {Name: "Caelid Dungeon (3)", Area: "Caelid"},
	62434: {Name: "Divine Tower of Caelid", Area: "Caelid"},
	62435: {Name: "Divine Tower of Caelid (2)", Area: "Caelid"},
	62436: {Name: "Divine Tower of Caelid (3)", Area: "Caelid"},
	62437: {Name: "Divine Tower of Caelid (4)", Area: "Caelid"},
	62438: {Name: "Divine Tower of Caelid (5)", Area: "Caelid"},
	62460: {Name: "Isolated Divine Tower", Area: "Caelid"},
	62470: {Name: "Divine Tower of East Altus", Area: "Caelid"},
	62471: {Name: "Divine Tower of East Altus (2)", Area: "Caelid"},
	62472: {Name: "Divine Tower of East Altus (3)", Area: "Caelid"},
	62473: {Name: "Divine Tower of East Altus (4)", Area: "Caelid"},
	62474: {Name: "Divine Tower of East Altus (5)", Area: "Caelid"},
	62475: {Name: "Divine Tower of East Altus (6)", Area: "Caelid"},
	// ── Dungeon maps — Mountaintops / Snowfield ──
	62511: {Name: "Spiritcaller's Cave", Area: "Mountaintops"},
	62514: {Name: "Giant-Conquering Hero's Grave", Area: "Mountaintops"},
	62516: {Name: "Giants' Mountaintop Catacombs", Area: "Mountaintops"},
	62520: {Name: "Consecrated Snowfield Catacombs", Area: "Mountaintops"},
	62521: {Name: "Cave of the Forlorn", Area: "Mountaintops"},
	62522: {Name: "Yelough Anix Tunnel", Area: "Mountaintops"},
	62524: {Name: "Hidden Path to the Haligtree", Area: "Mountaintops"},
	62526: {Name: "Ordina, Liturgical Town", Area: "Mountaintops"},
	62528: {Name: "Mountaintops Dungeon", Area: "Mountaintops"},
	62529: {Name: "Mountaintops Dungeon (2)", Area: "Mountaintops"},
	62530: {Name: "Mountaintops Dungeon (3)", Area: "Mountaintops"},
	62531: {Name: "Mountaintops Dungeon (4)", Area: "Mountaintops"},
	62560: {Name: "Flame Peak", Area: "Mountaintops"},
	62572: {Name: "Forge of the Giants", Area: "Mountaintops"},
	// ── Dungeon maps — Underground ──
	62610: {Name: "Nokron, Eternal City", Area: "Underground"},
	62620: {Name: "Siofra Aqueduct", Area: "Underground"},
	62621: {Name: "Siofra Aqueduct (2)", Area: "Underground"},
	62622: {Name: "Siofra Aqueduct (3)", Area: "Underground"},
	62630: {Name: "Nokstella, Eternal City", Area: "Underground"},
	62631: {Name: "Nokstella (2)", Area: "Underground"},
	62632: {Name: "Nokstella (3)", Area: "Underground"},
	62633: {Name: "Nokstella (4)", Area: "Underground"},
	62634: {Name: "Nokstella (5)", Area: "Underground"},
	62640: {Name: "Ainsel River Main", Area: "Underground"},
	// ── Dungeon maps — Other ──
	62700: {Name: "Leyndell, Ashen Capital", Area: "Altus"},
	62720: {Name: "Elden Throne", Area: "Altus"},
	62730: {Name: "Stone Platform", Area: "Altus"},
	62740: {Name: "Fractured Marika", Area: "Altus"},
	// ── Dungeon maps — DLC (Shadow of the Erdtree) ──
	62800: {Name: "Belurat, Tower Settlement", Area: "DLC"},
	62805: {Name: "Belurat (2)", Area: "DLC"},
	62806: {Name: "Belurat (3)", Area: "DLC"},
	62807: {Name: "Belurat (4)", Area: "DLC"},
	62808: {Name: "Belurat (5)", Area: "DLC"},
	62809: {Name: "Castle Ensis", Area: "DLC"},
	62810: {Name: "Castle Ensis (2)", Area: "DLC"},
	62811: {Name: "Castle Ensis (3)", Area: "DLC"},
	62812: {Name: "Castle Ensis (4)", Area: "DLC"},
	62813: {Name: "Castle Ensis (5)", Area: "DLC"},
	62814: {Name: "Castle Ensis (6)", Area: "DLC"},
	62815: {Name: "Castle Ensis (7)", Area: "DLC"},
	62820: {Name: "Shadow Keep", Area: "DLC"},
	62822: {Name: "Shadow Keep (2)", Area: "DLC"},
	62823: {Name: "Shadow Keep (3)", Area: "DLC"},
	62825: {Name: "Shadow Keep (4)", Area: "DLC"},
	62827: {Name: "Shadow Keep (5)", Area: "DLC"},
	62830: {Name: "Specimen Storehouse", Area: "DLC"},
	62831: {Name: "Specimen Storehouse (2)", Area: "DLC"},
	62880: {Name: "Ancient Ruins of Rauh", Area: "DLC"},
	62881: {Name: "Ancient Ruins of Rauh (2)", Area: "DLC"},
	62900: {Name: "Stone Coffin Fissure", Area: "DLC"},
	62902: {Name: "Taylew's Ruined Forge", Area: "DLC"},
	62903: {Name: "Ruined Forge Lava Intake", Area: "DLC"},
	62905: {Name: "Rivermouth Cave", Area: "DLC"},
	62906: {Name: "Dragon's Pit", Area: "DLC"},
	62907: {Name: "Dragon's Pit (2)", Area: "DLC"},
	62908: {Name: "Fog Rift Catacombs", Area: "DLC"},
	62909: {Name: "Scorpion River Catacombs", Area: "DLC"},
	62910: {Name: "DLC Dungeon", Area: "DLC"},
	62917: {Name: "Darklight Catacombs", Area: "DLC"},
	62920: {Name: "Midra's Manse", Area: "DLC"},
	62931: {Name: "Enir-Ilim", Area: "DLC"},
	62932: {Name: "Enir-Ilim (2)", Area: "DLC"},
	62940: {Name: "Finger Ruins of Rhia", Area: "DLC"},
	62941: {Name: "Finger Ruins of Dheo", Area: "DLC"},
	62942: {Name: "Finger Ruins (3)", Area: "DLC"},
	62980: {Name: "Gaol Dungeons", Area: "DLC"},
	62981: {Name: "Gaol Dungeons (2)", Area: "DLC"},
}

// MapUnsafe contains sub-region visibility flags that can cause black map tiles
// when set without the game's normal discovery flow. Shown in UI but excluded
// from "Reveal All" to prevent visual corruption.
var MapUnsafe = map[uint32]MapRegionData{
	62004: {Name: "Center (sub-region)", Area: "Limgrave"},
	62005: {Name: "SW (sub-region)", Area: "Limgrave"},
	62006: {Name: "NW (sub-region)", Area: "Limgrave"},
	62007: {Name: "SE (sub-region)", Area: "Limgrave"},
	62008: {Name: "NE (sub-region)", Area: "Limgrave"},
	62009: {Name: "N (sub-region)", Area: "Limgrave"},
	62053: {Name: "Mountaintops, North (sub-region)", Area: "Mountaintops"},
	62065: {Name: "Underground (sub-region)", Area: "Underground"},
}

// MapFragmentItems maps visible flag IDs (62xxx) to their corresponding
// map fragment inventory item IDs (0x400021xx / 0x401EAxxx).
// Used by SetMapRegion/RevealAllMap to add map items to inventory.
var MapFragmentItems = map[uint32]uint32{
	// Base game
	62010: 0x40002198, // Limgrave, West
	62011: 0x40002199, // Weeping Peninsula
	62012: 0x4000219A, // Limgrave, East
	62020: 0x4000219B, // Liurnia, East
	62021: 0x4000219C, // Liurnia, North
	62022: 0x4000219D, // Liurnia, West
	62030: 0x4000219E, // Altus Plateau
	62031: 0x4000219F, // Leyndell, Royal Capital
	62032: 0x400021A0, // Mt. Gelmir
	62040: 0x400021A1, // Caelid
	62041: 0x400021A2, // Dragonbarrow
	62050: 0x400021A3, // Mountaintops of the Giants, West
	62051: 0x400021A4, // Mountaintops of the Giants, East
	62052: 0x400021AA, // Consecrated Snowfield
	62060: 0x400021A5, // Ainsel River
	62061: 0x400021A6, // Lake of Rot
	62062: 0x400021A8, // Mohgwyn Palace
	62063: 0x400021A7, // Siofra River
	62064: 0x400021A9, // Deeproot Depths
	// DLC — Shadow of the Erdtree
	62080: 0x401EA618, // Gravesite Plain
	62081: 0x401EA619, // Scadu Altus
	62082: 0x401EA61A, // Southern Shore
	62083: 0x401EA61B, // Rauh Ruins
	62084: 0x401EA61C, // Abyss
}

// MapAcquired contains map fragment acquisition flags (63xxx).
// These are transient "pickup notification pending" triggers — the game clears them
// after showing the "Map Fragment acquired" popup. NOT used for map visibility or items.
var MapAcquired = map[uint32]MapRegionData{
	63010: {Name: "Limgrave, West", Area: "Limgrave"},
	63011: {Name: "Weeping Peninsula", Area: "Limgrave"},
	63012: {Name: "Limgrave, East", Area: "Limgrave"},
	63020: {Name: "Liurnia, East", Area: "Liurnia"},
	63021: {Name: "Liurnia, North", Area: "Liurnia"},
	63022: {Name: "Liurnia, West", Area: "Liurnia"},
	63030: {Name: "Altus Plateau", Area: "Altus"},
	63031: {Name: "Leyndell, Royal Capital", Area: "Altus"},
	63032: {Name: "Mt. Gelmir", Area: "Altus"},
	63040: {Name: "Caelid", Area: "Caelid"},
	63041: {Name: "Dragonbarrow", Area: "Caelid"},
	63050: {Name: "Mountaintops of the Giants, West", Area: "Mountaintops"},
	63051: {Name: "Mountaintops of the Giants, East", Area: "Mountaintops"},
	63052: {Name: "Consecrated Snowfield", Area: "Mountaintops"},
	63060: {Name: "Ainsel River", Area: "Underground"},
	63061: {Name: "Lake of Rot", Area: "Underground"},
	63062: {Name: "Mohgwyn Palace", Area: "Underground"},
	63063: {Name: "Siofra River", Area: "Underground"},
	63064: {Name: "Deeproot Depths", Area: "Underground"},
	63080: {Name: "Gravesite Plain", Area: "DLC"},
	63081: {Name: "Scadu Altus", Area: "DLC"},
	63082: {Name: "Southern Shore", Area: "DLC"},
	63083: {Name: "Rauh Ruins", Area: "DLC"},
	63084: {Name: "Abyss", Area: "DLC"},
}

// MapFragmentItemToFlagID is the reverse of MapFragmentItems (item ID → visible flag ID).
var MapFragmentItemToFlagID map[uint32]uint32

func init() {
	MapFragmentItemToFlagID = make(map[uint32]uint32, len(MapFragmentItems))
	for flagID, itemID := range MapFragmentItems {
		MapFragmentItemToFlagID[itemID] = flagID
	}
}

// IsDLCMapFlag returns true if the visible flag ID belongs to a DLC (Shadow of the Erdtree) map region.
func IsDLCMapFlag(flagID uint32) bool {
	return (flagID >= 62080 && flagID <= 62084) ||
		(flagID >= 62800 && flagID <= 62999)
}
