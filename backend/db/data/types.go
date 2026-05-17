package data

// ItemData represents the metadata for an item in the game database.
//
// Flags reference (string set; combine freely):
//   - "stackable"       — item stacks in a single inventory slot (vs. unique drops)
//   - "dlc"             — Shadow of the Erdtree content
//   - "cut_content"     — never shipped legitimately; spawning may flag EAC
//   - "ban_risk"        — adding this item carries elevated EAC ban risk
//   - "scales_with_ng"  — vanilla obtainable count scales linearly with NG+ cycle:
//     effective_cap = MaxInventory * (ClearCount + 1)
//     (ClearCount: 0 = NG, 1 = NG+1, ..., 7 = NG+7)
//     Used for: Stonesword Key, Dragon Heart, Larval Tear,
//     Golden Seed, Sacred Tear, Scadutree Fragment,
//     Revered Spirit Ash. See spec/34-item-caps.md.
type ItemData struct {
	Name         string
	Category     string
	SubCategory  string // sub-category within Category (1:1 with in-game grouping); empty = no sub-grouping
	MaxInventory uint32
	MaxStorage   uint32
	MaxUpgrade   uint32
	IconPath     string
	Flags        []string
	// Weapon AoW compatibility (populated from EquipParamWeapon via weapon_gem_mount.go):
	//   0 = cannot mount AoW, 1 = special/unique AoW (somber), 2 = standard infusable
	GemMountType uint8
	// Weapon type category integer from EquipParamWeapon.wepType (populated for weapons).
	WepType uint16
	// AoW → weapon compatibility bitmask from EquipParamGem.canMountWep_* (populated for AoWs).
	// Bit N = 1 means this AoW can be mounted on weapons whose wepType maps to bit N.
	// Bit ordering: 0=Dagger, 1=StraightSword, 2=Greatsword, ... see data.CanMountWepNames.
	AoWCompatBitmask uint64
}

// WeaponStats holds base stats for a weapon (before upgrades/infusions).
type WeaponStats struct {
	Weight     float64
	PhysDamage uint32
	MagDamage  uint32
	FireDamage uint32
	LitDamage  uint32
	HolyDamage uint32
	ScaleStr   uint32
	ScaleDex   uint32
	ScaleInt   uint32
	ScaleFai   uint32
	ReqStr     uint32
	ReqDex     uint32
	ReqInt     uint32
	ReqFai     uint32
	ReqArc     uint32
}

// ArmorStats holds damage negation and resistance values for armor.
type ArmorStats struct {
	Weight     float64
	Physical   float64
	Strike     float64
	Slash      float64
	Pierce     float64
	Magic      float64
	Fire       float64
	Lightning  float64
	Holy       float64
	Immunity   uint32
	Robustness uint32
	Focus      uint32
	Vitality   uint32
	Poise      float64
}

// SpellStats holds FP cost, slot count, and attribute requirements for spells.
type SpellStats struct {
	FPCost uint32
	Slots  uint32
	ReqInt uint32
	ReqFai uint32
	ReqArc uint32
}

// SortKey holds the in-game sort identifiers for an item.
//
//   - SortId      determines position within a group (higher = later in list).
//     Values come from sortId columns of EquipParamWeapon/Protector/Accessory.
//     Sentinel 9999999 = item has no defined sort order (sorts to end).
//   - SortGroupId determines the type group the item belongs to (e.g. 10 = dagger,
//     20 = straight sword; grouping mirrors in-game "Type" filter).
//     Max observed value: 255 — fits in uint8.
type SortKey struct {
	SortId      uint32
	SortGroupId uint8
}

// ItemDescription holds an item's in-game description text and optional stats.
type ItemDescription struct {
	Description string
	Location    string
	Weight      float64
	Weapon      *WeaponStats
	Armor       *ArmorStats
	Spell       *SpellStats
}

// Descriptions maps item IDs to their descriptions and stats.
// Populated by generated code in descriptions.go.
var Descriptions map[uint32]ItemDescription

// TextSource identifies where a piece of item text originated.
//
// Used to record provenance on each ItemTextData field so the UI and
// tests can reason about whether content came from the app's curated
// ItemData.Name, the FMG canonical strings extracted from regulation.bin,
// the curated descriptions.go (community/Fextralife sourced), or a
// combination of sources where the app value overrides FMG.
type TextSource string

const (
	TextSourceNone    TextSource = ""
	TextSourceApp     TextSource = "app"
	TextSourceFMG     TextSource = "fmg"
	TextSourceCurated TextSource = "curated"
	TextSourceMixed   TextSource = "mixed"
)

// ItemTextData carries all displayable text for one item, with per-field
// provenance.
//
// Resolution rules (applied by the generator, not at runtime):
//   - DisplayName  always = app ItemData.Name (preserves app overrides
//     such as "Letter from Volcano Manor (Istvan)" or
//     "Chain Gauntlets"). Source = App when FMG canonical
//     is missing or matches the app name; Mixed when FMG
//     canonical exists and differs from the app name.
//   - CanonicalName = FMG canonical when known (always populated for
//     items with an FMG name). Source = FMG.
//   - Caption       = FMG *Caption.fmg text. Source = FMG.
//   - Description   = FMG *Info.fmg text (Goods concat Info + Info2);
//     falls back to curated descriptions.go Description
//     when FMG is empty. Source = FMG, Curated, or Mixed.
//   - Location      = curated descriptions.go Location only (no FMG
//     equivalent exists). Source = Curated.
//
// All string fields use "" as the absence sentinel; the struct itself
// is a value type, so map lookups for unknown IDs return the zero
// value safely.
type ItemTextData struct {
	DisplayName       string
	CanonicalName     string
	Caption           string
	Description       string
	Location          string
	DisplayNameSource TextSource
	CanonicalSource   TextSource
	CaptionSource     TextSource
	DescriptionSource TextSource
	LocationSource    TextSource
	// DLCSource records which FMG variant contributed the FMG fields:
	// "base", "dlc01", "dlc02". Empty when no FMG entry was found.
	DLCSource string
	Notes     string
}

// ItemTexts maps item IDs to their displayable text data.
// Populated by generated code in item_text_generated.go.
var ItemTexts map[uint32]ItemTextData

// WeaponStatsV1 holds weapon-like base stats for a single item, sourced
// directly from EquipParamWeapon / ReinforceParamWeapon and shipped as a
// generated Go map. Covers melee_armaments, shields, ranged_and_catalysts
// and arrows_and_bolts.
//
// IMPORTANT — R-STA-01: Elden Ring's EquipParamWeapon CSV stores Holy
// damage in legacy "Dark"-named columns (`attackBaseDark`, `darkGuardCutRate`,
// `correctType_Dark`). AttackHoly / GuardHoly below ARE sourced from those
// columns — there is no separate "Holy" CSV column. App UI surfaces them
// as "Holy" so consumers do not see the `Dark` naming.
//
// Status damage applied by a weapon (poison/bleed/etc.) is NOT sourced
// from EquipParamWeapon directly; it requires traversing SpEffect /
// AtkParam tables which Phase 3C.1 intentionally does not interpret.
// Status* fields therefore stay zero in V1 and a "status-deferred"
// warning is recorded.
//
// Field provenance is documented per-field; numbers are taken verbatim
// from the CSV (with the documented "Dark"→"Holy" rename). Guard cut
// rates are stored as int32 — CSV values are integer percentages such
// as 30, 45, etc. in the rows surveyed.
type WeaponStatsV1 struct {
	ItemID uint32 // app item ID (matches category map keys, == regulation row ID)

	// Source classification
	WepType         uint16 // EquipParamWeapon.wepType
	SortGroupID     uint8  // EquipParamWeapon.sortGroupId
	ReinforceTypeID int32  // EquipParamWeapon.reinforceTypeId (band base in ReinforceParamWeapon)
	GemMountType    uint8  // EquipParamWeapon.gemMountType (0=none, 1=somber, 2=infusable)

	Weight float64 // EquipParamWeapon.weight

	// Base attack power (level +0). AttackHoly is sourced from attackBaseDark.
	AttackPhysical  int32 // EquipParamWeapon.attackBasePhysics
	AttackMagic     int32 // EquipParamWeapon.attackBaseMagic
	AttackFire      int32 // EquipParamWeapon.attackBaseFire
	AttackLightning int32 // EquipParamWeapon.attackBaseThunder
	AttackHoly      int32 // EquipParamWeapon.attackBaseDark — see R-STA-01
	AttackStamina   int32 // EquipParamWeapon.attackBaseStamina

	// Guard cut rates (percentages) and guard boost (staminaGuardDef).
	GuardPhysical  int32 // EquipParamWeapon.physGuardCutRate
	GuardMagic     int32 // EquipParamWeapon.magGuardCutRate
	GuardFire      int32 // EquipParamWeapon.fireGuardCutRate
	GuardLightning int32 // EquipParamWeapon.thunGuardCutRate
	GuardHoly      int32 // EquipParamWeapon.darkGuardCutRate — see R-STA-01
	GuardBoost     int32 // EquipParamWeapon.staminaGuardDef

	// Wielding stat requirements (properStrength etc.).
	StatReqStr int32 // EquipParamWeapon.properStrength
	StatReqDex int32 // EquipParamWeapon.properAgility
	StatReqInt int32 // EquipParamWeapon.properMagic
	StatReqFai int32 // EquipParamWeapon.properFaith
	StatReqArc int32 // EquipParamWeapon.properLuck

	// Critical damage rate displayed in-game. Sourced from
	// EquipParamWeapon.throwAtkRate which stores the *offset* above a
	// base of 100 (Misericorde row stores 40 → in-game shows 140,
	// Lordsworn's Straight Sword stores 10 → 110, default 0 → 100).
	// The generator pre-adds the 100 base so consumers can render the
	// value verbatim.
	Critical int32

	// Raw stat scaling coefficients (correctStrength etc.). NOT letter
	// grades — those require CalcCorrectGraph and are deferred to V2.
	ScalingStrRaw int32 // EquipParamWeapon.correctStrength
	ScalingDexRaw int32 // EquipParamWeapon.correctAgility
	ScalingIntRaw int32 // EquipParamWeapon.correctMagic
	ScalingFaiRaw int32 // EquipParamWeapon.correctFaith
	ScalingArcRaw int32 // EquipParamWeapon.correctLuck

	// Status-effect damage applied by the weapon. Phase 3C.1 leaves
	// these zero — derivation requires SpEffect / AtkParam traversal.
	StatusPoison     int32
	StatusBleed      int32
	StatusFrost      int32
	StatusSleep      int32
	StatusMadness    int32
	StatusScarletRot int32

	// PassiveEffects enumerates on-hit and resident SpEffects attached
	// to the weapon. Generated by resolving
	// EquipParamWeapon.spEffectBehaviorId0..2 (on-hit) and
	// residentSpEffectId/1/2 (resident) against SpEffectParam plus a
	// small curated label map for well-known resident IDs. Entries with
	// Known=false carry an unresolved SpEffect ID so the UI can still
	// surface the existence of an effect.
	PassiveEffects []WeaponPassiveEffect

	// Default Ash of War (swordArtsParamId — not a Gem item ID).
	DefaultAoWID int32 // EquipParamWeapon.swordArtsParamId

	// Reinforcement classification derived from ReinforceParamWeapon
	// band size at ReinforceTypeID:
	//   - band size 26 → IsInfusable=true,  IsSomber=false, MaxUpgrade=25
	//   - band size 11 → IsInfusable=false, IsSomber=true,  MaxUpgrade=10
	//   - band size 1  → IsInfusable=false, IsSomber=false, MaxUpgrade=0
	IsInfusable bool
	IsSomber    bool
	MaxUpgrade  int32

	// Provenance
	SourceRowID uint32   // EquipParamWeapon Row ID (always == ItemID)
	Warnings    []string // empty for fully-resolved rows
}

// WeaponStatsV1ByID maps weapon-like item IDs (melee_armaments, shields,
// ranged_and_catalysts, arrows_and_bolts) to their generated V1 stats.
// Populated by generated code in weapon_stats_generated.go.
var WeaponStatsV1ByID map[uint32]WeaponStatsV1

// WeaponPassiveEffect describes a single SpEffect attached to a weapon —
// either an on-hit proc (spEffectBehaviorId0..2) or a resident effect
// held while equipped (residentSpEffectId/1/2).
//
//   - Kind:       "on_hit" | "resident"
//   - Source:     CSV column name of the slot ("spEffectBehaviorId0",
//     "residentSpEffectId1", …) — preserves slot identity
//     for diagnostics and stable ordering.
//   - SpEffectID: row ID in SpEffectParam.
//   - Label:      human-readable name. For on-hit status procs this is
//     "Poison" / "Scarlet Rot" / "Blood Loss" / "Frost" /
//     "Sleep" / "Madness" / "Death Blight"; for resident
//     effects it comes from a small curated label map.
//     "Unknown on-hit effect" / "Unknown resident effect"
//     is used as a fallback so the UI can still surface
//     something.
//   - Value:      buildup amount for status procs; 0 otherwise.
//   - Known:      true when Label was resolved from SpEffectParam data
//     or the curated map; false for the unknown fallback.
type WeaponPassiveEffect struct {
	Kind       string
	Source     string
	SpEffectID int32
	Label      string
	Value      int32
	Known      bool
}

// ItemStatsKind tags an ItemStatsData payload with the category of stats
// it carries. Phase 3C.3 only emits "weapon"; future sub-phases will
// extend the enum (armor, spell, goods, ash_of_war) as their generators
// land. The empty string is the absence sentinel and is never emitted
// when ItemStatsData is non-nil at runtime.
type ItemStatsKind string

const (
	ItemStatsKindNone   ItemStatsKind = ""
	ItemStatsKindWeapon ItemStatsKind = "weapon"
	ItemStatsKindArmor  ItemStatsKind = "armor"
	ItemStatsKindSpell  ItemStatsKind = "spell"
	ItemStatsKindGoods  ItemStatsKind = "goods"
	ItemStatsKindAoW    ItemStatsKind = "ash_of_war"
)

// ItemStatsData is a Phase 3C.3 wrapper exposing generated stats payloads
// (V1+) on ItemEntry without disturbing the legacy `Weapon` / `Armor` /
// `Spell` pointers. The wrapper is intentionally minimal: only the
// concrete payload pointers that are actually generated today appear as
// fields, so the Wails TS bindings stay strongly typed (no `any` /
// `interface{}` projections).
//
// Resolution rule (Phase 3C.3): set Kind=Weapon and Weapon=&copy(v) when
// data.WeaponStatsV1ByID has an entry for the item ID. Future kinds will
// add their own pointers as the corresponding generated tables land.
type ItemStatsData struct {
	Kind        ItemStatsKind  `json:"kind"`
	Weapon      *WeaponStatsV1 `json:"weapon,omitempty"`
	SourceParam string         `json:"sourceParam,omitempty"`
	SourceRowID uint32         `json:"sourceRowId,omitempty"`
	Warnings    []string       `json:"warnings,omitempty"`
}
