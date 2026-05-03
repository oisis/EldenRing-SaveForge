package data

// GraceData holds metadata for a Site of Grace.
type GraceData struct {
	Name        string
	BossArena   bool   // true if this grace appears in/after a boss fight
	DungeonType string // "catacomb", "hero_grave", or "" for regular graces
	DoorFlag    uint32 // overworld ObjAct event flag that opens the entrance door (0 = none)
}

// G is a shorthand constructor for regular graces.
func G(name string) GraceData { return GraceData{Name: name} }

// B is a shorthand constructor for boss-arena graces.
func B(name string) GraceData { return GraceData{Name: name, BossArena: true} }

// Cat is a shorthand constructor for catacomb graces (sealed entrance doors).
func Cat(name string, doorFlag uint32) GraceData {
	return GraceData{Name: name, DungeonType: "catacomb", DoorFlag: doorFlag}
}

// HG is a shorthand constructor for hero's grave graces (sealed entrance doors).
func HG(name string, doorFlag uint32) GraceData {
	return GraceData{Name: name, DungeonType: "hero_grave", DoorFlag: doorFlag}
}

var Graces = map[uint32]GraceData{
	// --- Stormveil Castle ---
	0x00011558: B("Godrick the Grafted (Stormveil Castle)"),
	0x00011559: B("Margit, the Fell Omen (Stormveil Castle)"),
	0x0001155A: G("Castleward Tunnel (Stormveil Castle)"),
	0x0001155B: G("Gateside Chamber (Stormveil Castle)"),
	0x0001155C: G("Stormveil Cliffside (Stormveil Castle)"),
	0x0001155D: G("Rampart Tower (Stormveil Castle)"),
	0x0001155E: G("Liftside Chamber (Stormveil Castle)"),
	0x0001155F: G("Secluded Cell (Stormveil Castle)"),
	0x00011560: G("Stormveil Main Gate (Stormveil Castle)"),

	// --- Leyndell, Royal Capital ---
	0x000115BC: B("Elden Throne (Leyndell Royal Capital)"),
	0x000115BD: B("Erdtree Sanctuary (Leyndell Royal Capital)"),
	0x000115BE: G("East Capital Rampart (Leyndell Royal Capital)"),
	0x000115BF: G("Lower Capital Church (Leyndell Royal Capital)"),
	0x000115C0: G("Avenue Balcony (Leyndell Royal Capital)"),
	0x000115C1: G("West Capital Rampart (Leyndell Royal Capital)"),
	0x000115C3: G("Queen's Bedchamber (Leyndell Royal Capital)"),
	0x000115C4: G("Fortified Manor, First Floor (Leyndell Royal Capital)"),
	0x000115C5: G("Divine Bridge (Leyndell Royal Capital)"),

	// --- Leyndell, Ashen Capital ---
	0x000115D0: B("Elden Throne (Leyndell Ashen Capital)"),
	0x000115D1: B("Erdtree Sanctuary (Leyndell Ashen Capital)"),
	0x000115D2: G("East Capital Rampart (Leyndell Ashen Capital)"),
	0x000115D3: G("Leyndell, Capital of Ash (Leyndell Ashen Capital)"),
	0x000115D4: G("Queen's Bedchamber (Leyndell Ashen Capital)"),
	0x000115D5: G("Divine Bridge (Leyndell Ashen Capital)"),

	// --- Roundtable Hold ---
	0x00011616: G("Table of Lost Grace / Roundtable Hold (Roundtable Hold)"),

	// --- Ainsel River ---
	0x0001162A: B("Dragonkin Soldier of Nokstella (Ainsel River)"),
	0x0001162B: G("Ainsel River Well Depths (Ainsel River)"),
	0x0001162C: G("Ainsel River Sluice Gate (Ainsel River)"),
	0x0001162D: G("Ainsel River Downstream (Ainsel River)"),
	0x0001162E: G("Ainsel River Main (Ainsel River)"),
	0x0001162F: G("Nokstella, Eternal City (Ainsel River)"),
	0x00011633: G("Nokstella Waterfall Basin (Ainsel River)"),
	0x00011634: G("Great Waterfall Basin (Ainsel River)"),

	// --- Lake of Rot ---
	0x00011630: G("Lake of Rot Shoreside (Lake of Rot)"),
	0x00011632: G("Grand Cloister (Lake of Rot)"),

	// --- Siofra River ---
	0x00011635: B("Mimic Tear (Siofra River)"),
	0x00011636: G("Siofra River Bank (Siofra River)"),
	0x00011637: G("Worshippers' Woods (Siofra River)"),
	0x00011638: B("Ancestral Woods (Siofra River)"),
	0x00011639: G("Aqueduct-Facing Cliffs (Siofra River)"),
	0x0001163A: G("Night's Sacred Ground (Siofra River)"),
	0x0001163B: G("Below the Well (Siofra River)"),
	0x00011666: G("Siofra River Well Depths (Siofra River)"),
	0x00011667: G("Nokron, Eternal City (Siofra River)"),

	// --- Deeproot Depths ---
	0x0001163E: B("Prince of Death's Throne (Deeproot Depths)"),
	0x0001163F: G("Root-Facing Cliffs (Deeproot Depths)"),
	0x00011640: G("Great Waterfall Crest (Deeproot Depths)"),
	0x00011641: G("Deeproot Depths (Deeproot Depths)"),
	0x00011642: G("The Nameless Eternal City (Deeproot Depths)"),
	0x00011643: G("Across the Roots (Deeproot Depths)"),
	0x00011648: B("Astel, Naturalborn of the Void (Deeproot Depths)"),

	// --- Mohgwyn Palace ---
	0x00011652: B("Cocoon of the Empyrean (Mohgwyn Palace)"),
	0x00011653: G("Palace Approach Ledge-Road (Mohgwyn Palace)"),
	0x00011654: G("Dynasty Mausoleum Entrance (Mohgwyn Palace)"),
	0x00011655: G("Dynasty Mausoleum Midpoint (Mohgwyn Palace)"),

	// --- Crumbling Farum Azula ---
	0x00011684: B("Maliketh, the Black Blade (Crumbling Farum Azula)"),
	0x00011685: B("Dragonlord Placidusax (Crumbling Farum Azula)"),
	0x00011686: B("Dragon Temple Altar (Crumbling Farum Azula)"),
	0x00011687: G("Crumbling Beast Grave (Crumbling Farum Azula)"),
	0x00011688: G("Crumbling Beast Grave Depths (Crumbling Farum Azula)"),
	0x00011689: G("Tempest-Facing Balcony (Crumbling Farum Azula)"),
	0x0001168A: G("Dragon Temple (Crumbling Farum Azula)"),
	0x0001168B: G("Dragon Temple Transept (Crumbling Farum Azula)"),
	0x0001168C: G("Dragon Temple Lift (Crumbling Farum Azula)"),
	0x0001168D: G("Dragon Temple Rooftop (Crumbling Farum Azula)"),
	0x0001168E: G("Beside the Great Bridge (Crumbling Farum Azula)"),

	// --- Liurnia of the Lakes — Raya Lucaria Academy ---
	0x000116E8: B("Raya Lucaria Grand Library (Liurnia North)"),
	0x000116E9: B("Debate Parlour (Liurnia North)"),
	0x000116EA: G("Church of the Cuckoo (Liurnia North)"),
	0x000116EB: G("Schoolhouse Classroom (Liurnia North)"),

	// --- Miquella's Haligtree ---
	0x0001174C: B("Malenia, Goddess of Rot (Miquella's Haligtree)"),
	0x0001174D: G("Prayer Room (Miquella's Haligtree)"),
	0x0001174E: G("Elphael Inner Wall (Miquella's Haligtree)"),
	0x0001174F: G("Drainage Channel (Miquella's Haligtree)"),
	0x00011750: G("Haligtree Roots (Miquella's Haligtree)"),
	0x00011751: B("Haligtree Promenade (Miquella's Haligtree)"),
	0x00011752: G("Haligtree Canopy (Miquella's Haligtree)"),
	0x00011753: G("Haligtree Town (Miquella's Haligtree)"),
	0x00011754: G("Haligtree Town Plaza (Miquella's Haligtree)"),

	// --- Mt. Gelmir — Volcano Manor ---
	0x000117B0: B("Rykard, Lord of Blasphemy (Mt. Gelmir)"),
	0x000117B1: B("Temple of Eiglay (Mt. Gelmir)"),
	0x000117B2: G("Volcano Manor (Mt. Gelmir)"),
	0x000117B3: G("Prison Town Church (Mt. Gelmir)"),
	0x000117B4: G("Guest Hall (Mt. Gelmir)"),
	0x000117B5: G("Audience Pathway (Mt. Gelmir)"),
	0x000117B6: B("Abductor Virgin (Mt. Gelmir)"),
	0x000117B7: G("Subterranean Inquisition Chamber (Mt. Gelmir)"),

	// --- Tutorial / Stranded Graveyard ---
	0x00011878: G("Cave of Knowledge (Limgrave West)"),
	0x00011879: G("Stranded Graveyard (Limgrave West)"),

	// --- Fractured Marika ---
	0x000118DC: B("Fractured Marika (Leyndell Ashen Capital)"),

	// --- Shadow of the Erdtree (DLC) — Belurat, Tower Settlement ---
	0x00011940: B("Theatre of the Divine Beast (Belurat, Tower Settlement)"),
	0x00011941: G("Belurat, Tower Settlement (Belurat, Tower Settlement)"),
	0x00011942: G("Small Private Altar (Belurat, Tower Settlement)"),
	0x00011943: G("Stagefront (Belurat, Tower Settlement)"),

	// --- Shadow of the Erdtree (DLC) — Enir-Ilim ---
	0x0001194A: B("Gate of Divinity (Enir-Ilim)"),
	0x0001194C: G("Enir-Ilim: Outer Wall (Enir-Ilim)"),
	0x0001194D: G("First Rise (Enir-Ilim)"),
	0x0001194E: G("Spiral Rise (Enir-Ilim)"),
	0x0001194F: G("Cleansing Chamber Anteroom (Enir-Ilim)"),
	0x00011950: G("Divine Gate Front Staircase (Enir-Ilim)"),

	// --- Shadow of the Erdtree (DLC) — Shadow Keep ---
	0x000119A5: B("Main Gate Plaza (Shadow Keep)"),
	0x000119A6: G("Shadow Keep Main Gate (Shadow Keep)"),
	0x000119AA: G("Church District Entrance (Shadow Keep, Church District)"),
	0x000119AB: G("Sunken Chapel (Shadow Keep, Church District)"),
	0x000119AC: G("Tree-Worship Passage (Shadow Keep, Church District)"),
	0x000119AD: G("Tree-Worship Sanctum (Shadow Keep, Church District)"),
	0x000119AE: B("Messmer's Dark Chamber (Shadow Keep)"),
	0x000119AF: G("Storehouse, First Floor (Specimen Storehouse)"),
	0x000119B0: G("Storehouse, Fourth Floor (Specimen Storehouse)"),
	0x000119B1: G("Storehouse, Seventh Floor (Specimen Storehouse)"),
	0x000119B2: G("Dark Chamber Entrance (Specimen Storehouse)"),
	0x000119B4: G("Storehouse, Back Section (Specimen Storehouse)"),
	0x000119B5: G("Storehouse, Loft (Specimen Storehouse)"),
	0x000119B8: G("West Rampart (Shadow Keep)"),

	// --- Shadow of the Erdtree (DLC) — Stone Coffin Fissure ---
	0x00011A08: B("Garden of Deep Purple (Stone Coffin Fissure)"),
	0x00011A09: G("Stone Coffin Fissure (Stone Coffin Fissure)"),
	0x00011A0A: G("Fissure Cross (Stone Coffin Fissure)"),
	0x00011A0B: G("Fissure Waypoint (Stone Coffin Fissure)"),
	0x00011A0C: G("Fissure Depths (Stone Coffin Fissure)"),

	// --- Shadow of the Erdtree (DLC) — Cathedral of Manus Metyr ---
	0x00011B34: B("Finger Birthing Grounds (Cathedral of Manus Metyr)"),

	// --- Shadow of the Erdtree (DLC) — Midra's Manse ---
	0x00011C60: B("Discussion Chamber (Midra's Manse)"),
	0x00011C61: G("Manse Hall (Midra's Manse)"),
	0x00011C62: G("Midra's Library (Midra's Manse)"),
	0x00011C63: G("Second Floor Chamber (Midra's Manse)"),

	// --- Catacombs ---
	0x00011D28: Cat("Tombsward Catacombs (Weeping Peninsula)", 1043338600),
	0x00011D29: Cat("Impaler's Catacombs (Weeping Peninsula)", 1045348540),
	0x00011D2A: Cat("Stormfoot Catacombs (Limgrave West)", 1041378540),
	0x00011D2B: Cat("Road's End Catacombs (Liurnia West)", 1033438600),
	0x00011D2C: Cat("Murkwater Catacombs (Limgrave West)", 1043388540),
	0x00011D2D: Cat("Black Knife Catacombs (Liurnia East)", 1039488540),
	0x00011D2E: Cat("Cliffbottom Catacombs (Liurnia East)", 1039418540),
	0x00011D2F: Cat("Wyndham Catacombs (Altus Plateau)", 1038528600),
	0x00011D30: HG("Sainted Hero's Grave (Altus Plateau)", 1040528620),
	0x00011D31: HG("Gelmir Hero's Grave (Mt. Gelmir)", 1037538620),
	0x00011D32: HG("Auriza Hero's Grave (Altus Plateau)", 1045518620),
	0x00011D33: Cat("Deathtouched Catacombs (Limgrave East)", 1043398540),
	0x00011D34: Cat("Unsightly Catacombs (Mt. Gelmir)", 1036518600),
	0x00011D35: Cat("Auriza Side Tomb (Altus Plateau)", 1045528600),
	0x00011D36: Cat("Minor Erdtree Catacombs (Caelid)", 1047408540),
	0x00011D37: Cat("Caelid Catacombs (Caelid)", 1048368600),
	0x00011D38: Cat("War-Dead Catacombs (Caelid)", 0),
	0x00011D39: HG("Giant-Conquering Hero's Grave (Mountaintops of the Giants East)", 1050538620),
	0x00011D3A: Cat("Giant's Mountaintop Catacombs (Mountaintops of the Giants East)", 1050538600),
	0x00011D3B: Cat("Consecrated Snowfield Catacombs (Consecrated Snowfield)", 1050558540),
	0x00011D3C: G("Hidden Path to the Haligtree (Consecrated Snowfield)"),

	// --- Caves ---
	0x00011D8C: G("Murkwater Cave (Limgrave West)"),
	0x00011D8D: G("Earthbore Cave (Weeping Peninsula)"),
	0x00011D8E: G("Tombsward Cave (Weeping Peninsula)"),
	0x00011D8F: G("Groveside Cave (Limgrave West)"),
	0x00011D90: G("Stillwater Cave (Liurnia East)"),
	0x00011D91: G("Lakeside Crystal Cave (Liurnia East)"),
	0x00011D92: G("Academy Crystal Cave (Liurnia North)"),
	0x00011D93: G("Seethewater Cave (Mt. Gelmir)"),
	0x00011D95: G("Volcano Cave (Mt. Gelmir)"),
	0x00011D96: G("Dragonbarrow Cave (Dragonbarrow)"),
	0x00011D97: G("Sellia Hideaway (Caelid)"),
	0x00011D98: G("Cave of the Forlorn (Consecrated Snowfield)"),
	0x00011D9B: G("Coastal Cave (Limgrave West)"),
	0x00011D9D: G("Highroad Cave (Limgrave West)"),
	0x00011D9E: G("Perfumer's Grotto (Altus Plateau)"),
	0x00011D9F: G("Sage's Cave (Altus Plateau)"),
	0x00011DA0: G("Abandoned Cave (Caelid)"),
	0x00011Da1: G("Gaol Cave (Caelid)"),
	0x00011DA2: G("Spiritcaller's Cave (Mountaintops of the Giants West)"),

	// --- Mining Tunnels ---
	0x00011DF0: G("Morne Tunnel (Weeping Peninsula)"),
	0x00011DF1: G("Limgrave Tunnels (Limgrave West)"),
	0x00011DF2: G("Raya Lucaria Crystal Tunnel (Liurnia North)"),
	0x00011DF4: G("Old Altus Tunnel (Altus Plateau)"),
	0x00011DF5: G("Altus Tunnel (Altus Plateau)"),
	0x00011DF7: G("Gael Tunnel (Caelid)"),
	0x00011DF8: G("Sellia Crystal Tunnel (Caelid)"),
	0x00011DFB: G("Yelough Anix Tunnel (Consecrated Snowfield)"),
	0x00011E29: G("Rear Gael Tunnel Entrance (Caelid)"),

	// --- Divine Towers ---
	0x00011EC2: G("Limgrave Tower Bridge (Limgrave West)"),
	0x00011EC4: G("Divine Tower of Limgrave (Limgrave West)"),
	0x00011ECC: G("Study Hall Entrance (Liurnia East)"),
	0x00011ECD: G("Liurnia Tower Bridge (Liurnia East)"),
	0x00011ECE: G("Divine Tower of Liurnia (Liurnia East)"),
	0x00011ED6: G("Divine Tower of West Altus (Altus Plateau)"),
	0x00011ED7: G("Sealed Tunnel (Altus Plateau)"),
	0x00011ED8: G("Divine Tower of West Altus: Gate (Altus Plateau)"),
	0x00011EE0: G("Divine Tower of Caelid: Basement (Caelid)"),
	0x00011EE1: G("Divine Tower of Caelid: Center (Caelid)"),
	0x00011EEA: G("Divine Tower of the East Altus: Gate (Altus Plateau)"),
	0x00011EEB: G("Divine Tower of the East Altus (Altus Plateau)"),
	0x00011EF4: G("Isolated Divine Tower (Altus Plateau)"),

	// --- Underground / Leyndell Sewers ---
	0x00011F1C: B("Cathedral of the Forsaken (Leyndell Royal Capital)"),
	0x00011F1D: G("Underground Roadside (Leyndell Royal Capital)"),
	0x00011F1E: G("Forsaken Depths (Leyndell Royal Capital)"),
	0x00011F1F: G("Leyndell Catacombs (Leyndell Royal Capital)"),
	0x00011F20: G("Frenzied Flame Proscription (Leyndell Royal Capital)"),

	// --- Ruin-Strewn Precipice ---
	0x000120AC: B("Magma Wyrm (Liurnia North)"),
	0x000120AD: G("Ruin-Strewn Precipice (Liurnia North)"),
	0x000120AE: B("Ruin-Strewn Precipice Overlook (Altus Plateau)"),

	// --- Shadow of the Erdtree — DLC Dungeons ---
	0x00012110: Cat("Fog Rift Catacombs (Gravesite Plain)", 0),
	0x00012111: Cat("Scorpion River Catacombs (Scadu Altus)", 0),
	0x00012112: Cat("Darklight Catacombs (Abyssal Woods)", 0),
	0x00012174: G("Belurat Gaol (Gravesite Plain)"),
	0x00012175: G("Bonny Gaol (Scadu Altus)"),
	0x00012176: G("Lamenter's Gaol (Charo's Hidden Grave)"),
	0x000121D8: G("Ruined Forge Lava Intake (Gravesite Plain)"),
	0x000121DA: G("Ruined Forge of Starfall Past (Scadu Altus)"),
	0x000121DB: G("Taylew's Ruined Forge (Rauh Base)"),
	0x0001223C: G("Rivermouth Cave (Gravesite Plain)"),
	0x0001223D: G("Dragon's Pit (Gravesite Plain)"),
	0x0001226F: B("Dragon's Pit Terminus (Jagged Peak)"),

	// --- Limgrave West ---
	0x00012944: G("Church of Elleh (Limgrave West)"),
	0x00012945: G("The First Step (Limgrave West)"),
	0x00012946: G("Stormhill Shack (Limgrave West)"),
	0x00012947: G("Artist's Shack (Limgrave West)"),
	0x0001294C: G("Agheel Lake North (Limgrave West)"),
	0x0001294E: G("Church of Dragon Communion (Limgrave West)"),
	0x0001294F: G("Gatefront (Limgrave West)"),
	0x00012951: G("Seaside Ruins (Limgrave West)"),
	0x00012954: G("Murkwater Coast (Limgrave West)"),
	0x00012955: G("Saintsbridge (Limgrave West)"),
	0x00012956: G("Warmaster's Shack (Limgrave West)"),
	0x00012958: G("Waypoint Ruins Cellar (Limgrave West)"),
	0x0001297C: G("Isolated Merchant's Shack (Limgrave West)"),

	// --- Limgrave East ---
	0x00012948: G("Third Church of Marika (Limgrave East)"),
	0x00012949: G("Fort Haight West (Limgrave East)"),
	0x0001294A: G("Agheel Lake South (Limgrave East)"),
	0x00012952: G("Mistwood Outskirts (Limgrave East)"),
	0x00012957: G("Summonwater Village Outskirts (Limgrave East)"),

	// --- Weeping Peninsula ---
	0x00012976: G("Church of Pilgrimage (Weeping Peninsula)"),
	0x00012977: G("Castle Morne Rampart (Weeping Peninsula)"),
	0x00012978: G("Tombsward (Weeping Peninsula)"),
	0x00012979: G("South of the Lookout Tower (Weeping Peninsula)"),
	0x0001297A: G("Ailing Village Outskirts (Weeping Peninsula)"),
	0x0001297B: G("Beside the Crater-Pocked Glade (Weeping Peninsula)"),
	0x0001297D: G("Bridge of Sacrifice (Weeping Peninsula)"),
	0x0001297E: G("Castle Morne Lift (Weeping Peninsula)"),
	0x0001297F: G("Behind The Castle (Weeping Peninsula)"),
	0x00012980: G("Beside the Rampart Gaol (Weeping Peninsula)"),
	0x00012981: B("Morne Moangrave (Weeping Peninsula)"),
	0x00012982: G("Fourth Church of Marika (Weeping Peninsula)"),

	// --- Liurnia North ---
	0x000129A8: G("Lake-Facing Cliffs (Liurnia North)"),
	0x000129A9: G("Liurnia Lake Shore (Liurnia North)"),
	0x000129AA: G("Laskyar Ruins (Liurnia North)"),
	0x000129AB: G("Scenic Isle (Liurnia North)"),
	0x000129AC: G("Academy Gate Town (Liurnia North)"),
	0x000129AD: G("South Raya Lucaria Gate (Liurnia North)"),
	0x000129AE: G("Main Academy Gate (Liurnia North)"),
	0x000129B0: G("Bellum Church (Liurnia North)"),
	0x000129B1: G("Grand Lift of Dectus (Liurnia North)"),
	0x000129B3: G("Sorcerer's Isle (Liurnia North)"),
	0x000129B4: G("Northern Liurnia Lake Shore (Liurnia North)"),
	0x000129B8: G("Boilprawn Shack (Liurnia North)"),
	0x000129B9: G("Artist's Shack (Liurnia North)"),
	0x000129BB: G("Folly on the Lake (Liurnia North)"),
	0x000129BC: G("Village of the Albinaurics (Liurnia North)"),
	0x000129BD: G("Liurnia Highway North (Liurnia North)"),
	0x000129BE: G("Gate Town Bridge (Liurnia North)"),
	0x000129C1: G("Ruined Labyrinth (Liurnia North)"),
	0x000129C2: G("Mausoleum Compound (Liurnia North)"),
	0x000129C9: G("Gate Town North (Liurnia North)"),
	0x000129CC: G("Fallen Ruins of the Lake (Liurnia North)"),
	0x000129CF: G("Frenzied Flame Village Outskirts (Liurnia North)"),
	0x000129D0: G("Church of Inhibition (Liurnia North)"),
	0x000129D2: G("East Gate Bridge Trestle (Liurnia North)"),
	0x000129D4: G("Liurnia Highway South (Liurnia North)"),

	// --- Liurnia East ---
	0x000129AF: G("East Raya Lucaria Gate (Liurnia East)"),
	0x000129BF: G("Eastern Liurnia Lake Shore (Liurnia East)"),
	0x000129C0: G("Church of Vows (Liurnia East)"),
	0x000129C5: G("Ravine-Veiled Village (Liurnia East)"),
	0x000129CA: G("Eastern Tableland (Liurnia East)"),
	0x000129CB: G("The Ravine (Liurnia East)"),
	0x000129D3: G("Crystalline Woods (Liurnia East)"),
	0x000129D5: G("Jarburg (Liurnia East)"),

	// --- Liurnia West ---
	0x000129B2: G("Foot of the Four Belfries (Liurnia West)"),
	0x000129B5: G("Road to the Manor (Liurnia West)"),
	0x000129B6: G("Main Caria Manor Gate (Liurnia West)"),
	0x000129B7: B("Slumbering Wolf's Shack (Liurnia West)"),
	0x000129BA: G("Revenger's Shack (Liurnia West)"),
	0x000129C3: G("The Four Belfries (Liurnia West)"),
	0x000129C4: G("Ranni's Rise (Liurnia West)"),
	0x000129C6: G("Manor Upper Level (Liurnia West)"),
	0x000129C7: G("Manor Lower Level (Liurnia West)"),
	0x000129C8: B("Royal Moongazing Grounds (Liurnia West)"),
	0x000129CD: G("Converted Tower (Liurnia West)"),
	0x000129CE: G("Behind Caria Manor (Liurnia West)"),
	0x000129D1: G("Temple Quarter (Liurnia West)"),
	0x000129D7: G("Ranni's Chamber (Liurnia West)"),
	0x000129DA: G("Moonlight Altar (Liurnia West)"),
	0x000129DB: G("Cathedral of Manus Celes (Liurnia West)"),
	0x000129DC: G("Altar South (Liurnia West)"),

	// --- Altus Plateau ---
	0x00012A0C: G("Abandoned Coffin (Altus Plateau)"),
	0x00012A0D: G("Altus Plateau (Altus Plateau)"),
	0x00012A0E: G("Erdtree-Gazing Hill (Altus Plateau)"),
	0x00012A0F: G("Altus Highway Junction (Altus Plateau)"),
	0x00012A10: G("Forest-Spanning Greatbridge (Altus Plateau)"),
	0x00012A11: G("Rampartside Path (Altus Plateau)"),
	0x00012A12: G("Bower of Bounty (Altus Plateau)"),
	0x00012A13: G("Road of Iniquity Side Path (Altus Plateau)"),
	0x00012A14: G("Windmill Village (Altus Plateau)"),
	0x00012A15: G("Outer Wall Phantom Tree (Altus Plateau)"),
	0x00012A16: G("Minor Erdtree Church (Altus Plateau)"),
	0x00012A17: G("Hermit Merchant's Shack (Altus Plateau)"),
	0x00012A18: G("Outer Wall Battleground (Altus Plateau)"),
	0x00012A19: B("Windmill Heights (Altus Plateau)"),
	0x00012A1A: G("Capital Rampart (Altus Plateau)"),
	0x00012A20: G("Shaded Castle Ramparts (Altus Plateau)"),
	0x00012A21: G("Shaded Castle Inner Gate (Altus Plateau)"),
	0x00012A22: B("Castellan's Hall (Altus Plateau)"),

	// --- Mt. Gelmir ---
	0x00012A3E: G("Bridge of Iniquity (Mt. Gelmir)"),
	0x00012A3F: G("First Mt. Gelmir Campsite (Mt. Gelmir)"),
	0x00012A40: G("Ninth Mt. Gelmir Campsite (Mt. Gelmir)"),
	0x00012A41: G("Road of Iniquity (Mt. Gelmir)"),
	0x00012A42: G("Seethewater River (Mt. Gelmir)"),
	0x00012A43: G("Seethewater Terminus (Mt. Gelmir)"),
	0x00012A44: G("Craftsman's Shack (Mt. Gelmir)"),
	0x00012A45: G("Primeval Sorcerer Azur (Mt. Gelmir)"),

	// --- Caelid ---
	0x00012A70: G("Smoldering Church (Caelid)"),
	0x00012A71: G("Rotview Balcony (Caelid)"),
	0x00012A72: G("Fort Gael North (Caelid)"),
	0x00012A73: G("Caelem Ruins (Caelid)"),
	0x00012A74: G("Cathedral of Dragon Communion (Caelid)"),
	0x00012A75: G("Caelid Highway South (Caelid)"),
	0x00012A76: G("Aeonia Swamp Shore (Caelid)"),
	0x00012A77: G("Astray from Caelid Highway North (Caelid)"),
	0x00012A79: G("Smoldering Wall (Caelid)"),
	0x00012A7A: G("Deep Siofra Well (Caelid)"),
	0x00012A7B: G("Southern Aeonia Swamp Bank (Caelid)"),
	0x00012A7C: G("Heart of Aeonia (Caelid)"),
	0x00012A7D: G("Inner Aeonia (Caelid)"),
	0x00012A7E: G("Sellia Backstreets (Caelid)"),
	0x00012A7F: G("Chair-Crypt of Sellia (Caelid)"),
	0x00012A80: G("Sellia Under-Stair (Caelid)"),
	0x00012A81: G("Impassable Greatbridge (Caelid)"),
	0x00012A82: G("Church of the Plague (Caelid)"),
	0x00012A83: B("Redmane Castle Plaza (Caelid)"),
	0x00012A84: G("Chamber Outside the Plaza (Caelid)"),
	0x00012A86: B("Starscourge Radahn (Caelid)"),

	// --- Dragonbarrow ---
	0x00012AA2: G("Dragonbarrow West (Dragonbarrow)"),
	0x00012AA3: G("Isolated Merchant's Shack (Dragonbarrow)"),
	0x00012AA4: G("Dragonbarrow Fork (Dragonbarrow)"),
	0x00012AA5: G("Fort Faroth (Dragonbarrow)"),
	0x00012AA6: G("Bestial Sanctum (Dragonbarrow)"),
	0x00012AA7: G("Lenne's Rise (Dragonbarrow)"),
	0x00012AA8: G("Farum Greatbridge (Dragonbarrow)"),

	// --- Forbidden Lands ---
	0x00012AD4: G("Forbidden Lands (Forbidden Lands)"),
	0x00012AD6: G("Grand Lift of Rold (Forbidden Lands)"),

	// --- Mountaintops of the Giants West ---
	0x00012AD7: G("Ancient Snow Valley Ruins (Mountaintops of the Giants West)"),
	0x00012AD8: G("Freezing Lake (Mountaintops of the Giants West)"),
	0x00012AD9: G("First Church of Marika (Mountaintops of the Giants West)"),
	0x00012AE8: G("Whiteridge Road (Mountaintops of the Giants West)"),
	0x00012AE9: G("Snow Valley Ruins Overlook (Mountaintops of the Giants West)"),
	0x00012AEA: B("Castle Sol Main Gate (Mountaintops of the Giants West)"),
	0x00012AEB: G("Church of the Eclipse (Mountaintops of the Giants West)"),
	0x00012AEC: B("Castle Sol Rooftop (Mountaintops of the Giants West)"),

	// --- Mountaintops of the Giants East ---
	0x00012AD5: G("Zamor Ruins (Mountaintops of the Giants East)"),
	0x00012ADA: G("Giant's Gravepost (Mountaintops of the Giants East)"),
	0x00012ADB: G("Church of Repose (Mountaintops of the Giants East)"),
	0x00012ADC: G("Foot of the Forge (Mountaintops of the Giants East)"),
	0x00012ADD: B("Fire Giant (Mountaintops of the Giants East)"),
	0x00012ADE: B("Forge of the Giants (Mountaintops of the Giants East)"),

	// --- Consecrated Snowfield ---
	0x00012B06: G("Consecrated Snowfield (Consecrated Snowfield)"),
	0x00012B07: G("Inner Consecrated Snowfield (Consecrated Snowfield)"),
	0x00012B6C: G("Ordina, Liturgical Town (Consecrated Snowfield)"),
	0x00012B6D: G("Apostate Derelict (Consecrated Snowfield)"),

	// --- Shadow of the Erdtree — Gravesite Plain ---
	0x00012C00: G("Gravesite Plain (Gravesite Plain)"),
	0x00012C01: G("Scorched Ruins (Gravesite Plain)"),
	0x00012C02: G("Three-Path Cross (Gravesite Plain)"),
	0x00012C03: G("Main Gate Cross (Gravesite Plain)"),
	0x00012C04: G("Cliffroad Terminus (Gravesite Plain)"),
	0x00012C05: G("Greatbridge, North (Gravesite Plain)"),
	0x00012C0A: G("Pillar Path Cross (Gravesite Plain)"),
	0x00012C0B: G("Pillar Path Waypoint (Gravesite Plain)"),
	0x00012C0C: G("Ellac River Cave (Gravesite Plain)"),
	0x00012C0D: G("Castle Front (Gravesite Plain)"),
	0x00012C1E: G("Ellac River Downstream (Gravesite Plain)"),

	// --- Shadow of the Erdtree — Castle Ensis ---
	0x00012C15: G("Castle Ensis Checkpoint (Castle Ensis)"),
	0x00012C16: G("Castle-Lord's Chamber (Castle Ensis)"),
	0x00012C17: B("Ensis Moongazing Grounds (Castle Ensis)"),

	// --- Shadow of the Erdtree — Cerulean Coast ---
	0x00012C1F: G("Cerulean Coast (Cerulean Coast)"),
	0x00012C20: G("Cerulean Coast West (Cerulean Coast)"),
	0x00012C21: G("The Fissure (Cerulean Coast)"),
	0x00012C22: G("Finger Ruins of Rhia (Cerulean Coast)"),
	0x00012C23: G("Cerulean Coast Cross (Cerulean Coast)"),

	// --- Shadow of the Erdtree — Charo's Hidden Grave ---
	0x00012C29: G("Charo's Hidden Grave (Charo's Hidden Grave)"),

	// --- Shadow of the Erdtree — Jagged Peak ---
	0x00012C28: G("Grand Altar of Dragon Communion (Jagged Peak)"),
	0x00012C32: G("Foot of the Jagged Peak (Jagged Peak)"),
	0x00012C33: G("Jagged Peak Mountainside (Jagged Peak)"),
	0x00012C34: G("Jagged Peak Summit (Jagged Peak)"),
	0x00012C35: B("Rest of the Dread Dragon (Jagged Peak)"),

	// --- Shadow of the Erdtree — Abyssal Woods ---
	0x00012C3C: G("Abyssal Woods (Abyssal Woods)"),
	0x00012C3D: G("Divided Falls (Abyssal Woods)"),
	0x00012C3E: B("Forsaken Graveyard (Abyssal Woods)"),
	0x00012C3F: G("Woodland Trail (Abyssal Woods)"),
	0x00012C40: G("Church Ruins (Abyssal Woods)"),

	// --- Shadow of the Erdtree — Scadu Altus ---
	0x00012C64: G("Highroad Cross (Scadu Altus)"),
	0x00012C66: G("Moorth Ruins (Scadu Altus)"),
	0x00012C67: G("Bonny Village (Scadu Altus)"),
	0x00012C68: G("Bridge Leading to the Village (Scadu Altus)"),
	0x00012C69: G("Church District Highroad (Scadu Altus)"),
	0x00012C6A: G("Cathedral of Manus Metyr (Scadu Altus)"),
	0x00012C6B: G("Scadu Altus, West (Scadu Altus)"),
	0x00012C6C: G("Moorth Highway, South (Scadu Altus)"),
	0x00012C6D: G("Fort of Reprimand (Scadu Altus)"),
	0x00012C6E: G("Behind the Fort of Reprimand (Scadu Altus)"),
	0x00012C6F: G("Scaduview Cross (Scadu Altus)"),
	0x00012C74: G("Castle Watering Hole (Scadu Altus)"),
	0x00012C75: G("Recluses' River Upstream (Scadu Altus)"),
	0x00012C76: G("Recluses' River Downstream (Scadu Altus)"),

	// --- Shadow of the Erdtree — Rauh Base ---
	0x00012C70: G("Ancient Ruins Base (Rauh Base)"),
	0x00012C71: G("Temple Town Ruins (Rauh Base)"),
	0x00012C72: G("Ravine North (Rauh Base)"),

	// --- Shadow of the Erdtree — Scaduview ---
	0x00012C82: B("Scaduview (Scaduview)"),
	0x00012C83: G("Shadow Keep, Back Gate (Scaduview)"),
	0x00012C87: G("Hinterland (Scaduview)"),
	0x00012C88: G("Fingerstone Hill (Scaduview)"),
	0x00012C89: G("Hinterland Bridge (Scaduview)"),

	// --- Shadow of the Erdtree — Ancient Ruins of Rauh ---
	0x00012C8C: G("Viaduct Minor Tower (Ancient Ruins of Rauh)"),
	0x00012C8D: G("Rauh Ancient Ruins, East (Ancient Ruins of Rauh)"),
	0x00012C8E: G("Rauh Ancient Ruins, West (Ancient Ruins of Rauh)"),
	0x00012C8F: G("Church of the Bud, Main Entrance (Ancient Ruins of Rauh)"),
	0x00012C90: G("Ancient Ruins, Grand Stairway (Ancient Ruins of Rauh)"),
	0x00012C91: B("Church of the Bud (Ancient Ruins of Rauh)"),
	0x00012CA0: B("Scadutree Base (Shadow Keep)"),
}

