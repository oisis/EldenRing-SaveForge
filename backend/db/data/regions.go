package data

// RegionData describes an "unlocked region" entry stored in the per-slot Regions struct
// (count: u32 followed by count * u32 region IDs). Each ID corresponds to a discrete
// map area. The list controls:
//   - Invasion eligibility (PvP / NPC invaders) for that area
//   - Blue summons (Recusant Henricus, Bloody Finger questlines)
//   - The "You have entered <X>" map label after teleport
//
// Source: er-save-manager/src/er_save_manager/data/regions.py — community-researched IDs.
type RegionData struct {
	Name string
	Area string
}

// Regions maps region IDs to their human-readable name and grouping area.
var Regions = map[uint32]RegionData{
	// ============================================================
	// Limgrave
	// ============================================================
	6100000: {Name: "The First Step", Area: "Limgrave"},
	6100001: {Name: "Seaside Ruins", Area: "Limgrave"},
	6100002: {Name: "Agheel Lake North", Area: "Limgrave"},
	6100003: {Name: "Summonwater Village Outskirts", Area: "Limgrave"},
	6100004: {Name: "Mistwood Outskirts", Area: "Limgrave"},
	6100090: {Name: "Church of Dragon Communion", Area: "Limgrave"},
	6101000: {Name: "Stormhill Shack", Area: "Limgrave"},
	6101010: {Name: "Margit, the Fell Omen", Area: "Limgrave"},
	6102000: {Name: "Weeping Peninsula (West)", Area: "Limgrave"},
	6102001: {Name: "Weeping Peninsula (East)", Area: "Limgrave"},
	6102002: {Name: "Castle Morne", Area: "Limgrave"},

	// ============================================================
	// Liurnia of the Lakes
	// ============================================================
	6200000: {Name: "Lake-Facing Cliffs", Area: "Liurnia"},
	6200001: {Name: "Liurnia Highway South", Area: "Liurnia"},
	6200002: {Name: "Liurnia Lake Shore", Area: "Liurnia"},
	6200004: {Name: "Eastern Tableland", Area: "Liurnia"},
	6200005: {Name: "Crystalline Woods", Area: "Liurnia"},
	6200006: {Name: "The Ravine", Area: "Liurnia"},
	6200007: {Name: "Main Caria Manor Gate", Area: "Liurnia"},
	6200008: {Name: "Behind Caria Manor", Area: "Liurnia"},
	6200010: {Name: "Royal Moongazing Grounds", Area: "Liurnia"},
	6200090: {Name: "Grand Lift of Dectus", Area: "Liurnia"},
	6201000: {Name: "Bellum Church", Area: "Liurnia"},
	6202000: {Name: "Moonlight Altar", Area: "Liurnia"},

	// ============================================================
	// Altus Plateau
	// ============================================================
	6300000: {Name: "Stormcaller Church", Area: "Altus Plateau"},
	6300001: {Name: "The Shaded Castle", Area: "Altus Plateau"},
	6300002: {Name: "Altus Highway Junction", Area: "Altus Plateau"},
	6300004: {Name: "Dominula, Windmill Village", Area: "Altus Plateau"},
	6300005: {Name: "Rampartside Path", Area: "Altus Plateau"},
	6300030: {Name: "Castellan's Hall", Area: "Altus Plateau"},
	6301000: {Name: "Capital Outskirts", Area: "Altus Plateau"},
	6301090: {Name: "Capital Rampart", Area: "Altus Plateau"},
	6302000: {Name: "Ninth Mt. Gelmir Campsite", Area: "Mt. Gelmir"},
	6302001: {Name: "Road of Iniquity", Area: "Mt. Gelmir"},
	6302002: {Name: "Seethewater Terminus", Area: "Mt. Gelmir"},

	// ============================================================
	// Caelid
	// ============================================================
	6400000: {Name: "Caelid Highway South", Area: "Caelid"},
	6400001: {Name: "Caelem Ruins", Area: "Caelid"},
	6400002: {Name: "Chamber Outside the Plaza", Area: "Caelid"},
	6400010: {Name: "Redmane Castle Plaza", Area: "Caelid"},
	6400020: {Name: "Chair-Crypt of Sellia", Area: "Caelid"},
	6400040: {Name: "Starscourge Radahn", Area: "Caelid"},
	6401000: {Name: "Swamp of Aeonia", Area: "Caelid"},
	6402000: {Name: "Dragonbarrow West", Area: "Caelid"},
	6402001: {Name: "Bestial Sanctum", Area: "Caelid"},

	// ============================================================
	// Mountaintops of the Giants / Forbidden Lands / Snowfield
	// ============================================================
	6500000: {Name: "Forbidden Lands", Area: "Mountaintops"},
	6500090: {Name: "Grand Lift of Rold", Area: "Mountaintops"},
	6501000: {Name: "Zamor Ruins", Area: "Mountaintops"},
	6501001: {Name: "Central Mountaintops", Area: "Mountaintops"},
	6502000: {Name: "Consecrated Snowfield", Area: "Mountaintops"},
	6502001: {Name: "Inner Consecrated Snowfield", Area: "Mountaintops"},
	6502002: {Name: "Ordina, Liturgical Town", Area: "Mountaintops"},

	// ============================================================
	// Underground (Siofra / Ainsel / Nokron / Nokstella)
	// ============================================================
	6600000: {Name: "Siofra River", Area: "Underground"},
	6600001: {Name: "Ainsel River Main", Area: "Underground"},
	6600002: {Name: "Ainsel River Downstream", Area: "Underground"},
	6600003: {Name: "Ainsel River Downstream (deep)", Area: "Underground"},
	6600004: {Name: "Lake of Rot", Area: "Underground"},
	6600005: {Name: "Deeproot Depths", Area: "Underground"},
	6600006: {Name: "Nokron, Eternal City", Area: "Underground"},
	6600007: {Name: "Nokstella, Eternal City", Area: "Underground"},

	// ============================================================
	// Crumbling Farum Azula
	// ============================================================
	6700000: {Name: "Crumbling Farum Azula", Area: "Farum Azula"},
	6700001: {Name: "Dragon Temple", Area: "Farum Azula"},
	6700002: {Name: "Beside the Great Bridge", Area: "Farum Azula"},

	// ============================================================
	// Miquella's Haligtree
	// ============================================================
	6800000: {Name: "Miquella's Haligtree", Area: "Haligtree"},

	// ============================================================
	// Shadow of the Erdtree (DLC) — Land of Shadow
	// ============================================================
	6900000: {Name: "Gravesite Plain", Area: "Land of Shadow"},
	6900001: {Name: "Scadu Altus", Area: "Land of Shadow"},
	6900002: {Name: "Abyssal Woods", Area: "Land of Shadow"},
	6900003: {Name: "Ancient Ruins of Rauh", Area: "Land of Shadow"},
	6900004: {Name: "Cerulean Coast / Jagged Peak", Area: "Land of Shadow"},
	6900005: {Name: "Enir-Ilim", Area: "Land of Shadow"},
	6900006: {Name: "Shadow Keep", Area: "Land of Shadow"},

	// ============================================================
	// Legacy Dungeons — Stormveil Castle
	// ============================================================
	1000000: {Name: "Stormveil Castle", Area: "Legacy Dungeons"},
	1000001: {Name: "Stormveil Main Gate", Area: "Legacy Dungeons"},
	1000003: {Name: "Rampart Tower", Area: "Legacy Dungeons"},
	1000005: {Name: "Liftside Chamber", Area: "Legacy Dungeons"},
	1000006: {Name: "Gateside Chamber", Area: "Legacy Dungeons"},

	// ============================================================
	// Legacy Dungeons — Leyndell, Royal Capital
	// ============================================================
	1100000: {Name: "Leyndell, Royal Capital", Area: "Legacy Dungeons"},
	1100001: {Name: "Queen's Bedchamber", Area: "Legacy Dungeons"},
	1100010: {Name: "Leyndell - Erdtree Sanctuary", Area: "Legacy Dungeons"},
	1100012: {Name: "East Capital Rampart", Area: "Legacy Dungeons"},
	1100013: {Name: "Avenue Balcony", Area: "Legacy Dungeons"},
	1100015: {Name: "Lower Capital Church", Area: "Legacy Dungeons"},
	1100016: {Name: "West Capital Rampart", Area: "Legacy Dungeons"},
	1100017: {Name: "Divine Bridge", Area: "Legacy Dungeons"},
	1101000: {Name: "Roundtable Hold", Area: "Legacy Dungeons"},

	// ============================================================
	// Legacy Dungeons — Leyndell, Ashen Capital (post-Farum Azula)
	// ============================================================
	1105000: {Name: "Ashen Elden Throne", Area: "Legacy Dungeons"},
	1105001: {Name: "Ashen Queen's Bedchamber", Area: "Legacy Dungeons"},
	1105011: {Name: "Leyndell, Capital of Ash", Area: "Legacy Dungeons"},
	1105092: {Name: "Ashen Divine Bridge", Area: "Legacy Dungeons"},

	// ============================================================
	// Legacy Dungeons — Academy of Raya Lucaria
	// ============================================================
	1400000: {Name: "Academy of Raya Lucaria", Area: "Legacy Dungeons"},
	1400010: {Name: "Debate Parlor", Area: "Legacy Dungeons"},
	1400011: {Name: "Main Academy Gate", Area: "Legacy Dungeons"},
	1400013: {Name: "Church of the Cuckoo", Area: "Legacy Dungeons"},
	1400015: {Name: "School House Classroom", Area: "Legacy Dungeons"},

	// ============================================================
	// Legacy Dungeons — Volcano Manor
	// ============================================================
	1600000: {Name: "Volcano Manor", Area: "Legacy Dungeons"},
	1600006: {Name: "Audience Pathway", Area: "Legacy Dungeons"},
	1600010: {Name: "Temple of Eiglay", Area: "Legacy Dungeons"},
	1600012: {Name: "Volcano Manor (interior)", Area: "Legacy Dungeons"},
	1600014: {Name: "Prison Town Church", Area: "Legacy Dungeons"},
	1600016: {Name: "Guest Hall", Area: "Legacy Dungeons"},
	1600020: {Name: "Abductor Virgin", Area: "Legacy Dungeons"},
	1600022: {Name: "Subterranean Inquisition Chamber", Area: "Legacy Dungeons"},

	// ============================================================
	// Legacy Dungeons — Tutorial / Endgame
	// ============================================================
	1800001: {Name: "Stranded Graveyard", Area: "Legacy Dungeons"},
	1800090: {Name: "Cave of Knowledge", Area: "Legacy Dungeons"},
	1900000: {Name: "Stone Platform", Area: "Legacy Dungeons"},
	1900001: {Name: "Elden Beast", Area: "Legacy Dungeons"},
}

// IsDLCRegion reports whether a region ID belongs to the Shadow of the Erdtree DLC.
func IsDLCRegion(id uint32) bool {
	return id >= 6900000 && id <= 6999999
}
