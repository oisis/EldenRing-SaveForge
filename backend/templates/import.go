package templates

import (
	"encoding/json"
	"fmt"

	"github.com/oisis/EldenRing-SaveForge/backend/db"
)

// ImportPreviewOptions controls the (very small) Phase C policy surface.
// Mode is a forward-compat string that names the future merge strategy
// (append / replace-inventory / replace-all). Phase C only validates
// "append" (the default and only mode); other values are accepted and
// passed through but do not change behavior yet.
type ImportPreviewOptions struct {
	Mode string `json:"mode,omitempty"`
}

// ImportPreviewReport is the dry-run diff. Phase C is read-only; this
// struct never carries pointers back into mutable workspace state, so
// the frontend can render it without coordinating lifetime with a
// session.
type ImportPreviewReport struct {
	OK       bool                  `json:"ok"`
	Errors   []ImportPreviewIssue  `json:"errors"`
	Warnings []ImportPreviewIssue  `json:"warnings"`
	Summary  ImportPreviewSummary  `json:"summary"`
}

// ImportPreviewIssue is one row in the preview's errors/warnings table.
// Optional positional fields (Container/Position/BaseItemID/AoWItemID)
// let the UI deep-link to a specific item; Code is the stable
// machine-readable token.
type ImportPreviewIssue struct {
	Severity   string `json:"severity"`
	Code       string `json:"code"`
	Message    string `json:"message"`
	Container  string `json:"container,omitempty"`
	Position   int    `json:"position,omitempty"`
	BaseItemID uint32 `json:"baseItemID,omitempty"`
	AoWItemID  uint32 `json:"aowItemID,omitempty"`
}

// ImportPreviewSummary counts items by resolved DB category so the UI
// can show "12 weapons, 4 armor, 2 talismans" at a glance. Counts are
// based on what the DB says about each baseItemID — the template's
// Category field is debug only, mirroring the same rule applied to
// Name in §3.1 of spec/55-build-template.md.
//
// Phase 3C.0 additions (schema-version-aware metadata):
//   - Version mirrors the template's `version` field so a UI can render
//     a v1/v2 badge without re-parsing the payload.
//   - SelectedSections enumerates the sections this template elects to
//     ship. For v1 templates (no Selection object) the list is derived
//     from `sections` — `inventory.workspace` when that section exists.
//     For v2 templates the list reflects `selection`.
//   - ProfileFieldsPresent / StatFieldsPresent list the keys whose
//     payload pointer is non-nil in `sections.profile` / `sections.stats`.
//     Both lists are alphabetic (stable across runs).
type ImportPreviewSummary struct {
	InventoryItems       int      `json:"inventoryItems"`
	StorageItems         int      `json:"storageItems"`
	Weapons              int      `json:"weapons"`
	Armor                int      `json:"armor"`
	Talismans            int      `json:"talismans"`
	Stackables           int      `json:"stackables"`
	AoWAssignments       int      `json:"aowAssignments"`
	Version              int      `json:"version,omitempty"`
	SelectedSections     []string `json:"selectedSections,omitempty"`
	ProfileFieldsPresent []string `json:"profileFieldsPresent,omitempty"`
	StatFieldsPresent    []string `json:"statFieldsPresent,omitempty"`
}

// Issue codes — stable strings. UI surfaces and tests assert on these.
const (
	IssueCodeSchemaInvalid       = "schema_invalid"
	IssueCodeStructureInvalid    = "structure_invalid"
	IssueCodeUnknownItem         = "unknown_item"
	IssueCodeQuantityNonPositive = "quantity_non_positive"
	IssueCodeUpgradeOutOfRange   = "upgrade_out_of_range"
	IssueCodeUnknownInfusion     = "unknown_infusion"
	IssueCodeAoWNotWeapon        = "aow_not_weapon_target"
	IssueCodeAoWNotAshCategory   = "aow_not_ash_category"
	IssueCodeAoWIncompatible     = "aow_incompatible"
	IssueCodeAoWCompatUnknown    = "aow_compat_unknown"
	IssueCodeNameMismatch        = "name_mismatch_ignored"
	IssueCodeUnknownMode         = "unknown_mode"
	IssueCodeCapacityExceeded    = "capacity_exceeded"
	IssueCodeUnsupportedCategory = "unsupported_category"

	// Phase 6b — runtime-only weapon level override codes. Surfaced as
	// warnings on the apply report; never block the apply.
	IssueCodeWeaponLevelClamped  = "weapon_level_clamped"
	IssueCodeWeaponUnupgradeable = "weapon_unupgradeable"
)

// ashCategory is the DB tag for an Ash of War item. Used to detect
// "user pointed aowItemID at a non-AoW thing" — a common authoring
// error that must fail closed.
const ashCategory = "ashes_of_war"

// weapon/armor/talisman category buckets for the summary counter.
// Mirrors editor.SupportedCategories but kept locally so the templates
// package does not pull in editor (which would create a cycle).
var (
	weaponCategories = map[string]bool{
		"melee_armaments":      true,
		"ranged_and_catalysts": true,
		"shields":              true,
	}
	armorCategories = map[string]bool{
		"head":  true,
		"chest": true,
		"arms":  true,
		"legs":  true,
	}
)

// ParseBuildTemplateJSON unmarshals the JSON bytes into a BuildTemplate
// and runs the structural validator. Returned errors are flat (not
// wrapped into a preview report) because a malformed payload cannot
// produce a per-item diff. The App-level caller is responsible for
// translating these into a preview report with code=structure_invalid
// when that is the right UX.
func ParseBuildTemplateJSON(data []byte) (*BuildTemplate, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("ParseBuildTemplateJSON: empty payload")
	}
	var tpl BuildTemplate
	if err := json.Unmarshal(data, &tpl); err != nil {
		return nil, fmt.Errorf("ParseBuildTemplateJSON: %w", err)
	}
	if err := ValidateBuildTemplate(&tpl); err != nil {
		return nil, fmt.Errorf("ParseBuildTemplateJSON: %w", err)
	}
	return &tpl, nil
}

// PreviewBuildTemplateImport produces a dry-run report against the
// current DB. It does NOT touch any save, workspace, or session — the
// only state it reads is the in-memory item database loaded at app
// startup.
//
// Validation rules (Phase C):
//   - Each item's BaseItemID must resolve via db.GetItemDataFuzzy.
//   - Quantity > 0.
//   - InfusionName, when present, must match a db.InfuseTypes entry.
//   - Upgrade must satisfy 0 <= Upgrade <= MaxUpgrade for the resolved item.
//   - If aowItemID is set:
//     * the AoW item must resolve and have category "ashes_of_war"
//     * the target item must be weapon-like (db category in weaponCategories)
//     * db.IsAshOfWarCompatibleWithWeapon must report (true, true).
//       known=false produces an error (fail-closed) per the rule
//       documented in db.IsAoWCompatibleWithWepType.
//   - Template's own Name / Category fields are debug only.
//   - "name_mismatch_ignored" warning surfaces when the template's
//     Name field doesn't match the DB — informational, never blocking.
//   - Unknown Mode (anything other than "" / "append") produces a
//     warning so the user knows they hit a forward-compat path.
func PreviewBuildTemplateImport(tpl *BuildTemplate, opts ImportPreviewOptions) ImportPreviewReport {
	rep := ImportPreviewReport{
		Errors:   []ImportPreviewIssue{},
		Warnings: []ImportPreviewIssue{},
	}
	if tpl == nil {
		rep.Errors = append(rep.Errors, ImportPreviewIssue{
			Severity: "error",
			Code:     IssueCodeStructureInvalid,
			Message:  "template payload is nil",
		})
		rep.OK = false
		return rep
	}
	if err := ValidateBuildTemplate(tpl); err != nil {
		rep.Errors = append(rep.Errors, ImportPreviewIssue{
			Severity: "error",
			Code:     IssueCodeSchemaInvalid,
			Message:  err.Error(),
		})
		rep.OK = false
		return rep
	}
	if opts.Mode != "" && opts.Mode != "append" {
		rep.Warnings = append(rep.Warnings, ImportPreviewIssue{
			Severity: "warning",
			Code:     IssueCodeUnknownMode,
			Message:  fmt.Sprintf("import mode %q is not implemented yet; preview proceeds as if mode=append", opts.Mode),
		})
	}

	rep.Summary.Version = tpl.Version
	rep.Summary.SelectedSections = selectedSectionsForTemplate(tpl)
	rep.Summary.ProfileFieldsPresent = profileFieldsPresent(tpl.Sections.Profile)
	rep.Summary.StatFieldsPresent = statFieldsPresent(tpl.Sections.Stats)

	// v1 documents are guaranteed by ValidateBuildTemplate to carry a
	// non-nil InventoryWorkspace; v2 documents may omit it (selection
	// is the source of truth). Guard so the per-item preview path
	// stays safe for both shapes — Phase 3A intentionally skips v2-
	// only sections (profile / stats) here; per-section preview lands
	// in a later phase together with the apply layer.
	if sec := tpl.Sections.InventoryWorkspace; sec != nil {
		rep.Summary.InventoryItems = len(sec.InventoryItems)
		rep.Summary.StorageItems = len(sec.StorageItems)
		previewItems(sec.InventoryItems, ContainerInventory, &rep)
		previewItems(sec.StorageItems, ContainerStorage, &rep)
	}

	rep.OK = len(rep.Errors) == 0
	return rep
}

// previewItems applies the per-item validation rules. The errors /
// warnings / summary fields on rep are mutated in place.
func previewItems(items []TemplateItem, container string, rep *ImportPreviewReport) {
	for _, it := range items {
		// quantity is also enforced by ValidateBuildTemplate, but we
		// re-check here so the error surfaces with rich positional
		// context (the structural validator returns a flat error
		// message; the preview produces a per-item issue).
		if it.Quantity == 0 {
			rep.Errors = append(rep.Errors, ImportPreviewIssue{
				Severity:   "error",
				Code:       IssueCodeQuantityNonPositive,
				Message:    "quantity must be > 0",
				Container:  container,
				Position:   it.Position,
				BaseItemID: it.BaseItemID,
			})
			continue
		}

		itemData, _ := db.GetItemDataFuzzy(it.BaseItemID)
		if itemData.Name == "" {
			rep.Errors = append(rep.Errors, ImportPreviewIssue{
				Severity:   "error",
				Code:       IssueCodeUnknownItem,
				Message:    fmt.Sprintf("baseItemID 0x%08X does not resolve in the item database", it.BaseItemID),
				Container:  container,
				Position:   it.Position,
				BaseItemID: it.BaseItemID,
			})
			continue
		}

		// Surface a non-blocking warning when the template's Name was
		// captured from a different localisation / older patch than
		// the current DB. The import will still use the DB name; this
		// just helps users spot drifted templates.
		if it.Name != "" && it.Name != itemData.Name {
			rep.Warnings = append(rep.Warnings, ImportPreviewIssue{
				Severity:   "warning",
				Code:       IssueCodeNameMismatch,
				Message:    fmt.Sprintf("template name %q does not match DB name %q (DB is source of truth)", it.Name, itemData.Name),
				Container:  container,
				Position:   it.Position,
				BaseItemID: it.BaseItemID,
			})
		}

		if it.Upgrade < 0 || uint32(it.Upgrade) > itemData.MaxUpgrade {
			rep.Errors = append(rep.Errors, ImportPreviewIssue{
				Severity:   "error",
				Code:       IssueCodeUpgradeOutOfRange,
				Message:    fmt.Sprintf("upgrade %d outside 0..%d for %q", it.Upgrade, itemData.MaxUpgrade, itemData.Name),
				Container:  container,
				Position:   it.Position,
				BaseItemID: it.BaseItemID,
			})
		}

		if it.InfusionName != "" && !isKnownInfusion(it.InfusionName) {
			rep.Errors = append(rep.Errors, ImportPreviewIssue{
				Severity:   "error",
				Code:       IssueCodeUnknownInfusion,
				Message:    fmt.Sprintf("infusion %q is not in the DB", it.InfusionName),
				Container:  container,
				Position:   it.Position,
				BaseItemID: it.BaseItemID,
			})
		}

		// Bucket by resolved DB category for the summary. Items that
		// failed earlier checks (continue'd above) are excluded; items
		// with a non-fatal upgrade/infusion issue still count so the
		// user sees the intended shape of the import.
		switch {
		case weaponCategories[itemData.Category]:
			rep.Summary.Weapons++
		case armorCategories[itemData.Category]:
			rep.Summary.Armor++
		case itemData.Category == "talismans":
			rep.Summary.Talismans++
		default:
			rep.Summary.Stackables++
		}

		if it.AoWItemID != nil && *it.AoWItemID != 0 {
			previewAoWAssignment(it, itemData.Category, rep, container)
		}
	}
}

// previewAoWAssignment validates a single weapon ↔ AoW pairing. It
// produces at most one error per item — the first failure short-circuits
// the rest of the chain because compatibility check only makes sense
// once the target is confirmed weapon-like and the gem is confirmed an
// Ash of War.
func previewAoWAssignment(it TemplateItem, weaponCategory string, rep *ImportPreviewReport, container string) {
	aowID := *it.AoWItemID
	if !weaponCategories[weaponCategory] {
		rep.Errors = append(rep.Errors, ImportPreviewIssue{
			Severity:   "error",
			Code:       IssueCodeAoWNotWeapon,
			Message:    fmt.Sprintf("item %q has category %q and cannot mount an AoW", it.Name, weaponCategory),
			Container:  container,
			Position:   it.Position,
			BaseItemID: it.BaseItemID,
			AoWItemID:  aowID,
		})
		return
	}
	aowData := db.GetItemData(aowID)
	if aowData.Name == "" {
		rep.Errors = append(rep.Errors, ImportPreviewIssue{
			Severity:   "error",
			Code:       IssueCodeUnknownItem,
			Message:    fmt.Sprintf("AoW 0x%08X does not resolve in the item database", aowID),
			Container:  container,
			Position:   it.Position,
			BaseItemID: it.BaseItemID,
			AoWItemID:  aowID,
		})
		return
	}
	if aowData.Category != ashCategory {
		rep.Errors = append(rep.Errors, ImportPreviewIssue{
			Severity:   "error",
			Code:       IssueCodeAoWNotAshCategory,
			Message:    fmt.Sprintf("aowItemID 0x%08X resolves to %q in category %q, not %q", aowID, aowData.Name, aowData.Category, ashCategory),
			Container:  container,
			Position:   it.Position,
			BaseItemID: it.BaseItemID,
			AoWItemID:  aowID,
		})
		return
	}
	compatible, known := db.IsAshOfWarCompatibleWithWeapon(aowID, it.BaseItemID)
	if !known {
		rep.Errors = append(rep.Errors, ImportPreviewIssue{
			Severity:   "error",
			Code:       IssueCodeAoWCompatUnknown,
			Message:    fmt.Sprintf("AoW compatibility data missing for %q on %q — failing closed", aowData.Name, it.Name),
			Container:  container,
			Position:   it.Position,
			BaseItemID: it.BaseItemID,
			AoWItemID:  aowID,
		})
		return
	}
	if !compatible {
		rep.Errors = append(rep.Errors, ImportPreviewIssue{
			Severity:   "error",
			Code:       IssueCodeAoWIncompatible,
			Message:    fmt.Sprintf("AoW %q cannot be mounted on %q (incompatible weapon type)", aowData.Name, it.Name),
			Container:  container,
			Position:   it.Position,
			BaseItemID: it.BaseItemID,
			AoWItemID:  aowID,
		})
		return
	}
	rep.Summary.AoWAssignments++
}

// isKnownInfusion is the lookup helper for InfusionName validation.
// Empty input is allowed (the exporter omits "Standard"); callers handle
// that case before reaching here.
func isKnownInfusion(name string) bool {
	for _, t := range db.InfuseTypes {
		if t.Name == name {
			return true
		}
	}
	return false
}

// selectedSectionsForTemplate returns the stable-ordered list of sections
// this template elects to ship. Used by preview summary and library entry.
//
// v1 (Selection == nil): derive from sections — only the legacy
// `inventory.workspace` shape exists, so the result is either
// ["inventory.workspace"] or empty.
//
// v2 (Selection != nil): enumerate Selection.* in a fixed order so the
// output is deterministic regardless of map iteration. The chosen order
// is inventory.workspace → profile → stats — explicit rather than
// sort.Strings so future v2 sections (equipment, appearance, ...) slot
// in at a defined position rather than wherever their alphabetic
// position lands.
func selectedSectionsForTemplate(tpl *BuildTemplate) []string {
	if tpl == nil {
		return nil
	}
	if tpl.Selection == nil {
		var out []string
		if tpl.Sections.InventoryWorkspace != nil {
			out = append(out, "inventory.workspace")
		}
		return out
	}
	var out []string
	if tpl.Selection.InventoryWorkspace.HasAny() {
		out = append(out, "inventory.workspace")
	}
	if tpl.Selection.Profile.HasAny() {
		out = append(out, "profile")
	}
	if tpl.Selection.Stats.HasAny() {
		out = append(out, "stats")
	}
	return out
}

// profileFieldsPresent returns the alphabetic list of profile keys whose
// payload pointer is non-nil. Distinct from "selected" — a key may be
// selected without being present (selected but unset is dropped by the
// v2 builder, but a hand-written template can still ship that shape) and
// the preview surfaces the on-disk truth.
func profileFieldsPresent(p *ProfileSection) []string {
	if p == nil {
		return nil
	}
	var out []string
	if p.Class != nil {
		out = append(out, "class")
	}
	if p.ClearCount != nil {
		out = append(out, "clearCount")
	}
	if p.Level != nil {
		out = append(out, "level")
	}
	if p.Name != nil {
		out = append(out, "name")
	}
	if p.Runes != nil {
		out = append(out, "runes")
	}
	if p.ScadutreeBlessing != nil {
		out = append(out, "scadutreeBlessing")
	}
	if p.ShadowRealmBlessing != nil {
		out = append(out, "shadowRealmBlessing")
	}
	if p.SoulMemory != nil {
		out = append(out, "soulMemory")
	}
	if p.TalismanSlots != nil {
		out = append(out, "talismanSlots")
	}
	return out
}

// statFieldsPresent returns the alphabetic list of stat keys whose
// payload pointer is non-nil.
func statFieldsPresent(s *StatsSection) []string {
	if s == nil {
		return nil
	}
	var out []string
	if s.Arcane != nil {
		out = append(out, "arcane")
	}
	if s.Dexterity != nil {
		out = append(out, "dexterity")
	}
	if s.Endurance != nil {
		out = append(out, "endurance")
	}
	if s.Faith != nil {
		out = append(out, "faith")
	}
	if s.Intelligence != nil {
		out = append(out, "intelligence")
	}
	if s.Mind != nil {
		out = append(out, "mind")
	}
	if s.Strength != nil {
		out = append(out, "strength")
	}
	if s.Vigor != nil {
		out = append(out, "vigor")
	}
	return out
}
