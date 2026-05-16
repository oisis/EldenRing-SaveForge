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
