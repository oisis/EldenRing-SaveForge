// Package templates implements the SaveForge Build Template — a portable,
// versioned JSON representation of an Inventory Workspace snapshot.
//
// Templates capture only stable game-content identifiers (baseItemID,
// quantity, upgrade, infusion name, AoW item ID). Save-local addressing
// (GaItem handles, session UIDs, acquisition indices) is deliberately
// excluded so a template can be applied to another save without collision.
//
// Phase A is exporter-only: this package converts an editor snapshot into
// a BuildTemplate and validates the resulting schema. There is no import
// path, no file I/O, no Wails binding here. Phase B wires the exporter to
// a file dialog; Phase D/E introduce import.
package templates

// SchemaKey identifies a build template payload. Importers must reject
// any document whose `schema` field does not match exactly.
const SchemaKey = "saveforge.build-template"

// SchemaVersion is the maximum schema version this package can produce
// and validate. Bumped only on breaking changes to BuildTemplate /
// TemplateItem field semantics.
const SchemaVersion = 1

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
type BuildTemplate struct {
	Schema     string            `json:"schema"`
	Version    int               `json:"version"`
	CreatedAt  string            `json:"createdAt"`
	AppVersion string            `json:"appVersion,omitempty"`
	Metadata   *TemplateMetadata `json:"metadata,omitempty"`
	Sections   TemplateSections  `json:"sections"`
}

// TemplateMetadata is purely informational. None of these fields drive
// import behavior; they exist so a user can label and discover templates.
type TemplateMetadata struct {
	Name                 string   `json:"name,omitempty"`
	Description          string   `json:"description,omitempty"`
	Author               string   `json:"author,omitempty"`
	Tags                 []string `json:"tags,omitempty"`
	SourceCharacterIndex int      `json:"sourceCharacterIndex,omitempty"`
	SourceCharacterName  string   `json:"sourceCharacterName,omitempty"`
}

// TemplateSections groups payload sections by stable key. Phase A defines
// only inventory.workspace. Future sections (character.profile, etc.)
// extend this struct and are gated by section-level $enabled flags.
type TemplateSections struct {
	InventoryWorkspace *InventoryWorkspaceSection `json:"inventory.workspace,omitempty"`
}

// InventoryWorkspaceSection is the Phase A payload — items from the
// editor workspace's inventory and storage containers.
type InventoryWorkspaceSection struct {
	InventoryItems []TemplateItem `json:"inventoryItems"`
	StorageItems   []TemplateItem `json:"storageItems"`
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
	BaseItemID   uint32  `json:"baseItemID"`
	Name         string  `json:"name,omitempty"`
	Category     string  `json:"category,omitempty"`
	Quantity     uint32  `json:"quantity"`
	Upgrade      int     `json:"upgrade,omitempty"`
	InfusionName string  `json:"infusionName,omitempty"`
	AoWItemID    *uint32 `json:"aowItemID,omitempty"`
	Container    string  `json:"container"`
	Position     int     `json:"position"`
}

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
