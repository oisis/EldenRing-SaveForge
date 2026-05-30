package templates

import (
	"fmt"
	"sort"
	"time"

	"github.com/oisis/EldenRing-SaveForge/backend/editor"
)

// ExportOptions controls what BuildTemplateFromSnapshot copies into the
// resulting template. At least one of IncludeInventory / IncludeStorage
// must be true.
//
// Now is exposed so tests can produce deterministic CreatedAt values.
// Production callers leave it as zero; the exporter then uses time.Now
// in UTC.
type ExportOptions struct {
	IncludeInventory bool
	IncludeStorage   bool
	AppVersion       string
	Metadata         *TemplateMetadata
	Now              time.Time
}

// BuildTemplateFromSnapshot converts an editor.InventoryWorkspaceSnapshot
// into a portable BuildTemplate. It only reads from snap; the snapshot
// is not mutated.
//
// Phase A discipline:
//   - Only EditableItem fields that survive cross-save transfer are
//     copied. Handles, UIDs, acquisition indices, GaItem flags, pending*
//     edits and shape-detection flags (isWeapon, isArmor, isTalisman,
//     hasGaItem) are intentionally dropped.
//   - CurrentAoWStatus drives whether aowItemID is exported:
//     custom → emitted, none → omitted, missing/shared → omitted +
//     warning.
//   - Unsupported pass-through records from the snapshot are NOT
//     exported in Phase A. The MVP scope is the editable allow-list
//     (weapons / armor / talismans).
//   - Item positions are renormalized to array indices after a stable
//     sort by EditableItem.Position. Any divergence between the input
//     Position field and the resulting array index produces a
//     position_normalized warning so consumers can audit.
func BuildTemplateFromSnapshot(snap editor.InventoryWorkspaceSnapshot, opts ExportOptions) (*BuildTemplate, *ExportReport, error) {
	if !opts.IncludeInventory && !opts.IncludeStorage {
		return nil, nil, fmt.Errorf("BuildTemplateFromSnapshot: at least one of IncludeInventory/IncludeStorage must be true")
	}

	report := &ExportReport{Warnings: []ExportWarning{}}
	section := &InventoryWorkspaceSection{
		InventoryItems: []TemplateItem{},
		StorageItems:   []TemplateItem{},
	}

	if opts.IncludeInventory {
		items, err := convertItems(snap.InventoryItems, ContainerInventory, report)
		if err != nil {
			return nil, nil, err
		}
		section.InventoryItems = items
	}
	if opts.IncludeStorage {
		items, err := convertItems(snap.StorageItems, ContainerStorage, report)
		if err != nil {
			return nil, nil, err
		}
		section.StorageItems = items
	}

	now := opts.Now
	if now.IsZero() {
		now = time.Now()
	}
	tpl := &BuildTemplate{
		Schema:     SchemaKey,
		Version:    SchemaVersion,
		CreatedAt:  now.UTC().Format(time.RFC3339),
		AppVersion: opts.AppVersion,
		Metadata:   opts.Metadata,
		Sections:   TemplateSections{InventoryWorkspace: section},
	}
	return tpl, report, nil
}

// convertItems projects EditableItem rows into TemplateItem rows for one
// container. The input slice is copied before sorting so the caller's
// snapshot is preserved.
func convertItems(items []editor.EditableItem, container string, report *ExportReport) ([]TemplateItem, error) {
	if len(items) == 0 {
		return []TemplateItem{}, nil
	}

	sorted := make([]editor.EditableItem, len(items))
	copy(sorted, items)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].Position < sorted[j].Position
	})

	out := make([]TemplateItem, 0, len(sorted))
	for arrayIdx, it := range sorted {
		if it.BaseItemID == 0 {
			return nil, fmt.Errorf("convertItems[%s]: item %q at array index %d has baseItemID=0", container, it.Name, arrayIdx)
		}
		if it.Quantity == 0 {
			return nil, fmt.Errorf("convertItems[%s]: item %q (baseItemID=0x%08X) at array index %d has quantity=0", container, it.Name, it.BaseItemID, arrayIdx)
		}

		if it.Position != arrayIdx {
			report.Warnings = append(report.Warnings, ExportWarning{
				Code:      WarnCodePositionNormalized,
				UID:       it.UID,
				Container: container,
				Position:  arrayIdx,
				Message:   fmt.Sprintf("position %d normalized to array index %d", it.Position, arrayIdx),
			})
		}

		t := TemplateItem{
			BaseItemID:   it.BaseItemID,
			Name:         it.Name,
			Category:     it.Category,
			Quantity:     it.Quantity,
			Upgrade:      it.CurrentUpgrade,
			InfusionName: it.InfusionName,
			Container:    container,
			Position:     arrayIdx,
		}

		if it.IsWeapon {
			switch it.CurrentAoWStatus {
			case editor.AoWStatusCustom:
				if it.CurrentAoWItemID != 0 {
					id := it.CurrentAoWItemID
					t.AoWItemID = &id
				}
			case editor.AoWStatusMissing:
				report.Warnings = append(report.Warnings, ExportWarning{
					Code:      WarnCodeAoWMissingSkipped,
					UID:       it.UID,
					Container: container,
					Position:  arrayIdx,
					Message:   fmt.Sprintf("weapon %q has a dangling AoW handle; exporting without aowItemID", it.Name),
				})
			case editor.AoWStatusShared:
				report.Warnings = append(report.Warnings, ExportWarning{
					Code:      WarnCodeAoWSharedSkipped,
					UID:       it.UID,
					Container: container,
					Position:  arrayIdx,
					Message:   fmt.Sprintf("weapon %q shares its AoW handle with another weapon; exporting without aowItemID", it.Name),
				})
			}
		}

		out = append(out, t)
	}
	return out, nil
}

// ValidateBuildTemplate enforces the schema invariants without performing
// any DB lookups. It is the gate used at import time to fail fast on
// malformed payloads; DB-level checks (item ID exists, AoW compat) are a
// later phase.
//
// Phase 3A dispatches by Version: the v1 branch remains semantically
// identical to the original validator; the v2 branch is delegated to
// validateBuildTemplateV2 in schema.go. The shared preamble (nil /
// schema / version range) stays here so error messages and ordering are
// unchanged for every v1 caller.
func ValidateBuildTemplate(tpl *BuildTemplate) error {
	if tpl == nil {
		return fmt.Errorf("ValidateBuildTemplate: nil template")
	}
	if tpl.Schema != SchemaKey {
		return fmt.Errorf("ValidateBuildTemplate: wrong schema %q (expected %q)", tpl.Schema, SchemaKey)
	}
	if tpl.Version <= 0 {
		return fmt.Errorf("ValidateBuildTemplate: invalid version %d", tpl.Version)
	}
	if tpl.Version > MaxSchemaVersion {
		return fmt.Errorf("ValidateBuildTemplate: unsupported version %d (max supported %d)", tpl.Version, MaxSchemaVersion)
	}
	if tpl.Version == 1 {
		if tpl.Sections.InventoryWorkspace == nil {
			return fmt.Errorf("ValidateBuildTemplate: missing inventory.workspace section")
		}
		sec := tpl.Sections.InventoryWorkspace
		if len(sec.InventoryItems) == 0 && len(sec.StorageItems) == 0 {
			return fmt.Errorf("ValidateBuildTemplate: inventory.workspace is empty")
		}
		if err := validateItems(sec.InventoryItems, ContainerInventory); err != nil {
			return err
		}
		if err := validateItems(sec.StorageItems, ContainerStorage); err != nil {
			return err
		}
		return nil
	}
	return validateBuildTemplateV2(tpl)
}

func validateItems(items []TemplateItem, expectedContainer string) error {
	for i, it := range items {
		if it.BaseItemID == 0 {
			return fmt.Errorf("validateItems[%s][%d]: baseItemID=0", expectedContainer, i)
		}
		if it.Quantity == 0 {
			return fmt.Errorf("validateItems[%s][%d]: quantity=0 (baseItemID=0x%08X)", expectedContainer, i, it.BaseItemID)
		}
		if it.Container != expectedContainer {
			return fmt.Errorf("validateItems[%s][%d]: container=%q does not match section", expectedContainer, i, it.Container)
		}
	}
	return nil
}
