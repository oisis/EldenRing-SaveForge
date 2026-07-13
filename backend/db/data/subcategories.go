package data

// Sub-category constants — 1:1 with in-game inventory sub-grouping.
// See spec/36-inventory-categories-game-order.md for the canonical map.
//
// Categories WITHOUT sub-grouping (empty SubCategory by design):
//   Ashes, Crafting Materials, Sorceries, Incantations, Ashes of War,
//   Head, Chest, Arms, Legs, Talismans

// Tools (13 sub-groups, in-game order)
const (
	SubcatToolsFlasks        = "Flasks"
	SubcatToolsConsumables   = "Consumables"
	SubcatToolsThrowingPots  = "Throwing Pots"
	SubcatToolsPerfumeArts   = "Perfume Arts"
	SubcatToolsThrowables    = "Throwables"
	SubcatToolsCatalystTools = "Catalyst Tools"
	SubcatToolsGrease        = "Grease"
	SubcatToolsReusable      = "Reusable Tools"
	SubcatToolsMisc          = "Miscellaneous Tools"
	SubcatToolsQuest         = "Quest Tools"
	SubcatToolsGoldenRunes   = "Golden Runes"
	SubcatToolsRemembrances  = "Remembrances"
	SubcatToolsMultiplayer   = "Multiplayer Items"
)

// Bolstering Materials (6 sub-groups)
const (
	SubcatBolsteringFlaskEnhancers       = "Flask Enhancers"
	SubcatBolsteringShadowRealmBlessings = "Shadow Realm Blessings"
	SubcatBolsteringSmithingStones       = "Smithing Stones"
	SubcatBolsteringSomberstones         = "Somberstones"
	SubcatBolsteringGraveGlovewort       = "Grave Glovewort"
	SubcatBolsteringGhostGlovewort       = "Ghost Glovewort"
)

// Key Items (9 sub-groups, in-game order)
const (
	SubcatKeyActiveGreatRunes  = "Active Great Runes"
	SubcatKeyCrystalTears      = "Crystal Tears"
	SubcatKeyContainers        = "Containers + Slot Upgrades"
	SubcatKeyInactiveRunesKeys = "Inactive Great Runes + Keys + Medallions"
	SubcatKeyDLCKeys           = "DLC Keys"
	SubcatKeyLarvalDeathroot   = "Larval Tears + Deathroot + Lost Ashes of War"
	SubcatKeyCookbooks         = "Cookbooks"
	SubcatKeyWorldMaps         = "World Maps"
	SubcatKeySpellScrolls      = "Sorcery Scrolls + Incantation Scrolls"
)

// Melee Armaments (29 weapon classes — base + DLC interleaved)
const (
	SubcatMeleeDaggers              = "Daggers"
	SubcatMeleeThrowingBlades       = "Throwing Blades"
	SubcatMeleeStraightSwords       = "Straight Swords"
	SubcatMeleeLightGreatswords     = "Light Greatswords"
	SubcatMeleeGreatswords          = "Greatswords"
	SubcatMeleeColossalSwords       = "Colossal Swords"
	SubcatMeleeThrustingSwords      = "Thrusting Swords"
	SubcatMeleeHeavyThrustingSwords = "Heavy Thrusting Swords"
	SubcatMeleeCurvedSwords         = "Curved Swords"
	SubcatMeleeCurvedGreatswords    = "Curved Greatswords"
	SubcatMeleeBackhandBlades       = "Backhand Blades"
	SubcatMeleeKatanas              = "Katanas"
	SubcatMeleeGreatKatanas         = "Great Katanas"
	SubcatMeleeTwinblades           = "Twinblades"
	SubcatMeleeAxes                 = "Axes"
	SubcatMeleeGreataxes            = "Greataxes"
	SubcatMeleeHammers              = "Hammers"
	SubcatMeleeGreatHammers         = "Great Hammers"
	SubcatMeleeFlails               = "Flails"
	SubcatMeleeSpears               = "Spears"
	SubcatMeleeGreatSpears          = "Great Spears"
	SubcatMeleeHalberds             = "Halberds"
	SubcatMeleeReapers              = "Reapers"
	SubcatMeleeWhips                = "Whips"
	SubcatMeleeFists                = "Fists"
	SubcatMeleeHandToHand           = "Hand-to-Hand"
	SubcatMeleeClaws                = "Claws"
	SubcatMeleeBeastClaws           = "Beast Claws"
	SubcatMeleeColossalWeapons      = "Colossal Weapons"
	SubcatMeleePerfumeBottles       = "Perfume Bottles"
)

// Ranged Weapons / Catalysts (7 sub-groups)
const (
	SubcatRangedBows             = "Bows"
	SubcatRangedLightBows        = "Light Bows"
	SubcatRangedGreatbows        = "Greatbows"
	SubcatRangedCrossbows        = "Crossbows"
	SubcatRangedBallistas        = "Ballistas"
	SubcatRangedGlintstoneStaffs = "Glintstone Staffs"
	SubcatRangedSacredSeals      = "Sacred Seals"
)

// Arrows / Bolts (4 sub-groups)
const (
	SubcatArrowsArrows      = "Arrows"
	SubcatArrowsGreatarrows = "Greatarrows"
	SubcatArrowsBolts       = "Bolts"
	SubcatArrowsGreatbolts  = "Greatbolts"
)

// Shields (5 sub-groups, in-game sortGroupId order:
// Torch=10, Small=20, Medium=30, Great=40, Thrusting=50)
const (
	SubcatShieldsTorches      = "Torches"
	SubcatShieldsSmall        = "Small Shields"
	SubcatShieldsMedium       = "Medium Shields"
	SubcatShieldsGreatshields = "Greatshields"
	SubcatShieldsThrusting    = "Thrusting Shields"
)

// Info (3 sub-groups)
const (
	SubcatInfoLettersMaps        = "Letters / Maps / Paintings"
	SubcatInfoDLCLettersMaps     = "Letters / Maps / Paintings (DLC)"
	SubcatInfoMechanicsLocations = "Mechanics / Locations Info"
)
