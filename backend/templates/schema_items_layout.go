package templates

import "fmt"

// Phase 8B — schema foundation for the v2 items / inventory & storage
// layout / apply options surface.
//
// This file ONLY defines the on-the-wire model and structural validators.
// It deliberately ships no exporter, no importer, no apply path, no UI
// wiring, and no Wails bindings — those are Phase 8C+ scope. The intent
// is to lock the data contract (and its hard-fail invariants) before any
// writer touches a save.
//
// Naming convention: new sections use plain camelCase keys
// (`items`, `inventoryLayout`, `storageLayout`) without the legacy dotted
// `inventory.workspace` spelling. Mixing keying schemes is intentional:
// `inventory.workspace` is preserved verbatim for backward compatibility
// with v1 documents; the new sections do not inherit that legacy.

// ─── item categories ────────────────────────────────────────────────────

// Item category strings accepted in TemplateItemEntryV2.Category. The
// allowlist is fail-closed: unknown categories are rejected by the
// validator. Values mirror backend/db/data category strings so consumers
// can resolve TemplateItemEntryV2.ItemID against the in-memory item DB
// without a translation table.
const (
	ItemCategoryMeleeArmaments      = "melee_armaments"
	ItemCategoryRangedAndCatalysts  = "ranged_and_catalysts"
	ItemCategoryShields             = "shields"
	ItemCategoryAshesOfWar          = "ashes_of_war"
	ItemCategoryArmorHead           = "head"
	ItemCategoryArmorChest          = "chest"
	ItemCategoryArmorArms           = "arms"
	ItemCategoryArmorLegs           = "legs"
	ItemCategoryTalismans           = "talismans"
	ItemCategorySorceries           = "sorceries"
	ItemCategoryIncantations        = "incantations"
	ItemCategoryTools               = "tools"
	ItemCategoryCraftingMaterials   = "crafting_materials"
	ItemCategoryBolsteringMaterials = "bolstering_materials"
	ItemCategoryArrowsAndBolts      = "arrows_and_bolts"
	ItemCategoryKeyItems            = "key_items"
	ItemCategoryGestures            = "gestures"
	ItemCategoryDLC                 = "dlc"
)

var itemCategoryAllowlist = map[string]bool{
	ItemCategoryMeleeArmaments:      true,
	ItemCategoryRangedAndCatalysts:  true,
	ItemCategoryShields:             true,
	ItemCategoryAshesOfWar:          true,
	ItemCategoryArmorHead:           true,
	ItemCategoryArmorChest:          true,
	ItemCategoryArmorArms:           true,
	ItemCategoryArmorLegs:           true,
	ItemCategoryTalismans:           true,
	ItemCategorySorceries:           true,
	ItemCategoryIncantations:        true,
	ItemCategoryTools:               true,
	ItemCategoryCraftingMaterials:   true,
	ItemCategoryBolsteringMaterials: true,
	ItemCategoryArrowsAndBolts:      true,
	ItemCategoryKeyItems:            true,
	ItemCategoryGestures:            true,
	ItemCategoryDLC:                 true,
}

// ─── upgrade kinds ──────────────────────────────────────────────────────

// Upgrade-kind discriminator for TemplateItemEntryV2.UpgradeKind. "none"
// is the sentinel for non-upgradable items (talismans, consumables, key
// items, …). Standard infusable weapons go up to +25; somber weapons cap
// at +10. There is intentionally no "unknown" kind — Phase 8B is
// fail-closed on the upgrade dimension. The empty string is accepted as
// a shorthand for "none" so a producer that does not know the kind can
// omit the field rather than guess.
const (
	UpgradeKindNone     = "none"
	UpgradeKindStandard = "standard"
	UpgradeKindSomber   = "somber"
)

const (
	// MaxItemUpgradeStandard mirrors MaxEquipmentItemUpgrade for the
	// item entry path. Kept as a separate constant so future divergence
	// (e.g. a per-category cap) does not silently re-link the two.
	MaxItemUpgradeStandard = 25
	// MaxItemUpgradeSomber caps somber-only weapons at +10.
	MaxItemUpgradeSomber = 10
)

// ─── locations ──────────────────────────────────────────────────────────

// Item location values accepted in TemplateItemEntryV2.Location. "both"
// means the same baseItemID lives in inventory AND storage as separate
// stacks; the apply layer (Phase 8C+) will decide how to split the
// quantity. Unknown locations are fail-closed.
const (
	ItemLocationInventory = "inventory"
	ItemLocationStorage   = "storage"
	ItemLocationBoth      = "both"
)

var itemLocationAllowlist = map[string]bool{
	ItemLocationInventory: true,
	ItemLocationStorage:   true,
	ItemLocationBoth:      true,
}

// ─── apply modes ────────────────────────────────────────────────────────

// ItemApplyMode controls how the apply layer (Phase 8C+) reconciles the
// template's items list with the character's current inventory. Each
// value is the conservative reading of its name; "replace" is the only
// destructive option and is gated by the caller's explicit opt-in.
const (
	// ItemApplyModeAddMissing only inserts items that the character
	// does not already own. Existing stacks are untouched. Safest mode.
	ItemApplyModeAddMissing = "addMissing"
	// ItemApplyModeUpdateExisting refreshes attributes (upgrade, AoW,
	// infusion, quantity) of items the character already owns and adds
	// nothing new.
	ItemApplyModeUpdateExisting = "updateExisting"
	// ItemApplyModeMerge is addMissing ∪ updateExisting — adds new
	// items and refreshes existing ones.
	ItemApplyModeMerge = "merge"
	// ItemApplyModeReplace deletes any inventory/storage entry not
	// present in the template before applying the template. DESTRUCTIVE
	// — surfaces in the UI must require explicit confirmation before
	// emitting this mode.
	ItemApplyModeReplace = "replace"
)

var itemApplyModeAllowlist = map[string]bool{
	ItemApplyModeAddMissing:     true,
	ItemApplyModeUpdateExisting: true,
	ItemApplyModeMerge:          true,
	ItemApplyModeReplace:        true,
}

// LayoutApplyMode controls how the apply layer reconciles the template's
// inventoryLayout / storageLayout with the live ordering. "replace" is
// the only destructive option (it discards any ordering for items not
// covered by the template's layout); other modes only reorder.
const (
	// LayoutApplyModeIgnore drops the template's layout — items are
	// applied without touching positions.
	LayoutApplyModeIgnore = "ignore"
	// LayoutApplyModeAppend places template-ordered items first, then
	// keeps any non-template items in their existing relative order
	// after that block.
	LayoutApplyModeAppend = "append"
	// LayoutApplyModeReorderOnly applies the template ordering to
	// items already present; items not in the layout are not moved.
	LayoutApplyModeReorderOnly = "reorderOnly"
	// LayoutApplyModeReplace makes the live ordering match the
	// template exactly — items not in the layout are pushed to the end
	// in undefined order. DESTRUCTIVE to ordering metadata (the items
	// themselves are not deleted by layout apply — see ItemApplyMode
	// for that).
	LayoutApplyModeReplace = "replace"
)

var layoutApplyModeAllowlist = map[string]bool{
	LayoutApplyModeIgnore:      true,
	LayoutApplyModeAppend:      true,
	LayoutApplyModeReorderOnly: true,
	LayoutApplyModeReplace:     true,
}

// ─── sections ───────────────────────────────────────────────────────────

// ItemsSection is the v2 inventory contents description, decoupled from
// container placement and ordering. Each entry has a stable EntryID
// (string slug, unique within the template) that the layout sections
// reference. Storing items in one flat list rather than per-container
// arrays means a single entry can represent stacks that live in both
// inventory and storage via the "both" location value, and avoids
// duplicating heavy weapon/AoW metadata across containers.
type ItemsSection struct {
	Entries []TemplateItemEntryV2 `json:"entries" yaml:"entries"`
}

// TemplateItemEntryV2 is the v2 per-item record. EntryID is the
// template-local identity — it must be unique within ItemsSection.Entries
// and is the only stable handle the layout sections can reference.
// Multiple entries may share the same ItemID provided they differ in
// upgrade / infusion / Ash of War (the "two Longswords with different
// Ashes" case); the validator does not collapse them.
//
// Name is informational only (mirrors TemplateItem.Name behaviour) — the
// item DB is the source of truth at apply time.
//
// Upgrade is split into UpgradeKind + UpgradeLevel so producers that
// only know "this is +5 of something" cannot accidentally ship a value
// that the apply layer would have to silently clamp. See
// validateUpgradeKindAndLevel for the per-kind invariants.
type TemplateItemEntryV2 struct {
	EntryID        string  `json:"entryID" yaml:"entryID"`
	ItemID         uint32  `json:"itemID" yaml:"itemID"`
	Name           string  `json:"name,omitempty" yaml:"name,omitempty"`
	Category       string  `json:"category" yaml:"category"`
	Quantity       uint32  `json:"quantity" yaml:"quantity"`
	Location       string  `json:"location" yaml:"location"`
	UpgradeKind    string  `json:"upgradeKind,omitempty" yaml:"upgradeKind,omitempty"`
	UpgradeLevel   *uint8  `json:"upgradeLevel,omitempty" yaml:"upgradeLevel,omitempty"`
	InfusionName   string  `json:"infusionName,omitempty" yaml:"infusionName,omitempty"`
	AshOfWarItemID *uint32 `json:"ashOfWarItemID,omitempty" yaml:"ashOfWarItemID,omitempty"`
}

// InventoryLayoutSection is the ordered view of items that live in the
// character's inventory container. Each LayoutEntry points back at an
// ItemsSection.Entries[i].EntryID via EntryRef and assigns a Position
// (integer; smaller positions sort earlier). Positions and entry refs
// must be unique within the layout. The validator does NOT require the
// positions to be contiguous, dense, or zero-based — the writer (Phase
// 8C+) will normalise them when writing back to the save.
type InventoryLayoutSection struct {
	Entries []LayoutEntry `json:"entries" yaml:"entries"`
}

// StorageLayoutSection is the storage-container counterpart of
// InventoryLayoutSection. The two are kept as separate sections rather
// than a single tagged struct so a template can ship only one of them
// (selection.storageLayout=true, selection.inventoryLayout absent) and
// the YAML / JSON tree mirrors that fact directly.
type StorageLayoutSection struct {
	Entries []LayoutEntry `json:"entries" yaml:"entries"`
}

// LayoutEntry is one row in a layout section. EntryRef must equal an
// existing ItemsSection.Entries[i].EntryID; Position is the sort key the
// apply layer will use to reorder the live container.
type LayoutEntry struct {
	EntryRef string `json:"entryRef" yaml:"entryRef"`
	Position int    `json:"position" yaml:"position"`
}

// ─── apply options ──────────────────────────────────────────────────────

// ApplyOptions is the top-level container for per-section apply
// behaviour. Unlike Sections / Selection, this is metadata about HOW to
// apply, not WHAT to apply — it is optional in every template. v1
// documents must not carry it; v2 documents may omit it (the apply layer
// will fall back to addMissing / ignore / use-template-levels defaults).
type ApplyOptions struct {
	Items               *ItemApplyOptions    `json:"items,omitempty" yaml:"items,omitempty"`
	InventoryLayout     *LayoutApplyOptions  `json:"inventoryLayout,omitempty" yaml:"inventoryLayout,omitempty"`
	StorageLayout       *LayoutApplyOptions  `json:"storageLayout,omitempty" yaml:"storageLayout,omitempty"`
	WeaponLevelOverride *WeaponLevelOverride `json:"weaponLevelOverride,omitempty" yaml:"weaponLevelOverride,omitempty"`
}

// ItemApplyOptions controls how the items list is reconciled with the
// live inventory. Mode is required (validator rejects empty / unknown
// values). PreserveExtraItems is meaningful for modes that would
// otherwise drop unfamiliar items; setting it true under mode=replace
// downgrades replace to its non-destructive interpretation (overwrite
// template-known items, leave others alone).
type ItemApplyOptions struct {
	Mode               string `json:"mode" yaml:"mode"`
	PreserveExtraItems bool   `json:"preserveExtraItems,omitempty" yaml:"preserveExtraItems,omitempty"`
}

// LayoutApplyOptions controls how a layout section is reconciled with
// the live ordering. Mode is required.
type LayoutApplyOptions struct {
	Mode string `json:"mode" yaml:"mode"`
}

// WeaponLevelOverride lets a user re-aim the upgrade levels of every
// weapon-shaped item in the template at apply time. Use cases:
//   - "honor the template levels" → UseTemplateLevels=true, both
//     overrides nil. This is the default when ApplyOptions is omitted.
//   - "flatten everything to +10 / +5" → UseTemplateLevels=false,
//     StandardOverride / SomberOverride filled.
//   - "do not touch upgrade levels at all" → UseTemplateLevels=false,
//     both overrides nil. The apply layer leaves the live upgrade alone.
//
// Validation refuses the mixed case (UseTemplateLevels=true alongside
// non-nil overrides) — that combination has no coherent reading.
type WeaponLevelOverride struct {
	UseTemplateLevels bool   `json:"useTemplateLevels" yaml:"useTemplateLevels"`
	StandardOverride  *uint8 `json:"standardOverride,omitempty" yaml:"standardOverride,omitempty"`
	SomberOverride    *uint8 `json:"somberOverride,omitempty" yaml:"somberOverride,omitempty"`
}

// ─── validators ─────────────────────────────────────────────────────────

// validateItemsSection enforces structural invariants on every entry and
// on the relations between them (unique EntryID). Apply-time semantics
// (item exists in DB, AoW is compatible with the weapon) are deferred to
// the Phase 8C resolver.
func validateItemsSection(s *ItemsSection) error {
	if s == nil {
		return nil
	}
	seen := make(map[string]int, len(s.Entries))
	for i := range s.Entries {
		e := &s.Entries[i]
		if e.EntryID == "" {
			return fmt.Errorf("ValidateBuildTemplate: items.entries[%d]: entryID is empty", i)
		}
		if prev, dup := seen[e.EntryID]; dup {
			return fmt.Errorf("ValidateBuildTemplate: items.entries[%d]: entryID %q already used at index %d", i, e.EntryID, prev)
		}
		seen[e.EntryID] = i
		if e.ItemID == 0 {
			return fmt.Errorf("ValidateBuildTemplate: items.entries[%d]: itemID=0 (entryID=%q)", i, e.EntryID)
		}
		if !itemCategoryAllowlist[e.Category] {
			return fmt.Errorf("ValidateBuildTemplate: items.entries[%d]: unknown category %q (entryID=%q)", i, e.Category, e.EntryID)
		}
		if e.Quantity == 0 {
			return fmt.Errorf("ValidateBuildTemplate: items.entries[%d]: quantity=0 not allowed (entryID=%q); clear/remove semantics belong to applyOptions.items.mode, not entry payload", i, e.EntryID)
		}
		if !itemLocationAllowlist[e.Location] {
			return fmt.Errorf("ValidateBuildTemplate: items.entries[%d]: unknown location %q (entryID=%q; allowed: inventory, storage, both)", i, e.Location, e.EntryID)
		}
		if err := validateUpgradeKindAndLevel(i, e); err != nil {
			return err
		}
		if e.AshOfWarItemID != nil && *e.AshOfWarItemID == 0 {
			return fmt.Errorf("ValidateBuildTemplate: items.entries[%d]: ashOfWarItemID=0 (entryID=%q; omit the field to mean no custom AoW)", i, e.EntryID)
		}
	}
	return nil
}

// validateUpgradeKindAndLevel enforces the per-kind level range and the
// "no level for non-upgradable item" rule.
func validateUpgradeKindAndLevel(i int, e *TemplateItemEntryV2) error {
	switch e.UpgradeKind {
	case "", UpgradeKindNone:
		if e.UpgradeLevel != nil {
			return fmt.Errorf("ValidateBuildTemplate: items.entries[%d]: upgradeLevel=%d set but upgradeKind=%q (entryID=%q; omit upgradeLevel for non-upgradable items)", i, *e.UpgradeLevel, e.UpgradeKind, e.EntryID)
		}
		return nil
	case UpgradeKindStandard:
		if e.UpgradeLevel == nil {
			return fmt.Errorf("ValidateBuildTemplate: items.entries[%d]: upgradeKind=standard requires upgradeLevel (entryID=%q)", i, e.EntryID)
		}
		if *e.UpgradeLevel > MaxItemUpgradeStandard {
			return fmt.Errorf("ValidateBuildTemplate: items.entries[%d]: upgradeLevel=%d out of range [0, %d] for upgradeKind=standard (entryID=%q)", i, *e.UpgradeLevel, MaxItemUpgradeStandard, e.EntryID)
		}
		return nil
	case UpgradeKindSomber:
		if e.UpgradeLevel == nil {
			return fmt.Errorf("ValidateBuildTemplate: items.entries[%d]: upgradeKind=somber requires upgradeLevel (entryID=%q)", i, e.EntryID)
		}
		if *e.UpgradeLevel > MaxItemUpgradeSomber {
			return fmt.Errorf("ValidateBuildTemplate: items.entries[%d]: upgradeLevel=%d out of range [0, %d] for upgradeKind=somber (entryID=%q)", i, *e.UpgradeLevel, MaxItemUpgradeSomber, e.EntryID)
		}
		return nil
	default:
		return fmt.Errorf("ValidateBuildTemplate: items.entries[%d]: unknown upgradeKind %q (entryID=%q; allowed: standard, somber, none)", i, e.UpgradeKind, e.EntryID)
	}
}

// validateLayoutSection enforces that every LayoutEntry refers to a
// known ItemsSection entry, that no EntryRef is used twice, and that no
// position collides. The items argument may be nil — that path produces
// a clear "no items section" error rather than silently passing.
func validateLayoutSection(label string, entries []LayoutEntry, items *ItemsSection) error {
	knownRefs := map[string]bool{}
	if items != nil {
		for i := range items.Entries {
			knownRefs[items.Entries[i].EntryID] = true
		}
	}
	seenRefs := make(map[string]int, len(entries))
	seenPositions := make(map[int]int, len(entries))
	for i, le := range entries {
		if le.EntryRef == "" {
			return fmt.Errorf("ValidateBuildTemplate: %s[%d]: entryRef is empty", label, i)
		}
		if !knownRefs[le.EntryRef] {
			return fmt.Errorf("ValidateBuildTemplate: %s[%d]: entryRef %q does not match any items.entries.entryID", label, i, le.EntryRef)
		}
		if prev, dup := seenRefs[le.EntryRef]; dup {
			return fmt.Errorf("ValidateBuildTemplate: %s[%d]: entryRef %q already used at index %d", label, i, le.EntryRef, prev)
		}
		seenRefs[le.EntryRef] = i
		if prev, dup := seenPositions[le.Position]; dup {
			return fmt.Errorf("ValidateBuildTemplate: %s[%d]: position=%d already used at index %d (entryRef=%q)", label, i, le.Position, prev, le.EntryRef)
		}
		seenPositions[le.Position] = i
	}
	return nil
}

// validateApplyOptions enforces the per-field invariants on every
// sub-DTO that is present. A nil sub-DTO is "use the apply layer's
// default" and is always valid.
func validateApplyOptions(o *ApplyOptions) error {
	if o == nil {
		return nil
	}
	if o.Items != nil {
		if !itemApplyModeAllowlist[o.Items.Mode] {
			return fmt.Errorf("ValidateBuildTemplate: applyOptions.items.mode=%q is invalid (allowed: addMissing, updateExisting, merge, replace)", o.Items.Mode)
		}
	}
	if o.InventoryLayout != nil {
		if !layoutApplyModeAllowlist[o.InventoryLayout.Mode] {
			return fmt.Errorf("ValidateBuildTemplate: applyOptions.inventoryLayout.mode=%q is invalid (allowed: ignore, append, reorderOnly, replace)", o.InventoryLayout.Mode)
		}
	}
	if o.StorageLayout != nil {
		if !layoutApplyModeAllowlist[o.StorageLayout.Mode] {
			return fmt.Errorf("ValidateBuildTemplate: applyOptions.storageLayout.mode=%q is invalid (allowed: ignore, append, reorderOnly, replace)", o.StorageLayout.Mode)
		}
	}
	if o.WeaponLevelOverride != nil {
		if err := validateWeaponLevelOverride(o.WeaponLevelOverride); err != nil {
			return err
		}
	}
	return nil
}

// validateWeaponLevelOverride enforces the three legal shapes (see the
// type's doc comment) and the per-override range.
func validateWeaponLevelOverride(o *WeaponLevelOverride) error {
	if o.UseTemplateLevels {
		if o.StandardOverride != nil || o.SomberOverride != nil {
			return fmt.Errorf("ValidateBuildTemplate: applyOptions.weaponLevelOverride: useTemplateLevels=true is mutually exclusive with standardOverride/somberOverride")
		}
		return nil
	}
	if o.StandardOverride != nil && *o.StandardOverride > MaxItemUpgradeStandard {
		return fmt.Errorf("ValidateBuildTemplate: applyOptions.weaponLevelOverride.standardOverride=%d out of range [0, %d]", *o.StandardOverride, MaxItemUpgradeStandard)
	}
	if o.SomberOverride != nil && *o.SomberOverride > MaxItemUpgradeSomber {
		return fmt.Errorf("ValidateBuildTemplate: applyOptions.weaponLevelOverride.somberOverride=%d out of range [0, %d]", *o.SomberOverride, MaxItemUpgradeSomber)
	}
	return nil
}

// ─── selection guards ───────────────────────────────────────────────────

// validateBooleanOnlySelection refuses any per-field selection map for
// the section named by label. Used by selection.items /
// selection.inventoryLayout / selection.storageLayout — none of these
// have per-field semantics, so the boolean shortcut is the only legal
// shape.
func validateBooleanOnlySelection(label string, sel *SectionSelection) error {
	if sel == nil {
		return nil
	}
	if sel.Fields != nil {
		return fmt.Errorf("ValidateBuildTemplate: %s accepts only a boolean (got a field map)", label)
	}
	return nil
}
