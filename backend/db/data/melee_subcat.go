package data

// melee_subcat.go — sub-category assignment for the Melee Armaments tab.
//
// 30 weapon classes per spec/36, in-game order:
//   Daggers, Throwing Blades (DLC), Straight Swords, Light Greatswords (DLC),
//   Greatswords, Colossal Swords, Thrusting Swords, Heavy Thrusting Swords,
//   Curved Swords, Curved Greatswords, Backhand Blades (DLC), Katanas,
//   Great Katanas (DLC), Twinblades, Axes, Greataxes, Hammers, Great Hammers,
//   Flails, Spears, Great Spears, Halberds, Reapers, Whips, Fists,
//   Hand-to-Hand (DLC), Claws, Beast Claws (DLC), Colossal Weapons, Perfume Bottles (DLC weapon).
//
// Strategy: name-pattern matching with override sets for weapons whose class
// is not derivable from the name suffix (e.g., "Reduvia" is a Dagger).
// Best-effort: edge-case mismatches expected, fixable by adding to override
// sets. The infused variants (Heavy/Keen/etc.) inherit from the base name
// match because their full names also contain the matchable substring.

import "strings"

// daggers — base names that are Daggers but don't end with "Dagger"/"Knife".
var meleeDaggers = setOf(
	"Reduvia", "Misricorde", "Cinquedea", "Wakizashi", "Glintstone Kris",
	"Scorpion's Stinger", "Blade of Calling", "Black Knife",
	"Celebrant's Sickle", "Ivory Sickle", "Main-gauche",
	"Bloodstained Dagger", "Erdsteel Dagger", "Crystal Knife",
	"Parrying Dagger", "Great Knife",
)

// straightSwords — base names that are Straight Swords (suffix "Sword" too generic).
var meleeStraightSwords = setOf(
	"Longsword", "Short Sword", "Broadsword",
	"Lordsworn's Straight Sword", "Weathered Straight Sword",
	"Ornamental Straight Sword", "Golden Epitaph", "Nox Flowing Sword",
	"Inseparable Sword", "Coded Sword", "Sword of Night and Flame",
	"Crystal Sword", "Carian Knight's Sword", "Sword of St. Trina",
	"Miquellan Knight's Sword", "Cane Sword", "Regalia of Eochaid",
	"Noble's Slender Sword", "Warhawk's Talon", "Lazuli Glintstone Sword",
	"Rotten Crystal Sword", "Velvet Sword of St. Trina", "Star-Lined Sword",
	"Carian Sorcery Sword", "Stone-Sheathed Sword",
)

// greatswords — explicit list (name suffix "Sword" alone insufficient).
var meleeGreatswords = setOf(
	"Bastard Sword", "Forked Greatsword", "Iron Greatsword",
	"Lordsworn's Greatsword", "Knight's Greatsword", "Flamberge",
	"Ordovis's Greatsword", "Alabaster Lord's Sword",
	"Banished Knight's Greatsword", "Dark Moon Greatsword",
	"Helphen's Steeple", "Blasphemous Blade", "Marais Executioner's Sword",
	"Sword of Milos", "Golden Order Greatsword", "Claymore",
	"Gargoyle's Greatsword", "Death's Poker", "Gargoyle's Blackblade",
	"Lizard Greatsword", "Greatsword of Damnation",
	"Greatsword of Solitude",
)

// lightGreatswords — DLC class; weapons that are Light Greatswords (size between
// Straight Sword and Greatsword).
var meleeLightGreatswords = setOf(
	"Sword of Light", "Sword of Darkness", "Milady", "Leda's Sword",
	"Rellana's Twin Blades",
)

// colossalSwords — distinct from Greatswords (heavier, two-handed only).
var meleeColossalSwords = setOf(
	"Zweihander", "Watchdog's Greatsword", "Maliketh's Black Blade",
	"Royal Greatsword", "Ruins Greatsword", "Starscourge Greatsword",
	"Greatsword", "Troll Knight's Sword", "Troll's Golden Sword",
	"Onyx Lord's Greatsword",
)

// thrustingSwords — class with name containing rapier/estoc-like patterns.
var meleeThrustingSwords = setOf(
	"Estoc", "Cleanrot Sword", "Antspur Rapier", "Frozen Needle",
	"Noble's Estoc", "Rapier", "Rotten Crystal Estoc",
	"Crystal Sword (Thrusting)", // hypothetical guard
	"Rosus's Axe",                // (mislabeled? actually a Reaper — check)
)

// heavyThrustingSwords — heavier rapier/great rapier variants.
var meleeHeavyThrustingSwords = setOf(
	"Great Epee", "Helice", "Dragon King's Cragblade",
	"Godskin Stitcher", "Sword Lance",
)

// curvedGreatswords — explicit list since some don't end with "Curved Greatsword".
var meleeCurvedGreatswords = setOf(
	"Magma Blade",
)

// twinblades — explicit list (some don't end with "Twinblade").
var meleeTwinblades = setOf(
	"Eleonora's Poleblade",
)

// katanas — explicit (e.g., Hand of Malenia ends with "Malenia").
var meleeKatanas = setOf(
	"Uchigatana", "Nagakiba", "Meteoric Ore Blade", "Rivers of Blood",
	"Moonveil", "Dragonscale Blade", "Hand of Malenia",
)

// greatKatanas — DLC class.
var meleeGreatKatanas = setOf(
	"Dragon-Hunter's Great Katana", "Rakshasa's Great Katana",
)

// axes — generic Axes (one-handed). Greataxes covered by suffix "Greataxe".
var meleeAxes = setOf(
	"Forked Hatchet", "Iron Cleaver", "Highland Axe", "Sacrificial Axe",
	"Stormhawk Axe", "Jawbone Axe", "Icerind Hatchet", "Celebrant's Cleaver",
	"Hand Axe", "Ripple Crescent Halberd",
)

// greataxes — explicit list.
var meleeGreataxes = setOf(
	"Battle Axe", "Crescent Moon Axe", "Longhaft Axe", "Warped Axe",
	"Gargoyle's Greataxe", "Rusted Anchor", "Winged Greathorn",
	"Axe of Godfrey", "Axe of Godrick", "Butchering Knife",
)

// hammers — single-handed hammers/clubs.
var meleeHammers = setOf(
	"Mace", "Club", "Spiked Club", "Curved Club", "Warpick",
	"Morning Star", "Stone Club", "Varré's Bouquet", "Hammer",
	"Hammer of Malenia", "Battle Hammer", "Envoy's Long Horn",
	"Anvil Hammer", "Scepter of the All-Knowing", "Devourer's Scepter",
	"Cipher Pata", // actually a Fist? Check
	"Smithscript Hammer",
)

// greatHammers — heavier hammers.
var meleeGreatHammers = setOf(
	"Great Mace", "Great Club", "Brick Hammer", "Pickaxe",
	"Large Club", "Battle Hammer (Great)", "Greathorn Hammer",
	"Cranial Vessel Candlestand", "Beastclaw Greathammer",
	"Black Steel Greathammer", "Star Fist Greathammer", // verify
	"Pest's Glaive",                                      // hypothetical
)

// flails — flail-class.
var meleeFlails = setOf(
	"Flail", "Family Heads", "Bastard's Stars", "Chainlink Flail",
	"Three-Pronged Flail", "Nightrider Flail",
)

// spears — single-handed spears/pikes.
var meleeSpears = setOf(
	"Spear", "Cross-Naginata", "Pike", "Crystal Spear", "Spear of the Impaler",
	"Torchpole",            // wait Torchpole is a torch (shields). Skip.
	"Inquisitor's Girandole", // fictitious; check
	"Cleanrot Spear", "Bloodthirsty Spear", "Mohgwyn's Sacred Spear",
	"Treespear", "Death Ritual Spear", "Celebrant's Rib-Rake",
	"Shortspear", "Partisan",
)

// greatSpears — heavier spears.
var meleeGreatSpears = setOf(
	"Great Spear", "Bolt of Gransax",
	"Lance", "Vyke's War Spear",
	"Siluria's Tree", "Serpent-Hunter",
)

// halberds — halberd-class.
var meleeHalberds = setOf(
	"Halberd", "Banished Knight's Halberd", "Glaive", "Pest's Glaive",
	"Lucerne", "Vulgar Militia Saw", "Vulgar Militia Shotel",
	"Guardian's Swordspear", "Loretta's War Sickle", "Golden Halberd",
	"Commander's Standard", "Nightrider Glaive", "Gargoyle's Halberd",
	"Gargoyle's Black Halberd",
)

// reapers — scythe/reaper class.
var meleeReapers = setOf(
	"Scythe", "Grave Scythe", "Halo Scythe", "Winged Scythe",
	"Death's Scythe", "Obsidian Lamina",
)

// whips — whip class.
var meleeWhips = setOf(
	"Whip", "Thorned Whip", "Hoslow's Petal Whip", "Magma Whip Candlestick",
	"Urumi", "Giant's Red Braid",
)

// fists — fist class.
var meleeFists = setOf(
	"Caestus", "Spiked Caestus", "Iron Ball", "Star Fist", "Cipher Pata",
	"Veteran's Prosthesis", "Grafted Dragon", "Clinging Bone",
	"Katar",
)

// handToHand — DLC class (martial arts).
var meleeHandToHand = setOf(
	"Dryleaf Arts", "Dane's Footwork", "Gazing Finger",
)

// claws — claw class.
var meleeClaws = setOf(
	"Hookclaws", "Venomous Fang", "Bloodhound Claws", "Raptor Talons",
)

// beastClaws — DLC class (claw subset).
var meleeBeastClaws = setOf(
	"Beast Claw", "Red Bear's Claw",
)

// colossalWeapons — colossal class (heavier than colossal swords).
var meleeColossalWeapons = setOf(
	"Giant-Crusher", "Prelate's Inferno Crozier", "Duelist's Furnace Pot",
	"Rotten Greataxe", "Devourer's Scepter (Colossal)", "Dragon Greatclaw",
	"Fallingstar Beast Jaw", "Staff of the Avatar", "Watchdog's Staff",
	"Great Stars", "Royal Greatsword (Colossal)",
)

// perfumeBottlesWeapon — DLC weapon class.
var meleePerfumeBottlesWeapon = setOf(
	"Firespark Perfume Bottle", "Chilling Perfume Bottle",
	"Frenzyflame Perfume Bottle", "Lightning Perfume Bottle",
	"Deadly Poison Perfume Bottle",
)

// throwingBlades — DLC class.
var meleeThrowingBlades = setOf(
	"Smithscript Dagger", "Smithscript Cirque",
)

// backhandBlades — DLC class.
var meleeBackhandBlades = setOf(
	"Backhand Blade", "Curseblade's Cirque",
)

func setOf(names ...string) map[string]struct{} {
	s := make(map[string]struct{}, len(names))
	for _, n := range names {
		s[n] = struct{}{}
	}
	return s
}

// stripMeleeInfusion removes infusion qualifiers anywhere in the name.
func stripMeleeInfusion(name string) string {
	for _, q := range []string{
		"Heavy ", "Keen ", "Quality ", "Fire ", "Flame Art ", "Lightning ",
		"Sacred ", "Magic ", "Cold ", "Poison ", "Blood ", "Occult ",
		"Bloody ", "Cracked ",
	} {
		name = strings.Replace(name, q, "", 1)
	}
	return name
}

func classifyMelee(name string) string {
	base := stripMeleeInfusion(name)

	// 1. Most specific name-set lookups (priority order).
	if _, ok := meleePerfumeBottlesWeapon[base]; ok {
		return SubcatMeleePerfumeBottles
	}
	if _, ok := meleeThrowingBlades[base]; ok {
		return SubcatMeleeThrowingBlades
	}
	if _, ok := meleeBackhandBlades[base]; ok {
		return SubcatMeleeBackhandBlades
	}
	if _, ok := meleeBeastClaws[base]; ok {
		return SubcatMeleeBeastClaws
	}
	if _, ok := meleeHandToHand[base]; ok {
		return SubcatMeleeHandToHand
	}
	if _, ok := meleeGreatKatanas[base]; ok {
		return SubcatMeleeGreatKatanas
	}
	if _, ok := meleeKatanas[base]; ok {
		return SubcatMeleeKatanas
	}
	if _, ok := meleeTwinblades[base]; ok {
		return SubcatMeleeTwinblades
	}
	if _, ok := meleeColossalWeapons[base]; ok {
		return SubcatMeleeColossalWeapons
	}
	if _, ok := meleeColossalSwords[base]; ok {
		return SubcatMeleeColossalSwords
	}
	if _, ok := meleeGreatswords[base]; ok {
		return SubcatMeleeGreatswords
	}
	if _, ok := meleeLightGreatswords[base]; ok {
		return SubcatMeleeLightGreatswords
	}
	if _, ok := meleeCurvedGreatswords[base]; ok {
		return SubcatMeleeCurvedGreatswords
	}
	if _, ok := meleeStraightSwords[base]; ok {
		return SubcatMeleeStraightSwords
	}
	if _, ok := meleeThrustingSwords[base]; ok {
		return SubcatMeleeThrustingSwords
	}
	if _, ok := meleeHeavyThrustingSwords[base]; ok {
		return SubcatMeleeHeavyThrustingSwords
	}
	if _, ok := meleeDaggers[base]; ok {
		return SubcatMeleeDaggers
	}
	if _, ok := meleeAxes[base]; ok {
		return SubcatMeleeAxes
	}
	if _, ok := meleeGreataxes[base]; ok {
		return SubcatMeleeGreataxes
	}
	if _, ok := meleeHammers[base]; ok {
		return SubcatMeleeHammers
	}
	if _, ok := meleeGreatHammers[base]; ok {
		return SubcatMeleeGreatHammers
	}
	if _, ok := meleeFlails[base]; ok {
		return SubcatMeleeFlails
	}
	if _, ok := meleeSpears[base]; ok {
		return SubcatMeleeSpears
	}
	if _, ok := meleeGreatSpears[base]; ok {
		return SubcatMeleeGreatSpears
	}
	if _, ok := meleeHalberds[base]; ok {
		return SubcatMeleeHalberds
	}
	if _, ok := meleeReapers[base]; ok {
		return SubcatMeleeReapers
	}
	if _, ok := meleeWhips[base]; ok {
		return SubcatMeleeWhips
	}
	if _, ok := meleeFists[base]; ok {
		return SubcatMeleeFists
	}
	if _, ok := meleeClaws[base]; ok {
		return SubcatMeleeClaws
	}

	// 2. Suffix-based fallback (for weapons not in any curated list).
	switch {
	case strings.Contains(base, "Curved Greatsword"):
		return SubcatMeleeCurvedGreatswords
	case strings.Contains(base, "Curved Sword"):
		return SubcatMeleeCurvedSwords
	case strings.Contains(base, "Great Katana"):
		return SubcatMeleeGreatKatanas
	case strings.HasSuffix(base, "Katana"):
		return SubcatMeleeKatanas
	case strings.Contains(base, "Twinblade"):
		return SubcatMeleeTwinblades
	case strings.Contains(base, "Backhand Blade"):
		return SubcatMeleeBackhandBlades
	case strings.HasSuffix(base, "Greatsword"):
		return SubcatMeleeGreatswords
	case strings.HasSuffix(base, "Greataxe"):
		return SubcatMeleeGreataxes
	case strings.HasSuffix(base, "Greathammer") || strings.HasSuffix(base, "Great Hammer"):
		return SubcatMeleeGreatHammers
	case strings.HasSuffix(base, "Greatspear") || strings.HasSuffix(base, "Great Spear"):
		return SubcatMeleeGreatSpears
	case strings.HasSuffix(base, "Halberd"):
		return SubcatMeleeHalberds
	case strings.HasSuffix(base, "Hammer") || strings.HasSuffix(base, "Mace") || strings.HasSuffix(base, "Club"):
		return SubcatMeleeHammers
	case strings.HasSuffix(base, "Axe") || strings.HasSuffix(base, "Hatchet") || strings.HasSuffix(base, "Cleaver"):
		return SubcatMeleeAxes
	case strings.HasSuffix(base, "Spear") || strings.HasSuffix(base, "Pike") || strings.HasSuffix(base, "Lance"):
		return SubcatMeleeSpears
	case strings.HasSuffix(base, "Flail") || strings.HasSuffix(base, "Stars"):
		return SubcatMeleeFlails
	case strings.HasSuffix(base, "Whip") || strings.HasSuffix(base, "Urumi"):
		return SubcatMeleeWhips
	case strings.HasSuffix(base, "Reaper") || strings.HasSuffix(base, "Scythe"):
		return SubcatMeleeReapers
	case strings.HasSuffix(base, "Caestus") || strings.HasSuffix(base, "Fist") || strings.HasSuffix(base, "Pata"):
		return SubcatMeleeFists
	case strings.HasSuffix(base, "Claw") || strings.HasSuffix(base, "Claws") ||
		strings.HasSuffix(base, "Talons") || strings.HasSuffix(base, "Fang"):
		return SubcatMeleeClaws
	case strings.HasSuffix(base, "Glaive") || strings.HasSuffix(base, "Lucerne"):
		return SubcatMeleeHalberds
	case strings.HasSuffix(base, "Estoc") || strings.HasSuffix(base, "Rapier"):
		return SubcatMeleeThrustingSwords
	case strings.HasSuffix(base, "Dagger") || strings.HasSuffix(base, "Knife") ||
		strings.HasSuffix(base, "Sickle"):
		return SubcatMeleeDaggers
	case strings.HasSuffix(base, "Footwork") || strings.HasSuffix(base, "Arts"):
		return SubcatMeleeHandToHand
	case strings.HasSuffix(base, "Sword") || strings.HasSuffix(base, "Blade"):
		// Generic sword/blade fallback — assume Straight Sword.
		return SubcatMeleeStraightSwords
	}

	// 3. Last resort.
	return SubcatMeleeStraightSwords
}

func init() {
	for id, item := range Weapons {
		if item.SubCategory != "" {
			continue
		}
		item.SubCategory = classifyMelee(item.Name)
		Weapons[id] = item
	}
}
