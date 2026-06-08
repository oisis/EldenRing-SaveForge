package templates

import (
	"fmt"
	"sort"

	"github.com/oisis/EldenRing-SaveForge/backend/editor"
)

// Phase 8C — export-only builder for the v2 items / inventoryLayout /
// storageLayout sections. The schema for these sections is locked by
// Phase 8B; this file only translates `editor.EditableItem` rows into
// `TemplateItemEntryV2` entries (plus matching layout rows).
//
// Apply, importer, and writer paths are NOT implemented in Phase 8C.
// The output is suitable for YAML / library export and for round-trip
// validation only.

// ItemsLayoutSource is the neutral input DTO for the v2 items / layout
// export. Producers (App layer in Phase 8C.1, tests today) populate the
// two slices from the live workspace snapshot. The builder does not
// mutate the source; ordering is taken from `EditableItem.Position`
// after a stable sort (mirrors the v1 BuildTemplateFromSnapshot rule).
type ItemsLayoutSource struct {
	InventoryItems []editor.EditableItem
	StorageItems   []editor.EditableItem
}

// EntryID prefixes. Each entry's ID is `<prefix>_<4-digit-zero-padded
// post-sort index>`, e.g. `inv_0000`, `sto_0042`. The padding keeps a
// lexicographic sort identical to the numerical one (handy for diffing
// YAML output) and the prefix lets a human reader tell containers apart
// at a glance. Two copies of the same baseItemID land at different
// sorted indices and therefore receive different entryIDs — that is
// what makes the duplicate-same-item case round-trip faithfully.
const (
	inventoryEntryIDPrefix = "inv"
	storageEntryIDPrefix   = "sto"
)

// ItemsExportReport accumulates per-entry skip notices. The exporter
// never errors out on a single bad row — that would let one stray item
// (unknown category, quantity=0, zero baseItemID) kill a whole-template
// export. Instead the row is dropped and the caller can surface the
// list to the user. The v1 InventoryWorkspace exporter treats the same
// conditions as fatal because its scope is the narrow editable
// allow-list (weapons / armor / talismans); v2 items spans every
// inventory and storage category so per-row skipping is the correct
// default.
type ItemsExportReport struct {
	Skipped []ItemsSkipNotice
}

// ItemsSkipNotice records one dropped row.
type ItemsSkipNotice struct {
	// Container is "inventory" or "storage" — matches Phase 8B item
	// location values.
	Container string
	// UID is the workspace-internal identity (mirrored from
	// EditableItem.UID). Useful for cross-referencing the skip with the
	// SortOrderTab view; never reaches the YAML payload.
	UID string
	// BaseItemID is the offending row's DB-style item ID, or 0 if the
	// reason itself is "baseItemID=0".
	BaseItemID uint32
	// Name is the row's snapshot-time display name (debug only).
	Name string
	// Reason is a short, machine-parseable explanation
	// ("baseItemID=0", "quantity=0", "category=…").
	Reason string
}

// buildItemsAndLayouts emits the items section plus the optional
// inventory / storage layout sections from a single source snapshot.
//
// The returned ItemsSection always carries the post-skip entries for
// both containers (inventory entries first, then storage) in the order
// the entryIDs were generated. Layouts, when requested, reference only
// the entries from their own container — an inventoryLayout entry will
// never point at a `sto_*` entryID, mirroring the save-side fact that
// inventory and storage are different containers with independent
// ordering.
//
// `emitInventoryLayout` / `emitStorageLayout` may be false even when
// the corresponding container has entries — the caller controls layout
// emission via the Selection object, and this helper is purely
// mechanical translation.
//
// Position normalisation strategy: layout positions are compact
// 0..N-1 (after-skip), assigned in sorted-by-EditableItem.Position
// order. This matches the v1 inventory workspace exporter's
// renormalisation rule (`convertItems` re-emits positions as array
// indices) and avoids leaking the workspace's source acquisition-index
// gaps into a portable artifact.
func buildItemsAndLayouts(
	src *ItemsLayoutSource,
	emitInventoryLayout bool,
	emitStorageLayout bool,
) (*ItemsSection, *InventoryLayoutSection, *StorageLayoutSection, *ItemsExportReport) {
	report := &ItemsExportReport{Skipped: []ItemsSkipNotice{}}
	items := &ItemsSection{Entries: []TemplateItemEntryV2{}}
	var invLayout *InventoryLayoutSection
	var stoLayout *StorageLayoutSection
	if emitInventoryLayout {
		invLayout = &InventoryLayoutSection{Entries: []LayoutEntry{}}
	}
	if emitStorageLayout {
		stoLayout = &StorageLayoutSection{Entries: []LayoutEntry{}}
	}

	if src == nil {
		return items, invLayout, stoLayout, report
	}

	invSorted := make([]editor.EditableItem, len(src.InventoryItems))
	copy(invSorted, src.InventoryItems)
	sort.SliceStable(invSorted, func(i, j int) bool {
		return invSorted[i].Position < invSorted[j].Position
	})
	stoSorted := make([]editor.EditableItem, len(src.StorageItems))
	copy(stoSorted, src.StorageItems)
	sort.SliceStable(stoSorted, func(i, j int) bool {
		return stoSorted[i].Position < stoSorted[j].Position
	})

	layoutPos := 0
	for arrayIdx, it := range invSorted {
		entryID := fmt.Sprintf("%s_%04d", inventoryEntryIDPrefix, arrayIdx)
		entry, skip := convertEditableToV2Entry(it, entryID, ItemLocationInventory)
		if skip != nil {
			report.Skipped = append(report.Skipped, *skip)
			continue
		}
		items.Entries = append(items.Entries, entry)
		if invLayout != nil {
			invLayout.Entries = append(invLayout.Entries, LayoutEntry{
				EntryRef: entryID,
				Position: layoutPos,
			})
			layoutPos++
		}
	}

	layoutPos = 0
	for arrayIdx, it := range stoSorted {
		entryID := fmt.Sprintf("%s_%04d", storageEntryIDPrefix, arrayIdx)
		entry, skip := convertEditableToV2Entry(it, entryID, ItemLocationStorage)
		if skip != nil {
			report.Skipped = append(report.Skipped, *skip)
			continue
		}
		items.Entries = append(items.Entries, entry)
		if stoLayout != nil {
			stoLayout.Entries = append(stoLayout.Entries, LayoutEntry{
				EntryRef: entryID,
				Position: layoutPos,
			})
			layoutPos++
		}
	}

	return items, invLayout, stoLayout, report
}

// convertEditableToV2Entry maps one EditableItem to a TemplateItemEntryV2.
// Returns (entry, nil) on success or (zero, notice) when the row cannot
// be represented in the v2 schema (per-row skip — see ItemsExportReport
// doc for the rationale).
//
// The translation is deliberately lossy in places that match the v2
// schema's "portable / informational" contract: handle, UID, acquisition
// index, and pending-AoW state never reach the output. Name is copied
// for human readability but is informational only.
//
// Upgrade kind/level decision table:
//   - IsWeapon=true,  MaxUpgrade=25 → upgradeKind=standard,
//     upgradeLevel=CurrentUpgrade
//   - IsWeapon=true,  MaxUpgrade=10 → upgradeKind=somber,
//     upgradeLevel=CurrentUpgrade
//   - IsWeapon=true,  MaxUpgrade=0  → upgradeKind=none (rare; some
//     unupgradable special weapons)
//   - IsWeapon=false                → upgradeKind left empty
//     (validator treats "" as "none"); upgradeLevel nil
//
// AoW emission mirrors the v1 exporter: only `CurrentAoWStatus=custom`
// with a non-zero `CurrentAoWItemID` reaches the public schema. Status
// `missing` and `shared` are dropped silently — the per-row notice
// stays compact and the workspace UI already surfaces these states.
func convertEditableToV2Entry(it editor.EditableItem, entryID string, location string) (TemplateItemEntryV2, *ItemsSkipNotice) {
	if it.BaseItemID == 0 {
		return TemplateItemEntryV2{}, &ItemsSkipNotice{
			Container: location, UID: it.UID, Name: it.Name, Reason: "baseItemID=0",
		}
	}
	if it.Quantity == 0 {
		return TemplateItemEntryV2{}, &ItemsSkipNotice{
			Container: location, UID: it.UID, BaseItemID: it.BaseItemID, Name: it.Name, Reason: "quantity=0",
		}
	}
	if !itemCategoryAllowlist[it.Category] {
		return TemplateItemEntryV2{}, &ItemsSkipNotice{
			Container: location, UID: it.UID, BaseItemID: it.BaseItemID, Name: it.Name,
			Reason: fmt.Sprintf("category=%q not in v2 allowlist", it.Category),
		}
	}

	entry := TemplateItemEntryV2{
		EntryID:  entryID,
		ItemID:   it.BaseItemID,
		Name:     it.Name,
		Category: it.Category,
		Quantity: it.Quantity,
		Location: location,
	}

	if it.IsWeapon {
		switch it.MaxUpgrade {
		case MaxItemUpgradeStandard:
			entry.UpgradeKind = UpgradeKindStandard
			lvl := uint8(it.CurrentUpgrade)
			entry.UpgradeLevel = &lvl
		case MaxItemUpgradeSomber:
			entry.UpgradeKind = UpgradeKindSomber
			lvl := uint8(it.CurrentUpgrade)
			entry.UpgradeLevel = &lvl
		}
		// MaxUpgrade=0 weapons (or anything else) → upgradeKind stays
		// empty, which the validator reads as "none". No upgradeLevel.

		if it.InfusionName != "" {
			entry.InfusionName = it.InfusionName
		}
		if it.CurrentAoWStatus == editor.AoWStatusCustom && it.CurrentAoWItemID != 0 {
			id := it.CurrentAoWItemID
			entry.AshOfWarItemID = &id
		}
	}

	return entry, nil
}
