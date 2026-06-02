// Package templates implements the SaveForge Build Template — a portable,
// versioned JSON representation of an Inventory Workspace snapshot.
//
// Templates capture only stable game-content identifiers (baseItemID,
// quantity, upgrade, infusion name, AoW item ID). Save-local addressing
// (GaItem handles, session UIDs, acquisition indices) is deliberately
// excluded so a template can be applied to another save without collision.
//
// Phase A was exporter-only for the v1 (inventory + storage) schema.
// Phase 3A adds a structural-only draft of schema v2 — `version: 2`
// documents may declare optional `profile` / `stats` sections and a
// top-level `selection` object. There is NO v2 builder, NO v2 apply
// path, and NO Wails-side surface in this phase. The reader is
// extended additively; the v1 exporter continues to emit `version: 1`.
package templates

import (
	"encoding/json"
	"fmt"
	"unicode/utf16"

	"gopkg.in/yaml.v3"
)

// SchemaKey identifies a build template payload. Importers must reject
// any document whose `schema` field does not match exactly.
const SchemaKey = "saveforge.build-template"

// SchemaVersion is the version the v1 exporter (BuildTemplateFromSnapshot)
// emits. It is deliberately NOT bumped when MaxSchemaVersion grows — the
// v1 builder still produces v1 documents, and only an explicit future v2
// builder will emit `version: 2`.
const SchemaVersion = 1

// MaxSchemaVersion is the highest version this package can parse and
// validate. Reader range is `1 ≤ version ≤ MaxSchemaVersion`. v1
// readers (older app builds) reject v2 via the existing unsupported-
// version error path; v2 readers always accept v1.
const MaxSchemaVersion = 2

// Player-field range caps used by the v2 structural validator. They
// mirror the writer-side clamps in backend/vm/character_vm.go and
// backend/core/structures.go so a v2 preview refuses values the writer
// would silently clamp.
const (
	MaxProfileLevel               = 713
	MaxProfileClearCount          = 7
	MaxProfileScadutreeBlessing   = 20
	MaxProfileShadowRealmBlessing = 10
	MaxProfileTalismanSlots       = 3
	MaxProfileNameUTF16Units      = 16
	MinStatValue                  = 1
	MaxStatValue                  = 99
)

// Warning codes emitted by BuildTemplateFromSnapshot. Stable strings —
// importer UIs and tests assert on these.
const (
	WarnCodeAoWMissingSkipped  = "aow_missing_skipped"
	WarnCodeAoWSharedSkipped   = "aow_shared_skipped"
	WarnCodePositionNormalized = "position_normalized"
)

// Container values used in template payloads. Stable strings — must match
// editor.ContainerKind values so future import path can compare directly.
const (
	ContainerInventory = "inventory"
	ContainerStorage   = "storage"
)

// BuildTemplate is the on-disk representation of a portable inventory
// loadout. Only stable game-content identifiers are stored; nothing in
// this struct is bound to a specific source save.
//
// Phase 3A adds:
//   - Selection (top-level, v2 only) — describes which sections / fields
//     a v2 document elects to share. Required for `version: 2`; absent /
//     ignored for `version: 1`.
//   - Sections.Profile / Sections.Stats — optional v2 sections. v1
//     documents must not include them and v2 documents must select them
//     via Selection before they become load-bearing.
type BuildTemplate struct {
	Schema     string             `json:"schema" yaml:"schema"`
	Version    int                `json:"version" yaml:"version"`
	CreatedAt  string             `json:"createdAt" yaml:"createdAt"`
	AppVersion string             `json:"appVersion,omitempty" yaml:"appVersion,omitempty"`
	Metadata   *TemplateMetadata  `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	Selection  *TemplateSelection `json:"selection,omitempty" yaml:"selection,omitempty"`
	Sections   TemplateSections   `json:"sections" yaml:"sections"`
}

// TemplateMetadata is purely informational. None of these fields drive
// import behavior; they exist so a user can label and discover templates.
type TemplateMetadata struct {
	Name                 string   `json:"name,omitempty" yaml:"name,omitempty"`
	Description          string   `json:"description,omitempty" yaml:"description,omitempty"`
	Author               string   `json:"author,omitempty" yaml:"author,omitempty"`
	Tags                 []string `json:"tags,omitempty" yaml:"tags,omitempty"`
	SourceCharacterIndex int      `json:"sourceCharacterIndex,omitempty" yaml:"sourceCharacterIndex,omitempty"`
	SourceCharacterName  string   `json:"sourceCharacterName,omitempty" yaml:"sourceCharacterName,omitempty"`
}

// TemplateSections groups payload sections by stable key. Phase A
// defined only inventory.workspace; Phase 3A adds optional profile /
// stats. Section-level enablement for v2 is driven by Selection — the
// presence of a section in this struct is not, by itself, a request to
// include it on apply.
//
// The inventory.workspace key keeps its literal v1 spelling (with a
// dot). The preferred-alias decision (`inventoryWorkspace` without a
// dot) is deliberately deferred — see spec/56 §18 #4.
//
// Phase 7b.1 adds Equipment — the optional equipped weapons / ammo /
// armor section. Driven by the same Selection model as profile / stats.
// Talismans, spells, EquippedGreatRune, and the unknown slots 11 / 16
// stay out of scope of this phase per Phase 7b.0 writer coverage.
//
// Phase 7d.1 adds Spells — the optional 14-slot equipped-spells loadout.
// Spells live in their own save region (EquippedSpells offset, hash[10])
// and use raw MagicParam IDs at apply time, so they ship as a separate
// section rather than as more fields on EquipmentSection.
type TemplateSections struct {
	InventoryWorkspace *InventoryWorkspaceSection `json:"inventory.workspace,omitempty" yaml:"inventory.workspace,omitempty"`
	Profile            *ProfileSection            `json:"profile,omitempty" yaml:"profile,omitempty"`
	Stats              *StatsSection              `json:"stats,omitempty" yaml:"stats,omitempty"`
	Equipment          *EquipmentSection          `json:"equipment,omitempty" yaml:"equipment,omitempty"`
	Spells             *SpellsSection             `json:"spells,omitempty" yaml:"spells,omitempty"`
}

// InventoryWorkspaceSection is the v1 payload — items from the
// editor workspace's inventory and storage containers.
type InventoryWorkspaceSection struct {
	InventoryItems []TemplateItem `json:"inventoryItems" yaml:"inventoryItems"`
	StorageItems   []TemplateItem `json:"storageItems" yaml:"storageItems"`
}

// TemplateItem describes a single portable inventory entry.
//
// Why pointer for AoWItemID: a custom Ash of War assignment is optional.
// Encoding "no custom AoW" as a literal 0 or as the in-save sentinel
// handle would leak save-local addressing into the template. A nil
// pointer + omitempty produces a JSON document with the field absent for
// weapons that have no custom AoW (or where the source AoW state was
// missing/shared and could not be safely exported).
//
// Why no OriginalHandle / UID / AcquisitionIndex: those are session- and
// save-local. Including them would tie a template to one save's GaItem
// layout, defeating portability.
type TemplateItem struct {
	BaseItemID   uint32  `json:"baseItemID" yaml:"baseItemID"`
	Name         string  `json:"name,omitempty" yaml:"name,omitempty"`
	Category     string  `json:"category,omitempty" yaml:"category,omitempty"`
	Quantity     uint32  `json:"quantity" yaml:"quantity"`
	Upgrade      int     `json:"upgrade,omitempty" yaml:"upgrade,omitempty"`
	InfusionName string  `json:"infusionName,omitempty" yaml:"infusionName,omitempty"`
	AoWItemID    *uint32 `json:"aowItemID,omitempty" yaml:"aowItemID,omitempty"`
	Container    string  `json:"container" yaml:"container"`
	Position     int     `json:"position" yaml:"position"`
}

// ProfileSection carries the safe-semantic single-character profile
// fields that Phase 3A allows to appear in a v2 document. All fields
// are optional — a writer emits only the values it actually wants to
// share, and the reader treats absent fields as "not set". The Class
// field is captured as a stable display name string for forward
// compatibility; Phase 3A does not require a DB lookup at validation
// time. The intentional exclusions (gender, voiceType, raw FaceData,
// raw event flags) are documented in spec/56 §7.
type ProfileSection struct {
	Name                *string `json:"name,omitempty" yaml:"name,omitempty"`
	Level               *uint32 `json:"level,omitempty" yaml:"level,omitempty"`
	Runes               *uint32 `json:"runes,omitempty" yaml:"runes,omitempty"`
	SoulMemory          *uint32 `json:"soulMemory,omitempty" yaml:"soulMemory,omitempty"`
	Class               *string `json:"class,omitempty" yaml:"class,omitempty"`
	ClearCount          *uint32 `json:"clearCount,omitempty" yaml:"clearCount,omitempty"`
	ScadutreeBlessing   *uint8  `json:"scadutreeBlessing,omitempty" yaml:"scadutreeBlessing,omitempty"`
	ShadowRealmBlessing *uint8  `json:"shadowRealmBlessing,omitempty" yaml:"shadowRealmBlessing,omitempty"`
	TalismanSlots       *uint8  `json:"talismanSlots,omitempty" yaml:"talismanSlots,omitempty"`
}

// StatsSection carries the 8 character stats. Optional pointer fields
// follow the same "omitempty means not set" rule as ProfileSection so a
// writer can share only a subset.
type StatsSection struct {
	Vigor        *uint32 `json:"vigor,omitempty" yaml:"vigor,omitempty"`
	Mind         *uint32 `json:"mind,omitempty" yaml:"mind,omitempty"`
	Endurance    *uint32 `json:"endurance,omitempty" yaml:"endurance,omitempty"`
	Strength     *uint32 `json:"strength,omitempty" yaml:"strength,omitempty"`
	Dexterity    *uint32 `json:"dexterity,omitempty" yaml:"dexterity,omitempty"`
	Intelligence *uint32 `json:"intelligence,omitempty" yaml:"intelligence,omitempty"`
	Faith        *uint32 `json:"faith,omitempty" yaml:"faith,omitempty"`
	Arcane       *uint32 `json:"arcane,omitempty" yaml:"arcane,omitempty"`
}

// EquipmentSection carries the equipped slots inside ChrAsmEquipment that
// the writer foundation supports. Phase 7b.1 covered the 14 weapon / ammo /
// armor slots. Phase 7c extends the section with the 5 talisman slots
// (indices 17–21, hash 8) — they live in the same ChrAsmEquipment struct
// and reuse the same resolver/preview infrastructure, so they ship as more
// EquipmentSection fields rather than as a separate sections.equippedTalismans
// section (see Phase 7c preflight decision).
//
// Each pointer is optional — nil means "slot not selected, no-op on apply".
// To explicitly clear an equipped slot at apply time the producer emits an
// EquipmentItemRef with BaseItemID == 0; the resolver then sends Handle == 0
// to SaveSlot.WriteEquipment which writes 0xFFFFFFFF (empty marker) to the
// underlying byte slot.
//
// Talisman semantics (Phase 7c):
//   - Vanilla Elden Ring caps the Talisman Pouch at 1 base slot + 3 upgrades
//     = 4 active slots. Slot 5 (index 21) exists in the binary but is never
//     reachable via in-game gameplay; the resolver therefore warns + skips
//     any non-empty talisman5 ref. Clear (baseItemID = 0) is always allowed.
//   - Talisman refs use baseItemID + name only. Upgrade / infusionName /
//     aowItemID exist on EquipmentItemRef but talismans never carry them;
//     the resolver ignores those fields for talisman slot lookups.
//   - Active slot count = 1 + profile.talismanSlots. Refs targeting slots
//     beyond the active count emit talisman_slot_pouch_insufficient warning
//     and are skipped. Mixed templates that also set profile.talismanSlots
//     evaluate pouch state AFTER profile apply so they can lift the cap.
//
// Out of scope of Phase 7c:
//   - EquippedGreatRune (slot 10, written by SyncPlayerToData via
//     ProfileSection's future Great Rune field — never via WriteEquipment)
//   - unk0x2C / unk0x40 (slots 11, 16 — unknown semantics)
//   - EquippedSpells 14-slot loadout (Phase 7d)
//   - Quick items / pouch slots (no current write API)
type EquipmentSection struct {
	WeaponLeftHand1  *EquipmentItemRef `json:"weaponLeftHand1,omitempty" yaml:"weaponLeftHand1,omitempty"`
	WeaponRightHand1 *EquipmentItemRef `json:"weaponRightHand1,omitempty" yaml:"weaponRightHand1,omitempty"`
	WeaponLeftHand2  *EquipmentItemRef `json:"weaponLeftHand2,omitempty" yaml:"weaponLeftHand2,omitempty"`
	WeaponRightHand2 *EquipmentItemRef `json:"weaponRightHand2,omitempty" yaml:"weaponRightHand2,omitempty"`
	WeaponLeftHand3  *EquipmentItemRef `json:"weaponLeftHand3,omitempty" yaml:"weaponLeftHand3,omitempty"`
	WeaponRightHand3 *EquipmentItemRef `json:"weaponRightHand3,omitempty" yaml:"weaponRightHand3,omitempty"`
	Arrows1          *EquipmentItemRef `json:"arrows1,omitempty" yaml:"arrows1,omitempty"`
	Bolts1           *EquipmentItemRef `json:"bolts1,omitempty" yaml:"bolts1,omitempty"`
	Arrows2          *EquipmentItemRef `json:"arrows2,omitempty" yaml:"arrows2,omitempty"`
	Bolts2           *EquipmentItemRef `json:"bolts2,omitempty" yaml:"bolts2,omitempty"`
	ArmorHead        *EquipmentItemRef `json:"armorHead,omitempty" yaml:"armorHead,omitempty"`
	ArmorChest       *EquipmentItemRef `json:"armorChest,omitempty" yaml:"armorChest,omitempty"`
	ArmorArms        *EquipmentItemRef `json:"armorArms,omitempty" yaml:"armorArms,omitempty"`
	ArmorLegs        *EquipmentItemRef `json:"armorLegs,omitempty" yaml:"armorLegs,omitempty"`
	Talisman1        *EquipmentItemRef `json:"talisman1,omitempty" yaml:"talisman1,omitempty"`
	Talisman2        *EquipmentItemRef `json:"talisman2,omitempty" yaml:"talisman2,omitempty"`
	Talisman3        *EquipmentItemRef `json:"talisman3,omitempty" yaml:"talisman3,omitempty"`
	Talisman4        *EquipmentItemRef `json:"talisman4,omitempty" yaml:"talisman4,omitempty"`
	Talisman5        *EquipmentItemRef `json:"talisman5,omitempty" yaml:"talisman5,omitempty"`
}

// EquipmentItemRef points at one inventory item the apply path should
// equip into the slot named by its parent field on EquipmentSection.
// Resolution rules (apply layer, Phase 7b.1):
//
//   - BaseItemID == 0 → explicit clear (write 0xFFFFFFFF / empty slot).
//   - BaseItemID > 0 → match in slot.Inventory.CommonItems by BaseItemID,
//     optionally narrowed by Upgrade / InfusionName / AoWItemID. Storage
//     is intentionally NOT searched; an item that only exists in storage
//     is reported as "not in inventory" per Phase 7b.1 strict policy.
//   - Upgrade nil / omitted → match any upgrade level for the same
//     BaseItemID. Upgrade explicit → match the exact level.
//   - InfusionName empty → match any infusion. InfusionName set → match
//     the exact infusion.
//   - AoWItemID nil → match any AoW. AoWItemID set → match the exact AoW.
//   - Ambiguous match (>1 candidate after disambiguators) → first wins,
//     resolver emits an equipment_item_ambiguous warning.
//
// Name is informational only (mirrors TemplateItem.Name behaviour) — the
// DB is the source of truth.
type EquipmentItemRef struct {
	BaseItemID   uint32  `json:"baseItemID" yaml:"baseItemID"`
	Name         string  `json:"name,omitempty" yaml:"name,omitempty"`
	Upgrade      *int    `json:"upgrade,omitempty" yaml:"upgrade,omitempty"`
	InfusionName string  `json:"infusionName,omitempty" yaml:"infusionName,omitempty"`
	AoWItemID    *uint32 `json:"aowItemID,omitempty" yaml:"aowItemID,omitempty"`
}

// TemplateSelection is the v2 top-level "what is shared" object.
// Required for `version: 2`. The structure mirrors Sections — each
// optional pointer either elects (boolean shortcut) the entire section
// or names individual fields via SectionSelection.Fields. Unknown
// section keys are rejected by strict YAML decode (KnownFields(true))
// applied to this struct.
type TemplateSelection struct {
	Profile            *SectionSelection `json:"profile,omitempty" yaml:"profile,omitempty"`
	Stats              *SectionSelection `json:"stats,omitempty" yaml:"stats,omitempty"`
	InventoryWorkspace *SectionSelection `json:"inventory.workspace,omitempty" yaml:"inventory.workspace,omitempty"`
	Equipment          *SectionSelection `json:"equipment,omitempty" yaml:"equipment,omitempty"`
	Spells             *SectionSelection `json:"spells,omitempty" yaml:"spells,omitempty"`
}

// SectionSelection is a per-section toggle that accepts either a
// boolean shortcut (the whole section) or a per-field map. The on-the-
// wire representation chosen by the writer is preserved on round-trip:
//   - `true` / `false` → All set, Fields nil → emits as a bare boolean.
//   - `{ level: true, runes: true }` → Fields populated, All false →
//     emits as a mapping.
// HasAny / Selected drive validation and (in later phases) the apply
// plan; they treat the boolean shortcut as "every field selected".
type SectionSelection struct {
	All    bool
	Fields map[string]bool
}

// Selected reports whether the named field is included in this
// section's selection. The All shortcut wins over field-level state.
func (s *SectionSelection) Selected(field string) bool {
	if s == nil {
		return false
	}
	if s.All {
		return true
	}
	return s.Fields[field]
}

// HasAny reports whether at least one field (or the All shortcut) is
// selected. A nil selection or an all-false mapping returns false.
func (s *SectionSelection) HasAny() bool {
	if s == nil {
		return false
	}
	if s.All {
		return true
	}
	for _, v := range s.Fields {
		if v {
			return true
		}
	}
	return false
}

// HasAnySelected reports whether the TemplateSelection nominates at
// least one section. Used by the v2 validator to refuse empty
// `selection: {}` documents.
func (t *TemplateSelection) HasAnySelected() bool {
	if t == nil {
		return false
	}
	return t.Profile.HasAny() || t.Stats.HasAny() || t.InventoryWorkspace.HasAny() || t.Equipment.HasAny() || t.Spells.HasAny()
}

// UnmarshalJSON accepts either a JSON boolean (shortcut) or a JSON
// object whose values are booleans. Any other shape is a hard error.
func (s *SectionSelection) UnmarshalJSON(data []byte) error {
	var b bool
	if err := json.Unmarshal(data, &b); err == nil {
		s.All = b
		s.Fields = nil
		return nil
	}
	var m map[string]bool
	if err := json.Unmarshal(data, &m); err != nil {
		return fmt.Errorf("SectionSelection: must be a boolean or a map of field→bool: %w", err)
	}
	s.All = false
	s.Fields = m
	return nil
}

// MarshalJSON emits the original shape: bare boolean when Fields is
// nil, otherwise the field map. The map round-trips faithfully so
// "explicit false per field" survives a re-encode.
func (s SectionSelection) MarshalJSON() ([]byte, error) {
	if s.Fields != nil {
		return json.Marshal(s.Fields)
	}
	return json.Marshal(s.All)
}

// UnmarshalYAML mirrors UnmarshalJSON for the yaml.v3 codec. Scalar
// nodes decode as boolean; mapping nodes decode as map[string]bool.
// Any other node kind is rejected.
func (s *SectionSelection) UnmarshalYAML(node *yaml.Node) error {
	switch node.Kind {
	case yaml.ScalarNode:
		var b bool
		if err := node.Decode(&b); err != nil {
			return fmt.Errorf("SectionSelection: scalar must be a boolean: %w", err)
		}
		s.All = b
		s.Fields = nil
		return nil
	case yaml.MappingNode:
		var m map[string]bool
		if err := node.Decode(&m); err != nil {
			return fmt.Errorf("SectionSelection: %w", err)
		}
		s.All = false
		s.Fields = m
		return nil
	default:
		return fmt.Errorf("SectionSelection: unsupported YAML node kind %d", node.Kind)
	}
}

// MarshalYAML mirrors MarshalJSON: emit boolean when Fields is nil,
// otherwise the map.
func (s SectionSelection) MarshalYAML() (interface{}, error) {
	if s.Fields != nil {
		return s.Fields, nil
	}
	return s.All, nil
}

// profileSelectionFields enumerates the legal selection keys for the
// profile section. Anything outside this set is rejected by the v2
// validator (strict-mode policy chosen for Phase 3A; a softer
// "warning-only" channel may land in the preview layer later).
var profileSelectionFields = map[string]bool{
	"name":                true,
	"level":               true,
	"runes":               true,
	"soulMemory":          true,
	"class":               true,
	"clearCount":          true,
	"scadutreeBlessing":   true,
	"shadowRealmBlessing": true,
	"talismanSlots":       true,
}

// statsSelectionFields enumerates the legal selection keys for the
// stats section.
var statsSelectionFields = map[string]bool{
	"vigor":        true,
	"mind":         true,
	"endurance":    true,
	"strength":     true,
	"dexterity":    true,
	"intelligence": true,
	"faith":        true,
	"arcane":       true,
}

// equipmentSelectionFields enumerates the legal slot keys for the
// equipment section. Mirrors the JSON field names of EquipmentSection.
// Phase 7b.1 shipped the 14 weapon/ammo/armor slots; Phase 7c extends
// the set with the 5 talisman slots. Spell / great rune / unknown slots
// remain intentionally absent — they have no current writer entry point.
var equipmentSelectionFields = map[string]bool{
	"weaponLeftHand1":  true,
	"weaponRightHand1": true,
	"weaponLeftHand2":  true,
	"weaponRightHand2": true,
	"weaponLeftHand3":  true,
	"weaponRightHand3": true,
	"arrows1":          true,
	"bolts1":           true,
	"arrows2":          true,
	"bolts2":           true,
	"armorHead":        true,
	"armorChest":       true,
	"armorArms":        true,
	"armorLegs":        true,
	"talisman1":        true,
	"talisman2":        true,
	"talisman3":        true,
	"talisman4":        true,
	"talisman5":        true,
}

// EquipmentSlotOrder is the canonical iteration order over the supported
// equipment slot keys. The apply layer's AppliedFields / SkippedFields
// lists ("equipment.weaponRightHand1") and the export-side enumeration
// both walk this slice so the on-the-wire order is deterministic and
// matches the slot index order in core.ChrAsmEquipment (0..9, 12..15,
// 17..21).
var EquipmentSlotOrder = []string{
	"weaponLeftHand1",
	"weaponRightHand1",
	"weaponLeftHand2",
	"weaponRightHand2",
	"weaponLeftHand3",
	"weaponRightHand3",
	"arrows1",
	"bolts1",
	"arrows2",
	"bolts2",
	"armorHead",
	"armorChest",
	"armorArms",
	"armorLegs",
	"talisman1",
	"talisman2",
	"talisman3",
	"talisman4",
	"talisman5",
}

// MaxEquipmentItemUpgrade is the largest upgrade level any equippable
// item in Elden Ring carries (standard infusable weapons cap at +25).
// Somber weapons cap at +10 but the structural validator accepts any
// value in [0, MaxEquipmentItemUpgrade]; the per-item check that an
// upgrade > MaxUpgrade for the resolved item happens at apply time in
// the resolver, not here, so producers who do not know the upgrade cap
// can still ship a structurally-valid template.
const MaxEquipmentItemUpgrade = 25

// validateBuildTemplateV2 is the additive validator for `version: 2`.
// v1 documents take the dedicated v1 branch in ValidateBuildTemplate.
//
// Rules (Phase 3A):
//  1. Selection is required and must elect at least one section.
//  2. Every selected section must have a corresponding entry in
//     Sections.
//  3. Selection keys are restricted to the section-specific allowlist
//     (profile / stats). The inventory.workspace selection accepts only
//     a boolean shortcut.
//  4. Each present section is validated structurally — ranges for
//     profile / stats, existing item-level checks for inventory.workspace.
//     Sections present but not selected are still structurally validated;
//     metadata that fails its own invariants is rejected.
//  5. Unlike v1, inventory.workspace is NOT required and (when present)
//     is allowed to be empty — selection is the source of truth for
//     what is shared.
func validateBuildTemplateV2(tpl *BuildTemplate) error {
	if tpl.Selection == nil {
		return fmt.Errorf("ValidateBuildTemplate: v2 template requires a selection object")
	}
	if !tpl.Selection.HasAnySelected() {
		return fmt.Errorf("ValidateBuildTemplate: v2 template selection has no selected fields")
	}

	if err := validateProfileSelection(tpl.Selection.Profile); err != nil {
		return err
	}
	if err := validateStatsSelection(tpl.Selection.Stats); err != nil {
		return err
	}
	if err := validateInventoryWorkspaceSelection(tpl.Selection.InventoryWorkspace); err != nil {
		return err
	}
	if err := validateEquipmentSelection(tpl.Selection.Equipment); err != nil {
		return err
	}
	if err := validateSpellsSelection(tpl.Selection.Spells); err != nil {
		return err
	}

	if tpl.Selection.Profile.HasAny() && tpl.Sections.Profile == nil {
		return fmt.Errorf("ValidateBuildTemplate: selection.profile is selected but sections.profile is missing")
	}
	if tpl.Selection.Stats.HasAny() && tpl.Sections.Stats == nil {
		return fmt.Errorf("ValidateBuildTemplate: selection.stats is selected but sections.stats is missing")
	}
	if tpl.Selection.InventoryWorkspace.HasAny() && tpl.Sections.InventoryWorkspace == nil {
		return fmt.Errorf("ValidateBuildTemplate: selection.inventory.workspace is selected but sections.inventory.workspace is missing")
	}
	if tpl.Selection.Equipment.HasAny() && tpl.Sections.Equipment == nil {
		return fmt.Errorf("ValidateBuildTemplate: selection.equipment is selected but sections.equipment is missing")
	}
	if tpl.Selection.Spells.HasAny() && tpl.Sections.Spells == nil {
		return fmt.Errorf("ValidateBuildTemplate: selection.spells is selected but sections.spells is missing")
	}

	if tpl.Sections.Profile != nil {
		if err := validateProfileSection(tpl.Sections.Profile); err != nil {
			return err
		}
	}
	if tpl.Sections.Stats != nil {
		if err := validateStatsSection(tpl.Sections.Stats); err != nil {
			return err
		}
	}
	if tpl.Sections.InventoryWorkspace != nil {
		if err := validateItems(tpl.Sections.InventoryWorkspace.InventoryItems, ContainerInventory); err != nil {
			return err
		}
		if err := validateItems(tpl.Sections.InventoryWorkspace.StorageItems, ContainerStorage); err != nil {
			return err
		}
	}
	if tpl.Sections.Equipment != nil {
		if err := validateEquipmentSection(tpl.Sections.Equipment); err != nil {
			return err
		}
	}
	if tpl.Sections.Spells != nil {
		if err := validateSpellsSection(tpl.Sections.Spells); err != nil {
			return err
		}
	}
	return nil
}

// validateProfileSelection enforces the allowlist on
// selection.profile.Fields. Boolean shortcut and nil are accepted
// without further checks.
func validateProfileSelection(sel *SectionSelection) error {
	if sel == nil {
		return nil
	}
	for key := range sel.Fields {
		if !profileSelectionFields[key] {
			return fmt.Errorf("ValidateBuildTemplate: selection.profile has unknown field %q", key)
		}
	}
	return nil
}

// validateStatsSelection enforces the allowlist on selection.stats.Fields.
func validateStatsSelection(sel *SectionSelection) error {
	if sel == nil {
		return nil
	}
	for key := range sel.Fields {
		if !statsSelectionFields[key] {
			return fmt.Errorf("ValidateBuildTemplate: selection.stats has unknown field %q", key)
		}
	}
	return nil
}

// validateInventoryWorkspaceSelection refuses anything other than the
// boolean shortcut — there is no per-field selection inside
// inventory.workspace in Phase 3A.
func validateInventoryWorkspaceSelection(sel *SectionSelection) error {
	if sel == nil {
		return nil
	}
	if sel.Fields != nil {
		return fmt.Errorf("ValidateBuildTemplate: selection.inventory.workspace accepts only a boolean (got a field map)")
	}
	return nil
}

// validateProfileSection enforces structural ranges on every present
// profile field. Ranges mirror writer-side clamps so a preview refuses
// values the apply layer would silently coerce.
func validateProfileSection(p *ProfileSection) error {
	if p.Name != nil {
		name := *p.Name
		if name == "" {
			return fmt.Errorf("ValidateBuildTemplate: profile.name is empty")
		}
		if len(utf16.Encode([]rune(name))) > MaxProfileNameUTF16Units {
			return fmt.Errorf("ValidateBuildTemplate: profile.name exceeds %d UTF-16 code units (vm.CharacterName cap)", MaxProfileNameUTF16Units)
		}
	}
	if p.Level != nil {
		if *p.Level < 1 || *p.Level > MaxProfileLevel {
			return fmt.Errorf("ValidateBuildTemplate: profile.level=%d out of range [1, %d]", *p.Level, MaxProfileLevel)
		}
	}
	if p.Class != nil && *p.Class == "" {
		return fmt.Errorf("ValidateBuildTemplate: profile.class is empty")
	}
	if p.ClearCount != nil && *p.ClearCount > MaxProfileClearCount {
		return fmt.Errorf("ValidateBuildTemplate: profile.clearCount=%d out of range [0, %d]", *p.ClearCount, MaxProfileClearCount)
	}
	if p.ScadutreeBlessing != nil && *p.ScadutreeBlessing > MaxProfileScadutreeBlessing {
		return fmt.Errorf("ValidateBuildTemplate: profile.scadutreeBlessing=%d out of range [0, %d]", *p.ScadutreeBlessing, MaxProfileScadutreeBlessing)
	}
	if p.ShadowRealmBlessing != nil && *p.ShadowRealmBlessing > MaxProfileShadowRealmBlessing {
		return fmt.Errorf("ValidateBuildTemplate: profile.shadowRealmBlessing=%d out of range [0, %d]", *p.ShadowRealmBlessing, MaxProfileShadowRealmBlessing)
	}
	if p.TalismanSlots != nil && *p.TalismanSlots > MaxProfileTalismanSlots {
		return fmt.Errorf("ValidateBuildTemplate: profile.talismanSlots=%d out of range [0, %d]", *p.TalismanSlots, MaxProfileTalismanSlots)
	}
	return nil
}

// validateStatsSection enforces 1..99 on every present stat. Empty
// section (all pointers nil) is allowed — the section may still be
// referenced by selection but contribute nothing on apply.
func validateStatsSection(s *StatsSection) error {
	stats := []struct {
		name string
		val  *uint32
	}{
		{"vigor", s.Vigor},
		{"mind", s.Mind},
		{"endurance", s.Endurance},
		{"strength", s.Strength},
		{"dexterity", s.Dexterity},
		{"intelligence", s.Intelligence},
		{"faith", s.Faith},
		{"arcane", s.Arcane},
	}
	for _, st := range stats {
		if st.val == nil {
			continue
		}
		if *st.val < MinStatValue || *st.val > MaxStatValue {
			return fmt.Errorf("ValidateBuildTemplate: stats.%s=%d out of range [%d, %d]", st.name, *st.val, MinStatValue, MaxStatValue)
		}
	}
	return nil
}

// validateEquipmentSelection enforces the allowlist on
// selection.equipment.Fields. Boolean shortcut and nil are accepted
// without further checks (the boolean shortcut means "every slot the
// section ships").
func validateEquipmentSelection(sel *SectionSelection) error {
	if sel == nil {
		return nil
	}
	for key := range sel.Fields {
		if !equipmentSelectionFields[key] {
			return fmt.Errorf("ValidateBuildTemplate: selection.equipment has unknown slot %q", key)
		}
	}
	return nil
}

// validateEquipmentSection enforces structural ranges on every present
// equipment slot. baseItemID == 0 is accepted as the explicit-clear
// sentinel; non-zero IDs are only sanity-checked here (the resolver
// rejects unknown IDs at apply time with the richer per-slot context).
// Upgrade is validated against MaxEquipmentItemUpgrade — the "is it
// upgradable at all / what is the per-item cap" check is deferred to
// the apply resolver where the resolved DB entry is known.
func validateEquipmentSection(eq *EquipmentSection) error {
	for _, slotKey := range EquipmentSlotOrder {
		ref := equipmentSlotRef(eq, slotKey)
		if ref == nil {
			continue
		}
		if err := validateEquipmentItemRef(slotKey, ref); err != nil {
			return err
		}
	}
	return nil
}

// validateEquipmentItemRef runs the per-ref structural checks. Kept
// separate so the tests can exercise it directly.
func validateEquipmentItemRef(slotKey string, ref *EquipmentItemRef) error {
	if ref.Upgrade != nil {
		if *ref.Upgrade < 0 {
			return fmt.Errorf("ValidateBuildTemplate: equipment.%s.upgrade=%d is negative", slotKey, *ref.Upgrade)
		}
		if *ref.Upgrade > MaxEquipmentItemUpgrade {
			return fmt.Errorf("ValidateBuildTemplate: equipment.%s.upgrade=%d out of range [0, %d]", slotKey, *ref.Upgrade, MaxEquipmentItemUpgrade)
		}
	}
	if ref.AoWItemID != nil && *ref.AoWItemID == 0 {
		return fmt.Errorf("ValidateBuildTemplate: equipment.%s.aowItemID=0 is invalid (omit the field to mean any-AoW)", slotKey)
	}
	return nil
}

// equipmentSlotRef returns the pointer field on eq matching the given
// canonical slot key. Returns nil for unknown keys (defensive — callers
// only pass keys from EquipmentSlotOrder, which is allowlisted).
func equipmentSlotRef(eq *EquipmentSection, slotKey string) *EquipmentItemRef {
	if eq == nil {
		return nil
	}
	switch slotKey {
	case "weaponLeftHand1":
		return eq.WeaponLeftHand1
	case "weaponRightHand1":
		return eq.WeaponRightHand1
	case "weaponLeftHand2":
		return eq.WeaponLeftHand2
	case "weaponRightHand2":
		return eq.WeaponRightHand2
	case "weaponLeftHand3":
		return eq.WeaponLeftHand3
	case "weaponRightHand3":
		return eq.WeaponRightHand3
	case "arrows1":
		return eq.Arrows1
	case "bolts1":
		return eq.Bolts1
	case "arrows2":
		return eq.Arrows2
	case "bolts2":
		return eq.Bolts2
	case "armorHead":
		return eq.ArmorHead
	case "armorChest":
		return eq.ArmorChest
	case "armorArms":
		return eq.ArmorArms
	case "armorLegs":
		return eq.ArmorLegs
	case "talisman1":
		return eq.Talisman1
	case "talisman2":
		return eq.Talisman2
	case "talisman3":
		return eq.Talisman3
	case "talisman4":
		return eq.Talisman4
	case "talisman5":
		return eq.Talisman5
	}
	return nil
}

// EquipmentSlotRef returns the EquipmentItemRef pointer for the named
// canonical slot key, or nil when the slot is not populated. Exported so
// the apply resolver and export builder can walk slots by key without
// duplicating the switch. Returns nil when eq is nil or slotKey is not
// one of the 14 Phase 7b.1 slot names.
func EquipmentSlotRef(eq *EquipmentSection, slotKey string) *EquipmentItemRef {
	return equipmentSlotRef(eq, slotKey)
}

// SetEquipmentSlotRef stores ref at the canonical slot field named by
// slotKey. No-op when eq is nil or slotKey is not one of the 14 Phase
// 7b.1 slot names. Used by the export builder to populate slots from
// the saved character without duplicating the switch.
func SetEquipmentSlotRef(eq *EquipmentSection, slotKey string, ref *EquipmentItemRef) {
	if eq == nil {
		return
	}
	switch slotKey {
	case "weaponLeftHand1":
		eq.WeaponLeftHand1 = ref
	case "weaponRightHand1":
		eq.WeaponRightHand1 = ref
	case "weaponLeftHand2":
		eq.WeaponLeftHand2 = ref
	case "weaponRightHand2":
		eq.WeaponRightHand2 = ref
	case "weaponLeftHand3":
		eq.WeaponLeftHand3 = ref
	case "weaponRightHand3":
		eq.WeaponRightHand3 = ref
	case "arrows1":
		eq.Arrows1 = ref
	case "bolts1":
		eq.Bolts1 = ref
	case "arrows2":
		eq.Arrows2 = ref
	case "bolts2":
		eq.Bolts2 = ref
	case "armorHead":
		eq.ArmorHead = ref
	case "armorChest":
		eq.ArmorChest = ref
	case "armorArms":
		eq.ArmorArms = ref
	case "armorLegs":
		eq.ArmorLegs = ref
	case "talisman1":
		eq.Talisman1 = ref
	case "talisman2":
		eq.Talisman2 = ref
	case "talisman3":
		eq.Talisman3 = ref
	case "talisman4":
		eq.Talisman4 = ref
	case "talisman5":
		eq.Talisman5 = ref
	}
}

// ─── helpers ────────────────────────────────────────────────────────────

// ExportWarning is a non-fatal note produced during export. UI surfaces
// these so the user knows when AoW state was dropped or positions
// renormalized.
type ExportWarning struct {
	Code      string `json:"code"`
	UID       string `json:"uid,omitempty"`
	Container string `json:"container,omitempty"`
	Position  int    `json:"position"`
	Message   string `json:"message"`
}

// ExportReport is the side-channel returned alongside a built template.
// Empty Warnings means a clean export.
type ExportReport struct {
	Warnings []ExportWarning `json:"warnings"`
}

// ─── spells (Phase 7d.1) ────────────────────────────────────────────────

// SpellSlotCount is the fixed number of equipped-spell slots in the save
// (memory + skills shortcuts combined). Mirrors the constant used by the
// core writer (core.EquippedSpellSlotCount).
const SpellSlotCount = 14

// SpellItemIDPrefix is the goods-item prefix used by both sorceries and
// incantations in the project's item-ID database (e.g. Catch Flame is
// 0x40001770). Verified against backend/db/data/{sorceries,incantations}.go
// — both categories use the 0x40000000 prefix; there is NO 0x60-prefixed
// incantation form in this codebase.
const SpellItemIDPrefix uint32 = 0x40000000

// SpellItemIDPrefixMask is the high-nibble mask used to extract the
// type prefix from a full item ID. Mirrors backend/db.ItemIDToHandlePrefix
// conventions (4-bit prefix, 28-bit payload).
const SpellItemIDPrefixMask uint32 = 0xF0000000

// SpellsSection captures up to 14 equipped-spell slots as a template.
// Each slot is a pointer so absent fields round-trip as nil (the field
// is omitted from JSON/YAML via omitempty) — semantically "slot not
// targeted by this template; apply leaves the live slot untouched."
// An explicit *SpellSlotRef with BaseItemID == 0 means "clear this
// slot," mirroring the EquipmentSection clear convention.
//
// The DB-style full item ID is stored here (e.g. 0x40001770 for Catch
// Flame). Conversion to the raw 28-bit MagicParam ID that the core
// writer expects is deferred to the apply phase (Phase 7d.3) via a
// helper in backend/db. Storing the full DB-style ID keeps templates
// human-greppable and consistent with EquipmentSection.
type SpellsSection struct {
	Spell1  *SpellSlotRef `json:"spell1,omitempty" yaml:"spell1,omitempty"`
	Spell2  *SpellSlotRef `json:"spell2,omitempty" yaml:"spell2,omitempty"`
	Spell3  *SpellSlotRef `json:"spell3,omitempty" yaml:"spell3,omitempty"`
	Spell4  *SpellSlotRef `json:"spell4,omitempty" yaml:"spell4,omitempty"`
	Spell5  *SpellSlotRef `json:"spell5,omitempty" yaml:"spell5,omitempty"`
	Spell6  *SpellSlotRef `json:"spell6,omitempty" yaml:"spell6,omitempty"`
	Spell7  *SpellSlotRef `json:"spell7,omitempty" yaml:"spell7,omitempty"`
	Spell8  *SpellSlotRef `json:"spell8,omitempty" yaml:"spell8,omitempty"`
	Spell9  *SpellSlotRef `json:"spell9,omitempty" yaml:"spell9,omitempty"`
	Spell10 *SpellSlotRef `json:"spell10,omitempty" yaml:"spell10,omitempty"`
	Spell11 *SpellSlotRef `json:"spell11,omitempty" yaml:"spell11,omitempty"`
	Spell12 *SpellSlotRef `json:"spell12,omitempty" yaml:"spell12,omitempty"`
	Spell13 *SpellSlotRef `json:"spell13,omitempty" yaml:"spell13,omitempty"`
	Spell14 *SpellSlotRef `json:"spell14,omitempty" yaml:"spell14,omitempty"`
}

// SpellSlotRef is the per-slot payload. Name is optional metadata for
// human-readability and is not used by the validator or (future) apply
// resolver.
type SpellSlotRef struct {
	BaseItemID uint32 `json:"baseItemID" yaml:"baseItemID"`
	Name       string `json:"name,omitempty" yaml:"name,omitempty"`
}

// SpellSlotOrder is the canonical iteration order over the 14 spell
// slots. spell1 maps to save slot index 0, spell14 to index 13.
var SpellSlotOrder = []string{
	"spell1", "spell2", "spell3", "spell4", "spell5", "spell6", "spell7",
	"spell8", "spell9", "spell10", "spell11", "spell12", "spell13", "spell14",
}

// spellsSelectionFields enumerates the legal keys for a per-field
// selection of sections.spells. Mirrors equipmentSelectionFields.
var spellsSelectionFields = map[string]bool{
	"spell1": true, "spell2": true, "spell3": true, "spell4": true,
	"spell5": true, "spell6": true, "spell7": true, "spell8": true,
	"spell9": true, "spell10": true, "spell11": true, "spell12": true,
	"spell13": true, "spell14": true,
}

// spellSlotRef returns the pointer for slotKey, or nil if the key is
// not a known spell slot. Mirrors equipmentSlotRef.
func spellSlotRef(s *SpellsSection, slotKey string) *SpellSlotRef {
	if s == nil {
		return nil
	}
	switch slotKey {
	case "spell1":
		return s.Spell1
	case "spell2":
		return s.Spell2
	case "spell3":
		return s.Spell3
	case "spell4":
		return s.Spell4
	case "spell5":
		return s.Spell5
	case "spell6":
		return s.Spell6
	case "spell7":
		return s.Spell7
	case "spell8":
		return s.Spell8
	case "spell9":
		return s.Spell9
	case "spell10":
		return s.Spell10
	case "spell11":
		return s.Spell11
	case "spell12":
		return s.Spell12
	case "spell13":
		return s.Spell13
	case "spell14":
		return s.Spell14
	}
	return nil
}

// SpellSlotRefBySlotKey returns the pointer at the named slot field on s.
// Exported wrapper over the private spellSlotRef getter so the Phase
// 7d.3 apply resolver (which lives outside the templates package) can
// walk SpellsSection without re-implementing the 14-key switch. Returns
// nil when s is nil or the key is unknown. Mirrors
// EquipmentSlotRef-as-a-package-API.
func SpellSlotRefBySlotKey(s *SpellsSection, slotKey string) *SpellSlotRef {
	return spellSlotRef(s, slotKey)
}

// setSpellSlotRef assigns ref into the named slot field on s. No-op
// when the key is unknown so the export pipeline can iterate over
// SpellSlotOrder without re-checking allowlist membership. Mirrors
// SetEquipmentSlotRef but kept unexported — the only caller today is
// the v2 spells export builder inside this package.
func setSpellSlotRef(s *SpellsSection, slotKey string, ref *SpellSlotRef) {
	if s == nil {
		return
	}
	switch slotKey {
	case "spell1":
		s.Spell1 = ref
	case "spell2":
		s.Spell2 = ref
	case "spell3":
		s.Spell3 = ref
	case "spell4":
		s.Spell4 = ref
	case "spell5":
		s.Spell5 = ref
	case "spell6":
		s.Spell6 = ref
	case "spell7":
		s.Spell7 = ref
	case "spell8":
		s.Spell8 = ref
	case "spell9":
		s.Spell9 = ref
	case "spell10":
		s.Spell10 = ref
	case "spell11":
		s.Spell11 = ref
	case "spell12":
		s.Spell12 = ref
	case "spell13":
		s.Spell13 = ref
	case "spell14":
		s.Spell14 = ref
	}
}

// validateSpellsSelection enforces the per-field allowlist for
// selection.spells.Fields. Boolean shortcut and nil are accepted
// without further checks.
func validateSpellsSelection(sel *SectionSelection) error {
	if sel == nil {
		return nil
	}
	for key := range sel.Fields {
		if !spellsSelectionFields[key] {
			return fmt.Errorf("ValidateBuildTemplate: selection.spells has unknown slot %q", key)
		}
	}
	return nil
}

// validateSpellsSection enforces structural ranges on every present
// spell slot. baseItemID == 0 is accepted as the explicit-clear
// sentinel; non-zero IDs must carry the 0x40000000 spell prefix.
// DB membership lookups are intentionally deferred to the apply
// resolver (mirrors validateEquipmentSection).
func validateSpellsSection(s *SpellsSection) error {
	for _, slotKey := range SpellSlotOrder {
		ref := spellSlotRef(s, slotKey)
		if ref == nil {
			continue
		}
		if err := validateSpellSlotRef(slotKey, ref); err != nil {
			return err
		}
	}
	return nil
}

// validateSpellSlotRef runs the per-ref structural checks. Kept
// separate so the tests can exercise it directly.
func validateSpellSlotRef(slotKey string, ref *SpellSlotRef) error {
	if ref.BaseItemID == 0 {
		// Explicit clear: accepted unconditionally.
		return nil
	}
	if (ref.BaseItemID & SpellItemIDPrefixMask) != SpellItemIDPrefix {
		return fmt.Errorf("ValidateBuildTemplate: spells.%s.baseItemID=0x%08X has wrong prefix (expected 0x4XXXXXXX)", slotKey, ref.BaseItemID)
	}
	return nil
}
