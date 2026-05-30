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
type TemplateSections struct {
	InventoryWorkspace *InventoryWorkspaceSection `json:"inventory.workspace,omitempty" yaml:"inventory.workspace,omitempty"`
	Profile            *ProfileSection            `json:"profile,omitempty" yaml:"profile,omitempty"`
	Stats              *StatsSection              `json:"stats,omitempty" yaml:"stats,omitempty"`
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
	return t.Profile.HasAny() || t.Stats.HasAny() || t.InventoryWorkspace.HasAny()
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

	if tpl.Selection.Profile.HasAny() && tpl.Sections.Profile == nil {
		return fmt.Errorf("ValidateBuildTemplate: selection.profile is selected but sections.profile is missing")
	}
	if tpl.Selection.Stats.HasAny() && tpl.Sections.Stats == nil {
		return fmt.Errorf("ValidateBuildTemplate: selection.stats is selected but sections.stats is missing")
	}
	if tpl.Selection.InventoryWorkspace.HasAny() && tpl.Sections.InventoryWorkspace == nil {
		return fmt.Errorf("ValidateBuildTemplate: selection.inventory.workspace is selected but sections.inventory.workspace is missing")
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
