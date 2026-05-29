package main

import (
	"fmt"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
	"github.com/oisis/EldenRing-SaveForge/backend/db"
)

// SaveInventoryIntegrityReport describes the outcome of a read-only scan of
// every populated character slot for duplicate inventory acquisition indices.
// The scan covers Inventory.CommonItems and Inventory.KeyItems only — storage
// is intentionally out of scope at this stage. Clean == true means no slot
// holds a conflict and the editor is safe to mutate.
type SaveInventoryIntegrityReport struct {
	Clean bool                           `json:"clean"`
	Slots []SlotInventoryIntegrityReport `json:"slots"`
}

// SlotInventoryIntegrityReport is the per-slot view emitted when the slot
// contains at least one acquisition-index conflict.
//
// DuplicateEntryCount counts the additional occurrences beyond the first for
// every conflicting Index (matches the historical
// core.ScanDuplicateInventoryIndices counter, e.g. an Index that appears 3
// times contributes 2). ConflictingIndexCount counts the distinct Index
// values that participate in a conflict (an Index appearing 3 times
// contributes 1). Slots without conflicts are omitted from the parent report.
//
// Active mirrors a.save.ActiveSlots[i]: true when the in-game character is
// visible in the regular roster, false when the slot is a residual / phantom
// (Version != 0 but the active flag was cleared by an in-game delete). Both
// are included in the scan because both round-trip through WriteSave.
type SlotInventoryIntegrityReport struct {
	SlotIndex             int                          `json:"slotIndex"`
	CharacterName         string                       `json:"characterName"`
	Active                bool                         `json:"active"`
	DuplicateEntryCount   int                          `json:"duplicateEntryCount"`
	ConflictingIndexCount int                          `json:"conflictingIndexCount"`
	Conflicts             []InventoryIntegrityConflict `json:"conflicts"`
}

// InventoryIntegrityConflict groups every inventory record sharing one
// acquisition Index in the same slot. Items contains every participating
// record (including the first occurrence) exactly once, preserving the
// per-scope traversal order (CommonItems first, then KeyItems).
type InventoryIntegrityConflict struct {
	Index uint32                           `json:"index"`
	Items []InventoryIntegrityConflictItem `json:"items"`
}

// InventoryIntegrityConflictItem is one inventory row participating in a
// conflict. Name and Category are populated when the item resolves through
// the DB; Unknown == true signals that the frontend should render a
// hex-ItemID/handle fallback. CurrentUpgrade and InfusionName are populated
// only for weapon-like items (other categories leave them zero-valued).
type InventoryIntegrityConflictItem struct {
	Scope          string `json:"scope"`
	Row            int    `json:"row"`
	Handle         uint32 `json:"handle"`
	ItemID         uint32 `json:"itemId"`
	Name           string `json:"name"`
	Category       string `json:"category"`
	Quantity       int    `json:"quantity"`
	CurrentUpgrade int    `json:"currentUpgrade"`
	InfusionName   string `json:"infusionName"`
	Unknown        bool   `json:"unknown"`
}

const (
	integrityScopeCommon = "inventory_common"
	integrityScopeKey    = "inventory_key"
)

// GetSaveInventoryIntegrityReport scans every populated character slot of the
// active save for duplicate acquisition indices in Inventory.CommonItems and
// Inventory.KeyItems. The endpoint is strictly read-only: it never mutates a
// slot, never pushes undo and never triggers a repair. The frontend uses the
// returned report to decide whether the loaded save is safe to edit.
//
// Locking mirrors GetCharacterNames / AuditLoadedSaveIssues: saveMu.RLock plus
// every slotMu[0..9] in ascending order, so the scan observes a consistent
// snapshot across all slots without contending with a concurrent
// SelectAndOpenSave / DownloadRemoteSave.
func (a *App) GetSaveInventoryIntegrityReport() (SaveInventoryIntegrityReport, error) {
	a.saveMu.RLock()
	defer a.saveMu.RUnlock()
	if a.save == nil {
		return SaveInventoryIntegrityReport{}, fmt.Errorf("no save loaded")
	}
	a.lockAllSlots()
	defer a.unlockAllSlots()

	report := SaveInventoryIntegrityReport{Clean: true}
	for i := range a.save.Slots {
		slot := &a.save.Slots[i]
		if slot.Version == 0 {
			continue
		}
		issues := core.ScanDuplicateInventoryIndices(slot)
		if len(issues) == 0 {
			continue
		}
		report.Clean = false
		report.Slots = append(report.Slots, buildSlotIntegrityReport(a.save, i, slot, issues, a.save.ActiveSlots[i]))
	}
	return report, nil
}

// buildSlotIntegrityReport materialises a per-slot report when at least one
// conflict has been detected. The scanner already identified every duplicate
// row; here we expand the report so each conflict group also carries the
// first occurrence of its Index, in CommonItems-then-KeyItems traversal
// order. Each row is emitted at most once per conflict (relevant when an
// Index recurs three or more times).
func buildSlotIntegrityReport(save *core.SaveFile, slotIdx int, slot *core.SaveSlot, issues []core.DuplicateInventoryIndexIssue, active bool) SlotInventoryIntegrityReport {
	conflictingIndexes := make(map[uint32]struct{}, len(issues))
	for _, issue := range issues {
		conflictingIndexes[issue.Index] = struct{}{}
	}

	conflictByIndex := make(map[uint32]*InventoryIntegrityConflict, len(conflictingIndexes))
	conflictOrder := make([]uint32, 0, len(conflictingIndexes))
	seenRow := make(map[[2]int]bool, len(issues)*2)

	appendRow := func(scope string, row int, item core.InventoryItem) {
		if _, conflicted := conflictingIndexes[item.Index]; !conflicted {
			return
		}
		scopeKey := 0
		if scope == integrityScopeKey {
			scopeKey = 1
		}
		key := [2]int{scopeKey, row}
		if seenRow[key] {
			return
		}
		seenRow[key] = true

		group, ok := conflictByIndex[item.Index]
		if !ok {
			group = &InventoryIntegrityConflict{Index: item.Index}
			conflictByIndex[item.Index] = group
			conflictOrder = append(conflictOrder, item.Index)
		}
		group.Items = append(group.Items, resolveConflictItem(scope, row, item, slot))
	}

	for row, item := range slot.Inventory.CommonItems {
		if item.GaItemHandle == core.GaHandleEmpty || item.GaItemHandle == core.GaHandleInvalid {
			continue
		}
		appendRow(integrityScopeCommon, row, item)
	}
	for row, item := range slot.Inventory.KeyItems {
		if item.GaItemHandle == core.GaHandleEmpty || item.GaItemHandle == core.GaHandleInvalid {
			continue
		}
		appendRow(integrityScopeKey, row, item)
	}

	conflicts := make([]InventoryIntegrityConflict, 0, len(conflictOrder))
	for _, idx := range conflictOrder {
		conflicts = append(conflicts, *conflictByIndex[idx])
	}

	return SlotInventoryIntegrityReport{
		SlotIndex:             slotIdx,
		CharacterName:         resolveCharacterName(save, slotIdx),
		Active:                active,
		DuplicateEntryCount:   len(issues),
		ConflictingIndexCount: len(conflictingIndexes),
		Conflicts:             conflicts,
	}
}

// resolveConflictItem enriches one inventory row with DB-derived display
// fields. The lookup chain mirrors getInventoryOrderLocked: prefer the
// GaMap entry (so weapons/armor/AoW report their *encoded* ItemID, which
// embeds upgrade level and infusion), fall back to HandleToItemID for
// vanilla handle-encoded records (talismans/goods/keys). When the DB does
// not know the resolved ItemID (Name == ""), the row is flagged Unknown so
// the frontend can render a hex-ID/handle fallback without dropping the
// row from the report.
func resolveConflictItem(scope string, row int, item core.InventoryItem, slot *core.SaveSlot) InventoryIntegrityConflictItem {
	itemID, ok := slot.GaMap[item.GaItemHandle]
	if !ok {
		itemID = db.HandleToItemID(item.GaItemHandle)
	}
	itemData, baseID := db.GetItemDataFuzzy(itemID)

	result := InventoryIntegrityConflictItem{
		Scope:    scope,
		Row:      row,
		Handle:   item.GaItemHandle,
		ItemID:   itemID,
		Quantity: int(item.Quantity & 0x7FFFFFFF),
	}

	if itemData.Name == "" {
		result.Unknown = true
		return result
	}

	result.Name = itemData.Name
	result.Category = itemData.Category
	if isWeaponLikeCategory(itemData.Category) {
		result.CurrentUpgrade, result.InfusionName = decodeWeaponUpgradeInfusion(itemID, baseID)
	}
	return result
}

// resolveCharacterName mirrors GetCharacterNames: read from the slot's player
// block first, fall back to the profile summary, and yield "" if both are
// blank so the frontend can render its own placeholder if it wishes.
func resolveCharacterName(save *core.SaveFile, slotIdx int) string {
	if name := core.UTF16ToString(save.Slots[slotIdx].Player.CharacterName[:]); name != "" {
		return name
	}
	return core.UTF16ToString(save.ProfileSummaries[slotIdx].CharacterName[:])
}

// isWeaponLikeCategory selects the categories whose encoded ItemID embeds an
// upgrade level and an optional infusion offset (the only categories for
// which decodeWeaponUpgradeInfusion returns meaningful values).
func isWeaponLikeCategory(category string) bool {
	switch category {
	case "melee_armaments", "ranged_and_catalysts", "shields":
		return true
	default:
		return false
	}
}
