package data

import "testing"

// somberWeapons enumerates every weapon, catalyst, bow and shield that uses a
// Somber Smithing Stone reinforce path in regulation.bin
// (reinforceTypeId ∈ {2200, 2400, 3200, 3300, 8300, 8500},
// gemMountType=0). Each must carry MaxUpgrade=10 in the DB.
//
// Why this matters: the add-items pipeline treats MaxUpgrade==10 as the somber
// signal. When MaxUpgrade is left at 25 the frontend exposes infusion choices
// and the +11..+25 slider — both fabricate ItemIDs that don't exist in the
// game's parameter tables, which is the root cause of the Icon Shield bug
// (item disappears after the save is reloaded in-game).
//
// Source of truth: tmp/regulation-bin-dump/csv/EquipParamWeapon.csv joined
// against tmp/regulation-bin-dump/csv/ReinforceParamWeapon.csv. The full
// derivation lives in tmp/item-audit/.
var somberWeapons = []struct {
	ID          uint32 // hex form (must equal DecID)
	DecID       uint32
	Name        string
	Map         map[uint32]ItemData
	MapName     string
	Category    string
	ReinforceTy int // reinforceTypeId from regulation, for documentation
}{
	// --- Shields (5) ---
	{0x0148D3B0, 21550000, "Shield of Night", Shields, "Shields", "shields", 8500},
	{0x01CCD0C0, 30200000, "Coil Shield", Shields, "Shields", "shields", 8500},
	{0x01D9F020, 31060000, "Silver Mirrorshield", Shields, "Shields", "shields", 8300},
	{0x01E11C10, 31530000, "Golden Lion Shield", Shields, "Shields", "shields", 8300},
	{0x0175FE30, 24510000, "Lamenting Visage", Shields, "Shields", "shields", 2200},

	// --- Ranged / Catalysts (24) ---
	// Bows / Greatbows
	{0x0262CF30, 40030000, "Harp Bow", RangedAndCatalysts, "RangedAndCatalysts", "ranged_and_catalysts", 2200},
	{0x02721170, 41030000, "Erdtree Bow", RangedAndCatalysts, "RangedAndCatalysts", "ranged_and_catalysts", 2200},
	{0x02723880, 41040000, "Serpent Bow", RangedAndCatalysts, "RangedAndCatalysts", "ranged_and_catalysts", 2200},
	{0x027286A0, 41060000, "Pulley Bow", RangedAndCatalysts, "RangedAndCatalysts", "ranged_and_catalysts", 2200},
	{0x0272ADB0, 41070000, "Black Bow", RangedAndCatalysts, "RangedAndCatalysts", "ranged_and_catalysts", 2200},
	{0x02796470, 41510000, "Ansbach's Longbow", RangedAndCatalysts, "RangedAndCatalysts", "ranged_and_catalysts", 2200},
	{0x0280DE80, 42000000, "Lion Greatbow", RangedAndCatalysts, "RangedAndCatalysts", "ranged_and_catalysts", 2200},
	{0x02810590, 42010000, "Golem Greatbow", RangedAndCatalysts, "RangedAndCatalysts", "ranged_and_catalysts", 2200},
	{0x028153B0, 42030000, "Erdtree Greatbow", RangedAndCatalysts, "RangedAndCatalysts", "ranged_and_catalysts", 2200},
	// Crossbows / Hand Ballista
	{0x0290E410, 43050000, "Pulley Crossbow", RangedAndCatalysts, "RangedAndCatalysts", "ranged_and_catalysts", 3300},
	{0x02910B20, 43060000, "Full Moon Crossbow", RangedAndCatalysts, "RangedAndCatalysts", "ranged_and_catalysts", 3200},
	{0x0297C1E0, 43500000, "Repeating Crossbow", RangedAndCatalysts, "RangedAndCatalysts", "ranged_and_catalysts", 3200},
	{0x0291CE70, 43110000, "Crepus's Black-Key Crossbow", RangedAndCatalysts, "RangedAndCatalysts", "ranged_and_catalysts", 3200},
	{0x029F8A10, 44010000, "Jar Cannon", RangedAndCatalysts, "RangedAndCatalysts", "ranged_and_catalysts", 3200},
	// Glintstone staves
	{0x016116A0, 23140000, "Rotten Staff", RangedAndCatalysts, "RangedAndCatalysts", "ranged_and_catalysts", 2200},
	{0x01F82680, 33040000, "Crystal Staff", RangedAndCatalysts, "RangedAndCatalysts", "ranged_and_catalysts", 2400},
	{0x01F8E9D0, 33090000, "Carian Regal Scepter", RangedAndCatalysts, "RangedAndCatalysts", "ranged_and_catalysts", 2400},
	{0x01FB0CB0, 33230000, "Azur's Glintstone Staff", RangedAndCatalysts, "RangedAndCatalysts", "ranged_and_catalysts", 2400},
	{0x01FB33C0, 33240000, "Lusat's Glintstone Staff", RangedAndCatalysts, "RangedAndCatalysts", "ranged_and_catalysts", 2400},
	{0x01FBA8F0, 33270000, "Rotten Crystal Staff", RangedAndCatalysts, "RangedAndCatalysts", "ranged_and_catalysts", 2400},
	{0x01FF5270, 33510000, "Staff of the Great Beyond", RangedAndCatalysts, "RangedAndCatalysts", "ranged_and_catalysts", 2400},
	// Sacred Seals
	{0x0207B6E0, 34060000, "Golden Order Seal", RangedAndCatalysts, "RangedAndCatalysts", "ranged_and_catalysts", 2400},
	{0x0207DDF0, 34070000, "Erdtree Seal", RangedAndCatalysts, "RangedAndCatalysts", "ranged_and_catalysts", 2400},
	{0x02080500, 34080000, "Dragon Communion Seal", RangedAndCatalysts, "RangedAndCatalysts", "ranged_and_catalysts", 2400},

	// --- Melee armaments (16) ---
	{0x0026C1E0, 2540000, "Stone-Sheathed Sword", Weapons, "Weapons", "melee_armaments", 2200},
	{0x007270E0, 7500000, "Spirit Sword", Weapons, "Weapons", "melee_armaments", 2200},
	{0x00A8C320, 11060000, "Varr's Bouquet", Weapons, "Weapons", "melee_armaments", 2200},
	{0x00CDFE60, 13500000, "Serpent Flail", Weapons, "Weapons", "melee_armaments", 2200},
	{0x00D7C260, 14140000, "Stormhawk Axe", Weapons, "Weapons", "melee_armaments", 2200},
	{0x00ECA9F0, 15510000, "Bonny Butchering Knife", Weapons, "Weapons", "melee_armaments", 2200},
	{0x011A49A0, 18500000, "Spirit Glaive", Weapons, "Weapons", "melee_armaments", 2200},
	{0x0138CE20, 20500000, "Tooth Whip", Weapons, "Weapons", "melee_armaments", 2200},
	{0x01485E80, 21520000, "Poisoned Hand", Weapons, "Weapons", "melee_armaments", 2200},
	{0x01488590, 21530000, "Madding Hand", Weapons, "Weapons", "melee_armaments", 2200},
	{0x03AB06A0, 61540000, "Deadly Poison Perfume Bottle", Weapons, "Weapons", "melee_armaments", 2200},
	{0x010B5580, 17520000, "Barbed Staff-Spear", Weapons, "Weapons", "melee_armaments", 2200},
	{0x00A98670, 11110000, "Scepter of the All-Knowing", Weapons, "Weapons", "melee_armaments", 2200},
	{0x00BA2840, 12200000, "Devourer's Scepter", Weapons, "Weapons", "melee_armaments", 2200},
	{0x015F1AD0, 23010000, "Watchdog's Staff", Weapons, "Weapons", "melee_armaments", 2200},
	{0x01600530, 23070000, "Staff of the Avatar", Weapons, "Weapons", "melee_armaments", 2200},
}

func TestSomberWeapons_MaxUpgradeIs10(t *testing.T) {
	if got := len(somberWeapons); got != 45 {
		t.Fatalf("table size: got %d entries, want 45 (5 shields + 24 ranged/catalysts + 16 melee)", got)
	}
	for _, sw := range somberWeapons {
		t.Run(sw.Name, func(t *testing.T) {
			if sw.ID != sw.DecID {
				t.Errorf("table sanity: hex 0x%08X != decimal %d", sw.ID, sw.DecID)
			}
			item, ok := sw.Map[sw.ID]
			if !ok {
				t.Fatalf("0x%08X missing from %s map", sw.ID, sw.MapName)
			}
			if item.Name != sw.Name {
				t.Errorf("0x%08X name mismatch: DB=%q, want %q", sw.ID, item.Name, sw.Name)
			}
			if item.MaxUpgrade != 10 {
				t.Errorf("0x%08X (%s): MaxUpgrade=%d, want 10 (somber, reinforceTypeId=%d)",
					sw.ID, sw.Name, item.MaxUpgrade, sw.ReinforceTy)
			}
			if item.Category != sw.Category {
				t.Errorf("0x%08X (%s): Category=%q, want %q", sw.ID, sw.Name, item.Category, sw.Category)
			}
		})
	}
}

// standardControls guards that the somber fix did not accidentally touch
// items on the standard smithing path. Each of these has reinforceTypeId in
// the standard +25 ranges and must keep MaxUpgrade=25.
var standardControls = []struct {
	ID   uint32
	Name string
	Map  map[uint32]ItemData
}{
	{0x001E8480, "Longsword", Weapons},
	{0x00100590, "Crystal Knife", Weapons},
	{0x003D7E30, "Troll's Golden Sword", Weapons},
	{0x016EF950, "Ghostflame Torch", Shields},
	{0x02631D50, "Composite Bow", RangedAndCatalysts},
	{0x02719C40, "Longbow", RangedAndCatalysts},
	{0x02906EE0, "Light Crossbow", RangedAndCatalysts},
	{0x01FA9780, "Academy Glintstone Staff", RangedAndCatalysts},
	{0x0206CC80, "Finger Seal", RangedAndCatalysts},
}

func TestStandardControls_StayAt25(t *testing.T) {
	for _, sc := range standardControls {
		t.Run(sc.Name, func(t *testing.T) {
			item, ok := sc.Map[sc.ID]
			if !ok {
				t.Fatalf("0x%08X (%s) missing from map", sc.ID, sc.Name)
			}
			if item.Name != sc.Name {
				t.Errorf("0x%08X name mismatch: DB=%q, want %q", sc.ID, item.Name, sc.Name)
			}
			if item.MaxUpgrade != 25 {
				t.Errorf("0x%08X (%s): MaxUpgrade=%d, want 25 (standard infusable)", sc.ID, sc.Name, item.MaxUpgrade)
			}
		})
	}
}
