package editor

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/oisis/EldenRing-SaveForge/backend/db"
	"github.com/oisis/EldenRing-SaveForge/backend/db/data"
)

// newItemUIDPrefix tags every UID minted by AddItem so future Save flows
// can tell "needs handle allocation" items apart from existing ones
// (which use "hnd:0x%08X").
const newItemUIDPrefix = "new:"

// AddItemSpec describes a request to add a new editable item to the
// workspace.
//
// Resolution rules:
//   - ItemID, if non-zero, is treated as the effective (already-encoded)
//     item ID; CurrentUpgrade/InfusionName are decoded from it via the
//     same helper used for existing items.
//   - ItemID == 0 falls back to BaseItemID. No upgrade/infusion encoding
//     happens in Phase 1.6.
//   - Both zero is an error.
//   - Quantity == 0 normalizes to 1 (user-friendly: a new item must hold
//     at least one unit; the workspace has no concept of "zero stack").
//
// Phase 1.6 limitations:
//   - spec.Upgrade / spec.InfusionName / spec.AoWItemID are reserved for
//     Phase 4 weapon-edit encoding and are NOT consulted here. Frontend
//     wishing to add a +N weapon must encode the effective ItemID itself.
//   - No real handle is allocated — OriginalHandle stays 0, HasGaItem is
//     false. The eventual Save step will mint a real handle.
type AddItemSpec struct {
	ItemID       uint32 `json:"itemID"`
	BaseItemID   uint32 `json:"baseItemID"`
	Quantity     uint32 `json:"quantity"`
	Upgrade      int    `json:"upgrade"`
	InfusionName string `json:"infusionName"`
	AoWItemID    uint32 `json:"aowItemID"`
}

// AddItem inserts a new editable item at targetPosition in targetContainer.
//
// Errors:
//   - nil snapshot
//   - targetContainer not "inventory"/"storage"
//   - spec missing both ItemID and BaseItemID
//   - itemID unknown in DB
//   - category outside SupportedCategories (Phase 1 allow-list)
//
// targetPosition is clamped — negative → 0, past length → append.
// Positions are recomputed for both editable slices. Pass-through records
// are never touched. The snapshot is marked Dirty and Validate is re-run.
func AddItem(snap *InventoryWorkspaceSnapshot, spec AddItemSpec, targetContainer ContainerKind, targetPosition int) error {
	if snap == nil {
		return fmt.Errorf("AddItem: nil snapshot")
	}
	if targetContainer != ContainerInventory && targetContainer != ContainerStorage {
		return fmt.Errorf("AddItem: invalid target container %q (want %q or %q)",
			targetContainer, ContainerInventory, ContainerStorage)
	}

	effectiveID := spec.ItemID
	if effectiveID == 0 {
		effectiveID = spec.BaseItemID
	}
	if effectiveID == 0 {
		return fmt.Errorf("AddItem: spec must provide ItemID or BaseItemID")
	}

	itemData, baseID := db.GetItemDataFuzzy(effectiveID)
	if itemData.Name == "" {
		return fmt.Errorf("AddItem: item 0x%08X unknown in DB", effectiveID)
	}
	if !SupportedCategories[itemData.Category] {
		return fmt.Errorf("AddItem: category %q is not editable in Phase 1 (item %s)",
			itemData.Category, itemData.Name)
	}

	qty := spec.Quantity
	if qty == 0 {
		qty = 1
	}

	level, infusion := decodeWeaponUpgradeInfusion(effectiveID, baseID)
	sortKey := data.ItemSortKeys[baseID]
	defaultAoWID, defaultAoWName := defaultAoWForBaseID(baseID)
	baseItem := EditableItem{
		Source:           ItemSourceAdded,
		Container:        targetContainer,
		OriginalHandle:   0,
		ItemID:           effectiveID,
		BaseItemID:       baseID,
		Name:             itemData.Name,
		Category:         itemData.Category,
		Quantity:         qty,
		AcquisitionIndex: 0,
		Weight:           data.ItemWeights[baseID],
		SortID:           sortKey.SortId,
		SortGroupID:      sortKey.SortGroupId,
		CurrentUpgrade:   level,
		MaxUpgrade:       int(itemData.MaxUpgrade),
		InfusionName:     infusion,
		IconPath:         itemData.IconPath,
		HasGaItem:        false,
		IsWeapon:         isWeaponCategory(itemData.Category),
		IsArmor:          isArmorCategory(itemData.Category),
		IsTalisman:       itemData.Category == "talismans",
		DefaultAoWID:     defaultAoWID,
		DefaultAoWName:   defaultAoWName,
	}
	baseItem.OriginalSlotIndex = -1 // added items have no original physical slot
	if baseItem.IsWeapon {
		// Mirror BuildSnapshot — added weapons must carry the same
		// AoW-mounting metadata so the edit modal can compute
		// compatibility without round-tripping through GetCharacter
		// (added items have no save-side GaItemHandle yet). CurrentAoW*
		// stays empty: Source=added items have no current AoW until the
		// workspace is saved and re-loaded from the slot.
		baseItem.WepType = itemData.WepType
		baseItem.CanMountAoW = itemData.GemMountType == 2
	}

	dst := sliceFor(snap, targetContainer)
	if targetPosition < 0 {
		targetPosition = 0
	}
	if targetPosition > len(*dst) {
		targetPosition = len(*dst)
	}

	copies := uint32(1)
	itemQuantity := qty
	if baseItem.IsWeapon || baseItem.IsArmor || baseItem.IsTalisman {
		copies = qty
		itemQuantity = 1
	}
	for i := uint32(0); i < copies; i++ {
		item := baseItem
		item.UID = nextNewUID(snap)
		item.Quantity = itemQuantity
		pos := targetPosition + int(i)
		if pos > len(*dst) {
			pos = len(*dst)
		}
		tail := append([]EditableItem{item}, (*dst)[pos:]...)
		*dst = append((*dst)[:pos], tail...)
	}

	recomputePositions(snap.InventoryItems)
	recomputePositions(snap.StorageItems)

	snap.Dirty = true
	snap.Validation = Validate(*snap)
	return nil
}

// nextNewUID returns the next "new:N" UID that does not collide with any
// existing UID in either editable container. Scans both slices for the
// highest N already in use and returns N+1.
func nextNewUID(snap *InventoryWorkspaceSnapshot) string {
	maxN := 0
	scan := func(items []EditableItem) {
		for _, it := range items {
			if !strings.HasPrefix(it.UID, newItemUIDPrefix) {
				continue
			}
			n, err := strconv.Atoi(it.UID[len(newItemUIDPrefix):])
			if err != nil {
				continue
			}
			if n > maxN {
				maxN = n
			}
		}
	}
	scan(snap.InventoryItems)
	scan(snap.StorageItems)
	return fmt.Sprintf("%s%d", newItemUIDPrefix, maxN+1)
}
