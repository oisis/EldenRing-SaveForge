package db

import (
	"fmt"
	"slices"
	"sort"
	"strings"
	"sync"

	"github.com/oisis/EldenRing-SaveForge/backend/db/data"
)

// itemCacheMu guards itemCache — covers GetItemsByCategory and GetAllItems.
var (
	itemCacheMu sync.RWMutex
	itemCache   = make(map[string][]ItemEntry)
)

// ItemEntry represents a single item from the game database.
type ItemEntry struct {
	ID               uint32            `json:"id"`
	Name             string            `json:"name"`
	Category         string            `json:"category"`
	SubCategory      string            `json:"subCategory,omitempty"`
	MaxInventory     uint32            `json:"maxInventory"`
	MaxStorage       uint32            `json:"maxStorage"`
	MaxUpgrade       uint32            `json:"maxUpgrade"`
	IconPath         string            `json:"iconPath"`
	Flags            []string          `json:"flags"`
	Description      string            `json:"description,omitempty"`
	Location         string            `json:"location,omitempty"`
	Weight           float64           `json:"weight,omitempty"`
	Weapon           *data.WeaponStats `json:"weapon,omitempty"`
	Armor            *data.ArmorStats  `json:"armor,omitempty"`
	Spell            *data.SpellStats  `json:"spell,omitempty"`
	AoWCompatBitmask uint64            `json:"aowCompatBitmask,omitempty"`
	// Text is the generated Phase 3B.1 text payload (display + canonical
	// name, caption, description, location, per-field provenance).
	// Read-only for the frontend; nil when the item ID has no ItemTexts
	// entry. Legacy Description / Location remain populated independently
	// so existing UI bindings keep working unchanged.
	Text *data.ItemTextData `json:"text,omitempty"`
	// Stats is the generated Phase 3C.3 stats payload. For weapon-like
	// items it wraps a copy of the WeaponStatsV1 record (kind="weapon");
	// for non-weapon items, or weapon-like items without a V1 entry, it
	// is nil. Legacy `Weapon` / `Armor` / `Spell` pointers remain
	// populated independently so existing UI bindings keep working
	// unchanged — Phase 3C.3 only adds a new payload field.
	Stats *data.ItemStatsData `json:"stats,omitempty"`
	// FlagID is the event flag ID used by World-tab unlock endpoints
	// (SetCookbookUnlocked, SetWhetbladeUnlocked, SetBellBearingUnlocked).
	// Zero for items managed via standard AddItemsToCharacter.
	FlagID uint32 `json:"flagId,omitempty"`
	// UnlockCategory identifies items whose add/remove must go through
	// the World-tab unlock endpoints instead of AddItemsToCharacter.
	// Values: "cookbook" | "whetblade" | "bell_bearing" | "" (standard).
	UnlockCategory string `json:"unlockCategory,omitempty"`
}

// weightedCategory lists item categories that have physical weight from regulation.bin weapon/armor params.
// Spells, consumables, key items etc. share ID space with weapon/armor params — excluded to avoid false matches.
var weightedCategory = map[string]bool{
	"melee_armaments":      true,
	"ranged_and_catalysts": true,
	"shields":              true,
	"head":                 true,
	"chest":                true,
	"arms":                 true,
	"legs":                 true,
	"talismans":            true,
}

// InfuseType represents a weapon infusion type and its ID offset.
type InfuseType struct {
	Name   string `json:"name"`
	Offset int    `json:"offset"`
}

// InfuseTypes lists all weapon infusion types in Elden Ring order.
var InfuseTypes = []InfuseType{
	{"Standard", 0},
	{"Heavy", 100},
	{"Keen", 200},
	{"Quality", 300},
	{"Fire", 400},
	{"Flame Art", 500},
	{"Lightning", 600},
	{"Sacred", 700},
	{"Magic", 800},
	{"Cold", 900},
	{"Poison", 1000},
	{"Blood", 1100},
	{"Occult", 1200},
}

// GraceEntry represents a Site of Grace.
type GraceEntry struct {
	ID          uint32 `json:"id"`
	Name        string `json:"name"`
	Region      string `json:"region"`
	Visited     bool   `json:"visited"`
	IsBossArena bool   `json:"isBossArena"`
	DungeonType string `json:"dungeonType,omitempty"`
}

// BossEntry represents a boss encounter with defeat state.
type BossEntry struct {
	ID          uint32 `json:"id"`
	Name        string `json:"name"`
	Region      string `json:"region"`
	Type        string `json:"type"`        // "main" or "field"
	Remembrance bool   `json:"remembrance"` // drops Remembrance item
	Defeated    bool   `json:"defeated"`
}

// SummoningPoolEntry represents a Martyr Effigy with activation state.
type SummoningPoolEntry struct {
	ID        uint32 `json:"id"`
	Name      string `json:"name"`
	Region    string `json:"region"`
	Activated bool   `json:"activated"`
}

// ColosseumEntry represents a PvP colosseum with unlock state.
type ColosseumEntry struct {
	ID       uint32 `json:"id"`
	Name     string `json:"name"`
	Region   string `json:"region"`
	Unlocked bool   `json:"unlocked"`
}

// RegionEntry represents an "unlocked region" — controls invasion eligibility,
// blue summons, and the on-screen area label after teleport.
type RegionEntry struct {
	ID       uint32 `json:"id"`
	Name     string `json:"name"`
	Area     string `json:"area"`
	Unlocked bool   `json:"unlocked"`
}

// GestureEntry represents a gesture with its unlock state.
type GestureEntry struct {
	ID       uint32   `json:"id"`
	Name     string   `json:"name"`
	Category string   `json:"category"`
	Unlocked bool     `json:"unlocked"`
	Flags    []string `json:"flags"` // "cut_content" | "pre_order" | "dlc_duplicate" | "ban_risk"
}

// GetAllGestureSlots returns all known gestures, one entry per gesture.
// ID is the canonical save-slot ID (always odd in vanilla data).
var getAllGestureSlots = sync.OnceValue(func() []GestureEntry {
	entries := make([]GestureEntry, 0, len(data.AllGestures))
	for _, g := range data.AllGestures {
		entries = append(entries, GestureEntry{
			ID:       g.ID,
			Name:     g.Name,
			Category: g.Category,
			Flags:    g.Flags,
		})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Category != entries[j].Category {
			return entries[i].Category < entries[j].Category
		}
		return entries[i].Name < entries[j].Name
	})
	return entries
})

func GetAllGestureSlots() []GestureEntry { return getAllGestureSlots() }

// CookbookEntry represents a cookbook with its unlock state.
type CookbookEntry struct {
	ID       uint32 `json:"id"` // event flag ID
	Name     string `json:"name"`
	Category string `json:"category"` // series name for grouping
	Unlocked bool   `json:"unlocked"`
}

// MapEntry represents a map region flag with its current state.
type MapEntry struct {
	ID       uint32 `json:"id"`
	Name     string `json:"name"`
	Area     string `json:"area"`
	Category string `json:"category"` // "visible", "acquired", "system"
	Enabled  bool   `json:"enabled"`
}

// BellBearingEntry represents a bell bearing with its current state.
type BellBearingEntry struct {
	ID       uint32 `json:"id"`
	Name     string `json:"name"`
	Category string `json:"category"` // "npc", "merchant", "smithing", "peddler", "dlc"
	Unlocked bool   `json:"unlocked"`
}

// WhetbladeEntry represents a whetblade unlock with its current state.
type WhetbladeEntry struct {
	ID       uint32 `json:"id"`
	Name     string `json:"name"`
	Unlocked bool   `json:"unlocked"`
}

// globalItemIndex provides O(1) item lookup by ID, built once at startup.
var globalItemIndex map[uint32]data.ItemData

func init() {
	allMaps := []map[uint32]data.ItemData{
		data.Weapons, data.RangedAndCatalysts, data.Shields, data.ArrowsAndBolts,
		data.Helms, data.Chest, data.Arms, data.Legs,
		data.Talismans, data.Aows, data.Gestures,
		data.StandardAshes,
		data.Sorceries, data.Incantations, data.CraftingMaterials,
		data.BolsteringMaterials, data.KeyItems,
		data.Tools, data.Information,
	}
	size := 0
	for _, m := range allMaps {
		size += len(m)
	}
	globalItemIndex = make(map[uint32]data.ItemData, size)
	for _, m := range allMaps {
		for id, entry := range m {
			globalItemIndex[id] = entry
		}
	}
	// Merge weapon AoW mount data (gemMountType, wepType) from generated lookup maps.
	// WeaponGemMounts keys include base variants (upgrade 0) and infusion variants (+100, +200, ...).
	// We only update entries already in the index (i.e. base weapons in our DB).
	for id, mount := range data.WeaponGemMounts {
		if entry, ok := globalItemIndex[id]; ok {
			entry.GemMountType = mount.GemMountType
			entry.WepType = mount.WepType
			globalItemIndex[id] = entry
		}
	}
	// Merge AoW weapon compatibility bitmasks.
	for id, bitmask := range data.AoWCompatMasks {
		if entry, ok := globalItemIndex[id]; ok {
			entry.AoWCompatBitmask = bitmask
			globalItemIndex[id] = entry
		}
	}
}

// GetItemData returns the full metadata of an item by its ID via the global index.
func GetItemData(id uint32) data.ItemData {
	if item, ok := globalItemIndex[id]; ok {
		return item
	}
	return data.ItemData{}
}

// CanWeaponMountAoW returns true if the weapon (by base item ID) supports standard AoW mounting
// (gemMountType == 2). Returns false for unique/somber weapons (gemMountType == 1) and
// weapons that cannot mount AoW at all (gemMountType == 0 or not found in data).
func CanWeaponMountAoW(baseItemID uint32) bool {
	item := GetItemData(baseItemID)
	return item.GemMountType == 2
}

// IsAoWCompatibleWithWepType returns (compatible, known).
// known=false means data is insufficient — the caller must block the operation, not assume compatibility.
// Reasons for known=false: AoW not in compat table (bitmask=0), or wepType not in WepTypeToCanMountBit.
func IsAoWCompatibleWithWepType(aowItemID uint32, wepType uint16) (compatible bool, known bool) {
	aow := GetItemData(aowItemID)
	if aow.AoWCompatBitmask == 0 {
		return false, false // AoW compatibility data missing
	}
	bitPositions, ok := data.CanMountBitsForWepType(wepType)
	if !ok {
		return false, false // weapon type not in bit map
	}
	for _, bitPos := range bitPositions {
		if (aow.AoWCompatBitmask>>bitPos)&1 == 1 {
			return true, true
		}
	}
	return false, true
}

// IsAshOfWarCompatibleWithWeapon checks whether a specific Ash of War can be mounted on a
// specific weapon, combining the weapon-level GemMountType gate with the per-AoW bitmask check.
// Returns (compatible, known). known=false means data is insufficient; callers must block, not assume.
func IsAshOfWarCompatibleWithWeapon(aowItemID uint32, weaponItemID uint32) (compatible bool, known bool) {
	weaponData, _ := GetItemDataFuzzy(weaponItemID)
	if weaponData.GemMountType != 2 {
		return false, true // weapon doesn't support standard AoW (somber or no mount)
	}
	if weaponData.WepType == 0 {
		return false, false // weapon type unknown — data insufficient
	}
	return IsAoWCompatibleWithWepType(aowItemID, weaponData.WepType)
}

// enrichItemEntry populates Description, Location, Weight, and stat fields
// on an ItemEntry from the curated data tables.
//
// Resolution order:
//  1. Legacy data.Descriptions seeds Description / Location / Weight and
//     the legacy stat pointers (Weapon / Armor / Spell). This preserves
//     fallback coverage for IDs that pre-date the Phase 3B.1 ItemTexts
//     generator (e.g. descriptions.go orphan rows not present in any
//     app category map).
//  2. data.ItemTexts (Phase 3B.1) then overrides Description / Location
//     when the new generated entry has non-empty values. ItemTexts is
//     sourced from FMG with curated descriptions.go fallback for
//     Description, and curated descriptions.go is the sole source for
//     Location. Empty fields on ItemTexts leave the legacy value intact.
//  3. Phase 3B.3 also surfaces the full ItemTextData value on e.Text so
//     the frontend can read DisplayName / CanonicalName / Caption /
//     per-field provenance without re-querying the backend. e.Text is
//     nil for IDs without an ItemTexts entry.
//  4. Phase 3C.2 overrides e.Weapon and e.Weight with values mapped from
//     data.WeaponStatsV1ByID when an entry exists. The V1 table covers
//     all four weapon-like categories (melee_armaments, shields,
//     ranged_and_catalysts, arrows_and_bolts) with stats sourced
//     directly from EquipParamWeapon. Holy damage in particular flows
//     from V1.AttackHoly (← attackBaseDark) into legacy HolyDamage —
//     R-STA-01 guarded by enrich_weapon_stats_test.go. Items without a
//     V1 entry keep whatever step 1 supplied from descriptions.go.
//  5. Phase 3C.3 attaches a generated stats payload at e.Stats for
//     weapon-like items with a V1 entry. The payload wraps a copy of
//     the WeaponStatsV1 record together with provenance fields
//     (SourceParam / SourceRowID / Warnings) so the frontend can read
//     V1-only fields (guard cuts, stamina attack, somber flags, ...)
//     without touching the legacy projection. The legacy e.Weapon
//     pointer set in step 4 stays exactly as before — Phase 3C.3 is
//     additive, not a replacement.
//
// Armor and Spell pointers remain owned by data.Descriptions in Phase
// 3C.3; their generated layers will arrive in later sub-phases.
func enrichItemEntry(e *ItemEntry) {
	if data.Descriptions != nil {
		if desc, ok := data.Descriptions[e.ID]; ok {
			e.Description = desc.Description
			e.Location = desc.Location
			e.Weight = desc.Weight
			e.Weapon = desc.Weapon
			e.Armor = desc.Armor
			e.Spell = desc.Spell
		}
	}
	// Prefer the Phase 3B.1 ItemTexts entry for Description / Location
	// when populated. Empty strings leave the legacy fallback intact.
	// Also expose the full ItemTextData payload (Phase 3B.3) by copying
	// the map value into a fresh local — taking &data.ItemTexts[e.ID]
	// directly is illegal in Go (map values are not addressable).
	if data.ItemTexts != nil {
		if t, ok := data.ItemTexts[e.ID]; ok {
			if t.Description != "" {
				e.Description = t.Description
			}
			if t.Location != "" {
				e.Location = t.Location
			}
			text := t
			e.Text = &text
		}
	}
	// Phase 3C.2: prefer the generated WeaponStatsV1 table for weapon-like
	// items. The mapper preserves the legacy data.WeaponStats shape (and
	// therefore the existing ItemEntry payload) so the frontend renders
	// from item.weapon exactly as before — but with EquipParamWeapon as
	// the source of truth instead of partial descriptions.go entries.
	// Holy damage flows through V1.AttackHoly → legacy HolyDamage; the
	// R-STA-01 Dark→Holy rename happened entirely in the V1 generator.
	if data.WeaponStatsV1ByID != nil {
		if v, ok := data.WeaponStatsV1ByID[e.ID]; ok {
			e.Weapon = weaponStatsV1ToLegacy(v)
			if v.Weight > 0 {
				e.Weight = v.Weight
			}
			// Phase 3C.3: surface the full V1 record as a typed
			// payload alongside the legacy Weapon projection.
			// Take a local copy because map values are not
			// addressable — pointing at v keeps the payload safe
			// from later map mutations.
			weapon := v
			e.Stats = &data.ItemStatsData{
				Kind:        data.ItemStatsKindWeapon,
				Weapon:      &weapon,
				SourceParam: "EquipParamWeapon",
				SourceRowID: v.SourceRowID,
				Warnings:    v.Warnings,
			}
		}
	}
	// Only physical items carry weight — spells, consumables, key items etc. share ID space with weapon/armor params.
	if e.Weight == 0 && weightedCategory[e.Category] {
		if w, ok := data.ItemWeights[e.ID]; ok {
			e.Weight = w
		}
	}
}

// weaponStatsV1ToLegacy projects the Phase 3C.1 WeaponStatsV1 record
// onto the legacy data.WeaponStats shape consumed by the frontend.
//
// Field mapping is intentionally explicit (no reflection, no name-based
// auto-copy). Fields not present in legacy WeaponStats (AttackStamina,
// Guard*, ScalingArcRaw, WepType, IsInfusable/IsSomber/MaxUpgrade,
// Status*, DefaultAoWID, SourceRowID, Warnings) stay on the V1 record
// and will be exposed in Phase 3C.3 via a new payload field. They are
// NOT folded into unrelated legacy fields.
//
// CRITICAL — R-STA-01: V1.AttackHoly originates from
// EquipParamWeapon.attackBaseDark (Elden Ring's CSV keeps the legacy
// Dark naming) and is mapped here to legacy WeaponStats.HolyDamage —
// not to any Dark-named legacy field, of which there is none.
//
// Int32 → uint32 conversion clamps negative values to zero. V1 emits
// non-negative numbers in practice (CSV sources are non-negative),
// but the guard keeps the projection safe against future regressions.
func weaponStatsV1ToLegacy(v data.WeaponStatsV1) *data.WeaponStats {
	return &data.WeaponStats{
		Weight:     v.Weight,
		PhysDamage: nonNegU32(v.AttackPhysical),
		MagDamage:  nonNegU32(v.AttackMagic),
		FireDamage: nonNegU32(v.AttackFire),
		LitDamage:  nonNegU32(v.AttackLightning),
		HolyDamage: nonNegU32(v.AttackHoly),
		ScaleStr:   nonNegU32(v.ScalingStrRaw),
		ScaleDex:   nonNegU32(v.ScalingDexRaw),
		ScaleInt:   nonNegU32(v.ScalingIntRaw),
		ScaleFai:   nonNegU32(v.ScalingFaiRaw),
		ReqStr:     nonNegU32(v.StatReqStr),
		ReqDex:     nonNegU32(v.StatReqDex),
		ReqInt:     nonNegU32(v.StatReqInt),
		ReqFai:     nonNegU32(v.StatReqFai),
		ReqArc:     nonNegU32(v.StatReqArc),
	}
}

func nonNegU32(v int32) uint32 {
	if v <= 0 {
		return 0
	}
	return uint32(v)
}

// GetItemEntryByID returns a fully enriched ItemEntry for the given base item ID, or nil if not found.
func GetItemEntryByID(id uint32) *ItemEntry {
	item := GetItemData(id)
	if item.Name == "" {
		return nil
	}
	entry := &ItemEntry{
		ID:           id,
		Name:         item.Name,
		Category:     item.Category,
		SubCategory:  GetItemSubCategory(id, item, item.Category),
		MaxInventory: item.MaxInventory,
		MaxStorage:   item.MaxStorage,
		MaxUpgrade:   item.MaxUpgrade,
		IconPath:     item.IconPath,
		Flags:        item.Flags,
	}
	enrichItemEntry(entry)
	return entry
}

// findAshBase searches StandardAshes for the base (+0) entry matching the given name prefix.
// baseName must already have any " +N" suffix stripped. Returns (entry, baseID) or zero values.
func findAshBase(baseName string, idPrefix uint32) (data.ItemData, uint32) {
	for ashID, ashEntry := range data.StandardAshes {
		if ashEntry.Name == baseName {
			return ashEntry, (ashID & 0x0FFFFFFF) | idPrefix
		}
	}
	return data.ItemData{}, 0
}

// GetItemDataFuzzy returns item metadata for an exact ID, or falls back to:
//   - Handle→ItemID conversion: for stackable items read from inventory (0xA0→0x20, 0xB0→0x40)
//   - Spirit ashes: finds base (+0) entry so currentUpgrade can be computed from ID difference.
//   - Upgraded/infused weapons: byte-masked base search for 0x00... weapon IDs.
//
// The returned ItemData.Name is the base name without "+N" (caller appends "+N" if needed).
func GetItemDataFuzzy(id uint32) (data.ItemData, uint32) {
	exact := GetItemData(id)
	if exact.Name != "" {
		// Spirit ashes store each upgrade level as a separate DB entry with "+N" in the name.
		// Find the base (+0) entry so currentUpgrade = id - baseID is computed correctly.
		if exact.Category == "ashes" && strings.Contains(exact.Name, " +") {
			baseName := exact.Name[:strings.Index(exact.Name, " +")]
			if entry, baseID := findAshBase(baseName, id&0xF0000000); baseID != 0 {
				return entry, baseID
			}
		}
		return exact, id
	}

	prefix := id & 0xF0000000

	// Handle prefix → item ID prefix conversion for stackable items.
	// Inventory stores handles with GaItem prefixes (0xA0 talismans, 0xB0 goods),
	// but DB uses item ID prefixes (0x20, 0x40). Convert and retry.
	if prefix == 0xA0000000 || prefix == 0xB0000000 {
		pcID := HandleToItemID(id)
		pcEntry := GetItemData(pcID)
		if pcEntry.Name != "" {
			// Spirit ashes: find base (+0) for upgrade calculation
			if pcEntry.Category == "ashes" {
				baseName := pcEntry.Name
				if idx := strings.Index(baseName, " +"); idx >= 0 {
					baseName = baseName[:idx]
				}
				if entry, baseID := findAshBase(baseName, 0x40000000); baseID != 0 {
					return entry, baseID
				}
			}
			return pcEntry, pcID
		}
	}

	// Weapon fuzzy search: upgraded/infused weapons (prefixes 0x00, 0x01, 0x02).
	// Range-based: any id in [baseID, baseID+1225] maps to its base entry.
	// 1225 = max infusion offset (Occult=1200) + max upgrade level (25).
	// This handles byte-carry cases where (id & 0xFFFFFF00) != (baseID & 0xFFFFFF00),
	// which occurs for bows/greatbows/seals/staves when upgrade+infuse >= 0x100 - (baseID & 0xFF).
	if prefix == 0 || prefix == 0x01000000 || prefix == 0x02000000 {
		const maxCombinedOffset = uint32(1225)
		weaponMaps := []map[uint32]data.ItemData{data.Weapons, data.RangedAndCatalysts, data.Shields}
		for _, m := range weaponMaps {
			for baseID, item := range m {
				if item.Name == "" || baseID&0xF0000000 != prefix {
					continue
				}
				if id >= baseID && id-baseID <= maxCombinedOffset {
					return GetItemData(baseID), baseID
				}
			}
		}
	}

	return data.ItemData{}, id
}

// GetItemName returns the name of an item by its ID and category.
func GetItemName(id uint32, category string) string {
	// Special handling for weapons with levels
	for baseID, item := range data.Weapons {
		if (id & 0xFFFFFF00) == (baseID & 0xFFFFFF00) {
			level := id - baseID
			if level > 0 {
				return fmt.Sprintf("%s +%d", item.Name, level)
			}
			return item.Name
		}
	}
	// Check other weapon-like categories for levels
	weaponMaps := []map[uint32]data.ItemData{data.RangedAndCatalysts, data.Shields}
	for _, m := range weaponMaps {
		for baseID, item := range m {
			if (id & 0xFFFFFF00) == (baseID & 0xFFFFFF00) {
				level := id - baseID
				if level > 0 {
					return fmt.Sprintf("%s +%d", item.Name, level)
				}
				return item.Name
			}
		}
	}

	return fmt.Sprintf("Unknown Item (0x%X)", id)
}

// IsArrowID returns true if the given item ID corresponds to an arrow or bolt.
// Arrows have 0x02.../0x03... item IDs (weapon subtype) but are stackable in inventory.
func IsArrowID(id uint32) bool {
	_, ok := data.ArrowsAndBolts[id]
	return ok
}

// GetItemCategoryFromHandle returns the category string based on the GaItemHandle prefix.
func GetItemCategoryFromHandle(handle uint32) string {
	switch handle & 0xF0000000 {
	case 0x80000000:
		return "Weapon"
	case 0x90000000:
		return "Armor"
	case 0xA0000000:
		return "Talisman"
	case 0xB0000000:
		return "Item"
	case 0xC0000000:
		return "Ash of War"
	default:
		return "Unknown"
	}
}

// HandleToItemID converts a GaItem handle prefix to the corresponding item ID prefix.
// Handles always use GaItem type prefixes (0x80/0x90/0xA0/0xB0/0xC0) while item IDs
// in the database use item type prefixes (0x00/0x10/0x20/0x40/0x80).
// For stackable items (talismans, goods) where handle=id with handle prefix,
// this recovers the DB-compatible item ID.
func HandleToItemID(handle uint32) uint32 {
	prefix := handle & 0xF0000000
	lower := handle & 0x0FFFFFFF
	switch prefix {
	case 0x80000000:
		return lower // weapon: 0x80→0x00
	case 0x90000000:
		return lower | 0x10000000 // armor: 0x90→0x10
	case 0xA0000000:
		return lower | 0x20000000 // talisman: 0xA0→0x20
	case 0xB0000000:
		return lower | 0x40000000 // goods: 0xB0→0x40
	case 0xC0000000:
		return lower | 0x80000000 // AoW: 0xC0→0x80
	default:
		return handle
	}
}

// ItemIDToHandlePrefix returns the GaItem handle prefix for a given item ID.
// Item ID prefix (0x00/0x10/0x20/0x40/0x80) → handle prefix (0x80/0x90/0xA0/0xB0/0xC0).
func ItemIDToHandlePrefix(id uint32) uint32 {
	switch id & 0xF0000000 {
	case 0x00000000:
		return 0x80000000 // weapon
	case 0x10000000:
		return 0x90000000 // armor
	case 0x20000000:
		return 0xA0000000 // talisman
	case 0x40000000:
		return 0xB0000000 // goods
	case 0x80000000:
		return 0xC0000000 // AoW
	default:
		return 0x80000000 // fallback: weapon
	}
}

// ItemIDToMagicParamID extracts the raw MagicParam ID from a full DB-style
// spell item ID by stripping the 4-bit type prefix (0x40000000 for both
// sorceries and incantations in this codebase). Example: Catch Flame
// 0x40001770 → raw 0x1770.
//
// The 28-bit payload mask 0x0FFFFFFF mirrors ItemIDToHandlePrefix's 4-bit
// prefix convention — NOT a 16-bit mask. Spell IDs can carry payload bits
// above 0xFFFF (e.g. DLC sorceries with IDs in the high four-digit range
// once shifted), and a narrower mask would silently drop them.
//
// The function is prefix-agnostic by design: callers (the Phase 7d.3
// spells apply resolver) are expected to validate the prefix before
// invoking this helper. The Phase 7d.1 schema validator already rejects
// non-0x40 prefixes at template ingest time, so a templates → apply
// pipeline never feeds a non-spell ID through this conversion.
func ItemIDToMagicParamID(itemID uint32) uint32 {
	return itemID & 0x0FFFFFFF
}

// GetItemsByCategory returns a sorted list of items for a given category.
// The database stores only PC-style item IDs (0x00=weapon, 0x10=armor, 0x20=talisman,
// 0x40=goods, 0x80=AoW). Platform conversion to handle prefixes happens at runtime
// in AddItemsToSlot (writer.go) and MapParsedSlotToVM (character_vm.go).
func GetItemsByCategory(category, platform string) []ItemEntry {
	if category == "all" {
		return GetAllItems(platform)
	}

	itemCacheMu.RLock()
	if cached, ok := itemCache[category]; ok {
		itemCacheMu.RUnlock()
		return cached
	}
	itemCacheMu.RUnlock()

	var items []ItemEntry

	// processMap adds all items from source to the result list.
	processMap := func(source map[uint32]data.ItemData, catName string) {
		for id, item := range source {
			if item.Name == "" || item.Name == "Unarmed" {
				continue
			}
			entry := ItemEntry{
				ID:           id,
				Name:         item.Name,
				Category:     catName,
				SubCategory:  item.SubCategory,
				MaxInventory: item.MaxInventory,
				MaxStorage:   item.MaxStorage,
				MaxUpgrade:   item.MaxUpgrade,
				IconPath:     item.IconPath,
				Flags:        item.Flags,
			}
			enrichItemEntry(&entry)
			items = append(items, entry)
		}
	}

	switch category {
	case "melee_armaments":
		processMap(data.Weapons, "melee_armaments")
		items = filterInfuseVariants(items)
	case "ranged_and_catalysts":
		processMap(data.RangedAndCatalysts, "ranged_and_catalysts")
		items = filterInfuseVariants(items)
	case "shields":
		processMap(data.Shields, "shields")
		items = filterInfuseVariants(items)
	case "head":
		processMap(data.Helms, "head")
	case "arms":
		processMap(data.Arms, "arms")
	case "legs":
		processMap(data.Legs, "legs")
	case "chest":
		processMap(data.Chest, "chest")
	case "talismans":
		processMap(data.Talismans, "talismans")
	case "ashes_of_war":
		processMap(data.Aows, "ashes_of_war")
		for i := range items {
			if enriched, ok := globalItemIndex[items[i].ID]; ok {
				items[i].AoWCompatBitmask = enriched.AoWCompatBitmask
			}
		}
	case "ashes":
		// StandardAshes stores each upgrade level as a separate entry.
		// Only return base (+0) entries — filter out " +N" variants.
		for id, item := range data.StandardAshes {
			if item.Name == "" || strings.Contains(item.Name, " +") {
				continue
			}
			items = append(items, ItemEntry{
				ID:           id,
				Name:         item.Name,
				Category:     "ashes",
				SubCategory:  item.SubCategory,
				MaxInventory: item.MaxInventory,
				MaxStorage:   item.MaxStorage,
				MaxUpgrade:   item.MaxUpgrade,
				IconPath:     item.IconPath,
				Flags:        item.Flags,
			})
		}
	case "gestures":
		processMap(data.Gestures, "gestures")
	case "sorceries":
		processMap(data.Sorceries, "sorceries")
	case "incantations":
		processMap(data.Incantations, "incantations")
	case "crafting_materials":
		processMap(data.CraftingMaterials, "crafting_materials")
	case "bolstering_materials":
		processMap(data.BolsteringMaterials, "bolstering_materials")
	case "arrows_and_bolts":
		processMap(data.ArrowsAndBolts, "arrows_and_bolts")
	case "tools":
		for id, item := range data.Tools {
			if item.Name == "" {
				continue
			}
			// Filter upgraded Flask variants — only keep base versions (no " +N" suffix)
			if strings.Contains(item.Name, "Flask of") && strings.Contains(item.Name, " +") {
				continue
			}
			entry := ItemEntry{
				ID:           id,
				Name:         item.Name,
				Category:     "tools",
				SubCategory:  item.SubCategory,
				MaxInventory: item.MaxInventory,
				MaxStorage:   item.MaxStorage,
				MaxUpgrade:   item.MaxUpgrade,
				IconPath:     item.IconPath,
				Flags:        item.Flags,
			}
			if flagID, ok := data.WhetbladeItemToFlagID[id]; ok {
				entry.FlagID = flagID
				entry.UnlockCategory = "whetblade"
				entry.MaxInventory = 1
				entry.MaxStorage = 0
			}
			items = append(items, entry)
		}
	case "key_items":
		for id, item := range data.KeyItems {
			if item.Name == "" {
				continue
			}
			entry := ItemEntry{
				ID:           id,
				Name:         item.Name,
				Category:     "key_items",
				SubCategory:  item.SubCategory,
				MaxInventory: item.MaxInventory,
				MaxStorage:   item.MaxStorage,
				MaxUpgrade:   item.MaxUpgrade,
				IconPath:     item.IconPath,
				Flags:        item.Flags,
			}
			if flagID, ok := data.CookbookItemToFlagID[id]; ok {
				entry.FlagID = flagID
				entry.UnlockCategory = "cookbook"
				entry.MaxInventory = 1
				entry.MaxStorage = 0
			} else if flagID, ok := data.BellBearingItemToFlagID[id]; ok {
				entry.FlagID = flagID
				entry.UnlockCategory = "bell_bearing"
				entry.MaxInventory = 1
				entry.MaxStorage = 0
			} else if flagID, ok := data.MapFragmentItemToFlagID[id]; ok {
				entry.FlagID = flagID
				entry.UnlockCategory = "map"
				entry.MaxInventory = 1
				entry.MaxStorage = 0
			} else if flagID, ok := data.WhetbladeItemToFlagID[id]; ok && !slices.Contains(item.Flags, "no_database") {
				entry.FlagID = flagID
				entry.UnlockCategory = "whetblade"
				entry.MaxInventory = 1
				entry.MaxStorage = 0
			} else if slices.Contains(item.Flags, "no_database") {
				continue
			}
			items = append(items, entry)
		}
	case "info":
		for id, item := range data.Information {
			if item.Name == "" {
				continue
			}
			items = append(items, ItemEntry{
				ID:           id,
				Name:         item.Name,
				Category:     "info",
				SubCategory:  item.SubCategory,
				MaxInventory: item.MaxInventory,
				MaxStorage:   item.MaxStorage,
				MaxUpgrade:   item.MaxUpgrade,
				IconPath:     item.IconPath,
				Flags:        item.Flags,
			})
		}
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].Name < items[j].Name
	})

	itemCacheMu.Lock()
	itemCache[category] = items
	itemCacheMu.Unlock()
	return items
}

// GetItemSubCategory returns the granular category string for an item.
func GetItemSubCategory(id uint32, item data.ItemData, broadCategory string) string {
	if item.Category != "" {
		return item.Category
	}

	// Fallback for items without category
	switch broadCategory {
	case "Weapon":
		return "weapons"
	case "Armor":
		return "chest"
	case "Talisman":
		return "talismans"
	case "Ash of War":
		return "ashes_of_war"
	default:
		return "tools"
	}
}

// GetAllItems returns all items from all categories for the given platform.
func GetAllItems(platform string) []ItemEntry {
	const cacheKey = "all"
	itemCacheMu.RLock()
	if cached, ok := itemCache[cacheKey]; ok {
		itemCacheMu.RUnlock()
		return cached
	}
	itemCacheMu.RUnlock()

	var all []ItemEntry
	cats := []string{
		"melee_armaments", "ranged_and_catalysts", "shields", "arrows_and_bolts",
		"head", "chest", "arms", "legs",
		"talismans", "ashes_of_war", "gestures",
		"ashes",
		"sorceries", "incantations", "crafting_materials",
		"bolstering_materials", "key_items",
		"tools", "info",
	}
	for _, cat := range cats {
		all = append(all, GetItemsByCategory(cat, platform)...)
	}

	sort.Slice(all, func(i, j int) bool {
		return all[i].Name < all[j].Name
	})

	itemCacheMu.Lock()
	itemCache[cacheKey] = all
	itemCacheMu.Unlock()
	return all
}

// GetAllGraces returns all Sites of Grace as a flat list.
var getAllGraces = sync.OnceValue(func() []GraceEntry {
	graces := make([]GraceEntry, 0, len(data.Graces))

	// Map game regions to our specific map filenames
	regionMap := map[string]string{
		"Ainsel River":           "Ainsel River",
		"Altus Plateau":          "Altus Plateau",
		"Caelid":                 "Caelid",
		"Consecrated Snowfield":  "Consecrated Snowfield",
		"Crumbling Farum Azula":  "Crumbling Farum Azula",
		"Deeproot Depths":        "Deeproot Depths",
		"Dragonbarrow":           "Dragonbarrow",
		"Forbidden Lands":        "Forbidden Lands",
		"Lake of Rot":            "Lake of Rot",
		"Leyndell Ashen Capital": "Leyndell, Ashen Capital",
		"Leyndell Royal Capital": "Leyndell, Royal Capital",
		"Miquella's Haligtree":   "Miquella's Haligtree",
		"Mohgwyn Palace":         "Mohgwyn Palace",
		"Mt. Gelmir":             "Mt. Gelmir",
		"Shadow of the Erdtree":  "Shadow of the Erdtree",
		"Siofra River":           "Siofra River",
		"Weeping Peninsula":      "Weeping Peninsula",
	}

	for id, gd := range data.Graces {
		parts := strings.Split(gd.Name, " (")
		name := parts[0]
		region := "Unknown"

		if len(parts) > 1 {
			rawRegion := strings.TrimSuffix(parts[1], ")")

			// Detailed sub-region mapping
			if rawRegion == "Limgrave" || rawRegion == "Roundtable Hold" {
				region = "Limgrave West" // Default
				eastKeywords := []string{"Mistwood", "Haight", "Siofra River Well", "Third Church of Marika", "Agheel Lake South"}
				for _, kw := range eastKeywords {
					if strings.Contains(name, kw) {
						region = "Limgrave East"
						break
					}
				}
			} else if rawRegion == "Liurnia of the Lakes" {
				region = "Liurnia North" // Default
				eastKeywords := []string{"Eastern Liurnia", "Church of Vows", "Ainsel River Well", "Eastern Tableland", "Jarburg", "Liurnia Highway"}
				westKeywords := []string{"Western Liurnia", "Carian Manor", "Four Belfries", "Revenger's Shack", "Temple Quarter", "Moongazing", "Caria Manor"}

				for _, kw := range eastKeywords {
					if strings.Contains(name, kw) {
						region = "Liurnia East"
						break
					}
				}
				if region == "Liurnia North" {
					for _, kw := range westKeywords {
						if strings.Contains(name, kw) {
							region = "Liurnia West"
							break
						}
					}
				}
			} else if rawRegion == "Mountaintops of the Giants" {
				region = "Mountaintops of the Giants East" // Default
				westKeywords := []string{"Castle Sol", "Snow Valley", "Freezing Lake", "Ancient Snow Valley", "First Church of Marika", "Whiteridge"}
				for _, kw := range westKeywords {
					if strings.Contains(name, kw) {
						region = "Mountaintops of the Giants West"
						break
					}
				}
			} else if mapped, ok := regionMap[rawRegion]; ok {
				region = mapped
			} else {
				region = rawRegion
			}
		}

		graces = append(graces, GraceEntry{
			ID:          id,
			Name:        name,
			Region:      region,
			IsBossArena: gd.BossArena,
			DungeonType: gd.DungeonType,
		})
	}

	sort.Slice(graces, func(i, j int) bool {
		if graces[i].Region != graces[j].Region {
			return graces[i].Region < graces[j].Region
		}
		return graces[i].Name < graces[j].Name
	})

	return graces
})

func GetAllGraces() []GraceEntry { return getAllGraces() }

// GetEventFlag checks if a specific event flag is set in the bit array.
// Resolution order:
//  1. Precomputed lookup table (data.EventFlags) — exact byte/bit for known IDs.
//  2. BST lookup (data.EventFlagBST) — block-based mapping from game's CSFD4VirtualMemoryFlag.
//  3. Fallback formula: byte = id / 8, bit = 7 - (id % 8).
//
// Returns error if the computed byte offset is out of bounds.
func GetEventFlag(flags []byte, id uint32) (bool, error) {
	byteIdx, bitIdx := resolveEventFlagPosition(id)
	if int(byteIdx) >= len(flags) {
		return false, fmt.Errorf("event flag %d (byte %d) out of bounds (flags len %d)", id, byteIdx, len(flags))
	}
	return (flags[byteIdx] & (1 << bitIdx)) != 0, nil
}

// resolveEventFlagPosition returns the byte offset and bit index for an event flag ID.
func resolveEventFlagPosition(id uint32) (byteIdx uint32, bitIdx uint8) {
	// 1. Precomputed lookup table
	if info, ok := data.EventFlags[id]; ok {
		return info.Byte, info.Bit
	}
	// 2. BST lookup
	data.LoadBST()
	block := id / data.BSTFlagDivisor
	if bstPos, ok := data.EventFlagBST[block]; ok {
		idx := id % data.BSTFlagDivisor
		return bstPos*data.BSTBlockSize + idx/8, uint8(7 - (idx % 8))
	}
	// 3. Fallback formula
	return id / 8, uint8(7 - (id % 8))
}

// filterInfuseVariants removes infuse-variant entries from a weapon item list.
// A variant is detected when id - N×100 (N=1..12) exists in the same list,
// meaning it is a non-standard infuse copy of a base weapon already present.
// Items with maxUpgrade != 25 are always kept (boss weapons, non-upgradeable).
func filterInfuseVariants(items []ItemEntry) []ItemEntry {
	idSet := make(map[uint32]bool, len(items))
	for _, item := range items {
		idSet[item.ID] = true
	}

	result := items[:0]
	for _, item := range items {
		if item.MaxUpgrade != 25 {
			result = append(result, item)
			continue
		}
		isVariant := false
		for n := uint32(1); n <= 12; n++ {
			offset := n * 100
			if item.ID >= offset && idSet[item.ID-offset] {
				isVariant = true
				break
			}
		}
		if !isVariant {
			result = append(result, item)
		}
	}
	return result
}

// GetInfuseTypes returns all weapon infusion types.
func GetInfuseTypes() []InfuseType {
	return InfuseTypes
}

// GetAllBosses returns all boss encounters as a flat list sorted by region then name.
var getAllBosses = sync.OnceValue(func() []BossEntry {
	bosses := make([]BossEntry, 0, len(data.Bosses))
	for id, boss := range data.Bosses {
		bosses = append(bosses, BossEntry{
			ID:          id,
			Name:        boss.Name,
			Region:      boss.Region,
			Type:        boss.Type,
			Remembrance: boss.Remembrance,
		})
	}
	sort.Slice(bosses, func(i, j int) bool {
		if bosses[i].Region != bosses[j].Region {
			return bosses[i].Region < bosses[j].Region
		}
		return bosses[i].Name < bosses[j].Name
	})
	return bosses
})

func GetAllBosses() []BossEntry { return getAllBosses() }

// GetAllSummoningPools returns all summoning pools as a flat list sorted by region then name.
var getAllSummoningPools = sync.OnceValue(func() []SummoningPoolEntry {
	pools := make([]SummoningPoolEntry, 0, len(data.SummoningPools))
	for id, pool := range data.SummoningPools {
		pools = append(pools, SummoningPoolEntry{
			ID:     id,
			Name:   pool.Name,
			Region: pool.Region,
		})
	}
	sort.Slice(pools, func(i, j int) bool {
		if pools[i].Region != pools[j].Region {
			return pools[i].Region < pools[j].Region
		}
		return pools[i].Name < pools[j].Name
	})
	return pools
})

func GetAllSummoningPools() []SummoningPoolEntry { return getAllSummoningPools() }

// IsKnownSummoningPoolID reports whether id is a recognised summoning pool flag
// in the current database (670xxx range, game >= v1.12).
func IsKnownSummoningPoolID(id uint32) bool {
	_, ok := data.SummoningPools[id]
	return ok
}

// GetAllColosseums returns all colosseums as a flat list sorted by name.
var getAllColosseums = sync.OnceValue(func() []ColosseumEntry {
	colosseums := make([]ColosseumEntry, 0, len(data.Colosseums))
	for id, c := range data.Colosseums {
		colosseums = append(colosseums, ColosseumEntry{
			ID:     id,
			Name:   c.Name,
			Region: c.Region,
		})
	}
	sort.Slice(colosseums, func(i, j int) bool {
		return colosseums[i].Name < colosseums[j].Name
	})
	return colosseums
})

func GetAllColosseums() []ColosseumEntry { return getAllColosseums() }

// GetAllRegions returns every known invasion-region entry from the database,
// sorted by Area then Name. Unlocked is left at false; callers fill it in
// from the per-slot UnlockedRegions list.
var getAllRegions = sync.OnceValue(func() []RegionEntry {
	entries := make([]RegionEntry, 0, len(data.Regions))
	for id, r := range data.Regions {
		entries = append(entries, RegionEntry{
			ID:   id,
			Name: r.Name,
			Area: r.Area,
		})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Area != entries[j].Area {
			return entries[i].Area < entries[j].Area
		}
		return entries[i].Name < entries[j].Name
	})
	return entries
})

func GetAllRegions() []RegionEntry { return getAllRegions() }

// IsKnownRegionID reports whether id is part of the curated invasion-region
// allowlist (data.Regions) — the subset of regulation.bin PlayRegionParam rows
// that are valid standard invasion / blue-summon targets. It is membership-based,
// not range-based: the curated DLC IDs are non-contiguous, and many real
// PlayRegionParam rows (internal sub-areas, network-only rows) are deliberately
// excluded. Used to distinguish curated IDs from raw save-side region IDs.
func IsKnownRegionID(id uint32) bool {
	_, ok := data.Regions[id]
	return ok
}

// IsKnownGraceID reports whether id is a recognised Site of Grace EventFlag ID
// in the current database (71000–76960, including DLC 72xxx and 74xxx).
func IsKnownGraceID(id uint32) bool {
	_, ok := data.Graces[id]
	return ok
}

// IsKnownMapFlagID reports whether id is a recognised map flag ID across all
// four map datasets: MapVisible (62xxx), MapSystem (62xxx/82xxx), MapAcquired
// (63xxx), and MapUnsafe (62xxx/63xxx).
func IsKnownMapFlagID(id uint32) bool {
	if _, ok := data.MapVisible[id]; ok {
		return true
	}
	if _, ok := data.MapSystem[id]; ok {
		return true
	}
	if _, ok := data.MapAcquired[id]; ok {
		return true
	}
	_, ok := data.MapUnsafe[id]
	return ok
}

// IsKnownColosseumID reports whether id is a recognised colosseum activate flag
// present in data.ColosseumFlagSets (60350, 60360, 60370).
func IsKnownColosseumID(id uint32) bool {
	_, ok := data.ColosseumFlagSets[id]
	return ok
}

// GetColosseumFlagSet returns the full companion flag set for the given colosseum
// activate flag ID. Returns false if id is not a known colosseum activate flag.
func GetColosseumFlagSet(id uint32) (data.ColosseumFlagSet, bool) {
	fs, ok := data.ColosseumFlagSets[id]
	return fs, ok
}

// GetAllMapEntries returns all map region entries (visible + acquired + system) sorted by area then name.
var getAllMapEntries = sync.OnceValue(func() []MapEntry {
	entries := make([]MapEntry, 0, len(data.MapVisible)+len(data.MapAcquired)+len(data.MapSystem)+len(data.MapUnsafe))
	for id, m := range data.MapSystem {
		entries = append(entries, MapEntry{ID: id, Name: m.Name, Area: m.Area, Category: "system"})
	}
	for id, m := range data.MapVisible {
		entries = append(entries, MapEntry{ID: id, Name: m.Name, Area: m.Area, Category: "visible"})
	}
	for id, m := range data.MapAcquired {
		entries = append(entries, MapEntry{ID: id, Name: m.Name, Area: m.Area, Category: "acquired"})
	}
	for id, m := range data.MapUnsafe {
		entries = append(entries, MapEntry{ID: id, Name: m.Name, Area: m.Area, Category: "unsafe"})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Area != entries[j].Area {
			return entries[i].Area < entries[j].Area
		}
		if entries[i].Category != entries[j].Category {
			return entries[i].Category < entries[j].Category
		}
		return entries[i].Name < entries[j].Name
	})
	return entries
})

func GetAllMapEntries() []MapEntry { return getAllMapEntries() }

// QuestNPC represents an NPC with their quest progression.
type QuestNPC struct {
	Name  string      `json:"name"`
	Steps []QuestStep `json:"steps"`
}

// QuestStep represents one step in an NPC questline with current flag state.
type QuestStep struct {
	Description string           `json:"description"`
	Location    string           `json:"location,omitempty"`
	Flags       []QuestFlagState `json:"flags"`
	Complete    bool             `json:"complete"` // all flags match target values
}

// QuestFlagState is a flag with its target and current value.
type QuestFlagState struct {
	ID      uint32 `json:"id"`
	Target  uint8  `json:"target"`  // expected value (0 or 1)
	Current bool   `json:"current"` // actual value in save
}

// GetAllQuestNPCs returns the list of NPC names with quest data.
var getAllQuestNPCs = sync.OnceValue(func() []string {
	names := make([]string, 0, len(data.QuestData))
	for name := range data.QuestData {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
})

func GetAllQuestNPCs() []string { return getAllQuestNPCs() }

// GetAllCookbooks returns all cookbooks sorted by category then name.
var getAllCookbooks = sync.OnceValue(func() []CookbookEntry {
	entries := make([]CookbookEntry, 0, len(data.Cookbooks))
	for id, cb := range data.Cookbooks {
		entries = append(entries, CookbookEntry{
			ID:       id,
			Name:     cb.Name,
			Category: cb.Category,
		})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Category != entries[j].Category {
			return entries[i].Category < entries[j].Category
		}
		return entries[i].Name < entries[j].Name
	})
	return entries
})

func GetAllCookbooks() []CookbookEntry { return getAllCookbooks() }

// GetAllBellBearings returns all bell bearings sorted by category then name.
var getAllBellBearings = sync.OnceValue(func() []BellBearingEntry {
	entries := make([]BellBearingEntry, 0, len(data.BellBearings))
	for id, bb := range data.BellBearings {
		entries = append(entries, BellBearingEntry{
			ID:       id,
			Name:     bb.Name,
			Category: bb.Category,
		})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Category != entries[j].Category {
			return entries[i].Category < entries[j].Category
		}
		return entries[i].Name < entries[j].Name
	})
	return entries
})

func GetAllBellBearings() []BellBearingEntry { return getAllBellBearings() }

// GetAllWhetblades returns all whetblades sorted by name.
var getAllWhetblades = sync.OnceValue(func() []WhetbladeEntry {
	entries := make([]WhetbladeEntry, 0, len(data.Whetblades))
	for id, wb := range data.Whetblades {
		entries = append(entries, WhetbladeEntry{
			ID:   id,
			Name: wb.Name,
		})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name < entries[j].Name
	})
	return entries
})

func GetAllWhetblades() []WhetbladeEntry { return getAllWhetblades() }

// SetEventFlag sets or clears a specific event flag in the bit array.
// Uses the same resolution order as GetEventFlag (lookup table → BST → fallback).
// Returns error if the computed byte offset is out of bounds.
func SetEventFlag(flags []byte, id uint32, value bool) error {
	byteIdx, bitIdx := resolveEventFlagPosition(id)
	if int(byteIdx) >= len(flags) {
		return fmt.Errorf("event flag %d (byte %d) out of bounds (flags len %d)", id, byteIdx, len(flags))
	}
	if value {
		flags[byteIdx] |= (1 << bitIdx)
	} else {
		flags[byteIdx] &= ^(1 << bitIdx)
	}
	return nil
}
